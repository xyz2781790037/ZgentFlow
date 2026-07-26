<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import {
  cancelKnowledgeParse,
  getKnowledgeSpans,
  reparseKnowledge,
} from '@/api/knowledge-base'

interface SpanNode {
  span_id?: string
  name: string
  kind: string
  status: string
  started_at?: string | null
  finished_at?: string | null
  duration_ms?: number
  error_code?: string
  error_message?: string
  input?: Record<string, unknown>
  output?: Record<string, unknown>
  children?: SpanNode[]
}

interface ProgressPayload {
  attempt: number
  latest_attempt: number
  parse_status: string
  current_stage?: string
  trace?: SpanNode
  last_error?: {
    name?: string
    error_code?: string
    error_message?: string
  } | null
}

const props = withDefaults(defineProps<{
  knowledgeId: string
  parseStatus?: string
  docTitle?: string
  showClose?: boolean
}>(), {
  parseStatus: '',
  docTitle: '',
  showClose: false,
})

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
const STAGES = ['docreader', 'chunking', 'embedding', 'multimodal', 'postprocess'] as const
const POLL_INTERVAL = 2000

const data = ref<ProgressPayload | null>(null)
const loading = ref(false)
const refreshing = ref(false)
const cancelling = ref(false)
const retrying = ref(false)
const selectedAttempt = ref<number | undefined>()
const selectedStageName = ref<string>('')
let pollTimer: ReturnType<typeof setInterval> | null = null
let fetchInFlight = false

const stages = computed<SpanNode[]>(() => {
  const children = data.value?.trace?.children || []
  const byName = new Map<string, SpanNode>()
  for (const child of children) {
    if (child?.kind === 'stage') byName.set(child.name, child)
  }
  return STAGES.map((name) => byName.get(name) || {
    name,
    kind: 'stage',
    status: 'pending',
  })
})

function isInFlight(status?: string) {
  return status === 'pending' || status === 'processing' || status === 'finalizing'
}

function treeIsActive(node?: SpanNode): boolean {
  if (!node) return false
  if (node.status === 'running' || node.status === 'pending') return true
  return (node.children || []).some(treeIsActive)
}

const live = computed(() => {
  const status = data.value?.parse_status || props.parseStatus
  if (status === 'completed' || status === 'failed' || status === 'cancelled') return false
  return isInFlight(status) || treeIsActive(data.value?.trace)
})

const stageIndex = computed(() => {
  const status = data.value?.parse_status || props.parseStatus
  if (status === 'completed' || status === 'finalizing') return 5
  const current = String(data.value?.current_stage || '')
  const direct = STAGES.indexOf(current as typeof STAGES[number])
  if (direct >= 0) return direct + 1
  const activeIndex = stages.value.findIndex(stage => stage.status === 'running' || stage.status === 'failed')
  if (activeIndex >= 0) return activeIndex + 1
  const done = stages.value.filter(stage => stage.status === 'done').length
  return Math.max(1, Math.min(5, done + 1))
})

const progressPercent = computed(() => `${stageIndex.value * 20}%`)

const selectedStage = computed(() => {
  const named = stages.value.find(stage => stage.name === selectedStageName.value)
  if (named) return named
  return stages.value[Math.max(0, stageIndex.value - 1)] || stages.value[0]
})

const attempts = computed(() => {
  const latest = data.value?.latest_attempt || data.value?.attempt || 1
  return Array.from({ length: latest }, (_, index) => index + 1)
})

const totalDuration = computed(() => {
  const rootDuration = data.value?.trace?.duration_ms || 0
  const stageDuration = stages.value.reduce((sum, stage) => sum + (stage.duration_ms || 0), 0)
  return Math.max(rootDuration, stageDuration)
})

const detailEntries = computed(() => {
  const stage = selectedStage.value
  const source = stage?.output && Object.keys(stage.output).length ? stage.output : stage?.input
  if (!source) return []
  return Object.entries(source).slice(0, 16).map(([key, value]) => ({
    key,
    value: formatValue(value),
  }))
})

const stageChildren = computed(() => selectedStage.value?.children || [])

const activeError = computed(() => {
  const stage = selectedStage.value
  if (stage?.error_message) {
    return {
      code: stage.error_code || '',
      message: stage.error_message,
    }
  }
  const last = data.value?.last_error
  if (!last?.error_message) return null
  return {
    code: last.error_code || '',
    message: last.error_message,
  }
})

function formatDuration(ms?: number) {
  if (!ms || ms < 0) return '--'
  if (ms < 1000) return `${Math.round(ms)}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  const minutes = Math.floor(ms / 60000)
  const seconds = Math.round((ms % 60000) / 1000)
  return `${minutes}m ${seconds}s`
}

function formatTime(value?: string | null) {
  if (!value) return '--'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '--'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(date)
}

function formatValue(value: unknown) {
  if (value === null || value === undefined || value === '') return '--'
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  if (typeof value === 'number') return value.toLocaleString()
  if (typeof value === 'string') return value
  if (Array.isArray(value)) return value.length ? value.join(', ') : '--'
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

function stageLabel(name: string) {
  return t(`knowledgeStages.stage.${name}`)
}

function statusLabel(status: string) {
  const key = `knowledgeStages.status.${status}`
  const translated = t(key)
  return translated === key ? status : translated
}

function statusIcon(status: string) {
  if (status === 'done') return 'check'
  if (status === 'failed' || status === 'cancelled') return 'close'
  if (status === 'running') return 'loading'
  if (status === 'skipped') return 'minus'
  return 'time'
}

async function fetchProgress(manual = false) {
  if (!props.knowledgeId || fetchInFlight) return
  fetchInFlight = true
  if (!data.value) loading.value = true
  if (manual) refreshing.value = true
  try {
    const response: any = await getKnowledgeSpans(props.knowledgeId, selectedAttempt.value)
    const payload = response?.data || response
    if (payload) {
      data.value = payload
      if (selectedAttempt.value === undefined) selectedAttempt.value = payload.attempt
      if (!selectedStageName.value) {
        selectedStageName.value = STAGES[Math.max(0, stageIndex.value - 1)]
      }
    }
  } finally {
    loading.value = false
    refreshing.value = false
    fetchInFlight = false
  }
}

async function selectAttempt(attempt: number) {
  selectedAttempt.value = attempt
  selectedStageName.value = ''
  data.value = null
  await fetchProgress()
}

async function retryParse() {
  if (retrying.value) return
  retrying.value = true
  try {
    await reparseKnowledge(props.knowledgeId)
    selectedAttempt.value = undefined
    selectedStageName.value = ''
    data.value = null
    await fetchProgress()
    MessagePlugin.success(t('knowledgeBase.rebuildSubmitted'))
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('knowledgeBase.rebuildFailed'))
  } finally {
    retrying.value = false
  }
}

async function cancelParse() {
  if (cancelling.value) return
  cancelling.value = true
  try {
    await cancelKnowledgeParse(props.knowledgeId)
    await fetchProgress(true)
    MessagePlugin.success(t('knowledgeBase.cancelParseSubmitted'))
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('knowledgeBase.cancelParseFailed'))
  } finally {
    cancelling.value = false
  }
}

watch(() => props.knowledgeId, () => {
  selectedAttempt.value = undefined
  selectedStageName.value = ''
  data.value = null
  void fetchProgress()
})

onMounted(() => {
  void fetchProgress()
  pollTimer = setInterval(() => {
    if (live.value) void fetchProgress()
  }, POLL_INTERVAL)
})

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<template>
  <div class="zpp-shell">
    <header class="zpp-header">
      <div class="zpp-heading">
        <span class="zpp-eyebrow">{{ t('knowledgeStages.viewTrace') }}</span>
        <h2 :title="docTitle">{{ docTitle || t('knowledgeStages.title') }}</h2>
      </div>
      <div class="zpp-counter" :class="{ failed: data?.parse_status === 'failed' }">
        <strong>{{ stageIndex }}</strong><span>/5</span>
      </div>
      <div class="zpp-actions">
        <t-tooltip :content="t('knowledgeStages.refresh')" placement="bottom">
          <button type="button" class="zpp-icon-button" :disabled="refreshing || loading" @click="fetchProgress(true)">
            <t-icon name="refresh" :class="{ spinning: refreshing }" />
          </button>
        </t-tooltip>
        <t-popconfirm v-if="live" theme="warning"
          :content="t('knowledgeBase.cancelParseConfirmBody', { title: docTitle || knowledgeId })"
          :confirm-btn="{ content: t('knowledgeBase.cancelParse'), theme: 'danger' }"
          :cancel-btn="{ content: t('common.cancel') }" placement="bottom" @confirm="cancelParse">
          <button type="button" class="zpp-icon-button danger" :disabled="cancelling">
            <t-icon :name="cancelling ? 'loading' : 'stop-circle'" :class="{ spinning: cancelling }" />
          </button>
        </t-popconfirm>
        <button v-if="data?.parse_status === 'failed'" type="button" class="zpp-retry-button"
          :disabled="retrying" @click="retryParse">
          <t-icon :name="retrying ? 'loading' : 'refresh'" :class="{ spinning: retrying }" />
          <span>{{ t('knowledgeStages.retry') }}</span>
        </button>
        <t-tooltip v-if="showClose" :content="t('knowledgeStages.close')" placement="bottom">
          <button type="button" class="zpp-icon-button" @click="emit('close')">
            <t-icon name="close" />
          </button>
        </t-tooltip>
      </div>
    </header>

    <div class="zpp-progress-track" aria-hidden="true">
      <span class="zpp-progress-value" :style="{ width: progressPercent }" />
    </div>

    <div v-if="loading && !data" class="zpp-empty">
      <t-loading size="medium" />
    </div>
    <div v-else-if="!data" class="zpp-empty">
      <t-icon name="file-unknown" size="34px" />
      <span>{{ t('knowledgeStages.noActivity') }}</span>
    </div>

    <template v-else>
      <div class="zpp-meta-band">
        <div><span>{{ t('knowledgeStages.head.duration') }}</span><strong>{{ formatDuration(totalDuration) }}</strong></div>
        <div><span>{{ t('knowledgeStages.head.attempt') }}</span><strong>#{{ data.attempt }}</strong></div>
        <div><span>{{ t('knowledgeStages.head.stage') }}</span><strong>{{ stageIndex }}/5</strong></div>
      </div>

      <div v-if="attempts.length > 1" class="zpp-attempt-tabs" role="tablist">
        <button v-for="attempt in attempts" :key="attempt" type="button"
          :class="{ active: attempt === data.attempt }" @click="selectAttempt(attempt)">
          #{{ attempt }}
        </button>
      </div>

      <main class="zpp-main">
        <nav class="zpp-stage-list" aria-label="Parse stages">
          <button v-for="(stage, index) in stages" :key="stage.name" type="button" class="zpp-stage-row"
            :class="[stage.status, { active: selectedStage?.name === stage.name }]"
            @click="selectedStageName = stage.name">
            <span class="zpp-stage-number">{{ index + 1 }}</span>
            <span class="zpp-stage-copy">
              <strong>{{ stageLabel(stage.name) }}</strong>
              <small>{{ formatDuration(stage.duration_ms) }}</small>
            </span>
            <span class="zpp-stage-status" :title="statusLabel(stage.status)">
              <t-icon :name="statusIcon(stage.status)" :class="{ spinning: stage.status === 'running' }" />
            </span>
          </button>
        </nav>

        <section class="zpp-stage-detail">
          <div class="zpp-detail-header">
            <div>
              <span class="zpp-detail-index">{{ STAGES.indexOf(selectedStage.name as any) + 1 }}/5</span>
              <h3>{{ stageLabel(selectedStage.name) }}</h3>
            </div>
            <span class="zpp-detail-status" :class="selectedStage.status">
              {{ statusLabel(selectedStage.status) }}
            </span>
          </div>

          <div class="zpp-timing-row">
            <div><span>{{ t('knowledgeStages.detail.started') }}</span><strong>{{ formatTime(selectedStage.started_at) }}</strong></div>
            <div><span>{{ t('knowledgeStages.detail.finished') }}</span><strong>{{ formatTime(selectedStage.finished_at) }}</strong></div>
            <div><span>{{ t('knowledgeStages.detail.duration') }}</span><strong>{{ formatDuration(selectedStage.duration_ms) }}</strong></div>
          </div>

          <div v-if="activeError" class="zpp-error-band">
            <t-icon name="error-circle" />
            <div>
              <strong>{{ activeError.code || t('knowledgeStages.detail.error') }}</strong>
              <p>{{ activeError.message }}</p>
            </div>
          </div>

          <div v-if="detailEntries.length" class="zpp-detail-section">
            <h4>{{ t('knowledgeStages.detail.output') }}</h4>
            <dl class="zpp-kv-list">
              <div v-for="entry in detailEntries" :key="entry.key">
                <dt>{{ entry.key }}</dt>
                <dd :title="entry.value">{{ entry.value }}</dd>
              </div>
            </dl>
          </div>

          <div v-if="stageChildren.length" class="zpp-detail-section zpp-activity-section">
            <h4>{{ t('knowledgeStages.detail.childCount') }} · {{ stageChildren.length }}</h4>
            <div class="zpp-activity-list">
              <div v-for="child in stageChildren" :key="child.span_id || child.name" class="zpp-activity-row">
                <span class="zpp-activity-dot" :class="child.status" />
                <span class="zpp-activity-name" :title="child.name">{{ child.name }}</span>
                <span class="zpp-activity-duration">{{ formatDuration(child.duration_ms) }}</span>
                <span class="zpp-activity-status">{{ statusLabel(child.status) }}</span>
              </div>
            </div>
          </div>

          <div v-if="!detailEntries.length && !stageChildren.length && !activeError" class="zpp-detail-empty">
            <span>{{ statusLabel(selectedStage.status) }}</span>
          </div>
        </section>
      </main>
    </template>
  </div>
</template>

<style scoped lang="less">
.zpp-shell {
  width: 100%;
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  color: #17243a;
  background: #f7f9fc;
  overflow: hidden;
}

.zpp-header {
  min-height: 86px;
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 18px 24px;
  background: #ffffff;
  border-bottom: 1px solid #dce4ef;
  box-sizing: border-box;
}

.zpp-heading {
  flex: 1;
  min-width: 0;
}

.zpp-eyebrow {
  display: block;
  margin-bottom: 5px;
  color: #4f6f99;
  font-family: var(--app-font-family-mono);
  font-size: 10px;
  font-weight: 700;
  line-height: 1;
}

.zpp-heading h2 {
  margin: 0;
  overflow: hidden;
  color: #13213a;
  font-size: 17px;
  font-weight: 650;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.zpp-counter {
  display: flex;
  align-items: baseline;
  min-width: 78px;
  color: #1264d5;
  font-family: var(--app-font-family-mono);
}

.zpp-counter strong {
  font-size: 30px;
  line-height: 1;
}

.zpp-counter span {
  font-size: 16px;
  font-weight: 700;
}

.zpp-counter.failed { color: #c43a31; }

.zpp-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.zpp-icon-button,
.zpp-retry-button {
  height: 32px;
  border: 1px solid #d5dfeb;
  background: #ffffff;
  color: #4e617b;
  cursor: pointer;
}

.zpp-icon-button {
  width: 32px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
}

.zpp-icon-button:hover,
.zpp-retry-button:hover { border-color: #1264d5; color: #1264d5; }
.zpp-icon-button.danger:hover { border-color: #c43a31; color: #c43a31; }
.zpp-icon-button:disabled,
.zpp-retry-button:disabled { cursor: not-allowed; opacity: 0.45; }

.zpp-retry-button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 0 11px;
  font-size: 12px;
}

.spinning { animation: zpp-spin 0.9s linear infinite; }
@keyframes zpp-spin { to { transform: rotate(360deg); } }

.zpp-progress-track {
  position: relative;
  flex: 0 0 3px;
  background: #dce5f0;
}

.zpp-progress-value {
  position: absolute;
  inset: 0 auto 0 0;
  background: #1264d5;
  transition: width 220ms ease;
}

.zpp-meta-band {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  background: #ffffff;
  border-bottom: 1px solid #dce4ef;
}

.zpp-meta-band > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-width: 0;
  padding: 12px 20px;
  border-right: 1px solid #e6ebf2;
}

.zpp-meta-band > div:last-child { border-right: 0; }
.zpp-meta-band span { color: #718096; font-size: 11px; }
.zpp-meta-band strong { color: #20324d; font-family: var(--app-font-family-mono); font-size: 12px; }

.zpp-attempt-tabs {
  display: flex;
  gap: 0;
  padding: 0 24px;
  background: #ffffff;
  border-bottom: 1px solid #dce4ef;
  overflow-x: auto;
}

.zpp-attempt-tabs button {
  min-width: 54px;
  padding: 10px 14px 9px;
  border: 0;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: #718096;
  font-family: var(--app-font-family-mono);
  font-size: 11px;
  cursor: pointer;
}

.zpp-attempt-tabs button.active { border-bottom-color: #1264d5; color: #1264d5; font-weight: 700; }

.zpp-main {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: 268px minmax(0, 1fr);
  overflow: hidden;
}

.zpp-stage-list {
  overflow-y: auto;
  background: #eef3f9;
  border-right: 1px solid #d5dfeb;
}

.zpp-stage-row {
  width: 100%;
  min-height: 78px;
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr) 28px;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  border: 0;
  border-bottom: 1px solid #dce4ef;
  background: transparent;
  color: #52647d;
  text-align: left;
  cursor: pointer;
}

.zpp-stage-row:hover { background: #e5edf7; }
.zpp-stage-row.active { background: #ffffff; box-shadow: inset 3px 0 0 #1264d5; }

.zpp-stage-number {
  width: 30px;
  height: 30px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #b8c6d8;
  color: #61738b;
  font-family: var(--app-font-family-mono);
  font-size: 12px;
  font-weight: 700;
}

.zpp-stage-row.done .zpp-stage-number { border-color: #2a8f68; background: #e9f6f0; color: #237b5b; }
.zpp-stage-row.running .zpp-stage-number,
.zpp-stage-row.active .zpp-stage-number { border-color: #1264d5; background: #e8f1fd; color: #1264d5; }
.zpp-stage-row.failed .zpp-stage-number { border-color: #c43a31; background: #fcecea; color: #c43a31; }

.zpp-stage-copy { min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.zpp-stage-copy strong { overflow: hidden; color: #243651; font-size: 13px; font-weight: 650; text-overflow: ellipsis; white-space: nowrap; }
.zpp-stage-copy small { color: #7a8ba2; font-family: var(--app-font-family-mono); font-size: 10px; }

.zpp-stage-status {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #8797ab;
}
.zpp-stage-row.done .zpp-stage-status { color: #237b5b; }
.zpp-stage-row.running .zpp-stage-status { color: #1264d5; }
.zpp-stage-row.failed .zpp-stage-status { color: #c43a31; }

.zpp-stage-detail {
  min-width: 0;
  overflow-y: auto;
  padding: 24px 28px 32px;
  background: #ffffff;
}

.zpp-detail-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 18px;
  border-bottom: 1px solid #e3e9f1;
}

.zpp-detail-index { color: #1264d5; font-family: var(--app-font-family-mono); font-size: 11px; font-weight: 700; }
.zpp-detail-header h3 { margin: 5px 0 0; color: #17243a; font-size: 21px; font-weight: 650; }

.zpp-detail-status {
  padding: 5px 9px;
  border: 1px solid #cdd8e5;
  color: #687a92;
  font-size: 11px;
}
.zpp-detail-status.done { border-color: #9dcdb9; color: #237b5b; }
.zpp-detail-status.running { border-color: #8fb7eb; color: #1264d5; }
.zpp-detail-status.failed { border-color: #e4aaa5; color: #c43a31; }

.zpp-timing-row {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border-bottom: 1px solid #e3e9f1;
}

.zpp-timing-row > div { min-width: 0; padding: 16px 16px 16px 0; }
.zpp-timing-row span { display: block; margin-bottom: 6px; color: #7a8ba2; font-size: 10px; }
.zpp-timing-row strong { display: block; overflow: hidden; color: #263952; font-family: var(--app-font-family-mono); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }

.zpp-error-band {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  margin: 18px 0 0;
  padding: 13px 15px;
  border-left: 3px solid #c43a31;
  background: #fff5f3;
  color: #9f312a;
}
.zpp-error-band strong { font-size: 12px; }
.zpp-error-band p { margin: 4px 0 0; color: #7e3b36; font-family: var(--app-font-family-mono); font-size: 11px; line-height: 1.55; word-break: break-word; }

.zpp-detail-section { padding-top: 22px; }
.zpp-detail-section h4 { margin: 0 0 12px; color: #50627a; font-size: 11px; font-weight: 700; }

.zpp-kv-list { margin: 0; border-top: 1px solid #e3e9f1; }
.zpp-kv-list > div { display: grid; grid-template-columns: minmax(130px, 34%) minmax(0, 1fr); gap: 18px; padding: 11px 0; border-bottom: 1px solid #edf1f5; }
.zpp-kv-list dt { color: #718096; font-family: var(--app-font-family-mono); font-size: 10px; word-break: break-word; }
.zpp-kv-list dd { margin: 0; overflow: hidden; color: #263952; font-size: 12px; line-height: 1.5; text-overflow: ellipsis; white-space: nowrap; }

.zpp-activity-list { border-top: 1px solid #e3e9f1; }
.zpp-activity-row { min-height: 38px; display: grid; grid-template-columns: 10px minmax(0, 1fr) 70px 64px; align-items: center; gap: 10px; border-bottom: 1px solid #edf1f5; font-size: 11px; }
.zpp-activity-dot { width: 7px; height: 7px; border-radius: 50%; background: #a8b4c3; }
.zpp-activity-dot.done { background: #2a8f68; }
.zpp-activity-dot.running { background: #1264d5; }
.zpp-activity-dot.failed { background: #c43a31; }
.zpp-activity-name { overflow: hidden; color: #31445f; font-family: var(--app-font-family-mono); text-overflow: ellipsis; white-space: nowrap; }
.zpp-activity-duration { color: #718096; font-family: var(--app-font-family-mono); text-align: right; }
.zpp-activity-status { color: #718096; text-align: right; }

.zpp-detail-empty,
.zpp-empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: #8190a4;
  font-size: 12px;
}

.zpp-detail-empty { min-height: 220px; border-bottom: 1px solid #edf1f5; }

@media (max-width: 760px) {
  .zpp-header { min-height: 86px; padding: 14px 16px; gap: 9px; }
  .zpp-heading h2 { max-width: 170px; }
  .zpp-counter { min-width: 56px; }
  .zpp-counter strong { font-size: 24px; }
  .zpp-main { display: flex; flex-direction: column; }
  .zpp-stage-list {
    min-height: 84px;
    flex: 0 0 84px;
    display: flex;
    overflow: hidden;
    border-right: 0;
    border-bottom: 1px solid #d5dfeb;
  }
  .zpp-stage-row {
    width: auto;
    min-width: 0;
    flex: 1 1 20%;
    min-height: 83px;
    padding: 7px;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
    border-right: 1px solid #dce4ef;
    border-bottom: 0;
  }
  .zpp-stage-row.active { box-shadow: inset 0 -3px 0 #1264d5; }
  .zpp-stage-status { display: none; }
  .zpp-stage-number { width: 24px; height: 24px; }
  .zpp-stage-copy { width: 100%; gap: 1px; }
  .zpp-stage-copy strong {
    width: 100%;
    font-size: 10px;
    line-height: 1.3;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .zpp-stage-copy small { font-size: 9px; }
  .zpp-stage-detail { flex: 1; min-height: 0; padding: 18px 16px 28px; }
  .zpp-meta-band > div { padding: 10px 12px; }
  .zpp-timing-row > div { padding: 13px 8px 13px 0; }
  .zpp-kv-list > div { grid-template-columns: minmax(92px, 34%) minmax(0, 1fr); gap: 10px; }
  .zpp-activity-row { grid-template-columns: 8px minmax(0, 1fr) 48px 44px; gap: 7px; }
}
</style>
