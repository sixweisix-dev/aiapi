<template>
  <div>
    <!-- Filters -->
    <el-card shadow="hover" class="mb-6">
      <el-form :inline="true" :model="filters">
        <el-form-item label="类型">
          <el-select v-model="filters.type" clearable placeholder="全部" style="width: 140px">
            <el-option label="全部" value="" />
            <el-option label="消费" value="chat_completion" />
            <el-option label="充值" value="recharge" />
            <el-option label="调整" value="adjustment" />
            <el-option label="退款" value="refund" />
          </el-select>
        </el-form-item>
        <el-form-item label="开始时间">
          <el-date-picker v-model="filters.start" type="date" placeholder="选择日期" value-format="YYYY-MM-DD" />
        </el-form-item>
        <el-form-item label="结束时间">
          <el-date-picker v-model="filters.end" type="date" placeholder="选择日期" value-format="YYYY-MM-DD" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="fetchData">查询</el-button>
          <el-button @click="handleExport">导出 CSV</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Billing Table -->
    <el-card shadow="hover">
      <el-table :data="items" v-loading="loading" empty-text="暂无账单记录">
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            <el-tag :type="row.type === 'recharge' ? 'success' : row.type === 'chat_completion' ? 'warning' : 'info'" size="small">
              {{ typeLabel(row.type) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="amount" label="金额" width="120">
          <template #default="{ row }">
            <span :class="row.amount > 0 ? 'text-green-600' : 'text-red-600'" class="font-medium">
              {{ row.amount > 0 ? '+' : '' }}{{ row.amount?.toFixed(6) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="balance_before" label="余额(前)" width="120">
          <template #default="{ row }">
            {{ row.balance_before?.toFixed(4) }}
          </template>
        </el-table-column>
        <el-table-column prop="balance_after" label="余额(后)" width="120">
          <template #default="{ row }">
            {{ row.balance_after?.toFixed(4) }}
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column label="时间" width="160">
          <template #default="{ row }">
            <span class="text-xs text-gray-400">{{ dayjs(row.created_at).format('YYYY-MM-DD HH:mm') }}</span>
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
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { billingAPI } from '@/utils/api'
import dayjs from 'dayjs'

const loading = ref(false)
const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 20

const filters = reactive({
  type: '',
  start: '',
  end: '',
})

onMounted(fetchData)

async function fetchData() {
  loading.value = true
  try {
    const data = await billingAPI.list({
      page: page.value,
      page_size: pageSize,
      type: filters.type || undefined,
      start: filters.start || undefined,
      end: filters.end || undefined,
    })
    items.value = data.items || []
    total.value = data.total || 0
  } catch {
    // handled
  } finally {
    loading.value = false
  }
}

async function handleExport() {
  try {
    const data = await billingAPI.exportCSV()
    const blob = new Blob([data], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `billing_${dayjs().format('YYYY-MM-DD')}.csv`
    a.click()
    URL.revokeObjectURL(url)
    ElMessage.success('导出成功')
  } catch {
    ElMessage.error('导出失败')
  }
}

function typeLabel(t) {
  const map = { chat_completion: '消费', recharge: '充值', adjustment: '调整', refund: '退款' }
  return map[t] || t
}
</script>
