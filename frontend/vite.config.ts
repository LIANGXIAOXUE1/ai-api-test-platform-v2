import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  // App 使用运行时 template 字符串，必须显式使用带编译器的 Vue 构建。
  // 否则生产环境会出现请求正常但页面为空白的情况。
  resolve: {
    alias: {
      vue: 'vue/dist/vue.esm-bundler.js'
    }
  },
  server: {
    port: 5173,
    proxy: {
      // Docker Compose 将后端容器的 8080 映射到宿主机 8081；
      // 开发服务器也使用该入口，避免 5175 预览页出现“请求失败”。
      '/api': 'http://localhost:8081'
    }
  }
})
