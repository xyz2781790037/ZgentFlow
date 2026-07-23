import { get, post, put } from '@/utils/request'

export interface PromptTemplate {
  category: string
  template_id: string
  name: string
  description: string
  content: string
  user_prompt?: string
  version: number
}

export interface PromptVersion {
  id: number
  category: string
  template_id: string
  name: string
  content: string
  user_prompt?: string
  version: number
  is_active: boolean
  created_at: string
}

export const listPrompts = () => get('/api/v1/prompts')

export const getPromptHistory = (category: string, templateId: string) =>
  get(`/api/v1/prompts/${encodeURIComponent(category)}/${encodeURIComponent(templateId)}/history`)

export const updatePrompt = (
  category: string,
  templateId: string,
  data: { content: string; user_prompt?: string },
) => put(`/api/v1/prompts/${encodeURIComponent(category)}/${encodeURIComponent(templateId)}`, data)

export const rollbackPrompt = (category: string, templateId: string, version: number) =>
  post(`/api/v1/prompts/${encodeURIComponent(category)}/${encodeURIComponent(templateId)}/rollback/${version}`, {})
