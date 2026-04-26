<template>
  <div class="page">
    <!-- 充值卡 -->
    <div class="recharge-hero">
      <div class="hero-bg-shape"></div>
      <div class="hero-emoji">💳</div>
      <div class="hero-title">余额充值</div>
      <div class="hero-sub">支付宝快捷充值，到账秒级</div>
    </div>

    <div class="data-card">
      <div class="card-header"><span class="card-title">💰 选择金额</span></div>
      <div class="amount-grid">
        <div
          v-for="amt in presetAmounts"
          :key="amt"
          class="amount-chip"
          :class="{ active: rechargeForm.amount === amt }"
          @click="rechargeForm.amount = amt"
        >
          ¥{{ amt }}
        </div>
      </div>
      <div class="form-row">
        <label class="form-label">自定义金额 (¥)</label>
        <el-input-number v-model="rechargeForm.amount" :min="1" :max="100000" :step="10" size="large" style="width:100%" />
      </div>
      <div class="form-row" style="margin-top:14px">
        <label class="form-label">支付方式</label>
        <div class="pay-list">
          <div class="pay-item active">
            <span class="pay-icon">💙</span>
            <div class="pay-meta">
              <div class="pay-name">支付宝</div>
              <div class="pay-desc">推荐 · 即时到账</div>
            </div>
            <span class="pay-check">✓</span>
          </div>
          <div class="pay-item disabled">
            <span class="pay-icon">💳</span>
            <div class="pay-meta">
              <div class="pay-name">Stripe</div>
              <div class="pay-desc">即将支持</div>
            </div>
          </div>
        </div>
      </div>
      <button class="primary-btn" :disabled="submitting" @click="handleRecharge" style="margin-top:18px">
        <span v-if="submitting">处理中...</span>
        <span v-else>立即支付 ¥{{ rechargeForm.amount }}</span>
      </button>
      <div class="form-tip">充值金额将立即计入账户余额，由支付宝安全处理</div>
    </div>

    <!-- 充值记录 -->
    <div class="data-card">
      <div class="card-header"><span class="card-title">📜 充值记录</span></div>
      <div v-if="loadingOrders" class="empty-tip">加载中...</div>
      <div v-else-if="orders.length === 0" class="empty-tip">暂无充值记录</div>
      <div v-else class="order-list">
        <div v-for="o in orders" :key="o.order_no" class="order-item">
          <div class="order-left">
            <div class="order-amount">¥{{ o.amount?.toFixed(2) }}</div>
            <div class="order-meta">{{ dayjs(o.created_at).format('YYYY-MM-DD HH:mm') }}</div>
            <div class="order-no">{{ o.order_no }}</div>
          </div>
          <span class="order-status" :class="o.payment_status">
            {{ statusLabel(o.payment_status) }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { rechargeAPI } from '@/utils/api'
import dayjs from 'dayjs'

const submitting = ref(false)
const loadingOrders = ref(true)
const orders = ref([])
const presetAmounts = [10, 50, 100, 200, 500, 1000]
const rechargeForm = reactive({ amount: 100, method: 'alipay' })

onMounted(fetchOrders)

async function fetchOrders() {
  loadingOrders.value = true
  try {
    const data = await rechargeAPI.listOrders()
    orders.value = data.orders || []
  } catch {} finally { loadingOrders.value = false }
}

async function handleRecharge() {
  if (rechargeForm.amount < 1) return ElMessage.warning('充值金额至少 ¥1')
  submitting.value = true
  try {
    const data = await rechargeAPI.createOrder(rechargeForm.amount)
    if (data.pay_url) window.open(data.pay_url, '_blank')
    ElMessage.success('订单已创建，正在跳转支付...')
    await fetchOrders()
  } catch {} finally { submitting.value = false }
}

function statusLabel(s) {
  const map = { pending: '待支付', paid: '已支付', failed: '失败', refunded: '已退款' }
  return map[s] || s
}
</script>

<style scoped>
.page { padding-bottom: 20px; }
.recharge-hero {
  position: relative;
  background: linear-gradient(135deg, #11998e, #38ef7d);
  border-radius: 20px;
  padding: 24px 20px;
  color: #fff;
  margin-bottom: 14px;
  text-align: center;
  box-shadow: 0 10px 30px rgba(17,153,142,0.3);
  overflow: hidden;
}
.hero-bg-shape {
  position: absolute;
  top: -40px;
  right: -40px;
  width: 140px;
  height: 140px;
  background: rgba(255,255,255,0.1);
  border-radius: 50%;
}
.hero-emoji { font-size: 36px; margin-bottom: 6px; position: relative; z-index: 1; }
.hero-title { font-size: 22px; font-weight: 800; position: relative; z-index: 1; }
.hero-sub { font-size: 13px; opacity: 0.9; margin-top: 4px; position: relative; z-index: 1; }

.data-card {
  background: #fff;
  border-radius: 14px;
  padding: 16px;
  margin-bottom: 14px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.04);
}
.card-header { display: flex; justify-content: space-between; margin-bottom: 14px; }
.card-title { font-size: 15px; font-weight: 600; color: #1f2937; }

.amount-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
  margin-bottom: 14px;
}
.amount-chip {
  background: #f3f4f6;
  text-align: center;
  padding: 14px 0;
  border-radius: 12px;
  font-weight: 700;
  color: #4b5563;
  cursor: pointer;
  border: 2px solid transparent;
  transition: all 0.15s;
}
.amount-chip:active { transform: scale(0.96); }
.amount-chip.active {
  background: linear-gradient(135deg, #667eea22, #764ba222);
  border-color: #667eea;
  color: #667eea;
}

.form-row { display: flex; flex-direction: column; gap: 6px; }
.form-label { font-size: 13px; color: #4b5563; font-weight: 500; }
.form-tip { font-size: 11px; color: #9ca3af; text-align: center; margin-top: 8px; }

.pay-list { display: flex; flex-direction: column; gap: 8px; }
.pay-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px;
  border-radius: 12px;
  border: 2px solid #f3f4f6;
  background: #fff;
}
.pay-item.active {
  border-color: #667eea;
  background: linear-gradient(135deg, #667eea08, #764ba208);
}
.pay-item.disabled { opacity: 0.5; }
.pay-icon { font-size: 22px; }
.pay-meta { flex: 1; }
.pay-name { font-size: 14px; font-weight: 600; color: #1f2937; }
.pay-desc { font-size: 11px; color: #9ca3af; margin-top: 2px; }
.pay-check { color: #667eea; font-weight: 700; font-size: 18px; }

.primary-btn {
  background: linear-gradient(135deg, #11998e, #38ef7d);
  color: #fff;
  border: none;
  height: 48px;
  border-radius: 12px;
  font-size: 16px;
  font-weight: 700;
  cursor: pointer;
  width: 100%;
  box-shadow: 0 4px 12px rgba(17,153,142,0.3);
  transition: transform 0.15s;
}
.primary-btn:active { transform: scale(0.98); }
.primary-btn:disabled { opacity: 0.6; }

.empty-tip { text-align: center; color: #9ca3af; padding: 30px 0; font-size: 13px; }
.order-list { display: flex; flex-direction: column; }
.order-item {
  display: flex;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid #f3f4f6;
}
.order-item:last-child { border-bottom: none; }
.order-left { flex: 1; min-width: 0; }
.order-amount { font-size: 16px; font-weight: 700; color: #10b981; }
.order-meta { font-size: 12px; color: #6b7280; margin-top: 2px; }
.order-no { font-size: 10px; color: #9ca3af; font-family: monospace; margin-top: 2px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 220px; }
.order-status {
  padding: 4px 12px;
  border-radius: 10px;
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
}
.order-status.paid { background: #d1fae5; color: #065f46; }
.order-status.pending { background: #fef3c7; color: #92400e; }
.order-status.failed { background: #fee2e2; color: #991b1b; }
.order-status.refunded { background: #e0e7ff; color: #3730a3; }
</style>
