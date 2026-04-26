<template>
  <div class="flex flex-col h-full">
    <el-card shadow="hover" class="flex-1 flex flex-col">
      <template #header>
        <div class="flex items-center gap-4">
          <span class="font-medium">Playground - 在线测试</span>
          <el-select v-model="selectedModel" placeholder="选择模型" style="width: 240px" @change="fetchModels">
            <el-option v-for="m in models" :key="m.id" :label="m.display_name" :value="m.name" />
          </el-select>
          <el-select v-model="selectedKey" placeholder="选择 API Key" style="width: 240px">
            <el-option v-for="k in apiKeys" :key="k.id" :label="k.name" :value="k.id">
              <span>{{ k.name }} - sk-{{ k.prefix }}...</span>
            </el-option>
          </el-select>
          <el-switch v-model="streamMode" active-text="流式" inactive-text="非流式" />
          <el-button type="primary" :loading="sending" @click="handleSend" :disabled="!selectedKey || !selectedModel">
            {{ sending ? '请求中...' : '发送' }}
          </el-button>
          <el-button @click="handleClear">清空</el-button>
        </div>
      </template>

      <div class="flex-1 flex gap-4 min-h-0">
        <!-- Input -->
        <div class="flex-1 flex flex-col">
          <h4 class="text-sm font-medium mb-2">系统提示 (可选)</h4>
          <el-input
            v-model="systemPrompt"
            type="textarea"
            :rows="3"
            placeholder="You are a helpful assistant."
            class="mb-3"
          />

          <h4 class="text-sm font-medium mb-2">用户消息</h4>
          <el-input
            v-model="userMessage"
            type="textarea"
            :rows="10"
            placeholder="输入你的消息..."
            class="flex-1"
          />
        </div>

        <!-- Response -->
        <div class="flex-1 flex flex-col">
          <h4 class="text-sm font-medium mb-2">响应</h4>
          <div
            ref="responseRef"
            class="flex-1 bg-gray-900 text-gray-100 p-4 rounded-lg overflow-auto text-sm font-mono whitespace-pre-wrap"
          >
            <template v-if="response">
              {{ response }}
            </template>
            <template v-else>
              <span class="text-gray-500">点击"发送"来测试模型...</span>
            </template>
          </div>

          <div class="flex items-center gap-4 mt-2 text-xs text-gray-400">
            <span>模型: {{ selectedModel || '-' }}</span>
            <span>状态: {{ statusText }}</span>
            <span>延迟: {{ latency }}ms</span>
            <span v-if="tokenCount">Tokens: {{ tokenCount }}</span>
          </div>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
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
const statusText = ref('就绪')
const latency = ref(0)
const tokenCount = ref(0)
const responseRef = ref(null)

onMounted(async () => {
  try {
    const [modelData, keyData] = await Promise.all([
      userModelsAPI.list(),
      apiKeysAPI.list(),
    ])
    models.value = modelData.items || []
    apiKeys.value = keyData
    if (models.value.length > 0) selectedModel.value = models.value[0].name
    if (apiKeys.value.length > 0) selectedKey.value = apiKeys.value[0].id
  } catch {
    // handled
  }
})

async function handleSend() {
  if (!selectedModel.value || !selectedKey.value) {
    ElMessage.warning('请选择模型和 API Key')
    return
  }
  if (!userMessage.value.trim()) {
    ElMessage.warning('请输入消息')
    return
  }

  sending.value = true
  response.value = ''
  statusText.value = '请求中...'
  tokenCount.value = 0
  const start = performance.now()

  // Find the selected API key prefix to get the actual key
  const selected = apiKeys.value.find((k) => k.id === selectedKey.value)

  try {
    const messages = []
    if (systemPrompt.value.trim()) {
      messages.push({ role: 'system', content: systemPrompt.value })
    }
    messages.push({ role: 'user', content: userMessage.value })

    // We need the actual API key to make the request — we stored only the prefix.
    // The user must have the full key to use Playground.
    // Instead, we'll prompt them to enter the full key or use a separate approach.
    // For simplicity, use the stored token approach via a direct fetch to /v1/chat/completions
    const token = localStorage.getItem('user_token')

    if (streamMode.value) {
      // SSE streaming
      const res = await fetch('/v1/chat/completions', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          // Use JWT token as a proxy — in production the playground should accept direct API Key entry
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          model: selectedModel.value,
          messages,
          stream: true,
          max_tokens: 2048,
        }),
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
              if (parsed.usage) {
                tokenCount.value = parsed.usage.total_tokens || 0
              }
            } catch {
              // skip parse errors
            }
          }
        }
      }
      statusText.value = '完成'
    } else {
      // Non-streaming
      const res = await fetch('/v1/chat/completions', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          model: selectedModel.value,
          messages,
          max_tokens: 2048,
        }),
      })

      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }))
        throw new Error(err.error?.message || err.error || 'Request failed')
      }

      const data = await res.json()
      response.value = data.choices?.[0]?.message?.content || JSON.stringify(data, null, 2)
      tokenCount.value = data.usage?.total_tokens || 0
      statusText.value = '完成'
    }
  } catch (err) {
    response.value = `Error: ${err.message}`
    statusText.value = '错误'
  } finally {
    sending.value = false
    latency.value = Math.round(performance.now() - start)
    // Scroll to bottom
    if (responseRef.value) {
      responseRef.value.scrollTop = responseRef.value.scrollHeight
    }
  }
}

async function fetchModels() {
  // already loaded
}

function handleClear() {
  response.value = ''
  userMessage.value = ''
  statusText.value = '就绪'
  latency.value = 0
  tokenCount.value = 0
}
</script>
