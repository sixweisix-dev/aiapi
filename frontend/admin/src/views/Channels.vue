<template>
  <div>
    <!-- 🗂️ 渠道架构树形概览(分组 → 渠道 → 模型) -->
    <el-card shadow="hover" class="mb-4">
      <template #header>
        <div class="flex justify-between items-center">
          <span class="text-base font-medium">🗂️ 渠道架构</span>
          <div style="display:flex;gap:8px;">
            <el-button size="small" @click="$router.push('/channel-groups')">📦 管理分组</el-button>
            <el-button size="small" @click="$router.push('/models')">🤖 管理模型</el-button>
            <el-button size="small" type="primary" plain @click="refreshTree">🔄 刷新</el-button>
          </div>
        </div>
      </template>
      <el-tree
        v-if="treeData.length > 0"
        :data="treeData"
        node-key="id"
        :default-expand-all="true"
        :indent="20"
        :props="{ children: 'children', label: 'label' }"
      >
        <template #default="{ data }">
          <span class="flex items-center" style="gap:8px;flex-wrap:wrap;padding:4px 0;">
            <span>{{ data.type === 'group' ? '📦' : data.type === 'channel' ? '🔌' : '🤖' }}</span>
            <span class="font-medium" style="min-width:140px;">{{ data.label }}</span>
            <template v-if="data.type === 'group'">
              <el-tag size="small" type="success">{{ Number(data.meta.multiplier).toFixed(2) }}×</el-tag>
              <el-tag v-if="data.meta.is_default" size="small" type="warning">默认</el-tag>
              <span class="text-xs text-gray-400">{{ data.children?.length || 0 }} 渠道</span>
            </template>
            <template v-if="data.type === 'channel'">
              <el-tag size="small">{{ data.meta.provider }}</el-tag>
              <el-tag :type="data.meta.health === 'healthy' ? 'success' : data.meta.health === 'unhealthy' ? 'danger' : 'info'" size="small">{{ data.meta.health }}</el-tag>
              <el-tag size="small" type="info">权重 {{ data.meta.weight }}</el-tag>
              <span class="text-xs text-gray-400">{{ data.children?.length || 0 }} 模型</span>
            </template>
            <template v-if="data.type === 'model'">
              <el-tag size="small" type="info">${{ Number(data.meta.input).toFixed(3) }}/M in</el-tag>
              <el-tag size="small" type="info">${{ Number(data.meta.output).toFixed(3) }}/M out</el-tag>
              <el-tag v-if="!data.meta.enabled" size="small" type="danger">已禁用</el-tag>
            </template>
          </span>
        </template>
      </el-tree>
      <el-empty v-else description="暂无数据" :image-size="60" />
    </el-card>

    <!-- 原渠道列表 -->
    <el-card shadow="hover" class="mb-4">
      <div class="flex justify-between items-center">
        <div class="flex items-center" style="gap:12px;">
          <span class="text-base font-medium">上游渠道池</span>
          <el-select v-model="groupFilter" placeholder="全部分组" clearable style="width:200px;" size="small">
            <el-option label="全部分组" :value="null" />
            <el-option v-for="g in channelGroups" :key="g.id" :label="`${g.name} (${Number(g.multiplier).toFixed(2)}×)`" :value="g.id" />
            <el-option label="未分组" :value="0" />
          </el-select>
        </div>
        <el-button type="primary" @click="openCreate">添加渠道</el-button>
      </div>
    </el-card>

    <el-card shadow="hover">
      <el-table :data="filteredChannels" v-loading="loading" stripe style="width: 100%">
        <el-table-column prop="name" label="名称" min-width="140">
          <template #default="{ row }">
            <div>{{ row.name }}</div>
            <el-tag v-if="row.is_dedicated" size="small" type="warning" effect="dark" class="mt-1">专属</el-tag>
            <el-tag v-if="row.dedicated_user_ids_auto" size="small" type="warning" plain class="mt-1 ml-1">🟡 自动 {{ countUsers(row.dedicated_user_ids_auto) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="provider" label="提供商" width="100">
          <template #default="{ row }"><el-tag size="small">{{ row.provider }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="weight" label="权重" width="70" align="center" />
        <el-table-column label="分组" width="120">
          <template #default="{ row }">
            <el-tag v-if="row.group_name" type="primary" size="small">{{ row.group_name }}</el-tag>
            <span v-else class="text-xs text-gray-400">未分组</span>
          </template>
        </el-table-column>

        <el-table-column label="额度使用" min-width="240">
          <template #default="{ row }">
            <div v-if="row.quota_type === 'daily' && row.daily_quota_usd > 0">
              <div class="flex justify-between text-xs mb-1">
                <span>每日 {{ formatUSD(realUsed(row, row.quota_used_today_usd)) }} / {{ formatUSD(row.daily_quota_usd) }}</span>
                <span :style="{color: quotaColor(row)}">{{ quotaPercent(row) }}%</span>
              </div>
              <el-progress :percentage="Math.min(100, quotaPercent(row))" :color="quotaColor(row)" :stroke-width="8" :show-text="false" />
            </div>
            <div v-else-if="row.quota_type === 'fixed' && row.total_quota_usd > 0">
              <div class="flex justify-between text-xs mb-1">
                <span>固定 {{ formatUSD(realUsed(row, row.used_total_usd)) }} / {{ formatUSD(row.total_quota_usd) }}</span>
                <span :style="{color: quotaColor(row)}">{{ quotaPercent(row) }}%</span>
              </div>
              <el-progress :percentage="Math.min(100, quotaPercent(row))" :color="quotaColor(row)" :stroke-width="8" :show-text="false" />
            </div>
            <span v-else class="text-gray-400 text-xs">不限额</span>
          </template>
        </el-table-column>

        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="quotaTagType(row.quota_status)" size="small">{{ quotaLabel(row.quota_status) }}</el-tag>
            <el-tag v-if="row.health_status === 'unhealthy'" type="danger" size="small" class="ml-1">不健康</el-tag>
          </template>
        </el-table-column>

        <el-table-column label="缓存命中" width="100" align="center">
          <template #default="{ row }">
            <span v-if="row.cache_total_tokens > 0">{{ (row.cache_hit_rate * 100).toFixed(1) }}%</span>
            <span v-else class="text-gray-400">-</span>
          </template>
        </el-table-column>

        <el-table-column prop="daily_cost_cny" label="今日成本" width="100" align="right">
          <template #default="{ row }">${{ Number(row.daily_cost_cny || 0).toFixed(4) }}</template>
        </el-table-column>

        <el-table-column prop="monthly_cost_cny" label="本月成本" width="100" align="right">
          <template #default="{ row }">${{ Number(row.monthly_cost_cny || 0).toFixed(2) }}</template>
        </el-table-column>

        <el-table-column prop="error_streak" label="连失败" width="80" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.error_streak >= 3" type="danger" size="small">{{ row.error_streak }}</el-tag>
            <span v-else>{{ row.error_streak }}</span>
          </template>
        </el-table-column>

        <el-table-column prop="is_enabled" label="启用" width="70" align="center">
          <template #default="{ row }"><el-switch :model-value="row.is_enabled" disabled size="small" /></template>
        </el-table-column>

        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openEdit(row)">编辑</el-button>
            <el-button size="small" type="warning" @click="handleResetQuota(row)">重置额度</el-button>
            <el-button size="small" type="success" :loading="testingId === row.id" @click="handleTest(row)">测试</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="isEditing ? '编辑渠道' : '添加渠道'" width="640px">
      <el-form :model="form" :rules="rules" ref="formRef" label-width="120px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="例如: 云翼主力账号" />
        </el-form-item>
        <el-form-item label="提供商" prop="provider">
          <el-select v-model="form.provider" style="width:100%">
            <el-option label="OpenAI" value="openai" />
            <el-option label="Anthropic" value="anthropic" />
            <el-option label="Google" value="google" />
            <el-option label="Qwen" value="qwen" />
            <el-option label="DeepSeek" value="deepseek" />
          </el-select>
        </el-form-item>
        <el-form-item label="API Key" prop="api_key">
          <el-input v-model="form.api_key" type="password" show-password :placeholder="isEditing ? '留空则不修改' : ''" />
        </el-form-item>
        <el-form-item label="自定义URL">
          <el-input v-model="form.base_url" placeholder="如 https://yunyi.rdzhvip.com/claude" />
        </el-form-item>
        <el-form-item label="权重">
          <el-input-number v-model="form.weight" :min="1" :max="100" />
          <span class="ml-2 text-xs text-gray-400">值越大被选中概率越高</span>
        </el-form-item>

        <el-form-item label="分组">
          <el-select v-model="form.group_id" placeholder="选择渠道分组" clearable style="width:240px">
            <el-option v-for="g in channelGroups" :key="g.id" :label="`${g.name} (${Number(g.multiplier).toFixed(2)}×)`" :value="g.id" />
          </el-select>
          <span class="ml-2 text-xs text-gray-400">渠道隶属哪个分组, 决定路由 + 倍率</span>
        </el-form-item>

        <el-divider>额度管理</el-divider>

        <el-form-item label="额度模式">
          <el-radio-group v-model="form.quota_type">
            <el-radio value="unlimited">不限额</el-radio>
            <el-radio value="daily">每日刷新型</el-radio>
            <el-radio value="fixed">固定总额型</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="每日额度 USD" v-if="form.quota_type === 'daily'">
          <el-input-number v-model="form.daily_quota_usd" :min="0" :step="10" />
          <span class="ml-2 text-xs text-gray-400">每天北京时间 0 点自动重置, 例如 100/200</span>
        </el-form-item>
        <el-form-item label="固定总额 USD" v-if="form.quota_type === 'fixed'">
          <el-input-number v-model="form.total_quota_usd" :min="0" :step="50" />
          <span class="ml-2 text-xs text-gray-400">用完即止, 不重置</span>
        </el-form-item>

        <el-form-item label="订阅开始" v-if="form.quota_type === 'daily'">
          <el-date-picker v-model="form.subscription_start" type="date" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
        <el-form-item label="订阅结束" v-if="form.quota_type === 'daily'">
          <el-date-picker v-model="form.subscription_end" type="date" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>

        <el-divider>计费模式</el-divider>

        <el-form-item label="计费模式">
          <el-select v-model="form.billing_mode" style="width: 200px">
            <el-option label="按量计费 (pay-as-you-go)" value="pay_as_you_go" />
            <el-option label="包月套餐 (subscription)" value="subscription" />
          </el-select>
          <span class="ml-2 text-xs text-gray-400">subscription = 上游每月固定费用; pay_as_you_go = 按 token 计费</span>
        </el-form-item>

        <el-form-item label="月费 USD" v-if="form.billing_mode === 'subscription'">
          <el-input-number v-model="form.monthly_fee_cny" :min="0" :step="1" :precision="2" controls-position="right" />
          <span class="ml-2 text-xs text-gray-400">每月固定支付的费用 (利润看板会按 月费/30 摊销到每天)</span>
        </el-form-item>

        <el-divider>对账倍率（widget 用）</el-divider>

        <el-form-item label="对账倍率">
          <el-input-number v-model="form.reconcile_multiplier" :min="0.1" :max="2" :step="0.01" :precision="2" controls-position="right" />
          <span class="ml-2 text-xs text-gray-400">默认 1.0。用法：跑一段时间后对比上游后台真实消耗 vs 我方 quota_used_today_usd（DB），算出实际比值填入。Widget 余额 = daily_quota − quota_used_today / 对账倍率</span>
        </el-form-item>

        <el-divider>Cache 优化</el-divider>

        <el-form-item label="1h Cache Beta">
          <el-switch v-model="form.enable_cache_1h_beta" active-text="启用" inactive-text="关闭" />
          <span class="ml-2 text-xs text-gray-400">注入 anthropic-beta: extended-cache-ttl-2025-04-11 header, cache TTL 5min→60min。仅对真支持 cache 的上游 (Anthropic 直连) 有效, 反代池开了无用甚至亏损</span>
        </el-form-item>

        <el-form-item label="自动注入 Cache">
          <el-switch v-model="form.auto_inject_cache" active-text="启用" inactive-text="关闭" />
          <span class="ml-2 text-xs text-gray-400">网关层解析请求, 给长 system (≥4000 字符) 自动加 cache_control。客户端零改动即可享受 cache。配合 1h Beta 时使用 ttl:1h, 否则 5min</span>
        </el-form-item>

        <el-divider>专属配置</el-divider>

        <el-form-item label="专属渠道">
          <el-switch v-model="form.is_dedicated" />
          <span class="ml-2 text-xs text-gray-400">仅专属用户可路由到此渠道</span>
        </el-form-item>
        <el-form-item label="专属用户ID" v-if="form.is_dedicated">
          <el-input v-model="form.dedicated_user_ids" type="textarea" :rows="2" placeholder="逗号分隔, 如: uuid1, uuid2" />
        </el-form-item>
        <el-form-item label="自动隔离名单" v-if="form.is_dedicated && form.dedicated_user_ids_auto">
          <el-input v-model="form.dedicated_user_ids_auto" type="textarea" :rows="2" readonly />
          <div class="text-xs text-gray-400 mt-1">🟡 系统根据 30 分钟成本占比自动加入；每日 0 点重置</div>
        </el-form-item>

        <el-form-item label="启用" v-if="isEditing">
          <el-switch v-model="form.is_enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
const countUsers = (csv) => csv ? csv.split(',').filter(x => x.trim()).length : 0
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import dayjs from 'dayjs'
import api, { channelsAPI } from '../utils/api'

const channels = ref([])

const filteredChannels = computed(() => {
  const g = groupFilter.value
  if (g === null || g === undefined || g === '') return channels.value
  if (g === 0) return channels.value.filter(c => !c.group_id)
  return channels.value.filter(c => c.group_id === g)
})
const loading = ref(false)
const dialogVisible = ref(false)
const isEditing = ref(false)
const saving = ref(false)
const testingId = ref(null)
const formRef = ref(null)
const editingId = ref(null)

const form = reactive({
  name: '', provider: 'anthropic', api_key: '', base_url: '', weight: 1, is_enabled: true,
  quota_type: 'unlimited', daily_quota_usd: 0, total_quota_usd: 0,
  subscription_start: '', subscription_end: '',
  is_dedicated: false, dedicated_user_ids: '', reconcile_multiplier: 1.0, billing_mode: 'pay_as_you_go', monthly_fee_cny: 0, enable_cache_1h_beta: false, auto_inject_cache: false, group_id: null
})

const rules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  provider: [{ required: true, message: '请选择提供商', trigger: 'change' }],
}

const channelGroups = ref([])
const groupFilter = ref(null)
async function loadGroups() {
  try { const r = await api.get('/admin/channel-groups'); channelGroups.value = r?.items || [] } catch (e) {}
}

const modelsList = ref([])
async function loadModels() {
  try { const r = await api.get('/admin/models'); modelsList.value = r?.items || [] } catch (e) {}
}
async function refreshTree() {
  await Promise.all([fetchData(), loadGroups(), loadModels()])
}

// 渠道架构树形:分组 → 渠道 → 模型(每渠道下显示该分组下所有模型)
const treeData = computed(() => {
  if (!channelGroups.value.length) return []
  return channelGroups.value.map(g => {
    const gChannels = channels.value.filter(c => c.group_id === g.id)
    const gModels = modelsList.value.filter(m => m.group_id === g.id)
    const mkModel = (c, m) => ({
      id: `model-${c?.id || 'none'}-${m.id}`,
      type: 'model',
      label: m.display_name || m.name,
      meta: {
        input: (Number(m.input_price) || 0) * 1000 * (Number(m.multiplier) || 1) * (Number(g.multiplier) || 1),
        output: (Number(m.output_price) || 0) * 1000 * (Number(m.multiplier) || 1) * (Number(g.multiplier) || 1),
        enabled: m.is_enabled,
      },
    })
    return {
      id: `group-${g.id}`,
      type: 'group',
      label: g.name,
      meta: { multiplier: g.multiplier, is_default: g.is_default },
      children: gChannels.length > 0
        ? gChannels.map(c => ({
            id: `channel-${c.id}`,
            type: 'channel',
            label: c.name,
            meta: { provider: c.provider, health: c.health_status, weight: c.weight },
            children: gModels.map(m => mkModel(c, m)),
          }))
        : gModels.map(m => mkModel(null, m)),
    }
  })
})

onMounted(() => { fetchData(); loadGroups(); loadModels() })

async function fetchData() {
  loading.value = true
  try {
    const data = await channelsAPI.list()
    channels.value = data.items || []
  } catch { ElMessage.error('加载失败') } finally { loading.value = false }
}

// 反算上游真实消耗 USD (使用 reconcile_multiplier; 默认 1.0)
function realUsed(row, raw) {
  const m = Number(row.reconcile_multiplier) || 1.0
  return raw / m
}
function quotaPercent(row) {
  if (row.quota_type === 'daily' && row.daily_quota_usd > 0) {
    return Number((realUsed(row, row.quota_used_today_usd) / row.daily_quota_usd * 100).toFixed(1))
  }
  if (row.quota_type === 'fixed' && row.total_quota_usd > 0) {
    return Number((realUsed(row, row.used_total_usd) / row.total_quota_usd * 100).toFixed(1))
  }
  return 0
}
function quotaColor(row) {
  const p = quotaPercent(row)
  if (p >= 90) return '#dc2626'
  if (p >= 80) return '#f59e0b'
  if (p >= 50) return '#3b82f6'
  return '#10b981'
}
function quotaTagType(s) { return { normal:'success', warning:'warning', critical:'danger', exhausted:'info' }[s] || '' }
function quotaLabel(s) { return { normal:'正常', warning:'预警 80%', critical:'紧急 90%', exhausted:'已耗尽' }[s] || s }
function formatUSD(v) { return '$' + Number(v || 0).toFixed(2) }

function openCreate() {
  isEditing.value = false; editingId.value = null
  Object.assign(form, { name:'', provider:'anthropic', api_key:'', base_url:'', weight:1, is_enabled:true, quota_type:'unlimited', daily_quota_usd:0, total_quota_usd:0, subscription_start:'', subscription_end:'', is_dedicated:false, dedicated_user_ids:'', reconcile_multiplier:1.0, billing_mode:'pay_as_you_go', monthly_fee_cny:0, enable_cache_1h_beta:false, auto_inject_cache:false, group_id:null, account_balance_usd:0 })
  dialogVisible.value = true
}

function openEdit(row) {
  isEditing.value = true; editingId.value = row.id
  Object.assign(form, {
    name: row.name, provider: row.provider, api_key: '',
    base_url: row.base_url || '', weight: row.weight, is_enabled: row.is_enabled,
    quota_type: row.quota_type || 'unlimited',
    daily_quota_usd: row.daily_quota_usd || 0,
    total_quota_usd: row.total_quota_usd || 0,
    subscription_start: row.subscription_start ? dayjs(row.subscription_start).format('YYYY-MM-DD') : '',
    subscription_end: row.subscription_end ? dayjs(row.subscription_end).format('YYYY-MM-DD') : '',
    is_dedicated: row.is_dedicated || false,
    dedicated_user_ids: row.dedicated_user_ids || '',
    dedicated_user_ids_auto: row.dedicated_user_ids_auto || '',
    reconcile_multiplier: Number(row.reconcile_multiplier) || 1.0,
    billing_mode: row.billing_mode || 'pay_as_you_go',
    monthly_fee_cny: Number(row.monthly_fee_cny) || 0,
    enable_cache_1h_beta: !!row.enable_cache_1h_beta,
    auto_inject_cache: !!row.auto_inject_cache,
    group_id: row.group_id || null
  })
  dialogVisible.value = true
}

async function handleSave() {
  await formRef.value?.validate().catch(() => null)
  saving.value = true
  const payload = {
    name: form.name, provider: form.provider, weight: form.weight,
    quota_type: form.quota_type,
    daily_quota_usd: form.daily_quota_usd,
    total_quota_usd: form.total_quota_usd,
    is_dedicated: form.is_dedicated,
    dedicated_user_ids: form.dedicated_user_ids,
    reconcile_multiplier: Number(form.reconcile_multiplier) || 1.0,
    billing_mode: form.billing_mode || 'pay_as_you_go',
    monthly_fee_cny: Number(form.monthly_fee_cny) || 0,
    enable_cache_1h_beta: !!form.enable_cache_1h_beta,
    auto_inject_cache: !!form.auto_inject_cache,
    group_id: form.group_id ? Number(form.group_id) : 0,
  }
  if (form.api_key) payload.api_key = form.api_key
  if (form.base_url) payload.base_url = form.base_url
  if (form.subscription_start) payload.subscription_start = form.subscription_start
  if (form.subscription_end) payload.subscription_end = form.subscription_end
  if (isEditing.value) payload.is_enabled = form.is_enabled

  try {
    if (isEditing.value) {
      await channelsAPI.update(editingId.value, payload)
      ElMessage.success('更新成功')
    } else {
      await channelsAPI.create(payload)
      ElMessage.success('添加成功')
    }
    dialogVisible.value = false
    fetchData()
  } catch { ElMessage.error('保存失败') } finally { saving.value = false }
}

async function handleResetQuota(row) {
  await ElMessageBox.confirm(`重置 ${row.name} 的今日额度? 仅清零 quota_used_today.`, '提示', { type: 'warning' }).catch(() => null)
  try { await channelsAPI.update(row.id, { reset_quota: true }); ElMessage.success('已重置'); fetchData() } catch { ElMessage.error('重置失败') }
}

async function handleTest(row) {
  testingId.value = row.id
  try { await channelsAPI.test(row.id); ElMessage.success('连接正常') } catch { ElMessage.error('连接失败') } finally { testingId.value = null; fetchData() }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`删除 ${row.name}?`, '确认', { type: 'warning' }).catch(() => null)
  try { await channelsAPI.delete(row.id); ElMessage.success('已删除'); fetchData() } catch { ElMessage.error('删除失败') }
}
</script>

<style scoped>
.flex { display: flex; }
.justify-between { justify-content: space-between; }
.items-center { align-items: center; }
.mb-4 { margin-bottom: 16px; }
.mb-1 { margin-bottom: 4px; }
.mt-1 { margin-top: 4px; }
.ml-1 { margin-left: 4px; }
.ml-2 { margin-left: 8px; }
.text-base { font-size: 14px; }
.text-xs { font-size: 12px; }
.text-gray-400 { color: #9ca3af; }
.font-medium { font-weight: 500; }
</style>
