/**
 * axios 统一实例与响应拦截器
 *
 * 错误收敛约定（详见 REQ-002 需求确认）：
 * 1. 业务错误码：后端参数校验失败返回 HTTP 200 + { code: 7, msg: 中文提示 }，
 *    前端必须校验 code 字段而非仅 HTTP 状态码
 * 2. HTTP 错误：上传环节部分失败返回 4xx/5xx + { message: 中文提示 }（注意字段名是 message 不是 msg）
 * 3. 网络/超时：按 axios 错误码映射中文兜底文案
 *
 * 所有错误统一包装为 BizError 抛出，页面层只管展示 message，不再重复判断
 */
import axios from 'axios'

/** 业务错误：携带可直接展示的中文提示 */
export class BizError extends Error {
  constructor(message) {
    super(message)
    this.name = 'BizError'
  }
}

// 网络层错误码 → 中文提示映射
const NETWORK_ERROR_TEXT = {
  ECONNABORTED: '请求超时，请稍后重试',
  ERR_NETWORK: '网络异常，请检查网络连接后重试',
}

// LLM 生成知识图谱耗时较长（长文本拆片并发调用大模型），超时放宽至 5 分钟
const service = axios.create({
  baseURL: '/api',
  timeout: 300000,
})

// 响应拦截器：统一校验 HTTP 状态与业务 code
service.interceptors.response.use(
  (response) => {
    const res = response.data
    // 业务错误码校验：code 存在且非 0 视为失败（HTTP 200 + code=7 场景）
    if (res && res.code !== undefined && res.code !== 0) {
      return Promise.reject(new BizError(res.msg || '操作失败'))
    }
    return res
  },
  (error) => {
    // HTTP 错误：优先取后端返回的 message 中文字段
    const serverMsg = error.response?.data?.message
    const fallback =
      NETWORK_ERROR_TEXT[error.code] || `请求失败（${error.response?.status ?? '网络异常'}）`
    return Promise.reject(new BizError(serverMsg || fallback))
  }
)

export default service
