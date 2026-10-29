<template>
  <!-- 图谱画布容器：尺寸由 CSS 控制，随页面全宽自适应 -->
  <div ref="chartRef" class="graph-chart decorate-panel"></div>
</template>

<script setup>
/**
 * ECharts 知识图谱画布（纯渲染组件）
 * - echarts 按需注册（core + Graph 图 + 常用组件 + Canvas 渲染器），减小产物体积
 * - ResizeObserver 监听容器尺寸自适应（全宽布局下随窗口伸缩）
 * - 组件卸载时 dispose + 断开监听，防止内存泄漏
 * - 数据契约：graph 为已解析对象 { nodes, links, categories }（解析防护由页面层完成）
 */
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import * as echarts from 'echarts/core'
import { GraphChart as EChartsGraph } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  ToolboxComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

echarts.use([EChartsGraph, TitleComponent, TooltipComponent, LegendComponent, ToolboxComponent, CanvasRenderer])

const props = defineProps({
  // 已通过 parseGraphData 防护解析的图谱数据对象
  graph: { type: Object, required: true },
})

const chartRef = ref(null)
let chart = null
let resizeObserver = null

// 深色科技风节点色板：高亮高饱和多色，保证节点与深色背景强区分（变更 #1）
const NODE_COLORS = ['#22f4ff', '#4d9fff', '#8f7bff', '#2de2c3', '#ff6bd6']

// 组装深色科技风图表配置
function buildOption({ nodes, links, categories }) {
  return {
    backgroundColor: 'transparent',
    // 分类色板：节点按 category 序号循环取色
    color: NODE_COLORS,
    title: {
      text: '知识图谱',
      // 注意：canvas 不解析 CSS 变量，必须使用字面量色值（变更 #1 缺陷修复）
      textStyle: {
        color: '#e2e8f0',
        fontSize: 16,
        fontWeight: 600,
        textShadowColor: 'rgba(34, 244, 255, 0.6)',
        textShadowBlur: 8,
      },
      left: 16,
      top: 12,
    },
    tooltip: {
      backgroundColor: 'rgba(16, 24, 48, 0.92)',
      borderColor: 'rgba(34, 244, 255, 0.45)',
      textStyle: { color: '#e2e8f0' },
      // 契约修正：节点仅有 name 字段、关系仅有 source/target/value，
      // 旧代码引用不存在的 data.des 字段导致悬停内容为空
      formatter(params) {
        if (params.dataType === 'edge') {
          return `${params.data.source} → ${params.data.target}：${params.data.value}`
        }
        return params.data.name
      },
    },
    toolbox: {
      show: true,
      iconStyle: { borderColor: '#7c8db0' }, // 深色背景下工具图标可见性
      feature: {
        restore: { show: true },
        saveAsImage: { show: true, backgroundColor: '#060a16' }, // 导出图带底色
      },
    },
    legend: [
      {
        data: (categories || []).map((item) => item.name),
        textStyle: { color: '#7c8db0' },
        top: 12,
      },
    ],
    series: [
      {
        type: 'graph',
        layout: 'force',
        symbolSize: 34,
        roam: true, // 支持拖拽缩放
        edgeSymbol: ['circle', 'arrow'],
        edgeSymbolSize: [5, 11],
        label: {
          show: true,
          color: '#f1f5f9',
          fontSize: 12,
          fontWeight: 500,
          // 深色文字描边：任何背景下标签都清晰可读（变更 #1）
          textBorderColor: '#060a16',
          textBorderWidth: 2.5,
          // 文字发光效果：青色阴影增强科技感
          textShadowColor: 'rgba(34, 244, 255, 0.45)',
          textShadowBlur: 5,
        },
        edgeLabel: {
          show: true,
          formatter: (params) => params.data.value,
          color: '#bfe9ff',
          fontSize: 11,
          // 实底深色块 + 描边：关系标签从背景中"浮起"（变更 #1）
          backgroundColor: 'rgba(6, 10, 22, 0.92)',
          borderColor: 'rgba(34, 244, 255, 0.35)',
          borderWidth: 1,
          padding: [3, 6],
          borderRadius: 3,
        },
        itemStyle: {
          // 白色亮环描边：节点轮廓从背景中凸出（变更 #1）
          borderColor: 'rgba(255, 255, 255, 0.35)',
          borderWidth: 1.5,
          // 节点光晕：加大青色阴影
          shadowColor: 'rgba(34, 244, 255, 0.55)',
          shadowBlur: 20,
        },
        lineStyle: {
          width: 2,
          color: 'rgba(77, 159, 255, 0.6)', // 提高不透明度，边与背景区分度更强（变更 #1）
          curveness: 0.15, // 边微弧度，密集图下更易读
        },
        // 悬停邻接高亮：高亮相邻节点与边、淡化无关部分，提升大图可读性（变更 #1）
        emphasis: {
          focus: 'adjacency',
          itemStyle: {
            shadowBlur: 28,
            shadowColor: 'rgba(34, 244, 255, 0.8)',
          },
          lineStyle: { width: 3.5 },
          label: { fontSize: 14, fontWeight: 700 },
        },
        force: {
          repulsion: 500,
          edgeLength: [20, 50],
          friction: 0.2,
          gravity: 0.3,
        },
        draggable: true,
        data: nodes,
        links: links,
        categories: categories,
      },
    ],
  }
}

// 渲染（graph 变化时全量 setOption）
function render() {
  if (!chart) return
  chart.setOption(buildOption(props.graph))
}

onMounted(() => {
  chart = echarts.init(chartRef.value)
  render()

  // 容器尺寸变化时自适应（全宽布局随窗口伸缩）
  resizeObserver = new ResizeObserver(() => chart && chart.resize())
  resizeObserver.observe(chartRef.value)

  // 数据更新时重渲染
  watch(() => props.graph, render, { deep: true })
})

onBeforeUnmount(() => {
  // 资源释放：断开尺寸监听 + 销毁实例，防内存泄漏
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
  if (chart) {
    chart.dispose()
    chart = null
  }
})
</script>

<style scoped>
/* 全宽自适应画布：宽度撑满容器，高度固定视口比例
   深空径向渐变底：中心稍亮、边缘更深，与页面底色形成层次对比（变更 #1） */
.graph-chart {
  width: 100%;
  height: 640px;
  background: radial-gradient(ellipse at center, #101b38 0%, #060a16 72%);
  overflow: hidden;
}
</style>
