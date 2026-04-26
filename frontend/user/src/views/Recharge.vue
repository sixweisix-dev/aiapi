<template>
  <div>
    <!-- Create Recharge Order -->
    <el-card shadow="hover" class="mb-6">
      <template #header>
        <span class="font-medium">充值</span>
      </template>
      <div class="max-w-md">
        <el-form :model="rechargeForm">
          <el-form-item label="充值金额 (¥)">
            <el-input-number v-model="rechargeForm.amount" :min="1" :max="100000" :step="10" />
          </el-form-item>
          <el-form-item label="支付方式">
            <el-radio-group v-model="rechargeForm.method">
              <el-radio value="alipay">支付宝</el-radio>
              <el-radio value="stripe" disabled>Stripe (即将支持)</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" size="large" :loading="submitting" @click="handleRecharge">
              {{ submitting ? '处理中...' : '去支付' }}
            </el-button>
          </el-form-item>
        </el-form>
        <el-alert type="info" :closable="false" class="mt-2">
          <template #title>
            充值金额将立即计入账户余额。支付由支付宝安全处理。
          </template>
        </el-alert>
      </div>
    </el-card>

    <!-- Order History -->
    <el-card shadow="hover">
      <template #header>
        <span class="font-medium">充值记录</span>
      </template>
      <el-table :data="orders" v-loading="loadingOrders" empty-text="暂无充值记录">
        <el-table-column prop="order_no" label="订单号" width="200" />
        <el-table-column prop="amount" label="金额" width="100">
          <template #default="{ row }">
            <span class="text-green-600 font-medium">¥{{ row.amount?.toFixed(2) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="payment_method" label="支付方式" width="100" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusType(row.payment_status)" size="small">
              {{ statusLabel(row.payment_status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="时间" width="160">
          <template #default="{ row }">
            <span class="text-xs text-gray-400">{{ dayjs(row.created_at).format('YYYY-MM-DD HH:mm') }}</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { rechargeAPI } from '@/utils/api'
import dayjs from 'dayjs'

const submitting = ref(false)
const loadingOrders = ref(true)
const orders = ref([])
const rechargeForm = reactive({
  amount: 100,
  method: 'alipay',
})

onMounted(fetchOrders)

async function fetchOrders() {
  loadingOrders.value = true
  try {
    const data = await rechargeAPI.listOrders()
    orders.value = data.orders || []
  } catch {
    // handled
  } finally {
    loadingOrders.value = false
  }
}

async function handleRecharge() {
  if (rechargeForm.amount < 1) {
    ElMessage.warning('充值金额至少 ¥1')
    return
  }
  submitting.value = true
  try {
    const data = await rechargeAPI.createOrder(rechargeForm.amount)
    // Open the Alipay payment page
    if (data.pay_url) {
      window.open(data.pay_url, '_blank')
    }
    ElMessage.success('订单已创建，正在跳转到支付...')
    await fetchOrders()
  } catch {
    // handled
  } finally {
    submitting.value = false
  }
}

function statusType(s) {
  const map = { pending: 'warning', paid: 'success', failed: 'danger', refunded: 'info' }
  return map[s] || 'info'
}

function statusLabel(s) {
  const map = { pending: '待支付', paid: '已支付', failed: '失败', refunded: '已退款' }
  return map[s] || s
}
</script>
