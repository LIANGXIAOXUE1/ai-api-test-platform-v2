const methods=new Set(['get','head','options','post','put','delete','patch'])
export function parseOpenAPIImport(text:string) {
  const value=text.replace(/^\uFEFF/,'').trim()
  if(!value)throw new Error('文件为空，请选择包含接口定义的 OpenAPI JSON 文件。')
  let spec:any
  try {spec=JSON.parse(value)} catch {throw new Error('文件不是有效的 JSON，请检查括号、逗号和文件是否完整。业务 Markdown、Word 或 PDF 请上传到项目知识库。')}
  if(!spec||typeof spec!=='object'||Array.isArray(spec)||!spec.paths||typeof spec.paths!=='object'||Array.isArray(spec.paths))throw new Error('该 JSON 没有 OpenAPI 的 paths 接口定义。普通响应 JSON 和业务资料不能在这里导入。')
  let count=0
  for(const [path,operations] of Object.entries(spec.paths)) {
    if(!path.startsWith('/'))throw new Error('paths 中的接口路径必须以 / 开头，请检查接口文档。')
    if(!operations||typeof operations!=='object'||Array.isArray(operations))throw new Error('接口路径下面需要 GET、POST 等方法定义，请检查接口文档。')
    for(const [method,operation] of Object.entries(operations))if(methods.has(method.toLowerCase())) {
      if(!operation||typeof operation!=='object'||Array.isArray(operation))throw new Error('接口方法的配置不完整，请提供对应的 OpenAPI 操作定义。')
      count++
    }
  }
  if(!count)throw new Error('文档中没有可导入的 GET、POST 等接口。请导出包含实际接口定义的 OpenAPI JSON。')
  const serverUrl = Array.isArray(spec.servers) && typeof spec.servers[0]?.url === 'string' ? spec.servers[0].url : ''
  return {spec,count,serverUrl}
}
