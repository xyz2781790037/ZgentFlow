<template>
  <div class="kb-upload-source-dropdown">
    <input
      ref="fileInputRef"
      type="file"
      class="hidden-file-input"
      multiple
      :accept="acceptFileTypes || undefined"
      @change="(e) => handleFilesChange(e, false)"
    />
    <input
      ref="folderInputRef"
      type="file"
      class="hidden-file-input"
      webkitdirectory
      multiple
      @change="(e) => handleFilesChange(e, true)"
    />

    <t-tooltip :content="tooltipText" placement="top">
      <t-dropdown
        :options="dropdownOptions"
        trigger="click"
        :placement="placement"
        @click="handleActionSelect"
      >
        <t-button
          variant="text"
          theme="default"
          :class="['kb-upload-source-trigger', triggerClass]"
          size="small"
        >
          <template #icon><t-icon :name="triggerIcon" size="16px" /></template>
        </t-button>
      </t-dropdown>
    </t-tooltip>

  </div>
</template>

<script setup lang="ts">
import { ref, computed, h } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin, Icon as TIcon } from 'tdesign-vue-next'
import { filterUploadFiles } from '../utils/uploadSources'

const props = withDefaults(defineProps<{
  acceptFileTypes?: string
  supportedFileTypes?: string[]
  maxFileSizeMb?: number
  triggerIcon?: string
  triggerClass?: string
  tooltip?: string
  placement?: 'top' | 'bottom' | 'bottom-right' | 'bottom-left'
}>(), {
  acceptFileTypes: '',
  supportedFileTypes: () => [],
  maxFileSizeMb: 50,
  triggerIcon: 'file-add',
  triggerClass: '',
  tooltip: '',
  placement: 'bottom-right',
})

const emit = defineEmits<{
  files: [files: File[]]
}>()

const { t } = useI18n()

const fileInputRef = ref<HTMLInputElement | null>(null)
const folderInputRef = ref<HTMLInputElement | null>(null)

const tooltipText = computed(() => props.tooltip || t('knowledgeBase.addDocument'))

const dropdownOptions = computed(() => {
  return [
    {
      content: t('upload.uploadDocument'),
      value: 'upload',
      prefixIcon: () => h(TIcon, { name: 'upload', size: '16px' }),
    },
    {
      content: t('upload.uploadFolder'),
      value: 'uploadFolder',
      prefixIcon: () => h(TIcon, { name: 'folder-add', size: '16px' }),
    },
  ]
})

const handleActionSelect = (data: { value: string }) => {
  switch (data.value) {
    case 'upload':
      fileInputRef.value?.click()
      break
    case 'uploadFolder':
      folderInputRef.value?.click()
      break
    default:
      break
  }
}

const notifyFilterResult = (result: ReturnType<typeof filterUploadFiles>, emptyAllSkippedKey: string) => {
  const { validFiles, skippedCount, unsupportedVideoCount } = result
  if (validFiles.length === 0) {
    if (skippedCount > 0) {
      MessagePlugin.warning(t(emptyAllSkippedKey))
    }
    return false
  }
  if (unsupportedVideoCount > 0) {
    MessagePlugin.warning(t('knowledgeBase.unsupportedVideos', { count: unsupportedVideoCount }))
  }
  if (skippedCount > 0) {
    MessagePlugin.warning(t('knowledgeBase.filesSkippedNoEngine', { count: skippedCount }))
  }
  return true
}

const handleFilesChange = (event: Event, fromFolder: boolean) => {
  const input = event.target as HTMLInputElement
  const files = input.files
  if (!files || files.length === 0) return

  const result = filterUploadFiles(files, {
    supportedFileTypes: props.supportedFileTypes,
    maxFileSizeMB: props.maxFileSizeMb,
    fromFolder,
    multiFile: files.length > 1,
  })

  if (!notifyFilterResult(result, 'knowledgeBase.allFilesSkippedNoEngine')) {
    input.value = ''
    return
  }

  emit('files', result.validFiles)
  input.value = ''
}

</script>

<style lang="less" scoped>
.hidden-file-input {
  position: absolute;
  width: 0;
  height: 0;
  opacity: 0;
  pointer-events: none;
}

.kb-upload-source-trigger {
  color: var(--td-text-color-secondary);

  &:hover {
    color: var(--td-brand-color);
  }
}

</style>
