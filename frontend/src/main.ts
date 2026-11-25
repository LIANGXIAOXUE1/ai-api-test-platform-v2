import { computed, createApp, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import './style.css'
import UserManagementPage from './features/auth/UserManagementPage.vue'
import ReadOnlyRunDialog from './features/auth/ReadOnlyRunDialog.vue'
import { usePermissions } from './features/auth/usePermissions'
import AuthGate from './features/auth/AuthGate.vue'
import SessionControl from './features/auth/SessionControl.vue'
import { authRequiredEvent } from './features/auth/auth'
import ProjectTestAssistant from './features/project-test/ProjectTestAssistant.vue'
import ProjectTaskPage from './features/project-test/ProjectTaskPage.vue'
import BusinessEvaluationPage from './features/evaluation/BusinessEvaluationPage.vue'
import TestingWorkspaceNav from './features/shared/TestingWorkspaceNav.vue'
import ManualChainWorkspace from './features/project-test/ManualChainWorkspace.vue'
import TestRecordsPage from './features/history/TestRecordsPage.vue'
import AssistantComposer from './features/shared/AssistantComposer.vue'
import './features/shared/assistant-chat.css'
import KnowledgePage from './features/knowledge/KnowledgePage.vue'
import { verifiedModels, eligibleSelection } from './features/shared/models'
import { useProjectTest } from './features/project-test/useProjectTest'
import { isProjectConnectivityTask } from './features/project-test/intent'
import type { ProjectCheckItem } from './features/project-test/types'

type Project = { id: number; name: string; summary: string }
type Endpoint = { id: number; projectId: number; name: string; method: string; url: string; kind: string; headers: Record<string, string>; query: Record<string, string>; body: string; enabled: boolean }
type Assertion = { type: string; path: string; expected: string }
type RequestSpec = { method: string; url: string; headers: Record<string, string>; query: Record<string, string>; body: string }
type AgentCandidate = { endpointId: number; name: string; method: string; url: string; score: number; reason: string }
type AgentMissingInfo = { field: string; reason: string; question: string }
type AgentRisk = { level: 'low' | 'medium' | 'high' | 'critical'; reasons: string[]; scope: string }
type Draft = { summary: string; request: RequestSpec; assertions: Assertion[]; reasoning: string[]; mode?: string; intent?: string; endpoint?: Endpoint; requiresConfirmation?: boolean; confidence?: number; candidateEndpoints?: AgentCandidate[]; missingInformation?: AgentMissingInfo[]; risk?: AgentRisk; executionDecision?: 'ask_user' | 'await_review' | 'ready_to_execute' | 'executed' | 'cannot_execute'; plan?: AgentPlanStep[] }
type TestCase = { id: number; projectId: number; apiId: number; name: string; prompt: string; request: RequestSpec; assertions: Assertion[]; createdAt: string }
type Run = { id: number; caseId: number; apiId: number; caseName: string; createdBy?: string; startedAt: string; durationMs: number; firstByteMs: number; statusCode: number; success: boolean; errorCategory: string; errorMessage: string; responseHeaders: Record<string, string>; responseBody: string; contentType: string; stream: boolean; inputTokens: number; outputTokens: number; tokensPerSec: number; assertionPass: number; assertionTotal: number }
type LLMSettings = { baseUrl: string; apiKey: string; model: string; authHeader: string; reasoningMode?: 'auto' | 'off' | 'enabled'; reasoningEffort?: 'low' | 'medium' | 'high'; configured?: boolean; apiKeyMasked?: string; verified?: boolean; verifiedAt?: string; verificationError?: string; sourceProfileId?: number; sourceProfileName?: string; sourceProvider?: string }
type ChainStep = { apiId: number; name: string; headers: string; body: string; extract: string }
type ChainResult = { success: boolean; failedAt: number; steps: Run[]; variables: Record<string, string>; variableCount: number }
type AssistantMessage = { role: 'user' | 'assistant'; text: string }
type AgentPlanStep = { order: number; tool: string; title: string; description: string; requiresConfirmation: boolean }
type AgentPlan = { summary: string; intent: string; steps: AgentPlanStep[]; requiresConfirmation: boolean; mode: string }
type ProjectDocument = { id: number; projectId: number; name: string; docType: string; version: string; characters: number; chunkCount: number; createdAt: string }
type ModelProfile = { id: number; projectId: number; name: string; provider: string; baseUrl: string; apiKey: string; apiKeyMasked?: string; model: string; authHeader: string; reasoningMode?: 'auto' | 'off' | 'enabled'; reasoningEffort?: 'low' | 'medium' | 'high'; verified?: boolean; verificationError?: string; enabled: boolean }
type BusinessCase = { name: string; scenario: string; rule: string; request: RequestSpec; assertions: Assertion[]; evidence: string[] }
type EvaluationResult = { profileId: number; profileName: string; provider: string; question: string; answer: string; success: boolean; durationMs: number; inputTokens: number; outputTokens: number; ruleCoverage: number; ruleTotal: number; citationFound: boolean; hallucinationRisk: string; matchedRules: string[]; evidence: string[]; error?: string }
type EvaluationReport = { id: number; projectId: number; createdAt: string; questions: string[]; results: EvaluationResult[] }
type AssistantPermission = 'default' | 'project_full'

const emptyRequest = (): RequestSpec => ({ method: 'POST', url: '/api/demo/echo', headers: { 'Content-Type': 'application/json' }, query: {}, body: '{\n  "message": "hello"\n}' })
const emptyModelProfileForm = () => ({ name: '', provider: 'OpenAI 兼容', baseUrl: '', apiKey: '', model: '', authHeader: 'Authorization', reasoningMode: 'auto' as const, reasoningEffort: 'medium' as const })
const api = async (path: string, options: RequestInit = {}) => {
  const headers = new Headers(options.headers)
  if (!(options.body instanceof FormData) && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  const response = await fetch(path, { credentials: 'same-origin', ...options, headers })
  const data = await response.json().catch(() => ({}))
  if (response.status === 401) window.dispatchEvent(new Event(authRequiredEvent))
  if (!response.ok) throw Object.assign(new Error(data.error || '请求失败'), { details:data, status:response.status })
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
  components: { ProjectTestAssistant, ProjectTaskPage, BusinessEvaluationPage, TestingWorkspaceNav, ManualChainWorkspace, TestRecordsPage, AssistantComposer, KnowledgePage, SessionControl, UserManagementPage, ReadOnlyRunDialog },
  setup() {
    const { auth: platformAuth, isAdmin, readOnly, canExecute, canAuto } = usePermissions()
    const readonlyRun = ref<Run | null>(null)
    function viewRecord(run: Run) { if (readOnly.value) readonlyRun.value = run; else { latest.value = run; page.value = 'tester' } }
    const userStorage = (key: string) => 'pulse-api-user-' + (platformAuth?.user.value?.id || 'local') + '-' + key

    const projectTaskGoal = ref('')
    function openProjectTask() { page.value = 'projectCheck'; showAssistant.value = false; closeSidebar() }
    const page = ref<'tester' | 'history' | 'knowledge' | 'evaluation' | 'projectCheck' | 'settings' | 'users'>(readOnly.value ? 'history' : 'tester')
    const recordsTab = ref<'history' | 'regression'>('history')
    const lastTestingPage = ref<'projectCheck' | 'evaluation'>('projectCheck')
    function openTestingWorkspace() { page.value = lastTestingPage.value; showAssistant.value = false; closeSidebar() }
    const sidebarCollapsed = ref(false)
    const projects = ref<Project[]>([])
    const endpoints = ref<Endpoint[]>([])
    const runs = ref<Run[]>([])
    const cases = ref<TestCase[]>([])
    const selectedProjectId = ref((() => { try { return Number(localStorage.getItem(userStorage('selected-project-id'))) || 1 } catch { return 1 } })())
    const showProjectPicker = ref(false)
    const projectEndpointCounts = ref<Record<number, number>>({})
    const projectRunCounts = ref<Record<number, number>>({})
    const selectedApiId = ref(1)
    const searchText = ref('')
    const prompt = ref('测试这个接口是否可用，并检查返回结果')
    const request = ref<RequestSpec>(emptyRequest())
    const assertions = ref<Assertion[]>([{ type: 'status', path: '', expected: '200' }])
    const azureImageDeployment = ref('')
    const azureImageApiVersion = ref('2025-04-01-preview')
    const normalizeAssertion = (item: Assertion) => {
      if (item.type === 'json_field' || item.type === 'json_contract') {
        item.expected = ''
      } else if (item.type === 'status' && !item.expected.trim()) {
        item.expected = '200'
      }
    }
    watch(assertions, (items) => items.forEach(normalizeAssertion), { deep: true, immediate: true })
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
    const llmForm = ref<LLMSettings>({ baseUrl: '', apiKey: '', model: '', authHeader: 'Authorization', reasoningMode: 'auto', reasoningEffort: 'medium' })
    const llmConfigured = ref(false)
    const llmVerified = ref(false)
    const llmVerificationError = ref('')
    const llmVerifiedAt = ref('')
    const llmSourceProfileId = ref(0)
    const llmSourceProfileName = ref('')
    const llmSourceProvider = ref('')
    const environmentText = ref('')
    const showEndpointModal = ref(false)
    const showModelModal = ref(false)
    const showModelDetailModal = ref(false)
    const showProjectModal = ref(false)
    const showEvaluationResultModal = ref(false)
    const openApiFile = ref<File | null>(null)
    const projectForm = ref({ name: '', summary: '' })
    const projectWorkspaces = ref<Record<number, string>>({})
    const pendingProjectFolder = ref<any>(null)
    const pendingProjectFolderName = ref('')
    const endpointForm = ref({ name: '', method: 'GET', url: '', headers: '', body: '', kind: 'REST' })
    const editingEndpointId = ref(0)
    const pendingEndpointDeletion = ref<Endpoint | null>(null)
	const pendingModelDeletion = ref<ModelProfile | null>(null)
    const chainSteps = ref<ChainStep[]>([])
    const chainVariablesText = ref('')
    const chainResult = ref<ChainResult | null>(null)
    const showAssistant = ref(false)
    const assistantInput = ref(prompt.value)
    const assistantMessages = ref<AssistantMessage[]>([])
    const assistantEndpointDraft = ref<Endpoint | null>(null)
    const assistantDeleteDraft = ref<Endpoint | null>(null)
    const assistantDocumentName = ref('')
    const assistantDocumentText = ref('')
    const assistantDocumentStatus = ref('')
    const assistantPlan = ref<AgentPlan | null>(null)
    const assistantDecisionDraft = ref<Draft | null>(null)
    const assistantPermission = ref<AssistantPermission>((() => { try { return canAuto.value && localStorage.getItem(userStorage('assistant-permission')) === 'project_full' ? 'project_full' : 'default' } catch { return 'default' } })())
    const assistantOpenAPISpec = ref<Record<string, any> | null>(null)
    const {
      assistantProjectCheck, assistantProjectCheckDraft, projectCheckReports, assistantReviewOpen,
      assistantPendingReviewItems, loadProjectCheckReports, restoreProjectCheckReport,
      prepareAssistantProjectCheck, confirmAssistantProjectCheck, downloadProjectCheckReport,
      projectCheckBaseURL, projectCheckTask, projectCheckPermission, projectCheckFile,
      projectCheckLoading, projectCheckResult, selectProjectCheckFile, runProjectCheck
    } = useProjectTest({
      request: api, selectedProjectId, assistantOpenAPISpec, assistantPermission, loading,
      getEnvironment: () => parsePairs(environmentText.value),
      getProjectName: () => currentProject.value?.name || 'project',
      loadEndpoints, refresh, flash, streamAssistantReply,
    })
    const assistantManualReview = ref(false)
    const documents = ref<ProjectDocument[]>([])
    const selectedDocumentIds = ref<number[]>([])
    const documentType = ref('业务规则')
    const documentVersion = ref('v1')
    const businessCases = ref<BusinessCase[]>([])
    const modelProfiles = ref<ModelProfile[]>([])
    const assistantProfileId = ref(0)
    const assistantScopeApiId = ref(0)
    const availableProfiles = computed(() => verifiedModels(modelProfiles.value))
    watch(modelProfiles, () => {
      selectedProfileIds.value = eligibleSelection(selectedProfileIds.value, modelProfiles.value)
      if (assistantProfileId.value && !availableProfiles.value.some(item => item.id === assistantProfileId.value)) assistantProfileId.value = 0
    }, {deep:true})
    watch(assistantScopeApiId, id => { const item = endpoints.value.find(item => item.id === id); if (item) selectEndpoint(item) })
    watch(selectedProjectId, () => { assistantScopeApiId.value = 0 })
    const editingModel = ref<ModelProfile | null>(null)
    const editingModelApiKey = ref('')
    const selectedProfileIds = ref<number[]>([])
    const evaluationQuestions = ref('订单金额最低是多少？\n什么情况下可以退款？\n已退款订单能否再次退款？\n未登录下单应该返回什么？')
    const evaluationResults = ref<EvaluationResult[]>([])
    const evaluationReports = ref<EvaluationReport[]>([])
    const activeEvaluationReport = ref<EvaluationReport | null>(null)
    const evaluationLoading = ref(false)
    const modelForm = ref(emptyModelProfileForm())
    const discoveredModels = ref<string[]>([])
    const selectedDiscoveredModels = ref<string[]>([])
    const discoveringModels = ref(false)
    const importingDiscoveredModels = ref(false)
    const assistantDocumentId = ref(0)
    const assistantFabPosition = ref<{ x: number; y: number } | null>(null)
    const assistantFabDragging = ref(false)
    let assistantFabDragState: { offsetX: number; offsetY: number; startX: number; startY: number; moved: boolean } | null = null
    let suppressAssistantFabClick = false
    let modelPoolObserver: MutationObserver | null = null

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
    function closeSidebar() {
      if (sidebarCollapsed.value) return
      sidebarCollapsed.value = true
      try { localStorage.setItem('pulse-api-sidebar-collapsed', '1') } catch { /* storage is optional */ }
    }
    function onKeydown(event: KeyboardEvent) {
      if (event.key !== 'Escape') return
      // Esc 优先级：项目面板 > 新建项目弹窗 > AI 助手 > 侧栏
      // 从最内层浮层开始关，避免一次Esc 把多层全关掉
      if (showProjectPicker.value) { showProjectPicker.value = false; return }
      if (showProjectModal.value) { showProjectModal.value = false; return }
      if (showEvaluationResultModal.value) { showEvaluationResultModal.value = false; return }
      if (showAssistant.value) { closeAssistant(); return }
      closeSidebar()
    }
    onMounted(() => window.addEventListener('keydown', onKeydown))
    onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))

    /* ------------------------------------------------------------------
       项目下拉面板：点击空白区域关闭
       ------------------------------------------------------------------ */
    // 监听容器，捕获阶段执行 —— 这样即使触发按钮自身处理了 click，
    // 也能正确判断「这次点击是否落在面板之外」。
    function onDocumentPointerDown(event: PointerEvent) {
      const modelPoolTarget = event.target as HTMLElement | null
      if (page.value === 'evaluation' && !modelProfiles.value.length && modelPoolTarget?.closest('.model-pool-empty')) { openModelModal(); return }
      if (!showProjectPicker.value) return
      const target = event.target as HTMLElement | null
      if (target?.closest('.project-picker')) return  // 点在选择器内部（含面板），不关闭
      showProjectPicker.value = false
    }
    function onModelPoolEmptyClick(event: MouseEvent) {
      if (page.value !== 'evaluation' || modelProfiles.value.length) return
      const target = event.target as HTMLElement | null
      if (target?.closest('.model-pool-empty')) openModelModal()
    }
    onMounted(() => document.addEventListener('pointerdown', onDocumentPointerDown, true))
    onBeforeUnmount(() => document.removeEventListener('pointerdown', onDocumentPointerDown, true))
    onMounted(() => document.addEventListener('click', onModelPoolEmptyClick))
    onBeforeUnmount(() => document.removeEventListener('click', onModelPoolEmptyClick))
    onMounted(() => {
      bindModelPoolEmpty()
      modelPoolObserver = new MutationObserver(() => bindModelPoolEmpty())
      modelPoolObserver.observe(document.body, { childList: true, subtree: true })
    })
    onBeforeUnmount(() => modelPoolObserver?.disconnect())

    function syncListPageScrollMode() {
      const isListPage = page.value !== 'tester'
      document.body.classList.toggle('pulse-list-page-active', isListPage)
      const app = document.querySelector<HTMLElement>('.app-v2')
      const list = document.querySelector<HTMLElement>('.list-page:not(.chain-page)')
      if (!isListPage || !app || !list) {
        document.body.style.removeProperty('overflow')
        document.body.style.removeProperty('height')
        app?.style.removeProperty('overflow')
        app?.style.removeProperty('height')
        list?.style.removeProperty('overflow-y')
        list?.style.removeProperty('height')
        return
      }
      document.body.style.setProperty('height', '100dvh', 'important')
      document.body.style.setProperty('overflow', 'hidden', 'important')
      app.style.setProperty('height', '100dvh', 'important')
      app.style.setProperty('overflow', 'hidden', 'important')
      list.style.setProperty('height', '100dvh', 'important')
      list.style.setProperty('overflow-y', 'auto', 'important')
      list.style.setProperty('overflow-x', 'hidden', 'important')
    }
    function forwardListPageWheel(event: WheelEvent) {
      if (page.value === 'tester' || event.deltaY === 0) return
      const target = event.target as HTMLElement | null
      if (target?.closest('input, textarea, select, button, [contenteditable="true"]')) return
      const container = document.querySelector<HTMLElement>('.list-page:not(.chain-page)')
      if (!container || container.scrollHeight <= container.clientHeight) return
      const before = container.scrollTop
      container.scrollTop += event.deltaY
      if (container.scrollTop !== before) event.preventDefault()
    }
    function forwardAssistantWheel(event: WheelEvent) {
      if (event.deltaY === 0) return
      const target = event.target as HTMLElement | null
      // 输入和下拉控件保留浏览器自身的滚动行为；其余区域始终驱动对话记录。
      if (target?.closest('input, textarea, select, [contenteditable="true"]')) return
      const messages = document.querySelector<HTMLElement>('.assistant-messages')
      if (!messages || messages.scrollHeight <= messages.clientHeight) return
      const unit = event.deltaMode === WheelEvent.DOM_DELTA_LINE ? 24 : event.deltaMode === WheelEvent.DOM_DELTA_PAGE ? messages.clientHeight : 1
      const before = messages.scrollTop
      messages.scrollTop += event.deltaY * unit
      if (messages.scrollTop !== before) event.preventDefault()
    }
    onMounted(() => { syncListPageScrollMode(); window.addEventListener('wheel', forwardListPageWheel, { passive: false }) })
    onBeforeUnmount(() => { document.body.classList.remove('pulse-list-page-active'); window.removeEventListener('wheel', forwardListPageWheel) })
    watch(page, () => {
      if (page.value === 'projectCheck' || page.value === 'evaluation') lastTestingPage.value = page.value
      // 切页时统一收拢所有浮层，避免 AI 弹窗遮住新页面（表现为"点侧栏无响应"）
      showAssistant.value = false
      showProjectPicker.value = false
      if (!sidebarCollapsed.value) toggleSidebar()
      window.setTimeout(syncListPageScrollMode, 0)
      window.setTimeout(bindModelPoolEmpty, 0)
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
    const isLoginEndpoint = computed(() => /(^|[/_.-])login([/_.-]|$)|登录/i.test(`${selectedEndpoint.value?.name || ''} ${request.value.url}`))
    const isRegistrationEndpoint = computed(() => /(^|[/_.-])register([/_.-]|$)|注册/i.test(`${selectedEndpoint.value?.name || ''} ${request.value.url}`))
    const filteredEndpoints = computed(() => { const query = searchText.value.trim().toLowerCase(); return query ? endpoints.value.filter((item) => `${item.name} ${item.url}`.toLowerCase().includes(query)) : endpoints.value })
    const successRate = computed(() => runs.value.length ? Math.round(runs.value.filter((item) => item.success).length / runs.value.length * 100) : 0)
    const savedRuns = computed(() => runs.value.filter((item) => item.caseId))
    const evaluationQuestionCount = computed(() => evaluationQuestions.value.split(/\r?\n/).filter(Boolean).length)
    const responsePretty = computed(() => { const body = latest.value?.responseBody || ''; try { return JSON.stringify(JSON.parse(body), null, 2) } catch { return body } })
    function base64ImageSource(value: string) {
      const trimmed = value.trim()
      if (/^data:image\//i.test(trimmed)) return trimmed
      const compact = trimmed.replace(/\s/g, '')
      return compact.length > 128 && /^[a-z0-9+/]+={0,2}$/i.test(compact) ? `data:image/png;base64,${compact}` : ''
    }
    function findResponseImage(value: unknown, depth = 0): { src: string; source: 'url' | 'base64' } | null {
      if (!value || depth > 10) return null
      if (Array.isArray(value)) {
        for (const item of value) { const image = findResponseImage(item, depth + 1); if (image) return image }
        return null
      }
      if (typeof value !== 'object') return null
      const record = value as Record<string, unknown>
      for (const key of ['url', 'image_url', 'imageUrl']) {
        const candidate = record[key]
        if (typeof candidate === 'string' && /^https?:\/\//i.test(candidate.trim())) return { src: candidate.trim(), source: 'url' }
      }
      for (const key of ['b64_json', 'b64Json', 'base64', 'image_base64', 'imageBase64', 'result']) {
        const candidate = record[key]
        if (typeof candidate === 'string') { const src = base64ImageSource(candidate); if (src) return { src, source: 'base64' } }
      }
      for (const child of Object.values(record)) { const image = findResponseImage(child, depth + 1); if (image) return image }
      return null
    }
    const responseImagePreview = computed(() => {
      const body = latest.value?.responseBody || ''
      try { return findResponseImage(JSON.parse(body)) } catch { return null }
    })
    const responseHeadersText = computed(() => Object.entries(latest.value?.responseHeaders || {}).map(([key, value]) => `${key}: ${value}`).join('\n'))
    const headerEditorText = computed(() => Object.entries(request.value.headers || {}).map(([key, value]) => `${key}: ${showSecrets.value ? value : maskHeader(key, value)}`).join('\n'))

    function flash(message: string) { notice.value = message; window.setTimeout(() => { if (notice.value === message) notice.value = '' }, 3200) }
    async function auditAgent(action: string, result: Record<string, unknown> = {}, confirmed = false) {
      try { await api('/api/agent/audit', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, apiId: selectedApiId.value, action, prompt: prompt.value, result, confirmed }) }) } catch { /* 审计失败不影响主流程 */ }
    }
    async function loadProjectKnowledge() {
      try {
        const [projectDocuments, profiles] = await Promise.all([api(`/api/documents?projectId=${selectedProjectId.value}`), api(`/api/models?projectId=${selectedProjectId.value}`)])
        documents.value = Array.isArray(projectDocuments) ? projectDocuments : []
        modelProfiles.value = Array.isArray(profiles) ? profiles : []
      } catch { documents.value = []; modelProfiles.value = [] }
    }
    async function loadEvaluationReports() {
      try {
        const result = await api(`/api/evaluations?projectId=${selectedProjectId.value}`)
        evaluationReports.value = Array.isArray(result?.reports) ? result.reports : []
      } catch { evaluationReports.value = [] }
    }
    function openEvaluationReport(report: EvaluationReport) {
      activeEvaluationReport.value = report
      evaluationResults.value = report.results || []
      showEvaluationResultModal.value = true
    }
    async function openEvaluationHistory() {
      await loadEvaluationReports()
      activeEvaluationReport.value = evaluationReports.value[0] || null
      showEvaluationResultModal.value = true
    }
    function formatEvaluationTime(value: string) {
      if (!value) return '刚刚'
      const date = new Date(value)
      return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false })
    }
    function evaluationResultRiskClass(item: EvaluationResult) {
      if (!item.success) return 'high'
      if (item.hallucinationRisk === '低') return 'low'
      if (item.hallucinationRisk === '中') return 'medium'
      return 'high'
    }
    async function refresh() {
      const projectItems = await api('/api/projects')
      projects.value = Array.isArray(projectItems) ? projectItems : []
      if (!projects.value.some((item) => item.id === selectedProjectId.value)) selectedProjectId.value = projects.value[0]?.id || 0
      if (!projects.value.length) { endpoints.value=[]; runs.value=[]; cases.value=[]; documents.value=[]; modelProfiles.value=[]; return }
      await loadEndpoints()
      const [projectRuns, projectCases] = await Promise.all([api(`/api/runs?projectId=${selectedProjectId.value}`), api(`/api/cases?projectId=${selectedProjectId.value}`)])
      runs.value = Array.isArray(projectRuns) ? projectRuns : []
      cases.value = Array.isArray(projectCases) ? projectCases : []
      await loadProjectKnowledge()
      await loadProjectCheckReports()
      await loadEvaluationReports()
      loadProjectStats()
      await loadLLMSettings()
    }
    async function loadEndpoints() {
      const projectEndpoints = await api(`/api/apis?projectId=${selectedProjectId.value}`)
      endpoints.value = Array.isArray(projectEndpoints) ? projectEndpoints : []
      if (!endpoints.value.some((item) => item.id === selectedApiId.value)) selectedApiId.value = endpoints.value[0]?.id || 0
      if (selectedEndpoint.value) selectEndpoint(selectedEndpoint.value)
    }
    function selectEndpoint(item: Endpoint) { editingCaseId.value = 0; selectedApiId.value = item.id; request.value = { method: item.method, url: item.url, headers: item.headers || {}, query: item.query || {}, body: item.body || '' }; queryEditor.value = queryText(item.query || {}); aiSummary.value = ''; latest.value = null }
    function rememberSelectedProject(projectId: number) { try { localStorage.setItem(userStorage('selected-project-id'), String(projectId)) } catch { /* storage is optional */ } }
    async function selectProject() {
      rememberSelectedProject(selectedProjectId.value)
      showProjectPicker.value = false
      await loadEndpoints()
      const [projectRuns, projectCases] = await Promise.all([api(`/api/runs?projectId=${selectedProjectId.value}`), api(`/api/cases?projectId=${selectedProjectId.value}`)])
      runs.value = Array.isArray(projectRuns) ? projectRuns : []
      cases.value = Array.isArray(projectCases) ? projectCases : []
      await loadProjectKnowledge()
      await loadEvaluationReports()
      await loadLLMSettings()
      flash('已切换项目；接口和知识文档已切换，公共模型池与 AI 助手保持可用')
    }
    function toggleProjectPicker() { showProjectPicker.value = !showProjectPicker.value }
    function pickProject(item: Project) { if (item.id === selectedProjectId.value) { showProjectPicker.value = false; flash(`当前已在「${item.name}」项目中`); return } selectedProjectId.value = item.id; selectProject() }
    function projectEndpointsCount(id: number) { return projectEndpointCounts.value[id] ?? 0 }
    function projectRunsCount(id: number) { return projectRunCounts.value[id] ?? 0 }
    async function loadProjectStats() {
      if (projects.value.length <= 1) { projectEndpointCounts.value = {}; projectRunCounts.value = {}; return }
      try {
        const rows = await Promise.all(projects.value.map(async (item) => {
          const [apis, rs] = await Promise.all([api(`/api/apis?projectId=${item.id}`), api(`/api/runs?projectId=${item.id}`)])
          return [item.id, Array.isArray(apis) ? apis.length : 0, Array.isArray(rs) ? rs.length : 0] as const
        }))
        const e: Record<number, number> = {}
        const r: Record<number, number> = {}
        rows.forEach(([id, ec, rc]) => { e[id] = ec; r[id] = rc })
        projectEndpointCounts.value = e
        projectRunCounts.value = r
      } catch { projectEndpointCounts.value = {}; projectRunCounts.value = {} }
    }
    const currentProject = computed(() => projects.value.find((item) => item.id === selectedProjectId.value))
    function applyLLMSettings(data: LLMSettings) {
      llmConfigured.value = !!data.configured
      llmVerified.value = !!data.verified
      llmVerificationError.value = data.verificationError || ''
      llmVerifiedAt.value = data.verifiedAt || ''
      llmSourceProfileId.value = data.sourceProfileId || 0
      llmSourceProfileName.value = data.sourceProfileName || ''
      llmSourceProvider.value = data.sourceProvider || ''
      // 从模型池设为助手后，模型池是唯一配置来源；手动连接区回到默认空表单，避免用户误以为这里还能覆盖已选模型。
      llmForm.value = data.sourceProfileId
        ? { baseUrl: '', apiKey: '', model: '', authHeader: 'Authorization', reasoningMode: data.reasoningMode || 'auto', reasoningEffort: data.reasoningEffort || 'medium' }
        : { baseUrl: data.baseUrl || '', apiKey: data.apiKeyMasked || '', model: data.model || '', authHeader: data.authHeader || 'Authorization', reasoningMode: data.reasoningMode || 'auto', reasoningEffort: data.reasoningEffort || 'medium' }
    }
    const assistantConnectionText = computed(() => {
      if (!llmConfigured.value) return '● 尚未配置助手模型'
      const model = llmSourceProfileName.value || llmForm.value.model || '未命名模型'
      return llmVerified.value ? `● 已连通：${model}` : `● 已保存：${model}（未验证）`
    })
    const assistantConnectionHint = computed(() => {
      if (!llmConfigured.value) return '请先配置并验证助手模型，再生成测试草稿。'
      if (llmVerified.value) return `已通过真实 Chat 请求验证${llmVerifiedAt.value ? ` · ${new Date(llmVerifiedAt.value).toLocaleString()}` : ''}。`
      return llmVerificationError.value ? `上次连接失败：${llmVerificationError.value}` : '已保存连接信息；点击“测试连接”后才会确认模型可用。'
    })
    async function loadLLMSettings() { try { applyLLMSettings(await api(`/api/settings/llm?projectId=${selectedProjectId.value}`) as LLMSettings) } catch { /* settings are optional */ } }
    async function saveLLMSettings() { try { applyLLMSettings(await api('/api/settings/llm', { method: 'POST', body: JSON.stringify(llmForm.value) }) as LLMSettings); flash(llmConfigured.value ? '模型连接已保存；请点击“测试连接”确认可用' : '配置尚不完整，请填写并验证助手模型') } catch (error) { flash((error as Error).message) } }
    async function verifyLLMSettings() { try { const data = await api('/api/settings/llm/verify', { method: 'POST' }) as LLMSettings; applyLLMSettings(data); flash(data.verified ? `连接成功：已确认 ${data.model} 可以被 AI 助手调用` : `连接失败：${data.verificationError || '请检查地址、模型名、密钥与鉴权 Header'}`) } catch (error) { flash((error as Error).message) } }
    async function useProfileAsAssistant(item: ModelProfile) { try { const data = await api('/api/settings/llm/from-profile', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, profileId: item.id }) }) as LLMSettings; applyLLMSettings(data); flash(data.verified ? `AI 助手正在使用「${item.name}」；该模型已验证可用` : `AI 助手已切换为「${item.name}」；请先测试连接`) } catch (error) { flash((error as Error).message) } }
    function loadEnvironment() { try { environmentText.value = (localStorage.getItem(userStorage('environment')) || (platformAuth?.user.value?.id === 1 ? localStorage.getItem('pulse-api-v2-environment') : null)) || 'baseUrl: http://localhost:8080\ntoken: demo-token' } catch { environmentText.value = 'baseUrl: http://localhost:8080\ntoken: demo-token' } }
    function saveEnvironment() { try { localStorage.setItem(userStorage('environment'), environmentText.value) } catch { /* local storage can be unavailable */ } ; flash('环境变量已保存，可在请求中使用 {{变量名}}') }
    function resolveValue(value: string) { let result = value; Object.entries(parsePairs(environmentText.value)).forEach(([key, item]) => { result = result.split(`{{${key}}}`).join(item) }); return result }
    function formatPairs(value: Record<string, string> = {}) { return Object.entries(value).map(([key, item]) => `${key}: ${item}`).join('\n') }
    const azureImageMode = computed(() => {
      const name = selectedEndpoint.value?.name || ''
      return /\.openai\.azure\.com/i.test(request.value.url) && (/image/i.test(name) || /\/images\/generations/i.test(request.value.url) || !!azureImageDeployment.value)
    })
    function buildAzureImageURL(rawURL: string) {
      const url = rawURL.trim()
      if (!azureImageMode.value || /\/openai\/deployments\/[^/]+\/images\/generations/i.test(url) || !azureImageDeployment.value.trim()) return url
      const resourceURL = url.replace(/\/+$/, '')
      return `${resourceURL}/openai/deployments/${encodeURIComponent(azureImageDeployment.value.trim())}/images/generations?api-version=${encodeURIComponent(azureImageApiVersion.value.trim() || '2025-04-01-preview')}`
    }
    const azureResolvedURL = computed(() => buildAzureImageURL(resolveValue(request.value.url)))
    function azureImageConfigKey() { return `pulse-api-azure-image:${selectedApiId.value || request.value.url.trim()}` }
    function hydrateAzureImageConfig() {
      const url = request.value.url.trim()
      const match = url.match(/\/openai\/deployments\/([^/]+)\/images\/generations/i)
      if (match) {
        azureImageDeployment.value = decodeURIComponent(match[1])
        try {
          const parsed = new URL(url)
          azureImageApiVersion.value = parsed.searchParams.get('api-version') || '2025-04-01-preview'
          // 历史接口可能保存了完整 Azure 路由。界面只保留资源根地址，
          // 部署名与版本交给下方快捷配置管理，避免用户反复手拼长 URL。
          request.value.url = `${parsed.origin}/`
        } catch { azureImageApiVersion.value = '2025-04-01-preview' }
        return
      }
      try {
        const saved = JSON.parse(localStorage.getItem(azureImageConfigKey()) || '{}')
        azureImageDeployment.value = typeof saved.deployment === 'string' ? saved.deployment : ''
        azureImageApiVersion.value = typeof saved.apiVersion === 'string' && saved.apiVersion ? saved.apiVersion : '2025-04-01-preview'
      } catch { azureImageDeployment.value = ''; azureImageApiVersion.value = '2025-04-01-preview' }
    }
    watch([azureImageDeployment, azureImageApiVersion], () => {
      if (!azureImageMode.value) return
      try { localStorage.setItem(azureImageConfigKey(), JSON.stringify({ deployment: azureImageDeployment.value.trim(), apiVersion: azureImageApiVersion.value.trim() || '2025-04-01-preview' })) } catch { /* workspace preferences are optional */ }
    })
    watch(selectedApiId, () => hydrateAzureImageConfig(), { flush: 'post' })
    function resolvedRequest(): RequestSpec { return { method: request.value.method, url: azureResolvedURL.value, headers: Object.fromEntries(Object.entries(request.value.headers || {}).map(([key, value]) => [key, resolveValue(value)])), query: Object.fromEntries(Object.entries(request.value.query || {}).map(([key, value]) => [key, resolveValue(value)])), body: resolveValue(request.value.body || '') } }
    function updateHeaders(event: Event) { if (!showSecrets.value) return flash('请先点击“显示密钥”后再编辑敏感请求头'); request.value.headers = parsePairs((event.target as HTMLTextAreaElement).value) }
    function updateQuery(event: Event) { queryEditor.value = (event.target as HTMLTextAreaElement).value; request.value.query = parsePairs(queryEditor.value) }
    const loginUsername = ref('')
    const loginPassword = ref('')
    const registrationNickname = ref('接口测试用户')
    const registrationEmail = ref('')
    function fillLoginRequest() {
      if (!loginUsername.value.trim() || !loginPassword.value) return flash('请先填写测试用户名和密码')
      request.value = {
        ...request.value,
        method: 'POST',
        headers: { ...request.value.headers, 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: loginUsername.value.trim(), password: loginPassword.value }, null, 2),
      }
      assertions.value = [{ type: 'status', path: '', expected: '200' }, { type: 'json_field', path: 'data.token', expected: '' }]
      prompt.value = '使用测试账号登录，并验证响应中返回 Token'
      flash('已填入登录请求和 Token 断言；确认账号后即可发送')
    }
    function prepareRegistrationDraft() {
      const registration = endpoints.value.find((item) => /(^|[/_.-])register([/_.-]|$)|注册/i.test(`${item.name} ${item.url}`))
      if (!registration) return flash('当前项目未找到注册接口；请向项目开发者索要专用测试账号')
      const suffix = Date.now().toString().slice(-6)
      loginUsername.value = `cloud_test_${suffix}`
      loginPassword.value = 'Test123456'
      registrationNickname.value = '接口测试用户'
      registrationEmail.value = `${loginUsername.value}@example.com`
      selectEndpoint(registration)
      fillRegistrationRequest()
      flash('已生成注册草稿。先确认并发送注册；成功后回到登录接口，账号密码会保留。')
    }
    function fillRegistrationRequest() {
      const suffix = Date.now().toString().slice(-6)
      if (!loginUsername.value.trim()) loginUsername.value = `cloud_test_${suffix}`
      if (!loginPassword.value) loginPassword.value = 'Test123456'
      if (!registrationNickname.value.trim()) registrationNickname.value = '接口测试用户'
      if (!registrationEmail.value.trim()) registrationEmail.value = `${loginUsername.value.trim()}@example.com`
      request.value = {
        ...request.value,
        method: 'POST',
        headers: { ...request.value.headers, 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: loginUsername.value.trim(), password: loginPassword.value, nickname: registrationNickname.value.trim(), email: registrationEmail.value.trim() }, null, 2),
      }
      assertions.value = [{ type: 'status', path: '', expected: '200' }, { type: 'json_value', path: 'code', expected: '200' }]
      prompt.value = '注册专用测试账号，供后续登录和鉴权接口测试使用'
      flash('已填入注册请求；发送后只有业务 code 为 200 才会显示通过')
    }
    function saveLoginToken() {
      let token = ''
      try {
        const payload = JSON.parse(latest.value?.responseBody || '{}')
        token = String(payload?.data?.token || payload?.token || '')
      } catch { /* 响应不是 JSON 时不能提取 Token */ }
      if (!token) return flash('当前响应没有找到 data.token；请先确认登录请求是否成功')
      const values = parsePairs(environmentText.value)
      values.cloud_disk_token = token
      environmentText.value = Object.entries(values).map(([key, value]) => `${key}: ${value}`).join('\n')
      flash('已写入 cloud_disk_token 环境变量；请到“配置”页点击保存环境变量')
    }
    function selectOpenAPI(event: Event) { openApiFile.value = (event.target as HTMLInputElement).files?.[0] || null }
    async function importOpenAPI() { if (!openApiFile.value) return flash('请先选择 OpenAPI JSON 文件'); try { const spec = JSON.parse(await openApiFile.value.text()); const result = await api('/api/apis/import', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, spec }) }); await loadEndpoints(); flash(`已导入 ${result.imported} 个接口`) } catch (error) { flash(`导入失败：${(error as Error).message}`) } }
    function openProjectCheck() {
      const environment = parsePairs(environmentText.value)
      if (!projectCheckBaseURL.value) projectCheckBaseURL.value = environment.baseUrl || ''
      page.value = 'projectCheck'
      closeSidebar()
    }
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
      try { const result = await api('/api/agent/business-cases', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, apiId: selectedApiId.value, documentIds: selectedDocumentIds.value, prompt: '根据当前项目文档生成业务验收用例', mode:'design' }) }); businessCases.value = result.cases || []; flash(`已根据 ${result.documents?.length || 0} 份项目文档生成 ${businessCases.value.length} 条用例`) } catch (error) { flash((error as Error).message) }
    }
    function toggleProfile(id: number) { selectedProfileIds.value = selectedProfileIds.value.includes(id) ? selectedProfileIds.value.filter((item) => item !== id) : [...selectedProfileIds.value, id] }
    function openModelModal() { modelForm.value = emptyModelProfileForm(); discoveredModels.value = []; selectedDiscoveredModels.value = []; showModelModal.value = true }
    function bindModelPoolEmpty() {
      const empty = document.querySelector('.evaluation-page .model-pool-empty') as HTMLElement | null
      if (!empty || modelProfiles.value.length || empty.dataset.modelPoolBound) return
      empty.dataset.modelPoolBound = '1'
      empty.addEventListener('click', () => openModelModal())
      empty.setAttribute('role', 'button')
      empty.setAttribute('tabindex', '0')
      empty.setAttribute('aria-label', '添加评测模型')
    }
    function applyModelPreset() {
      const presets: Record<string, { baseUrl: string; authHeader: string }> = {
        DeepSeek: { baseUrl: 'https://api.deepseek.com/chat/completions', authHeader: 'Authorization' },
        OpenAI: { baseUrl: 'https://api.openai.com/v1/chat/completions', authHeader: 'Authorization' },
        'Azure OpenAI': { baseUrl: 'https://<resource>.openai.azure.com/openai/deployments/<deployment>/chat/completions?api-version=2024-10-21', authHeader: 'api-key' },
        'OpenAI 兼容': { baseUrl: 'https://your-provider.example.com/v1/chat/completions', authHeader: 'Authorization' },
      }
      const preset = presets[modelForm.value.provider]
      // 只更新接口地址与鉴权方式，绝不替换用户填写的模型 / 部署名。
      if (preset) { modelForm.value.baseUrl = preset.baseUrl; modelForm.value.authHeader = preset.authHeader }
    }
    async function createModelProfile() {
      const form = modelForm.value
      if (!form.name.trim() || !form.baseUrl.trim() || !form.model.trim() || !form.apiKey.trim()) return flash('模型名称、接口地址、模型名和 API Key 都不能为空')
      try {
        const item = await api(`/api/models?projectId=${selectedProjectId.value}`, { method: 'POST', body: JSON.stringify({ ...form, enabled: true }) }) as ModelProfile
        modelProfiles.value.push(item)
        selectedProfileIds.value = [...selectedProfileIds.value, item.id]
        showModelModal.value = false
        modelForm.value = emptyModelProfileForm()
        flash(`已保存模型「${item.name}」，正在测试连接…`)
        await verifyModelProfile(item)
      } catch (error) { flash((error as Error).message) }
    }
    async function discoverAvailableModels() {
      const form = modelForm.value
      if (!form.baseUrl.trim() || !form.apiKey.trim()) return flash('先填写模型网关地址和 API Key，再获取模型列表')
      discoveringModels.value = true
      try {
        const result = await api('/api/models/discover', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, baseUrl: form.baseUrl, apiKey: form.apiKey, authHeader: form.authHeader, provider: form.provider }) })
        discoveredModels.value = Array.isArray(result.models) ? result.models : []
        selectedDiscoveredModels.value = [...discoveredModels.value]
        flash(discoveredModels.value.length ? `已发现 ${discoveredModels.value.length} 个可用模型` : '网关没有返回可用模型')
      } catch (error) { flash((error as Error).message) } finally { discoveringModels.value = false }
    }
    function toggleDiscoveredModel(model: string) { selectedDiscoveredModels.value = selectedDiscoveredModels.value.includes(model) ? selectedDiscoveredModels.value.filter((item) => item !== model) : [...selectedDiscoveredModels.value, model] }
    async function importAvailableModels() {
      const form = modelForm.value
      if (!selectedDiscoveredModels.value.length) return flash('请至少选择一个要加入模型池的模型')
      importingDiscoveredModels.value = true
      try {
        const result = await api('/api/models/import', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, baseUrl: form.baseUrl, apiKey: form.apiKey, authHeader: form.authHeader, provider: form.provider, models: selectedDiscoveredModels.value }) })
        await loadProjectKnowledge()
        discoveredModels.value = []
        selectedDiscoveredModels.value = []
        modelForm.value = emptyModelProfileForm()
        showModelModal.value = false
        flash(result.message || '已加入公共模型池')
      } catch (error) { flash((error as Error).message) } finally { importingDiscoveredModels.value = false }
    }
    function replaceModelProfile(item: ModelProfile) {
      const index = modelProfiles.value.findIndex((profile) => profile.id === item.id)
      if (index >= 0) modelProfiles.value[index] = item
      if (editingModel.value?.id === item.id) editingModel.value = { ...item }
    }
    async function verifyModelProfile(item: ModelProfile) {
      try {
        const updated = await api('/api/models/verify', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, profileId: item.id }) }) as ModelProfile
        replaceModelProfile(updated)
        if (updated.verified && (!llmSourceProfileId.value || llmSourceProfileId.value === item.id)) await useProfileAsAssistant(updated)
        else if (llmSourceProfileId.value === item.id) await loadLLMSettings()
        flash(updated.verified ? `连接成功：${updated.model} 已验证可用` : `连接失败：${updated.verificationError || '请检查地址、部署名、Key 与推理模式'}`)
      } catch (error) { flash((error as Error).message) }
    }
    function verifyAssistantModel() {
      const item = modelProfiles.value.find((profile) => profile.id === llmSourceProfileId.value)
      if (!item) return flash('请先从模型池选择 AI 助手模型')
      verifyModelProfile(item)
    }
    function selectAssistantModel(event: Event) {
      const id = Number((event.target as HTMLSelectElement).value)
      const item = modelProfiles.value.find((profile) => profile.id === id)
      if (item) useProfileAsAssistant(item)
    }
    function openModelDetail(item: ModelProfile) {
      editingModel.value = { ...item }
      editingModelApiKey.value = ''
      showModelDetailModal.value = true
    }
	function deleteModelProfile(item: ModelProfile) { pendingModelDeletion.value = item }
	async function confirmModelDeletion() {
		const item = pendingModelDeletion.value
		if (!item) return
		pendingModelDeletion.value = null
		try {
			await api(`/api/models?projectId=${selectedProjectId.value}&id=${item.id}`, { method: 'DELETE' })
			modelProfiles.value = modelProfiles.value.filter((profile) => profile.id !== item.id)
			selectedProfileIds.value = selectedProfileIds.value.filter((id) => id !== item.id)
			if (editingModel.value?.id === item.id) { editingModel.value = null; showModelDetailModal.value = false }
			if (llmSourceProfileId.value === item.id) await loadLLMSettings()
			flash(`已删除模型「${item.name}」`)
		} catch (error) { flash((error as Error).message) }
	}
    async function saveModelDetail() {
      const item = editingModel.value
      if (!item || !item.name.trim() || !item.baseUrl.trim() || !item.model.trim()) return flash('模型名称、接口地址和模型 / 部署名不能为空')
      try {
        const updated = await api(`/api/models?projectId=${selectedProjectId.value}&id=${item.id}`, { method: 'PUT', body: JSON.stringify({ ...item, apiKey: editingModelApiKey.value }) }) as ModelProfile
        replaceModelProfile(updated)
        if (llmSourceProfileId.value === item.id) await loadLLMSettings()
        flash(`已保存「${updated.name}」配置；请重新测试连接`)
      } catch (error) { flash((error as Error).message) }
    }
    async function saveModelProfile(item: ModelProfile) {
      if (!item.model.trim()) return flash('请填写正确的模型 / 部署名')
      try {
        const updated = await api(`/api/models?projectId=${selectedProjectId.value}&id=${item.id}`, { method: 'PUT', body: JSON.stringify({ model: item.model, reasoningMode: item.reasoningMode || 'auto', reasoningEffort: item.reasoningEffort || 'medium' }) }) as ModelProfile
        replaceModelProfile(updated)
        if (llmSourceProfileId.value === item.id) applyLLMSettings(await api('/api/settings/llm') as LLMSettings)
        flash(`已保存「${updated.name}」的部署名和推理策略`)
      } catch (error) { flash((error as Error).message) }
    }
    async function runEvaluation() {
      const questions = evaluationQuestions.value.split('\n').map((item) => item.trim()).filter(Boolean)
      if (!modelProfiles.value.length) return openModelModal()
      if (!selectedProfileIds.value.length) return flash('请至少勾选一个模型')
      if (!questions.length) return flash('请至少填写一个评测问题')
      evaluationLoading.value = true
      try {
        const result = await api('/api/evaluations/run', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, profileIds: selectedProfileIds.value, documentIds: selectedDocumentIds.value, questions }) }) as EvaluationReport
        evaluationResults.value = result.results || []
        activeEvaluationReport.value = result
        evaluationReports.value = [result, ...evaluationReports.value.filter((item) => item.id !== result.id)]
        showEvaluationResultModal.value = true
        flash(`已完成 ${selectedProfileIds.value.length} 个模型 × ${questions.length} 个问题的并发评测；结果已保存`)
      } catch (error) { flash((error as Error).message) } finally { evaluationLoading.value = false }
    }
    async function createEndpoint() {
      if (!endpointForm.value.name.trim() || !endpointForm.value.url.trim()) return flash('接口名称和 URL 不能为空')
      try {
        const isEditing = editingEndpointId.value > 0
        const item = await api('/api/apis', { method: isEditing ? 'PUT' : 'POST', body: JSON.stringify({ ...endpointForm.value, id: editingEndpointId.value, projectId: selectedProjectId.value, headers: parsePairs(endpointForm.value.headers), query: {}, enabled: true }) }) as Endpoint
        await loadEndpoints(); selectEndpoint(item); showEndpointModal.value = false; editingEndpointId.value = 0; endpointForm.value = { name: '', method: 'GET', url: '', headers: '', body: '', kind: 'REST' }; flash(isEditing ? '接口已修改' : '接口已添加')
      } catch (error) { flash((error as Error).message) }
    }
    function openEndpointEditor(item: Endpoint) { editingEndpointId.value = item.id; endpointForm.value = { name: item.name, method: item.method, url: item.url, headers: formatPairs(item.headers || {}), body: item.body || '', kind: item.kind || 'REST' }; showEndpointModal.value = true }
    function deleteEndpoint(item: Endpoint) { pendingEndpointDeletion.value = item }
    async function confirmEndpointDeletion() {
      const item = pendingEndpointDeletion.value
      if (!item) return
      pendingEndpointDeletion.value = null
      try { await api(`/api/apis?projectId=${selectedProjectId.value}&id=${item.id}`, { method: 'DELETE' }); if (selectedApiId.value === item.id) { selectedApiId.value = 0; latest.value = null }; await loadEndpoints(); flash('接口已删除') } catch (error) { flash((error as Error).message) }
    }
    function openWorkspaceDatabase(): Promise<IDBDatabase> {
      return new Promise((resolve, reject) => {
        const request = indexedDB.open(platformAuth?.user.value?.id === 1 ? 'pulse-api-project-workspaces' : userStorage('project-workspaces'), 1)
        request.onupgradeneeded = () => { if (!request.result.objectStoreNames.contains('folders')) request.result.createObjectStore('folders', { keyPath: 'projectId' }) }
        request.onsuccess = () => resolve(request.result)
        request.onerror = () => reject(request.error || new Error('无法打开本地工作区存储'))
      })
    }
    async function saveProjectWorkspace(projectId: number, handle: any, updateView = true) {
      if (updateView) projectWorkspaces.value = { ...projectWorkspaces.value, [projectId]: handle.name }
      const db = await openWorkspaceDatabase()
      await new Promise<void>((resolve, reject) => {
        const transaction = db.transaction('folders', 'readwrite')
        transaction.objectStore('folders').put({ projectId, name: handle.name, handle })
        transaction.oncomplete = () => { db.close(); resolve() }
        transaction.onerror = () => { db.close(); reject(transaction.error || new Error('无法保存本地文件夹关联')) }
      })
    }
    async function loadProjectWorkspaces() {
      try {
        const db = await openWorkspaceDatabase()
        const rows = await new Promise<any[]>((resolve, reject) => {
          const request = db.transaction('folders', 'readonly').objectStore('folders').getAll()
          request.onsuccess = () => resolve(request.result || [])
          request.onerror = () => reject(request.error)
        })
        db.close()
        projectWorkspaces.value = Object.fromEntries(rows.map((item) => [item.projectId, item.name]))
      } catch { projectWorkspaces.value = {} }
    }
    function projectWorkspaceName(projectId: number) { return projectWorkspaces.value[projectId] || '' }
    async function chooseProjectFolder() {
      const picker = (window as any).showDirectoryPicker
      if (!picker) { flash('当前浏览器不支持本地文件夹选择；请使用新版 Chrome 或 Edge 打开本地平台'); return null }
      try { return await picker({ mode: 'read' }) } catch (error: any) { if (error?.name !== 'AbortError') flash('无法选择本地文件夹') ; return null }
    }
    async function chooseFolderForNewProject() {
      const handle = await chooseProjectFolder()
      if (!handle) return
      pendingProjectFolder.value = handle
      pendingProjectFolderName.value = handle.name
      if (!projectForm.value.name.trim()) projectForm.value.name = handle.name
      if (!projectForm.value.summary.trim()) projectForm.value.summary = `关联本地文件夹：${handle.name}`
      flash(`已选择本地文件夹「${handle.name}」`)
    }
    async function openProjectFromLocalFolder() {
      const handle = await chooseProjectFolder()
      if (!handle) return
      const existing = projects.value.find((project) => project.name === handle.name)
      if (existing) {
        try {
          await saveProjectWorkspace(existing.id, handle, false)
          rememberSelectedProject(existing.id)
          window.location.reload()
        } catch (error) { flash((error as Error).message) }
        return
      }
      try {
        const item = await api('/api/projects', { method: 'POST', body: JSON.stringify({ name: handle.name, summary: `关联本地文件夹：${handle.name}` }) })
        await saveProjectWorkspace(item.id, handle, false)
        rememberSelectedProject(item.id)
        window.location.reload()
      } catch (error) { flash((error as Error).message) }
    }
    async function createProject() {
      if (!projectForm.value.name.trim()) return flash('请填写项目名称')
      const folder = pendingProjectFolder.value
      try {
        const item = await api('/api/projects', { method: 'POST', body: JSON.stringify(projectForm.value) })
        projects.value.push(item)
        selectedProjectId.value = item.id
        rememberSelectedProject(item.id)
        if (folder) await saveProjectWorkspace(item.id, folder)
        projectForm.value = { name: '', summary: '' }
        pendingProjectFolder.value = null
        pendingProjectFolderName.value = ''
        showProjectModal.value = false
        await loadEndpoints()
        flash(folder ? `项目已创建，并关联本地文件夹「${folder.name}」` : '项目已创建')
      } catch (error) { flash((error as Error).message) }
    }
    async function generateDraft(): Promise<Draft | null> {
      if (!prompt.value.trim()) { flash('请先描述想测试什么'); return null }
      assistantDecisionDraft.value = null
      assistantPlan.value = null
      assistantEndpointDraft.value = null
      assistantDeleteDraft.value = null
      aiSummary.value = ''
      aiReasoning.value = []
      loading.value = true
      try {
        const draft = await api('/api/agent/draft', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, apiId: showAssistant.value ? assistantScopeApiId.value : selectedApiId.value, scope: showAssistant.value && assistantScopeApiId.value ? "endpoint" : "project", profileId: showAssistant.value ? assistantProfileId.value : 0, prompt: prompt.value, documentIds: assistantDocumentId.value ? [assistantDocumentId.value] : selectedDocumentIds.value }) }) as Draft
        assistantDecisionDraft.value = draft
        assistantPlan.value = draft.plan?.length ? { summary: draft.summary, intent: draft.intent || 'test_api', steps: draft.plan, requiresConfirmation: Boolean(draft.requiresConfirmation), mode: draft.mode || 'plan-agent' } : null
        if (draft.intent === 'answer') {
          aiSummary.value = ''
          aiReasoning.value = []
          return draft
        }
        assistantEndpointDraft.value = draft.intent === 'add_api' && draft.endpoint ? draft.endpoint : null
        assistantDeleteDraft.value = draft.intent === 'delete_api' && draft.endpoint ? draft.endpoint : null
        if (assistantDeleteDraft.value) {
          aiSummary.value = draft.summary
          aiReasoning.value = []
          flash('AI 已准备删除操作，等待你的确认')
          return draft
        }
        if (assistantEndpointDraft.value) {
          aiSummary.value = '已根据 AI 小助手生成接口配置草稿'
          aiReasoning.value = draft.reasoning || []
          flash('AI 已生成接口配置，请确认后添加')
          return draft
        }
        if (draft.intent === 'clarify') {
          aiSummary.value = ''
          aiReasoning.value = []
          flash('还需要确认目标接口或补充测试信息')
          return draft
        }
        const draftedURL = draft.request.url.replace(/\/+$/, '')
        const matchedEndpoint = endpoints.value.find((item) => item.url.replace(/\/+$/, '') === draftedURL)
        if (matchedEndpoint) selectedApiId.value = matchedEndpoint.id
        request.value = draft.request
        assertions.value = draft.assertions?.length ? draft.assertions : [{ type: 'status', path: '', expected: '200' }]
        queryEditor.value = queryText(draft.request.query || {})
        aiSummary.value = '已生成可审核的请求草稿'
        aiReasoning.value = draft.reasoning || []
        await auditAgent('draft_generated', { intent: draft.intent || 'test_api', mode: draft.mode || 'llm-agent', summary: draft.summary })
        flash('AI 已补齐请求，请确认后发送')
        return draft
      } catch (error) {
        const message = (error as Error).message
        assistantMessages.value.push({ role: 'assistant', text: message })
        flash(message)
        return null
      } finally { loading.value = false }
    }
    function setAssistantPermission() {
      try { localStorage.setItem(userStorage('assistant-permission'), assistantPermission.value) } catch { /* 浏览器不支持本地保存时仍保留本次选择 */ }
      const message = assistantPermission.value === 'project_full'
        ? '已切换为当前项目最高权限：助手可按你的明确任务自动导入文档、管理接口、执行测试和导出报告。'
        : '已切换为默认权限：助手会先生成计划和草稿，执行或修改前等待你的确认。'
      flash(message)
    }
    function openAssistant() {
      assistantInput.value = prompt.value
      showAssistant.value = true
      loadProjectCheckReports()
      if (!assistantMessages.value.length) assistantMessages.value.push({ role: 'assistant', text: '你好，我是当前项目的测试助手。直接告诉我任务即可；例如“导入 OpenAPI 后检查所有接口是否联通并导出报告”。你可以在输入框下方选择权限、接口和模型。' })
    }
    function closeAssistant() { showAssistant.value = false }
    async function selectAssistantDocument(event: Event) {
      const file = (event.target as HTMLInputElement).files?.[0]
      if (!file) return
      assistantDocumentName.value = file.name
      assistantDocumentStatus.value = '识别中…'
      try {
        if (file.name.toLowerCase().endsWith('.json')) {
          const candidate = JSON.parse(await file.text())
          assistantOpenAPISpec.value = candidate && typeof candidate === 'object' && candidate.paths ? candidate : null
        } else {
          assistantOpenAPISpec.value = null
        }
        const form = new FormData()
        form.append('file', file)
        const response = await fetch('/api/agent/document', { method: 'POST', body: form })
        const data = await response.json().catch(() => ({}))
        if (!response.ok) throw new Error(data.error || '文档识别失败')
        assistantDocumentText.value = data.text || ''
        assistantDocumentStatus.value = assistantOpenAPISpec.value ? `已识别 ${data.characters || 0} 字 · OpenAPI 可直接导入` : `已识别 ${data.characters || 0} 字`
        try {
          const knowledgeForm = new FormData(); knowledgeForm.append('file', file); knowledgeForm.append('projectId', String(selectedProjectId.value)); knowledgeForm.append('docType', file.name.toLowerCase().includes('openapi') || file.name.toLowerCase().includes('swagger') ? '接口契约' : '业务规则'); knowledgeForm.append('version', 'v1')
          const saved = await fetch('/api/documents', { method: 'POST', body: knowledgeForm }).then(async (item) => { const body = await item.json().catch(() => ({})); if (!item.ok) throw new Error(body.error || '项目文档保存失败'); return body })
          assistantDocumentId.value = saved.id || 0
          await loadProjectKnowledge()
        } catch { assistantDocumentId.value = 0 }
        assistantMessages.value.push({ role: 'assistant', text: assistantOpenAPISpec.value ? `已读取《${file.name}》，识别到 OpenAPI 接口文档。现在可以直接说“检查所有接口是否联通并导出报告”。` : `已读取《${file.name}》，识别出 ${data.characters || 0} 个字符。接下来可以让我根据文档添加接口或生成测试。` })
      } catch (error) {
        assistantDocumentName.value = ''
        assistantDocumentText.value = ''
        assistantDocumentStatus.value = ''
        assistantOpenAPISpec.value = null
        flash((error as Error).message)
      }
      ;(event.target as HTMLInputElement).value = ''
    }
    function clearAssistantDocument() { assistantDocumentName.value = ''; assistantDocumentText.value = ''; assistantDocumentStatus.value = ''; assistantDocumentId.value = 0; assistantOpenAPISpec.value = null }
    function streamAssistantReply(text: string) {
      const message: AssistantMessage = { role: 'assistant', text: '' }
      assistantMessages.value.push(message)
      return new Promise<void>((resolve) => {
        let cursor = 0
        const timer = window.setInterval(() => {
          message.text += text.slice(cursor, cursor + 4)
          cursor += 4
          if (cursor >= text.length) { window.clearInterval(timer); resolve() }
        }, 16)
      })
    }
    function openManualReview(item: ProjectCheckItem) {
      const endpoint = endpoints.value.find((candidate) => candidate.id === item.apiId)
      if (!endpoint) return flash('没有找到对应接口，请先重新导入 OpenAPI 文档')
      selectEndpoint(endpoint)
      assistantReviewOpen.value = false
      assistantManualReview.value = true
      const requirements = endpoint.method === 'DELETE'
        ? '一个可安全删除、且允许恢复的测试数据 ID，以及删除后的预期结果。'
        : /upload|chunk/i.test(`${endpoint.name} ${endpoint.url}`)
          ? '一个可用于测试的文件、必要的上传 ID，以及成功后的预期字段。'
          : /login|register|password|auth/i.test(`${endpoint.name} ${endpoint.url}`)
            ? '专用测试账号、密码或验证码规则，以及期望状态码。'
            : '有效 Token（如接口需要）、请求体中的业务字段、可使用的测试数据，以及预期状态码或返回字段。'
      const guide = `“${endpoint.name}”目前是待人工配置接口，因为它会${item.risk.includes('高风险') ? '删除或修改真实数据' : '写入或修改业务数据'}。请提供：${requirements}`
      assistantMessages.value.push({ role: 'assistant', text: guide })
      assistantInput.value = `请根据当前接口契约，帮我配置并测试“${endpoint.name}”。我会补充：${requirements} 先不要发送请求。`
      flash('已选中待审核接口；请在下方补充助手提示的信息')
    }
    async function executeAssistantEndpointDraft() {
      const draft = assistantEndpointDraft.value
      if (!draft) return
      try {
        const created = await api('/api/apis', { method: 'POST', body: JSON.stringify({ ...draft, id: 0, projectId: selectedProjectId.value }) }) as Endpoint
        assistantEndpointDraft.value = null
        await loadEndpoints()
        selectEndpoint(created)
        await auditAgent('endpoint_auto_created', { name: created.name, method: created.method, url: created.url }, true)
        await streamAssistantReply(`已按你的任务把“${created.name}”加入当前项目，并已选中它。`)
      } catch (error) { flash(`自动添加接口失败：${(error as Error).message}`) }
    }
    async function executeAssistantEndpointDeletion() {
      const draft = assistantDeleteDraft.value
      if (!draft) return
      try {
        await api(`/api/apis?projectId=${selectedProjectId.value}&id=${draft.id}`, { method: 'DELETE' })
        assistantDeleteDraft.value = null
        if (selectedApiId.value === draft.id) selectedApiId.value = 0
        await loadEndpoints()
        await auditAgent('endpoint_auto_deleted', { apiId: draft.id, name: draft.name }, true)
        await streamAssistantReply(`已按你的任务删除接口“${draft.name}”。历史测试记录仍会保留。`)
      } catch (error) { flash(`自动删除接口失败：${(error as Error).message}`) }
    }
    async function askAssistant() {
      const question = assistantInput.value.trim()
      if (!question) return flash('请先描述你想验证的内容')
      assistantMessages.value.push({ role: 'user', text: question })
      const taskPrompt = assistantDocumentText.value ? `${question}\n\n参考文档《${assistantDocumentName.value}》：\n${assistantDocumentText.value}` : question
      prompt.value = taskPrompt
      assistantInput.value = ''
      assistantEndpointDraft.value = null
      assistantDeleteDraft.value = null
      assistantPlan.value = null
      assistantDecisionDraft.value = null
      assistantProjectCheck.value = null
      assistantProjectCheckDraft.value = null
      businessCases.value = []
      if (isProjectConnectivityTask(question)) {
        projectTaskGoal.value = question
        assistantMessages.value.push({ role: 'assistant', text: '已将需求带入项目测试。选择账号和文件来源，生成流程后即可运行并保存报告。' })
        openProjectTask()
        prompt.value = '测试这个接口是否可用，并检查返回结果'
        return
      }
      if (documents.value.length && /测试用例|业务规则/.test(question)) {
        try {
          const result = await api('/api/agent/business-cases', { method: 'POST', body: JSON.stringify({ projectId: selectedProjectId.value, apiId: assistantScopeApiId.value, profileId: assistantProfileId.value, mode: 'design', documentIds: assistantDocumentId.value ? [assistantDocumentId.value] : selectedDocumentIds.value, prompt: question }) })
          businessCases.value = result.cases || []
          assistantMessages.value.push({ role:'assistant', text: `已生成 ${businessCases.value.length} 条业务草稿。${(result.missingInformation || []).join('；')} 可在项目知识库补充资料、核对依据并保存。` }); page.value = 'knowledge'; closeAssistant(); return
        } catch { /* 没有匹配到业务文档时继续走普通请求草稿流程 */ }
      }
      const draft = await generateDraft()
      if (draft) prompt.value = '测试这个接口是否可用，并检查返回结果'
      if (draft?.intent === 'answer') await streamAssistantReply(draft.summary)
      else if (draft?.intent === 'clarify') {
        const candidates = (draft.candidateEndpoints || []).map((item, index) => `${index + 1}. ${item.method} ${item.url}（${item.name}）`).join('\n')
        const questions = (draft.missingInformation || []).map((item) => item.question).join('；')
        await streamAssistantReply(`${draft.summary}${candidates ? `\n可选接口：\n${candidates}` : ''}${questions ? `\n下一步：${questions}` : ''}`)
      }
      else if (draft?.intent === 'delete_api' && assistantDeleteDraft.value) {
        await streamAssistantReply(`${aiSummary.value}。这是高风险操作，请点击“确认删除接口”继续。`)
      } else if (draft?.intent === 'add_api' && draft.endpoint) {
        await streamAssistantReply(`${aiSummary.value}。请检查名称、方法、地址和请求体，确认后添加到当前项目。`)
      } else if (aiSummary.value) {
        if (assistantPermission.value === 'project_full' && draft?.executionDecision === 'ready_to_execute') {
          await sendRequest(true)
          await streamAssistantReply(latest.value ? `已按你的任务执行测试：HTTP ${latest.value.statusCode || '请求错误'}，${latest.value.success ? '断言通过。' : '请查看测试台中的失败原因和响应结果。'}` : '请求未能完成，请检查服务地址和网络后重试。')
        } else {
          const missing = (draft?.missingInformation || []).map((item) => item.question).join('；')
          await streamAssistantReply(`${aiSummary.value}。${missing ? `还需要：${missing}` : '请检查草稿、测试数据和断言后再执行。'}`)
        }
      }
    }
    function useAssistantSuggestion(text: string) { assistantInput.value = text; askAssistant() }
    async function applyAssistantDraft() {
      if (assistantDeleteDraft.value) {
        pendingEndpointDeletion.value = assistantDeleteDraft.value
        assistantDeleteDraft.value = null
        showAssistant.value = false
        return
      }
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
      assistantManualReview.value = false
      await auditAgent('draft_applied', { method: request.value.method, url: request.value.url }, true)
      flash('已应用 Agent 草稿，请检查请求后再发送')
    }
    /* ------------------------------------------------------------------
       测试台自动捕获模型配置
       ------------------------------------------------------------------
       规则：仅当选中接口为 LLM 类型、且请求成功时，从当前请求中提取
       模型信息并登记到模型池。重复配置（同 baseUrl + model）不重复添加。
       任何异常都静默处理，不阻塞测试主流程。
       ------------------------------------------------------------------ */
    async function captureModelFromRun() {
      try {
        const endpoint = selectedEndpoint.value
        if (!endpoint || endpoint.kind !== 'LLM') return
        const url = request.value.url.trim()
        if (!url) return
        // 从请求体提取 model 标识
        let model = ''
        try { const parsed = JSON.parse(request.value.body || '{}'); model = String(parsed.model || '') } catch { /* 非 JSON 请求体，跳过 */ }
        if (!model) return
        // 提取 API Key（Authorization: Bearer xxx）
        const authKey = Object.keys(request.value.headers || {}).find((k) => k.toLowerCase() === 'authorization')
        const rawAuth = authKey ? String((request.value.headers as Record<string, string>)[authKey] || '') : ''
        const apiKey = rawAuth.replace(/^Bearer\s+/i, '').trim()
        if (!apiKey) return
        // 去重：同 baseUrl + model 已存在则跳过
        const exists = modelProfiles.value.some((p) => p.baseUrl === url && p.model === model)
        if (exists) return
        // 从 URL 推断提供商
        let provider = 'OpenAI 兼容'
        if (/deepseek/i.test(url)) provider = 'DeepSeek'
        else if (/azure/i.test(url)) provider = 'Azure OpenAI'
        else if (/openai/i.test(url)) provider = 'OpenAI'
        else if (/moonshot|kimi/i.test(url)) provider = 'Moonshot'
        else if (/qwen|dashscope/i.test(url)) provider = '通义千问'
        const item = await api(`/api/models?projectId=${selectedProjectId.value}`, {
          method: 'POST',
          body: JSON.stringify({ name: `${provider} · ${model}`, provider, baseUrl: url, model, apiKey, authHeader: 'Authorization', enabled: true })
        }) as ModelProfile
        modelProfiles.value.push(item)
        flash(`已自动登记模型：${item.name}，可在模型评测中选择`)
      } catch { /* 自动捕获失败不影响测试结果 */ }
    }

    async function sendRequest(saveCase: boolean) {
      if (!request.value.url.trim()) return flash('请输入请求 URL')
      if (azureImageMode.value && !azureImageDeployment.value.trim() && !/\/openai\/deployments\/[^/]+\/images\/generations/i.test(request.value.url)) return flash('请填写 Azure 部署名，例如 gpt-image-2；平台会自动拼接图片生成地址')
      loading.value = true
      try { const savedCase = cases.value.find(item => item.id === editingCaseId.value && item.apiId === selectedApiId.value && item.projectId === selectedProjectId.value); if (saveCase && savedCase) await api('/api/cases', {method:'PUT',body:JSON.stringify({...savedCase,request:resolvedRequest(),assertions:assertions.value,prompt:prompt.value})}); const run = await api('/api/runs', { method: 'POST', body: JSON.stringify({ caseId:savedCase?.id || 0, projectId: selectedProjectId.value, apiId: selectedApiId.value, name: prompt.value.slice(0, 40) || selectedEndpoint.value?.name || '临时请求', prompt: prompt.value, request: resolvedRequest(), assertions: assertions.value, saveCase:saveCase && !savedCase }) }) as Run; latest.value = run; runs.value.unshift(run); await auditAgent('api_test_executed', { runId: run.id, statusCode: run.statusCode, success: run.success }, true); flash(run.success ? '请求成功，结果已保存' : `请求完成：${run.errorCategory || '存在失败断言'}`); if (run.success) await captureModelFromRun() } catch (error) { flash((error as Error).message) } finally { loading.value = false }
    }
    function loadCase(item: TestCase) { editingCaseId.value = item.id; selectedApiId.value = item.apiId; request.value = item.request; queryEditor.value = queryText(item.request.query || {}); assertions.value = item.assertions?.length ? item.assertions : [{ type: 'status', path: '', expected: '200' }]; prompt.value = item.prompt || item.name; latest.value = null; page.value = 'tester'; flash(`已载入用例：${item.name}`) }
    async function updateCase() { if (!editingCaseId.value) return flash('请先从历史中载入一个测试用例'); try { const item = { id: editingCaseId.value, projectId: selectedProjectId.value, apiId: selectedApiId.value, name: prompt.value.slice(0, 40) || '已编辑测试用例', prompt: prompt.value, request: resolvedRequest(), assertions: assertions.value }; await api('/api/cases', { method: 'PUT', body: JSON.stringify(item) }); cases.value = await api('/api/cases'); flash('测试用例已更新') } catch (error) { flash((error as Error).message) } }
    async function runBatch() {
      if (!endpoints.value.length) return flash('当前项目没有接口')
      const readOnlyEndpoints = endpoints.value.filter((item) => ['GET', 'HEAD', 'OPTIONS'].includes(item.method.toUpperCase()))
      if (!readOnlyEndpoints.length) return flash('当前项目没有可安全巡检的只读接口；POST、PUT、DELETE 等接口请逐条人工配置。')
      loading.value = true
      try { const result = await api('/api/runs/batch', { method: 'POST', body: JSON.stringify({ concurrency: batchConcurrency.value, items: readOnlyEndpoints.map((item) => ({ projectId: selectedProjectId.value, apiId: item.id, name: `安全巡检 · ${item.name}`, request: { method: item.method, url: resolveValue(item.url), headers: {}, query: {}, body: '' }, assertions: [{ type: 'status', path: '', expected: '200' }], saveCase: false })) }) }); batchResult.value = { count: result.count || 0, errors: result.errors || [] }; runs.value.unshift(...(result.runs || [])); flash(`安全巡检完成：已检查 ${readOnlyEndpoints.length} 个只读接口`) } catch (error) { flash((error as Error).message) } finally { loading.value = false }
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
    async function inspectRegression(item: Run) { regressionDetail.value = null; try { regressionDetail.value = await api(`/api/regression?caseId=${item.caseId}`) } catch (error) { flash((error as Error).message) } }
    async function rerun(item: Run, compare = false) {
      loading.value = true
      try { const run = await api('/api/runs', { method: 'POST', body: JSON.stringify(item.caseId ? { caseId: item.caseId } : { apiId: item.apiId, request: request.value, assertions: assertions.value, name: item.caseName }) }) as Run; latest.value = run; runs.value.unshift(run); if (compare && item.caseId) { page.value = 'history'; recordsTab.value = 'regression'; await inspectRegression(run) } else page.value = 'tester'; flash(compare ? '已重新执行，正在展示前后结果差异' : '已重新执行请求，可继续比较前后结果') } catch (error) { flash((error as Error).message) } finally { loading.value = false }
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
      loadProjectWorkspaces()
      refresh().catch((error) => flash(error.message))
      // Docker 启动时后端可能比前端晚几秒，自动重试一次避免打开后列表为空。
      window.setTimeout(() => refresh().catch(() => undefined), 1800)
    })
    return {
      projectCheckBaseURL, projectCheckTask, projectCheckPermission, projectCheckFile,
      projectCheckLoading, projectCheckResult, selectProjectCheckFile, runProjectCheck,
      isAdmin, readOnly, canExecute, canAuto, readonlyRun, viewRecord, api, projectTaskGoal, openProjectTask, openTestingWorkspace, recordsTab, forwardAssistantWheel, assistantDecisionDraft,
      page, sidebarCollapsed, toggleSidebar, closeSidebar, showProjectPicker, toggleProjectPicker, pickProject, currentProject, projectEndpointsCount, projectRunsCount, loadProjectStats, projectWorkspaceName, openProjectFromLocalFolder, chooseFolderForNewProject, pendingProjectFolderName,
      assistantProfileId, assistantScopeApiId, availableProfiles, assistantFabStyle, assistantFabDragging, startAssistantFabDrag, clickAssistantFab, projects, endpoints, runs, cases, selectedProjectId, selectedApiId, prompt, request, assertions, normalizeAssertion, azureImageMode, azureImageDeployment, azureImageApiVersion, azureResolvedURL, queryEditor, aiSummary, aiReasoning, latest, notice, loading,
      showEndpointModal, showModelModal, showModelDetailModal, showProjectModal, showEvaluationResultModal, showAssistant, assistantInput, assistantMessages, assistantEndpointDraft, assistantDeleteDraft, assistantDocumentName, assistantDocumentText, assistantDocumentStatus, assistantPlan, assistantPermission, setAssistantPermission, assistantProjectCheck, assistantProjectCheckDraft, projectCheckReports, assistantReviewOpen, assistantPendingReviewItems,
      projectForm, endpointForm, editingEndpointId, pendingEndpointDeletion, pendingModelDeletion, modelForm, discoveredModels, selectedDiscoveredModels, discoveringModels, importingDiscoveredModels, editingModel, editingModelApiKey, selectedEndpoint, filteredEndpoints, successRate, savedRuns, responsePretty, responseImagePreview, responseHeadersText, headerEditorText, searchText, showSecrets, batchConcurrency, batchResult, editingCaseId, regressionDetail,
      llmForm, llmConfigured, llmVerified, llmVerificationError, llmSourceProfileId, llmSourceProfileName, llmSourceProvider, assistantConnectionText, assistantConnectionHint, environmentText, isLoginEndpoint, isRegistrationEndpoint, loginUsername, loginPassword, registrationNickname, registrationEmail, chainSteps, chainVariablesText, chainResult, documents, selectedDocumentIds, documentType, documentVersion, businessCases, modelProfiles, selectedProfileIds, evaluationQuestions, evaluationQuestionCount, evaluationResults, evaluationReports, activeEvaluationReport, evaluationLoading,
      emptyRequest, refresh, loadEndpoints, selectEndpoint, selectProject, updateHeaders, updateQuery, fillLoginRequest, fillRegistrationRequest, prepareRegistrationDraft, saveLoginToken, selectOpenAPI, importOpenAPI, uploadProjectDocument, deleteProjectDocument, toggleDocument, generateBusinessCases, toggleProfile, openModelModal, openModelDetail, deleteModelProfile, confirmModelDeletion, applyModelPreset, createModelProfile, discoverAvailableModels, toggleDiscoveredModel, importAvailableModels, saveModelProfile, saveModelDetail, verifyModelProfile, verifyAssistantModel, selectAssistantModel, runEvaluation, loadEvaluationReports, openEvaluationReport, openEvaluationHistory, formatEvaluationTime, evaluationResultRiskClass, createEndpoint, openEndpointEditor, deleteEndpoint, confirmEndpointDeletion, createProject, generateDraft, openAssistant, closeAssistant, selectAssistantDocument, clearAssistantDocument, askAssistant, useAssistantSuggestion, applyAssistantDraft, confirmAssistantProjectCheck, downloadProjectCheckReport, restoreProjectCheckReport, openManualReview, sendRequest, captureModelFromRun, loadCase, updateCase, runBatch, addChainStep, syncChainStep, removeChainStep, runChain, inspectRegression, loadLLMSettings, saveLLMSettings, verifyLLMSettings, useProfileAsAssistant, saveEnvironment, rerun, diagnosis, copyResponse, formatPairs
    }
  },
  template: `
  <div class="app-v2" :class="{ 'sidebar-collapsed': sidebarCollapsed }">
    <div v-if="!sidebarCollapsed" class="sidebar-scrim" @click="closeSidebar"></div>
    <header class="topbar-v2"><div class="brand-v2"><div class="brand-logo" aria-hidden="true"><svg viewBox="0 0 32 32" fill="none">
          <defs>
            <linearGradient id="bgGrad" x1="4" y1="4" x2="28" y2="28" gradientUnits="userSpaceOnUse">
              <stop stop-color="#68CBBE"/><stop offset=".55" stop-color="#39A99A"/><stop offset="1" stop-color="#1F7E75"/>
            </linearGradient>
          </defs>
          <rect width="32" height="32" rx="9" fill="url(#bgGrad)"/>
          <path d="M9.6 23.4V9.2h5.5a3.9 3.9 0 0 1 0 7.8H9.6" stroke="#fff" stroke-width="2.3" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M18.4 23.4l3.7-8.4 3.7 8.4" stroke="#fff" stroke-width="2.3" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M19.9 19.9h4.4" stroke="#fff" stroke-width="2" stroke-linecap="round" opacity=".9"/>
        </svg></div><div><strong>Pulse API</strong><span>AI API 测试工作台</span></div><button class="sidebar-toggle" :data-tip="sidebarCollapsed ? '展开侧边栏' : '收起侧边栏'" @click="toggleSidebar">☰</button></div><div class="top-center">
          <div class="project-picker">
            <button class="project-trigger" @click="toggleProjectPicker" :class="{open:showProjectPicker}" data-tip="选择项目">
              <svg class="project-trigger-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" width="16" height="16" aria-hidden="true"><path d="M3 7.5A1.5 1.5 0 0 1 4.5 6h4l2 2.5h9A1.5 1.5 0 0 1 21 10v7.5a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 17.5Z"/></svg>
              <span class="project-trigger-label">项目</span>
              <b>{{ currentProject?.name || '未选择项目' }}</b>
              <i class="caret"></i>
            </button>
            <div v-show="showProjectPicker" class="project-pop">
              <div class="project-pop-head">选择项目</div>
              <button v-for="project in projects" :key="project.id" type="button" class="project-card" :class="{active:project.id===selectedProjectId}" @click.stop="pickProject(project)">
                <b>{{ project.name }} <em v-if="project.id===selectedProjectId">当前项目</em></b>
                <span v-if="project.summary">{{ project.summary }}</span>
                <span v-show="projectWorkspaceName(project.id)" class="project-workspace">已关联本地文件夹 · {{ projectWorkspaceName(project.id) }}</span>
                <small><i>{{ projectEndpointsCount(project.id) }}</i> 个接口<i>{{ projectRunsCount(project.id) }}</i> 次执行</small>
              </button>
              <div v-if="!projects.length" class="project-pop-empty">还没有项目</div>
              <button class="project-pop-local" v-if="isAdmin" @click="openProjectFromLocalFolder">从本地文件夹创建 / 关联</button>
              <button class="project-pop-new" @click="showProjectPicker=false;showProjectModal=true">新建空白项目</button>
            </div>
          </div>

        </div><div class="top-right"><span class="online"><i></i>服务正常</span><button class="nav-btn" :class="{active:page==='tester'}" v-if="canExecute" data-tip="测试台" @click="page='tester';closeSidebar()"><svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round" width="20" height="20" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 9h18M7.5 13l2 2 3.5-4"/></svg><span>测试台</span></button><button class="nav-btn" :class="{active:page==='knowledge'}" v-if="canExecute" data-tip="知识库" @click="page='knowledge';closeSidebar()"><svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round" width="20" height="20" aria-hidden="true"><path d="M4 5.5A1.5 1.5 0 0 1 5.5 4H10a2 2 0 0 1 2 2v13a2 2 0 0 0-2-2H5.5A1.5 1.5 0 0 1 4 15.5Z"/><path d="M20 5.5A1.5 1.5 0 0 0 18.5 4H14a2 2 0 0 0-2 2v13a2 2 0 0 1 2-2h4.5a1.5 1.5 0 0 0 1.5-1.5Z"/></svg><span>知识库</span></button><button class="nav-btn" :class="{active:page==='evaluation'||page==='projectCheck'}" data-tip="测试与测评" aria-label="测试与测评" @click="openTestingWorkspace"><svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round" width="20" height="20" aria-hidden="true"><path d="M4 19V5M4 19h16"/><path d="M8 16v-4M12 16V8M16 16v-6M20 16v-9"/></svg><span>测试与测评</span></button><button class="nav-btn" :class="{active:page==='history'}" data-tip="测试记录" aria-label="测试记录" @click="page='history';closeSidebar()"><svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round" width="20" height="20" aria-hidden="true"><circle cx="12" cy="12" r="8.5"/><path d="M12 7.5V12l3 2"/></svg><span>测试记录</span></button><button class="nav-btn" :class="{active:page==='settings'}" v-if="canExecute" data-tip="配置" @click="page='settings';closeSidebar()"><svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round" width="20" height="20" aria-hidden="true"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.6 1.6 0 0 0 .32 1.77l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.6 1.6 0 0 0-1.77-.32 1.6 1.6 0 0 0-1 1.47V21a2 2 0 1 1-4 0v-.1A1.6 1.6 0 0 0 9 19.4a1.6 1.6 0 0 0-1.77.32l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.6 1.6 0 0 0 4.6 15a1.6 1.6 0 0 0-1.47-1H3a2 2 0 1 1 0-4h.1A1.6 1.6 0 0 0 4.6 9a1.6 1.6 0 0 0-.32-1.77l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.6 1.6 0 0 0 9 4.6a1.6 1.6 0 0 0 1-1.47V3a2 2 0 1 1 4 0v.1a1.6 1.6 0 0 0 1 1.47 1.6 1.6 0 0 0 1.77-.32l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.6 1.6 0 0 0 19.4 9v0a1.6 1.6 0 0 0 1.47 1H21a2 2 0 1 1 0 4h-.1a1.6 1.6 0 0 0-1.5 1Z"/></svg><span>配置</span></button><button v-if="isAdmin" class="nav-btn users-nav-btn" :class="{active:page==='users'}" data-tip="用户与权限" aria-label="用户与权限" @click="page='users';closeSidebar()"><svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" width="20" height="20" aria-hidden="true"><circle cx="9" cy="8" r="3"/><path d="M3 20v-2a6 6 0 0 1 12 0v2M17 5a3 3 0 0 1 0 6m2 9v-2a6 6 0 0 0-2-4"/></svg><span>用户与权限</span></button><SessionControl /></div></header>
    <section v-if="!projects.length && page!=='users' && page!=='settings'" class="list-page"><h1>尚未分配项目</h1><p>请联系管理员分配项目访问权限；已有测试记录会保留。</p></section><main v-else-if="page==='tester'" class="tester-page">
      <aside class="endpoint-panel"><div class="panel-heading"><div><span class="eyebrow">ENDPOINTS</span><h2>接口</h2></div><button class="add-btn" title="新增接口" @click="showEndpointModal=true">＋</button></div><div class="endpoint-search">⌕ <input v-model="searchText" placeholder="搜索接口" /></div><div class="endpoint-list"><button v-for="item in filteredEndpoints" :key="item.id" class="endpoint-item" :class="{selected:item.id===selectedApiId}" @click="selectEndpoint(item)"><span class="method" :class="item.method.toLowerCase()">{{ item.method }}</span><span class="endpoint-name"><b>{{ item.name }}</b><small>{{ item.url }}</small></span><span class="chevron">›</span></button><div v-if="!filteredEndpoints.length" class="empty-small">没有匹配的接口</div></div><div class="endpoint-footer"><label class="import-link">导入 OpenAPI <input type="file" accept=".json,application/json" @change="selectOpenAPI" /></label><button class="import-btn" @click="importOpenAPI">导入</button></div><div class="side-tip"><b>怎么开始？</b><span>选择接口 → 让 AI 补齐 → 发送请求</span></div></aside>
      <section class="request-panel"><div class="section-title"><div><span class="eyebrow">REQUEST BUILDER</span><h1>测试请求</h1><p>{{ selectedEndpoint?.name || '请选择一个接口' }} <span v-if="selectedEndpoint">· {{ selectedEndpoint.kind }}</span><span v-if="selectedEndpoint?.kind==='LLM'" class="stream-tag">SSE / 模型指标</span></p></div><button class="ghost-btn" @click="request=selectedEndpoint ? {method:selectedEndpoint.method,url:selectedEndpoint.url,headers:selectedEndpoint.headers||{},query:selectedEndpoint.query||{},body:selectedEndpoint.body||''} : emptyRequest()">恢复接口默认值</button></div><div class="assistant-entry"><div class="assistant-icon" aria-hidden="true"><svg viewBox="0 0 64 64" fill="none">
          <defs>
            <linearGradient id="eiShell" x1="12" y1="8" x2="54" y2="56" gradientUnits="userSpaceOnUse">
              <stop stop-color="#FFFFFF"/><stop offset=".52" stop-color="#F0FAF8"/><stop offset="1" stop-color="#D8EFEB"/>
            </linearGradient>
            <linearGradient id="eiVisor" x1="18" y1="22" x2="46" y2="46" gradientUnits="userSpaceOnUse">
              <stop stop-color="#68CBBE"/><stop offset=".55" stop-color="#39A99A"/><stop offset="1" stop-color="#1F7E75"/>
            </linearGradient>
            <linearGradient id="eiEar" x1="4" y1="28" x2="12" y2="40" gradientUnits="userSpaceOnUse">
              <stop stop-color="#9DE0D4"/><stop offset="1" stop-color="#5DBDAC"/>
            </linearGradient>
            <radialGradient id="eiGlow" cx=".5" cy=".5" r=".5">
              <stop stop-color="#83D3C5" stop-opacity=".72"/><stop offset="1" stop-color="#83D3C5" stop-opacity="0"/>
            </radialGradient>
          </defs>
          <ellipse cx="6.5" cy="34" rx="4.6" ry="7.4" fill="url(#eiEar)"/>
          <ellipse cx="57.5" cy="34" rx="4.6" ry="7.4" fill="url(#eiEar)"/>
          <rect x="8" y="10" width="48" height="44" rx="19" fill="url(#eiShell)"/>
          <rect x="12.5" y="18" width="39" height="29" rx="13" fill="url(#eiVisor)"/>
          <rect x="12.5" y="18" width="39" height="29" rx="13" fill="none" stroke="#BFE7DF" stroke-width="1.6"/>
          <ellipse cx="22" cy="31" rx="4" ry="6.4" fill="#FFFFFF"/>
          <ellipse cx="42" cy="31" rx="4" ry="6.4" fill="#FFFFFF"/>
          <path d="M26.5 39.5c1.9 2.6 4.1 3.9 5.5 3.9s3.6-1.3 5.5-3.9" stroke="#FFFFFF" stroke-width="2.4" stroke-linecap="round"/>
          <ellipse cx="32" cy="32" rx="15" ry="11" fill="url(#eiGlow)" opacity=".35"/>
        </svg></div><div class="assistant-copy"><b>不确定怎么填？让 AI 补齐参数</b><span>告诉它你想测试什么，它会生成 URL、请求体和断言草稿。</span></div><button class="assistant-preview" @click="openAssistant">{{ prompt }}</button><button class="ai-btn" :disabled="loading" @click="openAssistant">AI 补齐参数</button></div><div v-if="aiSummary" class="ai-result"><span>✓</span><div><b>{{ aiSummary }}</b><small v-if="aiReasoning.length">已识别测试目标并生成可编辑请求</small></div><details v-if="aiReasoning.length"><summary>生成依据</summary><span v-for="item in aiReasoning" :key="item">{{ item }}</span></details></div><div class="request-card"><div class="request-line"><select v-model="request.method" class="method-select"><option>GET</option><option>POST</option><option>PUT</option><option>DELETE</option><option>PATCH</option></select><input v-model="request.url" class="url-input" placeholder="输入接口 URL，例如 /api/demo/echo" /><button class="send-btn" :disabled="loading" @click="sendRequest(true)">▶ 发送并保存</button><button class="send-only" :disabled="loading" @click="sendRequest(false)">仅发送</button></div><div class="batch-line"><button class="batch-btn" :disabled="loading" @click="runBatch">批量测试当前项目接口</button><label>并发 <select v-model.number="batchConcurrency"><option :value="2">2</option><option :value="4">4</option><option :value="8">8</option></select></label><span v-if="batchResult">最近批量：{{ batchResult.count }} 成功记录</span></div><div class="editor-grid"><div class="editor-block"><div class="editor-label"><b>Headers</b><span>敏感值默认隐藏</span><button class="mini-add" @click="showSecrets=!showSecrets">{{ showSecrets ? '隐藏密钥' : '显示密钥' }}</button></div><textarea :value="headerEditorText" :readonly="!showSecrets" @input="updateHeaders" rows="6" placeholder="Authorization: Bearer xxx\nContent-Type: application/json"></textarea></div><div class="editor-block"><div class="editor-label"><b>Body</b><span>JSON 或文本</span></div><textarea v-model="request.body" rows="6" placeholder="{&quot;message&quot;:&quot;hello&quot;}"></textarea></div></div><details class="advanced-row"><summary>查询参数</summary><textarea :value="queryEditor" @input="updateQuery" rows="3" placeholder="page: 1\nkeyword: demo"></textarea></details><div class="assertion-block"><div class="editor-label"><b>发送后检查</b><span>基础断言</span><button class="mini-add" @click="assertions.push({type:'status',path:'',expected:'200'})">＋ 添加</button></div><div v-for="(item, index) in assertions" :key="index" class="assertion-row"><div class="assertion-field"><span class="assertion-field-label">断言类型</span><select v-model="item.type"><option value="status">状态码等于</option><option value="contains">响应包含文本</option><option value="json_field">JSON 存在字段</option><option value="json_contract">契约字段全部存在</option><option value="json_schema">JSON Schema 校验</option></select></div><div class="assertion-field"><span class="assertion-field-label">字段路径</span><input v-model="item.path" :placeholder="item.type==='json_schema' ? '根路径，可留空' : '多个字段用逗号分隔'" /></div><div class="assertion-field"><span class="assertion-field-label">期望值</span><input v-model="item.expected" :placeholder="item.type==='json_schema' ? 'JSON Schema 定义' : '例如 200'" /></div><button class="remove-btn" @click="assertions.splice(index,1)">×</button></div></div></div></section>
      <aside class="response-panel"><div class="response-heading"><div><span class="eyebrow">RESPONSE</span><h2>响应结果</h2></div><span v-if="latest" class="result-pill" :class="latest.success?'ok':'fail'">{{ latest.success ? '通过' : '失败' }}</span></div><div v-if="latest" class="response-content"><div class="status-card" :class="latest.success?'status-good':'status-bad'"><div><b>{{ latest.success ? '请求成功' : (latest.errorCategory || '请求失败') }}</b><span>{{ latest.errorMessage || '状态码和断言均符合预期' }}</span></div><strong>{{ latest.statusCode || 'ERR' }}</strong></div><div class="metric-row"><div><span>耗时</span><b>{{ latest.durationMs }}<i>ms</i></b></div><div><span>TTFT / 首字节</span><b>{{ latest.firstByteMs }}<i>ms</i></b></div><div><span>断言</span><b>{{ latest.assertionPass }}/{{ latest.assertionTotal }}</b></div></div><div v-if="latest.stream || latest.outputTokens" class="token-row"><span>输入 {{ latest.inputTokens }} tokens</span><span>输出 {{ latest.outputTokens }} tokens</span><span>生成 {{ latest.tokensPerSec.toFixed(1) }} tok/s</span></div><div class="diagnosis" :class="diagnosis(latest).tone"><span class="diagnosis-icon">{{ latest.success ? '✓' : '!' }}</span><div><b>{{ diagnosis(latest).title }}</b><p>{{ diagnosis(latest).text }}</p></div></div><div class="response-tabs"><details open><summary>Response Body</summary><div class="response-body"><div class="editor-label"><b>{{ latest.stream ? 'SSE 流式响应' : 'JSON / 文本响应' }}</b><button class="copy-btn" @click="copyResponse(latest)">复制</button></div><pre>{{ responsePretty || '没有响应内容' }}</pre></div></details><details><summary>Response Headers</summary><pre class="headers-preview">{{ responseHeadersText || '没有响应头' }}</pre></details></div><button class="replay-btn" @click="rerun(latest)">↻ 重新发送</button></div><div v-else class="response-empty"><div class="empty-icon">↗</div><b>等待发送请求</b><span>结果、错误原因和响应内容<br/>会显示在这里</span></div></aside>
    </main>
    <TestRecordsPage v-else-if="page==='history'" :runs="runs" :cases="cases" :detail="regressionDetail" :busy="loading" v-model:active-tab="recordsTab" @create="page='tester';closeSidebar()" @view="viewRecord" @replay="rerun" @compare="inspectRegression" @replay-compare="rerun($event,true)" @edit="loadCase(cases.find(item=>item.id===$event))" />
    <KeepAlive v-else-if="page==='projectCheck'||page==='evaluation'">
      <ProjectTaskPage v-if="page==='projectCheck'" :project-id="selectedProjectId" :project-name="currentProject?.name || '当前项目'" :request="api" :initial-goal="projectTaskGoal" @imported="refresh">
        <template #workspace-nav><TestingWorkspaceNav :active="page" @select="page=$event" /></template>
        <template #manual-chain><ManualChainWorkspace :steps="chainSteps" :endpoints="endpoints" v-model:variables="chainVariablesText" :result="chainResult" :busy="loading" @add="addChainStep" @remove="removeChainStep" @sync="syncChainStep" @run="runChain" /></template>
      </ProjectTaskPage>
      <BusinessEvaluationPage v-else :project-id="selectedProjectId" :profiles="modelProfiles" :request="api" @add-model="showModelModal=true">
        <template #workspace-nav><TestingWorkspaceNav :active="page" @select="page=$event" /></template>
      </BusinessEvaluationPage>
    </KeepAlive>
    <UserManagementPage v-else-if="page==='users' && isAdmin" :projects="projects" :request="api" /><KnowledgePage v-else-if="page==='knowledge'" :key="selectedProjectId" :project-id="selectedProjectId" :documents="documents" :endpoints="endpoints" :profiles="modelProfiles" :initial-cases="businessCases" :profile-id="assistantProfileId" :request="api" @refresh="refresh" @configure="loadCase($event)" /><section v-else class="list-page settings-page">
      <div class="page-title">
        <div><h1>模型与环境配置</h1><p>设置 AI 助手的模型连接、验证真实可用性，并管理请求变量。</p></div>
        <span class="config-state" :class="{ configured: llmVerified, pending: llmConfigured && !llmVerified }">{{ assistantConnectionText }}</span>
      </div>
      <div class="settings-grid assistant-settings-grid">
        <div class="settings-card assistant-connection-card">
          <div class="card-title"><div><h2>AI 助手模型</h2></div><span class="provider-dot">{{ llmSourceProvider || '未选择' }}</span></div>
          <div class="connection-summary" :class="{ verified: llmVerified, pending: llmConfigured && !llmVerified }">
            <b>{{ llmConfigured ? (llmSourceProfileName || llmForm.model) : '尚未选择模型' }}</b>
            <span>{{ assistantConnectionHint }}</span>
          </div>
          <div v-if="isAdmin" class="assistant-model-picker">
            <div class="assistant-model-picker-head"><span>助手使用的模型</span><em :class="llmVerified ? 'is-verified' : 'is-pending'">{{ llmVerified ? '已验证' : '需验证' }}</em></div>
            <label class="assistant-model-select"><select :value="llmSourceProfileId" @change="selectAssistantModel" aria-label="选择 AI 助手模型"><option :value="0" disabled>从模型池选择已配置模型</option><option v-for="item in modelProfiles" :key="item.id" :value="item.id" :disabled="!item.verified">{{ item.name }} · {{ item.model }}{{ item.verified ? '（已验证）' : '（请先测试）' }}</option></select></label>
            <span class="assistant-model-picker-help">只有通过连接测试的模型可被 AI 助手使用。</span>
          </div>
          <div v-if="isAdmin" class="assistant-action-row"><button class="primary-btn" @click="openModelModal">添加模型</button><button class="send-only verify-btn" :disabled="!llmSourceProfileId" @click="verifyAssistantModel">测试当前模型</button></div>
          <p class="settings-tip">需要修改地址、Key、部署名或推理策略？请到下方“当前项目模型池”打开详情。</p>
        </div>
        <div class="settings-card environment-card">
          <div class="card-title"><div><h2>环境变量</h2></div><span class="provider-dot">本地浏览器保存</span></div>
          <label>变量（每行 key: value）<textarea v-model="environmentText" rows="9" placeholder="baseUrl: https://test.example.com\ntoken: your-token"></textarea></label>
          <button class="primary-btn" @click="saveEnvironment">保存环境变量</button>
          <p class="settings-tip">请求 URL、Headers、Body 中可以使用 <code v-pre>{{baseUrl}}</code>、<code v-pre>{{token}}</code>，发送前自动替换。</p>
        </div>
        <div class="settings-card model-selector-card">
          <div class="card-title"><div><h2>当前项目模型池</h2></div><button v-if="isAdmin" class="mini-add" @click="openModelModal">＋ 添加模型</button></div>
          <p class="settings-tip">每个模型独立保存地址、Key、部署名、鉴权、推理策略和真实连通状态。未验证的模型不能设为助手。</p>
          <div v-if="modelProfiles.length" class="assistant-model-list">
            <div v-for="item in modelProfiles" :key="item.id" class="assistant-model-row" :class="{ active: llmSourceProfileId === item.id }">
              <div class="assistant-model-summary"><b>{{ item.name }}</b><small>{{ item.provider }} · {{ item.apiKeyMasked || '已保存密钥' }}</small></div>
              <div class="model-profile-facts"><span>部署名 <code>{{ item.model }}</code></span><span>推理 {{ item.reasoningMode === 'enabled' ? '深度思考 · ' + item.reasoningEffort : item.reasoningMode === 'off' ? '关闭' : '自动' }}</span><span :class="item.verified ? 'profile-verified' : 'profile-unverified'">{{ item.verified ? '● 已验证' : '● 未验证' }}</span></div>
              <div v-if="isAdmin" class="model-profile-actions"><button class="send-only" @click="openModelDetail(item)">详情</button><button class="send-only" @click="verifyModelProfile(item)">测试连接</button><button class="link-btn" :disabled="(llmSourceProfileId === item.id && item.verified) || !item.verified" @click="useProfileAsAssistant(item)">{{ llmSourceProfileId === item.id && item.verified ? '正在使用' : item.verified ? '设为助手' : '请先验证' }}</button><button class="send-only model-delete-btn" @click="deleteModelProfile(item)">删除</button></div>
            </div>
          </div>
          <div v-else class="model-pool-inline-empty"><b>还没有模型</b><span>点击“添加模型”，保存后它会同时出现在模型评测页和这里。</span></div>
        </div>
      </div>
    </section>
    <ReadOnlyRunDialog v-if="readonlyRun" :run="readonlyRun" @close="readonlyRun=null" /><button v-if="canExecute" class="assistant-fab" :class="{dragging:assistantFabDragging}" :style="assistantFabStyle" title="拖动 AI 助手；点击打开" @pointerdown="startAssistantFabDrag" @click="clickAssistantFab"><span class="fab-face robot-fab" aria-hidden="true"><svg viewBox="0 0 64 64" fill="none">
        <defs>
          <linearGradient id="rbShell" x1="12" y1="8" x2="54" y2="56" gradientUnits="userSpaceOnUse">
            <stop stop-color="#FFFFFF"/><stop offset=".52" stop-color="#F0FAF8"/><stop offset="1" stop-color="#D8EFEB"/>
          </linearGradient>
          <linearGradient id="rbVisor" x1="18" y1="22" x2="46" y2="46" gradientUnits="userSpaceOnUse">
            <stop stop-color="#68CBBE"/><stop offset=".55" stop-color="#39A99A"/><stop offset="1" stop-color="#1F7E75"/>
          </linearGradient>
          <linearGradient id="rbEar" x1="4" y1="28" x2="12" y2="40" gradientUnits="userSpaceOnUse">
            <stop stop-color="#9DE0D4"/><stop offset="1" stop-color="#5DBDAC"/>
          </linearGradient>
          <radialGradient id="rbGlow" cx=".5" cy=".5" r=".5">
            <stop stop-color="#83D3C5" stop-opacity=".72"/><stop offset="1" stop-color="#83D3C5" stop-opacity="0"/>
          </radialGradient>
        </defs>
        <ellipse cx="6.5" cy="34" rx="4.6" ry="7.4" fill="url(#rbEar)"/>
        <ellipse cx="57.5" cy="34" rx="4.6" ry="7.4" fill="url(#rbEar)"/>
        <rect x="8" y="10" width="48" height="44" rx="19" fill="url(#rbShell)"/>
        <rect x="12.5" y="18" width="39" height="29" rx="13" fill="url(#rbVisor)"/>
        <rect x="12.5" y="18" width="39" height="29" rx="13" fill="none" stroke="#BFE7DF" stroke-width="1.6"/>
        <ellipse cx="22" cy="31" rx="4" ry="6.4" fill="#FFFFFF"/>
        <ellipse cx="42" cy="31" rx="4" ry="6.4" fill="#FFFFFF"/>
        <path d="M26.5 39.5c1.9 2.6 4.1 3.9 5.5 3.9s3.6-1.3 5.5-3.9" stroke="#FFFFFF" stroke-width="2.4" stroke-linecap="round"/>
        <ellipse cx="32" cy="32" rx="15" ry="11" fill="url(#rbGlow)" opacity=".35"/>
      </svg></span><em>AI 助手</em></button>
    <div v-if="notice" class="toast-v2">{{ notice }}</div>
    <div v-if="showEndpointModal" class="modal-mask" @click.self="showEndpointModal=false"><div class="modal-card"><div class="modal-head"><button class="close-btn close-back" @click="showEndpointModal=false" title="返回" aria-label="返回"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" width="17" height="17" aria-hidden="true"><path d="M15 5l-7 7 7 7"/></svg></button><div><span class="eyebrow">NEW ENDPOINT</span><h2>添加接口</h2></div></div><label>接口名称<input v-model="endpointForm.name" placeholder="例如：用户登录" /></label><div class="form-two"><label>方法<select v-model="endpointForm.method"><option>GET</option><option>POST</option><option>PUT</option><option>DELETE</option></select></label><label>类型<select v-model="endpointForm.kind"><option>REST</option><option>LLM</option><option>Multimodal</option></select></label></div><label>URL<input v-model="endpointForm.url" placeholder="https://example.com/api/users 或 /api/demo/echo" /></label><label>Headers<textarea v-model="endpointForm.headers" rows="3" placeholder="Authorization: Bearer xxx"></textarea></label><label>Body<textarea v-model="endpointForm.body" rows="4" placeholder="{&quot;name&quot;:&quot;demo&quot;}"></textarea></label><div class="modal-actions"><label class="import-link">或导入 OpenAPI <input type="file" accept=".json,application/json" @change="selectOpenAPI" /></label><button class="primary-btn" @click="createEndpoint">保存接口</button></div></div></div>
    <div v-if="showProjectModal" class="modal-mask" @click.self="showProjectModal=false"><div class="modal-card small-modal"><div class="modal-head"><button class="close-btn close-back" @click="showProjectModal=false" title="返回" aria-label="返回"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" width="17" height="17" aria-hidden="true"><path d="M15 5l-7 7 7 7"/></svg></button><div><span class="eyebrow">NEW PROJECT</span><h2>新建项目</h2></div></div><label>项目名称<input v-model="projectForm.name" placeholder="例如：订单服务测试" /></label><label>项目说明<input v-model="projectForm.summary" placeholder="可选" /></label><section class="project-folder-field"><div><b>本地项目文件夹</b><span>{{ pendingProjectFolderName ? '已选择：' + pendingProjectFolderName : '可选，用于关联你的源码和测试文档' }}</span></div><button class="send-only" @click="chooseFolderForNewProject">选择本地文件夹</button></section><p class="project-folder-note">接口、测试记录仍安全保存在平台数据库；本地文件夹仅作为该项目的工作区关联。</p><button class="primary-btn full-btn" @click="createProject">创建项目</button></div></div>
    <div v-if="showAssistant" class="assistant-mask" @click.self="closeAssistant"><section class="assistant-window assistant-chat-page" @wheel.capture="forwardAssistantWheel"><header class="assistant-window-head"><button class="close-btn close-back" @click="closeAssistant" title="返回" aria-label="返回"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" width="17" height="17" aria-hidden="true"><path d="M15 5l-7 7 7 7"/></svg></button><svg class="robot-avatar" viewBox="0 0 64 64" fill="none" aria-hidden="true">
          <defs>
            <linearGradient id="raShell" x1="12" y1="8" x2="54" y2="56" gradientUnits="userSpaceOnUse">
              <stop stop-color="#FFFFFF"/><stop offset=".52" stop-color="#F0FAF8"/><stop offset="1" stop-color="#D8EFEB"/>
            </linearGradient>
            <linearGradient id="raVisor" x1="18" y1="22" x2="46" y2="46" gradientUnits="userSpaceOnUse">
              <stop stop-color="#68CBBE"/><stop offset=".55" stop-color="#39A99A"/><stop offset="1" stop-color="#1F7E75"/>
            </linearGradient>
            <linearGradient id="raEar" x1="4" y1="28" x2="12" y2="40" gradientUnits="userSpaceOnUse">
              <stop stop-color="#9DE0D4"/><stop offset="1" stop-color="#5DBDAC"/>
            </linearGradient>
            <radialGradient id="raGlow" cx=".5" cy=".5" r=".5">
              <stop stop-color="#83D3C5" stop-opacity=".72"/><stop offset="1" stop-color="#83D3C5" stop-opacity="0"/>
            </radialGradient>
          </defs>
          <ellipse cx="6.5" cy="34" rx="4.6" ry="7.4" fill="url(#raEar)"/>
          <ellipse cx="57.5" cy="34" rx="4.6" ry="7.4" fill="url(#raEar)"/>
          <rect x="8" y="10" width="48" height="44" rx="19" fill="url(#raShell)"/>
          <rect x="12.5" y="18" width="39" height="29" rx="13" fill="url(#raVisor)"/>
          <rect x="12.5" y="18" width="39" height="29" rx="13" fill="none" stroke="#BFE7DF" stroke-width="1.6"/>
          <ellipse cx="22" cy="31" rx="4" ry="6.4" fill="#FFFFFF"/>
          <ellipse cx="42" cy="31" rx="4" ry="6.4" fill="#FFFFFF"/>
          <path d="M26.5 39.5c1.9 2.6 4.1 3.9 5.5 3.9s3.6-1.3 5.5-3.9" stroke="#FFFFFF" stroke-width="2.4" stroke-linecap="round"/>
          <ellipse cx="32" cy="32" rx="15" ry="11" fill="url(#raGlow)" opacity=".35"/>
        </svg><div><h2>API 测试助手</h2><p>{{ assistantScopeApiId ? selectedEndpoint?.name : currentProject?.name || '当前项目' }}</p></div></header><div class="assistant-messages"><div v-for="(message, index) in assistantMessages" :key="index" class="assistant-message" :class="message.role"><span v-if="message.role === 'assistant'" class="msg-avatar" aria-hidden="true"><svg viewBox="0 0 64 64" fill="none"><defs><linearGradient id="maShell" x1="12" y1="8" x2="54" y2="56" gradientUnits="userSpaceOnUse"><stop stop-color="#FFFFFF"/><stop offset=".52" stop-color="#F2F0FE"/><stop offset="1" stop-color="#DED9FA"/></linearGradient><linearGradient id="maVisor" x1="18" y1="22" x2="46" y2="46" gradientUnits="userSpaceOnUse"><stop stop-color="#5082FA"/><stop offset=".55" stop-color="#3E6FF0"/><stop offset="1" stop-color="#8C64DC"/></linearGradient><linearGradient id="maEar" x1="4" y1="28" x2="12" y2="40" gradientUnits="userSpaceOnUse"><stop stop-color="#B98DF0"/><stop offset="1" stop-color="#8A6FE0"/></linearGradient></defs><ellipse cx="6.5" cy="34" rx="4.6" ry="7.4" fill="url(#maEar)"/><ellipse cx="57.5" cy="34" rx="4.6" ry="7.4" fill="url(#maEar)"/><rect x="8" y="10" width="48" height="44" rx="19" fill="url(#maShell)"/><rect x="12.5" y="18" width="39" height="29" rx="13" fill="url(#maVisor)"/><rect x="12.5" y="18" width="39" height="29" rx="13" fill="none" stroke="#F7C9E8" stroke-width="1.6"/><ellipse cx="22" cy="31" rx="4" ry="6.4" fill="#FFFFFF"/><ellipse cx="42" cy="31" rx="4" ry="6.4" fill="#FFFFFF"/><path d="M26.5 39.5c1.9 2.6 4.1 3.9 5.5 3.9s3.6-1.3 5.5-3.9" stroke="#FFFFFF" stroke-width="2.4" stroke-linecap="round"/></svg></span><span v-else class="msg-avatar msg-avatar-user">{{ message.role === 'assistant' ? 'AI' : '你' }}</span><p>{{ message.text }}</p></div><div v-if="assistantPlan" class="assistant-plan-card"><div class="draft-card-title"><b>Agent 执行计划</b><span>{{ assistantPlan.mode }}</span></div><p class="assistant-plan-summary">{{ assistantPlan.summary }}</p><div v-for="step in assistantPlan.steps" :key="step.order" class="assistant-plan-step"><i>{{ step.order }}</i><div><b>{{ step.title }}</b><small>{{ step.description }}</small></div><em v-if="step.requiresConfirmation">需确认</em></div></div><div v-if="assistantEndpointDraft" class="assistant-draft-card endpoint-draft-card"><div class="draft-card-title"><b>已生成接口配置草稿</b><span>确认后才会加入项目</span></div><div class="endpoint-draft-grid"><span>名称</span><b>{{ assistantEndpointDraft.name }}</b><span>方法</span><code>{{ assistantEndpointDraft.method }}</code><span>URL</span><b>{{ assistantEndpointDraft.url }}</b><span>类型</span><b>{{ assistantEndpointDraft.kind }}</b></div><div class="draft-block"><small>Headers</small><pre>{{ formatPairs(assistantEndpointDraft.headers || {}) || '无' }}</pre></div><div class="draft-block"><small>Body</small><pre>{{ assistantEndpointDraft.body || '无请求体' }}</pre></div></div><div v-else-if="aiSummary" class="assistant-draft-card"><div class="draft-card-title"><b>已生成请求草稿</b><span>可回到测试台继续编辑</span></div><div class="draft-preview-line"><code>{{ request.method }}</code><span>{{ request.url }}</span></div><pre>{{ request.body || '无请求体' }}</pre><div class="draft-meta"><span>断言 {{ assertions.length }} 条</span><span v-if="aiReasoning.length">已生成依据</span></div></div><ProjectTestAssistant :draft="assistantProjectCheckDraft" :result="assistantProjectCheck"
      :reports="projectCheckReports" :loading="loading" v-model:review-open="assistantReviewOpen"
      @confirm="confirmAssistantProjectCheck" @download="downloadProjectCheckReport"
      @review="openManualReview" @restore="restoreProjectCheckReport" /><div v-if="businessCases.length" class="assistant-draft-card assistant-business-cases"><div class="draft-card-title"><b>业务规则测试用例</b><span>{{ businessCases.length }} 条</span></div><div v-for="item in businessCases" :key="item.name" class="assistant-case-row"><b>{{ item.name }}</b><span>{{ item.scenario }} · 预期 {{ item.assertions[0]?.expected || "待确认" }}</span></div></div></div><AssistantComposer
        v-model="assistantInput" :endpoints="endpoints" :profiles="modelProfiles"
        v-model:api-id="assistantScopeApiId" v-model:profile-id="assistantProfileId"
        v-model:permission="assistantPermission" :can-auto="canAuto" :read-only="readOnly"
        :default-model="llmSourceProfileName || llmForm.model" :busy="loading"
        :document-name="assistantDocumentName" :document-status="assistantDocumentStatus"
        @permission-change="setAssistantPermission" @submit="askAssistant"
        @suggestion="useAssistantSuggestion" @project-test="openProjectTask"
        @upload="selectAssistantDocument" @clear-document="clearAssistantDocument"
      /><footer v-if="aiSummary" class="assistant-window-foot"><span>{{ assistantPermission === 'project_full' ? '项目最高权限：按明确任务自动执行已授权操作' : '默认权限：核对草稿后，确认执行或修改' }}</span><div><button class="send-only" @click="closeAssistant">继续编辑</button><button v-if="aiSummary" class="primary-btn" @click="applyAssistantDraft">{{ assistantEndpointDraft ? '添加到接口列表' : '应用到测试台' }}</button></div></footer></section></div>
    <div v-if="showModelModal" class="modal-mask" @click.self="showModelModal=false"><div class="modal-card model-modal"><div class="modal-head"><button class="close-btn close-back" @click="showModelModal=false" title="返回" aria-label="返回"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" width="17" height="17" aria-hidden="true"><path d="M15 5l-7 7 7 7"/></svg></button><div><span class="eyebrow">EVALUATION MODEL</span><h2>添加评测模型</h2><p class="modal-help">填写真实模型连接后，模型才会出现在模型池中。</p></div></div><label>模型名称<input v-model="modelForm.name" placeholder="例如：DeepSeek 业务评测" /></label><div class="form-two"><label>提供商<select v-model="modelForm.provider" @change="applyModelPreset"><option>DeepSeek</option><option>OpenAI</option><option>Azure OpenAI</option><option>OpenAI 兼容</option></select></label><label>模型 / 部署名<input v-model="modelForm.model" placeholder="由服务商提供，例如：deepseek-flash" /></label></div><label>接口地址<input v-model="modelForm.baseUrl" placeholder="https://api.deepseek.com/chat/completions" /></label><div class="form-two"><label>API Key<input v-model="modelForm.apiKey" type="password" placeholder="只保存到当前项目服务端" /></label><label>鉴权 Header<input v-model="modelForm.authHeader" placeholder="Authorization 或 api-key" /></label></div><p class="settings-tip">切换提供商只会更新地址和鉴权 Header，不会替换你填写的模型 / 部署名；推理策略在保存后到模型池设置。</p><div class="modal-actions"><button class="send-only" @click="showModelModal=false">取消</button><button class="primary-btn" @click="createModelProfile">保存并加入模型池</button></div></div></div>
    <div v-if="showModelDetailModal && editingModel" class="modal-mask" @click.self="showModelDetailModal=false"><div class="modal-card model-modal"><div class="modal-head"><button class="close-btn close-back" @click="showModelDetailModal=false" title="返回" aria-label="返回"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" width="17" height="17" aria-hidden="true"><path d="M15 5l-7 7 7 7"/></svg></button><div><h2>模型详情</h2><p class="modal-help">修改后会要求重新测试连接；未验证模型不能设为助手。</p></div></div><label>模型名称<input v-model="editingModel.name" /></label><div class="form-two"><label>提供商<input v-model="editingModel.provider" /></label><label>模型 / 部署名<input v-model="editingModel.model" /></label></div><label>Chat Completions 地址<input v-model="editingModel.baseUrl" /></label><div class="form-two"><label>更新 API Key（不修改则留空）<input v-model="editingModelApiKey" type="password" :placeholder="editingModel.apiKeyMasked || '输入新的 API Key'" /></label><label>鉴权 Header<input v-model="editingModel.authHeader" /></label></div><section class="reasoning-options"><div class="reasoning-options-head"><div><b>推理策略</b><span>仅在服务商明确支持时开启深度思考</span></div><span class="reasoning-state" :class="editingModel.reasoningMode">{{ editingModel.reasoningMode === 'enabled' ? '深度思考已开启' : editingModel.reasoningMode === 'off' ? '已关闭' : '自动判断' }}</span></div><div class="reasoning-controls"><label class="reasoning-control"><span>推理模式</span><select v-model="editingModel.reasoningMode"><option value="auto">自动</option><option value="off">关闭</option><option value="enabled">开启深度思考</option></select></label><label class="reasoning-control"><span>思考强度</span><select v-model="editingModel.reasoningEffort" :disabled="editingModel.reasoningMode !== 'enabled'"><option value="low">low</option><option value="medium">medium</option><option value="high">high</option></select></label></div></section><div class="model-detail-status" :class="editingModel.verified ? 'ok' : 'pending'">{{ editingModel.verified ? '● 最近一次连接测试通过' : '● ' + (editingModel.verificationError || '尚未通过连接测试') }}</div><div class="modal-actions"><button class="send-only" @click="showModelDetailModal=false">关闭</button><button class="send-only" @click="verifyModelProfile(editingModel)">测试连接</button><button class="primary-btn" @click="saveModelDetail">保存配置</button></div></div></div>
  </div>`
    .replace('</button></div><div class="evaluation-grid">', '</button></div></div><div class="evaluation-grid">')
    .replace('保存一次，后面一键重放', '保存测试用例，更新后一键重新执行')
    .replace('接口、模型或 Prompt 修改后，重新执行旧用例，平台会记录状态码、成功状态、耗时和响应结构变化。', '接口、模型或 Prompt 更新后，可一键重新执行保存的用例，并自动对比状态码、耗时和响应结构。')
    .replace('重放并比较 →', '重新执行并比较 →')
    .replace('每次请求都会留下可重放记录。', '每次请求都会保存为可再次执行的记录。')
    .replace('MODEL POOL</span><h2>模型池</h2>', 'MODEL POOL</span><h2>公共模型池</h2>')
    .replace('当前项目模型池', '公共模型池')
    .replace('每个模型独立保存地址、Key、部署名、鉴权、推理策略和真实连通状态。未验证的模型不能设为助手。', '模型池是平台公共资源：任意项目添加、修改或验证后，所有项目都能使用。未验证的模型不能设为助手。')
    .replace('填写真实模型连接后，模型才会出现在模型池中。', '保存后会进入公共模型池，所有项目都能选择它。')
    .replace('只保存到当前项目服务端', '保存到平台公共模型池（所有项目共享）')
    .replace('只有通过连接测试的模型可被 AI 助手使用。', '只有通过连接测试的模型可被 AI 助手使用；切换后对所有项目生效。')
    .replace('需要修改地址、Key、部署名或推理策略？请到下方“当前项目模型池”打开详情。', '需要修改地址、Key、部署名或推理策略？请到下方“公共模型池”打开详情。')
    .replace('修改后会要求重新测试连接；未验证模型不能设为助手。', '修改后会影响所有项目，并要求重新测试连接；未验证模型不能设为助手。')
    .replace('<div v-if="latest.stream || latest.outputTokens" class="token-row"><span>输入 {{ latest.inputTokens }} tokens</span><span>输出 {{ latest.outputTokens }} tokens</span><span>生成 {{ latest.tokensPerSec.toFixed(1) }} tok/s</span></div>', '<div v-if="latest.stream || latest.outputTokens" class="token-row"><span>输入 {{ latest.inputTokens }} tokens</span><span>输出 {{ latest.outputTokens }} tokens</span><span>生成 {{ latest.tokensPerSec.toFixed(1) }} tok/s</span></div><section v-if="responseImagePreview" class="response-image-preview"><div class="response-image-head"><div><b>生成图片</b><span>已从 API 响应自动还原图片</span></div><a :href="responseImagePreview.src" target="_blank" rel="noopener">在新窗口查看</a></div><a class="response-image-canvas" :href="responseImagePreview.src" target="_blank" rel="noopener"><img :src="responseImagePreview.src" alt="API 生成的图片预览" /></a></section>')
    .replace('<button class="send-only" :disabled="loading" @click="sendRequest(false)">仅发送</button></div><div class="batch-line">', '<button class="send-only" :disabled="loading" @click="sendRequest(false)">仅发送</button></div><section v-if="azureImageMode" class="azure-image-helper"><div class="azure-image-helper-copy"><b>Azure 图片生成快捷配置</b><span>资源地址保留在上方；填写一次部署名，平台自动拼接实际请求地址。</span></div><label>部署名<input v-model="azureImageDeployment" placeholder="例如 gpt-image-2" /></label><label>API 版本<input v-model="azureImageApiVersion" placeholder="2025-04-01-preview" /></label><div class="azure-image-preview"><span>实际调用</span><code>{{ azureResolvedURL }}</code></div></section><div class="batch-line">')
    .replace('</section><div class="batch-line">', '</section><section v-if="isLoginEndpoint" class="login-test-helper"><div class="login-test-copy"><b>登录测试配置</b><span>填写专用测试账号后，平台会生成登录请求并检查响应中的 Token。</span></div><label>测试用户名<input v-model="loginUsername" autocomplete="username" placeholder="例如 cloud_test_01" /></label><label>测试密码<input v-model="loginPassword" type="password" autocomplete="current-password" placeholder="输入该测试账号的密码" /></label><div class="login-test-actions"><button class="mini-add" @click="fillLoginRequest">填入登录请求</button><button class="link-btn" @click="prepareRegistrationDraft">没有测试账号？准备注册草稿</button><button v-if="latest && latest.success" class="link-btn" @click="saveLoginToken">保存本次 Token 到环境变量</button></div><p>登录成功后，点击“保存本次 Token 到环境变量”，后续接口请求头使用 <code v-pre>cloud_disk_token: {{cloud_disk_token}}</code>。</p></section><div class="batch-line">')
    .replace('</section><div class="batch-line">', '</section><section v-if="isRegistrationEndpoint" class="login-test-helper registration-test-helper"><div class="login-test-copy"><b>注册测试配置</b><span>这是一次性测试账号。生成请求后发送注册；只有业务 code 为 200 才算注册成功。</span></div><label>测试用户名<input v-model="loginUsername" autocomplete="username" placeholder="例如 cloud_test_01" /></label><label>测试密码<input v-model="loginPassword" type="password" autocomplete="new-password" placeholder="默认 Test123456，可修改" /></label><label>昵称<input v-model="registrationNickname" placeholder="例如 接口测试用户" /></label><label>邮箱<input v-model="registrationEmail" type="email" placeholder="例如 cloud_test_01@example.com" /></label><div class="login-test-actions"><button class="mini-add" @click="fillRegistrationRequest">生成并填入注册请求</button></div><p>注册成功后，重新选择 <b>login</b> 接口；账号和密码会自动保留，接着完成登录测试。</p></section><div class="batch-line">')
    .replace('<div class="batch-line"><button class="batch-btn" :disabled="loading" @click="runBatch">批量测试当前项目接口</button><label>并发 <select v-model.number="batchConcurrency"><option :value="2">2</option><option :value="4">4</option><option :value="8">8</option></select></label><span v-if="batchResult">最近批量：{{ batchResult.count }} 成功记录</span></div>', '<div class="batch-line"><div class="batch-copy"><button class="batch-btn" :disabled="loading" @click="runBatch">批量安全巡检（仅只读）</button><span>仅发送 GET / HEAD / OPTIONS；登录、注册、上传、删除等接口请逐条人工配置。</span></div><label>并发 <select v-model.number="batchConcurrency"><option :value="2">2</option><option :value="4">4</option><option :value="8">8</option></select></label><span v-if="batchResult">最近巡检：{{ batchResult.count }} 条执行记录</span></div>')
    .replace('<option value="json_field">JSON 存在字段</option>', '<option value="json_field">JSON 存在字段</option><option value="json_value">JSON 字段值等于</option>')
    .replace('<button class="assistant-preview" @click="openAssistant">{{ prompt }}</button>', '<button class="assistant-preview" @click="openAssistant">点击描述测试目标，AI 会生成草稿</button>')
    .replace('<small v-if="aiReasoning.length">已识别测试目标并生成可编辑请求</small>', '')
    .replace('<details v-if="aiReasoning.length"><summary>生成依据</summary><span v-for="item in aiReasoning" :key="item">{{ item }}</span></details>', '')
    .replace('<summary>查询参数</summary>', '<summary>查询参数 <span class="query-help">适用于筛选、分页等；不会放进请求体</span></summary>')
    .replace('<button class="ghost-btn" @click="request=selectedEndpoint ? {method:selectedEndpoint.method,url:selectedEndpoint.url,headers:selectedEndpoint.headers||{},query:selectedEndpoint.query||{},body:selectedEndpoint.body||\'\'} : emptyRequest()">恢复接口默认值</button></div><div class="assistant-entry">', '<div class="request-title-actions"><button class="ghost-btn" @click="request=selectedEndpoint ? {method:selectedEndpoint.method,url:selectedEndpoint.url,headers:selectedEndpoint.headers||{},query:selectedEndpoint.query||{},body:selectedEndpoint.body||\'\'} : emptyRequest()">恢复接口默认值</button><button v-if="selectedEndpoint" class="ghost-btn" @click="openEndpointEditor(selectedEndpoint)">编辑接口</button><button v-if="selectedEndpoint" class="ghost-btn ghost-danger" @click="deleteEndpoint(selectedEndpoint)">删除接口</button></div></div><div class="assistant-entry">')
    .replace('<div v-else class="response-empty"><div class="empty-icon">↗</div><b>等待发送请求</b><span>结果、错误原因和响应内容<br/>会显示在这里</span></div>', '<div v-else-if="loading" class="response-empty response-running"><div class="empty-icon">◌</div><b>正在执行请求…</b><span>平台正在等待服务响应<br/>完成后将自动展示状态、断言和结果</span></div><div v-else class="response-empty"><div class="empty-icon">↗</div><b>等待发送请求</b><span>结果、错误原因和响应内容<br/>会显示在这里</span></div>')
    .replace('<div v-if="latest" class="response-content">', '<div v-if="loading" class="response-empty response-running"><div class="empty-icon">◌</div><b>正在执行请求…</b><span>平台正在等待服务响应<br/>完成后将自动展示状态、断言和结果</span></div><div v-else-if="latest" class="response-content">')
    .replace('<h2>添加接口</h2>', '<h2><span v-if="editingEndpointId">编辑</span><span v-else>添加</span>接口</h2>')
    .replace('<button class="primary-btn" @click="createEndpoint">保存接口</button>', '<button class="primary-btn" @click="createEndpoint"><span v-if="editingEndpointId">保存修改</span><span v-else>保存接口</span></button>')
    .replace('<div v-if="showEndpointModal" class="modal-mask"', '<div v-if="pendingEndpointDeletion" class="modal-mask" @click.self="pendingEndpointDeletion=null"><section class="modal-card confirm-modal"><div class="confirm-icon">!</div><div><h2>删除接口？</h2><p>确定删除“{{ pendingEndpointDeletion.name }}”吗？历史测试记录会保留，但不再关联此接口。</p></div><div class="modal-actions"><button class="send-only" @click="pendingEndpointDeletion=null">取消</button><button class="primary-btn confirm-delete" @click="confirmEndpointDeletion">删除接口</button></div></section></div><div v-if="showEndpointModal" class="modal-mask"')
    .replace('<div v-if="pendingEndpointDeletion" class="modal-mask"', '<div v-if="pendingModelDeletion" class="modal-mask" @click.self="pendingModelDeletion=null"><section class="modal-card confirm-modal"><div class="confirm-icon">!</div><div><h2>删除模型？</h2><p>确定删除“{{ pendingModelDeletion.name }}”吗？它会从公共模型池移除，所有项目都不能再使用它；如果它正在作为助手，也会自动解除。</p></div><div class="modal-actions"><button class="send-only" @click="pendingModelDeletion=null">取消</button><button class="primary-btn confirm-delete" @click="confirmModelDeletion">删除模型</button></div></section></div><div v-if="pendingEndpointDeletion" class="modal-mask"')
    .replace("{{ assistantEndpointDraft ? '添加到接口列表' : '应用到测试台' }}", "{{ assistantDeleteDraft ? '确认删除接口' : assistantEndpointDraft ? '添加到接口列表' : '应用到测试台' }}")
    .replace('Agent 执行计划', '任务计划')
    .replace('</em></div></div><div v-if="assistantEndpointDraft" class="assistant-draft-card endpoint-draft-card">', '</em></div></div><section v-if="assistantDecisionDraft && (assistantDecisionDraft.risk || assistantDecisionDraft.missingInformation?.length || assistantDecisionDraft.candidateEndpoints?.length)" class="assistant-decision-card"><div class="assistant-decision-head"><b>执行判断</b><span :class="\'risk-\' + (assistantDecisionDraft.risk?.level || \'low\')">{{ assistantDecisionDraft.risk?.level === \'high\' ? \'高风险\' : assistantDecisionDraft.risk?.level === \'medium\' ? \'需审核\' : \'低风险\' }}</span></div><p v-if="assistantDecisionDraft.risk">{{ assistantDecisionDraft.risk.reasons.join(\'；\') }}</p><div v-if="assistantDecisionDraft.missingInformation?.length" class="assistant-decision-list"><b>还需要的信息</b><span v-for="item in assistantDecisionDraft.missingInformation" :key="item.field">{{ item.question }}</span></div><div v-if="assistantDecisionDraft.candidateEndpoints?.length" class="assistant-decision-list"><b>候选接口</b><button v-for="item in assistantDecisionDraft.candidateEndpoints" :key="item.endpointId" class="assistant-candidate" @click="selectedApiId=item.endpointId; flash(\'已选中 \' + item.name + \'，请补充信息后重新发送\')"><code>{{ item.method }}</code><span>{{ item.name }} · {{ item.url }}</span></button></div></section><div v-if="assistantEndpointDraft" class="assistant-draft-card endpoint-draft-card">')
    .replace('<p class="settings-tip">切换提供商只会更新地址和鉴权 Header，不会替换你填写的模型 / 部署名；推理策略在保存后到模型池设置。</p>', '<p class="settings-tip">可单独保存一个模型；若网关支持 OpenAI 兼容 <code>/models</code>，也可以读取后批量加入公共模型池。</p><section class="model-discovery"><div><b>从网关读取可用模型</b><span>需要先填入地址和 Key；Key 仅用于本次读取及保存配置，不会显示在列表中。</span></div><button class="send-only" :disabled="discoveringModels" @click="discoverAvailableModels">{{ discoveringModels ? \'读取中…\' : \'获取模型列表\' }}</button><div v-if="discoveredModels.length" class="discovered-model-list"><label v-for="item in discoveredModels" :key="item"><input type="checkbox" :checked="selectedDiscoveredModels.includes(item)" @change="toggleDiscoveredModel(item)" /><span>{{ item }}</span></label></div><div v-if="discoveredModels.length" class="model-discovery-actions"><span>已选 {{ selectedDiscoveredModels.length }} / {{ discoveredModels.length }} 个</span><button class="primary-btn" :disabled="importingDiscoveredModels || !selectedDiscoveredModels.length" @click="importAvailableModels">{{ importingDiscoveredModels ? \'加入中…\' : \'加入公共模型池\' }}</button></div></section>'),
}

const app = createApp(AuthGate, { workspace: App })
app.config.errorHandler = (error) => { console.error('[Pulse API V2]', error) }
app.mount('#app')
