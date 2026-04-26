<template>
  <div class="page">
    <!-- 创建表单 -->
    <div class="data-card">
      <div class="card-header">
        <span class="card-title">🔑 创建 API Key</span>
      </div>
      <div class="form-body">
        <div class="form-row">
          <label class="form-label">名称</label>
          <el-input v-model="createForm.name" placeholder="例如: 开发环境" size="large" />
        </div>
        <div class="form-row form-grid-2">
          <div>
            <label class="form-label">RPM (每分钟请求)</label>
            <el-input-number v-model="createForm.rpm_limit" :min="0" :max="10000" size="large" :controls="false" style="width:100%" />
          </div>
          <div>
            <label class="form-label">TPM (每分钟 Token)</label>
            <el-input-number v-model="createForm.tpm_limit" :min="0" :max="1000000" size="large" :controls="false" style="width:100%" />
          </div>
        </div>
        <button class="primary-btn" :disabled="creating" @click="handleCreate">
          <span v-if="creating">创建中...</span>
          <span v-else>＋ 创建 API Key</span>
        </button>
        <div class="form-tip">RPM/TPM 设为 0 表示不限制</div>
      </div>
    </div>

    <!-- Key 列表 -->
    <div class="data-card">
      <div class="card-header">
        <span class="card-title">📦 我的 API Key</span>
        <span class="card-tag">{{ total }} 个</span>
      </div>
      <div v-if="loading" class="empty-tip">加载中...</div>
      <div v-else-if="keys.length === 0" class="empty-tip">暂无 API Key，请先创建</div>
      <div v-else class="key-list">
        <div v-for="k in keys" :key="k.id" class="key-item">
          <div class="key-top">
            <div class="key-name">{{ k.name }}</div>
            <span class="key-status" :class="k.is_active ? 'active' : 'inactive'">
              {{ k.is_active ? '启用' : '禁用' }}
            </span>
          </div>
          <div class="key-prefix">sk-{{ k.prefix }}••••••••</div>
          <div class="key-meta">
            <span>📊 使用 {{ k.total_used || 0 }} 次</span>
            <span>·</span>
            <span>{{ k.last_used_at ? dayjs(k.last_used_at).format('MM-DD HH:mm') : '从未使用' }}</span>
          </div>
          <div class="key-actions">
            <button class="action-btn" :class="k.is_active ? 'btn-warn' : 'btn-success'" @click="handleToggle(k)">
              {{ k.is_active ? '禁用' : '启用' }}
            </button>
            <el-popconfirm title="确定删除此 Key？" @confirm="handleDelete(k.id)">
              <template #reference>
                <button class="action-btn btn-danger">删除</button>
              </template>
            </el-popconfirm>
          </div>
        </div>
      </div>
    </div>

    <!-- 新 Key 弹窗 -->
    <el-dialog v-model="showNewKey" title="🎉 创建成功" width="92%" style="max-width:480px">
      <div class="warn-box">⚠️ 请立即复制并妥善保存！关闭后将无法再次查看完整 Key。</div>
      <el-input v-model="newKeyValue" type="textarea" :rows="3" readonly style="margin-top:12px" />
      <button class="primary-btn" style="margin-top:14px" @click="copyKey">📋 一键复制</button>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { apiKeysAPI } from '@/utils/api'
import dayjs from 'dayjs'

const loading = ref(true)
const creating = ref(false)
const keys = ref([])
const total = ref(0)
const showNewKey = ref(false)
const newKeyValue = ref('')
const createForm = ref({ name: '', rpm_limit: 0, tpm_limit: 0 })

onMounted(fetchKeys)

async function fetchKeys() {
  loading.value = true
  try {
    const data = await apiKeysAPI.list()
    keys.value = data
    total.value = data.length
  } catch {} finally { loading.value = false }
}

async function handleCreate() {
  if (!createForm.value.name) return ElMessage.warning('请输入名称')
  creating.value = true
  try {
    const data = await apiKeysAPI.create({
      name: createForm.value.name,
      rpm_limit: createForm.value.rpm_limit > 0 ? createForm.value.rpm_limit : undefined,
      tpm_limit: createForm.value.tpm_limit > 0 ? createForm.value.tpm_limit : undefined,
    })
    newKeyValue.value = data.key
    showNewKey.value = true
    createForm.value.name = ''
    ElMessage.success('创建成功')
    await fetchKeys()
  } catch {} finally { creating.value = false }
}

async function handleToggle(row) {
  try { await apiKeysAPI.toggle(row.id); ElMessage.success(row.is_active ? '已禁用' : '已启用'); await fetchKeys() } catch {}
}
async function handleDelete(id) {
  try { await apiKeysAPI.delete(id); ElMessage.success('已删除'); await fetchKeys() } catch {}
}
function copyKey() {
  navigator.clipboard.writeText(newKeyValue.value).then(() => ElMessage.success('已复制'))
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
.card-tag {
  background: #eef2ff;
  color: #6366f1;
  padding: 2px 10px;
  border-radius: 10px;
  font-size: 12px;
}
.empty-tip { text-align: center; color: #9ca3af; padding: 30px 0; font-size: 13px; }

.form-body { display: flex; flex-direction: column; gap: 14px; }
.form-row { display: flex; flex-direction: column; gap: 6px; }
.form-grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.form-grid-2 > div { display: flex; flex-direction: column; gap: 6px; }
.form-label { font-size: 13px; color: #4b5563; font-weight: 500; }
.form-tip { font-size: 11px; color: #9ca3af; text-align: center; }

.primary-btn {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  border: none;
  height: 44px;
  border-radius: 12px;
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  width: 100%;
  box-shadow: 0 4px 12px rgba(102,126,234,0.3);
  transition: transform 0.15s;
}
.primary-btn:active { transform: scale(0.98); }
.primary-btn:disabled { opacity: 0.6; }

/* Key 列表 */
.key-list { display: flex; flex-direction: column; gap: 10px; }
.key-item {
  border: 1px solid #f3f4f6;
  border-radius: 12px;
  padding: 14px;
  background: #fafbfc;
}
.key-top { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; }
.key-name { font-size: 15px; font-weight: 600; color: #1f2937; }
.key-status {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 8px;
  font-weight: 600;
}
.key-status.active { background: #d1fae5; color: #065f46; }
.key-status.inactive { background: #fee2e2; color: #991b1b; }
.key-prefix {
  font-family: 'SF Mono', Menlo, monospace;
  background: #fff;
  border: 1px dashed #d1d5db;
  padding: 6px 10px;
  border-radius: 8px;
  font-size: 12px;
  color: #4b5563;
  margin-bottom: 8px;
}
.key-meta { display: flex; gap: 6px; font-size: 11px; color: #9ca3af; margin-bottom: 10px; flex-wrap: wrap; }
.key-actions { display: flex; gap: 8px; }
.action-btn {
  flex: 1;
  border: none;
  height: 34px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}
.action-btn:active { opacity: 0.7; }
.btn-warn { background: #fef3c7; color: #92400e; }
.btn-success { background: #d1fae5; color: #065f46; }
.btn-danger { background: #fee2e2; color: #991b1b; }

.warn-box {
  background: #fef3c7;
  color: #92400e;
  padding: 10px 12px;
  border-radius: 8px;
  font-size: 13px;
}
</style>
