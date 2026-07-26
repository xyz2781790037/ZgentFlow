<template>
  <aside class="zeal-nav" aria-label="主导航">
    <button class="brand" type="button" aria-label="ZgentFlow 首页" @click="go('/platform/creatChat')">
      <span class="brand-mark">Z</span>
      <span class="brand-copy">
        <strong>ZgentFlow</strong>
        <small>智能知识工作台</small>
      </span>
    </button>

    <span class="nav-caption">工作区</span>
    <nav class="nav-stack">
      <button
        v-for="item in primaryItems"
        :key="item.key"
        type="button"
        class="nav-item"
        :class="{ active: activeArea === item.key }"
        @click="go(item.path)"
      >
        <span class="nav-item-icon"><t-icon :name="item.icon" size="19px" /></span>
        <span class="nav-item-copy">{{ item.label }}</span>
        <t-icon class="nav-item-arrow" name="chevron-right" size="15px" />
      </button>
    </nav>

    <div class="nav-bottom">
      <button class="nav-tool" type="button" aria-label="搜索" @click="commandPalette.openPalette('')">
        <span class="nav-item-icon"><t-icon name="search" size="18px" /></span>
        <span class="nav-tool-copy">搜索</span>
      </button>
      <button class="nav-tool" type="button" aria-label="模型与设置" @click="uiStore.openSettings()">
        <span class="nav-item-icon"><t-icon name="setting" size="18px" /></span>
        <span class="nav-tool-copy">设置</span>
      </button>
      <div class="account-summary" :title="auth.user?.email">
        <span class="account-avatar">{{ auth.user?.username?.slice(0, 1).toUpperCase() }}</span>
        <span class="account-copy"><strong>{{ auth.user?.username }}</strong><small>{{ auth.user?.email }}</small></span>
      </div>
      <button class="nav-tool logout-tool" type="button" aria-label="退出登录" @click="logout">
        <span class="nav-item-icon"><t-icon name="logout" size="18px" /></span>
        <span class="nav-tool-copy">退出登录</span>
      </button>
      <span class="local-status" title="服务正常"><i></i><span>ZgentFlow 服务正常</span></span>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useCommandPaletteStore } from '@/stores/commandPalette'
import { useUIStore } from '@/stores/ui'
import { useAuthStore } from '@/stores/auth'
import { MessagePlugin } from 'tdesign-vue-next'

const route = useRoute()
const router = useRouter()
const commandPalette = useCommandPaletteStore()
const uiStore = useUIStore()
const auth = useAuthStore()

const primaryItems = [
  { key: 'ask', label: '问答', icon: 'chat-double', path: '/platform/creatChat' },
  { key: 'library', label: '知识库', icon: 'book-open', path: '/platform/knowledge-bases' },
  { key: 'recycle', label: '回收站', icon: 'delete', path: '/platform/recycle-bin' },
]

const activeArea = computed(() => {
  const name = String(route.name || '')
  if (name === 'globalCreatChat' || name === 'kbCreatChat' || name === 'chat') return 'ask'
  if (name === 'knowledgeBaseList' || name === 'knowledgeBaseDetail') return 'library'
  if (name === 'recycleBin') return 'recycle'
  return ''
})

const go = (path: string) => {
  if (route.path !== path) router.push(path)
}

const logout = async () => {
  try {
    await auth.logout()
  } catch (error: any) {
    MessagePlugin.error(error?.message || '退出登录失败')
  } finally {
    await router.replace('/login')
  }
}
</script>

<style scoped lang="less">
.zeal-nav {
  width: 216px;
  flex: 0 0 216px;
  min-height: 0;
  padding: 18px 14px 16px;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #242a34;
  background: var(--zeal-sidebar, #171b22);
  box-sizing: border-box;
  user-select: none;
}

button { font: inherit; }

.brand {
  height: 48px;
  padding: 0 8px;
  border: 0;
  background: transparent;
  color: #fff;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 11px;
  text-align: left;
}

.brand-mark {
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  background: #1268e3;
  color: #fff;
  font-size: 17px;
  font-weight: 800;
  line-height: 1;
  border-radius: 7px;
  box-shadow: 0 6px 18px rgba(23, 105, 220, 0.26);
}

.brand-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.brand-copy strong {
  font-size: 15px;
  line-height: 18px;
  font-weight: 760;
}

.brand-copy small {
  color: #8995a6;
  font-size: 10px;
  line-height: 13px;
}

.nav-caption {
  padding: 28px 10px 9px;
  color: #6f7a8a;
  font-size: 10px;
  font-weight: 700;
}

.nav-stack {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.nav-item {
  width: 100%;
  height: 44px;
  padding: 0 9px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: #a4adbb;
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 620;
  text-align: left;
  transition: color 150ms ease, background 150ms ease;
}

.nav-item:hover {
  color: #fff;
  background: #222832;
}

.nav-item.active {
  color: #fff;
  background: #2a313c;
  box-shadow: inset 3px 0 0 #4d91ed;
}

.nav-item-icon {
  width: 28px;
  height: 28px;
  flex: 0 0 28px;
  display: grid;
  place-items: center;
  border-radius: 6px;
  border: 1px solid transparent;
  background: #20262f;
  color: #8c99aa;
  transition: color 150ms ease, border-color 150ms ease, background 150ms ease, box-shadow 150ms ease;
}

.nav-item.active .nav-item-icon {
  color: #7db2f7;
  border-color: #315b8e;
  background: #142640;
  box-shadow: 0 0 0 3px rgba(77, 145, 237, 0.08);
}

.nav-item-copy,
.nav-tool-copy {
  min-width: 0;
  flex: 1;
}

.nav-item-arrow {
  color: #626d7d;
}

.nav-bottom {
  margin-top: auto;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.nav-tool {
  width: 100%;
  height: 40px;
  padding: 0 9px;
  border: 0;
  background: transparent;
  color: #929dab;
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  border-radius: 7px;
  font-size: 12px;
  text-align: left;
}

.nav-tool:hover {
  color: #fff;
  background: #222832;
}

.nav-tool .nav-item-icon {
  border-color: #2b333f;
}

.nav-tool:hover .nav-item-icon {
  border-color: #48617f;
  color: #b5d4fb;
  background: #1a2b40;
}

.local-status {
  margin: 12px 8px 0;
  padding-top: 13px;
  border-top: 1px solid #2a3039;
  font-size: 10px;
  color: #768292;
  display: inline-flex;
  align-items: center;
  gap: 7px;
}

.local-status i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #18a66a;
  box-shadow: 0 0 0 2px #dff4ea;
}

.account-summary {
  min-width: 0;
  margin: 9px 7px 2px;
  padding: 9px 2px 8px;
  border-top: 1px solid #2a3039;
  display: flex;
  align-items: center;
  gap: 9px;
}

.account-avatar {
  width: 28px;
  height: 28px;
  flex: 0 0 28px;
  display: grid;
  place-items: center;
  border-radius: 7px;
  color: #d9e9ff;
  background: #25476f;
  font-size: 12px;
  font-weight: 750;
}

.account-copy { min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.account-copy strong { overflow: hidden; color: #d3d9e2; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.account-copy small { overflow: hidden; color: #707d8e; font-size: 9px; text-overflow: ellipsis; white-space: nowrap; }
.logout-tool { color: #9ca7b6; }

@media (max-width: 1180px) {
  .zeal-nav {
    width: 76px;
    flex-basis: 76px;
    padding-inline: 10px;
  }

  .brand { justify-content: center; padding: 0; }
  .brand-copy,
  .nav-caption,
  .nav-item-copy,
  .nav-item-arrow,
  .nav-tool-copy,
  .local-status span { display: none; }
	.account-copy { display: none; }
	.account-summary { justify-content: center; margin-inline: 0; }
  .nav-item,
  .nav-tool { justify-content: center; padding: 0; }
  .local-status { justify-content: center; margin-inline: 0; }
}

@media (max-width: 760px) {
  .zeal-nav {
    position: fixed;
    inset: auto 0 0;
    z-index: 2200;
    width: 100%;
    height: 64px;
    min-height: 64px;
    padding: 5px 8px calc(5px + env(safe-area-inset-bottom));
    flex: 0 0 64px;
    flex-direction: row;
    border-top: 1px solid #2a3039;
    border-right: 0;
  }

  .brand,
  .local-status,
	.account-summary { display: none; }
  .nav-stack,
  .nav-bottom {
    width: 40%;
    margin: 0;
    flex-direction: row;
    gap: 2px;
  }
  .nav-stack { width: 60%; }
  .nav-item { width: 33.333%; }
  .nav-tool {
    width: 33.333%;
    height: 52px;
    flex-direction: column;
    justify-content: center;
    gap: 1px;
    padding: 0;
    border-radius: 6px;
    font-size: 10px;
  }
  .nav-item-copy,
  .nav-tool-copy { display: block; flex: 0 0 auto; }
  .nav-item-arrow { display: none; }
  .nav-item-icon { width: 24px; height: 24px; flex-basis: 24px; }
  .nav-item.active { box-shadow: inset 0 2px 0 #4d91ed; }
}
</style>
