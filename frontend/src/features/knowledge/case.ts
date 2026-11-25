export function businessCaseForEditor(item: any, projectId: number, prompt: string) {
  if (item.status !== 'ready_for_review' || !item.apiId || !item.request?.url || !item.assertions?.length) return null
  return {
    id: item.savedId || 0,
    projectId,
    apiId: item.apiId,
    name: item.name,
    prompt: prompt + '\n' + item.rule,
    request: JSON.parse(JSON.stringify(item.request)),
    assertions: JSON.parse(JSON.stringify(item.assertions)),
  }
}
