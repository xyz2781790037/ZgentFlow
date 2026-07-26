<template>
  <Teleport to="body">
    <Transition name="model-workbench-fade">
      <div v-if="visible" class="model-workbench-overlay">
        <section class="model-workbench" role="dialog" aria-modal="true" :aria-label="title">
          <header class="model-workbench__header">
            <div class="header-identity">
              <span class="header-mark">
                <slot name="headerIcon"><t-icon v-if="icon" :name="icon" /></slot>
              </span>
              <div>
                <span class="header-kicker">ZEALRAG / 模型服务</span>
                <h2>{{ title }}</h2>
              </div>
            </div>
            <p>{{ description }}</p>
            <t-tooltip content="关闭" placement="bottom">
              <button type="button" class="close-button" aria-label="关闭" @click="handleCancel">
                <t-icon name="close" size="18px" />
              </button>
            </t-tooltip>
          </header>

          <div class="model-workbench__body">
            <slot />
          </div>

          <footer v-if="!hideFooter" class="model-workbench__footer">
            <div class="footer-left"><slot name="footer-left" /></div>
            <div class="footer-actions">
              <t-button variant="outline" @click="handleCancel">
                {{ cancelText || t('common.cancel') }}
              </t-button>
              <t-button
                theme="primary"
                :loading="confirmLoading"
                :disabled="confirmDisabled"
                @click="$emit('confirm')"
              >
                <template #icon><t-icon name="check" /></template>
                {{ confirmText || t('common.save') }}
              </t-button>
            </div>
          </footer>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'

const props = withDefaults(defineProps<{
  visible: boolean
  title: string
  description?: string
  icon?: string
  confirmLoading?: boolean
  confirmDisabled?: boolean
  confirmText?: string
  cancelText?: string
  hideFooter?: boolean
}>(), {
  description: '',
  icon: 'control-platform',
  confirmLoading: false,
  confirmDisabled: false,
  confirmText: '',
  cancelText: '',
  hideFooter: false,
})

const emit = defineEmits<{
  (event: 'update:visible', value: boolean): void
  (event: 'confirm'): void
  (event: 'cancel'): void
}>()

const { t } = useI18n()

const handleCancel = () => {
  emit('cancel')
  emit('update:visible', false)
}

const handleKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape' && props.visible && !props.confirmLoading) handleCancel()
}

onMounted(() => window.addEventListener('keydown', handleKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', handleKeydown))
</script>

<style scoped lang="less">
.model-workbench-overlay {
  position: fixed;
  inset: 0;
  z-index: 2600;
  padding: 26px;
  display: grid;
  place-items: center;
  background: rgba(14, 24, 39, 0.62);
}

.model-workbench {
  width: min(980px, calc(100vw - 52px));
  height: min(860px, calc(100vh - 52px));
  min-width: 820px;
  min-height: 640px;
  display: grid;
  grid-template-rows: 98px minmax(0, 1fr) 68px;
  overflow: hidden;
  border: 1px solid #bdc8d8;
  border-radius: 6px;
  background: var(--td-bg-color-container, #fff);
  box-shadow: 0 26px 76px rgba(6, 18, 36, 0.28);
}

.model-workbench__header {
  padding: 0 24px;
  display: grid;
  grid-template-columns: minmax(300px, 1fr) minmax(260px, 0.85fr) 36px;
  align-items: center;
  gap: 24px;
  border-bottom: 1px solid var(--td-component-stroke, #dce2ea);
}

.header-identity {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 13px;

  > div { min-width: 0; }
  h2 {
    margin: 4px 0 0;
    color: var(--td-text-color-primary, #182235);
    font-size: 18px;
    line-height: 1.25;
    letter-spacing: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.header-mark {
  width: 42px;
  height: 42px;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  border-radius: 5px;
  background: #1268e3;
  color: #fff;
  font-size: 19px;
}

.header-kicker {
  display: block;
  color: #1268e3;
  font-size: 9px;
  font-weight: 750;
}

.model-workbench__header > p {
  margin: 0;
  color: var(--td-text-color-secondary, #697589);
  font-size: 12px;
  line-height: 1.55;
}

.close-button {
  width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  border: 1px solid transparent;
  border-radius: 5px;
  background: transparent;
  color: var(--td-text-color-secondary, #667286);
  cursor: pointer;

  &:hover { border-color: #ccd5e1; background: #f5f7fa; color: #172033; }
}

.model-workbench__body {
  min-height: 0;
  padding: 28px clamp(30px, 6vw, 74px) 36px;
  overflow-y: auto;
  background: var(--td-bg-color-page, #f7f9fc);
}

.model-workbench__body :deep(.t-form) {
  width: min(720px, 100%);
  margin: 0 auto;
}

.model-workbench__body :deep(.setting-drawer__section) {
  padding: 23px 0 27px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  border-bottom: 1px solid var(--td-component-stroke, #dce2ea);

  &:first-child { padding-top: 0; }
  &:last-child { padding-bottom: 0; border-bottom: 0; }
}

.model-workbench__body :deep(.setting-drawer__section-title) {
  margin: 0 0 3px;
  display: flex;
  align-items: center;
  gap: 9px;
  color: var(--td-text-color-primary, #1d283a);
  font-size: 13px;
  font-weight: 700;

  &::before {
    content: '';
    width: 3px;
    height: 15px;
    background: #1268e3;
  }
}

.model-workbench__footer {
  padding: 0 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  border-top: 1px solid var(--td-component-stroke, #dce2ea);
  background: var(--td-bg-color-container, #fff);
}

.footer-left {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 9px;
  flex: 1;
}

.footer-actions { display: flex; align-items: center; gap: 10px; flex: 0 0 auto; }

.model-workbench-fade-enter-active, .model-workbench-fade-leave-active { transition: opacity 0.18s ease; }
.model-workbench-fade-enter-from, .model-workbench-fade-leave-to { opacity: 0; }

@media (max-width: 680px) {
  .model-workbench-overlay {
    padding: 0;
    place-items: stretch;
  }
  .model-workbench {
    width: 100vw;
    height: 100dvh;
    min-width: 0;
    min-height: 0;
    grid-template-rows: 88px minmax(0, 1fr) 68px;
    border: 0;
    border-radius: 0;
  }
  .model-workbench__header {
    min-width: 0;
    padding: 0 16px;
    grid-template-columns: minmax(0, 1fr) 36px;
    gap: 12px;
  }
  .model-workbench__header > p { display: none; }
  .header-mark { width: 40px; height: 40px; }
  .header-identity h2 { font-size: 17px; }
  .model-workbench__body {
    min-width: 0;
    padding: 22px 16px 30px;
    overflow-x: hidden;
  }
  .model-workbench__body :deep(.t-form) { min-width: 0; }
  .model-workbench__body :deep(.setting-drawer__section) { padding: 20px 0 23px; }
  .model-workbench__body :deep(.model-type-options) {
    width: 100%;
    min-width: 0;
    padding-bottom: 4px;
    flex-wrap: nowrap;
    overflow-x: auto;
  }
  .model-workbench__body :deep(.model-type-option) { flex: 0 0 auto; }
  .model-workbench__body :deep(.form-item),
  .model-workbench__body :deep(.t-input),
  .model-workbench__body :deep(.t-select),
  .model-workbench__body :deep(.t-select__wrap) { min-width: 0; max-width: 100%; }
  .model-workbench__footer {
    min-width: 0;
    padding: 0 12px;
    gap: 8px;
  }
  .footer-left { flex: 1 1 auto; overflow: hidden; }
  .footer-left :deep(.footer-test-message) { display: none; }
  .footer-left :deep(.t-button) { min-width: 0; padding-inline: 10px; }
  .footer-actions { gap: 6px; }
  .footer-actions :deep(.t-button) { min-width: 68px; padding-inline: 12px; }
}
</style>
