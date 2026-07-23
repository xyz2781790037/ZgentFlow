import { del, get, post, put } from '@/utils/request'

export type ModelAPIConfigProvider = 'siliconflow' | 'deepseek' | 'hunyuan' | 'generic'

export interface ModelAPIConfig {
  id: string
  name: string
  provider: ModelAPIConfigProvider
  configured: boolean
  model_count: number
}

export interface ModelAPIConfigInput {
  name: string
  provider: ModelAPIConfigProvider
  api_key?: string
}

export async function listModelAPIConfigs(): Promise<ModelAPIConfig[]> {
  const response: any = await get('/api/v1/model-api-configs')
  return response.data ?? []
}

export async function createModelAPIConfig(input: ModelAPIConfigInput): Promise<ModelAPIConfig> {
  const response: any = await post('/api/v1/model-api-configs', input)
  return response.data
}

export async function updateModelAPIConfig(
  id: string,
  input: ModelAPIConfigInput,
): Promise<ModelAPIConfig> {
  const response: any = await put(`/api/v1/model-api-configs/${id}`, input)
  return response.data
}

export async function deleteModelAPIConfig(id: string): Promise<{ affected_model_count: number }> {
  const response: any = await del(`/api/v1/model-api-configs/${id}`)
  return response.data ?? { affected_model_count: 0 }
}
