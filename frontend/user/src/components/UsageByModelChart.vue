<template>
  <div class="data-card">
    <div class="card-header">
      <span class="card-title">📊 每日模型消耗</span>
      <div class="metric-tabs">
        <button v-for="m in metrics" :key="m.key"
          :class="['tab', { active: metric === m.key }]"
          @click="metric = m.key">{{ m.label }}</button>
      </div>
    </div>
    <div v-if="loading" class="loading-tip">加载中...</div>
    <div v-else-if="!hasData" class="empty-tip">最近 7 天暂无消耗数据</div>
    <v-chart v-else :option="chartOption" autoresize style="height: 320px" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { BarChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent, TitleComponent } from 'echarts/components'
import api from '@/utils/api'

use([CanvasRenderer, BarChart, GridComponent, TooltipComponent, LegendComponent, TitleComponent])

const loading = ref(true)
const rawData = ref([])
const metric = ref('cost')

const metrics = [
  { key: 'cost', label: '金额(¥)' },
  { key: 'tokens', label: 'Token' },
  { key: 'requests', label: '请求数' },
]

const colorPalette = ['#667eea', '#764ba2', '#f093fb', '#4facfe', '#43e97b', '#f6d365', '#fa709a', '#30cfd0']

async function load() {
  try {
    const res = await api.get('/user/usage/by-model?days=7')
    rawData.value = res.data || []
  } finally {
    loading.value = false
  }
}
onMounted(load)

const hasData = computed(() => rawData.value.length > 0)

const chartOption = computed(() => {
  // 按日期+模型聚合
  const dateSet = new Set()
  const modelSet = new Set()
  rawData.value.forEach(r => { dateSet.add(r.date); modelSet.add(r.model) })
  const dates = Array.from(dateSet).sort()
  const models = Array.from(modelSet)

  // 数据矩阵 [模型][日期]
  const matrix = {}
  models.forEach(m => { matrix[m] = {} })
  rawData.value.forEach(r => {
    matrix[r.model][r.date] = r[metric.value] || 0
  })

  const series = models.map((m, idx) => ({
    name: shortName(m),
    type: 'bar',
    stack: 'total',
    itemStyle: { color: colorPalette[idx % colorPalette.length] },
    data: dates.map(d => +(matrix[m][d] || 0).toFixed(metric.value === 'cost' ? 4 : 0)),
  }))

  return {
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      valueFormatter: (v) => formatValue(v),
    },
    legend: { type: 'scroll', bottom: 0, textStyle: { fontSize: 11 } },
    grid: { left: 50, right: 16, top: 24, bottom: 50, containLabel: true },
    xAxis: { type: 'category', data: dates.map(d => d.slice(5)), axisLabel: { fontSize: 11 } },
    yAxis: { type: 'value', axisLabel: { fontSize: 11, formatter: (v) => formatValue(v, true) } },
    series,
  }
})

function shortName(m) {
  return m.replace('claude-', '').replace('-20', ' ').slice(0, 16)
}

function formatValue(v, axis) {
  if (metric.value === 'cost') return '¥' + Number(v).toFixed(axis ? 2 : 4)
  if (metric.value === 'tokens') {
    if (v >= 10000) return (v / 1000).toFixed(1) + 'K'
    return v.toString()
  }
  return v.toString()
}
</script>

<style scoped>
.data-card {
  background: #fff;
  border-radius: 16px;
  padding: 16px;
  margin-bottom: 16px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.06);
}
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  flex-wrap: wrap;
  gap: 8px;
}
.card-title { font-size: 15px; font-weight: 600; }
.metric-tabs { display: flex; gap: 4px; }
.tab {
  background: #f3f4f6;
  border: none;
  border-radius: 6px;
  padding: 4px 10px;
  font-size: 12px;
  color: #6b7280;
  cursor: pointer;
}
.tab.active { background: #667eea; color: #fff; }
.tab:hover:not(.active) { background: #e5e7eb; }
.empty-tip, .loading-tip {
  text-align: center;
  color: #9ca3af;
  padding: 60px 0;
  font-size: 14px;
}
</style>
