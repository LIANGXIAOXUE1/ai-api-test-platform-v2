import assert from 'node:assert/strict'
import {after,test} from 'node:test'
import {createServer} from 'vite'
const server=await createServer({configFile:false,server:{middlewareMode:true,hmr:false},appType:'custom',logLevel:'error'})
after(()=>server.close())
const {documentationFor,filterApiDocumentGroups,flattenSchemaFields,groupApiDocuments}=await server.ssrLoadModule('/src/features/api-docs/catalogue.ts')
const entries=[
  {id:1,name:'新增文件夹',method:'POST',url:'http://local/api/folder',schema:{tags:['文件管理'],summary:'创建文件夹','x-pulse-document':{path:'/api/folder',tagOrder:['文件管理','用户'],tagDescriptions:{文件管理:'上传与管理文件'}}}},
  {id:2,name:'注册',method:'POST',url:'/api/auth/register',schema:{tags:['用户'],summary:'用户注册','x-pulse-document':{tagOrder:['文件管理','用户']}}},
  {id:3,name:'未归类',method:'GET',url:'/api/health',schema:{}},
]
test('groups API operations by OpenAPI tag and preserves declared tag order',()=>{const groups=groupApiDocuments(entries);assert.deepEqual(groups.map(item=>item.name),['文件管理','用户','未分组']);assert.equal(groups[0].description,'上传与管理文件');assert.equal(groups[0].endpoints.length,1)})
test('search and method filters retain only matching operations',()=>{const groups=filterApiDocumentGroups(groupApiDocuments(entries),'注册','POST');assert.equal(groups.length,1);assert.equal(groups[0].endpoints[0].id,2);assert.equal(filterApiDocumentGroups(groupApiDocuments(entries),'missing').length,0)})
test('prefers imported OpenAPI path and does not invent absent documentation',()=>{assert.equal(documentationFor(entries[0]).path,'/api/folder');assert.equal(documentationFor(entries[2]).summary,'未归类');assert.equal(documentationFor(entries[2]).path,'/api/health')})
test('schema fields expose nested types and required flags without reading examples',()=>{const fields=flattenSchemaFields({type:'object',required:['user'],properties:{user:{type:'object',required:['name'],properties:{name:{type:'string',description:'账号名'},password:{type:'string',example:'never render'}}},count:{type:'integer'}}});assert.deepEqual(fields.map(({name,type,required})=>({name,type,required})),[{name:'user',type:'object',required:true},{name:'user.name',type:'string',required:true},{name:'user.password',type:'string',required:false},{name:'count',type:'integer',required:false}]);assert.equal(fields[2].description,'')})
