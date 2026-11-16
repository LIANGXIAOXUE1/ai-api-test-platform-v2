import type { InjectionKey, Ref } from 'vue'

export type PlatformUser = { id: number; username: string; role: 'administrator' | 'tester' | 'viewer'; projectIds: number[]; allowAuto: boolean; enabled: boolean }
export const roleLabel = (role: PlatformUser['role']) => ({administrator:'管理员',tester:'测试员',viewer:'只读用户'}[role])
export type SessionState = { authenticationRequired: boolean; authenticated: boolean; user: PlatformUser | null }
export const authRequiredEvent = 'pulse-auth-required'
export const platformAuthKey: InjectionKey<{
  user: Ref<PlatformUser | null>
  loggingOut: Ref<boolean>
  logoutError: Ref<string>
  logout: () => Promise<void>
}> = Symbol('platform-auth')

export async function authRequest(path: string, payload?: unknown): Promise<any> {
  const controller = new AbortController()
  const timer = window.setTimeout(() => controller.abort(), 12000)
  try {
    const response = await fetch(path, {
      method: payload === undefined ? 'GET' : 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: payload === undefined ? undefined : JSON.stringify(payload),
      signal: controller.signal,
    })
    const data = await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(data.error || '暂时无法连接登录服务，请稍后重试')
    return data
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      throw new Error('登录服务响应超时，请检查服务状态后重试')
    }
    if (error instanceof TypeError) throw new Error('无法连接登录服务，请检查网络或服务状态')
    throw error
  } finally {
    window.clearTimeout(timer)
  }
}
