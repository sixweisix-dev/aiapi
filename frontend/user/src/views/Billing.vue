<template>
  <div class="page">
    <!-- 筛选 -->
    <div class="data-card">
      <div class="card-header">
        <span class="card-title">🔍 筛选</span>
        <button class="export-btn" @click="handleExport">📥 导出 CSV</button>
      </div>
      <div class="filter-grid">
        <div class="filter-row">
          <label class="form-label">类型</label>
          <el-select v-model="filters.type" placeholder="全部类型" size="large" style="width:100%" clearable>
            <el-option label="全部" value="" />
            <el-option label="消费" value="chat_completion" />
            <el-option label="充值" value="recharge" />
            <el-option label="调整" value="adjustment" />
            <el-option label="退款" value="refund" />
          </el-select>
        </div>
        <div class="filter-row-2">
          <div>
            <label class="form-label">开始日期</label>
            <el-date-picker v-model="filters.start" type="date" placeholder="开始" value-format="YYYY-MM-DD" size="large" style="width:100%" />
          </div>
          <div>
            <label class="form-label">结束日期</label>
            <el-date-picker v-model="filters.end" type="date" placeholder="结束" value-format="YYYY-MM-DD" size="large" style="width:100%" />
          </div>
        </div>
        <button class="primary-btn" @click="fetchData">🔎 查询</button>
      </div>
    </div>

    <!-- 列表 -->
    <div class="data-card">
      <div class="card-header">
        <span class="card-title">📋 账单明细</span>
        <span class="card-tag">{{ total }} 条</span>
      </div>
      <div v-if="loading" class="empty-tip">加载中...</div>
      <div v-else-if="items.length === 0" class="empty-tip">暂无账单记录</div>
      <div v-else class="bill-list">
        <div v-for="(b, i) in items" :key="i" class="bill-item">
          <div class="bill-row">
            <span class="bill-tag" :class="tagCls(b.type)">{{ typeLabel(b.type) }}</span>
            <span class="bill-amount" :class="b.amount > 0 ? 'income' : 'outcome'">
              {{ b.amount > 0 ? '+' : '' }}¥{{ Number(b.amount || 0).toFixed(6) }}
            </span>
          </div>
          <div class="bill-desc">{{ b.description || '-' }}</div>
          <div class="bill-meta">
            <span>余额: ¥{{ Number(b.balance_after || 0).toFixed(4) }}</span>
            <span>·</span>
            <span>{{ dayjs(b.created_at).format('YYYY-MM-DD HH:mm') }}</span>
          </div>
        </div>
      </div>

      <div v-if="total > pageSize" class="pagination-wrap">
        <el-pagination
          v-model:current-page="page"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next"
          small
          @current-change="fetchData"
        />
      </div>
    </div>
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
const filters = reactive({ type: '', start: '', end: '' })

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
  } catch {} finally { loading.value = false }
}

async function handleExport() {
  try {
    const params = {
      start_date: filters.start || undefined,
      end_date: filters.end || undefined,
    }
    const data = await billingAPI.exportCSV(params)
    // 后端可能返回 string 或 ArrayBuffer
    const blob = new Blob([data], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    const range = filters.start && filters.end
      ? `${filters.start}_${filters.end}`
      : dayjs().format('YYYY-MM')
    a.download = `账单明细_${range}.csv`
    a.click()
    URL.revokeObjectURL(url)
    ElMessage.success('导出成功')
  } catch (e) {
    console.error(e)
    ElMessage.error('导出失败')
  }
}

function typeLabel(t) {
  const map = { chat_completion: '消费', recharge: '充值', adjustment: '调整', refund: '退款' }
  return map[t] || t
}
function tagCls(t) {
  return t === 'recharge' ? 'tag-in' : t === 'chat_completion' ? 'tag-out' : 'tag-other'
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

.export-btn {
  background: #eef2ff;
  color: #6366f1;
  border: none;
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}
.export-btn:active { background: #e0e7ff; }

.filter-grid { display: flex; flex-direction: column; gap: 12px; }
.filter-row { display: flex; flex-direction: column; gap: 6px; }
.filter-row-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.filter-row-2 > div { display: flex; flex-direction: column; gap: 6px; }
.form-label { font-size: 13px; color: #4b5563; font-weight: 500; }

.primary-btn {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  border: none;
  height: 42px;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  width: 100%;
}
.primary-btn:active { opacity: 0.9; }

.empty-tip { text-align: center; color: #9ca3af; padding: 30px 0; font-size: 13px; }

.bill-list { display: flex; flex-direction: column; }
.bill-item {
  padding: 12px 0;
  border-bottom: 1px solid #f3f4f6;
}
.bill-item:last-child { border-bottom: none; }
.bill-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}
.bill-tag {
  font-size: 11px;
  font-weight: 600;
  padding: 3px 10px;
  border-radius: 8px;
}
.tag-in { background: #d1fae5; color: #065f46; }
.tag-out { background: #fef3c7; color: #92400e; }
.tag-other { background: #e0e7ff; color: #3730a3; }
.bill-amount {
  font-size: 15px;
  font-weight: 700;
}
.bill-amount.income { color: #10b981; }
.bill-amount.outcome { color: #ef4444; }
.bill-desc {
  font-size: 13px;
  color: #4b5563;
  margin-bottom: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.bill-meta { display: flex; gap: 6px; font-size: 11px; color: #9ca3af; }

.pagination-wrap {
  display: flex;
  justify-content: center;
  margin-top: 16px;
  padding-top: 14px;
  border-top: 1px solid #f3f4f6;
}
</style>
