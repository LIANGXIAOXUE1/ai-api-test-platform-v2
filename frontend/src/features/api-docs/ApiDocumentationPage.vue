<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import OpenAPIImport from '../shared/OpenAPIImport.vue'
import ApiOperationDetails from './ApiOperationDetails.vue'
import { documentationFor, filterApiDocumentGroups, groupApiDocuments, type ApiDocumentEndpoint } from './catalogue'

const props = defineProps<{
  projectId: number
  projectName: string
  endpoints: ApiDocumentEndpoint[]
  request: (path: string, options?: RequestInit) => Promise<any>
  canImport: boolean
  canTest: boolean
}>()
const emit = defineEmits<{ test: [endpoint: ApiDocumentEndpoint]; imported: [result: any] }>()
const search = ref('')
const method = ref('全部')
const importRef = ref<InstanceType<typeof OpenAPIImport> | null>(null)
const expandedGroups = ref<Record<string, boolean>>({})
const groups = computed(() => filterApiDocumentGroups(groupApiDocuments(props.endpoints), search.value, method.value))
const documentedTitle = computed(() => props.endpoints.map(item => item.schema?.['x-pulse-document']?.title).find(Boolean) || '接口文档')
const methodOptions = ['全部', 'GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS']

watch(groups, value => {
  for (const group of value) if (!(group.name in expandedGroups.value)) expandedGroups.value[group.name] = true
}, { immediate: true })

function operation(endpoint: ApiDocumentEndpoint) { return documentationFor(endpoint) }
function toggleGroup(name: string) { expandedGroups.value[name] = !expandedGroups.value[name] }
function toggleAll(open: boolean) { for (const group of groups.value) expandedGroups.value[group.name] = open }
function importComplete(result: any) { emit('imported', result) }
</script>

<template>
  <main class="list-page api-doc-page">
    <header class="api-doc-page-head">
      <div class="api-doc-heading-copy">
        <span class="eyebrow">API REFERENCE</span>
        <h1>{{ documentedTitle }}</h1>
        <p><b>{{ projectName }}</b><span> · </span><span>按业务标签浏览接口契约</span></p>
      </div>
      <div class="api-doc-page-stats"><b>{{ endpoints.length }}</b><span>个接口</span><i></i><b>{{ groups.length }}</b><span>个分组</span></div>
    </header>

    <section class="api-doc-toolbar" aria-label="筛选接口">
      <label class="api-doc-search"><span aria-hidden="true">⌕</span><input v-model="search" type="search" placeholder="搜索接口名称、路径或分组" aria-label="搜索接口名称、路径或分组"></label>
      <div class="api-doc-method-filters" aria-label="请求方法筛选">
        <button v-for="item in methodOptions" :key="item" type="button" :class="{ active: method === item, [`method-${item.toLowerCase()}`]: item !== '全部' }" @click="method = item">{{ item }}</button>
      </div>
      <div class="api-doc-toolbar-actions">
        <button type="button" @click="toggleAll(true)">展开分组</button><button type="button" @click="toggleAll(false)">收起分组</button>
      </div>
    </section>

    <details v-if="canImport" class="api-doc-import-card" :open="endpoints.length === 0">
      <summary><span><b>导入 OpenAPI 文档</b><small>按文档标签整理接口，保留参数和响应结构</small></span><em>导入 / 更新</em></summary>
      <div class="api-doc-import-content"><OpenAPIImport ref="importRef" :project-id="projectId" :request="request" @imported="importComplete" /></div>
    </details>

    <div v-if="!groups.length" class="api-doc-empty">
      <div aria-hidden="true">⌕</div><b>{{ endpoints.length ? '没有匹配的接口' : '当前项目还没有接口文档' }}</b>
      <span>{{ endpoints.length ? '调整搜索内容或请求方法筛选。' : '在上方导入 OpenAPI JSON，或先到测试台添加接口。' }}</span>
      <button v-if="canImport && !endpoints.length" type="button" @click="importRef?.chooseFile()">选择 OpenAPI JSON</button>
    </div>

    <section v-for="group in groups" :key="group.name" class="api-doc-group">
      <button class="api-doc-group-heading" type="button" :aria-expanded="expandedGroups[group.name] !== false" @click="toggleGroup(group.name)">
        <span class="api-doc-group-marker" aria-hidden="true"></span><span class="api-doc-group-copy"><b>{{ group.name }}</b><small v-if="group.description">{{ group.description }}</small></span>
        <span class="api-doc-group-count">{{ group.endpoints.length }} 个接口</span><span class="api-doc-chevron" :class="{open:expandedGroups[group.name] !== false}" aria-hidden="true">⌄</span>
      </button>
      <div v-if="expandedGroups[group.name] !== false" class="api-doc-operation-list">
        <details v-for="endpoint in group.endpoints" :key="endpoint.id" class="api-doc-operation" :class="`method-${operation(endpoint).method.toLowerCase()}`">
          <summary class="api-doc-operation-summary">
            <span class="api-doc-method-badge">{{ operation(endpoint).method }}</span><code class="api-doc-path">{{ operation(endpoint).path }}</code><span class="api-doc-operation-name">{{ operation(endpoint).summary }}</span>
            <span v-if="endpoint.schema?.deprecated" class="api-doc-operation-deprecated">弃用</span><span class="api-doc-operation-chevron" aria-hidden="true">⌄</span>
          </summary>
          <ApiOperationDetails :endpoint="endpoint" :can-test="canTest" @test="emit('test', $event)" />
        </details>
      </div>
    </section>
  </main>
</template>
