<script setup lang="ts">
import { computed } from 'vue'
import { verifiedModels } from './models'
import AssistantPermissionMenu from './AssistantPermissionMenu.vue'
const props = defineProps<{ endpoints: any[]; profiles: any[]; apiId: number; profileId: number; defaultModel: string; busy: boolean; permission:'default'|'project_full'; canAuto:boolean; readOnly:boolean }>()
const emit = defineEmits<{ 'update:apiId':[id:number]; 'update:profileId':[id:number]; 'update:permission':[permission:'default'|'project_full']; 'permission-change':[] }>()
const models = computed(() => verifiedModels(props.profiles))
const selectedModel = computed(() => models.value.find(item=>item.id===props.profileId))
const selectedEndpoint = computed(() => props.endpoints.find(item=>item.id===props.apiId))
</script>
<template>
  <div class="assistant-scope-controls" role="group" aria-label="本次助手执行配置">
    <div class="scope-attachment"><slot name="attachment" /></div>
    <div class="scope-permission"><AssistantPermissionMenu :model-value="permission" :can-auto="canAuto" :read-only="readOnly" :busy="busy" @update:model-value="emit('update:permission',$event)" @change="emit('permission-change')" /></div>
    <label class="scope-picker scope-api" :title="selectedEndpoint ? selectedEndpoint.method+' '+selectedEndpoint.url+' · '+selectedEndpoint.name : '根据下一条消息在当前项目全部接口中自动匹配'">
      <span>接口</span><select aria-label="助手工作范围接口" :disabled="busy" :value="apiId" @change="emit('update:apiId',Number(($event.target as HTMLSelectElement).value))"><option :value="0">全部 · 自动匹配</option><option v-for="item in endpoints" :key="item.id" :value="item.id">{{ item.method }} {{ item.url }} · {{ item.name }}</option></select>
      <svg class="scope-chevron" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" aria-hidden="true"><path d="M4 6l4 4 4-4"/></svg>
    </label>
    <label class="scope-picker scope-model" :title="selectedModel ? selectedModel.name+' · '+selectedModel.model : '默认助手 · '+(defaultModel||'未配置')">
      <span>模型</span><select aria-label="助手对话模型" :disabled="busy" :value="profileId" @change="emit('update:profileId',Number(($event.target as HTMLSelectElement).value))"><option :value="0">默认 · {{ defaultModel || '未配置' }}</option><option v-for="item in models" :key="item.id" :value="item.id">{{ item.name===item.model ? item.model : item.name+' · '+item.model }}</option></select>
      <svg class="scope-chevron" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" aria-hidden="true"><path d="M4 6l4 4 4-4"/></svg>
    </label>
    <div class="scope-send"><slot name="send" /></div>
  </div>
</template>
<style scoped>
.assistant-scope-controls { display:flex; align-items:center; gap:8px; padding:10px 12px; background:var(--chat-soft,#f4f9f8); color:var(--chat-ink,#173c48); border-top:1px solid var(--chat-line,#c8dcd7); }
.scope-picker { display:flex; align-items:center; gap:7px; min-width:0; height:42px; padding:0 8px; border:1px solid transparent; border-radius:8px; font-size:13px; line-height:1.4; }
.scope-picker:hover { background:#e7f1ee; }
.scope-picker:focus-within { border-color:#237c6d; outline:2px solid #237c6d; outline-offset:1px; }
.scope-permission { flex:0 1 212px; min-width:0; }
.scope-api { flex:0 1 300px; }
.scope-model { flex:0 1 300px; margin-left:auto; }
.scope-picker>span { flex:none; color:var(--chat-muted,#49636b); font-size:12px; }
.scope-icon { width:19px; height:19px; flex:none; }
.scope-chevron { width:14px; height:14px; flex:none; pointer-events:none; }
.scope-picker select { display:block; flex:1; width:100%; min-width:0; height:40px; padding:0; border:0; outline:0; border-radius:0; appearance:none; background:transparent; color:inherit; font:inherit; font-weight:600; cursor:pointer; text-overflow:ellipsis; }
.scope-picker option { color:#173c48; background:white; font-size:13px; }
.scope-picker select:disabled { opacity:.65; cursor:not-allowed; }
.scope-attachment,.scope-send { flex:none; display:flex; align-items:center; }
@media(max-width:1050px) { .scope-api { flex-basis:220px; } .scope-model { flex-basis:240px; } .scope-permission { flex-basis:196px; } }
@media(max-width:760px) {
  .assistant-scope-controls { display:grid; grid-template-columns:44px minmax(0,1fr) 44px; gap:6px; padding:8px; }
  .scope-attachment { grid-column:1; grid-row:1; }
  .scope-permission { grid-column:2/4; grid-row:1; }
  .scope-api { grid-column:1/4; grid-row:2; }
  .scope-model { grid-column:1/3; grid-row:3; margin-left:0; }
  .scope-send { grid-column:3; grid-row:3; }
}
</style>
