export function isProjectConnectivityTask(value: string) {
  return /(?:所有|全部).{0,8}接口.*(?:联通|连通|巡检|检查)|(?:联通|连通)(?:性)?(?:巡检|检查)|项目巡检/.test(value)
}
