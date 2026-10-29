/**
 * 应用入口
 * - ant-design-vue 组件由 unplugin-vue-components 在模板中按需自动引入（见 vite.config.js），
 *   此处仅引入全局 reset 样式；不再全量 import Antd
 * - echarts 由 GraphChart 组件内按需注册，入口不再引入（修复旧代码 echats 拼写与无效 use()）
 * - 深色科技风主题：theme.css 设计 token + decorate.css 装饰图案
 */
import { createApp } from 'vue'
import 'ant-design-vue/dist/reset.css'
import '@/styles/theme.css'
import '@/styles/decorate.css'
import App from './App.vue'
import router from './router'

createApp(App).use(router).mount('#app')
