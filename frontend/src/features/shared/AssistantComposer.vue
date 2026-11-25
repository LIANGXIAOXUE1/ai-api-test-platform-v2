<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue'
import AssistantScope from './AssistantScope.vue'
const props=defineProps<{ modelValue:string; endpoints:any[]; profiles:any[]; apiId:number; profileId:number; defaultModel:string; permission:'default'|'project_full'; canAuto:boolean; readOnly:boolean; busy:boolean; documentName:string; documentStatus:string }>()
const emit=defineEmits<{ 'update:modelValue':[value:string]; 'update:apiId':[id:number]; 'update:profileId':[id:number]; 'update:permission':[permission:'default'|'project_full']; 'permission-change':[]; submit:[]; suggestion:[text:string]; 'project-test':[]; upload:[event:Event]; 'clear-document':[] }>()
const input=ref<HTMLTextAreaElement|null>(null)
function resize(){if(input.value){input.value.style.height='auto';input.value.style.height=Math.min(176,Math.max(96,input.value.scrollHeight))+'px'}}
function edit(event:Event){emit('update:modelValue',(event.target as HTMLTextAreaElement).value);resize()}
function submit(){if(!props.busy&&!props.readOnly&&props.modelValue.trim())emit('submit')}
function enter(event:KeyboardEvent){if(event.isComposing||event.keyCode===229)return;event.preventDefault();submit()}
watch(()=>props.modelValue,()=>nextTick(resize));onMounted(resize)
</script>
<template>
  <section class="assistant-chat-entry" aria-label="给测试助手发送任务">
    <div v-if="!readOnly" class="assistant-chat-suggestions" aria-label="常用操作">
      <span>常用操作</span><button type="button" :disabled="busy" @click="emit('project-test')">项目业务测试</button><button type="button" :disabled="busy" @click="emit('suggestion','测试这个接口是否可用，并检查返回 JSON')">检查接口</button><button type="button" :disabled="busy" @click="emit('suggestion','添加一个用户登录接口，POST /api/login，请求体包含 email 和 password')">添加登录接口</button><button type="button" :disabled="busy" @click="emit('suggestion','添加一个 Azure OpenAI 模型接口，POST /openai/deployments/demo/chat/completions')">添加模型接口</button><button type="button" :disabled="busy" @click="emit('suggestion','项目已启动，请检查所有接口是否联通并导出报告')">巡检当前项目</button>
    </div>
    <div class="assistant-chat-composer">
      <textarea ref="input" :value="modelValue" :disabled="busy||readOnly" rows="3" aria-label="助手任务" placeholder="描述你要测试的功能，或需要助手完成的任务…" @input="edit" @keydown.enter.exact="enter" />
      <div v-if="documentName" class="assistant-chat-document"><span :title="documentName">{{documentName}} · {{documentStatus}}</span><button type="button" :disabled="busy" @click="emit('clear-document')">移除资料</button></div>
      <AssistantScope :endpoints="endpoints" :profiles="profiles" :api-id="apiId" :profile-id="profileId" :default-model="defaultModel" :permission="permission" :can-auto="canAuto" :read-only="readOnly" :busy="busy" @update:api-id="emit('update:apiId',$event)" @update:profile-id="emit('update:profileId',$event)" @update:permission="emit('update:permission',$event)" @permission-change="emit('permission-change')">
        <template #attachment><label class="assistant-chat-upload" :class="{disabled:busy||readOnly}" title="导入文档"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M12 5v14M5 12h14"/></svg><input type="file" accept=".txt,.md,.json,.yaml,.yml,.docx,.pdf" :disabled="busy||readOnly" aria-label="导入助手文档" @change="emit('upload',$event)"></label></template>
        <template #send><button type="button" class="assistant-chat-send" :disabled="busy||readOnly||!modelValue.trim()" :aria-label="busy?'助手处理中':'发送给 AI'" :title="busy?'助手处理中':'发送给 AI（Enter）'" @click="submit"><svg v-if="!busy" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M12 19V5m-6 6l6-6 6 6"/></svg><svg v-else class="assistant-chat-loading" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M21 12a9 9 0 1 1-9-9"/></svg></button></template>
      </AssistantScope>
    </div>
    <p class="assistant-chat-hint">{{readOnly?'当前账号为只读权限，无法执行或修改项目。':permission==='project_full'?'项目最高权限：按明确任务自动执行已授权操作。':'默认权限：先生成计划和草稿，确认后执行。'}} <span>Enter 发送 · Shift + Enter 换行</span></p>
  </section>
</template>
<style scoped>
.assistant-chat-entry { --chat-ink:#173c48; --chat-muted:#49636b; --chat-line:#c8dcd7; --chat-soft:#f4f9f8; min-width:0; padding:10px clamp(18px,6vw,96px) 16px; color:var(--chat-ink); background:white; }
.assistant-chat-suggestions { display:flex; align-items:center; gap:8px; padding:0 0 12px; overflow-x:auto; scrollbar-width:thin; scrollbar-color:#8bb3a8 transparent; }
.assistant-chat-suggestions>span { flex:none; color:var(--chat-muted); font-size:12px; }
.assistant-chat-suggestions button { flex:none; min-height:34px; padding:6px 11px; border:1px solid var(--chat-line); border-radius:8px; background:white; color:#285f54; font:inherit; font-size:12px; cursor:pointer; }
.assistant-chat-suggestions button:hover { background:#eef6f3; border-color:#87b8aa; }
.assistant-chat-composer { min-width:0; overflow:hidden; border:1px solid #a7c7bc; border-radius:18px; background:white; box-shadow:0 5px 16px rgb(27 71 59 / 6%); }
.assistant-chat-composer:focus-within { border-color:#237c6d; box-shadow:0 5px 16px rgb(27 71 59 / 8%); }
.assistant-chat-composer textarea { display:block; width:100%; min-height:96px; height:96px; max-height:176px; padding:18px 20px 12px; resize:none; overflow-y:auto; border:0; border-radius:0; outline:0; background:white; color:var(--chat-ink); font:inherit; font-size:14px; line-height:1.65; box-shadow:none; caret-color:#237c6d; }
.assistant-chat-composer textarea::placeholder { color:var(--chat-muted); opacity:1; }
.assistant-chat-document { display:flex; align-items:center; gap:12px; padding:0 20px 10px; color:var(--chat-muted); font-size:12px; }
.assistant-chat-document span { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.assistant-chat-document button { flex:none; padding:4px; border:0; background:transparent; color:#165d52; font:inherit; cursor:pointer; text-decoration:underline; text-underline-offset:3px; }
.assistant-chat-upload,.assistant-chat-send { display:flex; align-items:center; justify-content:center; width:42px; height:42px; flex:none; border-radius:50%; cursor:pointer; }
.assistant-chat-upload { position:relative; color:var(--chat-ink); }
.assistant-chat-upload:hover { background:#e1eeea; }
.assistant-chat-upload svg,.assistant-chat-send svg { width:22px; height:22px; }
.assistant-chat-upload input { position:absolute; inset:0; width:100%; height:100%; opacity:0; cursor:inherit; }
.assistant-chat-upload:focus-within { outline:2px solid #237c6d; outline-offset:1px; }
.assistant-chat-send { padding:0; border:0; background:#227b6c; color:white; }
.assistant-chat-send:hover { background:#165d52; }
.assistant-chat-send:disabled { background:#d2dfda; color:#49636b; cursor:not-allowed; }
.assistant-chat-entry .assistant-chat-send svg,.assistant-chat-entry .assistant-chat-send svg path { stroke:#fff !important; fill:none; }
.assistant-chat-entry .assistant-chat-send:disabled svg,.assistant-chat-entry .assistant-chat-send:disabled svg path { stroke:#49636b !important; }
.assistant-chat-suggestions button:disabled,.assistant-chat-upload.disabled { opacity:.65; cursor:not-allowed; }
.assistant-chat-hint { display:flex; justify-content:space-between; gap:16px; margin:9px 4px 0; color:var(--chat-muted); font-size:12px; line-height:1.6; }
.assistant-chat-hint span { flex:none; }
.assistant-chat-entry button:focus-visible { outline:2px solid #237c6d; outline-offset:3px; }
.assistant-chat-composer ::selection { background:#c4e4d8; color:#153b33; }
.assistant-chat-loading { animation:chat-spin 1s linear infinite; }
@keyframes chat-spin { to { transform:rotate(360deg); } }
@media(prefers-reduced-motion:reduce) { .assistant-chat-loading { animation:none; } }
@media(max-width:760px) { .assistant-chat-entry { padding:10px 16px 14px; } .assistant-chat-hint { display:block; } .assistant-chat-hint span { display:none; } .assistant-chat-composer textarea { padding:14px 16px 10px; font-size:16px; } }
</style>
