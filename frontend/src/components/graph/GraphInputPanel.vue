<template>
  <!-- 输入面板：文本输入 + PDF 上传两种生成方式 -->
  <div class="input-panel decorate-panel decorate-grid">
    <!-- 文本输入区 -->
    <div class="section">
      <div class="section-title">文本生成</div>
      <a-textarea
        v-model:value="text"
        :rows="6"
        placeholder="请输入文本，大模型将自动拆解并生成知识图谱…"
        :maxlength="50000"
      />
      <div class="section-actions">
        <a-button
          type="primary"
          ghost
          :loading="loading"
          :disabled="!text.trim()"
          @click="onSubmitText"
        >
          生成图谱
        </a-button>
      </div>
    </div>

    <a-divider class="panel-divider" plain>或</a-divider>

    <!-- 文件上传区：仅接受 PDF，前端拦截类型与大小 -->
    <div class="section">
      <div class="section-title">文件生成</div>
      <div class="upload-row">
        <a-upload accept=".pdf" :show-upload-list="false" :before-upload="beforeUpload">
          <a-button>
            <UploadOutlined />
            选择 PDF 文件
          </a-button>
        </a-upload>
        <span class="file-name" v-if="file">{{ file.name }}</span>
        <a-button
          type="primary"
          ghost
          :loading="loading"
          :disabled="!file"
          @click="onSubmitFile"
        >
          上传解析
        </a-button>
      </div>
      <div class="upload-tip">支持 .pdf 格式，大小不超过 20MB</div>
    </div>
  </div>
</template>

<script setup>
/**
 * 图谱输入面板（受控组件）
 * - 职责边界：只做输入采集与前置校验，不发起请求（由页面层统一编排 API 调用）
 * - 校验规则：PDF 类型（后端走 python OCR 解析）、大小 ≤ 20MB，违规中文提示
 * - beforeUpload 返回 false 阻止 a-upload 自动上传，文件暂存后由页面手动提交
 */
import { ref } from 'vue'
import { App } from 'ant-design-vue'
import { UploadOutlined } from '@ant-design/icons-vue'

// 通过 App 组件的 useApp 静态方法获取 message 实例（继承 a-app 上下文与深色主题 token）
const { message } = App.useApp()

// 上传文件大小上限（MB）
const MAX_FILE_SIZE_MB = 20

defineProps({
  // 请求进行中：两个提交按钮共享 loading 态，防重复提交
  loading: { type: Boolean, default: false },
})

const emit = defineEmits(['submit-text', 'submit-file'])

const text = ref('')
const file = ref(null)

// 文本生成
function onSubmitText() {
  const trimmed = text.value.trim()
  if (!trimmed) return
  emit('submit-text', trimmed)
}

// 上传前置校验：类型 + 大小，违规提示并拦截
function beforeUpload(rawFile) {
  const isPdf =
    rawFile.type === 'application/pdf' || rawFile.name.toLowerCase().endsWith('.pdf')
  if (!isPdf) {
    message.warning('仅支持上传 PDF 文件')
    return false
  }
  if (rawFile.size / 1024 / 1024 > MAX_FILE_SIZE_MB) {
    message.warning(`文件大小不能超过 ${MAX_FILE_SIZE_MB}MB`)
    return false
  }
  file.value = rawFile
  // 阻止组件自动上传，改为页面层手动提交
  return false
}

// 文件生成
function onSubmitFile() {
  if (!file.value) return
  emit('submit-file', file.value)
}
</script>

<style scoped>
.input-panel {
  padding: 20px 24px;
}

.section-title {
  color: var(--primary);
  font-weight: 600;
  margin-bottom: 10px;
  letter-spacing: 1px;
}

.section-actions {
  margin-top: 12px;
}

.panel-divider {
  border-color: var(--border-faint);
  color: var(--text-sub);
  margin: 16px 0;
}

.upload-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.file-name {
  color: var(--text-sub);
  max-width: 240px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.upload-tip {
  margin-top: 8px;
  font-size: 12px;
  color: var(--text-sub);
}
</style>
