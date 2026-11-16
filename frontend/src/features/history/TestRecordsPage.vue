<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import '../shared/business-testing.css'
type Run = { id: number; caseId: number; caseName: string; startedAt: string; statusCode: number; durationMs: number; success: boolean; errorCategory: string }
type Case = { id: number }
type Difference = { hasBaseline?: boolean; runCount?: number; baseline?: Run; latest?: Run; diff?: { message: string; changes?: { path: string; message: string }[] }[] }
const props = defineProps<{ runs: Run[]; cases: Case[]; detail: Difference | null; activeTab: 'history' | 'regression'; busy: boolean }>()
const emit = defineEmits<{ 'update:activeTab': [value: 'history' | 'regression']; create: []; view: [run: Run]; replay: [run: Run]; compare: [run: Run]; 'replay-compare': [run: Run]; edit: [id: number] }>()
const latestCases = computed(() => {
  const latest = new Map<number, Run>()
  for (const run of props.runs) {
    if (!run.caseId) continue
    const previous = latest.get(run.caseId)
    if (!previous || new Date(run.startedAt).getTime() > new Date(previous.startedAt).getTime() || (run.startedAt === previous.startedAt && run.id > previous.id)) latest.set(run.caseId, run)
  }
  return [...latest.values()].sort((a,b) => b.id - a.id)
})
const successRate = computed(() => props.runs.length ? Math.round(props.runs.filter(run => run.success).length / props.runs.length * 100) : 0)
const differencePanel = ref<HTMLElement | null>(null)
watch(() => props.detail, async detail => {
  if (!detail) return
  await nextTick()
  differencePanel.value?.scrollIntoView({ block: 'start', behavior: 'instant' })
})
</script>

<template>
  <section class="list-page business-testing test-records-page">
    <div class="page-title"><div><h1>测试记录</h1><p>查看单接口和手动链路请求的历史，或重新执行保存用例并对比结果。</p></div><button class="primary-btn" @click="emit('create')">新建测试</button></div>
    <nav class="bt-tabs" aria-label="测试记录工作区"><button :class="{active:activeTab==='history'}" :aria-current="activeTab==='history' ? 'page' : undefined" @click="emit('update:activeTab','history')">执行历史</button><button :class="{active:activeTab==='regression'}" :aria-current="activeTab==='regression' ? 'page' : undefined" @click="emit('update:activeTab','regression')">保存用例与回归对比</button></nav>
    <p class="bt-hint records-count">{{ runs.length }} 次执行 · 通过率 {{ successRate }}% · {{ latestCases.length }} 个已执行的保存用例</p>
    <section v-if="activeTab==='history'" class="bt-section">
      <div class="bt-table-wrap"><table class="bt-table"><thead><tr><th>结果</th><th>测试名称</th><th>状态码</th><th>耗时</th><th>执行时间</th><th>操作</th></tr></thead><tbody><tr v-for="item in runs" :key="item.id"><td><span class="bt-status" :class="item.success?'passed':'failed'">{{ item.success?'通过':'失败' }}</span></td><td><b>{{ item.caseName }}</b><small>{{ item.errorCategory || '请求正常' }}</small></td><td>{{ item.statusCode || 'ERR' }}</td><td>{{ item.durationMs }} ms</td><td>{{ new Date(item.startedAt).toLocaleString() }}</td><td class="records-operations"><button class="link-btn" @click="emit('view',item)">查看结果</button><button class="link-btn" :disabled="busy" @click="emit('replay',item)">重新执行</button><button v-if="item.caseId && cases.some(testCase=>testCase.id===item.caseId)" class="link-btn" @click="emit('edit',item.caseId)">编辑用例</button></td></tr></tbody></table></div>
      <p v-if="!runs.length" class="bt-hint">暂无执行记录，发送测试请求后自动保存。</p>
    </section>
    <template v-else>
      <p class="bt-hint">每个保存用例显示最近一次结果。“查看差异”读取已有记录；“重新执行并比较”会发送新请求。</p>
      <section class="bt-section"><div class="bt-table-wrap"><table class="bt-table"><thead><tr><th>保存用例</th><th>最近结果</th><th>状态码</th><th>耗时</th><th>操作</th></tr></thead><tbody><tr v-for="item in latestCases" :key="item.caseId"><td><b>{{ item.caseName }}</b><small>用例 #{{ item.caseId }}</small></td><td><span class="bt-status" :class="item.success?'passed':'failed'">{{ item.success?'通过':'失败' }}</span></td><td>{{ item.statusCode || 'ERR' }}</td><td>{{ item.durationMs }} ms</td><td class="records-operations"><button class="link-btn" :disabled="busy" @click="emit('compare',item)">查看差异</button><button class="link-btn" :disabled="busy" @click="emit('replay-compare',item)">重新执行并比较</button></td></tr></tbody></table></div><p v-if="!latestCases.length" class="bt-hint">先在测试台“发送并保存”，再从这里重用测试用例。</p></section>
      <section v-if="detail" ref="differencePanel" class="bt-section bt-detail records-difference" aria-live="polite"><div class="bt-heading"><h2>前后结果差异<span v-if="detail.latest"> · {{ detail.latest.caseName }}</span></h2><span class="bt-status" :class="detail.hasBaseline===false || detail.diff?.length?'blocked':'passed'">{{ detail.hasBaseline===false?'尚无对比基线':detail.diff?.length?'发现变化':'没有明显变化' }}</span></div>
        <p v-if="detail.hasBaseline===false" class="bt-hint">这个用例目前只有一次执行记录。再次执行后，可与第一次结果比较。</p>
        <template v-else>
          <div v-if="detail.baseline && detail.latest" class="bt-summary"><span>基线 #{{ detail.baseline.id }}：{{ detail.baseline.statusCode || 'ERR' }} · {{ detail.baseline.durationMs }} ms</span><span>最近 #{{ detail.latest.id }}：{{ detail.latest.statusCode || 'ERR' }} · {{ detail.latest.durationMs }} ms</span></div>
          <div v-for="(item,index) in detail.diff" :key="index" class="records-change"><b>{{ item.message }}</b><p v-for="change in item.changes" :key="change.path" class="bt-hint">{{ change.message }}</p></div><p v-if="!detail.diff?.length" class="bt-hint">基线和最新记录没有明显变化。请结合用例预期判断是否通过。</p>
        </template>
      </section>
    </template>
  </section>
</template>
