<template>
  <div class="api-docs">

    <!-- 快速开始 -->
    <el-card shadow="hover" class="doc-card">
      <template #header><span class="card-title">{{ t('apiDocs.quickstart') }}</span></template>
      <div class="steps">
        <div class="step">
          <div class="step-num">1</div>
          <div class="step-body">
            <div class="step-title">{{ t('apiDocs.quickstep1') }}</div>
            <div class="step-desc">{{ t('apiDocs.quickstep1desc') }}</div>
          </div>
        </div>
        <div class="step">
          <div class="step-num">2</div>
          <div class="step-body">
            <div class="step-title">{{ t('apiDocs.quickstep2') }}</div>
            <div class="step-desc">{{ t('apiDocs.quickstep2desc') }}</div>
          </div>
        </div>
        <div class="step">
          <div class="step-num">3</div>
          <div class="step-body">
            <div class="step-title">{{ t('apiDocs.quickstep3') }}</div>
            <div class="step-desc">{{ t('apiDocs.quickstep3desc') }}</div>
          </div>
        </div>
      </div>
    </el-card>

    <!-- 接口概览 -->
    <el-card shadow="hover" class="doc-card">
      <template #header><span class="card-title">{{ t('apiDocs.overview') }}</span></template>
      <p class="desc-text">{{ t('apiDocs.overviewDesc') }}</p>
      <div class="url-block">
        <div class="url-label">{{ t('apiDocs.baseUrlOpenAI') }}</div>
        <code class="url-code">https://transitai.cloud/v1</code>
      </div>
      <div class="url-block">
        <div class="url-label">{{ t('apiDocs.baseUrlAnthropic') }}</div>
        <code class="url-code">https://transitai.cloud</code>
      </div>
    </el-card>



    <!-- 两种格式对比 -->
    <el-card shadow="hover" class="doc-card">
      <template #header><span class="card-title">{{ t('apiDocs.compareTitle') }}</span></template>
      <p class="desc-text">{{ t('apiDocs.compareDesc') }}</p>

      <el-table :data="compareRows" border class="compare-table">
        <el-table-column :label="t('apiDocs.compareTableFeature')" prop="feature" min-width="140" />
        <el-table-column :label="t('apiDocs.compareTableOpenAI')" prop="openai" min-width="180">
          <template #default="{ row }"><span v-html="row.openai" /></template>
        </el-table-column>
        <el-table-column :label="t('apiDocs.compareTableAnthropic')" prop="anthropic" min-width="180">
          <template #default="{ row }"><span v-html="row.anthropic" /></template>
        </el-table-column>
      </el-table>

      <div class="choose-section">
        <h4 class="choose-title">{{ t('apiDocs.compareWhich') }}</h4>
        <div class="choose-block">
          <strong>{{ t('apiDocs.compareUseOpenAI') }}</strong>
          <p>{{ t('apiDocs.compareUseOpenAIDesc') }}</p>
        </div>
        <div class="choose-block">
          <strong>{{ t('apiDocs.compareUseAnthropic') }}</strong>
          <p>{{ t('apiDocs.compareUseAnthropicDesc') }}</p>
        </div>
        <p class="tip-text">{{ t('apiDocs.compareTip') }}</p>
      </div>
    </el-card>

    <!-- 模型分组与版本 (移到两种格式对比后) -->
    <el-card v-if="channelGroups.length > 0" shadow="hover" class="doc-card">
      <template #header><span class="card-title">{{ t('apiDocs.groupsTitle') }}</span></template>
      <p class="desc-text">{{ t('apiDocs.groupsDesc') }}</p>
      <el-table :data="channelGroups" border class="compare-table">
        <el-table-column :label="t('apiDocs.groupColName')" min-width="140">
          <template #default="{ row }"><strong>{{ groupDisplayName(row) }}</strong></template>
        </el-table-column>
        <el-table-column :label="t('apiDocs.groupColSuffix')" min-width="120">
          <template #default="{ row }">
            <code v-if="row.is_default" class="inline-code">{{ t('apiDocs.groupSuffixNone') }}</code>
            <code v-else class="inline-code">-{{ row.slug }}</code>
          </template>
        </el-table-column>
        <el-table-column :label="t('apiDocs.groupColMult')" width="100" align="right">
          <template #default="{ row }"><span class="mult-badge">{{ Number(row.multiplier).toFixed(2) }}×</span></template>
        </el-table-column>
        <el-table-column :label="t('apiDocs.groupColDesc')" min-width="280">
          <template #default="{ row }"><span>{{ groupDisplayDesc(row) }}</span></template>
        </el-table-column>
      </el-table>
      <p class="desc-text" style="margin-top: 12px;">{{ t('apiDocs.groupsExample') }}</p>
      <pre class="code-block">claude-haiku-4-5-20251001         {{ t('apiDocs.groupExampleEcon') }}
claude-haiku-4-5-20251001-pro     {{ t('apiDocs.groupExamplePro') }}</pre>

      <!-- 完整格式示例 (合并 OpenAI/Anthropic 两种格式 + 多语言) -->
      <p class="desc-text" style="margin-top: 20px; font-weight: 600;">{{ t('apiDocs.groupCodeTitle') }}</p>
      <p class="desc-text" style="font-size:12px;color:#6b7280;margin-bottom:8px;">{{ t('apiDocs.groupCodeDesc') }}</p>
      <el-tabs v-model="formatTab" type="border-card">
        <el-tab-pane label="🔄 OpenAI 兼容格式" name="openai">
          <div class="endpoint-row" style="margin-bottom:8px;">
            <span class="method-badge">POST</span>
            <code>https://transitai.cloud/v1/chat/completions</code>
          </div>
          <p class="desc-text">{{ t('apiDocs.openaiFormatDesc') }}</p>
          <div class="section-label">Python</div>
          <pre class="code-block">{{ pythonOpenAI }}</pre>
          <div class="section-label">Node.js</div>
          <pre class="code-block">{{ nodeOpenAI }}</pre>
          <div class="section-label">cURL</div>
          <pre class="code-block">{{ curlOpenAI }}</pre>
          <div class="section-label">{{ t('apiDocs.streamRequest') }}</div>
          <p class="desc-text" v-html="t('apiDocs.streamDesc')"></p>
          <pre class="code-block">{{ streamExample }}</pre>
        </el-tab-pane>
        <el-tab-pane label="🧠 Anthropic 原生格式" name="anthropic">
          <div class="endpoint-row" style="margin-bottom:8px;">
            <span class="method-badge">POST</span>
            <code>https://transitai.cloud/v1/messages</code>
          </div>
          <p class="desc-text">{{ t('apiDocs.anthropicFormatDesc') }}</p>
          <el-alert type="warning" :closable="false" class="mb-4">{{ t('apiDocs.anthropicNote') }}</el-alert>
          <div class="section-label">Python (Anthropic SDK)</div>
          <pre class="code-block">{{ pythonAnthropic }}</pre>
          <div class="section-label">Node.js (Anthropic SDK)</div>
          <pre class="code-block">{{ nodeAnthropic }}</pre>
          <div class="section-label">cURL</div>
          <pre class="code-block">{{ curlAnthropic }}</pre>
        </el-tab-pane>
      </el-tabs>
      <p class="desc-text" style="margin-top:10px;font-size:12px;color:#6b7280;">
        💡 {{ t('apiDocs.groupSwitchTip') }}
      </p>
    </el-card>


    <!-- 鉴权 -->
    <el-card shadow="hover" class="doc-card">
      <template #header><span class="card-title">{{ t('apiDocs.auth') }}</span></template>
      <p class="desc-text">{{ t('apiDocs.authDesc') }}</p>
      <pre class="code-block">Authorization: Bearer YOUR_API_KEY</pre>
      <p class="desc-text mt-2">{{ t('apiDocs.authDescAnthropic') }}</p>
      <pre class="code-block">x-api-key: YOUR_API_KEY
anthropic-version: 2023-06-01</pre>
      <p class="tip-text">{{ t('apiDocs.authTip') }}</p>
    </el-card>

    <!-- 查询模型 -->
    <el-card shadow="hover" class="doc-card">
      <template #header><span class="card-title">{{ t('apiDocs.modelList') }}</span></template>
      <div class="endpoint-row">
        <span class="method-badge get">GET</span>
        <code>https://transitai.cloud/v1/models</code>
      </div>
      <p class="desc-text">{{ t('apiDocs.modelListDesc') }}</p>
      <pre class="code-block">{{ modelsExample }}</pre>
    </el-card>

    <!-- 错误码 -->
    <el-card shadow="hover" class="doc-card">
      <template #header><span class="card-title">{{ t('apiDocs.errorCodes') }}</span></template>
      <p class="desc-text">{{ t('apiDocs.errorsDesc') }}</p>
      <pre class="code-block">{
  "error": {
    "message": "insufficient balance",
    "type": "billing_error"
  }
}</pre>
      <el-table :data="errorCodes" border class="mt-4">
        <el-table-column label="HTTP Code" width="120" prop="code" />
        <el-table-column :label="t('apiDocs.endpoint')" prop="desc" />
      </el-table>
    </el-card>


    <!-- 工具集成教程 -->
    <el-card shadow="hover" class="doc-card">
      <template #header><span class="card-title">{{ t('apiDocs.tutorials') }}</span></template>
      <p class="desc-text">{{ t('apiDocs.tutorialsDesc') }}</p>

      <el-tabs v-model="tutTab">

        <!-- Claude Code 完整保姆级 -->
        <el-tab-pane label="Claude Code" name="cc">
          <h4 class="tut-title">{{ t('apiDocs.tutCC') }}</h4>
          <p class="desc-text">{{ t('apiDocs.tutCCdesc') }}</p>

          <!-- Step 1: Node.js -->
          <div class="tut-step">{{ t('apiDocs.ccFull1Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.ccFull1Desc') }}</p>
          <p class="desc-text">📥 <a href="https://nodejs.org" target="_blank" class="link">https://nodejs.org</a></p>
          <p class="desc-text">{{ t('apiDocs.ccFull1Verify') }}</p>
          <pre class="code-block">node -v
# 应输出 v18.x.x 或更高</pre>

          <!-- Step 2: Install CC -->
          <div class="tut-step">{{ t('apiDocs.ccFull2Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.ccFull2Desc') }}</p>
          <pre class="code-block">npm install -g @anthropic-ai/claude-code</pre>
          <p class="desc-text">{{ t('apiDocs.ccFull2Verify') }}</p>
          <pre class="code-block">cc --version</pre>

          <!-- Step 3: Config -->
          <div class="tut-step">{{ t('apiDocs.ccFull3Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.ccFull3Desc') }}</p>

          <div class="section-label">🍎 {{ t('apiDocs.ccFull3Mac') }}</div>
          <pre class="code-block">echo 'export ANTHROPIC_API_KEY="YOUR_API_KEY"' >> ~/.zshrc
echo 'export ANTHROPIC_BASE_URL="https://transitai.cloud"' >> ~/.zshrc
source ~/.zshrc</pre>

          <div class="section-label">🪟 {{ t('apiDocs.ccFull3WinPS') }}</div>
          <pre class="code-block">$env:ANTHROPIC_API_KEY = "YOUR_API_KEY"
$env:ANTHROPIC_BASE_URL = "https://transitai.cloud"</pre>

          <div class="section-label">🪟 {{ t('apiDocs.ccFull3WinPerm') }}</div>
          <pre class="code-block">[Environment]::SetEnvironmentVariable("ANTHROPIC_API_KEY", "YOUR_API_KEY", "User")
[Environment]::SetEnvironmentVariable("ANTHROPIC_BASE_URL", "https://transitai.cloud", "User")
# 重启 PowerShell 后生效</pre>

          <!-- Step 4: Start -->
          <div class="tut-step">{{ t('apiDocs.ccFull4Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.ccFull4Desc') }}</p>
          <pre class="code-block">cd ~/your-project
cc</pre>

          <!-- Step 5: Usage -->
          <div class="tut-step">{{ t('apiDocs.ccFull5Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.ccFull5Desc') }}</p>
          <ul class="config-list">
            <li><code>/help</code> — {{ t('apiDocs.ccFull5Cmd1').split('—')[1] }}</li>
            <li><code>/clear</code> — {{ t('apiDocs.ccFull5Cmd2').split('—')[1] }}</li>
            <li><code>/model claude-opus-4-7</code> — {{ t('apiDocs.ccFull5Cmd3').split('—')[1] }}</li>
            <li><code>/exit</code> — {{ t('apiDocs.ccFull5Cmd4').split('—')[1] }}</li>
          </ul>
          <p class="tip-text">💡 {{ t('apiDocs.ccFull5Tip') }}</p>

          <!-- Step 6: Verify billing -->
          <div class="tut-step">{{ t('apiDocs.ccFull6Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.ccFull6Desc') }}</p>

          <p class="success-text">✅ {{ t('apiDocs.tutCCStep4') }}</p>
        </el-tab-pane>

        <!-- CCSwitch 完整保姆级 -->
        <el-tab-pane label="CCSwitch" name="switch">
          <h4 class="tut-title">{{ t('apiDocs.tutSwitch') }}</h4>
          <p class="desc-text">{{ t('apiDocs.tutSwitchDesc') }}</p>

          <div class="tut-step">{{ t('apiDocs.swFull1Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.swFull1Desc') }}</p>

          <div class="tut-step">{{ t('apiDocs.swFull2Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.swFull2Desc') }}</p>
          <pre class="code-block">npm install -g ccswitch</pre>
          <p class="desc-text">{{ t('apiDocs.swFull2Verify') }}</p>
          <pre class="code-block">ccswitch --version</pre>

          <div class="tut-step">{{ t('apiDocs.swFull3Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.swFull3Desc') }}</p>
          <pre class="code-block">ccswitch add transitai \
  --base-url https://transitai.cloud \
  --api-key YOUR_API_KEY</pre>

          <div class="tut-step">{{ t('apiDocs.swFull4Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.swFull4Desc') }}</p>
          <pre class="code-block">ccswitch add official \
  --base-url https://api.anthropic.com \
  --api-key sk-ant-xxx</pre>

          <div class="tut-step">{{ t('apiDocs.swFull5Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.swFull5Desc') }}</p>
          <pre class="code-block">ccswitch use transitai
cc  # 走 TransitAI

ccswitch use official
cc  # 走官方</pre>

          <div class="tut-step">{{ t('apiDocs.swFull6Title') }}</div>
          <ul class="config-list">
            <li><code>ccswitch list</code> — {{ t('apiDocs.swFull6Cmd1').split('—')[1] }}</li>
            <li><code>ccswitch current</code> — {{ t('apiDocs.swFull6Cmd2').split('—')[1] }}</li>
            <li><code>ccswitch remove transitai</code> — {{ t('apiDocs.swFull6Cmd3').split('—')[1] }}</li>
          </ul>

          <p class="tip-text">💡 {{ t('apiDocs.tutSwitchHint') }}</p>
        </el-tab-pane>

        <!-- Cline 完整保姆级 -->
        <el-tab-pane label="VS Code Cline" name="cline">
          <h4 class="tut-title">{{ t('apiDocs.tutCline') }}</h4>
          <p class="desc-text">{{ t('apiDocs.tutClineDesc') }}</p>

          <div class="tut-step">{{ t('apiDocs.clFull1Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.clFull1Desc') }}</p>
          <p class="desc-text">📥 <a href="https://code.visualstudio.com" target="_blank" class="link">https://code.visualstudio.com</a></p>

          <div class="tut-step">{{ t('apiDocs.clFull2Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.clFull2Desc') }}</p>
          <p class="tip-text">{{ t('apiDocs.clFull2Note') }}</p>

          <div class="tut-step">{{ t('apiDocs.clFull3Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.clFull3Desc') }}</p>

          <div class="tut-step">{{ t('apiDocs.clFull4Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.clFull4Desc') }}</p>

          <div class="tut-step">{{ t('apiDocs.clFull5Title') }}</div>
          <ul class="config-list">
            <li>{{ t('apiDocs.clFull5Cfg1') }}</li>
            <li>{{ t('apiDocs.clFull5Cfg2') }}</li>
            <li>{{ t('apiDocs.clFull5Cfg3') }}</li>
            <li><code>{{ t('apiDocs.clFull5Cfg4').split('：')[1] || t('apiDocs.clFull5Cfg4').split(':')[1] }}</code></li>
            <li>{{ t('apiDocs.clFull5Cfg5') }}</li>
          </ul>

          <div class="tut-step">{{ t('apiDocs.clFull6Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.clFull6Desc') }}</p>
          <p class="tip-text">{{ t('apiDocs.clFull6Note') }}</p>
        </el-tab-pane>

        <!-- Cursor 完整保姆级 -->
        <el-tab-pane label="Cursor" name="cursor">
          <h4 class="tut-title">{{ t('apiDocs.tutCursor') }}</h4>
          <p class="desc-text">{{ t('apiDocs.tutCursorDesc') }}</p>

          <div class="tut-step">{{ t('apiDocs.cuFull1Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.cuFull1Desc') }}</p>
          <p class="desc-text">📥 <a href="https://cursor.com" target="_blank" class="link">https://cursor.com</a></p>

          <div class="tut-step">{{ t('apiDocs.cuFull2Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.cuFull2Desc') }}</p>

          <div class="tut-step">{{ t('apiDocs.cuFull3Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.cuFull3Desc') }}</p>

          <div class="tut-step">{{ t('apiDocs.cuFull4Title') }}</div>
          <ul class="config-list">
            <li>{{ t('apiDocs.cuFull4Cfg1') }}</li>
            <li>{{ t('apiDocs.cuFull4Cfg2') }}</li>
            <li><code>{{ t('apiDocs.cuFull4Cfg3').split('：')[1] || t('apiDocs.cuFull4Cfg3').split(':')[1] }}</code></li>
          </ul>

          <div class="tut-step">{{ t('apiDocs.cuFull5Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.cuFull5Desc') }}</p>

          <div class="tut-step">{{ t('apiDocs.cuFull6Title') }}</div>
          <p class="desc-text">{{ t('apiDocs.cuFull6Desc') }}</p>
          <p class="tip-text">{{ t('apiDocs.cuFull6Note') }}</p>
        </el-tab-pane>

      </el-tabs>

      <el-alert type="info" :closable="false" class="mt-4">{{ t('apiDocs.tutCommonNote') }}</el-alert>
    </el-card>

    <!-- MCP -->
    <el-card shadow="hover" class="doc-card">
      <template #header><span class="card-title">{{ t('apiDocs.mcp') }}</span></template>
      <p class="desc-text">{{ t('apiDocs.mcpDesc') }}</p>
      <pre class="code-block">{{ mcpExample }}</pre>
    </el-card>

    <!-- 计费 -->
    <el-card shadow="hover" class="doc-card">
      <template #header><span class="card-title">{{ t('apiDocs.billing') }}</span></template>
      <ul class="bill-list">
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
import { ref, onMounted, computed } from 'vue'
const { t, locale } = useI18n()
const tutTab = ref('cc')

const channelGroups = ref([])
const formatTab = ref('openai')


const codeAnthropicPro = `# Anthropic 原生格式 · 官方直连 (2.0× 倍率, 完整 cache 支持)
import anthropic

client = anthropic.Anthropic(
    api_key="YOUR_API_KEY",
    base_url="https://transitai.cloud"
)

msg = client.messages.create(
    model="claude-haiku-4-5-20251001-pro",   # -pro 后缀 → 官方直连
    max_tokens=1024,
    messages=[
        {"role": "user", "content": "Hello"}
    ]
)
print(msg.content[0].text)`

async function loadChannelGroups() {
  try {
    const res = await fetch('/v1/public/channel-groups')
    const data = await res.json()
    channelGroups.value = data.items || []
  } catch (e) { console.error('loadChannelGroups failed:', e) }
}

function groupDisplayName(row) {
  const isEn = locale && locale.value && locale.value.startsWith('en')
  return isEn && row.name_en ? row.name_en : row.name
}

function groupDisplayDesc(row) {
  const isEn = locale && locale.value && locale.value.startsWith('en')
  return isEn && row.description_en ? row.description_en : row.description
}

const pythonOpenAI = `from openai import OpenAI

client = OpenAI(
    api_key="YOUR_API_KEY",
    base_url="https://transitai.cloud/v1"
)

response = client.chat.completions.create(
    model="claude-sonnet-4-5",
    messages=[
        {"role": "user", "content": "Hello, Claude!"}
    ],
    max_tokens=1024
)
print(response.choices[0].message.content)`

const nodeOpenAI = `import OpenAI from 'openai';

const client = new OpenAI({
  apiKey: 'YOUR_API_KEY',
  baseURL: 'https://transitai.cloud/v1',
});

const response = await client.chat.completions.create({
  model: 'claude-sonnet-4-5',
  messages: [{ role: 'user', content: 'Hello, Claude!' }],
  max_tokens: 1024,
});
console.log(response.choices[0].message.content);`

const streamExample = `from openai import OpenAI

client = OpenAI(
    api_key="YOUR_API_KEY",
    base_url="https://transitai.cloud/v1"
)

with client.chat.completions.stream(
    model="claude-sonnet-4-5",
    messages=[{"role": "user", "content": "写一首诗"}],
    max_tokens=1024,
) as stream:
    for text in stream.text_stream:
        print(text, end="", flush=True)`

const curlOpenAI = `curl https://transitai.cloud/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 1024
  }'`

const pythonAnthropic = `import anthropic

client = anthropic.Anthropic(
    api_key="YOUR_API_KEY",
    base_url="https://transitai.cloud"  # 不带 /v1
)

message = client.messages.create(
    model="claude-sonnet-4-5",
    max_tokens=1024,
    system="You are a helpful assistant.",
    messages=[
        {"role": "user", "content": "Hello, Claude!"}
    ]
)
print(message.content[0].text)`

const nodeAnthropic = `import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic({
  apiKey: "YOUR_API_KEY",
  baseURL: "https://transitai.cloud"  // 不带 /v1
});

const message = await client.messages.create({
  model: "claude-sonnet-4-5",
  max_tokens: 1024,
  system: "You are a helpful assistant.",
  messages: [
    { role: "user", content: "Hello, Claude!" }
  ]
});

console.log(message.content[0].text);`

const curlAnthropic = `curl https://transitai.cloud/v1/messages \
  -H "x-api-key: YOUR_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello!"}]
  }'`

const modelsExample = `curl https://transitai.cloud/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"`

const mcpExample = `# ~/.claude_desktop_config.json (MacOS)
# ~/AppData/Roaming/Claude/claude_desktop_config.json (Windows)

{
  "mcpServers": {
    "transitai": {
      "command": "npx",
      "args": ["-y", "@anthropic-ai/mcp-server-claude"],
      "env": {
        "ANTHROPIC_API_KEY": "YOUR_API_KEY",
        "ANTHROPIC_BASE_URL": "https://transitai.cloud"
      }
    }
  }
}`

const compareRows = computed(() => {
  const Y = t('apiDocs.compareYes')
  const N = t('apiDocs.compareNo')
  const L = t('apiDocs.compareLimited')
  return [
    { feature: t('apiDocs.comparePath'), openai: '<code>/v1/chat/completions</code>', anthropic: '<code>/v1/messages</code>' },
    { feature: t('apiDocs.compareAuth'), openai: 'Authorization: Bearer', anthropic: 'x-api-key 或 Bearer' },
    { feature: t('apiDocs.compareStream'), openai: Y, anthropic: Y },
    { feature: t('apiDocs.compareCache'), openai: N, anthropic: Y },
    { feature: t('apiDocs.compareThinking'), openai: N, anthropic: Y },
    { feature: t('apiDocs.compareTool'), openai: L, anthropic: Y },
    { feature: t('apiDocs.compareVision'), openai: L, anthropic: Y },
    { feature: t('apiDocs.compareCompat'), openai: t('apiDocs.compareCompatOpenAI'), anthropic: t('apiDocs.compareCompatAnthropic') },
  ]
})

const errorCodes = computed(() => [
  { code: '401', desc: t('apiDocs.errorTable401') },
  { code: '402', desc: t('apiDocs.errorTable402') },
  { code: '429', desc: t('apiDocs.errorTable429') },
  { code: '500', desc: t('apiDocs.errorTable500') },
  { code: '503', desc: t('apiDocs.errorTable503') },
])

onMounted(() => loadChannelGroups())
</script>

<style scoped>
.api-docs { display: flex; flex-direction: column; gap: 16px; }
.doc-card { border-radius: 16px; }
.card-title { font-size: 15px; font-weight: 700; }
.desc-text { color: #4b5563; font-size: 14px; margin-bottom: 12px; line-height: 1.6; }
.tip-text { color: #9ca3af; font-size: 13px; margin-top: 8px; }
.code-block {
  background: #1e1e2e;
  color: #cdd6f4;
  padding: 16px;
  border-radius: 10px;
  font-size: 13px;
  line-height: 1.6;
  overflow-x: auto;
  white-space: pre;
  margin-bottom: 12px;
  font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
}
.url-block { margin-bottom: 12px; }
.url-label { font-size: 13px; color: #6b7280; margin-bottom: 4px; }
.url-code {
  background: #eff6ff;
  color: #1d4ed8;
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 14px;
  display: inline-block;
}
.endpoint-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
  padding: 10px 14px;
  background: #f8fafc;
  border-radius: 10px;
}
.method-badge {
  background: #6366f1;
  color: #fff;
  padding: 3px 10px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 700;
}
.method-badge.get { background: #10b981; }
.section-label {
  font-size: 13px;
  font-weight: 600;
  color: #374151;
  margin: 16px 0 6px;
}
.bill-list {
  list-style: none;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.bill-list li {
  padding: 10px 14px;
  background: #f9fafb;
  border-radius: 10px;
  font-size: 14px;
  color: #374151;
  border-left: 3px solid #6366f1;
}
.steps { display: flex; flex-direction: column; gap: 16px; }
.step { display: flex; gap: 16px; align-items: flex-start; }
.step-num {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  flex-shrink: 0;
}
.step-title { font-weight: 600; color: #1f2937; margin-bottom: 4px; }
.step-desc { font-size: 14px; color: #6b7280; line-height: 1.5; }
.mt-2 { margin-top: 8px; }
.mb-4 { margin-bottom: 16px; }

.tut-title { font-size: 16px; font-weight: 700; margin-bottom: 8px; color: #1f2937; }
.tut-step {
  background: #eff6ff;
  border-left: 3px solid #6366f1;
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 14px;
  color: #1e40af;
  font-weight: 500;
  margin: 12px 0 8px;
}
.success-text {
  background: #ecfdf5;
  color: #059669;
  padding: 12px 14px;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 500;
  margin-top: 12px;
}
.config-list {
  list-style: none;
  padding: 12px 16px;
  margin: 12px 0;
  background: #f9fafb;
  border-radius: 10px;
}
.config-list li {
  font-size: 14px;
  padding: 6px 0;
  color: #374151;
}
.config-list li code {
  background: #e0e7ff;
  color: #4338ca;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 13px;
}
.mt-4 { margin-top: 16px; }
.link { color: #6366f1; text-decoration: underline; }

.compare-table { margin: 12px 0; font-size: 13px; }
.compare-table code { background: #eef2ff; color: #4338ca; padding: 1px 6px; border-radius: 4px; font-size: 12px; }
.choose-section { margin-top: 16px; padding: 14px 16px; background: #f9fafb; border-radius: 10px; }
.choose-title { font-size: 14px; font-weight: 700; margin: 0 0 10px; color: #1f2937; }
.choose-block { margin-bottom: 10px; font-size: 13px; line-height: 1.5; }
.choose-block strong { color: #4338ca; }
.choose-block p { margin: 4px 0 0; color: #4b5563; }
</style>
