<script setup lang="ts">
import zhCNConfig from 'tdesign-vue-next/esm/locale/zh_CN'
import UploadConfirmHost from '@/components/UploadConfirmHost.vue'

const tdGlobalConfig = zhCNConfig
</script>

<template>
  <t-config-provider :global-config="tdGlobalConfig">
    <div id="app">
      <RouterView />
      <UploadConfirmHost />
    </div>
  </t-config-provider>
</template>

<style>
html {
  color-scheme: light dark;
}

body,
html,
#app {
  width: 100%;
  height: 100%;
  margin: 0;
  padding: 0;
  font-size: 14px;
  font-family: var(--app-font-family);
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  background: var(--td-bg-color-page);
  color: var(--td-text-color-primary);
}

#app {
  isolation: isolate;
  transform: translateZ(0);
  backface-visibility: hidden;
}
</style>
