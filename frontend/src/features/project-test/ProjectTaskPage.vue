<script setup lang="ts">
import { usePermissions } from '../auth/usePermissions'
const { readOnly, canAuto, isAdmin } = usePermissions()
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { authRequiredEvent } from '../auth/auth'
import ProjectTaskSetup from './ProjectTaskSetup.vue'
import ProjectTaskReport from './ProjectTaskReport.vue'
import { newProjectTask, type ProjectTask, type ProjectRun } from './taskTypes'
import '../shared/business-testing.css'
const props = defineProps<{ projectId: number; projectName: string; initialGoal?: string; request: (path: string, options?: RequestInit) => Promise<any> }>()
const emit = defineEmits<{ imported: [] }>()
const tab = ref(readOnly.value ? 'history' : 'setup')
const form = ref(newProjectTask(props.projectId))
if (props.initialGoal) { form.value.goal = props.initialGoal; form.value.template = 'ai' }
const tasks = ref<ProjectTask[]>([])
const runs = ref<ProjectRun[]>([])
const activeRun = ref<ProjectRun | null>(null)
const busy = ref(false)
const error = ref('')
const notice = ref('')
const stepEditor = ref('')
const confirmed = ref(false)
const variableEditor = ref('{}')
const inputs = ref<{field:string;question:string;secret?:boolean;value:string}[]>([])
const inputAction = ref('plan')
function readVariables() { const data=JSON.parse(variableEditor.value || '{}'); if (!data || Array.isArray(data) || typeof data !== 'object' || Object.values(data).some(v=>typeof v !== 'string')) throw new Error('补充测试数据需为 JSON 对象，每个值使用字符串。'); form.value.initialVariables=data }
async function supplyInputs() {
  try {
    readVariables()
    for (const item of inputs.value) { if (!item.value.trim()) throw new Error('请补充：'+item.question); form.value.initialVariables![item.field]=item.value }
    variableEditor.value=JSON.stringify(form.value.initialVariables,null,2); inputs.value=[]
    if (inputAction.value==='plan') await generate(); else await execute()
  } catch(e) { error.value=(e as Error).message }
}
let restoring = false
let timer: ReturnType<typeof setTimeout> | undefined
const api = (path: string, options?: RequestInit) => props.request('/api/project-tests/' + path, options)
const post = (value: unknown) => ({ method: 'POST', body: JSON.stringify(value) })
async function load() {
  try {
    const [saved, history] = await Promise.all([api('tasks?projectId=' + props.projectId), api('runs?projectId=' + props.projectId)])
    tasks.value = saved; runs.value = history
  } catch (e) { error.value = (e as Error).message }
}
async function save() {
  form.value.projectId = props.projectId
  readVariables()
  if (stepEditor.value) form.value.steps = JSON.parse(stepEditor.value)
  const task = await api('tasks', post(form.value))
  form.value = { ...task, password: '', fileData: '' }
  variableEditor.value=JSON.stringify(task.initialVariables || {},null,2)
  await load()
  notice.value = '任务配置已保存。查看记录不会执行请求。'
  return task as ProjectTask
}
async function saveOnly() {
  busy.value = true; error.value = ''
  try { await save(); tab.value = 'history' } catch (e) { error.value = (e as Error).message } finally { busy.value = false }
}
async function generate() {
  busy.value = true; error.value = ''; confirmed.value = false
  try { readVariables(); const result = await api('plan', post(form.value)); form.value.steps = result.steps; stepEditor.value = result.steps.length ? JSON.stringify(result.steps, null, 2) : ''; notice.value = result.message; if (result.missingInformation?.length) { inputAction.value='plan'; inputs.value=result.missingInformation.map((item:any)=>({...item,value:''})) } }
  catch (e) { error.value = (e as Error).message } finally { busy.value = false }
}
async function execute(sourceRunId = 0) {
  error.value = ''; busy.value = true
  try {
    if (!sourceRunId && form.value.permission === 'review' && !confirmed.value) throw new Error('请先确认本次整套操作范围。')
    if (!sourceRunId) {
      readVariables()
      const available = new Set(['username','password','runId','folderName','fileName','renamedFile','fileSize','fileHash',...Object.keys(form.value.initialVariables || {}),...form.value.steps.flatMap(step=>(step.extract || []).map(item=>item.name))])
      const missing = [...new Set((stepEditor.value.match(/\{\{([a-zA-Z0-9_]+)\}\}/g) || []).map(item=>item.slice(2,-2)))].filter(name=>!available.has(name))
      if (missing.length) { inputAction.value='execute'; inputs.value=missing.map(field=>({field,question:'请提供测试变量 '+field,secret:/token|password|key|secret/i.test(field),value:''})); return }
    }
    const task = sourceRunId ? { id: activeRun.value!.taskId } : await save()
    activeRun.value = await api('runs', post({ projectId: props.projectId, taskId: task.id, confirmed: true, sourceRunId }))
    tab.value = 'result'; poll()
  } catch (e) { error.value = (e as Error).message } finally { busy.value = false }
}
async function poll() {
  clearTimeout(timer)
  if (!activeRun.value || !['queued', 'running'].includes(activeRun.value.status)) { await load(); return }
  const project = props.projectId, id = activeRun.value.id
  try {
    const run = await api('runs?projectId=' + project + '&id=' + id)
    if (project !== props.projectId || id !== activeRun.value?.id) return
    activeRun.value = run
    if (['queued', 'running'].includes(run.status)) timer = setTimeout(poll, 1000)
    else await load()
  } catch (e) { error.value = '进度读取失败，已完成结果保留：' + (e as Error).message; timer = setTimeout(poll, 2500) }
}
async function openRun(run: ProjectRun) { clearTimeout(timer); activeRun.value = run; tab.value = 'result'; poll() }
function edit(task: ProjectTask) { restoring = true; form.value = { ...JSON.parse(JSON.stringify(task)), password: '', fileData: '' }; variableEditor.value=JSON.stringify(task.initialVariables || {},null,2); stepEditor.value = task.steps.length ? JSON.stringify(task.steps, null, 2) : ''; confirmed.value = false; tab.value = readOnly.value ? 'history' : 'setup'; restoring = false }
async function file(event: Event) {
  try {
    const selected = (event.target as HTMLInputElement).files?.[0]; if (!selected) return
    if (selected.size > 5 * 1024 * 1024) throw new Error('测试样本最多 5 MB。')
    const data = await new Promise<string>((resolve, reject) => { const reader = new FileReader(); reader.onload = () => resolve(String(reader.result).split(',')[1]); reader.onerror = reject; reader.readAsDataURL(selected) })
    form.value.fileName = selected.name; form.value.fileSize = selected.size; form.value.fileData = data
  } catch (e) { error.value = (e as Error).message }
}
async function openapi(event: Event) {
  try {
    const file = (event.target as HTMLInputElement).files?.[0]; if (!file) return
    await props.request('/api/apis/import', post({ projectId: props.projectId, spec: JSON.parse(await file.text()) }))
    form.value.openApiUrl = ''; notice.value = '接口文档已导入当前项目。'; emit('imported')
  } catch (e) { error.value = (e as Error).message }
}
async function stop() { try { if (activeRun.value) await api('cancel', post({ projectId: props.projectId, id: activeRun.value.id })) } catch (e) { error.value = (e as Error).message } }
async function download(format: string) {
  if (!activeRun.value) return
  try {
    const response = await fetch('/api/project-tests/export?projectId=' + props.projectId + '&id=' + activeRun.value.id + '&format=' + format)
    if (response.status === 401) window.dispatchEvent(new Event(authRequiredEvent))
    if (!response.ok) throw new Error('报告导出失败')
    const blob = await response.blob(); const url = URL.createObjectURL(blob); const a = document.createElement('a')
    a.href = url; a.download = 'project-test-' + activeRun.value.id + (format === 'json' ? '.json' : '.md'); a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000)
    notice.value = '报告已开始下载，请查看浏览器下载列表。'
  } catch (e) { error.value = (e as Error).message }
}
watch(() => props.projectId, () => { clearTimeout(timer); form.value = newProjectTask(props.projectId); activeRun.value = null; tasks.value = []; runs.value = []; stepEditor.value = ''; tab.value = readOnly.value ? 'history' : 'setup'; load() })
watch(() => props.initialGoal, goal => { if (goal && goal !== form.value.goal) { form.value.goal = goal; form.value.template = 'ai'; tab.value = readOnly.value ? 'history' : 'setup' } })
watch(() => JSON.stringify([form.value.accountMode, form.value.template, form.value.features, form.value.baseUrl, form.value.openApiUrl, form.value.goal]), (_value, old) => {
  if (!restoring && old && form.value.steps.length) { form.value.steps = []; stepEditor.value = ''; confirmed.value = false; notice.value = '任务范围已变化，请重新生成测试流程。' }
}, { flush: 'sync' })
onMounted(load)
onBeforeUnmount(() => clearTimeout(timer))
const runLabel = (run: ProjectRun) => ['queued', 'running'].includes(run.status) ? '正在执行' : run.summary.passed + '通过 / ' + run.summary.failed + '失败 / ' + run.summary.blocked + '阻塞 / ' + run.summary.notRun + '未执行'
</script>
<template>
  <section class="list-page business-testing">
    <slot name="workspace-nav" />
    <div class="page-title"><div><h1>项目测试</h1><p>{{ projectName }} · 验证已启动项目的核心业务，并保留每次执行证据。</p></div><button v-if="!readOnly" class="send-only" @click="form = newProjectTask(projectId); stepEditor = ''; variableEditor = '{}'; inputs = []; confirmed = false; tab = 'setup'">创建新任务</button></div>
    <nav class="bt-tabs" aria-label="项目测试工作区"><button v-if="!readOnly" :class="{ active: tab === 'setup' }" @click="tab = 'setup'">任务配置</button><button :class="{ active: tab === 'result' }" @click="tab = 'result'">执行结果</button><button :class="{ active: tab === 'history' }" @click="tab = 'history'; load()">已保存任务与记录</button><button v-if="!readOnly" :class="{active:tab==='chain'}" @click="tab='chain'">手动链路</button></nav>
    <slot v-if="tab==='chain'" name="manual-chain" />
    <p v-if="error" class="bt-error" role="alert">{{ error }}</p><p v-if="notice" class="bt-notice" role="status">{{ notice }}</p>
    <template v-if="tab === 'setup'">
      <ProjectTaskSetup v-model="form" :busy="busy" @save="saveOnly" @plan="generate" @file="file" @openapi="openapi"/>
      <details class="bt-section"><summary>补充测试数据（助手提示缺少字段时填写）</summary><p class="bt-hint">填写 JSON 字符串变量，例如邮箱、验证码或测试商品 ID；敏感字段保存后脱敏。账号和文件变量由上方配置生成。</p><textarea v-model="variableEditor" rows="3" placeholder='{"email":"test@example.com"}'></textarea></details>
      <section v-if="form.steps.length" class="bt-section bt-plan">
        <div class="bt-heading"><h2>执行计划 · {{ form.steps.length }} 个步骤</h2><small>生成后可修改请求、依赖和业务检查</small></div>
        <ol class="bt-flow"><li v-for="step in form.steps" :key="step.id"><b>{{ step.name }}</b><span>{{ step.method }} {{ step.url }}</span></li></ol>
        <details><summary>编辑步骤 JSON（高级）</summary><textarea v-model="stepEditor" rows="12" spellcheck="false"></textarea><small>支持变量引用、数组提取、预期错误和下载哈希断言。</small></details>
        <label v-if="form.permission === 'review'" class="bt-confirm"><input v-model="confirmed" type="checkbox">我已核对账号方式、文件来源及本次测试资源范围</label>
        <div class="bt-actions"><button class="primary-btn" :disabled="busy" @click="execute()">{{ busy ? '正在处理…' : form.permission === 'review' ? '确认并开始' : '开始自动执行' }}</button></div>
      </section>
    </template>
    <template v-if="tab === 'result'"><ProjectTaskReport v-if="activeRun" :run="activeRun" @rerun="execute(activeRun.id)" @stop="stop" @download="download"/><div v-else class="bt-empty">还没有选择执行记录。先配置任务，或从历史记录中打开报告。</div></template>
    <div v-if="tab === 'history'" class="bt-form">
      <section v-if="!readOnly" class="bt-section"><h2>已保存的测试任务</h2><p class="bt-hint">保存地址、账号方式、样本、流程和标准；再次运行会产生新记录。</p><div v-for="task in tasks" :key="task.id" class="bt-record"><div><b>{{ task.name }}</b><small>{{ task.baseUrl }} · {{ task.accountMode === 'auto' ? '自动创建账号' : '指定账号' }} · {{ task.steps.length }} 个步骤</small></div><button class="send-only" @click="edit(task)">查看 / 修改 / 再次执行</button></div><p v-if="!tasks.length" class="bt-hint">还没有任务。保存后可跨刷新复用。</p></section>
      <section class="bt-section"><h2>历史执行记录</h2><p class="bt-hint">打开报告只读取原结果，不会重新发起请求。</p><div v-for="run in runs" :key="run.id" class="bt-record"><div><b>{{ run.task.name }}</b><small>{{ new Date(run.createdAt).toLocaleString() }} · {{ runLabel(run) }}</small></div><button class="send-only" @click="openRun(run)">查看报告</button></div><p v-if="!runs.length" class="bt-hint">还没有执行记录。</p></section>
    </div>
    <div v-if="inputs.length" class="modal-mask"><section class="modal-card" role="dialog" aria-modal="true" aria-label="补充测试信息"><h2>助手需要这些测试信息</h2><p class="bt-hint">填写后继续生成或执行当前任务。</p><label v-for="item in inputs" :key="item.field">{{ item.question }}<input v-model="item.value" :type="item.secret ? 'password' : 'text'"></label><p v-if="error" class="bt-error">{{ error }}</p><div class="modal-actions"><button class="send-only" @click="inputs=[]">稍后填写</button><button class="primary-btn" @click="supplyInputs">补充并继续</button></div></section></div>
  </section>
</template>
