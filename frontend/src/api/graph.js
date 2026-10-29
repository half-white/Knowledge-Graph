/**
 * 知识图谱接口封装
 *
 * 后端契约（只读约定，后端零修改）：
 * - POST /model   { title: string }         → { code, data: 图谱JSON字符串, msg: 抽取文本 }
 * - POST /upload  multipart（字段名 file）  → 同上（后端经 python OCR 提取文本后走同一管线）
 */
import request from './index'

/** 文本生成知识图谱 */
export function generateGraphByText(title) {
  return request.post('/model', { title })
}

/** 上传 PDF 文件生成知识图谱 */
export function generateGraphByFile(file) {
  const formData = new FormData()
  formData.append('file', file)
  return request.post('/upload', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}
