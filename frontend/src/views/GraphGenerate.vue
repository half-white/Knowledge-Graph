<template>
  <!-- 图谱生成页（首页）：编排输入 → 请求 → 解析防护 → 渲染 -->
  <div class="graph-generate">
    <GraphInputPanel :loading="loading" @submit-text="onSubmitText" @submit-file="onSubmitFile" />

    <!-- 错误提示：拦截器已收敛为中文 BizError，此处直接展示 -->
    <a-alert
      v-if="errorMsg"
      class="error-alert"
      type="error"
      :message="errorMsg"
      show-icon
      closable
      @close="errorMsg = ''"
    />

    <!-- 图谱渲染：数据已通过 parseGraphData 防护校验 -->
    <GraphChart v-if="graph" :graph="graph" />

    <!-- 空图谱态：后端 OCR 失败静默继续等场景 -->
    <div v-else-if="emptyFlag" class="empty-state decorate-panel">
      <a-empty description="未识别出实体关系，请调整文本后重试" />
    </div>

    <!-- 初始引导态 -->
    <div v-else-if="!loading" class="empty-state decorate-panel decorate-grid">
      <a-empty description="输入文本或上传 PDF，一键生成知识图谱" />
    </div>

    <!-- 实体关系抽取明细：后端 msg 字段（三元组文本） -->
    <a-collapse v-if="extractText" class="extract-panel" ghost>
      <a-collapse-panel key="detail" header="实体关系抽取明细">
        <pre class="extract-text">{{ extractText }}</pre>
      </a-collapse-panel>
    </a-collapse>
  </div>
</template>

<script setup>
/**
 * 图谱生成页（页面编排层）
 * - 唯一发起 API 调用的位置：输入组件只 emit，网络/解析/渲染在此编排
 * - 数据流：API 响应 → parseGraphData 防护解析 → 成功渲染 / 空态 / 错误提示
 */
import { ref } from 'vue'
import GraphInputPanel from '@/components/graph/GraphInputPanel.vue'
import GraphChart from '@/components/graph/GraphChart.vue'
import { generateGraphByText, generateGraphByFile } from '@/api/graph'
import { parseGraphData } from '@/utils/graphParser'

// 页面状态
const loading = ref(false) // 请求进行中（输入面板按钮 loading 防重复提交）
const errorMsg = ref('') // 错误提示（拦截器收敛后的中文文案）
const graph = ref(null) // 已解析的图谱数据（null 表示不渲染画布）
const extractText = ref('') // 后端 msg：实体关系抽取文本明细
const emptyFlag = ref(false) // 空图谱标记（区分"未识别"与"初始引导"两种空态）

// 文本生成入口
async function onSubmitText(text) {
  await requestGraph(() => generateGraphByText(text))
}

// 文件生成入口
async function onSubmitFile(file) {
  await requestGraph(() => generateGraphByFile(file))
}

// 统一请求编排：调 API → 处理成功/异常 → 刷新页面状态
async function requestGraph(requestFn) {
  loading.value = true
  errorMsg.value = ''
  emptyFlag.value = false
  try {
    const res = await requestFn()
    handleResult(res)
  } catch (err) {
    // 拦截器已统一包装为 BizError（含中文提示），直接展示
    errorMsg.value = err.message || '操作失败'
    graph.value = null
    extractText.value = ''
  } finally {
    loading.value = false
  }
}

// 成功响应处理：防护解析，按解析结果分流（渲染/空态/数据异常）
function handleResult(res) {
  const parsed = parseGraphData(res?.data)
  if (parsed.ok) {
    graph.value = parsed.graph
    extractText.value = res?.msg || ''
  } else {
    // 数据为空/非法 JSON/结构缺失/空图谱：不渲染，给出对应提示
    graph.value = null
    extractText.value = ''
    if (parsed.empty) {
      emptyFlag.value = true
    } else {
      errorMsg.value = parsed.error
    }
  }
}
</script>

<style scoped>
.graph-generate {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.error-alert {
  border-color: var(--border-glow);
}

.empty-state {
  min-height: 420px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.extract-panel {
  background: var(--bg-panel);
  border: 1px solid var(--border-faint);
  border-radius: 8px;
  padding: 0 16px;
}

/* 抽取文本：等宽布局 + 自动换行，保留后端原始结构 */
.extract-text {
  color: var(--text-sub);
  font-size: 13px;
  line-height: 1.8;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
  max-height: 320px;
  overflow: auto;
}
</style>
