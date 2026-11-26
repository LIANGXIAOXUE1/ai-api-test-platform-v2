import { computed,ref } from 'vue'
type Operation='generation'|'upload'|'save'
export function useKnowledgeOperation(){
  const activeOperation=ref<Operation|null>(null)
  const busy=computed(()=>activeOperation.value!==null)
  const generating=computed(()=>activeOperation.value==='generation')
  async function run<T>(operation:Operation,task:()=>Promise<T>):Promise<T>{
    if(busy.value)throw new Error('当前操作尚未结束，请等待后重试')
    activeOperation.value=operation
    try{return await task()}finally{activeOperation.value=null}
  }
  return{activeOperation,busy,generating,run}
}
