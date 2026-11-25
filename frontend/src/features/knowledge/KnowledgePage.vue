<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { usePermissions } from '../auth/usePermissions'
import { verifiedModels } from '../shared/models'
import { businessCaseForEditor } from './case'
import KnowledgeFeedback from './KnowledgeFeedback.vue'
import { generationFeedback, knowledgeRequestError, type KnowledgeFeedback as Feedback } from './feedback'
import '../shared/business-testing.css'
const props = defineProps<{ projectId:number; documents:any[]; endpoints:any[]; profiles:any[]; initialCases:any[]; profileId:number; request:(path:string,options?:RequestInit)=>Promise<any> }>()
const emit = defineEmits<{ refresh:[]; configure:[testCase:any] }>()
const { readOnly } = usePermissions()
const type = ref('业务规则'), version = ref('v1'), mode = ref('design'), apiId = ref(0), selectedModel = ref(props.profileId)
const prompt = ref('根据当前项目资料设计核心业务验收用例，覆盖正常、边界和异常情况。说明前置条件、预期结果与资料依据。')
const supplement = ref(''), selectedDocuments = ref<number[]>(props.documents.map(item=>item.id)), cases = ref<any[]>(props.initialCases || []), missing = ref<string[]>([])
const busy = ref(false), error = ref(''), notice = ref(''), preview = ref<any>(null)
const feedback=ref<Feedback|null>(null)
async function showFeedback(value:Feedback|null) { feedback.value=value; if(value){await nextTick();const panel=document.querySelector<HTMLElement>('.knowledge-feedback');panel?.scrollIntoView({block:'start',behavior:'smooth'});panel?.focus({preventScroll:true})} }
function repair(phase:Feedback['phase']) {if(phase==='upload'){document.querySelector<HTMLInputElement>('.knowledge-page input[type=file]')?.click()}else{document.querySelector<HTMLTextAreaElement>('#knowledge-supplement')?.focus();document.querySelector('#knowledge-supplement')?.scrollIntoView({block:'center',behavior:'smooth'})}}
const availableModels = computed(() => verifiedModels(props.profiles))
const post = (payload:any) => ({method:'POST',body:JSON.stringify(payload)})
async function generate() {
  if (!prompt.value.trim()) {await showFeedback({title:'无法生成测试用例：没有填写测试目标',reasons:[],nextSteps:['填写要验证的功能和场景，再点击生成'],severity:'error',phase:'generation'});return}
  busy.value=true; error.value=''; notice.value=''
  try {
    const result=await props.request('/api/agent/business-cases',post({projectId:props.projectId,apiId:apiId.value,profileId:selectedModel.value,mode:mode.value,documentIds:selectedDocuments.value,useDocuments:selectedDocuments.value.length>0,prompt:prompt.value,supplement:supplement.value}))
    cases.value=result.cases || []; missing.value=result.missingInformation || []
    const responseFeedback=generationFeedback(result);await showFeedback(responseFeedback)
    notice.value=responseFeedback ? '' : `已生成 ${cases.value.length} 条可审核请求，请核对依据和预期。`
  } catch(e) { cases.value=[];missing.value=[];await showFeedback(knowledgeRequestError(e,'generation')) } finally { busy.value=false }
}
async function upload(event:Event) {
  const input=event.target as HTMLInputElement, file=input.files?.[0]; if(!file)return
  notice.value=''; error.value=''
  if(file.size===0 || file.size>12*1024*1024 || !/\.(txt|md|json|ya?ml|docx|pdf|csv|rst)$/i.test(file.name)){await showFeedback({title:'文档上传失败：'+(file.size===0?'文件为空':file.size>12*1024*1024?'文件超过12MB限制':'不支持此文件格式'),reasons:[],nextSteps:['选择包含文字内容的TXT、Markdown、JSON、YAML、DOCX或文本型PDF文件'],severity:'error',phase:'upload'});input.value='';return}
  busy.value=true; error.value=''; notice.value=''
  try { const data=new FormData(); data.set('file',file); data.set('projectId',String(props.projectId)); data.set('docType',type.value); data.set('version',version.value)
    const item=await props.request('/api/documents',{method:'POST',body:data}); selectedDocuments.value.push(item.id);feedback.value=null; notice.value='文档上传成功。请查看提取内容；能否生成用例还需要判断业务规则与接口是否完整。'; emit('refresh')
  } catch(e) { await showFeedback(knowledgeRequestError(e,'upload')) } finally { busy.value=false; input.value='' }
}
async function remove(item:any) {
  error.value=''
  try { await props.request(`/api/documents?projectId=${props.projectId}&id=${item.id}`,{method:'DELETE'}); selectedDocuments.value=selectedDocuments.value.filter(id=>id!==item.id); if(preview.value?.id===item.id)preview.value=null; emit('refresh') } catch(e){error.value=(e as Error).message}
}
async function view(item:any) {
  try { preview.value=await props.request(`/api/documents?projectId=${props.projectId}&id=${item.id}`) } catch(e){error.value=(e as Error).message}
}
async function save(item:any) {
  if(item.status!=='ready_for_review')return
  busy.value=true; notice.value=''
  try { const saved=await props.request('/api/cases',post({projectId:props.projectId,apiId:item.apiId,name:item.name,prompt:prompt.value+'\n'+item.rule,request:item.request,assertions:item.assertions})); item.saved=true; item.savedId=saved.id;feedback.value=null; notice.value='已保存为测试用例，可直接载入测试台，或从测试记录中的保存用例载入执行。'; emit('refresh') } catch(e){await showFeedback(knowledgeRequestError(e,'save'))} finally{busy.value=false}
}
function configure(item:any) { const testCase=businessCaseForEditor(item,props.projectId,prompt.value); if(testCase)emit('configure',testCase) }
function download() {
  const url=URL.createObjectURL(new Blob([JSON.stringify({projectId:props.projectId,prompt:prompt.value,supplement:supplement.value,mode:mode.value,cases:cases.value,missingInformation:missing.value},null,2)],{type:'application/json;charset=utf-8'}))
  const link=document.createElement('a'); link.href=url; link.download='业务用例草稿.json'; link.click(); setTimeout(()=>URL.revokeObjectURL(url),1000)
}
</script>
<template>
  <section class="list-page business-testing knowledge-page">
    <div class="page-title"><div><h1>项目知识库</h1><p>保存项目资料，通过可编辑目标生成业务草稿或可执行用例。</p></div><button v-if="!readOnly" class="primary-btn" :disabled="busy" @click="generate">{{busy?'正在生成…':'根据资料生成用例'}}</button></div>
    <p v-if="error" class="bt-error" role="alert">{{error}}</p><p v-if="notice" class="bt-notice" role="status">{{notice}}</p>
    <KnowledgeFeedback v-if="feedback" :feedback="feedback" @repair="repair" />
    <div class="bt-form">
      <section class="bt-section">
        <h2>项目资料</h2><p class="bt-hint">选择本次使用的文档。上传后可查看提取的文本，确认内容完整。资料仅用于当前项目。</p>
        <div v-if="!readOnly" class="bt-fields"><label>文档类型<select v-model="type"><option>业务规则</option><option>PRD</option><option>错误码</option><option>接口契约</option><option>认证说明</option></select></label><label>版本<input v-model="version"></label></div>
        <label v-if="!readOnly" class="bt-file">上传文档（最多12MB；TXT / Markdown / JSON / YAML / DOCX / 文本型 PDF）<input type="file" accept=".txt,.md,.json,.yaml,.yml,.docx,.pdf,.csv,.rst" :disabled="busy" @change="upload"></label>
        <div v-for="item in documents" :key="item.id" class="bt-record"><label class="bt-confirm"><input v-model="selectedDocuments" type="checkbox" :value="item.id"><span>{{item.name}}<small>{{item.docType}} · {{item.version}} · {{item.chunkCount}} 个文本块</small></span></label><div class="bt-actions"><button class="send-only" @click="view(item)">查看内容</button><button v-if="!readOnly" class="link-btn" :disabled="busy" @click="remove(item)">删除</button></div></div>
        <p v-if="!documents.length" class="bt-hint">暂无文档。也可以在下方直接补充业务资料和预期行为。</p>
        <details v-if="preview" open class="bt-detail"><summary>{{preview.name}} 提取内容</summary><pre>{{preview.content || '当前账号无法查看原始内容'}}</pre></details>
      </section>
      <section v-if="!readOnly" class="bt-section">
        <h2>生成目标与提示词</h2>
        <div class="bt-fields"><label>生成方式<select v-model="mode"><option value="design">业务场景草稿（可先整理业务，稍后绑定接口）</option><option value="executable">可执行用例（需要真实接口与明确预期）</option></select></label><label>接口范围<select v-model.number="apiId"><option :value="0">当前项目全部接口</option><option v-for="item in endpoints" :key="item.id" :value="item.id">{{item.method}} {{item.url}} · {{item.name}}</option></select></label></div>
        <label>生成模型<select v-model.number="selectedModel"><option :value="0">平台默认助手模型</option><option v-for="item in availableModels" :key="item.id" :value="item.id">{{item.name}} · {{item.model}}</option></select></label>
        <label class="knowledge-prompt">提示词<textarea v-model="prompt" rows="3" placeholder="描述要验收的功能、关注的场景与期望输出"></textarea></label>
        <label>补充资料与预期<textarea id="knowledge-supplement" v-model="supplement" rows="4" placeholder="补充完整业务条件、预期状态或响应字段。账号与Token使用环境变量名，不填写真实密钥。"></textarea></label>
        <p class="bt-hint">当前项目有 {{endpoints.length}} 个接口。资料与接口不匹配时，可先生成业务草稿；执行前再导入接口、配置数据并确认断言。</p>
        <div v-if="missing.length" class="bt-detail" role="status"><h3>继续生成需要补充什么？</h3><ol><li v-for="(item,index) in missing" :key="index">{{item}}</li></ol><p class="bt-hint">修改上面的接口范围、提示词或补充资料后，点击“继续生成”。已有草稿可先查看和下载。</p></div>
        <div class="bt-actions"><button class="primary-btn" :disabled="busy" @click="generate">{{busy?'正在生成…':missing.length?'补充后继续生成':'生成用例'}}</button></div>
      </section>
      <section class="bt-section">
        <div class="bt-heading"><h2>生成结果与资料依据</h2><button v-if="cases.length" class="send-only" @click="download">下载用例草稿</button></div>
        <div v-for="(item,index) in cases" :key="index" class="bt-detail"><div class="bt-heading"><h3>{{item.name}}</h3><span class="bt-status" :class="item.status==='ready_for_review'?'satisfied':'unknown'">{{item.status==='ready_for_review'?'可审核请求':'待配置业务草稿'}}</span></div><p>{{item.scenario}} · {{item.rule}}</p><p v-if="item.preconditions">前置条件：{{item.preconditions}}</p><p v-if="item.expectedResult">预期结果：{{item.expectedResult}}</p><ul v-if="item.missingInformation?.length"><li v-for="message in item.missingInformation" :key="message">{{message}}</li></ul><details><summary>查看依据与请求</summary><p v-for="evidence in item.evidence" :key="evidence">{{evidence}}</p><pre v-if="item.request?.url">{{JSON.stringify({request:item.request,assertions:item.assertions},null,2)}}</pre></details><div v-if="!readOnly && item.status==='ready_for_review'" class="bt-actions"><button class="send-only" @click="configure(item)">载入测试台</button><button class="primary-btn" :disabled="busy||item.saved" @click="save(item)">{{item.saved?'已保存':'确认并保存用例'}}</button></div></div>
        <p v-if="!cases.length" class="bt-hint">尚无生成结果。填写目标并选择资料后开始生成；缺少信息会在页面显示具体补充说明。</p>
      </section>
    </div>
  </section>
</template>
<style scoped>
.knowledge-prompt { margin-top:16px; }
ol,ul { padding-left:22px; line-height:1.8; overflow-wrap:anywhere; }
small { display:block; margin-top:4px; color:var(--text-muted); }
</style>
