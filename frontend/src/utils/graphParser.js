/**
 * 图谱数据解析防护
 *
 * 后端 data 为 JSON 字符串：{ nodes, links, categories }
 * 特殊契约：后端无三元组时（LLM 未抽取到 / OCR 失败静默继续），Go 空切片
 * 序列化为 null，即 { "nodes": null, "links": null }——按业务语义视为"空图谱"，
 * 走空态提示，与"未识别出实体关系"规则对齐；仅非法类型才判定为结构异常。
 * 任一环节异常都返回统一结构，由页面层据此展示错误或空态，杜绝白屏
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

  // 类型校验：nodes/links 为 null 视为空图谱语义（后端 Go 空切片序列化结果），
  // 其余非数组类型（对象/字符串/数字等）才是真正的结构异常
  const isNullOrArray = (v) => v === null || v === undefined || Array.isArray(v)
  if (!isNullOrArray(graph?.nodes) || !isNullOrArray(graph?.links)) {
    return { ok: false, error: '图谱数据结构异常' }
  }

  // 归一化 null 为空数组，保证下游消费安全
  const nodes = graph.nodes || []
  const links = graph.links || []

  // 空图谱：未识别出任何实体，走空态提示
  if (nodes.length === 0) {
    return { ok: false, error: '未识别出实体关系', empty: true }
  }

  return { ok: true, graph: { ...graph, nodes, links } }
}
