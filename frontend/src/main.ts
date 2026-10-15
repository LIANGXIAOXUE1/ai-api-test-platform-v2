import { computed, createApp, onMounted, ref, watch } from 'vue'
import './style.css'

type Project = { id: number; name: string; summary: string }
type Endpoint = { id: number; projectId: number; name: string; method: string; url: string; kind: string; headers: Record<string, string>; query: Record<string, string>; body: string; enabled: boolean }
type Assertion = { type: string; path: string; expected: string }
type RequestSpec = { method: string; url: string; headers: Record<string, string>; query: Record<string, string>; body: string }
type Draft = { summary: string; request: RequestSpec; assertions: Assertion[]; reasoning: string[]; mode?: string; intent?: string; endpoint?: Endpoint; requiresConfirmation?: boolean }
type TestCase = { id: number; projectId: number; apiId: number; name: string; prompt: string; request: RequestSpec; assertions: Assertion[]; createdAt: string }
type Run = { id: number; caseId: number; apiId: number; caseName: string; startedAt: string; durationMs: number; firstByteMs: number; statusCode: number; success: boolean; errorCategory: string; errorMessage: string; responseHeaders: Record<string, string>; responseBody: string; contentType: string; stream: boolean; inputTokens: number; outputTokens: number; tokensPerSec: number; assertionPass: number; assertionTotal: number }
type LLMSettings = { baseUrl: string; apiKey: string; model: string; authHeader: string; configured?: boolean; apiKeyMasked?: string }
type ChainStep = { apiId: number; name: string; headers: string; body: string; extract: string }
type ChainResult = { success: boolean; failedAt: number; steps: Run[]; variables: Record<string, string>; variableCount: number }
type AssistantMessage = { role: 'user' | 'assistant'; text: string }
type AgentPlanStep = { order: number; tool: string; title: string; description: string; requiresConfirmation: boolean }
type AgentPlan = { summary: string; intent: string; steps: AgentPlanStep[]; requiresConfirmation: boolean; mode: string }
type ProjectDocument = { id: number; projectId: number; name: string; docType: string; version: string; characters: number; chunkCount: number; createdAt: string }
type ModelProfile = { id: number; projectId: number; name: string; provider: string; baseUrl: string; apiKey: string; apiKeyMasked?: string; model: string; authHeader: string; enabled: boolean }
type BusinessCase = { name: string; scenario: string; rule: string; request: RequestSpec; assertions: Assertion[]; evidence: string[] }
type EvaluationResult = { profileId: number; profileName: string; provider: string; question: string; answer: string; success: boolean; durationMs: number; inputTokens: number; outputTokens: number; ruleCoverage: number; ruleTotal: number; citationFound: boolean; hallucinationRisk: string; matchedRules: string[]; evidence: string[]; error?: string }

const emptyRequest = (): RequestSpec => ({ method: 'POST', url: '/api/demo/echo', headers: { 'Content-Type': 'application/json' }, query: {}, body: '{\n  "message": "hello"\n}' })
const api = async (path: string, options: RequestInit = {}) => {
  const response = await fetch(path, { headers: { 'Content-Type': 'application/json', ...(options.headers || {}) }, ...options })
  const data = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(data.error || '请求失败')
  return data
}
const parsePairs = (value: unknown): Record<string, string> => {
  if (typeof value !== 'string') return (value as Record<string, string>) || {}
  const result: Record<string, string> = {}
  value.split('\n').forEach((line) => { const index = line.indexOf(':'); if (index > 0) result[line.slice(0, index).trim()] = line.slice(index + 1).trim() })
  return result
}
const queryText = (value: Record<string, string> = {}) => Object.entries(value).map(([key, item]) => `${key}: ${item}`).join('\n')
const maskHeader = (key: string, value: string) => /authorization|api[-_]?key|token|secret/i.test(key) && value ? `${value.slice(0, 6)}••••••` : value

const App = {
  setup() {
    const page = ref<'tester' | 'history' | 'regression' | 'chain' | 'knowledge' | 'evaluation' | 'settings'>('tester')
    const sidebarCollapsed = ref(false)
    const projects = ref<Project[]>([])
    const endpoints = ref<Endpoint[]>([])
    const runs = ref<Run[]>([])
    const cases = ref<TestCase[]>([])
    const selectedProjectId = ref(1)
    const selectedApiId = ref(1)
    const searchText = ref('')
    const prompt = ref('测试这个接口是否可用，并检查返回结果')
    const request = ref<RequestSpec>(emptyRequest())
    const assertions = ref<Assertion[]>([{ type: 'status', path: '', expected: '200' }])
    const queryEditor = ref('')
    const aiSummary = ref('')
    const aiReasoning = ref<string[]>([])
    const latest = ref<Run | null>(null)
    const notice = ref('')
    const loading = ref(false)
    const batchConcurrency = ref(4)
    const batchResult = ref<{ count: number; errors: string[] } | null>(null)
    const editingCaseId = ref(0)
    const regressionDetail = ref<any>(null)
    const showSecrets = ref(false)
    const llmForm = ref<LLMSettings>({ baseUrl: '', apiKey: '', model: 'gpt-4o-mini', authHeader: 'Authorization' })
    const llmConfigured = ref(false)
    const environmentText = ref('')
    const showEndpointModal = ref(false)
    const showProjectModal = ref(false)
    const openApiFile = ref<File | null>(null)
    const projectForm = ref({ name: '', summary: '' })
    const endpointForm = ref({ name: '', method: 'GET', url: '', headers: '', body: '', kind: 'REST' })
    const chainSteps = ref<ChainStep[]>([])
    const chainVariablesText = ref('token: demo-token')
    const chainResult = ref<ChainResult | null>(null)
    const showAssistant = ref(false)
    const assistantInput = ref(prompt.value)
    const assistantMessages = ref<AssistantMessage[]>([])
    const assistantEndpointDraft = ref<Endpoint | null>(null)
    const assistantDocumentName = ref('')
    const assistantDocumentText = ref('')
    const assistantDocumentStatus = ref('')
    const assistantPlan = ref<AgentPlan | null>(null)
    const documents = ref<ProjectDocument[]>([])
    const selectedDocumentIds = ref<number[]>([])
    const documentType = ref('业务规则')
    const documentVersion = ref('v1')
    const businessCases = ref<BusinessCase[]>([])
    const modelProfiles = ref<ModelProfile[]>([])
    const selectedProfileIds = ref<number[]>([])
    const modelProfileForm = ref({ name: '', provider: 'DeepSeek', baseUrl: 'https://api.deepseek.com/chat/completions', apiKey: '', model: 'deepseek-chat', authHeader: 'Authorization' })
    const evaluationQuestions = ref('订单金额最低是多少？\n什么情况下可以退款？\n已退款订单能否再次退款？\n未登录下单应该返回什么？')
    const evaluationResults = ref<EvaluationResult[]>([])
    const evaluationLoading = ref(false)
    const assistantDocumentId = ref(0)
    const assistantFabPosition = ref<{ x: number; y: number } | null>(null)
    const assistantFabDragging = ref(false)
    let assistantFabDragState: { offsetX: number; offsetY: number; startX: number; startY: number; moved: boolean } | null = null
    let suppressAssistantFabClick = false

    function loadUiPreferences() {
      try {
        sidebarCollapsed.value = localStorage.getItem('pulse-api-sidebar-collapsed') === '1'
        const savedFab = localStorage.getItem('pulse-api-fab-position')
        if (savedFab) {
          const parsed = JSON.parse(savedFab)
          if (Number.isFinite(parsed.x) && Number.isFinite(parsed.y)) assistantFabPosition.value = { x: parsed.x, y: parsed.y }
        }
      } catch { sidebarCollapsed.value = false }
    }
    function toggleSidebar() {
      sidebarCollapsed.value = !sidebarCollapsed.value
      try { localStorage.setItem('pulse-api-sidebar-collapsed', sidebarCollapsed.value ? '1' : '0') } catch { /* storage is optional */ }
    }
    watch(page, () => {
      if (!sidebarCollapsed.value) toggleSidebar()
    })
    const assistantFabStyle = computed(() => assistantFabPosition.value ? { left: `${assistantFabPosition.value.x}px`, top: `${assistantFabPosition.value.y}px`, right: 'auto', bottom: 'auto' } : {})
    function startAssistantFabDrag(event: PointerEvent) {
      if (event.button !== 0) return
      const element = event.currentTarget as HTMLElement
      const rect = element.getBoundingClientRect()
      assistantFabDragState = { offsetX: event.clientX - rect.left, offsetY: event.clientY - rect.top, startX: event.clientX, startY: event.clientY, moved: false }
      assistantFabDragging.value = true
      element.setPointerCapture?.(event.pointerId)
      window.addEventListener('pointermove', moveAssistantFab)
      window.addEventListener('pointerup', stopAssistantFabDrag, { once: true })
    }
    function moveAssistantFab(event: PointerEvent) {
      if (!assistantFabDragState) return
      const drag = assistantFabDragState
      if (Math.hypot(event.clientX - drag.startX, event.clientY - drag.startY) > 5) drag.moved = true
      if (!drag.moved) return
      const size = 76
      const x = Math.max(8, Math.min(window.innerWidth - size - 8, event.clientX - drag.offsetX))
      const y = Math.max(8, Math.min(window.innerHeight - size - 8, event.clientY - drag.offsetY))
      assistantFabPosition.value = { x, y }
    }
    function stopAssistantFabDrag() {
      if (!assistantFabDragState) return
      suppressAssistantFabClick = assistantFabDragState.moved
      if (assistantFabDragState.moved && assistantFabPosition.value) {
        try { localStorage.setItem('pulse-api-fab-position', JSON.stringify(assistantFabPosition.value)) } catch { /* storage is optional */ }
      }
      assistantFabDragState = null
      assistantFabDragging.value = false
      window.removeEventListener('pointermove', moveAssistantFab)
    }
    function clickAssistantFab() {
      if (suppressAssistantFabClick) { suppressAssistantFabClick = false; return }
      openAssistant()
    }

    const selectedEndpoint = computed(() => endpoints.value.find((item) => item.id === selectedApiId.value))
    const filteredEndpoints = computed(() => { const query = searchText.value.trim().toLowerCase(); return query ? endpoints.value.filter((item) => `${item.name} ${item.url}`.toLowerCase().includes(query)) : endpoints.value })
    const successRate = computed(() => runs.value.length ? Math.round(runs.value.filter((item) => item.success).length / runs.value.length * 100) : 0)
    const savedRuns = computed(() => runs.value.filter((item) => item.caseId))
    const evaluationQuestionCount = computed(() => evaluationQuestions.value.split(/\r?\n/).filter(Boolean).length)
    const responsePretty = computed(() => { const body = latest.value?.responseBody || ''; try { return JSON.stringify(JSON.parse(body), null, 2) } catch { return body } })
    const responseHeadersText = computed(() => Object.entries(latest.value?.responseHeaders || {}).map(([key, value]) => `${key}: ${value}`).join('\n'))
    const headerEditorText = computed(() => Object.entries(request.value.headers || {}).map(([key, value]) => `${key}: ${showSecrets.value ? value : maskHeader(key, value)}`).join('\n'))

    function flash(message: string) { notice.value = message; window.setTimeout(() => { if (notice.value === message) notice.value = '' }, 3200) }
    async function auditAgent(action: string, result: Record<string, unknown> = {}, confirmed = false) {
      try { await api('/api/agent/audit', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, apiId: selectedApiId.value, action, prompt: prompt.value, result, confirmed }) }) } catch { /* 审计失败不影响主流程 */ }
    }
    async function loadProjectKnowledge() { try { documents.value = await api(`/api/documents?projectId=${selectedProjectId.value}`); modelProfiles.value = await api(`/api/models?projectId=${selectedProjectId.value}`) } catch { documents.value = []; modelProfiles.value = [] } }
    async function refresh() { projects.value = await api('/api/projects'); if (!projects.value.some((item) => item.id === selectedProjectId.value)) selectedProjectId.value = projects.value[0]?.id || 1; await loadEndpoints(); runs.value = await api(`/api/runs?projectId=${selectedProjectId.value}`); cases.value = await api(`/api/cases?projectId=${selectedProjectId.value}`); await loadProjectKnowledge(); await loadLLMSettings() }
    async function loadEndpoints() { endpoints.value = await api(`/api/apis?projectId=${selectedProjectId.value}`); if (!endpoints.value.some((item) => item.id === selectedApiId.value)) selectedApiId.value = endpoints.value[0]?.id || 0; if (selectedEndpoint.value) selectEndpoint(selectedEndpoint.value) }
    function selectEndpoint(item: Endpoint) { selectedApiId.value = item.id; request.value = { method: item.method, url: item.url, headers: item.headers || {}, query: item.query || {}, body: item.body || '' }; queryEditor.value = queryText(item.query || {}); aiSummary.value = ''; latest.value = null }
    async function selectProject() { await loadEndpoints(); runs.value = await api(`/api/runs?projectId=${selectedProjectId.value}`); cases.value = await api(`/api/cases?projectId=${selectedProjectId.value}`); await loadProjectKnowledge(); flash('已切换项目，已切换到对应项目的接口和知识文档') }
    async function loadLLMSettings() { try { const data = await api('/api/settings/llm') as LLMSettings; llmConfigured.value = !!data.configured; llmForm.value = { baseUrl: data.baseUrl || '', apiKey: data.apiKeyMasked || '', model: data.model || 'gpt-4o-mini', authHeader: data.authHeader || 'Authorization' } } catch { /* settings are optional */ } }
    async function saveLLMSettings() { try { const data = await api('/api/settings/llm', { method: 'POST', body: JSON.stringify(llmForm.value) }); llmConfigured.value = !!data.configured; flash(llmConfigured.value ? '模型配置已保存，Agent 将优先调用真实模型' : '配置已保存，未填写完整密钥时使用规则 Agent') } catch (error) { flash((error as Error).message) } }
    function loadEnvironment() { try { environmentText.value = localStorage.getItem('pulse-api-v2-environment') || 'baseUrl: http://localhost:8080\ntoken: demo-token' } catch { environmentText.value = 'baseUrl: http://localhost:8080\ntoken: demo-token' } }
    function saveEnvironment() { try { localStorage.setItem('pulse-api-v2-environment', environmentText.value) } catch { /* local storage can be unavailable */ } ; flash('环境变量已保存，可在请求中使用 {{变量名}}') }
    function resolveValue(value: string) { let result = value; Object.entries(parsePairs(environmentText.value)).forEach(([key, item]) => { result = result.split(`{{${key}}}`).join(item) }); return result }
    function formatPairs(value: Record<string, string> = {}) { return Object.entries(value).map(([key, item]) => `${key}: ${item}`).join('\n') }
    function resolvedRequest(): RequestSpec { return { method: request.value.method, url: resolveValue(request.value.url), headers: Object.fromEntries(Object.entries(request.value.headers || {}).map(([key, value]) => [key, resolveValue(value)])), query: Object.fromEntries(Object.entries(request.value.query || {}).map(([key, value]) => [key, resolveValue(value)])), body: resolveValue(request.value.body || '') } }
    function updateHeaders(event: Event) { if (!showSecrets.value) return flash('请先点击“显示密钥”后再编辑敏感请求头'); request.value.headers = parsePairs((event.target as HTMLTextAreaElement).value) }
    function updateQuery(event: Event) { queryEditor.value = (event.target as HTMLTextAreaElement).value; request.value.query = parsePairs(queryEditor.value) }
    function selectOpenAPI(event: Event) { openApiFile.value = (event.target as HTMLInputElement).files?.[0] || null }
    async function importOpenAPI() { if (!openApiFile.value) return flash('请先选择 OpenAPI JSON 文件'); try { const spec = JSON.parse(await openApiFile.value.text()); const result = await api('/api/apis/import', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, spec }) }); await loadEndpoints(); flash(`已导入 ${result.imported} 个接口`) } catch (error) { flash(`导入失败：${(error as Error).message}`) } }
    async function uploadProjectDocument(event: Event) {
      const file = (event.target as HTMLInputElement).files?.[0]
      if (!file) return
      try {
        const form = new FormData(); form.append('file', file); form.append('projectId', String(selectedProjectId.value)); form.append('docType', documentType.value); form.append('version', documentVersion.value || 'v1')
        await fetch('/api/documents', { method: 'POST', body: form }).then(async (response) => { const data = await response.json().catch(() => ({})); if (!response.ok) throw new Error(data.error || '文档保存失败'); return data })
        await loadProjectKnowledge(); flash(`已把《${file.name}》保存到当前项目知识库`)
      } catch (error) { flash((error as Error).message) }
      ;(event.target as HTMLInputElement).value = ''
    }
    async function deleteProjectDocument(item: ProjectDocument) { try { await api(`/api/documents?projectId=${selectedProjectId.value}&id=${item.id}`, { method: 'DELETE' }); selectedDocumentIds.value = selectedDocumentIds.value.filter((id) => id !== item.id); await loadProjectKnowledge(); flash(`已删除《${item.name}》`) } catch (error) { flash((error as Error).message) } }
    function toggleDocument(id: number) { selectedDocumentIds.value = selectedDocumentIds.value.includes(id) ? selectedDocumentIds.value.filter((item) => item !== id) : [...selectedDocumentIds.value, id] }
    async function generateBusinessCases() {
      if (!documents.value.length) return flash('请先上传业务规则、PRD 或错误码文档')
      try { const result = await api('/api/agent/business-cases', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, apiId: selectedApiId.value, documentIds: selectedDocumentIds.value, prompt: '根据当前项目业务规则生成订单接口测试用例' }) }); businessCases.value = result.cases || []; flash(`已根据 ${result.documents?.length || 0} 份项目文档生成 ${businessCases.value.length} 条用例`) } catch (error) { flash((error as Error).message) }
    }
    async function saveModelProfile() {
      if (!modelProfileForm.value.name || !modelProfileForm.value.apiKey) return flash('请填写模型名称和 API Key')
      try { const item = await api(`/api/models?projectId=${selectedProjectId.value}`, { method: 'POST', body: JSON.stringify(modelProfileForm.value) }) as ModelProfile; modelProfiles.value.push(item); selectedProfileIds.value.push(item.id); modelProfileForm.value = { ...modelProfileForm.value, name: '', apiKey: '' }; flash(`已保存模型配置：${item.name}`) } catch (error) { flash((error as Error).message) }
    }
    function toggleProfile(id: number) { selectedProfileIds.value = selectedProfileIds.value.includes(id) ? selectedProfileIds.value.filter((item) => item !== id) : [...selectedProfileIds.value, id] }
    async function runEvaluation() {
      const questions = evaluationQuestions.value.split('\n').map((item) => item.trim()).filter(Boolean)
      if (!selectedProfileIds.value.length) return flash('请至少选择一个模型')
      if (!questions.length) return flash('请至少填写一个评测问题')
      evaluationLoading.value = true
      try { const result = await api('/api/evaluations/run', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, profileIds: selectedProfileIds.value, documentIds: selectedDocumentIds.value, questions }) }); evaluationResults.value = result.results || []; flash(`已完成 ${selectedProfileIds.value.length} 个模型 × ${questions.length} 个问题的并发评测`) } catch (error) { flash((error as Error).message) } finally { evaluationLoading.value = false }
    }
    async function createEndpoint() {
      if (!endpointForm.value.name.trim() || !endpointForm.value.url.trim()) return flash('接口名称和 URL 不能为空')
      try {
        const item = await api('/api/apis', { method: 'POST', body: JSON.stringify({ ...endpointForm.value, projectId: selectedProjectId.value, headers: parsePairs(endpointForm.value.headers), query: {}, enabled: true }) })
        await loadEndpoints(); selectEndpoint(item); showEndpointModal.value = false; endpointForm.value = { name: '', method: 'GET', url: '', headers: '', body: '', kind: 'REST' }; flash('接口已添加')
      } catch (error) { flash((error as Error).message) }
    }
    async function createProject() {
      if (!projectForm.value.name.trim()) return flash('请填写项目名称')
      try { const item = await api('/api/projects', { method: 'POST', body: JSON.stringify(projectForm.value) }); projects.value.push(item); selectedProjectId.value = item.id; projectForm.value = { name: '', summary: '' }; showProjectModal.value = false; await loadEndpoints(); flash('项目已创建') } catch (error) { flash((error as Error).message) }
    }
    async function generateDraft(): Promise<Draft | null> {
      if (!prompt.value.trim()) { flash('请先描述想测试什么'); return null }
      if (!selectedApiId.value) { flash('请先选择一个接口'); return null }
      loading.value = true
      try {
        const draft = await api('/api/agent/draft', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, apiId: selectedApiId.value, prompt: prompt.value, documentIds: assistantDocumentId.value ? [assistantDocumentId.value] : selectedDocumentIds.value }) }) as Draft
        assistantEndpointDraft.value = draft.intent === 'add_api' && draft.endpoint ? draft.endpoint : null
        if (assistantEndpointDraft.value) {
          aiSummary.value = draft.summary
          aiReasoning.value = draft.reasoning || []
          flash('AI 已生成接口配置，请确认后添加')
          return draft
        }
        request.value = draft.request
        assertions.value = draft.assertions?.length ? draft.assertions : [{ type: 'status', path: '', expected: '200' }]
        queryEditor.value = queryText(draft.request.query || {})
        aiSummary.value = draft.summary
        aiReasoning.value = draft.reasoning || []
        await auditAgent('draft_generated', { intent: draft.intent || 'test_api', mode: draft.mode || 'rule-agent', summary: draft.summary })
        flash('AI 已补齐请求，请确认后发送')
        return draft
      } catch (error) { flash((error as Error).message); return null } finally { loading.value = false }
    }
    function openAssistant() { assistantInput.value = prompt.value; showAssistant.value = true; if (!assistantMessages.value.length) assistantMessages.value.push({ role: 'assistant', text: `你好，我可以帮你配置“${selectedEndpoint.value?.name || '当前项目'}”。告诉我你想测试什么、添加什么接口，也可以上传接口文档。` }) }
    function closeAssistant() { showAssistant.value = false }
    async function selectAssistantDocument(event: Event) {
      const file = (event.target as HTMLInputElement).files?.[0]
      if (!file) return
      assistantDocumentName.value = file.name
      assistantDocumentStatus.value = '识别中…'
      try {
        const form = new FormData()
        form.append('file', file)
        const response = await fetch('/api/agent/document', { method: 'POST', body: form })
        const data = await response.json().catch(() => ({}))
        if (!response.ok) throw new Error(data.error || '文档识别失败')
        assistantDocumentText.value = data.text || ''
        assistantDocumentStatus.value = `已识别 ${data.characters || 0} 字`
        try {
          const knowledgeForm = new FormData(); knowledgeForm.append('file', file); knowledgeForm.append('projectId', String(selectedProjectId.value)); knowledgeForm.append('docType', file.name.toLowerCase().includes('openapi') || file.name.toLowerCase().includes('swagger') ? '接口契约' : '业务规则'); knowledgeForm.append('version', 'v1')
          const saved = await fetch('/api/documents', { method: 'POST', body: knowledgeForm }).then(async (item) => { const body = await item.json().catch(() => ({})); if (!item.ok) throw new Error(body.error || '项目文档保存失败'); return body })
          assistantDocumentId.value = saved.id || 0
          await loadProjectKnowledge()
        } catch { assistantDocumentId.value = 0 }
        assistantMessages.value.push({ role: 'assistant', text: `已读取《${file.name}》，识别出 ${data.characters || 0} 个字符。接下来可以让我根据文档添加接口或生成测试。` })
      } catch (error) {
        assistantDocumentName.value = ''
        assistantDocumentText.value = ''
        assistantDocumentStatus.value = ''
        flash((error as Error).message)
      }
      ;(event.target as HTMLInputElement).value = ''
    }
    function clearAssistantDocument() { assistantDocumentName.value = ''; assistantDocumentText.value = ''; assistantDocumentStatus.value = ''; assistantDocumentId.value = 0 }
    async function askAssistant() {
      const question = assistantInput.value.trim()
      if (!question) return flash('请先描述你想验证的内容')
      assistantMessages.value.push({ role: 'user', text: question })
      const taskPrompt = assistantDocumentText.value ? `${question}\n\n参考文档《${assistantDocumentName.value}》：\n${assistantDocumentText.value}` : question
      prompt.value = taskPrompt
      assistantInput.value = ''
      assistantEndpointDraft.value = null
      assistantPlan.value = null
      businessCases.value = []
      if (documents.value.length && /测试用例|业务规则|订单|退款|库存/.test(question)) {
        try {
          const result = await api('/api/agent/business-cases', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, apiId: selectedApiId.value, documentIds: assistantDocumentId.value ? [assistantDocumentId.value] : selectedDocumentIds.value, prompt: question }) })
          businessCases.value = result.cases || []
          if (businessCases.value.length) assistantMessages.value.push({ role: 'assistant', text: `我根据当前项目文档生成了 ${businessCases.value.length} 条业务测试用例，下面会显示每条规则依据。你确认后可以回到知识库或测试台执行。` })
        } catch { /* 没有匹配到业务文档时继续走普通请求草稿流程 */ }
      }
      try {
        assistantPlan.value = await api('/api/agent/plan', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, apiId: selectedApiId.value, prompt: taskPrompt }) }) as AgentPlan
      } catch (error) { flash(`Agent 计划生成失败：${(error as Error).message}`) }
      const draft = await generateDraft()
      prompt.value = question
      if (draft?.intent === 'add_api' && draft.endpoint) assistantMessages.value.push({ role: 'assistant', text: `${aiSummary.value} 你可以先检查名称、方法和 URL，确认后添加到当前项目。` })
      else if (aiSummary.value) assistantMessages.value.push({ role: 'assistant', text: `${aiSummary.value} 你可以先查看下面的请求草稿，确认后应用到测试台。` })
    }
    function useAssistantSuggestion(text: string) { assistantInput.value = text; askAssistant() }
    async function applyAssistantDraft() {
      if (assistantEndpointDraft.value) {
        try {
          await auditAgent('endpoint_confirmed', { name: assistantEndpointDraft.value.name, method: assistantEndpointDraft.value.method, url: assistantEndpointDraft.value.url }, true)
          const created = await api('/api/apis', { method: 'POST', body: JSON.stringify(assistantEndpointDraft.value) }) as Endpoint
          await loadEndpoints()
          selectEndpoint(created)
          assistantEndpointDraft.value = null
          showAssistant.value = false
          page.value = 'tester'
          flash('接口已添加到当前项目，已自动填入测试台')
        } catch (error) { flash(`添加接口失败：${(error as Error).message}`) }
        return
      }
      showAssistant.value = false
      page.value = 'tester'
      await auditAgent('draft_applied', { method: request.value.method, url: request.value.url }, true)
      flash('已应用 Agent 草稿，请检查请求后再发送')
    }
    async function sendRequest(saveCase: boolean) {
      if (!request.value.url.trim()) return flash('请输入请求 URL')
      loading.value = true
      try { const run = await api('/api/runs', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, apiId: selectedApiId.value, name: prompt.value.slice(0, 40) || selectedEndpoint.value?.name || '临时请求', prompt: prompt.value, request: resolvedRequest(), assertions: assertions.value, saveCase }) }) as Run; latest.value = run; runs.value.unshift(run); await auditAgent('api_test_executed', { runId: run.id, statusCode: run.statusCode, success: run.success }, true); flash(run.success ? '请求成功，结果已保存' : `请求完成：${run.errorCategory || '存在失败断言'}`) } catch (error) { flash((error as Error).message) } finally { loading.value = false }
    }
    function loadCase(item: TestCase) { editingCaseId.value = item.id; selectedApiId.value = item.apiId; request.value = item.request; queryEditor.value = queryText(item.request.query || {}); assertions.value = item.assertions?.length ? item.assertions : [{ type: 'status', path: '', expected: '200' }]; prompt.value = item.prompt || item.name; latest.value = null; page.value = 'tester'; flash(`已载入用例：${item.name}`) }
    async function updateCase() { if (!editingCaseId.value) return flash('请先从历史中载入一个测试用例'); try { const item = { id: editingCaseId.value, projectId: selectedProjectId.value, apiId: selectedApiId.value, name: prompt.value.slice(0, 40) || '已编辑测试用例', prompt: prompt.value, request: resolvedRequest(), assertions: assertions.value }; await api('/api/cases', { method: 'PUT', body: JSON.stringify(item) }); cases.value = await api('/api/cases'); flash('测试用例已更新') } catch (error) { flash((error as Error).message) } }
    async function runBatch() {
      if (!endpoints.value.length) return flash('当前项目没有接口')
      loading.value = true
      try { const result = await api('/api/runs/batch', { method: 'POST', body: JSON.stringify({ concurrency: batchConcurrency.value, items: endpoints.value.map((item) => ({ projectId: selectedProjectId.value, apiId: item.id, name: `批量 · ${item.name}`, request: { method: item.method, url: resolveValue(item.url), headers: item.headers || {}, query: item.query || {}, body: item.body || '' }, assertions: [{ type: 'status', path: '', expected: '200' }], saveCase: false })) }) }); batchResult.value = { count: result.count || 0, errors: result.errors || [] }; runs.value.unshift(...(result.runs || [])); flash(`批量测试完成：${result.count || 0} 个请求`) } catch (error) { flash((error as Error).message) } finally { loading.value = false }
    }
    function makeChainStep(item?: Endpoint): ChainStep { return { apiId: item?.id || endpoints.value[0]?.id || 0, name: item?.name || '', headers: queryText(item?.headers || {}), body: item?.body || '', extract: '' } }
    function addChainStep() { chainSteps.value.push(makeChainStep(endpoints.value[chainSteps.value.length] || endpoints.value[0])); chainResult.value = null }
    function syncChainStep(step: ChainStep) { const item = endpoints.value.find((endpoint) => endpoint.id === step.apiId); if (item) { step.name = item.name; step.headers = queryText(item.headers || {}); step.body = item.body || ''; step.extract = ''; } }
    function removeChainStep(index: number) { chainSteps.value.splice(index, 1); chainResult.value = null }
    async function runChain() {
      if (!chainSteps.value.length) return flash('请先添加至少一个链路步骤')
      loading.value = true
      try {
        const result = await api('/api/runs/chain', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, variables: parsePairs(chainVariablesText.value), steps: chainSteps.value.map((step) => ({ apiId: step.apiId, name: step.name, request: (() => { const endpoint = endpoints.value.find((item) => item.id === step.apiId); return { method: endpoint?.method || 'GET', url: endpoint?.url || '', headers: parsePairs(step.headers), query: endpoint?.query || {}, body: step.body } })(), assertions: [{ type: 'status', path: '', expected: '200' }], extract: parsePairs(step.extract) })) }) }) as ChainResult
        chainResult.value = result; runs.value.unshift(...(result.steps || [])); flash(result.success ? `链路测试通过，共 ${result.steps.length} 步` : `链路在第 ${result.failedAt} 步失败`)
      } catch (error) { flash((error as Error).message) } finally { loading.value = false }
    }
    async function inspectRegression(item: Run) { try { regressionDetail.value = await api(`/api/regression?caseId=${item.caseId}`) } catch (error) { flash((error as Error).message) } }
    async function rerun(item: Run) {
      loading.value = true
      try { const run = await api('/api/runs', { method: 'POST', body: JSON.stringify(item.caseId ? { caseId: item.caseId } : { apiId: item.apiId, request: request.value, assertions: assertions.value, name: item.caseName }) }) as Run; latest.value = run; runs.value.unshift(run); page.value = 'tester'; flash('已重放请求，可继续比较前后结果') } catch (error) { flash((error as Error).message) } finally { loading.value = false }
    }
    function diagnosis(item: Run | null) {
      if (!item) return { title: '发送请求后，结果会显示在这里', text: '状态码、耗时、响应内容和错误原因会自动整理。', tone: 'neutral' }
      if (item.success) return { title: '接口调用成功', text: '状态码和当前断言均符合预期，可以保存为回归用例。', tone: 'good' }
      if (item.statusCode === 401) return { title: '鉴权失败', text: '检查 Authorization、API Key 或 Token 是否缺失、过期。', tone: 'bad' }
      if (item.statusCode === 403) return { title: '权限不足', text: '当前凭证有效，但没有访问这个接口的权限。', tone: 'bad' }
      if (item.statusCode === 404) return { title: '接口地址不存在', text: '检查 URL、接口版本和网关路由是否正确。', tone: 'bad' }
      if (item.statusCode === 429) return { title: '请求过快或配额不足', text: '检查限流配置、模型配额和重试间隔。', tone: 'warn' }
      if (item.statusCode >= 500) return { title: '服务端异常', text: '请求已经到达服务端，建议查看上游服务日志和参数。', tone: 'bad' }
      return { title: item.errorCategory || '请求未通过', text: item.errorMessage || '请检查地址、网络和服务状态。', tone: 'bad' }
    }
    function copyResponse(item: Run | null) { if (item) navigator.clipboard?.writeText(item.responseBody || '') ; flash('响应内容已复制') }
    onMounted(() => {
      loadUiPreferences()
      loadEnvironment()
      refresh().catch((error) => flash(error.message))
      // Docker 启动时后端可能比前端晚几秒，自动重试一次避免打开后列表为空。
      window.setTimeout(() => refresh().catch(() => undefined), 1800)
    })
    return { page, sidebarCollapsed, toggleSidebar, assistantFabStyle, assistantFabDragging, startAssistantFabDrag, clickAssistantFab, projects, endpoints, runs, cases, selectedProjectId, selectedApiId, prompt, request, assertions, queryEditor, aiSummary, aiReasoning, latest, notice, loading, showEndpointModal, showProjectModal, showAssistant, assistantInput, assistantMessages, assistantEndpointDraft, assistantDocumentName, assistantDocumentText, assistantDocumentStatus, assistantPlan, projectForm, endpointForm, selectedEndpoint, filteredEndpoints, successRate, savedRuns, responsePretty, responseHeadersText, headerEditorText, searchText, showSecrets, batchConcurrency, batchResult, editingCaseId, regressionDetail, llmForm, llmConfigured, environmentText, chainSteps, chainVariablesText, chainResult, documents, selectedDocumentIds, documentType, documentVersion, businessCases, modelProfiles, selectedProfileIds, modelProfileForm, evaluationQuestions, evaluationQuestionCount, evaluationResults, evaluationLoading, emptyRequest, refresh, loadEndpoints, selectEndpoint, selectProject, updateHeaders, updateQuery, selectOpenAPI, importOpenAPI, uploadProjectDocument, deleteProjectDocument, toggleDocument, generateBusinessCases, saveModelProfile, toggleProfile, runEvaluation, createEndpoint, createProject, generateDraft, openAssistant, closeAssistant, selectAssistantDocument, clearAssistantDocument, askAssistant, useAssistantSuggestion, applyAssistantDraft, sendRequest, loadCase, updateCase, runBatch, addChainStep, syncChainStep, removeChainStep, runChain, inspectRegression, loadLLMSettings, saveLLMSettings, saveEnvironment, rerun, diagnosis, copyResponse, formatPairs }
  },
  template: `
  <div class="app-v2" :class="{ 'sidebar-collapsed': sidebarCollapsed }">
    <header class="topbar-v2"><div class="brand-v2"><div class="brand-logo">P</div><div><strong>Pulse API</strong><span>AI API 测试工作台</span></div><button class="sidebar-toggle" :title="sidebarCollapsed ? '展开侧边栏' : '收起侧边栏'" @click="toggleSidebar">☰</button></div><div class="top-center"><span class="project-label">项目</span><select v-model="selectedProjectId" @change="selectProject"><option v-for="project in projects" :key="project.id" :value="project.id">{{ project.name }}</option></select><button class="icon-btn" title="新建项目" @click="showProjectModal=true">＋</button></div><div class="top-right"><span class="online"><i></i>服务正常</span><button class="assistant-nav-btn" @click="openAssistant">✦ <span>AI 助手</span></button><button class="nav-btn" :class="{active:page==='tester'}" @click="page='tester'">▣ <span>测试台</span></button><button class="nav-btn" :class="{active:page==='knowledge'}" @click="page='knowledge'">▤ <span>知识库</span></button><button class="nav-btn" :class="{active:page==='evaluation'}" @click="page='evaluation'">◈ <span>模型评测</span></button><button class="nav-btn" :class="{active:page==='history'}" @click="page='history'">◷ <span>历史</span></button><button class="nav-btn" :class="{active:page==='regression'}" @click="page='regression'">↻ <span>回归</span></button><button class="nav-btn" :class="{active:page==='chain'}" @click="page='chain'">⌁ <span>链路</span></button><button class="nav-btn" :class="{active:page==='settings'}" @click="page='settings'">⚙ <span>配置</span></button></div></header>
    <main v-if="page==='tester'" class="tester-page">
      <aside class="endpoint-panel"><div class="panel-heading"><div><span class="eyebrow">ENDPOINTS</span><h2>接口</h2></div><button class="add-btn" title="新增接口" @click="showEndpointModal=true">＋</button></div><div class="endpoint-search">⌕ <input v-model="searchText" placeholder="搜索接口" /></div><div class="endpoint-list"><button v-for="item in filteredEndpoints" :key="item.id" class="endpoint-item" :class="{selected:item.id===selectedApiId}" @click="selectEndpoint(item)"><span class="method" :class="item.method.toLowerCase()">{{ item.method }}</span><span class="endpoint-name"><b>{{ item.name }}</b><small>{{ item.url }}</small></span><span class="chevron">›</span></button><div v-if="!filteredEndpoints.length" class="empty-small">没有匹配的接口</div></div><div class="endpoint-footer"><label class="import-link">导入 OpenAPI <input type="file" accept=".json,application/json" @change="selectOpenAPI" /></label><button class="import-btn" @click="importOpenAPI">导入</button></div><div class="side-tip"><b>怎么开始？</b><span>选择接口 → 让 AI 补齐 → 发送请求</span></div></aside>
      <section class="request-panel"><div class="section-title"><div><span class="eyebrow">REQUEST BUILDER</span><h1>测试请求</h1><p>{{ selectedEndpoint?.name || '请选择一个接口' }} <span v-if="selectedEndpoint">· {{ selectedEndpoint.kind }}</span><span v-if="selectedEndpoint?.kind==='LLM'" class="stream-tag">SSE / 模型指标</span></p></div><button class="ghost-btn" @click="request=selectedEndpoint ? {method:selectedEndpoint.method,url:selectedEndpoint.url,headers:selectedEndpoint.headers||{},query:selectedEndpoint.query||{},body:selectedEndpoint.body||''} : emptyRequest()">恢复接口默认值</button></div><div class="assistant-entry"><div class="assistant-icon">✦</div><div class="assistant-copy"><b>不确定怎么填？让 AI 补齐参数</b><span>告诉它你想测试什么，它会生成 URL、请求体和断言草稿。</span></div><button class="assistant-preview" @click="openAssistant">{{ prompt }}</button><button class="ai-btn" :disabled="loading" @click="openAssistant">AI 补齐参数</button></div><div v-if="aiSummary" class="ai-result"><span>✓</span><div><b>{{ aiSummary }}</b><small v-if="aiReasoning.length">已识别测试目标并生成可编辑请求</small></div><details v-if="aiReasoning.length"><summary>生成依据</summary><span v-for="item in aiReasoning" :key="item">{{ item }}</span></details></div><div class="request-card"><div class="request-line"><select v-model="request.method" class="method-select"><option>GET</option><option>POST</option><option>PUT</option><option>DELETE</option><option>PATCH</option></select><input v-model="request.url" class="url-input" placeholder="输入接口 URL，例如 /api/demo/echo" /><button class="send-btn" :disabled="loading" @click="sendRequest(true)">▶ 发送并保存</button><button class="send-only" :disabled="loading" @click="sendRequest(false)">仅发送</button></div><div class="batch-line"><button class="batch-btn" :disabled="loading" @click="runBatch">批量测试当前项目接口</button><label>并发 <select v-model.number="batchConcurrency"><option :value="2">2</option><option :value="4">4</option><option :value="8">8</option></select></label><span v-if="batchResult">最近批量：{{ batchResult.count }} 成功记录</span></div><div class="editor-grid"><div class="editor-block"><div class="editor-label"><b>Headers</b><span>敏感值默认隐藏</span><button class="mini-add" @click="showSecrets=!showSecrets">{{ showSecrets ? '隐藏密钥' : '显示密钥' }}</button></div><textarea :value="headerEditorText" :readonly="!showSecrets" @input="updateHeaders" rows="6" placeholder="Authorization: Bearer xxx\nContent-Type: application/json"></textarea></div><div class="editor-block"><div class="editor-label"><b>Body</b><span>JSON 或文本</span></div><textarea v-model="request.body" rows="6" placeholder="{&quot;message&quot;:&quot;hello&quot;}"></textarea></div></div><details class="advanced-row"><summary>查询参数</summary><textarea :value="queryEditor" @input="updateQuery" rows="3" placeholder="page: 1\nkeyword: demo"></textarea></details><div class="assertion-block"><div class="editor-label"><b>发送后检查</b><span>基础断言</span><button class="mini-add" @click="assertions.push({type:'status',path:'',expected:'200'})">＋ 添加</button></div><div v-for="(item, index) in assertions" :key="index" class="assertion-row"><select v-model="item.type"><option value="status">状态码等于</option><option value="contains">响应包含文本</option><option value="json_field">JSON 存在字段</option><option value="json_contract">契约字段全部存在</option><option value="json_schema">JSON Schema 校验</option></select><input v-model="item.path" :placeholder="item.type==='json_schema' ? '根路径（可留空）' : '字段路径，多个字段用逗号分隔'" /><input v-model="item.expected" :placeholder="item.type==='json_schema' ? '{&quot;type&quot;:&quot;object&quot;,&quot;required&quot;:[...]}' : '期望值'" /><button class="remove-btn" @click="assertions.splice(index,1)">×</button></div></div></div></section>
      <aside class="response-panel"><div class="response-heading"><div><span class="eyebrow">RESPONSE</span><h2>响应结果</h2></div><span v-if="latest" class="result-pill" :class="latest.success?'ok':'fail'">{{ latest.success ? '通过' : '失败' }}</span></div><div v-if="latest" class="response-content"><div class="status-card" :class="latest.success?'status-good':'status-bad'"><div><b>{{ latest.success ? '请求成功' : (latest.errorCategory || '请求失败') }}</b><span>{{ latest.errorMessage || '状态码和断言均符合预期' }}</span></div><strong>{{ latest.statusCode || 'ERR' }}</strong></div><div class="metric-row"><div><span>耗时</span><b>{{ latest.durationMs }}<i>ms</i></b></div><div><span>TTFT / 首字节</span><b>{{ latest.firstByteMs }}<i>ms</i></b></div><div><span>断言</span><b>{{ latest.assertionPass }}/{{ latest.assertionTotal }}</b></div></div><div v-if="latest.stream || latest.outputTokens" class="token-row"><span>输入 {{ latest.inputTokens }} tokens</span><span>输出 {{ latest.outputTokens }} tokens</span><span>生成 {{ latest.tokensPerSec.toFixed(1) }} tok/s</span></div><div class="diagnosis" :class="diagnosis(latest).tone"><span class="diagnosis-icon">{{ latest.success ? '✓' : '!' }}</span><div><b>{{ diagnosis(latest).title }}</b><p>{{ diagnosis(latest).text }}</p></div></div><div class="response-tabs"><details open><summary>Response Body</summary><div class="response-body"><div class="editor-label"><b>{{ latest.stream ? 'SSE 流式响应' : 'JSON / 文本响应' }}</b><button class="copy-btn" @click="copyResponse(latest)">复制</button></div><pre>{{ responsePretty || '没有响应内容' }}</pre></div></details><details><summary>Response Headers</summary><pre class="headers-preview">{{ responseHeadersText || '没有响应头' }}</pre></details></div><button class="replay-btn" @click="rerun(latest)">↻ 重新发送</button></div><div v-else class="response-empty"><div class="empty-icon">↗</div><b>等待发送请求</b><span>结果、错误原因和响应内容<br/>会显示在这里</span></div></aside>
    </main>
    <section v-else-if="page==='history'" class="list-page"><div class="page-title"><div><span class="eyebrow">RUN HISTORY</span><h1>执行历史</h1><p>每次请求都会留下可重放记录。</p></div><button class="primary-btn" @click="page='tester'">＋ 新建测试</button></div><div class="summary-cards"><div><span>执行次数</span><b>{{ runs.length }}</b></div><div><span>成功率</span><b>{{ successRate }}%</b></div><div><span>可回归用例</span><b>{{ savedRuns.length }}</b></div></div><div class="table-card"><table><thead><tr><th>结果</th><th>测试名称</th><th>状态码</th><th>耗时</th><th>时间</th><th>操作</th></tr></thead><tbody><tr v-for="item in runs" :key="item.id"><td><span class="table-status" :class="item.success?'ok':'fail'">{{ item.success?'通过':'失败' }}</span></td><td><b>{{ item.caseName }}</b><small>{{ item.errorCategory || '请求正常' }}</small></td><td><code>{{ item.statusCode || 'ERR' }}</code></td><td>{{ item.durationMs }}ms</td><td>{{ new Date(item.startedAt).toLocaleString() }}</td><td><button class="link-btn" @click="latest=item; page='tester'">查看</button><button class="link-btn" @click="rerun(item)">重放</button><button v-if="item.caseId" class="link-btn" @click="loadCase(cases.find(testCase => testCase.id === item.caseId))">编辑用例</button></td></tr></tbody></table><div v-if="!runs.length" class="empty-small">暂无执行记录</div></div></section>
    <section v-else-if="page==='regression'" class="list-page"><div class="page-title"><div><span class="eyebrow">REGRESSION</span><h1>回归对比</h1><p>同一个保存用例重新执行后，比较前后结果有没有变差。</p></div><button class="primary-btn" @click="page='tester'">去执行测试</button></div><div class="regression-banner"><div class="regression-icon">↻</div><div><b>保存一次，后面一键重放</b><span>接口、模型或 Prompt 修改后，重新执行旧用例，平台会记录状态码、成功状态、耗时和响应结构变化。</span></div></div><div class="table-card"><table><thead><tr><th>测试用例</th><th>最近状态</th><th>状态码</th><th>耗时</th><th>操作</th></tr></thead><tbody><tr v-for="item in savedRuns" :key="item.id"><td><b>{{ item.caseName }}</b><small>Case #{{ item.caseId }}</small></td><td><span class="table-status" :class="item.success?'ok':'fail'">{{ item.success?'正常':'回归风险' }}</span></td><td>{{ item.statusCode || 'ERR' }}</td><td>{{ item.durationMs }}ms</td><td><button class="link-btn" @click="inspectRegression(item)">查看差异</button><button class="link-btn" @click="rerun(item)">重放并比较 →</button></td></tr></tbody></table><div v-if="!savedRuns.length" class="empty-small">先在测试台点击“发送并保存”，这里就会出现回归用例。</div></div><div v-if="regressionDetail" class="diff-card"><div class="diff-head"><div><span class="eyebrow">DIFF RESULT</span><h2>前后结果差异</h2></div><span class="table-status" :class="regressionDetail.diff?.length ? 'fail' : 'ok'">{{ regressionDetail.diff?.length ? '发现变化' : '没有明显变化' }}</span></div><div v-if="regressionDetail.diff?.length" class="diff-list"><div v-for="item in regressionDetail.diff" :key="item.message">! {{ item.message }}<div v-if="item.changes" class="diff-sublist"><span v-for="change in item.changes" :key="change.path">{{ change.message }}</span></div></div></div><div v-else class="empty-small">基线和最近一次结果一致。</div></div></section>
    <section v-else-if="page==='chain'" class="list-page chain-page"><div class="page-title"><div><span class="eyebrow">CHAIN TEST</span><h1>接口链路测试</h1><p>把多个接口串成一条业务流程，自动把上一步响应里的 Token、订单号或用户 ID 传给下一步。</p></div><button class="primary-btn" :disabled="loading" @click="runChain">▶ 执行链路</button></div><div class="regression-banner chain-banner"><div class="regression-icon">↗</div><div><b>可视化的接口串联</b><span>每一步都能看到真实请求和结果；变量只在本次链路内传递，Token 等敏感变量在结果中自动脱敏。</span></div></div><div class="chain-grid"><div class="settings-card"><div class="card-title"><div><span class="eyebrow">STEP FLOW</span><h2>链路步骤</h2></div><button class="mini-add" @click="addChainStep">＋ 添加步骤</button></div><label>初始变量（每行 key: value）<textarea v-model="chainVariablesText" rows="4" placeholder="token: demo-token"></textarea></label><div v-if="!chainSteps.length" class="empty-small">还没有步骤，点击“添加步骤”开始配置。</div><div v-for="(step, index) in chainSteps" :key="index" class="chain-step"><div class="chain-step-head"><b>步骤 {{ index + 1 }}</b><button class="remove-btn" @click="removeChainStep(index)">×</button></div><label>接口<select v-model.number="step.apiId" @change="syncChainStep(step)"><option v-for="item in endpoints" :key="item.id" :value="item.id">{{ item.name }} · {{ item.method }}</option></select></label><label>Headers 覆盖<textarea v-model="step.headers" rows="2" placeholder="Authorization: Bearer {{token}}"></textarea></label><label>Body<textarea v-model="step.body" rows="2" placeholder="可使用 {{上一步变量}}"></textarea></label><label>提取响应变量<textarea v-model="step.extract" rows="2" placeholder="token: token\nuserId: data.userId"></textarea></label></div><button class="primary-btn full-btn" :disabled="loading" @click="runChain">{{ loading ? '执行中…' : '执行整条链路' }}</button></div><div class="settings-card"><div class="card-title"><div><span class="eyebrow">CHAIN RESULT</span><h2>执行结果</h2></div><span v-if="chainResult" class="config-state" :class="chainResult.success?'configured':''"><span v-if="chainResult.success">● </span><span v-else>● 第 {{ chainResult.failedAt }} 步失败</span><span v-if="chainResult.success">全部通过</span></span></div><div v-if="chainResult" class="chain-result"><div v-for="(item, index) in chainResult.steps" :key="item.id" class="chain-result-row"><span class="step-index">{{ index + 1 }}</span><div><b>{{ item.caseName }}</b><small>{{ item.statusCode }} · {{ item.durationMs }}ms · {{ item.success ? '通过' : item.errorCategory || '失败' }}</small></div><span class="table-status" :class="item.success?'ok':'fail'">{{ item.success ? '通过' : '失败' }}</span></div><div class="variable-box"><b>本次提取变量（{{ chainResult.variableCount }}）</b><span v-for="(value, key) in chainResult.variables" :key="key"><code>{{ key }}</code> = {{ value }}</span></div></div><div v-else class="response-empty"><div class="empty-icon">↗</div><b>等待执行链路</b><span>执行后会看到每个步骤的状态和提取变量。</span></div></div></div></section>
    <section v-else-if="page==='knowledge'" class="list-page knowledge-page"><div class="page-title"><div><span class="eyebrow">PROJECT KNOWLEDGE</span><h1>项目知识库</h1><p>按当前项目保存 OpenAPI、PRD、业务规则和错误码；Agent 只检索当前项目。</p></div><button class="primary-btn" @click="generateBusinessCases">✦ 根据规则生成测试用例</button></div><div class="knowledge-grid"><div class="settings-card"><div class="card-title"><div><span class="eyebrow">DOCUMENTS</span><h2>上传项目文档</h2></div><span class="provider-dot">{{ documents.length }} 份</span></div><p class="settings-tip">业务规则文档就是“订单金额、退款、库存、鉴权”等项目约束。上传后用于生成测试用例和评测模型是否遵守规则。</p><div class="form-two"><label>文档类型<select v-model="documentType"><option>业务规则</option><option>PRD</option><option>错误码</option><option>接口契约</option><option>认证说明</option></select></label><label>版本<input v-model="documentVersion" placeholder="v1" /></label></div><label class="document-upload-box">选择文件<input type="file" accept=".txt,.md,.json,.yaml,.yml,.docx,.pdf" @change="uploadProjectDocument" /><small>支持 TXT / Markdown / JSON / YAML / DOCX / 文本型 PDF</small></label><div class="document-list"><div v-for="item in documents" :key="item.id" class="document-item"><input type="checkbox" :checked="selectedDocumentIds.includes(item.id)" @change="toggleDocument(item.id)" /><div><b>{{ item.name }}</b><small>{{ item.docType }} · {{ item.version }} · {{ item.chunkCount }} 个检索片段</small></div><button class="link-btn" @click="deleteProjectDocument(item)">删除</button></div><div v-if="!documents.length" class="empty-small">当前项目还没有知识文档</div></div></div><div class="settings-card"><div class="card-title"><div><span class="eyebrow">RULE → TEST CASES</span><h2>业务规则生成用例</h2></div><span class="provider-dot">轻量 RAG</span></div><p class="settings-tip">选中左侧文档后，系统检索相关规则，生成正常、边界、异常、鉴权和库存场景；生成结果仍需你确认再保存。</p><div v-if="businessCases.length" class="business-case-list"><div v-for="item in businessCases" :key="item.name" class="business-case-item"><div><b>{{ item.name }}</b><small>{{ item.scenario }}</small></div><span class="table-status ok">{{ item.assertions[0]?.expected || '待确认' }}</span><details><summary>查看依据</summary><p>{{ item.rule }}</p><small v-for="evidence in item.evidence" :key="evidence">{{ evidence }}</small></details></div></div><div v-else class="response-empty compact-empty"><div class="empty-icon">✦</div><b>还没有生成测试用例</b><span>上传业务规则后点击右上角按钮</span></div></div></div></section><section v-else-if="page==='evaluation'" class="list-page evaluation-page"><div class="page-title"><div><span class="eyebrow">MODEL EVALUATION</span><h1>大模型业务知识评测</h1><p>同一批业务问题并发调用多个模型，比较回答准确性、延迟和文档依据。</p></div><button class="primary-btn" :disabled="evaluationLoading" @click="runEvaluation">{{ evaluationLoading ? '评测中…' : '▶ 开始并发评测' }}</button></div><div class="evaluation-grid"><div class="settings-card"><div class="card-title"><div><span class="eyebrow">MODEL POOL</span><h2>模型池</h2></div><span class="provider-dot">{{ modelProfiles.length }} 个模型</span></div><p class="settings-tip">可以同时选择 DeepSeek、Azure OpenAI、OpenAI 等配置；平台按“问题 × 模型”并发执行，不需要逐个点击。</p><div class="model-profile-list"><label v-for="item in modelProfiles" :key="item.id" class="model-profile-row"><input type="checkbox" :checked="selectedProfileIds.includes(item.id)" @change="toggleProfile(item.id)" /><span><b>{{ item.name }}</b><small>{{ item.provider }} · {{ item.model }} · {{ item.apiKeyMasked || '已配置密钥' }}</small></span></label><div v-if="!modelProfiles.length" class="empty-small">还没有模型配置，请在下方添加</div></div><div class="profile-form"><input v-model="modelProfileForm.name" placeholder="名称，例如 DeepSeek 业务评测" /><input v-model="modelProfileForm.provider" placeholder="提供商" /><input v-model="modelProfileForm.baseUrl" placeholder="https://api.deepseek.com/chat/completions" /><input v-model="modelProfileForm.model" placeholder="deepseek-chat" /><input v-model="modelProfileForm.apiKey" type="password" placeholder="API Key" /><input v-model="modelProfileForm.authHeader" placeholder="Header：Authorization / api-key" /><button class="primary-btn" @click="saveModelProfile">保存模型</button></div></div><div class="settings-card"><div class="card-title"><div><span class="eyebrow">QUESTION SET</span><h2>统一评测问题</h2></div><span class="provider-dot">{{ evaluationQuestionCount }} 题</span></div><textarea class="evaluation-questions" v-model="evaluationQuestions" rows="8" placeholder="每行一个问题"></textarea><p class="settings-tip">建议演示：上传订单业务规则文档，然后测试“最低金额、退款条件、取消订单、未登录返回什么”。</p><div v-if="evaluationResults.length" class="evaluation-summary"><b>本次结果 {{ evaluationResults.length }} 条</b><span>同一问题已分别发送给所选模型</span></div></div></div><div v-if="evaluationResults.length" class="table-card evaluation-results"><table><thead><tr><th>模型</th><th>问题</th><th>回答</th><th>规则覆盖</th><th>文档依据</th><th>耗时</th><th>风险</th></tr></thead><tbody><tr v-for="item in evaluationResults" :key="item.profileId + '-' + item.question"><td><b>{{ item.profileName }}</b><small>{{ item.provider }}</small></td><td>{{ item.question }}</td><td class="answer-cell">{{ item.error || item.answer }}</td><td><span :class="item.ruleCoverage===item.ruleTotal?'table-status ok':'table-status fail'">{{ item.ruleCoverage }}/{{ item.ruleTotal }}</span></td><td>{{ item.citationFound ? '已引用' : '未引用' }}</td><td>{{ item.durationMs }}ms</td><td>{{ item.hallucinationRisk }}</td></tr></tbody></table></div></section>
    <section v-else class="list-page"><div class="page-title"><div><span class="eyebrow">MODEL & ENVIRONMENT</span><h1>模型与环境配置</h1><p>配置 Agent 使用的模型、密钥和请求变量，敏感值默认不回显。</p></div><span class="config-state" :class="llmConfigured?'configured':''">{{ llmConfigured ? '● 已配置真实模型' : '● 规则 Agent 模式' }}</span></div><div class="settings-grid"><div class="settings-card"><div class="card-title"><div><span class="eyebrow">LLM PROVIDER</span><h2>模型连接</h2></div><span class="provider-dot">Azure / OpenAI</span></div><label>Chat Completions 地址<input v-model="llmForm.baseUrl" placeholder="https://.../chat/completions" /></label><label>API Key<input v-model="llmForm.apiKey" type="password" placeholder="输入后保存，服务端只返回脱敏结果" /></label><div class="form-two"><label>模型 / 部署名<input v-model="llmForm.model" placeholder="gpt-4o-mini" /></label><label>鉴权 Header<input v-model="llmForm.authHeader" placeholder="Authorization 或 api-key" /></label></div><button class="primary-btn" @click="saveLLMSettings">保存模型配置</button><p class="settings-tip">配置后点击“AI 补齐参数”，后端会优先调用 Tool Calling；未配置时自动使用规则 Agent。</p></div><div class="settings-card"><div class="card-title"><div><span class="eyebrow">ENVIRONMENT</span><h2>环境变量</h2></div><span class="provider-dot">本地浏览器保存</span></div><label>变量（每行 key: value）<textarea v-model="environmentText" rows="9" placeholder="baseUrl: https://test.example.com\ntoken: your-token"></textarea></label><button class="primary-btn" @click="saveEnvironment">保存环境变量</button><p class="settings-tip">请求 URL、Headers、Body 中可以使用 {{baseUrl}}、{{token}}，发送前自动替换。</p></div></div></section>
    <button class="assistant-fab" :class="{dragging:assistantFabDragging}" :style="assistantFabStyle" title="拖动 AI 助手；点击打开" @pointerdown="startAssistantFabDrag" @click="clickAssistantFab"><span class="fab-ear left"></span><span class="fab-ear right"></span><span class="fab-face"><i></i><i></i><b>⌣</b></span><em>AI 助手</em></button>
    <div v-if="notice" class="toast-v2">{{ notice }}</div>
    <div v-if="showEndpointModal" class="modal-mask" @click.self="showEndpointModal=false"><div class="modal-card"><div class="modal-head"><div><span class="eyebrow">NEW ENDPOINT</span><h2>添加接口</h2></div><button class="close-btn" @click="showEndpointModal=false">×</button></div><label>接口名称<input v-model="endpointForm.name" placeholder="例如：用户登录" /></label><div class="form-two"><label>方法<select v-model="endpointForm.method"><option>GET</option><option>POST</option><option>PUT</option><option>DELETE</option></select></label><label>类型<select v-model="endpointForm.kind"><option>REST</option><option>LLM</option><option>Multimodal</option></select></label></div><label>URL<input v-model="endpointForm.url" placeholder="https://example.com/api/users 或 /api/demo/echo" /></label><label>Headers<textarea v-model="endpointForm.headers" rows="3" placeholder="Authorization: Bearer xxx"></textarea></label><label>Body<textarea v-model="endpointForm.body" rows="4" placeholder="{&quot;name&quot;:&quot;demo&quot;}"></textarea></label><div class="modal-actions"><label class="import-link">或导入 OpenAPI <input type="file" accept=".json,application/json" @change="selectOpenAPI" /></label><button class="primary-btn" @click="createEndpoint">保存接口</button></div></div></div>
    <div v-if="showProjectModal" class="modal-mask" @click.self="showProjectModal=false"><div class="modal-card small-modal"><div class="modal-head"><div><span class="eyebrow">NEW PROJECT</span><h2>新建项目</h2></div><button class="close-btn" @click="showProjectModal=false">×</button></div><label>项目名称<input v-model="projectForm.name" placeholder="例如：订单服务测试" /></label><label>项目说明<input v-model="projectForm.summary" placeholder="可选" /></label><button class="primary-btn full-btn" @click="createProject">创建项目</button></div></div>
    <div v-if="showAssistant" class="assistant-mask" @click.self="closeAssistant"><section class="assistant-window"><header class="assistant-window-head"><div><span class="eyebrow">AI ASSISTANT</span><h2>API 测试助手</h2><p>{{ selectedEndpoint?.name || '当前项目' }}</p></div><button class="close-btn" @click="closeAssistant">×</button></header><div class="assistant-context"><span>工作范围</span><b>{{ selectedEndpoint ? '当前接口：' + selectedEndpoint.method + ' ' + selectedEndpoint.url : '当前项目：可测试或新增接口' }}</b></div><div class="assistant-messages"><div v-for="(message, index) in assistantMessages" :key="index" class="assistant-message" :class="message.role"><span>{{ message.role === 'assistant' ? 'AI' : '你' }}</span><p>{{ message.text }}</p></div><div v-if="assistantPlan" class="assistant-plan-card"><div class="draft-card-title"><b>Agent 执行计划</b><span>{{ assistantPlan.mode }}</span></div><p class="assistant-plan-summary">{{ assistantPlan.summary }}</p><div v-for="step in assistantPlan.steps" :key="step.order" class="assistant-plan-step"><i>{{ step.order }}</i><div><b>{{ step.title }}</b><small>{{ step.description }}</small></div><em v-if="step.requiresConfirmation">需确认</em></div></div><div v-if="assistantEndpointDraft" class="assistant-draft-card endpoint-draft-card"><div class="draft-card-title"><b>已生成接口配置草稿</b><span>确认后才会加入项目</span></div><div class="endpoint-draft-grid"><span>名称</span><b>{{ assistantEndpointDraft.name }}</b><span>方法</span><code>{{ assistantEndpointDraft.method }}</code><span>URL</span><b>{{ assistantEndpointDraft.url }}</b><span>类型</span><b>{{ assistantEndpointDraft.kind }}</b></div><div class="draft-block"><small>Headers</small><pre>{{ formatPairs(assistantEndpointDraft.headers || {}) || '无' }}</pre></div><div class="draft-block"><small>Body</small><pre>{{ assistantEndpointDraft.body || '无请求体' }}</pre></div></div><div v-else-if="aiSummary" class="assistant-draft-card"><div class="draft-card-title"><b>已生成请求草稿</b><span>可回到测试台继续编辑</span></div><div class="draft-preview-line"><code>{{ request.method }}</code><span>{{ request.url }}</span></div><pre>{{ request.body || '无请求体' }}</pre><div class="draft-meta"><span>断言 {{ assertions.length }} 条</span><span v-if="aiReasoning.length">已生成依据</span></div></div><div v-if="businessCases.length" class="assistant-draft-card assistant-business-cases"><div class="draft-card-title"><b>业务规则测试用例</b><span>{{ businessCases.length }} 条</span></div><div v-for="item in businessCases" :key="item.name" class="assistant-case-row"><b>{{ item.name }}</b><span>{{ item.scenario }} · 预期 {{ item.assertions[0]?.expected || "待确认" }}</span></div></div></div><div class="assistant-input-row"><div class="assistant-composer"><textarea v-model="assistantInput" rows="3" placeholder="例如：添加一个用户登录接口，POST /api/login" @keydown.enter.exact.prevent="askAssistant"></textarea><div class="assistant-composer-toolbar"><span class="assistant-tool-caption">常用操作</span><button class="composer-chip" @click="useAssistantSuggestion('测试这个接口是否可用，并检查返回 JSON')">检查接口是否可用</button><button class="composer-chip" @click="useAssistantSuggestion('添加一个用户登录接口，POST /api/login，请求体包含 email 和 password')">添加用户登录接口</button><button class="composer-chip" @click="useAssistantSuggestion('添加一个 Azure OpenAI 模型接口，POST /openai/deployments/demo/chat/completions')">添加模型接口</button><label class="assistant-upload-btn composer-upload">▣ 导入文档<input type="file" accept=".txt,.md,.json,.yaml,.yml,.docx,.pdf" @change="selectAssistantDocument" /></label><span v-if="assistantDocumentName" class="composer-document">{{ assistantDocumentName }} · {{ assistantDocumentStatus }}</span><button v-if="assistantDocumentName" class="document-clear-btn composer-clear" @click="clearAssistantDocument">清除</button></div></div><button class="ai-btn" :disabled="loading" @click="askAssistant">{{ loading ? '分析中…' : '发送给 AI' }}</button></div><footer class="assistant-window-foot"><span>{{ assistantEndpointDraft ? '接口会先以草稿展示，点击确认后才写入 PostgreSQL' : 'AI 只生成草稿，不会未经确认发送请求' }}</span><div><button class="send-only" @click="closeAssistant">继续编辑</button><button v-if="aiSummary" class="primary-btn" @click="applyAssistantDraft">{{ assistantEndpointDraft ? '添加到接口列表' : '应用到测试台' }}</button></div></footer></section></div>
  </div>`,
}

const app = createApp(App)
app.config.errorHandler = (error) => { console.error('[Pulse API V2]', error); const root = document.getElementById('app'); if (root) root.innerHTML = `<div style="font-family:Microsoft YaHei,Arial,sans-serif;padding:40px;color:#7b3f3f"><h2>页面加载失败</h2><p>${String(error)}</p></div>` }
app.mount('#app')
