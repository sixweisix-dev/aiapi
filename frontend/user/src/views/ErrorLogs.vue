<template>
  <div class="page">
    <div class="page-header">
      <h1>🚨 {{ t('errorLogs.title') }}</h1>
      <p class="muted">{{ t('errorLogs.sub') }}</p>
    </div>

    <div v-if="loading" class="state">{{ t('errorLogs.loading') }}</div>
    <div v-else-if="logs.length === 0" class="state ok">
      <div class="ok-emoji">✨</div>
      <div>{{ t('errorLogs.empty') }}</div>
      <div class="ok-sub">{{ t('errorLogs.emptySub') }}</div>
    </div>
    <div v-else class="log-list">
      <details v-for="log in logs" :key="log.id" class="log-item">
        <summary class="log-summary">
          <span class="status" :class="statusClass(log.status_code)">{{ log.status_code }}</span>
          <span class="model">{{ log.model_name || "—" }}</span>
          <span class="path">{{ log.path }}</span>
          <span class="time">{{ formatTime(log.created_at) }}</span>
        </summary>
        <div class="log-detail">
          <div class="detail-grid">
            <div class="di"><span class="k">{{ t('errorLogs.time') }}</span><span class="v">{{ formatFullTime(log.created_at) }}</span></div>
            <div class="di"><span class="k">{{ t('errorLogs.status') }}</span><span class="v">{{ log.status_code }}</span></div>
            <div class="di"><span class="k">{{ t('errorLogs.model') }}</span><span class="v">{{ log.model_name || "—" }}</span></div>
            <div class="di"><span class="k">{{ t('errorLogs.duration') }}</span><span class="v">{{ log.duration_ms }} ms</span></div>
          </div>

          <div class="section">
            <div class="section-title">{{ t('errorLogs.suggestion') }}</div>
            <div class="tips" v-html="getTips(log)"></div>
          </div>

          <div class="section" v-if="log.error_message">
            <div class="section-title">{{ t('errorLogs.errorMsg') }}</div>
            <pre class="code err">{{ log.error_message }}</pre>
          </div>

          <div class="section">
            <div class="section-title">
              {{ t('errorLogs.reqBody') }}
              <button class="copy-btn" @click="copy(pretty(log.request_body))">{{ t('errorLogs.copy') }}</button>
            </div>
            <pre class="code">{{ pretty(log.request_body) }}</pre>
          </div>

          <div class="section">
            <div class="section-title">
              {{ t('errorLogs.respBody') }}
              <button class="copy-btn" @click="copy(pretty(log.response_body))">{{ t('errorLogs.copy') }}</button>
            </div>
            <pre class="code">{{ pretty(log.response_body) }}</pre>
          </div>
        </div>
      </details>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import api from '@/utils/api'
import dayjs from 'dayjs'

const { t } = useI18n()
const logs = ref([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const res = await api.get('/user/error-logs', { params: { limit: 50 } })
    logs.value = res.logs || []
  } finally {
    loading.value = false
  }
}

function formatTime(v) { return dayjs(v).format('MM-DD HH:mm') }
function formatFullTime(v) { return dayjs(v).format('YYYY-MM-DD HH:mm:ss') }
function statusClass(s) {
  if (s >= 500) return 'st-danger'
  if (s >= 400) return 'st-warn'
  return 'st-ok'
}

function pretty(v) {
  if (v === null || v === undefined || v === '') return t('errorLogs.empty_placeholder')
  try {
    if (typeof v === 'string') return JSON.stringify(JSON.parse(v), null, 2)
    return JSON.stringify(v, null, 2)
  } catch { return String(v) }
}

async function copy(text) {
  try { await navigator.clipboard.writeText(text); ElMessage.success(t('errorLogs.copied')) }
  catch { ElMessage.error(t('errorLogs.copyFailed')) }
}

function getTips(log) {
  const code = log.status_code
  const msg = (log.error_message || '').toLowerCase()
  if (code === 400) return t('errorLogs.tip400')
  if (code === 401) return t('errorLogs.tip401')
  if (code === 402 || msg.includes('balance') || msg.includes('insufficient')) return t('errorLogs.tip402')
  if (code === 403) return t('errorLogs.tip403')
  if (code === 404) return t('errorLogs.tip404')
  if (code === 429) return t('errorLogs.tip429')
  if (code === 413) return t('errorLogs.tip413')
  if (code === 500) return t('errorLogs.tip500')
  if (code === 502 || code === 503 || code === 504) return t('errorLogs.tip502')
  return t('errorLogs.tipUnknown')
}

onMounted(load)
</script>

<style scoped>
.page { padding: 16px; max-width: 900px; margin: 0 auto; }
.page-header h1 { font-size: 20px; margin: 0 0 4px; }
.muted { color: #9ca3af; font-size: 13px; margin: 0 0 16px; }
.state { text-align: center; padding: 60px 0; color: #9ca3af; }
.state.ok { color: #10b981; }
.ok-emoji { font-size: 48px; margin-bottom: 8px; }
.ok-sub { font-size: 12px; color: #9ca3af; margin-top: 4px; }
.log-list { display: flex; flex-direction: column; gap: 8px; }
.log-item { background: #fff; border-radius: 12px; box-shadow: 0 1px 4px rgba(0,0,0,0.05); overflow: hidden; }
.log-summary {
  display: flex; align-items: center; gap: 8px;
  padding: 12px 14px; cursor: pointer; user-select: none;
  font-size: 13px;
}
.log-summary:hover { background: #f9fafb; }
.status { padding: 2px 8px; border-radius: 8px; font-weight: 700; font-size: 11px; }
.st-ok { background: #d1fae5; color: #065f46; }
.st-warn { background: #fef3c7; color: #92400e; }
.st-danger { background: #fee2e2; color: #991b1b; }
.model { color: #4b5563; font-weight: 500; min-width: 100px; }
.path { color: #6b7280; font-family: monospace; font-size: 11px; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.time { color: #9ca3af; font-size: 11px; }
.log-detail { padding: 12px 16px 16px; background: #f9fafb; border-top: 1px solid #f3f4f6; }
.detail-grid {
  display: grid; grid-template-columns: repeat(2, 1fr);
  gap: 8px 20px; margin-bottom: 12px; font-size: 12px;
}
.di { display: flex; gap: 8px; }
.di .k { color: #9ca3af; min-width: 50px; }
.di .v { color: #1f2937; }
.section { margin-top: 10px; }
.section-title {
  font-size: 12px; font-weight: 600; color: #4b5563;
  margin-bottom: 6px; display: flex; align-items: center; justify-content: space-between;
}
.tips {
  background: #eff6ff; border-left: 3px solid #3b82f6;
  padding: 10px 12px; border-radius: 6px;
  font-size: 12.5px; color: #1e40af; line-height: 1.7;
}
.tips :deep(code) {
  background: #dbeafe; padding: 1px 6px; border-radius: 4px;
  font-family: monospace; font-size: 11px;
}
.code {
  background: #1f2937; color: #e5e7eb;
  padding: 10px 12px; border-radius: 6px;
  font-size: 11px; font-family: monospace;
  max-height: 200px; overflow: auto;
  white-space: pre-wrap; word-break: break-all;
  margin: 0; line-height: 1.5;
}
.code.err { background: #7f1d1d; color: #fecaca; }
.copy-btn {
  border: none; background: transparent; color: #3b82f6;
  cursor: pointer; font-size: 11px;
}
</style>
