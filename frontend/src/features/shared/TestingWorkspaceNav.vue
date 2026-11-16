<script setup lang="ts">
defineProps<{ active: 'projectCheck' | 'evaluation' }>()
const emit = defineEmits<{ select: [page: 'projectCheck' | 'evaluation'] }>()
</script>

<template>
  <nav class="testing-workspace-nav" aria-label="测试与测评类型">
    <button :class="{ active: active === 'projectCheck' }" :aria-current="active === 'projectCheck' ? 'page' : undefined" @click="emit('select', 'projectCheck')">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 4.5H6a2 2 0 0 0-2 2V20h16V6.5a2 2 0 0 0-2-2h-3"/><rect x="9" y="3" width="6" height="4" rx="1"/><path d="m8 13 2.5 2.5L16 10"/></svg>
      项目测试
    </button>
    <button :class="{ active: active === 'evaluation' }" :aria-current="active === 'evaluation' ? 'page' : undefined" @click="emit('select', 'evaluation')">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 19V5M4 19h16M8 16v-4M12 16V8M16 16v-6M20 16v-9"/></svg>
      模型测评
    </button>
  </nav>
</template>
