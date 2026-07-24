<template>
  <AuthLayout>
    <header class="auth-card-header">
      <span class="auth-card-brand"><b>Z</b>ZgentFlow</span>
      <h2>创建账号</h2>
      <p>验证邮箱后即可进入知识工作台</p>
    </header>

    <form class="auth-form" @submit.prevent="submit">
      <label>
        <span>用户名</span>
        <input v-model.trim="form.username" autocomplete="username" maxlength="32" placeholder="3-32 位英文字母或数字" />
      </label>
      <label>
        <span>邮箱地址</span>
        <input v-model.trim="form.email" type="email" autocomplete="email" maxlength="255" placeholder="name@example.com" />
      </label>
      <label>
        <span>密码</span>
        <input v-model="form.password" type="password" autocomplete="new-password" maxlength="128" placeholder="至少 1 个字符" />
      </label>
      <label>
        <span>确认密码</span>
        <input v-model="form.confirmPassword" type="password" autocomplete="new-password" maxlength="128" placeholder="再次输入密码" />
      </label>
      <label>
        <span>邮箱验证码</span>
        <div class="auth-code-row">
          <input v-model.trim="form.code" inputmode="numeric" autocomplete="one-time-code" maxlength="6" placeholder="6 位验证码" />
          <button type="button" :disabled="sendingCode || countdown > 0" @click="sendCode">
            {{ countdown > 0 ? `${countdown}s 后重发` : sendingCode ? '发送中…' : '发送验证码' }}
          </button>
        </div>
      </label>
      <p v-if="noticeMessage" class="auth-notice">{{ noticeMessage }}</p>
      <p v-if="errorMessage" class="auth-error">{{ errorMessage }}</p>
      <button class="auth-submit" type="submit" :disabled="submitting">{{ submitting ? '注册中…' : '注册并登录' }}</button>
    </form>

    <p class="auth-switch">已有账号？<RouterLink to="/login">返回登录</RouterLink></p>
  </AuthLayout>
</template>

<script setup lang="ts">
import { onUnmounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import AuthLayout from '@/components/AuthLayout.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()
const form = reactive({ username: '', email: '', password: '', confirmPassword: '', code: '' })
const submitting = ref(false)
const sendingCode = ref(false)
const countdown = ref(0)
const errorMessage = ref('')
const noticeMessage = ref('')
let countdownTimer: number | undefined

const errorText = (error: any) => error?.message || '操作失败，请稍后重试'

const validateBaseFields = () => {
  if (!/^[A-Za-z0-9]{3,32}$/.test(form.username)) return '用户名需为 3-32 位英文字母或数字'
  if (!/^\S+@\S+\.\S+$/.test(form.email)) return '请输入有效的邮箱地址'
  if (Array.from(form.password).length < 1) return '密码不能为空'
  if (form.password !== form.confirmPassword) return '两次输入的密码不一致'
  return ''
}

const sendCode = async () => {
  errorMessage.value = ''
  noticeMessage.value = ''
  if (!/^\S+@\S+\.\S+$/.test(form.email)) {
    errorMessage.value = '请输入有效的邮箱地址'
    return
  }
  sendingCode.value = true
  try {
    const response = await auth.sendCode(form.email, 'register')
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

const submit = async () => {
  errorMessage.value = validateBaseFields()
  if (errorMessage.value) return
  if (!/^\d{6}$/.test(form.code)) {
    errorMessage.value = '请输入 6 位邮箱验证码'
    return
  }
  submitting.value = true
  try {
    await auth.register(form.username, form.email, form.password, form.code)
    await router.replace('/platform/creatChat')
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
