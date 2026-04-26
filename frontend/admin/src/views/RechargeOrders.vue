<template>
  <div>
    <el-card shadow="hover" class="mb-4">
      <el-form :inline="true" @submit.prevent="fetchData">
        <el-form-item label="状态">
          <el-select v-model="status" placeholder="全部" clearable @change="fetchData">
            <el-option label="全部" value="" />
            <el-option label="待支付" value="pending" />
            <el-option label="处理中" value="processing" />
            <el-option label="已支付" value="paid" />
            <el-option label="失败" value="failed" />
            <el-option label="已退款" value="refunded" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="fetchData">查询</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="hover">
      <el-table :data="orders" v-loading="loading" stripe style="width: 100%">
        <el-table-column prop="order_no" label="订单号" width="200" />
        <el-table-column prop="user_id" label="用户ID" width="100" show-overflow-tooltip>
          <template #default="{ row }">{{ row.user_id.slice(0, 8) }}...</template>
        </el-table-column>
        <el-table-column prop="amount" label="金额" width="100" align="right">
          <template #default="{ row }">¥{{ row.amount.toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="payment_method" label="支付方式" width="100" />
        <el-table-column prop="payment_status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.payment_status)" size="small">{{ statusLabel(row.payment_status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="paid_at" label="支付时间" width="170">
          <template #default="{ row }">{{ row.paid_at ? dayjs(row.paid_at).format('YYYY-MM-DD HH:mm') : '-' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="170">
          <template #default="{ row }">{{ dayjs(row.created_at).format('YYYY-MM-DD HH:mm') }}</template>
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
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { rechargeAPI } from '@/utils/api'
import dayjs from 'dayjs'

const orders = ref([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const status = ref('')

function statusTagType(s) {
  return { pending: 'warning', processing: 'info', paid: 'success', failed: 'danger', refunded: '' }[s] || 'info'
}
function statusLabel(s) {
  return { pending: '待支付', processing: '处理中', paid: '已支付', failed: '失败', refunded: '已退款' }[s] || s
}

async function fetchData() {
  loading.value = true
  try {
    const params = { page: page.value, page_size: pageSize.value }
    if (status.value) params.status = status.value
    const res = await rechargeAPI.list(params)
    orders.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

onMounted(fetchData)
</script>
