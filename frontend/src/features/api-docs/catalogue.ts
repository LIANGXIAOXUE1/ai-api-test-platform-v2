export type ApiDocumentEndpoint = {
  id: number
  projectId: number
  name: string
  method: string
  url: string
  kind: string
  enabled: boolean
  schema?: Record<string, any>
}

export type ApiDocumentGroup = {
  name: string
  description: string
  endpoints: ApiDocumentEndpoint[]
  order: number
}

export function documentationFor(endpoint: ApiDocumentEndpoint) {
  const schema = endpoint.schema || {}
  const metadata = schema['x-pulse-document'] || {}
  const tags = Array.isArray(schema.tags) ? schema.tags.filter((tag: unknown): tag is string => typeof tag === 'string') : []
  const path = typeof metadata.path === 'string' && metadata.path ? metadata.path : endpointPath(endpoint.url)
  return {
    method: endpoint.method.toUpperCase(),
    path,
    summary: typeof schema.summary === 'string' && schema.summary.trim() ? schema.summary.trim() : endpoint.name,
    description: typeof schema.description === 'string' ? schema.description : '',
    tags,
    metadata,
  }
}

export function endpointPath(value: string) {
  const input = String(value || '').trim()
  if (!input) return '/'
  try {
    return input.startsWith('/') ? input.split(/[?#]/, 1)[0] || '/' : new URL(input).pathname || '/'
  } catch {
    return input.split(/[?#]/, 1)[0] || '/'
  }
}

export function groupApiDocuments(endpoints: ApiDocumentEndpoint[]): ApiDocumentGroup[] {
  const groups = new Map<string, ApiDocumentGroup>()
  for (const endpoint of endpoints) {
    const details = documentationFor(endpoint)
    const name = details.tags[0] || '未分组'
    if (!groups.has(name)) {
      const descriptions = details.metadata.tagDescriptions || {}
      const tagOrder = Array.isArray(details.metadata.tagOrder) ? details.metadata.tagOrder : []
      const declaredIndex = tagOrder.indexOf(name)
      groups.set(name, {
        name,
        description: typeof descriptions[name] === 'string' ? descriptions[name] : '',
        endpoints: [],
        order: declaredIndex < 0 ? Number.MAX_SAFE_INTEGER : declaredIndex,
      })
    }
    groups.get(name)!.endpoints.push(endpoint)
  }
  const result = [...groups.values()].sort((a, b) => a.order - b.order || a.name.localeCompare(b.name, 'zh-CN'))
  const methodOrder = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']
  for (const group of result) {
    group.endpoints.sort((a, b) => {
      const left = documentationFor(a), right = documentationFor(b)
      return left.path.localeCompare(right.path, 'zh-CN') || methodOrder.indexOf(left.method) - methodOrder.indexOf(right.method)
    })
  }
  return result
}

export function filterApiDocumentGroups(groups: ApiDocumentGroup[], query: string, method = '全部') {
  const normalizedQuery = query.trim().toLocaleLowerCase()
  const normalizedMethod = method.toUpperCase()
  return groups.map(group => ({
    ...group,
    endpoints: group.endpoints.filter(endpoint => {
      const details = documentationFor(endpoint)
      const methodMatches = normalizedMethod === '全部' || details.method === normalizedMethod
      const queryMatches = !normalizedQuery || `${group.name} ${details.summary} ${details.description} ${details.path} ${details.method}`.toLocaleLowerCase().includes(normalizedQuery)
      return methodMatches && queryMatches
    }),
  })).filter(group => group.endpoints.length > 0)
}

export type SchemaField = { name: string; type: string; required: boolean; description: string }

export function flattenSchemaFields(schema: any, requiredNames: string[] = [], prefix = '', depth = 0): SchemaField[] {
  if (!schema || typeof schema !== 'object' || depth > 4) return []
  const properties = schema.properties && typeof schema.properties === 'object' ? schema.properties : null
  if (properties) {
    const required = Array.isArray(schema.required) ? schema.required : requiredNames
    return Object.entries(properties).flatMap(([name, raw]) => {
      const field = raw && typeof raw === 'object' ? raw as Record<string, any> : {}
      const fullName = prefix ? `${prefix}.${name}` : name
      const type = schemaType(field)
      const own = [{ name: fullName, type, required: required.includes(name), description: typeof field.description === 'string' ? field.description : '' }]
      const nested = field.type === 'array'
        ? flattenSchemaFields(field.items, [], `${fullName}[]`, depth + 1)
        : flattenSchemaFields(field, [], fullName, depth + 1)
      return [...own, ...nested]
    })
  }
  if (schema.type === 'array') return flattenSchemaFields(schema.items, [], prefix ? `${prefix}[]` : '[]', depth + 1)
  return []
}

export function schemaType(schema: Record<string, any>) {
  const type = typeof schema.type === 'string' ? schema.type : ''
  const format = typeof schema.format === 'string' ? schema.format : ''
  if (type && format) return `${type} (${format})`
  if (type) return type
  const reference = schema['x-pulse-ref'] || schema.$ref
  if (typeof reference === 'string') return reference.split('/').at(-1) || '引用类型'
  if (schema.properties) return 'object'
  return '未声明'
}
