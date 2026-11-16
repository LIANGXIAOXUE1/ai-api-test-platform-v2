<script setup lang="ts">
import { inject, nextTick, ref, watch } from 'vue'
import { authRequest, platformAuthKey, roleLabel } from './auth'
const auth = inject(platformAuthKey)
const showPassword = ref(false)
const currentPassword = ref('')
const newPassword = ref('')
const passwordError = ref('')
const saving = ref(false)
const passwordDialog = ref<HTMLFormElement>()
let previousFocus: HTMLElement | null = null
watch(showPassword, async visible => {
 if (visible) { previousFocus = document.activeElement as HTMLElement; await nextTick(); passwordDialog.value?.querySelector<HTMLInputElement>('input')?.focus() }
 else { currentPassword.value = ''; newPassword.value = ''; previousFocus?.focus() }
})
function closePassword() { if (!saving.value) showPassword.value = false }
function handleDialogKey(event: KeyboardEvent) {
 if (event.key === 'Escape') { event.preventDefault(); closePassword(); return }
 if (event.key !== 'Tab') return
 const controls = passwordDialog.value?.querySelectorAll<HTMLElement>('input:not(:disabled),button:not(:disabled)')
 if (!controls?.length) return
 const first = controls[0], last = controls[controls.length - 1]
 if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
 else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
}
async function changePassword() {
  saving.value = true; passwordError.value = ''
  try {
    await authRequest('/api/auth/password', { currentPassword: currentPassword.value, password: newPassword.value })
    currentPassword.value = ''; newPassword.value = ''; showPassword.value = false
    await auth?.logout()
  } catch (error) { passwordError.value = (error as Error).message }
  finally { saving.value = false }
}
</script>

<template>
  <div v-if="auth?.user.value" class="session-control">
    <span class="session-user" :title="`${auth.user.value.username} · ${roleLabel(auth.user.value.role)}`">{{ auth.user.value.username }}<small>{{ roleLabel(auth.user.value.role) }}</small></span>
    <button type="button" class="session-password" aria-label="修改密码" title="修改密码" @click="showPassword=true;passwordError=''"><svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="5" y="10" width="14" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3M12 14v3"/></svg></button>
    <button type="button" class="session-logout" aria-label="退出登录" title="退出登录" :disabled="auth.loggingOut.value" @click="auth.logout"><svg class="session-logout-icon" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 4H5a1 1 0 0 0-1 1v14a1 1 0 0 0 1 1h4M10 12h10m-4-4 4 4-4 4"/></svg><span>{{ auth.loggingOut.value ? '退出中…' : '退出登录' }}</span></button>
    <p v-if="auth.logoutError.value" role="alert" class="session-error">{{ auth.logoutError.value }}，可再次点击退出登录。</p>
  </div>
  <Teleport to="body"><div v-if="showPassword" class="password-scrim" @click.self="closePassword"><form ref="passwordDialog" class="password-dialog" role="dialog" aria-modal="true" aria-labelledby="password-title" @submit.prevent="changePassword" @keydown="handleDialogKey"><h2 id="password-title">修改自己的密码</h2><p>保存后需重新登录。</p><label>当前密码<input v-model="currentPassword" type="password" autocomplete="current-password" required :disabled="saving"></label><label>新密码<input v-model="newPassword" type="password" autocomplete="new-password" minlength="12" maxlength="1024" required placeholder="至少12个字符" :disabled="saving"></label><p v-if="passwordError" class="password-error" role="alert">{{ passwordError }}</p><div class="password-actions"><button type="button" :disabled="saving" @click="closePassword">取消</button><button type="submit" :disabled="saving">{{ saving?'保存中…':'保存并重新登录' }}</button></div></form></div></Teleport>
</template>

<style scoped>
.session-control { position: relative; display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin: 8px 0 0; padding: 0; flex-shrink: 0; }
.session-user { display: flex; flex-direction: column; gap: 2px; max-width: 100px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; color: #eef5f7; }
.session-user small { color: #c4d5de; font-size: 11px; }
.session-user { flex-basis: 100%; max-width: 180px; }
.session-logout { border: 1px solid #547287; background: #20445d; color: #eef5f7; border-radius: 8px; font: inherit; font-size: 13px; padding: 8px 10px; min-height: 36px; cursor: pointer; }
.session-logout:hover:not(:disabled) { background: #206c60; border-color: #55a897; color: #fff; }
.session-logout:focus-visible { outline: 2px solid #2c8c7b; outline-offset: 2px; }
.session-logout:disabled { cursor: wait; color: #c4d5de; }
.session-logout-icon { display: none; }
.session-control .session-logout span,.session-control .session-logout svg,.session-control .session-password svg { color: #eef5f7 !important; }
.session-password { display: grid; place-items: center; color: #eef5f7; border: 1px solid #547287; background: #20445d; border-radius: 8px; padding: 8px; width: 36px; height: 36px; cursor: pointer; }
.session-password:focus-visible { outline: 2px solid #55a897; outline-offset: 2px; }
:global(.app-v2.sidebar-collapsed .session-control) { width: 40px; margin: 8px auto 0; padding: 0; border: 0; }
:global(.app-v2.sidebar-collapsed .session-control) { flex-direction: column; }
:global(.app-v2.sidebar-collapsed .session-user), :global(.app-v2.sidebar-collapsed .session-logout span) { display: none; }
:global(.app-v2.sidebar-collapsed .session-logout) { display: grid; place-items: center; width: 40px; height: 40px; padding: 0; }
:global(.app-v2.sidebar-collapsed .session-logout-icon) { display: block; }
.session-error { position: absolute; right: 0; top: 100%; z-index: 120; width: min(300px, 80vw); padding: 12px; color: #a53230; background: #fff; border: 1px solid #d5b7b6; border-radius: 10px; box-shadow: 0 4px 16px rgba(18,50,74,.12); font-size: 13px; line-height: 1.7; }
.password-scrim { position: fixed; inset: 0; display: grid; place-items: center; background: rgba(18,50,74,.35); padding: 24px; z-index: 600; }
.password-dialog { width: min(440px,100%); background: #fff; color: #12324a; padding: 26px; border-radius: 18px; box-shadow: 0 12px 36px rgba(18,50,74,.15); }
.password-dialog h2 { margin: 0; font-size: 22px; }
.password-dialog p { font-size: 14px; line-height: 1.6; color: #536b7b; }
.password-dialog label { display: flex; flex-direction: column; gap: 8px; margin: 18px 0; font-size: 14px; }
.password-dialog input { padding: 12px; border: 1px solid #bccbd3; color: #12324a; border-radius: 8px; font-size: 16px; }
.password-actions { display: flex; justify-content: flex-end; gap: 10px; }
.password-actions button { padding: 10px 14px; border: 1px solid #267969; border-radius: 8px; background: #267969; color: #fff; cursor: pointer; }
.password-dialog .password-error { color: #a53230; }
@media (max-width: 900px) {
  .session-user { display: none; }
  .session-control { padding: 0; margin-left: 40px; gap: 6px; flex-wrap: nowrap; }
  .session-logout { display: grid; place-items: center; width: 36px; height: 36px; padding: 0; }
  .session-logout span { display: none; }
  .session-logout-icon { display: block; }
}
</style>
