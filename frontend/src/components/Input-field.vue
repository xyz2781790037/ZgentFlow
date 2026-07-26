<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch, nextTick, h } from "vue";
import { storeToRefs } from 'pinia';
import { useRoute, useRouter } from 'vue-router';
import { onBeforeRouteUpdate } from 'vue-router';
import { MessagePlugin } from "tdesign-vue-next";
import { useSettingsStore } from '@/stores/settings';
import { useUIStore } from '@/stores/ui';
import { useMenuStore } from '@/stores/menu';
import { listKnowledgeBases, searchKnowledge, batchQueryKnowledge } from '@/api/knowledge-base';
import { stopSession } from '@/api/chat';
import KnowledgeBaseSelector from './KnowledgeBaseSelector.vue';
import MentionSelector from './MentionSelector.vue';

import { getCaretCoordinates } from '@/utils/caret';
import { getRootZoom, rectToCssPx, cssViewportSize } from '@/utils/zoom';
import {
  BUILTIN_QUICK_ANSWER_ID,
  type AnswerModeConfig,
} from '@/api/agent';
import { useChatResourcesStore } from '@/stores/chatResources';
import { useI18n } from 'vue-i18n';
import AttachmentUpload, { type AttachmentFile } from './AttachmentUpload.vue';
import {
  kbSatisfiesAgentRequirements,
  deriveKbFilterForAgent,
  toolsConsumeFiles,
  type ScopeCapabilities,
} from '@/utils/tool-capabilities';

const route = useRoute();
const router = useRouter();
const settingsStore = useSettingsStore();
const uiStore = useUIStore();
const menuStore = useMenuStore();
const chatResources = useChatResourcesStore();
const {
  agents,
  webSearchProviders,
} = storeToRefs(chatResources);
const { t } = useI18n();

let query = ref("");
const showKbSelector = ref(false);

// Image upload state
const uploadedImages = ref<Array<{ file: File; preview: string }>>([]);
const imageInputRef = ref<HTMLInputElement>();
const imageUploading = ref(false);

// Attachment upload state
const attachmentUploadRef = ref<InstanceType<typeof AttachmentUpload>>();
const uploadedAttachments = ref<AttachmentFile[]>([]);
const CHAT_FILE_DROP_EVENT = 'zealrag:chat-file-drop';

const isImageFile = (file: File) => {
  if (file.type.startsWith('image/')) {
    return true;
  }
  const fileName = file.name.toLowerCase();
  return ['.jpg', '.jpeg', '.png', '.gif', '.webp', '.bmp'].some(ext => fileName.endsWith(ext));
};

const handleDroppedFiles = (files: File[]) => {
  if (!files.length) return;

  const imageFiles = files.filter(isImageFile);
  const attachmentFiles = files.filter(file => !isImageFile(file));

  if (imageFiles.length > 0) {
    if (isImageUploadEnabledByAgent.value) {
      addImageFiles(imageFiles);
    } else {
      MessagePlugin.warning(t('input.imageUploadDisabledByAgent'));
    }
  }

  if (attachmentFiles.length > 0) {
    attachmentUploadRef.value?.addFiles(attachmentFiles);
  }
};

const handleChatFileDrop = (event: Event) => {
  const customEvent = event as CustomEvent<{ files?: File[] }>;
  const files = customEvent.detail?.files;
  if (!files || files.length === 0) return;
  handleDroppedFiles(files);
};

const handleImageSelect = (event: Event) => {
  const input = event.target as HTMLInputElement;
  if (!input.files) return;
  addImageFiles(Array.from(input.files));
  input.value = '';
};

const addImageFiles = (files: File[]) => {
  if (!isImageUploadEnabledByAgent.value) return;
  const allowed = ['image/jpeg', 'image/png', 'image/gif', 'image/webp'];
  const maxSize = 10 * 1024 * 1024;
  for (const file of files) {
    if (uploadedImages.value.length >= 5) {
      MessagePlugin.warning(t('chat.imageTooMany'));
      break;
    }
    if (!allowed.includes(file.type)) {
      MessagePlugin.warning(t('chat.imageTypeSizeError'));
      continue;
    }
    if (file.size > maxSize) {
      MessagePlugin.warning(t('chat.imageTypeSizeError'));
      continue;
    }
    uploadedImages.value.push({ file, preview: URL.createObjectURL(file) });
  }
};

const removeImage = (index: number) => {
  const removed = uploadedImages.value.splice(index, 1);
  if (removed.length > 0) URL.revokeObjectURL(removed[0].preview);
};

const triggerImageUpload = () => {
  imageInputRef.value?.click();
};
const atButtonRef = ref<HTMLElement>();

const selectedAgentId = computed(() => settingsStore.selectedAgentId || BUILTIN_QUICK_ANSWER_ID);
// 判断当前内置问答模式是否已加载运行配置。
const hasAgentConfig = computed(() => {
  return !!agents.value.find(a => a.id === selectedAgentId.value)?.config;
});

// 获取当前内置问答模式的实际配置。
const currentAgentConfig = computed<Partial<AnswerModeConfig>>(() => {
  return agents.value.find(a => a.id === selectedAgentId.value)?.config || {};
});

// 问答模式预配置的知识库 IDs
const agentKnowledgeBases = computed(() => {
  if (!hasAgentConfig.value) return [];
  return currentAgentConfig.value?.knowledge_bases || [];
});

// 问答模式的知识库选择范围
const agentKBSelectionMode = computed(() => {
  if (!hasAgentConfig.value) return null; // null 表示不受智能体控制
  return currentAgentConfig.value?.kb_selection_mode || 'all';
});

// 当问答模式改变时，模型、网络搜索、可@知识库列表均跟随新配置。
watch([selectedAgentId, agentKnowledgeBases, agentKBSelectionMode], ([newAgentId, newAgentKbs, newKbMode], [oldAgentId]) => {
  if (settingsStore._isApplyingSessionState) return;
  if (newAgentId !== oldAgentId && oldAgentId !== undefined) {
    if (newKbMode === 'none') {
      settingsStore.selectKnowledgeBases([]);
    } else {
      settingsStore.selectKnowledgeBases(newAgentKbs && newAgentKbs.length > 0 ? [...newAgentKbs] : []);
    }
    // 若 @ 面板已打开，刷新可@列表以立即反映新模式的知识库范围
    if (showMention.value) {
      loadMentionItems(mentionQuery.value, true);
    }
    // 切换到不支持图片的模式时清空待上传图片
    if (!isImageUploadEnabledByAgent.value && uploadedImages.value.length > 0) {
      uploadedImages.value.forEach(img => URL.revokeObjectURL(img.preview));
      uploadedImages.value = [];
    }
  }
}, { immediate: true });

// 当前模式是否启用了网络搜索
const agentWebSearchEnabled = computed(() => {
  if (!hasAgentConfig.value) return null; // null 表示不受智能体控制
  return currentAgentConfig.value?.web_search_enabled ?? true;
});

const agentWebSearchProviderId = computed(() => {
  if (!hasAgentConfig.value) return '';
  return currentAgentConfig.value?.web_search_provider_id || '';
});

// 网络搜索是否被当前模式禁用。
const isWebSearchDisabledByAgent = computed(() => {
  return hasAgentConfig.value && agentWebSearchEnabled.value === false;
});

// 如果模式配置了 kb_selection_mode = 'none'，完全禁用知识库。
// 其他情况用户都可以在允许的范围内通过 @ 选择知识库
const isKnowledgeBaseLockedByAgent = computed(() => {
  if (!hasAgentConfig.value) return false;
  // 只有禁用了知识库才锁定
  return agentKBSelectionMode.value === 'none';
});

// 知识库是否被当前模式完全禁用。
const isKnowledgeBaseDisabledByAgent = computed(() => {
  if (!hasAgentConfig.value) return false;
  return agentKBSelectionMode.value === 'none';
});

// 当前模式的工具列表，驱动 @ 菜单的 KB 兼容性过滤。
const agentAllowedTools = computed<string[]>(() => {
  if (!hasAgentConfig.value) return [];
  return currentAgentConfig.value?.allowed_tools || [];
});

// 从 KB 对象里抽能力位，优先用 backend 显式的 capabilities 字段；否则回退到 indexing_strategy，
// 最后拿 kb.type === 'faq' 兜底。shared / owned / agent-scope 三路的 KB 响应结构一致。
const kbToScopeCaps = (kb: any): Partial<ScopeCapabilities> => {
  if (kb?.capabilities) {
    return {
      vector: !!kb.capabilities.vector,
      keyword: !!kb.capabilities.keyword,
      wiki: !!kb.capabilities.wiki,
      faq: !!kb.capabilities.faq,
    };
  }
  const s = kb?.indexing_strategy;
  return {
    vector: s ? !!s.vector_enabled : false,
    keyword: s ? !!s.keyword_enabled : false,
    wiki: s ? !!s.wiki_enabled : false,
    faq: kb?.type === 'faq',
  };
};

// 当前问答模式（quick-answer / smart-reasoning），用于把
// "RAG-only 模式不能 @ wiki-only 知识库"这种隐式约束带进 KB 过滤。
const agentMode = computed(() => {
  if (!hasAgentConfig.value) return '';
  return currentAgentConfig.value?.agent_mode || '';
});

// "all" 模式 + 工具有 KB 依赖时的兼容性过滤；'selected'/'none' 不在这里二次过滤
// （selected 由编辑器负责，none 已经空表）。
const isKbCompatibleWithAgent = (kb: any): boolean => {
  if (!hasAgentConfig.value) return true;
  if (agentKBSelectionMode.value !== 'all') return true;
  return kbSatisfiesAgentRequirements(kbToScopeCaps(kb), agentMode.value, agentAllowedTools.value);
};

// 仅在用户没输入搜索词、且是因智能体工具兼容性把列表清空的场景展示专用空态文案
const mentionEmptyHint = computed(() => {
  if (mentionQuery.value) return '';
  if (!hasAgentConfig.value) return '';
  if (agentKBSelectionMode.value !== 'all') return '';
  // 列表为空 && 兼容性过滤器其实是有效的（否则"全部"不会被剔空）
  if (mentionItems.value.length !== 0) return '';
  const filter = deriveKbFilterForAgent(agentMode.value, agentAllowedTools.value);
  if (!filter) return '';
  return t('mentionDetail.noCompatibleKbForAgent');
});

// 当前模式是否启用了图片上传（多模态）。
const isImageUploadEnabledByAgent = computed(() => {
  if (!hasAgentConfig.value) return false;
  return currentAgentConfig.value?.image_upload_enabled === true;
});

// Mention related state
const showMention = ref(false);
const mentionQuery = ref("");
const mentionItems = ref<Array<{ id: string; name: string; type: 'kb' | 'file'; kbType?: 'document' | 'faq'; count?: number; kbName?: string; orgName?: string; kbId?: string }>>([]);
/** 文件 ID -> 知识库 ID（用于批量查询时传 kb_id）。 */
const fileIdToKbId = ref<Record<string, string>>({});
const mentionActiveIndex = ref(0);
const mentionStyle = ref<Record<string, string>>({});
const textareaRef = ref<any>(null); // Ref to t-textarea component
const mentionStartPos = ref(0);
const isComposing = ref(false);
const isMentionTriggeredByButton = ref(false);
const mentionHasMore = ref(false);
// 当前 @ 会话可见的 KB ID 集合（含工具兼容性过滤），分页加载文件时复用，
// 避免 append 请求把不兼容 KB 的文件漏进来。`null` 表示"不受限制"（非智能体场景）
const mentionAllowedKbIds = ref<Set<string> | null>(null);
const mentionLoading = ref(false);
const mentionOffset = ref(0);
const MENTION_PAGE_SIZE = 20;

const props = defineProps({
  isReplying: {
    type: Boolean,
    required: false
  },
  sessionId: {
    type: String,
    required: false
  },
  assistantMessageId: {
    type: String,
    required: false
  }
});

const isWebSearchEnabled = computed(() => settingsStore.isWebSearchEnabled);
const selectedKbIds = computed(() => settingsStore.settings.selectedKnowledgeBases || []);
const selectedFileIds = computed(() => settingsStore.settings.selectedFiles || []);

// 已就绪的知识库（来自租户级缓存）
const knowledgeBases = computed(() => chatResources.validKnowledgeBases);
const fileList = ref<Array<{ id: string; name: string }>>([]);

const selectedKbs = computed(() =>
  knowledgeBases.value.filter(kb => selectedKbIds.value.includes(kb.id))
);

const selectedFiles = computed(() => {
  // If we have file details in fileList, use them.
  // Otherwise we might show ID or Loading...
  return selectedFileIds.value.map((id: string) => {
    const found = fileList.value.find(f => f.id === id);
    return found || { id, name: 'Loading...' };
  });
});

// 合并所有选中项（用于输入框内显示）
// 现在智能体配置的知识库也在 store 中，统一从 selectedKbs 获取
const allSelectedItems = computed(() => {
  // 获取智能体预配置的知识库 IDs（用于标记和排序）
  const agentKbIds = agentKnowledgeBases.value;

  // 所有选中的知识库，标记是否为智能体配置
  const allKbs = selectedKbs.value.map(kb => ({
    ...kb,
    type: 'kb' as const,
    kbType: kb.type,
    isAgentConfigured: agentKbIds.includes(kb.id)
  }));

  const files = selectedFiles.value.map((f: { id: string; name: string }) => ({
    ...f,
    type: 'file' as const,
    isAgentConfigured: false,
  }));

  // 智能体配置的放在前面
  const agentConfiguredKbs = allKbs.filter(kb => kb.isAgentConfigured);
  const userSelectedKbs = allKbs.filter(kb => !kb.isAgentConfigured);

  return [...agentConfiguredKbs, ...userSelectedKbs, ...files];
});

// 移除选中项（智能体配置的项也可以移除）
const removeSelectedItem = (item: { id: string; type: 'kb' | 'file'; isAgentConfigured?: boolean }) => {
  if (item.type === 'kb') {
    settingsStore.removeKnowledgeBase(item.id);
  } else {
    settingsStore.removeFile(item.id);
  }
};

// 显示的知识库标签（最多显示2个）
const displayedKbs = computed(() => selectedKbs.value.slice(0, 2));
const remainingCount = computed(() => Math.max(0, selectedKbs.value.length - 2));

// 根据不同状态组合计算输入框的 placeholder
const inputPlaceholder = computed(() => {
  const hasKnowledge = allSelectedItems.value.length > 0;
  const hasWebSearch = isWebSearchEnabled.value && isWebSearchConfigured.value;

  if (hasKnowledge && hasWebSearch) {
    // 有知识库 + 有网络搜索
    return t('input.placeholderKbAndWeb');
  } else if (hasKnowledge) {
    // 有知识库 + 无网络搜索
    return t('input.placeholderWithContext');
  } else if (hasWebSearch) {
    // 无知识库 + 有网络搜索
    return t('input.placeholderWebOnly');
  } else {
    // 无知识库 + 无网络搜索（纯模型对话）
    return t('input.placeholder');
  }
});

// 加载当前工作区知识库列表（用于 @ 提及等）。
const loadKnowledgeBases = async (force = false) => {
  try {
    await chatResources.ensureKnowledgeBases(force);
    const validKbs = knowledgeBases.value;

    const validKbIds = new Set(validKbs.map((kb: any) => kb.id));
    const currentSelectedIds = settingsStore.settings.selectedKnowledgeBases || [];
    const validSelectedIds = currentSelectedIds.filter((id: string) => validKbIds.has(id));

    if (validSelectedIds.length !== currentSelectedIds.length) {
      settingsStore.selectKnowledgeBases(validSelectedIds);
    }
  } catch (error) {
    console.error('Failed to load knowledge bases:', error);
  }
};

const loadFiles = async () => {
  const ids = selectedFileIds.value;
  if (ids.length === 0) return;

  const missingIds = ids.filter((id: string) => !fileList.value.find(f => f.id === id));
  if (missingIds.length === 0) return;

  try {
    // 按 kb_id 分组，避免跨知识库查询歧义。
    const byKbId = new Map<string, string[]>();
    const noKbId: string[] = [];
    missingIds.forEach((id: string) => {
      const kbId = fileIdToKbId.value[id];
      if (kbId) {
        if (!byKbId.has(kbId)) byKbId.set(kbId, []);
        byKbId.get(kbId)!.push(id);
      } else {
        noKbId.push(id);
      }
    });

    const allNewFiles: Array<{ id: string; name: string }> = [];
    const runBatch = async (batchIds: string[], kbId?: string) => {
      const query = new URLSearchParams();
      batchIds.forEach((id: string) => query.append('ids', id));
      const res: any = await batchQueryKnowledge(query.toString(), kbId);
      if (res.data && Array.isArray(res.data)) {
        res.data.forEach((f: any) => allNewFiles.push({ id: f.id, name: f.title || f.file_name }));
      }
    };

    for (const [kbId, batchIds] of byKbId) {
      await runBatch(batchIds, kbId);
    }
    if (noKbId.length > 0) {
      await runBatch(noKbId);
    }
    if (allNewFiles.length > 0) {
      fileList.value = [...fileList.value, ...allNewFiles];
    }
  } catch (e) {
    console.error("Failed to load files", e);
  }
};

watch(selectedFileIds, () => {
  loadFiles();
}, { immediate: true });

const isWebSearchConfigured = computed(() => {
  const agentProviderId = agentWebSearchProviderId.value;
  if (agentProviderId) {
    return webSearchProviders.value.some(p => p.id === agentProviderId);
  }

  return webSearchProviders.value.some(p => p.is_default);
});

const loadWebSearchConfig = async (force = false) => {
  try {
    await chatResources.ensureWebSearchProviders(force);

    if (!isWebSearchConfigured.value && settingsStore.isWebSearchEnabled) {
      settingsStore.toggleWebSearch(false);
    }
  } catch (error) {
    console.error('Failed to load web search config:', error);
    chatResources.invalidate('webSearchProviders');
    if (settingsStore.isWebSearchEnabled) {
      settingsStore.toggleWebSearch(false);
    }
  }
};

// 加载快速/深度问答的运行配置。
const loadAgents = async (force = false) => {
  try {
    await chatResources.ensureAgents(force);
  } catch (error) {
    console.error('Failed to load answer modes:', error);
  }
};

// Mention Logic
let lastMentionQuery = '';
const loadMentionItems = async (q: string, resetIndex = true, append = false) => {
  console.log('[Mention] loadMentionItems called with query:', q, 'append:', append);

  if (!append) {
    mentionOffset.value = 0;
  }

  // 根据智能体的 kb_selection_mode 过滤当前工作区知识库。
  let kbItems: any[] = [];
  if (!append) {
    let availableKbs: any[] = [...knowledgeBases.value];

    if (hasAgentConfig.value) {
      const kbMode = agentKBSelectionMode.value;
      if (kbMode === 'none') {
        availableKbs = [];
      } else if (kbMode === 'selected') {
        const configuredKbIds = agentKnowledgeBases.value;
        availableKbs = availableKbs.filter((kb: any) => configuredKbIds.includes(kb.id));
      } else if (kbMode === 'all') {
        availableKbs = availableKbs.filter((kb: any) => isKbCompatibleWithAgent(kb));
      }
    }

    // 非智能体场景不限制文件过滤；智能体场景按当前 availableKbs 的 ID 集合过滤文件
    mentionAllowedKbIds.value = hasAgentConfig.value
      ? new Set(availableKbs.map((kb: any) => String(kb.id)))
      : null;

    const kbs = availableKbs.filter((kb: any) =>
      !q || (kb.name && kb.name.toLowerCase().includes(q.toLowerCase()))
    );
    kbItems = kbs.map((kb: any) => ({
      id: kb.id,
      name: kb.name,
      type: 'kb' as const,
      kbType: kb.type || 'document',
      count: kb.type === 'faq' ? (kb.chunk_count || 0) : (kb.knowledge_count || 0),
    }));
  }

  // Fetch Files from API
  // 仅当满足以下两点才加载文件：
  //   1. 智能体确实会用到知识库（kb_selection_mode !== 'none'）；
  //   2. 智能体启用的工具里至少有一个能消费 @ 的文件 ID
  //      （比如 wiki-qa 全是 wiki_* 工具，用户 @ 的文件根本进不到任何工具里，就没必要展示）。
  let fileItems: any[] = [];
  const kbModeAllowsFiles = !hasAgentConfig.value || agentKBSelectionMode.value !== 'none';
  const toolsAllowFiles = !hasAgentConfig.value || toolsConsumeFiles(agentAllowedTools.value);
  const shouldLoadFiles = kbModeAllowsFiles && toolsAllowFiles;

  // 后端 /knowledge/search 要求非空 keyword；打开 @ 面板时仅展示本地 KB 列表，
  // 用户输入搜索词后再拉取文件结果。
  const fileSearchKeyword = q.trim();
  if (shouldLoadFiles && fileSearchKeyword) {
    mentionLoading.value = true;
    try {
      const res: any = await searchKnowledge(
        fileSearchKeyword,
        mentionOffset.value,
        MENTION_PAGE_SIZE
      );
      console.log('[Mention] searchKnowledge response:', res);
      if (res.data && Array.isArray(res.data)) {
        let files = res.data;
        // 按当前 @ 会话的兼容 KB 集合过滤：
        //   - 非智能体场景：`mentionAllowedKbIds` 为 null，跳过；
        //   - 智能体场景：'selected' 会把 ID 收敛到用户勾的 KB，
        //     'all' 会收敛到"兼容"的 KB，'none' 根本走不到这里（shouldLoadFiles=false）。
        //   这样分页 append 也能用同一份集合，不再只兜住 'selected' + 非共享的分支。
        if (mentionAllowedKbIds.value) {
          const allowed = mentionAllowedKbIds.value;
          files = files.filter((f: any) => {
            const kbId = f.knowledge_base_id ?? f.kb_id;
            return kbId != null && allowed.has(String(kbId));
          });
        }
        fileItems = files.map((f: any) => {
          const kbId = f.knowledge_base_id ?? f.kb_id;
          return {
            id: f.id,
            name: f.title || f.file_name,
            type: 'file' as const,
            kbName: f.knowledge_base_name || '',
            kbId: kbId || undefined,
          };
        });
      }
      mentionHasMore.value = res.has_more || false;
      mentionOffset.value += fileItems.length;
    } catch (e) {
      console.error('[Mention] searchKnowledge error:', e);
      mentionHasMore.value = false;
    } finally {
      mentionLoading.value = false;
    }
  } else {
    mentionHasMore.value = false;
  }

  if (append) {
    // Append file items to existing list
    mentionItems.value = [...mentionItems.value, ...fileItems];
  } else {
    mentionItems.value = [...kbItems, ...fileItems];
  }
  console.log('[Mention] Total items:', mentionItems.value.length, { kbItems: kbItems.length, fileItems: fileItems.length });

  // Only reset index if query changed or explicitly requested
  if (resetIndex || q !== lastMentionQuery) {
    mentionActiveIndex.value = 0;
  }
  // Ensure index is within bounds
  if (mentionActiveIndex.value >= mentionItems.value.length) {
    mentionActiveIndex.value = Math.max(0, mentionItems.value.length - 1);
  }
  lastMentionQuery = q;
};

const loadMoreMentionItems = () => {
  if (mentionHasMore.value && !mentionLoading.value) {
    loadMentionItems(lastMentionQuery, false, true);
  }
};

const getTextareaEl = () => {
  if (!textareaRef.value) return null;
  // If it's a native element
  if (textareaRef.value instanceof HTMLTextAreaElement) return textareaRef.value;
  // If it's a component wrapper
  const el = textareaRef.value.$el || textareaRef.value;
  if (!el) return null;
  if (el.tagName === 'TEXTAREA') return el as HTMLTextAreaElement;
  return el.querySelector('textarea');
};

const onInput = (val: string | InputEvent) => {
  // 如果正在输入法组合中，不处理搜索逻辑，等待 compositionend
  if (isComposing.value) return;

  // TDesign t-textarea passes the value directly, not an event
  const inputVal = typeof val === 'string' ? val : query.value;

  const textarea = getTextareaEl();
  if (!textarea) {
    console.warn('[Mention] Could not get textarea element');
    return;
  }

  const cursor = textarea.selectionStart;
  const textBeforeCursor = inputVal.slice(0, cursor);

  console.log('[Mention] onInput called', { inputVal, cursor, textBeforeCursor, showMention: showMention.value });

  if (showMention.value) {
    // 如果不是按钮触发的，检查 @ 符号
    if (!isMentionTriggeredByButton.value) {
      if (!inputVal || inputVal.length <= mentionStartPos.value || inputVal.charAt(mentionStartPos.value) !== '@') {
        showMention.value = false;
        return;
      }
    }

    // 如果是按钮触发的，mentionStartPos 指向的是光标位置（即虚拟的 @ 位置前），所以实际上不应该往左删
    // 但如果用户删除了前面的内容导致长度变短，也需要处理
    if (cursor < mentionStartPos.value) {
      showMention.value = false;
      return;
    }

    // Get query
    // 如果是按钮触发，mentionStartPos 是起始位置，不需要 +1 跳过 @
    const start = isMentionTriggeredByButton.value ? mentionStartPos.value : mentionStartPos.value + 1;
    const q = inputVal.slice(start, cursor);

    if (q.includes(' ')) {
      showMention.value = false;
      return;
    }
    // Only reload if query changed
    if (q !== mentionQuery.value) {
      mentionQuery.value = q;
      loadMentionItems(q, true); // Reset index when query changes
    }
  } else {
    if (textBeforeCursor.endsWith('@')) {
      // 如果智能体禁用了知识库，不触发 @ 菜单
      if (isKnowledgeBaseDisabledByAgent.value) {
        return;
      }
      // 如果智能体锁定了知识库且不允许用户选择，也不触发 @ 菜单
      if (isKnowledgeBaseLockedByAgent.value) {
        return;
      }

      console.log('[Mention] @ detected, opening menu');
      isMentionTriggeredByButton.value = false;
      mentionStartPos.value = cursor - 1;
      showMention.value = true;
      mentionQuery.value = "";

      const coords = getCaretCoordinates(textarea, cursor);
      // Normalize coordinates to CSS pixels (root <html> may carry `zoom`).
      const zoom = getRootZoom();
      const rect = rectToCssPx(textarea.getBoundingClientRect(), zoom);
      const { width: vw, height: vh } = cssViewportSize(zoom);
      const scrollTop = textarea.scrollTop;
      const menuHeight = 320; // 预估最大高度

      let left = rect.left + coords.left;
      // Prevent menu from going off-screen horizontally
      if (left + 300 > vw) {
        left = vw - 300 - 10;
      }

      // 光标相对于视口的实际 top 位置（CSS 像素）
      const cursorAbsoluteTop = rect.top + coords.top - scrollTop;
      const lineHeight = coords.height; // 光标高度

      // Check vertical space below cursor
      const spaceBelow = vh - (cursorAbsoluteTop + lineHeight);

      if (spaceBelow < menuHeight && cursorAbsoluteTop > menuHeight) {
        // Show above cursor (using bottom positioning)
        const bottom = vh - cursorAbsoluteTop;
        mentionStyle.value = {
          left: `${left}px`,
          bottom: `${bottom}px`,
          top: 'auto'
        };
      } else {
        // Show below cursor (using top positioning)
        const top = cursorAbsoluteTop + lineHeight;
        mentionStyle.value = {
          left: `${left}px`,
          top: `${top}px`,
          bottom: 'auto'
        };
      }

      loadMentionItems("");
    }
  }
};

const onCompositionStart = () => {
  isComposing.value = true;
};

const onCompositionEnd = (e: CompositionEvent) => {
  isComposing.value = false;
  // 手动触发 onInput 逻辑
  // 注意：在 compositionend 时，v-model 可能还没更新，或者已经更新但我们需要用最新值
  // TDesign textarea 可能需要 nextTick
  nextTick(() => {
    onInput(query.value);
  });
};

const triggerMention = () => {
  // 如果智能体锁定或禁用了知识库，不允许打开选择器
  if (isKnowledgeBaseLockedByAgent.value) {
    const msgKey = isKnowledgeBaseDisabledByAgent.value ? 'input.kbDisabledByAgent' : 'input.kbLockedByAgent';
    MessagePlugin.warning(t(msgKey));
    return;
  }

  const textarea = getTextareaEl();
  if (!textarea) return;

  textarea.focus();

  // 直接显示菜单，不插入 @
  showMention.value = true;
  isMentionTriggeredByButton.value = true;
  mentionQuery.value = "";
  mentionStartPos.value = textarea.selectionStart;

  // Normalize coordinates to CSS pixels (root <html> may carry `zoom`).
  const zoom = getRootZoom();
  const rect = rectToCssPx(textarea.getBoundingClientRect(), zoom);
  const { height: vh } = cssViewportSize(zoom);
  const menuHeight = 320;

  // 判断输入框上方空间
  const spaceAbove = rect.top;
  const spaceBelow = vh - rect.bottom;

  // 优先显示在上方，除非上方空间不足且下方空间充足
  if (spaceAbove > menuHeight || spaceAbove > spaceBelow) {
    // Show above textarea
    mentionStyle.value = {
      left: `${rect.left}px`,
      bottom: `${vh - rect.top + 8}px`, // 8px padding
      top: 'auto'
    };
  } else {
    // Show below textarea
    mentionStyle.value = {
      left: `${rect.left}px`,
      top: `${rect.bottom + 8}px`,
      bottom: 'auto'
    };
  }

  loadMentionItems("");
};

const onMentionSelect = (item: any) => {
  if (item.type === 'kb') {
    settingsStore.addKnowledgeBase(item.id);
  } else if (item.type === 'file') {
    settingsStore.addFile(item.id);
    if (item.kbId) {
      fileIdToKbId.value[item.id] = item.kbId;
      settingsStore.setFileKbMap({ [item.id]: item.kbId });
    }
    // Add to local cache immediately
    if (!fileList.value.find(f => f.id === item.id)) {
      fileList.value.push({ id: item.id, name: item.name });
    }
  }

  const textarea = getTextareaEl();
  if (textarea) {
    // 如果是通过输入 @ 触发的，需要删除 @ 和后面的查询文字
    if (!isMentionTriggeredByButton.value) {
      const cursor = textarea.selectionStart;
      const textBeforeAt = query.value.slice(0, mentionStartPos.value);
      const textAfterCursor = query.value.slice(cursor);
      query.value = textBeforeAt + textAfterCursor;

      nextTick(() => {
        textarea.selectionStart = textarea.selectionEnd = mentionStartPos.value;
        textarea.focus();
      });
    } else {
      // 通过按钮触发的，如果用户输入了查询词，需要删除查询词
      const cursor = textarea.selectionStart;
      if (cursor > mentionStartPos.value) {
        const textBeforeStart = query.value.slice(0, mentionStartPos.value);
        const textAfterCursor = query.value.slice(cursor);
        query.value = textBeforeStart + textAfterCursor;

        nextTick(() => {
          textarea.selectionStart = textarea.selectionEnd = mentionStartPos.value;
          textarea.focus();
        });
      } else {
        // 直接聚焦
        textarea.focus();
      }
    }
  }

  showMention.value = false;
};

const removeFile = (id: string) => {
  settingsStore.removeFile(id);
  delete fileIdToKbId.value[id];
};

const closeMentionSelector = (e: MouseEvent) => {
  const target = e.target as HTMLElement;
  // 如果点击的是输入框区域，不关闭 Mention 列表（由光标逻辑控制）
  if (target.closest('.rich-input-container')) {
    return;
  }
  showMention.value = false;
};

onMounted(() => {
  // 并行拉取；若 platform 已预取且缓存未过期则直接复用
  void Promise.all([
    loadKnowledgeBases(),
    loadWebSearchConfig(),
    loadAgents(),
  ]);
  window.addEventListener(CHAT_FILE_DROP_EVENT, handleChatFileDrop as EventListener);

  // 从持久化恢复 fileId -> kbId，刷新后共享知识库文件可带 kb_id 拉取（仅保留当前仍选中的文件）
  const persisted = settingsStore.settings.selectedFileKbMap;
  const ids = settingsStore.settings.selectedFiles || [];
  if (persisted && typeof persisted === 'object' && ids.length > 0) {
    const next: Record<string, string> = {};
    ids.forEach((id: string) => {
      if (persisted[id]) next[id] = persisted[id];
    });
    fileIdToKbId.value = next;
  }

  // 如果从知识库内部进入，自动选中该知识库
  const kbId = (route.params as any)?.kbId as string;
  if (kbId && !selectedKbIds.value.includes(kbId)) {
    settingsStore.addKnowledgeBase(kbId);
  }

  const prefill = menuStore.consumePrefillQuery();
  if (prefill) {
    query.value = prefill;
    nextTick(() => {
      const textarea = getTextareaEl();
      if (textarea) textarea.focus();
    });
  }

  document.addEventListener('click', closeMentionSelector);
});

onUnmounted(() => {
  window.removeEventListener(CHAT_FILE_DROP_EVENT, handleChatFileDrop as EventListener);
  document.removeEventListener('click', closeMentionSelector);
});

// 监听路由变化
watch(() => route.params.kbId, (newKbId) => {
  if (newKbId && typeof newKbId === 'string' && !selectedKbIds.value.includes(newKbId)) {
    settingsStore.addKnowledgeBase(newKbId);
  }
});

watch(() => uiStore.showSettingsModal, (visible, prevVisible) => {
  if (prevVisible && !visible) {
    loadWebSearchConfig(true);
  }
});

const emit = defineEmits<{
  (e: 'send-msg', query: string, mentionedItems: any[], imageFiles: File[], attachmentFiles: AttachmentFile[]): void;
  (e: 'stop-generation'): void;
}>();

const createSession = async (val: string) => {
  if (!val.trim()) {
    MessagePlugin.info(t('input.messages.enterContent'));
    return;
  }
  if (props.isReplying) {
    return MessagePlugin.error(t('input.messages.replying'));
  }

  // 获取@提及的知识库和文件信息
  const mentionedItems = allSelectedItems.value.map(item => ({
    id: item.id,
    name: item.name,
    type: item.type,
    kb_type: item.type === 'kb' ? (item.kbType || 'document') : undefined
  }));
  const imageFiles = uploadedImages.value.map(img => img.file);
  const attachmentFiles = uploadedAttachments.value;

  // Blur the textarea BEFORE emitting, so that when the parent navigates away
  // and Vue unmounts this component, TDesign's blur handler won't fire on a
  // detached DOM element (which causes getComputedStyle to throw).
  const textarea = getTextareaEl();
  if (textarea) textarea.blur();
  emit('send-msg', val, mentionedItems, imageFiles, attachmentFiles);

  // Clean up image previews
  uploadedImages.value.forEach(img => URL.revokeObjectURL(img.preview));
  uploadedImages.value = [];

  // Clean up attachments
  attachmentUploadRef.value?.clear();
  uploadedAttachments.value = [];

  clearvalue();
}

const clearvalue = () => {
  // Guard: only clear when the textarea DOM element is still mounted,
  // otherwise TDesign's autosize will call getComputedStyle on a non-Element.
  if (!getTextareaEl()) return;
  query.value = "";
}

const onKeydown = (val: string, event: { e: { preventDefault(): unknown; keyCode: number; shiftKey: any; ctrlKey: any; }; }) => {
  if (showMention.value) {
    if (event.e.keyCode === 38) { // Up
      event.e.preventDefault();
      mentionActiveIndex.value = Math.max(0, mentionActiveIndex.value - 1);
      return;
    }
    if (event.e.keyCode === 40) { // Down
      event.e.preventDefault();
      mentionActiveIndex.value = Math.min(mentionItems.value.length - 1, mentionActiveIndex.value + 1);
      return;
    }
    if (event.e.keyCode === 13) { // Enter
      event.e.preventDefault();
      if (mentionItems.value[mentionActiveIndex.value]) {
        onMentionSelect(mentionItems.value[mentionActiveIndex.value]);
      }
      return;
    }
    if (event.e.keyCode === 27) { // Esc
      showMention.value = false;
      return;
    }
  }

  // 退格键：当输入框为空且有选中项时，删除最后一个选中项
  if (event.e.keyCode === 8) { // Backspace
    const textarea = getTextareaEl();
    if (textarea && textarea.selectionStart === 0 && textarea.selectionEnd === 0 && query.value === '') {
      const items = allSelectedItems.value;
      if (items.length > 0) {
        event.e.preventDefault();
        const lastItem = items[items.length - 1];
        removeSelectedItem(lastItem);
        return;
      }
    }
  }

  if ((event.e.keyCode == 13 && event.e.shiftKey) || (event.e.keyCode == 13 && event.e.ctrlKey)) {
    return;
  }
  if (event.e.keyCode == 13) {
    event.e.preventDefault();
    createSession(val)
  }
}

const onPaste = (e: ClipboardEvent) => {
  const items = e.clipboardData?.items;
  if (!items) return;
  const imageFiles: File[] = [];
  for (const item of items) {
    if (item.type.startsWith('image/')) {
      const file = item.getAsFile();
      if (file) imageFiles.push(file);
    }
  }
  if (imageFiles.length > 0 && isImageUploadEnabledByAgent.value) {
    e.preventDefault();
    addImageFiles(imageFiles);
  }
};

const onDrop = (e: DragEvent) => {
  e.preventDefault();
  const files = e.dataTransfer?.files;
  if (!files || files.length === 0) return;
  handleDroppedFiles(Array.from(files));
};

const onDragOver = (e: DragEvent) => {
  e.preventDefault();
};

const handleGoToWebSearchSettings = () => {
  uiStore.openSettings('websearch');
  if (route.path !== '/platform/settings') {
    router.push('/platform/settings');
  }
};

const handleGoToModeSettings = (section?: string) => {
  const target = section === 'websearch' ? 'websearch' : 'models';
  uiStore.openSettings(target);
  if (route.path !== '/platform/settings') {
    router.push('/platform/settings');
  }
};

const toggleWebSearch = () => {
  // 关闭知识库选择层。
  showMention.value = false;

  // 如果智能体禁用了网络搜索，不允许开启
  if (isWebSearchDisabledByAgent.value) {
    MessagePlugin.warning(t('input.webSearchDisabledByAgent'));
    return;
  }

  if (!isWebSearchConfigured.value) {
    const messageContent = h('div', { style: 'display: flex; flex-direction: column; gap: 6px; max-width: 280px;' }, [
      h('span', { style: 'color: var(--td-text-color-primary); line-height: 1.5;' }, t('input.messages.webSearchNotConfigured')),
      h('a', {
        href: '#',
        onClick: (e: Event) => {
          e.preventDefault();
          handleGoToWebSearchSettings();
        },
        style: 'color: var(--td-brand-color); text-decoration: none; font-weight: 500; cursor: pointer; align-self: flex-start;',
        onMouseenter: (e: Event) => {
          (e.target as HTMLElement).style.textDecoration = 'underline';
        },
        onMouseleave: (e: Event) => {
          (e.target as HTMLElement).style.textDecoration = 'none';
        }
      }, t('input.goToSettings'))
    ]);
    MessagePlugin.warning({
      content: () => messageContent,
      duration: 5000
    });
    return;
  }

  const currentValue = settingsStore.isWebSearchEnabled;
  const newValue = !currentValue;
  settingsStore.toggleWebSearch(newValue);
  MessagePlugin.success(newValue ? t('input.messages.webSearchEnabled') : t('input.messages.webSearchDisabled'));
};

const toggleKbSelector = () => {
  showKbSelector.value = !showKbSelector.value;
}

const removeKb = (kbId: string) => {
  settingsStore.removeKnowledgeBase(kbId);
}

const handleStop = async () => {
  if (!props.sessionId) {
    MessagePlugin.warning(t('input.messages.sessionMissing'));
    return;
  }

  if (!props.assistantMessageId) {
    console.error('[Stop] Assistant message ID is empty');
    MessagePlugin.warning(t('input.messages.messageMissing'));
    return;
  }

  console.log('[Stop] Stopping generation for message:', props.assistantMessageId);

  // 发送 stop 事件，通知父组件立即清除 loading 状态
  emit('stop-generation');

  try {
    await stopSession(props.sessionId, props.assistantMessageId);
    MessagePlugin.success(t('input.messages.stopSuccess'));
  } catch (error) {
    console.error('Failed to stop session:', error);
    MessagePlugin.error(t('input.messages.stopFailed'));
  }
}

onBeforeRouteUpdate((to, from, next) => {
  clearvalue()
  next()
})

defineExpose({
  clearDraft() {
    clearvalue();
  },
  setDraft(text: string) {
    if (!text.trim()) return;
    query.value = text;
    nextTick(() => {
      const textarea = getTextareaEl();
      if (!textarea) return;
      textarea.focus();
      textarea.setSelectionRange(text.length, text.length);
      textarea.scrollTop = textarea.scrollHeight;
    });
  },
  triggerSend(text: string) {
    if (!text.trim()) return;
    query.value = text;
    nextTick(() => createSession(text));
  }
});

</script>
<template>
  <div class="answers-input" @drop="onDrop" @dragover="onDragOver">
    <!-- Hidden file input for image upload -->
    <input ref="imageInputRef" type="file" accept="image/jpeg,image/png,image/gif,image/webp" multiple
      style="display:none" @change="handleImageSelect" />
    <!-- 富文本输入框容器 -->
    <div class="rich-input-container">
      <!-- 图片预览区域 -->
      <div v-if="uploadedImages.length > 0" class="image-preview-bar">
        <div v-for="(img, idx) in uploadedImages" :key="idx" class="image-preview-item">
          <img :src="img.preview" class="image-preview-thumb" />
          <span class="image-preview-remove" @click="removeImage(idx)">×</span>
        </div>
      </div>

      <!-- 附件列表区域 (由 AttachmentUpload 组件渲染) -->
      <AttachmentUpload ref="attachmentUploadRef" :max-files="5" :max-size="20"
        @update:files="uploadedAttachments = $event" />

      <!-- 选中的知识库和文件标签（显示在输入框内顶部） -->
      <div v-if="allSelectedItems.length > 0" class="selected-tags-inline">
        <span v-for="item in allSelectedItems" :key="item.id" class="mention-chip" :class="[
          item.type === 'kb' ? (item.kbType === 'faq' ? 'mention-chip--faq' : 'mention-chip--kb') : 'mention-chip--file',
          { 'mention-chip--agent': item.isAgentConfigured }
        ]">
          <span class="mention-chip__icon-wrap">
            <span class="mention-chip__icon">
              <t-icon v-if="item.type === 'kb'" :name="item.kbType === 'faq' ? 'chat-bubble-help' : 'folder'" />
              <t-icon v-else name="file" />
            </span>
          </span>
          <span class="mention-chip__name" :title="item.name">{{ item.name }}</span>
          <span class="mention-chip__remove" @click.stop="removeSelectedItem(item)"
            :aria-label="$t('common.remove')">×</span>
        </span>
      </div>

      <!-- 实际输入框 -->
      <t-textarea ref="textareaRef" v-model="query" :placeholder="inputPlaceholder" name="description" :autosize="true"
        @keydown="onKeydown" @input="onInput" @compositionstart="onCompositionStart" @compositionend="onCompositionEnd"
        @paste="onPaste" />

      <!-- 控制栏（放在 rich-input-container 内，相对输入框边框定位） -->
      <div class="control-bar">
        <!-- 左侧控制按钮 -->
        <div class="control-left">
          <!-- WebSearch 开关按钮 -->
          <t-tooltip placement="top" theme="light" :popupProps="{ overlayClassName: 'input-field-tooltip' }">
            <template #content>
              <div v-if="isWebSearchDisabledByAgent" class="tooltip-with-link">
                <span>{{ $t('input.webSearchDisabledByAgent') }}</span>
                <a href="#" @click.prevent="handleGoToModeSettings('websearch')">{{ $t('input.goToSettings')
                  }}</a>
              </div>
              <span v-else-if="isWebSearchConfigured">{{ isWebSearchEnabled ? $t('input.webSearch.toggleOff') :
                $t('input.webSearch.toggleOn') }}</span>
              <div v-else class="tooltip-with-link">
                <span>{{ $t('input.webSearch.notConfigured') }}</span>
                <a href="#" @click.prevent="handleGoToWebSearchSettings">{{ $t('input.goToSettings') }}</a>
              </div>
            </template>
            <div class="control-btn websearch-btn" :class="{
              'active': isWebSearchEnabled && isWebSearchConfigured,
              'disabled': !isWebSearchConfigured || isWebSearchDisabledByAgent
            }" @click.stop="toggleWebSearch">
              <svg width="18" height="18" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg"
                class="control-icon websearch-icon" :class="{ 'active': isWebSearchEnabled && isWebSearchConfigured }">
                <circle cx="9" cy="9" r="7" stroke="currentColor" stroke-width="1.2" fill="none" />
                <path d="M 9 2 A 3.5 7 0 0 0 9 16" stroke="currentColor" stroke-width="1.2" fill="none" />
                <path d="M 9 2 A 3.5 7 0 0 1 9 16" stroke="currentColor" stroke-width="1.2" fill="none" />
                <line x1="2.94" y1="5.5" x2="15.06" y2="5.5" stroke="currentColor" stroke-width="1.2"
                  stroke-linecap="round" />
                <line x1="2.94" y1="12.5" x2="15.06" y2="12.5" stroke="currentColor" stroke-width="1.2"
                  stroke-linecap="round" />
              </svg>
            </div>
          </t-tooltip>

          <!-- 图片上传按钮 -->
          <t-tooltip placement="top" theme="light" :popupProps="{ overlayClassName: 'input-field-tooltip' }">
            <template #content>
              <div v-if="!isImageUploadEnabledByAgent" class="tooltip-with-link">
                <span>{{ $t('input.imageUploadDisabledByAgent') }}</span>
                <a href="#" @click.prevent="handleGoToModeSettings('model')">{{ $t('input.goToSettings') }}</a>
              </div>
              <span v-else>{{ $t('chat.imageUploadTooltip') }}</span>
            </template>
            <div class="control-btn image-upload-btn" :class="{
              'active': uploadedImages.length > 0,
              'disabled': !isImageUploadEnabledByAgent
            }" @click.stop="isImageUploadEnabledByAgent && triggerImageUpload()">
              <svg width="18" height="18" viewBox="0 0 1024 1024" fill="currentColor" class="control-icon">
                <path
                  d="M896 128H128c-35.3 0-64 28.7-64 64v640c0 35.3 28.7 64 64 64h768c35.3 0 64-28.7 64-64V192c0-35.3-28.7-64-64-64zM128 832V192h768l0.1 640H128z" />
                <path d="M352 448a96 96 0 1 0 0-192 96 96 0 0 0 0 192z" />
                <path d="M128 768l224-288 160 160 192-256L896 640v128H128z" />
              </svg>
              <span v-if="uploadedImages.length > 0" class="image-count">{{ uploadedImages.length }}</span>
            </div>
          </t-tooltip>

          <!-- 附件上传按钮 -->
          <t-tooltip placement="top" theme="light" :popupProps="{ overlayClassName: 'input-field-tooltip' }">
            <template #content>
              <span>{{ uploadedAttachments.length > 0 ? $t('chat.attachmentWithCount', {
                count: uploadedAttachments.length
              }) : $t('chat.attachmentUploadTooltip') }}</span>
            </template>
            <div class="control-btn attachment-upload-btn" :class="{ 'active': uploadedAttachments.length > 0 }"
              @click.stop="attachmentUploadRef?.triggerFileSelect()">
              <!-- 回形针图标 -->
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"
                stroke-linecap="round" stroke-linejoin="round" class="control-icon">
                <path
                  d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48" />
              </svg>
              <span v-if="uploadedAttachments.length > 0" class="attachment-count">{{ uploadedAttachments.length
                }}</span>
            </div>
          </t-tooltip>

          <!-- @ 知识库/文件选择按钮 -->
          <t-tooltip placement="top" theme="light" :popupProps="{ overlayClassName: 'input-field-tooltip' }">
            <template #content>
              <div v-if="isKnowledgeBaseDisabledByAgent" class="tooltip-with-link">
                <span>{{ $t('input.kbDisabledByAgent') }}</span>
                <a href="#" @click.prevent="handleGoToModeSettings('knowledge')">{{ $t('input.goToSettings')
                  }}</a>
              </div>
              <span v-else>{{ allSelectedItems.length > 0 ? $t('input.knowledgeBaseWithCount', {
                count:
                  allSelectedItems.length
              }) : $t('input.knowledgeBase') }}</span>
            </template>
            <div ref="atButtonRef" class="control-btn kb-btn" :class="{
              'active': allSelectedItems.length > 0,
              'disabled': isKnowledgeBaseDisabledByAgent
            }" @click.stop @mousedown.prevent="triggerMention">
              <svg width="18" height="18" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg"
                class="control-icon at-icon">
                <circle cx="10" cy="10" r="3.5" stroke="currentColor" stroke-width="1.8" />
                <path
                  d="M13.5 10V11.5C13.5 12.163 13.7634 12.7989 14.2322 13.2678C14.7011 13.7366 15.337 14 16 14C16.663 14 17.2989 13.7366 17.7678 13.2678C18.2366 12.7989 18.5 12.163 18.5 11.5V10C18.5 7.74566 17.6045 5.58365 16.0104 3.98959C14.4163 2.39553 12.2543 1.5 10 1.5C7.74566 1.5 5.58365 2.39553 3.98959 3.98959C2.39553 5.58365 1.5 7.74566 1.5 10C1.5 12.2543 2.39553 14.4163 3.98959 16.0104C5.58365 17.6045 7.74566 18.5 10 18.5H12"
                  stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
              <span v-if="allSelectedItems.length > 0" class="kb-count">{{ allSelectedItems.length }}</span>
            </div>
          </t-tooltip>

        </div>

        <!-- 右侧控制按钮组 -->
        <div class="control-right">
          <!-- 停止按钮（仅在回复中时显示） -->
          <t-tooltip v-if="isReplying" :content="$t('input.stopGeneration')" placement="top">
            <div @click="handleStop" class="control-btn stop-btn">
              <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
                <rect x="5" y="5" width="6" height="6" rx="1" />
              </svg>
            </div>
          </t-tooltip>

          <!-- 发送按钮 -->
          <div v-if="!isReplying" @click="createSession(query)" class="control-btn send-btn"
            :class="{ 'disabled': !query.length }">
            <img src="../assets/img/sending-aircraft.svg" :alt="$t('input.send')" />
          </div>
        </div>
      </div>
    </div>

    <!-- Mention Selector -->
    <Teleport to="body">
      <MentionSelector :visible="showMention" :style="mentionStyle" :items="mentionItems" :hasMore="mentionHasMore"
        :loading="mentionLoading" :emptyHint="mentionEmptyHint" v-model:activeIndex="mentionActiveIndex"
        @select="onMentionSelect" @loadMore="loadMoreMentionItems" />
    </Teleport>

    <!-- 知识库选择下拉（使用 Teleport 传送到 body，避免父容器定位影响） -->
    <Teleport to="body">
      <KnowledgeBaseSelector v-model:visible="showKbSelector" :anchorEl="atButtonRef" @close="showKbSelector = false" />
    </Teleport>
  </div>
</template>
<style scoped lang="less">
.answers-input {
  position: absolute;
  z-index: 99;
  bottom: 60px;
  left: 50%;
  transform: translateX(-50%);
  width: 100%;
  display: flex;
  justify-content: center;

}

/* 富文本输入框容器 */
.rich-input-container {
  position: relative;
  width: 100%;
  max-width: 800px;
  background: var(--td-bg-color-container, #FFF);
  border-radius: 12px;
  border: 1px solid var(--td-component-stroke, #dcdcdc);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04), 0 8px 16px -4px rgba(0, 0, 0, 0.06);

  &:focus-within {
    border-color: var(--td-brand-color, #07C05F);
  }
}

/* 选中的知识库/文件标签（mention list 已选项） */
.selected-tags-inline {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 5px;
  padding: 6px 12px 6px;
  border-bottom: 1px solid var(--td-component-stroke, #dcdcdc);
  background: var(--td-bg-color-container, #fff);
  border-radius: 11px 11px 0 0;
  /* 与 .rich-input-container 内缘上边圆角一致（12px - 1px 边框） */
}

.mention-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 6px 3px 5px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
  cursor: default;
  transition: background 0.2s, border-color 0.2s, box-shadow 0.2s;
  border: .5px solid transparent;
  color: var(--td-text-color-primary, #1f2937);
  line-height: 1.3;
}

.mention-chip__icon-wrap {
  position: relative;
  display: inline-flex;
  width: 16px;
  height: 16px;
  flex: 0 1 auto;
  min-width: 0;
  align-items: center;
  justify-content: center;
  border-radius: 3px;
}

.mention-chip__icon {
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: inherit;
}

.mention-chip__org-badge {
  position: absolute;
  right: -1px;
  bottom: -1px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--td-bg-color-secondarycontainer, #f0f2f5);
  box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.06);
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
}

.mention-chip__org-img {
  width: 5px;
  height: 5px;
  object-fit: contain;
}

.mention-chip__name {
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: currentColor;
}

.mention-chip__remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  margin-left: 1px;
  border-radius: 50%;
  font-size: 14px;
  line-height: 1;
  font-weight: 400;
  cursor: pointer;
  opacity: 0.45;
  transition: opacity 0.15s, background 0.15s, color 0.15s;
  color: currentColor;
  flex-shrink: 0;
}

.mention-chip:hover .mention-chip__remove {
  opacity: 0.85;
}

.mention-chip__remove:hover {
  opacity: 1;
  background: rgba(0, 0, 0, 0.08);
  color: var(--td-text-color-primary, #1f2937);
}

/* 知识库：浅绿/青色调 */
.mention-chip--kb {
  background: rgba(5, 192, 95, 0.08);
  border-color: rgba(5, 192, 95, 0.25);
  color: var(--td-text-color-primary, #1f2937);
}

.mention-chip--kb .mention-chip__icon-wrap {
  background: rgba(5, 192, 95, 0.12);
  color: var(--td-brand-color, #07c05f);
}

.mention-chip--kb:hover {
  background: rgba(5, 192, 95, 0.12);
  border-color: rgba(5, 192, 95, 0.35);
}

/* FAQ：浅紫/靛色调 */
.mention-chip--faq {
  background: rgba(107, 114, 228, 0.08);
  border-color: rgba(107, 114, 228, 0.25);
  color: var(--td-text-color-primary, #1f2937);
}

.mention-chip--faq .mention-chip__icon-wrap {
  background: rgba(107, 114, 228, 0.12);
  color: var(--td-brand-color);
}

.mention-chip--faq:hover {
  background: rgba(107, 114, 228, 0.12);
  border-color: rgba(107, 114, 228, 0.35);
}

/* 文件：浅灰/中性色 */
.mention-chip--file {
  background: var(--td-bg-color-secondarycontainer, #f3f4f6);
  border-color: var(--td-component-stroke, #e5e7eb);
  color: var(--td-text-color-primary, #1f2937);
}

.mention-chip--file .mention-chip__icon-wrap {
  background: rgba(107, 114, 128, 0.12);
  color: var(--td-text-color-secondary, #6b7280);
}

.mention-chip--file:hover {
  background: var(--td-bg-color-component, #e5e7eb);
  border-color: var(--td-component-stroke, #d1d5db);
}

/* 智能体预配置：虚线边框区分 */
.mention-chip--agent {
  border-style: dashed;
}

.mention-chip--agent.mention-chip--kb {
  border-color: rgba(5, 192, 95, 0.4);
}

.mention-chip--agent.mention-chip--faq {
  border-color: rgba(107, 114, 228, 0.4);
}

:deep(.t-textarea__inner) {
  width: 100%;
  max-height: 200px !important;
  min-height: 120px !important;
  resize: none;
  color: var(--td-text-color-primary, #000000e6);
  font-size: 16px;
  font-weight: 400;
  line-height: 24px;
  font-family: var(--app-font-family);
  padding: 12px 16px 56px 16px;
  border-radius: 0 0 12px 12px;
  border: none;
  box-sizing: border-box;
  background: transparent;
  box-shadow: none;

  &:focus {
    border: none;
    box-shadow: none;
  }

  &::placeholder {
    color: var(--td-text-color-placeholder, #00000066);
    font-family: var(--app-font-family);
    font-size: 16px;
    font-weight: 400;
    line-height: 24px;
  }
}

/* 当没有选中标签时，textarea 样式 */
.rich-input-container:not(:has(.selected-tags-inline)) :deep(.t-textarea__inner) {
  border-radius: 12px;
  padding-top: 16px;
}

/* 控制栏 */
.control-bar {
  position: absolute;
  bottom: 12px;
  left: 16px;
  right: 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex-wrap: wrap;
  max-height: 56px;
  z-index: 10;
  background: linear-gradient(to bottom, rgba(255, 255, 255, 0) 0%, var(--td-bg-color-container, #fff) 40%, var(--td-bg-color-container, #fff) 100%);
  pointer-events: auto;
  padding-top: 8px;
}

.control-left {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  flex-wrap: wrap;
  min-width: 0;
}

.control-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 6px 10px;
  border-radius: 6px;
  color: var(--td-text-color-secondary, #666);
  cursor: pointer;
  transition: background 0.12s, color 0.12s;
  user-select: none;
  flex-shrink: 0;

  &:hover {
    background: var(--td-bg-color-secondarycontainer-hover, #e6e6e6);
  }

  &.disabled {
    opacity: 0.5;
    cursor: not-allowed;

    &:hover {
      background: var(--td-bg-color-secondarycontainer, #f5f5f5);
    }
  }
}

.answer-mode-control {
  display: inline-grid;
  grid-template-columns: repeat(2, minmax(52px, 1fr));
  height: 28px;
  padding: 2px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 6px;
  background: var(--td-bg-color-secondarycontainer);
  flex-shrink: 0;
}

.answer-mode-option {
  min-width: 52px;
  padding: 0 10px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--td-text-color-secondary);
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;

  &.active {
    color: #fff;
    background: var(--td-brand-color);
    box-shadow: 0 1px 3px rgba(0, 82, 217, 0.2);
  }
}

.knowledge-feature-toggles {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.knowledge-feature-toggle {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 28px;
  padding: 0 8px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 6px;
  color: var(--td-text-color-secondary);
  font-size: 12px;
  white-space: nowrap;
}

.agent-mode-btn {
  height: 28px;
  padding: 0 10px;
  min-width: auto;
  font-weight: 500;
  position: relative;
  border: .5px solid var(--td-component-border, #e7e7e7);
}

.agent-icon {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}

.agent-btn-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 5px;
  flex-shrink: 0;
  color: var(--td-text-color-secondary, #666);
}

.agent-mode-text {
  font-size: 13px;
  color: var(--td-text-color-secondary, #666);
  font-weight: 500;
  white-space: nowrap;
  margin: 0 4px;
}

.control-icon {
  width: 18px;
  height: 18px;
}

.kb-btn {
  height: 28px;
  padding: 0 10px;
  min-width: auto;
  position: relative;

  &.active {
    background: rgba(16, 185, 129, 0.1);
    color: var(--td-brand-color);

    &:hover {
      background: rgba(16, 185, 129, 0.15);
    }
  }

  &.agent-controlled {
    cursor: not-allowed;
    opacity: 0.85;

    &:hover {
      background: var(--td-bg-color-secondarycontainer, #f5f5f5);
    }

    &.active:hover {
      background: rgba(16, 185, 129, 0.1);
    }
  }
}

.kb-count {
  position: absolute;
  top: -4px;
  right: -4px;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  background: var(--td-brand-color);
  color: white;
  font-size: 10px;
  font-weight: 600;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.kb-btn-text {
  font-size: 13px;
  color: var(--td-text-color-secondary, #666);
  font-weight: 500;
  white-space: nowrap;
}

.kb-btn.active .kb-btn-text {
  color: var(--td-brand-color);
}

/* Image upload */
.image-upload-btn {
  width: 28px;
  height: 28px;
  padding: 0;
  min-width: auto;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  color: var(--td-text-color-secondary, #666);

  &:hover {
    background: var(--td-bg-color-secondarycontainer-hover, #f0f0f0);
    color: var(--td-text-color-primary, #333);
  }

  &.active {
    background: rgba(16, 185, 129, 0.1);
    color: #07C05F;
  }

  .image-count {
    position: absolute;
    top: -2px;
    right: -2px;
    background: #07C05F;
    color: #fff;
    font-size: 10px;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    line-height: 1;
  }
}

/* Attachment upload */
.attachment-upload-btn {
  width: 28px;
  height: 28px;
  padding: 0;
  min-width: auto;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  color: var(--td-text-color-secondary, #666);

  &:hover {
    background: var(--td-bg-color-secondarycontainer-hover, #f0f0f0);
    color: var(--td-text-color-primary, #333);
  }

  &.active {
    background: rgba(16, 185, 129, 0.1);
    color: #07C05F;
  }

  .attachment-count {
    position: absolute;
    top: -2px;
    right: -2px;
    background: #07C05F;
    color: #fff;
    font-size: 10px;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    line-height: 1;
  }
}

.image-preview-bar {
  display: flex;
  gap: 8px;
  padding: 8px 12px 4px;
  flex-wrap: wrap;
}

.image-preview-item {
  position: relative;
  width: 60px;
  height: 60px;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid var(--td-border-level-1-color, #e7e7e7);

  .image-preview-thumb {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .image-preview-remove {
    position: absolute;
    top: 2px;
    right: 2px;
    width: 16px;
    height: 16px;
    background: rgba(0, 0, 0, 0.5);
    color: #fff;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    cursor: pointer;
    line-height: 1;

    &:hover {
      background: rgba(0, 0, 0, 0.7);
    }
  }
}

.websearch-btn {
  width: 28px;
  height: 28px;
  padding: 0;
  min-width: auto;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;

  &.active {
    background: rgba(16, 185, 129, 0.1);

    .websearch-icon {
      color: var(--td-brand-color);
    }

    &:hover {
      background: rgba(16, 185, 129, 0.15);
    }
  }

  &:not(.active) {
    .websearch-icon {
      color: var(--td-text-color-secondary, #666);
    }

    &:hover {
      background: var(--td-bg-color-secondarycontainer-hover, #f0f0f0);

      .websearch-icon {
        color: var(--td-text-color-primary, #333);
      }
    }
  }

  &.agent-controlled {
    cursor: not-allowed;
    opacity: 0.85;

    &:hover {
      background: var(--td-bg-color-secondarycontainer, #f5f5f5);
    }

    &.active:hover {
      background: rgba(16, 185, 129, 0.1);
    }
  }
}

:global(.input-field-tooltip) {
  .t-popup__content {
    box-shadow: var(--td-shadow-2);
    border: .5px solid var(--td-component-border, #e7e7e7);
  }
}

:global(.tooltip-with-link) {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-width: 220px;
  font-size: 12px;
  color: var(--td-text-color-primary, #333);
}

:global(.tooltip-with-link a) {
  color: var(--td-brand-color);
  font-weight: 500;
  text-decoration: none;
}

:global(.tooltip-with-link a:hover) {
  text-decoration: underline;
}

.websearch-icon {
  width: 18px;
  height: 18px;
}

.dropdown-arrow {
  width: 10px;
  height: 10px;
  margin-left: 2px;
  transition: transform 0.12s;

  &.rotate {
    transform: rotate(180deg);
  }
}

.control-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.stop-btn {
  width: 28px;
  height: 28px;
  padding: 0;
  background: rgba(16, 185, 129, 0.08);
  color: var(--td-brand-color);
  border: 1.5px solid rgba(16, 185, 129, 0.2);
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;

  &:hover {
    background: rgba(16, 185, 129, 0.12);
    border-color: var(--td-brand-color);
  }

  &:active {
    background: rgba(16, 185, 129, 0.15);
  }

  svg {
    display: none;
  }

  &::before {
    content: '';
    width: 12px;
    height: 12px;
    background: var(--td-brand-color);
    border-radius: 50%;
    display: block;
    animation: stopBtnPulse 1.5s ease-in-out infinite;
  }
}

@keyframes stopBtnPulse {

  0%,
  100% {
    transform: scale(1);
    opacity: 1;
  }

  50% {
    transform: scale(0.75);
    opacity: 0.6;
  }
}

.send-btn {
  width: 28px;
  height: 28px;
  padding: 0;
  background-color: var(--td-brand-color);

  &:hover:not(.disabled) {
    background-color: var(--td-brand-color-active);
  }

  &.disabled {
    background-color: var(--td-success-color-light);
  }

  img {
    width: 16px;
    height: 16px;
  }
}

/* Agent 模式选择下拉菜单 */
.agent-mode-selector-overlay {
  position: fixed;
  inset: 0;
  z-index: 9998;
  background: transparent;
  touch-action: none;
}

.agent-mode-selector-dropdown {
  position: fixed !important;
  z-index: 9999;
  background: var(--td-bg-color-container, #fff);
  border-radius: 10px;
  box-shadow: var(--td-shadow-2, 0 6px 28px rgba(15, 23, 42, 0.08));
  border: 1px solid var(--td-component-border, #e7e9eb);
  overflow: hidden;
  padding: 6px 8px;
  min-width: 200px;
  display: flex;
  flex-direction: column;
  margin: 0 !important;
  padding: 0 !important;
  transform: none !important;
}

.agent-mode-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  cursor: pointer;
  transition: background 0.12s;
  border-radius: 6px;
  position: relative;
  margin: 4px 6px;

  &:hover:not(.disabled) {
    background: var(--td-bg-color-container-hover, #f6f8f7);
  }

  &.disabled {
    opacity: 0.6;
    cursor: not-allowed;

    &:hover {
      background: transparent;
    }
  }

  &.selected {
    background: var(--td-brand-color-light, #eefdf5);

    .agent-mode-option-name {
      color: var(--td-success-color);
      font-weight: 700;
    }
  }
}

.agent-mode-option-main {
  display: flex;
  flex-direction: column;
  gap: 1px;
  flex: 1;
  min-width: 0;
}

.agent-mode-option-name {
  font-size: 12px;
  font-weight: 600;
  color: var(--td-text-color-primary, #222);
  line-height: 1.4;
  transition: color 0.12s;
}

.agent-mode-option-desc {
  font-size: 11px;
  color: var(--td-text-color-secondary, #8b9196);
  line-height: 1.3;
}

.check-icon {
  width: 14px;
  height: 14px;
  color: var(--td-success-color);
  flex-shrink: 0;
  margin-left: 6px;
}

.agent-mode-warning {
  display: flex;
  align-items: center;
  margin-left: 6px;

  .warning-icon {
    color: var(--td-warning-color);
    font-size: 14px;
  }
}

.agent-mode-footer {
  padding: 6px 10px;
  border-top: 1px solid var(--td-component-border, #f2f4f5);
  margin-top: 2px;
  background: var(--td-bg-color-secondarycontainer, #fafcfc);
}

.agent-mode-link {
  color: var(--td-success-color);
  text-decoration: none;
  font-size: 11px;
  font-weight: 500;
  display: inline-flex;
  align-items: center;
  gap: 3px;
  transition: all 0.12s;

  &:hover {
    color: var(--td-brand-color-active);
    text-decoration: underline;
  }
}
</style>
