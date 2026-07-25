<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, reactive, computed, nextTick } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import DocContent from "@/components/doc-content.vue";
import KnowledgeProcessingProgress from "@/components/knowledge-processing-progress.vue";
import useKnowledgeBase from '@/hooks/useKnowledgeBase';
import { useRoute, useRouter } from 'vue-router';
import EmptyKnowledge from '@/components/empty-knowledge.vue';
import KBSwitcherDropdown from '@/components/KBSwitcherDropdown.vue';
import { createSessions } from "@/api/chat/index";
import { useMenuStore } from '@/stores/menu';
import { useUIStore } from '@/stores/ui';
import { useChatResourcesStore } from '@/stores/chatResources';
import { useEditorResourcesStore } from '@/stores/editorResources';
import KnowledgeBaseEditorModal from './KnowledgeBaseEditorModal.vue';
import KnowledgeBaseSharingDialog from './components/KnowledgeBaseSharingDialog.vue';
import { useAuthStore } from '@/stores/auth';
const usemenuStore = useMenuStore();
const uiStore = useUIStore();
const chatResources = useChatResourcesStore();
const editorResources = useEditorResourcesStore();
const authStore = useAuthStore();
const router = useRouter();
import {
  batchQueryKnowledge,
  uploadKnowledgeFile,
  reparseKnowledge,
  cancelKnowledgeParse,
  batchDeleteKnowledge,
  getKnowledgeSpans,
  getKnowledgeDetails,
} from "@/api/knowledge-base/index";
import FAQEntryManager from './components/FAQEntryManager.vue';
import DocumentListView from './components/DocumentListView.vue';
import DocumentBatchBar from './components/DocumentBatchBar.vue';
import KbUploadSourceDropdown from './components/KbUploadSourceDropdown.vue';
import type { KnowledgeProcessOverrides } from '@/types/knowledgeProcess';
import { useUploadConfirmStore, type UploadConfirmResult } from '@/stores/uploadConfirm';
import {
  getKBRebuildStatus,
  rebuildKBIndex,
  listMoveTargets,
  moveKnowledge,
  getKnowledgeMoveProgress,
  type KBRebuildState,
} from '@/api/knowledge-base';
import { useI18n } from 'vue-i18n';
import { formatStringDate } from '@/utils';
import { formatFileSize } from '@/utils/files';
import { useMarqueeSelect } from '@/hooks/useMarqueeSelect';
import type { ParserEngineInfo } from '@/api/system';
import { filterUploadFiles } from './utils/uploadSources';
const route = useRoute();
const { t } = useI18n();
const kbId = computed(() => (route.params as any).kbId as string || '');
const kbInfo = ref<any>(null);
const uploading = ref(false);
const emptyDropActive = ref(false);
const kbLoading = ref(false);
const docListLoading = ref(true);
const docListError = ref('');
const isFAQ = computed(() => (kbInfo.value?.type || '') === 'faq');
const validTabs = ['documents'] as const
type KbTab = typeof validTabs[number]
const initTab = validTabs.includes(route.query.tab as any) ? (route.query.tab as KbTab) : 'documents'
const activeKbTab = ref<KbTab>(initTab);

const fullRebuildState = ref<KBRebuildState | null>(null);
const fullRebuildRunning = computed(() =>
  fullRebuildState.value?.status === 'pending' || fullRebuildState.value?.status === 'running'
);
let fullRebuildTimer: number | null = null;
let watchedRebuildGeneration: number | null = null;

const stopFullRebuildPolling = () => {
  if (fullRebuildTimer) {
    window.clearInterval(fullRebuildTimer);
    fullRebuildTimer = null;
  }
};

const fetchFullRebuildState = async () => {
  if (!kbId.value) return;
  try {
    const response: any = await getKBRebuildStatus(kbId.value);
    const state = (response?.data || response) as KBRebuildState;
    fullRebuildState.value = state;
    const generation = state.building_generation || state.active_generation;
    if (state.status === 'pending' || state.status === 'running') {
      if (!fullRebuildTimer) {
        fullRebuildTimer = window.setInterval(fetchFullRebuildState, 2000);
      }
      return;
    }
    stopFullRebuildPolling();
    if (watchedRebuildGeneration !== null && generation >= watchedRebuildGeneration) {
      if (state.status === 'succeeded') {
        MessagePlugin.success(t('knowledgeBase.fullRebuildSucceeded'));
        await loadKnowledgeList();
      } else if (state.status === 'failed') {
        MessagePlugin.error(t('knowledgeBase.fullRebuildFailedReason', { reason: state.error || t('common.unknownError') }));
      }
      watchedRebuildGeneration = null;
    }
  } catch {
    // Keep the current state; the next poll can recover from a transient error.
  }
};

const startFullRebuild = async () => {
  if (!kbId.value || fullRebuildRunning.value) return;
  try {
    const response: any = await rebuildKBIndex(kbId.value);
    const state = (response?.data || response) as KBRebuildState;
    fullRebuildState.value = state;
    watchedRebuildGeneration = state.building_generation || null;
    MessagePlugin.success(t('knowledgeBase.fullRebuildSubmitted'));
    stopFullRebuildPolling();
    fullRebuildTimer = window.setInterval(fetchFullRebuildState, 2000);
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('knowledgeBase.rebuildFailed'));
  }
};

watch(kbId, (newKbId) => {
  stopFullRebuildPolling();
  watchedRebuildGeneration = null;
  fullRebuildState.value = null;
  if (newKbId) fetchFullRebuildState();
}, { immediate: true });

const parserEngines = computed<ParserEngineInfo[]>(() => editorResources.parserEngines);

const supportedFileTypes = computed<Set<string>>(() => {
  const engines = parserEngines.value
  if (!engines.length) return new Set<string>()

  const rules: { file_types: string[]; engine: string }[] =
    kbInfo.value?.chunking_config?.parser_engine_rules || []

  const ruleMap = new Map<string, string>()
  for (const r of rules) {
    for (const ft of r.file_types) ruleMap.set(ft, r.engine)
  }

  const available = new Set<string>()
  const availableEngineNames = new Set(
    engines.filter(e => e.Available !== false).map(e => e.Name)
  )

  for (const engine of engines) {
    for (const ft of engine.FileTypes || []) {
      if (available.has(ft)) continue

      const explicitEngine = ruleMap.get(ft)
      if (explicitEngine) {
        if (availableEngineNames.has(explicitEngine)) available.add(ft)
      } else {
        if (engine.Available !== false) available.add(ft)
      }
    }
  }
  return available
})

const acceptFileTypes = computed(() =>
  [...supportedFileTypes.value].map(t => '.' + t).join(',')
)

const unsupportedFileTypes = computed<string[]>(() => {
  const engines = parserEngines.value
  if (!engines.length) return []

  const allTypes = new Set<string>()
  for (const engine of engines) {
    for (const ft of engine.FileTypes || []) allTypes.add(ft)
  }

  const supported = supportedFileTypes.value
  return [...allTypes].filter(ft => !supported.has(ft)).sort()
})

const goToParserSettings = () => {
  if (kbId.value) {
    uiStore.openKBSettings(kbId.value, 'parser')
  }
}

const accessRank = computed(() => (({ reader: 1, writer: 2, admin: 3, owner: 4 } as Record<string, number>)[kbInfo.value?.access_role || (kbInfo.value ? 'owner' : '')] || 0));
const canWrite = computed(() => accessRank.value >= 2);
const canEdit = computed(() => accessRank.value >= 3);
const canManage = computed(() => accessRank.value >= 3);
const canConfigure = computed(() => accessRank.value >= 4);
const canMutateKnowledge = computed(() => accessRank.value >= 3);
const canRetryOwnDocument = (item: any) => canWrite.value && (
  canManage.value || (
    item?.uploaded_by_user_id === authStore.user?.id &&
    (item?.parse_status === 'failed' || item?.parse_status === 'cancelled')
  )
);
const sharingVisible = ref(false);

const knowledgeList = ref<Array<{ id: string; name: string; type?: string }>>([]);
let { cardList, total, moreIndex, details, getKnowled, delKnowledge, openMore, onVisibleChange: _onVisibleChange, getCardDetails, getfDetails } = useKnowledgeBase(kbId.value)

const onVisibleChange = (visible: boolean) => {
  _onVisibleChange(visible);
  if (!visible) {
    moveMenuMode.value = 'normal';
  }
};

const KNOWLEDGE_STAGE_ORDER = ['docreader', 'chunking', 'embedding', 'multimodal', 'postprocess'] as const;
const stageProgressById = reactive<Record<string, number>>({});
const stageProgressInflight = new Set<string>();

function clearStageProgressCache() {
  for (const key of Object.keys(stageProgressById)) {
    delete stageProgressById[key];
  }
  stageProgressInflight.clear();
}

// Parse phases where the backend pipeline is still actively running
// (primary parse OR post-process fan-out). Trace data exists and the
// UI should treat the row as "in flight" rather than terminal.
function isParseInFlight(status?: string): boolean {
	return status === 'pending' || status === 'processing' || status === 'finalizing';
}

function fallbackStageProgress(item: KnowledgeCard): number {
  if (item.parse_status === 'completed' || item.parse_status === 'finalizing') return 5;
  if (item.parse_status === 'draft') return 0;
  return stageProgressById[item.id] || 1;
}

function stageProgressLabel(item: KnowledgeCard): string {
  const stage = Math.max(1, Math.min(5, fallbackStageProgress(item)));
  return `${stage}/5`;
}

function stageProgressFromSpans(payload: any, item: KnowledgeCard): number {
  const status = payload?.parse_status || item.parse_status;
  if (status === 'completed' || status === 'finalizing') return 5;
  const currentStage = String(payload?.current_stage || '');
  const directIndex = KNOWLEDGE_STAGE_ORDER.indexOf(currentStage as any);
  if (directIndex >= 0) return directIndex + 1;

  const stageNodes = Array.isArray(payload?.trace?.children)
    ? payload.trace.children.filter((node: any) => node?.kind === 'stage')
    : [];
  const active = stageNodes.find((node: any) => node.status === 'running' || node.status === 'failed');
  if (active) {
    const activeIndex = KNOWLEDGE_STAGE_ORDER.indexOf(active.name as any);
    if (activeIndex >= 0) return activeIndex + 1;
  }
  const doneCount = stageNodes.filter((node: any) => node.status === 'done').length;
  return Math.max(1, Math.min(5, doneCount + 1));
}

async function refreshStageProgress(item: KnowledgeCard) {
  const id = item.id;
  if (!id || stageProgressInflight.has(id)) return;
  if (item.parse_status === 'completed' || item.parse_status === 'finalizing') {
    stageProgressById[id] = 5;
    return;
  }
  if (!isParseInFlight(item.parse_status) && item.parse_status !== 'failed') return;
  stageProgressInflight.add(id);
  try {
    const res: any = await getKnowledgeSpans(id);
    const payload = res?.data || res;
    if (payload) stageProgressById[id] = stageProgressFromSpans(payload, item);
  } catch {
    stageProgressById[id] = fallbackStageProgress(item);
  } finally {
    stageProgressInflight.delete(id);
  }
}

const onCardMoreVisibleChange = (visible: boolean, item: KnowledgeCard) => {
  onVisibleChange(visible);
  if (visible) void refreshStageProgress(item);
};
let isCardDetails = ref(false);
const progressDrawerVisible = ref(false);
const progressItem = ref<any>(null);
let timeout: ReturnType<typeof setTimeout> | null = null;
let knowledgeScroll = ref()
let page = 1;
let pageSize = 35;
let scrollLoading = false;
const resetPage = () => { page = 1; scrollLoading = false; };

// Move state — inline in card menu
const moveMenuMode = ref<'normal' | 'targets' | 'confirm'>('normal');
const moveKnowledgeId = ref('');
const moveTargetKbs = ref<any[]>([]);
const moveTargetsLoading = ref(false);
const moveSelectedTargetId = ref('');
const moveSelectedTargetName = ref('');
const moveMode = ref<'reuse_vectors' | 'reparse'>('reuse_vectors');
const moveSubmitting = ref(false);
let movePollTimer: ReturnType<typeof setInterval> | null = null;

// View mode (grid / list) — persisted per browser
type DocViewMode = 'grid' | 'list';
const VIEW_MODE_KEY = 'zealrag.kb.docs.viewMode';
const initViewMode = (): DocViewMode => {
  try {
    return localStorage.getItem(VIEW_MODE_KEY) === 'grid' ? 'grid' : 'list';
  } catch { return 'list'; }
};
const viewMode = ref<DocViewMode>(initViewMode());
watch(viewMode, (v) => {
  try { localStorage.setItem(VIEW_MODE_KEY, v); } catch { /* ignore */ }
});

// Multi-select state — shared between grid and list views.
// Vue 3.5 tracks Set#add/delete natively, so direct mutation is reactive.
const selectedIds = ref<Set<string>>(new Set());
let lastSelectedIndex = -1;
const batchDeleting = ref(false);

let docSearchDebounce: number | null = null;
const docSearchKeyword = ref('');
const selectedFileType = ref('');
const fileTypeOptions = computed(() => [
  { label: t('knowledgeBase.allFileTypes'), value: '' },
  { label: 'PDF', value: 'pdf' },
  { label: 'DOCX', value: 'docx' },
  { label: 'DOC', value: 'doc' },
  { label: 'PPTX', value: 'pptx' },
  { label: 'PPT', value: 'ppt' },
  { label: 'EPUB', value: 'epub' },
  { label: 'MHTML', value: 'mhtml' },
  { label: 'TXT', value: 'txt' },
  { label: 'MD', value: 'md' },
]);
const selectedParseStatus = ref('');
const parseStatusOptions = computed(() => [
  { label: t('knowledgeBase.allParseStatuses'), value: '' },
  { label: t('knowledgeBase.parseStatusPending'), value: 'pending' },
  { label: t('knowledgeBase.parseStatusProcessing'), value: 'processing' },
  { label: t('knowledgeBase.parseStatusCompleted'), value: 'completed' },
  { label: t('knowledgeBase.parseStatusFailed'), value: 'failed' },
]);
// Date range as [start, end] in "YYYY-MM-DD" form (t-date-range-picker default).
const updatedTimeRange = ref<string[]>([]);
// Disable any date after today so users cannot filter into the future.
const disableFutureDate = { after: new Date(new Date().setHours(23, 59, 59, 999)) };
const filterParams = computed(() => {
  const [start, end] = updatedTimeRange.value || [];
  return {
    keyword: docSearchKeyword.value ? docSearchKeyword.value.trim() : undefined,
    file_type: selectedFileType.value || undefined,
    parse_status: selectedParseStatus.value || undefined,
    start_time: start ? `${start} 00:00:00` : undefined,
    end_time: end ? `${end} 23:59:59` : undefined,
  };
});
const getPageSize = () => {
  const viewportHeight = window.innerHeight || document.documentElement.clientHeight;
  const itemHeight = 148;
  let itemsInView = Math.floor(viewportHeight / itemHeight) * 5;
  pageSize = Math.max(35, itemsInView);
}
getPageSize()
const formatDocTime = (time?: string) => {
  if (!time) return '--'
  const formatted = formatStringDate(new Date(time))
  return formatted.slice(2, 16) // "YY-MM-DD HH:mm"
}

// 获取知识条目的显示类型
const getKnowledgeType = (item: any) => {
  if (item.file_type) {
    return item.file_type.toUpperCase();
  }
  return '--';
}

let docListRequestGeneration = 0;
const loadKnowledgeFiles = async (kbIdValue: string): Promise<void> => {
  if (!kbIdValue) return;
  const requestGeneration = ++docListRequestGeneration;
  docListLoading.value = true;
  docListError.value = '';
  try {
    await getKnowled(
      {
        page: 1,
        page_size: pageSize,
        ...filterParams.value,
      },
      kbIdValue,
    );
  } catch (error: any) {
    if (!isCurrentKb(kbIdValue) || requestGeneration !== docListRequestGeneration) return;
    docListError.value = error?.message || '文档列表加载失败';
    console.error('Failed to load knowledge files:', error);
    MessagePlugin.error(docListError.value);
  } finally {
    if (isCurrentKb(kbIdValue) && requestGeneration === docListRequestGeneration) {
      docListLoading.value = false;
    }
  }
};

const isCurrentKb = (targetKbId: string) => targetKbId === kbId.value;

const loadKnowledgeBaseInfo = async (targetKbId: string, force = false) => {
  if (!targetKbId) {
    kbInfo.value = null;
    cardList.value = [];
    total.value = 0;
    return;
  }
  kbLoading.value = true;
  try {
    const data = await chatResources.fetchKnowledgeBaseById(targetKbId, force);
    if (!isCurrentKb(targetKbId)) return;

    kbInfo.value = data;
    if (!isFAQ.value) {
      loadKnowledgeFiles(targetKbId);
    } else {
      cardList.value = [];
      total.value = 0;
    }
  } catch (error) {
    if (!isCurrentKb(targetKbId)) return;

    console.error('Failed to load knowledge base info:', error);
    kbInfo.value = null;
    cardList.value = [];
    total.value = 0;
  } finally {
    if (isCurrentKb(targetKbId)) {
      kbLoading.value = false;
    }
  }
};

const loadKnowledgeList = async () => {
  try {
    await chatResources.ensureKnowledgeBases();
    const myKbs = chatResources.rawKnowledgeBases.map((item: any) => ({
      id: String(item.id),
      name: item.name,
      type: item.type || 'document',
    }));

    knowledgeList.value = myKbs;
  } catch (error) {
    console.error('Failed to load knowledge list:', error);
  }
};

// 监听路由参数变化，重新获取知识库内容
// Sync activeKbTab to URL query so it survives page refresh
watch(activeKbTab, (tab) => {
  const query = { ...route.query }
  if (tab === 'documents') {
    delete query.tab
    delete query.slug
  } else {
    query.tab = tab
  }
  router.replace({ query })
})

watch(() => kbId.value, (newKbId, oldKbId) => {
  if (!newKbId) {
    kbInfo.value = null;
    cardList.value = [];
    total.value = 0;
    return;
  }
  if (newKbId === oldKbId && kbInfo.value) return;

  if (newKbId !== oldKbId) {
    clearStageProgressCache();
    cardList.value = [];
    total.value = 0;
    docListLoading.value = true;
    resetPage();
  }
  loadKnowledgeBaseInfo(newKbId);
}, { immediate: true });

// 监听文档搜索关键词变化
watch(docSearchKeyword, (newVal, oldVal) => {
  if (newVal === oldVal) return;
  if (docSearchDebounce) {
    window.clearTimeout(docSearchDebounce);
  }
  docSearchDebounce = window.setTimeout(() => {
    if (kbId.value) {
      resetPage();
      loadKnowledgeFiles(kbId.value);
    }
  }, 300);
});

// 监听文件类型筛选变化
watch(selectedFileType, (newVal, oldVal) => {
  if (newVal === oldVal) return;
  if (kbId.value) {
    resetPage();
    loadKnowledgeFiles(kbId.value);
  }
});

// 监听解析状态/更新时间范围筛选变化（与文件类型行为一致）
watch([selectedParseStatus, updatedTimeRange], () => {
  if (kbId.value) {
    resetPage();
    loadKnowledgeFiles(kbId.value);
  }
}, { deep: true });

// 监听文件上传事件
const handleFileUploaded = (event: CustomEvent) => {
  const uploadedKbId = event.detail.kbId;
  console.log('接收到文件上传事件，上传的知识库ID:', uploadedKbId, '当前知识库ID:', kbId.value);
  if (uploadedKbId && uploadedKbId === kbId.value && !isFAQ.value) {
    console.log('匹配当前知识库，开始刷新文件列表');
    // 如果上传的文件属于当前知识库，使用 loadKnowledgeFiles 刷新文件列表
    resetPage(); // Reset page counter when reloading files after upload
    loadKnowledgeFiles(uploadedKbId);
  }
};

// Auto-open document detail when navigated with ?knowledge_id=xxx.
// Note: this runs both when the KB page mounts with a query param AND when a
// subsequent in-page navigation (e.g. from the global command palette) only
// changes the query without re-mounting the component — in that case kbId is
// the same and cardList may already be populated, so relying solely on the
// cardList watcher misses the trigger.
const pendingKnowledgeId = ref<string | null>(
  (route.query.knowledge_id as string) || null
);

const tryAutoOpenDocument = () => {
  if (!pendingKnowledgeId.value || !cardList.value?.length) return;
  const targetId = pendingKnowledgeId.value;
  pendingKnowledgeId.value = null;
  const card = cardList.value.find((c: KnowledgeCard) => c.id === targetId);
  if (card) {
    nextTick(() => openCardDetails(card));
  } else {
    nextTick(() => {
      openCardDetails({ id: targetId } as KnowledgeCard);
    });
  }
};

// React to later ?knowledge_id= changes on the same KB route (no remount).
watch(
  () => route.query.knowledge_id,
  (newId) => {
    if (typeof newId !== 'string' || !newId) return;
    pendingKnowledgeId.value = newId;
    // cardList is almost always already loaded at this point; if not, the
    // cardList watcher below will pick it up.
    tryAutoOpenDocument();
  },
);

// Dispatched by the global command palette when the user picks a chunk that
// lives in the KB they are already viewing — vue-router dedupes identical
// navigations, so we rely on this event instead of a URL change.
const handleOpenKnowledgeEvent = (e: Event) => {
  const detail = (e as CustomEvent<{ kbId: string; knowledgeId: string }>).detail;
  if (!detail || !detail.knowledgeId) return;
  if (detail.kbId && detail.kbId !== kbId.value) return;
  pendingKnowledgeId.value = detail.knowledgeId;
  tryAutoOpenDocument();
};

onMounted(() => {
  loadKnowledgeList();
  editorResources.ensureParserEngines();

  window.addEventListener('knowledgeFileUploaded', handleFileUploaded as EventListener);
  window.addEventListener('zealrag:open-knowledge', handleOpenKnowledgeEvent as EventListener);
});

onUnmounted(() => {
  window.removeEventListener('knowledgeFileUploaded', handleFileUploaded as EventListener);
  window.removeEventListener('zealrag:open-knowledge', handleOpenKnowledgeEvent as EventListener);
  stopMovePoll();
  if (timeout !== null) {
    clearTimeout(timeout);
    timeout = null;
  }
});
watch(() => cardList.value, (newValue) => {
  if (isFAQ.value) return;

  for (const item of newValue || []) {
    if (item.parse_status === 'completed' || item.parse_status === 'finalizing') {
      stageProgressById[item.id] = 5;
    } else if (isParseInFlight(item.parse_status) || item.parse_status === 'failed') {
      void refreshStageProgress(item);
    }
  }

  // Auto-open document if navigated with ?knowledge_id=xxx
  if (pendingKnowledgeId.value && newValue?.length) {
    tryAutoOpenDocument();
  }

  let analyzeList = [];
  // Filter items that need polling: parsing in progress OR summary generation in progress
  analyzeList = newValue.filter(needsStatusPolling);
  if (timeout !== null) {
    clearTimeout(timeout);
    timeout = null;
  }
  if (analyzeList.length) {
    updateStatus(analyzeList)
  }

}, { deep: true })
type KnowledgeCard = {
  id: string;
  knowledge_base_id?: string;
  parse_status: string;
  summary_status?: string;
  description?: string;
  file_name?: string;
  original_file_name?: string;
  display_name?: string;
  title?: string;
  type?: string;
  updated_at?: string;
  file_type?: string;
  isMore?: boolean;
  metadata?: any;
  error_message?: string;
};
// needsStatusPolling decides whether a card row is still "in flight"
// enough that the doc list should keep refreshing it. Keep in sync with
// the backend lifecycle: pending / processing are the primary parse
// phase, finalizing is the post-process fan-out (summary / question
// generation still running), and a `completed` row whose summary
// hasn't landed yet keeps polling so the description fills in.
const needsStatusPolling = (item: KnowledgeCard) => {
	return isParseInFlight(item.parse_status) ||
		(item.parse_status === 'completed' && item.summary_status === 'processing');
};

const updateStatus = (analyzeList: KnowledgeCard[]) => {
  if (timeout !== null) {
    clearTimeout(timeout);
    timeout = null;
  }
  if (!analyzeList.length) return;

  analyzeList.forEach((item) => { void refreshStageProgress(item); });

  let query = ``;
  for (let i = 0; i < analyzeList.length; i++) {
    query += `ids=${analyzeList[i].id}&`;
  }
  timeout = setTimeout(() => {
    batchQueryKnowledge(query).then((result: any) => {
      let hasChanges = false;
      if (result.success && result.data) {
        (result.data as KnowledgeCard[]).forEach((item: KnowledgeCard) => {
          const index = cardList.value.findIndex(card => card.id == item.id);
          if (index == -1) return;

          if (cardList.value[index].parse_status !== item.parse_status ||
            cardList.value[index].summary_status !== item.summary_status ||
            cardList.value[index].description !== item.description) {
            // Always update the card data
            cardList.value[index].parse_status = item.parse_status;
            cardList.value[index].summary_status = item.summary_status;
            cardList.value[index].description = item.description;
            if (item.parse_status === 'completed' || item.parse_status === 'finalizing') {
              stageProgressById[item.id] = 5;
            } else {
              void refreshStageProgress(item);
            }
            hasChanges = true;
          }
        });
      }
      // If there are no changes, the watch won't trigger, so we must manually poll again
      // Even if there are changes, we can manually poll again just to be safe.
      // The watch will clear this timeout if it triggers.
      const stillPending = cardList.value.filter(needsStatusPolling);
      if (stillPending.length > 0) {
        updateStatus(stillPending);
      }
    }).catch((_err) => {
      // 错误处理
      const stillPending = cardList.value.filter(needsStatusPolling);
      if (stillPending.length > 0) {
        updateStatus(stillPending);
      }
    });
  }, 1500);
};


// 恢复文档处理状态（用于刷新后恢复）

const closeDoc = () => {
  isCardDetails.value = false;
};
const openCardDetails = (item: KnowledgeCard) => {
  isCardDetails.value = true;
  getCardDetails(item);
};

// Open source document preview from WikiBrowser
const openSourceDoc = (knowledgeId: string) => {
  isCardDetails.value = true;
  getCardDetails({ id: knowledgeId });
};

// 悬停知识卡片时跟随鼠标显示详情气泡
const hoveredCardItem = ref<KnowledgeCard | null>(null);
const cardPopoverPos = ref({ x: 0, y: 0 });
const CARD_POPOVER_OFFSET = 16;
const cardHoverShowDelay = 300;
let cardHoverTimer: ReturnType<typeof setTimeout> | null = null;

const onCardMouseEnter = (ev: MouseEvent, item: KnowledgeCard) => {
  if (cardHoverTimer) {
    clearTimeout(cardHoverTimer);
    cardHoverTimer = null;
  }
  cardHoverTimer = setTimeout(() => {
    cardHoverTimer = null;
    hoveredCardItem.value = item;
    cardPopoverPos.value = {
      x: ev.clientX + CARD_POPOVER_OFFSET,
      y: ev.clientY + CARD_POPOVER_OFFSET,
    };
  }, cardHoverShowDelay);
};

const onCardMouseMove = (ev: MouseEvent) => {
  if (hoveredCardItem.value) {
    cardPopoverPos.value = {
      x: ev.clientX + CARD_POPOVER_OFFSET,
      y: ev.clientY + CARD_POPOVER_OFFSET,
    };
  }
};

const onCardMouseLeave = () => {
  if (cardHoverTimer) {
    clearTimeout(cardHoverTimer);
    cardHoverTimer = null;
  }
  hoveredCardItem.value = null;
};

const closeCardMoreMenu = (index: number) => {
  if (cardList.value?.[index]) {
    cardList.value[index].isMore = false;
  }
  moreIndex.value = -1;
};

const confirmDeleteKnowledge = (index: number, item: KnowledgeCard) => {
  closeCardMoreMenu(index);
  const deletedId = item?.id;
  delKnowledge(index, item, async () => {
    resetPage();
    const maxPolls = 30;
    const delayMs = 400;
    for (let i = 0; i < maxPolls; i++) {
      await loadKnowledgeFiles(kbId.value);
      const stillPresent = (cardList.value || []).some((c: KnowledgeCard) => c.id === deletedId);
      if (!stillPresent) break;
      await new Promise<void>((r) => setTimeout(r, delayMs));
    }
  });
};

const onReparseMenuClick = (index: number, item: KnowledgeCard) => {
  if (isParseInFlight(item.parse_status)) {
    MessagePlugin.info(t('knowledgeBase.rebuildInProgress'));
  }
};

const handleMoveKnowledge = async (item: KnowledgeCard) => {
  moveKnowledgeId.value = item.id;
  moveMenuMode.value = 'targets';
  moveTargetsLoading.value = true;
  moveTargetKbs.value = [];
  try {
    const res: any = await listMoveTargets(kbId.value);
    moveTargetKbs.value = res.data || [];
  } catch {
    moveTargetKbs.value = [];
  } finally {
    moveTargetsLoading.value = false;
  }
};

const handleMoveSelectTarget = (kb: any) => {
  moveSelectedTargetId.value = kb.id;
  moveSelectedTargetName.value = kb.name;
  moveMode.value = 'reuse_vectors';
  moveMenuMode.value = 'confirm';
};

const handleMoveBack = () => {
  if (moveMenuMode.value === 'confirm') {
    moveMenuMode.value = 'targets';
  } else {
    moveMenuMode.value = 'normal';
  }
};

const handleMoveConfirm = async () => {
  if (!moveSelectedTargetId.value || moveSubmitting.value) return;
  moveSubmitting.value = true;
  try {
    const res: any = await moveKnowledge({
      knowledge_ids: [moveKnowledgeId.value],
      source_kb_id: kbId.value,
      target_kb_id: moveSelectedTargetId.value,
      mode: moveMode.value,
    });
    const taskId = res.data?.task_id;
    MessagePlugin.info(t('knowledgeBase.moveStarted'));
    // Close the card menu
    moveMenuMode.value = 'normal';
    cardList.value.forEach(c => { c.isMore = false; });

    if (taskId) {
      startMovePoll(taskId);
    } else {
      moveSubmitting.value = false;
      resetPage(); // Reset page counter when reloading files after move
      loadKnowledgeFiles(kbId.value);
    }
  } catch (e: any) {
    MessagePlugin.error(e?.message || t('knowledgeBase.moveFailed'));
    moveSubmitting.value = false;
  }
};

const startMovePoll = (taskId: string) => {
  if (movePollTimer) clearInterval(movePollTimer);
  movePollTimer = setInterval(async () => {
    try {
      const res: any = await getKnowledgeMoveProgress(taskId);
      const data = res.data;
      if (!data) return;
      if (data.status === 'completed') {
        stopMovePoll();
        moveSubmitting.value = false;
        const failed = data.failed || 0;
        if (failed > 0) {
          MessagePlugin.warning(t('knowledgeBase.moveCompletedWithErrors', { success: (data.processed || 0) - failed, failed }));
        } else {
          MessagePlugin.success(t('knowledgeBase.moveCompleted'));
        }
        resetPage(); // Reset page counter when reloading files after move completion
        loadKnowledgeFiles(kbId.value);
      } else if (data.status === 'failed') {
        stopMovePoll();
        moveSubmitting.value = false;
        MessagePlugin.error(t('knowledgeBase.moveFailed'));
      }
    } catch {
      // ignore poll errors
    }
  }, 2000);
};

const stopMovePoll = () => {
  if (movePollTimer) {
    clearInterval(movePollTimer);
    movePollTimer = null;
  }
};

const documentTitle = computed(() => {
  if (kbInfo.value?.name) {
    return `${kbInfo.value.name} · ${t('knowledgeEditor.document.title')}`;
  }
  return t('knowledgeEditor.document.title');
});

const ensureDocumentKbReady = () => {
  if (isFAQ.value) {
    MessagePlugin.warning(t('knowledgeBase.operationNotSupportedForType'));
    return false;
  }
  if (!kbId.value) {
    MessagePlugin.warning(t('knowledgeEditor.messages.missingId'));
    return false;
  }
  if (!kbInfo.value || !kbInfo.value.summary_model_id) {
    MessagePlugin.warning(t('knowledgeBase.notInitialized'));
    return false;
  }
  // Embedding model only required when RAG indexing is enabled
  const strategy = (kbInfo.value as any).indexing_strategy
  const needsEmbedding = !strategy || strategy.vector_enabled || strategy.keyword_enabled
  if (needsEmbedding && !kbInfo.value.embedding_model_id) {
    MessagePlugin.warning(t('knowledgeBase.notInitialized'));
    return false;
  }
  return true;
};


const IMAGE_EXTENSIONS = ['jpg', 'jpeg', 'png', 'gif', 'bmp', 'webp'];

const uploadConfirmStore = useUploadConfirmStore();

const getFolderUploadFileName = (file: File) => {
  const relativePath = (file as any).webkitRelativePath;
  if (!relativePath) return undefined;
  const pathParts = relativePath.split('/');
  if (pathParts.length <= 2) return undefined;
  const subPath = pathParts.slice(1, -1).join('/');
  return `${subPath}/${file.name}`;
};

const showUploadResultMessages = (
  successCount: number,
  failCount: number,
  totalCount: number,
  mode: 'document' | 'folder',
) => {
  if (mode === 'folder') {
    if (failCount === 0) {
      MessagePlugin.success(t('knowledgeBase.uploadAllSuccess', { count: successCount }));
    } else if (successCount > 0) {
      MessagePlugin.warning(t('knowledgeBase.uploadPartialSuccess', { success: successCount, fail: failCount }));
    } else {
      MessagePlugin.error(t('knowledgeBase.uploadAllFailed'));
    }
    return;
  }

  if (totalCount === 1) {
    if (successCount === 1) {
      MessagePlugin.success(t('knowledgeBase.uploadSuccess'));
    }
    return;
  }

  if (failCount === 0) {
    MessagePlugin.success(t('knowledgeBase.allUploadSuccess', { count: successCount }));
  } else if (successCount > 0) {
    MessagePlugin.warning(t('knowledgeBase.partialUploadSuccess', { success: successCount, fail: failCount }));
  } else {
    MessagePlugin.error(t('knowledgeBase.allUploadFailed', { count: failCount }));
  }
};

const executeUploadBatch = async (
  files: File[],
  options: { processConfig?: KnowledgeProcessOverrides } = {},
) => {
  const targetKbId = kbId.value;
  if (!targetKbId || files.length === 0) {
    return { successCount: 0, failCount: files.length };
  }

  let successCount = 0;
  let failCount = 0;
  const totalCount = files.length;
  const hasFolderPaths = files.some((file) => {
    const relativePath = (file as File & { webkitRelativePath?: string }).webkitRelativePath;
    return !!relativePath && relativePath.split('/').length > 2;
  });

  for (const file of files) {
    try {
      const uploadData: {
        file: File
        fileName?: string
        process_config?: KnowledgeProcessOverrides
      } = { file };

      const fileName = getFolderUploadFileName(file);
      if (fileName) uploadData.fileName = fileName;
      if (options.processConfig) {
        uploadData.process_config = options.processConfig;
      }

      const responseData: any = await uploadKnowledgeFile(targetKbId, uploadData);
      const isSuccess = responseData?.success || responseData?.code === 200 || responseData?.status === 'success' || (!responseData?.error && responseData);
      if (isSuccess) {
        successCount++;
      } else {
        failCount++;
        if (totalCount === 1) {
          let errorMessage = t('knowledgeBase.uploadFailed');
          if (responseData?.error?.message) {
            errorMessage = responseData.error.message;
          } else if (responseData?.message) {
            errorMessage = responseData.message;
          }
          if (responseData?.code === 'duplicate_file' || responseData?.error?.code === 'duplicate_file') {
            errorMessage = t('knowledgeBase.fileExists');
          }
          MessagePlugin.error(errorMessage);
        }
      }
    } catch (error: any) {
      failCount++;
      if (totalCount === 1) {
        let errorMessage = error?.error?.message || error?.message || t('knowledgeBase.uploadFailed');
        if (error?.code === 'duplicate_file') {
          errorMessage = t('knowledgeBase.fileExists');
        }
        MessagePlugin.error(errorMessage);
      }
    }
  }

  if (successCount > 0) {
    window.dispatchEvent(new CustomEvent('knowledgeFileUploaded', {
      detail: { kbId: targetKbId },
    }));
  }

  showUploadResultMessages(successCount, failCount, totalCount, hasFolderPaths ? 'folder' : 'document');
  return { successCount, failCount };
};

const handleUploadConfirmResult = async (result: UploadConfirmResult) => {
  const files = result.files || [];
  const processConfig = result.processConfig;

  if (files.length > 0) {
    const hasFolderPaths = files.some((file) => {
      const relativePath = (file as File & { webkitRelativePath?: string }).webkitRelativePath;
      return !!relativePath && relativePath.split('/').length > 2;
    });
    if (hasFolderPaths) {
      MessagePlugin.info(t('knowledgeBase.uploadingFolder', { total: files.length }));
    }
    await executeUploadBatch(files, { processConfig });
  }

};

const openUploadConfirmDialog = async (files: File[]) => {
  if (!kbInfo.value) return;
  if (files.length === 0) return;
  try {
    const result = await uploadConfirmStore.open({
      mode: 'file',
      kbInfo: kbInfo.value,
      files,
      acceptFileTypes: acceptFileTypes.value,
      supportedFileTypes: [...supportedFileTypes.value],
    });
    await handleUploadConfirmResult(result);
  } catch {
    // cancelled
  }
};

const handleUploadSourceFiles = (files: File[]) => {
  if (!ensureDocumentKbReady()) return;
  if (files.length === 0) return;
  openUploadConfirmDialog(files);
};

const handleEmptyDragEnter = () => {
  if (canWrite.value) emptyDropActive.value = true;
};

const handleEmptyDragOver = (event: DragEvent) => {
  if (!canWrite.value || !event.dataTransfer) return;
  event.dataTransfer.dropEffect = 'copy';
  emptyDropActive.value = true;
};

const handleEmptyDragLeave = (event: DragEvent) => {
  const target = event.currentTarget as HTMLElement | null;
  const related = event.relatedTarget as Node | null;
  if (!target || !related || !target.contains(related)) {
    emptyDropActive.value = false;
  }
};

const handleEmptyDrop = (event: DragEvent) => {
  emptyDropActive.value = false;
  if (!canWrite.value || !ensureDocumentKbReady()) return;
  const files = event.dataTransfer?.files;
  if (!files?.length) return;

  const maxFileSizeMB = kbInfo.value?.max_file_size_mb || 50;
  const result = filterUploadFiles(files, {
    supportedFileTypes: supportedFileTypes.value,
    maxFileSizeMB,
    multiFile: files.length > 1,
  });
  if (result.unsupportedVideoCount > 0) {
    MessagePlugin.warning(t('knowledgeBase.unsupportedVideos', { count: result.unsupportedVideoCount }));
  }
  if (result.skippedCount > 0) {
    MessagePlugin.warning(`已跳过 ${result.skippedCount} 个格式不支持或超过 ${maxFileSizeMB} MB 的文件`);
  }
  if (result.validFiles.length > 0) {
    handleUploadSourceFiles(result.validFiles);
  }
};

const handleOpenKBSettings = () => {
  if (!kbId.value) {
    MessagePlugin.warning(t('knowledgeEditor.messages.missingId'));
    return;
  }
  uiStore.openKBSettings(kbId.value);
};

const handleNavigateToKbList = () => {
  router.push('/platform/knowledge-bases');
};

const handleNavigateToCurrentKB = () => {
  if (!kbId.value) return;
  router.push(`/platform/knowledge-bases/${kbId.value}`);
};

const handleKnowledgeDropdownSelect = (data: { value: string }) => {
  if (!data?.value) return;
  if (data.value === kbId.value) return;
  router.push(`/platform/knowledge-bases/${data.value}`);
};

const handleViewTrace = (index: number, item: KnowledgeCard) => {
  if (cardList.value[index]) {
    cardList.value[index].isMore = false;
  }
  moreIndex.value = -1;
  progressItem.value = { ...item };
  progressDrawerVisible.value = true;
  void refreshStageProgress(item);
};

const confirmRebuildKnowledge = async (index: number, item: KnowledgeCard) => {
  if (isFAQ.value) return;
  if (!canRetryOwnDocument(item)) return;
  if (!item?.id) {
    MessagePlugin.warning(t('knowledgeEditor.messages.missingId'));
    return;
  }
  if (isParseInFlight(item.parse_status)) {
    MessagePlugin.info(t('knowledgeBase.rebuildInProgress'));
    return;
  }
  closeCardMoreMenu(index);

  // No KB context to seed the dialog defaults — fall back to a direct reparse
  // that reuses the overrides stored at upload time.
  if (!kbInfo.value) {
    await submitReparse(item.id);
    return;
  }

  // Prefill the confirm dialog with the overrides this doc was last parsed with.
  let processOverrides: KnowledgeProcessOverrides | null = item.metadata?.process_overrides ?? null;
  let fileName = item.file_name || item.title || '';
  let fileType = item.file_type || '';
  try {
    const detail: any = await getKnowledgeDetails(item.id);
    if (detail?.success && detail.data) {
      processOverrides = detail.data.metadata?.process_overrides ?? processOverrides;
      fileName = detail.data.file_name || detail.data.title || fileName;
      fileType = detail.data.file_type || fileType;
    }
  } catch {
    // fall back to the list item's fields
  }

  try {
    const result = await uploadConfirmStore.open({
      mode: 'reparse',
      kbInfo: kbInfo.value,
      reparse: { knowledgeId: item.id, fileName, fileType, processOverrides },
    });
    if (result.mode === 'reparse' && result.reparse) {
      await submitReparse(result.reparse.knowledgeId, result.processConfig);
    }
  } catch {
    // cancelled
  }
};

const submitReparse = async (id: string, processConfig?: KnowledgeProcessOverrides) => {
  try {
    await reparseKnowledge(id, processConfig ? { process_config: processConfig } : undefined);
    stageProgressById[id] = 1;
    MessagePlugin.success(t('knowledgeBase.rebuildSubmitted'));
    resetPage();
    loadKnowledgeFiles(kbId.value);
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('knowledgeBase.rebuildFailed'));
  }
};

const handleScroll = () => {
  if (isFAQ.value) return;
  if (docListLoading.value) return;
  if (scrollLoading) return;
  const currentKbId = kbId.value;
  if (!currentKbId) return;
  const element = knowledgeScroll.value;
  if (element) {
    let pageNum = Math.ceil(total.value / pageSize)
    const { scrollTop, scrollHeight, clientHeight } = element;
    if (scrollTop + clientHeight >= scrollHeight - 10) {
      if (cardList.value.length < total.value && page < pageNum) {
        page++;
        scrollLoading = true;
        getKnowled({ page, page_size: pageSize, ...filterParams.value }, currentKbId).finally(() => {
          if (isCurrentKb(currentKbId)) {
            scrollLoading = false;
          }
        });
      }
    }
  }
};
const getDoc = (page: number) => {
  getfDetails(details.id, page)
};

const toggleSelectRow = (id: string, checked: boolean, shiftKey?: boolean) => {
  const items = cardList.value || [];
  const idx = items.findIndex((i: KnowledgeCard) => i.id === id);
  if (shiftKey && lastSelectedIndex >= 0 && idx >= 0) {
    const [s, e] = idx < lastSelectedIndex
      ? [idx, lastSelectedIndex]
      : [lastSelectedIndex, idx];
    for (let i = s; i <= e; i++) {
      if (checked) selectedIds.value.add(items[i].id);
      else selectedIds.value.delete(items[i].id);
    }
  } else {
    if (checked) selectedIds.value.add(id);
    else selectedIds.value.delete(id);
  }
  lastSelectedIndex = idx;
};

const onCardGridCheckboxChange = (id: string, checked: boolean, ctx?: { e?: Event }) => {
  const me = ctx?.e as MouseEvent | undefined;
  toggleSelectRow(id, checked, !!me?.shiftKey);
};

const toggleSelectAll = (checked: boolean) => {
  if (checked) {
    for (const item of cardList.value || []) selectedIds.value.add(item.id);
  } else {
    for (const item of cardList.value || []) selectedIds.value.delete(item.id);
  }
};

const clearSelection = () => {
  selectedIds.value.clear();
  lastSelectedIndex = -1;
};

// Batch (multi-select) mode mirrors the session list's "批量管理" UX: while off,
// no checkbox is rendered so the title doesn't jitter on hover; while on,
// checkboxes are persistent and clicking a card toggles its selection.
const batchMode = ref(false);
const toggleBatchMode = () => {
  batchMode.value = !batchMode.value;
  if (!batchMode.value) clearSelection();
};
// "取消选择" / 退出批量管理：清空选择，并退出 grid 视图下的批量模式。
const handleBatchCancel = () => {
  clearSelection();
  batchMode.value = false;
};
// 切到卡片视图时，如果列表视图里已经勾选过文档，需要自动开启批量管理模式，
// 否则卡片视图默认不渲染 checkbox，会看不到勾选态。
watch(viewMode, (mode) => {
  if (mode === 'grid' && selectedIds.value.size > 0) {
    batchMode.value = true;
  }
});
// Triggered from a card / row "..." menu — match the session-list UX where
// the menu item simply opens batch mode (no auto-selection).
const handleEnterBatchFromCard = (item: any) => {
  if (item) item.isMore = false;
  moreIndex.value = -1;
  clearSelection();
  batchMode.value = true;
};
const {
  onContainerMouseDown: onDocMarqueeMouseDown,
  marqueeVisible: docMarqueeVisible,
  marqueeMode: docMarqueeMode,
  boxStyle: docMarqueeBoxStyle,
  shouldSuppressClick: shouldSuppressDocClick,
} = useMarqueeSelect({
  containerRef: knowledgeScroll,
  itemSelector: '.knowledge-card[data-select-id], .doc-list-row[data-select-id]',
  selectedIds,
  getItemId: (el) => el.dataset.selectId || null,
  enabled: computed(() => canEdit.value && !isFAQ.value && cardList.value.length > 0),
  onSelectionStart: () => {
    batchMode.value = true;
  },
});

const openKnowledgeItem = async (item: KnowledgeCard) => {
  if (shouldSuppressDocClick()) return;
	isCardDetails.value = true;
	await getCardDetails(item);
};

const onCardClick = (item: KnowledgeCard) => {
  if (batchMode.value) {
    onCardGridCheckboxChange(item.id, !selectedIds.value.has(item.id));
    return;
  }
  openKnowledgeItem(item);
};

const confirmBatchDelete = async () => {
  if (batchDeleting.value || selectedIds.value.size === 0) return;
  const ids = Array.from(selectedIds.value);
  const deletedIdSet = new Set(ids);
  batchDeleting.value = true;
  try {
    const res: any = await batchDeleteKnowledge(kbId.value, ids);
    if (res?.success) {
      MessagePlugin.success(t('knowledgeBase.batchDeleteSuccess', { count: ids.length }));
      clearSelection();
      batchMode.value = false;
      resetPage();
      // 后端将批量删除放入异步队列，立刻拉列表仍可能包含待删项；短轮询直到列表与后端一致或超时
      const maxPolls = 30;
      const delayMs = 400;
      for (let i = 0; i < maxPolls; i++) {
        await loadKnowledgeFiles(kbId.value);
        const stillPresent = (cardList.value || []).some((c: KnowledgeCard) => deletedIdSet.has(c.id));
        if (!stillPresent) break;
        await new Promise<void>((r) => setTimeout(r, delayMs));
      }
    } else {
      MessagePlugin.error(res?.message || t('knowledgeBase.batchDeleteFailed'));
    }
  } catch (e: any) {
    MessagePlugin.error(e?.message || t('knowledgeBase.batchDeleteFailed'));
  } finally {
    batchDeleting.value = false;
  }
};

const confirmCancelParseKnowledge = async (item: KnowledgeCard) => {
  if (!item?.id) return;
  try {
    await cancelKnowledgeParse(item.id);
    MessagePlugin.success(t('knowledgeBase.cancelParseSubmitted'));
    loadKnowledgeFiles(kbId.value);
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('knowledgeBase.cancelParseFailed'));
  }
};

// Bridge list-view actions back to existing per-card handlers.
const handleListAction = (
  action: 'trace' | 'reparse' | 'cancel-parse' | 'move' | 'delete',
  item: KnowledgeCard,
) => {
  const idx = (cardList.value || []).findIndex((i: KnowledgeCard) => i.id === item.id);
  if (action === 'trace') return handleViewTrace(idx, item);
  if (action === 'reparse') return confirmRebuildKnowledge(idx, item);
  if (action === 'cancel-parse') return confirmCancelParseKnowledge(item);
  if (action === 'move') return handleMoveKnowledge(item);
  if (action === 'delete') return confirmDeleteKnowledge(idx, item);
};

// Clear selection on filter/kb change to avoid acting on hidden items.
watch(
  [docSearchKeyword, selectedFileType, selectedParseStatus, updatedTimeRange, kbId],
  () => {
    clearSelection();
  },
);

// After cardList reloads: stable keys rely on correct indices for shift-range; clamp anchor index.
watch(cardList, () => {
  const items = cardList.value || [];
  const n = items.length;
  if (lastSelectedIndex >= n) {
    lastSelectedIndex = n > 0 ? n - 1 : -1;
  }
  if (moreIndex.value >= n) {
    moreIndex.value = -1;
  }
  if (selectedIds.value.size === 0) return;
  const visible = new Set(items.map((i: KnowledgeCard) => i.id));
  for (const id of selectedIds.value) {
    if (!visible.has(id)) selectedIds.value.delete(id);
  }
}, { deep: false });

// 处理知识库编辑成功后的回调
const handleKBEditorSuccess = (kbIdValue: string) => {
  chatResources.invalidateKnowledgeBaseDetail(kbIdValue);
  chatResources.invalidate('knowledgeBases');
  loadKnowledgeList();
  if (kbIdValue === kbId.value) {
    loadKnowledgeBaseInfo(kbIdValue, true);
  }
};

const getTitle = (session_id: string, value: string) => {
  const now = new Date().toISOString();
  let obj = {
    title: t('knowledgeBase.newSession'),
    path: `chat/${session_id}`,
    id: session_id,
    isMore: false,
    isNoTitle: true,
    created_at: now,
    updated_at: now
  };
  usemenuStore.updataMenuChildren(obj);
  usemenuStore.changeIsFirstSession(true);
  usemenuStore.changeFirstQuery(value);
  router.push(`/platform/chat/${session_id}`);
};

async function createNewSession(value: string): Promise<void> {
  // Session 不再和知识库绑定，直接创建 Session
  createSessions({}).then(res => {
    if (res.data && res.data.id) {
      getTitle(res.data.id, value);
    } else {
      // 错误处理
      console.error(t('knowledgeBase.createSessionFailed'));
    }
  }).catch(error => {
    console.error(t('knowledgeBase.createSessionError'), error);
  });
}
</script>

<template>
  <template v-if="!isFAQ">
    <div class="knowledge-layout">
      <div class="document-header">
        <div class="document-header-title">
          <div class="workspace-heading">
            <div>
              <span class="workspace-eyebrow">知识库工作区</span>
              <h1>{{ kbInfo?.name || '知识库' }}</h1>
            </div>
            <div class="workspace-indexes" aria-label="Index status">
              <span class="is-active"><i></i>向量</span>
            </div>
          </div>
          <div class="document-title-row">
            <h2 class="document-breadcrumb">
              <button type="button" class="breadcrumb-link" @click="handleNavigateToKbList">
                {{ $t('menu.knowledgeBase') }}
              </button>
              <t-icon name="chevron-right" class="breadcrumb-separator" />
              <KBSwitcherDropdown v-if="knowledgeList.length" :kb-list="knowledgeList" :current-kb-id="kbId"
                @select="(id) => handleKnowledgeDropdownSelect({ value: id })">
                <button type="button" class="breadcrumb-link dropdown" :disabled="!kbId">
                  <template v-if="!kbInfo">
                    <t-skeleton animation="gradient" :row-col="[{ width: '120px', height: '20px' }]" />
                  </template>
                  <template v-else>
                    <span>{{ kbInfo.name }}</span>
                    <t-icon name="chevron-down" />
                  </template>
                </button>
              </KBSwitcherDropdown>
              <button v-else type="button" class="breadcrumb-link" :disabled="!kbId" @click="handleNavigateToCurrentKB">
                <template v-if="!kbInfo">
                  <t-skeleton animation="gradient" :row-col="[{ width: '120px', height: '20px' }]" />
                </template>
                <template v-else>
                  {{ kbInfo.name }}
                </template>
              </button>
              <t-icon name="chevron-right" class="breadcrumb-separator" />
              <span class="breadcrumb-current">{{ $t('knowledgeEditor.document.title') }}</span>
            </h2>
            <!-- 标题行右侧的动作锚点：聚拢"信息"和"设置"两个圆形按钮。 -->
            <div class="kb-title-actions">
              <t-popconfirm
                v-if="canManage"
                theme="warning"
                :content="$t('knowledgeBase.fullRebuildConfirm')"
                :confirm-btn="{ content: $t('common.confirm'), theme: 'primary' }"
                :cancel-btn="{ content: $t('common.cancel') }"
                placement="bottom-right"
                @confirm="startFullRebuild"
              >
                <t-tooltip :content="$t('knowledgeBase.fullRebuildTip')" placement="top">
                  <button
                    type="button"
                    class="kb-settings-button"
                    :class="{ 'is-spinning': fullRebuildRunning }"
                    :disabled="!kbId || fullRebuildRunning"
                    :aria-label="$t('knowledgeBase.fullRebuild')"
                  >
                    <t-icon :name="fullRebuildRunning ? 'loading' : 'refresh'" size="16px" />
                  </button>
                </t-tooltip>
              </t-popconfirm>
			  <t-tooltip content="共享成员" placement="top">
				<button type="button" class="kb-settings-button" :disabled="!kbId" aria-label="共享成员" @click="sharingVisible = true">
				  <t-icon name="usergroup" size="16px" />
				</button>
			  </t-tooltip>
              <t-tooltip v-if="canConfigure" :content="$t('knowledgeBase.settings')" placement="top">
                <button type="button" class="kb-settings-button" :disabled="!kbId" @click="handleOpenKBSettings">
                  <t-icon name="setting" size="16px" />
                </button>
              </t-tooltip>
            </div>
          </div>
          <p class="document-subtitle">{{ $t('knowledgeEditor.document.subtitle') }}</p>
          <p v-if="fullRebuildRunning" class="full-rebuild-status is-running">
            <t-icon name="loading" class="status-icon" />
            <span>{{ $t('knowledgeBase.fullRebuildRunning') }}</span>
          </p>
          <p v-else-if="fullRebuildState?.status === 'failed'" class="full-rebuild-status is-failed">
            <t-icon name="error-circle" class="status-icon" />
            <span>{{ $t('knowledgeBase.fullRebuildFailedReason', { reason: fullRebuildState.error || $t('common.unknownError') }) }}</span>
          </p>
          <p v-if="unsupportedFileTypes.length" class="parser-hint" @click="goToParserSettings">
            <t-icon name="info-circle" class="parser-hint-icon" />
            <span>{{$t('knowledgeBase.unsupportedTypesHint', {
              types: unsupportedFileTypes.map(t => '.' + t).join('、')
            })
              }}</span>
            <span class="parser-hint-link">{{ $t('knowledgeBase.goToParserSettings') }} →</span>
          </p>
        </div>
      </div>

      <div class="knowledge-main">
          <div class="document-content">
            <div class="doc-card-area">
              <!-- 搜索栏、筛选与添加文档 -->
              <div class="doc-filter-bar">
                <t-input v-model.trim="docSearchKeyword" :placeholder="$t('knowledgeBase.docSearchPlaceholder')"
                  clearable class="doc-search-input" @clear="loadKnowledgeFiles(kbId)"
                  @enter="loadKnowledgeFiles(kbId)">
                  <template #prefix-icon>
                    <t-icon name="search" size="16px" />
                  </template>
                </t-input>
                <t-select v-model="selectedFileType" :options="fileTypeOptions"
                  :placeholder="$t('knowledgeBase.fileTypeFilter')" class="doc-type-select" clearable />
                <t-select v-model="selectedParseStatus" :options="parseStatusOptions"
                  :placeholder="$t('knowledgeBase.parseStatusFilter')" class="doc-type-select" clearable />
                <t-date-range-picker v-model="updatedTimeRange"
                  :placeholder="[$t('knowledgeBase.updatedTimeFrom'), $t('knowledgeBase.updatedTimeTo')]"
                  :disable-date="disableFutureDate" class="doc-date-range" clearable allow-input />
                <div class="doc-view-toggle" role="group" :aria-label="$t('knowledgeBase.viewModeToggle')">
                  <t-tooltip :content="$t('knowledgeBase.viewModeGrid')" placement="top">
                    <button type="button" class="doc-view-toggle-btn" :class="{ active: viewMode === 'grid' }"
                      @click="viewMode = 'grid'" :aria-pressed="viewMode === 'grid'">
                      <t-icon name="view-module" size="16px" />
                    </button>
                  </t-tooltip>
                  <t-tooltip :content="$t('knowledgeBase.viewModeList')" placement="top">
                    <button type="button" class="doc-view-toggle-btn" :class="{ active: viewMode === 'list' }"
                      @click="viewMode = 'list'" :aria-pressed="viewMode === 'list'">
                      <t-icon name="view-list" size="16px" />
                    </button>
                  </t-tooltip>
                </div>
                <div v-if="canWrite" class="doc-filter-actions">
                  <KbUploadSourceDropdown
                    :accept-file-types="acceptFileTypes"
                    :supported-file-types="[...supportedFileTypes]"
                    :max-file-size-mb="kbInfo?.max_file_size_mb || 50"
                    trigger-icon="file-add"
                    trigger-class="content-bar-icon-btn"
                    :tooltip="t('knowledgeBase.addDocument')"
                    placement="bottom-right"
                    @files="handleUploadSourceFiles"
                  />
                </div>
              </div>
              <div class="doc-scroll-container" :class="{ 'is-empty': !cardList.length && !docListLoading, 'is-marquee-active': docMarqueeVisible }" ref="knowledgeScroll"
                @scroll="handleScroll" @mousedown="onDocMarqueeMouseDown">
                <div
                  v-if="docMarqueeVisible"
                  class="doc-marquee-box"
                  :class="{ 'is-add': docMarqueeMode === 'add', 'is-subtract': docMarqueeMode === 'subtract' }"
                  :style="docMarqueeBoxStyle"
                  aria-hidden="true"
                />
                <!-- 文档骨架屏 -->
                <div v-if="docListLoading && cardList.length === 0" class="doc-card-list doc-card-list-animated">
                  <div v-for="n in 8" :key="'doc-skel-' + n" class="knowledge-card knowledge-card-skeleton">
                    <div class="card-content">
                      <div class="card-content-nav">
                        <t-skeleton animation="gradient" :row-col="[{ width: '70%', height: '18px' }]" />
                      </div>
                      <t-skeleton animation="gradient"
                        :row-col="[{ width: '100%', height: '14px' }, { width: '60%', height: '14px' }]" />
                    </div>
                    <div class="card-bottom">
                      <t-skeleton animation="gradient"
                        :row-col="[[{ width: '80px', height: '14px' }, { width: '40px', height: '18px', type: 'rect' }]]" />
                    </div>
                  </div>
                </div>
                <template v-else-if="cardList.length && viewMode === 'grid'">
                  <div class="doc-card-list doc-card-list-animated">
                    <!-- 现有文档卡片 -->
                    <div class="knowledge-card"
                      :class="{ 'is-selected': selectedIds.has(item.id), 'batch-mode': batchMode }"
                      :data-select-id="item.id"
                      v-for="(item, index) in cardList" :key="item.id" @click="onCardClick(item)"
                      @mouseenter="onCardMouseEnter($event, item)" @mousemove="onCardMouseMove($event)"
                      @mouseleave="onCardMouseLeave">
                      <div class="card-content">
                        <div class="card-content-nav">
                          <div v-if="canEdit && batchMode" class="card-nav-check" @click.stop>
                            <t-checkbox class="card-select-checkbox" size="small" :checked="selectedIds.has(item.id)"
                              :title="item.file_name"
                              @change="(checked: boolean, ctx: { e?: Event }) => onCardGridCheckboxChange(item.id, checked, ctx)" />
                          </div>
                          <span class="card-content-title" :title="item.file_name">{{ item.file_name }}</span>
                          <t-popup v-if="canEdit || canRetryOwnDocument(item)" v-model="item.isMore" overlayClassName="card-more"
                            :on-visible-change="(v: boolean) => onCardMoreVisibleChange(v, item)" trigger="click"
                            destroy-on-close
                            placement="bottom-right">
                            <div variant="outline" class="more-wrap" @click.stop="openMore(index)"
                              :class="[moreIndex == index ? 'active-more' : '']">
                              <t-icon class="more-icon" name="more" size="16px" />
                            </div>
                            <template #content>
                              <!-- Normal menu -->
                              <div v-if="moveMenuMode === 'normal'" class="card-menu">
                                <div v-if="item.parse_status !== 'draft'" class="card-menu-item"
                                  @click.stop="handleViewTrace(index, item)">
                                  <t-icon class="icon" name="chart-bar" />
                                  <span>{{ t('knowledgeStages.viewTrace') }}</span>
                                </div>
                                <div
                                  v-if="canEdit && isParseInFlight(item.parse_status)"
                                  class="card-menu-item"
                                  @click.stop="onReparseMenuClick(index, item)"
                                >
                                  <t-icon class="icon" name="refresh" />
                                  <span>{{ t('knowledgeBase.rebuildDocument') }}</span>
                                </div>
                                <div
                                  v-else
                                  class="card-menu-item"
                                  @click.stop="confirmRebuildKnowledge(index, item)"
                                >
                                  <t-icon class="icon" name="refresh" />
                                  <span>{{ t('knowledgeBase.rebuildDocument') }}</span>
                                </div>
                                <t-popconfirm
                                  v-if="isParseInFlight(item.parse_status)"
                                  theme="warning"
                                  :content="t('knowledgeBase.cancelParseConfirmBody', { title: item.file_name || item.title || item.id })"
                                  :confirm-btn="{ content: t('knowledgeBase.cancelParse'), theme: 'danger' }"
                                  :cancel-btn="{ content: t('common.cancel') }"
                                  placement="left"
                                  @confirm="confirmCancelParseKnowledge(item)"
                                >
                                  <div class="card-menu-item danger" @click.stop>
                                    <t-icon class="icon" name="close-circle" />
                                    <span>{{ t('knowledgeBase.cancelParse') }}</span>
                                  </div>
                                </t-popconfirm>
                                <div v-if="canMutateKnowledge" class="card-menu-item"
                                  @click.stop="handleMoveKnowledge(item)">
                                  <t-icon class="icon" name="swap" />
                                  <span>{{ t('knowledgeBase.moveDocument') }}</span>
                                </div>
                                <div v-if="canMutateKnowledge" class="card-menu-item"
                                  @click.stop="handleEnterBatchFromCard(item)">
                                  <t-icon class="icon" name="queue" />
                                  <span>{{ t('menu.batchManage') }}</span>
                                </div>
                                <t-popconfirm v-if="canEdit"
                                  theme="warning"
                                  :content="t('knowledgeBase.confirmDeleteDocument', { fileName: item.file_name || '' })"
                                  :confirm-btn="{ content: t('knowledgeBase.confirmDelete'), theme: 'danger' }"
                                  :cancel-btn="{ content: t('common.cancel') }"
                                  placement="left"
                                  @confirm="confirmDeleteKnowledge(index, item)"
                                >
                                  <div class="card-menu-item danger" @click.stop>
                                    <t-icon class="icon" name="delete" />
                                    <span>{{ t('knowledgeBase.deleteDocument') }}</span>
                                  </div>
                                </t-popconfirm>
                              </div>

                              <!-- Move: target KB list -->
                              <div v-else-if="moveMenuMode === 'targets'" class="card-menu move-menu">
                                <div class="move-menu-header" @click.stop="handleMoveBack">
                                  <t-icon name="chevron-left" size="16px" />
                                  <span>{{ t('knowledgeBase.moveToKnowledgeBase') }}</span>
                                </div>
                                <div v-if="moveTargetsLoading" class="move-menu-loading">
                                  <t-loading size="small" />
                                </div>
                                <div v-else-if="moveTargetKbs.length === 0" class="move-menu-empty">
                                  {{ t('knowledgeBase.moveNoTargets') }}
                                </div>
                                <template v-else>
                                  <div v-for="kb in moveTargetKbs" :key="kb.id" class="card-menu-item"
                                    @click.stop="handleMoveSelectTarget(kb)">
                                    <t-icon class="icon" name="root-list" />
                                    <span class="move-target-name">{{ kb.name }}</span>
                                    <span v-if="kb.knowledge_count !== undefined" class="move-target-count">{{
                                      kb.knowledge_count }}</span>
                                  </div>
                                </template>
                              </div>

                              <!-- Move: confirm with mode selection -->
                              <div v-else-if="moveMenuMode === 'confirm'" class="card-menu move-menu">
                                <div class="move-menu-header" @click.stop="handleMoveBack">
                                  <t-icon name="chevron-left" size="16px" />
                                  <span>{{ t('knowledgeBase.moveConfirmTitle') }}</span>
                                </div>
                                <div class="move-confirm-body">
                                  <div class="move-target-info">
                                    <t-icon name="arrow-right" size="14px" />
                                    <span>{{ moveSelectedTargetName }}</span>
                                  </div>
                                  <div class="move-mode-item" :class="{ active: moveMode === 'reuse_vectors' }"
                                    @click.stop="moveMode = 'reuse_vectors'">
                                    <t-radio :checked="moveMode === 'reuse_vectors'" />
                                    <div class="move-mode-text">
                                      <span class="move-mode-label">{{ t('knowledgeBase.moveModeReuseVectors') }}</span>
                                      <span class="move-mode-desc">{{ t('knowledgeBase.moveModeReuseVectorsDesc')
                                      }}</span>
                                    </div>
                                  </div>
                                  <div class="move-mode-item" :class="{ active: moveMode === 'reparse' }"
                                    @click.stop="moveMode = 'reparse'">
                                    <t-radio :checked="moveMode === 'reparse'" />
                                    <div class="move-mode-text">
                                      <span class="move-mode-label">{{ t('knowledgeBase.moveModeReparse') }}</span>
                                      <span class="move-mode-desc">{{ t('knowledgeBase.moveModeReparseDesc') }}</span>
                                    </div>
                                  </div>
                                  <div class="move-confirm-actions">
                                    <t-button size="small" variant="outline" @click.stop="handleMoveBack">{{
                                      t('common.cancel') }}</t-button>
                                    <t-button size="small" theme="primary" :loading="moveSubmitting"
                                      @click.stop="handleMoveConfirm">{{
                                        t('knowledgeBase.moveConfirm') }}</t-button>
                                  </div>
                                </div>
                              </div>
                            </template>
                          </t-popup>
                        </div>
                        <div
                          v-if="isParseInFlight(item.parse_status)"
                          class="card-analyze"
                        >
                          <t-icon name="loading" class="card-analyze-loading"></t-icon>
                          <span class="card-analyze-txt">{{ stageProgressLabel(item) }}</span>
                        </div>
                        <div
                          v-else-if="item.parse_status === 'failed'"
                          class="card-analyze failure"
                        >
                          <t-icon name="close-circle" class="card-analyze-loading failure"></t-icon>
                          <span class="card-analyze-txt failure">{{ stageProgressLabel(item) }}</span>
                        </div>
                        <div v-else-if="item.parse_status === 'draft'" class="card-draft">
                          <t-tag size="small" theme="warning" variant="light-outline">{{ t('knowledgeBase.draft')
                          }}</t-tag>
                          <span class="card-draft-tip">{{ t('knowledgeBase.draftTip') }}</span>
                        </div>
                        <div
                          v-else-if="item.parse_status === 'completed' && (item.summary_status === 'pending' || item.summary_status === 'processing')"
                          class="card-analyze">
                          <t-icon name="loading" class="card-analyze-loading"></t-icon>
                          <span class="card-analyze-txt">5/5</span>
                        </div>
                        <div v-else-if="item.parse_status === 'completed'" class="card-complete-block">
                          <span class="card-progress-complete"><t-icon name="check-circle" />5/5</span>
                          <div class="card-content-txt">{{ item.description }}</div>
                        </div>
                      </div>
                      <div class="card-bottom">
                        <span class="card-time">{{ formatDocTime(item.updated_at) }}</span>
                        <div class="card-bottom-right">
                          <div class="card-type">
                            <span>{{ getKnowledgeType(item) }}</span>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                  <!-- 悬停卡片时跟随鼠标的详情气泡 -->
                  <Teleport to="body">
                    <div v-show="hoveredCardItem" class="knowledge-card-hover-popover"
                      :style="{ left: cardPopoverPos.x + 'px', top: cardPopoverPos.y + 'px' }">
                      <template v-if="hoveredCardItem">
                        <div class="card-popover-title">{{ hoveredCardItem.file_name }}</div>
                        <div
                          v-if="isParseInFlight(hoveredCardItem.parse_status)"
                          class="card-popover-status parsing">
                          {{ stageProgressLabel(hoveredCardItem) }}
                        </div>
                        <div v-else-if="hoveredCardItem.parse_status === 'failed'" class="card-popover-status failure">
                          {{ stageProgressLabel(hoveredCardItem) }}
                        </div>
                        <div v-else-if="hoveredCardItem.parse_status === 'draft'" class="card-popover-status draft">
                          {{ t('knowledgeBase.draft') }}
                        </div>
                        <template v-else>
                          <div v-if="hoveredCardItem.description" class="card-popover-desc">{{
                            hoveredCardItem.description }}</div>
                          <div v-if="(hoveredCardItem as any).source" class="card-popover-source"
                            :title="(hoveredCardItem as any).source">
                            <t-icon name="link" size="12px" /> {{ (hoveredCardItem as any).source }}
                          </div>
                          <div class="card-popover-extra">
                            <span v-if="(hoveredCardItem as any).created_at" class="card-popover-created">
                              {{ t('knowledgeBase.createdAt') }}：{{ formatDocTime((hoveredCardItem as any).created_at)
                              }}
                            </span>
                            <span v-if="formatFileSize((hoveredCardItem as any).file_size)" class="card-popover-size">
                              {{ formatFileSize((hoveredCardItem as any).file_size) }}
                            </span>
                          </div>
                        </template>
                        <div class="card-popover-meta">
                          <span class="card-popover-time">{{ t('knowledgeBase.updatedAt') }}：{{
                            formatDocTime(hoveredCardItem.updated_at)
                          }}</span>
                          <span class="card-popover-type">{{ getKnowledgeType(hoveredCardItem) }}</span>
                        </div>
                        <div class="card-popover-hint">{{ t('knowledgeBase.clickToViewFull') }}</div>
                      </template>
                    </div>
                  </Teleport>
                </template>
                <template v-else-if="cardList.length && viewMode === 'list'">
                  <DocumentListView :items="cardList" :selected-ids="selectedIds"
                    :progress-by-id="stageProgressById"
                    :can-edit="canEdit" :can-retry-own="canRetryOwnDocument"
                    @open="(item: any) => openKnowledgeItem(item)" @toggle-row="toggleSelectRow"
                    @toggle-all="toggleSelectAll"
                    @action="(action: any, item: any) => handleListAction(action, item)" />
                </template>
                <template v-else-if="docListError">
                  <div class="doc-empty-state">
                    <div class="doc-load-error">
                      <t-icon name="error-circle" size="28px" />
                      <p>{{ docListError }}</p>
                      <t-button variant="outline" @click="loadKnowledgeFiles(kbId)">重新加载</t-button>
                    </div>
                  </div>
                </template>
                <template v-else-if="!docListLoading">
                  <div
                    class="doc-empty-state"
                    :class="{ 'is-drag-active': emptyDropActive }"
                    @dragenter.prevent="handleEmptyDragEnter"
                    @dragover.prevent="handleEmptyDragOver"
                    @dragleave.prevent="handleEmptyDragLeave"
                    @drop.prevent="handleEmptyDrop"
                  >
					<EmptyKnowledge v-if="canWrite" :max-file-size-mb="kbInfo?.max_file_size_mb || 50" />
					<div v-else class="doc-load-error"><p>该共享知识库暂时没有文档</p></div>
                  </div>
                </template>
              </div>
              <div class="doc-batch-bar-anchor" v-show="batchMode || selectedIds.size > 0">
                <DocumentBatchBar :count="selectedIds.size" :loading="batchDeleting"
                  :visible="batchMode || selectedIds.size > 0" @cancel="handleBatchCancel"
                  @delete="confirmBatchDelete" />
              </div>
            </div>
          </div>
      </div>

      <!-- DocContent drawer (shared by documents tab and wiki source refs) -->
      <DocContent :visible="isCardDetails" :details="details" :canEditKB="canManage"
        @closeDoc="closeDoc" @getDoc="getDoc">
      </DocContent>

      <t-drawer v-model:visible="progressDrawerVisible" :header="false" :footer="false" :close-btn="false"
        placement="right" size="920px" destroy-on-close class="zeal-progress-drawer">
        <KnowledgeProcessingProgress v-if="progressItem?.id" :knowledge-id="progressItem.id"
          :parse-status="progressItem.parse_status" :doc-title="progressItem.file_name || progressItem.title || ''"
          show-close @close="progressDrawerVisible = false" />
      </t-drawer>
    </div>
  </template>
  <template v-else>
    <div class="faq-manager-wrapper">
	  <t-button class="faq-share-button" variant="outline" @click="sharingVisible = true">
		<template #icon><t-icon name="usergroup" /></template>共享成员
	  </t-button>
      <FAQEntryManager v-if="kbId" :kb-id="kbId" />
    </div>
  </template>

  <!-- 知识库编辑器（创建/编辑统一组件） -->
  <KnowledgeBaseEditorModal :visible="uiStore.showKBEditorModal" :mode="uiStore.kbEditorMode"
    :kb-id="uiStore.currentKBId || undefined" :initial-type="uiStore.kbEditorType"
    @update:visible="(val) => val ? null : uiStore.closeKBEditor()" @success="handleKBEditorSuccess" />

	<KnowledgeBaseSharingDialog v-model:visible="sharingVisible" :kb-id="kbId" :access-role="kbInfo?.access_role"
	  @changed="loadKnowledgeBaseInfo(kbId, true)" @left="router.push('/platform/knowledge-bases')" />

</template>
<style>
/* 下拉菜单容器样式已统一至 @/assets/dropdown-menu.less */
.zeal-progress-drawer .t-drawer__body {
  height: 100%;
  padding: 0;
  overflow: hidden;
}

.zeal-progress-drawer .t-drawer {
  max-width: calc(100vw - 16px);
}
</style>
<style scoped lang="less">
.knowledge-layout {
  display: flex;
  flex-direction: column;
  margin: 0 16px 0 4px;
  gap: 20px;
  height: 100%;
  flex: 1;
  width: 100%;
  min-width: 0;
  padding: 24px 32px 0px;
  box-sizing: border-box;
}

// Breadcrumb tab switch (文档/Wiki in breadcrumb)
.breadcrumb-tab {
  cursor: pointer;
  color: var(--td-text-color-placeholder);
  font-weight: 400;
  transition: color 0.15s;
  display: inline-flex;
  align-items: center;
  gap: 4px;

  &:hover {
    color: var(--td-text-color-primary);
  }

  &.active {
    color: var(--td-brand-color);
    font-weight: 600;
  }

  &.indexing {
    color: var(--td-brand-color);
  }
}

.breadcrumb-tab-indicator {
  display: inline-flex;
  align-items: center;
  color: var(--td-brand-color);
  font-size: 12px;
  line-height: 1;
}

.breadcrumb-tab-sep {
  margin: 0 6px;
  color: var(--td-text-color-disabled);
  font-weight: 400;
}

.wiki-main-area {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.knowledge-main {
  display: flex;
  flex: 1;
  min-height: 0;
  background: transparent;
  border: none;
}

.document-content {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 0;
  border: none;
  overflow: hidden;
  background: transparent;
}

.doc-card-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  position: relative;
  /* 作为批量工具栏悬浮的定位上下文 */
}

.doc-filter-bar {
  padding: 0 0 12px 0;
  flex-shrink: 0;
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;

  .doc-search-input {
    flex: 1 1 220px;
    min-width: 220px;
  }

  .doc-type-select {
    width: 140px;
    flex-shrink: 0;
  }

  .doc-date-range {
    width: 280px;
    flex-shrink: 0;

    // TDesign focuses both the outer popup reference and inner inputs, which
    // visually stacks into a "double border" — drop the inner shadow.
    :deep(.t-input--focused),
    :deep(.t-is-focused) {
      box-shadow: none;
    }
  }

  .doc-view-toggle {
    flex-shrink: 0;
    display: inline-flex;
    align-items: center;
    padding: 2px;
    background: var(--td-bg-color-secondarycontainer);
    border-radius: 6px;
    gap: 0;

    .doc-view-toggle-btn {
      width: 28px;
      height: 24px;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      border: 0;
      background: transparent;
      border-radius: 4px;
      color: var(--td-text-color-secondary, #888);
      cursor: pointer;
      transition: background-color 0.12s ease, color 0.12s ease;

      &:hover {
        color: var(--td-text-color-primary, #232323);
      }

      &.active {
        background: var(--td-bg-color-container, #fff);
        color: var(--td-brand-color, #0052d9);
        box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
      }
    }
  }

  .doc-filter-actions {
    flex-shrink: 0;

    :deep(.content-bar-icon-btn) {
      color: var(--td-text-color-secondary);
      background: transparent;
      border: none;

      &:hover {
        color: var(--td-brand-color);
        background: var(--td-bg-color-secondarycontainer);
      }
    }
  }

  :deep(.t-input) {
    font-size: 13px;
    background-color: var(--td-bg-color-secondarycontainer);
    border-color: transparent;
    border-radius: 6px;
    box-shadow: none !important;

    &:hover,
    &:focus,
    &.t-is-focused {
      border-color: var(--td-brand-color);
      background-color: var(--td-bg-color-container);
      box-shadow: none !important;
    }
  }

  :deep(.t-select) {
    .t-input {
      font-size: 13px;
      background-color: var(--td-bg-color-secondarycontainer);
      border-color: transparent;
      border-radius: 6px;
      box-shadow: none !important;

      &:hover,
      &.t-is-focused {
        border-color: var(--td-brand-color);
        background-color: var(--td-bg-color-container);
        box-shadow: none !important;
      }
    }
  }
}

.doc-scroll-container {
  position: relative;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 4px;

  &.is-empty {
    display: flex;
    align-items: center;
    justify-content: center;
    overflow-y: hidden;
  }

  &.is-marquee-active {
    cursor: crosshair;
  }
}

.doc-marquee-box {
  position: absolute;
  z-index: 4;
  pointer-events: none;
  border: 1px solid var(--td-brand-color);
  background: color-mix(in srgb, var(--td-brand-color) 12%, transparent);
  border-radius: 2px;

  &.is-add {
    border-color: var(--td-brand-color);
    background: color-mix(in srgb, var(--td-brand-color) 14%, transparent);
  }

  &.is-subtract {
    border-color: var(--td-error-color-6);
    background: color-mix(in srgb, var(--td-error-color-6) 12%, transparent);
  }
}

/* 批量条悬浮在滚动区底部，不挤占列表高度 */
.doc-batch-bar-anchor {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 12px;
  z-index: 6;
  display: flex;
  justify-content: center;
  padding: 0 16px;
  pointer-events: none;

  &>* {
    pointer-events: auto;
  }
}

// Header 样式（无底部分割线，留更多空间给下方内容区）
.document-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  flex-shrink: 0;

  .document-header-title {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .document-title-row {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }

  .kb-title-actions {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
    margin-left: 4px;
  }

  .document-breadcrumb {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 0;
    font-size: 20px;
    font-weight: 600;
    color: var(--td-text-color-primary);
  }

  .breadcrumb-link {
    border: none;
    background: transparent;
    padding: 4px 8px;
    margin: -4px -8px;
    font: inherit;
    color: var(--td-text-color-secondary);
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    border-radius: 6px;
    transition: all 0.12s ease;

    &:hover:not(:disabled) {
      color: var(--td-success-color);
      background: var(--td-bg-color-container);
    }

    &:disabled {
      cursor: not-allowed;
      color: var(--td-text-color-placeholder);
    }

    &.dropdown {
      padding-right: 6px;

      :deep(.t-icon) {
        font-size: 14px;
        transition: transform 0.12s ease;
      }

      &:hover:not(:disabled) {
        :deep(.t-icon) {
          transform: translateY(1px);
        }
      }
    }
  }

  .breadcrumb-separator {
    font-size: 14px;
    color: var(--td-text-color-placeholder);
  }

  .breadcrumb-current {
    color: var(--td-text-color-primary);
    font-weight: 600;
  }

  h2 {
    margin: 0;
    color: var(--td-text-color-primary);
    font-family: var(--app-font-family);
    font-size: 24px;
    font-weight: 600;
    line-height: 32px;
  }

  .document-subtitle {
    margin: 0;
    color: var(--td-text-color-placeholder);
    font-family: var(--app-font-family);
    font-size: 14px;
    font-weight: 400;
    line-height: 20px;
  }

  .parser-hint {
    display: flex;
    align-items: center;
    gap: 4px;
    margin: 2px 0 0;
    color: var(--td-warning-color);
    font-size: 12px;
    line-height: 1.4;
    cursor: pointer;
    transition: color 0.15s ease;

    &:hover {
      color: var(--td-warning-color-active);

      .parser-hint-link {
        text-decoration: underline;
      }
    }

    .parser-hint-icon {
      font-size: 12px;
      flex-shrink: 0;
    }

    .parser-hint-link {
      color: var(--td-brand-color);
      margin-left: 2px;
      white-space: nowrap;
    }
  }

}


.document-upload-input {
  display: none;
}

.kb-settings-button {
  width: 30px;
  height: 30px;
  border: none;
  border-radius: 50%;
  background: var(--td-bg-color-secondarycontainer);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--td-text-color-secondary);
  cursor: pointer;
  transition: all 0.2s ease;
  padding: 0;

  &:hover:not(:disabled) {
    background: var(--td-success-color-light);
    color: var(--td-brand-color);
    box-shadow: none;
  }

  &:disabled {
    cursor: not-allowed;
    opacity: 0.4;
  }

  :deep(.t-icon) {
    font-size: 18px;
  }

  &.is-spinning :deep(.t-icon) {
    animation: full-rebuild-spin 1s linear infinite;
  }
}

@keyframes full-rebuild-spin {
  to { transform: rotate(360deg); }
}

.full-rebuild-status {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  margin: 8px 0 0;
  max-width: 760px;
  font-size: 13px;
  line-height: 20px;

  .status-icon {
    flex: 0 0 auto;
    margin-top: 2px;
  }

  &.is-running { color: var(--td-brand-color); }
  &.is-failed { color: var(--td-error-color); }
}

.card-bottom-right {
  display: flex;
  align-items: center;
  gap: 6px;
}

.faq-manager-wrapper {
	position: relative;
  flex: 1;
  min-height: 0;
  padding: 24px 32px;
  overflow-y: auto;
  margin: 0 16px 0 4px;
}

.faq-share-button {
	position: absolute;
	top: 28px;
	right: 36px;
	z-index: 4;
}

@media (max-width: 1250px) and (min-width: 1045px) {
  .answers-input {
    transform: translateX(-329px);
  }

  :deep(.t-textarea__inner) {
    width: 654px !important;
  }
}

@media (max-width: 1045px) {
  .answers-input {
    transform: translateX(-250px);
  }

  :deep(.t-textarea__inner) {
    width: 500px !important;
  }
}

@media (max-width: 750px) {
  .answers-input {
    transform: translateX(-182px);
  }

  :deep(.t-textarea__inner) {
    width: 340px !important;
  }
}

@media (max-width: 600px) {
  .answers-input {
    transform: translateX(-164px);
  }

  :deep(.t-textarea__inner) {
    width: 300px !important;
  }
}

@keyframes contentFadeIn {
  from {
    opacity: 0;
    transform: translateY(6px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.doc-card-list {
  box-sizing: border-box;
  display: grid;
  // 文档卡片信息量较大（标题 + 摘要 + 标签/类型），保持稍宽的最小列宽，避免一行塞太多导致内容拥挤。
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 12px;
  align-content: flex-start;
  width: 100%;

  &.doc-card-list-animated {
    animation: contentFadeIn 0.32s ease-out;
  }
}

.knowledge-card-skeleton {
  cursor: default;

  .card-content {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    padding: 10px 14px 8px;
  }

  .card-content-nav {
    margin-bottom: 8px;
  }

  .card-bottom {
    flex-shrink: 0;
    margin-top: auto;
    width: 100%;
    padding: 0 14px;
    box-sizing: border-box;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-top: 1px solid var(--td-component-stroke);
  }
}

.doc-empty-state {
  flex: 1;
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
  min-height: 100%;
  border: 1px dashed transparent;
  transition: border-color 0.16s ease, background-color 0.16s ease;

  &.is-drag-active {
    border-color: var(--td-brand-color);
    background: var(--td-brand-color-light);
  }
}

.doc-load-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  color: var(--td-error-color);

  p {
    margin: 0;
    color: var(--td-text-color-secondary);
  }
}


.card-menu {
  display: flex;
  flex-direction: column;
  min-width: 140px;
  gap: 1px;
}

.card-menu-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  cursor: pointer;
  color: var(--td-text-color-primary);
  transition: all 0.15s cubic-bezier(0.2, 0, 0, 1);
  border-radius: 6px;
  font-size: 14px;
  line-height: 20px;

  &:hover {
    background: var(--td-bg-color-container-hover);
  }

  &:active {
    background: var(--td-bg-color-container-active);
    transform: scale(0.98);
  }

  .icon {
    font-size: 16px;
    color: var(--td-text-color-secondary);
    transition: all 0.15s cubic-bezier(0.2, 0, 0, 1);
  }

  &:hover .icon {
    color: var(--td-text-color-primary);
  }

  &.danger {
    color: var(--td-error-color-6);
    margin-top: 4px;
    position: relative;

    &::before {
      content: '';
      position: absolute;
      top: -3px;
      left: 8px;
      right: 8px;
      height: 1px;
      background: var(--td-component-stroke);
    }

    .icon {
      color: var(--td-error-color-6);
    }

    &:hover {
      background: var(--td-error-color-1);
      color: var(--td-error-color-6);

      .icon {
        color: var(--td-error-color-6);
      }
    }

    &:active {
      background: var(--td-error-color-2);
    }
  }
}

.move-menu {
  min-width: 220px;
  max-width: 280px;
  max-height: 360px;
  overflow-y: auto;

  .move-menu-header {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 12px;
    font-size: 13px;
    font-weight: 500;
    color: var(--td-text-color-primary);
    border-bottom: 1px solid var(--td-component-stroke);
    cursor: pointer;

    &:hover {
      background: var(--td-bg-color-container-hover);
    }
  }

  .move-menu-loading {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 20px 0;
  }

  .move-menu-empty {
    padding: 12px 16px;
    font-size: 12px;
    color: var(--td-text-color-placeholder);
    text-align: center;
    line-height: 1.5;
  }

  .move-target-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .move-target-count {
    font-size: 12px;
    color: var(--td-text-color-placeholder);
  }

  .move-confirm-body {
    padding: 8px;

    .move-target-info {
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 6px 8px;
      background: var(--td-bg-color-container-hover);
      border-radius: 6px;
      font-size: 13px;
      color: var(--td-text-color-secondary);
      margin-bottom: 8px;
    }

    .move-mode-item {
      display: flex;
      align-items: flex-start;
      gap: 6px;
      padding: 6px 8px;
      border-radius: 6px;
      cursor: pointer;
      margin-bottom: 4px;

      &:hover {
        background: var(--td-bg-color-container-hover);
      }

      &.active {
        background: var(--td-brand-color-light);
      }

      .move-mode-text {
        display: flex;
        flex-direction: column;
        gap: 2px;

        .move-mode-label {
          font-size: 13px;
          font-weight: 500;
          color: var(--td-text-color-primary);
        }

        .move-mode-desc {
          font-size: 11px;
          color: var(--td-text-color-placeholder);
          line-height: 1.4;
        }
      }
    }

    .move-confirm-actions {
      display: flex;
      justify-content: flex-end;
      gap: 8px;
      margin-top: 8px;
    }
  }
}

.card-draft {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  flex-shrink: 0;
}

.card-draft-tip {
  color: var(--td-warning-color);
  font-size: 11px;
}

.knowledge-card {
  min-width: 240px;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--td-component-border);
  height: 136px;
  border-radius: 8px;
  overflow: hidden;
  box-sizing: border-box;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
  background: var(--td-bg-color-container);
  position: relative;
  cursor: pointer;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;

  /* 仅在批量管理模式下渲染 checkbox，常态下不占位，避免标题在 hover 时右滑 */
  .card-nav-check {
    flex-shrink: 0;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 29px;
    margin-right: 8px;
    cursor: pointer;

    .card-select-checkbox {
      margin: 0;
      line-height: 0;

      :deep(.t-checkbox) {
        align-items: center;
      }

      :deep(.t-checkbox__label) {
        display: none !important;
        width: 0 !important;
        min-width: 0 !important;
        margin: 0 !important;
        padding: 0 !important;
      }

      :deep(.t-checkbox__input) {
        margin: 0;
      }

      :deep(.t-checkbox__input-wrapper) {
        margin: 0;
      }
    }
  }

  .card-content {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    padding: 10px 14px 8px;
  }

  .card-analyze {
    flex-shrink: 0;
    height: 30px;
    display: flex;
    align-items: center;
  }

  .card-analyze-loading {
    display: block;
    color: var(--td-brand-color);
    font-size: 14px;
  }

  .card-analyze-txt {
    color: var(--td-brand-color);
    font-family: var(--app-font-family-mono);
    font-size: 12px;
    font-weight: 700;
    margin-left: 8px;
  }

  .failure {
    color: var(--td-error-color);
  }

  .card-content-nav {
    flex-shrink: 0;
    display: flex;
    align-items: flex-start;
    gap: 0;
    margin-bottom: 6px;
  }

  .card-content-title {
    flex: 1;
    min-width: 0;
    height: 24px;
    line-height: 24px;
    display: inline-block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--td-text-color-primary);
    font-family: var(--app-font-family);
    font-size: 14px;
    font-weight: 600;
    letter-spacing: 0.01em;
    margin-right: 8px;
  }

  .more-wrap {
    flex-shrink: 0;
    display: flex;
    width: 25px;
    height: 25px;
    justify-content: center;
    align-items: center;
    border-radius: 5px;
    cursor: pointer;
  }

  .more-wrap:hover {
    background: var(--td-component-stroke);
  }

  .more-icon {
    width: 14px;
    height: 14px;
  }

  .active-more {
    background: var(--td-component-stroke);
  }

  .card-content-txt {
    flex: 1;
    min-height: 0;
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    overflow: hidden;
    color: var(--td-text-color-secondary);
    font-family: var(--app-font-family);
    font-size: 12px;
    font-weight: 400;
    line-height: 19px;
  }

  .card-complete-block {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 5px;
  }

  .card-progress-complete {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    color: #1769d2;
    font-family: var(--app-font-family-mono);
    font-size: 12px;
    font-weight: 700;
    line-height: 18px;
  }

  .card-bottom {
    flex-shrink: 0;
    margin-top: auto;
    padding: 0 14px;
    box-sizing: border-box;
    height: 32px;
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    background: var(--td-bg-color-container);
    border-top: 1px solid var(--td-component-stroke);
  }

  .card-time {
    color: var(--td-text-color-secondary);
    font-family: var(--app-font-family);
    font-size: 12px;
    font-weight: 400;
  }

  .card-type {
    color: var(--td-text-color-placeholder);
    font-family: var(--app-font-family);
    font-size: 11px;
    font-weight: 500;
    padding: 0;
    background: transparent;
    letter-spacing: 0.02em;
  }
}

.knowledge-card:hover {
  border-color: color-mix(in srgb, var(--td-component-stroke) 55%, var(--td-brand-color));
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.07);
}

/* 悬停知识卡片时跟随鼠标的详情气泡 */
.knowledge-card-hover-popover {
  position: fixed;
  z-index: 9999;
  pointer-events: none;
  min-width: 220px;
  max-width: 360px;
  padding: 12px 14px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.12);
  font-family: var(--app-font-family);
  transition: opacity 0.15s ease;

  .card-popover-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--td-text-color-primary);
    margin-bottom: 8px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .card-popover-status {
    font-size: 12px;
    margin-bottom: 6px;
    display: flex;
    align-items: center;
    gap: 6px;

    &.parsing {
      color: var(--td-brand-color);
    }

    &.failure {
      color: var(--td-error-color);
    }

    &.draft {
      color: var(--td-warning-color);
    }
  }

  .card-popover-desc {
    font-size: 12px;
    color: var(--td-text-color-secondary);
    line-height: 1.5;
    margin-bottom: 8px;
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 5;
    line-clamp: 5;
    overflow: hidden;
  }

  .card-popover-error-msg {
    display: block;
    margin-top: 4px;
    font-size: 11px;
    color: var(--td-error-color);
    opacity: 0.95;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 280px;
  }

  .card-popover-source {
    font-size: 11px;
    color: var(--td-brand-color);
    margin-bottom: 6px;
    display: flex;
    align-items: center;
    gap: 4px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 100%;
  }

  .card-popover-extra {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 10px;
    font-size: 11px;
    color: var(--td-text-color-secondary);
    margin-bottom: 6px;
  }

  .card-popover-created,
  .card-popover-size {
    flex-shrink: 0;
  }

  .card-popover-meta {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    font-size: 11px;
    color: var(--td-text-color-secondary);
  }

  .card-popover-type {
    padding: 1px 6px;
    background: var(--td-bg-color-secondarycontainer);
    color: var(--td-text-color-secondary);
    border-radius: 4px;
  }

  .card-popover-hint {
    margin-top: 8px;
    padding-top: 8px;
    border-top: 1px solid var(--td-component-stroke);
    font-size: 11px;
    color: var(--td-text-color-secondary);
  }
}

.knowledge-card-upload {
  color: var(--td-text-color-primary);
  font-family: var(--app-font-family);
  font-size: 14px;
  font-weight: 400;
  cursor: pointer;

  .btn-upload {
    margin: 33px auto 0;
    width: 112px;
    height: 32px;
    border: 1px solid var(--td-component-border);
    display: flex;
    justify-content: center;
    align-items: center;
    margin-bottom: 24px;
  }

  .svg-icon-download {
    margin-right: 8px;
  }
}

.upload-described {
  color: var(--td-text-color-disabled);
  font-family: var(--app-font-family);
  font-size: 12px;
  font-weight: 400;
  text-align: center;
  display: block;
  width: 188px;
  margin: 0 auto;
}

.del-card {
  vertical-align: middle;
}

/* ZgentFlow knowledge operations layout */
.knowledge-layout {
  margin: 0;
  gap: 0;
  padding: 0;
  background: var(--zeal-canvas, #f3f6fa);
}

.document-header {
  padding: 28px 34px 0;
  border-bottom: 0;
  background: transparent;
}

.document-header .document-header-title {
  width: 100%;
  gap: 0;
}

.workspace-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
}

.workspace-eyebrow {
  color: var(--zeal-primary, #1769dc);
  font-size: 11px;
  font-weight: 750;
  text-transform: uppercase;
}

.workspace-heading h1 {
  margin: 3px 0 0;
  color: var(--zeal-ink, #18212f);
  font-size: 28px;
  line-height: 35px;
  font-weight: 750;
}

.workspace-indexes {
  display: flex;
  align-items: center;
  gap: 5px;
}

.workspace-indexes span {
  height: 28px;
  padding: 0 9px;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  border: 1px solid var(--zeal-line, #dbe3ed);
  border-radius: 6px;
  color: var(--zeal-muted, #778398);
  background: var(--zeal-surface, #fff);
  font-size: 10px;
}

.workspace-indexes i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #c3c9d1;
}

.workspace-indexes span.is-active {
  color: var(--zeal-primary, #1769dc);
  border-color: #bdd3f4;
  background: var(--zeal-primary-soft, #eaf2ff);
}

.workspace-indexes span.is-active i {
  background: var(--zeal-primary, #1769dc);
}

.document-header .document-title-row {
  width: 100%;
  min-height: 54px;
  margin-top: 20px;
  padding: 8px 12px 8px 16px;
  border: 1px solid var(--zeal-line, #dbe3ed);
  border-radius: 8px;
  background: var(--zeal-surface, #fff);
  box-shadow: var(--zeal-shadow-xs);
  flex-wrap: nowrap;
}

.document-header .document-breadcrumb {
  min-width: 0;
  font-size: 12px;
  line-height: 24px;
  font-weight: 600;
}

.document-header .breadcrumb-link {
  color: var(--zeal-muted, #778398);
}

.document-header .breadcrumb-current,
.breadcrumb-tab.active {
  color: var(--zeal-primary, #1769dc);
}

.document-header .kb-title-actions {
  margin-left: auto;
}

.document-header .document-subtitle {
  display: none;
}

.kb-settings-button {
  width: 34px;
  height: 34px;
  border-radius: 7px;
  border: 1px solid var(--zeal-line, #dbe3ed);
  background: var(--zeal-surface-subtle, #f8fafc);
}

.knowledge-main {
  gap: 14px;
  padding: 18px 34px 30px;
  box-sizing: border-box;
}

.document-content {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--zeal-line, #dbe3ed);
  border-radius: 8px;
  background: var(--zeal-surface, #fff);
  box-shadow: var(--zeal-shadow-xs);
}

.doc-card-area {
  background: var(--zeal-surface, #fff);
}

.doc-filter-bar {
  min-height: 66px;
  border-bottom: 1px solid var(--zeal-line, #dbe3ed);
  background: var(--zeal-surface-subtle, #f8fafc);
}

.wiki-main-area {
  margin: 18px 34px 30px;
  overflow: hidden;
  border: 1px solid var(--zeal-line, #dbe3ed);
  border-radius: 8px;
  background: var(--zeal-surface, #fff);
  box-shadow: var(--zeal-shadow-xs);
}

@media (max-width: 1100px) {
  .document-header { padding-inline: 24px; }
  .knowledge-main { padding-inline: 24px; }
  .wiki-main-area { margin-inline: 24px; }
}

@media (max-width: 860px) {
  .knowledge-main {
    flex-direction: column;
    overflow-y: auto;
  }
  .document-content { min-height: 520px; flex: 0 0 auto; }
}

@media (max-width: 760px) {
  .document-header { padding: 18px 16px 0; }
  .document-header,
  .document-header .document-header-title,
  .workspace-heading { width: 100%; max-width: 100%; box-sizing: border-box; }
  .workspace-heading { align-items: flex-start; gap: 10px; flex-wrap: wrap; }
  .workspace-heading h1 { font-size: 24px; line-height: 30px; }
  .workspace-indexes { align-self: center; margin-left: auto; max-width: 100%; flex-wrap: wrap; }
  .workspace-indexes span { padding-inline: 7px; }
  .document-header .document-title-row {
    min-height: 52px;
    margin-top: 16px;
    padding: 7px 8px 7px 11px;
    flex-wrap: wrap;
  }
  .document-header .document-breadcrumb {
    width: calc(100% - 80px);
    overflow: hidden;
    white-space: nowrap;
  }
  .document-header .kb-title-actions { margin-left: auto; }
  .knowledge-main { padding: 12px 16px 24px; }
  .wiki-main-area { margin: 12px 16px 24px; }
  .doc-filter-bar { min-height: 0; padding: 12px !important; flex-wrap: wrap; }
}
</style>
