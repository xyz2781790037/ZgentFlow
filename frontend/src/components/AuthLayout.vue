<template>
  <div class="auth-page" :style="backgroundStyle">
    <main class="auth-form-panel">
      <div class="auth-card">
        <slot />
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
const backgroundURL = `${import.meta.env.BASE_URL}auth-background.png`
const backgroundStyle = {
  backgroundImage: `linear-gradient(90deg, rgba(7, 13, 13, 0.58) 0%, rgba(7, 13, 13, 0.3) 48%, rgba(7, 13, 13, 0.12) 100%), url("${backgroundURL}")`,
}
</script>
