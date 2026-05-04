<template>
  <div>
    <el-card shadow="hover" class="mb-6">
      <template #header>
        <span class="font-medium">API 概述</span>
      </template>
      <p class="text-gray-600 mb-2">
        本平台提供 Claude 系列模型的 API 中转服务，接口格式兼容 OpenAI SDK。您可以使用 OpenAI SDK 或直接 HTTP 请求调用 Claude 模型，无需修改现有代码。
      </p>
      <el-alert type="info" :closable="false">
        <template #title>
          基础 URL: <code class="bg-blue-100 px-2 py-0.5 rounded">https://transitai.cloud/v1</code>
        </template>
      </el-alert>
    </el-card>

    <el-card shadow="hover" class="mb-6">
      <template #header>
        <span class="font-medium">鉴权方式</span>
      </template>
      <p class="text-gray-600 mb-3">
        所有 API 请求需要在 HTTP Header 中携带 API Key:
      </p>
      <el-input
        :model-value="'Authorization: Bearer sk-xxxxxxxxxxxxxxxx'"
        readonly
        class="mb-3"
      >
        <template #prepend>Header</template>
      </el-input>
      <p class="text-sm text-gray-500">你可以在"API Key 管理"页面创建和管理你的 API Key。</p>
    </el-card>

    <!-- Chat Completions -->
    <el-card shadow="hover" class="mb-6">
      <template #header>
        <span class="font-medium">聊天补全</span>
      </template>
      <el-descriptions :column="1" border class="mb-4">
        <el-descriptions-item label="端点">POST /v1/chat/completions</el-descriptions-item>
        <el-descriptions-item label="Content-Type">application/json</el-descriptions-item>
      </el-descriptions>

      <h4 class="font-medium mb-2">请求示例:</h4>
      <pre class="bg-gray-900 text-gray-100 p-4 rounded-lg overflow-x-auto text-sm mb-4"><code>{{ chatExample }}</code></pre>

      <h4 class="font-medium mb-2">流式请求:</h4>
      <p class="text-gray-600 mb-2">添加 <code>stream: true</code> 即可启用 SSE 流式响应。</p>
      <pre class="bg-gray-900 text-gray-100 p-4 rounded-lg overflow-x-auto text-sm mb-4"><code>{{ streamExample }}</code></pre>

      <h4 class="font-medium mb-2">CURL 示例:</h4>
      <pre class="bg-gray-900 text-gray-100 p-4 rounded-lg overflow-x-auto text-sm"><code>{{ curlExample }}</code></pre>
    </el-card>

    <!-- Models -->
    <el-card shadow="hover" class="mb-6">
      <template #header>
        <span class="font-medium">模型列表</span>
      </template>
      <el-descriptions :column="1" border class="mb-4">
        <el-descriptions-item label="端点">GET /v1/models</el-descriptions-item>
      </el-descriptions>
      <p class="text-gray-600 mb-2">获取所有可用模型的列表，返回格式兼容 OpenAI。</p>
      <pre class="bg-gray-900 text-gray-100 p-4 rounded-lg overflow-x-auto text-sm"><code>{{ modelsExample }}</code></pre>
    </el-card>

    <!-- SDK Examples -->
    <el-card shadow="hover" class="mb-6">
      <template #header>
        <span class="font-medium">SDK 使用示例</span>
      </template>

      <el-tabs v-model="sdkTab">
        <el-tab-pane label="Python (OpenAI SDK)" name="python">
          <pre class="bg-gray-900 text-gray-100 p-4 rounded-lg overflow-x-auto text-sm"><code>{{ pythonExample }}</code></pre>
        </el-tab-pane>
        <el-tab-pane label="Node.js" name="node">
          <pre class="bg-gray-900 text-gray-100 p-4 rounded-lg overflow-x-auto text-sm"><code>{{ nodeExample }}</code></pre>
        </el-tab-pane>
        <el-tab-pane label="cURL" name="curl">
          <pre class="bg-gray-900 text-gray-100 p-4 rounded-lg overflow-x-auto text-sm"><code>{{ curlExample }}</code></pre>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <!-- Pricing Note -->
    <el-card shadow="hover">
      <template #header>
        <span class="font-medium">计费说明</span>
      </template>
      <ul class="list-disc list-inside text-gray-600 space-y-2">
        <li>所有价格以 <strong>USD</strong> 计价，按每 1,000 tokens 计算</li>
        <li>实际扣费 = (输入tokens/1000 × 输入单价 + 输出tokens/1000 × 输出单价) × 倍率</li>
        <li>具体模型的单价和倍率请在"模型与价格"页面查看</li>
        <li>余额不足时请求将返回 402 Payment Required</li>
        <li>消费记录可在"消费明细"页面实时查看并导出 CSV</li>
      </ul>
    </el-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const sdkTab = ref('python')

const models = [
  { id: 'claude-opus-4-7', name: 'Claude Opus 4.7', provider: 'Anthropic', input: '¥0.0216/1K', output: '¥0.108/1K' },
  { id: 'claude-opus-4-6', name: 'Claude Opus 4.6', provider: 'Anthropic', input: '¥0.0216/1K', output: '¥0.108/1K' },
  { id: 'claude-sonnet-4-6', name: 'Claude Sonnet 4.6', provider: 'Anthropic', input: '¥0.01296/1K', output: '¥0.0648/1K' },
  { id: 'claude-sonnet-4-5', name: 'Claude Sonnet 4.5', provider: 'Anthropic', input: '¥0.01296/1K', output: '¥0.0648/1K' },
  { id: 'claude-haiku-4-5-20251001', name: 'Claude Haiku 4.5', provider: 'Anthropic', input: '¥0.01296/1K', output: '¥0.0648/1K' },
]

const chatExample = JSON.stringify({
  model: 'claude-opus-4-7',
  messages: [
    { role: 'system', content: 'You are a helpful assistant.' },
    { role: 'user', content: 'Hello!' },
  ],
  max_tokens: 1024,
  temperature: 0.7,
}, null, 2)

const streamExample = JSON.stringify({
  model: 'claude-opus-4-7',
  messages: [
    { role: 'user', content: 'Hello!' },
  ],
  stream: true,
}, null, 2)

const modelsExample = `curl https://transitai.cloud/v1/models \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx"

{
  "object": "list",
  "data": [
    {
      "id": "claude-opus-4-7",
      "object": "model",
      "created": 1710000000,
      "owned_by": "anthropic"
    },
    {
      "id": "claude-opus-4-6",
      "object": "model",
      "created": 1710000000,
      "owned_by": "anthropic"
    },
    {
      "id": "claude-sonnet-4-5",
      "object": "model",
      "created": 1710000000,
      "owned_by": "anthropic"
    },
    {
      "id": "claude-sonnet-4-6",
      "object": "model",
      "created": 1748000000,
      "owned_by": "anthropic"
    },
    {
      "id": "claude-haiku-4-5-20251001",
      "object": "model",
      "created": 1710000000,
      "owned_by": "anthropic"
    }
  ]
}`

const curlExample = `curl https://transitai.cloud/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "claude-opus-4-7",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'`

const pythonExample = `from openai import OpenAI

client = OpenAI(
    base_url="https://transitai.cloud/v1",
    api_key="sk-xxxxxxxxxxxxxxxx"
)

response = client.chat.completions.create(
    model="claude-opus-4-7",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "Hello!"}
    ]
)

print(response.choices[0].message.content)`

const nodeExample = `import OpenAI from 'openai';

const client = new OpenAI({
  baseURL: 'https://transitai.cloud/v1',
  apiKey: 'sk-xxxxxxxxxxxxxxxx',
});

const response = await client.chat.completions.create({
  model: 'claude-opus-4-7',
  messages: [
    { role: 'system', content: 'You are a helpful assistant.' },
    { role: 'user', content: 'Hello!' },
  ],
});

console.log(response.choices[0].message.content);`
</script>

<style scoped>
pre {
  max-height: 400px;
  overflow: auto;
}
</style>
