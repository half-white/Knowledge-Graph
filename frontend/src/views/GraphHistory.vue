<template>
  <!-- 历史图谱页（预留骨架）：后端历史查询接口就绪后，补充列表/详情并注册路由 -->
  <div class="graph-history decorate-panel">
    <a-empty description="历史记录功能开发中，敬请期待" />
  </div>
</template>

<script setup>
// 预留组件位：当前不注册路由、不加入口（用户确认结论）
// 后端已有 saveGraph 存量数据（每次生成图谱存 MySQL），接口就绪后直接在此消费
</script>

<style scoped>
.graph-history {
  min-height: 400px;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
