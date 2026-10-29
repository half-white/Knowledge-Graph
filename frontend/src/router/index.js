/**
 * 路由骨架
 *
 * 结构约定（扩展性预留，REQ-002）：
 * - BasicLayout 作为一级布局，业务页面挂在其 children 下
 * - 新增页面时在 children 中追加即可，无需改动布局
 *
 * 【预留】历史记录页：后端图谱查询接口就绪后，
 * 取消下方 history 路由注释并在页面中加入口即可点亮
 */
import { createRouter, createWebHistory } from 'vue-router'
import BasicLayout from '@/layout/BasicLayout.vue'

const routes = [
  {
    path: '/',
    component: BasicLayout,
    children: [
      {
        path: '',
        name: 'graph-generate',
        component: () => import('@/views/GraphGenerate.vue'),
        meta: { title: '知识图谱生成' },
      },
      // 预留位：历史图谱页（组件骨架已就绪：views/GraphHistory.vue）
      // {
      //   path: 'history',
      //   name: 'graph-history',
      //   component: () => import('@/views/GraphHistory.vue'),
      //   meta: { title: '历史图谱' },
      // },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// 同步页面标题
router.afterEach((to) => {
  document.title = to.meta.title ? `${to.meta.title} - 知识图谱` : '知识图谱'
})

export default router
