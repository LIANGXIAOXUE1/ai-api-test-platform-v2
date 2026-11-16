import { computed, ref, type Ref } from 'vue'
import { createProjectTestApi } from './api'
import { downloadProjectTestReport } from './report'
import type {
  ApiRequest,
  AssistantProjectCheckDraft,
  ProjectCheckPermission,
  ProjectCheckResult,
} from './types'

type Dependencies = {
  request: ApiRequest
  selectedProjectId: Ref<number>
  assistantOpenAPISpec: Ref<Record<string, any> | null>
  assistantPermission: Ref<'default' | 'project_full'>
  loading: Ref<boolean>
  getEnvironment: () => Record<string, string>
  getProjectName: () => string
  loadEndpoints: () => Promise<void>
  refresh: () => Promise<void>
  flash: (message: string) => void
  streamAssistantReply: (message: string) => Promise<void>
}

// All project-test state belongs to this instance, not to a global singleton.
export function useProjectTest(deps: Dependencies) {
  const {
    selectedProjectId,
    assistantOpenAPISpec,
    assistantPermission,
    loading,
    getEnvironment,
    getProjectName,
    loadEndpoints,
    refresh,
    flash,
    streamAssistantReply,
  } = deps
  const projectApi = createProjectTestApi(deps.request)
  const assistantProjectCheck = ref<ProjectCheckResult | null>(null)
  const assistantProjectCheckDraft = ref<AssistantProjectCheckDraft | null>(null)
  const projectCheckReports = ref<ProjectCheckResult[]>([])
  const assistantReviewOpen = ref(false)
  const projectCheckBaseURL = ref('')
  const projectCheckTask = ref('')
  const projectCheckPermission = ref<ProjectCheckPermission>('review')
  const projectCheckFile = ref<File | null>(null)
  const projectCheckLoading = ref(false)
  const projectCheckResult = ref<ProjectCheckResult | null>(null)

  async function loadProjectCheckReports() {
    try {
      const result = await projectApi.listReports(selectedProjectId.value)
      projectCheckReports.value = Array.isArray(result) ? result : []
    } catch {
      projectCheckReports.value = []
    }
  }

  function selectProjectCheckFile(event: Event) {
    projectCheckFile.value = (event.target as HTMLInputElement).files?.[0] || null
  }

  async function runProjectCheck() {
    const environment = getEnvironment()
    const baseUrl = projectCheckBaseURL.value.trim() || environment.baseUrl || ''
    if (!baseUrl) return flash('请填写已启动项目的根地址，例如 http://localhost:8082')
    projectCheckLoading.value = true
    try {
      if (projectCheckFile.value) {
        const spec = JSON.parse(await projectCheckFile.value.text())
        const imported = await projectApi.importOpenAPI(selectedProjectId.value, spec)
        await loadEndpoints()
        flash(`已从《${projectCheckFile.value.name}》导入 ${imported.imported} 个接口，开始巡检`)
        projectCheckFile.value = null
      }
      const result = await projectApi.run({
        projectId: selectedProjectId.value,
        baseUrl,
        task: projectCheckTask.value.trim(),
        permission: projectCheckPermission.value,
        confirmed: projectCheckPermission.value === 'auto',
      })
      projectCheckResult.value = result
      if (result.executed) {
        await refresh()
        flash(`安全巡检完成：${result.summary.reachable} 个已联通，${result.summary.pending} 个待审核`)
      } else {
        flash(`已生成审核计划：${result.summary.total} 个接口，确认后才执行`)
      }
    } catch (error) {
      flash((error as Error).message)
    } finally {
      projectCheckLoading.value = false
    }
  }

  const assistantPendingReviewItems = computed(() =>
    (assistantProjectCheck.value?.items || []).filter((item) => !item.reachable && item.decision !== '已执行'),
  )

  function restoreProjectCheckReport(report: ProjectCheckResult) {
    assistantProjectCheck.value = report
    assistantProjectCheckDraft.value = null
    assistantReviewOpen.value = false
    flash(`已打开 ${new Date(report.createdAt || Date.now()).toLocaleString()} 的巡检记录`)
  }

  function assistantBaseURL(value: string) {
    const mentioned = value.match(/https?:\/\/[^\s，。；,;]+/i)?.[0]?.replace(/\/+$/, '')
    if (mentioned) return mentioned
    const environment = getEnvironment()
    if (environment.baseUrl) return environment.baseUrl.replace(/\/+$/, '')
    const server = assistantOpenAPISpec.value?.servers?.[0]?.url
    return typeof server === 'string' ? server.replace(/\/+$/, '') : ''
  }

  async function runAssistantProjectCheck(task: string, baseUrl: string) {
    loading.value = true
    try {
      let imported = 0
      if (assistantOpenAPISpec.value) {
        const result = await projectApi.importOpenAPI(selectedProjectId.value, assistantOpenAPISpec.value)
        imported = result.imported || 0
        await loadEndpoints()
        assistantOpenAPISpec.value = null
      }
      const result = await projectApi.run({
        projectId: selectedProjectId.value,
        baseUrl,
        task,
        permission: 'auto',
        confirmed: true,
      })
      assistantProjectCheck.value = result
      assistantProjectCheckDraft.value = null
      assistantReviewOpen.value = false
      await refresh()
      const prefix = imported ? `已导入 ${imported} 个接口；` : ''
      await streamAssistantReply(`${prefix}安全巡检已完成：${result.summary.reachable} 个接口已联通，${result.summary.unavailable} 个不可达，${result.summary.pending} 个待审核。下方可下载 Markdown 报告。`)
      flash(`项目巡检完成：${result.summary.reachable} 个已联通`)
    } catch (error) {
      flash((error as Error).message)
    } finally {
      loading.value = false
    }
  }

  async function prepareAssistantProjectCheck(question: string) {
    const baseUrl = assistantBaseURL(question)
    if (!baseUrl) {
      await streamAssistantReply('我可以执行项目巡检，但还缺少已启动项目的根地址。请在消息中补充，例如：`http://localhost:8082`。')
      return true
    }
    if (assistantPermission.value === 'project_full') {
      await runAssistantProjectCheck(question, baseUrl)
    } else {
      assistantProjectCheckDraft.value = { task: question, baseUrl }
      await streamAssistantReply(`已生成巡检计划：将${assistantOpenAPISpec.value ? '导入当前 OpenAPI 文档，并' : ''}对 ${baseUrl} 进行无凭证只读联通性检查。请点击下方“确认并开始巡检”；写入、删除、上传等接口仍会保留为待审核。`)
    }
    return true
  }

  async function confirmAssistantProjectCheck() {
    const draft = assistantProjectCheckDraft.value
    if (!draft) return
    await runAssistantProjectCheck(draft.task, draft.baseUrl)
  }

  function downloadProjectCheckReport() {
    return downloadProjectTestReport(
      assistantProjectCheck.value?.markdown || projectCheckResult.value?.markdown,
      getProjectName(),
      flash,
    )
  }

  return {
    assistantProjectCheck,
    assistantProjectCheckDraft,
    projectCheckReports,
    assistantReviewOpen,
    assistantPendingReviewItems,
    loadProjectCheckReports,
    restoreProjectCheckReport,
    prepareAssistantProjectCheck,
    confirmAssistantProjectCheck,
    downloadProjectCheckReport,
    projectCheckBaseURL,
    projectCheckTask,
    projectCheckPermission,
    projectCheckFile,
    projectCheckLoading,
    projectCheckResult,
    selectProjectCheckFile,
    runProjectCheck,
  }
}
