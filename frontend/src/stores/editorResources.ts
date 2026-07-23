import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  getParserEngines,
  type ParserEngineInfo,
} from '@/api/system'

const CACHE_TTL_MS = 60_000

type EditorResourceKey = 'parserEngines'

export const useEditorResourcesStore = defineStore('editorResources', () => {
  const parserEngines = ref<ParserEngineInfo[]>([])
  const loadedAt = ref<Partial<Record<EditorResourceKey, number>>>({})
  const inflight = new Map<EditorResourceKey, Promise<void>>()

  function isFresh(key: EditorResourceKey): boolean {
    const at = loadedAt.value[key]
    return !!at && Date.now() - at < CACHE_TTL_MS
  }

  async function runOnce(key: EditorResourceKey, force: boolean, loader: () => Promise<void>): Promise<void> {
    if (!force && isFresh(key)) return
    const existing = inflight.get(key)
    if (existing) return existing
    const promise = loader().finally(() => inflight.delete(key))
    inflight.set(key, promise)
    return promise
  }

  async function ensureParserEngines(force = false): Promise<void> {
    return runOnce('parserEngines', force, async () => {
      const response = await getParserEngines()
      parserEngines.value = Array.isArray(response?.data) ? response.data : []
      loadedAt.value.parserEngines = Date.now()
    })
  }

  function invalidate(...keys: EditorResourceKey[]) {
    if (keys.length === 0) {
      loadedAt.value = {}
      parserEngines.value = []
      inflight.clear()
      return
    }
    keys.forEach((key) => {
      delete loadedAt.value[key]
      inflight.delete(key)
    })
  }

  return {
    parserEngines,
    ensureParserEngines,
    invalidate,
  }
})
