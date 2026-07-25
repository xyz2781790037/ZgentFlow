<template>
  <div class="library-page">
    <header class="library-header">
      <div class="library-title-cluster">
        <span class="library-title-icon"><t-icon name="book-open" size="22px" /></span>
        <div>
          <span class="eyebrow">知识运营</span>
          <h1>知识库</h1>
          <p>{{ knowledgeBases.length }} 个知识库 · {{ totalDocuments }} 份文档 · {{ totalChunks }} 个分段</p>
        </div>
      </div>
      <div class="header-actions">
		<t-button variant="outline" @click="joinVisible = true">
		  <template #icon><t-icon name="user-add" /></template>
		  加入已有知识库
		</t-button>
        <t-button variant="outline" @click="router.push('/platform/recycle-bin')">
          <template #icon><t-icon name="delete" /></template>
          回收站
        </t-button>
        <t-button variant="outline" @click="loadKnowledgeBases">
          <template #icon><t-icon name="refresh" /></template>
          刷新
        </t-button>
        <t-button theme="primary" @click="uiStore.openCreateKB('document')">
          <template #icon><t-icon name="add" /></template>
          新建知识库
        </t-button>
      </div>
    </header>

    <section class="library-summary" aria-label="索引概览">
      <div class="summary-block">
          <span class="summary-icon is-blue"><t-icon name="check-circle-filled" /></span>
        <span><strong class="summary-value">{{ readyCount }}/{{ knowledgeBases.length }}</strong><small class="summary-label">知识库就绪</small></span>
      </div>
      <div class="summary-block">
          <span class="summary-icon is-teal"><t-icon name="file-attachment" /></span>
        <span><strong class="summary-value">{{ totalDocuments }}</strong><small class="summary-label">源文档</small></span>
      </div>
    </section>

    <section class="library-table" aria-label="知识库列表">
      <div class="table-toolbar">
        <div>
          <h2>知识源</h2>
          <span>文档与 FAQ 知识库统一管理。</span>
        </div>
        <label class="table-search">
          <t-icon name="search" size="16px" />
          <input v-model="query" type="search" placeholder="筛选知识源" />
        </label>
      </div>

      <div class="table-head">
        <span>知识库</span>
        <span>内容</span>
        <span>索引</span>
        <span>状态</span>
        <span aria-hidden="true"></span>
      </div>

      <div v-if="loading" class="library-state"><t-loading size="small" /> 正在加载知识库...</div>
      <div v-else-if="filteredKnowledgeBases.length === 0" class="library-empty">
        <div class="empty-icon"><t-icon name="folder-add" size="24px" /></div>
        <h3>{{ knowledgeBases.length ? '没有匹配的知识库' : '创建知识库' }}</h3>
        <p v-if="!knowledgeBases.length">上传文档生成混合检索索引。</p>
        <t-button v-if="!knowledgeBases.length" theme="primary" @click="uiStore.openCreateKB('document')">创建知识库</t-button>
      </div>

      <button
        v-for="kb in filteredKnowledgeBases"
        v-else
        :key="kb.id"
        type="button"
        class="library-row"
        @click="openKnowledgeBase(kb.id)"
      >
        <span class="library-identity">
          <span class="library-icon"><t-icon name="book-open" size="19px" /></span>
          <span>
            <strong>{{ kb.name }}</strong>
			<span v-if="kb.is_shared" class="shared-badge">共享 · {{ roleLabel(kb.access_role) }}</span>
            <small>{{ kb.description || '暂无描述' }}</small>
          </span>
        </span>
        <span class="content-metrics">
          <strong>{{ kb.knowledge_count || 0 }}</strong>
          <small>{{ kb.chunk_count || 0 }} 个分段</small>
        </span>
        <span class="index-list">
          <span class="index-chip active"><i></i>向量</span>
        </span>
        <span class="ready-state" :class="{ pending: !isReady(kb) }">
          <i></i>{{ isReady(kb) ? '就绪' : '需要配置' }}
        </span>
        <span class="row-action"><t-icon name="chevron-right" size="18px" /></span>
      </button>
    </section>

    <KnowledgeBaseEditorModal
      :visible="uiStore.showKBEditorModal"
      :mode="uiStore.kbEditorMode"
      :kb-id="uiStore.currentKBId || undefined"
      :initial-type="uiStore.kbEditorType"
      @update:visible="(value) => value ? null : uiStore.closeKBEditor()"
      @success="handleEditorSuccess"
    />
	<JoinKnowledgeBaseDialog v-model:visible="joinVisible" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { listKnowledgeBases } from '@/api/knowledge-base'
import { useUIStore } from '@/stores/ui'
import KnowledgeBaseEditorModal from './KnowledgeBaseEditorModal.vue'
import JoinKnowledgeBaseDialog from './components/JoinKnowledgeBaseDialog.vue'

type KnowledgeBase = {
  id: string
  name: string
  description?: string
  knowledge_count?: number
  chunk_count?: number
  embedding_model_id?: string
  summary_model_id?: string
  indexing_strategy?: {
    vector_enabled?: boolean
    keyword_enabled?: boolean
  }
	access_role?: 'owner' | 'admin' | 'writer' | 'reader'
	is_shared?: boolean
	owner_username?: string
}

const router = useRouter()
const uiStore = useUIStore()
const loading = ref(false)
const query = ref('')
const knowledgeBases = ref<KnowledgeBase[]>([])
const joinVisible = ref(false)
const roleLabel = (role?: string) => ({ owner: '所有者', admin: '管理员', writer: '写入用户', reader: '读取用户' }[role || ''] || '成员')

const isReady = (kb: KnowledgeBase) => {
  const needsEmbedding = !kb.indexing_strategy || kb.indexing_strategy.vector_enabled || kb.indexing_strategy.keyword_enabled
  return !!kb.summary_model_id && (!needsEmbedding || !!kb.embedding_model_id)
}

const filteredKnowledgeBases = computed(() => {
  const term = query.value.trim().toLowerCase()
  if (!term) return knowledgeBases.value
  return knowledgeBases.value.filter((kb) => (kb.name + ' ' + (kb.description || '')).toLowerCase().includes(term))
})
const totalDocuments = computed(() => knowledgeBases.value.reduce((sum, kb) => sum + (kb.knowledge_count || 0), 0))
const totalChunks = computed(() => knowledgeBases.value.reduce((sum, kb) => sum + (kb.chunk_count || 0), 0))
const readyCount = computed(() => knowledgeBases.value.filter(isReady).length)

const loadKnowledgeBases = async () => {
  loading.value = true
  try {
    const response: any = await listKnowledgeBases()
    knowledgeBases.value = Array.isArray(response?.data) ? response.data : []
  } catch (error) {
    console.error('[ZealLibrary] Failed to load knowledge bases:', error)
    MessagePlugin.error('知识库加载失败')
  } finally {
    loading.value = false
  }
}

const openKnowledgeBase = (id: string) => router.push('/platform/knowledge-bases/' + id)
const handleEditorSuccess = async (id: string) => {
  uiStore.closeKBEditor()
  await loadKnowledgeBases()
  if (id) openKnowledgeBase(id)
}

onMounted(loadKnowledgeBases)
</script>

<style scoped lang="less">
.library-page {
  height: 100%;
  min-height: 0;
  overflow-y: auto;
  padding: 42px 48px 64px;
  box-sizing: border-box;
  background: var(--zeal-canvas, #f3f6fa);
  color: var(--zeal-ink, #18212f);
}

.library-header {
  max-width: 1240px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
}

.library-title-cluster {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 15px;
}

.library-title-icon {
  width: 48px;
  height: 48px;
  flex: 0 0 48px;
  display: grid;
  place-items: center;
  border: 1px solid #bcd3f3;
  border-radius: 8px;
  color: var(--zeal-primary, #1769dc);
  background: var(--zeal-primary-soft, #eaf2ff);
  box-shadow: inset 0 0 0 3px rgba(255, 255, 255, 0.46);
}

.eyebrow {
  color: var(--zeal-primary, #1769dc);
  font-size: 11px;
  font-weight: 750;
  text-transform: uppercase;
}

h1 {
  margin: 3px 0 2px;
  font-size: 28px;
  line-height: 35px;
  font-weight: 750;
}

.library-header p {
  margin: 0;
  color: var(--zeal-muted, #778398);
  font-size: 13px;
}

.header-actions { display: flex; gap: 8px; }

.library-summary {
  max-width: 1240px;
  margin: 30px auto 0;
  min-height: 92px;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.summary-block {
  min-width: 0;
  padding: 18px 20px;
  display: flex;
  align-items: center;
  gap: 13px;
  border: 1px solid var(--zeal-line, #dbe3ed);
  border-radius: 8px;
  background: var(--zeal-surface, #fff);
  box-shadow: var(--zeal-shadow-xs);
}

.summary-block > span:last-child { min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.summary-icon {
  width: 38px;
  height: 38px;
  flex: 0 0 38px;
  display: grid;
  place-items: center;
  border-radius: 7px;
  border: 1px solid color-mix(in srgb, currentColor 18%, transparent);
  font-size: 18px;
  box-shadow: inset 0 0 0 3px rgba(255, 255, 255, 0.34);
}
.summary-icon.is-blue { color: var(--zeal-primary); background: var(--zeal-primary-soft); }
.summary-icon.is-teal { color: var(--zeal-teal); background: var(--zeal-teal-soft); }
.summary-value { font-size: 22px; line-height: 27px; font-weight: 750; }
.summary-label { color: var(--zeal-muted, #778398); font-size: 11px; font-weight: 500; }

.library-table {
  max-width: 1240px;
  margin: 22px auto 0;
  overflow: hidden;
  border: 1px solid var(--zeal-line, #dbe3ed);
  border-radius: 8px;
  background: var(--zeal-surface, #fff);
  box-shadow: var(--zeal-shadow-xs);
}

.table-toolbar {
  min-height: 82px;
  padding: 18px 22px;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  border-bottom: 1px solid var(--zeal-line, #dbe3ed);
}

.table-toolbar h2 { margin: 0 0 3px; font-size: 15px; font-weight: 700; }
.table-toolbar span { color: var(--zeal-muted, #778398); font-size: 11px; }

.table-search {
  width: 230px;
  height: 38px;
  padding: 0 11px;
  display: flex;
  align-items: center;
  gap: 7px;
  border: 1px solid var(--zeal-line, #dbe3ed);
  border-radius: 7px;
  color: var(--zeal-faint, #9aa5b5);
  background: var(--zeal-surface-subtle, #f8fafc);
}

.table-search:focus-within {
  border-color: var(--zeal-primary, #1769dc);
  background: var(--zeal-surface, #fff);
  box-shadow: 0 0 0 3px rgba(23, 105, 220, 0.1);
}

.table-search input {
  width: 100%;
  min-width: 0;
  border: 0;
  outline: 0;
  background: transparent;
  font: inherit;
  font-size: 12px;
}

.table-head,
.library-row {
  display: grid;
  grid-template-columns: minmax(280px, 1.6fr) 130px minmax(250px, 1fr) 130px 28px;
  align-items: center;
  column-gap: 18px;
}

.table-head {
  min-height: 42px;
  padding: 0 22px;
  color: var(--zeal-muted, #778398);
  background: var(--zeal-surface-subtle, #f8fafc);
  border-bottom: 1px solid var(--zeal-line, #dbe3ed);
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
}

.library-row {
  width: 100%;
  min-height: 88px;
  padding: 13px 22px;
  box-sizing: border-box;
  border: 0;
  border-bottom: 1px solid var(--zeal-line, #dbe3ed);
  background: var(--zeal-surface, #fff);
  color: inherit;
  text-align: left;
  cursor: pointer;
  font: inherit;
  transition: background 160ms ease, box-shadow 160ms ease;
}

.library-row:last-child { border-bottom: 0; }
.library-row:hover {
  position: relative;
  z-index: 1;
  background: #f7faff;
  box-shadow: inset 3px 0 0 var(--zeal-primary, #1769dc);
}

.library-identity {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 12px;
}

.library-icon,
.empty-icon {
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  display: grid;
  place-items: center;
  border: 1px solid #bed2f2;
  background: #edf4ff;
  color: #1268e3;
  border-radius: 8px;
  box-shadow: inset 0 0 0 3px rgba(255, 255, 255, 0.48);
}

.library-identity > span:last-child,
.content-metrics {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.library-identity strong,
.content-metrics strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  font-weight: 680;
}

.library-identity small,
.content-metrics small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #8a93a3;
  font-size: 11px;
}

.shared-badge {
  width: fit-content;
  padding: 2px 6px;
  border-radius: 4px;
  color: #1268e3;
  background: #edf4ff;
  font-size: 10px;
  font-weight: 650;
}

.index-list { display: flex; flex-wrap: wrap; gap: 5px; }

.index-chip {
  height: 24px;
  padding: 0 7px;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  border: 1px solid #e0e4e9;
  color: #9aa2b0;
  background: #fafbfc;
  border-radius: 5px;
  font-size: 10px;
}

.index-chip i,
.ready-state i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #c3c9d1;
}

.index-chip.active {
  border-color: #c8daf5;
  color: #245b9f;
  background: #f2f7ff;
}

.index-chip.active i { background: #1268e3; }

.ready-state {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: #168057;
  font-size: 11px;
  font-weight: 650;
}

.ready-state i { background: #18a66a; }
.ready-state.pending { color: #a35b00; }
.ready-state.pending i { background: #e49424; }
.row-action { color: #98a2b3; }

.library-state,
.library-empty {
  min-height: 220px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  color: #7c8696;
}

.library-empty { flex-direction: column; text-align: center; }
.library-empty h3 { margin: 5px 0 0; color: #263247; font-size: 15px; }
.library-empty p { margin: 0 0 8px; color: #7c8696; font-size: 12px; }

@media (max-width: 1100px) {
  .library-page { padding-inline: 30px; }
  .table-head,
  .library-row { grid-template-columns: minmax(240px, 1.5fr) 100px minmax(150px, 0.8fr) 110px 22px; column-gap: 12px; }
}

@media (max-width: 760px) {
  .library-page { padding: 24px 16px 34px; }
  .library-header { align-items: flex-start; flex-direction: column; gap: 18px; }
  .library-title-icon { width: 42px; height: 42px; flex-basis: 42px; }
  h1 { font-size: 24px; line-height: 30px; }
  .header-actions { width: 100%; min-width: 0; gap: 7px; }
  .header-actions :deep(.t-button) {
    flex: 1 1 0;
    min-width: 0;
    width: 0;
    padding-inline: 8px;
  }
  .library-summary { margin-top: 22px; grid-template-columns: 1fr; gap: 8px; }
  .summary-block { min-height: 70px; padding: 13px 15px; }
  .library-table { margin-top: 16px; }
  .table-toolbar { align-items: stretch; flex-direction: column; gap: 14px; }
  .table-search { width: 100%; }
  .table-head { display: none; }
  .library-row {
    min-height: 0;
    padding: 16px;
    grid-template-columns: 1fr auto;
    grid-template-areas: "identity action" "content content" "indexes state";
    row-gap: 13px;
  }
  .library-identity { grid-area: identity; }
  .content-metrics { grid-area: content; flex-direction: row; align-items: baseline; gap: 8px; padding-left: 54px; }
  .index-list { grid-area: indexes; padding-left: 54px; }
  .ready-state { grid-area: state; justify-self: end; }
  .row-action { grid-area: action; align-self: center; }
}
</style>
