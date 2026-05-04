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

    <!-- 会员卡片 -->
    <div v-if="stats.membership" class="member-card" :class="`tier-${stats.membership.effective || 'free'}`">
      <div class="member-header">
        <div class="member-info">
          <span class="member-emoji">
            {{ stats.membership.effective === 'enterprise' ? '👑' : stats.membership.effective === 'pro' ? '⭐' : '🌱' }}
          </span>
          <div class="member-text">
            <div class="member-tier">{{ stats.membership.display_name || '免费版' }}</div>
            <div class="member-expires" v-if="stats.membership.expires_at && stats.membership.effective !== 'free'">
              {{ formatExpiry(stats.membership.expires_at) }}
            </div>
            <div class="member-expires" v-else>
              永久免费 · 充值 ¥99 即升级
            </div>
          </div>
        </div>
        <button class="member-upgrade-btn" @click="$router.push('/recharge')">
          {{ stats.membership.effective === 'free' ? '升级' : '续费' }}
        </button>
      </div>

      <div class="member-limits" v-if="stats.membership.limits">
        <div class="limit-item">
          <div class="limit-label">RPM</div>
          <div class="limit-value">{{ stats.membership.limits.RPM || '∞' }}</div>
        </div>
        <div class="limit-item">
          <div class="limit-label">TPM</div>
          <div class="limit-value">{{ formatTPM(stats.membership.limits.TPM) }}</div>
        </div>
        <div class="limit-item">
          <div class="limit-label">最大 Key</div>
          <div class="limit-value">{{ stats.membership.limits.MaxAPIKeys || '∞' }}</div>
        </div>
        <div class="limit-item">
          <div class="limit-label">发票</div>
          <div class="limit-value">{{ stats.membership.limits.InvoiceSupport ? '✓' : '✗' }}</div>
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
          <div class="quick-sub">闲鱼下单兑换</div>
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

    <UsageByModelChart />

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
import UsageByModelChart from '@/components/UsageByModelChart.vue'
import { ref, onMounted } from 'vue'
import { dashboardAPI } from '@/utils/api'

const loading = ref(true)
const stats = ref({})

onMounted(async () => {
  try { stats.value = await dashboardAPI.stats() } catch {}
  finally { loading.value = false }
})

function formatExpiry(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  const now = new Date()
  const days = Math.ceil((d - now) / (1000 * 60 * 60 * 24))
  if (days <= 0) return '已到期'
  if (days <= 7) return `${days} 天后到期`
  return `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')} 到期`
}
function formatTPM(tpm) {
  if (!tpm || tpm === 0) return '∞'
  if (tpm >= 1000000) return (tpm/1000000).toFixed(1).replace(/\.0$/, '') + 'M'
  if (tpm >= 1000) return (tpm/1000) + 'k'
  return tpm
}
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
  margin-top: 14px;
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

/* 会员卡片 */
.member-card {
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 16px;
  padding: 18px 20px;
  margin-top: 14px;
}
.member-card.tier-free {
  background: #fff;
  border-color: #e5e7eb;
}
.member-card.tier-pro {
  background: linear-gradient(135deg, #f5f3ff, #ede9fe);
  border-color: #c7d2fe;
}
.member-card.tier-enterprise {
  background: linear-gradient(135deg, #fef3c7, #fde68a);
  border-color: #f59e0b;
}
.member-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 14px;
}
.member-info { display: flex; align-items: center; gap: 12px; }
.member-emoji { font-size: 28px; }
.member-tier { font-size: 16px; font-weight: 700; color: #1f2937; }
.member-expires { font-size: 12px; color: #6b7280; margin-top: 2px; }
.member-upgrade-btn {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff; border: none; padding: 8px 18px;
  border-radius: 10px; font-size: 13px; font-weight: 600; cursor: pointer;
}
.member-upgrade-btn:active { opacity: 0.85; }
.member-limits {
  display: grid; grid-template-columns: repeat(4, 1fr);
  gap: 8px; padding-top: 12px;
  border-top: 1px solid rgba(0,0,0,0.06);
}
.limit-item { text-align: center; }
.limit-label { font-size: 11px; color: #9ca3af; margin-bottom: 4px; }
.limit-value { font-size: 14px; font-weight: 600; color: #1f2937; }


@media (min-width: 769px) {
  .dashboard {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 20px;
    align-items: stretch;
  }
  /* 余额卡和会员卡左右各一半 */
  .balance-card,
  .member-card {
    grid-column: span 1;
    margin: 0;
  }
  /* 快捷入口跨两列 */
  .quick-grid {
    grid-column: 1 / -1;
  }
  /* 数据卡片(账单+用量)左右各一半 */
  .data-card {
    grid-column: span 1;
    margin: 0;
  }
  /* 余额数字加大 */
  .balance-amount { font-size: 36px; }
  /* 卡片 hover 微动效 */
  .balance-card:hover,
  .member-card:hover,
  .data-card:hover {
    transition: transform 0.2s, box-shadow 0.2s;
    transform: translateY(-2px);
    box-shadow: 0 8px 24px rgba(102,126,234,0.12);
  }
}


@media (min-width: 769px) {
  .balance-card,
  .member-card,
  .data-card {
    width: 100%;
    height: 100%;
    box-sizing: border-box;
  }
  /* 余额卡内部用 flex 撑满 */
  .balance-card {
    display: flex;
    flex-direction: column;
    justify-content: space-between;
  }
  /* 会员卡同理 */
  .member-card {
    display: flex;
    flex-direction: column;
    justify-content: space-between;
  }
}
</style>
