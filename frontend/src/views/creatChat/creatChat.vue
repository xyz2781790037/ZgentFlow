<template>
    <div class="ask-workspace">
        <header class="ask-toolbar">
            <div class="ask-breadcrumb">
                <span>问答</span>
                <t-icon name="chevron-right" size="13px" />
                <strong>新对话</strong>
            </div>
            <div class="source-state">
                <i :class="{ ready: selectedKBCount > 0 }"></i>
                {{ scopeLabel }}
            </div>
        </header>

	        <main class="ask-main">
	            <section class="ask-prompt">
	                <div class="prompt-heading">
                    <div class="prompt-title-block">
                        <span class="prompt-kicker"><t-icon name="chat-bubble" size="14px" />新建问答</span>
                        <h1>想从知识中了解什么？</h1>
                    </div>
                    <div class="active-context">
                        <span><t-icon name="folder" size="15px" />已选择 {{ selectedKBCount }} 个知识库</span>
                        <span class="context-divider"></span>
                        <span><i class="mode-indicator"></i>{{ answerModeLabel }}</span>
                    </div>
                </div>

            </section>

            <div class="ask-composer">
                <InputField ref="inputFieldRef" @send-msg="sendMsg"></InputField>
                <div class="composer-meta">
                    <span>ZgentFlow</span>
                    <span>回答将在可用时附带来源引用</span>
                </div>
            </div>
        </main>
    </div>

    <KnowledgeBaseEditorModal :visible="uiStore.showKBEditorModal" :mode="uiStore.kbEditorMode"
        :kb-id="uiStore.currentKBId || undefined" :initial-type="uiStore.kbEditorType"
        @update:visible="(val) => val ? null : uiStore.closeKBEditor()" @success="handleKBEditorSuccess" />
</template>
<script setup lang="ts">
import { ref, computed } from 'vue';
import InputField from '@/components/Input-field.vue';
import { createSessions } from "@/api/chat/index";
import { useMenuStore } from '@/stores/menu';
import { useSettingsStore } from '@/stores/settings';
import { useUIStore } from '@/stores/ui';
import { useRouter } from 'vue-router';
import { MessagePlugin } from 'tdesign-vue-next';
import { useI18n } from 'vue-i18n';
import KnowledgeBaseEditorModal from '@/views/knowledge/KnowledgeBaseEditorModal.vue';
import { useKnowledgeBaseCreationNavigation } from '@/hooks/useKnowledgeBaseCreationNavigation';

const router = useRouter();
const usemenuStore = useMenuStore();
const settingsStore = useSettingsStore();
const uiStore = useUIStore();
const { t } = useI18n();
const { navigateToKnowledgeBaseList } = useKnowledgeBaseCreationNavigation();

const selectedKBCount = computed(() => settingsStore.settings.selectedKnowledgeBases?.length || 0);
const scopeLabel = computed(() => selectedKBCount.value > 0 ? '知识库已连接' : '请选择知识库');
const answerModeLabel = computed(() => '快速问答');

const inputFieldRef = ref();

const sendMsg = (value: string, mentionedItems: any[], imageFiles: any[] = [], attachmentFiles: any[] = []) => {
    createNewSession(value, mentionedItems, imageFiles, attachmentFiles);
}

async function createNewSession(value: string, mentionedItems: any[] = [], imageFiles: any[] = [], attachmentFiles: any[] = []) {
    try {
        const res = await createSessions();
        if (res.data && res.data.id) {
            await navigateToSession(res.data.id, value, mentionedItems, imageFiles, attachmentFiles);
        } else {
            console.error('[createChat] Failed to create session');
            MessagePlugin.error(t('createChat.messages.createFailed'));
        }
    } catch (error) {
        console.error('[createChat] Create session error:', error);
        MessagePlugin.error(t('createChat.messages.createError'));
    }
}

const navigateToSession = async (sessionId: string, value: string, mentionedItems: any[], imageFiles: any[] = [], attachmentFiles: any[] = []) => {
    const now = new Date().toISOString();
    let obj = {
        title: t('createChat.newSessionTitle'),
        path: `chat/${sessionId}`,
        id: sessionId,
        isMore: false,
        isNoTitle: true,
        created_at: now,
        updated_at: now
    };
    usemenuStore.updataMenuChildren(obj);
    usemenuStore.changeIsFirstSession(true);
    usemenuStore.changeFirstQuery(value, mentionedItems, imageFiles, attachmentFiles);
    router.push(`/platform/chat/${sessionId}`);
}

const handleKBEditorSuccess = (kbId: string) => {
    navigateToKnowledgeBaseList(kbId)
}

</script>
<style lang="less" scoped>
.ask-workspace {
    flex: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    background: var(--zeal-canvas, #f3f6fa);
    color: var(--zeal-ink, #18212f);
}

.ask-toolbar {
    height: 64px;
    flex: 0 0 64px;
    padding: 0 30px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-bottom: 1px solid var(--zeal-line, #dbe3ed);
    background: var(--zeal-surface, #fff);
    box-sizing: border-box;
}

.ask-breadcrumb {
    display: flex;
    align-items: center;
    gap: 7px;
    color: var(--zeal-muted, #778398);
    font-size: 12px;
}

.ask-breadcrumb strong {
    color: var(--zeal-ink, #18212f);
    font-weight: 680;
}

.source-state {
    height: 30px;
    padding: 0 10px;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    border: 1px solid var(--zeal-line, #dbe3ed);
    background: var(--zeal-surface-subtle, #f8fafc);
    border-radius: 6px;
    color: var(--zeal-ink-soft, #445166);
    font-size: 11px;
    font-weight: 650;
}

.source-state i,
.mode-indicator {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: #e0942d;
}

.source-state i.ready,
.mode-indicator {
    background: #18a66a;
}

.ask-main {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 52px 48px 30px;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    align-items: center;
}

.ask-prompt,
.ask-composer {
    width: min(920px, 100%);
}

.prompt-heading {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 28px;
}

.prompt-title-block {
    min-width: 0;
}

.prompt-kicker {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    color: var(--zeal-primary, #1769dc);
    font-size: 11px;
    font-weight: 720;
}

h1 {
    margin: 9px 0 0;
    color: var(--zeal-ink, #18212f);
    font-size: 34px;
    line-height: 42px;
    font-weight: 750;
}

.active-context {
    min-height: 38px;
    padding: 0 12px;
    display: flex;
    align-items: center;
    gap: 10px;
    border: 1px solid var(--zeal-line, #dbe3ed);
    border-radius: 7px;
    background: var(--zeal-surface, #fff);
    color: var(--zeal-ink-soft, #445166);
    font-size: 11px;
    box-shadow: var(--zeal-shadow-xs);
}

.active-context > span {
    display: inline-flex;
    align-items: center;
    gap: 6px;
}

.context-divider {
    width: 1px;
    height: 13px;
    background: var(--zeal-line, #dbe3ed);
}

.ask-composer {
    margin-top: auto;
    padding-top: 42px;
}

.ask-composer :deep(.answers-input) {
    position: static;
    left: auto;
    bottom: auto;
    transform: none;
    width: 100%;
    display: block;
}

.ask-composer :deep(.rich-input-container) {
    max-width: none;
    border-radius: 8px;
    border-color: var(--zeal-line-strong, #c6d1df);
    background: var(--zeal-surface, #fff);
    box-shadow: 0 14px 38px rgba(23, 34, 51, 0.1);
}

.ask-composer :deep(.t-textarea__inner) {
    min-height: 126px !important;
    border-radius: 8px;
}

.composer-meta {
    padding: 9px 3px 0;
    display: flex;
    justify-content: space-between;
    color: var(--zeal-muted, #778398);
    font-size: 10px;
}

@media (max-height: 760px) {
    .ask-main { padding-top: 32px; }
    .ask-composer { padding-top: 30px; }
}

@media (max-width: 920px) {
    .ask-main { padding-inline: 32px; }
    .prompt-heading { align-items: flex-start; flex-direction: column; gap: 16px; }
}

@media (max-width: 760px) {
    .ask-toolbar {
        height: 56px;
        flex-basis: 56px;
        padding: 0 18px;
    }
    .ask-main {
        align-items: stretch;
        padding: 28px 20px 18px;
    }
    h1 {
        font-size: 28px;
        line-height: 35px;
    }
    .active-context {
        width: 100%;
        min-height: 38px;
        overflow-x: auto;
        white-space: nowrap;
    }
    .ask-composer {
        margin-top: 28px;
        padding-top: 0;
    }
    .ask-composer :deep(.t-textarea__inner) { min-height: 104px !important; }
    .composer-meta { gap: 16px; }
    .composer-meta span:last-child { text-align: right; }
}
</style>
<style lang="less">
.del-menu-popup {
    z-index: 99 !important;

    .t-popup__content {
        width: 100px;
        height: 40px;
        line-height: 30px;
        padding-left: 14px;
        cursor: pointer;
        margin-top: 4px !important;

    }
}
</style>
