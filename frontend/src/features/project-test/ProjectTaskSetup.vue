<script setup lang="ts">
import type { ProjectTask } from './taskTypes'
const task = defineModel<ProjectTask>({ required: true })
defineProps<{ busy: boolean }>()
const emit = defineEmits<{ save: []; plan: []; file: [event: Event]; openapi: [event: Event] }>()
</script>
<template>
  <div class="bt-form">
    <section class="bt-section">
      <h2>测试目标与接口文档</h2>
      <div class="bt-fields">
        <label>任务名称<input v-model="task.name" placeholder="例如：云盘核心业务测试"></label>
        <label>已启动项目地址<input v-model="task.baseUrl" placeholder="http://localhost:8082"></label>
        <label>流程来源<select v-model="task.template"><option value="cloud_disk">Cloud Disk 核心流程 · 预置验收模板</option><option value="ai">由当前助手模型根据接口文档生成</option></select></label>
        <label>OpenAPI 地址（也可使用已导入接口）<input v-model="task.openApiUrl" placeholder="http://localhost:8082/v3/api-docs"></label>
      </div>
      <label>希望验证什么？<textarea v-model="task.goal" rows="2" placeholder="说明核心业务和预期结果"></textarea></label>
      <label class="bt-file">已有 OpenAPI 文件？<input type="file" accept=".json,application/json" @change="emit('openapi', $event)"><small>导入到当前项目；AI 流程也可以读取当前项目接口。</small></label>
    </section>
    <div class="bt-columns">
      <section class="bt-section">
        <h2>测试账号怎么准备？</h2>
        <label class="bt-choice"><input v-model="task.accountMode" type="radio" value="auto"><span>自动生成并注册新账号<small>每次完整运行生成新账号，注册后自动登录。</small></span></label>
        <label class="bt-choice"><input v-model="task.accountMode" type="radio" value="register"><span>我填写新账号，由平台注册</span></label>
        <label class="bt-choice"><input v-model="task.accountMode" type="radio" value="existing"><span>使用已有账号，直接登录</span></label>
        <div v-if="task.accountMode !== 'auto'" class="bt-fields">
          <label>用户名<input v-model="task.username" autocomplete="off"></label>
          <label>密码<input v-model="task.password" type="password" autocomplete="new-password" :placeholder="task.hasPassword ? '已加密保存，留空继续使用' : '填写测试账号密码'"></label>
        </div>
        <p v-else class="bt-hint">平台生成账号与密码，Token 在本次流程内自动传递，报告脱敏显示。</p>
      </section>
      <section class="bt-section">
        <h2>上传文件怎么准备？</h2>
        <label class="bt-choice"><input v-model="task.fileMode" type="radio" value="auto"><span>平台生成 2 KB 文本样本<small>上传后下载，校验 SHA256 是否与原文件一致。</small></span></label>
        <label class="bt-choice"><input v-model="task.fileMode" type="radio" value="manual"><span>使用我提供的测试文件</span></label>
        <label v-if="task.fileMode === 'manual'" class="bt-file">测试样本（最多 5 MB）<input type="file" @change="emit('file', $event)"><small>{{ task.fileName ? task.fileName + ' · ' + task.fileSize + ' 字节' : '请选择文件；保存任务后可复用' }}</small></label>
        <p v-else class="bt-hint">文件夹和文件使用本次唯一名称，不需要手工准备测试数据。</p>
      </section>
    </div>
    <section class="bt-section">
      <h2>业务范围与执行方式</h2>
      <div class="bt-checks">
        <label><input v-model="task.features" type="checkbox" value="folder">创建文件夹与重名检查</label>
        <label><input v-model="task.features" type="checkbox" value="file">上传、查询、下载校验</label>
        <label><input v-model="task.features" type="checkbox" value="lifecycle">重命名、删除与还原</label>
        <label><input v-model="task.features" type="checkbox" value="auth">未登录访问检查</label>
      </div>
      <p class="bt-hint">写操作限定于本次创建的测试目录和文件。失败资源保留用于排查；不自动清空回收站或删除账号。</p>
      <div class="bt-checks">
        <label><input v-model="task.permission" type="radio" value="review">执行前确认整套流程</label>
        <label><input v-model="task.permission" type="radio" value="auto">按本任务范围自动执行</label>
      </div>
    </section>
    <div class="bt-actions"><button class="send-only" :disabled="busy" @click="emit('save')">保存任务</button><button class="primary-btn" :disabled="busy" @click="emit('plan')">{{ busy ? '正在处理…' : '生成测试流程' }}</button></div>
  </div>
</template>
