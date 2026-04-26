<template>
  <div class="dashboard">
    <!-- 统计卡片网格 -->
    <div class="stats-grid">
      <div class="stat-card stat-users">
        <div class="stat-icon">👥</div>
        <div class="stat-label">总用户</div>
        <div class="stat-value">{{ stats.total_users || 0 }}</div>
        <div class="stat-sub">活跃 {{ stats.active_users || 0 }}</div>
      </div>
      <div class="stat-card stat-requests">
        <div class="stat-icon">📡</div>
        <div class="stat-label">请求次数</div>
        <div class="stat-value">{{ fmtNum(stats.total_requests) }}</div>
        <div class="stat-sub">今日 {{ fmtNum(stats.today_requests) }}</div>
      </div>
      <div class="stat-card stat-revenue">
        <div class="stat-icon">💰</div>
        <div class="stat-label">总收入</div>
        <div class="stat-value">¥{{ Number(stats.total_revenue || 0).toFixed(2) }}</div>
        <div class="stat-sub">今日 ¥{{ Number(stats.today_revenue || 0).toFixed(2) }}</div>
      </div>
      <div class="stat-card stat-channels">
        <div class="stat-icon">🔌</div>
        <div class="stat-label">上游渠道</div>
        <div class="stat-value">{{ stats.online_channels || 0 }}/{{ stats.total_channels || 0 }}</div>
        <div class="stat-sub">模型 {{ stats.total_models || 0 }}</div>
      </div>
    </div>

    <!-- 系统状态 -->
    <div class="data-card">
      <div class="card-header">
        <span class="card-title">⚙️ 系统状态</span>
      </div>
      <div class="status-list">
        <div class="status-item">
          <span class="status-dot ok"></span>
          <span class="status-name">后端服务</span>
          <span class="status-val">运行中</span>
        </div>
        <div class="status-item">
          <span class="status-dot ok"></span>
          <span class="status-name">数据库</span>
          <span class="status-val">健康</span>
        </div>
        <div class="status-item">
          <span class="status-dot" :class="(stats.online_channels || 0) > 0 ? 'ok' : 'fail'"></span>
          <span class="status-name">上游通道</span>
          <span class="status-val">
            {{ stats.online_channels || 0 }} / {{ stats.total_channels || 0 }} 健康
          </span>
        </div>
        <div class="status-item">
          <span class="status-dot" :class="(stats.pending_orders || 0) > 0 ? 'warn' : 'ok'"></span>
          <span class="status-name">待处理订单</span>
          <span class="status-val">{{ stats.pending_orders || 0 }}</span>
        </div>
      </div>
    </div>

    <!-- 快速操作 -->
    <div class="data-card">
      <div class="card-header">
        <span class="card-title">⚡ 快速操作</span>
      </div>
      <div class="quick-grid">
        <div class="quick-btn" @click="$router.push('/profit')">
          <div class="quick-emoji">💰</div>
          <div class="quick-text">利润看板</div>
        </div>
        <div class="quick-btn" @click="$router.push('/users')">
          <div class="quick-emoji">👥</div>
          <div class="quick-text">用户管理</div>
        </div>
        <div class="quick-btn" @click="$router.push('/channels')">
          <div class="quick-emoji">🔌</div>
          <div class="quick-text">上游渠道</div>
        </div>
        <div class="quick-btn" @click="$router.push('/models')">
          <div class="quick-emoji">🤖</div>
          <div class="quick-text">模型管理</div>
        </div>
        <div class="quick-btn" @click="$router.push('/logs')">
          <div class="quick-emoji">📝</div>
          <div class="quick-text">请求日志</div>
        </div>
        <div class="quick-btn" @click="$router.push('/recharge')">
          <div class="quick-emoji">💳</div>
          <div class="quick-text">充值记录</div>
        </div>
        <div class="quick-btn" @click="$router.push('/audit-logs')">
          <div class="quick-emoji">🔍</div>
          <div class="quick-text">操作日志</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { dashboardAPI } from '@/utils/api'

const stats = ref({})

const fmtNum = (n) => {
  if (!n) return '0'
  if (n >= 1e6) return (n / 1e6).toFixed(2) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K'
  return n.toString()
}

onMounted(async () => {
  try {
    stats.value = await dashboardAPI.stats()
  } catch {}
})
</script>

<style scoped>
.dashboard { padding-bottom: 20px; }

/* === 统计卡片网格（响应式） === */
.stats-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-bottom: 16px;
}
@media (min-width: 768px) {
  .stats-grid { grid-template-columns: repeat(4, 1fr); }
}

.stat-card {
  position: relative;
  border-radius: 16px;
  padding: 16px;
  color: #fff;
  min-height: 120px;
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.08);
  overflow: hidden;
}
.stat-users    { background: linear-gradient(135deg, #667eea, #764ba2); }
.stat-requests { background: linear-gradient(135deg, #11998e, #38ef7d); }
.stat-revenue  { background: linear-gradient(135deg, #fa709a, #fee140); }
.stat-channels { background: linear-gradient(135deg, #4facfe, #00f2fe); }

.stat-icon {
  position: absolute;
  top: 10px;
  right: 12px;
  font-size: 26px;
  opacity: 0.7;
}
.stat-label {
  font-size: 12px;
  opacity: 0.92;
  margin-bottom: 4px;
}
.stat-value {
  font-size: 24px;
  font-weight: 800;
  letter-spacing: -0.5px;
  line-height: 1.15;
  margin-bottom: 6px;
}
.stat-sub {
  font-size: 11px;
  opacity: 0.85;
}

/* === 通用卡片 === */
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
  margin-bottom: 14px;
}
.card-title {
  font-size: 15px;
  font-weight: 600;
  color: #1f2937;
}

/* === 系统状态列表 === */
.status-list { display: flex; flex-direction: column; }
.status-item {
  display: flex;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid #f3f4f6;
  gap: 10px;
}
.status-item:last-child { border-bottom: none; }
.status-dot {
  width: 8px; height: 8px; border-radius: 50%;
  flex-shrink: 0;
}
.status-dot.ok { background: #10b981; box-shadow: 0 0 0 3px rgba(16,185,129,0.15); }
.status-dot.fail { background: #ef4444; box-shadow: 0 0 0 3px rgba(239,68,68,0.15); }
.status-dot.warn { background: #f59e0b; box-shadow: 0 0 0 3px rgba(245,158,11,0.15); }
.status-name {
  flex: 1;
  font-size: 14px;
  color: #374151;
  font-weight: 500;
}
.status-val {
  font-size: 13px;
  color: #6b7280;
}

/* === 快速操作网格 === */
.quick-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
@media (min-width: 600px) {
  .quick-grid { grid-template-columns: repeat(3, 1fr); }
}
@media (min-width: 900px) {
  .quick-grid { grid-template-columns: repeat(4, 1fr); }
}

.quick-btn {
  background: #f9fafb;
  border: 1px solid #f3f4f6;
  border-radius: 12px;
  padding: 16px 10px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  transition: all 0.15s;
}
.quick-btn:active {
  transform: scale(0.96);
  background: #f3f4f6;
}
.quick-emoji { font-size: 26px; }
.quick-text {
  font-size: 13px;
  color: #4b5563;
  font-weight: 600;
}
</style>
