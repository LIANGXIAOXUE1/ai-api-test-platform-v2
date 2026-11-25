import assert from 'node:assert/strict'
import { after, test } from 'node:test'
import { createServer } from 'vite'
const server = await createServer({ configFile:false, server:{middlewareMode:true,hmr:false}, appType:'custom', logLevel:'error' })
after(()=>server.close())
const { verifiedModels, eligibleSelection } = await server.ssrLoadModule('/src/features/shared/models.ts')
test('only enabled verified models appear',()=>{
  const models=[{id:1,enabled:true,verified:true},{id:2,enabled:true,verified:false},{id:3,enabled:false,verified:true},{id:4,enabled:true}]
  assert.deepEqual(verifiedModels(models).map(model=>model.id),[1])
  assert.deepEqual(eligibleSelection([1,2,3,4],models),[1])
})
test('reverification removes stale selected models without deleting pool entries',()=>{
  const models=[{id:1,enabled:true,verified:true},{id:2,enabled:true,verified:true}]
  models[1].verified=false
  assert.deepEqual(eligibleSelection([1,2],models),[1]);assert.equal(models.length,2)
})
