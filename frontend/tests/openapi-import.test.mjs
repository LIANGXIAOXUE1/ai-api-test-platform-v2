import assert from 'node:assert/strict'
import {after,test} from 'node:test'
import {createServer} from 'vite'
const server=await createServer({configFile:false,server:{middlewareMode:true,hmr:false},appType:'custom',logLevel:'error'})
after(()=>server.close())
const {parseOpenAPIImport}=await server.ssrLoadModule('/src/features/shared/openapi-import.ts')
test('valid OpenAPI JSON exposes operations and a replaceable server default',()=>{const parsed=parseOpenAPIImport('\uFEFF'+JSON.stringify({openapi:'3.0.0',servers:[{url:'http://127.0.0.1:8082'}],paths:{'/echo':{post:{summary:'echo'},parameters:[]},'/auth':{get:{}}}}));assert.equal(parsed.count,2);assert.equal(parsed.serverUrl,'http://127.0.0.1:8082')})
test('business text and ordinary response JSON return actionable errors',()=>{assert.throws(()=>parseOpenAPIImport('# 业务规则\n提交成功后返回200'),/项目知识库/);assert.throws(()=>parseOpenAPIImport('{"message":"ok"}'),/没有 OpenAPI 的 paths/);assert.throws(()=>parseOpenAPIImport(''),/文件为空/)})
test('empty or invalid paths cannot appear as successful zero-interface imports',()=>{assert.throws(()=>parseOpenAPIImport('{"paths":{}}'),/没有可导入/);assert.throws(()=>parseOpenAPIImport('{"paths":{"echo":{"get":{}}}}'),/必须以/);assert.throws(()=>parseOpenAPIImport('{"paths":{"/echo":{"post":null}}}'),/配置不完整/)})
