<script setup lang="ts">
import { ref,watch } from 'vue'
import { parseOpenAPIImport } from './openapi-import'
const props=defineProps<{projectId:number;request:(path:string,options?:RequestInit)=>Promise<any>}>()
const emit=defineEmits<{imported:[result:any];knowledge:[]}>()
const input=ref<HTMLInputElement|null>(null),name=ref(''),error=ref(''),notice=ref(''),busy=ref(false),spec=ref<any>(null),count=ref(0),targetBaseUrl=ref('')
function chooseFile(){if(!busy.value)input.value?.click()}
defineExpose({chooseFile})
watch(()=>props.projectId,()=>{name.value='';error.value='';notice.value='';spec.value=null;count.value=0;targetBaseUrl.value=''})
async function select(event:Event){const field=event.target as HTMLInputElement,file=field.files?.[0];if(!file)return;name.value=file.name;error.value='';notice.value='';spec.value=null;count.value=0;busy.value=true
  try{if(!/\.json$/i.test(file.name))throw new Error('这里只支持 OpenAPI JSON 文件。业务 Markdown、Word 或 PDF 请上传到项目知识库。');if(file.size>12*1024*1024)throw new Error('文件过大，请按模块拆分接口定义后再导入。');const parsed=parseOpenAPIImport(await file.text());spec.value=parsed.spec;count.value=parsed.count;targetBaseUrl.value=parsed.serverUrl}
  catch(cause){error.value=(cause as Error).message}finally{busy.value=false;field.value=''}
}
async function importFile(){if(busy.value||!spec.value)return;busy.value=true;error.value='';notice.value=''
  try{const body=JSON.stringify({projectId:props.projectId,spec:spec.value,baseUrl:targetBaseUrl.value.trim()});if(new TextEncoder().encode(body).length>2*1024*1024)throw new Error('接口文档内容超过单次导入上限（2MB），请按模块拆分后导入。');const result=await props.request('/api/apis/import',{method:'POST',body});if(!(result.imported>0))throw new Error('本次没有导入任何接口，请检查文档中的 paths 和请求方法。');notice.value=`已导入 ${result.imported} 个接口。`;spec.value=null;emit('imported',result)}
  catch(cause){const problem=cause as Error&{status?:number};error.value=problem.status===403?'当前账号没有导入此项目接口的权限，请联系管理员。':problem.status===401?'登录已过期，请重新登录后导入。':/Failed to fetch|NetworkError/i.test(problem.message)?'无法连接平台服务，请确认项目正在运行后重试。':problem.message}
  finally{busy.value=false}
}
</script>
<template>
  <section class="openapi-import" aria-label="OpenAPI 接口导入">
    <div class="openapi-import-heading"><b>导入接口定义</b><span>OpenAPI JSON</span></div>
    <div class="openapi-import-actions"><button type="button" class="openapi-choose" :disabled="busy" @click="chooseFile">选择 JSON 文件</button><button type="button" class="openapi-submit" :disabled="busy||!spec" @click="importFile">{{busy?'处理中…':'导入接口'}}</button></div>
    <input ref="input" hidden type="file" accept=".json,application/json" aria-label="选择 OpenAPI JSON 文件" :disabled="busy" @change="select">
    <p v-if="name" class="openapi-file-name" :title="name">已选择：{{name}}<span v-if="spec"> · {{count}} 个接口</span></p>
    <label v-if="spec" class="openapi-target-url">被测服务地址<input v-model="targetBaseUrl" type="url" placeholder="例如：http://cloud-disk:8082" aria-label="被测服务地址"></label>
    <p v-if="spec" class="openapi-import-help">测试请求由平台后端发出。服务在同一个 Docker Compose 网络时，请填写服务名和容器端口（如 http://cloud-disk:8082），不要填 127.0.0.1 或 localhost。</p>
    <p v-if="error" class="openapi-import-error" role="alert">导入失败：{{error}}</p>
    <p v-if="notice" class="openapi-import-notice" role="status">{{notice}}</p>
    <p class="openapi-import-help">业务规则、Word/PDF 等资料请到<button type="button" @click="emit('knowledge')">项目知识库上传</button>。</p>
  </section>
</template>
<style scoped>
.openapi-import { flex:none; min-width:0; padding-top:12px; border-top:1px solid var(--line,#d2e3df); color:#173c48; }
.openapi-import-heading { display:flex; align-items:center; gap:8px; margin-bottom:9px; }
.openapi-import-heading b { font-size:13px; font-weight:600; }
.openapi-import-heading span { color:#49636b; font-size:11px; }
.openapi-import-actions { display:flex; gap:8px; }
.openapi-import-actions button { min-height:36px; padding:7px 11px; border:1px solid #b8d2cb; border-radius:8px; font:inherit; font-size:12px; cursor:pointer; }
.openapi-choose { background:white; color:#216758; }
.openapi-submit { margin-left:auto; background:#267969; color:white; }
.openapi-import-actions button:disabled { opacity:.6; cursor:not-allowed; }
.openapi-file-name { margin:8px 0 0; color:#49636b; font-size:12px; overflow-wrap:anywhere; line-height:1.6; }
.openapi-target-url { display:flex; flex-direction:column; gap:5px; margin-top:10px; color:#365960; font-size:11px; font-weight:600; }
.openapi-target-url input { width:100%; min-height:36px; box-sizing:border-box; padding:7px 10px; border:1px solid #b8d2cb; border-radius:7px; background:white; color:#24484f; font:inherit; font-weight:400; }
.openapi-import-error,.openapi-import-notice { margin:8px 0 0; padding:9px 10px; border-radius:8px; font-size:12px; line-height:1.65; overflow-wrap:anywhere; }
.openapi-import-error { color:#8f2938; background:#fceff0; }
.openapi-import-notice { color:#216758; background:#edf6f2; }
.openapi-import-help { margin:9px 0 0; color:#49636b; font-size:11px; line-height:1.65; }
.openapi-import-help button { border:0; padding:0; background:transparent; color:#216758; font:inherit; text-decoration:underline; text-underline-offset:3px; cursor:pointer; }
.openapi-import button:focus-visible { outline:2px solid #267969; outline-offset:2px; }
.openapi-import button:hover:not(:disabled) { border-color:#267969; }
</style>
