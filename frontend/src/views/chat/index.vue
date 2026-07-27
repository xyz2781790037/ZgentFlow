<template>
    <div class="chat" :class="{ 'is-sidebar-collapsed': uiStore.sidebarCollapsed }">
        <header class="chat-context-header">
            <div class="chat-context-path">
                <span>问答</span>
                <t-icon name="chevron-right" size="13px" />
                <strong>当前会话</strong>
            </div>
            <div class="chat-pipeline-state">
                <i></i>
                {{ isAgentStreamSession() ? '深度问答' : '快速问答' }}
            </div>
        </header>
        <div ref="scrollContainer" class="chat_scroll_box" @scroll="handleScroll">
            <div class="msg_list">
                <!-- 消息列表骨架屏 -->
                <div v-if="historyLoading && messagesList.length === 0" class="msg-skeleton-list">
                    <div class="msg-skeleton msg-skeleton-user">
                        <t-skeleton animation="gradient" :row-col="[{ width: '45%', height: '36px', type: 'rect' }]" />
                    </div>
                    <div class="msg-skeleton msg-skeleton-bot">
                        <t-skeleton animation="gradient"
                            :row-col="[{ width: '80%', height: '16px' }, { width: '100%', height: '16px' }, { width: '60%', height: '16px' }]" />
                    </div>
                    <div class="msg-skeleton msg-skeleton-user">
                        <t-skeleton animation="gradient" :row-col="[{ width: '35%', height: '36px', type: 'rect' }]" />
                    </div>
                    <div class="msg-skeleton msg-skeleton-bot">
                        <t-skeleton animation="gradient"
                            :row-col="[{ width: '70%', height: '16px' }, { width: '90%', height: '16px' }]" />
                    </div>
                </div>
                <!--
                  关键：必须用 session.id 作为 key，不能用 v-for 的索引。
                  向上滚动加载历史时会插入一批消息（push/unshift）到列表，
                  若用索引作 key 会让所有已渲染消息的 key 漂移，触发整个列表的销毁重建
                  （botmsg / AgentStreamDisplay 全部重新挂载、markdown 重新渲染），
                  这是历史加载时白屏 + layout shift 蔓延到 session 列表的根因。
                  仅对极少数尚未拿到 id 的本地占位消息 fallback 到 role+created_at+index。
                -->
                <div v-for="(session, index) in messagesList"
                    :key="session.id || `${session.role}-${session.created_at}-${index}`" class="msg-item-wrapper">

                    <div v-if="session.role == 'user'">
                        <usermsg :content="session.content" :mentioned_items="session.mentioned_items"
                            :images="session.images" :attachments="session.attachments"
                            :editable="canEditUserMessage(session, index)" @edit="startEditingMessage(session, index)">
                        </usermsg>
                    </div>
                    <div v-if="session.role == 'assistant' && shouldRenderAssistantMessage(session)">
                        <botmsg :content="session.content" :session="session" :session-id="session_id"
                            :user-query="getUserQuery(index)" @scroll-bottom="scrollToBottom"
                            :isFirstEnter="isFirstEnter"></botmsg>
                    </div>
                </div>
                <div v-if="showGlobalTypingIndicator"
                    style="height: 41px;display: flex;align-items: center;padding-left: 4px;">
                    <div class="loading-typing">
                        <span></span>
                        <span></span>
                        <span></span>
                    </div>
                </div>
            </div>
        </div>
        <transition name="scroll-btn-fade">
            <div v-show="userHasScrolledUp" class="scroll-to-bottom-btn" @click="onClickScrollToBottom">
                <t-icon name="chevron-down" size="20px" />
            </div>
        </transition>
        <div class="input-container">
            <div v-if="editingMessage" class="editing-message-bar">
                <span><t-icon name="edit-1" size="14px" />正在修改上一条问题</span>
                <button type="button" title="取消编辑" aria-label="取消编辑" @click="cancelEditingMessage">
                    <t-icon name="close" size="14px" />
                </button>
            </div>
            <InputField ref="inputFieldRef"
                @send-msg="sendMsg"
                @stop-generation="handleStopGeneration" :isReplying="isReplying" :sessionId="session_id"
                :assistantMessageId="currentAssistantMessageId"></InputField>
        </div>
    </div>
    <KnowledgeBaseEditorModal :visible="uiStore.showKBEditorModal" :mode="uiStore.kbEditorMode"
        :kb-id="uiStore.currentKBId || undefined" :initial-type="uiStore.kbEditorType"
        @update:visible="(val) => val ? null : uiStore.closeKBEditor()" @success="handleKBEditorSuccess" />
</template>
<script setup>
import { storeToRefs } from 'pinia';
import { ref, onMounted, onBeforeMount, onUnmounted, nextTick, watch, reactive, computed } from 'vue';
import { useRoute, onBeforeRouteLeave, onBeforeRouteUpdate } from 'vue-router';
import InputField from '../../components/Input-field.vue';
import botmsg from './components/botmsg.vue';
import usermsg from './components/usermsg.vue';
import { getMessageList, getSession, delMessage } from "@/api/chat/index";
import { useStream } from '../../api/chat/streame'
import { useMenuStore } from '@/stores/menu';
import { useSettingsStore } from '@/stores/settings';
import { MessagePlugin } from 'tdesign-vue-next';
import { useI18n } from 'vue-i18n';
import { useUIStore } from '@/stores/ui';
import KnowledgeBaseEditorModal from '@/views/knowledge/KnowledgeBaseEditorModal.vue';
import { useKnowledgeBaseCreationNavigation } from '@/hooks/useKnowledgeBaseCreationNavigation';
import { useChatStreamHandler } from '@/composables/useChatStreamHandler';
import { useStickyBottomOnResize } from '@/composables/useStickyBottomOnResize';
import { clearCitationChunkCache } from '@/utils/citationChunkCache';

const usemenuStore = useMenuStore();
const useSettingsStoreInstance = useSettingsStore();

const isAgentStreamSession = () => false;

const uiStore = useUIStore();
const { navigateToKnowledgeBaseList } = useKnowledgeBaseCreationNavigation();
const { t } = useI18n();
const { firstQuery, firstMentionedItems, firstImageFiles, firstAttachmentFiles } = storeToRefs(usemenuStore);
const { onChunk, error, startStream, stopStream, lastStreamRequest } = useStream();
/** Snapshot of the in-flight HTTP request for attaching to the next assistant message. */
const pendingStreamDebug = ref(null);

const buildStreamDebugPayload = () => {
    const meta = lastStreamRequest.value;
    if (!meta) return null;
    return {
        requestId: meta.requestId,
        url: meta.url,
        method: meta.method,
        body: meta.body,
        sentAt: meta.sentAt,
        sessionId: session_id.value,
    };
};

const attachStreamDebugToMessage = (message) => {
    if (!message) return;
    const payload = pendingStreamDebug.value || buildStreamDebugPayload();
    if (!payload) return;
    if (payload.requestId && !message.request_id) {
        message.request_id = payload.requestId;
    }
    message.debugRequest = payload;
};
const route = useRoute();
const session_id = ref(route.params.chatid);

// 拉 session 详情，并按其 last_request_state 把输入栏状态恢复到当时的发起态。
const loadSessionAndHydrate = async (sid) => {
    if (!sid) return;
    try {
        const sessionRes = await getSession(sid);
        if (sessionRes?.data) {
            const lastState = sessionRes.data.last_request_state;
            if (lastState) {
                // 先把当前的"全局默认"快照下来，再用 session 状态覆盖；
                // 离开会话时会从快照还原，避免本会话的状态污染新建对话。
                useSettingsStoreInstance.snapshotAsDefaultsIfNeeded();
                useSettingsStoreInstance.applyLastRequestState(lastState);
            }
        }
    } catch (error) {
        console.error('Failed to load session data:', error);
    }
};
const inputFieldRef = ref();
const created_at = ref('');
const limit = ref(20);
const messagesList = reactive([]);
const isReplying = ref(false);
const editingMessage = ref(null);
const editingMessageBusy = ref(false);
const currentAssistantMessageId = ref(''); // 当前正在生成的 assistant message ID
const lastUserMessageIndex = computed(() => {
    for (let index = messagesList.length - 1; index >= 0; index -= 1) {
        if (messagesList[index]?.role === 'user') return index;
    }
    return -1;
});

const canEditUserMessage = (message, index) => {
    if (isReplying.value || editingMessageBusy.value) return false;
    if (index !== lastUserMessageIndex.value || message?.role !== 'user') return false;
    if (message.channel && message.channel !== 'web') return false;
    return !(message.images?.length || message.attachments?.length);
};

const startEditingMessage = (message, index) => {
    if (!canEditUserMessage(message, index)) return;
    editingMessage.value = {
        id: message.id,
        index,
        content: message.content,
        mentionedItems: message.mentioned_items || [],
    };
    inputFieldRef.value?.setDraft?.(message.content || '');
};

const cancelEditingMessage = () => {
    editingMessage.value = null;
    inputFieldRef.value?.clearDraft?.();
};
const scrollLock = ref(false);
const isFirstEnter = ref(true);
const loading = ref(false);
const historyLoading = ref(true);
const historyLoadingMore = ref(false);
const hasMoreHistory = ref(true);
let fullContent = ref('')
const scrollContainer = ref(null)
const userHasScrolledUp = ref(false)
const SCROLL_BOTTOM_THRESHOLD = 80

const isNearBottom = () => {
    if (!scrollContainer.value) return true;
    const { scrollTop, scrollHeight, clientHeight } = scrollContainer.value;
    return scrollHeight - scrollTop - clientHeight < SCROLL_BOTTOM_THRESHOLD;
}

const handleKBEditorSuccess = (kbId) => {
    navigateToKnowledgeBaseList(kbId)
}

function fileToBase64(file) {
    return new Promise((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result);
        reader.onerror = reject;
        reader.readAsDataURL(file);
    });
}

const getUserQuery = (index) => {
    if (index <= 0) {
        return '';
    }
    const previous = messagesList[index - 1];
    if (previous && previous.role === 'user') {
        return previous.content || '';
    }
    return '';
};

watch([() => route.params], async (newvalue) => {
    isFirstEnter.value = true;
    if (newvalue[0].chatid) {
        if (!firstQuery.value) {
            scrollLock.value = false;
        }
        messagesList.splice(0);
        session_id.value = newvalue[0].chatid;
        clearCitationChunkCache();

        // 切换会话时，重置状态
        historyLoading.value = true;
        historyLoadingMore.value = false;
        hasMoreHistory.value = true;
        created_at.value = '';
        loading.value = false;
        isReplying.value = false;
        currentAssistantMessageId.value = '';
        userHasScrolledUp.value = false;

        // 跨会话切换：先把旧会话覆盖前的全局默认还原，再让新会话重新拍快照
        // 并应用自己的 last_request_state（在 loadSessionAndHydrate 内部完成）。
        useSettingsStoreInstance.restoreDefaultsIfSnapshotted();

        await loadSessionAndHydrate(session_id.value);
        let data = {
            session_id: session_id.value,
            created_at: '',
            limit: limit.value
        }
        getmsgList(data);
    }
});
const scrollToBottom = (force = false) => {
    if (!force && userHasScrolledUp.value) return;
    nextTick(() => {
        if (scrollContainer.value) {
            scrollContainer.value.scrollTop = scrollContainer.value.scrollHeight;
        }
    })
}
const onClickScrollToBottom = () => {
    userHasScrolledUp.value = false;
    scrollToBottom(true);
}

// Images and other rich Markdown content can grow after the SSE chunk that
// introduced them. Follow those delayed height changes while the user remains
// at the live edge; preserve position when they intentionally scroll upward.
useStickyBottomOnResize(scrollContainer, userHasScrolledUp, scrollToBottom);

const debounce = (fn, delay) => {
    let timer
    return (...args) => {
        clearTimeout(timer)
        timer = setTimeout(() => fn(...args), delay)
    }
}
const onChatScrollTop = () => {
    if (scrollLock.value || historyLoadingMore.value || !hasMoreHistory.value) return;
    if (!scrollContainer.value) return;
    const { scrollTop, scrollHeight } = scrollContainer.value;
    isFirstEnter.value = false
    if (scrollTop <= 0) {
        let data = {
            session_id: session_id.value,
            created_at: created_at.value,
            limit: limit.value
        }
        getmsgList(data, true, scrollHeight);
    }
}
const debouncedScrollTop = debounce(onChatScrollTop, 500);
let lastScrollTop = 0;
const handleScroll = () => {
    const el = scrollContainer.value;
    if (el) {
        const currentTop = el.scrollTop;
        // Only an actual upward scroll detaches from the live edge. Content that
        // grows after a chunk (images, diagrams) keeps scrollTop fixed and would
        // otherwise fire a stale scroll event that falsely marks the user as
        // scrolled up, killing the auto-follow during streaming.
        if (currentTop < lastScrollTop - 1) {
            userHasScrolledUp.value = !isNearBottom();
        } else if (isNearBottom()) {
            userHasScrolledUp.value = false;
        }
        lastScrollTop = currentTop;
    }
    debouncedScrollTop();
};

const fetchMessageList = (data) => getMessageList(data);

const {
    findLastMessage,
    shouldRenderAssistantMessage,
    shouldShowGlobalTypingIndicator,
    handleMsgList,
    processStreamChunk,
    prepareForNewOutgoingMessage,
    markInFlightAssistantStopped,
} = useChatStreamHandler({
    messagesList,
    loading,
    isReplying,
    currentAssistantMessageId,
    fullContent,
    isAgentStreamSession,
    scrollToBottom,
    onError: (msg) => MessagePlugin.error(msg),
    preserveIncompleteStreamReactive: true,
    isFirstEnter,
    scrollContainer,
    debug: import.meta.env.DEV,
    onAfterMsgList: async () => {
        const lastMessage = messagesList[messagesList.length - 1];
        if (lastMessage && !lastMessage.is_completed) {
            isReplying.value = true;
            if (lastMessage.role === 'assistant') {
                currentAssistantMessageId.value = lastMessage.id;
                console.log('[Continue Stream] Set assistant message ID:', lastMessage.id);
            }
            await startStream({
                session_id: session_id.value,
                query: lastMessage.id,
                method: 'GET',
                url: '/api/v1/sessions/continue-stream',
            });
        }
    },
    onAgentQuery: (data, existingMessage) => {
        pendingStreamDebug.value = buildStreamDebugPayload();
        if (existingMessage) attachStreamDebugToMessage(existingMessage);
    },
    onMessageCreated: (message) => attachStreamDebugToMessage(message),
    onMessageUpdated: (message, payload) => {
        attachStreamDebugToMessage(message);
        if (payload?.is_completed) pendingStreamDebug.value = null;
    },
    onAgentAnswerDone: (message) => {
        attachStreamDebugToMessage(message);
        pendingStreamDebug.value = null;
    },
    onAgentChunkBound: (message) => {
        attachStreamDebugToMessage(message);
        pendingStreamDebug.value = null;
    },
});

const showGlobalTypingIndicator = computed(() =>
    shouldShowGlobalTypingIndicator(messagesList, loading.value),
);

const getmsgList = (data, isScrollType = false, scrollHeight) => {
    if (isScrollType) {
        if (historyLoadingMore.value || !hasMoreHistory.value) return;
        historyLoadingMore.value = true;
    }
    fetchMessageList(data).then(async (res) => {
        const batch = res?.data;
        if (!batch?.length) {
            if (isScrollType) {
                hasMoreHistory.value = false;
            }
            return;
        }
        const nextCursor = batch[0].created_at;
        if (isScrollType && created_at.value && nextCursor === created_at.value) {
            hasMoreHistory.value = false;
            return;
        }
        if (batch.length < limit.value) {
            hasMoreHistory.value = false;
        }
        created_at.value = nextCursor;
        await handleMsgList(batch, isScrollType, scrollHeight);
    }).catch((err) => {
        console.error('Failed to load messages:', err);
        if (isScrollType) {
            hasMoreHistory.value = false;
        }
    }).finally(() => {
        historyLoading.value = false;
        historyLoadingMore.value = false;
    })
}

// 发送消息
// 处理停止生成事件 - 立即清除 loading 状态
const handleStopGeneration = () => {
    console.log('[Stop Generation] Immediately clearing loading state');
    stopStream();
    loading.value = false;
    isReplying.value = false;
    // 标记当前 assistant 为已结束，避免下一条 query 复用该消息行
    markInFlightAssistantStopped(currentAssistantMessageId.value);
    // 保留 currentAssistantMessageId，Input-field 仍需用它调用 stop API
};

const sendMsg = async (value, mentionedItems = [], imageFiles = [], attachmentFiles = []) => {
    if (editingMessage.value) {
        const editTarget = editingMessage.value;
        if ((!mentionedItems || mentionedItems.length === 0) && editTarget.mentionedItems?.length) {
            mentionedItems = editTarget.mentionedItems;
        }
        editingMessageBusy.value = true;
        try {
            const tail = messagesList.slice(editTarget.index).filter((message) => message?.id);
            for (const message of [...tail].reverse()) {
                await delMessage(session_id.value, message.id);
            }
            messagesList.splice(editTarget.index);
            editingMessage.value = null;
        } catch (error) {
            console.error('[Chat] Failed to replace edited message:', error);
            MessagePlugin.error('修改问题失败，请稍后重试');
            inputFieldRef.value?.setDraft?.(value);
            return;
        } finally {
            editingMessageBusy.value = false;
        }
    }
    stopStream();
    prepareForNewOutgoingMessage();
    isReplying.value = true;
    loading.value = true;

    // Convert images to base64 data URIs for backend processing and local display
    let imageAttachments = [];
    let userImages = [];
    if (imageFiles && imageFiles.length > 0) {
        try {
            for (const file of imageFiles) {
                const dataURI = await fileToBase64(file);
                imageAttachments.push({ data: dataURI });
                userImages.push({ url: dataURI });
            }
        } catch (e) {
            console.error('[Image] Failed to read images:', e);
            loading.value = false;
            isReplying.value = false;
            return;
        }
    }

    // Convert attachment files to base64 for backend processing
    let attachmentUploads = [];
    if (attachmentFiles && attachmentFiles.length > 0) {
        try {
            for (const attachment of attachmentFiles) {
                const reader = new FileReader();
                const base64Promise = new Promise((resolve, reject) => {
                    reader.onload = () => {
                        const result = reader.result;
                        // Extract base64 content (remove data:...;base64, prefix)
                        const base64 = result.split(',')[1];
                        resolve(base64);
                    };
                    reader.onerror = reject;
                    reader.readAsDataURL(attachment.file);
                });
                const base64Data = await base64Promise;
                attachmentUploads.push({
                    data: base64Data,
                    file_name: attachment.name,
                    file_size: attachment.size
                });
            }
        } catch (e) {
            console.error('[Attachment] Failed to read attachments:', e);
            loading.value = false;
            isReplying.value = false;
            return;
        }
    }

    // 将@提及的知识库和文件信息存入用户消息
    messagesList.push({ content: value, role: 'user', mentioned_items: mentionedItems, images: userImages, attachments: attachmentFiles.map(a => ({ file_name: a.name, file_size: a.size, file_type: '.' + a.name.split('.').pop()?.toLowerCase() })) });
    userHasScrolledUp.value = false;
    scrollToBottom(true);

    // Get web search status from settings store
    const webSearchEnabled = useSettingsStoreInstance.isWebSearchEnabled;

    // Get knowledge_base_ids from settings store (selected by user via KnowledgeBaseSelector)
    // Merge @mentioned KB/file IDs so retrieval uses the same targets user @mentioned (including shared KBs)
    const sidebarKbIds = useSettingsStoreInstance.settings.selectedKnowledgeBases || [];
    const sidebarFileIds = useSettingsStoreInstance.settings.selectedFiles || [];
    const kbIdSet = new Set(sidebarKbIds);
    const fileIdSet = new Set(sidebarFileIds);
    for (const item of mentionedItems || []) {
        if (!item?.id) continue;
        if (item.type === 'kb' && !kbIdSet.has(item.id)) {
            kbIdSet.add(item.id);
        } else if (item.type === 'file' && !fileIdSet.has(item.id)) {
            fileIdSet.add(item.id);
        }
    }
    const kbIds = [...kbIdSet];
    const knowledgeIds = [...fileIdSet];

    await startStream({
        session_id: session_id.value,
        knowledge_base_ids: kbIds,
        knowledge_ids: knowledgeIds,
        web_search_enabled: webSearchEnabled,
        mentioned_items: mentionedItems,
        images: imageAttachments.length > 0 ? imageAttachments : undefined,
        attachment_uploads: attachmentUploads.length > 0 ? attachmentUploads : undefined,
        query: value,
        method: 'POST',
		url: '/api/v1/knowledge-chat',
    });
}

// Watch for stream errors and show message
watch(error, (newError) => {
    if (!newError) return;
    MessagePlugin.error(newError);
    isReplying.value = false;
    loading.value = false;
    // 清空当前 assistant message ID
    currentAssistantMessageId.value = '';
});

onChunk((data) => {
    if (data.response_type === 'session_title') {
        const title = data.content || data.data?.title;
        if (title && data.data?.session_id) {
            console.log('[Session Title Update]', {
                session_id: data.data.session_id,
                title: title,
            });
            usemenuStore.updatasessionTitle(data.data.session_id, title);
            usemenuStore.changeIsFirstSession(false);
            window.dispatchEvent(new CustomEvent('session-title-updated', {
                detail: { sessionId: data.data.session_id, title },
            }));
        }
        return;
    }
    processStreamChunk(data);
});

const handleSessionCleared = (e) => {
    if (e.detail?.sessionId === session_id.value) {
        messagesList.splice(0);
        created_at.value = '';
        hasMoreHistory.value = true;
        historyLoadingMore.value = false;
    }
};

onBeforeMount(async () => {
    // 必须在 Input-field onMounted 之前完成：按 session.last_request_state 恢复输入栏
    await loadSessionAndHydrate(session_id.value);
});

onMounted(async () => {
    window.addEventListener('session-messages-cleared', handleSessionCleared);
    messagesList.splice(0);

    // 初始化状态：加载历史消息时不应显示loading
    loading.value = false;
    isReplying.value = false;

    if (firstQuery.value) {
        scrollLock.value = true;
        historyLoading.value = false;
        sendMsg(firstQuery.value, firstMentionedItems.value || [], firstImageFiles.value || [], firstAttachmentFiles.value || []);
        usemenuStore.changeFirstQuery('', [], [], []);
    } else {
        scrollLock.value = false;
        hasMoreHistory.value = true;
        historyLoadingMore.value = false;
        let data = {
            session_id: session_id.value,
            created_at: '',
            limit: limit.value
        }
        getmsgList(data)
    }
})
const clearData = () => {
    stopStream();
    isReplying.value = false;
    editingMessage.value = null;
    editingMessageBusy.value = false;
    fullContent.value = '';
}
onUnmounted(() => {
    window.removeEventListener('session-messages-cleared', handleSessionCleared);
});
onBeforeRouteLeave((to, from, next) => {
    clearData()
    // 离开聊天会话 → 还原"用户全局默认"，避免旧会话的请求态泄漏到新建对话。
    useSettingsStoreInstance.restoreDefaultsIfSnapshotted();
    next()
})
onBeforeRouteUpdate((to, from, next) => {
    clearData()
    // 仅"会话 → 会话"会落到这里；跨会话覆盖的还原放到 route.params 的 watch 里，
    // 因为新会话的 getSession 也在那边触发，便于保证 restore→snapshot→apply 顺序。
    next()
})
</script>
<style lang="less" scoped>
.chat {
    font-size: 14px;
    padding: 0;
    box-sizing: border-box;
    flex: 1;
    min-height: 0;
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: stretch;
    width: 100%;
    max-width: none;
    min-width: 0;
    background: #f4f6f8;

    &.is-sidebar-collapsed {
        max-width: calc(100vw - 60px);
    }

    :deep(.answers-input) {
        position: static;
        transform: translateX(0);

        .t-textarea__inner {
            width: 100% !important;
        }
    }

}

.chat-context-header {
    height: 58px;
    flex: 0 0 58px;
    padding: 0 26px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-bottom: 1px solid #e0e5ea;
    background: rgba(255, 255, 255, 0.86);
    box-sizing: border-box;
}

.chat-context-path {
    display: flex;
    align-items: center;
    gap: 7px;
    color: #8a93a3;
    font-size: 11px;
}

.chat-context-path strong {
    color: #344054;
    font-weight: 650;
}

.chat-pipeline-state {
    height: 26px;
    padding: 0 9px;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    border: 1px solid #dce2e8;
    background: #fff;
    border-radius: 4px;
    color: #667085;
    font-size: 10px;
    font-weight: 650;
}

.chat-pipeline-state i {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: #18a66a;
}

.chat_scroll_box {
    flex: 1;
    // Without min-height: 0, a flex-column child defaults to min-height: auto
    // and expands to fit all inner content. When there are many messages,
    // that pushes .input-container out of the viewport. Clamping min-height
    // to 0 lets overflow-y: auto take effect so the messages scroll inside
    // this box instead of stretching it.
    min-height: 0;
    width: 100%;
    padding: 26px 28px 18px;
    box-sizing: border-box;
    overflow-y: auto;
    // 使用系统原生滚动条（macOS 滚动时自动显示 overlay 滚动条，类似 ChatGPT）
    scrollbar-width: auto;
    scrollbar-color: auto;
}

// 深色模式下 redesign.css 对 * 做了 webkit 滚动条着色，这里恢复为系统默认
:global(:root[theme-mode="dark"]) .chat_scroll_box {
    &::-webkit-scrollbar-thumb {
        background-color: initial !important;
    }

    &::-webkit-scrollbar-thumb:hover {
        background-color: initial !important;
    }

    &::-webkit-scrollbar-track {
        background-color: initial !important;
    }
}

.scroll-to-bottom-btn {
    position: absolute;
    left: 50%;
    transform: translateX(-50%);
    bottom: 140px;
    z-index: 10;
    width: 36px;
    height: 36px;
    border-radius: 50%;
    background: var(--td-bg-color-container);
    border: 1px solid var(--td-component-stroke);
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    color: var(--td-text-color-secondary);
    transition: all 0.2s ease;

    &:hover {
        background: var(--td-bg-color-container-hover);
        color: var(--td-text-color-primary);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    }

    &:active {
        transform: translateX(-50%) scale(0.92);
    }
}

.scroll-btn-fade-enter-active,
.scroll-btn-fade-leave-active {
    transition: opacity 0.2s ease, transform 0.2s ease;
}

.scroll-btn-fade-enter-from,
.scroll-btn-fade-leave-to {
    opacity: 0;
    transform: translateX(-50%) translateY(8px);
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

.msg-skeleton-list {
    display: flex;
    flex-direction: column;
    gap: 20px;
    max-width: 800px;
    padding: 16px 0;
    animation: contentFadeIn 0.3s ease-out;
}

.msg-skeleton-user {
    display: flex;
    justify-content: flex-end;
}

.msg-skeleton-bot {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding-left: 4px;
}

.input-container {
    min-height: 115px;
    // Keep the input visible when messages overflow: without flex-shrink: 0
    // a tall .chat_scroll_box can squeeze this container down to 0 height.
    flex-shrink: 0;
    margin: 0 auto;
    padding: 0 28px 22px;
    width: 100%;
    max-width: 896px;
    box-sizing: border-box;
}

.editing-message-bar {
    width: min(920px, calc(100% - 32px));
    min-height: 30px;
    margin: 0 auto 6px;
    padding: 0 8px 0 11px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    border: 1px solid #bdd5f5;
    border-radius: 6px;
    background: #f0f6ff;
    color: #1769dc;
    font-size: 11px;
}

.editing-message-bar span {
    display: inline-flex;
    align-items: center;
    gap: 6px;
}

.editing-message-bar button {
    width: 24px;
    height: 24px;
    padding: 0;
    border: 0;
    border-radius: 5px;
    background: transparent;
    color: #71809a;
    display: grid;
    place-items: center;
    cursor: pointer;
}

.editing-message-bar button:hover {
    color: #1769dc;
    background: #dceaff;
}

.msg_list {
    display: flex;
    flex-direction: column;
    gap: 16px;
    max-width: 840px;
    flex: 1;
    margin: 0 auto;
    width: 100%;

    /*
      给每条消息加 layout/style containment：
      - 一条消息的内部布局变化不再让浏览器去 invalidate 整个文档，
        这是修掉"hover 到 session 列表也变白"那个问题的关键。
      - 不要再用 content-visibility: auto / contain-intrinsic-size：
        agent 消息真实高度差异巨大（几百 ~ 数千 px），估的占位高度会让消息进入视口时
        反复发生"占位 -> 真实高度"的大幅 layout shift + 首次 paint 滞后，
        反而在向上滚动时制造"未画完"的白屏闪烁。
        当前 handleMsgList 全流程 ~50ms，根本无需跳过渲染，老老实实正常渲染最稳。
      - 不开 contain: paint：AgentStreamDisplay 里有 tooltip / popover 等会溢出的浮层，
        paint containment 会把它们裁掉。
    */
    .msg-item-wrapper {
        contain: layout style;
    }

    .botanswer_laoding_gif {
        width: 24px;
        height: 18px;
        margin-left: 16px;
    }

    .loading-typing {
        display: flex;
        align-items: center;
        gap: 4px;

        span {
            width: 6px;
            height: 6px;
            border-radius: 50%;
            background: var(--td-text-color-placeholder);
            animation: typingBounce 1.4s ease-in-out infinite;

            &:nth-child(1) {
                animation-delay: 0s;
            }

            &:nth-child(2) {
                animation-delay: 0.2s;
            }

            &:nth-child(3) {
                animation-delay: 0.4s;
            }
        }
    }
}

@keyframes typingBounce {

    0%,
    60%,
    100% {
        transform: translateY(0);
    }

    30% {
        transform: translateY(-8px);
    }
}

</style>
