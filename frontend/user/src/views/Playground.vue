<template>
  <div class="page">
    <!-- 配置卡 -->
    <div class="data-card">
      <div class="card-header"><span class="card-title">🎮 Playground</span></div>

      <div class="form-row">
        <label class="form-label">{{ t('playground.modelLabel') }}</label>
        <el-select v-model="selectedModel" :placeholder="t('playground.selectModel')" size="large" style="width:100%">
          <el-option v-for="m in models" :key="m.id" :label="m.display_name" :value="m.name" />
        </el-select>
      </div>

      <div class="form-row">
        <label class="form-label">API Key</label>
        <el-select v-model="selectedKey" :placeholder="t('playground.selectKey')" size="large" style="width:100%">
          <el-option v-for="k in apiKeys" :key="k.id" :label="`${k.name} - sk-${k.prefix}…`" :value="k.id" />
        </el-select>
      </div>

      <div class="toggle-row">
        <span class="toggle-label">{{ t('playground.streamMode') }}</span>
        <el-switch v-model="streamMode" />
      </div>
    </div>

    <!-- 输入卡 -->
    <div class="data-card">
      <div class="card-header"><span class="card-title">{{ t('playground.inputCard') }}</span></div>
      <div class="form-row">
        <label class="form-label">{{ t('playground.systemPrompt') }}</label>
        <el-input v-model="systemPrompt" type="textarea" :rows="2" placeholder="You are a helpful assistant." />
      </div>
      <div class="form-row">
        <label class="form-label">{{ t('playground.userMessage') }}</label>
        <el-input v-model="userMessage" type="textarea" :rows="6" :placeholder="t('playground.userMessagePh')" />
      </div>
      <div class="btn-row">
        <button class="primary-btn" :disabled="sending || !selectedKey || !selectedModel" @click="handleSend">
          {{ sending ? t('playground.sending') : t('playground.sendBtn') }}
        </button>
        <button class="secondary-btn" @click="handleClear">{{ t('playground.clearBtn') }}</button>
      </div>
    </div>

    <!-- 响应卡 -->
    <div class="data-card">
      <div class="card-header">
        <span class="card-title">{{ t('playground.responseCard') }}</span>
        <div class="status-bar">
          <span :class="statusClass">{{ statusText }}</span>
          <span v-if="latency > 0" class="status-meta">{{ latency }}ms</span>
          <span v-if="tokenCount > 0" class="status-meta">{{ tokenCount }} tk</span>
        </div>
      </div>
      <div ref="responseRef" class="response-box">
        <span v-if="!response" class="response-placeholder">{{ t('playground.responsePh') }}</span>
        <template v-else>{{ response }}</template>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { userModelsAPI, apiKeysAPI } from '@/utils/api'

const models = ref([])
const apiKeys = ref([])
const selectedModel = ref('')
const selectedKey = ref('')
const streamMode = ref(true)
const sending = ref(false)
const systemPrompt = ref('You are a helpful assistant.')
const userMessage = ref('')
const response = ref('')
const statusCode = ref('ready')
const statusText = computed(() => {
  const map = { ready: t('playground.statusReady'), pending: t('playground.statusPending'), done: t('playground.statusDone'), fail: t('playground.statusFail') }
  return map[statusCode.value] || ''
})
const latency = ref(0)
const tokenCount = ref(0)
const responseRef = ref(null)

const statusClass = computed(() => {
  if (statusCode.value === 'done') return 'status-ok'
  if (statusCode.value === 'fail') return 'status-fail'
  if (statusCode.value === 'pending') return 'status-pending'
  return 'status-idle'
})

onMounted(async () => {
  try {
    const [modelData, keyData] = await Promise.all([userModelsAPI.list(), apiKeysAPI.list()])
    models.value = modelData.items || []
    apiKeys.value = keyData
    if (models.value.length > 0) {
      // 优先选 Sonnet 4.6（速度快3倍）
      const sonnet = models.value.find(m => m.name && m.name.includes('sonnet-4-6'))
      selectedModel.value = sonnet ? sonnet.name : models.value[0].name
    }
    if (apiKeys.value.length > 0) selectedKey.value = apiKeys.value[0].id
  } catch {}
})

async function handleSend() {
  if (!selectedModel.value || !selectedKey.value) return ElMessage.warning(t('playground.needModelKey'))
  if (!userMessage.value.trim()) return ElMessage.warning(t('playground.needMessage'))

  sending.value = true
  response.value = ''
  statusCode.value = 'pending'
  tokenCount.value = 0
  const start = performance.now()

  try {
    const messages = []
    if (systemPrompt.value.trim()) messages.push({ role: 'system', content: systemPrompt.value })
    messages.push({ role: 'user', content: userMessage.value })
    const token = localStorage.getItem('user_token')

    if (streamMode.value) {
      const res = await fetch(`/v1/user/playground/chat?api_key_id=${selectedKey.value}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ model: selectedModel.value, messages, stream: true, max_tokens: 2048 })
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }))
        throw new Error(err.error?.message || err.error || 'Request failed')
      }
      const reader = res.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
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
              const content = parsed.choices?.[0]?.delta?.content || parsed.choices?.[0]?.text || ''
              response.value += content
              if (parsed.usage) tokenCount.value = parsed.usage.total_tokens || 0
            } catch {}
          }
        }
      }
      statusCode.value = 'done'
      window.dispatchEvent(new Event('balance-changed'))
    } else {
      const res = await fetch(`/v1/user/playground/chat?api_key_id=${selectedKey.value}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ model: selectedModel.value, messages, max_tokens: 2048 })
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }))
        throw new Error(err.error?.message || err.error || 'Request failed')
      }
      const data = await res.json()
      response.value = data.choices?.[0]?.message?.content || JSON.stringify(data, null, 2)
      tokenCount.value = data.usage?.total_tokens || 0
      statusCode.value = 'done'
      window.dispatchEvent(new Event('balance-changed'))
    }
  } catch (err) {
    response.value = `Error: ${err.message}`
    statusCode.value = 'fail'
  } finally {
    sending.value = false
    latency.value = Math.round(performance.now() - start)
    if (responseRef.value) responseRef.value.scrollTop = responseRef.value.scrollHeight
  }
}

function handleClear() {
  response.value = ''
  userMessage.value = ''
  statusCode.value = 'ready'
  latency.value = 0
  tokenCount.value = 0
}
</script>

<style scoped>
.page { padding-bottom: 20px; }
.data-card {
  background: #fff;
  border-radius: 14px;
  padding: 16px;
  margin-bottom: 14px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.04);
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 14px;
}
.card-title { font-size: 15px; font-weight: 600; color: #1f2937; }
.form-row { display: flex; flex-direction: column; gap: 6px; margin-bottom: 12px; }
.form-row:last-child { margin-bottom: 0; }
.form-label { font-size: 13px; color: #4b5563; font-weight: 500; }
.toggle-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 0 4px;
}
.toggle-label { font-size: 14px; color: #4b5563; font-weight: 500; }

.btn-row { display: flex; gap: 8px; margin-top: 4px; }
.primary-btn {
  flex: 1;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  border: none;
  height: 44px;
  border-radius: 12px;
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(102,126,234,0.3);
}
.primary-btn:active { opacity: 0.9; }
.primary-btn:disabled { opacity: 0.5; }
.secondary-btn {
  background: #f3f4f6;
  color: #4b5563;
  border: none;
  height: 44px;
  padding: 0 18px;
  border-radius: 12px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
}
.secondary-btn:active { background: #e5e7eb; }

.status-bar { display: flex; gap: 8px; align-items: center; }
.status-bar > span:first-child {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 8px;
}
.status-ok { background: #d1fae5; color: #065f46; }
.status-fail { background: #fee2e2; color: #991b1b; }
.status-pending { background: #dbeafe; color: #1e3a8a; }
.status-idle { background: #f3f4f6; color: #6b7280; }
.status-meta { font-size: 11px; color: #9ca3af; }

.response-box {
  background: #1f2937;
  color: #f9fafb;
  border-radius: 10px;
  padding: 14px;
  min-height: 200px;
  max-height: 400px;
  overflow-y: auto;
  font-family: 'SF Mono', Menlo, monospace;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}
.response-placeholder { color: #6b7280; font-style: italic; }
</style>
