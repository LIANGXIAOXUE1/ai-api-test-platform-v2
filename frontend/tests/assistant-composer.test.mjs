import assert from 'node:assert/strict'
import {after,test} from 'node:test'
import {createServer} from 'vite'
import vue from '@vitejs/plugin-vue'
import {createSSRApp} from 'vue'
import {renderToString} from 'vue/server-renderer'
const server=await createServer({configFile:false,plugins:[vue()],server:{middlewareMode:true,hmr:false},appType:'custom',logLevel:'error'})
after(()=>server.close())
const {default:Composer}=await server.ssrLoadModule('/src/features/shared/AssistantComposer.vue')
const props={modelValue:'说明接口参数',endpoints:[],profiles:[],apiId:0,profileId:0,defaultModel:'demo-model',permission:'default',canAuto:true,readOnly:false,busy:false,documentName:'',documentStatus:''}
test('the actual composer renders permission selection next to the model picker',async()=>{
 const html=await renderToString(createSSRApp(Composer,props))
 assert.match(html,/aria-label="助手执行权限：默认权限 · 先审核"/);assert.match(html,/aria-label="助手对话模型"/);assert.match(html,/默认权限 · 先审核/)
 assert.match(html,/aria-label="发送给 AI"/)
})
test('automatic execution remains unavailable without account authorization',async()=>{
 const html=await renderToString(createSSRApp(Composer,{...props,canAuto:false}))
 assert.match(html,/当前账号未获自动执行权限/)
})
test('read-only accounts cannot send tasks or change execution permission',async()=>{
 const html=await renderToString(createSSRApp(Composer,{...props,readOnly:true}))
 assert.match(html,/<button[^>]*class="permission-trigger"[^>]*disabled/)
 assert.match(html,/<button[^>]*class="assistant-chat-send"[^>]*disabled/)
 assert.match(html,/当前账号为只读权限/)
})
