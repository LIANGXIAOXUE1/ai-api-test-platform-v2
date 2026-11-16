<script setup lang="ts">
import type { ProjectCheckPermission, ProjectCheckResult } from './types'

defineProps<{ file: File | null; loading: boolean; result: ProjectCheckResult | null }>()
const baseUrl = defineModel<string>('baseUrl', { default: '' })
const task = defineModel<string>('task', { default: '' })
const permission = defineModel<ProjectCheckPermission>('permission', { default: 'review' })
const emit = defineEmits<{ run: []; download: []; 'select-file': [event: Event] }>()
</script>

<template>
  <section class="list-page project-check-page">
    <div class="page-title project-check-title">
      <div>
        <h1>项目巡检 Agent</h1>
        <p>一句话描述任务，平台会先生成审核计划，或只对当前项目发起不带凭证的安全联通巡检。</p>
      </div>
      <span class="config-state" :class="permission === 'auto' ? 'configured' : 'pending'">{{ permission === 'auto' ? '安全自动执行' : '人工审核模式' }}</span>
    </div>
    <div class="project-check-layout">
      <section class="project-check-composer">
        <label>你希望助手完成什么？
          <textarea v-model="task" rows="4" placeholder="例如：接口文档在这里，项目已启动，请检查所有接口是否联通并导出报告。"></textarea>
        </label>
        <div class="project-check-fields">
          <label>已启动项目的根地址
            <input v-model="baseUrl" placeholder="例如 http://localhost:8082" />
          </label>
          <label class="project-check-upload">接口文档（可选，未导入时选择）
            <input type="file" accept=".json,application/json" @change="emit('select-file', $event)" />
            <small>{{ file ? file.name : '支持 OpenAPI JSON；上传后自动导入接口列表' }}</small>
          </label>
        </div>
        <fieldset class="permission-picker">
          <legend>执行权限</legend>
          <label :class="{ selected: permission === 'review' }">
            <input v-model="permission" type="radio" value="review" />
            <span>
              <b>需要人工审核</b>
              <small>只生成完整计划，不会发起任何请求。</small>
            </span>
          </label>
          <label :class="{ selected: permission === 'auto' }">
            <input v-model="permission" type="radio" value="auto" />
            <span>
              <b>安全自动执行</b>
              <small>仅测试当前项目中的只读接口；不会携带 Token、请求体或调用外部地址。</small>
            </span>
          </label>
        </fieldset>
        <div class="project-check-actions">
          <button class="primary-btn" :disabled="loading" @click="emit('run')">{{ loading ? '正在处理…' : permission === 'auto' ? '开始安全巡检' : '生成审核计划' }}</button>
          <small>高风险写入、删除、上传和真实业务链路始终保留给人工审核。</small>
        </div>
      </section>
      <section class="project-check-output">
        <template v-if="result">
          <div class="project-check-result-head">
            <div>
              <h2>{{ result.executed ? '巡检结果' : '待审核执行计划' }}</h2>
              <p>{{ result.executed ? '已保存巡检记录，并按接口给出联通结论。' : '当前未发起请求；选择安全自动执行后才会运行只读巡检。' }}</p>
            </div>
            <button class="send-only" @click="emit('download')">导出报告</button>
          </div>
          <div class="project-check-stats">
            <div><span>接口总数</span><b>{{ result.summary.total }}</b></div>
            <div><span>已联通</span><b>{{ result.summary.reachable }}</b></div>
            <div><span>不可达</span><b>{{ result.summary.unavailable }}</b></div>
            <div><span>待审核</span><b>{{ result.summary.pending }}</b></div>
          </div>
          <div class="project-check-list">
            <div v-for="item in result.items" :key="item.apiId" class="project-check-row">
              <span class="method" :class="item.method.toLowerCase()">{{ item.method }}</span>
              <div>
                <b>{{ item.name }}</b>
                <small>{{ item.url }}</small>
              </div>
              <span class="project-check-risk">{{ item.risk }}</span>
              <span
                class="project-check-state"
                :class="item.reachable ? 'ok' : item.decision === '已执行' ? 'fail' : 'pending'"
              >{{ item.reachable ? '已联通' : item.decision === '已执行' ? '不可达' : '待审核' }}</span>
            </div>
          </div>
        </template>
        <div v-else class="project-check-empty">
          <b>还没有巡检任务</b>
          <span>填入项目地址；首次使用时选择 OpenAPI 文件，然后只需选择权限并点击一次。</span>
        </div>
      </section>
    </div>
  </section>
</template>
