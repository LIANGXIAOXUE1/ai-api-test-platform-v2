<script setup lang="ts">
import { markRaw, onBeforeUnmount, onMounted, provide, ref, type Component } from 'vue'
import LoginPage from './LoginPage.vue'
import { authRequest, authRequiredEvent, platformAuthKey, type SessionState, type PlatformUser } from './auth'

const props = defineProps<{ workspace: Component }>()
const workspace = markRaw(props.workspace)
const ready = ref(false)
const checking = ref(true)
const connectionError = ref('')
const loginNotice = ref('')
const user = ref<PlatformUser | null>(null)
const loggingOut = ref(false)
const logoutError = ref('')
let generation = 0

async function checkSession() {
  checking.value = true
  connectionError.value = ''
  try {
    const session: SessionState = await authRequest('/api/auth/session')
    user.value = session.user
    ready.value = session.authenticated
  } catch (error) {
    ready.value = false
    connectionError.value = (error as Error).message
  } finally {
    checking.value = false
  }
}
function loggedIn(session: SessionState) {
  user.value = session.user
  loginNotice.value = ''
  logoutError.value = ''
  ready.value = true
  generation++
}
function requireLogin() {
  // Unmount the workspace so projects, keys and responses leave the visible UI.
  if (!ready.value) return
  ready.value = false
  user.value = null
  loginNotice.value = '登录已过期，请重新登录。已保存的测试记录仍保留。'
}
async function recheckOnFocus() {
  if (!ready.value || loggingOut.value) return
  try {
    const session: SessionState = await authRequest('/api/auth/session')
    if (session.authenticated === false) requireLogin()
  } catch { /* A network interruption is not proof that the session expired. */ }
}
async function logout() {
  if (loggingOut.value) return
  loggingOut.value = true
  logoutError.value = ''
  try {
    await authRequest('/api/auth/logout', {})
    ready.value = false
    user.value = null
    loginNotice.value = '已退出登录'
  } catch (error) {
    logoutError.value = (error as Error).message
  } finally {
    loggingOut.value = false
  }
}
provide(platformAuthKey, { user, loggingOut, logoutError, logout })
onMounted(() => {
  window.addEventListener(authRequiredEvent, requireLogin)
  window.addEventListener('focus', recheckOnFocus)
  void checkSession()
})
onBeforeUnmount(() => {
  window.removeEventListener(authRequiredEvent, requireLogin)
  window.removeEventListener('focus', recheckOnFocus)
})
</script>

<template>
  <component :is="workspace" v-if="ready" :key="generation" />
  <LoginPage v-else :checking="checking" :connection-error="connectionError" :notice="loginNotice" @retry="checkSession" @authenticated="loggedIn" />
</template>
