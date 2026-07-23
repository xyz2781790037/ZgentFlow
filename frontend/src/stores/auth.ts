import { defineStore } from 'pinia'
import {
  getCurrentUser,
  loginWithEmailCode,
  loginWithPassword,
  logout as requestLogout,
  register as requestRegister,
  sendVerificationCode,
  type AuthData,
  type AuthUser,
} from '@/api/auth'
import { setCSRFToken } from '@/utils/request'

let currentUserRequest: Promise<boolean> | null = null

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null as AuthUser | null,
    initialized: false,
  }),

  getters: {
    isAuthenticated: (state) => state.user !== null,
  },

  actions: {
    applyAuth(data: AuthData) {
      this.user = data.user
      this.initialized = true
      setCSRFToken(data.csrf_token)
    },

    clearAuth() {
      this.user = null
      this.initialized = true
      setCSRFToken('')
    },

    async ensureInitialized(): Promise<boolean> {
      if (this.initialized) return this.isAuthenticated
      if (!currentUserRequest) {
        currentUserRequest = getCurrentUser()
          .then((response) => {
            this.applyAuth(response.data)
            return true
          })
          .catch(() => {
            this.clearAuth()
            return false
          })
          .finally(() => {
            currentUserRequest = null
          })
      }
      return currentUserRequest
    },

    async passwordLogin(username: string, password: string) {
      const response = await loginWithPassword(username, password)
      this.applyAuth(response.data)
    },

    async emailCodeLogin(email: string, code: string) {
      const response = await loginWithEmailCode(email, code)
      this.applyAuth(response.data)
    },

    async register(username: string, email: string, password: string, code: string) {
      const response = await requestRegister(username, email, password, code)
      this.applyAuth(response.data)
    },

    sendCode(email: string, purpose: 'register' | 'email_login') {
      return sendVerificationCode(email, purpose)
    },

    async logout() {
      try {
        await requestLogout()
      } finally {
        this.clearAuth()
      }
    },
  },
})
