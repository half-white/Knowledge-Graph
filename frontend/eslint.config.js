// ESLint 9 flat config：JS 推荐规则 + Vue 官方插件（essential 级别）
// 说明：flat/essential 预设已内置 vue-eslint-parser，无需额外配置 parser
// 运行：npm run lint
import js from '@eslint/js'
import pluginVue from 'eslint-plugin-vue'

export default [
  // 全局忽略：构建产物与依赖目录
  { ignores: ['dist/**', 'node_modules/**'] },

  // JS 推荐规则集
  js.configs.recommended,

  // Vue 单文件组件规则（含 parser 与处理器）
  ...pluginVue.configs['flat/essential'],

  // 项目约定
  {
    rules: {
      'no-console': 'warn',
      'no-debugger': 'error',
      'vue/multi-word-component-names': 'off',
    },
  },
]
