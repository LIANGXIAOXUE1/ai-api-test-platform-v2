<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
const props = defineProps<{ run: any }>()
const emit = defineEmits<{ close: [] }>()
const dialog = ref<HTMLElement>()
const previousFocus = document.activeElement as HTMLElement | null
onMounted(() => dialog.value?.focus())
onUnmounted(() => previousFocus?.focus())
function trapFocus(event: KeyboardEvent) {
  if (event.key !== 'Tab') return
  const buttons = dialog.value?.querySelectorAll<HTMLButtonElement>('button')
  if (!buttons?.length) return
  const first = buttons[0], last = buttons[buttons.length - 1]
  if (event.shiftKey && (document.activeElement === first || document.activeElement === dialog.value)) { event.preventDefault(); last.focus() }
  else if (!event.shiftKey && (document.activeElement === last || document.activeElement === dialog.value)) { event.preventDefault(); first.focus() }
}
function download() {
  const url = URL.createObjectURL(new Blob([JSON.stringify(props.run, null, 2)], { type: 'application/json;charset=utf-8' }))
  const link = document.createElement('a')
  link.href = url; link.download = `run-${props.run.id}-readonly.json`; link.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
</script>
<template>
  <div class="readonly-scrim" @click.self="emit('close')"><section ref="dialog" @keydown="trapFocus" role="dialog" aria-modal="true" aria-labelledby="readonly-run-title" class="readonly-dialog" tabindex="-1" @keydown.esc="emit('close')"><header><div><h2 id="readonly-run-title">{{ run.caseName || '执行记录' }}</h2><p>只读报告 · 原始请求、响应和凭据已隐藏</p></div><div class="dialog-actions"><button type="button" @click="download">导出 JSON</button><button type="button" @click="emit('close')">关闭</button></div></header><p>执行人：{{ run.createdBy || '历史记录未记录' }}</p><dl><div><dt>结果</dt><dd>{{ run.success ? '通过' : '失败' }}</dd></div><div><dt>状态码</dt><dd>{{ run.statusCode }}</dd></div><div><dt>耗时</dt><dd>{{ run.durationMs }} ms</dd></div><div><dt>断言通过</dt><dd>{{ run.assertionPass }} / {{ run.assertionTotal }}</dd></div></dl><pre>{{ JSON.stringify(run,null,2) }}</pre></section></div>
</template>
<style scoped>
.readonly-scrim { position: fixed; inset: 0; z-index: 600; background: rgba(18,50,74,.35); display: grid; place-items: center; padding: 24px; }
.readonly-dialog { width: min(860px,100%); max-height: calc(100dvh - 48px); overflow: auto; background: #fff; color: #12324a; padding: 24px; border-radius: 18px; box-shadow: 0 12px 36px rgba(18,50,74,.15); }
header { display: flex; justify-content: space-between; align-items: flex-start; gap: 24px; } h2 { margin: 0; font-size: 22px; } p { color: #536b7b; font-size: 14px; } button { background: #fff; color: #365266; border: 1px solid #bccbd3; border-radius: 8px; padding: 8px 14px; cursor: pointer; }
dl { display: flex; flex-wrap: wrap; gap: 28px; } dt { color: #536b7b; font-size: 13px; } dd { margin: 6px 0; font-weight: 600; }
pre { white-space: pre-wrap; overflow-wrap: anywhere; background: #f1f5f7; padding: 18px; border-radius: 10px; font-size: 13px; line-height: 1.7; }
</style>

<style scoped>
.dialog-actions { display: flex; flex-wrap: wrap; gap: 8px; }
button:focus-visible,.readonly-dialog:focus-visible { outline: 2px solid #267969; outline-offset: 2px; }
</style>
