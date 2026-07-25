<template>
  <ZealKnowledgeCreate
    v-if="mode === 'create'"
    :visible="visible"
    @update:visible="emit('update:visible', $event)"
    @success="handleZealCreateSuccess"
  />

  <Teleport v-else to="body">
    <Transition name="modal">
      <div v-if="visible" class="settings-overlay" @click.self="handleClose">
        <div class="settings-modal">
          <header class="settings-topbar">
            <div class="settings-brand-block">
              <span class="settings-brand-mark">K</span>
              <div>
                <span class="settings-brand-kicker">知识空间</span>
                <strong>入库配置台</strong>
              </div>
            </div>
            <div class="settings-title-block">
              <span>知识库设置</span>
              <h1>{{ formData?.name || '未命名知识库' }}</h1>
            </div>
            <div v-if="formData" class="settings-top-meta">
              <span class="settings-meta-chip"><i class="meta-dot"></i>{{ hasFiles ? '已有内容' : '等待首份文档' }}</span>
              <span class="settings-meta-chip">{{ formData.maxFileSizeMB }} MB / 文件</span>
            </div>
            <button class="close-btn" @click="handleClose" :aria-label="$t('general.close')">
              <t-icon name="close" size="18px" />
            </button>
          </header>

          <nav class="settings-nav">
            <div v-for="group in navGroups" :key="group.key" class="nav-cluster">
              <span class="nav-cluster-label">{{ group.label }}</span>
              <div class="nav-cluster-items">
                <button
                  v-for="item in group.items"
                  :key="item.key"
                  type="button"
                  :class="['nav-item', { active: currentSection === item.key }]"
                  @click="currentSection = item.key"
                >
                  <t-icon :name="item.icon" class="nav-icon" />
                  <span class="nav-label">{{ item.label }}</span>
                  <span v-if="item.badge" class="nav-badge">{{ item.badge }}</span>
                </button>
              </div>
            </div>
          </nav>

          <div class="settings-container">
            <div class="settings-content">
              <div class="content-wrapper">
                <!-- 基本信息 -->
                <div v-show="currentSection === 'basic'" class="section">
                  <div v-if="formData" class="section-content">
                    <div class="section-header">
                      <h3 class="section-title">{{ $t('knowledgeEditor.basic.title') }}</h3>
                      <p class="section-desc">{{ $t('knowledgeEditor.basic.description') }}</p>
                    </div>
                    <div class="section-body">
                      <div v-if="mode === 'edit' && props.kbId" class="form-item">
                        <label class="form-label">{{ $t('knowledgeEditor.basic.kbId') }}</label>
                        <p class="form-tip">{{ $t('knowledgeEditor.basic.kbIdDesc') }}</p>
                        <div class="kb-id-field">
                          <code class="kb-id-value" :title="props.kbId">{{ props.kbId }}</code>
                          <t-tooltip :content="$t('common.copy')" placement="top">
                            <t-button theme="default" size="small" variant="text" class="kb-id-copy"
                              @click="copyKbId">
                              <t-icon name="file-copy" />
                            </t-button>
                          </t-tooltip>
                        </div>
                      </div>

                      <div class="form-item">
                        <label class="form-label required">{{ $t('knowledgeEditor.basic.typeLabel') }}</label>
                        <t-radio-group
                          v-model="formData.type"
                          :disabled="mode === 'edit'"
                        >
                          <t-radio-button value="document">{{ $t('knowledgeEditor.basic.typeDocument') }}</t-radio-button>
                          <t-radio-button value="faq">{{ $t('knowledgeEditor.basic.typeFAQ') }}</t-radio-button>
                        </t-radio-group>
                        <p class="form-tip">{{ $t('knowledgeEditor.basic.typeDescription') }}</p>
                      </div>

                      <div v-if="!isFAQ" class="form-item indexing-workbench">
                        <div class="indexing-workbench-heading">
                          <span>索引能力</span>
                          <div>
                            <strong>选择入库后生成的知识结构</strong>
                            <p>混合检索固定启用，为快速问答提供稳定的文档召回。</p>
                          </div>
                        </div>
                        <div class="capability-board">
                          <article class="capability-row fixed">
                            <div class="capability-icon"><t-icon name="search" size="20px" /></div>
                            <div class="capability-copy">
                              <div><strong>混合检索</strong><span class="status-tag">始终启用</span></div>
                              <p>同时生成向量与关键词索引，负责快速问答的基础召回。</p>
                            </div>
                            <t-icon name="check-circle-filled" class="enabled-icon" size="22px" />
                          </article>
                        </div>
                      </div>

                      <div class="form-item">
                        <label class="form-label required">{{ $t('knowledgeEditor.basic.nameLabel') }}</label>
                        <t-input 
                          v-model="formData.name" 
                          :placeholder="$t('knowledgeEditor.basic.namePlaceholder')"
                          :maxlength="50"
                        />
                      </div>
                      <div class="form-item">
                        <label class="form-label">{{ $t('knowledgeEditor.basic.descriptionLabel') }}</label>
                        <t-textarea
                          v-model="formData.description"
                          :placeholder="$t('knowledgeEditor.basic.descriptionPlaceholder')"
                          :maxlength="200"
                          :autosize="{ minRows: 3, maxRows: 6 }"
                        />
                      </div>

                      <div v-if="!isFAQ" class="form-item">
                        <label class="form-label">单文件大小上限</label>
                        <t-radio-group v-model="formData.maxFileSizeMB" variant="default-filled">
                          <t-radio-button v-for="size in fileSizeOptions" :key="size" :value="size">
                            {{ size }} MB
                          </t-radio-button>
                        </t-radio-group>
                        <p class="form-tip">修改后仅限制新上传文件，不影响已入库文档。</p>
                      </div>

                    </div>
                  </div>
                </div>

                <!-- FAQ 配置 -->
                <div v-if="isFAQ && formData" v-show="currentSection === 'faq'" class="section">
                  <div class="section-content">
                    <div class="section-header">
                      <h3 class="section-title">{{ $t('knowledgeEditor.faq.title') }}</h3>
                      <p class="section-desc">{{ $t('knowledgeEditor.faq.description') }}</p>
                    </div>
                    <div class="section-body">
                      <div class="form-item">
                        <label class="form-label required">{{ $t('knowledgeEditor.faq.indexModeLabel') }}</label>
                        <t-radio-group
                          v-model="formData.faqConfig.indexMode"
                        >
                          <t-radio-button value="question_only">{{ $t('knowledgeEditor.faq.modes.questionOnly') }}</t-radio-button>
                          <t-radio-button value="question_answer">{{ $t('knowledgeEditor.faq.modes.questionAnswer') }}</t-radio-button>
                        </t-radio-group>
                        <p class="form-tip">{{ $t('knowledgeEditor.faq.indexModeDescription') }}</p>
                      </div>
                      <div class="form-item">
                        <label class="form-label required">{{ $t('knowledgeEditor.faq.questionIndexModeLabel') }}</label>
                        <t-radio-group
                          v-model="formData.faqConfig.questionIndexMode"
                        >
                          <t-radio-button value="combined">{{ $t('knowledgeEditor.faq.modes.combined') }}</t-radio-button>
                          <t-radio-button value="separate">{{ $t('knowledgeEditor.faq.modes.separate') }}</t-radio-button>
                        </t-radio-group>
                        <p class="form-tip">{{ $t('knowledgeEditor.faq.questionIndexModeDescription') }}</p>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- 解析引擎 -->
                <div v-if="!isFAQ && formData && currentSection === 'parser'" class="section">
                  <KBParserSettings
                    :parser-engine-rules="formData.chunkingConfig.parserEngineRules"
                    @update:parser-engine-rules="handleParserEngineRulesUpdate"
                  />
                </div>

                <!-- 分块设置 -->
                <div v-if="!isFAQ" v-show="currentSection === 'chunking'" class="section">
                  <KBChunkingSettings
                    v-if="formData"
                    :config="formData.chunkingConfig"
                    @update:config="handleChunkingConfigUpdate"
                  />
                </div>

                <!-- 高级设置 -->
                <div v-if="!isFAQ" v-show="currentSection === 'advanced'" class="section">
                  <KBAdvancedSettings
                    ref="advancedSettingsRef"
                    v-if="formData"
                    :question-generation="formData.questionGenerationConfig"
                    :rag-enabled="formData.indexingStrategy?.vectorEnabled || formData.indexingStrategy?.keywordEnabled"
                    @update:question-generation="handleQuestionGenerationUpdate"
                  />
                </div>

              </div>

              <!-- 保存按钮 -->
              <div class="settings-footer">
                <t-button theme="default" variant="outline" @click="handleClose">
                  {{ $t('common.cancel') }}
                </t-button>
                <t-button theme="primary" @click="handleSubmit" :loading="saving">
                  {{ $t('knowledgeEditor.buttons.save') }}
                </t-button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>

</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { MessagePlugin, DialogPlugin } from 'tdesign-vue-next'
import { createKnowledgeBase, getKnowledgeBaseById, listKnowledgeFiles, updateKnowledgeBase, rebuildKBIndex } from '@/api/knowledge-base'
import { updateKBConfig, type KBModelConfigRequest } from '@/api/initialization'
import { useChatResourcesStore } from '@/stores/chatResources'
import { useEditorResourcesStore } from '@/stores/editorResources'
import { useUIStore } from '@/stores/ui'
import KBParserSettings from './settings/KBParserSettings.vue'
import KBChunkingSettings from './settings/KBChunkingSettings.vue'
import KBAdvancedSettings from './settings/KBAdvancedSettings.vue'
import ZealKnowledgeCreate from './ZealKnowledgeCreate.vue'
import { useI18n } from 'vue-i18n'
import { MAX_FILE_SIZE_MB } from '@/utils'

const uiStore = useUIStore()
const chatResources = useChatResourcesStore()
const editorResources = useEditorResourcesStore()
const { t } = useI18n()

// Props
const props = defineProps<{
  visible: boolean
  mode: 'create' | 'edit'
  kbId?: string
  initialType?: 'document' | 'faq'
}>()

// Emits
const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
  (e: 'success', kbId: string): void
}>()

const handleZealCreateSuccess = (kbId: string) => {
  emit('success', kbId)
}

const copyKbId = async () => {
  const id = props.kbId
  if (!id) return

  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(id)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = id
      textarea.setAttribute('readonly', '')
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    MessagePlugin.success(t('common.copied'))
  } catch {
    MessagePlugin.error(t('common.copyFailed'))
  }
}

const currentSection = ref<string>('basic')

const saving = ref(false)
const loading = ref(false)
const allModels = ref<any[]>([])
const hasFiles = ref(false)
const initialIndexingStrategy = ref<any>(null)

const navItems = computed(() => {
  const items: { key: string; icon: string; label: string; badge?: number }[] = [
    { key: 'basic', icon: 'dashboard-1', label: t('knowledgeEditor.sidebar.basic') }
  ]
  if (formData.value?.type === 'faq') {
    items.push({ key: 'faq', icon: 'help-circle', label: t('knowledgeEditor.sidebar.faq') })
  } else {
    items.push(
      { key: 'parser', icon: 'file-search', label: t('settings.parserEngine') },
		{ key: 'chunking', icon: 'git-branch', label: t('knowledgeEditor.sidebar.chunking') },
      { key: 'advanced', icon: 'adjustment', label: t('knowledgeEditor.sidebar.advanced') }
    )
  }
  return items
})

// 左侧导航分组（与 AgentEditorModal 对齐）
const navGroups = computed(() => {
  const itemMap = new Map(navItems.value.map((item) => [item.key, item]))
  const pickItems = (keys: string[]) =>
    keys.map((key) => itemMap.get(key)).filter(Boolean) as typeof navItems.value
  return [
    {
      key: 'basic',
      label: t('knowledgeEditor.navGroups.basic'),
      items: pickItems(['basic', 'faq']),
    },
    {
      key: 'processing',
      label: t('knowledgeEditor.navGroups.processing'),
		items: pickItems(['parser', 'chunking', 'advanced']),
    },
  ].filter((group) => group.items.length > 0)
})

const advancedSettingsRef = ref<InstanceType<typeof KBAdvancedSettings>>()

// 表单数据
const formData = ref<any>(null)
const isFAQ = computed(() => formData.value?.type === 'faq')
const fileSizeOptions = computed(() => {
  const options = [10, 25, 50, 75, 100].filter((size) => size <= MAX_FILE_SIZE_MB)
  if (!options.includes(MAX_FILE_SIZE_MB)) options.push(MAX_FILE_SIZE_MB)
  return options.sort((a, b) => a - b)
})

const defaultVLM = computed(() =>
  allModels.value.find((model) => model.type === 'VLLM' && model.is_default)
)
const defaultVLMName = computed(() =>
  defaultVLM.value?.display_name?.trim() || defaultVLM.value?.name || '未配置'
)

watch(
  () => formData.value?.type,
  (newType, oldType) => {
    if (!formData.value) return
    if (newType === 'faq') {
      if (!formData.value.faqConfig) {
        formData.value.faqConfig = { indexMode: 'question_only', questionIndexMode: 'separate' }
      }
      if (!['basic', 'faq'].includes(currentSection.value)) {
        currentSection.value = 'faq'
      }
    } else if (oldType === 'faq' && currentSection.value === 'faq') {
      currentSection.value = 'basic'
    }
  }
)

// 初始化表单数据
const initFormData = (type: 'document' | 'faq' = 'document') => {
  return {
    type,
    name: '',
    description: '',
    maxFileSizeMB: Math.min(50, MAX_FILE_SIZE_MB),
    faqConfig: {
      indexMode: 'question_only',
      questionIndexMode: 'separate'
    },
    chunkingConfig: {
      chunkSize: 512,
      // 80 ≈ 15% of chunkSize — community-recommended sweet spot.
      // Aligned with chunker.DefaultChunkOverlap on the backend.
      chunkOverlap: 80,
      separators: ['\n\n', '\n', '。', '！', '？', ';', '；'],
      parserEngineRules: undefined as any,
      enableParentChild: true,
      parentChunkSize: 4096,
      childChunkSize: 384,
      // New KBs default to the adaptive auto-strategy. User can change in the UI.
      strategy: 'auto' as string,
      tokenLimit: 0,
      languages: [] as string[]
    },
    questionGenerationConfig: {
      enabled: true,
      questionCount: 3
    },
    indexingStrategy: {
      vectorEnabled: true,
      keywordEnabled: true,
    },
  }
}

// 加载所有模型
const loadAllModels = async (force = false) => {
  try {
    await chatResources.ensureModels(force)
    allModels.value = chatResources.allModels || []
  } catch (error) {
    console.error('Failed to load model list:', error)
    MessagePlugin.error(t('knowledgeEditor.messages.loadModelsFailed'))
    allModels.value = []
  }
}

// 加载知识库数据（编辑模式）
const loadKBData = async () => {
  if (props.mode !== 'edit' || !props.kbId) return
  
  loading.value = true
  try {
    const [kbInfo, filesResult] = await Promise.all([
      getKnowledgeBaseById(props.kbId),
      listKnowledgeFiles(props.kbId, { page: 1, page_size: 1 })
    ])
    
    if (!kbInfo || !kbInfo.data) {
      throw new Error(t('knowledgeEditor.messages.notFound'))
    }

    const kb = kbInfo.data
    const loadedIndexingStrategy = {
      vectorEnabled: kb.indexing_strategy?.vector_enabled ?? true,
      keywordEnabled: kb.indexing_strategy?.keyword_enabled ?? true,
    }
    hasFiles.value = (filesResult as any)?.total > 0
    // 设置表单数据
    const kbType = (kb.type as 'document' | 'faq') || 'document'
    formData.value = {
      type: kbType,
      name: kb.name || '',
      description: kb.description || '',
      maxFileSizeMB: kb.max_file_size_mb || Math.min(50, MAX_FILE_SIZE_MB),
      faqConfig: {
        indexMode: kb.faq_config?.index_mode || 'question_only',
        questionIndexMode: kb.faq_config?.question_index_mode || 'separate'
      },
      chunkingConfig: {
        chunkSize: kb.chunking_config?.chunk_size || 512,
        // Fallback only used when the loaded KB has no chunk_overlap stored.
        // Aligned with chunker.DefaultChunkOverlap on the backend.
        chunkOverlap: kb.chunking_config?.chunk_overlap || 80,
        separators: kb.chunking_config?.separators || ['\n\n', '\n', '。', '！', '？', ';', '；'],
        parserEngineRules: kb.chunking_config?.parser_engine_rules || undefined,
        enableParentChild: kb.chunking_config?.enable_parent_child || false,
        parentChunkSize: kb.chunking_config?.parent_chunk_size || 4096,
        childChunkSize: kb.chunking_config?.child_chunk_size || 384,
        // Existing KBs without strategy field render as empty (= legacy behavior).
        // The user has to actively pick a value to opt in to the new tiers.
        strategy: kb.chunking_config?.strategy || '',
        tokenLimit: kb.chunking_config?.token_limit || 0,
        languages: kb.chunking_config?.languages || []
      },
      questionGenerationConfig: {
        enabled: kb.question_generation_config?.enabled || false,
        questionCount: kb.question_generation_config?.question_count || 3
      },
      indexingStrategy: {
        vectorEnabled: true,
        keywordEnabled: true,
      },
    }
    initialIndexingStrategy.value = loadedIndexingStrategy
  } catch (error) {
    console.error('Failed to load knowledge base data:', error)
    MessagePlugin.error(t('knowledgeEditor.messages.loadDataFailed'))
    handleClose()
  } finally {
    loading.value = false
  }
}

const handleChunkingConfigUpdate = (config: any) => {
  if (formData.value) {
    formData.value.chunkingConfig = { ...config }
  }
}

const handleParserEngineRulesUpdate = (rules: any[]) => {
  if (formData.value) {
    formData.value.chunkingConfig.parserEngineRules = rules?.length ? rules : undefined
  }
}

const handleMultimodalToggle = () => {
  if (!formData.value) return
  if (formData.value.multimodalConfig.enabled) {
    formData.value.multimodalConfig.vllmModelId = defaultVLM.value?.id || ''
  } else {
    formData.value.multimodalConfig.vllmModelId = ''
  }
}

const handleAddVLLMModel = () => {
  uiStore.openSettings('models', 'vllm')
}

const handleQuestionGenerationUpdate = (config: any) => {
  if (formData.value) {
    formData.value.questionGenerationConfig = { ...config }
  }
}

// 验证表单
const validateForm = (): boolean => {
  if (!formData.value) return false

  // 验证基本信息
  if (!formData.value.name || !formData.value.name.trim()) {
    MessagePlugin.warning(t('knowledgeEditor.messages.nameRequired'))
    currentSection.value = 'basic'
    return false
  }

  if (formData.value.type === 'faq' && !formData.value.faqConfig?.indexMode) {
    MessagePlugin.warning(t('knowledgeEditor.messages.indexModeRequired'))
    currentSection.value = 'faq'
    return false
  }

  return true
}

// 构建提交数据
const buildSubmitData = () => {
  if (!formData.value) return null

  const data: any = {
    name: formData.value.name,
    description: formData.value.description,
    type: formData.value.type,
    max_file_size_mb: formData.value.maxFileSizeMB,
    chunking_config: {
      chunk_size: formData.value.chunkingConfig.chunkSize,
      chunk_overlap: formData.value.chunkingConfig.chunkOverlap,
      separators: formData.value.chunkingConfig.separators,
      enable_parent_child: formData.value.chunkingConfig.enableParentChild,
      parent_chunk_size: formData.value.chunkingConfig.parentChunkSize,
      child_chunk_size: formData.value.chunkingConfig.childChunkSize,
      // Adaptive chunking fields are always sent (empty/zero values
      // included) so the user can clear them — backend uses pointer DTOs
      // to distinguish "not in payload" from "explicitly empty".
      strategy: formData.value.chunkingConfig.strategy ?? '',
      token_limit: formData.value.chunkingConfig.tokenLimit ?? 0,
      languages: formData.value.chunkingConfig.languages ?? [],
      ...(formData.value.chunkingConfig.parserEngineRules?.length
        ? { parser_engine_rules: formData.value.chunkingConfig.parserEngineRules }
        : {})
    }
  }

  // 添加问题生成配置
  if (formData.value.questionGenerationConfig?.enabled) {
    data.question_generation_config = {
      enabled: true,
      question_count: formData.value.questionGenerationConfig.questionCount || 3
    }
  }

  if (formData.value.type === 'faq') {
    data.faq_config = {
      index_mode: formData.value.faqConfig?.indexMode || 'question_only',
      question_index_mode: formData.value.faqConfig?.questionIndexMode || 'separate'
    }
  }

  // Send indexing strategy
  if (formData.value.type !== 'faq') {
    data.indexing_strategy = {
      vector_enabled: true,
      keyword_enabled: true,
    }
  }

  return data
}

// 提交表单
const handleSubmit = async () => {
  if (!validateForm()) {
    return
  }

  doSubmit()
}

const doSubmit = async () => {
  saving.value = true
  try {
    const data = buildSubmitData()
    if (!data) {
      throw new Error(t('knowledgeEditor.messages.buildDataFailed'))
    }

    if (props.mode === 'create') {
      // 创建模式：一次性创建知识库及所有配置
      const result: any = await createKnowledgeBase(data)
      if (!result.success || !result.data?.id) {
        throw new Error(result.message || t('knowledgeEditor.messages.createFailed'))
      }
      MessagePlugin.success(t('knowledgeEditor.messages.createSuccess'))
      emit('success', result.data.id)
    } else {
      // 编辑模式：分别更新基本信息和配置
      if (!props.kbId) {
        throw new Error(t('knowledgeEditor.messages.missingId'))
      }

      // 1. 更新基本信息（名称、描述）和 FAQ 配置
      const updateConfig: any = {}
      updateConfig.max_file_size_mb = formData.value.maxFileSizeMB
      if (formData.value.type === 'faq' && formData.value.faqConfig) {
        updateConfig.faq_config = {
          index_mode: formData.value.faqConfig.indexMode || 'question_only',
          question_index_mode: formData.value.faqConfig.questionIndexMode || 'separate'
        }
      }
      if (formData.value.type !== 'faq') {
        updateConfig.indexing_strategy = {
          vector_enabled: true,
          keyword_enabled: true,
        }
      }
      await updateKnowledgeBase(props.kbId, {
        name: data.name,
        description: data.description,
        config: updateConfig
      })

      // 2. 更新完整配置（模型、分块、多模态、存储引擎等）
      const config: KBModelConfigRequest = {
        documentSplitting: {
          chunkSize: data.chunking_config.chunk_size,
          chunkOverlap: data.chunking_config.chunk_overlap,
          separators: data.chunking_config.separators,
          parserEngineRules: data.chunking_config.parser_engine_rules || undefined,
          enableParentChild: data.chunking_config.enable_parent_child || false,
          parentChunkSize: data.chunking_config.parent_chunk_size || 4096,
          childChunkSize: data.chunking_config.child_chunk_size || 384,
          // Always send strategy / tokenLimit / languages — backend treats
          // empty/0/[] as a valid clear, so we must include them in the
          // payload to let users reset back to defaults.
          strategy: formData.value?.chunkingConfig.strategy ?? '',
          tokenLimit: formData.value?.chunkingConfig.tokenLimit ?? 0,
          languages: formData.value?.chunkingConfig.languages ?? []
        },
        questionGeneration: {
          enabled: data.question_generation_config?.enabled || false,
          questionCount: data.question_generation_config?.question_count || 3
        }
      }

      await updateKBConfig(props.kbId, config)
      MessagePlugin.success(t('knowledgeEditor.messages.updateSuccess'))

      // Check if indexing strategy changed and offer rebuild
      if (hasFiles.value && initialIndexingStrategy.value && formData.value) {
        const curr = formData.value.indexingStrategy
        const prev = initialIndexingStrategy.value
        const strategyChanged = (
          curr.vectorEnabled !== prev.vectorEnabled ||
          curr.keywordEnabled !== prev.keywordEnabled
        )
        if (strategyChanged) {
          const dialog = DialogPlugin.confirm({
            header: t('knowledgeEditor.indexing.rebuildConfirmTitle'),
            body: t('knowledgeEditor.indexing.rebuildConfirmBody', { count: '...' }),
            confirmBtn: t('common.confirm'),
            cancelBtn: t('common.cancel'),
            onConfirm: async () => {
              dialog.destroy()
              try {
                await rebuildKBIndex(props.kbId!)
                MessagePlugin.success(t('knowledgeBase.fullRebuildSubmitted'))
              } catch (e) {
                console.error('Rebuild index failed:', e)
              }
            },
            onCancel: () => {
              dialog.destroy()
              MessagePlugin.info(t('knowledgeEditor.indexing.rebuildSkip'))
            },
          })
        }
      }

      emit('success', props.kbId)
    }
    
    handleClose()
  } catch (error: any) {
    console.error('Knowledge base operation failed:', error)
    MessagePlugin.error(error?.message || t('common.operationFailed'))
  } finally {
    saving.value = false
  }
}

// 重置所有状态
const resetState = () => {
  currentSection.value = 'basic'
  formData.value = null
  hasFiles.value = false
  initialIndexingStrategy.value = null
  saving.value = false
  loading.value = false
}

// 关闭弹窗
const handleClose = () => {
  emit('update:visible', false)
  setTimeout(() => {
    resetState()
  }, 300)
}

// 监听弹窗打开/关闭
watch(() => props.visible, async (newVal) => {
  if (newVal) {
    // 打开弹窗时，先重置状态
    resetState()
    
    // 检查是否有初始 section，如果有则跳转
    if (uiStore.kbEditorInitialSection && navItems.value.some((item) => item.key === uiStore.kbEditorInitialSection)) {
      currentSection.value = uiStore.kbEditorInitialSection
    }
    
    await loadAllModels()
    
    // 根据模式加载数据
    if (props.mode === 'edit' && props.kbId) {
      await loadKBData()
    } else {
      formData.value = initFormData(props.initialType || 'document')
      hasFiles.value = false
    }
  } else {
    // 关闭弹窗时，延迟重置状态（等待动画结束）
    setTimeout(() => {
      resetState()
      currentSection.value = 'basic' // 重置为默认 section
    }, 300)
  }
})

// 监听全局设置弹窗关闭后刷新模型列表
watch(
  () => uiStore.showSettingsModal,
  async (visible, previous) => {
    if (!visible && previous && props.visible) {
      await loadAllModels(true)
    }
  }
)
</script>

<style scoped lang="less">
// 复用创建知识库的样式
.settings-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  backdrop-filter: blur(4px);
}

.settings-modal {
  position: relative;
  width: 90vw;
  max-width: 1000px;
  height: 85vh;
  max-height: 750px;
  background: var(--td-bg-color-container);
  border-radius: 12px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.12);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.close-btn {
  position: absolute;
  top: 20px;
  right: 20px;
  width: 32px;
  height: 32px;
  border: none;
  background: var(--td-bg-color-secondarycontainer);
  border-radius: 6px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--td-text-color-secondary);
  transition: all 0.2s ease;
  z-index: 10;

  &:hover {
    background: var(--td-bg-color-secondarycontainer);
    color: var(--td-text-color-primary);
  }
}

.settings-container {
  display: flex;
  height: 100%;
  width: 100%;
  overflow: hidden;
}

/* 左侧导航：与 AgentEditorModal 对齐 */
.settings-sidebar {
  width: 208px;
  background-color: var(--td-bg-color-settings-modal);
  border-right: 1px solid var(--td-component-stroke);
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.sidebar-header {
  padding: 16px 14px 12px;
  border-bottom: 1px solid var(--td-component-stroke);
  flex-shrink: 0;
}

.sidebar-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.settings-nav {
  flex: 1;
  padding: 8px 8px 12px;
  overflow-y: auto;
  min-height: 0;
}

.nav-group-title {
  padding: 6px 14px 2px;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.02em;

  .settings-nav > &:first-child {
    padding-top: 2px;
  }

  .settings-nav > &:not(:first-child) {
    padding-top: 8px;
  }
}

.nav-item {
  display: flex;
  align-items: center;
  padding: 6px 12px;
  margin-bottom: 2px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
  font-size: 14px;
  color: var(--td-text-color-primary);
  user-select: none;

  &:hover {
    background-color: var(--td-bg-color-secondarycontainer-hover);
    color: var(--td-text-color-primary);
  }

  &.active {
    background-color: rgba(7, 192, 95, 0.1);
    color: var(--td-brand-color);
    font-weight: 500;
  }
}

.nav-icon {
  margin-right: 9px;
  font-size: 16px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: inherit;
}

.nav-label {
  flex: 1;
}

.nav-badge {
  flex-shrink: 0;
  min-width: 18px;
  padding: 0 6px;
  border-radius: 10px;
  background: var(--td-bg-color-secondarycontainer);
  font-size: 11px;
  font-weight: 600;
  line-height: 18px;
  text-align: center;
  color: var(--td-text-color-secondary);

  .nav-item.active & {
    background: color-mix(in srgb, var(--td-brand-color) 18%, transparent);
    color: var(--td-brand-color);
  }
}

.settings-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.content-wrapper {
  flex: 1;
  overflow-y: auto;
  padding: 24px 32px;
}

.section {
  margin-bottom: 32px;

  &:last-child {
    margin-bottom: 0;
  }
}

.section-content {
  .section-header {
    margin-bottom: 16px;
  }

  .section-title {
    margin: 0 0 6px 0;
    font-family: var(--app-font-family);
    font-size: 20px;
    font-weight: 600;
    color: var(--td-text-color-primary);
  }

  .section-desc {
    margin: 0;
    font-family: var(--app-font-family);
    font-size: 14px;
    color: var(--td-text-color-placeholder);
    line-height: 22px;
  }

  .section-body {
    background: var(--td-bg-color-container);
  }
}

.form-item {
  margin-bottom: 16px;

  &:last-child {
    margin-bottom: 0;
  }
}

.form-label {
  display: block;
  margin-bottom: 8px;
  font-family: var(--app-font-family);
  font-size: 15px;
  font-weight: 500;
  color: var(--td-text-color-primary);

  &.required::after {
    content: '*';
    color: var(--td-error-color);
    margin-left: 4px;
  }
}

.form-tip {
  margin-top: 6px;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.kb-id-field {
  display: flex;
  align-items: center;
  gap: 4px;
  width: 100%;
  max-width: 480px;
  margin-top: 8px;
  padding: 6px 8px 6px 12px;
  background: var(--td-bg-color-secondarycontainer);
  border: 1px solid var(--td-component-stroke);
  border-radius: 6px;

  .kb-id-value {
    flex: 1;
    min-width: 0;
    margin: 0;
    padding: 0;
    background: none;
    border: none;
    font-family: var(--app-font-family-mono);
    font-size: 13px;
    line-height: 1.5;
    color: var(--td-text-color-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .kb-id-copy {
    flex-shrink: 0;
    color: var(--td-text-color-secondary);

    &:hover {
      color: var(--td-brand-color);
    }
  }
}

.granularity-radio-group {
  margin-top: 4px;
}

.granularity-hint {
  margin-top: 8px;
  line-height: 1.6;
  color: var(--td-text-color-secondary);
  white-space: normal;
  word-break: break-word;
}

.indexing-checks {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 12px;
  margin-top: 10px;
}

.indexing-check-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 14px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-container);
  cursor: pointer;
  user-select: none;
  transition: border-color 0.2s ease, background 0.2s ease;

  &:hover {
    border-color: var(--td-brand-color);
  }

  &.is-checked {
    border-color: var(--td-brand-color);
    background: var(--td-brand-color-light);
  }

  &.is-disabled {
    cursor: not-allowed;
    opacity: 0.7;

    &:hover {
      border-color: var(--td-component-stroke);
    }

    &.is-checked:hover {
      border-color: var(--td-brand-color);
    }
  }

  :deep(.t-checkbox__label) {
    font-weight: 500;
    color: var(--td-text-color-primary);
  }
}

.locked-tip {
  color: var(--td-warning-color);
  margin-top: 8px;
}

// 禁用内部 checkbox 自身的点击事件，统一由卡片处理
.indexing-check-box {
  pointer-events: none;
}

.indexing-check-title {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.indexing-new-badge {
  display: inline-flex;
  align-items: center;
  padding: 0 6px;
  height: 16px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 600;
  line-height: 1;
  letter-spacing: 0.4px;
  color: var(--td-brand-color);
  background: var(--td-brand-color-light);
}

.indexing-check-desc {
  margin: 0;
  padding-left: 24px;
  font-size: 12px;
  line-height: 18px;
  color: var(--td-text-color-placeholder);
}

.settings-footer {
  padding: 16px 32px;
  border-top: 1px solid var(--td-component-stroke);
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  flex-shrink: 0;
}

// 过渡动画
.modal-enter-active,
.modal-leave-active {
  transition: all 0.3s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;

  .settings-modal {
    transform: scale(0.95);
  }
}

// 多模态配置内联样式（与 KBAdvancedSettings 一致）
.kb-multimodal-settings {
  width: 100%;

  .section-header {
    margin-bottom: 20px;

    h2 {
      font-size: 20px;
      font-weight: 600;
      color: var(--td-text-color-primary);
      margin: 0 0 6px 0;
    }

    .section-description {
      font-size: 14px;
      color: var(--td-text-color-secondary);
      margin: 0;
      line-height: 1.5;
    }
  }

  .settings-group {
    display: flex;
    flex-direction: column;
  }

  .setting-row {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    padding: 16px 0;
    border-bottom: 1px solid var(--td-component-stroke);

    &:last-child {
      border-bottom: none;
    }
  }

  .setting-info {
    flex: 1;
    max-width: 65%;
    padding-right: 24px;

    label {
      font-size: 15px;
      font-weight: 500;
      color: var(--td-text-color-primary);
      display: block;
      margin-bottom: 4px;
    }

    .desc {
      font-size: 13px;
      color: var(--td-text-color-secondary);
      margin: 0;
      line-height: 1.5;
    }
  }

  .setting-control {
    flex-shrink: 0;
    min-width: 280px;
    display: flex;
    justify-content: flex-end;
    align-items: center;
  }

  .required {
    color: var(--td-error-color);
    margin-left: 2px;
    font-weight: 500;
  }
}

// ZgentFlow knowledge configuration workbench.
.settings-modal {
  width: min(94vw, 1240px);
  max-width: none;
  height: min(90vh, 860px);
  max-height: none;
  border: 1px solid rgba(205, 217, 232, 0.9);
  border-radius: 7px;
  box-shadow: 0 24px 70px rgba(17, 34, 58, 0.28);
  background: #f5f7fa;
}

.settings-topbar {
  min-height: 76px;
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr) auto;
  align-items: center;
  gap: 24px;
  padding: 12px 66px 12px 18px;
  border-bottom: 1px solid #dce4ee;
  background: #fff;
}

.settings-brand-block { display: flex; align-items: center; gap: 10px; }
.settings-brand-mark { width: 34px; height: 34px; display: grid; place-items: center; border-radius: 5px; color: #fff; background: #1268e3; font-size: 16px; font-weight: 800; }
.settings-brand-block > div { display: grid; gap: 2px; }
.settings-brand-kicker { color: #7c899a; font-size: 9px; font-weight: 700; }
.settings-brand-block strong { color: #243249; font-size: 13px; }
.settings-title-block { min-width: 0; }
.settings-title-block > span { color: #1268e3; font-size: 10px; font-weight: 700; }
.settings-title-block h1 { overflow: hidden; margin: 3px 0 0; color: #172033; font-size: 19px; line-height: 1.25; letter-spacing: 0; text-overflow: ellipsis; white-space: nowrap; }
.settings-top-meta { display: flex; align-items: center; gap: 7px; }
.settings-meta-chip { min-height: 26px; display: inline-flex; align-items: center; gap: 6px; padding: 0 9px; border: 1px solid #dce4ee; border-radius: 4px; color: #607087; background: #f8fafc; font-size: 10px; white-space: nowrap; }
.meta-dot { width: 6px; height: 6px; border-radius: 50%; background: #14a06f; }

.close-btn { top: 20px; right: 18px; background: transparent; }

.settings-nav {
  flex: 0 0 62px;
  height: 62px;
  min-height: 62px;
  display: flex;
  align-items: stretch;
  gap: 26px;
  padding: 0 18px;
  border-bottom: 1px solid #dce4ee;
  background: #fff;
  overflow-x: auto;
  overflow-y: hidden;
}

.nav-cluster { display: flex; align-items: center; gap: 8px; flex: 0 0 auto; }
.nav-cluster-label { color: #9aa4b2; font-size: 8px; font-weight: 700; writing-mode: vertical-rl; transform: rotate(180deg); }
.nav-cluster-items { height: 100%; display: flex; align-items: stretch; gap: 1px; }
.settings-nav .nav-item {
  min-width: 82px;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 5px;
  margin: 0;
  padding: 7px 10px 6px;
  border: 0;
  border-bottom: 3px solid transparent;
  border-radius: 0;
  color: #657287;
  background: transparent;
  cursor: pointer;
  font: inherit;
}
.settings-nav .nav-item:hover { color: #1268e3; background: #f7f9fc; }
.settings-nav .nav-item.active { border-bottom-color: #1268e3; color: #1268e3; background: #f3f7fe; font-weight: 700; }
.settings-nav .nav-icon { margin: 0; font-size: 17px; }
.settings-nav .nav-label { flex: none; font-size: 10px; white-space: nowrap; }

.settings-container {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  min-height: 0;
  flex: 1;
  height: auto;
}

.indexing-workbench {
  margin-top: 24px;
  padding-top: 22px;
  border-top: 1px solid #dce4ee;
}

.indexing-workbench-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 14px;

  > span {
    color: #1268e3;
    font-size: 10px;
    font-weight: 750;
    letter-spacing: 0;
  }

  > div {
    flex: 1;
    max-width: 560px;
  }

  strong {
    display: block;
    color: #1d2738;
    font-size: 15px;
  }

  p {
    margin: 5px 0 0;
    color: #657185;
    font-size: 12px;
    line-height: 1.55;
  }
}

.capability-board {
  overflow: hidden;
  border: 1px solid #dce2ea;
  border-radius: 6px;
  background: #fff;
}

.capability-row {
  min-height: 92px;
  padding: 18px 22px;
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr) auto;
  align-items: center;
  gap: 16px;
  border-bottom: 1px solid #e1e6ed;
  transition: background 0.16s ease;

  &:last-child { border-bottom: 0; }
  &.selected { background: #f3f7ff; }
}

.capability-icon {
  width: 40px;
  height: 40px;
  display: grid;
  place-items: center;
  border-radius: 5px;
  background: #e8f1ff;
  color: #1268e3;

  &.wiki { background: #e8f7f3; color: #14806b; }
}

.capability-copy {
  min-width: 0;

  > div { display: flex; align-items: center; gap: 10px; }
  strong { color: #1d2738; font-size: 14px; }
  span { color: #8490a2; font-size: 11px; }
  p { margin: 5px 0 0; color: #657185; font-size: 12px; line-height: 1.55; }
  .status-tag { padding: 2px 6px; border-radius: 3px; background: #e8f1ff; color: #1268e3; }
}

.enabled-icon { color: #1268e3; }

.indexing-rebuild-tip {
  display: flex;
  align-items: center;
  gap: 5px;
}

.settings-content { min-width: 0; background: #fff; }
.content-wrapper { padding: 26px 30px 40px; }
.section { margin-bottom: 0; }
.settings-footer { min-height: 62px; padding: 12px 30px; border-top-color: #dce4ee; background: #fff; }

@media (max-width: 1040px) {
  .settings-modal { width: 96vw; }
  .settings-topbar { grid-template-columns: 180px minmax(0, 1fr); }
  .settings-top-meta { display: none; }
  .settings-container { grid-template-columns: minmax(0, 1fr); }
  .settings-nav .nav-item { min-width: 72px; padding-inline: 8px; }
}

/* Responsive two-column knowledge configuration workbench */
.settings-modal {
  display: grid;
  grid-template: 84px minmax(0, 1fr) / 238px minmax(0, 1fr);
  width: min(94vw, 1240px);
  height: min(90vh, 860px);
  overflow: hidden;
  border-color: var(--zeal-line-strong, #c6d1df);
  border-radius: 8px;
  background: var(--zeal-canvas, #f3f6fa);
}

.settings-topbar {
  grid-column: 1 / 3;
  grid-row: 1;
  min-width: 0;
  min-height: 84px;
  padding: 12px 64px 12px 20px;
  grid-template-columns: 194px minmax(0, 1fr) auto;
  border-bottom-color: var(--zeal-line, #dbe3ed);
}

.settings-brand-mark { border-radius: 7px; background: var(--zeal-primary, #1769dc); }
.settings-title-block > span { color: var(--zeal-primary, #1769dc); }
.settings-title-block h1 { color: var(--zeal-ink, #18212f); }
.settings-meta-chip {
  min-height: 28px;
  border-color: var(--zeal-line, #dbe3ed);
  border-radius: 6px;
  color: var(--zeal-ink-soft, #445166);
  background: var(--zeal-surface-subtle, #f8fafc);
}

.settings-nav {
  grid-column: 1;
  grid-row: 2;
  width: 238px;
  height: auto;
  min-height: 0;
  padding: 20px 12px;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 20px;
  overflow-x: hidden;
  overflow-y: auto;
  border-right: 1px solid var(--zeal-line, #dbe3ed);
  border-bottom: 0;
  background: var(--zeal-surface-subtle, #f8fafc);
}

.nav-cluster {
  width: 100%;
  display: flex;
  align-items: stretch;
  flex-direction: column;
  gap: 7px;
}

.nav-cluster-label {
  padding: 0 9px;
  color: var(--zeal-muted, #778398);
  font-size: 10px;
  font-weight: 700;
  writing-mode: horizontal-tb;
  transform: none;
}

.nav-cluster-items {
  width: 100%;
  height: auto;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.settings-nav .nav-item {
  width: 100%;
  min-width: 0;
  height: 42px;
  padding: 0 10px;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  flex-direction: row;
  gap: 10px;
  border: 0;
  border-radius: 7px;
  color: var(--zeal-ink-soft, #445166);
  text-align: left;
}

.settings-nav .nav-item:hover {
  color: var(--zeal-ink, #18212f);
  background: var(--zeal-surface-muted, #edf2f7);
}

.settings-nav .nav-item.active {
  color: var(--zeal-primary, #1769dc);
  background: var(--zeal-surface, #fff);
  box-shadow: inset 3px 0 0 var(--zeal-primary, #1769dc), var(--zeal-shadow-xs);
}

.settings-nav .nav-icon {
  width: 28px;
  height: 28px;
  flex: 0 0 28px;
  margin: 0;
  display: grid;
  place-items: center;
  border: 1px solid var(--zeal-line, #dbe3ed);
  border-radius: 6px;
  background: var(--zeal-surface-muted, #edf2f7);
  color: var(--zeal-muted, #778398);
  font-size: 16px;
}
.settings-nav .nav-label { min-width: 0; flex: 1; font-size: 12px; }

.settings-nav .nav-item.active .nav-icon {
  border-color: #b8d0f3;
  color: var(--zeal-primary, #1769dc);
  background: var(--zeal-primary-soft, #eaf2ff);
}

.settings-container {
  grid-column: 2;
  grid-row: 2;
  min-width: 0;
  min-height: 0;
  height: auto;
  display: flex;
}

.settings-content { background: var(--zeal-surface, #fff); }
.content-wrapper {
  width: 100%;
  max-width: 920px;
  margin: 0 auto;
  padding: 30px 34px 44px;
}
.settings-footer {
  min-height: 66px;
  padding: 13px 30px;
  border-top-color: var(--zeal-line, #dbe3ed);
  background: var(--zeal-surface, #fff);
}
.indexing-workbench { border-top-color: var(--zeal-line, #dbe3ed); }
.indexing-workbench-heading > span { color: var(--zeal-primary, #1769dc); }
.capability-board { border-color: var(--zeal-line, #dbe3ed); border-radius: 8px; }
.capability-row { border-bottom-color: var(--zeal-line, #dbe3ed); }
.capability-row.selected { background: var(--zeal-primary-soft, #eaf2ff); }

@media (max-width: 900px) {
  .settings-modal { width: 96vw; grid-template-columns: 210px minmax(0, 1fr); }
  .settings-nav { width: 210px; }
  .settings-topbar { grid-template-columns: 170px minmax(0, 1fr); }
  .settings-top-meta { display: none; }
  .content-wrapper { padding-inline: 24px; }
}

@media (max-width: 680px) {
  .settings-overlay { align-items: stretch; background: var(--zeal-canvas, #f3f6fa); backdrop-filter: none; }
  .settings-modal {
    width: 100vw;
    height: calc(100vh - 64px - env(safe-area-inset-bottom));
    margin-bottom: calc(64px + env(safe-area-inset-bottom));
    grid-template: 70px 64px minmax(0, 1fr) / 1fr;
    border: 0;
    border-radius: 0;
  }
  .settings-topbar {
    grid-column: 1;
    grid-row: 1;
    min-height: 70px;
    padding: 10px 54px 10px 14px;
    grid-template-columns: minmax(0, 1fr);
  }
  .settings-brand-block { display: none; }
  .settings-title-block h1 { font-size: 18px; }
  .settings-title-block > span { font-size: 9px; }
  .close-btn { top: 19px; right: 14px; }
  .settings-nav {
    grid-column: 1;
    grid-row: 2;
    width: 100%;
    height: 64px;
    padding: 6px 10px;
    flex-direction: row;
    gap: 4px;
    overflow-x: auto;
    overflow-y: hidden;
    border-right: 0;
    border-bottom: 1px solid var(--zeal-line, #dbe3ed);
  }
  .nav-cluster { width: auto; flex: 0 0 auto; flex-direction: row; gap: 4px; }
  .nav-cluster-label { display: none; }
  .nav-cluster-items { width: auto; flex-direction: row; gap: 4px; }
  .settings-nav .nav-item {
    width: 76px !important;
    min-width: 76px;
    flex: 0 0 76px;
    height: 50px;
    padding: 4px 8px;
    justify-content: center;
    flex-direction: column;
    gap: 2px;
    text-align: center;
  }
  .settings-nav .nav-item.active { box-shadow: inset 0 2px 0 var(--zeal-primary), var(--zeal-shadow-xs); }
  .settings-nav .nav-icon { width: auto; font-size: 16px; }
  .settings-nav .nav-label { flex: 0 0 auto; font-size: 10px; }
  .settings-container { grid-column: 1; grid-row: 3; }
  .content-wrapper { padding: 20px 16px 30px; }
  .section-content .section-title { font-size: 18px; }
  .indexing-workbench-heading { flex-direction: column; gap: 8px; }
  .capability-row { min-height: 86px; padding: 14px; grid-template-columns: 36px minmax(0, 1fr) auto; gap: 10px; }
  .capability-icon { width: 34px; height: 34px; }
  .capability-copy p { display: -webkit-box; overflow: hidden; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
  .settings-footer { min-height: 60px; padding: 10px 16px; }
  .kb-multimodal-settings .setting-row { align-items: stretch; flex-direction: column; gap: 16px; }
  .kb-multimodal-settings .setting-info { max-width: none; padding-right: 0; }
  .kb-multimodal-settings .setting-control { min-width: 0; justify-content: flex-start; }
}
</style>
