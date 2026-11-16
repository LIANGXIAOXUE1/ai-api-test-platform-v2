<script setup lang="ts">
import { inject, onMounted, ref } from 'vue'
import { platformAuthKey, roleLabel, type PlatformUser } from './auth'
import '../shared/business-testing.css'

const props = defineProps<{ projects: { id: number; name: string }[]; request: (path: string, options?: RequestInit) => Promise<any> }>()
const auth = inject(platformAuthKey)
const users = ref<PlatformUser[]>([])
const audits = ref<any[]>([])
const tab = ref('users')
const editing = ref(false)
const busy = ref(false)
const error = ref('')
const notice = ref('')
const password = ref('')
const resetting = ref<PlatformUser | null>(null)
const fresh = () => ({ id: 0, username: '', role: 'tester' as PlatformUser['role'], projectIds: [] as number[], allowAuto: false, enabled: true })
const form = ref(fresh())
const post = (value: any, method = 'POST') => ({ method, body: JSON.stringify(value) })
async function load() {
  error.value = ''
  try { users.value = await props.request('/api/admin/users'); audits.value = await props.request('/api/admin/audit') }
  catch (cause) { error.value = (cause as Error).message }
}
function edit(user?: PlatformUser) {
  resetting.value = null; error.value = ''; notice.value = ''; password.value = ''
  form.value = user ? { id: user.id, username: user.username, role: user.role, projectIds: [...user.projectIds], allowAuto: user.allowAuto, enabled: user.enabled } : fresh()
  editing.value = true
}
function selectProject(id: number, checked: boolean) {
  form.value.projectIds = checked ? [...new Set([...form.value.projectIds, id])] : form.value.projectIds.filter(value => value !== id)
}
function projectNames(user: PlatformUser) {
  return user.role === 'administrator' ? '全部项目' : user.projectIds.map(id => props.projects.find(p => p.id === id)?.name || `项目 #${id}`).join('、') || '尚未分配'
}
async function save() {
  if (busy.value) return
  busy.value = true; error.value = ''
  try {
    const input: any = { ...form.value, allowAuto: form.value.role === 'administrator' || (form.value.role === 'tester' && form.value.allowAuto) }
    if (!form.value.id) input.password = password.value
    await props.request('/api/admin/users', post(input, form.value.id ? 'PUT' : 'POST'))
    password.value = ''; editing.value = false; notice.value = '用户配置已保存；权限变化会使该用户的旧会话失效。'
    if (form.value.id === auth?.user.value?.id) { await auth.logout(); return }
    await load()
  } catch (cause) { error.value = (cause as Error).message }
  finally { busy.value = false }
}
async function resetPassword() {
  if (!resetting.value || busy.value) return
  busy.value = true; error.value = ''
  try {
    const id = resetting.value.id
    await props.request('/api/admin/users/reset-password', post({ id, password: password.value }))
    password.value = ''; resetting.value = null; notice.value = '密码已重置，该账号需重新登录。'
    if (id === auth?.user.value?.id) { await auth.logout(); return }
    await load()
  } catch (cause) { error.value = (cause as Error).message }
  finally { busy.value = false }
}
onMounted(load)
</script>

<template>
  <section class="list-page business-testing users-page">
    <div class="page-title"><div><h1>用户与权限</h1><p>分配项目、管理账号和助手执行权限。模型池与助手配置仍是平台公共资源。</p></div><button class="primary-btn" @click="edit()">新增用户</button></div>
    <nav class="bt-tabs" aria-label="用户管理"><button :class="{active:tab==='users'}" @click="tab='users'">用户与权限</button><button :class="{active:tab==='audit'}" @click="tab='audit';load()">操作日志</button></nav>
    <p v-if="error" role="alert" class="bt-error">{{ error }}</p><p v-if="notice" role="status" class="bt-notice">{{ notice }}</p>
    <form v-if="editing" class="bt-section user-editor" @submit.prevent="save">
      <div class="bt-heading"><h2>{{ form.id ? '编辑用户' : '新增用户' }}</h2><button type="button" class="link-btn" :disabled="busy" @click="editing=false;password=''">取消</button></div>
      <div class="user-fields"><label>平台账号<input v-model="form.username" autocomplete="off" required minlength="3" maxlength="64" placeholder="3～64个字母、数字、中文或 ._-" :disabled="busy"></label><label>角色<select v-model="form.role" :disabled="busy"><option value="administrator">管理员</option><option value="tester">测试员</option><option value="viewer">只读用户</option></select></label><label v-if="!form.id">初始密码<input v-model="password" type="password" autocomplete="new-password" required minlength="12" maxlength="1024" placeholder="至少12个字符" :disabled="busy"></label></div>
      <fieldset v-if="form.role!=='administrator'" class="user-projects"><legend>可访问项目</legend><label v-for="project in projects" :key="project.id"><input type="checkbox" :checked="form.projectIds.includes(project.id)" :disabled="busy" @change="selectProject(project.id,($event.target as HTMLInputElement).checked)">{{ project.name }}</label><p v-if="!projects.length" class="bt-hint">先创建项目，再分配访问范围。</p></fieldset>
      <p v-else class="bt-hint">管理员可访问全部项目，并管理公共模型及助手配置。</p>
      <div class="user-switches"><label><input v-model="form.enabled" type="checkbox" :disabled="busy">启用账号</label><label v-if="form.role==='tester'"><input v-model="form.allowAuto" type="checkbox" :disabled="busy">允许助手自动执行</label></div>
      <p class="bt-hint">测试员仅可操作分配的项目；只读用户不能执行、修改或发起模型调用。自动执行权限不代表操作系统权限。</p>
      <div class="bt-actions"><button type="submit" class="primary-btn" :disabled="busy">{{ busy?'保存中…':'保存用户' }}</button></div>
    </form>
    <form v-if="resetting" class="bt-section user-editor" @submit.prevent="resetPassword"><div class="bt-heading"><h2>重置 {{ resetting.username }} 的密码</h2><button type="button" class="link-btn" @click="resetting=null;password=''">取消</button></div><label>新密码<input v-model="password" type="password" autocomplete="new-password" minlength="12" maxlength="1024" required placeholder="至少12个字符" :disabled="busy"></label><p class="bt-hint">保存后该账号的现有会话失效，请通过受控渠道告知新密码。</p><button type="submit" class="primary-btn" :disabled="busy">{{ busy?'重置中…':'确认重置' }}</button></form>
    <div v-if="tab==='users'" class="bt-table-wrap"><table class="bt-table"><thead><tr><th>用户</th><th>角色</th><th>可访问项目</th><th>助手自动执行</th><th>状态</th><th>操作</th></tr></thead><tbody><tr v-for="user in users" :key="user.id"><td><b>{{ user.username }}</b><small v-if="auth?.user.value?.id===user.id">当前账号</small></td><td>{{ roleLabel(user.role) }}</td><td>{{ projectNames(user) }}</td><td>{{ user.role === 'viewer' ? '不允许' : user.allowAuto ? '允许' : '先审核' }}</td><td>{{ user.enabled ? '启用' : '已禁用' }}</td><td class="user-actions"><button class="link-btn" @click="edit(user)">编辑权限 / 状态</button><button class="link-btn" @click="editing=false;resetting=user;password='';error=''">重置密码</button></td></tr></tbody></table><p v-if="!users.length&&!busy" class="bt-hint">暂无用户。账号由管理员创建，不开放自助注册。</p></div>
    <template v-else><div class="bt-heading"><p class="bt-hint">最近200条操作，记录操作者、项目、资源编号和HTTP状态；不记录密码或请求体。</p><button class="send-only" :disabled="busy" @click="load">刷新日志</button></div><div class="bt-table-wrap"><table class="bt-table"><thead><tr><th>时间</th><th>操作者</th><th>操作</th><th>项目</th><th>资源编号</th><th>状态码</th></tr></thead><tbody><tr v-for="event in audits" :key="event.id"><td>{{ new Date(event.createdAt).toLocaleString() }}</td><td>{{ event.actor }}</td><td>{{ event.method }} {{ event.path }}</td><td>{{ projects.find(p=>p.id===event.projectId)?.name || (event.projectId ? '#'+event.projectId : '公共资源') }}</td><td>{{ event.resourceId || '—' }}</td><td>{{ event.statusCode || '未完成' }}</td></tr></tbody></table></div></template>
  </section>
</template>

<style scoped>
.users-page { --blue: #267969; --blue-dark: #216758; --text-muted: #536b7b; --bt-muted: #536b7b; color: var(--text-title, #12324a); }
:global(.app-v2 .topbar-v2 button.users-nav-btn.active), :global(.app-v2.sidebar-collapsed .topbar-v2 button.users-nav-btn.active) { background: #267969 !important; }
:global(.topbar-v2 .users-nav-btn.active span) { color: #fff !important; }
.users-page .link-btn { color: #216758; }
/* The incumbent theme declares global button colors with !important. */
.users-page .primary-btn { background: #267969 !important; border-color: #267969 !important; color: #fff; white-space: nowrap; }
.users-page .primary-btn:hover:not(:disabled) { background: #216758 !important; }
.user-editor { margin: 20px 0; }
.user-fields { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 18px; }
.user-projects { border: 0; margin: 24px 0 18px; padding: 0; display: flex; flex-wrap: wrap; gap: 14px 24px; }
.user-projects legend { padding: 0 6px; font-weight: 600; font-size: 14px; }
.user-projects label,.user-switches label { display: flex; align-items: center; gap: 8px; font-size: 14px; }
.user-projects input,.user-switches input { width: 16px; height: 16px; accent-color: #267969; }
.user-switches { display: flex; flex-wrap: wrap; gap: 24px; margin: 16px 0; }
.user-actions { white-space: nowrap; }
.user-actions button { margin-right: 12px; }
.users-page button:focus-visible,.users-page input:focus-visible,.users-page select:focus-visible { outline: 2px solid #267969; outline-offset: 2px; }
@media(max-width:700px) { .user-fields { grid-template-columns: 1fr; } }
</style>
