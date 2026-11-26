import assert from 'node:assert/strict'
import {after,test} from 'node:test'
import {createServer} from 'vite'
const server=await createServer({configFile:false,server:{middlewareMode:true,hmr:false},appType:'custom',logLevel:'error'})
after(()=>server.close())
const {useKnowledgeOperation}=await server.ssrLoadModule('/src/features/knowledge/useKnowledgeOperation.ts')
test('a rejected generation releases loading before its caller displays the failure',async()=>{
 const state=useKnowledgeOperation()
 await assert.rejects(state.run('generation',async()=>{throw new Error('timeout')}),/timeout/)
 assert.equal(state.busy.value,false);assert.equal(state.generating.value,false);assert.equal(state.activeOperation.value,null)
 assert.equal(await state.run('generation',async()=>'retry completed'),'retry completed');assert.equal(state.busy.value,false)
})
test('uploads and saves cannot be mislabeled as generation or start duplicate requests',async()=>{
 const state=useKnowledgeOperation();let finish,duplicate=false
 const first=state.run('upload',()=>new Promise(resolve=>{finish=resolve}))
 assert.equal(state.busy.value,true);assert.equal(state.generating.value,false);assert.equal(state.activeOperation.value,'upload')
 await assert.rejects(state.run('generation',async()=>{duplicate=true}),/尚未结束/);assert.equal(duplicate,false)
 finish('uploaded');await first;assert.equal(state.busy.value,false)
 await assert.rejects(state.run('save',async()=>{throw new Error('save failed')}));assert.equal(state.busy.value,false)
})
