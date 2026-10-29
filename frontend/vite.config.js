import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import Components from 'unplugin-vue-components/vite'
import { AntDesignVueResolver } from 'unplugin-vue-components/resolvers'

// https://vite.dev/config/
// 说明：vue-devtools 已按 REQ-002 变更 #3 彻底移除，任何环境不注入调试组件
export default defineConfig(() => ({
  plugins: [
    vue(),
    // ant-design-vue 组件按需自动引入（antd 4.x 为 CSS-in-JS，无需组件级样式）
    Components({
      resolvers: [AntDesignVueResolver({ importStyle: false })],
      dts: false,
    }),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  // 本地开发代理：前端代码统一使用相对路径 /api，
  // 开发时由 Vite 转发到本机后端；生产环境由 nginx 代理到 backend 容器
  server: {
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    chunkSizeWarningLimit: 900,
    rollupOptions: {
      output: {
        // 第三方大依赖按包路径分包，利于浏览器长缓存
        // （函数式按路径归组，避免对象式 ['echarts'] 强制引入包入口导致全量打包）
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined
          if (id.includes('echarts') || id.includes('zrender')) return 'echarts'
          if (id.includes('ant-design-vue') || id.includes('@ant-design')) return 'antd'
          if (id.includes('/@vue/') || id.includes('/vue/') || id.includes('vue-router')) {
            return 'vue-vendor'
          }
          return undefined
        },
      },
    },
  },
}))
