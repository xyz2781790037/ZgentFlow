<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="dialogVisible" class="upload-confirm-overlay">
        <div class="upload-confirm-modal" role="dialog" :aria-label="t('uploadConfirm.title')">
          <button class="close-btn" type="button" :aria-label="t('general.close')" @click="handleCancel">
            <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
              <path d="M15 5L5 15M5 5L15 15" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
            </svg>
          </button>

          <div class="upload-confirm-container">
            <aside class="files-panel">
              <div class="files-panel-header">
                <h2 class="files-panel-title">{{ sourcePanelTitle }}</h2>
                <div v-if="mode === 'file'" class="files-panel-actions">
                  <span class="files-count">{{ batchItemCount }}</span>
                  <KbUploadSourceDropdown
                    :accept-file-types="acceptFileTypes"
                    :supported-file-types="supportedFileTypes"
                    :tooltip="t('uploadConfirm.continueAdd')"
                    placement="bottom-left"
                    @files="appendFiles"
                  />
                </div>
              </div>
              <div v-if="mode === 'reparse' && reparsePreview" class="reparse-source-panel">
                <p class="reparse-source-title" :title="reparsePreview.fileName">
                  {{ reparsePreview.fileName || t('uploadConfirm.reparseSource') }}
                </p>
                <p class="reparse-source-meta">{{ t('uploadConfirm.reparseHint') }}</p>
              </div>
              <ul v-else-if="mode === 'file' && batchItemCount > 0" class="files-list">
                <li v-for="(file, index) in localFiles" :key="`${file.name}-${index}`" class="file-item">
                  <t-icon :name="getFileIcon(file.name)" class="file-icon" />
                  <div class="file-meta">
                    <span class="file-name" :title="file.name">{{ file.name }}</span>
                    <span class="file-size">{{ formatFileSize(file.size) }}</span>
                  </div>
                  <t-button
                    theme="default"
                    variant="text"
                    size="small"
                    shape="square"
                    :aria-label="t('common.remove')"
                    @click="removeFile(index)"
                  >
                    <t-icon name="close" />
                  </t-button>
                </li>
              </ul>
              <div v-else-if="mode === 'file'" class="files-empty">{{ t('uploadConfirm.noItems') }}</div>
            </aside>

            <main class="main-panel">
              <header class="main-header">
                <template v-if="activeSection === 'overview'">
                  <h2 class="main-title">{{ dialogTitle }}</h2>
                  <p class="main-desc">{{ dialogDesc }}</p>
                </template>
                <template v-else>
                  <button
                    type="button"
                    class="back-link"
                    @click="activeSection = 'overview'"
                  >
                    <t-icon name="chevron-left" />
                    <span>{{ t('uploadConfirm.backToOverview') }}</span>
                  </button>
                  <h2 class="edit-title">{{ currentSectionTitle }}</h2>
                  <p v-if="currentSectionDesc" class="edit-desc">{{ currentSectionDesc }}</p>
                </template>
              </header>

              <div class="main-body">
                <div v-if="activeSection === 'overview'" class="overview-list">
                    <button
                      v-for="line in overviewLines"
                      :key="line.key"
                      type="button"
                      class="overview-row"
                      :class="{ 'overview-row--issue': issueSectionKeys.has(line.key) }"
                      @click="goToSection(line.key)"
                    >
                      <span class="overview-label">{{ line.title }}</span>
                      <span
                        class="overview-value"
                        :class="{ 'overview-value--issue': issueSectionKeys.has(line.key) }"
                        :title="line.value"
                      >{{ line.value }}</span>
                      <t-icon name="chevron-right" class="overview-chevron" />
                    </button>
                </div>

                <div v-else class="edit-section edit-section--embedded">
                  <div v-show="activeSection === 'parser'" class="section">
                    <KBParserSettings
                      embedded
                      :relevant-extensions="batchFileExts"
                      :parser-engine-rules="uiState.chunkingConfig.parserEngineRules"
                      @update:parser-engine-rules="handleParserEngineRulesUpdate"
                    />
                  </div>
                  <div v-show="activeSection === 'chunking'" class="section">
                    <KBChunkingSettings
                      embedded
                      :config="uiState.chunkingConfig"
                      @update:config="handleChunkingConfigUpdate"
                    />
                  </div>
                  <div v-show="activeSection === 'multimodal'" class="section">
                    <div class="kb-embedded-settings">
                      <div class="setting-row setting-row--toggle">
                        <div class="setting-info">
                          <label>{{ t('knowledgeEditor.advanced.multimodal.label') }}</label>
                          <p class="desc">{{ t('knowledgeEditor.advanced.multimodal.description') }}</p>
                        </div>
                        <div class="setting-control">
                          <t-switch
                            v-model="uiState.multimodalConfig.enabled"
                            size="medium"
                            @change="handleMultimodalToggle"
                          />
                        </div>
                      </div>
                      <div v-if="uiState.multimodalConfig.enabled" class="setting-row setting-row--field">
                        <div class="setting-info">
                          <label>
                            {{ t('knowledgeEditor.advanced.multimodal.vllmLabel') }}
                            <span class="required">*</span>
                          </label>
                        </div>
                        <div class="setting-control setting-control--full">
                          <strong>{{ getModelName(uiState.multimodalConfig.vllmModelId) }}</strong>
                          <t-button variant="text" size="small" @click="handleAddVLLMModel">管理模型</t-button>
                          <p v-if="showMultimodalModelError" class="field-error">
                            {{ t('uploadConfirm.vlmModelSelectRequired') }}
                          </p>
                        </div>
                      </div>
                    </div>
                  </div>
                  <div v-show="activeSection === 'question'" class="section">
                    <KBAdvancedSettings
                      embedded
                      :question-generation="uiState.questionGenerationConfig"
                      :rag-enabled="ragEnabled"
                      @update:question-generation="handleQuestionGenerationUpdate"
                    />
                  </div>
                </div>
              </div>
            </main>
          </div>

          <footer class="modal-footer">
            <t-button theme="default" variant="outline" @click="handleCancel">
              {{ t('uploadConfirm.cancel') }}
            </t-button>
            <t-button theme="primary" :disabled="!canConfirm" @click="handleConfirm">
              {{ confirmButtonText }}
            </t-button>
          </footer>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import KBParserSettings from '../settings/KBParserSettings.vue'
import KBChunkingSettings from '../settings/KBChunkingSettings.vue'
import KBAdvancedSettings from '../settings/KBAdvancedSettings.vue'
import { useChatResourcesStore } from '@/stores/chatResources'
import { useUIStore } from '@/stores/ui'
import { formatFileSize, getFileIcon } from '@/utils/files'
import { getUploadFileKey } from '../utils/uploadSources'
import KbUploadSourceDropdown from './KbUploadSourceDropdown.vue'
import type { KnowledgeProcessOverrides } from '@/types/knowledgeProcess'
import type {
  UploadConfirmMode,
  UploadConfirmReparseSource,
  UploadConfirmResult,
} from '@/stores/uploadConfirm'

const IMAGE_EXTENSIONS = ['jpg', 'jpeg', 'png', 'gif', 'bmp', 'webp']

interface ChunkingUIConfig {
  chunkSize: number
  chunkOverlap: number
  separators: string[]
  parserEngineRules?: Array<{ file_types: string[]; engine: string }>
  enableParentChild: boolean
  parentChunkSize: number
  childChunkSize: number
  strategy?: string
  tokenLimit?: number
  languages?: string[]
}

interface UploadUIState {
  chunkingConfig: ChunkingUIConfig
  multimodalConfig: { enabled: boolean; vllmModelId: string }
  questionGenerationConfig: { enabled: boolean; questionCount: number }
}

const props = withDefaults(defineProps<{
  visible: boolean
  kbInfo: any
  mode?: UploadConfirmMode
  files?: File[]
  reparsePreview?: UploadConfirmReparseSource | null
  acceptFileTypes?: string
  supportedFileTypes?: string[]
}>(), {
  mode: 'file',
  files: () => [],
  reparsePreview: null,
  acceptFileTypes: '',
  supportedFileTypes: () => [],
})

const emit = defineEmits<{
  'update:visible': [value: boolean]
  confirm: [payload: UploadConfirmResult]
  cancel: []
}>()

const { t } = useI18n()
const chatResources = useChatResourcesStore()
const uiStore = useUIStore()

const allModels = ref<any[]>([])
const localFiles = ref<File[]>([])
const activeSection = ref('overview')
const uiState = ref<UploadUIState>(createDefaultUIState())

const dialogVisible = computed({
  get: () => props.visible,
  set: (value: boolean) => emit('update:visible', value),
})

function getEngineDisplayName(engineName: string): string {
  const key = `kbSettings.parser.engines.${engineName}.name`
  const translated = t(key)
  return translated !== key ? translated : engineName
}

function getStrategyLabel(strategy?: string): string {
  if (!strategy) return t('uploadConfirm.summaryStrategyDefault')
  const key = `knowledgeEditor.chunking.strategies.${strategy}.label`
  const translated = t(key)
  return translated !== key ? translated : strategy
}

function getModelName(modelId: string): string {
  if (!modelId) return t('uploadConfirm.notSet')
  const model = allModels.value.find((m: any) => m.id === modelId)
  return model?.name || modelId
}

function getFileExt(file: File): string {
  const dot = file.name.lastIndexOf('.')
  if (dot < 0) return ''
  return file.name.substring(dot + 1).toLowerCase()
}

const batchItemCount = computed(() => localFiles.value.length)

const sourcePanelTitle = computed(() => {
  if (props.mode === 'reparse') return t('uploadConfirm.reparseSource')
  return t('uploadConfirm.fileList')
})

const dialogTitle = computed(() => {
  if (props.mode === 'reparse') return t('uploadConfirm.titleReparse')
  return t('uploadConfirm.title')
})

const dialogDesc = computed(() => {
  if (props.mode === 'reparse') return t('uploadConfirm.overviewDescReparse')
  return t('uploadConfirm.overviewDesc')
})

const confirmButtonText = computed(() => {
  if (props.mode === 'reparse') return t('uploadConfirm.confirmReparse')
  return t('uploadConfirm.confirm')
})

const batchFileExts = computed(() => {
  const set = new Set<string>()
  if (props.mode === 'reparse') {
    const ext = (props.reparsePreview?.fileType || '').toLowerCase()
    if (ext) set.add(ext)
  }
  for (const file of localFiles.value) {
    const ext = getFileExt(file)
    if (ext) set.add(ext)
  }
  return [...set]
})

function resolveEngineForExt(ext: string): string {
  const rules = uiState.value.chunkingConfig.parserEngineRules
  if (rules?.length) {
    for (const rule of rules) {
      if (rule.file_types.includes(ext)) {
        return getEngineDisplayName(rule.engine)
      }
    }
  }
  return t('uploadConfirm.summaryParserBuiltin')
}

const parserOverviewValue = computed(() => {
  const exts = batchFileExts.value
  if (!exts.length) return t('uploadConfirm.summaryParserBuiltin')
  return exts.map(ext => `.${ext} → ${resolveEngineForExt(ext)}`).join(' · ')
})

const chunkingOverviewValue = computed(() => {
  const c = uiState.value.chunkingConfig
  const parts = [
    t('uploadConfirm.navChunkingSummary', { size: c.chunkSize }),
    t('uploadConfirm.summaryChunkOverlapShort', { overlap: c.chunkOverlap }),
    getStrategyLabel(c.strategy),
  ]
  parts.push(
    c.enableParentChild
      ? t('uploadConfirm.summaryParentChildShort')
      : t('uploadConfirm.summaryParentChildOff'),
  )
  return parts.join(' · ')
})

const overviewLines = computed(() => {
  const mm = uiState.value.multimodalConfig
  const qg = uiState.value.questionGenerationConfig

  return [
    { key: 'parser', title: t('uploadConfirm.tabParser'), value: parserOverviewValue.value },
    { key: 'chunking', title: t('uploadConfirm.tabChunking'), value: chunkingOverviewValue.value },
    {
      key: 'multimodal',
      title: t('uploadConfirm.tabMultimodal'),
      value: mm.enabled
        ? `${t('uploadConfirm.statusOn')} · ${mm.vllmModelId ? getModelName(mm.vllmModelId) : t('uploadConfirm.notSet')}`
        : (hasImages.value ? t('uploadConfirm.multimodalRequiredForImages') : t('uploadConfirm.statusOff')),
    },
    {
      key: 'question',
      title: t('uploadConfirm.tabQuestion'),
      value: qg.enabled
        ? t('uploadConfirm.summaryQuestionCountValue', { count: qg.questionCount })
        : t('uploadConfirm.statusOff'),
    },
  ]
})

const sectionMeta: Record<string, { titleKey: string; descKey?: string }> = {
  parser: { titleKey: 'uploadConfirm.tabParser', descKey: 'kbSettings.parser.description' },
  chunking: { titleKey: 'uploadConfirm.tabChunking', descKey: 'knowledgeEditor.chunking.description' },
  multimodal: { titleKey: 'uploadConfirm.tabMultimodal', descKey: 'knowledgeEditor.multimodal.description' },
  question: { titleKey: 'uploadConfirm.tabQuestion', descKey: 'knowledgeEditor.advanced.questionGeneration.description' },
}

const currentSectionTitle = computed(() => {
  const meta = sectionMeta[activeSection.value]
  return meta ? t(meta.titleKey) : ''
})

const currentSectionDesc = computed(() => {
  const meta = sectionMeta[activeSection.value]
  return meta?.descKey ? t(meta.descKey) : ''
})

const goToSection = (key: string) => {
  activeSection.value = key
}

const ragEnabled = computed(() => {
  const strategy = props.kbInfo?.indexing_strategy
  return (strategy?.vector_enabled ?? true) || (strategy?.keyword_enabled ?? true)
})

const hasImages = computed(() => {
  return batchFileExts.value.some(ext => IMAGE_EXTENSIONS.includes(ext))
})

const showMultimodalModelError = computed(() => {
  return uiState.value.multimodalConfig.enabled && !uiState.value.multimodalConfig.vllmModelId
})

const issueSectionKeys = computed(() => {
  const keys = new Set<string>()
  if (hasImages.value) {
    if (!uiState.value.multimodalConfig.enabled || !uiState.value.multimodalConfig.vllmModelId) {
      keys.add('multimodal')
    }
  } else if (showMultimodalModelError.value) {
    keys.add('multimodal')
  }
  return keys
})

const canConfirm = computed(() => {
  if (props.mode === 'file' && batchItemCount.value === 0) return false
  if (hasImages.value) {
    if (!uiState.value.multimodalConfig.enabled || !uiState.value.multimodalConfig.vllmModelId) {
      return false
    }
  }
  if (showMultimodalModelError.value) {
    return false
  }
  return true
})

function createDefaultUIState(): UploadUIState {
  return {
    chunkingConfig: {
      chunkSize: 512,
      chunkOverlap: 80,
      separators: ['\n\n', '\n', '。', '！', '？', ';', '；'],
      parserEngineRules: undefined,
      enableParentChild: true,
      parentChunkSize: 4096,
      childChunkSize: 384,
      strategy: 'auto',
      tokenLimit: 0,
      languages: [],
    },
    multimodalConfig: { enabled: false, vllmModelId: '' },
    questionGenerationConfig: { enabled: true, questionCount: 3 },
  }
}

function initFromKbInfo(kb: any) {
  if (!kb) {
    uiState.value = createDefaultUIState()
    return
  }

  uiState.value = {
    chunkingConfig: {
      chunkSize: kb.chunking_config?.chunk_size || 512,
      chunkOverlap: kb.chunking_config?.chunk_overlap || 80,
      separators: kb.chunking_config?.separators || ['\n\n', '\n', '。', '！', '？', ';', '；'],
      parserEngineRules: kb.chunking_config?.parser_engine_rules || undefined,
      enableParentChild: kb.chunking_config?.enable_parent_child ?? false,
      parentChunkSize: kb.chunking_config?.parent_chunk_size || 4096,
      childChunkSize: kb.chunking_config?.child_chunk_size || 384,
      strategy: kb.chunking_config?.strategy || 'auto',
      tokenLimit: kb.chunking_config?.token_limit || 0,
      languages: kb.chunking_config?.languages || [],
    },
    multimodalConfig: {
      enabled: !!kb.vlm_config?.enabled,
      vllmModelId: kb.vlm_config?.model_id || '',
    },
    questionGenerationConfig: {
      enabled: kb.question_generation_config?.enabled ?? true,
      questionCount: kb.question_generation_config?.question_count || 3,
    },
  }
}

function buildProcessOverrides(): KnowledgeProcessOverrides {
  const state = uiState.value
  const chunking = state.chunkingConfig

  return {
    parser_engine_rules: chunking.parserEngineRules,
    chunking_config: {
      chunk_size: chunking.chunkSize,
      chunk_overlap: chunking.chunkOverlap,
      separators: chunking.separators,
      enable_parent_child: chunking.enableParentChild,
      parent_chunk_size: chunking.parentChunkSize,
      child_chunk_size: chunking.childChunkSize,
      strategy: chunking.strategy,
      token_limit: chunking.tokenLimit,
      languages: chunking.languages,
    },
    enable_multimodel: state.multimodalConfig.enabled,
    vlm_config: {
      enabled: state.multimodalConfig.enabled,
      model_id: state.multimodalConfig.vllmModelId,
    },
    question_generation_config: {
      enabled: state.questionGenerationConfig.enabled,
      question_count: state.questionGenerationConfig.questionCount,
    },
  }
}

function applyOverridesToState(o?: KnowledgeProcessOverrides | null) {
  if (!o) return
  const s = uiState.value
  const cc = o.chunking_config
  if (cc) {
    if (cc.chunk_size != null) s.chunkingConfig.chunkSize = cc.chunk_size
    if (cc.chunk_overlap != null) s.chunkingConfig.chunkOverlap = cc.chunk_overlap
    if (cc.separators) s.chunkingConfig.separators = cc.separators
    if (cc.enable_parent_child != null) s.chunkingConfig.enableParentChild = cc.enable_parent_child
    if (cc.parent_chunk_size != null) s.chunkingConfig.parentChunkSize = cc.parent_chunk_size
    if (cc.child_chunk_size != null) s.chunkingConfig.childChunkSize = cc.child_chunk_size
    if (cc.strategy != null) s.chunkingConfig.strategy = cc.strategy
    if (cc.token_limit != null) s.chunkingConfig.tokenLimit = cc.token_limit
    if (cc.languages) s.chunkingConfig.languages = cc.languages
    if (cc.parser_engine_rules) s.chunkingConfig.parserEngineRules = cc.parser_engine_rules
  }
  if (o.parser_engine_rules) s.chunkingConfig.parserEngineRules = o.parser_engine_rules
  if (o.enable_multimodel != null) s.multimodalConfig.enabled = o.enable_multimodel
  if (o.vlm_config) {
    if (o.vlm_config.enabled != null) s.multimodalConfig.enabled = o.vlm_config.enabled
    if (o.vlm_config.model_id != null) s.multimodalConfig.vllmModelId = o.vlm_config.model_id
  }
  const qg = o.question_generation_config
  if (qg) {
    if (qg.enabled != null) s.questionGenerationConfig.enabled = qg.enabled
    if (qg.question_count != null) s.questionGenerationConfig.questionCount = qg.question_count
  }
}

async function loadModels() {
  try {
    await chatResources.ensureModels()
    allModels.value = chatResources.allModels || []
    if (uiState.value.multimodalConfig.enabled) {
      uiState.value.multimodalConfig.vllmModelId =
        allModels.value.find((model: any) => model.type === 'VLLM' && model.is_default)?.id || ''
    }
  } catch {
    allModels.value = []
  }
}

watch(
  () => props.visible,
  (visible) => {
    if (!visible) return
    localFiles.value = props.mode === 'file' ? [...(props.files || [])] : []
    initFromKbInfo(props.kbInfo)
    if (props.mode === 'reparse') {
      applyOverridesToState(props.reparsePreview?.processOverrides)
    }
    activeSection.value = 'overview'
    loadModels()
  },
)

const appendFiles = (incoming: File[]) => {
  const existingKeys = new Set(localFiles.value.map(getUploadFileKey))
  const toAdd: File[] = []
  let duplicateCount = 0

  for (const file of incoming) {
    const key = getUploadFileKey(file)
    if (existingKeys.has(key)) {
      duplicateCount++
      continue
    }
    existingKeys.add(key)
    toAdd.push(file)
  }

  if (toAdd.length > 0) {
    localFiles.value = [...localFiles.value, ...toAdd]
    MessagePlugin.success(t('uploadConfirm.filesAdded', { count: toAdd.length }))
  } else if (duplicateCount > 0) {
    MessagePlugin.warning(t('uploadConfirm.filesAllDuplicate'))
  }
}

const removeFile = (index: number) => {
  localFiles.value = localFiles.value.filter((_, i) => i !== index)
}

const handleParserEngineRulesUpdate = (rules: Array<{ file_types: string[]; engine: string }>) => {
  uiState.value.chunkingConfig.parserEngineRules = rules
}

const handleChunkingConfigUpdate = (config: ChunkingUIConfig) => {
  uiState.value.chunkingConfig = { ...config }
}

const handleMultimodalToggle = () => {
  uiState.value.multimodalConfig.vllmModelId = uiState.value.multimodalConfig.enabled
    ? (allModels.value.find((model: any) => model.type === 'VLLM' && model.is_default)?.id || '')
    : ''
}

const handleAddVLLMModel = () => {
  uiStore.openSettings('models', 'vllm')
}

const handleQuestionGenerationUpdate = (config: { enabled: boolean; questionCount: number }) => {
  uiState.value.questionGenerationConfig = { ...config }
}

const validateBeforeConfirm = (): boolean => {
  if (hasImages.value) {
    if (!uiState.value.multimodalConfig.enabled || !uiState.value.multimodalConfig.vllmModelId) {
      MessagePlugin.warning(t('uploadConfirm.vlmModelRequired'))
      activeSection.value = 'multimodal'
      return false
    }
  } else if (showMultimodalModelError.value) {
    MessagePlugin.warning(t('uploadConfirm.vlmModelSelectRequired'))
    activeSection.value = 'multimodal'
    return false
  }

  return true
}

const handleCancel = () => {
  emit('cancel')
  emit('update:visible', false)
}

const handleConfirm = () => {
  if (props.mode === 'file' && batchItemCount.value === 0) {
    MessagePlugin.warning(t('uploadConfirm.noItems'))
    return
  }
  if (!validateBeforeConfirm()) return

  const processConfig = buildProcessOverrides()
  if (props.mode === 'reparse' && props.reparsePreview) {
    emit('confirm', { processConfig, mode: 'reparse', reparse: { ...props.reparsePreview } })
  } else {
    emit('confirm', {
      processConfig,
      mode: 'file',
      files: [...localFiles.value],
    })
  }
  emit('update:visible', false)
}
</script>

<style lang="less" scoped>
.upload-confirm-overlay {
  position: fixed;
  inset: 0;
  z-index: 2500;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(4px);
}

.upload-confirm-modal {
  position: relative;
  display: flex;
  flex-direction: column;
  width: min(880px, 92vw);
  height: min(640px, 86vh);
  overflow: hidden;
  border: 1px solid var(--zeal-line-strong, #c6d1df);
  border-radius: 8px;
  background: var(--zeal-surface, #fff);
  box-shadow: var(--zeal-shadow-md, 0 18px 48px rgba(16, 26, 41, 0.14));
}

.close-btn {
  position: absolute;
  top: 16px;
  right: 16px;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  border-radius: 6px;
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-secondary);
  cursor: pointer;

  &:hover {
    color: var(--td-text-color-primary);
  }
}

.upload-confirm-container {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.files-panel {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  width: 220px;
  border-right: 1px solid var(--zeal-line, #dbe3ed);
  background: var(--zeal-surface-subtle, #f8fafc);
}

.files-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 20px 16px 12px;
}

.files-panel-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.files-panel-title {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.files-count {
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  border-radius: 10px;
  font-size: 11px;
  font-weight: 600;
  line-height: 20px;
  text-align: center;
  background: var(--td-bg-color-component);
  color: var(--td-text-color-secondary);
}

.files-list {
  flex: 1;
  margin: 0;
  padding: 4px 8px 12px;
  overflow-y: auto;
  list-style: none;
}

.file-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  border-radius: 6px;

  &:hover {
    background: var(--td-bg-color-container-hover);
  }
}

.file-icon {
  flex-shrink: 0;
  font-size: 18px;
  color: var(--td-brand-color);
}

.file-meta {
  flex: 1;
  min-width: 0;
}

.file-name {
  display: block;
  overflow: hidden;
  font-size: 13px;
  color: var(--td-text-color-primary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-size {
  display: block;
  margin-top: 2px;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.files-empty {
  flex: 1;
  padding: 24px 16px;
  font-size: 13px;
  color: var(--td-text-color-secondary);
  text-align: center;
}

.reparse-source-panel {
  flex: 1;
  min-height: 0;
  padding: 8px 16px 12px;
  overflow-y: auto;
}

.reparse-source-title {
  margin: 0 0 6px;
  font-size: 14px;
  font-weight: 500;
  color: var(--td-text-color-primary);
  word-break: break-word;
}

.reparse-source-meta {
  margin: 0;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.main-panel {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
}

.main-header {
  flex-shrink: 0;
  padding: 20px 48px 0 24px;
}

.main-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.main-desc {
  margin: 6px 0 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--td-text-color-placeholder);
}

.back-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin: 0 0 10px;
  padding: 0;
  border: none;
  background: none;
  font-size: 13px;
  color: var(--td-brand-color);
  cursor: pointer;

  &:hover {
    opacity: 0.85;
  }
}

.edit-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.edit-desc {
  margin: 6px 0 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--td-text-color-placeholder);
}

.main-body {
  flex: 1;
  min-height: 0;
  padding: 16px 24px 20px;
  overflow-y: auto;
}

.overview-list {
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  overflow: hidden;
}

.overview-row {
  display: grid;
  grid-template-columns: 108px 1fr 20px;
  gap: 12px;
  align-items: center;
  width: 100%;
  margin: 0;
  padding: 14px 16px;
  border: none;
  border-bottom: 1px solid var(--td-component-stroke);
  background: var(--td-bg-color-container);
  text-align: left;
  cursor: pointer;
  transition: background 0.15s ease;

  &:last-child {
    border-bottom: none;
  }

  &:hover {
    background: var(--td-bg-color-container-hover);
  }

  &--issue {
    background: var(--td-error-color-1);

    &:hover {
      background: var(--td-error-color-2);
    }
  }
}

.overview-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-color-secondary);
}

.overview-value {
  overflow: hidden;
  font-size: 13px;
  color: var(--td-text-color-primary);
  text-overflow: ellipsis;
  white-space: nowrap;

  &--issue {
    color: var(--td-error-color);
  }
}

.overview-chevron {
  font-size: 16px;
  color: var(--td-text-color-placeholder);
}

.edit-section {
  width: 100%;
}

.section {
  width: 100%;
}

.kb-embedded-settings {
  .setting-row {
    padding: 12px 0;
    border-bottom: 1px solid var(--td-component-stroke);

    &:last-child {
      border-bottom: none;
    }
  }

  .setting-row--toggle {
    display: flex;
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
    gap: 16px;

    .setting-info {
      flex: 1;
      min-width: 0;
    }

    .setting-control {
      flex: none;
      flex-shrink: 0;
    }
  }

  .setting-row--field {
    display: flex;
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }

  .setting-info {
    label {
      font-size: 14px;
      font-weight: 500;
      color: var(--td-text-color-primary);
    }

    .desc {
      margin: 4px 0 0;
      font-size: 12px;
      line-height: 1.5;
      color: var(--td-text-color-secondary);
    }
  }

  .setting-control--full {
    width: 100%;
  }

  .required {
    color: var(--td-error-color);
  }

  .field-error {
    margin: 6px 0 0;
    font-size: 12px;
    line-height: 1.4;
    color: var(--td-error-color);
  }
}

.edit-section--embedded {
  :deep(.setting-row) {
    &:last-child {
      border-bottom: none;
    }
  }
}

.modal-footer {
  display: flex;
  flex-shrink: 0;
  justify-content: flex-end;
  gap: 12px;
  padding: 14px 20px;
  border-top: 1px solid var(--td-component-stroke);
}

.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

@media (max-width: 680px) {
  .upload-confirm-overlay {
    align-items: stretch;
    background: var(--zeal-canvas, #f3f6fa);
    backdrop-filter: none;
  }
  .upload-confirm-modal {
    width: 100vw;
    height: 100dvh;
    border: 0;
    border-radius: 0;
  }
  .upload-confirm-container { flex-direction: column; }
  .files-panel {
    width: 100%;
    height: 132px;
    flex: 0 0 132px;
    border-right: 0;
    border-bottom: 1px solid var(--zeal-line, #dbe3ed);
  }
  .files-panel-header { padding: 14px 52px 8px 16px; }
  .files-list {
    display: flex;
    gap: 7px;
    padding: 2px 12px 12px;
    overflow-x: auto;
    overflow-y: hidden;
  }
  .file-item {
    width: 180px;
    min-width: 180px;
    min-height: 56px;
    padding: 8px 10px;
    border: 1px solid var(--zeal-line, #dbe3ed);
    background: var(--zeal-surface, #fff);
  }
  .reparse-source-panel { padding: 4px 16px 12px; }
  .main-header { padding: 18px 52px 0 16px; }
  .main-title { font-size: 17px; }
  .main-body { padding: 14px 16px 18px; }
  .overview-row {
    grid-template-columns: 86px minmax(0, 1fr) 18px;
    gap: 8px;
    padding: 13px 12px;
  }
  .overview-label,
  .overview-value { font-size: 12px; }
  .modal-footer { padding: 10px 12px; }
}
</style>
