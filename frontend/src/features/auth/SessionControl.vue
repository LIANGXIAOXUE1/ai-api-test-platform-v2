<script setup lang="ts">
import { inject } from 'vue'
import { platformAuthKey } from './auth'
const auth = inject(platformAuthKey)
</script>

<template>
  <div v-if="auth?.user.value" class="session-control">
    <span class="session-user" :title="`${auth.user.value.username} · 平台管理员`">{{ auth.user.value.username }}<small>管理员</small></span>
    <button type="button" class="session-logout" aria-label="退出登录" title="退出登录" :disabled="auth.loggingOut.value" @click="auth.logout"><svg class="session-logout-icon" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 4H5a1 1 0 0 0-1 1v14a1 1 0 0 0 1 1h4M10 12h10m-4-4 4 4-4 4"/></svg><span>{{ auth.loggingOut.value ? '退出中…' : '退出登录' }}</span></button>
    <p v-if="auth.logoutError.value" role="alert" class="session-error">{{ auth.logoutError.value }}，可再次点击退出登录。</p>
  </div>
</template>

<style scoped>
.session-control { position: relative; display: flex; align-items: center; gap: 10px; margin-left: 6px; padding-left: 14px; border-left: 1px solid #315a73; flex-shrink: 0; }
.session-user { display: flex; flex-direction: column; gap: 2px; max-width: 100px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; color: #eef5f7; }
.session-user small { color: #c4d5de; font-size: 11px; }
.session-logout { border: 1px solid #547287; background: #20445d; color: #eef5f7; border-radius: 8px; font: inherit; font-size: 13px; padding: 8px 10px; min-height: 36px; cursor: pointer; }
.session-logout:hover:not(:disabled) { background: #206c60; border-color: #55a897; color: #fff; }
.session-logout:focus-visible { outline: 2px solid #2c8c7b; outline-offset: 2px; }
.session-logout:disabled { cursor: wait; color: #c4d5de; }
.session-logout-icon { display: none; }
:global(.app-v2.sidebar-collapsed .session-control) { width: 40px; margin: 8px auto 0; padding: 0; border: 0; }
:global(.app-v2.sidebar-collapsed .session-user), :global(.app-v2.sidebar-collapsed .session-logout span) { display: none; }
:global(.app-v2.sidebar-collapsed .session-logout) { display: grid; place-items: center; width: 40px; height: 40px; padding: 0; }
:global(.app-v2.sidebar-collapsed .session-logout-icon) { display: block; }
.session-error { position: absolute; right: 0; top: 100%; z-index: 120; width: min(300px, 80vw); padding: 12px; color: #a53230; background: #fff; border: 1px solid #d5b7b6; border-radius: 10px; box-shadow: 0 4px 16px rgba(18,50,74,.12); font-size: 13px; line-height: 1.7; }
@media (max-width: 900px) { .session-user { display: none; } .session-control { padding-left: 8px; margin-left: 0; } }
</style>
