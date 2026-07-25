<template>
  <Teleport to="body">
    <Transition name="zeal-create-fade">
      <div v-if="visible" class="zeal-create-overlay">
        <section class="zeal-create-workbench" role="dialog" aria-modal="true" aria-label="新建知识库">
          <header class="workbench-header">
            <div class="brand-lockup">
              <span class="brand-mark">Z</span>
              <div>
                <strong>新建知识库</strong>
              </div>
            </div>

            <ol class="step-track" aria-label="创建进度">
              <li
                v-for="item in steps"
                :key="item.id"
                :class="{ active: step === item.id, done: step > item.id }"
              >
                <button type="button" :disabled="item.id > highestReachableStep" @click="goToStep(item.id)">
                  <span class="step-number">
                    <t-icon v-if="step > item.id" name="check" size="14px" />
                    <template v-else>{{ item.id }}</template>
                  </span>
                  <span>
                    <strong>{{ item.title }}</strong>
                    <small>{{ item.subtitle }}</small>
                  </span>
                </button>
              </li>
            </ol>

            <t-tooltip content="关闭" placement="bottom">
              <button class="icon-button" type="button" aria-label="关闭" @click="close">
                <t-icon name="close" size="18px" />
              </button>
            </t-tooltip>
          </header>

          <main class="workbench-body">
            <section v-if="step === 1" class="stage stage-basic">
              <div class="stage-heading">
                <span class="stage-kicker">01 / 定义</span>
                <h1>先定义这组知识的边界</h1>
                <p>名称会出现在问答时的知识库选择器中，描述用于帮助你区分内容范围。</p>
              </div>

              <div class="basic-form">
                <label class="field-label" for="zeal-kb-name">知识库名称 <span>必填</span></label>
                <t-input
                  id="zeal-kb-name"
                  v-model="form.name"
                  size="large"
                  :maxlength="50"
                  placeholder="例如：产品研发资料"
                  autofocus
                />
                <div class="field-meta">
                  <span>建议按团队、项目或主题划分</span>
                  <span>{{ form.name.length }}/50</span>
                </div>

                <label class="field-label" for="zeal-kb-description">内容说明 <span class="optional">选填</span></label>
                <t-textarea
                  id="zeal-kb-description"
                  v-model="form.description"
                  :maxlength="200"
                  :autosize="{ minRows: 5, maxRows: 7 }"
                  placeholder="说明这里会存放哪些文档，以及主要服务于什么问答场景"
                />
                <div class="field-meta">
                  <span>后续可在知识库设置中修改</span>
                </div>

                <label class="field-label">单文件大小上限 <span>可修改</span></label>
                <t-radio-group v-model="form.maxFileSizeMB" class="file-limit-options" variant="default-filled">
                  <t-radio-button v-for="size in fileSizeOptions" :key="size" :value="size">
                    {{ size }} MB
                  </t-radio-button>
                </t-radio-group>
                <div class="field-meta">
                  <span>限制后续上传到此知识库的每个文件</span>
                </div>
              </div>

              <aside class="scope-preview" aria-label="知识库范围预览">
                <span class="preview-label">知识范围</span>
                <div class="preview-folder"><t-icon name="folder-open" size="28px" /></div>
                <strong>{{ form.name.trim() || '未命名知识库' }}</strong>
                <p>{{ form.description.trim() || '等待填写内容说明' }}</p>
                <dl>
                  <div><dt>文档上限</dt><dd>单文件 {{ form.maxFileSizeMB }} MB</dd></div>
                  <div><dt>原始文件</dt><dd>本地存储</dd></div>
                  <div><dt>检索索引</dt><dd>PostgreSQL + pgvector</dd></div>
                </dl>
              </aside>
            </section>

            <section v-else-if="step === 2" class="stage stage-index">
              <div class="stage-heading compact">
                <span class="stage-kicker">02 / 索引</span>
                <h1>选择入库后生成的知识结构</h1>
                <p>混合检索始终启用，为快速问答提供稳定的文档召回。</p>
              </div>

              <div class="capability-board">
                <article class="capability-row fixed">
                  <div class="capability-icon"><t-icon name="search" size="20px" /></div>
                  <div class="capability-copy">
                    <div><strong>混合检索</strong><span class="status-tag">始终启用</span></div>
                    <p>同时生成向量和关键词索引，负责快速问答的基础召回。</p>
                  </div>
                  <t-icon name="check-circle-filled" class="enabled-icon" size="22px" />
                </article>

              </div>

              <div class="model-section">
                <div class="model-section-heading">
                  <div>
                    <strong>全局默认模型</strong>
                    <span>自动使用模型设置中的默认项</span>
                  </div>
                  <button type="button" class="text-button" @click="openModelSettings">
                    <t-icon name="setting" /> 管理模型
                  </button>
                </div>
                <div class="model-fields">
                  <div class="model-field">
                    <label>问答与知识合成模型</label>
                    <strong>{{ selectedChatModelName }}</strong>
                    <span>用于生成回答与文档摘要</span>
                  </div>
                  <div class="model-field">
                    <label>向量模型</label>
                    <strong>{{ selectedEmbeddingModelName }}</strong>
                    <span>用于文档分段向量化和语义召回</span>
                  </div>
                </div>
                <div v-if="!modelsLoading && !modelsReady" class="model-warning">
                  <t-icon name="error-circle" />
                  <span>创建知识库前需要先配置默认问答模型和默认向量模型。</span>
                </div>
              </div>
            </section>

            <section v-else class="stage stage-review">
              <div class="stage-heading compact">
                <span class="stage-kicker">03 / 确认</span>
                <h1>确认知识库工作方式</h1>
                <p>创建完成后即可上传文档，系统会按下面的策略自动入库。</p>
              </div>

              <div class="review-layout">
                <section class="review-primary">
                  <div class="review-title">
                    <span class="review-folder"><t-icon name="folder-open" size="22px" /></span>
                    <div>
                      <strong>{{ form.name }}</strong>
                      <p>{{ form.description || '暂无内容说明' }}</p>
                    </div>
                  </div>
                  <dl class="review-list">
                    <div><dt>文档处理</dt><dd>自动解析 · 父子分段 · 混合检索</dd></div>
                    <div><dt>单文件上限</dt><dd>{{ form.maxFileSizeMB }} MB</dd></div>
                    <div><dt>原始文档</dt><dd>本地文件系统</dd></div>
                  </dl>
                </section>

                <section class="review-models">
                  <span class="review-section-label">模型路由</span>
                  <div>
                    <span><t-icon name="chat" /> 问答与合成</span>
                    <strong>{{ selectedChatModelName }}</strong>
                  </div>
                  <div>
                    <span><t-icon name="chart-bubble" /> 向量化</span>
                    <strong>{{ selectedEmbeddingModelName }}</strong>
                  </div>
                  <p><t-icon name="info-circle" /> 后续更换模型时需执行一次完整重建，系统会复用兼容的向量缓存。</p>
                </section>
              </div>
            </section>
          </main>

          <footer class="workbench-footer">
            <span class="footer-note">
              <t-icon name="secured" /> 配置仅保存在当前本地工作区
            </span>
            <div class="footer-actions">
              <t-button v-if="step > 1" variant="outline" @click="step -= 1">上一步</t-button>
              <t-button v-else variant="outline" @click="close">取消</t-button>
              <t-button v-if="step < 3" theme="primary" @click="nextStep">
                下一步 <template #suffix><t-icon name="chevron-right" /></template>
              </t-button>
              <t-button v-else theme="primary" :loading="saving" @click="submit">
                <template #icon><t-icon name="add" /></template>
                创建知识库
              </t-button>
            </div>
          </footer>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { createKnowledgeBase } from '@/api/knowledge-base'
import { listModels, type ModelConfig } from '@/api/model'
import { useUIStore } from '@/stores/ui'
import { MAX_FILE_SIZE_MB } from '@/utils'

const props = defineProps<{
  visible: boolean
}>()

const emit = defineEmits<{
  (event: 'update:visible', value: boolean): void
  (event: 'success', kbId: string): void
}>()

const uiStore = useUIStore()
const step = ref(1)
const highestReachableStep = ref(1)
const models = ref<ModelConfig[]>([])
const modelsLoading = ref(false)
const saving = ref(false)

const steps = [
  { id: 1, title: '基础信息', subtitle: '定义知识范围' },
  { id: 2, title: '索引能力', subtitle: '选择生成方式' },
  { id: 3, title: '确认创建', subtitle: '检查模型与存储' },
]

const fileSizeOptions = computed(() => {
  const options = [10, 25, 50, 75, 100].filter((size) => size <= MAX_FILE_SIZE_MB)
  if (!options.includes(MAX_FILE_SIZE_MB)) options.push(MAX_FILE_SIZE_MB)
  return options.sort((a, b) => a - b)
})

const form = reactive({
  name: '',
  description: '',
  maxFileSizeMB: Math.min(50, MAX_FILE_SIZE_MB),
})

const chatModels = computed(() => models.value.filter((model) => model.type === 'KnowledgeQA' && model.id))
const embeddingModels = computed(() => models.value.filter((model) => model.type === 'Embedding' && model.id))

const modelLabel = (model: ModelConfig) => model.display_name?.trim() || model.name
const defaultChatModel = computed(() => chatModels.value.find((model) => model.is_default))
const defaultEmbeddingModel = computed(() => embeddingModels.value.find((model) => model.is_default))
const modelsReady = computed(() => !!defaultChatModel.value && !!defaultEmbeddingModel.value)
const selectedChatModelName = computed(() => {
  return defaultChatModel.value ? modelLabel(defaultChatModel.value) : '未配置'
})
const selectedEmbeddingModelName = computed(() => {
  return defaultEmbeddingModel.value ? modelLabel(defaultEmbeddingModel.value) : '未配置'
})

const loadAvailableModels = async () => {
  modelsLoading.value = true
  try {
    models.value = await listModels()
  } catch (error: any) {
    models.value = []
    MessagePlugin.error(error?.message || '模型列表加载失败')
  } finally {
    modelsLoading.value = false
  }
}

const reset = () => {
  step.value = 1
  highestReachableStep.value = 1
  saving.value = false
  form.name = ''
  form.description = ''
  form.maxFileSizeMB = Math.min(50, MAX_FILE_SIZE_MB)
}

const validateStep = (targetStep: number) => {
  if (targetStep >= 2 && !form.name.trim()) {
    MessagePlugin.warning('请先填写知识库名称')
    step.value = 1
    return false
  }
  if (targetStep >= 3 && !modelsReady.value) {
    MessagePlugin.warning('请先在模型设置中配置默认问答模型和默认向量模型')
    step.value = 2
    return false
  }
  return true
}

const nextStep = () => {
  const target = step.value + 1
  if (!validateStep(target)) return
  highestReachableStep.value = Math.max(highestReachableStep.value, target)
  step.value = target
}

const goToStep = (target: number) => {
  if (target > highestReachableStep.value || !validateStep(target)) return
  step.value = target
}

const close = () => emit('update:visible', false)

const openModelSettings = () => {
  close()
  window.setTimeout(() => uiStore.openSettings('models'), 180)
}

const submit = async () => {
  if (!validateStep(3) || saving.value) return
  saving.value = true
  try {
    const result: any = await createKnowledgeBase({
      name: form.name.trim(),
      description: form.description.trim(),
      type: 'document',
      max_file_size_mb: form.maxFileSizeMB,
      chunking_config: {
        chunk_size: 512,
        chunk_overlap: 80,
        separators: ['\n\n', '\n', '。', '！', '？', ';', '；'],
        enable_parent_child: true,
        parent_chunk_size: 4096,
        child_chunk_size: 384,
        strategy: 'auto',
      },
		indexing_strategy: {
			vector_enabled: true,
			keyword_enabled: true,
		},
    })
    if (!result?.success || !result?.data?.id) {
      throw new Error(result?.message || '知识库创建失败')
    }
    MessagePlugin.success('知识库已创建')
    close()
    emit('success', result.data.id)
  } catch (error: any) {
    MessagePlugin.error(error?.message || '知识库创建失败')
  } finally {
    saving.value = false
  }
}

const handleKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape' && props.visible && !saving.value) close()
}

watch(() => props.visible, (visible) => {
  if (!visible) return
  reset()
  void loadAvailableModels()
})

onMounted(() => window.addEventListener('keydown', handleKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', handleKeydown))
</script>

<style scoped lang="less">
.zeal-create-overlay {
  position: fixed;
  inset: 0;
  z-index: 2700;
  padding: 24px;
  display: grid;
  place-items: center;
  background: rgba(15, 23, 42, 0.58);
}

.zeal-create-workbench {
  width: min(1280px, calc(100vw - 48px));
  height: min(850px, calc(100vh - 48px));
  min-width: 1020px;
  min-height: 680px;
  display: grid;
  grid-template-rows: 92px minmax(0, 1fr) 72px;
  overflow: hidden;
  border: 1px solid #bfc9d8;
  border-radius: 8px;
  background: var(--td-bg-color-container, #fff);
  box-shadow: 0 24px 72px rgba(8, 22, 44, 0.24);
}

.workbench-header {
  display: grid;
  grid-template-columns: 245px minmax(600px, 1fr) 40px;
  align-items: center;
  gap: 28px;
  padding: 0 28px;
  border-bottom: 1px solid var(--td-component-stroke, #dfe5ec);
}

.brand-lockup {
  display: flex;
  align-items: center;
  gap: 11px;

  > div {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  strong { color: var(--td-text-color-primary, #172033); font-size: 16px; }
}

.brand-mark {
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  border-radius: 5px;
  background: #1268e3;
  color: #fff !important;
  font-size: 17px !important;
  font-weight: 800;
}

.step-track {
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  list-style: none;

  li { position: relative; }
  li:not(:last-child)::after {
    content: '';
    position: absolute;
    top: 17px;
    left: calc(50% + 42px);
    right: calc(-50% + 42px);
    height: 1px;
    background: #d8dee8;
  }

  li.done:not(:last-child)::after { background: #1268e3; }

  button {
    width: 100%;
    padding: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 9px;
    border: 0;
    background: transparent;
    color: var(--td-text-color-placeholder, #8a94a5);
    font: inherit;
    cursor: pointer;
  }

  button:disabled { cursor: default; }
  button > span:last-child { display: flex; flex-direction: column; align-items: flex-start; gap: 1px; }
  strong { font-size: 13px; font-weight: 650; }
  small { font-size: 11px; color: var(--td-text-color-placeholder, #8a94a5); }
  li.active button, li.done button { color: #1268e3; }
}

.step-number {
  width: 30px;
  height: 30px;
  z-index: 1;
  display: grid;
  place-items: center;
  border: 1px solid #cbd3df;
  border-radius: 50%;
  background: var(--td-bg-color-container, #fff);
  font-size: 12px;
  font-weight: 700;
}

.active .step-number, .done .step-number { border-color: #1268e3; }
.done .step-number { background: #1268e3; color: #fff; }

.icon-button {
  width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  border: 1px solid transparent;
  border-radius: 5px;
  background: transparent;
  color: var(--td-text-color-secondary, #596579);
  cursor: pointer;

  &:hover { border-color: #cbd5e1; background: #f5f7fa; color: #172033; }
}

.workbench-body {
  min-height: 0;
  overflow-y: auto;
  background: var(--td-bg-color-page, #f6f8fb);
}

.stage {
  width: min(1060px, calc(100% - 72px));
  min-height: 100%;
  margin: 0 auto;
  padding: 46px 0;
  box-sizing: border-box;
}

.stage-basic {
  display: grid;
  grid-template-columns: minmax(0, 1.55fr) minmax(290px, 0.75fr);
  grid-template-rows: auto 1fr;
  column-gap: 72px;
}

.stage-heading {
  grid-column: 1 / -1;
  margin-bottom: 38px;

  &.compact { margin-bottom: 30px; }
  h1 { margin: 7px 0 9px; color: var(--td-text-color-primary, #172033); font-size: 27px; line-height: 1.25; letter-spacing: 0; }
  p { margin: 0; color: var(--td-text-color-secondary, #687386); font-size: 14px; line-height: 1.7; }
}

.stage-kicker {
  color: #1268e3;
  font-size: 11px;
  font-weight: 750;
}

.basic-form {
  padding: 28px 32px;
  align-self: start;
  border: 1px solid var(--td-component-stroke, #dce2ea);
  border-radius: 8px;
  background: var(--td-bg-color-container, #fff);
}

.field-label {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 9px;
  color: var(--td-text-color-primary, #1d2738);
  font-size: 13px;
  font-weight: 650;

  &:not(:first-child) { margin-top: 27px; }
  span { color: #1268e3; font-size: 11px; font-weight: 500; }
  .optional { color: var(--td-text-color-placeholder, #8b95a5); }
}

.field-meta {
  margin-top: 7px;
  display: flex;
  justify-content: space-between;
  color: var(--td-text-color-placeholder, #8b95a5);
  font-size: 11px;
}

.file-limit-options {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(72px, 1fr));
  width: 100%;

  :deep(.t-radio-button) {
    min-width: 0;
    padding: 0 8px;
  }
}

.scope-preview {
  align-self: stretch;
  min-height: 350px;
  padding: 28px;
  border-left: 3px solid #1268e3;
  background: #edf4ff;
  color: #1d2b3f;

  .preview-label { display: block; margin-bottom: 28px; color: #4b6f9f; font-size: 10px; font-weight: 750; }
  > strong { display: block; margin-top: 16px; font-size: 18px; overflow-wrap: anywhere; }
  > p { min-height: 52px; margin: 8px 0 28px; color: #66758a; font-size: 13px; line-height: 1.6; overflow-wrap: anywhere; }
  dl { margin: 0; border-top: 1px solid #c9d8eb; }
  dl div { padding: 12px 0; display: flex; justify-content: space-between; gap: 16px; border-bottom: 1px solid #d6e1ef; }
  dt { color: #6d7c91; font-size: 11px; }
  dd { margin: 0; color: #2b3d56; font-size: 11px; font-weight: 650; text-align: right; }
}

.preview-folder {
  width: 52px;
  height: 52px;
  display: grid;
  place-items: center;
  border-radius: 6px;
  background: #1268e3;
  color: #fff;
}

.capability-board {
  border: 1px solid var(--td-component-stroke, #dce2ea);
  border-radius: 8px;
  overflow: hidden;
  background: var(--td-bg-color-container, #fff);
}

.capability-row {
  min-height: 92px;
  padding: 18px 22px;
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr) auto;
  align-items: center;
  gap: 16px;
  border-bottom: 1px solid var(--td-component-stroke, #e1e6ed);
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
  strong { color: var(--td-text-color-primary, #1d2738); font-size: 14px; }
  span { color: var(--td-text-color-placeholder, #8490a2); font-size: 11px; }
  p { margin: 5px 0 0; color: var(--td-text-color-secondary, #657185); font-size: 12px; line-height: 1.55; }
  .status-tag { padding: 2px 6px; border-radius: 3px; background: #e8f1ff; color: #1268e3; }
}

.enabled-icon { color: #1268e3; }

.model-section {
  margin-top: 22px;
  border-top: 3px solid #1b293d;
  background: var(--td-bg-color-container, #fff);
}

.model-section-heading {
  padding: 19px 22px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border: 1px solid var(--td-component-stroke, #dce2ea);
  border-top: 0;

  > div { display: flex; flex-direction: column; gap: 3px; }
  strong { font-size: 14px; color: var(--td-text-color-primary, #1d2738); }
  span { color: var(--td-text-color-placeholder, #8490a2); font-size: 11px; }
}

.text-button {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  border: 0;
  background: transparent;
  color: #1268e3;
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}

.model-fields {
  padding: 22px;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
  border: 1px solid var(--td-component-stroke, #dce2ea);
  border-top: 0;
}

.model-field {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  label { color: var(--td-text-color-primary, #1d2738); font-size: 12px; font-weight: 650; }
  > span { color: var(--td-text-color-placeholder, #8590a1); font-size: 11px; }
}

.model-warning {
  padding: 10px 14px;
  display: flex;
  align-items: center;
  gap: 8px;
  border: 1px solid #f0c77b;
  border-top: 0;
  background: #fff9ec;
  color: #8d5a08;
  font-size: 12px;
}

.review-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.45fr) minmax(300px, 0.75fr);
  gap: 24px;
}

.review-primary, .review-models {
  border: 1px solid var(--td-component-stroke, #dce2ea);
  border-radius: 8px;
  background: var(--td-bg-color-container, #fff);
}

.review-title {
  padding: 24px;
  display: flex;
  align-items: flex-start;
  gap: 14px;
  border-bottom: 1px solid var(--td-component-stroke, #e0e5ec);
  strong { display: block; color: var(--td-text-color-primary, #1d2738); font-size: 17px; overflow-wrap: anywhere; }
  p { margin: 5px 0 0; color: var(--td-text-color-secondary, #697589); font-size: 12px; line-height: 1.55; overflow-wrap: anywhere; }
}

.review-folder {
  width: 40px;
  height: 40px;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  border-radius: 5px;
  background: #1268e3;
  color: #fff;
}

.review-list {
  margin: 0;
  padding: 7px 24px;
  div { padding: 15px 0; display: flex; justify-content: space-between; gap: 24px; border-bottom: 1px solid var(--td-component-stroke, #edf0f4); }
  div:last-child { border-bottom: 0; }
  dt { color: var(--td-text-color-secondary, #687386); font-size: 12px; }
  dd { margin: 0; color: var(--td-text-color-primary, #263247); font-size: 12px; font-weight: 650; text-align: right; }
  dd.muted { color: var(--td-text-color-placeholder, #919aaa); font-weight: 500; }
}

.review-models {
  padding: 24px;
  border-top: 3px solid #1268e3;
  .review-section-label { display: block; margin-bottom: 18px; color: #1268e3; font-size: 10px; font-weight: 750; }
  > div { padding: 15px 0; border-bottom: 1px solid var(--td-component-stroke, #e4e9ef); }
  > div span { display: flex; align-items: center; gap: 6px; color: var(--td-text-color-placeholder, #8590a1); font-size: 11px; }
  > div strong { display: block; margin-top: 6px; color: var(--td-text-color-primary, #223047); font-size: 13px; overflow-wrap: anywhere; }
  > p { margin: 18px 0 0; display: flex; align-items: flex-start; gap: 7px; color: var(--td-text-color-secondary, #6a7689); font-size: 11px; line-height: 1.55; }
}

.workbench-footer {
  padding: 0 28px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-top: 1px solid var(--td-component-stroke, #dfe5ec);
  background: var(--td-bg-color-container, #fff);
}

.footer-note {
  display: flex;
  align-items: center;
  gap: 7px;
  color: var(--td-text-color-placeholder, #8a95a6);
  font-size: 11px;
}

.footer-actions { display: flex; align-items: center; gap: 10px; }

.zeal-create-fade-enter-active, .zeal-create-fade-leave-active { transition: opacity 0.18s ease; }
.zeal-create-fade-enter-from, .zeal-create-fade-leave-to { opacity: 0; }

@media (max-height: 760px) {
  .stage { padding-top: 30px; padding-bottom: 30px; }
  .stage-heading { margin-bottom: 24px; }
  .capability-row { min-height: 78px; padding-top: 13px; padding-bottom: 13px; }
}

@media (max-width: 1080px) {
  .zeal-create-workbench {
    min-width: 0;
    width: calc(100vw - 32px);
    height: calc(100vh - 32px);
  }
  .workbench-header {
    grid-template-columns: 170px minmax(0, 1fr) 36px;
    gap: 18px;
    padding-inline: 20px;
  }
  .step-track small { display: none; }
  .stage { width: calc(100% - 48px); }
  .stage-basic { column-gap: 34px; grid-template-columns: minmax(0, 1.35fr) minmax(250px, 0.75fr); }
}

@media (max-width: 760px) {
  .zeal-create-overlay { padding: 0; place-items: stretch; }
  .zeal-create-workbench {
    width: 100vw;
    height: 100dvh;
    min-height: 0;
    grid-template-rows: 78px minmax(0, 1fr) 64px;
    border: 0;
    border-radius: 0;
  }
  .workbench-header {
    padding: 0 12px 0 16px;
    grid-template-columns: minmax(0, 1fr) 36px;
    gap: 8px;
  }
  .brand-lockup { display: none; }
  .step-track { min-width: 0; }
  .step-track button { gap: 5px; }
  .step-track button > span:last-child { align-items: center; }
  .step-track strong { font-size: 10px; }
  .step-track li:not(:last-child)::after {
    left: calc(50% + 24px);
    right: calc(-50% + 24px);
  }
  .step-number { width: 28px; height: 28px; }
  .stage {
    width: calc(100% - 32px);
    min-height: 0;
    padding: 24px 0 32px;
  }
  .stage-heading { margin-bottom: 22px; }
  .stage-heading.compact { margin-bottom: 20px; }
  .stage-heading h1 { font-size: 23px; }
  .stage-heading p { font-size: 12px; line-height: 1.55; }
  .stage-basic { display: block; }
  .basic-form { padding: 20px 18px; }
  .scope-preview {
    min-height: 0;
    margin-top: 14px;
    padding: 20px;
    border-top: 3px solid var(--zeal-primary, #1769dc);
    border-left: 0;
  }
  .scope-preview .preview-label { margin-bottom: 16px; }
  .preview-folder { width: 44px; height: 44px; }
  .file-limit-options { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .capability-row { min-height: 84px; padding: 14px; grid-template-columns: 36px minmax(0, 1fr) auto; gap: 10px; }
  .capability-icon { width: 34px; height: 34px; }
  .capability-copy p { display: -webkit-box; overflow: hidden; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
  .model-section-heading { padding: 16px; }
  .model-fields { padding: 16px; grid-template-columns: 1fr; gap: 18px; }
  .review-layout { grid-template-columns: 1fr; }
  .workbench-footer { padding: 0 14px; }
  .footer-note { display: none; }
  .footer-actions { width: 100%; justify-content: flex-end; }
}
</style>
