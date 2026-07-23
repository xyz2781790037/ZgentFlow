import { get, post, put, del, postUpload, getDown } from "../../utils/request";
import type { KnowledgeProcessOverrides } from '@/types/knowledgeProcess';

// 知识库管理 API（列表、创建、获取、更新、删除）
export function listKnowledgeBases() {
  return get('/api/v1/knowledge-bases');
}

export function createKnowledgeBase(data: {
  name: string;
  description?: string;
  type?: 'document' | 'faq';
  max_file_size_mb?: number;
  chunking_config?: any;
  embedding_model_id?: string;
  summary_model_id?: string;
  faq_config?: { index_mode: string; question_index_mode?: string };
  indexing_strategy?: {
    vector_enabled: boolean;
    keyword_enabled: boolean;
  };
}) {
  return post(`/api/v1/knowledge-bases`, data);
}

export function getKnowledgeBaseById(id: string, options?: { agent_id?: string }) {
  const query = new URLSearchParams();
  if (options?.agent_id) query.set('agent_id', options.agent_id);
  const qs = query.toString();
  return get(qs ? `/api/v1/knowledge-bases/${id}?${qs}` : `/api/v1/knowledge-bases/${id}`);
}

export function updateKnowledgeBase(id: string, data: {
  name: string;
  description?: string;
  config?: {
    max_file_size_mb?: number;
    chunking_config?: any;
    faq_config?: any;
    indexing_strategy?: {
      vector_enabled: boolean;
      keyword_enabled: boolean;
    };
  }
}) {
  return put(`/api/v1/knowledge-bases/${id}` , data);
}

export type KBRebuildStatus = 'idle' | 'pending' | 'running' | 'succeeded' | 'failed';

export interface KBRebuildState {
  knowledge_base_id: string;
  active_generation: number;
  building_generation?: number;
  status: KBRebuildStatus;
  error?: string;
  started_at?: string;
  completed_at?: string;
}

// One request rebuilds vectors using reusable cache entries.
export function rebuildKBIndex(kbId: string) {
  return post(`/api/v1/knowledge-bases/${kbId}/rebuild-index`, {});
}

export function getKBRebuildStatus(kbId: string) {
  return get(`/api/v1/knowledge-bases/${kbId}/rebuild-index/status`);
}

export function deleteKnowledgeBase(id: string) {
  return del(`/api/v1/knowledge-bases/${id}`);
}

export function listTrashedKnowledgeBases() {
  return get('/api/v1/trash/knowledge-bases');
}

export function restoreTrashedKnowledgeBase(id: string) {
  return post(`/api/v1/trash/knowledge-bases/${id}/restore`, {});
}

export function purgeTrashedKnowledgeBase(id: string) {
  return del(`/api/v1/trash/knowledge-bases/${id}`);
}

// 获取可移动目标知识库列表（同类型、同Embedding模型）
export function listMoveTargets(sourceKbId: string) {
  return get(`/api/v1/knowledge-bases/${sourceKbId}/move-targets`);
}

// 移动知识到其他知识库
export function moveKnowledge(data: {
  knowledge_ids: string[];
  source_kb_id: string;
  target_kb_id: string;
  mode: 'reuse_vectors' | 'reparse';
}) {
  return post('/api/v1/knowledge/move', data);
}

// 获取知识移动进度
export function getKnowledgeMoveProgress(taskId: string) {
  return get(`/api/v1/knowledge/move/progress/${taskId}`);
}

export function togglePinKnowledgeBase(id: string) {
  return put(`/api/v1/knowledge-bases/${id}/pin`);
}

export type KnowledgeBaseRole = 'owner' | 'admin' | 'writer' | 'reader';

export function lookupKnowledgeBaseInvitation(code: string) {
  return post('/api/v1/knowledge-base-invitations/lookup', { code });
}

export function submitKnowledgeBaseJoinRequest(code: string) {
  return post('/api/v1/knowledge-base-join-requests', { code });
}

export function listMyKnowledgeBaseJoinRequests() {
  return get('/api/v1/knowledge-base-join-requests/mine');
}

export function getKnowledgeBaseSharing(id: string) {
  return get(`/api/v1/knowledge-bases/${id}/sharing`);
}

export function updateKnowledgeBaseInvitation(id: string, enabled: boolean, regenerate = false) {
  return put(`/api/v1/knowledge-bases/${id}/sharing/invitation`, { enabled, regenerate });
}

export function listKnowledgeBaseMembers(id: string) {
  return get(`/api/v1/knowledge-bases/${id}/sharing/members`);
}

export function updateKnowledgeBaseMemberRole(id: string, userId: string, role: Exclude<KnowledgeBaseRole, 'owner'>) {
  return put(`/api/v1/knowledge-bases/${id}/sharing/members/${userId}`, { role });
}

export function removeKnowledgeBaseMember(id: string, userId: string) {
  return del(`/api/v1/knowledge-bases/${id}/sharing/members/${userId}`);
}

export function leaveKnowledgeBase(id: string) {
  return post(`/api/v1/knowledge-bases/${id}/sharing/leave`, {});
}

export function listKnowledgeBaseJoinRequests(id: string) {
  return get(`/api/v1/knowledge-bases/${id}/sharing/requests`);
}

export function reviewKnowledgeBaseJoinRequest(id: string, requestId: string, decision: 'approved' | 'rejected') {
  return post(`/api/v1/knowledge-bases/${id}/sharing/requests/${requestId}/review`, { decision });
}

export function listKnowledgeBaseAuditLogs(id: string) {
  return get(`/api/v1/knowledge-bases/${id}/sharing/logs`);
}

// 知识文件 API（基于具体知识库）
export function uploadKnowledgeFile(
  kbId: string,
  data: {
    file: File
    fileName?: string
    process_config?: KnowledgeProcessOverrides | string
    [key: string]: any
  } = { file: new File([], '') },
  onProgress?: (progressEvent: any) => void,
) {
  const formData = new FormData();
  Object.keys(data).forEach(key => {
    const value = data[key];
    if (value === undefined) return;
    if (key === 'process_config' && value && typeof value !== 'string') {
      formData.append(key, JSON.stringify(value));
    } else {
      formData.append(key, value);
    }
  });
  return postUpload(`/api/v1/knowledge-bases/${kbId}/knowledge/file`, formData, onProgress);
}

export function listKnowledgeFiles(
  kbId: string,
  params: {
    page: number;
    page_size: number;
    keyword?: string;
    file_type?: string;
    parse_status?: string;
    start_time?: string;
    end_time?: string;
  },
) {
  const query = new URLSearchParams();
  query.append('page', String(params.page));
  query.append('page_size', String(params.page_size));
  if (params.keyword) query.append('keyword', params.keyword);
  if (params.file_type) query.append('file_type', params.file_type);
  if (params.parse_status) query.append('parse_status', params.parse_status);
  if (params.start_time) query.append('start_time', params.start_time);
  if (params.end_time) query.append('end_time', params.end_time);
  const qs = query.toString();
  return get(`/api/v1/knowledge-bases/${kbId}/knowledge?${qs}`);
}

export function getKnowledgeDetails(id: string, options?: { agent_id?: string }) {
  const query = new URLSearchParams();
  if (options?.agent_id) query.set('agent_id', options.agent_id);
  const qs = query.toString();
  return get(qs ? `/api/v1/knowledge/${id}?${qs}` : `/api/v1/knowledge/${id}`);
}

export function reparseKnowledge(id: string, data?: { process_config?: KnowledgeProcessOverrides }) {
  return post(`/api/v1/knowledge/${id}/reparse`, data);
}

export function cancelKnowledgeParse(id: string) {
  return post(`/api/v1/knowledge/${id}/cancel-parse`);
}

export function getKnowledgeSpans(id: string, attempt?: number) {
  const qs = attempt ? `?attempt=${attempt}` : '';
  return get(`/api/v1/knowledge/${id}/spans${qs}`);
}

export function delKnowledgeDetails(id: string) {
  return del(`/api/v1/knowledge/${id}`);
}

export function listTrashedKnowledge() {
  return get('/api/v1/trash/knowledge');
}

export function restoreTrashedKnowledge(id: string) {
  return post(`/api/v1/trash/knowledge/${id}/restore`, {});
}

export function purgeTrashedKnowledge(id: string) {
  return del(`/api/v1/trash/knowledge/${id}`);
}

// 批量删除（同一知识库内）。后端会校验所有 id 隶属于 kb_id 且具有编辑权限。
export function batchDeleteKnowledge(kbId: string, ids: string[]) {
  return post(`/api/v1/knowledge/batch-delete`, { kb_id: kbId, ids });
}

export function downKnowledgeDetails(id: string) {
  return getDown(`/api/v1/knowledge/${id}/download`);
}

export function previewKnowledgeFile(id: string) {
  return getDown(`/api/v1/knowledge/${id}/preview`);
}

/** @param idsQueryString - query string with ids (e.g. ids=xxx&ids=yyy) */
export function batchQueryKnowledge(idsQueryString: string, kbId?: string, agentId?: string) {
  let qs = idsQueryString;
  if (kbId) qs += `&kb_id=${encodeURIComponent(kbId)}`;
  if (agentId) qs += `&agent_id=${encodeURIComponent(agentId)}`;
  return get(`/api/v1/knowledge/batch?${qs}`);
}

export function getKnowledgeDetailsCon(id: string, page: number) {
  return get(`/api/v1/chunks/${id}?page=${page}&page_size=25`);
}

// Get chunk by chunk_id only (new endpoint - to be added to backend)
export function getChunkByIdOnly(chunkId: string) {
  return get(`/api/v1/chunks/by-id/${chunkId}`);
}

// Delete a single generated question from a chunk by question ID
export function deleteGeneratedQuestion(chunkId: string, questionId: string) {
  return del(`/api/v1/chunks/by-id/${chunkId}/questions`, { question_id: questionId });
}

const buildQuery = (params?: Record<string, any>) => {
  if (!params) return '';
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === null || value === '') return;
    query.append(key, String(value));
  });
  const queryString = query.toString();
  return queryString ? `?${queryString}` : '';
};

export function listFAQEntries(
  kbId: string,
  params?: { page?: number; page_size?: number; keyword?: string },
) {
  const query = buildQuery(params);
  return get(`/api/v1/knowledge-bases/${kbId}/faq/entries${query}`);
}

export function upsertFAQEntries(kbId: string, data: { entries: any[]; mode: 'append' | 'replace' }) {
  return post(`/api/v1/knowledge-bases/${kbId}/faq/entries`, data);
}

export function createFAQEntry(kbId: string, data: any) {
  return post(`/api/v1/knowledge-bases/${kbId}/faq/entry`, data);
}

export function updateFAQEntry(kbId: string, entryId: number, data: any) {
  return put(`/api/v1/knowledge-bases/${kbId}/faq/entries/${entryId}`, data);
}

// Unified batch update API for enabled and recommended state.
export interface FAQEntryFieldsUpdate {
  is_enabled?: boolean
  is_recommended?: boolean
}

export interface FAQEntryFieldsBatchRequest {
  by_id?: Record<number, FAQEntryFieldsUpdate>
}

export function updateFAQEntryFieldsBatch(kbId: string, data: FAQEntryFieldsBatchRequest) {
  return put(`/api/v1/knowledge-bases/${kbId}/faq/entries/fields`, data);
}

export function deleteFAQEntries(kbId: string, ids: number[]) {
  return del(`/api/v1/knowledge-bases/${kbId}/faq/entries`, { ids });
}

export function searchFAQEntries(
  kbId: string,
  data: {
    query_text: string
    vector_threshold?: number
    match_count?: number
  }
) {
  return post(`/api/v1/knowledge-bases/${kbId}/faq/search`, data);
}

// Export FAQ entries as CSV file
export async function exportFAQEntries(kbId: string): Promise<Blob> {
  const response = await getDown(`/api/v1/knowledge-bases/${kbId}/faq/entries/export`);
  return response as unknown as Blob;
}

// FAQ Import Progress API
export interface FAQBlockedEntry {
  index: number
  standard_question: string
  reason: string
}

export interface FAQSuccessEntry {
  index: number
  seq_id: number
  standard_question: string
}

export interface FAQImportProgress {
  task_id: string
  kb_id: string
  knowledge_id: string
  status: 'pending' | 'processing' | 'completed' | 'failed'
  progress: number
  total: number
  processed: number
  blocked: number
  blocked_entries?: FAQBlockedEntry[]
  success_entries?: FAQSuccessEntry[]
  message: string
  error: string
  created_at: number
  updated_at: number
}

export function getFAQImportProgress(taskId: string) {
  return get(`/api/v1/faq/import/progress/${taskId}`);
}

export function updateFAQImportResultDisplayStatus(knowledgeBaseId: string, displayStatus: 'open' | 'close') {
  return put(`/api/v1/knowledge-bases/${knowledgeBaseId}/faq/import/last-result/display`, {
    display_status: displayStatus
  });
}

export function searchKnowledge(
  keyword: string,
  offset = 0,
  limit = 20,
  fileTypes?: string[],
  options?: { agent_id?: string }
) {
  const query = new URLSearchParams();
  query.set('keyword', keyword);
  query.set('offset', String(offset));
  query.set('limit', String(limit));
  if (fileTypes && fileTypes.length > 0) {
    query.set('file_types', fileTypes.join(','));
  }
  if (options?.agent_id) query.set('agent_id', options.agent_id);
  return get(`/api/v1/knowledge/search?${query.toString()}`);
}

export function knowledgeSemanticSearch(data: {
  query: string;
  knowledge_base_ids?: string[];
  knowledge_ids?: string[];
}) {
  return post('/api/v1/knowledge-search', data);
}
