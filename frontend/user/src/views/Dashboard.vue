<template>
  <div class="dashboard">
    <!-- 大渐变余额卡 -->
    <div class="balance-card">
      <div class="balance-bg-shape"></div>
      <div class="balance-header">
        <span class="balance-emoji">💰</span>
        <span class="balance-tip">当前余额</span>
      </div>
      <div class="balance-amount">¥{{ stats.balance?.toFixed(4) ?? '0.0000' }}</div>
      <div class="balance-stats">
        <div class="stat-item">
          <div class="stat-label">本月消费</div>
          <div class="stat-value">¥{{ stats.month_spent?.toFixed(2) ?? '0.00' }}</div>
        </div>
        <div class="stat-divider"></div>
        <div class="stat-item">
          <div class="stat-label">本月请求</div>
          <div class="stat-value">{{ stats.month_requests ?? 0 }}</div>
        </div>
        <div class="stat-divider"></div>
        <div class="stat-item">
          <div class="stat-label">累计消费</div>
          <div class="stat-value">¥{{ stats.total_spent?.toFixed(2) ?? '0.00' }}</div>
        </div>
      </div>
    </div>

    <!-- 快捷操作 -->
    <div class="quick-grid">
      <div class="quick-btn quick-key" @click="$router.push('/api-keys')">
        <div class="quick-icon">🔑</div>
        <div class="quick-text">
          <div class="quick-title">创建 API Key</div>
          <div class="quick-sub">立即获取 API 密钥</div>
        </div>
      </div>
      <div class="quick-btn quick-recharge" @click="$router.push('/recharge')">
        <div class="quick-icon">💳</div>
        <div class="quick-text">
          <div class="quick-title">立即充值</div>
          <div class="quick-sub">支付宝快捷充值</div>
        </div>
      </div>
      <div class="quick-btn quick-play" @click="$router.push('/playground')">
        <div class="quick-icon">🎮</div>
        <div class="quick-text">
          <div class="quick-title">在线测试</div>
          <div class="quick-sub">Playground 体验</div>
        </div>
      </div>
      <div class="quick-btn quick-doc" @click="$router.push('/api-docs')">
        <div class="quick-icon">📖</div>
        <div class="quick-text">
          <div class="quick-title">API 文档</div>
          <div class="quick-sub">接入指南</div>
        </div>
      </div>
    </div>

    <!-- 最近请求 -->
    <div class="data-card">
      <div class="card-header">
        <span class="card-title">📡 最近请求</span>
        <span class="card-link" @click="$router.push('/billing')">查看全部 ›</span>
      </div>
      <div v-if="loading" class="empty-tip">加载中...</div>
      <div v-else-if="!stats.recent_requests || stats.recent_requests.length === 0" class="empty-tip">
        暂无请求记录
      </div>
      <div v-else class="record-list">
        <div v-for="(r, i) in stats.recent_requests" :key="i" class="record-item">
          <div class="record-left">
            <div class="record-model">{{ r.model_name }}</div>
            <div class="record-meta">{{ r.total_tokens }} tokens</div>
          </div>
          <div class="record-right">
            <div class="record-cost">−¥{{ Number(r.cost || 0).toFixed(6) }}</div>
            <span class="record-status" :class="r.status_code === 200 ? 'ok' : 'fail'">
              {{ r.status_code === 200 ? '成功' : '失败' }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- 最近账单 -->
    <div class="data-card">
      <div class="card-header">
        <span class="card-title">📋 最近账单</span>
        <span class="card-link" @click="$router.push('/billing')">查看全部 ›</span>
      </div>
      <div v-if="loading" class="empty-tip">加载中...</div>
      <div v-else-if="!stats.recent_billing || stats.recent_billing.length === 0" class="empty-tip">
        暂无账单记录
      </div>
      <div v-else class="record-list">
        <div v-for="(b, i) in stats.recent_billing" :key="i" class="record-item">
          <div class="record-left">
            <div class="bill-type">
              <span class="bill-tag" :class="b.type === 'recharge' ? 'tag-in' : 'tag-out'">
                {{ b.type === 'recharge' ? '充值' : '消费' }}
              </span>
            </div>
            <div class="record-meta">{{ b.description || '-' }}</div>
          </div>
          <div class="bill-amount" :class="b.amount > 0 ? 'income' : 'outcome'">
            {{ b.amount > 0 ? '+' : '' }}¥{{ Number(b.amount || 0).toFixed(4) }}
          </div>
        </div>
      </div>
    </div>
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

<style scoped>
.dashboard { padding-bottom: 20px; }

/* 大渐变余额卡 */
.balance-card {
  position: relative;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-radius: 20px;
  padding: 22px 20px;
  color: #fff;
  margin-bottom: 16px;
  box-shadow: 0 10px 30px rgba(102, 126, 234, 0.35);
  overflow: hidden;
}
.balance-bg-shape {
  position: absolute;
  top: -40px;
  right: -40px;
  width: 160px;
  height: 160px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 50%;
}
.balance-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
  position: relative;
  z-index: 1;
}
.balance-emoji { font-size: 16px; }
.balance-tip { font-size: 13px; opacity: 0.9; }
.balance-amount {
  font-size: 36px;
  font-weight: 800;
  letter-spacing: -1px;
  margin-bottom: 18px;
  position: relative;
  z-index: 1;
}
.balance-stats {
  display: flex;
  align-items: center;
  background: rgba(255, 255, 255, 0.15);
  border-radius: 12px;
  padding: 12px;
  position: relative;
  z-index: 1;
}
.stat-item {
  flex: 1;
  text-align: center;
}
.stat-divider {
  width: 1px;
  height: 24px;
  background: rgba(255, 255, 255, 0.25);
}
.stat-label {
  font-size: 11px;
  opacity: 0.85;
  margin-bottom: 4px;
}
.stat-value {
  font-size: 15px;
  font-weight: 700;
}

/* 快捷操作网格 */
.quick-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-bottom: 16px;
}
.quick-btn {
  background: #fff;
  border-radius: 14px;
  padding: 14px;
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  transition: all 0.2s ease;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
  border: 1px solid #f3f4f6;
}
.quick-btn:active {
  transform: scale(0.97);
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.04);
}
.quick-icon {
  width: 40px;
  height: 40px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  flex-shrink: 0;
}
.quick-key      .quick-icon { background: linear-gradient(135deg, #667eea22, #764ba222); }
.quick-recharge .quick-icon { background: linear-gradient(135deg, #11998e22, #38ef7d22); }
.quick-play     .quick-icon { background: linear-gradient(135deg, #f093fb22, #f5576c22); }
.quick-doc      .quick-icon { background: linear-gradient(135deg, #4facfe22, #00f2fe22); }
.quick-text { flex: 1; min-width: 0; }
.quick-title {
  font-size: 14px;
  font-weight: 600;
  color: #1f2937;
  margin-bottom: 2px;
}
.quick-sub {
  font-size: 11px;
  color: #9ca3af;
}

/* 数据卡片 */
.data-card {
  background: #fff;
  border-radius: 14px;
  padding: 16px;
  margin-bottom: 14px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
.card-title {
  font-size: 15px;
  font-weight: 600;
  color: #1f2937;
}
.card-link {
  font-size: 12px;
  color: #667eea;
  cursor: pointer;
}
.empty-tip {
  text-align: center;
  color: #9ca3af;
  padding: 24px 0;
  font-size: 13px;
}

/* 记录列表 */
.record-list { display: flex; flex-direction: column; }
.record-item {
  display: flex;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid #f3f4f6;
}
.record-item:last-child { border-bottom: none; }
.record-left { flex: 1; min-width: 0; }
.record-model {
  font-size: 13px;
  font-weight: 600;
  color: #1f2937;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.record-meta {
  font-size: 11px;
  color: #9ca3af;
  margin-top: 2px;
}
.record-right {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
}
.record-cost {
  font-size: 13px;
  font-weight: 600;
  color: #ef4444;
}
.record-status {
  font-size: 10px;
  padding: 2px 7px;
  border-radius: 8px;
}
.record-status.ok { background: #d1fae5; color: #065f46; }
.record-status.fail { background: #fee2e2; color: #991b1b; }

/* 账单 */
.bill-tag {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 8px;
  font-size: 11px;
  font-weight: 600;
  margin-bottom: 2px;
}
.tag-in { background: #d1fae5; color: #065f46; }
.tag-out { background: #fef3c7; color: #92400e; }
.bill-amount {
  font-size: 15px;
  font-weight: 700;
}
.bill-amount.income { color: #10b981; }
.bill-amount.outcome { color: #ef4444; }
</style>
