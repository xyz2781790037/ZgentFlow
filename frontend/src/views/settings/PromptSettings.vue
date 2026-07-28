<template>
  <div class="prompt-settings">
    <aside class="prompt-list">
      <label class="search">
        <t-icon name="search" />
        <input v-model="query" placeholder="搜索提示词" />
      </label>
      <button
        v-for="item in filteredPrompts"
        :key="`${item.category}/${item.template_id}`"
        type="button"
        :class="{ active: samePrompt(item, selected) }"
        @click="selectPrompt(item)"
      >
        <strong>{{ promptName(item) }}</strong>
        <small>{{ categoryName(item.category) }} · v{{ item.version }}</small>
      </button>
    </aside>

    <main v-if="selected" ref="editorRef" class="editor">
      <div class="editor-head">
        <div>
          <h3>{{ promptName(selected) }}</h3>
          <p>{{ selected.description || selected.template_id }}</p>
        </div>
        <t-button theme="primary" :loading="saving" @click="save">保存新版本</t-button>
      </div>

      <label class="field">
        <span>主提示词</span>
        <textarea v-model="content" spellcheck="false" />
      </label>
      <label v-if="selected.user_prompt !== undefined" class="field user-field">
        <span>用户消息模板</span>
        <textarea v-model="userPrompt" spellcheck="false" />
      </label>

      <section class="history">
        <div class="history-head">
          <h4>版本历史</h4>
          <span>回滚会生成一个新的当前版本，历史记录不会被覆盖。</span>
        </div>
        <div v-if="historyLoading" class="history-state"><t-loading size="small" /></div>
        <div v-for="version in history" v-else :key="version.id" class="version-row">
          <span>
            <strong>v{{ version.version }}</strong>
            <small>{{ formatTime(version.created_at) }}</small>
          </span>
          <span v-if="version.is_active" class="current">当前</span>
          <t-button
            v-else
            size="small"
            variant="text"
            :loading="rollingBack === version.version"
            @click="rollback(version.version)"
          >回滚到此版本</t-button>
        </div>
      </section>
    </main>
    <div v-else class="empty">暂无可编辑提示词</div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  getPromptHistory,
  listPrompts,
  rollbackPrompt,
  updatePrompt,
  type PromptTemplate,
  type PromptVersion,
} from '@/api/prompt'

const prompts = ref<PromptTemplate[]>([])
const selected = ref<PromptTemplate | null>(null)
const history = ref<PromptVersion[]>([])
const query = ref('')
const content = ref('')
const userPrompt = ref('')
const saving = ref(false)
const historyLoading = ref(false)
const rollingBack = ref<number | null>(null)
const editorRef = ref<HTMLElement | null>(null)

const categoryLabels: Record<string, string> = {
  system_prompt: '问答系统提示词',
  context_template: '检索上下文',
  rewrite: '问题改写',
  fallback: '兜底回答',
  generate_session_title: '会话标题',
  generate_summary: '文档摘要',
  agent_system_prompt: '深度问答',
  generate_questions: '分块问题生成',
  intent_prompts: '非检索回复',
}

const promptLabels: Record<string, string> = {
  'system_prompt/default_kb': '知识库问答主提示词',
  'context_template/default_context': '默认检索上下文模板',
  'rewrite/default_rewrite': '默认问题改写提示词',
  'fallback/default_fallback_prompt': '无检索结果兜底提示词',
  'generate_session_title/default_session_title': '会话标题生成提示词',
  'generate_summary/default_summary': '文档摘要生成提示词',
  'agent_system_prompt/hybrid_rag_wiki_agent': '深度问答系统提示词',
  'generate_questions/default_generate_questions': '分块问题生成提示词',
  'intent_prompts/greeting': '问候回复提示词',
  'intent_prompts/chitchat': '闲聊回复提示词',
  'intent_prompts/follow_up': '追问回复提示词',
  'intent_prompts/image_only': '图片分析回复提示词',
  'intent_prompts/summarize': '对话总结回复提示词',
  'intent_prompts/web_search': '联网不可用回复提示词',
  'intent_prompts/doc_only': '文档分析回复提示词',
}

const categoryName = (category: string) => categoryLabels[category] || category
const promptName = (item: PromptTemplate) =>
  promptLabels[`${item.category}/${item.template_id}`]
  || `${categoryName(item.category)} - ${item.name || item.template_id}`
const samePrompt = (a?: PromptTemplate | null, b?: PromptTemplate | null) =>
  !!a && !!b && a.category === b.category && a.template_id === b.template_id
const filteredPrompts = computed(() => {
  const term = query.value.trim().toLowerCase()
  if (!term) return prompts.value
  return prompts.value.filter((item) =>
    `${promptName(item)} ${item.name} ${item.template_id} ${categoryName(item.category)}`.toLowerCase().includes(term),
  )
})

const loadHistory = async () => {
  if (!selected.value) return
  historyLoading.value = true
  try {
    const response: any = await getPromptHistory(selected.value.category, selected.value.template_id)
    history.value = Array.isArray(response?.data) ? response.data : []
  } finally {
    historyLoading.value = false
  }
}

const selectPrompt = async (item: PromptTemplate) => {
  selected.value = item
  content.value = item.content
  userPrompt.value = item.user_prompt || ''
  await nextTick()
  const settingsContent = editorRef.value?.closest('.zeal-settings-content')
  if (settingsContent instanceof HTMLElement) settingsContent.scrollTop = 0
  await loadHistory()
}

const load = async (keepSelection = false) => {
  const previous = selected.value
  const response: any = await listPrompts()
  prompts.value = Array.isArray(response?.data) ? response.data : []
  const next = keepSelection && previous
    ? prompts.value.find((item) => samePrompt(item, previous))
    : prompts.value[0]
  if (next) await selectPrompt(next)
}

const save = async () => {
  if (!selected.value || !content.value.trim()) {
    MessagePlugin.warning('主提示词不能为空')
    return
  }
  saving.value = true
  try {
    await updatePrompt(selected.value.category, selected.value.template_id, {
      content: content.value,
      user_prompt: userPrompt.value,
    })
    MessagePlugin.success('提示词新版本已生效')
    await load(true)
  } catch (error: any) {
    MessagePlugin.error(error?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

const rollback = async (version: number) => {
  if (!selected.value) return
  rollingBack.value = version
  try {
    await rollbackPrompt(selected.value.category, selected.value.template_id, version)
    MessagePlugin.success(`已回滚到 v${version} 的内容`)
    await load(true)
  } catch (error: any) {
    MessagePlugin.error(error?.message || '回滚失败')
  } finally {
    rollingBack.value = null
  }
}

const formatTime = (value: string) =>
  new Date(value).toLocaleString('zh-CN', { hour12: false })

onMounted(async () => {
  try {
    await load()
  } catch (error: any) {
    MessagePlugin.error(error?.message || '提示词加载失败')
  }
})
</script>

<style scoped lang="less">
.prompt-settings { min-height: 560px; display: grid; grid-template-columns: 250px minmax(0, 1fr); align-items: start; border: 1px solid #dfe6ef; border-radius: 9px; background: #fff; }
.prompt-list { position: sticky; top: 0; max-height: calc(100vh - 200px); padding: 12px; overflow-y: auto; box-sizing: border-box; border-right: 1px solid #e6ebf2; border-radius: 8px 0 0 8px; background: #f8fafc; scrollbar-width: thin; }
.search { height: 36px; padding: 0 10px; margin-bottom: 10px; display: flex; align-items: center; gap: 7px; border: 1px solid #d8e0ea; border-radius: 6px; background: #fff; }
.search input { width: 100%; border: 0; outline: 0; background: transparent; }
.prompt-list button { width: 100%; padding: 10px; display: flex; flex-direction: column; gap: 4px; border: 0; border-radius: 6px; background: transparent; text-align: left; cursor: pointer; }
.prompt-list button:hover, .prompt-list button.active { background: #e9f2ff; color: #1769dc; }
.prompt-list small { color: #788496; }
.editor { min-width: 0; padding: 22px; }
.editor-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 20px; }
h3, h4 { margin: 0; }
.editor-head p { margin: 5px 0 0; color: #6d7888; }
.field { margin-top: 20px; display: flex; flex-direction: column; gap: 8px; font-weight: 650; }
.field textarea { min-height: 230px; padding: 13px; resize: vertical; border: 1px solid #ced7e3; border-radius: 7px; outline: none; font: 13px/1.65 ui-monospace, SFMono-Regular, Menlo, monospace; }
.field textarea:focus { border-color: #1769dc; box-shadow: 0 0 0 2px #e6f0ff; }
.user-field textarea { min-height: 130px; }
.history { margin-top: 24px; border-top: 1px solid #e5eaf1; padding-top: 18px; }
.history-head { display: flex; justify-content: space-between; gap: 16px; }
.history-head span { color: #7a8595; font-size: 12px; }
.version-row { min-height: 48px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid #eef1f5; }
.version-row > span:first-child { display: flex; flex-direction: column; gap: 2px; }
.version-row small { color: #7a8595; }
.current { padding: 3px 8px; border-radius: 10px; color: #147a52; background: #e8f8f0; font-size: 12px; }
.history-state, .empty { min-height: 180px; display: grid; place-items: center; color: #778294; }
</style>
