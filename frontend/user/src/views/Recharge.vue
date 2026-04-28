<template>
  <div class="page">
    <!-- 充值卡 -->
    <div class="recharge-hero">
      <div class="hero-bg-shape"></div>
      <div class="hero-emoji">💳</div>
      <div class="hero-title">余额充值</div>
      <div class="hero-sub">支付宝快捷充值，到账秒级</div>
    </div>

    <!-- 会员套餐 -->
    <div class="data-card">
      <div class="card-header"><span class="card-title">⭐ 会员套餐</span></div>
      <div class="plan-grid">
        <div
          v-for="p in memberPlans"
          :key="p.value"
          class="plan-card"
          :class="[`plan-${p.tier}`, { active: rechargeForm.amount === p.value }]"
          @click="rechargeForm.amount = p.value"
        >
          <div class="plan-name">{{ p.name }}</div>
          <div class="plan-price">{{ p.label }}</div>
          <div class="plan-bonus">到账 ¥{{ p.bonus }}</div>
          <div class="plan-period">{{ p.period }}</div>
        </div>
      </div>
    </div>

    <!-- 充值优惠规则 -->
    <div v-if="promo.enabled && (promo.tiers.length || promo.firstBonus > 0)" class="data-card promo-card" :class="{ 'promo-disabled': isMembershipAmount }">
      <div v-if="isMembershipAmount" class="membership-overlay">
        ⭐ 会员套餐独立计算，享专属套餐福利
      </div>
      <div class="card-header">
        <span class="card-title">🎁 充值优惠</span>
        <span v-if="isFirstRecharge" class="first-badge">首充专享</span>
      </div>
      <div v-if="promo.tiers.length" class="promo-tiers">
        <div v-for="t in promo.tiers" :key="t.min" class="promo-tier" :class="{ active: rechargeForm.amount >= t.min }">
          <span class="tier-min">充 ¥{{ t.min }}</span>
          <span class="tier-arrow">→</span>
          <span class="tier-bonus">额外送 ¥{{ t.bonus }}</span>
        </div>
      </div>
      <div v-if="promo.firstBonus > 0 && isFirstRecharge" class="first-bonus-tip">
        🎉 新人首充再额外加赠 <b>¥{{ promo.firstBonus }}</b>（仅限第一次）
      </div>
      <div v-if="promo.firstBonus > 0 && !isFirstRecharge" class="first-bonus-used">
        新人首充礼已使用过
      </div>
    </div>

    <div class="data-card">
      <div class="card-header"><span class="card-title">💰 选择金额</span></div>
      <div class="amount-grid">
        <div
          v-for="item in presetAmounts"
          :key="item.value"
          class="amount-chip"
          :class="{ active: rechargeForm.amount === item.value }"
          @click="rechargeForm.amount = item.value"
        >{{ item.label }}</div>
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
      <div v-if="totalBonus > 0" class="bonus-preview">
        ✨ 实际到账 <b>¥{{ (rechargeForm.amount + totalBonus).toFixed(2) }}</b>
        <span class="bonus-detail">（本金 ¥{{ rechargeForm.amount }} + 赠送 ¥{{ totalBonus.toFixed(2) }}）</span>
      </div>
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
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { rechargeAPI } from '@/utils/api'
import dayjs from 'dayjs'
import api from '@/utils/api'

const submitting = ref(false)
const loadingOrders = ref(true)
const orders = ref([])
const promo = reactive({ enabled: true, tiers: [], firstBonus: 0 })

const isFirstRecharge = computed(() => {
  return !orders.value.some(o => o.payment_status === 'paid')
})

const isMembershipAmount = computed(() => {
  return rechargeForm.amount === 99 || rechargeForm.amount === 499
})

const totalBonus = computed(() => {
  if (!promo.enabled) return 0
  if (rechargeForm.amount === 99 || rechargeForm.amount === 499) return 0
  let tierBonus = 0
  for (const t of promo.tiers) {
    if (rechargeForm.amount >= t.min && t.bonus > tierBonus) tierBonus = t.bonus
  }
  const firstBonus = isFirstRecharge.value ? (promo.firstBonus || 0) : 0
  return tierBonus + firstBonus
})

async function loadPromo() {
  try {
    const cfg = await api.get('/auth/config')
    promo.enabled = cfg.recharge_promo_enabled !== false
    promo.firstBonus = parseFloat(cfg.first_recharge_bonus || '0')
    try {
      const arr = JSON.parse(cfg.recharge_tiers || '[]')
      promo.tiers = Array.isArray(arr) ? arr.map(t => ({ min: Number(t.min), bonus: Number(t.bonus) })).sort((a,b) => a.min - b.min) : []
    } catch { promo.tiers = [] }
  } catch {}
}
const presetAmounts = [
  { value: 10,  label: '¥10' },
  { value: 50,  label: '¥50' },
  { value: 100, label: '¥100' },
  { value: 200, label: '¥200' },
  { value: 500, label: '¥500' },
  { value: 1000, label: '¥1000' },
]
const memberPlans = [
  { value: 99,  label: '¥99',  tier: 'pro',        bonus: 120, name: '专业版', period: '1 个月' },
  { value: 499, label: '¥499', tier: 'enterprise', bonus: 600, name: '企业版', period: '1 个月' },
]
const rechargeForm = reactive({ amount: 100, method: 'alipay' })

onMounted(() => { fetchOrders(); loadPromo() })

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

.promo-card { background: linear-gradient(135deg, #fff7ed, #ffedd5); border: 1px solid #fed7aa; }
.promo-tiers { display: grid; gap: 8px; }
.promo-tier {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 12px; border-radius: 10px;
  background: rgba(255,255,255,0.6);
  font-size: 14px; transition: all 0.2s;
}
.promo-tier.active {
  background: linear-gradient(135deg, #f97316, #ea580c);
  color: #fff; transform: scale(1.02);
  box-shadow: 0 4px 12px rgba(249,115,22,0.3);
}
.tier-min { font-weight: 600; min-width: 70px; }
.tier-arrow { opacity: 0.5; }
.tier-bonus { color: inherit; font-weight: 600; }
.promo-tier.active .tier-bonus { color: #fff; }
.first-badge {
  background: linear-gradient(135deg, #ef4444, #dc2626);
  color: #fff; padding: 2px 10px; border-radius: 12px;
  font-size: 11px; font-weight: 600;
}
.first-bonus-tip {
  margin-top: 12px; padding: 10px 14px;
  background: rgba(239,68,68,0.08); color: #b91c1c;
  border-radius: 10px; font-size: 13px;
}
.first-bonus-used {
  margin-top: 10px; font-size: 12px; color: #9ca3af; text-align: center;
}
.bonus-preview {
  margin-top: 10px; text-align: center; font-size: 13px;
  color: #059669; padding: 8px;
  background: rgba(16,185,129,0.08); border-radius: 8px;
}
.bonus-preview b { font-size: 15px; }
.bonus-detail { color: #6b7280; font-size: 12px; margin-left: 4px; }
.promo-card { position: relative; }
.promo-card.promo-disabled { filter: saturate(0.3) opacity(0.55); pointer-events: none; }
.membership-overlay {
  position: absolute; top: 0; left: 0; right: 0; bottom: 0;
  display: flex; align-items: center; justify-content: center;
  background: rgba(255,255,255,0.55); backdrop-filter: blur(2px);
  border-radius: 14px;
  font-size: 14px; font-weight: 600; color: #92400e;
  z-index: 5; pointer-events: auto;
  text-align: center; padding: 0 20px;
}
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

.amount-chip.chip-pro {
  background: linear-gradient(135deg, #f5f3ff, #ede9fe);
  border: 2px solid #a5b4fc;
}
.amount-chip.chip-pro.active {
  background: linear-gradient(135deg, #6366f1, #8b5cf6);
  color: #fff;
  border-color: #4f46e5;
}
.amount-chip.chip-enterprise {
  background: linear-gradient(135deg, #fef3c7, #fde68a);
  border: 2px solid #f59e0b;
}
.amount-chip.chip-enterprise.active {
  background: linear-gradient(135deg, #f59e0b, #d97706);
  color: #fff;
  border-color: #b45309;
}
.chip-label { font-size: 16px; font-weight: 700; }
.chip-bonus {
  font-size: 11px; margin-top: 2px; opacity: 0.85;
}
.chip-tier-tag {
  font-size: 10px; margin-top: 4px;
  padding: 1px 6px; border-radius: 4px;
  background: rgba(0,0,0,0.08);
  display: inline-block;
}
.amount-chip.chip-pro.active .chip-tier-tag,
.amount-chip.chip-enterprise.active .chip-tier-tag {
  background: rgba(255,255,255,0.25);
}


.plan-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.plan-card {
  border-radius: 14px;
  padding: 16px 12px;
  text-align: center;
  cursor: pointer;
  border: 2px solid transparent;
  transition: all 0.15s;
}
.plan-card:active { transform: scale(0.97); }
.plan-pro {
  background: linear-gradient(135deg, #eef2ff, #e0e7ff);
  color: #4338ca;
}
.plan-pro.active { border-color: #6366f1; box-shadow: 0 4px 14px rgba(99,102,241,0.25); }
.plan-enterprise {
  background: linear-gradient(135deg, #fef3c7, #fde68a);
  color: #92400e;
}
.plan-enterprise.active { border-color: #f59e0b; box-shadow: 0 4px 14px rgba(245,158,11,0.25); }
.plan-name { font-size: 13px; font-weight: 600; opacity: 0.85; }
.plan-price { font-size: 24px; font-weight: 800; margin: 4px 0; letter-spacing: -0.5px; }
.plan-bonus { font-size: 12px; font-weight: 600; }
.plan-period { font-size: 11px; opacity: 0.75; margin-top: 2px; }
</style>
