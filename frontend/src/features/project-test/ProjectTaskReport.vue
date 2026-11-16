<script setup lang="ts">
import { usePermissions } from '../auth/usePermissions'
const { readOnly, canAuto, isAdmin } = usePermissions()
import { computed, ref, watch, nextTick } from 'vue'
import type { ProjectRun } from './taskTypes'
const props = defineProps<{ run: ProjectRun }>()
const emit = defineEmits<{ rerun: []; stop: []; download: [format: string] }>()
const selected = ref('')
const detailElement = ref<HTMLElement>()
watch(selected, async () => { await nextTick(); detailElement.value?.scrollIntoView({ block: 'nearest', behavior: 'smooth' }) })
watch(() => props.run.id, () => { selected.value = '' })
const detail = computed(() => props.run.results.find(item => item.id === selected.value))
const active = computed(() => ['queued', 'running'].includes(props.run.status))
const verdict = computed(() => active.value ? '正在执行' : props.run.status !== 'completed' ? '本次执行未完成' : props.run.summary.failed || props.run.summary.blocked || props.run.summary.notRun ? '核心流程未全部通过' : '本次所列核心流程通过')
</script>
<template>
  <div class="bt-form">
    <div class="bt-section">
      <div class="bt-heading"><div><h2>{{ run.task.name }}</h2><p class="bt-hint">{{ run.task.baseUrl }} · {{ new Date(run.createdAt).toLocaleString() }}</p></div><div class="bt-actions"><button v-if="active && !readOnly" class="send-only" @click="emit('stop')">停止任务</button><button v-else-if="!readOnly && (run.summary.failed || run.summary.blocked || run.summary.notRun)" class="send-only" @click="emit('rerun')">重跑失败及依赖步骤</button><button class="send-only" @click="emit('download', 'markdown')">导出 Markdown</button><button class="send-only" @click="emit('download', 'json')">导出 JSON</button></div></div>
      <p class="bt-hint">执行人：{{ run.createdBy || '历史记录未记录' }}</p><div class="bt-summary" aria-live="polite"><b>{{ verdict }}</b><span>总计 {{ run.summary.total }}</span><span>通过 {{ run.summary.passed }}</span><span>失败 {{ run.summary.failed }}</span><span>阻塞 {{ run.summary.blocked }}</span><span>未执行 {{ run.summary.notRun }}</span></div>
      <p v-if="active && !readOnly" class="bt-hint">当前操作：{{ run.currentStep || '任务排队中' }}。已完成结果持续保存。</p>
      <p v-if="run.error" class="bt-error">{{ run.error }}</p>
    </div>
    <section class="bt-section">
      <h2>逐项结果与证据</h2>
      <div class="bt-table-wrap"><table class="bt-table"><thead><tr><th>检查步骤</th><th>结果</th><th>响应</th><th>证据</th></tr></thead><tbody>
        <tr v-for="item in run.results" :key="item.id"><td>{{ item.name }}<small v-if="item.reused">沿用原运行通过结果</small></td><td><span class="bt-status" :class="item.status">{{ ({passed:'通过',failed:'失败',blocked:'阻塞'})[item.status] || item.status }}</span></td><td>{{ item.statusCode || '未请求' }}<small>{{ item.durationMs || 0 }} ms</small></td><td><button class="link-btn" @click="selected = item.id">查看详情</button></td></tr>
        <tr v-if="!run.results.length"><td colspan="4">执行后会持续显示每一步的结果。</td></tr>
      </tbody></table></div>
      <section v-if="detail" ref="detailElement" class="bt-detail">
        <h3>{{ detail.name }}</h3><p v-if="detail.error" class="bt-error">{{ detail.error }}</p>
        <div class="bt-table-wrap"><table class="bt-table"><thead><tr><th>检查</th><th>预期</th><th>实际</th><th>结果</th></tr></thead><tbody><tr v-for="(check, index) in detail.checks" :key="index"><td>{{ check.type }} {{ check.path }}</td><td>{{ check.expected }}</td><td>{{ check.actual }}</td><td>{{ check.passed ? '通过' : '失败' }}</td></tr></tbody></table></div>
        <details><summary>请求、响应与变量提取（脱敏）</summary><pre>{{ detail.method }} {{ detail.url }}
Headers: {{ JSON.stringify(detail.requestHeaders, null, 2) }}
Body: {{ detail.requestBody || '无' }}
Response: {{ detail.responseBody || '未请求' }}
Extracted: {{ JSON.stringify(detail.extracted, null, 2) }}</pre></details>
      </section>
    </section>
    <details class="bt-section"><summary>本次变量与范围</summary><p class="bt-hint">仅验证所列接口流程；页面交互、未覆盖功能及性能/安全专项不在本次结论范围内。</p><pre>{{ JSON.stringify(run.variables, null, 2) }}</pre></details>
  </div>
</template>
