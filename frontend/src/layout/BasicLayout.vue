<template>
  <!-- 全局布局：发光标题头部 + 网格背景内容区（路由页面的承载容器） -->
  <a-layout class="basic-layout">
    <a-layout-header class="layout-header">
      <div class="header-inner">
        <h1 class="header-title decorate-title-glow">一键生成知识图谱</h1>
        <span class="header-sub">大模型驱动的实体关系抽取</span>
      </div>
      <div class="decorate-divider header-divider"></div>
    </a-layout-header>

    <a-layout-content class="layout-content decorate-grid">
      <!-- 业务页面出口：路由 children 渲染位 -->
      <router-view />
    </a-layout-content>
  </a-layout>
</template>

<script setup>
// 布局组件：仅负责框架结构与装饰，业务逻辑在各路由页面内
// 页脚已按 REQ-002 变更 #2 删除
</script>

<style scoped>
.basic-layout {
  min-height: 100vh;
}

.layout-header {
  position: sticky;
  top: 0;
  z-index: 100;
  background: rgba(10, 15, 30, 0.92);
  backdrop-filter: blur(6px);
  padding: 0 32px;
}

.header-inner {
  display: flex;
  align-items: center;
  gap: 16px;
  height: 56px;
}

.header-title {
  margin: 0;
  font-size: 20px;
}

.header-sub {
  color: var(--text-sub);
  font-size: 13px;
}

.header-divider {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
}

.layout-content {
  padding: 24px 32px 32px;
  max-width: 1400px;
  width: 100%;
  margin: 0 auto;
}
</style>
