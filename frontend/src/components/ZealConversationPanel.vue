<template>
  <aside class="conversation-panel">
    <div class="conversation-head">
      <div>
        <span class="section-label">工作区</span>
        <h2>历史会话</h2>
      </div>
      <t-tooltip content="新建问答" placement="bottom">
        <button class="new-chat" type="button" aria-label="新建问答" @click="newConversation">
          <t-icon name="add" size="18px" />
        </button>
      </t-tooltip>
    </div>

    <label class="conversation-search">
      <t-icon name="search" size="16px" />
      <input v-model="query" type="search" placeholder="搜索会话" />
    </label>

    <div class="conversation-list">
      <div class="list-caption">最近</div>
      <div v-if="loading" class="panel-state">加载中...</div>
      <template v-else>
        <article
          v-for="session in filteredSessions"
          :key="session.id"
          class="conversation-row"
          :class="{ active: String(route.params.chatid || '') === String(session.id) }"
          role="button"
          tabindex="0"
          @click="openSession(session.id)"
          @keydown.enter="openSession(session.id)"
          @keydown.space.prevent="openSession(session.id)"
        >
          <div class="conversation-row-main">
            <span class="conversation-title">
              <t-icon v-if="session.is_pinned" name="pin" size="12px" />
              <span class="conversation-title-text">{{ cleanTitle(session.title) || '未命名会话' }}</span>
            </span>
            <span class="conversation-time">{{ formatDate(session.updated_at || session.created_at) }}</span>
          </div>
          <t-dropdown
            :options="sessionMenuOptions(session)"
            placement="bottom-right"
            attach="body"
            trigger="click"
            @click="(data: any) => handleSessionAction(data.value, session)"
          >
            <button type="button" class="conversation-more" title="会话操作" aria-label="会话操作" @click.stop>
              <t-icon name="ellipsis" size="16px" />
            </button>
          </t-dropdown>
        </article>
        <div v-if="filteredSessions.length === 0" class="panel-state">暂无会话</div>
      </template>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import { delSession, getSessionsList, pinSession, unpinSession } from '@/api/chat'

type SessionRow = {
  id: string
  title?: string
  is_pinned?: boolean
  created_at?: string
  updated_at?: string
}

const route = useRoute()
const router = useRouter()
const query = ref('')
const loading = ref(false)
const sessions = ref<SessionRow[]>([])

const filteredSessions = computed(() => {
  const term = query.value.trim().toLowerCase()
  const visible = term
    ? sessions.value.filter((item) => cleanTitle(item.title).toLowerCase().includes(term))
    : sessions.value
  return [...visible].sort((a, b) => Number(Boolean(b.is_pinned)) - Number(Boolean(a.is_pinned)))
})

const cleanTitle = (title?: string) => {
  let value = String(title || '').replace(/<think>[\s\S]*?<\/think>/gi, '').trim()
  const prefixes = [
    '根据用户的问题，生成的短会话标题为：',
    '根据用户的问题，生成的短会话标题为',
    '根据用户的问题生成的短会话标题为：',
    '根据用户的问题生成的短会话标题为',
    '短会话标题为：',
    '短会话标题为',
    '会话标题：',
    '会话标题:',
    '标题：',
    '标题:',
  ]
  for (const prefix of prefixes) {
    if (value.startsWith(prefix)) {
      value = value.slice(prefix.length).trim()
      break
    }
  }
  value = value.split(/\r?\n/)[0].trim()
  return value.replace(/^[-*`\s]+|[-*`\s]+$/g, '').replace(/^[“”"'《【]|[”"'》】]$/g, '').trim()
}

const loadSessions = async () => {
  loading.value = true
  try {
    const response: any = await getSessionsList(1, 30)
    sessions.value = Array.isArray(response?.data) ? response.data : []
  } catch (error) {
    console.warn('[ZealConversationPanel] Failed to load sessions:', error)
    sessions.value = []
  } finally {
    loading.value = false
  }
}

const newConversation = () => router.push('/platform/creatChat')
const openSession = (id: string) => router.push('/platform/chat/' + id)

const sessionMenuOptions = (session: SessionRow) => ([
  { content: session.is_pinned ? '取消置顶' : '置顶会话', value: 'pin' },
  { content: '删除会话', value: 'delete', theme: 'error' },
])

const handleSessionAction = async (action: string, session: SessionRow) => {
  if (action === 'pin') {
    try {
      if (session.is_pinned) {
        await unpinSession(session.id)
        session.is_pinned = false
        MessagePlugin.success('已取消置顶')
      } else {
        await pinSession(session.id)
        session.is_pinned = true
        MessagePlugin.success('会话已置顶')
      }
    } catch (error: any) {
      MessagePlugin.error(error?.message || '置顶操作失败')
    }
    return
  }

  if (action !== 'delete') return
  const dialog = DialogPlugin.confirm({
    header: '删除会话',
    body: '删除后该会话及其问答记录将无法恢复，确定继续吗？',
    confirmBtn: '删除',
    cancelBtn: '取消',
    theme: 'warning',
    onConfirm: async () => {
      try {
        await delSession(session.id)
        sessions.value = sessions.value.filter((item) => item.id !== session.id)
        if (String(route.params.chatid || '') === String(session.id)) {
          await newConversation()
        }
        MessagePlugin.success('会话已删除')
      } catch (error: any) {
        MessagePlugin.error(error?.message || '删除会话失败')
      } finally {
        dialog.hide()
      }
    },
  })
}

const formatDate = (value?: string) => {
  if (!value) return ''
  const date = new Date(value)
  const today = new Date()
  if (date.toDateString() === today.toDateString()) {
    return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  }
  return date.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })
}

const handleTitleUpdate = (event: Event) => {
  const detail = (event as CustomEvent<{ sessionId?: string; title?: string }>).detail
  if (!detail?.sessionId || !detail.title) return
  const session = sessions.value.find((item) => item.id === detail.sessionId)
  if (session) session.title = detail.title
}

onMounted(() => {
  loadSessions()
  window.addEventListener('session-title-updated', handleTitleUpdate)
})
onUnmounted(() => window.removeEventListener('session-title-updated', handleTitleUpdate))
watch(() => route.params.chatid, () => loadSessions())
</script>

<style scoped lang="less">
.conversation-panel {
  width: 286px;
  flex: 0 0 286px;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: #f7f9fc;
  border-right: 1px solid var(--zeal-line, #dbe3ed);
}

.conversation-head {
  height: 104px;
  padding: 24px 20px 16px;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  box-sizing: border-box;
}

.section-label,
.list-caption {
  color: var(--zeal-muted, #778398);
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
}

h2 {
  margin: 4px 0 0;
  color: var(--zeal-ink, #18212f);
  font-size: 18px;
  line-height: 22px;
  font-weight: 720;
}

.new-chat {
  width: 34px;
  height: 34px;
  border: 1px solid var(--zeal-line-strong, #c6d1df);
  border-radius: 7px;
  background: var(--zeal-surface, #fff);
  color: var(--zeal-ink, #18212f);
  display: grid;
  place-items: center;
  cursor: pointer;
}

.new-chat:hover {
  border-color: var(--zeal-primary, #1769dc);
  color: var(--zeal-primary, #1769dc);
  background: var(--zeal-primary-soft, #eaf2ff);
}

.conversation-search {
  height: 38px;
  margin: 0 16px 16px;
  padding: 0 11px;
  display: flex;
  align-items: center;
  gap: 7px;
  color: #98a2b3;
  border: 1px solid var(--zeal-line, #dbe3ed);
  background: var(--zeal-surface, #fff);
  border-radius: 7px;
}

.conversation-search:focus-within {
  border-color: var(--zeal-primary, #1769dc);
  box-shadow: 0 0 0 3px rgba(23, 105, 220, 0.1);
}

.conversation-search input {
  width: 100%;
  min-width: 0;
  border: 0;
  outline: 0;
  background: transparent;
  color: #253047;
  font: inherit;
  font-size: 12px;
}

.conversation-list {
  min-height: 0;
  overflow-y: auto;
  padding: 0 12px 20px;
}

.list-caption { padding: 6px 8px 8px; }

.conversation-row {
  position: relative;
  width: 100%;
  min-height: 54px;
  padding: 9px 10px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: #344054;
  text-align: left;
  cursor: pointer;
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.conversation-row-main {
  min-width: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 3px;
}

.conversation-row:hover { background: #edf2f7; }

.conversation-row.active {
  background: var(--zeal-surface, #fff);
  color: var(--zeal-primary, #1769dc);
  box-shadow: var(--zeal-shadow-xs, 0 1px 2px rgba(23, 34, 51, 0.06));
}

.conversation-title {
  width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 5px;
}

.conversation-title :deep(.t-icon) {
  flex: 0 0 auto;
  color: var(--zeal-primary, #1769dc);
}

.conversation-title-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.conversation-more {
  width: 26px;
  height: 26px;
  flex: 0 0 26px;
  padding: 0;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: #98a2b3;
  display: grid;
  place-items: center;
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.16s ease, color 0.16s ease, background 0.16s ease;
}

.conversation-row:hover .conversation-more,
.conversation-more:focus-visible {
  opacity: 1;
}

.conversation-more:hover {
  color: var(--zeal-primary, #1769dc);
  background: var(--zeal-primary-soft, #eaf2ff);
}

.conversation-time {
  color: #98a2b3;
  font-size: 10px;
}

.panel-state {
  padding: 18px 8px;
  color: #98a2b3;
  font-size: 12px;
}

@media (max-width: 1240px) {
  .conversation-panel {
    width: 238px;
    flex-basis: 238px;
  }
}

@media (max-width: 760px) {
  .conversation-panel {
    width: 100%;
    height: 70px;
    flex: 0 0 70px;
    flex-direction: row;
    align-items: center;
    border-right: 0;
    border-bottom: 1px solid var(--zeal-line, #dbe3ed);
    overflow: hidden;
  }
  .conversation-head {
    width: 58px;
    height: 70px;
    flex: 0 0 58px;
    padding: 0;
    align-items: center;
    justify-content: center;
  }
  .conversation-head > div,
  .conversation-search,
  .list-caption,
  .panel-state { display: none; }
  .conversation-list {
    flex: 1;
    display: flex;
    gap: 6px;
    overflow-x: auto;
    overflow-y: hidden;
    padding: 8px 10px 8px 0;
  }
  .conversation-row {
    width: 152px;
    min-width: 152px;
    min-height: 52px;
    padding: 6px 9px;
    background: var(--zeal-surface, #fff);
  }
  .conversation-more { opacity: 1; }
}
</style>
