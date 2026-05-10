<template>
  <div>
    <!-- Settings Card -->
    <el-card shadow="hover" class="mb-4">
      <template #header>
        <div class="flex justify-between items-center">
          <span class="text-base font-medium">🐟 闲鱼/闲管家集成</span>
          <el-tag :type="settings.goofish_enabled === 'true' ? 'success' : 'info'">
            {{ settings.goofish_enabled === 'true' ? '已启用' : '未启用' }}
          </el-tag>
        </div>
      </template>
      <el-form :model="settings" label-width="160px" size="default">
        <el-form-item label="启用集成">
          <el-switch
            :model-value="settings.goofish_enabled === 'true'"
            @update:model-value="(v) => settings.goofish_enabled = v ? 'true' : 'false'"
          />
        </el-form-item>
        <el-form-item label="AppKey">
          <el-input v-model="settings.goofish_app_key" placeholder="登录 goofish.pro -> 我的应用 -> 应用详情" />
        </el-form-item>
        <el-form-item label="AppSecret">
          <el-input v-model="settings.goofish_app_secret" type="password" show-password placeholder="同上, 注意保密" />
        </el-form-item>
        <el-form-item label="商家ID (可选)">
          <el-input v-model="settings.goofish_seller_id" placeholder="自研对接可留空" />
        </el-form-item>
        <el-form-item label="Webhook URL">
          <el-input v-model="settings.goofish_webhook_url" readonly>
            <template #append>
              <el-button @click="copyUrl">复制</el-button>
            </template>
          </el-input>
          <div style="font-size:12px;color:#909399;margin-top:4px;">
            把这个 URL 填到闲管家 -> 应用配置 -> 订单推送地址
          </div>
        </el-form-item>
        <el-form-item label="库存预警阈值">
          <el-input-number v-model.number="alertThreshold" :min="1" :max="100" />
          <span style="margin-left:10px;font-size:12px;color:#909399;">某面额未使用卡密低于此数时告警</span>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="saveSettings">保存配置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Stock Summary Card -->
    <el-card shadow="hover" class="mb-4">
      <template #header>
        <div class="flex justify-between items-center">
          <span class="text-base font-medium">📦 卡密库存概况</span>
          <div>
            <el-button size="small" @click="loadStock" :loading="stockLoading">刷新</el-button>
            <el-button size="small" type="primary" @click="exportCodes">📥 导出未使用 CSV</el-button>
          </div>
        </div>
      </template>
      <el-table :data="stockSummary.items" stripe size="small" v-loading="stockLoading">
        <el-table-column prop="note" label="商品" min-width="160" />
        <el-table-column label="面额" width="100" align="right">
          <template #default="{ row }">¥{{ Number(row.balance_amount).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column label="未使用" width="100" align="right">
          <template #default="{ row }">
            <el-tag :type="row.unused < stockSummary.threshold ? 'danger' : 'success'" size="small">
              {{ row.unused }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="used" label="已使用" width="100" align="right" />
      </el-table>
      <el-empty v-if="stockSummary.items?.length === 0" description="暂无闲鱼卡密 (cron 会自动生成, 或手动在卡密管理生成)" />
    </el-card>

    <!-- Orders Card -->
    <el-card shadow="hover">
      <template #header>
        <div class="flex justify-between items-center">
          <span class="text-base font-medium">📋 闲鱼订单 (来自 webhook 推送)</span>
          <el-button size="small" @click="loadOrders" :loading="ordersLoading">刷新</el-button>
        </div>
      </template>
      <el-table :data="orders.items" stripe size="small" v-loading="ordersLoading">
        <el-table-column prop="order_no" label="订单号" min-width="180">
          <template #default="{ row }"><code style="font-size:12px;">{{ row.order_no }}</code></template>
        </el-table-column>
        <el-table-column prop="user_name" label="买家" width="120" />
        <el-table-column label="类型" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.order_type === 7 ? 'warning' : 'info'">{{ orderTypeLabel(row.order_type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">{{ orderStatusLabel(row.order_status) }}</template>
        </el-table-column>
        <el-table-column label="退款" width="80">
          <template #default="{ row }"><el-tag v-if="row.refund_status > 0" size="small" type="danger">{{ refundStatusLabel(row.refund_status) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="更新时间" width="160">
          <template #default="{ row }">{{ row.modify_time ? new Date(row.modify_time * 1000).toLocaleString() : '-' }}</template>
        </el-table-column>
      </el-table>
      <el-empty v-if="orders.items?.length === 0" description="暂无订单 (闲管家成功推送后会显示在这里)" />
      <div v-if="orders.total > 0" style="text-align:right;margin-top:10px;">
        <el-pagination
          v-model:current-page="page"
          :page-size="50"
          :total="orders.total"
          layout="total, prev, pager, next"
          @current-change="loadOrders"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/utils/api'

const settings = reactive({})
const saving = ref(false)
const alertThreshold = computed({
  get: () => Number(settings.goofish_stock_alert_threshold || 5),
  set: (v) => { settings.goofish_stock_alert_threshold = String(v) },
})

const stockSummary = ref({ items: [], threshold: 5 })
const stockLoading = ref(false)

const orders = ref({ items: [], total: 0 })
const ordersLoading = ref(false)
const page = ref(1)

async function loadSettings() {
  try {
    const r = await api.get('/admin/settings')
    Object.assign(settings, r || {})
  } catch (e) { ElMessage.error('加载配置失败: ' + e.message) }
}

async function saveSettings() {
  saving.value = true
  try {
    const payload = {
      goofish_enabled: settings.goofish_enabled,
      goofish_app_key: settings.goofish_app_key,
      goofish_app_secret: settings.goofish_app_secret,
      goofish_seller_id: settings.goofish_seller_id,
      goofish_stock_alert_threshold: settings.goofish_stock_alert_threshold,
    }
    await api.put('/admin/settings', payload)
    ElMessage.success('保存成功')
  } catch (e) { ElMessage.error('保存失败: ' + e.message) }
  saving.value = false
}

async function loadStock() {
  stockLoading.value = true
  try {
    const r = await api.get('/admin/goofish/stock-summary')
    stockSummary.value = r || { items: [], threshold: 5 }
  } catch (e) { ElMessage.error(e.message) }
  stockLoading.value = false
}

async function loadOrders() {
  ordersLoading.value = true
  try {
    const r = await api.get('/admin/goofish/orders', { params: { page: page.value, page_size: 50 } })
    orders.value = r || { items: [], total: 0 }
  } catch (e) { ElMessage.error(e.message) }
  ordersLoading.value = false
}

function copyUrl() {
  navigator.clipboard.writeText(settings.goofish_webhook_url || '')
  ElMessage.success('已复制')
}

function exportCodes() {
  const url = (api.defaults?.baseURL || '') + '/admin/goofish/export-codes'
  window.open(url, '_blank')
}

function orderTypeLabel(t) {
  return { 1: '普通', 2: '分销', 3: '验货宝', 4: '拍卖', 7: '卡密', 8: '直充', 9: '严选', 10: '特卖' }[t] || t
}
function orderStatusLabel(s) {
  return { 1: '待付款', 2: '已付款', 3: '待发货', 4: '已发货', 5: '已完成', 6: '已取消', 11: '退款中' }[s] || s
}
function refundStatusLabel(r) {
  return { 1: '待处理', 2: '待退货', 3: '待收货', 4: '关闭', 5: '成功', 6: '拒绝' }[r] || r
}

onMounted(() => {
  loadSettings()
  loadStock()
  loadOrders()
})
</script>

<style scoped>
.mb-4 { margin-bottom: 16px; }
</style>
