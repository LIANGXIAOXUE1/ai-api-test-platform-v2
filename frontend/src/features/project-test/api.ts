import type { ApiRequest, ProjectCheckInput, ProjectCheckResult } from './types'

export function createProjectTestApi(request: ApiRequest) {
  return {
    listReports(projectId: number) {
      return request<ProjectCheckResult[]>('/api/project-check-reports?projectId=' + projectId)
    },
    run(input: ProjectCheckInput) {
      return request<ProjectCheckResult>('/api/project-check', {
        method: 'POST',
        body: JSON.stringify(input),
      })
    },
    importOpenAPI(projectId: number, spec: Record<string, unknown>) {
      return request<{ imported: number }>('/api/apis/import', {
        method: 'POST',
        body: JSON.stringify({ projectId, spec }),
      })
    },
  }
}
