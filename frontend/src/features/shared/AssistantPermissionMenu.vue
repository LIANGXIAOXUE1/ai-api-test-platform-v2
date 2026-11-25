<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
const props=defineProps<{ modelValue:'default'|'project_full'; canAuto:boolean; readOnly:boolean; busy:boolean }>()
const emit=defineEmits<{ 'update:modelValue':[value:'default'|'project_full']; change:[] }>()
const open=ref(false),trigger=ref<HTMLButtonElement|null>(null),panel=ref<HTMLElement|null>(null)
const position=ref<Record<string,string>>({})
const label=computed(()=>props.modelValue==='project_full'?'项目最高权限 · 自动执行':'默认权限 · 先审核')
function locate(){if(!open.value||!trigger.value)return;const rect=trigger.value.getBoundingClientRect(),width=Math.min(336,innerWidth-24);position.value={width:width+'px',left:Math.max(12,Math.min(rect.left,innerWidth-width-12))+'px',bottom:Math.max(8,innerHeight-rect.top+8)+'px',maxHeight:Math.max(1,Math.min(320,rect.top-16))+'px'}}
function close(focus=false){open.value=false;if(focus)trigger.value?.focus()}
async function show(){if(props.busy||props.readOnly)return;open.value=true;locate();await nextTick();const selected=panel.value?.querySelector<HTMLButtonElement>('[aria-checked="true"]:not(:disabled)');(selected||panel.value?.querySelector<HTMLButtonElement>('button:not(:disabled)'))?.focus()}
function toggle(){if(open.value)close();else show()}
function choose(value:'default'|'project_full'){if(props.busy||props.readOnly||(value==='project_full'&&!props.canAuto))return;emit('update:modelValue',value);emit('change');close(true)}
function keyboard(event:KeyboardEvent){if(event.key==='Escape'){event.preventDefault();event.stopPropagation();close(true);return}if(event.key==='Tab'){close(true);return}if(!['ArrowUp','ArrowDown','Home','End'].includes(event.key))return;event.preventDefault();const items=Array.from(panel.value?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')||[]);if(!items.length)return;const current=items.indexOf(document.activeElement as HTMLButtonElement);const next=event.key==='Home'?0:event.key==='End'?items.length-1:(current+(event.key==='ArrowDown'?1:-1)+items.length)%items.length;items[next].focus()}
function outside(event:PointerEvent){if(open.value&&!trigger.value?.contains(event.target as Node)&&!panel.value?.contains(event.target as Node))close()}
onMounted(()=>{document.addEventListener('pointerdown',outside);window.addEventListener('resize',locate);window.addEventListener('scroll',locate,true)})
onBeforeUnmount(()=>{document.removeEventListener('pointerdown',outside);window.removeEventListener('resize',locate);window.removeEventListener('scroll',locate,true)})
watch(()=>[props.busy,props.readOnly],()=>{if(props.busy||props.readOnly)close()})
</script>
<template>
  <button ref="trigger" type="button" class="permission-trigger" :class="{open}" :disabled="busy||readOnly" :aria-label="'助手执行权限：'+label" aria-haspopup="menu" :aria-expanded="open" :title="canAuto?'默认权限先审核；项目最高权限可自动执行获授权的平台操作':'当前账号未获自动执行权限，请由管理员授权'" @click="toggle" @keydown.down.prevent="show" @keydown.up.prevent="show" @keydown.esc.prevent="close(true)">
    <svg class="permission-shield" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true"><path d="M12 3l8 3v6c0 5-8 9-8 9s-8-4-8-9V6z"/><path d="M9 12l2 2 4-4"/></svg><span>{{label}}</span><svg class="permission-chevron" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" aria-hidden="true"><path d="M4 6l4 4 4-4"/></svg>
  </button>
  <Teleport to="body">
    <section v-if="open" ref="panel" class="assistant-permission-menu" :style="position" role="menu" aria-label="助手执行权限" @keydown="keyboard">
      <h3>执行权限</h3>
      <button type="button" role="menuitemradio" :aria-checked="modelValue==='default'" @click="choose('default')"><div><b>默认权限 · 先审核</b><small>先生成计划和草稿，确认后执行。</small></div><svg v-if="modelValue==='default'" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M4 10l4 4 8-8"/></svg></button>
      <button type="button" role="menuitemradio" :aria-checked="modelValue==='project_full'" :disabled="!canAuto" @click="choose('project_full')"><div><b>项目最高权限 · 自动执行</b><small>{{canAuto?'按明确任务自动执行已授权的项目操作。':'当前账号未获自动执行授权，请联系管理员。'}}</small></div><svg v-if="modelValue==='project_full'" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M4 10l4 4 8-8"/></svg></button>
    </section>
  </Teleport>
</template>
<style scoped>
.permission-trigger { display:flex; align-items:center; gap:7px; width:100%; min-width:0; height:42px; padding:0 8px; border:1px solid transparent; border-radius:8px; background:transparent; color:#165d52; font:inherit; font-size:13px; font-weight:600; cursor:pointer; }
.permission-trigger:hover,.permission-trigger.open { background:#e7f1ee; }
.permission-trigger:focus-visible { outline:2px solid #237c6d; outline-offset:1px; }
.permission-trigger:disabled { opacity:.65; cursor:not-allowed; }
.permission-trigger span { min-width:0; flex:1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; text-align:left; }
.permission-shield { width:19px; height:19px; flex:none; }
.permission-chevron { width:14px; height:14px; flex:none; }
.permission-trigger.open .permission-chevron { transform:rotate(180deg); }
.assistant-permission-menu { position:fixed; z-index:2000; overflow-y:auto; padding:8px; border:1px solid #b9d4ca; border-radius:12px; background:white; color:#173c48; box-shadow:0 8px 28px rgb(27 71 59 / 18%); font-family:inherit; scrollbar-width:thin; scrollbar-color:#8bb3a8 transparent; }
.assistant-permission-menu h3 { margin:6px 10px 8px; color:#49636b; font-size:12px; font-weight:500; }
.assistant-permission-menu button { display:flex; align-items:center; gap:12px; width:100%; min-height:72px; padding:12px 10px; border:0; border-radius:8px; background:white; color:#173c48; font:inherit; text-align:left; cursor:pointer; }
.assistant-permission-menu button:hover,.assistant-permission-menu button[aria-checked=true] { background:#edf6f2; }
.assistant-permission-menu button:focus-visible { outline:2px solid #237c6d; outline-offset:-2px; }
.assistant-permission-menu button:disabled { color:#526969; background:#f5f7f6; cursor:not-allowed; }
.assistant-permission-menu button>div { flex:1; min-width:0; }
.assistant-permission-menu b { display:block; font-size:13px; line-height:1.6; }
.assistant-permission-menu small { display:block; margin-top:3px; color:#49636b; font-size:12px; line-height:1.6; }
.assistant-permission-menu svg { width:20px; height:20px; flex:none; color:#165d52; }
</style>
