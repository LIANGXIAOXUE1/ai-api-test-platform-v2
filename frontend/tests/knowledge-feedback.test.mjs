import assert from 'node:assert/strict'
import {after,test} from 'node:test'
import {createServer} from 'vite'
import vue from '@vitejs/plugin-vue'
import {createSSRApp} from 'vue'
import {renderToString} from 'vue/server-renderer'
const server=await createServer({configFile:false,plugins:[vue()],server:{middlewareMode:true,hmr:false},appType:'custom',logLevel:'error'})
after(()=>server.close())
const {generationFeedback,knowledgeRequestError}=await server.ssrLoadModule('/src/features/knowledge/feedback.ts')
test('zero cases is a visible failure rather than a success notice',async()=>{
 const feedback=generationFeedback({status:'cannot_generate',message:'无法生成：文档没有业务规则',cases:[],missingInformation:['只有普通介绍'],nextSteps:['补充条件与预期']})
 assert.equal(feedback.severity,'error')
 const {default:Component}=await server.ssrLoadModule('/src/features/knowledge/KnowledgeFeedback.vue')
 const html=await renderToString(createSSRApp(Component,{feedback}))
 assert.match(html,/role="alert"/);assert.match(html,/文档没有业务规则/);assert.match(html,/补充条件与预期/)
})
test('draft-only output is clearly marked as not executable',()=>{
 const feedback=generationFeedback({status:'drafts_only',message:'没有可执行用例',cases:[{status:'needs_configuration'}]})
 assert.equal(feedback.severity,'warning');assert.match(feedback.title,/没有可执行/)
 assert.equal(generationFeedback({status:'generated',cases:[{status:'ready_for_review'}],missingInformation:[]}),null)
})
test('upload API reasons survive the frontend error wrapper',()=>{
 const feedback=knowledgeRequestError(Object.assign(new Error('upload failed'),{details:{message:'文件超过12MB',missingInformation:['文件太大'],nextSteps:['拆分后上传']}}),'upload')
 assert.match(feedback.title,/文档上传失败.*12MB/);assert.deepEqual(feedback.nextSteps,['拆分后上传'])
})
