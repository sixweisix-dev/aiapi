<template>
  <div class="page">
    <div class="page-header">
      <h1>📡 访问日志</h1>
      <p class="muted">Caddy 层原始访问记录（含所有到达服务器的请求，比 requests 表更完整）</p>
    </div>

    <el-card shadow="hover" class="mb-4">
      <div class="toolbar">
        <el-radio-group v-model="filter" @change="fetchData">
          <el-radio-button label="all">全部</el-radio-button>
          <el-radio-button label="error">仅错误 (4xx/5xx)</el-radio-button>
        </el-radio-group>
        <el-select v-model="limit" @change="fetchData" style="width:120px">
          <el-option label="50 条" :value="50" />
          <el-option label="100 条" :value="100" />
          <el-option label="200 条" :value="200" />
          <el-option label="500 条" :value="500" />
        </el-select>
        <el-button @click="fetchData" :loading="loading">🔄 刷新</el-button>
        <el-switch v-model="autoRefresh" active-text="自动刷新(10s)" />
      </div>
    </el-card>

    <el-card shadow="hover">
      <el-table :data="logs" v-loading="loading" size="small" stripe style="width:100%">
        <el-table-column label="时间" width="150">
          <template #default="{ row }">
            <span style="font-size:12px">{{ formatTime(row.ts) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="70" align="center">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="方法" width="70" align="center">
          <template #default="{ row }">
            <span class="method">{{ row.method }}</span>
          </template>
        </el-table-column>
        <el-table-column label="路径" min-width="240" show-overflow-tooltip>
          <template #default="{ row }">
            <code class="path">{{ row.uri }}</code>
          </template>
        </el-table-column>
        <el-table-column label="耗时" width="90" align="right">
          <template #default="{ row }">
            <span style="font-size:12px">{{ row.duration_ms.toFixed(0) }}ms</span>
          </template>
        </el-table-column>
        <el-table-column label="大小" width="80" align="right">
          <template #default="{ row }">
            <span style="font-size:12px">{{ formatBytes(row.size) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="IP" width="130">
          <template #default="{ row }">
            <div style="font-size:11px;color:#6b7280">
              <div>{{ row.client_ip }}</div>
              <div style="color:#9ca3af">{{ row.country }}</div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="客户端 UA" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <span style="font-size:11px;color:#6b7280">{{ row.user_agent || '—' }}</span>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="!loading && logs.length === 0" style="text-align:center;padding:60px 0;color:#9ca3af">
        暂无日志数据
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/utils/api'
import dayjs from 'dayjs'

const logs = ref([])
const loading = ref(false)
const filter = ref('all')
const limit = ref(100)
const autoRefresh = ref(false)
let timer = null

function formatTime(ts) {
  return dayjs(ts * 1000).format('HH:mm:ss.SSS')
}

function formatBytes(n) {
  if (n < 1024) return n + 'B'
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + 'K'
  return (n / 1024 / 1024).toFixed(1) + 'M'
}

function statusType(s) {
  if (s >= 500) return 'danger'
  if (s >= 400) return 'warning'
  if (s >= 300) return 'info'
  return 'success'
}

async function fetchData() {
  loading.value = true
  try {
    const params = { limit: limit.value }
    if (filter.value === 'error') params.status = 'error'
    const res = await api.get('/admin/access-logs', { params })
    logs.value = res.logs || []
  } finally {
    loading.value = false
  }
}

watch(autoRefresh, (v) => {
  if (v) {
    timer = setInterval(fetchData, 10000)
    ElMessage.success('已开启自动刷新')
  } else {
    if (timer) { clearInterval(timer); timer = null }
  }
})

onMounted(fetchData)
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.page { padding: 24px; }
.page-header { margin-bottom: 20px; }
.page-header h1 { font-size: 22px; margin: 0 0 4px; }
.muted { color: #9ca3af; font-size: 13px; margin: 0; }
.mb-4 { margin-bottom: 16px; }
.toolbar { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.method { font-family: monospace; font-size: 11px; font-weight: 600; color: #4b5563; }
.path { font-family: monospace; font-size: 12px; color: #059669; }
</style>
