export type EvaluationSuite = {
  id: number; projectId: number; name: string; scenario: string; goal: string; facts: string; requirements: string
  shotCount: number; durationSeconds: number; outputFields: string[]
  cases: { id: string; name: string; input: string; expected?: string }[]
  profileIds: number[]; judgeProfileId: number; repeats: number
}
export type BusinessEvaluationRun = {
  id: number; projectId: number; suite: EvaluationSuite; status: string; total: number; completed: number
  results: any[]; conclusion: string; error?: string; createdBy?: string; createdAt: string
}
export function defaultSuite(projectId: number, scenario = 'short_video'): EvaluationSuite {
  const common = { id: 0, projectId, scenario, shotCount: 6, durationSeconds: 30, outputFields: ['customer', 'request', 'priority'], profileIds: [] as number[], judgeProfileId: 0, repeats: 1 }
  if (scenario === 'knowledge') return {
    ...common, name: 'Cloud Disk 业务知识问答',
    goal: '准确回答云盘业务问题，资料未说明的内容应明确说明未知。',
    facts: '同目录不允许同名文件夹；未登录不能访问文件；密码长度为6～32位。没有说明文件保存期限。',
    requirements: '回答符合所给资料，不能编造政策；说明回答依据。',
    cases: [
      { id: 'same_name', name: '同名文件夹', input: '同一个目录可以创建两个同名文件夹吗？', expected: '不能，应该拒绝同名创建。' },
      { id: 'auth', name: '未登录访问', input: '没有登录，能读取用户文件吗？', expected: '不能，需要登录鉴权。' },
      { id: 'unknown', name: '资料未说明的问题', input: '文件会在多少天后自动删除？', expected: '资料未说明，不能给出保存天数。' },
    ],
  }
  if (scenario === 'extraction') return {
    ...common, name: '客户需求信息提取', goal: '将客户需求整理成固定字段，便于后续处理。',
    facts: 'customer 是客户名，request 是要求，priority 保留客户原话；资料没有的字段使用 null。',
    requirements: '输出 JSON；不漏掉已有字段，不编造缺失信息。',
    cases: [
      { id: 'normal', name: '字段齐全', input: '客户：星河公司；要求：接入客服助手；优先级：高', expected: '{"customer":"星河公司","request":"接入客服助手","priority":"高"}' },
      { id: 'missing', name: '信息缺失', input: '客户：晨光工作室；要求：生成短视频脚本', expected: '{"customer":"晨光工作室","request":"生成短视频脚本","priority":null}' },
    ],
  }
  return {
    ...common, name: '通勤杯短视频脚本对比', goal: '为通勤杯生成30秒推广脚本，面向上班族。',
    facts: '产品容量450ml、重量280g。没有提供保温时长、折扣或优惠价格。',
    requirements: '6段分镜，包含画面、口播和字幕；规划时长合计30秒；不编造产品性能。',
    cases: [
      { id: 'commute', name: '通勤场景', input: '生成通勤场景的完整脚本，重点展示携带和日常使用。' },
      { id: 'office', name: '办公室场景', input: '生成办公室场景的完整脚本，不能新增产品规格或促销承诺。' },
      { id: 'facts', name: '事实约束', input: '生成面向上班族的完整脚本；客户没有提供保温数据，不要虚构。' },
    ],
  }
}
