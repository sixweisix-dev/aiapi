<template>
  <div>
    <el-card shadow="hover" class="mb-6">
      <template #header>
        <span class="font-medium">{{ t('apiDocs.overview') }}</span>
      </template>
      <p class="text-gray-600 mb-2">
        {{ t('apiDocs.overviewDesc') }}
      </p>
      <el-alert type="info" :closable="false">
        <template #title>
          {{ t('apiDocs.baseUrlPrefix') }}: <code class="bg-blue-100 px-2 py-0.5 rounded">https://transitai.cloud/v1</code>
        </template>
      </el-alert>
    </el-card>

    <el-card shadow="hover" class="mb-6">
      <template #header>
        <span class="font-medium">{{ t('apiDocs.auth') }}</span>
      </template>
      <p class="text-gray-600 mb-3">
        {{ t('apiDocs.authDesc') }}
      </p>
      <el-input
        :model-value="'Authorization: Bearer sk-xxxxxxxxxxxxxxxx'"
        readonly
        class="mb-3"
      >
        <template #prepend>Header</template>
      </el-input>
      <p class="text-sm text-gray-500">{{ t('apiDocs.authTip') }}</p>
    </el-card>

    <!-- Chat Completions -->
    <el-card shadow="hover" class="mb-6">
      <template #header>
        <span class="font-medium">{{ t('apiDocs.chatCompletion') }}</span>
      </template>
      <el-descriptions :column="1" border class="mb-4">
        <el-descriptions-item :label="t('apiDocs.endpoint')">POST /v1/chat/completions</el-descriptions-item>
        <el-descriptions-item label="Content-Type">application/json</el-descriptions-item>
      </el-descriptions>

      <h4 class="font-medium mb-2">{{ t('apiDocs.requestExample') }}</h4>
      <pre class="bg-gray-900 text-gray-100 p-4 rounded-lg overflow-x-auto text-sm mb-4"><code>{{ chatExample }}</code></pre>

      <h4 class="font-medium mb-2">{{ t('apiDocs.streamRequest') }}</h4>
      <p class="text-gray-600 mb-2" v-html="t('apiDocs.streamDesc')"></p>
      <pre class="bg-gray-900 text-gray-100 p-4 rounded-lg overflow-x-auto text-sm mb-4"><code>{{ streamExample }}</code></pre>

      <h4 class="font-medium mb-2">{{ t('apiDocs.curlExample') }}</h4>
      <pre class="bg-gray-900 text-gray-100 p-4 rounded-lg overflow-x-auto text-sm"><code>{{ curlExample }}</code></pre>
    </el-card>

    <!-- Models -->
    <el-card shadow="hover" class="mb-6">
      <template #header>
        <span class="font-medium">{{ t('apiDocs.modelList') }}</span>
      </template>
      <el-descriptions :column="1" border class="mb-4">
        <el-descriptions-item :label="t('apiDocs.endpoint')">GET /v1/models</el-descriptions-item>
      </el-descriptions>
      <p class="text-gray-600 mb-2">{{ t('apiDocs.modelListDesc') }}</p>
      <pre class="bg-gray-900 text-gray-100 p-4 rounded-lg overflow-x-auto text-sm"><code>{{ modelsExample }}</code></pre>
    </el-card>

    <!-- SDK Examples -->
    <el-card shadow="hover" class="mb-6">
      <template #header>
        <span class="font-medium">{{ t('apiDocs.sdkExample') }}</span>
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
        <span class="font-medium">{{ t('apiDocs.billing') }}</span>
      </template>
      <ul class="list-disc list-inside text-gray-600 space-y-2">
        <li v-html="t('apiDocs.billLi1')"></li>
        <li>{{ t('apiDocs.billLi2') }}</li>
        <li>{{ t('apiDocs.billLi3') }}</li>
        <li>{{ t('apiDocs.billLi4') }}</li>
        <li>{{ t('apiDocs.billLi5') }}</li>
      </ul>
    </el-card>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
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
