<script setup lang="ts">
import type { KnowledgeFeedback } from './feedback'
defineProps<{ feedback:KnowledgeFeedback }>()
const emit=defineEmits<{ repair:[phase:KnowledgeFeedback['phase']] }>()
</script>
<template>
  <section class="knowledge-feedback" :class="feedback.severity==='error'?'bt-error':'knowledge-warning'" role="alert" tabindex="-1" aria-labelledby="knowledge-feedback-title">
    <div class="feedback-heading"><svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 7v6m0 3v1"/></svg><h2 id="knowledge-feedback-title">{{feedback.title}}</h2></div>
    <ul v-if="feedback.reasons.length"><li v-for="reason in feedback.reasons" :key="reason">{{reason}}</li></ul>
    <h3>怎么处理</h3><ol><li v-for="step in feedback.nextSteps" :key="step">{{step}}</li></ol>
    <button type="button" class="send-only" @click="emit('repair',feedback.phase)">{{feedback.phase==='upload'?'重新选择文件':'返回补充资料与预期'}}</button>
  </section>
</template>
<style scoped>
.knowledge-feedback { margin:0 0 22px; padding:18px 20px; border-radius:12px; border:1px solid currentColor; white-space:normal; }
.feedback-heading { display:flex; align-items:flex-start; gap:10px; }
.feedback-heading svg { flex:none; margin-top:3px; }
.feedback-heading h2 { margin:0; font-size:17px; line-height:1.55; color:inherit; }
h3 { margin:14px 0 6px; color:inherit; }
ul,ol { padding-left:24px; margin:8px 0 14px; line-height:1.75; }
.knowledge-warning { background:#fbf1df; color:#7d541c; }
.knowledge-feedback:focus-visible { outline:2px solid var(--blue-dark); outline-offset:3px; }
</style>
