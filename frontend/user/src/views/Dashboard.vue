<template>
  <div>
    <!-- Balance Card -->
    <div style="background:linear-gradient(135deg,#3b82f6,#6366f1);border-radius:16px;padding:24px;color:white;margin-bottom:16px;">
      <p style="font-size:13px;opacity:0.8;margin-bottom:4px;">当前余额</p>
      <p style="font-size:32px;font-weight:700;margin-bottom:16px;">¥{{ stats.balance?.toFixed(4) ?? '0.0000' }}</p>
      <div style="display:flex;gap:24px;">
        <div>
          <p style="font-size:12px;opacity:0.7;">本月消费</p>
          <p style="font-size:16px;font-weight:600;">¥{{ stats.month_spent?.toFixed(4) ?? '0.0000' }}</p>
        </div>
        <div>
          <p style="font-size:12px;opacity:0.7;">本月请求</p>
          <p style="font-size:16px;font-weight:600;">{{ stats.month_requests ?? 0 }}</p>
        </div>
        <div>
          <p style="font-size:12px;opacity:0.7;">累计消费</p>
          <p style="font-size:16px;font-weight:600;">¥{{ stats.total_spent?.toFixed(4) ?? '0.0000' }}</p>
        </div>
      </div>
    </div>

    <!-- Quick Actions -->
    <div style="display:grid;grid-template-columns:1fr 1fr;gap:10px;margin-bottom:16px;">
      <el-button type="primary" size="large" style="height:48px;border-radius:12px;" @click="$router.push('/api-keys')">
        创建 API Key
      </el-button>
      <el-button type="success" size="large" style="height:48px;border-radius:12px;" @click="$router.push('/recharge')">
        立即充值
      </el-button>
      <el-button size="large" style="height:48px;border-radius:12px;" @click="$router.push('/playground')">
        在线测试
      </el-button>
      <el-button size="large" style="height:48px;border-radius:12px;" @click="$router.push('/api-docs')">
        API 文档
      </el-button>
    </div>

    <!-- Recent Requests -->
    <el-card shadow="never" style="border-radius:12px;margin-bottom:16px;">
      <template #header><span style="font-weight:600;">最近请求</span></template>
      <el-table :data="stats.recent_requests || []" size="small" v-loading="loading" empty-text="暂无记录">
        <el-table-column prop="model_name" label="模型" min-width="100" />
        <el-table-column prop="total_tokens" label="Tokens" width="70" />
        <el-table-column label="费用" width="80">
          <template #default="{ row }">
            <span style="color:#ef4444;">-{{ row.cost?.toFixed(6) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="60">
          <template #default="{ row }">
            <el-tag :type="row.status_code === 200 ? 'success' : 'danger'" size="small">{{ row.status_code }}</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Recent Billing -->
    <el-card shadow="never" style="border-radius:12px;">
      <template #header><span style="font-weight:600;">最近账单</span></template>
      <el-table :data="stats.recent_billing || []" size="small" v-loading="loading" empty-text="暂无记录">
        <el-table-column label="类型" width="60">
          <template #default="{ row }">
            <el-tag :type="row.type === 'recharge' ? 'success' : 'warning'" size="small">
              {{ row.type === 'recharge' ? '充值' : '消费' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="金额" width="90">
          <template #default="{ row }">
            <span :style="row.amount > 0 ? 'color:#22c55e' : 'color:#ef4444'">
              {{ row.amount > 0 ? '+' : '' }}{{ row.amount?.toFixed(4) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="100" show-overflow-tooltip />
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { dashboardAPI } from '@/utils/api'

const loading = ref(true)
const stats = ref({})

onMounted(async () => {
  try { stats.value = await dashboardAPI.stats() } catch {}
  finally { loading.value = false }
})
</script>
