import { get } from '@/utils/request'

export interface ParserEngineInfo {
  Name: string
  Description: string
  FileTypes: string[]
  Available?: boolean
  UnavailableReason?: string
}

/** 解析引擎配置（引擎相关存租户；docreader 地址由环境变量配置） */
export interface ParserEngineConfig {
  docreader_addr?: string
  docreader_transport?: string
  mineru_endpoint?: string
  mineru_api_key?: string
  // MinerU 自建参数
  mineru_model?: string
  mineru_vlm_server_url?: string
  mineru_enable_formula?: boolean | null
  mineru_enable_table?: boolean | null
  mineru_enable_ocr?: boolean | null
  mineru_language?: string
  // MinerU 云 API 参数
  mineru_cloud_model?: string
  mineru_cloud_enable_formula?: boolean | null
  mineru_cloud_enable_table?: boolean | null
  mineru_cloud_enable_ocr?: boolean | null
  mineru_cloud_language?: string
  // PaddleOCR-VL 自建参数
  paddleocr_vl_endpoint?: string
  paddleocr_vl_use_seal_recognition?: boolean | null
  paddleocr_vl_use_chart_recognition?: boolean | null
  // PaddleOCR-VL 云 API 参数
  paddleocr_vl_cloud_token?: string
  paddleocr_vl_cloud_model?: string
  paddleocr_vl_cloud_use_seal_recognition?: boolean | null
  paddleocr_vl_cloud_use_chart_recognition?: boolean | null
}

export interface ParserEnginesResponse {
  data: ParserEngineInfo[]
  docreader_addr?: string
  /** 连接方式：grpc | http，由服务端环境/配置决定 */
  docreader_transport?: string
  connected?: boolean
}

export function getParserEngines(): Promise<ParserEnginesResponse> {
  return get('/api/v1/system/parser-engines')
}
