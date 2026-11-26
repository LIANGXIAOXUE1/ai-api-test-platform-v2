<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { usePermissions } from '../auth/usePermissions'
import { verifiedModels } from '../shared/models'
import { businessCaseForEditor } from './case'
import KnowledgeFeedback from './KnowledgeFeedback.vue'
import { generationFeedback, knowledgeRequestError, type KnowledgeFeedback as Feedback } from './feedback'
import { clearKnowledgeDraft, readKnowledgeDraft, writeKnowledgeDraft } from './draft'
import { useKnowledgeOperation } from './useKnowledgeOperation'
import '../shared/business-testing.css'
const props = defineProps<{ projectId:number; documents:any[]; endpoints:any[]; profiles:any[]; initialCases:any[]; savedCases:any[]; profileId:number; request:(path:string,options?:RequestInit)=>Promise<any> }>()
const emit = defineEmits<{ refresh:[]; configure:[testCase:any] }>()
const { readOnly } = usePermissions()
const defaultPrompt = '根据当前项目资料设计核心业务验收用例，覆盖正常、边界和异常情况。说明前置条件、预期结果与资料依据。'
const restoredDraft = readKnowledgeDraft(props.projectId)
function hasKnowledgeWork(draft:any) {
  const defaultDocumentIds = props.documents.map(item=>item.id).sort((a,b)=>a-b)
  return draft.prompt.trim() !== defaultPrompt || Boolean(draft.supplement.trim()) || draft.mode !== 'design' || draft.apiId !== 0 || draft.selectedModel !== props.profileId || JSON.stringify([...draft.selectedDocuments].sort((a:number,b:number)=>a-b)) !== JSON.stringify(defaultDocumentIds) || draft.cases.length > 0 || draft.missing.length > 0
}
const hasRestoredDraft = ref(Boolean(restoredDraft && hasKnowledgeWork(restoredDraft)))
const type = ref('业务规则'), version = ref('v1'), mode = ref(restoredDraft?.mode || 'design'), apiId = ref(restoredDraft?.apiId || 0), selectedModel = ref(restoredDraft?.selectedModel || props.profileId)
const prompt = ref(restoredDraft?.prompt || defaultPrompt)
const supplement = ref(restoredDraft?.supplement || ''), selectedDocuments = ref<number[]>(restoredDraft?.selectedDocuments || props.documents.map(item=>item.id)), cases = ref<any[]>(restoredDraft?.cases || props.initialCases || []), missing = ref<string[]>(restoredDraft?.missing || [])
const {activeOperation,busy,generating,run}=useKnowledgeOperation()
const error = ref(''), notice = ref(''), preview = ref<any>(null)
const feedback=ref<Feedback|null>(null)
const historyItems=ref<any[]>([]),historyLoading=ref(false),historyError=ref(''),historyTotal=ref(0)
const historySearch=ref(''),historyStatus=ref(''),historyMode=ref(''),historyPage=ref(1),historyPageSize=10
const selectedHistory=ref<any>(null),historyDetailLoading=ref(false),historyRestoringId=ref<number|null>(null)
let historySearchTimer:ReturnType<typeof setTimeout>|undefined
let historyRequestSequence=0,historyDetailSequence=0
const historyDetailItemId=ref<number|null>(null)
const historyPageCount=computed(()=>Math.max(1,Math.ceil(historyTotal.value/historyPageSize)))
const historyStart=computed(()=>historyTotal.value===0?0:(historyPage.value-1)*historyPageSize+1)
const historyEnd=computed(()=>Math.min(historyPage.value*historyPageSize,historyTotal.value))
const generationFailed=computed(()=>feedback.value?.phase==='generation'&&feedback.value.severity==='error')
watch([prompt,supplement,mode,apiId,selectedModel,selectedDocuments,cases,missing],()=>{
  const draft={prompt:prompt.value,supplement:supplement.value,mode:mode.value==='executable'?'executable' as const:'design' as const,apiId:apiId.value,selectedModel:selectedModel.value,selectedDocuments:selectedDocuments.value,cases:cases.value,missing:missing.value}
  if(hasKnowledgeWork(draft)){writeKnowledgeDraft(props.projectId,draft);hasRestoredDraft.value=true}
  else{clearKnowledgeDraft(props.projectId);hasRestoredDraft.value=false}
},{deep:true,flush:'sync'})
async function showFeedback(value:Feedback|null) { feedback.value=value; if(value){await nextTick();const panel=document.querySelector<HTMLElement>('.knowledge-feedback');panel?.scrollIntoView({block:'nearest',behavior:'smooth'});panel?.focus({preventScroll:true})} }
function repair(phase:Feedback['phase']) {if(phase==='upload'){document.querySelector<HTMLInputElement>('.knowledge-page input[type=file]')?.click()}else{document.querySelector<HTMLTextAreaElement>('#knowledge-supplement')?.focus();document.querySelector('#knowledge-supplement')?.scrollIntoView({block:'center',behavior:'smooth'})}}
const availableModels = computed(() => verifiedModels(props.profiles))
const post = (payload:any) => ({method:'POST',body:JSON.stringify(payload)})
async function refreshHistory() {
  if(!props.projectId)return
  const sequence=++historyRequestSequence
  historyLoading.value=true;historyError.value=''
  const params=new URLSearchParams({projectId:String(props.projectId),limit:String(historyPageSize),offset:String((historyPage.value-1)*historyPageSize)})
  if(historySearch.value.trim())params.set('q',historySearch.value.trim())
  if(historyStatus.value)params.set('status',historyStatus.value)
  if(historyMode.value)params.set('mode',historyMode.value)
  try {
    const result=await props.request(`/api/knowledge/history?${params.toString()}`)
    if(sequence!==historyRequestSequence)return
    historyItems.value=Array.isArray(result)?result:(result.items||[])
    historyTotal.value=Array.isArray(result)?result.length:(result.total||0)
  }
  catch(e) { if(sequence===historyRequestSequence)historyError.value=`读取生成历史失败：${(e as Error).message}` }
  finally { if(sequence===historyRequestSequence)historyLoading.value=false }
}
function historyStatusLabel(status:string) { return status==='ready_for_review'?'可继续审核':status==='needs_information'?'需要补充信息':'部分用例待配置' }
function historyPrompt(promptText:string) { const text=(promptText||'未命名生成任务').replace(/\s+/g,' ').trim(); return text.length>110?text.slice(0,110)+'…':text }
function historyTime(value:string) { const date=new Date(value); return Number.isNaN(date.getTime())?'时间未知':date.toLocaleString() }
function filterHistory() {
  historyPage.value=1;selectedHistory.value=null
  if(historySearchTimer)clearTimeout(historySearchTimer)
  historySearchTimer=setTimeout(()=>void refreshHistory(),250)
}
watch([historySearch,historyStatus,historyMode],filterHistory)
function changeHistoryPage(page:number) {
  if(page<1||page>historyPageCount.value||page===historyPage.value)return
  historyPage.value=page;selectedHistory.value=null;void refreshHistory()
}
async function toggleHistoryDetail(item:any) {
  historyError.value=''
  if(historyDetailItemId.value===item.id&&historyDetailLoading.value){historyDetailSequence++;historyDetailItemId.value=null;historyDetailLoading.value=false;return}
  const sequence=++historyDetailSequence
  if(selectedHistory.value?.id===item.id){selectedHistory.value=null;historyDetailItemId.value=null;historyDetailLoading.value=false;return}
  selectedHistory.value=null;historyDetailItemId.value=item.id;historyDetailLoading.value=true
  try { const record=await props.request(`/api/knowledge/history?projectId=${props.projectId}&id=${item.id}`);if(sequence===historyDetailSequence)selectedHistory.value=record }
  catch(e) { if(sequence===historyDetailSequence)historyError.value=`读取历史详情失败：${(e as Error).message}` }
  finally { if(sequence===historyDetailSequence)historyDetailLoading.value=false }
}
async function restoreHistory(item:any) {
  historyError.value=''
  historyRestoringId.value=item.id
  try {
    const record=selectedHistory.value?.id===item.id?selectedHistory.value:await props.request(`/api/knowledge/history?projectId=${props.projectId}&id=${item.id}`)
    prompt.value=record.prompt||defaultPrompt;supplement.value=record.supplement||'';mode.value=record.mode==='executable'?'executable':'design'
    apiId.value=props.endpoints.some(endpoint=>endpoint.id===record.apiId)?record.apiId:0
    selectedModel.value=availableModels.value.some(profile=>profile.id===record.profileId)?record.profileId:0
    const existingDocuments=new Set(props.documents.map(document=>document.id))
    selectedDocuments.value=(record.documentIds||[]).filter((id:number)=>existingDocuments.has(id))
    cases.value=(record.cases||[]).map((item:any)=>{
      const saved=props.savedCases.find((saved:any)=>saved.projectId===props.projectId&&saved.apiId===item.apiId&&saved.name===item.name&&saved.request?.method===item.request?.method&&saved.request?.url===item.request?.url&&saved.request?.body===item.request?.body)
      return {...item,saved:Boolean(saved),savedId:saved?.id}
    })
    missing.value=record.missingInformation||[];feedback.value=null;notice.value=`已恢复 ${historyTime(record.createdAt)} 的生成结果。可以继续核对，或载入测试台。`
    selectedHistory.value=null
    await nextTick();document.querySelector('.knowledge-generated-results')?.scrollIntoView({block:'start',behavior:'smooth'})
  } catch(e) { historyError.value=`恢复历史记录失败：${(e as Error).message}` }
  finally { historyRestoringId.value=null }
}
onMounted(()=>{void refreshHistory()})
onBeforeUnmount(()=>{if(historySearchTimer)clearTimeout(historySearchTimer)})
async function generate() {
  if(busy.value||readOnly.value)return
  if (!prompt.value.trim()) {await showFeedback({title:'无法生成测试用例：没有填写测试目标',reasons:[],nextSteps:['填写要验证的功能和场景，再点击生成'],severity:'error',phase:'generation'});return}
  feedback.value=null; error.value=''; notice.value=''
  try {
    const result=await run('generation',()=>props.request('/api/agent/business-cases',post({projectId:props.projectId,apiId:apiId.value,profileId:selectedModel.value,mode:mode.value,documentIds:selectedDocuments.value,useDocuments:selectedDocuments.value.length>0,prompt:prompt.value,supplement:supplement.value})))
    cases.value=result.cases || []; missing.value=result.missingInformation || []
    const responseFeedback=generationFeedback(result);await showFeedback(responseFeedback)
    notice.value=result.historySaved===false?'用例已生成，但历史记录暂时无法保存；当前结果仍会保留在本次浏览器会话中。':responseFeedback ? '' : `已生成 ${cases.value.length} 条可审核请求，请核对依据和预期。`
    historyPage.value=1
    await refreshHistory()
  } catch(e) { await showFeedback(knowledgeRequestError(e,'generation')) }
}
async function upload(event:Event) {
  const input=event.target as HTMLInputElement, file=input.files?.[0]; if(!file)return
  if(busy.value||readOnly.value){input.value='';return}
  notice.value=''; error.value=''; feedback.value=null
  if(file.size===0 || file.size>12*1024*1024 || !/\.(txt|md|json|ya?ml|docx|pdf|csv|rst)$/i.test(file.name)){await showFeedback({title:'文档上传失败：'+(file.size===0?'文件为空':file.size>12*1024*1024?'文件超过12MB限制':'不支持此文件格式'),reasons:[],nextSteps:['选择包含文字内容的TXT、Markdown、JSON、YAML、DOCX或文本型PDF文件'],severity:'error',phase:'upload'});input.value='';return}
  try { const data=new FormData(); data.set('file',file); data.set('projectId',String(props.projectId)); data.set('docType',type.value); data.set('version',version.value)
    const item=await run('upload',()=>props.request('/api/documents',{method:'POST',body:data})); selectedDocuments.value.push(item.id);feedback.value=null; notice.value='文档上传成功。请查看提取内容；能否生成用例还需要判断业务规则与接口是否完整。'; emit('refresh')
  } catch(e) { await showFeedback(knowledgeRequestError(e,'upload')) } finally { input.value='' }
}
async function remove(item:any) {
  error.value=''
  try { await props.request(`/api/documents?projectId=${props.projectId}&id=${item.id}`,{method:'DELETE'}); selectedDocuments.value=selectedDocuments.value.filter(id=>id!==item.id); if(preview.value?.id===item.id)preview.value=null; emit('refresh') } catch(e){error.value=(e as Error).message}
}
async function view(item:any) {
  try { preview.value=await props.request(`/api/documents?projectId=${props.projectId}&id=${item.id}`) } catch(e){error.value=(e as Error).message}
}
async function save(item:any) {
  if(item.status!=='ready_for_review'||busy.value||readOnly.value)return
  notice.value='';feedback.value=null
  try { const saved=await run('save',()=>props.request('/api/cases',post({projectId:props.projectId,apiId:item.apiId,name:item.name,prompt:prompt.value+'\n'+item.rule,request:item.request,assertions:item.assertions}))); item.saved=true; item.savedId=saved.id;feedback.value=null; notice.value='已保存为测试用例，可直接载入测试台，或从测试记录中的保存用例载入执行。'; emit('refresh') } catch(e){await showFeedback(knowledgeRequestError(e,'save'))}
}
function configure(item:any) { const testCase=businessCaseForEditor(item,props.projectId,prompt.value); if(testCase)emit('configure',testCase) }
function download() {
  const url=URL.createObjectURL(new Blob([JSON.stringify({projectId:props.projectId,prompt:prompt.value,supplement:supplement.value,mode:mode.value,cases:cases.value,missingInformation:missing.value},null,2)],{type:'application/json;charset=utf-8'}))
  const link=document.createElement('a'); link.href=url; link.download='业务用例草稿.json'; link.click(); setTimeout(()=>URL.revokeObjectURL(url),1000)
}
</script>
<template>
  <section class="list-page business-testing knowledge-page">
    <div class="page-title"><div><h1>项目知识库</h1><p>保存项目资料，通过可编辑目标生成业务草稿或可执行用例。</p></div><button v-if="!readOnly" class="primary-btn" :disabled="busy" @click="generate">{{generating?'正在生成…':generationFailed?'重新生成用例':'根据资料生成用例'}}</button></div>
    <p v-if="hasRestoredDraft" class="bt-notice" role="status">已恢复本项目未完成的草稿。提示词、补充资料和生成结果会保留在当前浏览器会话中，测试失败后可返回继续处理。</p>
    <p v-if="activeOperation==='upload'||activeOperation==='save'" class="bt-notice" role="status">{{activeOperation==='upload'?'正在上传并解析文档…':'正在保存测试用例…'}}</p>
    <p v-if="error" class="bt-error" role="alert">{{error}}</p><p v-if="notice" class="bt-notice" role="status">{{notice}}</p>
    <KnowledgeFeedback v-if="feedback" :feedback="feedback" @repair="repair" />
    <section class="bt-section knowledge-history" aria-labelledby="knowledge-history-title">
      <div class="bt-heading"><div><h2 id="knowledge-history-title">生成历史 <span class="knowledge-history-count">共 {{historyTotal}} 次</span></h2><p class="bt-hint">按目标搜索并分页查看。展开记录可核对资料和用例，再恢复到当前页面继续审核。</p></div><button class="send-only" :disabled="historyLoading" @click="refreshHistory">{{historyLoading?'正在读取…':'刷新历史'}}</button></div>
      <div class="knowledge-history-filters">
        <label class="knowledge-history-search"><span class="sr-only">搜索生成目标</span><input v-model="historySearch" type="search" placeholder="搜索生成目标，例如：文件夹创建" /></label>
        <label><span class="sr-only">按状态筛选</span><select v-model="historyStatus"><option value="">全部状态</option><option value="ready_for_review">可继续审核</option><option value="needs_attention">部分待配置</option><option value="needs_information">需要补充信息</option></select></label>
        <label><span class="sr-only">按类型筛选</span><select v-model="historyMode"><option value="">全部类型</option><option value="executable">可执行用例</option><option value="design">业务场景草稿</option></select></label>
        <button v-if="historySearch||historyStatus||historyMode" class="link-btn" @click="historySearch='';historyStatus='';historyMode=''">清除筛选</button>
      </div>
      <p v-if="historyError" class="bt-error" role="alert">{{historyError}}</p>
      <p v-else-if="!historyItems.length&&!historyLoading&&!historyTotal&&!historySearch&&!historyStatus&&!historyMode" class="bt-hint">还没有生成历史。生成一次用例后会自动保存；之后可以在这里搜索、查看和恢复。</p>
      <p v-else-if="!historyItems.length&&!historyLoading" class="bt-hint">没有符合筛选条件的记录。试试其他关键词或清除筛选。</p>
      <div v-else class="knowledge-history-list" :aria-busy="historyLoading">
        <article v-for="item in historyItems" :key="item.id" class="knowledge-history-entry" :class="{selected:selectedHistory?.id===item.id}">
          <div class="knowledge-history-row">
            <button class="knowledge-history-open" :aria-expanded="selectedHistory?.id===item.id" @click="toggleHistoryDetail(item)">
              <span class="knowledge-history-main"><b>{{historyPrompt(item.prompt)}}</b><small>{{historyTime(item.createdAt)}} · {{item.mode==='executable'?'可执行用例':'业务场景草稿'}}</small></span>
              <span class="knowledge-history-meta"><span class="bt-status" :class="item.status==='ready_for_review'?'satisfied':'unknown'">{{historyStatusLabel(item.status)}}</span><span>{{item.caseCount}} 条用例</span><span v-if="item.missingCount">待补充 {{item.missingCount}}</span></span>
            </button>
            <div class="knowledge-history-actions"><button class="send-only" @click="toggleHistoryDetail(item)">{{selectedHistory?.id===item.id||historyDetailItemId===item.id&&historyDetailLoading?'收起详情':'查看详情'}}</button><button v-if="!readOnly" class="primary-btn" :disabled="historyRestoringId!==null" @click="restoreHistory(item)">{{historyRestoringId===item.id?'正在恢复…':'恢复到知识库'}}</button></div>
          </div>
          <div v-if="historyDetailLoading&&historyDetailItemId===item.id" class="knowledge-history-detail-loading" role="status">正在读取这次生成的完整内容…</div>
          <section v-if="selectedHistory?.id===item.id" class="knowledge-history-detail" :aria-label="`历史记录详情：${item.prompt}`">
            <div class="knowledge-history-context">
              <div><h3>生成目标</h3><p>{{selectedHistory.prompt||'未填写'}}</p></div>
              <div><h3>使用资料</h3><p v-if="selectedHistory.documents?.length">{{selectedHistory.documents.join('、')}}</p><p v-else>没有关联文档</p></div>
              <div><h3>补充资料与预期</h3><p>{{selectedHistory.supplement||'没有额外补充'}}</p></div>
            </div>
            <div v-if="selectedHistory.missingInformation?.length" class="knowledge-history-missing"><b>当时标记的待补充信息</b><ul><li v-for="(message,index) in selectedHistory.missingInformation" :key="index">{{message}}</li></ul></div>
            <div class="knowledge-history-cases"><h3>生成结果 · {{selectedHistory.cases?.length||0}} 条</h3>
              <article v-for="(testCase,index) in selectedHistory.cases" :key="index" class="knowledge-history-case">
                <div class="bt-heading"><h4>{{testCase.name}}</h4><span class="bt-status" :class="testCase.status==='ready_for_review'?'satisfied':'unknown'">{{testCase.status==='ready_for_review'?'可审核请求':'待配置草稿'}}</span></div>
                <p>{{testCase.scenario}} · {{testCase.rule}}</p><p v-if="testCase.preconditions">前置条件：{{testCase.preconditions}}</p><p v-if="testCase.expectedResult">预期结果：{{testCase.expectedResult}}</p>
                <ul v-if="testCase.missingInformation?.length"><li v-for="(message,index) in testCase.missingInformation" :key="index">{{message}}</li></ul>
                <details><summary>查看资料依据、请求和断言</summary><p v-for="(evidence,index) in testCase.evidence" :key="index">{{evidence}}</p><pre>{{JSON.stringify({request:testCase.request,assertions:testCase.assertions},null,2)}}</pre></details>
              </article>
              <p v-if="!selectedHistory.cases?.length" class="bt-hint">本次没有生成可恢复用例，查看“待补充信息”并补全资料后重新生成。</p>
            </div>
            <div class="knowledge-history-restore-note"><span>恢复会替换本页当前草稿；不会重新调用模型。测试用例只有点击“确认并保存用例”后才会写入测试用例列表。</span><button v-if="!readOnly" class="primary-btn" :disabled="historyRestoringId!==null" @click="restoreHistory(item)">恢复这次结果</button></div>
          </section>
        </article>
      </div>
      <div v-if="historyTotal>historyPageSize" class="knowledge-history-pagination" aria-label="生成历史分页">
        <span>显示 {{historyStart}}–{{historyEnd}} 条，共 {{historyTotal}} 条</span>
        <div><button class="send-only" :disabled="historyPage<=1||historyLoading" @click="changeHistoryPage(historyPage-1)">上一页</button><span>第 {{historyPage}} / {{historyPageCount}} 页</span><button class="send-only" :disabled="historyPage>=historyPageCount||historyLoading" @click="changeHistoryPage(historyPage+1)">下一页</button></div>
      </div>
    </section>
    <div class="bt-form">
      <section class="bt-section knowledge-results">
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
        <div class="bt-actions"><button class="primary-btn" :disabled="busy" @click="generate">{{generating?'正在生成…':missing.length?'补充后继续生成':generationFailed?'重新生成':'生成用例'}}</button></div>
      </section>
      <section class="bt-section knowledge-generated-results">
        <div class="bt-heading"><h2>生成结果与资料依据</h2><button v-if="cases.length" class="send-only" @click="download">下载用例草稿</button></div>
        <p class="bt-hint">生成草稿会暂存在当前浏览器会话中；点击“确认并保存用例”后，才会写入项目测试用例，方便之后重复执行。</p>
        <div v-for="(item,index) in cases" :key="index" class="bt-detail"><div class="bt-heading"><h3>{{item.name}}</h3><span class="bt-status" :class="item.status==='ready_for_review'?'satisfied':'unknown'">{{item.status==='ready_for_review'?'可审核请求':'待配置业务草稿'}}</span></div><p>{{item.scenario}} · {{item.rule}}</p><p v-if="item.preconditions">前置条件：{{item.preconditions}}</p><p v-if="item.expectedResult">预期结果：{{item.expectedResult}}</p><ul v-if="item.missingInformation?.length"><li v-for="message in item.missingInformation" :key="message">{{message}}</li></ul><details><summary>查看依据与请求</summary><p v-for="evidence in item.evidence" :key="evidence">{{evidence}}</p><pre v-if="item.request?.url">{{JSON.stringify({request:item.request,assertions:item.assertions},null,2)}}</pre></details><div v-if="!readOnly && item.status==='ready_for_review'" class="bt-actions"><button class="send-only" @click="configure(item)">载入测试台</button><button class="primary-btn" :disabled="busy||item.saved" @click="save(item)">{{item.saved?'已保存':'确认并保存用例'}}</button></div></div>
        <p v-if="!cases.length" class="bt-hint">尚无生成结果。填写目标并选择资料后开始生成；缺少信息会在页面显示具体补充说明。</p>
      </section>
    </div>
  </section>
</template>
<style scoped>
.knowledge-prompt { margin-top:16px; }
.knowledge-history { margin:0 0 20px; }
.knowledge-history .bt-heading { align-items:center; }
.knowledge-history .bt-heading h2 { margin-bottom:4px; }
.knowledge-history .bt-heading .bt-hint { margin:0; }
.knowledge-history-count { margin-left:8px; color:var(--text-muted); font-size:13px; font-weight:500; }
.knowledge-history-filters { display:grid; grid-template-columns:minmax(220px,1fr) minmax(150px,190px) minmax(150px,190px) auto; align-items:center; gap:10px; margin:16px 0 12px; }
.knowledge-history-filters label { min-width:0; }
.knowledge-history-filters input,.knowledge-history-filters select { width:100%; min-height:40px; padding:8px 11px; border:1px solid var(--line); border-radius:9px; background:var(--surface); color:var(--text); font:inherit; font-size:13px; }
.knowledge-history-filters input:focus-visible,.knowledge-history-filters select:focus-visible,.knowledge-history button:focus-visible { outline:2px solid var(--blue-dark); outline-offset:2px; }
.knowledge-history-filters .link-btn { white-space:nowrap; }
.knowledge-history-list { display:grid; gap:8px; margin-top:12px; }
.knowledge-history-entry { min-width:0; overflow:hidden; border:1px solid var(--line); border-radius:10px; background:var(--surface); }
.knowledge-history-entry.selected { background:var(--surface-alt); }
.knowledge-history-row { display:flex; align-items:center; justify-content:space-between; gap:14px; padding:0 14px; }
.knowledge-history-open { display:flex; flex:1; align-items:center; justify-content:space-between; gap:14px; min-width:0; padding:12px 0; border:0; background:transparent; color:inherit; text-align:left; cursor:pointer; }
.knowledge-history-main { display:grid; gap:3px; min-width:0; }
.knowledge-history-main b { display:-webkit-box; overflow:hidden; font-size:14px; line-height:1.5; font-weight:600; overflow-wrap:anywhere; -webkit-box-orient:vertical; -webkit-line-clamp:2; }
.knowledge-history-main small { margin-top:0; }
.knowledge-history-meta { display:flex; flex:0 0 auto; flex-wrap:wrap; align-items:center; gap:8px 12px; color:var(--text-muted); font-size:12px; }
.knowledge-history-meta .bt-status { font-size:12px; }
.knowledge-history-actions { display:flex; flex:0 0 auto; align-items:center; gap:8px; }
.knowledge-history-actions .primary-btn,.knowledge-history-restore-note .primary-btn { min-height:36px; padding:7px 12px; font-size:13px; }
.knowledge-history-detail-loading { padding:12px 16px; border-top:1px solid var(--line); color:var(--text-muted); font-size:13px; }
.knowledge-history-detail { padding:16px; border-top:1px solid var(--line); }
.knowledge-history-context { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:16px; }
.knowledge-history-context h3,.knowledge-history-cases>h3 { margin:0 0 6px; font-size:13px; }
.knowledge-history-context p,.knowledge-history-case p { margin:0 0 7px; color:var(--text-muted); font-size:13px; line-height:1.6; overflow-wrap:anywhere; white-space:pre-wrap; }
.knowledge-history-missing { margin-top:14px; padding:10px 12px; border-radius:8px; background:var(--surface); color:var(--text-muted); font-size:13px; }
.knowledge-history-missing ul { margin:6px 0 0; }
.knowledge-history-cases { margin-top:16px; }
.knowledge-history-case { padding:12px 0 4px; border-top:1px solid var(--line); }
.knowledge-history-case .bt-heading { align-items:center; }
.knowledge-history-case h4 { margin:0; font-size:14px; }
.knowledge-history-case details { margin:6px 0 0; color:var(--text-muted); font-size:13px; }
.knowledge-history-case details pre { max-height:280px; overflow:auto; padding:10px; border-radius:7px; background:var(--surface); color:var(--text); white-space:pre-wrap; overflow-wrap:anywhere; }
.knowledge-history-restore-note { display:flex; justify-content:space-between; align-items:center; gap:12px; margin-top:14px; padding-top:12px; border-top:1px solid var(--line); color:var(--text-muted); font-size:12px; line-height:1.5; }
.knowledge-history-pagination { display:flex; justify-content:space-between; align-items:center; gap:12px; margin-top:14px; color:var(--text-muted); font-size:13px; }
.knowledge-history-pagination>div { display:flex; align-items:center; gap:10px; }
.knowledge-history-pagination .send-only { min-height:34px; padding:6px 10px; }
.knowledge-history-pagination .send-only:disabled { opacity:.5; cursor:not-allowed; }
.knowledge-history .sr-only { position:absolute; width:1px; height:1px; padding:0; margin:-1px; overflow:hidden; clip:rect(0,0,0,0); white-space:nowrap; border:0; }
ol,ul { padding-left:22px; line-height:1.8; overflow-wrap:anywhere; }
small { display:block; margin-top:4px; color:var(--text-muted); }
@media(max-width:900px){.knowledge-history-filters{grid-template-columns:minmax(180px,1fr) minmax(140px,180px) minmax(140px,180px)}.knowledge-history-filters .link-btn{grid-column:1/-1;justify-self:start}}
@media(max-width:700px){.knowledge-history-filters{grid-template-columns:1fr 1fr}.knowledge-history-search{grid-column:1/-1}.knowledge-history-row{align-items:stretch;flex-direction:column;gap:0;padding-bottom:10px}.knowledge-history-open{align-items:flex-start;flex-direction:column;gap:8px}.knowledge-history-meta{padding-bottom:0}.knowledge-history-actions{justify-content:flex-start;flex-wrap:wrap}.knowledge-history-context{grid-template-columns:1fr;gap:10px}.knowledge-history-detail{padding:12px}.knowledge-history-restore-note,.knowledge-history-pagination{align-items:flex-start;flex-direction:column}.knowledge-history-pagination>div{width:100%;justify-content:space-between}}
</style>
