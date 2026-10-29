/**
 * 图谱数据解析防护
 *
 * 后端 data 为 JSON 字符串：{ nodes, links, categories }
 * 任一环节异常（空数据 / 非法 JSON / 结构缺失 / 空图谱）都返回统一结构，
 * 由页面层据此展示错误或空态，杜绝 JSON.parse 直接抛错导致白屏
 */

/**
 * 解析图谱数据
 * @param {string} raw 后端返回的 data 字段（JSON 字符串）
 * @returns {{ok: true, graph: object} | {ok: false, error: string, empty?: boolean}}
 */
export function parseGraphData(raw) {
  // 空数据防护
  if (!raw || typeof raw !== 'string') {
    return { ok: false, error: '图谱数据为空' }
  }

  // 非法 JSON 防护
  let graph
  try {
    graph = JSON.parse(raw)
  } catch {
    return { ok: false, error: '图谱数据格式异常' }
  }

  // 结构校验：nodes / links 必须为数组
  if (!Array.isArray(graph?.nodes) || !Array.isArray(graph?.links)) {
    return { ok: false, error: '图谱数据结构异常' }
  }

  // 空图谱：后端 OCR 失败时会静默继续，可能返回空节点
  if (graph.nodes.length === 0) {
    return { ok: false, error: '未识别出实体关系', empty: true }
  }

  return { ok: true, graph }
}
