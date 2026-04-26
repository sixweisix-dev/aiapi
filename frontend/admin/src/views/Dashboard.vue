<template>
  <div>
    <!-- Stats cards -->
    <el-row :gutter="16" class="mb-6">
      <el-col :span="6" v-for="card in statsCards" :key="card.label">
        <el-card shadow="hover" class="stat-card">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm text-gray-500">{{ card.label }}</p>
              <p class="text-2xl font-bold mt-1">{{ card.value }}</p>
              <p class="text-xs text-gray-400 mt-1">
                <template v-if="card.trend !== undefined">
                  今日: {{ card.trend }}
                </template>
              </p>
            </div>
            <el-icon :size="40" :color="card.color">
              <component :is="card.icon" />
            </el-icon>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Charts row -->
    <el-row :gutter="16" class="mb-6">
      <el-col :span="14">
        <el-card shadow="hover">
          <template #header><span class="font-medium">请求概览</span></template>
          <div style="height: 300px">
            <v-chart :option="requestChartOption" autoresize />
          </div>
        </el-card>
      </el-col>
      <el-col :span="10">
        <el-card shadow="hover">
          <template #header><span class="font-medium">快速操作</span></template>
          <div class="space-y-3">
            <el-button class="w-full justify-start" text @click="$router.push('/users')">
              <el-icon><User /></el-icon> 用户管理
            </el-button>
            <el-button class="w-full justify-start" text @click="$router.push('/channels')">
              <el-icon><Connection /></el-icon> 上游渠道
            </el-button>
            <el-button class="w-full justify-start" text @click="$router.push('/models')">
              <el-icon><Grid /></el-icon> 模型管理
            </el-button>
            <el-button class="w-full justify-start" text @click="$router.push('/logs')">
              <el-icon><Document /></el-icon> 请求日志
            </el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { dashboardAPI } from '@/utils/api'

const stats = ref({
  total_users: 0,
  active_users: 0,
  total_requests: 0,
  today_requests: 0,
  total_revenue: 0,
  today_revenue: 0,
  total_channels: 0,
  online_channels: 0,
  total_models: 0,
  pending_orders: 0,
})

const statsCards = computed(() => [
  { label: '总用户', value: stats.value.total_users, trend: `活跃 ${stats.value.active_users}`, color: '#409EFF', icon: 'User' },
  { label: '请求次数', value: stats.value.total_requests.toLocaleString(), trend: `今日 ${stats.value.today_requests.toLocaleString()}`, color: '#67C23A', icon: 'Finished' },
  { label: '总收入', value: `¥${stats.value.total_revenue.toFixed(2)}`, trend: `今日 ¥${stats.value.today_revenue.toFixed(2)}`, color: '#E6A23C', icon: 'Coin' },
  { label: '上游渠道', value: `${stats.value.online_channels}/${stats.value.total_channels}`, trend: `模型 ${stats.value.total_models}`, color: '#909399', icon: 'Connection' },
])

const requestChartOption = computed(() => ({
  tooltip: { trigger: 'axis' },
  grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
  xAxis: { type: 'category', data: ['近7日'], boundaryGap: false },
  yAxis: { type: 'value' },
  series: [
    {
      name: '请求量',
      type: 'line',
      smooth: true,
      data: [stats.value.today_requests || 0],
      areaStyle: { opacity: 0.3 },
      lineStyle: { width: 2 },
      itemStyle: { color: '#409EFF' },
    },
  ],
}))

onMounted(async () => {
  try {
    stats.value = await dashboardAPI.stats()
  } catch { /* handled by interceptor */ }
})
</script>

<style scoped>
.stat-card :deep(.el-card__body) {
  padding: 20px;
}
</style>
