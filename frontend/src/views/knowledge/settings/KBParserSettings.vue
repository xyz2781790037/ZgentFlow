<template>
  <div class="kb-parser-settings" :class="{ 'kb-parser-settings--embedded': embedded }">
    <header v-if="!embedded" class="parser-header">
      <div>
        <span class="parser-eyebrow">DOCUMENT INTAKE</span>
        <h2>文档解析工作台</h2>
        <p>{{ $t('kbSettings.parser.description') }}</p>
      </div>
      <div class="parser-header-status">
        <span><i></i>{{ fileTypeGroups.length }} 类文件</span>
        <strong>{{ parserEngines.filter(engine => engine.Available !== false).length }}</strong>
        <small>可用引擎</small>
      </div>
    </header>

    <div v-if="loading" class="loading-inline">
      <t-loading size="small" />
      <span>{{ $t('kbSettings.parser.loading') }}</span>
    </div>

    <div v-else-if="fileTypeGroups.length === 0" class="empty-hint">
      <p>{{ $t('kbSettings.parser.noEngineAvailable') }}</p>
    </div>

    <div v-else class="parser-canvas">
      <div v-if="!embedded" class="parser-canvas-note">
        <span class="note-index">RULES</span>
        <div>
          <strong>按文件类型指定解析引擎</strong>
          <p>上传时会自动识别扩展名，并按下方规则进入对应解析链路。</p>
        </div>
        <t-icon name="arrow-down" />
      </div>

      <div class="file-type-matrix">
        <article v-for="group in fileTypeGroups" :key="group.key" class="file-type-tile">
          <header class="file-type-head">
            <span class="file-type-icon"><t-icon :name="group.icon" /></span>
            <div>
              <h3>{{ group.label }}</h3>
              <div class="ext-tags">
                <span v-for="ext in group.extensions" :key="ext" class="ext-tag">.{{ ext }}</span>
              </div>
            </div>
            <span class="engine-availability" :class="{ unavailable: !hasAvailableEngine(group.extensions) }">
              <i></i>{{ hasAvailableEngine(group.extensions) ? '可解析' : '需配置' }}
            </span>
          </header>

          <div class="engine-current">
            <span>当前引擎</span>
            <strong>{{ getEngineDisplayName(getEngineForGroup(group.extensions) || '未配置') }}</strong>
          </div>

          <div class="engine-picker">
            <label>切换解析链路</label>
          <t-select
            :value="getEngineForGroup(group.extensions) || undefined"
            @change="(val: string) => handleEngineChange(group.extensions, val)"
            style="width: 100%"
            :status="hasAvailableEngine(group.extensions) ? 'default' : 'warning'"
            :placeholder="$t('kbSettings.parser.noEngine')"
          >
            <t-option
              v-for="opt in getEngineOptions(group.extensions)"
              :key="opt.value"
              :value="opt.value"
              :label="opt.selectLabel"
              :disabled="opt.disabled"
            >
              <t-tooltip
                :content="$t('kbSettings.supportedFormats') + ': ' + opt.fileTypes.map(ft => '.' + ft).join('  ')"
                placement="left"
                :show-arrow="false"
              >
                <div class="engine-option">
                  <div class="engine-option-top">
                    <span class="engine-option-name">{{ getEngineDisplayName(opt.value) }}</span>
                    <t-tag
                      v-if="opt.isDefault"
                      theme="primary"
                      variant="light"
                      size="small"
                    >{{ $t('kbSettings.parser.default') }}</t-tag>
                    <t-tag
                      v-if="opt.disabled"
                      theme="danger"
                      variant="light"
                      size="small"
                    >{{ $t('kbSettings.parser.unavailable') }}</t-tag>
                  </div>
                  <div class="engine-option-desc">{{ getEngineDisplayDesc(opt.value, opt.desc) }}</div>
                  <div v-if="opt.disabled && opt.reason" class="engine-option-reason">
                    {{ opt.reason }}
                    <a class="go-settings" @click.stop.prevent="goToParserSettings">{{ $t('kbSettings.parser.goSettings') }}</a>
                  </div>
                </div>
              </t-tooltip>
            </t-option>
          </t-select>
          <div v-if="!hasAvailableEngine(group.extensions)" class="no-engine-warning">
            <a class="go-settings" @click.prevent="goToParserSettings">{{ $t('kbSettings.parser.goConfig') }}</a>
          </div>
          </div>
        </article>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { type ParserEngineInfo } from '@/api/system'
import { useEditorResourcesStore } from '@/stores/editorResources'
import { useUIStore } from '@/stores/ui'
import { storeToRefs } from 'pinia'

const { t } = useI18n()
const editorResources = useEditorResourcesStore()

function getEngineDisplayName(engineName: string): string {
  const key = `kbSettings.parser.engines.${engineName}.name`
  const translated = t(key)
  return translated !== key ? translated : engineName
}

function getEngineDisplayDesc(engineName: string, fallback: string): string {
  const key = `kbSettings.parser.engines.${engineName}.desc`
  const translated = t(key)
  return translated !== key ? translated : fallback
}

export interface ParserEngineRule {
  file_types: string[]
  engine: string
}

interface EngineOption {
  value: string
  selectLabel: string
  desc: string
  fileTypes: string[]
  disabled: boolean
  isDefault: boolean
  reason?: string
}

interface Props {
  parserEngineRules?: ParserEngineRule[]
  /** Compact layout for upload-confirm dialog */
  embedded?: boolean
  /** When set, only show file-type groups matching these extensions */
  relevantExtensions?: string[]
}

const props = withDefaults(defineProps<Props>(), {
  parserEngineRules: () => [],
  embedded: false,
  relevantExtensions: () => [],
})

const emit = defineEmits<{
  'update:parserEngineRules': [value: ParserEngineRule[]]
}>()

const uiStore = useUIStore()
const localEngineRules = ref<ParserEngineRule[]>([...props.parserEngineRules])
const parserEngines = ref<ParserEngineInfo[]>([])
const loading = ref(true)

const allFileTypes = computed(() => {
  const s = new Set<string>()
  for (const engine of parserEngines.value) {
    for (const ft of engine.FileTypes || []) {
      s.add(ft)
    }
  }
  return s
})

const fileTypeGroups = computed(() => {
  const ft = allFileTypes.value
  const groups: { key: string; label: string; icon: string; extensions: string[] }[] = []

  const pdfExts = ['pdf'].filter(e => ft.has(e))
  const officeExts = ['docx', 'doc'].filter(e => ft.has(e))
  const pptExts = ['pptx', 'ppt'].filter(e => ft.has(e))
  const excelExts = ['xlsx', 'xls'].filter(e => ft.has(e))
  const ebookExts = ['epub'].filter(e => ft.has(e))
  const webArchiveExts = ['mhtml'].filter(e => ft.has(e))
  const csvExts = ['csv'].filter(e => ft.has(e))
  const mdExts = ['md', 'markdown'].filter(e => ft.has(e))
  const txtExts = ['txt'].filter(e => ft.has(e))
  const jsonExts = ['json'].filter(e => ft.has(e))
  const imageExts = ['jpg', 'jpeg', 'png', 'gif', 'bmp', 'tiff', 'webp'].filter(e => ft.has(e))

  if (pdfExts.length) groups.push({ key: 'pdf', label: t('kbSettings.parser.fileTypePdf'), icon: 'file-pdf', extensions: pdfExts })
  if (officeExts.length) groups.push({ key: 'office', label: t('kbSettings.parser.fileTypeWord'), icon: 'file-word', extensions: officeExts })
  if (pptExts.length) groups.push({ key: 'ppt', label: t('kbSettings.parser.fileTypePpt'), icon: 'file-powerpoint', extensions: pptExts })
  if (excelExts.length) groups.push({ key: 'excel', label: t('kbSettings.parser.fileTypeExcel'), icon: 'file-excel', extensions: excelExts })
  if (ebookExts.length) groups.push({ key: 'ebook', label: t('kbSettings.parser.fileTypeEbook'), icon: 'file', extensions: ebookExts })
  if (webArchiveExts.length) groups.push({ key: 'webarchive', label: t('kbSettings.parser.fileTypeWebArchive'), icon: 'file', extensions: webArchiveExts })
  if (csvExts.length) groups.push({ key: 'csv', label: t('kbSettings.parser.fileTypeCsv'), icon: 'file-excel', extensions: csvExts })
  if (mdExts.length) groups.push({ key: 'markdown', label: 'Markdown', icon: 'file-code', extensions: mdExts })
  if (txtExts.length) groups.push({ key: 'text', label: t('kbSettings.parser.fileTypeText'), icon: 'file', extensions: txtExts })
  if (jsonExts.length) groups.push({ key: 'json', label: t('kbSettings.parser.fileTypeJson'), icon: 'file-code', extensions: jsonExts })
  if (imageExts.length) groups.push({ key: 'image', label: t('kbSettings.parser.fileTypeImage'), icon: 'image', extensions: imageExts })
  const rel = props.relevantExtensions
  if (!rel?.length) return groups
  const relSet = new Set(rel)
  const filtered = groups.filter(g => g.extensions.some(e => relSet.has(e)))
  return filtered.length > 0 ? filtered : groups
})

function getEngineOptions(extensions: string[]): EngineOption[] {
  const raw: { name: string; desc: string; fileTypes: string[]; available: boolean; reason: string }[] = []
  for (const engine of parserEngines.value) {
    const supports = extensions.some(ext => (engine.FileTypes || []).includes(ext))
    if (supports) {
      raw.push({
        name: engine.Name,
        desc: engine.Description || engine.Name,
        fileTypes: engine.FileTypes || [],
        available: engine.Available !== false,
        reason: engine.UnavailableReason || '',
      })
    }
  }
  const defaultName = raw.find(e => e.available)?.name ?? ''
  return raw.map(e => ({
    value: e.name,
    selectLabel: `${getEngineDisplayName(e.name)}  —  ${getEngineDisplayDesc(e.name, e.desc)}`,
    desc: e.desc,
    fileTypes: e.fileTypes,
    disabled: !e.available,
    isDefault: defaultName !== '' && e.name === defaultName,
    reason: e.reason,
  }))
}

function hasAvailableEngine(extensions: string[]): boolean {
  return getEngineOptions(extensions).some(opt => !opt.disabled)
}

function getDefaultEngine(extensions: string[]): string {
  const opts = getEngineOptions(extensions)
  return opts.find(o => o.isDefault)?.value ?? ''
}

function getEngineForGroup(extensions: string[]): string {
  for (const rule of localEngineRules.value) {
    if (rule.file_types.some(ft => extensions.includes(ft))) {
      return rule.engine
    }
  }
  return getDefaultEngine(extensions)
}

function handleEngineChange(extensions: string[], engine: string) {
  const otherRules = localEngineRules.value.filter(
    r => !r.file_types.some(ft => extensions.includes(ft))
  )
  if (engine) {
    otherRules.push({ file_types: [...extensions], engine })
  }
  localEngineRules.value = otherRules
  emit('update:parserEngineRules', buildCompleteRules())
}

function buildCompleteRules(): ParserEngineRule[] {
  const rules: ParserEngineRule[] = []
  for (const group of fileTypeGroups.value) {
    const engine = getEngineForGroup(group.extensions)
    if (engine) {
      rules.push({ file_types: [...group.extensions], engine })
    }
  }
  return rules
}

function goToParserSettings() {
  uiStore.openSettings('parser')
}

async function loadEngines(force = false) {
  loading.value = true
  try {
    await editorResources.ensureParserEngines(force)
    parserEngines.value = editorResources.parserEngines as ParserEngineInfo[]
  } catch {
    parserEngines.value = []
  } finally {
    loading.value = false
    ensureCompleteRules()
  }
}

function ensureCompleteRules() {
  if (!parserEngines.value.length) return
  const complete = buildCompleteRules()
  if (complete.length && complete.length > localEngineRules.value.length) {
    localEngineRules.value = complete
    emit('update:parserEngineRules', complete)
  }
}

onMounted(loadEngines)

const { showSettingsModal } = storeToRefs(uiStore)
watch(showSettingsModal, (open, wasOpen) => {
  if (wasOpen && !open) {
    loadEngines(true)
  }
})

watch(() => props.parserEngineRules, (v) => {
  localEngineRules.value = v?.length ? [...v] : []
}, { deep: true })
</script>

<style lang="less" scoped>
.kb-parser-settings {
  width: 100%;
}

.parser-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  padding: 0 0 22px;
  border-bottom: 1px solid #dce5f0;

  h2 { margin: 5px 0 7px; color: #172033; font-size: 22px; line-height: 1.3; letter-spacing: 0; }
  p { margin: 0; color: #6f7c90; font-size: 13px; line-height: 1.6; }
}

.parser-eyebrow { color: #1268e3; font-family: var(--app-font-family-mono); font-size: 10px; font-weight: 700; }

.parser-header-status {
  min-width: 170px;
  display: grid;
  grid-template-columns: 1fr auto;
  align-items: end;
  gap: 2px 12px;
  padding: 12px 14px;
  border: 1px solid #d8e3f1;
  border-radius: 6px;
  background: #f7faff;

  span { grid-column: 1 / -1; display: flex; align-items: center; gap: 6px; color: #52647d; font-size: 11px; }
  span i { width: 6px; height: 6px; border-radius: 50%; background: #14a06f; }
  strong { color: #0d55bd; font-size: 24px; line-height: 1; }
  small { color: #7a8799; font-size: 11px; }
}

.loading-inline {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 16px 0;
}

.empty-hint {
  padding: 24px 0;
  color: var(--td-text-color-secondary);
}

.parser-canvas { padding-top: 20px; }

.parser-canvas-note {
  display: grid;
  grid-template-columns: 50px minmax(0, 1fr) auto;
  align-items: center;
  gap: 14px;
  margin-bottom: 16px;
  padding: 12px 15px;
  border-left: 3px solid #1268e3;
  background: #f3f7fc;

  strong { color: #26354b; font-size: 13px; }
  p { margin: 3px 0 0; color: #7a8799; font-size: 11px; }
  > .t-icon { color: #7890ae; }
}

.note-index { color: #1268e3; font-family: var(--app-font-family-mono); font-size: 10px; font-weight: 700; }

.file-type-matrix { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }

.file-type-tile {
  min-width: 0;
  padding: 14px;
  border: 1px solid #dce4ef;
  border-radius: 6px;
  background: #fff;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;

  &:hover { border-color: #a9c4e9; box-shadow: 0 4px 14px rgba(32, 67, 111, 0.07); }
}

.file-type-head { display: grid; grid-template-columns: 36px minmax(0, 1fr) auto; align-items: start; gap: 10px; }
.file-type-icon { width: 34px; height: 34px; display: grid; place-items: center; border-radius: 5px; color: #1268e3; background: #eaf2ff; font-size: 17px; }
.file-type-head h3 { margin: 0 0 6px; color: #26354b; font-size: 14px; letter-spacing: 0; }
.ext-tags { display: flex; flex-wrap: wrap; gap: 4px; }
.ext-tag { color: #6f7d90; font-family: var(--app-font-family-mono); font-size: 10px; }

.engine-availability { display: flex; align-items: center; gap: 5px; color: #16805e; font-size: 10px; white-space: nowrap; }
.engine-availability i { width: 5px; height: 5px; border-radius: 50%; background: currentColor; }
.engine-availability.unavailable { color: #b36b2c; }

.engine-current { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin: 14px 0 10px; padding: 9px 10px; border-top: 1px solid #edf1f6; border-bottom: 1px solid #edf1f6; }
.engine-current span { color: #8792a3; font-size: 10px; }
.engine-current strong { overflow: hidden; color: #34445b; font-family: var(--app-font-family-mono); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.engine-picker { display: grid; gap: 6px; }
.engine-picker > label { color: #65738a; font-size: 10px; font-weight: 600; }

.no-engine-warning {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 8px;
  font-size: 12px;
  color: var(--td-warning-color);
  line-height: 1.4;

  .go-settings {
    color: var(--td-brand-color);
    cursor: pointer;
    white-space: nowrap;
    text-decoration: none;

    &:hover {
      text-decoration: underline;
    }
  }
}

// ---- 下拉选项样式 ----
.engine-option {
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 3px 0;
}

.engine-option-top {
  display: flex;
  align-items: center;
  gap: 6px;
}

.engine-option-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  font-family: var(--app-font-family-mono);
}

.engine-option-desc {
  font-size: 12px;
  color: var(--td-text-color-placeholder);
  line-height: 1.4;
}

.engine-option-reason {
  font-size: 12px;
  color: var(--td-error-color);
  line-height: 1.4;

  .go-settings {
    color: var(--td-brand-color);
    cursor: pointer;
    margin-left: 4px;
    font-size: 12px;
    text-decoration: none;

    &:hover {
      text-decoration: underline;
    }
  }
}

.kb-parser-settings--embedded {
  .parser-canvas { padding-top: 0; }
  .file-type-matrix { grid-template-columns: 1fr; }
  .file-type-tile { padding: 12px; }
}

@media (max-width: 900px) { .file-type-matrix { grid-template-columns: 1fr; } }
</style>

<style lang="less">
.t-select__dropdown .t-select-option {
  height: auto;
  align-items: flex-start;
  padding-top: 6px;
  padding-bottom: 6px;
}
.t-select__dropdown .t-select-option__content {
  white-space: normal;
}
</style>
