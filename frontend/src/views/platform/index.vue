<template>
    <div class="main zeal-app-shell" ref="dropzone">
        <ZealWorkspaceNav />
        <div class="workspace-frame">
            <ZealConversationPanel v-if="isAskWorkspace" />
            <div class="platform-route-outlet">
                <RouterView />
            </div>
        </div>
        <div class="upload-mask" v-show="ismask">
            <input type="file" style="display: none" ref="uploadInput" accept=".pdf,.docx,.doc,.pptx,.ppt,.epub,.mhtml,.txt,.md,.jpg,.jpeg,.png,.csv,.xls,.xlsx" />
            <UploadMask></UploadMask>
        </div>
        <!-- 全局设置模态框，供所有 platform 子路由使用 -->
        <Settings />
        <!-- 全局命令面板 (⌘K)，随 platform 路由存活 -->
        <GlobalCommandPalette />
    </div>
</template>
<script setup lang="ts">
import ZealWorkspaceNav from '@/components/ZealWorkspaceNav.vue'
import ZealConversationPanel from '@/components/ZealConversationPanel.vue'
import { ref, onMounted, onUnmounted, watch, computed } from 'vue';
import { useRoute, useRouter } from 'vue-router'
import useKnowledgeBase from '@/hooks/useKnowledgeBase'
import UploadMask from '@/components/upload-mask.vue'
import Settings from '@/views/settings/Settings.vue'
import GlobalCommandPalette from '@/components/GlobalCommandPalette.vue'
import { useCommandPaletteStore } from '@/stores/commandPalette'
import { useChatResourcesStore } from '@/stores/chatResources'
import { getKnowledgeBaseById } from '@/api/knowledge-base/index'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'

let { requestMethod } = useKnowledgeBase()
const route = useRoute();
const router = useRouter();
const commandPaletteStore = useCommandPaletteStore();
let ismask = ref(false)
let uploadInput = ref();
const { t } = useI18n();

const isAskWorkspace = computed(() => {
    const name = String(route.name || '')
    return name === 'globalCreatChat' || name === 'kbCreatChat' || name === 'chat'
})

// 用于跟踪拖拽进入/离开的计数器，解决子元素触发 dragleave 的问题
let dragCounter = 0;

// 获取当前知识库ID
const getCurrentKbId = (): string | null => {
    return (route.params as any)?.kbId as string || null
}

const CHAT_DROP_ROUTE_NAMES = new Set(['chat', 'globalCreatChat', 'kbCreatChat']);

const isChatDropRoute = () => {
    return CHAT_DROP_ROUTE_NAMES.has(String(route.name || ''));
}

const collectDroppedFiles = async (event: DragEvent): Promise<File[]> => {
    const dataTransferFiles = event.dataTransfer?.files ? Array.from(event.dataTransfer.files) : [];
    if (dataTransferFiles.length > 0) {
        return dataTransferFiles;
    }

    const dataTransferItems = event.dataTransfer?.items ? Array.from(event.dataTransfer.items) : [];
    if (dataTransferItems.length === 0) {
        return [];
    }

    const files = await Promise.all(dataTransferItems.map(item => new Promise<File | null>((resolve) => {
        const fileEntry = (item as any).webkitGetAsEntry?.();
        if (fileEntry?.isFile && typeof fileEntry.file === 'function') {
            fileEntry.file((file: File) => resolve(file), () => resolve(null));
            return;
        }
        resolve(null);
    })));

    return files.filter((file): file is File => file instanceof File);
}

// 检查知识库初始化状态
const checkKnowledgeBaseInitialization = async (): Promise<boolean> => {
    const currentKbId = getCurrentKbId();
    
    if (!currentKbId) {
        MessagePlugin.error(t('knowledgeBase.missingId'));
        return false;
    }
    
    try {
        const kbResponse = await getKnowledgeBaseById(currentKbId);
        const kb = kbResponse.data;
        
        if (!kb.summary_model_id) {
            MessagePlugin.warning(t('knowledgeBase.notInitialized'));
            return false;
        }
        const strategy = kb.indexing_strategy;
        const needsEmbedding = !strategy || strategy.vector_enabled || strategy.keyword_enabled;
        if (needsEmbedding && !kb.embedding_model_id) {
            MessagePlugin.warning(t('knowledgeBase.notInitialized'));
            return false;
        }
        return true;
    } catch (error) {
        MessagePlugin.error(t('knowledgeBase.getInfoFailed'));
        return false;
    }
}


// isFileDrag distinguishes an OS file drag (the only thing the global upload
// drop zone cares about) from an in-app element drag such as the wiki
// folder/page drag-and-drop. Element drags carry only "text/*" types, never
// "Files", so we bail out and let the originating component handle the drop.
const isFileDrag = (event: DragEvent): boolean => {
    const types = event.dataTransfer?.types
    if (!types) return false
    return Array.from(types).includes('Files')
}

// 全局拖拽事件处理
const handleGlobalDragEnter = (event: DragEvent) => {
    if (!isFileDrag(event)) return;
    event.preventDefault();
    dragCounter++;
    if (event.dataTransfer) {
        event.dataTransfer.effectAllowed = 'all';
    }
    ismask.value = true;
}

const handleGlobalDragOver = (event: DragEvent) => {
    if (!isFileDrag(event)) return;
    event.preventDefault();
    if (event.dataTransfer) {
        event.dataTransfer.dropEffect = 'copy';
    }
}

const handleGlobalDragLeave = (event: DragEvent) => {
    if (!isFileDrag(event)) return;
    event.preventDefault();
    dragCounter--;
    if (dragCounter === 0) {
        ismask.value = false;
    }
}

const handleGlobalDrop = async (event: DragEvent) => {
    if (!isFileDrag(event)) return;
    event.preventDefault();
    dragCounter = 0;
    ismask.value = false;

    const droppedFiles = await collectDroppedFiles(event);
    if (droppedFiles.length === 0) {
        MessagePlugin.warning(t('knowledgeBase.dragFileNotText'));
        return;
    }

    if (isChatDropRoute()) {
        event.stopPropagation();
        window.dispatchEvent(new CustomEvent('zealrag:chat-file-drop', {
            detail: { files: droppedFiles }
        }));
        return;
    }
    
    const isInitialized = await checkKnowledgeBaseInitialization();
    if (!isInitialized) {
        return;
    }

    droppedFiles.forEach(file => requestMethod(file, uploadInput));
}

// 组件挂载时添加全局事件监听器
onMounted(() => {
    document.addEventListener('dragenter', handleGlobalDragEnter, true);
    document.addEventListener('dragover', handleGlobalDragOver, true);
    document.addEventListener('dragleave', handleGlobalDragLeave, true);
    document.addEventListener('drop', handleGlobalDrop, true);
    // 支持通过 URL 查询参数打开全局命令面板，例如旧路径
    // /platform/knowledge-search?q=foo 重定向后携带 ?cmdk=foo
    maybeOpenCmdkFromRoute()
    // 后台预取对话输入栏资源，进入 creatChat / chat 时复用缓存
    void useChatResourcesStore().prefetchChatInput()
});

// 监听路由变化，兼容 SPA 内部跳转时的 ?cmdk= 参数
watch(() => route.query.cmdk, () => {
    maybeOpenCmdkFromRoute()
})

function maybeOpenCmdkFromRoute() {
    if (!('cmdk' in route.query)) return
    const q = String(route.query.cmdk ?? '')
    commandPaletteStore.openPalette(q)
    // 清除 query，避免回退/刷新时反复触发
    const newQuery = { ...route.query }
    delete (newQuery as any).cmdk
    router.replace({ path: route.path, query: newQuery, hash: route.hash })
}

// 组件卸载时移除全局事件监听器
onUnmounted(() => {
    document.removeEventListener('dragenter', handleGlobalDragEnter, true);
    document.removeEventListener('dragover', handleGlobalDragOver, true);
    document.removeEventListener('dragleave', handleGlobalDragLeave, true);
    document.removeEventListener('drop', handleGlobalDrop, true);
    dragCounter = 0;
});
</script>
<style lang="less">
.main {
    display: flex;
    align-items: stretch;
    width: 100%;
    height: 100%;
    min-width: 0;
    min-height: 0;
    background: var(--zeal-canvas, #f3f6fa);
    --td-brand-color: #1268e3;
    --td-brand-color-hover: #0f5fcf;
    --td-brand-color-active: #0b4fae;
    --td-brand-color-light: #eaf2ff;
}

.workspace-frame {
    flex: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    overflow: hidden;
}

/* 右侧路由区：占满剩余宽度与整列高度，并把 min-height:0 传给子页面以便内部 flex 滚动 */
.platform-route-outlet {
    flex: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
}

.upload-mask {
    background-color: rgba(255, 255, 255, 0.8);
    position: fixed;
    width: 100%;
    height: 100%;
    z-index: 999;
    display: flex;
    justify-content: center;
    align-items: center;
}

img {
    -webkit-user-drag: none;
    -khtml-user-drag: none;
    -moz-user-drag: none;
    -o-user-drag: none;
    user-drag: none;
}

@media (max-width: 760px) {
    .main {
        padding-bottom: calc(64px + env(safe-area-inset-bottom));
    }

    .workspace-frame {
        flex-direction: column;
    }
}
</style>
