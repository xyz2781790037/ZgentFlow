import { post } from '@/utils/request'

// MessageSearchRequest defines search parameters for message search
export interface MessageSearchRequest {
  query: string
  limit?: number
  session_ids?: string[]
}

// MessageSearchGroupItem represents a merged Q&A pair in search results
export interface MessageSearchGroupItem {
  request_id: string
  session_id: string
  session_title: string
  query_content: string
  answer_content: string
  score: number
  match_type: string
  created_at: string
}

// MessageSearchResult represents the full search result
export interface MessageSearchResult {
  items: MessageSearchGroupItem[]
  total: number
}

// Search stored messages across all sessions.
export function searchMessages(data: MessageSearchRequest) {
  return post('/api/v1/messages/search', data)
}
