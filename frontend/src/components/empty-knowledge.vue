<script setup lang="ts">
defineProps<{
  maxFileSizeMb: number
}>()
</script>

<template>
  <div class="empty">
    <p>拖拽文件到此处，单文件不超过 {{ maxFileSizeMb }} MB</p>
  </div>
</template>
<style scoped lang="less">
.empty {
  flex: 1;
  display: grid;
  place-items: center;

  p {
    margin: 0;
    color: var(--td-text-color-placeholder);
    font-family: var(--app-font-family);
    font-size: 14px;
    line-height: 22px;
  }
}
</style>
