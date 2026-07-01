<template>
  <div>
    <!-- Search bar -->
    <el-card shadow="hover" class="mb-4">
      <el-form :inline="true" :model="query" @submit.prevent="fetchData">
        <el-form-item label="搜索">
          <el-input v-model="query.search" placeholder="邮箱/用户名" clearable @clear="fetchData" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="query.role" placeholder="全部" clearable @change="fetchData">
            <el-option label="全部" value="" />
            <el-option label="普通" value="user" />
            <el-option label="VIP" value="vip" />
            <el-option label="管理员" value="admin" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="fetchData">搜索</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Users table -->
    <el-card shadow="hover">
      <el-table :data="users" v-loading="loading" stripe style="width: 100%">
        <el-table-column prop="email" label="邮箱" min-width="200" />
        <el-table-column prop="username" label="用户名" width="120" />
        <el-table-column prop="role" label="角色" width="80">
          <template #default="{ row }">
            <el-tag :type="roleTagType(row.role)" size="small">{{ row.role }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="balance" label="余额" width="120" align="right">
          <template #default="{ row }">${{ row.balance.toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="total_spent" label="总消费" width="120" align="right">
          <template #default="{ row }">${{ row.total_spent.toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="request_count" label="请求数" width="90" align="right" />
        <el-table-column prop="is_active" label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.is_active ? 'success' : 'danger'" size="small">
              {{ row.is_active ? '正常' : '封禁' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="membership_tier" label="会员等级" width="100">
          <template #default="{ row }">
            <el-tag v-if="row.membership_tier === 'pro'" type="warning" size="small">专业版</el-tag>
            <el-tag v-else-if="row.membership_tier === 'enterprise'" type="danger" size="small">企业版</el-tag>
            <span v-else style="color:#9ca3af;font-size:12px">免费版</span>
          </template>
        </el-table-column>
        <el-table-column prop="membership_expires_at" label="会员到期" width="120">
          <template #default="{ row }">
            <span v-if="row.membership_expires_at" style="font-size:12px">
              {{ dayjs(row.membership_expires_at).format('YYYY-MM-DD') }}
            </span>
            <span v-else style="color:#9ca3af;font-size:12px">—</span>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="注册时间" width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openEdit(row)">编辑</el-button>
            <el-button
              size="small"
              :type="row.is_active ? 'warning' : 'success'"
              @click="toggleActive(row)"
            >
              {{ row.is_active ? '封禁' : '解封' }}
            </el-button>
            <el-button size="small" @click="openErrorLogs(row)">错误日志</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="flex justify-center mt-4">
        <el-pagination
          v-model:current-page="page"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next"
          @current-change="fetchData"
        />
      </div>
    </el-card>

    <!-- Edit User Dialog -->
    <el-dialog v-model="dialogVisible" title="编辑用户" width="450px">
      <el-form :model="editForm" label-width="100px">
        <el-form-item label="角色">
          <el-select v-model="editForm.role">
            <el-option label="普通用户" value="user" />
            <el-option label="管理员" value="admin" />
          </el-select>
        </el-form-item>
        <el-form-item label="调整余额">
          <el-input-number v-model="editForm.balance_adjust" :precision="2" :step="10" />
          <span class="text-xs text-gray-400 ml-2">正数增加，负数扣除</span>
        </el-form-item>
        <el-form-item label="邮箱验证">
          <el-switch v-model="editForm.email_verified" />
        </el-form-item>
        <el-form-item label="会员等级">
          <el-select v-model="editForm.membership_tier" style="width:140px">
            <el-option label="免费版" value="free" />
            <el-option label="专业版 (Pro)" value="pro" />
            <el-option label="企业版 (Enterprise)" value="enterprise" />
          </el-select>
        </el-form-item>
        <el-form-item label="会员天数" v-if="editForm.membership_tier !== 'free'">
          <el-input-number v-model="editForm.membership_days" :min="0" :max="3650" :step="30" />
          <div style="font-size:11px;color:#9ca3af;margin-top:4px">0=清除会员；>0=从现在起顺延N天</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
      </template>
    </el-dialog>

    <!-- Error Logs Drawer -->
    <el-drawer v-model="errorDrawerVisible" title="错误日志" size="70%" direction="rtl">
      <div v-if="errorLogsLoading" style="text-align:center;padding:40px 0;color:#9ca3af">加载中...</div>
      <div v-else-if="errorLogs.length === 0" style="text-align:center;padding:60px 0;color:#9ca3af">
        <div style="font-size:44px;margin-bottom:12px">✨</div>
        <div>该用户暂无错误请求</div>
      </div>
      <div v-else>
        <div style="margin-bottom:12px;color:#6b7280;font-size:13px">
          共 {{ errorLogs.length }} 条最近的 4xx/5xx 或带错误信息的请求（{{ errorUserEmail }}）
        </div>
        <el-table :data="errorLogs" size="small" stripe style="width:100%">
          <el-table-column label="时间" width="150">
            <template #default="{ row }">
              <span style="font-size:12px">{{ formatTime(row.created_at) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="70" align="center">
            <template #default="{ row }">
              <el-tag :type="row.status_code >= 500 ? 'danger' : 'warning'" size="small">
                {{ row.status_code }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="model_name" label="模型" width="150" show-overflow-tooltip />
          <el-table-column prop="path" label="路径" width="140" show-overflow-tooltip>
            <template #default="{ row }">
              <code style="font-size:11px">{{ row.path }}</code>
            </template>
          </el-table-column>
          <el-table-column prop="error_message" label="错误信息" min-width="240">
            <template #default="{ row }">
              <div style="font-size:12px;color:#dc2626;word-break:break-all;max-height:60px;overflow:auto">
                {{ row.error_message || '—' }}
              </div>
            </template>
          </el-table-column>
          <el-table-column label="耗时" width="80" align="right">
            <template #default="{ row }">
              <span style="font-size:12px">{{ row.duration_ms }}ms</span>
            </template>
          </el-table-column>
          <el-table-column label="客户端" width="180" show-overflow-tooltip>
            <template #default="{ row }">
              <div style="font-size:11px;color:#6b7280">
                <div>{{ row.user_agent || '—' }}</div>
                <div style="color:#9ca3af">{{ row.ip_address || '' }}</div>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="80" fixed="right">
            <template #default="{ row }">
              <el-button size="small" type="primary" link @click="openLogDetail(row)">详情</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-drawer>

    <!-- Log Detail Dialog -->
    <el-dialog v-model="detailVisible" title="请求详情" width="90%" top="5vh" append-to-body>
      <div v-if="detailLog" class="log-detail">
        <div class="detail-grid">
          <div class="detail-item">
            <span class="k">时间</span>
            <span class="v">{{ formatTime(detailLog.created_at) }}</span>
          </div>
          <div class="detail-item">
            <span class="k">状态码</span>
            <el-tag :type="detailLog.status_code >= 500 ? 'danger' : 'warning'" size="small">
              {{ detailLog.status_code }}
            </el-tag>
          </div>
          <div class="detail-item">
            <span class="k">模型</span>
            <span class="v">{{ detailLog.model_name || '—' }}</span>
          </div>
          <div class="detail-item">
            <span class="k">路径</span>
            <code class="v">{{ detailLog.path }}</code>
          </div>
          <div class="detail-item">
            <span class="k">耗时</span>
            <span class="v">{{ detailLog.duration_ms }} ms</span>
          </div>
          <div class="detail-item">
            <span class="k">上游渠道</span>
            <code class="v">{{ detailLog.upstream_channel_id || '—' }}</code>
          </div>
          <div class="detail-item">
            <span class="k">客户端 UA</span>
            <span class="v">{{ detailLog.user_agent || '—' }}</span>
          </div>
          <div class="detail-item">
            <span class="k">IP</span>
            <span class="v">{{ detailLog.ip_address || '—' }}</span>
          </div>
        </div>

        <div class="section">
          <div class="section-title">错误信息</div>
          <pre class="code-block err">{{ detailLog.error_message || '(空)' }}</pre>
        </div>

        <div class="section">
          <div class="section-title">
            请求体
            <el-button size="small" link @click="copyText(prettyJSON(detailLog.request_body))">复制</el-button>
          </div>
          <pre class="code-block">{{ prettyJSON(detailLog.request_body) }}</pre>
        </div>

        <div class="section">
          <div class="section-title">
            上游响应
            <el-button size="small" link @click="copyText(prettyJSON(detailLog.response_body))">复制</el-button>
          </div>
          <pre class="code-block">{{ prettyJSON(detailLog.response_body) }}</pre>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api, { usersAPI } from '@/utils/api'
import dayjs from 'dayjs'

const users = ref([])
const loading = ref(false)
const saving = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const dialogVisible = ref(false)

const query = ref({ search: '', role: '' })
const editForm = ref({ role: 'user', balance_adjust: 0, email_verified: false, membership_tier: 'free', membership_days: 30 })
const editingUser = ref(null)

// Error logs drawer
const errorDrawerVisible = ref(false)
const errorLogsLoading = ref(false)
const errorLogs = ref([])
const errorUserEmail = ref("")
const detailVisible = ref(false)
const detailLog = ref(null)

function formatTime(t) {
  return t ? dayjs(t).format('YYYY-MM-DD HH:mm') : '-'
}

function roleTagType(role) {
  const map = { admin: 'danger', vip: 'warning', user: 'info', guest: '' }
  return map[role] || 'info'
}

async function fetchData() {
  loading.value = true
  try {
    const res = await usersAPI.list({ page: page.value, page_size: pageSize.value, ...query.value })
    users.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

function openEdit(row) {
  editingUser.value = row
  editForm.value = {
    role: row.role,
    balance_adjust: 0,
    email_verified: row.email_verified,
    membership_tier: row.membership_tier || 'free',
    membership_days: 30,
  }
  dialogVisible.value = true
}

async function handleSave() {
  if (!editingUser.value) return
  saving.value = true
  try {
    const data = {}
    if (editForm.value.role !== editingUser.value.role) data.role = editForm.value.role
    if (editForm.value.balance_adjust !== 0) data.balance_adjust = editForm.value.balance_adjust
    if (editForm.value.email_verified !== editingUser.value.email_verified) data.email_verified = editForm.value.email_verified
    const origTier = editingUser.value.membership_tier || 'free'
    if (editForm.value.membership_tier !== origTier) {
      data.membership_tier = editForm.value.membership_tier
      data.membership_days = editForm.value.membership_tier === 'free' ? 0 : (editForm.value.membership_days || 30)
    } else if (editForm.value.membership_tier !== 'free' && editForm.value.membership_days > 0) {
      data.membership_days = editForm.value.membership_days
    }
    if (Object.keys(data).length === 0) {
      ElMessage.info('没有修改')
      return
    }
    await usersAPI.update(editingUser.value.id, data)
    ElMessage.success('更新成功')
    dialogVisible.value = false
    await fetchData()
  } finally {
    saving.value = false
  }
}

async function toggleActive(row) {
  try {
    await ElMessageBox.confirm(
      row.is_active ? '确定要封禁该用户吗？' : '确定要解封该用户吗？',
      '提示'
    )
    await usersAPI.update(row.id, { is_active: !row.is_active })
    ElMessage.success(row.is_active ? '已封禁' : '已解封')
    await fetchData()
  } catch { /* cancelled */ }
}

async function openErrorLogs(row) {
  errorUserEmail.value = row.email
  errorLogs.value = []
  errorDrawerVisible.value = true
  errorLogsLoading.value = true
  try {
    const res = await api.get(`/admin/users/${row.id}/error-logs`, { params: { limit: 100 } })
    errorLogs.value = res.logs || []
  } finally {
    errorLogsLoading.value = false
  }
}

function openLogDetail(row) {
  detailLog.value = row
  detailVisible.value = true
}

function prettyJSON(v) {
  if (v === null || v === undefined || v === '') return '(空)'
  try {
    if (typeof v === 'string') return JSON.stringify(JSON.parse(v), null, 2)
    return JSON.stringify(v, null, 2)
  } catch {
    return String(v)
  }
}

async function copyText(t) {
  try {
    await navigator.clipboard.writeText(t)
    ElMessage.success('已复制')
  } catch {
    ElMessage.error('复制失败')
  }
}

onMounted(fetchData)
</script>

<style scoped>
.log-detail { padding: 0 4px; }
.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px 20px;
  padding: 16px;
  background: #f9fafb;
  border-radius: 8px;
  margin-bottom: 20px;
}
.detail-item { display: flex; align-items: center; gap: 8px; font-size: 13px; }
.detail-item .k { color: #6b7280; min-width: 68px; font-weight: 500; }
.detail-item .v { color: #1f2937; word-break: break-all; }
.detail-item code.v { font-family: monospace; font-size: 12px; color: #059669; }
.section { margin-bottom: 20px; }
.section-title {
  font-size: 13px;
  font-weight: 600;
  color: #4b5563;
  margin-bottom: 8px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.code-block {
  background: #1f2937;
  color: #e5e7eb;
  padding: 12px 14px;
  border-radius: 8px;
  font-size: 12px;
  font-family: monospace;
  max-height: 320px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}
.code-block.err { background: #7f1d1d; color: #fecaca; }
@media (max-width: 640px) {
  .detail-grid { grid-template-columns: 1fr; }
}
</style>
