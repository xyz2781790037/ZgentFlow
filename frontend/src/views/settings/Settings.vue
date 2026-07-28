<template>
  <Teleport to="body">
    <Transition name="settings-fade">
      <div v-if="visible" class="zeal-settings-overlay">
        <section class="zeal-settings-workbench settings-v2" aria-label="ZgentFlow 设置">
          <aside class="settings-v2-rail">
            <div class="settings-identity">
              <span class="settings-mark">Z</span>
              <div>
                <strong>设置</strong>
                <small>ZgentFlow</small>
              </div>
            </div>
            <nav class="settings-tabs" aria-label="设置导航">
              <button
                v-for="item in navItems"
                :key="item.key"
                type="button"
                :class="{ active: currentSection === item.key }"
                @click="handleNavClick(item)"
              >
                <span class="tab-icon"><t-icon :name="item.icon" /></span>
                <span>
                  <strong>{{ item.label }}</strong>
                  <small>{{ SECTION_META[item.key].short }}</small>
                </span>
                <t-icon class="settings-tab-arrow" name="chevron-right" />
              </button>
            </nav>
            <span class="settings-runtime"><i></i>本地工作区</span>
          </aside>

          <div class="settings-v2-stage">
            <header class="zeal-settings-header">
              <div class="settings-heading">
                <h1>{{ currentSectionMeta.title }}</h1>
                <p>{{ currentSectionMeta.description }}</p>
              </div>
              <t-tooltip content="关闭设置" placement="bottom">
                <button class="zeal-settings-close" type="button" aria-label="关闭设置" @click="handleClose">
                  <t-icon name="close" size="18px" />
                </button>
              </t-tooltip>
            </header>

            <main class="zeal-settings-content">
              <div class="settings-section">
                <ModelSettings v-if="currentSection === 'models'" />
                <ModelAPIConfigSettings v-else-if="currentSection === 'api-configs'" />
                <WebSearchSettings v-else-if="currentSection === 'websearch'" />
                <RetrievalSettings v-else-if="currentSection === 'retrieval'" />
                <PromptSettings v-else-if="currentSection === 'prompts'" />
              </div>
            </main>
          </div>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUIStore } from '@/stores/ui'
import ModelSettings from './ModelSettings.vue'
import ModelAPIConfigSettings from './ModelAPIConfigSettings.vue'
import WebSearchSettings from './WebSearchSettings.vue'
import RetrievalSettings from './RetrievalSettings.vue'
import PromptSettings from './PromptSettings.vue'

const route = useRoute()
const router = useRouter()
const uiStore = useUIStore()

const currentSection = ref<string>('models')

const SECTION_META: Record<string, { title: string; description: string; short: string }> = {
  models: {
    title: '模型服务',
    description: '管理问答、向量化、重排和多模态模型。',
    short: '模型与连接凭证',
  },
  'api-configs': {
    title: 'API 管理',
    description: '集中维护可供多个模型持续引用的 API Key。',
    short: '共享密钥与模型引用',
  },
  websearch: {
    title: '网页搜索',
    description: '配置深度问答可使用的外部搜索服务。',
    short: '深度问答外部检索',
  },
  retrieval: {
    title: '检索参数',
    description: '统一配置向量召回、关键词召回和重排参数。',
    short: '召回阈值与重排',
  },
  prompts: {
    title: '提示词',
    description: '编辑实际运行的提示词，查看版本历史并回滚。',
    short: '编辑、版本与回滚',
  },
}

const currentSectionMeta = computed(() =>
  SECTION_META[currentSection.value] || SECTION_META.general
)

type NavItem = {
  key: string
  icon: string
  label: string
}

const navItems = computed(() => {
  return [
    { key: 'models', icon: 'control-platform', label: '模型服务' },
    { key: 'api-configs', icon: 'key', label: 'API 管理' },
    { key: 'websearch', icon: 'internet', label: '网页搜索' },
    { key: 'retrieval', icon: 'filter', label: '检索参数' },
    { key: 'prompts', icon: 'edit-2', label: '提示词' },
  ] as NavItem[]
})

const LEGACY_SECTION_FALLBACK: Record<string, string> = {
  ollama: 'models',
  parser: 'models',
  storage: 'models',
  vectorstore: 'models',
  system: 'models',
  'system-global': 'models',
}

const resolveSection = (requested?: string) => {
  const candidate = requested ? (LEGACY_SECTION_FALLBACK[requested] || requested) : 'models'
  return navItems.value.some((item) => item.key === candidate)
    ? candidate
    : (navItems.value[0]?.key || 'general')
}

const handleNavClick = (item: NavItem) => {
  currentSection.value = resolveSection(item.key)
}

// 控制弹窗显示
const visible = computed(() => {
  return route.path === '/platform/settings' || uiStore.showSettingsModal
})

// 关闭弹窗
const handleClose = () => {
  uiStore.closeSettings()
  // 如果当前路由是设置页，返回上一页
  if (route.path === '/platform/settings') {
    const sec = route.query.section
    if (sec === 'system-global') {
      router.push('/platform/knowledge-bases')
    } else {
      router.back()
    }
  }
}

// 监听初始导航设置
watch(() => uiStore.settingsInitialSection, (section) => {
  if (visible.value) currentSection.value = resolveSection(section ?? undefined)
}, { immediate: true })

watch(
  () => [visible.value, route.query.section],
  ([isVisible, section]) => {
    if (!isVisible || typeof section !== 'string') return
    currentSection.value = resolveSection(section)
  },
  { immediate: true },
)

// 切换租户后角色可能变化，原本可见的 admin-only 面板可能消失。
// 如果 currentSection 落到了不再显示的 key 上，就回退到第一个可见项。
watch(navItems, (items) => {
  if (!items.some((item) => item.key === currentSection.value)) {
    currentSection.value = items[0]?.key || 'models'
  }
})

// ESC 键关闭
const handleEscape = (e: KeyboardEvent) => {
  if (e.key === 'Escape' && visible.value) {
    handleClose()
  }
}

// 处理快捷导航事件
const handleSettingsNav = (e: CustomEvent) => {
  currentSection.value = resolveSection(e.detail?.section)
}

onMounted(() => {
  window.addEventListener('keydown', handleEscape)
  window.addEventListener('settings-nav', handleSettingsNav as EventListener)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleEscape)
  window.removeEventListener('settings-nav', handleSettingsNav as EventListener)
})
</script>

<style lang="less" scoped>
.settings-mark {
  width: 32px;
  height: 32px;
  display: grid;
  place-items: center;
  border-radius: 6px;
  background: #1268e3;
  color: #fff;
  font-size: 17px;
  font-weight: 800;
}

.role-denied {
  min-height: 320px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #7d8797;
  text-align: center;
}

.settings-fade-enter-active,
.settings-fade-leave-active {
  transition: opacity 0.16s ease;
}

.settings-fade-enter-active .zeal-settings-workbench,
.settings-fade-leave-active .zeal-settings-workbench {
  transition: transform 0.16s ease, opacity 0.16s ease;
}

.settings-fade-enter-from,
.settings-fade-leave-to {
  opacity: 0;
}

.settings-fade-enter-from .zeal-settings-workbench,
.settings-fade-leave-to .zeal-settings-workbench {
  opacity: 0;
  transform: translateY(8px);
}

.zeal-settings-overlay {
  position: fixed;
  inset: 0;
  z-index: 2300;
  padding: 28px;
  display: grid;
  place-items: center;
  background: rgba(14, 24, 39, 0.6);
}

.zeal-settings-workbench {
  width: min(1240px, calc(100vw - 56px));
  height: min(840px, calc(100vh - 56px));
  min-width: 980px;
  min-height: 650px;
  display: grid;
  grid-template-rows: 100px 78px minmax(0, 1fr);
  overflow: hidden;
  border: 1px solid #bdc8d8;
  border-radius: 6px;
  background: var(--td-bg-color-container, #fff);
  box-shadow: 0 26px 72px rgba(7, 20, 39, 0.26);
}

.zeal-settings-header {
  padding: 0 28px;
  display: grid;
  grid-template-columns: 210px minmax(0, 1fr) 36px;
  align-items: center;
  gap: 34px;
  border-bottom: 1px solid var(--td-component-stroke, #dfe5ec);
  background: var(--td-bg-color-container, #fff);
}

.settings-identity {
  display: flex;
  align-items: center;
  gap: 12px;

  > div { display: flex; flex-direction: column; gap: 2px; }
  strong { color: var(--td-text-color-primary, #172033); font-size: 18px; line-height: 1.2; }
}

.settings-heading {
  min-width: 0;
  h1 { margin: 0 0 5px; color: var(--td-text-color-primary, #172033); font-size: 20px; letter-spacing: 0; }
  p { margin: 0; color: var(--td-text-color-secondary, #6f7a8d); font-size: 12px; }
}

.zeal-settings-close {
  width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  border: 1px solid transparent;
  border-radius: 5px;
  background: transparent;
  color: var(--td-text-color-secondary, #667286);
  cursor: pointer;

  &:hover { border-color: #cbd4df; background: #f5f7fa; color: #172033; }
}

.settings-tabs {
  padding: 0 28px;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  border-bottom: 1px solid var(--td-component-stroke, #dfe5ec);
  background: var(--td-bg-color-container, #fff);

  > button {
    min-width: 0;
    padding: 0 12px;
    display: flex;
    align-items: center;
    gap: 11px;
    border: 0;
    border-bottom: 3px solid transparent;
    background: transparent;
    color: var(--td-text-color-secondary, #687487);
    text-align: left;
    font: inherit;
    cursor: pointer;

    &:hover { background: var(--td-bg-color-secondarycontainer, #f6f8fb); }
    &.active { border-bottom-color: #1268e3; color: #1268e3; }
    > span:last-child { min-width: 0; display: flex; flex-direction: column; gap: 2px; }
    strong { font-size: 12px; font-weight: 700; }
    small { overflow: hidden; color: var(--td-text-color-placeholder, #8a95a5); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
  }
}

.tab-icon {
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  border-radius: 5px;
  background: var(--td-bg-color-secondarycontainer, #f0f3f7);
  font-size: 16px;
}

.settings-tabs > button.active .tab-icon { background: #e8f1ff; }

.zeal-settings-content {
  min-height: 0;
  overflow-y: auto;
  background: var(--td-bg-color-page, #f5f7fa);
  scrollbar-width: thin;
  scrollbar-color: #c9d0d9 transparent;
}

.settings-section {
  width: min(1080px, calc(100% - 64px));
  margin: 0 auto;
  padding: 34px 0 54px;
  box-sizing: border-box;
}

.settings-section :deep(.section-header) { display: none; }

.settings-section :deep(.model-grid) {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.zeal-settings-workbench .role-denied {
  min-height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--td-text-color-placeholder, #8792a3);
}

.zeal-settings-workbench .role-denied strong {
  color: var(--td-text-color-primary, #273347);
  font-size: 14px;
}

.zeal-settings-workbench .role-denied span { font-size: 11px; }

/* 2026 settings workbench */
.zeal-settings-workbench.settings-v2 {
  width: min(1180px, calc(100vw - 56px));
  height: min(820px, calc(100vh - 56px));
  min-width: 0;
  min-height: 560px;
  display: grid;
  grid-template: minmax(0, 1fr) / 254px minmax(0, 1fr);
  border-color: var(--zeal-line-strong, #c6d1df);
  border-radius: 8px;
  background: var(--zeal-surface, #fff);
  box-shadow: 0 26px 72px rgba(7, 20, 39, 0.28);
}

.settings-v2-rail {
  min-width: 0;
  min-height: 0;
  padding: 22px 14px 16px;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #282f39;
  background: var(--zeal-sidebar, #171b22);
  color: #fff;
}

.settings-v2-rail .settings-identity {
  min-height: 54px;
  padding: 0 10px;
}

.settings-v2-rail .settings-identity strong { color: #fff; font-size: 16px; }
.settings-v2-rail .settings-identity small { color: #7f8b9b; font-size: 10px; }

.settings-v2-rail .settings-tabs {
  min-height: 0;
  margin-top: 24px;
  padding: 0;
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 5px;
  overflow-y: auto;
  border: 0;
  background: transparent;
}

.settings-v2-rail .settings-tabs > button {
  width: 100%;
  min-height: 54px;
  padding: 7px 9px;
  display: flex;
  align-items: center;
  gap: 10px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: #9da8b7;
  text-align: left;
  font: inherit;
  cursor: pointer;
}

.settings-v2-rail .settings-tabs > button:hover { color: #fff; background: #222832; }
.settings-v2-rail .settings-tabs > button.active {
  color: #fff;
  background: #2a313c;
  box-shadow: inset 3px 0 0 #4d91ed;
}
.settings-v2-rail .settings-tabs > button > span:nth-child(2) {
  min-width: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.settings-v2-rail .settings-tabs strong { color: inherit; font-size: 12px; font-weight: 680; }
.settings-v2-rail .settings-tabs small {
  overflow: hidden;
  color: #6f7b8b;
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.settings-v2-rail .settings-tabs .tab-icon {
  width: 30px;
  height: 30px;
  flex: 0 0 30px;
  display: grid;
  place-items: center;
  border: 1px solid #2d3541;
  border-radius: 6px;
  background: #20262f;
  color: #8491a2;
  transition: color 150ms ease, border-color 150ms ease, background 150ms ease, box-shadow 150ms ease;
}
.settings-v2-rail .settings-tabs > button:hover .tab-icon {
  border-color: #465568;
  color: #c5d2e1;
}
.settings-v2-rail .settings-tabs > button.active .tab-icon {
  color: #86b9fb;
  border-color: #315b8e;
  background: #142640;
  box-shadow: 0 0 0 3px rgba(77, 145, 237, 0.08);
}
.settings-tab-arrow { color: #5f6a79; font-size: 14px; }

.settings-runtime {
  margin: 12px 8px 0;
  padding-top: 14px;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border-top: 1px solid #2b323c;
  color: #788494;
  font-size: 10px;
}
.settings-runtime i { width: 7px; height: 7px; border-radius: 50%; background: #35c58a; }

.settings-v2-stage {
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--zeal-canvas, #f3f6fa);
  overflow: hidden;
}

.settings-v2 .zeal-settings-header {
  min-height: 92px;
  padding: 0 26px 0 32px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--zeal-line, #dbe3ed);
  background: var(--zeal-surface, #fff);
}
.settings-v2 .settings-heading h1 { margin-bottom: 4px; font-size: 21px; }
.settings-v2 .settings-heading p { font-size: 12px; }
.settings-v2 .zeal-settings-close {
  flex: 0 0 36px;
  border-color: var(--zeal-line, #dbe3ed);
  background: var(--zeal-surface-subtle, #f8fafc);
}
.settings-v2 .zeal-settings-content { flex: 1; min-height: 0; }
.settings-v2 .settings-section {
  width: min(820px, calc(100% - 56px));
  padding: 28px 0 48px;
}
.settings-v2 .settings-section :deep(.settings-group),
.settings-v2 .settings-section :deep(.setting-card),
.settings-v2 .settings-section :deep(.model-card) {
  border-color: var(--zeal-line, #dbe3ed);
  border-radius: 8px;
  box-shadow: var(--zeal-shadow-xs);
}

@media (max-width: 900px) {
  .zeal-settings-overlay { padding: 16px; }
  .zeal-settings-workbench.settings-v2 {
    width: calc(100vw - 32px);
    height: calc(100vh - 32px);
    grid-template-columns: 210px minmax(0, 1fr);
  }
  .settings-v2 .settings-section { width: calc(100% - 36px); }
}

@media (max-width: 680px) {
  .zeal-settings-overlay { padding: 0; }
  .zeal-settings-workbench.settings-v2 {
    width: 100vw;
    height: calc(100vh - 64px - env(safe-area-inset-bottom));
    min-height: 0;
    margin-bottom: calc(64px + env(safe-area-inset-bottom));
    grid-template: 76px minmax(0, 1fr) / 1fr;
    border: 0;
    border-radius: 0;
  }
  .settings-v2-rail {
    min-width: 0;
    overflow: hidden;
    padding: 8px 10px;
    flex-direction: row;
    align-items: center;
    border-right: 0;
    border-bottom: 1px solid #282f39;
  }
  .settings-v2-rail .settings-identity,
  .settings-runtime { display: none; }
  .settings-v2-rail .settings-tabs {
    width: 100%;
    min-width: 0;
    margin: 0;
    flex-direction: row;
    gap: 5px;
    overflow-x: auto;
    overflow-y: hidden;
  }
  .settings-v2-rail .settings-tabs > button {
    width: auto;
    min-width: 92px;
    min-height: 56px;
    padding: 5px 8px;
    flex-direction: column;
    justify-content: center;
    gap: 2px;
    text-align: center;
  }
  .settings-v2-rail .settings-tabs > button > span:nth-child(2) { flex: 0 0 auto; }
  .settings-v2-rail .settings-tabs small,
  .settings-tab-arrow { display: none; }
  .settings-v2-rail .settings-tabs .tab-icon { width: 24px; height: 24px; flex-basis: 24px; }
  .settings-v2-stage { min-height: 0; }
  .settings-v2 .zeal-settings-header { min-height: 74px; padding: 0 16px 0 20px; }
  .settings-v2 .settings-heading h1 { font-size: 18px; }
  .settings-v2 .settings-heading p { max-width: 240px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .settings-v2 .settings-section { width: calc(100% - 28px); padding-top: 18px; }
  .settings-v2 .settings-section :deep(.model-grid) { grid-template-columns: 1fr; }
}
</style>
