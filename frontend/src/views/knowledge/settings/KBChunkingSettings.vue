<template>
  <div class="kb-chunking-settings" :class="{ 'kb-chunking-settings--embedded': embedded }">
    <header v-if="!embedded" class="chunking-header">
      <div>
        <span class="chunking-eyebrow">SEGMENT LAB</span>
        <h2>内容切分实验台</h2>
        <p>{{ $t('knowledgeEditor.chunking.description') }}</p>
      </div>
      <KBChunkingDebug :config="debugConfig" />
    </header>

    <div v-if="!embedded" class="chunking-console">
      <section class="strategy-deck">
        <div class="console-section-heading">
          <span>01</span>
          <div><strong>选择切分策略</strong><small>决定系统如何识别标题、段落与语义边界</small></div>
        </div>
        <div class="strategy-cards">
          <button
            v-for="option in strategyOptions"
            :key="option.value"
            type="button"
            :class="{ active: localStrategy === option.value }"
            @click="localStrategy = option.value; handleStrategyChange()"
          >
            <span class="strategy-radio"><i></i></span>
            <strong>{{ option.label }}</strong>
            <small>{{ option.tooltip }}</small>
          </button>
        </div>
      </section>

      <div class="chunking-workbench">
        <section class="chunk-preview">
          <div class="console-section-heading">
            <span>02</span>
            <div><strong>片段结构预览</strong><small>根据当前参数展示入库后的层级关系</small></div>
          </div>
          <div class="preview-document">
            <div class="preview-doc-head"><i></i><span>sample-document.md</span><small>{{ localChunkSize }} chars</small></div>
            <div class="preview-content">
              <div class="preview-line preview-line--title"></div>
              <div class="preview-line"></div>
              <div class="preview-line preview-line--short"></div>
              <div class="preview-chunks">
                <div v-for="block in 3" :key="block" class="preview-chunk" :class="`preview-chunk--${block}`">
                  <span>CHUNK {{ String(block).padStart(2, '0') }}</span>
                  <div><i></i><i></i><i></i></div>
                  <small>{{ block === 1 ? localChunkSize : Math.max(localChunkSize - block * localChunkOverlap, 100) }} 字符</small>
                </div>
              </div>
              <div v-if="localEnableParentChild" class="preview-parent-link">
                <t-icon name="git-branch" /><span>父块 {{ localParentChunkSize }} / 子块 {{ localChildChunkSize }}</span>
              </div>
            </div>
          </div>
          <div class="preview-metrics">
            <div><span>重叠比例</span><strong>{{ Math.round((localChunkOverlap / Math.max(localChunkSize, 1)) * 100) }}%</strong></div>
            <div><span>分隔符</span><strong>{{ localSeparators.length }}</strong></div>
            <div><span>层级</span><strong>{{ localEnableParentChild ? '2' : '1' }}</strong></div>
          </div>
        </section>

        <section class="parameter-console">
          <div class="console-section-heading">
            <span>03</span>
            <div><strong>调整核心参数</strong><small>滑块变化会同步更新左侧预览</small></div>
          </div>

          <div class="parameter-block">
            <div class="parameter-head"><div><strong>{{ $t('knowledgeEditor.chunking.sizeLabel') }}</strong><small>{{ $t('knowledgeEditor.chunking.sizeDescription') }}</small></div><output>{{ localChunkSize }}</output></div>
            <t-slider v-model="localChunkSize" :min="100" :max="4000" :step="50" @change="handleChunkSizeChange" />
          </div>

          <div class="parameter-block" :class="{ warning: overlapTooHigh }">
            <div class="parameter-head"><div><strong>{{ $t('knowledgeEditor.chunking.overlapLabel') }}</strong><small>{{ $t('knowledgeEditor.chunking.overlapDescription') }}</small></div><output>{{ localChunkOverlap }}</output></div>
            <t-slider v-model="localChunkOverlap" :min="0" :max="500" :step="20" @change="handleChunkOverlapChange" />
            <p v-if="overlapTooHigh" class="parameter-warning">{{ $t('knowledgeEditor.chunking.overlapWarning') }}</p>
          </div>

          <div class="parameter-block parameter-block--select">
            <div class="parameter-head"><div><strong>{{ $t('knowledgeEditor.chunking.separatorsLabel') }}</strong><small>{{ $t('knowledgeEditor.chunking.separatorsDescription') }}</small></div></div>
            <t-select v-model="localSeparators" :options="separatorOptions" multiple creatable filterable
              :placeholder="$t('knowledgeEditor.chunking.separatorsPlaceholder')" @change="handleSeparatorsChange" />
          </div>

          <div class="hierarchy-switch">
            <div><strong>{{ $t('knowledgeEditor.chunking.parentChildLabel') }}</strong><small>{{ $t('knowledgeEditor.chunking.parentChildDescription') }}</small></div>
            <t-switch v-model="localEnableParentChild" @change="handleParentChildChange" />
          </div>

          <div v-if="localEnableParentChild" class="hierarchy-controls">
            <div class="parameter-block">
              <div class="parameter-head"><div><strong>{{ $t('knowledgeEditor.chunking.parentChunkSizeLabel') }}</strong></div><output>{{ localParentChunkSize }}</output></div>
              <t-slider v-model="localParentChunkSize" :min="512" :max="8192" :step="128" @change="handleParentChunkSizeChange" />
            </div>
            <div class="parameter-block">
              <div class="parameter-head"><div><strong>{{ $t('knowledgeEditor.chunking.childChunkSizeLabel') }}</strong></div><output>{{ localChildChunkSize }}</output></div>
              <t-slider v-model="localChildChunkSize" :min="64" :max="2048" :step="32" @change="handleChildChunkSizeChange" />
            </div>
          </div>

          <button type="button" class="advanced-toggle" @click="advancedOpen = !advancedOpen">
            <chevron-right-icon class="toggle-arrow" :class="{ open: advancedOpen }" />
            <span>{{ $t('knowledgeEditor.chunking.advancedLabel') }}</span>
          </button>
          <div v-if="advancedOpen" class="advanced-console">
            <div :class="['advanced-field', { disabled: advancedDisabled }]">
              <label>{{ $t('knowledgeEditor.chunking.tokenLimitLabel') }}</label>
              <t-input-number v-model="localTokenLimit" :min="0" :max="8192" :step="64" :disabled="advancedDisabled" @change="handleTokenLimitChange" />
            </div>
            <div :class="['advanced-field', { disabled: advancedDisabled }]">
              <label>{{ $t('knowledgeEditor.chunking.languagesLabel') }}</label>
              <t-select v-model="localLanguages" :options="languageOptions" multiple :disabled="advancedDisabled"
                :placeholder="$t('knowledgeEditor.chunking.languagesPlaceholder')" @change="handleLanguagesChange" />
            </div>
          </div>
        </section>
      </div>
    </div>

    <div v-else class="settings-group">
      <!-- Strategy -->
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('knowledgeEditor.chunking.strategyLabel') }}</label>
          <p class="desc">{{ $t('knowledgeEditor.chunking.strategyDescription') }}</p>
        </div>
        <div class="setting-control strategy-control">
          <t-select
            v-model="localStrategy"
            :options="strategyOptions"
            :placeholder="$t('knowledgeEditor.chunking.strategyPlaceholder')"
            :clearable="true"
            @change="handleStrategyChange"
            :style="selectStyle"
          />
          <!-- Test trigger sits right next to the strategy picker so users
               discover it exactly when they're deciding which strategy to
               use on their content. -->
          <KBChunkingDebug v-if="!embedded" :config="debugConfig" />
        </div>
      </div>

      <!-- Strategy explanation panel -->
      <div v-if="currentStrategyInfo" class="strategy-info-panel">
        <p>
          <strong>{{ currentStrategyInfo.label }}:</strong>
          {{ currentStrategyInfo.tooltip }}
        </p>
      </div>

      <!-- Chunk Size -->
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('knowledgeEditor.chunking.sizeLabel') }}</label>
          <p class="desc">{{ $t('knowledgeEditor.chunking.sizeDescription') }}</p>
        </div>
        <div class="setting-control">
          <div class="slider-container">
            <t-slider
              v-model="localChunkSize"
              :min="100"
              :max="4000"
              :step="50"
              :marks="embedded ? undefined : chunkSizeMarks"
              @change="handleChunkSizeChange"
              :style="sliderStyle"
            />
            <span class="value-display">{{ localChunkSize }} {{ $t('knowledgeEditor.chunking.characters') }}</span>
          </div>
        </div>
      </div>

      <!-- Chunk Overlap -->
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('knowledgeEditor.chunking.overlapLabel') }}</label>
          <p class="desc">{{ $t('knowledgeEditor.chunking.overlapDescription') }}</p>
          <p v-if="overlapTooHigh" class="warn">{{ $t('knowledgeEditor.chunking.overlapWarning') }}</p>
        </div>
        <div class="setting-control">
          <div class="slider-container">
            <t-slider
              v-model="localChunkOverlap"
              :min="0"
              :max="500"
              :step="20"
              :marks="embedded ? undefined : chunkOverlapMarks"
              @change="handleChunkOverlapChange"
              :style="sliderStyle"
            />
            <span class="value-display">{{ localChunkOverlap }} {{ $t('knowledgeEditor.chunking.characters') }}</span>
          </div>
        </div>
      </div>

      <!-- Separators -->
      <div class="setting-row setting-row--separators">
        <div class="setting-info">
          <label>{{ $t('knowledgeEditor.chunking.separatorsLabel') }}</label>
          <p class="desc">{{ $t('knowledgeEditor.chunking.separatorsDescription') }}</p>
        </div>
        <div class="setting-control">
          <t-select
            v-model="localSeparators"
            :options="separatorOptions"
            multiple
            creatable
            filterable
            :placeholder="$t('knowledgeEditor.chunking.separatorsPlaceholder')"
            @change="handleSeparatorsChange"
            :style="selectStyle"
          />
        </div>
      </div>

      <!-- Parent-Child Chunking -->
      <div class="setting-row setting-row--toggle">
        <div class="setting-info">
          <label>{{ $t('knowledgeEditor.chunking.parentChildLabel') }}</label>
          <p class="desc">{{ $t('knowledgeEditor.chunking.parentChildDescription') }}</p>
        </div>
        <div class="setting-control">
          <t-switch
            v-model="localEnableParentChild"
            @change="handleParentChildChange"
          />
        </div>
      </div>

      <!-- Parent Chunk Size -->
      <div v-if="localEnableParentChild" class="setting-row">
        <div class="setting-info">
          <label>{{ $t('knowledgeEditor.chunking.parentChunkSizeLabel') }}</label>
          <p class="desc">{{ $t('knowledgeEditor.chunking.parentChunkSizeDescription') }}</p>
        </div>
        <div class="setting-control">
          <div class="slider-container">
            <t-slider
              v-model="localParentChunkSize"
              :min="512"
              :max="8192"
              :step="64"
              :marks="embedded ? undefined : parentChunkSizeMarks"
              @change="handleParentChunkSizeChange"
              :style="sliderStyle"
            />
            <span class="value-display">{{ localParentChunkSize }} {{ $t('knowledgeEditor.chunking.characters') }}</span>
          </div>
        </div>
      </div>

      <!-- Child Chunk Size -->
      <div v-if="localEnableParentChild" class="setting-row">
        <div class="setting-info">
          <label>{{ $t('knowledgeEditor.chunking.childChunkSizeLabel') }}</label>
          <p class="desc">{{ $t('knowledgeEditor.chunking.childChunkSizeDescription') }}</p>
        </div>
        <div class="setting-control">
          <div class="slider-container">
            <t-slider
              v-model="localChildChunkSize"
              :min="64"
              :max="2048"
              :step="32"
              :marks="embedded ? undefined : childChunkSizeMarks"
              @change="handleChildChunkSizeChange"
              :style="sliderStyle"
            />
            <span class="value-display">{{ localChildChunkSize }} {{ $t('knowledgeEditor.chunking.characters') }}</span>
          </div>
        </div>
      </div>

      <!-- Advanced section toggle -->
      <button type="button" class="advanced-toggle" @click="advancedOpen = !advancedOpen">
        <chevron-right-icon class="toggle-arrow" :class="{ open: advancedOpen }" />
        <span>{{ $t('knowledgeEditor.chunking.advancedLabel') }}</span>
      </button>

      <div v-if="advancedOpen" class="advanced-section">
        <!-- Token Limit -->
        <div class="setting-row" :class="{ disabled: advancedDisabled }">
          <div class="setting-info">
            <label>{{ $t('knowledgeEditor.chunking.tokenLimitLabel') }}</label>
            <p class="desc">{{ $t('knowledgeEditor.chunking.tokenLimitDescription') }}</p>
          </div>
          <div class="setting-control">
            <t-input-number
              v-model="localTokenLimit"
              :min="0"
              :max="8192"
              :step="64"
              :disabled="advancedDisabled"
              @change="handleTokenLimitChange"
              style="width: 200px;"
            />
          </div>
        </div>

        <!-- Languages -->
        <div class="setting-row" :class="{ disabled: advancedDisabled }">
          <div class="setting-info">
            <label>{{ $t('knowledgeEditor.chunking.languagesLabel') }}</label>
            <p class="desc">{{ $t('knowledgeEditor.chunking.languagesDescription') }}</p>
          </div>
          <div class="setting-control">
            <t-select
              v-model="localLanguages"
              :options="languageOptions"
              multiple
              :disabled="advancedDisabled"
              :placeholder="$t('knowledgeEditor.chunking.languagesPlaceholder')"
              @change="handleLanguagesChange"
              :style="selectStyle"
            />
          </div>
        </div>
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronRightIcon } from 'tdesign-icons-vue-next'
import KBChunkingDebug from './KBChunkingDebug.vue'

interface ParserEngineRule {
  file_types: string[]
  engine: string
}

// Slider ranges defined in this file (min/max props on t-slider) mirror
// the validated bounds in the backend splitter:
//   ChunkSize:      100–4000  (default 512). 100 = too fragmented to be
//                   useful; 4000 = approaches the 7500-char absoluteMaxSize
//                   that the splitter hard-caps to anyway.
//   ChunkOverlap:   0–500     (default 80). Backend caps to ChunkSize/2
//                   when set higher than that.
//   ParentChunkSize: 512–8192 (default 4096 ≈ 1000 EN tokens).
//   ChildChunkSize:  64–2048  (default 384 ≈ 80 EN tokens, sweet spot for
//                   sentence-transformer / BGE embedders).
//   TokenLimit:      0–8192   (default 0 = off, char-based budget only).
//                   Set to 200 for MiniLM (256-tok limit), 400 for BGE/
//                   Cohere (512-tok), leave at 0 for OpenAI/Voyage/Jina-v3.
interface ChunkingConfig {
  chunkSize: number
  chunkOverlap: number
  separators: string[]
  parserEngineRules?: ParserEngineRule[]
  enableParentChild: boolean
  parentChunkSize: number
  childChunkSize: number
  // Adaptive chunking strategy. Empty string = legacy / not set.
  strategy?: string
  // Cap chunk size in approx tokens. 0 = char-based budget only.
  tokenLimit?: number
  // Language hints for heuristic patterns (de/en/zh).
  languages?: string[]
}

interface Props {
  config: ChunkingConfig
  embedded?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  embedded: false,
})

const selectStyle = computed(() => (props.embedded ? { width: '100%' } : { width: '280px' }))
const sliderStyle = computed(() => (props.embedded ? { width: '100%' } : { width: '200px' }))

const chunkSizeMarks = { 100: '100', 1000: '1000', 2000: '2000', 4000: '4000' }
const chunkOverlapMarks = { 0: '0', 250: '250', 500: '500' }
const parentChunkSizeMarks = { 512: '512', 2048: '2048', 4096: '4096', 8192: '8192' }
const childChunkSizeMarks = { 64: '64', 384: '384', 1024: '1024', 2048: '2048' }

const emit = defineEmits<{
  'update:config': [value: ChunkingConfig]
}>()

const { t } = useI18n()

const localChunkSize = ref(props.config.chunkSize)
const localChunkOverlap = ref(props.config.chunkOverlap)
const localSeparators = ref([...props.config.separators])
const localEnableParentChild = ref(props.config.enableParentChild ?? false)
const localParentChunkSize = ref(props.config.parentChunkSize || 4096)
const localChildChunkSize = ref(props.config.childChunkSize || 384)
const localStrategy = ref(props.config.strategy ?? '')
const localTokenLimit = ref(props.config.tokenLimit ?? 0)
const localLanguages = ref<string[]>([...(props.config.languages ?? [])])
const advancedOpen = ref(false)

const strategyOptions = computed(() => [
  {
    label: t('knowledgeEditor.chunking.strategies.auto.label'),
    value: 'auto',
    tooltip: t('knowledgeEditor.chunking.strategies.auto.tooltip')
  },
  {
    label: t('knowledgeEditor.chunking.strategies.heading.label'),
    value: 'heading',
    tooltip: t('knowledgeEditor.chunking.strategies.heading.tooltip')
  },
  {
    label: t('knowledgeEditor.chunking.strategies.heuristic.label'),
    value: 'heuristic',
    tooltip: t('knowledgeEditor.chunking.strategies.heuristic.tooltip')
  },
  {
    label: t('knowledgeEditor.chunking.strategies.legacy.label'),
    value: 'legacy',
    tooltip: t('knowledgeEditor.chunking.strategies.legacy.tooltip')
  }
])

const currentStrategyInfo = computed(() => {
  if (!localStrategy.value) {
    return null
  }
  return strategyOptions.value.find(o => o.value === localStrategy.value) ?? null
})

const advancedDisabled = computed(() => localStrategy.value === 'legacy')

const overlapTooHigh = computed(
  () => localChunkOverlap.value > 0 && localChunkOverlap.value >= localChunkSize.value / 2
)

// Live config snapshot for the debug panel — uses current local form values
// so the panel reflects edits immediately without waiting for save.
const debugConfig = computed(() => ({
  chunkSize: localChunkSize.value,
  chunkOverlap: localChunkOverlap.value,
  separators: localSeparators.value,
  strategy: localStrategy.value,
  tokenLimit: localTokenLimit.value,
  languages: localLanguages.value
}))

const languageOptions = computed(() => [
  { label: t('knowledgeEditor.chunking.languageOptions.de'), value: 'de' },
  { label: t('knowledgeEditor.chunking.languageOptions.en'), value: 'en' },
  { label: t('knowledgeEditor.chunking.languageOptions.zh'), value: 'zh' }
])

const separatorOptions = computed(() => [
  { label: t('knowledgeEditor.chunking.separators.doubleNewline'), value: '\n\n' },
  { label: t('knowledgeEditor.chunking.separators.singleNewline'), value: '\n' },
  { label: t('knowledgeEditor.chunking.separators.periodCn'), value: '。' },
  { label: t('knowledgeEditor.chunking.separators.exclamationCn'), value: '！' },
  { label: t('knowledgeEditor.chunking.separators.questionCn'), value: '？' },
  { label: t('knowledgeEditor.chunking.separators.semicolonCn'), value: '；' },
  { label: t('knowledgeEditor.chunking.separators.semicolonEn'), value: ';' },
  { label: t('knowledgeEditor.chunking.separators.space'), value: ' ' }
])

watch(() => props.config, (newConfig) => {
  localChunkSize.value = newConfig.chunkSize
  localChunkOverlap.value = newConfig.chunkOverlap
  localSeparators.value = [...newConfig.separators]
  localEnableParentChild.value = newConfig.enableParentChild ?? false
  localParentChunkSize.value = newConfig.parentChunkSize || 4096
  localChildChunkSize.value = newConfig.childChunkSize || 384
  localStrategy.value = newConfig.strategy ?? ''
  localTokenLimit.value = newConfig.tokenLimit ?? 0
  localLanguages.value = [...(newConfig.languages ?? [])]
}, { deep: true })

const handleChunkSizeChange = () => { emitUpdate() }
const handleChunkOverlapChange = () => { emitUpdate() }
const handleSeparatorsChange = () => { emitUpdate() }
const handleParentChildChange = () => { emitUpdate() }
const handleParentChunkSizeChange = () => { emitUpdate() }
const handleChildChunkSizeChange = () => { emitUpdate() }
const handleStrategyChange = () => { emitUpdate() }
const handleTokenLimitChange = () => { emitUpdate() }
const handleLanguagesChange = () => { emitUpdate() }

const emitUpdate = () => {
  // Spread arrays so the parent gets its own copy. Mutating the emitted
  // arrays from outside must not leak back into our reactive state and
  // cause two-way ref drift between the form and the editor model.
  emit('update:config', {
    chunkSize: localChunkSize.value,
    chunkOverlap: localChunkOverlap.value,
    separators: [...localSeparators.value],
    parserEngineRules: props.config.parserEngineRules,
    enableParentChild: localEnableParentChild.value,
    parentChunkSize: localParentChunkSize.value,
    childChunkSize: localChildChunkSize.value,
    strategy: localStrategy.value,
    tokenLimit: localTokenLimit.value,
    languages: [...localLanguages.value]
  })
}
</script>

<style lang="less" scoped>
.kb-chunking-settings {
  width: 100%;
}

.section-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  // Stick to the top of the scrollable section so the section title remains
  // visible while the user scrolls through a long form. Negative top and
  // matching negative margins compensate for content-wrapper's padding
  // (24px 32px) so the sticky band visually spans the full width when stuck.
  position: sticky;
  top: -24px;
  z-index: 5;
  background: var(--td-bg-color-container);
  padding: 24px 32px 12px 32px;
  margin: -24px -32px 16px -32px;
  border-bottom: 1px solid var(--td-component-stroke);

  .section-header-text {
    flex: 1;
    min-width: 0;
  }

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
  gap: 0;
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

  &.disabled {
    opacity: 0.5;
  }
}

.strategy-info-panel {
  margin: 0 0 16px 0;
  padding: 10px 14px;
  background: var(--td-bg-color-container-hover);
  border-left: 3px solid var(--td-brand-color);
  border-radius: 0 4px 4px 0;
  font-size: 13px;
  color: var(--td-text-color-secondary);
  line-height: 1.5;

  p {
    margin: 0;
    word-break: break-word;
  }

  strong {
    color: var(--td-text-color-primary);
  }
}

.setting-info {
  flex: 0 0 40%;
  max-width: 40%;
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

  .warn {
    font-size: 12px;
    color: var(--td-warning-color);
    margin: 4px 0 0 0;
    line-height: 1.4;
  }
}

.setting-control {
  flex: 0 0 55%;
  max-width: 55%;
  display: flex;
  justify-content: flex-end;
  align-items: center;
}

// Strategy row stacks the picker above the test trigger so the action has
// room to breathe without competing with the select for horizontal space.
// Both children stay right-aligned under the section's right column.
.strategy-control {
  flex-direction: column;
  align-items: flex-end;
  gap: 6px;
}

.slider-container {
  display: flex;
  align-items: center;
  gap: 16px;
  width: 100%;
  justify-content: flex-end;
}

.value-display {
  font-size: 14px;
  color: var(--td-text-color-primary);
  font-weight: 500;
  min-width: 80px;
  text-align: right;
}

.advanced-toggle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 16px 0 8px 0;
  margin: 0;
  background: transparent;
  border: none;
  cursor: pointer;
  font-size: 14px;
  font-weight: 500;
  color: var(--td-text-color-secondary);
  user-select: none;

  &:hover {
    color: var(--td-text-color-primary);
  }

  &:focus-visible {
    outline: 2px solid var(--td-brand-color-focus);
    outline-offset: 2px;
    border-radius: 4px;
  }
}

.toggle-arrow {
  font-size: 16px;
  transition: transform 0.15s ease;

  &.open {
    transform: rotate(90deg);
  }
}

.advanced-section {
  // Visually grouped via the toggle above; avoid a left rule that hugs the
  // panel edge and looks detached from the rest of the form.
  margin-top: 4px;
}

.chunking-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  padding-bottom: 20px;
  border-bottom: 1px solid #dce5f0;

  h2 { margin: 5px 0 7px; color: #172033; font-size: 22px; line-height: 1.3; letter-spacing: 0; }
  p { margin: 0; color: #6f7c90; font-size: 13px; line-height: 1.6; }
}

.chunking-eyebrow { color: #1268e3; font-family: var(--app-font-family-mono); font-size: 10px; font-weight: 700; }
.chunking-console { padding-top: 20px; }
.console-section-heading { display: flex; align-items: flex-start; gap: 10px; margin-bottom: 13px; }
.console-section-heading > span { width: 28px; height: 22px; display: grid; place-items: center; flex: 0 0 auto; color: #1268e3; background: #e9f2ff; font-family: var(--app-font-family-mono); font-size: 10px; font-weight: 700; }
.console-section-heading > div { display: grid; gap: 3px; }
.console-section-heading strong { color: #25344a; font-size: 13px; }
.console-section-heading small { color: #7d899a; font-size: 11px; line-height: 1.4; }

.strategy-deck { margin-bottom: 18px; }
.strategy-cards { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px; }
.strategy-cards button { min-width: 0; min-height: 90px; display: grid; grid-template-columns: 16px minmax(0, 1fr); align-content: start; gap: 5px 8px; padding: 11px; border: 1px solid #dce4ee; border-radius: 6px; color: #4f5f75; background: #fff; text-align: left; cursor: pointer; }
.strategy-cards button:hover { border-color: #9fbde5; }
.strategy-cards button.active { border-color: #1268e3; background: #f3f7ff; box-shadow: inset 0 0 0 1px #1268e3; }
.strategy-cards strong { overflow: hidden; color: #29384e; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.strategy-cards small { grid-column: 1 / -1; display: -webkit-box; overflow: hidden; color: #7d899a; font-size: 10px; line-height: 1.45; -webkit-line-clamp: 2; -webkit-box-orient: vertical; }
.strategy-radio { width: 14px; height: 14px; display: grid; place-items: center; border: 1px solid #aeb9c8; border-radius: 50%; }
.strategy-radio i { width: 6px; height: 6px; border-radius: 50%; background: transparent; }
.strategy-cards button.active .strategy-radio { border-color: #1268e3; }
.strategy-cards button.active .strategy-radio i { background: #1268e3; }

.chunking-workbench { display: grid; grid-template-columns: minmax(300px, 0.9fr) minmax(360px, 1.1fr); gap: 14px; align-items: start; }
.chunk-preview, .parameter-console { min-width: 0; padding: 15px; border: 1px solid #dce4ee; border-radius: 6px; background: #fff; }
.chunk-preview { background: #f7f9fc; }
.preview-document { overflow: hidden; border: 1px solid #d5dfeb; border-radius: 5px; background: #fff; }
.preview-doc-head { height: 34px; display: flex; align-items: center; gap: 7px; padding: 0 10px; border-bottom: 1px solid #e5ebf2; color: #5f6e82; font-family: var(--app-font-family-mono); font-size: 10px; }
.preview-doc-head i { width: 7px; height: 7px; border-radius: 50%; background: #1268e3; }
.preview-doc-head small { margin-left: auto; color: #96a1af; }
.preview-content { padding: 14px; }
.preview-line { width: 88%; height: 5px; margin-bottom: 7px; border-radius: 2px; background: #dfe6ef; }
.preview-line--title { width: 48%; height: 7px; background: #9bb8dd; }
.preview-line--short { width: 65%; }
.preview-chunks { display: grid; gap: 7px; margin-top: 14px; }
.preview-chunk { display: grid; grid-template-columns: 58px minmax(0, 1fr) auto; align-items: center; gap: 8px; padding: 8px; border-left: 3px solid #1268e3; background: #f1f6fd; }
.preview-chunk--2 { border-left-color: #0d8b91; background: #eef8f8; }
.preview-chunk--3 { border-left-color: #536b91; background: #f1f3f7; }
.preview-chunk > span { color: #45617f; font-family: var(--app-font-family-mono); font-size: 8px; font-weight: 700; }
.preview-chunk > div { display: grid; gap: 3px; }
.preview-chunk > div i { width: 100%; height: 3px; background: rgba(80, 106, 137, 0.2); }
.preview-chunk small { color: #6f7c8d; font-size: 9px; white-space: nowrap; }
.preview-parent-link { display: flex; align-items: center; gap: 6px; margin-top: 10px; padding: 7px 8px; border: 1px dashed #9fb5d0; color: #526b8b; font-size: 10px; }
.preview-metrics { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 6px; margin-top: 9px; }
.preview-metrics div { display: grid; gap: 3px; padding: 7px; border: 1px solid #e0e7f0; background: #fff; }
.preview-metrics span { color: #8994a4; font-size: 9px; }
.preview-metrics strong { color: #2e4058; font-size: 13px; }

.parameter-console { display: grid; gap: 11px; }
.parameter-block { padding: 10px 11px 13px; border: 1px solid #e1e7ef; background: #fbfcfe; }
.parameter-block.warning { border-color: #e2b27c; background: #fffaf3; }
.parameter-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 9px; }
.parameter-head > div { display: grid; gap: 2px; }
.parameter-head strong { color: #344359; font-size: 11px; }
.parameter-head small { color: #8792a2; font-size: 9px; line-height: 1.4; }
.parameter-head output { min-width: 48px; padding: 4px 6px; color: #0d55bd; background: #e9f2ff; font-family: var(--app-font-family-mono); font-size: 11px; text-align: center; }
.parameter-warning { margin: 8px 0 0; color: #a86528; font-size: 10px; }
.hierarchy-switch { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 11px; border-left: 3px solid #0d8b91; background: #f2f8f8; }
.hierarchy-switch > div { display: grid; gap: 2px; }
.hierarchy-switch strong { color: #314654; font-size: 11px; }
.hierarchy-switch small { color: #7b8e94; font-size: 9px; }
.hierarchy-controls { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.advanced-console { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; padding: 10px; border-top: 1px solid #e2e8f0; background: #f7f9fc; }
.advanced-field { min-width: 0; display: grid; gap: 6px; }
.advanced-field label { color: #5b687a; font-size: 10px; }
.advanced-field.disabled { opacity: 0.5; }

@media (max-width: 1040px) {
  .strategy-cards { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .chunking-workbench { grid-template-columns: 1fr; }
}

.kb-chunking-settings--embedded {
  .setting-row {
    flex-direction: column;
    align-items: stretch;
    gap: 10px;
    padding: 14px 0;
  }

  .setting-row--toggle {
    flex-direction: row;
    align-items: center;
    gap: 16px;

    .setting-info {
      flex: 1;
      min-width: 0;
    }

    .setting-control {
      flex: none;
      width: auto;
      justify-content: flex-end;
    }
  }

  .setting-row--separators {
    padding-bottom: 18px;
  }

  .setting-info {
    flex: none;
    max-width: none;
    padding-right: 0;

    label {
      font-size: 14px;
    }

    .desc {
      font-size: 12px;
    }
  }

  .setting-control {
    flex: none;
    max-width: none;
    justify-content: flex-start;
    align-items: stretch;
    width: 100%;
  }

  .strategy-control {
    flex-direction: column;
    align-items: stretch;
    width: 100%;
  }

  .strategy-info-panel {
    margin: -4px 0 10px;
    padding: 8px 12px;
    font-size: 12px;
  }

  .slider-container {
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
    width: 100%;
    justify-content: flex-start;
  }

  .value-display {
    order: -1;
    text-align: left;
    min-width: 0;
  }

  :deep(.t-slider) {
    padding-bottom: 0;
  }

  :deep(.t-select__wrap) {
    max-width: 100%;
  }

  :deep(.t-tag) {
    max-width: 100%;
  }

  .advanced-toggle {
    padding-top: 10px;
  }

  .advanced-section .setting-row {
    padding: 14px 0;
  }
}
</style>
