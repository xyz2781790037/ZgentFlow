<template>
  <div class="trash-page">
    <header>
      <div>
        <span class="eyebrow">数据管理</span>
        <h1>回收站</h1>
        <p>知识库和文档保留 7 天，过期后自动永久删除。</p>
      </div>
      <div class="actions">
        <t-button variant="outline" @click="router.push('/platform/knowledge-bases')">返回知识库</t-button>
        <t-button variant="outline" :loading="loading" @click="load">刷新</t-button>
      </div>
    </header>

    <div class="tabs">
      <button :class="{ active: tab === 'kb' }" @click="tab = 'kb'">知识库 {{ knowledgeBases.length }}</button>
      <button :class="{ active: tab === 'doc' }" @click="tab = 'doc'">文档 {{ documents.length }}</button>
    </div>

    <section class="trash-list">
      <div v-if="loading" class="state"><t-loading size="small" /> 正在加载...</div>
      <div v-else-if="currentRows.length === 0" class="state empty">
        <t-icon name="delete" size="30px" />
        <strong>回收站为空</strong>
      </div>
      <div v-for="item in currentRows" v-else :key="item.id" class="trash-row">
        <span class="item-icon"><t-icon :name="tab === 'kb' ? 'book-open' : 'file-attachment'" /></span>
        <span class="item-main">
          <strong>{{ displayName(item) }}</strong>
          <small>{{ tab === 'kb' ? (item.description || '知识库') : (item.file_type || '文档') }}</small>
        </span>
        <span class="expiry">
          <small>删除时间</small>
          <strong>{{ formatTime(item.deleted_at) }}</strong>
        </span>
        <t-button size="small" variant="outline" @click="restore(item.id)">恢复</t-button>
        <t-button size="small" theme="danger" variant="outline" @click="confirmPurge(item)">永久删除</t-button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import {
  listTrashedKnowledge,
  listTrashedKnowledgeBases,
  purgeTrashedKnowledge,
  purgeTrashedKnowledgeBase,
  restoreTrashedKnowledge,
  restoreTrashedKnowledgeBase,
} from '@/api/knowledge-base'

type TrashItem = {
  id: string
  name?: string
  title?: string
  file_name?: string
  description?: string
  file_type?: string
  deleted_at?: string | { Time?: string }
}

const router = useRouter()
const tab = ref<'kb' | 'doc'>('kb')
const loading = ref(false)
const knowledgeBases = ref<TrashItem[]>([])
const documents = ref<TrashItem[]>([])
const currentRows = computed(() => tab.value === 'kb' ? knowledgeBases.value : documents.value)

const load = async () => {
  loading.value = true
  try {
    const [kbRes, docRes]: any[] = await Promise.all([
      listTrashedKnowledgeBases(),
      listTrashedKnowledge(),
    ])
    knowledgeBases.value = Array.isArray(kbRes?.data) ? kbRes.data : []
    documents.value = Array.isArray(docRes?.data) ? docRes.data : []
  } catch (error: any) {
    MessagePlugin.error(error?.message || '回收站加载失败')
  } finally {
    loading.value = false
  }
}

const displayName = (item: TrashItem) => item.name || item.title || item.file_name || item.id
const deletedTime = (value: TrashItem['deleted_at']) => typeof value === 'string' ? value : value?.Time
const formatTime = (value: TrashItem['deleted_at']) => {
  const time = deletedTime(value)
  return time ? new Date(time).toLocaleString('zh-CN', { hour12: false }) : '未知'
}

const restore = async (id: string) => {
  try {
    if (tab.value === 'kb') await restoreTrashedKnowledgeBase(id)
    else await restoreTrashedKnowledge(id)
    MessagePlugin.success('已恢复')
    await load()
  } catch (error: any) {
    MessagePlugin.error(error?.message || '恢复失败')
  }
}

const confirmPurge = (item: TrashItem) => {
  const dialog = DialogPlugin.confirm({
    header: '永久删除',
    body: `“${displayName(item)}”删除后无法恢复，确定继续吗？`,
    confirmBtn: '永久删除',
    cancelBtn: '取消',
    theme: 'danger',
    onConfirm: async () => {
      try {
        if (tab.value === 'kb') await purgeTrashedKnowledgeBase(item.id)
        else await purgeTrashedKnowledge(item.id)
        MessagePlugin.success('已开始永久删除')
        await load()
      } catch (error: any) {
        MessagePlugin.error(error?.message || '永久删除失败')
      } finally {
        dialog.hide()
      }
    },
  })
}

onMounted(load)
</script>

<style scoped lang="less">
.trash-page {
  height: 100%;
  overflow: auto;
  padding: 42px 48px 64px;
  box-sizing: border-box;
  background: #f3f6fa;
  color: #18212f;
}

header {
  max-width: 1120px;
  margin: 0 auto 28px;
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  gap: 24px;
}

.eyebrow { color: #1769dc; font-size: 11px; font-weight: 750; }
h1 { margin: 4px 0; font-size: 28px; }
p { margin: 0; color: #667085; }
.actions { display: flex; gap: 10px; }

.tabs {
  max-width: 1120px;
  margin: 0 auto;
  display: flex;
  gap: 6px;
  border-bottom: 1px solid #d9e1ec;
}

.tabs button {
  padding: 11px 16px;
  border: 0;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: #667085;
  cursor: pointer;
  font: inherit;
}
.tabs button.active { color: #1769dc; border-bottom-color: #1769dc; font-weight: 700; }

.trash-list {
  max-width: 1120px;
  min-height: 260px;
  margin: 18px auto 0;
  border: 1px solid #dfe6ef;
  border-radius: 10px;
  background: #fff;
  overflow: hidden;
}

.trash-row {
  min-height: 72px;
  padding: 12px 18px;
  display: grid;
  grid-template-columns: 38px minmax(0, 1fr) 190px auto auto;
  align-items: center;
  gap: 14px;
  border-bottom: 1px solid #edf1f5;
}
.trash-row:last-child { border-bottom: 0; }
.item-icon { width: 34px; height: 34px; display: grid; place-items: center; border-radius: 7px; color: #1769dc; background: #eaf2ff; }
.item-main, .expiry { min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.item-main strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.item-main small, .expiry small { color: #7a8595; }
.expiry strong { font-size: 13px; font-weight: 600; }
.state { min-height: 260px; display: flex; align-items: center; justify-content: center; gap: 9px; color: #667085; }
.state.empty { flex-direction: column; }
</style>
