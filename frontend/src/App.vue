<template>
  <!-- 根组件：antd 深色主题配置 + message 上下文 + 路由出口 -->
  <a-config-provider :theme="themeConfig">
    <a-app>
      <router-view />
    </a-app>
  </a-config-provider>
</template>

<script setup>
/**
 * 应用根组件
 * - ConfigProvider：darkAlgorithm 全局深色算法 + 科技风主题 token（与 theme.css 色板一致）
 * - a-app：为 useApp() 提供 message 上下文（输入面板校验提示使用）
 */
import { theme } from 'ant-design-vue'

// 主题 token：主色霓虹青，组件底色与 CSS 变量对齐
const themeConfig = {
  algorithm: theme.darkAlgorithm,
  token: {
    colorPrimary: '#00e5ff',
    colorInfo: '#00e5ff',
    colorBgBase: '#0a0f1e',
    colorBgContainer: '#101830',
    colorBgElevated: '#16213c',
    colorBorder: 'rgba(0, 229, 255, 0.18)',
    colorBorderSecondary: 'rgba(0, 229, 255, 0.12)',
    borderRadius: 6,
  },
}
</script>
