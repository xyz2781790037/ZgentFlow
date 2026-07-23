import { get, post } from '@/utils/request'

export interface AuthUser {
  id: string
  username: string
  email: string
  avatar?: string
  tenant_id: number
  is_active: boolean
}

export interface AuthData {
  user: AuthUser
  csrf_token: string
}

interface AuthResponse {
  success: boolean
  message?: string
  data: AuthData
}

export const sendVerificationCode = (email: string, purpose: 'register' | 'email_login') =>
  post<{ success: boolean; message: string }>('/api/v1/auth/codes', { email, purpose })

export const register = (username: string, email: string, password: string, code: string) =>
  post<AuthResponse>('/api/v1/auth/register', { username, email, password, code })

export const loginWithPassword = (username: string, password: string) =>
  post<AuthResponse>('/api/v1/auth/login/password', { username, password })

export const loginWithEmailCode = (email: string, code: string) =>
  post<AuthResponse>('/api/v1/auth/login/email-code', { email, code })

export const getCurrentUser = () => get<AuthResponse>('/api/v1/auth/me')

export const logout = () => post<{ success: boolean; message: string }>('/api/v1/auth/logout')
