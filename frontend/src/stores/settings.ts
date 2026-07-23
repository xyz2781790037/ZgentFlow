import { defineStore } from 'pinia'
import { nextTick } from 'vue'
import { BUILTIN_QUICK_ANSWER_ID } from '@/api/agent'

interface ConversationModels {
  summaryModelId: string
  rerankModelId: string
  selectedChatModelId: string
}

interface Settings {
	selectedKnowledgeBases: string[]
  selectedFiles: string[]
  selectedFileKbMap: Record<string, string>
  webSearchEnabled: boolean
  conversationModels: ConversationModels
  selectedAgentId: string
}

const defaultSettings: Settings = {
  selectedKnowledgeBases: [],
  selectedFiles: [],
  selectedFileKbMap: {},
  webSearchEnabled: false,
  conversationModels: {
    summaryModelId: '',
    rerankModelId: '',
    selectedChatModelId: '',
  },
  selectedAgentId: BUILTIN_QUICK_ANSWER_ID,
}

const persist = (settings: Settings) => {
  localStorage.setItem('ZealRAG_settings', JSON.stringify(settings))
}

const loadSettings = (): Settings => {
  try {
    const saved = JSON.parse(localStorage.getItem('ZealRAG_settings') || '{}')
		return {
			...defaultSettings,
			selectedAgentId: BUILTIN_QUICK_ANSWER_ID,
      selectedKnowledgeBases: Array.isArray(saved.selectedKnowledgeBases) ? saved.selectedKnowledgeBases : [],
      selectedFiles: Array.isArray(saved.selectedFiles) ? saved.selectedFiles : [],
      selectedFileKbMap: saved.selectedFileKbMap && typeof saved.selectedFileKbMap === 'object'
        ? saved.selectedFileKbMap
        : {},
      webSearchEnabled: saved.webSearchEnabled === true,
      conversationModels: {
        ...defaultSettings.conversationModels,
        ...(saved.conversationModels || {}),
      },
    }
  } catch {
    return { ...defaultSettings }
  }
}

export const useSettingsStore = defineStore('settings', {
  state: () => ({
    settings: loadSettings(),
    _defaultsSnapshot: null as Settings | null,
    _isApplyingSessionState: false,
  }),

  getters: {
    conversationModels: (state) => state.settings.conversationModels,
    isWebSearchEnabled: (state) => state.settings.webSearchEnabled,
    selectedAgentId: (state) => state.settings.selectedAgentId,
  },

  actions: {
    getSelectedAgentId(): string {
      return this.settings.selectedAgentId
    },

    updateConversationModels(models: Partial<ConversationModels>) {
      this.settings.conversationModels = { ...this.settings.conversationModels, ...models }
      persist(this.settings)
    },

    selectKnowledgeBases(kbIds: string[]) {
      this.settings.selectedKnowledgeBases = [...new Set(kbIds.filter(Boolean))]
      persist(this.settings)
    },

    addKnowledgeBase(kbId: string) {
      if (!kbId || this.settings.selectedKnowledgeBases.includes(kbId)) return
      this.settings.selectedKnowledgeBases.push(kbId)
      persist(this.settings)
    },

    removeKnowledgeBase(kbId: string) {
      this.settings.selectedKnowledgeBases = this.settings.selectedKnowledgeBases.filter(id => id !== kbId)
      persist(this.settings)
    },

    clearKnowledgeBases() {
      this.settings.selectedKnowledgeBases = []
      persist(this.settings)
    },

    getSelectedKnowledgeBases(): string[] {
      return this.settings.selectedKnowledgeBases
    },

    toggleWebSearch(enabled: boolean) {
      this.settings.webSearchEnabled = enabled
      persist(this.settings)
    },

    addFile(fileId: string) {
      if (!fileId || this.settings.selectedFiles.includes(fileId)) return
      this.settings.selectedFiles.push(fileId)
      persist(this.settings)
    },

    removeFile(fileId: string) {
      this.settings.selectedFiles = this.settings.selectedFiles.filter(id => id !== fileId)
      delete this.settings.selectedFileKbMap[fileId]
      persist(this.settings)
    },

    clearFiles() {
      this.settings.selectedFiles = []
      this.settings.selectedFileKbMap = {}
      persist(this.settings)
    },

    setFileKbMap(updates: Record<string, string>) {
      Object.assign(this.settings.selectedFileKbMap, updates)
      persist(this.settings)
    },

    removeFileKbId(fileId: string) {
      delete this.settings.selectedFileKbMap[fileId]
      persist(this.settings)
    },

    getSelectedFiles(): string[] {
      return this.settings.selectedFiles
    },

    snapshotAsDefaultsIfNeeded() {
      if (this._defaultsSnapshot) return
      this._defaultsSnapshot = JSON.parse(JSON.stringify(this.settings))
    },

    restoreDefaultsIfSnapshotted() {
      if (!this._defaultsSnapshot) return
      this.settings = this._defaultsSnapshot
      this._defaultsSnapshot = null
    },

    applyLastRequestState(state: SessionLastRequestStatePayload | null | undefined) {
      if (!state) return
      this._isApplyingSessionState = true
      try {
			this.settings.selectedAgentId = BUILTIN_QUICK_ANSWER_ID
        if (Array.isArray(state.knowledge_base_ids)) {
          this.settings.selectedKnowledgeBases = [...new Set(state.knowledge_base_ids)]
        }
        if (Array.isArray(state.knowledge_ids)) {
          this.settings.selectedFiles = [...state.knowledge_ids]
        }
        if (typeof state.web_search_enabled === 'boolean') {
          this.settings.webSearchEnabled = state.web_search_enabled
        }
      } finally {
        nextTick(() => {
          this._isApplyingSessionState = false
        })
      }
    },
  },
})

export interface SessionLastRequestStatePayload {
  knowledge_base_ids?: string[]
  knowledge_ids?: string[]
  web_search_enabled?: boolean
}
