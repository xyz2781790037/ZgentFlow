<template>
  <AuthLayout>
    <header class="auth-card-header">
      <span class="auth-card-brand"><b>Z</b>ZgentFlow</span>
      <h2>欢迎回来</h2>
      <p>登录后继续使用你的知识工作台</p>
    </header>

    <div class="auth-tabs" role="tablist">
      <button type="button" :class="{ active: mode === 'password' }" @click="switchMode('password')">密码登录</button>
      <button type="button" :class="{ active: mode === 'email' }" @click="switchMode('email')">邮箱验证码</button>
    </div>

    <form v-if="mode === 'password'" class="auth-form" @submit.prevent="submitPassword">
      <label>
        <span>用户名</span>
        <input v-model.trim="passwordForm.username" autocomplete="username" maxlength="32" placeholder="请输入用户名" />
      </label>
      <label>
        <span>密码</span>
        <input v-model="passwordForm.password" type="password" autocomplete="current-password" maxlength="128" placeholder="请输入密码" />
      </label>
      <p v-if="errorMessage" class="auth-error">{{ errorMessage }}</p>
      <button class="auth-submit" type="submit" :disabled="submitting">{{ submitting ? '登录中…' : '登录' }}</button>
    </form>

    <form v-else class="auth-form" @submit.prevent="submitEmailCode">
      <label>
        <span>邮箱地址</span>
        <input v-model.trim="emailForm.email" type="email" autocomplete="email" maxlength="255" placeholder="name@example.com" />
      </label>
      <label>
        <span>验证码</span>
        <div class="auth-code-row">
          <input v-model.trim="emailForm.code" inputmode="numeric" autocomplete="one-time-code" maxlength="6" placeholder="6 位验证码" />
          <button type="button" :disabled="sendingCode || countdown > 0" @click="sendCode">
            {{ countdown > 0 ? `${countdown}s 后重发` : sendingCode ? '发送中…' : '发送验证码' }}
          </button>
        </div>
      </label>
      <p v-if="noticeMessage" class="auth-notice">{{ noticeMessage }}</p>
      <p v-if="errorMessage" class="auth-error">{{ errorMessage }}</p>
      <button class="auth-submit" type="submit" :disabled="submitting">{{ submitting ? '登录中…' : '登录' }}</button>
    </form>

    <p class="auth-switch">还没有账号？<RouterLink to="/register">创建账号</RouterLink></p>
  </AuthLayout>
</template>

<script setup lang="ts">
import { onUnmounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AuthLayout from '@/components/AuthLayout.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const mode = ref<'password' | 'email'>('password')
const submitting = ref(false)
const sendingCode = ref(false)
const countdown = ref(0)
const errorMessage = ref('')
const noticeMessage = ref('')
const passwordForm = reactive({ username: '', password: '' })
const emailForm = reactive({ email: '', code: '' })
let countdownTimer: number | undefined

const switchMode = (nextMode: 'password' | 'email') => {
  mode.value = nextMode
  errorMessage.value = ''
  noticeMessage.value = ''
}

const errorText = (error: any) => error?.message || '操作失败，请稍后重试'

const destination = () => {
  const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : ''
  return redirect.startsWith('/') && !redirect.startsWith('//') ? redirect : '/platform/creatChat'
}

const submitPassword = async () => {
  errorMessage.value = ''
  if (!passwordForm.username || !passwordForm.password) {
    errorMessage.value = '请输入用户名和密码'
    return
  }
  submitting.value = true
  try {
    await auth.passwordLogin(passwordForm.username, passwordForm.password)
    await router.replace(destination())
  } catch (error) {
    errorMessage.value = errorText(error)
  } finally {
    submitting.value = false
  }
}

const sendCode = async () => {
  errorMessage.value = ''
  noticeMessage.value = ''
  if (!/^\S+@\S+\.\S+$/.test(emailForm.email)) {
    errorMessage.value = '请输入有效的邮箱地址'
    return
  }
  sendingCode.value = true
  try {
    const response = await auth.sendCode(emailForm.email, 'email_login')
    noticeMessage.value = response.message
    countdown.value = 60
    countdownTimer = window.setInterval(() => {
      countdown.value -= 1
      if (countdown.value <= 0 && countdownTimer) window.clearInterval(countdownTimer)
    }, 1000)
  } catch (error) {
    errorMessage.value = errorText(error)
  } finally {
    sendingCode.value = false
  }
}

const submitEmailCode = async () => {
  errorMessage.value = ''
  if (!emailForm.email || !/^\d{6}$/.test(emailForm.code)) {
    errorMessage.value = '请输入邮箱和 6 位验证码'
    return
  }
  submitting.value = true
  try {
    await auth.emailCodeLogin(emailForm.email, emailForm.code)
    await router.replace(destination())
  } catch (error) {
    errorMessage.value = errorText(error)
  } finally {
    submitting.value = false
  }
}

onUnmounted(() => {
  if (countdownTimer) window.clearInterval(countdownTimer)
})
</script>
