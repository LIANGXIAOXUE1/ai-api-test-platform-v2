import assert from 'node:assert/strict'
import { after, test } from 'node:test'
import { createServer } from 'vite'

const server = await createServer({ configFile: false, server: { middlewareMode: true, hmr: false }, appType: 'custom', logLevel: 'error' })
after(() => server.close())
const { clearKnowledgeDraft, readKnowledgeDraft, writeKnowledgeDraft } = await server.ssrLoadModule('/src/features/knowledge/draft.ts')

function memoryStorage() {
  const values = new Map()
  return { getItem: key => values.get(key) ?? null, setItem: (key, value) => values.set(key, value), removeItem: key => values.delete(key) }
}

test('draft fields and unfinished generated cases survive route remounts per project', () => {
  const storage = memoryStorage()
  const draft = { prompt: '测试文件列表', supplement: '根目录只显示本人文件', mode: 'executable', apiId: 262, selectedModel: 4, selectedDocuments: [9], cases: [{ name: '根目录列表', saved: false }], missing: ['需要配置 token'] }
  assert.equal(writeKnowledgeDraft(1, draft, storage), true)
  assert.deepEqual(readKnowledgeDraft(1, storage), draft)
  assert.equal(readKnowledgeDraft(2, storage), null)
})

test('broken or outdated session drafts are ignored safely', () => {
  const storage = memoryStorage()
  storage.setItem('pulse-knowledge-draft:v1:1', '{broken json')
  assert.equal(readKnowledgeDraft(1, storage), null)
  storage.setItem('pulse-knowledge-draft:v1:2', JSON.stringify({ version: 0, projectId: 2, draft: {} }))
  assert.equal(readKnowledgeDraft(2, storage), null)
})

test('storage quota failures do not break knowledge page actions', () => {
  const storage = { getItem: () => null, setItem: () => { throw new Error('quota exceeded') } }
  assert.equal(writeKnowledgeDraft(1, { prompt: '', supplement: '', mode: 'design', apiId: 0, selectedModel: 0, selectedDocuments: [], cases: [], missing: [] }, storage), false)
})

test('clearing an empty draft removes only the current project draft', () => {
  const storage = memoryStorage()
  const draft = { prompt: '测试', supplement: '', mode: 'design', apiId: 0, selectedModel: 0, selectedDocuments: [], cases: [], missing: [] }
  writeKnowledgeDraft(1, draft, storage)
  writeKnowledgeDraft(2, draft, storage)
  assert.equal(clearKnowledgeDraft(1, storage), true)
  assert.equal(readKnowledgeDraft(1, storage), null)
  assert.deepEqual(readKnowledgeDraft(2, storage), draft)
})
