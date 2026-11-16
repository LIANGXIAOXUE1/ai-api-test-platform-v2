<script setup lang="ts">
import { usePermissions } from '../auth/usePermissions'
const { readOnly, canAuto, isAdmin } = usePermissions()
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { defaultSuite, type EvaluationSuite, type BusinessEvaluationRun } from './types'
import EvaluationComparison from './EvaluationComparison.vue'
import '../shared/business-testing.css'
const props = defineProps<{ projectId: number; profiles: any[]; request: (path: string, options?: RequestInit) => Promise<any> }>()
const emit = defineEmits<{ 'add-model': [] }>()
const form = ref(defaultSuite(props.projectId))
const suites = ref<EvaluationSuite[]>([])
const runs = ref<BusinessEvaluationRun[]>([])
const legacy = ref<any[]>([])
const legacyReport = ref<any>(null)
const activeRun = ref<BusinessEvaluationRun | null>(null)
const tab = ref(readOnly.value ? 'history' : 'setup')
const busy = ref(false)
const error = ref('')
const notice = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
const post = (value: unknown) => ({ method: 'POST', body: JSON.stringify(value) })
const api = (path: string, options?: RequestInit) => props.request('/api/evaluation-suites' + path, options)
function pickScenario() {
  const ids = form.value.profileIds
  form.value = defaultSuite(props.projectId, form.value.scenario); form.value.profileIds = ids
}
async function load() {
  try {
    const [saved, history, old] = await Promise.all([api('?projectId=' + props.projectId), api('/runs?projectId=' + props.projectId), props.request('/api/evaluations?projectId=' + props.projectId)])
    suites.value = saved; runs.value = history; legacy.value = old.reports || []
  } catch (e) { error.value = (e as Error).message }
}
async function save() {
  busy.value = true; error.value = ''
  try { form.value.projectId = props.projectId; form.value = await api('', post(form.value)); await load(); notice.value = '测评任务已保存，样本与标准可重复使用。' } catch (e) { error.value = (e as Error).message } finally { busy.value = false }
}
async function execute() {
  busy.value = true; error.value = ''
  try { form.value.projectId = props.projectId; activeRun.value = await api('/runs', post(form.value)); tab.value = 'result'; legacyReport.value = null; poll() }
  catch (e) { error.value = (e as Error).message } finally { busy.value = false }
}
async function poll() {
  clearTimeout(timer)
  if (!activeRun.value || !['queued','running'].includes(activeRun.value.status)) { await load(); return }
  const id = activeRun.value.id, project = props.projectId
  try {
    const run = await api('/runs?projectId=' + project + '&id=' + id)
    if (project !== props.projectId || id !== activeRun.value?.id) return
    activeRun.value = run
    if (['queued','running'].includes(run.status)) timer = setTimeout(poll, 1200); else await load()
  } catch (e) { error.value = (e as Error).message; timer = setTimeout(poll, 2500) }
}
function open(run: BusinessEvaluationRun) { clearTimeout(timer); activeRun.value = run; tab.value = 'result'; legacyReport.value = null; poll() }
function edit(suite: EvaluationSuite) { form.value = JSON.parse(JSON.stringify(suite)); tab.value = readOnly.value ? 'history' : 'setup' }
async function upload(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]; if (!file) return
  if (file.size > 1024 * 1024) { error.value = '业务资料最多 1 MB 文本。'; return }
  form.value.facts += '\n\n' + await file.text(); notice.value = '资料已加入本次测评，保存任务可复用。'
}
async function stop() { try { if (activeRun.value) await api('/cancel', post({ id: activeRun.value.id, projectId: props.projectId })) } catch (e) { error.value = (e as Error).message } }
async function review(value: any) {
  if (!activeRun.value) return
  try { activeRun.value = await api('/review', post({ ...value, id: activeRun.value.id, projectId: props.projectId })); await load(); notice.value = '复核已保存为新报告版本。' } catch (e) { error.value = (e as Error).message }
}
function download(format: string) {
  if (!activeRun.value) return
  const run = activeRun.value
  const lines = ['# ' + run.suite.name, '', '客户目标：' + run.suite.goal, '资料：' + run.suite.facts, '要求：' + run.suite.requirements, '', '结论：' + run.conclusion, '']
  for (const result of run.results) {
    lines.push('## ' + result.profile.name + ' / ' + result.sample.name, '部署名：' + result.profile.deployment, '耗时：' + result.durationMs + 'ms；Token：' + result.inputTokens + '/' + result.outputTokens + '（' + result.usageSource + '）', result.error || result.answer)
    for (const check of result.checks) lines.push('- ' + check.label + '：' + check.status + '；' + check.actual + '；证据：' + check.evidence + '；来源：' + check.source)
  }
  const blob = new Blob([format === 'json' ? JSON.stringify(run,null,2) : lines.join('\n\n')], { type: format === 'json' ? 'application/json' : 'text/markdown;charset=utf-8' })
  const url = URL.createObjectURL(blob); const a = document.createElement('a'); a.href = url; a.download = 'evaluation-' + run.id + (format === 'json' ? '.json' : '.md'); a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000)
  notice.value = '报告已开始下载，请查看浏览器下载列表。'
}
watch(() => props.projectId, () => { clearTimeout(timer); form.value = defaultSuite(props.projectId); activeRun.value = null; legacyReport.value = null; tab.value = readOnly.value ? 'history' : 'setup'; load() })
onMounted(load)
onBeforeUnmount(() => clearTimeout(timer))
</script>
<template>
  <section class="list-page business-testing">
    <slot name="workspace-nav" />
    <div class="page-title"><div><h1>业务场景模型测评</h1><p>依据客户目标和明确标准，对照真实模型输出、耗时和用量。</p></div><button class="send-only" v-if="isAdmin" @click="emit('add-model')">管理 / 添加模型</button></div>
    <nav class="bt-tabs"><button v-if="!readOnly" :class="{active:tab==='setup'}" @click="tab='setup'">客户需求与测试样本</button><button :class="{active:tab==='result'}" @click="tab='result'">测评结果</button><button :class="{active:tab==='history'}" @click="tab='history';load()">已保存任务与记录</button></nav>
    <p v-if="error" class="bt-error" role="alert">{{ error }}</p><p v-if="notice" class="bt-notice" role="status">{{ notice }}</p>
    <div v-if="tab==='setup'" class="bt-form">
      <section class="bt-section">
        <h2>客户需要完成什么？</h2>
        <div class="bt-fields"><label>业务任务<select v-model="form.scenario" @change="pickScenario"><option value="short_video">短视频脚本与分镜</option><option value="knowledge">企业业务知识问答</option><option value="extraction">客户信息提取</option></select></label><label>测评任务名称<input v-model="form.name"></label></div>
        <label>客户目标<input v-model="form.goal"></label><div class="bt-fields" style="margin-top:14px"><label>已知资料<textarea v-model="form.facts" rows="4"></textarea></label><label>必须满足<textarea v-model="form.requirements" rows="4"></textarea></label></div>
        <div v-if="form.scenario==='short_video'" class="bt-fields"><label>分镜数量<input v-model.number="form.shotCount" type="number" min="1" max="30"></label><label>规划时长（秒）<input v-model.number="form.durationSeconds" type="number" min="1"></label></div>
        <label v-if="form.scenario==='extraction'">输出字段（逗号分隔）<input :value="form.outputFields.join(', ')" @input="form.outputFields=($event.target as HTMLInputElement).value.split(',').map(item=>item.trim()).filter(Boolean)"></label>
        <label class="bt-file">补充业务资料（TXT / Markdown）<input type="file" accept=".txt,.md" @change="upload"></label>
      </section>
      <section class="bt-section">
        <div class="bt-heading"><h2>候选模型与评审来源</h2><button class="link-btn" v-if="isAdmin" @click="emit('add-model')">添加模型</button></div>
        <div class="bt-checks"><label v-for="profile in profiles.filter(item=>item.enabled)" :key="profile.id"><input v-model="form.profileIds" type="checkbox" :value="profile.id"><span>{{ profile.name }}<small>{{ profile.provider }} · {{ profile.model }}</small></span></label></div>
        <p v-if="!profiles.length" class="bt-hint">请先添加真实文本模型，密钥在模型池中填写。</p>
        <div class="bt-fields"><label>语义评审模型<select v-model.number="form.judgeProfileId"><option :value="0">使用当前助手模型；不可用时保留待复核</option><option v-for="profile in profiles.filter(item=>item.enabled)" :key="profile.id" :value="profile.id">{{ profile.name }} · {{ profile.model }}</option></select></label><label>每个样本重复次数<select v-model.number="form.repeats"><option :value="1">1 次</option><option :value="2">2 次</option><option :value="3">3 次</option></select></label></div>
        <p class="bt-hint">格式和数量由程序核对；事实和业务要求由模型辅助评价，证据可查看并人工复核。建议选择不同于候选的评审模型。</p>
      </section>
      <section class="bt-section">
        <div class="bt-heading"><h2>测试样本 · {{ form.cases.length }} 条</h2><button class="link-btn" @click="form.cases.push({id:'sample_'+Date.now(),name:'新样本',input:''})">添加样本</button></div>
        <div v-for="(sample,index) in form.cases" :key="sample.id" class="bt-detail"><div class="bt-heading"><label>样本名称<input v-model="sample.name"></label><button class="link-btn" @click="form.cases.splice(index,1)">删除样本</button></div><label>发给候选模型的任务<textarea v-model="sample.input" rows="2"></textarea></label><label>参考要点（只用于评价，不发给候选模型）<textarea v-model="sample.expected" rows="2" :placeholder="form.scenario==='extraction' ? '可填写预期 JSON 字段和值' : '创作任务可留空，按事实和业务要求评价'"></textarea></label></div>
      </section>
      <div class="bt-actions"><span>将调用 {{ form.profileIds.length * form.cases.length * form.repeats }} 个候选样本，另有辅助评审调用。</span><button class="send-only" :disabled="busy" @click="save">保存测评任务</button><button class="primary-btn" :disabled="busy" @click="execute">{{ busy?'正在处理…':'开始比较' }}</button></div>
    </div>
    <template v-if="tab==='result'">
      <EvaluationComparison v-if="activeRun" :run="activeRun" @stop="stop" @export="download" @review="review"/>
      <section v-else-if="legacyReport" class="bt-section"><h2>旧版测评历史</h2><p class="bt-hint">原始回答保留；旧关键词启发式指标不作为业务准确率。</p><div v-for="(item,index) in legacyReport.results" :key="index" class="bt-detail"><h3>{{ item.profileName }} · {{ item.question }}</h3><pre>{{ item.error || item.answer }}</pre><small>{{ item.durationMs }} ms</small></div></section>
      <div v-else class="bt-empty">完成一次测评，或从历史中打开报告。</div>
    </template>
    <div v-if="tab==='history'" class="bt-form">
      <section class="bt-section"><h2>已保存的测评任务</h2><div v-for="suite in suites" :key="suite.id" class="bt-record"><div><b>{{ suite.name }}</b><small>{{ suite.cases.length }} 个样本 · {{ suite.profileIds.length }} 个模型</small></div><button class="send-only" @click="edit(suite)">查看 / 修改 / 再次比较</button></div><p v-if="!suites.length" class="bt-hint">保存后可复用客户资料、样本和标准。</p></section>
      <section class="bt-section"><h2>测评执行记录</h2><div v-for="run in runs" :key="run.id" class="bt-record"><div><b>{{ run.suite.name }}</b><small>{{ new Date(run.createdAt).toLocaleString() }} · 完成 {{ run.completed }}/{{ run.total }}</small></div><button class="send-only" @click="open(run)">查看报告</button></div><p v-if="!runs.length" class="bt-hint">尚无场景测评记录，开始比较后自动保存。</p></section>
      <details v-if="legacy.length" class="bt-section"><summary>旧版测评记录（原回答保留）</summary><div v-for="run in legacy" :key="run.id" class="bt-record"><span>{{ new Date(run.createdAt).toLocaleString() }} · {{ run.results.length }} 条回答</span><button class="send-only" @click="legacyReport=run;activeRun=null;tab='result'">查看原始回答</button></div></details>
    </div>
  </section>
</template>
