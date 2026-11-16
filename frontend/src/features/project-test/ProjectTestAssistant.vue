<script setup lang="ts">
import { computed } from 'vue'
import type { AssistantProjectCheckDraft, ProjectCheckItem, ProjectCheckResult } from './types'

const props = defineProps<{
  draft: AssistantProjectCheckDraft | null
  result: ProjectCheckResult | null
  reports: ProjectCheckResult[]
  loading: boolean
  reviewOpen: boolean
}>()
const emit = defineEmits<{
  confirm: []
  download: []
  review: [item: ProjectCheckItem]
  restore: [report: ProjectCheckResult]
  'update:reviewOpen': [value: boolean]
}>()
const pendingItems = computed(() =>
  (props.result?.items || []).filter((item) => !item.reachable && item.decision !== '已执行'),
)
</script>

<template>
  <section v-if="draft || result" class="assistant-project-check-card">
    <div class="draft-card-title">
      <b>{{ result ? '项目巡检结果' : '项目巡检计划' }}</b>
      <span>{{ result ? '已完成' : '等待确认' }}</span>
    </div>
    <p v-if="draft">目标地址：{{ draft.baseUrl }}。仅对 GET、HEAD、OPTIONS 做无凭证探测，其他接口将列为待审核。</p>
    <div v-if="result" class="assistant-check-stats">
      <span>已联通 <b>{{ result.summary.reachable }}</b></span>
      <span>不可达 <b>{{ result.summary.unavailable }}</b></span>
      <span>待审核 <b>{{ result.summary.pending }}</b></span>
    </div>
    <div class="assistant-check-actions">
      <button v-if="draft" class="primary-btn" :disabled="loading" @click="emit('confirm')">确认并开始巡检</button>
      <button v-if="result" class="send-only" @click="emit('download')">下载 Markdown 报告</button>
      <button
        v-if="pendingItems.length"
        class="send-only"
        @click="emit('update:reviewOpen', !reviewOpen)"
      >{{ reviewOpen ? '收起待审核接口' : '审核待测试接口（' + pendingItems.length + '）' }}</button>
      <div v-if="reviewOpen" class="assistant-review-list">
        <div v-for="item in pendingItems" :key="item.apiId" class="assistant-review-row">
          <span class="method" :class="item.method.toLowerCase()">{{ item.method }}</span>
          <div>
            <b>{{ item.name }}</b>
            <small>{{ item.risk }} · {{ item.reason || '需要人工配置测试数据' }}</small>
          </div>
          <button class="link-btn" @click="emit('review', item)">带我配置</button>
        </div>
      </div>
    </div>
  </section>
  <details v-if="reports.length" class="assistant-report-history">
    <summary>巡检记录（{{ reports.length }}）</summary>
    <button
      v-for="report in reports.slice(0, 5)"
      :key="report.id || report.createdAt"
      class="assistant-report-row"
      @click="emit('restore', report)"
    >
      <span>{{ new Date(report.createdAt || Date.now()).toLocaleString() }}</span>
      <b>已联通 {{ report.summary.reachable }} · 待审核 {{ report.summary.pending }}</b>
    </button>
  </details>
</template>
