export type KnowledgeDraft = {
  prompt: string
  supplement: string
  mode: 'design' | 'executable'
  apiId: number
  selectedModel: number
  selectedDocuments: number[]
  cases: any[]
  missing: string[]
}

type DraftStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>
const keyFor = (projectId: number) => `pulse-knowledge-draft:v1:${projectId}`

function sessionStorageIfAvailable(): DraftStorage | null {
  try { return typeof window === 'undefined' ? null : window.sessionStorage } catch { return null }
}

export function readKnowledgeDraft(projectId: number, storage = sessionStorageIfAvailable()): KnowledgeDraft | null {
  if (!projectId || !storage) return null
  try {
    const raw = storage.getItem(keyFor(projectId))
    if (!raw) return null
    const value = JSON.parse(raw)
    if (value?.version !== 1 || value?.projectId !== projectId || !value.draft || typeof value.draft !== 'object') return null
    const draft = value.draft
    return {
      prompt: typeof draft.prompt === 'string' ? draft.prompt : '',
      supplement: typeof draft.supplement === 'string' ? draft.supplement : '',
      mode: draft.mode === 'executable' ? 'executable' : 'design',
      apiId: Number.isFinite(draft.apiId) ? draft.apiId : 0,
      selectedModel: Number.isFinite(draft.selectedModel) ? draft.selectedModel : 0,
      selectedDocuments: Array.isArray(draft.selectedDocuments) ? draft.selectedDocuments.filter((id: unknown): id is number => Number.isInteger(id)) : [],
      cases: Array.isArray(draft.cases) ? draft.cases : [],
      missing: Array.isArray(draft.missing) ? draft.missing.filter((item: unknown): item is string => typeof item === 'string') : [],
    }
  } catch { return null }
}

export function writeKnowledgeDraft(projectId: number, draft: KnowledgeDraft, storage = sessionStorageIfAvailable()): boolean {
  if (!projectId || !storage) return false
  try {
    storage.setItem(keyFor(projectId), JSON.stringify({ version: 1, projectId, draft }))
    return true
  } catch { return false }
}

export function clearKnowledgeDraft(projectId: number, storage = sessionStorageIfAvailable()): boolean {
  if (!projectId || !storage) return false
  try {
    storage.removeItem(keyFor(projectId))
    return true
  } catch { return false }
}
