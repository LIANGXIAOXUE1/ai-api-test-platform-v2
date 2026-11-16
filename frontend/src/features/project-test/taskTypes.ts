export type TaskStep = {
  id: string; name: string; method: string; url: string
  headers: Record<string, string>; query: Record<string, string>; body: string
  upload: boolean; fileField?: string; dependsOn: string[]; assertions: any[]; extract: any[]; guard?: string
}
export type ProjectTask = {
  id: number; projectId: number; name: string; baseUrl: string; openApiUrl: string; goal: string
  template: string; accountMode: string; username: string; password: string; hasPassword: boolean
  fileMode: string; fileName: string; fileData: string; fileSize: number; permission: string
  initialVariables?: Record<string,string>; features: string[]; steps: TaskStep[]; updatedAt?: string
}
export type ProjectRun = {
  id: number; projectId: number; taskId: number; sourceRunId?: number; task: ProjectTask
  status: string; currentStep: string; results: any[]; variables: Record<string, string>; error?: string
  summary: { total: number; passed: number; failed: number; blocked: number; notRun: number }
  createdAt: string; finishedAt?: string
}
export const newProjectTask = (projectId: number): ProjectTask => ({
  id: 0, projectId, name: '核心业务流程测试', baseUrl: 'http://localhost:8082',
  openApiUrl: 'http://localhost:8082/v3/api-docs', goal: '验证注册登录、文件夹重名限制、文件上传下载及删除还原。',
  template: 'cloud_disk', accountMode: 'auto', username: '', password: '', hasPassword: false,
  fileMode: 'auto', fileName: '', fileData: '', fileSize: 2048, permission: 'review',
  features: ['folder', 'file', 'lifecycle', 'auth'], steps: [],
})
