<template>
  <div class="profit-page">
    <!-- 顶部标题栏 -->
    <div class="header-bar">
      <div>
        <h2 class="page-title">💰 利润看板</h2>
        <p class="page-subtitle">实时收入、成本、毛利分析</p>
      </div>
      <el-radio-group v-model="range" @change="loadData" size="default">
        <el-radio-button label="today">今日</el-radio-button>
        <el-radio-button label="month">本月</el-radio-button>
        <el-radio-button label="year">本年</el-radio-button>
      </el-radio-group>
    </div>

    <!-- 顶部 4 张数据卡 -->
    <el-row :gutter="16" v-loading="loading" class="kpi-row">
      <el-col :xs="12" :sm="12" :md="6">
        <div class="kpi-card kpi-revenue">
          <div class="kpi-icon">📈</div>
          <div class="kpi-label">收入</div>
          <div class="kpi-value">¥{{ fmt(data.summary?.revenue) }}</div>
          <div class="kpi-sub">{{ data.summary?.request_count || 0 }} 次请求</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6">
        <div class="kpi-card kpi-cost">
          <div class="kpi-icon">💸</div>
          <div class="kpi-label">成本</div>
          <div class="kpi-value">¥{{ fmt(data.summary?.cost) }}</div>
          <div class="kpi-sub">Anthropic 实付</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6">
        <div class="kpi-card kpi-profit">
          <div class="kpi-icon">💎</div>
          <div class="kpi-label">毛利</div>
          <div class="kpi-value">¥{{ fmt(data.summary?.profit) }}</div>
          <div class="kpi-sub">收入 − 成本</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6">
        <div class="kpi-card kpi-margin">
          <div class="kpi-icon">🎯</div>
          <div class="kpi-label">毛利率</div>
          <div class="kpi-value">{{ (data.summary?.profit_margin || 0).toFixed(1) }}%</div>
          <div class="kpi-sub">倍率 {{ data.multiplier || '1.5' }}x</div>
        </div>
      </el-col>
    </el-row>

    <!-- token 用量条 -->
    <div class="token-bar">
      <div class="token-item">
        <span class="token-icon">📥</span>
        <span class="token-label">输入 Tokens</span>
        <span class="token-num">{{ fmtToken(data.summary?.prompt_tokens) }}</span>
      </div>
      <div class="token-divider"></div>
      <div class="token-item">
        <span class="token-icon">📤</span>
        <span class="token-label">输出 Tokens</span>
        <span class="token-num">{{ fmtToken(data.summary?.output_tokens) }}</span>
      </div>
    </div>

    <!-- 分模型收益 -->
    <el-card shadow="never" class="data-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">🤖 分模型收益</span>
          <span class="card-tag">{{ (data.by_model || []).length }} 个模型</span>
        </div>
      </template>
      <div v-if="(data.by_model || []).length === 0" class="empty-tip">暂无数据</div>
      <div v-else class="model-list">
        <div v-for="(m, i) in data.by_model" :key="i" class="model-item">
          <div class="model-row">
            <div class="model-info">
              <span class="model-rank" :class="`rank-${i + 1}`">#{{ i + 1 }}</span>
              <span class="model-name">{{ m.model_name }}</span>
              <span class="model-count">{{ m.request_count }} 次</span>
            </div>
            <div class="model-money">
              <span class="m-revenue">+¥{{ m.revenue.toFixed(2) }}</span>
              <span class="m-profit">毛利 ¥{{ m.profit.toFixed(2) }}</span>
            </div>
          </div>
          <el-progress
            :percentage="Number(m.share.toFixed(1))"
            :stroke-width="8"
            :color="progressColor(i)"
            :show-text="false"
          />
          <div class="model-share">{{ m.share.toFixed(1) }}% 收入占比</div>
        </div>
      </div>
    </el-card>

    <!-- TOP 用户 -->
    <el-card shadow="never" class="data-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">👑 TOP 10 消费用户</span>
        </div>
      </template>
      <div v-if="(data.top_users || []).length === 0" class="empty-tip">暂无数据</div>
      <div v-else class="user-list">
        <div v-for="(u, i) in data.top_users" :key="i" class="user-item">
          <div class="user-rank-wrap">
            <span class="user-rank" :class="`urank-${i + 1}`">{{ medalFor(i) }}</span>
          </div>
          <div class="user-info">
            <div class="user-email">{{ u.email }}</div>
            <div class="user-meta">{{ u.request_count }} 次请求</div>
          </div>
          <div class="user-money">¥{{ u.revenue.toFixed(2) }}</div>
        </div>
      </div>
    </el-card>

    <!-- 30 天趋势 -->
    <el-card shadow="never" class="data-card">
      <template #header>
        <div class="card-header">
          <span class="card-title">📊 最近 30 天趋势</span>
        </div>
      </template>
      <div v-if="(data.trend_30d || []).length === 0" class="empty-tip">暂无数据</div>
      <el-table v-else :data="data.trend_30d" stripe :max-height="360" size="default">
        <el-table-column prop="date" label="日期" min-width="110" />
        <el-table-column label="收入" align="right" min-width="100">
          <template #default="{ row }">
            <span class="t-revenue">¥{{ row.revenue.toFixed(2) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="成本" align="right" min-width="100">
          <template #default="{ row }">
            <span class="t-cost">¥{{ row.cost.toFixed(2) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="毛利" align="right" min-width="100">
          <template #default="{ row }">
            <span class="t-profit">¥{{ row.profit.toFixed(2) }}</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '@/utils/api'

const range = ref('month')
const loading = ref(false)
const data = ref({})

const fmt = (n) => Number(n || 0).toFixed(2)
const fmtToken = (n) => {
  if (!n) return '0'
  if (n >= 1e6) return (n / 1e6).toFixed(2) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K'
  return n.toString()
}
const medalFor = (i) => ['🥇', '🥈', '🥉'][i] || (i + 1)
const progressColor = (i) => {
  const colors = ['#667eea', '#f093fb', '#4facfe', '#43e97b', '#fa709a', '#feca57']
  return colors[i % colors.length]
}

const loadData = async () => {
  loading.value = true
  try {
    const res = await api.get('/admin/profit', { params: { range: range.value } })
    data.value = res.data
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

onMounted(loadData)
</script>

<style scoped>
.profit-page {
  padding: 8px 4px 24px;
}

.header-bar {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 20px;
  padding: 4px;
}
.page-title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  color: #1f2937;
}
.page-subtitle {
  margin: 4px 0 0;
  font-size: 13px;
  color: #6b7280;
}

/* KPI 卡片 */
.kpi-row {
  margin-bottom: 16px;
}
.kpi-row > .el-col {
  margin-bottom: 12px;
}
.kpi-card {
  border-radius: 16px;
  padding: 18px 16px;
  color: #fff;
  position: relative;
  overflow: hidden;
  min-height: 120px;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.08);
  transition: transform 0.2s ease;
}
.kpi-card:hover {
  transform: translateY(-2px);
}
.kpi-revenue { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); }
.kpi-cost    { background: linear-gradient(135deg, #fa709a 0%, #fee140 100%); }
.kpi-profit  { background: linear-gradient(135deg, #11998e 0%, #38ef7d 100%); }
.kpi-margin  { background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%); }
.kpi-icon {
  position: absolute;
  top: 12px;
  right: 14px;
  font-size: 28px;
  opacity: 0.6;
}
.kpi-label {
  font-size: 13px;
  opacity: 0.92;
  margin-bottom: 6px;
}
.kpi-value {
  font-size: 24px;
  font-weight: 700;
  letter-spacing: -0.5px;
  line-height: 1.2;
}
.kpi-sub {
  font-size: 11px;
  opacity: 0.85;
  margin-top: 6px;
}

/* token bar */
.token-bar {
  display: flex;
  align-items: center;
  background: #fff;
  border-radius: 12px;
  padding: 14px 16px;
  margin-bottom: 16px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}
.token-item {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 8px;
}
.token-icon { font-size: 18px; }
.token-label { color: #6b7280; font-size: 13px; }
.token-num {
  margin-left: auto;
  font-weight: 600;
  color: #1f2937;
  font-size: 16px;
}
.token-divider {
  width: 1px;
  height: 24px;
  background: #e5e7eb;
  margin: 0 12px;
}

/* 通用卡片 */
.data-card {
  border-radius: 14px;
  margin-bottom: 16px;
  border: none;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
}
.data-card :deep(.el-card__header) {
  border-bottom: 1px solid #f3f4f6;
  padding: 14px 18px;
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.card-title {
  font-size: 15px;
  font-weight: 600;
  color: #1f2937;
}
.card-tag {
  background: #eef2ff;
  color: #6366f1;
  padding: 2px 10px;
  border-radius: 10px;
  font-size: 12px;
}
.empty-tip {
  text-align: center;
  color: #9ca3af;
  padding: 40px 0;
  font-size: 14px;
}

/* 模型列表 */
.model-list { display: flex; flex-direction: column; gap: 16px; }
.model-item {
  padding: 4px 0;
  border-bottom: 1px dashed #f3f4f6;
  padding-bottom: 14px;
}
.model-item:last-child { border-bottom: none; padding-bottom: 0; }
.model-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  flex-wrap: wrap;
  gap: 8px;
}
.model-info { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.model-rank {
  background: #f3f4f6;
  color: #6b7280;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
}
.rank-1 { background: linear-gradient(135deg, #667eea, #764ba2); color: #fff; }
.rank-2 { background: #fef3c7; color: #92400e; }
.rank-3 { background: #fce7f3; color: #be185d; }
.model-name { font-weight: 600; color: #1f2937; font-size: 14px; }
.model-count { color: #9ca3af; font-size: 12px; }
.model-money { display: flex; flex-direction: column; align-items: flex-end; gap: 2px; }
.m-revenue { color: #10b981; font-weight: 700; font-size: 15px; }
.m-profit { color: #6b7280; font-size: 11px; }
.model-share { color: #9ca3af; font-size: 11px; margin-top: 4px; }

/* 用户列表 */
.user-list { display: flex; flex-direction: column; }
.user-item {
  display: flex;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid #f3f4f6;
  gap: 12px;
}
.user-item:last-child { border-bottom: none; }
.user-rank-wrap { width: 36px; display: flex; justify-content: center; }
.user-rank {
  font-size: 22px;
  font-weight: 700;
  color: #9ca3af;
}
.urank-1, .urank-2, .urank-3 { font-size: 26px; }
.user-info { flex: 1; min-width: 0; }
.user-email {
  font-weight: 600;
  color: #1f2937;
  font-size: 14px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.user-meta { color: #9ca3af; font-size: 12px; margin-top: 2px; }
.user-money {
  font-weight: 700;
  color: #10b981;
  font-size: 16px;
  white-space: nowrap;
}

/* 趋势表格颜色 */
.t-revenue { color: #10b981; font-weight: 600; }
.t-cost    { color: #f59e0b; }
.t-profit  { color: #6366f1; font-weight: 600; }
</style>
