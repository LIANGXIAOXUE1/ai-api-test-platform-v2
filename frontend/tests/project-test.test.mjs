import assert from 'node:assert/strict'
import { after, test } from 'node:test'
import { createServer } from 'vite'
import { ref } from 'vue'

const server = await createServer({
  configFile: false,
  server: { middlewareMode: true },
  appType: 'custom',
  logLevel: 'error',
})
after(() => server.close())
const { useProjectTest } = await server.ssrLoadModule('/src/features/project-test/useProjectTest.ts')

const report = {
  id: 73, projectId: 7, baseUrl: 'http://localhost:8082', task: '检查所有接口',
  permission: 'auto', executed: true, createdAt: '2026-11-10T00:00:00Z',
  summary: { total: 2, reachable: 1, unavailable: 0, pending: 1 },
  items: [
    { apiId: 1, name: 'list', method: 'GET', risk: '只读', decision: '已执行', reachable: true },
    { apiId: 2, name: 'create', method: 'POST', risk: '写入', decision: '等待审核', reachable: false },
  ],
  markdown: '# saved report',
}

function fixture(overrides = {}) {
  const calls = []
  const messages = []
  const deps = {
    selectedProjectId: ref(7),
    assistantOpenAPISpec: ref(null),
    assistantPermission: ref('default'),
    loading: ref(false),
    getEnvironment: () => ({ baseUrl: 'http://localhost:8082' }),
    getProjectName: () => 'Cloud Disk',
    loadEndpoints: async () => { calls.push({ action: 'reload-endpoints' }) },
    refresh: async () => { calls.push({ action: 'refresh' }) },
    flash: (text) => messages.push(text),
    streamAssistantReply: async (text) => { messages.push(text) },
    request: async (path, options) => {
      calls.push({ path, body: options?.body ? JSON.parse(options.body) : undefined })
      if (path === '/api/apis/import') return { imported: 2 }
      if (path.startsWith('/api/project-check-reports')) return [structuredClone(report)]
      if (path === '/api/project-check') return structuredClone(report)
      throw new Error('unexpected request: ' + path)
    },
    ...overrides,
  }
  return { deps, calls, messages, state: useProjectTest(deps) }
}

test('review creates a draft without making network requests', async () => {
  const { state, calls, messages, deps } = fixture()
  await state.prepareAssistantProjectCheck('检查所有接口 http://127.0.0.1:8082/')
  assert.equal(calls.length, 0)
  assert.equal(state.assistantProjectCheck.value, null)
  assert.equal(state.assistantProjectCheckDraft.value.baseUrl, 'http://127.0.0.1:8082')
  assert.equal(deps.loading.value, false)
  assert.ok(messages.some((text) => text.includes('确认并开始巡检')))
})

test('confirmation imports first, runs the existing API, and stores the result', async () => {
  const { state, calls, deps } = fixture()
  deps.assistantOpenAPISpec.value = { openapi: '3.0.0', paths: {} }
  await state.prepareAssistantProjectCheck('检查所有接口')
  await state.confirmAssistantProjectCheck()
  assert.deepEqual(calls.map((call) => call.path || call.action),
    ['/api/apis/import', 'reload-endpoints', '/api/project-check', 'refresh'])
  assert.deepEqual(calls[2].body, {
    projectId: 7, baseUrl: 'http://localhost:8082', task: '检查所有接口',
    permission: 'auto', confirmed: true,
  })
  assert.equal(state.assistantProjectCheck.value.id, 73)
  assert.equal(state.assistantProjectCheckDraft.value, null)
  assert.equal(deps.assistantOpenAPISpec.value, null)
  assert.equal(deps.loading.value, false)
})

test('automatic mode preserves the same execution path without a review draft', async () => {
  const { state, deps, calls } = fixture()
  deps.assistantPermission.value = 'project_full'
  await state.prepareAssistantProjectCheck('检查所有接口')
  assert.equal(state.assistantProjectCheckDraft.value, null)
  assert.equal(state.assistantProjectCheck.value.id, 73)
  assert.equal(calls[0].path, '/api/project-check')
})

test('missing address asks for input and does not call an API', async () => {
  const { state, calls, messages } = fixture({ getEnvironment: () => ({}) })
  await state.prepareAssistantProjectCheck('检查所有接口')
  assert.equal(calls.length, 0)
  assert.equal(state.assistantProjectCheck.value, null)
  assert.ok(messages.some((text) => text.includes('还缺少已启动项目的根地址')))
})

test('an API error releases loading and keeps the pending draft for review', async () => {
  const { state, deps, messages } = fixture({
    request: async () => { throw new Error('backend unavailable') },
  })
  await state.prepareAssistantProjectCheck('检查所有接口')
  await state.confirmAssistantProjectCheck()
  assert.equal(deps.loading.value, false)
  assert.equal(state.assistantProjectCheck.value, null)
  assert.ok(state.assistantProjectCheckDraft.value)
  assert.ok(messages.includes('backend unavailable'))
})

test('history restores results and pending review items independently of other instances', async () => {
  const first = fixture()
  const second = fixture()
  await first.state.loadProjectCheckReports()
  first.state.assistantReviewOpen.value = true
  first.state.restoreProjectCheckReport(first.state.projectCheckReports.value[0])
  assert.equal(first.state.assistantReviewOpen.value, false)
  assert.equal(first.state.assistantProjectCheck.value.markdown, '# saved report')
  assert.deepEqual(first.state.assistantPendingReviewItems.value.map((item) => item.apiId), [2])
  assert.equal(second.state.projectCheckReports.value.length, 0)
  assert.equal(second.state.assistantProjectCheck.value, null)
})

test('history failure clears stale history without inventing reports', async () => {
  const { state } = fixture({ request: async () => { throw new Error('failed') } })
  state.projectCheckReports.value = [structuredClone(report)]
  await state.loadProjectCheckReports()
  assert.deepEqual(state.projectCheckReports.value, [])
})
