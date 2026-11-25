import assert from 'node:assert/strict'
import { after, test } from 'node:test'
import { createServer } from 'vite'
import { createSSRApp, ref } from 'vue'
import { renderToString } from 'vue/server-renderer'
import vue from '@vitejs/plugin-vue'
const server=await createServer({configFile:false,plugins:[vue()],server:{middlewareMode:true,hmr:false},appType:'custom',logLevel:'error'})
after(()=>server.close())
const {businessCaseForEditor}=await server.ssrLoadModule('/src/features/knowledge/case.ts')
test('generated case loads its parameters and assertions rather than endpoint defaults',()=>{
 const item={status:'ready_for_review',apiId:2,savedId:9,name:'重复名称',rule:'同名拒绝',request:{method:'POST',url:'/folder',headers:{Authorization:'{{token}}'},query:{},body:'{"name":"existing"}'},assertions:[{type:'status',expected:'409'}]}
 const loaded=businessCaseForEditor(item,1,'测试重复名称')
 assert.equal(loaded.id,9);assert.equal(loaded.request.body,item.request.body);assert.equal(loaded.assertions[0].expected,'409')
 loaded.request.body='changed';assert.equal(item.request.body,'{"name":"existing"}')
 assert.equal(businessCaseForEditor({...item,status:'needs_configuration'},1,''),null)
})
test('saved cases without execution history remain visible and loadable',async()=>{
 const {default:Component}=await server.ssrLoadModule('/src/features/history/TestRecordsPage.vue')
 const {platformAuthKey}=await server.ssrLoadModule('/src/features/auth/auth.ts')
 const app=createSSRApp(Component,{runs:[],cases:[{id:9,name:'知识库新用例'}],detail:null,activeTab:'regression',busy:false})
 app.provide(platformAuthKey,{user:ref({role:'administrator'})})
 const html=await renderToString(app)
 assert.match(html,/知识库新用例/);assert.match(html,/尚未执行的保存用例/);assert.match(html,/载入测试台/)
})
