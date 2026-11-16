export type ProjectCheckPermission = 'review' | 'auto'

export type ProjectCheckItem = {
  apiId: number
  name: string
  method: string
  url: string
  risk: string
  decision: string
  reason?: string
  reachable: boolean
  statusCode: number
  businessCode?: string
  durationMs: number
  error?: string
}

export type ProjectCheckResult = {
  id?: number
  projectId: number
  baseUrl: string
  task: string
  permission: ProjectCheckPermission
  executed: boolean
  createdAt?: string
  summary: {
    total: number
    reachable: number
    unavailable: number
    pending: number
  }
  items: ProjectCheckItem[]
  markdown: string
}

export type AssistantProjectCheckDraft = {
  task: string
  baseUrl: string
}

export type ProjectCheckInput = {
  projectId: number
  baseUrl: string
  task: string
  permission: ProjectCheckPermission
  confirmed: boolean
}

export type ApiRequest = <T = unknown>(path: string, options?: RequestInit) => Promise<T>
