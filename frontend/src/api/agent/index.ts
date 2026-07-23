import { get } from '../../utils/request'

export type AnswerMode = 'quick-answer' | 'smart-reasoning'

export interface AnswerModeConfig {
  agent_mode: AnswerMode
  model_id?: string
  allowed_tools?: string[]
  kb_selection_mode?: 'all' | 'selected' | 'none'
  knowledge_bases?: string[]
  image_upload_enabled?: boolean
  web_search_enabled?: boolean
  web_search_provider_id?: string
}

export interface AnswerModeDefinition {
  id: string
  name: string
  description?: string
  config: AnswerModeConfig
}

export const BUILTIN_QUICK_ANSWER_ID = 'builtin-quick-answer'
export const AGENT_MODE_QUICK_ANSWER = 'quick-answer'
export const AGENT_MODE_SMART_REASONING = 'smart-reasoning'

export function listAnswerModes() {
  return get<{ data: AnswerModeDefinition[] }>('/api/v1/answer-modes')
}
