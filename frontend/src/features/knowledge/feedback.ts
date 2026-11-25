export type KnowledgeFeedback = { title:string; reasons:string[]; nextSteps:string[]; severity:'error'|'warning'; phase:'upload'|'generation'|'save' }
const strings = (value:unknown) => Array.isArray(value) ? [...new Set(value.filter((item):item is string => typeof item==='string' && !!item.trim()))] : []
export function generationFeedback(result:any):KnowledgeFeedback|null {
  const cases=Array.isArray(result.cases)?result.cases:[]
  const ready=cases.filter((item:any)=>item.status==='ready_for_review').length
	const failed=result.status==='cannot_generate' || result.status==='needs_information'
  if (result.status==='generated' && cases.length && !result.missingInformation?.length) return null
  if (!failed && ready===cases.length && ready>0 && !result.missingInformation?.length) return null
  return {
    title:result.message || result.error || (cases.length ? ready ? '部分用例仍需补充配置' : '当前只有业务草稿，没有可执行用例' : '暂时无法生成测试用例，请补充业务规则与预期结果'),
    reasons:strings(result.missingInformation),
    nextSteps:strings(result.nextSteps).length ? strings(result.nextSteps) : ['检查文档内容、接口范围和预期结果，补充后继续生成'],
    severity:!failed && cases.length ? 'warning' : 'error',phase:'generation',
  }
}
export function knowledgeRequestError(error:any,phase:KnowledgeFeedback['phase']):KnowledgeFeedback {
  const data=error?.details || {}
  let message=data.message || data.error || error?.message || '请求未完成'
  if (/Failed to fetch|NetworkError|fetch failed/i.test(message)) message='无法连接平台服务，请检查服务运行状态后重试'
  const prefix=phase==='upload'?'文档上传失败':phase==='save'?'用例保存失败':'测试用例生成失败'
  return {title:prefix+'：'+message,reasons:strings(data.missingInformation),nextSteps:strings(data.nextSteps).length ? strings(data.nextSteps) : [phase==='upload'?'确认文件格式与内容后重新上传':'检查文档、接口范围与模型连接后重试'],severity:'error',phase}
}
