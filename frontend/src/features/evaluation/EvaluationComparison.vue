<script setup lang="ts">
import { usePermissions } from '../auth/usePermissions'
const { readOnly, canAuto, isAdmin } = usePermissions()
import { computed, ref, watch, nextTick } from 'vue'
import type { BusinessEvaluationRun } from './types'
const props = defineProps<{ run: BusinessEvaluationRun }>()
const emit = defineEmits<{ stop: []; export: [format: string]; review: [value: any] }>()
const sample = ref('')
const selectedIndex = ref(-1)
const detailElement = ref<HTMLElement>()
watch(selectedIndex, async () => { await nextTick(); detailElement.value?.scrollIntoView({ block: 'nearest', behavior: 'smooth' }) })
const review = ref({ checkId: '', status: 'satisfied', reason: '' })
watch(() => props.run.id, () => { sample.value = props.run.suite.cases[0]?.id || ''; selectedIndex.value = -1 }, { immediate: true })
const candidates = computed(() => Array.from(new Map(props.run.results.map(item => [item.profile.id, item.profile])).values()))
const visibleResults = computed(() => props.run.results.filter(item => item.sample.id === sample.value && item.repeat === 1))
const checks = computed(() => Array.from(new Map(visibleResults.value.flatMap(item => item.checks).map(item => [item.id, item.label])).entries()))
const cell = (profileId: number, checkId: string) => visibleResults.value.find(item => item.profile.id === profileId)?.checks.find((item: any) => item.id === checkId)
const result = (profileId: number) => visibleResults.value.find(item => item.profile.id === profileId)
const status = (value?: string) => ({ satisfied: '满足', not_satisfied: '不满足', unknown: '待复核', call_failed: '调用失败' }[value || ''] || '等待结果')
const selected = computed(() => props.run.results[selectedIndex.value])
function open(profileId: number) { selectedIndex.value = props.run.results.findIndex(item => item.profile.id === profileId && item.sample.id === sample.value && item.repeat === 1) }
</script>
<template>
  <div class="bt-form">
    <section class="bt-section">
      <div class="bt-heading"><div><h2>{{ run.suite.name }}</h2><small>{{ new Date(run.createdAt).toLocaleString() }} · 实际候选模型输出</small></div><div class="bt-actions"><button v-if="!readOnly && ['running','queued'].includes(run.status)" class="send-only" @click="emit('stop')">停止测评</button><button class="send-only" @click="emit('export','markdown')">导出 Markdown</button><button class="send-only" @click="emit('export','json')">导出 JSON</button></div></div>
      <p class="bt-hint">执行人：{{ run.createdBy || '历史记录未记录' }}</p><div class="bt-summary" aria-live="polite"><b>{{ run.status === 'completed' ? '测评已完成' : run.status === 'running' || run.status === 'queued' ? '正在测评' : '测评未完成' }}</b><span>完成 {{ run.completed }} / {{ run.total }}</span><span>调用成功 {{ run.results.filter(item => item.status !== 'call_failed').length }}</span></div>
      <p class="bt-hint">{{ run.conclusion }}</p><p v-if="run.error" class="bt-error">{{ run.error }}</p>
    </section>
    <section class="bt-section">
      <div class="bt-heading"><h2>逐项对照</h2><label>测试样本<select v-model="sample"><option v-for="item in run.suite.cases" :key="item.id" :value="item.id">{{ item.name }}</option></select></label></div>
      <div class="bt-table-wrap"><table class="bt-table"><thead><tr><th>检查项</th><th v-for="profile in candidates" :key="profile.id">{{ profile.name }}<small>{{ profile.deployment }}</small></th></tr></thead><tbody>
        <tr v-for="[id,label] in checks" :key="id"><th>{{ label }}</th><td v-for="profile in candidates" :key="profile.id"><template v-if="cell(profile.id,id)"><span class="bt-status" :class="cell(profile.id,id).status">{{ status(cell(profile.id,id).status) }}</span><small>{{ cell(profile.id,id).actual }}</small></template><span v-else>{{ result(profile.id)?.status === 'call_failed' ? '调用失败' : '等待结果' }}</span></td></tr>
        <tr><th>请求耗时 / Token</th><td v-for="profile in candidates" :key="profile.id"><template v-if="result(profile.id)">{{ result(profile.id).durationMs }} ms<small>输入 {{ result(profile.id).inputTokens }} / 输出 {{ result(profile.id).outputTokens }} · {{ result(profile.id).usageSource === 'provider' ? '上游用量' : '估算用量' }}</small></template></td></tr>
        <tr><th>原始输出和检查证据</th><td v-for="profile in candidates" :key="profile.id"><button class="link-btn" @click="open(profile.id)">查看原文与证据</button></td></tr>
      </tbody></table></div>
      <p v-if="!run.results.length" class="bt-hint">等待候选模型返回；结果逐条保存。</p>
      <p class="bt-hint">辅助评价会标注评审来源；证据不能定位或评审调用失败时，保留为待复核。当前未配置渠道价格，报告不推算费用。</p>
    </section>
    <section v-if="selected" ref="detailElement" class="bt-section">
      <h2>{{ selected.profile.name }} · {{ selected.sample.name }}</h2><p v-if="selected.error" class="bt-error">{{ selected.error }}</p><pre>{{ selected.answer || '未返回内容' }}</pre>
      <div class="bt-table-wrap"><table class="bt-table"><thead><tr><th>检查项</th><th>来源</th><th>依据</th><th>结果</th></tr></thead><tbody><tr v-for="check in selected.checks" :key="check.id"><td>{{ check.label }}</td><td>{{ check.source }}<small>{{ selected.judgeName }}</small></td><td>{{ check.actual }}<small v-if="check.evidence">原文：“{{ check.evidence }}”</small></td><td>{{ status(check.status) }}</td></tr></tbody></table></div>
      <details v-if="run.status !== 'running' && run.status !== 'queued'"><summary>人工复核辅助评价项</summary>
        <div class="bt-fields"><label>检查项<select v-model="review.checkId"><option value="">请选择</option><option v-for="check in selected.checks.filter((item:any)=>item.source !== '程序校验')" :key="check.id" :value="check.id">{{ check.label }}</option></select></label><label>判断<select :disabled="readOnly" v-model="review.status"><option value="satisfied">满足要求</option><option value="not_satisfied">不满足要求</option></select></label></div>
        <label>复核依据<textarea :readonly="readOnly" v-model="review.reason" rows="2" placeholder="说明预期和对应输出证据"></textarea></label><div class="bt-actions"><button class="send-only" v-if="!readOnly" @click="emit('review',{...review,result:selectedIndex})">保存复核版本</button></div><small>保存为新报告版本，原记录保留。</small>
      </details>
    </section>
    <details class="bt-section"><summary>全部样本与重复结果</summary><div v-for="(item,index) in run.results" :key="index" class="bt-record"><div><b>{{ item.profile.name }} · {{ item.sample.name }} · 第{{ item.repeat }}次</b><small>{{ item.status === 'call_failed' ? item.error : item.checks.filter((c:any)=>c.status==='satisfied').length + '/' + item.checks.length + ' 项满足' }}</small></div><button class="send-only" @click="selectedIndex=index">查看</button></div></details>
    <p class="bt-hint">本结论仅适用于所选任务与本次配置；脚本规划时长不代表真实配音时长或视频成片质量。</p>
  </div>
</template>
