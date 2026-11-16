<script setup lang="ts">
import { ref } from 'vue'
import { authRequest, type SessionState } from './auth'

defineProps<{ checking: boolean; connectionError: string; notice: string }>()
const emit = defineEmits<{ retry: []; authenticated: [session: SessionState] }>()
const username = ref('')
const password = ref('')
const passwordVisible = ref(false)
const submitting = ref(false)
const error = ref('')
async function login() {
  if (submitting.value) return
  error.value = ''
  submitting.value = true
  try {
    const session = await authRequest('/api/auth/login', { username: username.value.trim(), password: password.value })
    if (session.authenticated !== true) throw new Error('登录服务返回异常，请检查服务状态后重试')
    password.value = ''
    emit('authenticated', session)
  } catch (cause) {
    error.value = (cause as Error).message
    password.value = ''
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <main class="login-page">
    <div class="login-frame">
      <header class="login-brand">
        <svg viewBox="0 0 32 32" fill="none" width="38" height="38" aria-hidden="true">
          <rect width="32" height="32" rx="9" fill="currentColor" />
          <path d="M9.6 23.4V9.2h5.5a3.9 3.9 0 0 1 0 7.8H9.6M18.4 23.4l3.7-8.4 3.7 8.4M19.9 19.9h4.4" stroke="#fff" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
        <div><strong>Pulse API</strong><span>智能 API 测试与评测平台</span></div>
      </header>
      <section class="login-content" aria-labelledby="login-title">
        <div class="login-heading">
          <h1 id="login-title">登录测试工作台</h1>
          <p>使用平台账号，继续你的项目测试与模型测评。</p>
        </div>
        <div v-if="checking" class="login-status" role="status">正在连接登录服务…</div>
        <div v-else-if="connectionError" class="login-status">
          <p role="alert">{{ connectionError }}</p>
          <button type="button" class="login-submit" @click="emit('retry')">重新连接</button>
        </div>
        <form v-else class="login-form" @submit.prevent="login" :aria-busy="submitting">
          <p v-if="notice && !error" class="login-notice" role="status">{{ notice }}</p>
          <div class="login-field">
            <label for="platform-username">平台账号</label>
            <input id="platform-username" v-model="username" name="username" autocomplete="username" placeholder="请输入平台账号" required maxlength="128" :disabled="submitting" :aria-invalid="!!error" :aria-describedby="error ? 'login-error' : undefined" />
          </div>
          <div class="login-field">
            <label for="platform-password">密码</label>
            <div class="login-password">
              <input id="platform-password" v-model="password" name="password" :type="passwordVisible ? 'text' : 'password'" autocomplete="current-password" placeholder="请输入密码" required maxlength="1024" :disabled="submitting" :aria-invalid="!!error" :aria-describedby="error ? 'login-error' : undefined" />
              <button type="button" class="login-visibility" :aria-label="passwordVisible ? '隐藏密码' : '显示密码'" :aria-pressed="passwordVisible" :disabled="submitting" @click="passwordVisible = !passwordVisible">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" width="20" height="20" aria-hidden="true"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z"/><circle cx="12" cy="12" r="3"/><path v-if="passwordVisible" d="m3 3 18 18"/></svg>
              </button>
            </div>
          </div>
          <p v-if="error" id="login-error" class="login-error" role="alert">{{ error }}</p>
          <button class="login-submit" type="submit" :disabled="submitting">{{ submitting ? '正在登录…' : '登录工作台' }}</button>
        </form>
        <div class="login-help">
          <p>使用管理员提供的平台账号，当前无需注册。</p>
          <p>平台账号用于访问获分配的项目。被测项目的登录账号在测试任务中另行配置。</p>
        </div>
      </section>
      <footer class="login-footer">项目测试 · 业务模型测评 · 测试记录</footer>
    </div>
  </main>
</template>

<style scoped>
.login-page { min-height: 100dvh; padding: 24px; color: var(--text-title, #12324a); background: var(--surface-sunken, #f1f5f7); display: flex; font-family: inherit; }
.login-frame { width: 100%; min-height: calc(100dvh - 48px); display: flex; flex-direction: column; border: 1px solid var(--line, #dde5ea); border-radius: 24px; background: var(--surface, #fff); box-shadow: 0 8px 32px rgba(18,50,74,.04); }
.login-brand { display: flex; align-items: center; gap: 12px; padding: 28px 36px; color: var(--blue, #2c8c7b); }
.login-brand div { display: flex; flex-direction: column; gap: 3px; }
.login-brand strong { color: var(--text-title, #12324a); font-size: 21px; line-height: 1.3; }
.login-brand span { color: var(--text-body, #365266); font-size: 13px; }
.login-content { width: min(100% - 48px, 420px); margin: auto; padding: 28px 0 48px; }
.login-heading h1 { margin: 0 0 12px; font-size: clamp(26px, 3vw, 32px); font-weight: 650; letter-spacing: -.025em; line-height: 1.35; text-wrap: balance; }
.login-heading p { margin: 0; color: var(--text-body, #365266); font-size: 15px; line-height: 1.7; }
.login-form { display: flex; flex-direction: column; gap: 22px; margin-top: 34px; }
.login-field { display: flex; flex-direction: column; gap: 9px; }
.login-field label { font-size: 15px; font-weight: 600; color: var(--text-title, #12324a); }
.login-field input { box-sizing: border-box; width: 100%; height: 48px; border: 1px solid #bccbd3; border-radius: 10px; background: var(--surface-alt, #f8fafb); color: var(--text-title, #12324a); padding: 0 14px; font-family: inherit; font-size: 16px; caret-color: var(--blue, #2c8c7b); outline: none; }
.login-field input::placeholder { color: #647786; opacity: 1; }
.login-field input:focus-visible { border-color: var(--blue, #2c8c7b); outline: 2px solid var(--blue, #2c8c7b); outline-offset: 2px; }
.login-field input[aria-invalid="true"] { border-color: #b63b39; }
.login-password { position: relative; }
.login-password input { padding-right: 52px; }
.login-visibility { position: absolute; right: 4px; top: 4px; display: grid; place-items: center; width: 40px; height: 40px; background: transparent; border: 0; border-radius: 8px; color: var(--text-body, #365266); cursor: pointer; }
.login-visibility:hover { background: var(--surface-sunken, #f1f5f7); }
.login-submit { width: 100%; min-height: 48px; border: 1px solid #267969; border-radius: 10px; background: #267969; color: #fff; font-size: 16px; font-weight: 600; font-family: inherit; cursor: pointer; }
.login-submit:hover:not(:disabled) { background: #206658; border-color: #206658; }
.login-submit:disabled { background: #567b72; border-color: #567b72; cursor: wait; }
.login-visibility:focus-visible, .login-submit:focus-visible { outline: 2px solid var(--blue, #2c8c7b); outline-offset: 3px; }
.login-field input:disabled { color: #596f7a; cursor: wait; }
.login-error, .login-notice { margin: 0; font-size: 14px; line-height: 1.65; }
.login-error { color: #a53230; }
.login-notice { color: #206658; }
.login-status { margin-top: 32px; color: var(--text-body, #365266); font-size: 15px; line-height: 1.7; }
.login-status p { margin: 0 0 20px; }
.login-help { margin-top: 30px; padding-top: 22px; border-top: 1px solid var(--line-soft, #ebf0f3); color: #536b7b; font-size: 13px; line-height: 1.8; }
.login-help p { margin: 0 0 5px; }
.login-footer { padding: 18px 24px 24px; text-align: center; color: #657786; font-size: 13px; }
.login-page ::selection { background: #cce9e2; color: #12324a; }
@media (max-width: 600px) {
  .login-page { padding: 12px; }
  .login-frame { min-height: calc(100dvh - 24px); border-radius: 18px; }
  .login-brand { padding: 24px; }
  .login-content { width: calc(100% - 48px); padding: 32px 0; }
  .login-form { margin-top: 28px; }
  .login-footer { padding-bottom: 20px; font-size: 12px; }
}
</style>
