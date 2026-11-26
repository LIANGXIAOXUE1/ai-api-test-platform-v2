<script setup lang="ts">
import { computed } from 'vue'
import { documentationFor, flattenSchemaFields, schemaType, type ApiDocumentEndpoint } from './catalogue'

const props = defineProps<{ endpoint: ApiDocumentEndpoint; canTest: boolean }>()
const emit = defineEmits<{ test: [endpoint: ApiDocumentEndpoint] }>()
const details = computed(() => documentationFor(props.endpoint))
const operation = computed(() => props.endpoint.schema || {})
const requestBody = computed(() => operation.value.requestBody || null)
const parameters = computed(() => {
  const all = Array.isArray(operation.value.parameters) ? operation.value.parameters : []
  const body = all.filter((item: any) => item?.in === 'body')
  return { regular: all.filter((item: any) => item?.in !== 'body'), body }
})
const bodyMedia = computed(() => {
  const content = requestBody.value?.content
  if (content && typeof content === 'object') return Object.entries(content).map(([type, value]: [string, any]) => ({ type, schema: value?.schema }))
  const bodyParam = parameters.value.body[0]
  if (bodyParam?.schema) return [{ type: 'request body', schema: bodyParam.schema }]
  return []
})
const responses = computed(() => {
  const value = operation.value.responses
  return value && typeof value === 'object' ? Object.entries(value) : []
})
const requirements = computed(() => Array.isArray(operation.value.security) ? operation.value.security : [])
const securitySchemes = computed(() => operation.value['x-pulse-document']?.securitySchemes || {})
const securityLabels = computed(() => [...new Set(requirements.value.flatMap((entry: any) => entry && typeof entry === 'object' ? Object.keys(entry) : []))])
const hasContract = computed(() => parameters.value.regular.length || parameters.value.body.length || bodyMedia.value.length || responses.value.length || securityLabels.value.length)

function fields(schema: any) { return flattenSchemaFields(schema) }
function fieldType(parameter: any) {
  if (parameter?.schema) return schemaType(parameter.schema)
  if (parameter?.type) return `${parameter.type}${parameter.format ? ` (${parameter.format})` : ''}`
  return '未声明'
}
function responseFields(response: any) {
  const content = response?.content
  if (content && typeof content === 'object') {
    const first = Object.values(content)[0] as any
    return fields(first?.schema)
  }
  return fields(response?.schema)
}
function responseMedia(response: any) {
  const content = response?.content
  return content && typeof content === 'object' ? Object.keys(content).join('、') : (response?.schema ? 'Swagger 响应结构' : '')
}
</script>

<template>
  <div class="api-doc-detail">
    <p v-if="details.description" class="api-doc-description">{{ details.description }}</p>
    <div class="api-doc-meta-line">
      <span v-for="tag in details.tags" :key="tag" class="api-doc-tag">{{ tag }}</span>
      <span v-if="operation.deprecated" class="api-doc-deprecated">已弃用</span>
      <span v-if="operation.operationId" class="api-doc-operation-id">operationId · {{ operation.operationId }}</span>
    </div>

    <section v-if="securityLabels.length" class="api-doc-contract-section">
      <div class="api-doc-section-title"><h3>鉴权要求</h3><span>不会展示或读取已保存的密钥</span></div>
      <div class="api-doc-auth-list">
        <div v-for="name in securityLabels" :key="name" class="api-doc-auth-item">
          <b>{{ name }}</b>
          <span>{{ securitySchemes[name]?.type || '安全方案' }}<template v-if="securitySchemes[name]?.scheme"> · {{ securitySchemes[name].scheme }}</template><template v-if="securitySchemes[name]?.in"> · {{ securitySchemes[name].in }}</template></span>
          <small v-if="securitySchemes[name]?.description">{{ securitySchemes[name].description }}</small>
        </div>
      </div>
    </section>

    <section v-if="parameters.regular.length" class="api-doc-contract-section">
      <div class="api-doc-section-title"><h3>请求参数</h3><span>{{ parameters.regular.length }} 项</span></div>
      <div class="api-doc-table-wrap"><table class="api-doc-table"><thead><tr><th>名称</th><th>位置</th><th>类型</th><th>必填</th><th>说明</th></tr></thead>
        <tbody><tr v-for="(item, index) in parameters.regular" :key="`${item.name}-${item.in}-${index}`"><td><code>{{ item.name || '未命名' }}</code></td><td>{{ item.in || '未声明' }}</td><td>{{ fieldType(item) }}</td><td><span :class="item.required ? 'api-doc-required' : 'api-doc-optional'">{{ item.required ? '是' : '否' }}</span></td><td>{{ item.description || '—' }}</td></tr></tbody>
      </table></div>
    </section>

    <section v-if="bodyMedia.length" class="api-doc-contract-section">
      <div class="api-doc-section-title"><h3>请求体</h3><span>{{ requestBody?.required || parameters.body.some((item: any) => item.required) ? '必填' : '结构说明' }}</span></div>
      <div v-for="media in bodyMedia" :key="media.type" class="api-doc-schema-block">
        <div class="api-doc-schema-type">{{ media.type }}</div>
        <div v-if="fields(media.schema).length" class="api-doc-table-wrap"><table class="api-doc-table"><thead><tr><th>字段</th><th>类型</th><th>必填</th><th>说明</th></tr></thead><tbody>
          <tr v-for="field in fields(media.schema)" :key="field.name"><td><code>{{ field.name }}</code></td><td>{{ field.type }}</td><td><span :class="field.required ? 'api-doc-required' : 'api-doc-optional'">{{ field.required ? '是' : '否' }}</span></td><td>{{ field.description || '—' }}</td></tr>
        </tbody></table></div><p v-else class="api-doc-unavailable">文档未声明可展开的字段结构。{{ media.schema ? ` 当前类型：${schemaType(media.schema)}。` : '' }}</p>
      </div>
    </section>

    <section v-if="responses.length" class="api-doc-contract-section">
      <div class="api-doc-section-title"><h3>响应</h3><span>{{ responses.length }} 种状态</span></div>
      <div class="api-doc-responses">
        <article v-for="[status, response] in responses" :key="status" class="api-doc-response">
          <div class="api-doc-response-head"><code>{{ status }}</code><b>{{ response?.description || '未提供说明' }}</b><small>{{ responseMedia(response) }}</small></div>
          <div v-if="responseFields(response).length" class="api-doc-table-wrap"><table class="api-doc-table"><thead><tr><th>字段</th><th>类型</th><th>必填</th><th>说明</th></tr></thead><tbody>
            <tr v-for="field in responseFields(response)" :key="field.name"><td><code>{{ field.name }}</code></td><td>{{ field.type }}</td><td><span :class="field.required ? 'api-doc-required' : 'api-doc-optional'">{{ field.required ? '是' : '否' }}</span></td><td>{{ field.description || '—' }}</td></tr>
          </tbody></table></div>
          <p v-else class="api-doc-unavailable">文档未提供此状态的响应字段结构。</p>
        </article>
      </div>
    </section>

    <div v-if="!hasContract" class="api-doc-unavailable api-doc-no-contract">这个接口没有导入参数或响应结构。页面只展示文档已有内容，不会推测接口字段。</div>
    <div class="api-doc-detail-actions"><small>查看契约不会发送请求。</small><button v-if="canTest" type="button" class="api-doc-test-button" @click="emit('test', endpoint)">去测试台配置 →</button></div>
  </div>
</template>
