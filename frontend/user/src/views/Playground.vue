<template>
  <div class="playground-page">
    <div class="top-bar">
      <div class="top-row">
        <el-select :teleported="false" v-model="selectedModel" placeholder="选择模型" size="default" style="flex:1" popper-class="pg-select-popper">
          <el-option v-for="m in models" :key="m.id" :label="formatModelLabel(m)" :value="m.name" />
        </el-select>
      </div>
      <div class="top-row">
        <el-select :teleported="false" v-model="selectedKey" placeholder="API Key" size="default" style="flex:1" popper-class="pg-select-popper">
          <el-option v-for="k in apiKeys" :key="k.id" :label="`${k.name} - sk-${k.prefix}…`" :value="k.id" />
        </el-select>
        <button class="icon-btn stream-btn" :class="{active: streamMode}" @click="streamMode = !streamMode">{{ streamMode ? '⚡ ' + t('playground.streamOn') : '⏸ ' + t('playground.streamOff') }}</button>
        <button class="icon-btn" @click="clearChat" title="清空">🗑️</button>
      </div>
    </div>

    <div ref="messagesRef" class="messages-area">
      <div v-if="messages.length === 0" class="empty-state">
        <div class="empty-icon">✨</div>
        <div class="empty-text">{{ t('playground.empty') }}</div>
        <div class="empty-sub" v-if="isImageModel && modelMeta">{{ modelMeta.display_name }} · ¥{{ (modelMeta.cost_per_call || 0).toFixed(3) }}/张</div>
      </div>
      <div v-for="(msg, idx) in messages" :key="idx" class="msg-row" :class="`msg-${msg.role}`">
        <div class="msg-bubble">
          <div v-if="msg.attachments && msg.attachments.length" class="msg-attachments">
            <div v-for="(att, i) in msg.attachments" :key="i" class="att-thumb">
              <img v-if="att.type && att.type.startsWith('image/')" :src="att.url" />
              <div v-else class="file-tag">📎 {{ att.name }}</div>
            </div>
          </div>
          <details v-if="msg.reasoning" class="reasoning-block">
            <summary>💭 思考过程 ({{ msg.reasoning.length }} 字符)</summary>
            <div class="reasoning-text">{{ msg.reasoning }}</div>
          </details>
          <div v-if="msg.text" class="msg-text">{{ msg.text }}</div>
          <div v-if="msg.imageUrl" class="msg-gen-image">
            <img :src="msg.imageUrl" />
            <a href="javascript:void(0)" @click="downloadImage(msg.imageUrl)" class="download-link">⬇️ {{ t('playground.download') }}</a>
          </div>
          <div v-if="msg.meta" class="msg-meta">{{ msg.meta }}</div>
        </div>
      </div>
      <div v-if="sending && (pendingResponse || pendingReasoning)" class="msg-row msg-assistant">
        <div class="msg-bubble">
          <details v-if="pendingReasoning" class="reasoning-block" open>
            <summary>💭 思考中... ({{ pendingReasoning.length }} 字符)</summary>
            <div ref="reasoningScrollRef" class="reasoning-text">{{ pendingReasoning }}</div>
          </details>
          <div v-if="pendingResponse" class="msg-text">{{ pendingResponse }}<span class="cursor">▋</span></div>
          <div v-else-if="pendingReasoning" class="msg-text" style="color:#9ca3af;font-style:italic">正在思考...</div>
        </div>
      </div>
    </div>

    <div class="input-area">
      <div v-if="pendingAttachments.length > 0" class="pending-attachments">
        <div v-for="(att, i) in pendingAttachments" :key="i" class="pending-thumb">
          <img v-if="att.url && att.type && att.type.startsWith('image/')" :src="att.url" />
          <div v-else class="file-tag">📎 {{ att.name }}</div>
          <svg v-if="(att.progress || 0) < 100" class="progress-ring" viewBox="0 0 36 36">
            <circle cx="18" cy="18" r="15" fill="none" stroke="rgba(0,0,0,0.08)" stroke-width="3" />
            <circle cx="18" cy="18" r="15" fill="none" stroke="#667eea" stroke-width="3"
              :stroke-dasharray="`${(att.progress || 0) * 0.94} 94`"
              stroke-linecap="round" transform="rotate(-90 18 18)" />
          </svg>
          <button class="remove-btn" @click="removeAttachment(i)">×</button>
        </div>
      </div>

      <div class="input-bar">
        <el-popover v-model:visible="uploadMenuOpen" placement="top-start" :width="160" trigger="click" :disabled="!supportsVision">
          <template #reference>
            <button class="upload-btn" :class="{disabled: !supportsVision}" :title="supportsVision ? '' : t('playground.textOnlyHint')">+</button>
          </template>
          <div class="upload-menu">
            <button class="upload-option" @click="triggerFileInput('image')">🖼️ {{ t('playground.image') }}</button>
            <button class="upload-option" @click="triggerFileInput('any')">📎 {{ t('playground.file') }}</button>
          </div>
        </el-popover>

        <textarea ref="textareaRef" v-model="userMessage" @keydown="handleKeyDown" @input="autoGrow" :placeholder="t('playground.placeholder')" class="input-textarea" rows="1"></textarea>

        <button class="send-btn" :disabled="sending || !canSend" @click="handleSend">
          {{ sending ? '⏳' : '↑' }}
        </button>
      </div>

      <div v-if="isImageModel" class="image-controls">
        <div class="ctrl-group">
          <label>{{ t('playground.size') }}</label>
          <select v-model="imageSize" class="native-select size-select">
            <option value="1024x1024">📐 1024×1024 方形</option>
            <option value="1024x1536">📱 1024×1536 竖屏</option>
            <option value="1536x1024">🖥️ 1536×1024 横屏</option>
            <option value="2048x2048" :disabled="!imageSizeAvailable('2048x2048')">📐 2K (2048×2048)</option>
            <option value="3840x2160" :disabled="!imageSizeAvailable('3840x2160')">🖥️ 4K (3840×2160) 横</option>
            <option value="2160x3840" :disabled="!imageSizeAvailable('2160x3840')">📱 4K (2160×3840) 竖</option>
          </select>
        </div>
        <div class="ctrl-group">
          <label>{{ t('playground.quality') }}</label>
          <select v-model="imageQuality" class="native-select quality-select">
            <option value="low">{{ t('playground.qualityLow') }}</option>
            <option value="medium">{{ t('playground.qualityMid') }}</option>
            <option value="high">{{ t('playground.qualityHigh') }}</option>
          </select>
        </div>
        <div class="ctrl-meta" v-if="modelMeta">¥{{ (modelMeta.cost_per_call || 0).toFixed(3) }}/张</div>
      </div>
    </div>

    <input ref="fileInputImage" type="file" accept="image/*" multiple style="display:none" @change="handleFileSelect($event)" />
    <input ref="fileInputAny" type="file" multiple style="display:none" @change="handleFileSelect($event)" />
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { userModelsAPI, apiKeysAPI } from '@/utils/api'

const { t } = useI18n()

const STORAGE_KEY = 'playground_messages_v2'

const models = ref([])
const apiKeys = ref([])
const selectedModel = ref('')
const selectedKey = ref('')
const streamMode = ref(true)
const sending = ref(false)
const userMessage = ref('')
const messages = ref([])
const pendingReasoning = ref("")
const reasoningScrollRef = ref(null)
const pendingResponse = ref('')
const pendingAttachments = ref([])
const imageSize = ref('1024x1024')
const imageQuality = ref('high')
const uploadMenuOpen = ref(false)

const messagesRef = ref(null)
const fileInputImage = ref(null)
const fileInputAny = ref(null)
const textareaRef = ref(null)

const modelMeta = computed(() => models.value.find(m => m.name === selectedModel.value))
const isImageModel = computed(() => modelMeta.value && modelMeta.value.cost_per_call > 0)

// 纯文本模型清单 (不支持图片/文件上传)
const TEXT_ONLY_MODELS = [
  'qwen-3-6-35b',
  'minimax-m3.6',
  'deepseek-v4-flash',
  'deepseek-v4-pro',
  'gpt-5.4-compact',
  'gpt-5.5-compact',
  'codex-auto-review'
]
const supportsVision = computed(() => {
  if (!selectedModel.value) return false
  if (isImageModel.value) return true  // image gen 模型自带 vision (用于 edits)
  return !TEXT_ONLY_MODELS.includes(selectedModel.value)
})
const canSend = computed(() => selectedModel.value && selectedKey.value && (userMessage.value.trim() || pendingAttachments.value.length > 0))

watch(pendingReasoning, () => {
  nextTick(() => {
    const el = reasoningScrollRef.value
    if (el) el.scrollTop = el.scrollHeight
  })
})

function computeMaxTokens() {
  const ctx = modelMeta.value?.context_length || 4096
  // 预留 30% 给 prompt, 剩下给 output, 上限 32K
  const budget = Math.floor(ctx * 0.7)
  return Math.min(Math.max(budget, 2048), 32768)
}

function formatModelLabel(m) {
  if (m.cost_per_call > 0) {
    return `${m.display_name}  ¥${(m.cost_per_call).toFixed(2)}/张`
  } else if (m.input_price && m.output_price) {
    const inK = (m.input_price * 1000).toFixed(2)
    const outK = (m.output_price * 1000).toFixed(2)
    return `${m.display_name}  $${inK}/$${outK}`
  }
  return m.display_name
}

function imageSizeAvailable(size) {
  if (selectedModel.value === 'gpt-image-2') return true  // 官转支持全部
  if (selectedModel.value === 'gpt-image-2-pro') {
    // Pro 最大 2K, 不支持 4K
    return !['3840x2160', '2160x3840'].includes(size)
  }
  if (selectedModel.value === 'gpt-image-2-1k') {
    // 1K 经济版只 1024 方形
    return size === '1024x1024'
  }
  return true
}

watch(selectedModel, (m) => {
  if (m === 'gpt-image-2') imageSize.value = '3840x2160'
  else if (m === 'gpt-image-2-pro') imageSize.value = '2048x2048'
  else if (m === 'gpt-image-2-1k') imageSize.value = '1024x1024'
})

watch(userMessage, async (val) => {
  if (val === '') {
    await nextTick()
    if (textareaRef.value) textareaRef.value.style.height = ''
  }
})

watch(messages, (val) => {
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify(val.slice(-50))) } catch {}
}, { deep: true })

function loadHistory() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) messages.value = JSON.parse(raw)
  } catch {}
}

function clearChat() {
  if (!confirm(t('playground.confirmClear'))) return
  messages.value = []
  pendingResponse.value = ''
  pendingAttachments.value = []
  try { localStorage.removeItem(STORAGE_KEY) } catch {}
}

function triggerFileInput(kind) {
  uploadMenuOpen.value = false
  if (kind === 'image') fileInputImage.value?.click()
  else fileInputAny.value?.click()
}

function fileToBase64(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result)
    reader.onerror = reject
    reader.readAsDataURL(file)
  })
}

// Normalize image for OpenAI image edits API:
// - PNG format (OpenAI requires PNG)
// - Square canvas with white padding (edit API prefers square)
// - Max 1024x1024 to stay under 4MB
// Returns { dataUrl, size } (size in bytes of the PNG)
async function normalizeImageForOpenAI(file) {
  const bitmap = await createImageBitmap(file)
  const origW = bitmap.width
  const origH = bitmap.height
  const ratio = origW / origH

  // OpenAI gpt-image-2 edit supports 1024x1024 / 1024x1792 / 1792x1024
  // Pick the target closest to the source aspect ratio, minimize white padding
  let targetW = 1024
  let targetH = 1024
  if (ratio > 1.4) {
    targetW = 1792
    targetH = 1024
  } else if (ratio < 1 / 1.4) {
    targetW = 1024
    targetH = 1792
  }

  const canvas = document.createElement("canvas")
  canvas.width = targetW
  canvas.height = targetH
  const ctx = canvas.getContext("2d")
  ctx.fillStyle = "#ffffff"
  ctx.fillRect(0, 0, targetW, targetH)
  const scale = Math.min(targetW / origW, targetH / origH)
  const w = origW * scale
  const h = origH * scale
  const dx = (targetW - w) / 2
  const dy = (targetH - h) / 2
  ctx.drawImage(bitmap, dx, dy, w, h)
  bitmap.close?.()
  const dataUrl = canvas.toDataURL("image/png")
  const b64 = dataUrl.split(",")[1] || ""
  const size = Math.floor(b64.length * 3 / 4)
  return { dataUrl, size, width: targetW, height: targetH }
}

// Friendly error message translator for upstream errors
// Removes stack traces, request IDs, and translates common upstream errors
// to concise, user-facing messages (bilingual: Chinese + English).
function friendlyError(raw) {
  if (!raw) return ""
  const msg = String(raw)
  const lower = msg.toLowerCase()

  // OpenAI content moderation (image or text)
  if (lower.includes("safety_violations") || lower.includes("safety system") ||
      lower.includes("abuse]") || lower.includes("[sexual") || lower.includes("[violence") ||
      lower.includes("[self_harm") || lower.includes("[hate")) {
    return "该图片或描述可能包含敏感内容，已被上游安全系统拒绝。请更换图片或修改描述后重试。"
  }
  if (lower.includes("moderation") || lower.includes("content_policy_violation")) {
    return "内容不符合上游服务的使用政策，请修改后重试。"
  }
  if (lower.includes("invalid image file") || lower.includes("invalid image")) {
    return "图片格式无效，请使用 PNG/JPG 图片，尺寸不超过 4MB。"
  }
  if (lower.includes("image_too_large") || lower.includes("image too large")) {
    return "图片过大，请压缩后重试（推荐 <4MB）。"
  }

  // Rate limit
  if (lower.includes("rate_limit") || lower.includes("rate limit") ||
      lower.includes("429") || lower.includes("too many requests")) {
    return "请求过于频繁，请稍后再试。"
  }

  // Quota / billing
  if (lower.includes("insufficient_quota") || lower.includes("quota exceeded") ||
      lower.includes("balance") || lower.includes("payment required") || lower.includes("402")) {
    return "余额不足，请前往充值页面。"
  }

  // Context length
  if (lower.includes("context_length_exceeded") || lower.includes("context length") ||
      lower.includes("maximum context") || lower.includes("token limit")) {
    return "输入内容过长，请精简后重试。"
  }

  // Timeout
  if (lower.includes("timeout") || lower.includes("deadline exceeded") || lower.includes("context canceled")) {
    return "上游响应超时，请稍后重试。"
  }

  // Upstream 500 series
  if (lower.includes("upstream 5") || lower.includes("internal server error") ||
      lower.includes("bad gateway") || lower.includes("service unavailable") ||
      lower.includes("gateway timeout")) {
    return "上游服务暂时不可用，请稍后重试。"
  }

  // Auth issues
  if (lower.includes("invalid api key") || lower.includes("unauthorized") || lower.includes("401")) {
    return "认证失败，请检查 API Key 或重新登录。"
  }

  // Model not found / permission denied
  if (lower.includes("model_not_found") || lower.includes("does not exist") || lower.includes("not available")) {
    return "该模型当前不可用，请更换模型。"
  }

  // Strip request IDs and long tails to make raw upstream errors readable
  // Cut at "request ID", "req_", "correlation" etc.
  let cleaned = msg
  const cutMarkers = ["request id", "req_", "correlation", "trace_id", "stack:", "at line"]
  const lowerCleaned = cleaned.toLowerCase()
  for (const marker of cutMarkers) {
    const idx = lowerCleaned.indexOf(marker)
    if (idx > 20 && idx < cleaned.length) {
      cleaned = cleaned.substring(0, idx).trim()
      break
    }
  }
  // Remove trailing sentence starters like "If you believe..." "Contact us..."
  cleaned = cleaned.replace(/If you believe.*/i, "").replace(/Contact us.*/i, "").replace(/Please contact.*/i, "").trim()
  // Trim trailing punctuation garbage
  cleaned = cleaned.replace(/[.,;\s]+$/, "")
  // Cap length to avoid huge dumps
  if (cleaned.length > 200) cleaned = cleaned.substring(0, 200) + "..."
  return cleaned || "请求失败，请重试。"
}

async function handleFileSelect(event) {
  const files = Array.from(event.target.files || [])
  for (const file of files) {
    if (file.size > 10 * 1024 * 1024) {
      ElMessage.warning(`${file.name} ${t('playground.tooLarge')}`)
      continue
    }
    // 先插入占位 (progress: 0), FileReader 边读边更新
    const placeholder = { type: file.type || '', name: file.name, url: '', base64: '', progress: 0 }
    pendingAttachments.value.push(placeholder)
    const idx = pendingAttachments.value.length - 1

    try {
      if (file.type && file.type.startsWith("image/")) {
        pendingAttachments.value[idx].progress = 20
        const { dataUrl, size } = await normalizeImageForOpenAI(file)
        if (pendingAttachments.value[idx]) {
          pendingAttachments.value[idx].url = dataUrl
          pendingAttachments.value[idx].base64 = dataUrl
          pendingAttachments.value[idx].type = "image/png"
          pendingAttachments.value[idx].name = pendingAttachments.value[idx].name.replace(/\.(jpg|jpeg|webp|gif|bmp)$/i, ".png")
          pendingAttachments.value[idx].progress = 100
        }
        if (size > 4 * 1024 * 1024) {
          ElMessage.warning(`${file.name}: PNG too large after normalize (${(size/1024/1024).toFixed(1)}MB)`)
        }
      } else {
        await new Promise((resolve, reject) => {
          const reader = new FileReader()
          reader.onprogress = (e) => {
            if (e.lengthComputable && pendingAttachments.value[idx]) {
              pendingAttachments.value[idx].progress = Math.max(5, Math.round((e.loaded / e.total) * 100))
            }
          }
          reader.onload = () => {
            if (pendingAttachments.value[idx]) {
              pendingAttachments.value[idx].url = reader.result
              pendingAttachments.value[idx].base64 = reader.result
              pendingAttachments.value[idx].progress = 100
            }
            resolve()
          }
          reader.onerror = reject
          reader.readAsDataURL(file)
        })
      }
    } catch {
      ElMessage.error(`${file.name}: ${t('playground.readFailed')}`)
      const failIdx = pendingAttachments.value.indexOf(placeholder)
      if (failIdx >= 0) pendingAttachments.value.splice(failIdx, 1)
    }
  }
  event.target.value = ''
}

function removeAttachment(idx) { pendingAttachments.value.splice(idx, 1) }

function handleKeyDown(e) {
  if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
    e.preventDefault()
    handleSend()
  }
}

function autoGrow() {
  const el = textareaRef.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = Math.min(el.scrollHeight, 200) + 'px'
}

async function scrollToBottom() {
  await nextTick()
  if (messagesRef.value) messagesRef.value.scrollTop = messagesRef.value.scrollHeight
}

async function downloadImage(imageUrl) {
  const isIOS = /iPhone|iPad|iPod/i.test(navigator.userAgent)
  try {
    // 用 fetch 转 blob (base64 或远程 URL 都能用 fetch 读)
    const response = await fetch(imageUrl)
    if (!response.ok) throw new Error('fetch failed')
    const blob = await response.blob()
    const filename = `transitai-${Date.now()}.png`

    // iOS: Web Share API 调出系统分享/保存到相册菜单 (iOS 13+)
    if (isIOS && navigator.share && navigator.canShare) {
      const file = new File([blob], filename, { type: blob.type || 'image/png' })
      if (navigator.canShare({ files: [file] })) {
        await navigator.share({ files: [file] })
        return
      }
    }

    // 桌面: blob URL + <a download> 强制下载
    const blobUrl = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = blobUrl
    a.download = filename
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    setTimeout(() => URL.revokeObjectURL(blobUrl), 1000)
  } catch (e) {
    // 终极 fallback: 新窗口打开, 用户手动长按保存
    window.open(imageUrl, '_blank')
    ElMessage.info(isIOS ? '长按图片选"存储到照片"' : '已新窗口打开, 右键另存')
  }
}

async function handleSend() {
  if (!canSend.value || sending.value) return

  const userMsg = { role: 'user', text: userMessage.value.trim(), attachments: [...pendingAttachments.value], timestamp: Date.now() }
  messages.value.push(userMsg)
  const sentMessage = userMessage.value.trim()
  userMessage.value = ''
  pendingAttachments.value = []
  scrollToBottom()

  sending.value = true
  pendingResponse.value = ''
  const start = performance.now()

  try {
    const token = localStorage.getItem('user_token')

    if (isImageModel.value) {
      // 检测附件: 有图就走 edits, 无图走 generations
      const imageAtt = userMsg.attachments.find(a => a.type && a.type.startsWith('image/'))
      const isEdit = !!imageAtt
      const action = isEdit ? '编辑' : '生成'
      pendingResponse.value = `${action}图片中... (${imageSize.value}, ~30s)`
      const body = isEdit
        ? { model: selectedModel.value, prompt: sentMessage, image: imageAtt.url, size: imageSize.value, quality: imageQuality.value, n: 1 }
        : { model: selectedModel.value, prompt: sentMessage, size: imageSize.value, quality: imageQuality.value, n: 1 }
      const res = await fetch(`/v1/user/playground/chat?api_key_id=${selectedKey.value}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify(body)
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }))
        throw new Error(err.error?.message || 'Image gen failed')
      }
      const data = await res.json()
      const b64 = data.data?.[0]?.b64_json
      const url = data.data?.[0]?.url
      const imageUrl = b64 ? `data:image/png;base64,${b64}` : url
      if (!imageUrl) throw new Error('No image data')

      const latency = Math.round(performance.now() - start)
      messages.value.push({ role: 'assistant', imageUrl, meta: `${imageSize.value} · ¥${(modelMeta.value?.cost_per_call || 0).toFixed(2)} · ${latency}ms`, timestamp: Date.now() })
      pendingResponse.value = ''
      window.dispatchEvent(new Event('balance-changed'))
      scrollToBottom()
      return
    }

    const apiMessages = messages.value.map(m => {
      if (m.role === 'user') {
        if (m.attachments && m.attachments.length > 0) {
          const content = []
          if (m.text) content.push({ type: 'text', text: m.text })
          for (const att of m.attachments) {
            if (att.type && att.type.startsWith('image/')) content.push({ type: 'image_url', image_url: { url: att.base64 } })
          }
          return { role: 'user', content }
        }
        return { role: 'user', content: m.text }
      } else if (m.role === 'assistant') {
        if (m.imageUrl && !m.text) return null
        return { role: 'assistant', content: m.text || '' }
      }
      return null
    }).filter(Boolean)

    if (streamMode.value) {
      const res = await fetch(`/v1/user/playground/chat?api_key_id=${selectedKey.value}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ model: selectedModel.value, messages: apiMessages, stream: true, max_tokens: computeMaxTokens() })
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }))
        throw new Error(err.error?.message || 'Request failed')
      }
      const reader = res.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let totalTokens = 0
      let thinkingTokens = 0
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop()
        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const data = line.slice(6)
            if (data === '[DONE]') continue
            try {
              const parsed = JSON.parse(data)
              const delta = parsed.choices?.[0]?.delta || {}
              const content = delta.content || parsed.choices?.[0]?.text || ''
              const reasoning = delta.reasoning_content || delta.reasoning || ''
              pendingResponse.value += content
              if (reasoning) pendingReasoning.value += reasoning
              if (parsed.usage) {
                totalTokens = parsed.usage.total_tokens || 0
                thinkingTokens = Math.max(0, totalTokens - (parsed.usage.prompt_tokens || 0) - (parsed.usage.completion_tokens || 0))
              }
              scrollToBottom()
            } catch {}
          }
        }
      }
      const latency = Math.round(performance.now() - start)
      messages.value.push({ role: 'assistant', text: pendingResponse.value, reasoning: pendingReasoning.value, meta: `${totalTokens} tokens${thinkingTokens > 0 ? ` (🤔${thinkingTokens})` : ''} · ${latency}ms`, timestamp: Date.now() })
      pendingResponse.value = ''
      pendingReasoning.value = ''
      window.dispatchEvent(new Event('balance-changed'))
    } else {
      const res = await fetch(`/v1/user/playground/chat?api_key_id=${selectedKey.value}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ model: selectedModel.value, messages: apiMessages, max_tokens: computeMaxTokens() })
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }))
        throw new Error(err.error?.message || 'Request failed')
      }
      const data = await res.json()
      const msgObj = data.choices?.[0]?.message || {}
      const reply = msgObj.content || JSON.stringify(data, null, 2)
      const reasoning = msgObj.reasoning_content || msgObj.reasoning || ''
      const totalTokens = data.usage?.total_tokens || 0
      const thinkingTokens = Math.max(0, totalTokens - (data.usage?.prompt_tokens || 0) - (data.usage?.completion_tokens || 0))
      const latency = Math.round(performance.now() - start)
      messages.value.push({ role: 'assistant', text: reply, reasoning, meta: `${totalTokens} tokens${thinkingTokens > 0 ? ` (🤔${thinkingTokens})` : ''} · ${latency}ms`, timestamp: Date.now() })
      window.dispatchEvent(new Event('balance-changed'))
    }
  } catch (err) {
    const friendly = friendlyError(err && err.message)
    messages.value.push({ role: 'assistant', text: `❌ ${friendly}`, meta: 'Failed', timestamp: Date.now() })
    pendingResponse.value = ''
  } finally {
    sending.value = false
    scrollToBottom()
  }
}

// 监听键盘弹起/收起, 重置 scroll 防止页面错位
// Mobile viewport: 浏览器原生处理 + viewport meta interactive-widget hint (Day 6 用 detach 方案重做)

onMounted(async () => {
  loadHistory()
  try {
    const [modelData, keyData] = await Promise.all([userModelsAPI.list(), apiKeysAPI.list()])
    models.value = modelData.items || []
    apiKeys.value = keyData
    if (models.value.length > 0) {
      const sonnet = models.value.find(m => m.name && m.name.includes('sonnet-4-6'))
      selectedModel.value = sonnet ? sonnet.name : models.value[0].name
    }
    if (apiKeys.value.length > 0) selectedKey.value = apiKeys.value[0].id
  } catch {}
  scrollToBottom()
  
  // 移动端: 输入框失焦后强制 scroll 到顶, 防止 iOS Safari 不自动归位
  if (window.matchMedia && window.matchMedia('(max-width: 1023px)').matches) {
    const handleBlur = () => {
      setTimeout(() => {
        window.scrollTo({ top: 0, behavior: 'auto' })
        document.documentElement.scrollTop = 0
        document.body.scrollTop = 0
      }, 100)
    }
    const textareas = document.querySelectorAll('.input-textarea')
    textareas.forEach(ta => ta.addEventListener('blur', handleBlur))
  }
})
</script>

<style scoped>
.playground-page {
  display: flex;
  flex-direction: column;
  height: calc(100dvh - 56px);
  height: calc(var(--vvh, 100dvh) - 56px);
  margin: -14px;
  background: #f9fafb;
  box-sizing: border-box;
  overflow: hidden;
}
/* PC 端: 用 fixed 定位撑满 main-area 可视区 */
@media (min-width: 769px) {
  .playground-page {
    height: 100vh;
    margin: -32px -48px;
    background: #f9fafb;
  }
}
.top-bar { background: #fff; padding: 10px 14px; border-bottom: 1px solid #e5e7eb; display: flex; flex-direction: column; gap: 8px; flex-shrink: 0; }
.top-row { display: flex; gap: 8px; align-items: center; }
.icon-btn { width: 36px; height: 36px; border: 1px solid #e5e7eb; background: #fff; border-radius: 8px; font-size: 14px; cursor: pointer; flex-shrink: 0; transition: all 0.15s; }
.icon-btn.active { background: linear-gradient(135deg, #667eea, #764ba2); color: #fff; border-color: transparent; box-shadow: 0 2px 6px rgba(102,126,234,0.3); }
.icon-btn:active { transform: scale(0.95); }
.icon-btn.stream-btn { width: auto; padding: 0 10px; font-size: 13px; min-width: 70px; }
.messages-area { flex: 1; overflow-y: auto; -webkit-overflow-scrolling: touch; padding: 16px; display: flex; flex-direction: column; gap: 12px; }
.empty-state { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; color: #9ca3af; padding-bottom: 40px; }
.empty-icon { font-size: 56px; margin-bottom: 12px; }
.empty-text { font-size: 16px; font-weight: 500; }
.empty-sub { font-size: 13px; margin-top: 8px; color: #6b7280; }
.msg-row { display: flex; }
.msg-user { justify-content: flex-end; }
.msg-assistant { justify-content: flex-start; }
.msg-bubble { max-width: 85%; padding: 12px 14px; border-radius: 16px; font-size: 14px; line-height: 1.5; }
.msg-user .msg-bubble { background: linear-gradient(135deg, #667eea, #764ba2); color: #fff; }
.msg-assistant .msg-bubble { background: #fff; color: #1f2937; border: 1px solid #e5e7eb; }
.msg-text { white-space: pre-wrap; word-break: break-word; }
.msg-attachments { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 8px; }
.att-thumb img { max-width: 120px; max-height: 120px; border-radius: 10px; object-fit: cover; display: block; }
.att-thumb .file-tag { background: rgba(255,255,255,0.2); padding: 6px 10px; border-radius: 8px; font-size: 12px; }
.msg-assistant .file-tag { background: #f3f4f6; color: #4b5563; }
.msg-gen-image { margin-top: 10px; }
.msg-gen-image img { max-width: 100%; border-radius: 10px; display: block; }
.download-link { display: inline-block; margin-top: 6px; font-size: 12px; color: #667eea; text-decoration: none; padding: 4px 10px; background: #f3f4f6; border-radius: 6px; }
.msg-meta { margin-top: 6px; font-size: 11px; opacity: 0.6; }
.cursor { display: inline-block; animation: blink 1s infinite; color: #667eea; font-weight: 700; margin-left: 2px; }
@keyframes blink { 0%, 50% { opacity: 1; } 51%, 100% { opacity: 0; } }
.input-area { background: #fff; border-top: 1px solid #e5e7eb; padding: 10px 12px; flex-shrink: 0; }
.pending-attachments { display: flex; gap: 12px; flex-wrap: wrap; margin-bottom: 12px; padding: 6px 2px; }
.pending-thumb { position: relative; width: 72px; height: 72px; }
.pending-thumb img { width: 100%; height: 100%; object-fit: cover; border-radius: 8px; }
.pending-thumb .file-tag { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; background: #f3f4f6; color: #4b5563; border-radius: 8px; font-size: 10px; padding: 4px; text-align: center; word-break: break-all; overflow: hidden; }
.progress-ring { position: absolute; top: 0; left: 0; width: 100%; height: 100%; pointer-events: none; }
.progress-ring circle:nth-child(2) { transition: stroke-dasharray 0.15s ease-out; }
.remove-btn { position: absolute; top: -6px; right: -6px; width: 18px; height: 18px; border-radius: 50%; background: #ef4444; color: #fff; border: none; font-size: 12px; line-height: 1; cursor: pointer; display: flex; align-items: center; justify-content: center; }
.input-bar { display: flex; align-items: center; gap: 8px; background: #f3f4f6; border-radius: 22px; padding: 6px; }
.upload-btn { width: 36px; height: 36px; min-width: 36px; border: none; border-radius: 50%; background: #e5e7eb; color: #4b5563; font-size: 24px; line-height: 1; cursor: pointer; display: flex; align-items: center; justify-content: center; padding-bottom: 3px; }
.upload-btn:active { background: #d1d5db; }
.upload-btn.disabled { opacity: 0.35; cursor: not-allowed; }
.upload-btn.disabled:active { transform: none; }
.input-textarea { flex: 1; border: none; background: transparent; font-size: 16px; resize: none; max-height: 200px; min-height: 40px; padding: 10px 6px; outline: none; font-family: inherit; line-height: 1.5; -webkit-user-select: text; }
.send-btn { width: 36px; height: 36px; min-width: 36px; border: none; border-radius: 50%; background: linear-gradient(135deg, #667eea, #764ba2); color: #fff; font-size: 18px; cursor: pointer; box-shadow: 0 2px 6px rgba(102,126,234,0.3); display: flex; align-items: center; justify-content: center; }
.send-btn:disabled { opacity: 0.4; background: #9ca3af; box-shadow: none; }
.native-select {
  height: 28px;
  padding: 0 8px;
  border: 1px solid #dcdfe6;
  border-radius: 6px;
  background: #fff;
  font-size: 13px;
  color: #1f2937;
  cursor: pointer;
  outline: none;
  -webkit-appearance: menulist;
  appearance: menulist;
}
.native-select:focus { border-color: #667eea; }
.size-select { width: 180px; }
.quality-select { width: 90px; }
.image-controls { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; margin-top: 10px; padding: 0 6px; font-size: 12px; }
.ctrl-group { display: flex; align-items: center; gap: 6px; color: #4b5563; }
.ctrl-group label { font-weight: 500; }
.ctrl-meta { font-size: 11px; color: #6b7280; margin-left: auto; }
.upload-menu { display: flex; flex-direction: column; gap: 2px; }
.upload-option { background: transparent; border: none; text-align: left; padding: 8px 12px; font-size: 14px; cursor: pointer; border-radius: 6px; color: #1f2937; width: 100%; }
.upload-option:active { background: #f3f4f6; }


/* 修复 image controls select popper 引起的 layout 抖动 */
.pg-select-popper {
  position: fixed !important;
  z-index: 9999 !important;
}
/* 强制 image-controls 高度恒定, 避免 popper 触发后内容上移 */
.image-controls {
  min-height: 36px;
  contain: layout style;
}
.input-area {
  contain: layout style;
}
</style>

<style>
/* 提升 topbar z-index, 防止 Playground fixed positioning 在 iOS 键盘事件后覆盖 */
/* mobile viewport: 浏览器默认处理 */

.reasoning-block {
  margin-bottom: 8px;
  font-size: 12.5px;
}
.reasoning-block summary {
  cursor: pointer;
  color: #9ca3af;
  user-select: none;
  padding: 4px 0 4px 18px;
  position: relative;
  list-style: none;
  transition: color 0.15s;
}
.reasoning-block summary::-webkit-details-marker { display: none; }
.reasoning-block summary::before {
  content: '';
  position: absolute;
  left: 4px;
  top: 50%;
  width: 0;
  height: 0;
  transform: translateY(-50%);
  border-left: 4px solid #9ca3af;
  border-top: 3px solid transparent;
  border-bottom: 3px solid transparent;
  transition: transform 0.15s;
  transform-origin: 2px center;
}
.reasoning-block[open] summary::before {
  transform: translateY(-50%) rotate(90deg);
}
.reasoning-block summary:hover { color: #6b7280; }
.reasoning-text {
  margin: 4px 0 4px 18px;
  padding-left: 12px;
  border-left: 2px solid #e5e7eb;
  color: #6b7280;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 320px;
  overflow-y: auto;
  line-height: 1.65;
  font-size: 12.5px;
}
</style>
