<template>
  <div class="page">
    <div class="page-header">
      <h1>🗄️ 备份恢复</h1>
      <p class="muted">从 Cloudflare R2 下载加密备份 → 就地解密 → 查看内容摘要（不落盘，不写入生产）</p>
    </div>

    <!-- Step 1: 二次密码验证 -->
    <div class="card" v-if="step === 1">
      <div class="step-title">🔐 第 1 步 · 验证身份</div>
      <div class="hint">出于安全考虑，请重新输入你的管理员登录密码以启动备份操作。会话有效期 15 分钟。</div>
      <el-input
        v-model="loginPassword"
        type="password"
        placeholder="管理员登录密码"
        size="large"
        show-password
        style="margin-top:16px;max-width:400px"
        @keyup.enter="verifyPassword"
      />
      <div style="margin-top:16px">
        <el-button type="primary" size="large" :loading="loading" @click="verifyPassword">
          验证 →
        </el-button>
      </div>
    </div>

    <!-- Step 2: 选择备份 -->
    <div class="card" v-if="step === 2">
      <div class="step-title">📅 第 2 步 · 选择备份</div>
      <div class="hint">R2 桶: transitai-backups · 保留策略: 云端 30 天。选一份备份继续。</div>
      <el-button size="small" @click="loadList" :loading="loading" style="margin-top:12px">🔄 刷新列表</el-button>
      <el-table
        v-if="backupList.length > 0"
        :data="backupList"
        style="margin-top:16px"
        highlight-current-row
        @current-change="onSelectBackup"
      >
        <el-table-column prop="key" label="文件名" min-width="280" />
        <el-table-column label="日期时间" width="180">
          <template #default="{ row }">{{ formatDate(row.modified) }}</template>
        </el-table-column>
        <el-table-column label="大小" width="120">
          <template #default="{ row }">{{ formatSize(row.size) }}</template>
        </el-table-column>
      </el-table>
      <el-empty v-else-if="!loading" description="暂无备份文件" :image-size="80" />

      <div v-if="selectedBackup" style="margin-top:20px;padding:12px 16px;background:#f9fafb;border-radius:8px">
        已选择: <strong>{{ selectedBackup.key }}</strong>
      </div>

      <div style="margin-top:20px">
        <el-button @click="reset">← 上一步</el-button>
        <el-button type="primary" :disabled="!selectedBackup" @click="step = 3">下一步 · 解密 →</el-button>
      </div>
    </div>

    <!-- Step 3: 输入解密密码 -->
    <div class="card" v-if="step === 3">
      <div class="step-title">🔓 第 3 步 · 输入解密密码</div>
      <div class="hint">
        备份文件用 <code>BACKUP_ENC_PASSWORD</code>（.env 里那 43 位字符）加密。请输入以解密查看内容。<br>
        <strong>解密后内容仅在浏览器内存中，关闭页面即销毁。</strong>
      </div>
      <el-input
        v-model="decryptPassword"
        type="password"
        placeholder="备份解密密码（BACKUP_ENC_PASSWORD）"
        size="large"
        show-password
        style="margin-top:16px;max-width:500px"
      />
      <div style="margin-top:16px">
        <el-button @click="step = 2">← 上一步</el-button>
        <el-button type="primary" :loading="loading" @click="doDecrypt">解密并查看 →</el-button>
      </div>
    </div>

    <!-- Step 4: 展示解密结果 -->
    <div class="card" v-if="step === 4 && result">
      <div class="step-title">✅ 解密成功</div>

      <div class="stats-row">
        <div class="stat">
          <div class="stat-label">解密后总大小</div>
          <div class="stat-value">{{ formatSize(result.total_size) }}</div>
        </div>
        <div class="stat">
          <div class="stat-label">DB dump 大小 (gzip)</div>
          <div class="stat-value">{{ formatSize(result.db_size) }}</div>
        </div>
        <div class="stat">
          <div class="stat-label">备份内文件数</div>
          <div class="stat-value">{{ result.files.length }}</div>
        </div>
      </div>

      <div style="margin-top:20px">
        <div class="section-title">📁 文件清单</div>
        <div class="file-list">
          <div v-for="f in result.files" :key="f" class="file-item">{{ f }}</div>
        </div>
      </div>

      <div v-if="result.db_summary" style="margin-top:20px">
        <div class="section-title">📄 DB dump 前 100 行摘要</div>
        <pre class="db-preview">{{ result.db_summary }}</pre>
      </div>

      <div class="sop-notice">
        <div style="font-weight:600;margin-bottom:8px">🧑‍💻 恢复到生产（SSH 手动操作）</div>
        <div style="font-size:13px;color:#6b7280;line-height:1.8">
          此页面仅提供下载 + 校验能力。真正恢复到生产需要 SSH 到服务器手动执行 SOP。<br>
          流程详见 <code>scripts/restore_drill.md</code>：<br>
          <span style="color:#374151">停 backend → 灌 dump → 重启 backend</span>
        </div>
      </div>

      <div style="margin-top:24px">
        <el-button @click="reset">↺ 从头开始</el-button>
        <el-button type="warning" :loading="dryRunLoading" @click="doDryRun">🧪 运行 Dry-Run 对比</el-button>
      </div>
    </div>

    <!-- Step 5: Dry-run 结果 -->
    <div class="card" v-if="dryRunResult">
      <div class="step-title">🧪 Dry-Run 对比结果</div>
      <div class="hint">
        备份已恢复到临时 database <code>{{ dryRunResult.test_db_name }}</code>, 对比完成后自动销毁 (未改主库)。<br>
        耗时 {{ dryRunResult.duration_ms }}ms · 共 {{ dryRunResult.tables.length }} 张表。
      </div>
      <el-table :data="dryRunResult.tables" style="margin-top:16px" size="small" max-height="500">
        <el-table-column prop="name" label="表名" min-width="200" />
        <el-table-column prop="current_count" label="当前主库" width="120" align="right">
          <template #default="{ row }">{{ row.current_count.toLocaleString() }}</template>
        </el-table-column>
        <el-table-column prop="restored_count" label="备份恢复后" width="120" align="right">
          <template #default="{ row }">{{ row.restored_count.toLocaleString() }}</template>
        </el-table-column>
        <el-table-column label="变化 (delta)" width="140" align="right">
          <template #default="{ row }">
            <el-tag v-if="row.delta > 0" type="danger" size="small">+{{ row.delta.toLocaleString() }} (数据倒退)</el-tag>
            <el-tag v-else-if="row.delta < 0" type="warning" size="small">{{ row.delta.toLocaleString() }} (数据新增)</el-tag>
            <el-tag v-else type="info" size="small">0 (无变化)</el-tag>
          </template>
        </el-table-column>
      </el-table>
      <div class="sop-notice" style="margin-top:20px">
        <div style="font-weight:600;margin-bottom:8px">📊 如何解读</div>
        <div style="font-size:13px;color:#6b7280;line-height:1.8">
          <strong>delta 为正 (红)</strong>: 备份里比主库多 = 恢复后会"倒退"(比如 users 从 100 -> 80)<br>
          <strong>delta 为负 (黄)</strong>: 备份里比主库少 = 恢复后会"扩展"(通常是备份后新写入的数据)<br>
          <strong>delta 为 0 (灰)</strong>: 表数据完全一致
        </div>
      </div>
    </div>
    <!-- Step 6: 正式恢复 (破坏性!) -->
    <div class="card danger-card" v-if="dryRunResult">
      <div class="step-title">☢️ 第 6 步 · 正式恢复到生产 (破坏性)</div>
      <div class="hint">
        此操作会将当前选中的备份 <code>{{ selectedBackup?.key }}</code> 覆盖到主库。<br>
        <strong style="color:#dc2626">主库当前数据会被替换成备份里的数据。</strong>
        建议先仔细看 Step 5 dry-run 结果。
      </div>

      <div style="margin-top:20px">
        <el-checkbox v-model="ackDryRun">我已查看 Step 5 dry-run 结果, 确认差异可接受</el-checkbox>
      </div>
      <div>
        <el-checkbox v-model="autoEmergency">允许在恢复前自动创建紧急备份 (强烈推荐)</el-checkbox>
      </div>

      <el-input
        v-model="confirmText"
        placeholder="精确输入: 我确认覆盖生产"
        size="large"
        style="margin-top:16px;max-width:400px"
      />

      <el-input
        v-model="restorePassword"
        type="password"
        placeholder="再次输入 BACKUP_ENC_PASSWORD"
        size="large"
        show-password
        style="margin-top:12px;max-width:400px"
      />

      <div style="margin-top:20px">
        <el-button
          type="danger"
          size="large"
          :loading="restoreLoading"
          :disabled="!canRestore"
          @click="doRestore"
        >
          ☢️ 开始恢复到生产
        </el-button>
        <span v-if="restoreLoading" style="margin-left:12px;color:#dc2626">
          恢复中 · 请勿关闭页面 · 预计 5-30 秒
        </span>
      </div>

      <div v-if="restoreResult" class="sop-notice" style="margin-top:20px;background:#f0fdf4;border-left-color:#16a34a">
        <div style="font-weight:600;margin-bottom:8px;color:#166534">✅ 恢复成功</div>
        <div style="font-size:13px;color:#166534;line-height:1.8">
          从: <code>{{ restoreResult.restored_from_key }}</code><br>
          耗时: {{ restoreResult.duration_ms }}ms<br>
          DB 大小: {{ formatSize(restoreResult.db_size_bytes) }}<br>
          <strong>紧急备份 key: <code>{{ restoreResult.emergency_backup_key }}</code></strong> (万一发现问题可从此恢复)<br>
          维护模式已解锁: {{ restoreResult.maintenance_expired ? '是' : '否 (需手动清 Redis key: maintenance:mode)' }}
        </div>
      </div>
    </div>

  </div>
</template>

<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/utils/api'

const step = ref(1)
const loading = ref(false)
const dryRunLoading = ref(false)
const dryRunResult = ref(null)
const ackDryRun = ref(false)
const autoEmergency = ref(true)
const confirmText = ref('')
const restorePassword = ref('')
const restoreLoading = ref(false)
const restoreResult = ref(null)
const loginPassword = ref('')
const backupToken = ref('')
const backupList = ref([])
const selectedBackup = ref(null)
const decryptPassword = ref('')
const result = ref(null)

async function verifyPassword() {
  if (!loginPassword.value) return ElMessage.warning('请输入管理员密码')
  loading.value = true
  try {
    const r = await api.post('/admin/backup/verify-password', { password: loginPassword.value })
    backupToken.value = r.token
    loginPassword.value = ''  // 立刻清掉
    step.value = 2
    ElMessage.success('身份验证通过')
    await loadList()
  } finally {
    loading.value = false
  }
}

async function loadList() {
  loading.value = true
  try {
    const r = await api.post('/admin/backup/list', { backup_token: backupToken.value })
    backupList.value = r.items || []
    if (backupList.value.length === 0) {
      ElMessage.warning('R2 桶里没有备份文件')
    }
  } finally {
    loading.value = false
  }
}

function onSelectBackup(row) {
  selectedBackup.value = row
}

async function doDecrypt() {
  if (!decryptPassword.value) return ElMessage.warning('请输入解密密码')
  loading.value = true
  try {
    const r = await api.post('/admin/backup/decrypt', {
      backup_token: backupToken.value,
      key: selectedBackup.value.key,
      password: decryptPassword.value,
    })
    decryptPassword.value = ''  // 立刻清掉
    result.value = r
    step.value = 4
    ElMessage.success('解密成功')
  } finally {
    loading.value = false
  }
}

async function doDryRun() {
  // 弹窗重新输入解密密码 (安全考虑不复用 Step 3 的密码)
  let dryPwd
  try {
    const { value } = await ElMessageBox.prompt('请再次输入备份解密密码 (仅本次 dry-run 使用)', 'Dry-Run 密码确认', {
      confirmButtonText: '开始 Dry-Run',
      cancelButtonText: '取消',
      inputType: 'password',
      inputPlaceholder: 'BACKUP_ENC_PASSWORD',
    })
    dryPwd = value
  } catch {
    return  // 用户取消
  }
  if (!dryPwd) return ElMessage.warning('密码不能为空')

  dryRunLoading.value = true
  dryRunResult.value = null
  try {
    const r = await api.post('/admin/backup/dry-run', {
      backup_token: backupToken.value,
      key: selectedBackup.value.key,
      password: dryPwd,
    })
    dryRunResult.value = r
    ElMessage.success(`Dry-Run 完成, 对比了 ${r.tables.length} 张表, 主库未受影响`)
  } finally {
    dryRunLoading.value = false
    dryPwd = ''  // 清掉 (GC 会回收)
  }
}

function reset() {
  step.value = 1
  loginPassword.value = ''
  backupToken.value = ''
  backupList.value = []
  selectedBackup.value = null
  decryptPassword.value = ''
  result.value = null
}

const canRestore = computed(() =>
  ackDryRun.value &&
  confirmText.value === '我确认覆盖生产' &&
  !!restorePassword.value &&
  !!selectedBackup.value
)

async function doRestore() {
  restoreLoading.value = true
  restoreResult.value = null
  try {
    const r = await api.post('/admin/backup/restore', {
      backup_token: backupToken.value,
      key: selectedBackup.value.key,
      password: restorePassword.value,
      confirm_text: confirmText.value,
      skip_emergency_backup: !autoEmergency.value,
    })
    restorePassword.value = ''
    confirmText.value = ''
    restoreResult.value = r
    ElMessage.success('生产恢复完成')
  } catch (e) {
    // 显示 emergency_backup_key 供用户回滚
    const errData = e?.response?.data
    if (errData?.emergency_backup_key) {
      ElMessageBox.alert(
        `恢复失败但紧急备份已创建, 可从中回滚:\n\n${errData.emergency_backup_key}\n\n错误: ${errData.error}`,
        '⚠️ 恢复失败',
        { type: 'error', confirmButtonText: '知道了' }
      )
    }
  } finally {
    restoreLoading.value = false
  }
}

function formatDate(iso) {
  if (!iso) return ''
  return new Date(iso).toLocaleString('zh-CN', { hour12: false })
}
function formatSize(bytes) {
  if (!bytes) return '-'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0, n = bytes
  while (n >= 1024 && i < units.length - 1) { n /= 1024; i++ }
  return n.toFixed(1) + ' ' + units[i]
}
</script>

<style scoped>
.page { padding: 24px; max-width: 900px; margin: 0 auto; }
.page-header { margin-bottom: 24px; }
.page-header h1 { font-size: 22px; margin: 0 0 4px; }
.muted { color: #9ca3af; font-size: 13px; margin: 0; }
.card {
  background: #fff; border-radius: 16px; padding: 24px 28px;
  box-shadow: 0 4px 16px rgba(0,0,0,0.04); margin-bottom: 16px;
}
.step-title { font-size: 16px; font-weight: 600; color: #1f2937; margin-bottom: 8px; }
.hint { color: #6b7280; font-size: 13px; line-height: 1.6; }
.hint code { background: #f3f4f6; padding: 1px 6px; border-radius: 4px; font-family: monospace; }
.stats-row { display: flex; gap: 16px; margin-top: 16px; flex-wrap: wrap; }
.stat {
  flex: 1; min-width: 140px; background: #f9fafb;
  border-radius: 10px; padding: 12px 16px;
}
.stat-label { font-size: 12px; color: #9ca3af; }
.stat-value { font-size: 20px; font-weight: 700; color: #1f2937; margin-top: 4px; }
.section-title { font-size: 14px; font-weight: 600; color: #374151; margin-bottom: 8px; }
.file-list { background: #f9fafb; border-radius: 8px; padding: 12px 16px; max-height: 200px; overflow: auto; }
.file-item { font-family: monospace; font-size: 13px; color: #4b5563; line-height: 1.8; }
.db-preview {
  background: #1f2937; color: #d1d5db;
  padding: 16px; border-radius: 8px;
  max-height: 400px; overflow: auto;
  font-family: 'Menlo', monospace; font-size: 12px; line-height: 1.5;
  white-space: pre-wrap; word-break: break-all;
}
.danger-card {
  background: #fff1f2;
  border: 2px solid #fecaca;
}
.danger-card .step-title { color: #dc2626; }
.sop-notice {
  margin-top: 24px; padding: 16px 20px;
  background: #fffbeb; border-left: 3px solid #f59e0b; border-radius: 8px;
}
</style>
