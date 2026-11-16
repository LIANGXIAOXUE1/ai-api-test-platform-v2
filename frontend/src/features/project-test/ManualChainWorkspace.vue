<script setup lang="ts">
import '../shared/business-testing.css'
type Step = { apiId: number; name: string; headers: string; body: string; extract: string }
type Endpoint = { id: number; name: string; method: string }
type Result = { success: boolean; failedAt: number; steps: { id: number; caseName: string; statusCode: number; durationMs: number; success: boolean; errorCategory: string }[]; variables: Record<string, string>; variableCount: number }
defineProps<{ steps: Step[]; endpoints: Endpoint[]; variables: string; result: Result | null; busy: boolean }>()
const emit = defineEmits<{ 'update:variables': [value: string]; add: []; remove: [index: number]; sync: [step: Step]; run: [] }>()
</script>

<template>
  <div class="manual-chain-workspace">
    <p class="bt-hint">手动组合已导入的接口，提取 Token、订单号等变量并传给下一步。自动准备账号、文件和业务检查请使用“任务配置”。</p>
    <div class="bt-columns">
      <section class="bt-section">
        <div class="bt-heading"><h2>链路步骤</h2><button class="send-only" :disabled="busy" @click="emit('add')">添加步骤</button></div>
        <label>初始变量（每行 key: value）<textarea :value="variables" rows="3" placeholder="token: 已有的测试 Token" @input="emit('update:variables', ($event.target as HTMLTextAreaElement).value)"></textarea></label>
        <p v-if="!steps.length" class="bt-hint">还没有步骤，添加接口后可配置请求和变量提取。</p>
        <div v-for="(step, index) in steps" :key="index" class="manual-chain-step">
          <div class="bt-heading"><h3>步骤 {{ index + 1 }}</h3><button class="link-btn" :disabled="busy" :aria-label="'删除步骤 ' + (index + 1)" @click="emit('remove', index)">删除</button></div>
          <label>接口<select v-model.number="step.apiId" :disabled="busy || !endpoints.length" @change="emit('sync', step)"><option v-if="!endpoints.length" :value="0" disabled>请先到测试台添加接口</option><option v-for="item in endpoints" :key="item.id" :value="item.id">{{ item.name }} · {{ item.method }}</option></select></label>
          <label>请求头覆盖<textarea v-model="step.headers" rows="2" :disabled="busy" placeholder="Authorization: Bearer {{token}}"></textarea></label>
          <label>请求体<textarea v-model="step.body" rows="3" :disabled="busy" placeholder="可使用 {{上一步变量}}"></textarea></label>
          <label>提取响应变量<textarea v-model="step.extract" rows="2" :disabled="busy" placeholder="token: data.token"></textarea></label>
        </div>
        <div class="bt-actions manual-chain-actions"><button class="primary-btn" :disabled="busy || !steps.length" @click="emit('run')">{{ busy ? '执行中…' : '执行整条链路' }}</button></div>
      </section>
      <section class="bt-section">
        <div class="bt-heading"><h2>执行结果</h2><span v-if="result" class="bt-status" :class="result.success ? 'passed' : 'failed'">{{ result.success ? '全部通过' : '第 ' + result.failedAt + ' 步失败' }}</span></div>
        <template v-if="result">
          <div v-for="(item,index) in result.steps" :key="item.id" class="bt-record"><div><b>{{ index + 1 }}. {{ item.caseName }}</b><small>{{ item.statusCode || 'ERR' }} · {{ item.durationMs }} ms · {{ item.errorCategory || '请求完成' }}</small></div><span class="bt-status" :class="item.success ? 'passed' : 'failed'">{{ item.success ? '通过' : '失败' }}</span></div>
          <details class="bt-detail" open><summary>本次提取变量（{{ result.variableCount }}）</summary><pre>{{ JSON.stringify(result.variables, null, 2) }}</pre><small>敏感变量在结果中脱敏。各步骤请求记录可在“测试记录”查看。</small></details>
        </template>
        <p v-else class="bt-hint">执行后在这里查看每一步的状态和提取变量。</p>
      </section>
    </div>
  </div>
</template>
