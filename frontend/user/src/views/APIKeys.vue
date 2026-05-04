<template>
  <div class="page">
    <!-- 创建表单 -->
    <div class="data-card">
      <div class="card-header">
        <span class="card-title">{{ t('apiKeys.createTitleFull') }}</span>
      </div>
      <div class="form-body">
        <div class="form-row">
          <label class="form-label">{{ t('apiKeys.keyName') }} <span class="required">*</span></label>
          <el-input v-model="createForm.name" :placeholder="t('apiKeys.keyNamePlaceholder')" size="large" />
        </div>
        <div class="form-row">
          <label class="form-label">{{ t('apiKeys.projectName') }} <span class="optional">{{ t('apiKeys.optional') }}</span></label>
          <el-input v-model="createForm.project_name" :placeholder="t('apiKeys.projectNamePlaceholder')" size="large" />
          <div class="field-tip">{{ t('apiKeys.projectTip') }}</div>
        </div>
        <div class="form-row form-grid-2">
          <div>
            <label class="form-label">{{ t('apiKeys.rpmLimit') }}</label>
            <el-input-number v-model="createForm.rpm_limit" :min="0" :max="10000" size="large" :controls="false" style="width:100%" />
            <div class="field-tip">{{ t('apiKeys.rpmTipDetail', { rpm: currentLimits().rpm, label: t(currentLimits().label) }) }}</div>
          </div>
          <div>
            <label class="form-label">{{ t('apiKeys.tpmLimit') }}</label>
            <el-input-number v-model="createForm.tpm_limit" :min="0" :max="1000000" size="large" :controls="false" style="width:100%" />
            <div class="field-tip">{{ t('apiKeys.tpmTipDetail', { tpm: currentLimits().tpm.toLocaleString(), label: t(currentLimits().label) }) }}</div>
          </div>
        </div>
        <div class="form-row form-grid-2">
          <div>
            <label class="form-label">{{ t('apiKeys.budgetWithUnit') }}</label>
            <el-input-number v-model="createForm.monthly_budget" :min="0" :precision="2" size="large" :controls="false" style="width:100%" />
            <div class="field-tip">{{ t('apiKeys.budgetAutoDisable') }}</div>
          </div>
          <div>
            <label class="form-label">{{ t('apiKeys.alertThreshold') }}</label>
            <el-input-number v-model="createForm.budget_alert_pct" :min="0" :max="100" size="large" :controls="false" style="width:100%" />
            <div class="field-tip">{{ t('apiKeys.alertDefault') }}</div>
          </div>
        </div>
        <button class="primary-btn" :disabled="creating" @click="handleCreate">
          <span v-if="creating">{{ t('apiKeys.creating') }}</span>
          <span v-else>{{ t('apiKeys.createBtnFull') }}</span>
        </button>
        <div class="form-tip">{{ t('apiKeys.allZeroNote') }}</div>
      </div>
    </div>

    <!-- Key 列表 -->
    <div class="data-card">
      <div class="card-header">
        <span class="card-title">{{ t('apiKeys.myKeysTitle') }}</span>
        <span class="card-tag">{{ total }} {{ t('apiKeys.countUnit') }}</span>
      </div>
      <div v-if="loading" class="empty-tip">{{ t('apiKeys.loadingTip') }}</div>
      <div v-else-if="keys.length === 0" class="empty-tip">{{ t('apiKeys.noKeysHint') }}</div>
      <div v-else class="key-list">
        <div v-for="k in keys" :key="k.id" class="key-item">
          <div class="key-top">
            <div class="key-name">
              <span v-if="k.project_name" class="project-tag">📂 {{ k.project_name }}</span>
              {{ k.name }}
            </div>
            <span class="key-status" :class="k.is_active ? 'active' : 'inactive'">
              {{ k.is_active ? t('apiKeys.enable') : t('apiKeys.disable') }}
            </span>
          </div>
          <div class="key-prefix">sk-{{ k.prefix }}••••••••</div>

          <!-- 限制信息 -->
          <div class="key-limits">
            <span class="limit-chip">RPM {{ k.rpm_limit || '∞' }}</span>
            <span class="limit-chip">TPM {{ k.tpm_limit || '∞' }}</span>
            <span class="limit-chip">📊 {{ k.total_used || 0 }} {{ t('apiKeys.timesUnit') }}</span>
          </div>

          <!-- 预算进度条 -->
          <div v-if="k.monthly_budget" class="budget-section">
            <div class="budget-header">
              <span class="budget-label">{{ t('apiKeys.monthlyBudget') }}</span>
              <span class="budget-amount" :class="getBudgetClass(k)">
                ¥{{ Number(k.budget_used || 0).toFixed(2) }} / ¥{{ Number(k.monthly_budget).toFixed(2) }}
                ({{ getBudgetPct(k) }}%)
              </span>
            </div>
            <div class="progress-bar">
              <div class="progress-fill" :class="getBudgetClass(k)"
                   :style="{ width: Math.min(getBudgetPct(k), 100) + '%' }"></div>
            </div>
          </div>

          <!-- 最后使用 -->
          <div class="key-meta">
            {{ k.last_used_at ? t('apiKeys.recentUsePrefix') + ' ' + dayjs(k.last_used_at).format('MM-DD HH:mm') : t('apiKeys.neverUsed') }}
          </div>

          <!-- 操作按钮 -->
          <div class="key-actions">
            <button class="action-btn btn-edit" @click="openEdit(k)">{{ t('apiKeys.edit') }}</button>
            <button class="action-btn" :class="k.is_active ? 'btn-warn' : 'btn-success'" @click="handleToggle(k)">
              {{ k.is_active ? '禁用' : '启用' }}
            </button>
            <el-popconfirm :title="t('apiKeys.confirmDeleteShort')" @confirm="handleDelete(k.id)">
              <template #reference>
                <button class="action-btn btn-danger">{{ t('apiKeys.delete') }}</button>
              </template>
            </el-popconfirm>
          </div>
        </div>
      </div>
    </div>

    <!-- 新 Key 弹窗 -->
    <el-dialog v-model="showNewKey" :title="t('apiKeys.successCreate')" width="92%" style="max-width:480px">
      <div class="warn-box">{{ t('apiKeys.copyWarning') }}</div>
      <el-input v-model="newKeyValue" type="textarea" :rows="3" readonly style="margin-top:12px" />
      <button class="primary-btn" style="margin-top:14px" @click="copyKey">{{ t('apiKeys.oneClickCopy') }}</button>
    </el-dialog>

    <!-- 编辑弹窗 -->
    <el-dialog v-model="showEdit" :title="t('apiKeys.editKeyTitle')" width="92%" style="max-width:480px">
      <div class="form-body">
        <div class="form-row">
          <label class="form-label">{{ t('apiKeys.keyName') }}</label>
          <el-input v-model="editForm.name" size="large" />
        </div>
        <div class="form-row">
          <label class="form-label">{{ t('apiKeys.projectName') }}</label>
          <el-input v-model="editForm.project_name" size="large" :placeholder="t('apiKeys.projectOptionalPh')" />
        </div>
        <div class="form-row form-grid-2">
          <div>
            <label class="form-label">RPM</label>
            <el-input-number v-model="editForm.rpm_limit" :min="0" :max="10000" size="large" :controls="false" style="width:100%" />
          </div>
          <div>
            <label class="form-label">TPM</label>
            <el-input-number v-model="editForm.tpm_limit" :min="0" :max="1000000" size="large" :controls="false" style="width:100%" />
          </div>
        </div>
        <div class="form-row form-grid-2">
          <div>
            <label class="form-label">{{ t('apiKeys.budgetWithUnit') }}</label>
            <el-input-number v-model="editForm.monthly_budget" :min="0" :precision="2" size="large" :controls="false" style="width:100%" />
          </div>
          <div>
            <label class="form-label">{{ t('apiKeys.alertThreshold') }}</label>
            <el-input-number v-model="editForm.budget_alert_pct" :min="0" :max="100" size="large" :controls="false" style="width:100%" />
          </div>
        </div>
        <button class="primary-btn" :disabled="saving" @click="handleUpdate">
          {{ saving ? t('apiKeys.saving') : t('apiKeys.saveModify') }}
        </button>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
import { useAuthStore } from '@/stores/auth' 
import { ref, onMounted, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { apiKeysAPI } from '@/utils/api'
import dayjs from 'dayjs'

const loading = ref(true)
const creating = ref(false)
const saving = ref(false)
const keys = ref([])
const total = ref(0)
const showNewKey = ref(false)
const showEdit = ref(false)
const newKeyValue = ref('')

const auth = useAuthStore()
const tierLimitsMap = {
  free: { rpm: 6, tpm: 10000, label: 'apiKeys.planFree' },
  pro: { rpm: 60, tpm: 100000, label: 'apiKeys.planPro' },
  enterprise: { rpm: 600, tpm: 1000000, label: 'apiKeys.planEnterprise' },
}
const userTier = () => {
  const t = auth.user?.membership_tier || 'free'
  const exp = auth.user?.membership_expires_at
  if (t !== 'free' && exp && new Date(exp) < new Date()) return 'free'
  return t
}
const currentLimits = () => tierLimitsMap[userTier()] || tierLimitsMap.free

const createForm = ref({
  name: '',
  project_name: '',
  rpm_limit: 0,
  tpm_limit: 0,
  monthly_budget: 0,
  budget_alert_pct: 80,
})

const editForm = reactive({
  id: '',
  name: '',
  project_name: '',
  rpm_limit: 0,
  tpm_limit: 0,
  monthly_budget: 0,
  budget_alert_pct: 80,
})

onMounted(fetchKeys)

async function fetchKeys() {
  loading.value = true
  try {
    const data = await apiKeysAPI.list()
    keys.value = data
    total.value = data.length
  } catch {} finally { loading.value = false }
}

function getBudgetPct(k) {
  if (!k.monthly_budget || k.monthly_budget <= 0) return 0
  return Math.round((k.budget_used / k.monthly_budget) * 100 * 10) / 10
}

function getBudgetClass(k) {
  const pct = getBudgetPct(k)
  if (pct >= 95) return 'critical'
  if (pct >= (k.budget_alert_pct || 80)) return 'warning'
  return 'normal'
}

async function handleCreate() {
  if (!createForm.value.name) return ElMessage.warning(t('apiKeys.needKeyName'))
  creating.value = true
  try {
    const f = createForm.value
    const data = await apiKeysAPI.create({
      name: f.name,
      project_name: f.project_name || undefined,
      rpm_limit: f.rpm_limit > 0 ? f.rpm_limit : undefined,
      tpm_limit: f.tpm_limit > 0 ? f.tpm_limit : undefined,
      monthly_budget: f.monthly_budget > 0 ? f.monthly_budget : undefined,
      budget_alert_pct: f.budget_alert_pct,
    })
    newKeyValue.value = data.key
    showNewKey.value = true
    createForm.value = { name: '', project_name: '', rpm_limit: 0, tpm_limit: 0, monthly_budget: 0, budget_alert_pct: 80 }
    ElMessage.success(t('apiKeys.created'))
    await fetchKeys()
  } catch {} finally { creating.value = false }
}

function openEdit(k) {
  editForm.id = k.id
  editForm.name = k.name
  editForm.project_name = k.project_name || ''
  editForm.rpm_limit = k.rpm_limit || 0
  editForm.tpm_limit = k.tpm_limit || 0
  editForm.monthly_budget = k.monthly_budget || 0
  editForm.budget_alert_pct = k.budget_alert_pct || 80
  showEdit.value = true
}

async function handleUpdate() {
  saving.value = true
  try {
    const payload = {
      name: editForm.name,
      project_name: editForm.project_name || '',
      rpm_limit: editForm.rpm_limit,
      tpm_limit: editForm.tpm_limit,
      monthly_budget: editForm.monthly_budget,
      budget_alert_pct: editForm.budget_alert_pct,
    }
    await apiKeysAPI.update(editForm.id, payload)
    ElMessage.success(t('apiKeys.saved'))
    showEdit.value = false
    await fetchKeys()
  } catch {} finally { saving.value = false }
}

async function handleToggle(row) {
  try { await apiKeysAPI.toggle(row.id); ElMessage.success(row.is_active ? t('apiKeys.disabledMsg') : t('apiKeys.enabledMsg')); await fetchKeys() } catch {}
}
async function handleDelete(id) {
  try { await apiKeysAPI.delete(id); ElMessage.success(t('apiKeys.deleted')); await fetchKeys() } catch {}
}
function copyKey() {
  navigator.clipboard.writeText(newKeyValue.value).then(() => ElMessage.success(t('apiKeys.copiedMsg')))
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
.card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 14px; }
.card-title { font-size: 15px; font-weight: 600; color: #1f2937; }
.card-tag { background: #eef2ff; color: #6366f1; padding: 2px 10px; border-radius: 10px; font-size: 12px; }
.empty-tip { text-align: center; color: #9ca3af; padding: 30px 0; font-size: 13px; }
.required { color: #ef4444; }
.optional { color: #9ca3af; font-size: 11px; font-weight: normal; }

.form-body { display: flex; flex-direction: column; gap: 14px; }
.form-row { display: flex; flex-direction: column; gap: 6px; }
.form-grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.form-grid-2 > div { display: flex; flex-direction: column; gap: 6px; }
.form-label { font-size: 13px; color: #4b5563; font-weight: 500; }
.field-tip { font-size: 11px; color: #9ca3af; }
.form-tip { font-size: 11px; color: #9ca3af; text-align: center; }

.primary-btn {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff; border: none; height: 44px; border-radius: 12px;
  font-size: 15px; font-weight: 600; cursor: pointer; width: 100%;
  box-shadow: 0 4px 12px rgba(102,126,234,0.3); transition: transform 0.15s;
}
.primary-btn:active { transform: scale(0.98); }
.primary-btn:disabled { opacity: 0.6; }

.key-list { display: flex; flex-direction: column; gap: 10px; }
.key-item {
  border: 1px solid #f3f4f6; border-radius: 12px; padding: 14px; background: #fafbfc;
}
.key-top { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; gap: 8px; }
.key-name { font-size: 15px; font-weight: 600; color: #1f2937; flex: 1; min-width: 0; }
.project-tag {
  display: inline-block;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff; font-size: 11px; padding: 2px 8px; border-radius: 8px;
  margin-right: 6px; font-weight: 600;
}
.key-status { font-size: 11px; padding: 2px 8px; border-radius: 8px; font-weight: 600; flex-shrink: 0; }
.key-status.active { background: #d1fae5; color: #065f46; }
.key-status.inactive { background: #fee2e2; color: #991b1b; }
.key-prefix {
  font-family: 'SF Mono', Menlo, monospace;
  background: #fff; border: 1px dashed #d1d5db;
  padding: 6px 10px; border-radius: 8px; font-size: 12px;
  color: #4b5563; margin-bottom: 8px;
}
.key-limits { display: flex; gap: 6px; flex-wrap: wrap; margin-bottom: 10px; }
.limit-chip {
  background: #fff; border: 1px solid #e5e7eb;
  padding: 2px 8px; border-radius: 6px; font-size: 11px; color: #6b7280;
}

/* 预算进度条 */
.budget-section { margin-bottom: 10px; }
.budget-header { display: flex; justify-content: space-between; margin-bottom: 4px; }
.budget-label { font-size: 12px; color: #6b7280; font-weight: 500; }
.budget-amount { font-size: 12px; font-weight: 600; }
.budget-amount.normal { color: #10b981; }
.budget-amount.warning { color: #f59e0b; }
.budget-amount.critical { color: #ef4444; }
.progress-bar {
  width: 100%; height: 6px; background: #f3f4f6; border-radius: 3px; overflow: hidden;
}
.progress-fill {
  height: 100%; transition: width 0.3s; border-radius: 3px;
}
.progress-fill.normal { background: linear-gradient(90deg, #10b981, #059669); }
.progress-fill.warning { background: linear-gradient(90deg, #f59e0b, #d97706); }
.progress-fill.critical { background: linear-gradient(90deg, #ef4444, #b91c1c); }

.key-meta { font-size: 11px; color: #9ca3af; margin-bottom: 10px; }
.key-actions { display: flex; gap: 6px; }
.action-btn {
  flex: 1; border: none; height: 34px; border-radius: 8px;
  font-size: 13px; font-weight: 600; cursor: pointer;
}
.action-btn:active { opacity: 0.7; }
.btn-edit { background: #eef2ff; color: #6366f1; }
.btn-warn { background: #fef3c7; color: #92400e; }
.btn-success { background: #d1fae5; color: #065f46; }
.btn-danger { background: #fee2e2; color: #991b1b; }

.warn-box {
  background: #fef3c7; color: #92400e; padding: 10px 12px;
  border-radius: 8px; font-size: 13px;
}
</style>
