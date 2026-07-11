<template>
  <div class="page">
    <!-- 页头 -->
    <div class="recharge-hero">
      <div class="hero-bg-shape"></div>
      <div class="hero-emoji">💳</div>
      <div class="hero-title">{{ t('recharge.heroTitle') }}</div>
      <div class="hero-sub">{{ t('recharge.heroSub') }}</div>
    </div>

    <!-- 当前等级 -->
    <div v-if="me" class="current-card">
      <div class="current-label">{{ t('recharge.currentTier') }}</div>
      <div class="current-tier">
        <span class="tier-badge" :class="(me.membership?.effective || me.membership?.tier) || 'free'">
          {{ tierLabel(me.membership?.effective || me.membership?.tier) }}
        </span>
      </div>
      <div v-if="me.membership?.expires_at && (me.membership?.effective || me.membership?.tier) !== 'free'" class="current-expire">
        {{ t('recharge.expiresAt') }}：{{ dayjs(me.membership.expires_at).format('YYYY-MM-DD HH:mm') }}
      </div>
    </div>

    <!-- Tab 切换:充值 / 兑换码 -->
    <div class="tab-switch">
      <button :class="{ active: activeTab === 'pay' }" @click="activeTab = 'pay'">💳 {{ t('recharge.stripe.tabPay') }}</button>
      <button :class="{ active: activeTab === 'redeem' }" @click="activeTab = 'redeem'">🎫 {{ t('recharge.stripe.tabRedeem') }}</button>
    </div>

    <!-- 信用卡充值 Tab -->
    <div v-if="activeTab === 'pay'" class="data-card stripe-pay-c1">
      <div class="first-recharge-banner">
        <div class="frb-title">{{ t('recharge.stripe.firstRechargeBonus') }}</div>
        <div class="frb-sub">{{ t('recharge.stripe.firstRechargeBonusSub') }}</div>
      </div>
      <div class="stripe-header">💳 {{ t('recharge.stripe.header') }}</div>
      <div class="stripe-sub">
        <span v-if="stripeEnabled">{{ t('recharge.stripe.sub') }}</span>
        <span v-else style="color:#f56c6c;">{{ t('recharge.stripe.comingSoon') }}</span>
      </div>
      <div class="stripe-section-title">{{ t('recharge.stripe.balanceTitle') }}</div>
      <div class="stripe-tiers">
        <button v-for="item in stripeTiers" :key="item.id"
                class="stripe-tier-btn"
                :class="{ disabled: !stripeEnabled, recommended: item.recommended, selected: selectedTier === item.id }"
                :disabled="!stripeEnabled || stripeLoading"
                @click="selectedTier = item.id">
          <span v-if="item.recommended" class="hot-tag">🔥</span>
          <span class="bonus-tag">+{{ item.bonusPct }}%</span>
          <div class="tier-cny" :class="{ primary: !isEn }">¥{{ item.cny }}</div>
          <div class="tier-usd" :class="{ primary: isEn }">${{ item.usd }}</div>
          <div class="tier-balance">→ ${{ item.balance }}</div>
        </button>

        <button class="stripe-tier-btn membership-card mtier-pro"
                :class="{ disabled: !stripeEnabled, selected: selectedTier === 'pro' }"
                :disabled="!stripeEnabled || stripeLoading"
                @click="selectedTier = 'pro'">
          <span class="m-icon">⭐</span>
          <div class="tier-cny m-label">{{ t('recharge.planPro') }}</div>
          <div class="tier-usd" :class="{ primary: !isEn }" v-if="!isEn">¥99<span class="m-period">/月</span></div>
          <div class="tier-usd primary" v-else>$14.14<span class="m-period">/mo</span></div>
          <div class="tier-balance">+ $120 {{ isEn ? 'balance' : '余额' }}</div>
        </button>

        <button class="stripe-tier-btn membership-card mtier-enterprise"
                :class="{ disabled: !stripeEnabled, selected: selectedTier === 'enterprise' }"
                :disabled="!stripeEnabled || stripeLoading"
                @click="selectedTier = 'enterprise'">
          <span class="m-icon">👑</span>
          <div class="tier-cny m-label">{{ t('recharge.planEnterprise') }}</div>
          <div class="tier-usd" :class="{ primary: !isEn }" v-if="!isEn">¥499<span class="m-period">/月</span></div>
          <div class="tier-usd primary" v-else>$71.29<span class="m-period">/mo</span></div>
          <div class="tier-balance">+ $600 {{ isEn ? 'balance' : '余额' }}</div>
        </button>

<div class="stripe-tier-btn stripe-custom-card"
             :class="{ disabled: !stripeEnabled, selected: selectedTier === 'custom' }"
             @click="selectedTier = 'custom'">
          <div class="custom-header">{{ t('recharge.stripe.customTitle') }}</div>
          <div class="custom-subtitle">{{ isEn ? 'Any amount, auto bonus tier' : '任意金额,自动匹配赠送档位' }}</div>
          <div class="custom-input-wrap">
            <span class="custom-prefix">{{ isEn ? '$' : '¥' }}</span>
            <input
              v-model.number="customAmount"
              type="number"
              :min="isEn ? 1.43 : 10"
              :max="isEn ? 1428 : 10000"
              :step="isEn ? 1 : 10"
              :placeholder="t('recharge.stripe.customPlaceholder')"
              class="custom-input"
              :disabled="!stripeEnabled || stripeLoading"
              @focus="selectedTier = 'custom'"
              @click.stop
            />
          </div>
          <div class="custom-preview" v-if="customBonus">
            <span v-if="customBonus.eligible">→ ${{ customBonus.balance }} (+{{ customBonus.pct }}%)</span>
            <span v-else class="custom-no-bonus">→ ${{ customBonus.balance }} (≥¥100)</span>
          </div>
          <div class="custom-preview placeholder" v-else>{{ isEn ? '$1.43 - $1428' : '¥10 - ¥10000' }}</div>
        </div>

        
      </div>

      <button
        class="big-pay-btn"
        :disabled="!stripeEnabled || stripeLoading || !canPay"
        @click="handlePay">
        {{ stripeLoading ? '⏳ ...' : (canPay ? t('recharge.stripe.customPayBtn', { amount: payAmount }) : t('recharge.stripe.selectTier')) }}
      </button>
    </div>

    <!-- 兑换码 Tab -->
    <div v-if="activeTab === 'redeem'" class="data-card redeem-top-card">
      <div class="redeem-header">{{ t('recharge.redeemHeader') }}</div>
      <div class="redeem-sub">{{ t('recharge.redeemSub') }}</div>
      <div class="redeem-row">
        <el-input
          v-model="redeemCode"
          placeholder="XXXX-XXXX-XXXX-XXXX"
          size="large"
          :disabled="redeeming"
          @input="onCodeInput"
          @change="onCodeInput"
          @keyup.enter="doRedeem"
          class="redeem-input"
        />
        <el-button
          type="success"
          size="large"
          :loading="redeeming"
          @click="doRedeem"
          class="redeem-btn"
        >{{ t('recharge.redeemBtn') }}</el-button>
      </div>
      <div class="redeem-status">
        <span v-if="redeemMsg" :style="{ color: redeemOk ? '#67c23a' : '#f56c6c' }">{{ redeemMsg }}</span>
        <span v-else-if="previewDisplay" :style="{ color: previewDisplayColor }">{{ previewDisplay }}</span>
      </div>
    </div>

    <!-- 套餐卡片 -->
    <div class="plan-grid">
      <!-- 专业版 -->
      <div class="plan-card pro">
        <div class="plan-badge">{{ t('recharge.popular') }}</div>
        <div class="plan-icon">💼</div>
        <div class="plan-name">{{ t('recharge.planPro') }}</div>
        <div class="plan-price">
          <span class="price-num">{{ t('recharge.proPrice') }}</span>
          <span class="price-unit">{{ t('recharge.perMonth') }}</span>
        </div>
        <div class="price-note">{{ t('recharge.proPriceNote') }}</div>
        <div class="plan-bonus">{{ t('recharge.proBonus') }}</div>
        <ul class="plan-features">
          <li><span class="ok">✓</span> <span v-html="t('recharge.proLi1')"></span></li>
          <li><span class="ok">✓</span> <span v-html="t('recharge.proLi2')"></span></li>
          <li><span class="ok">✓</span> <span v-html="t('recharge.proLi3')"></span></li>
          <li><span class="ok">✓</span> <span v-html="t('recharge.proLi4')"></span></li>
          <li><span class="ok">✓</span> {{ t('recharge.budgetAlert') }}</li>
          <li><span class="ok">✓</span> {{ t('recharge.csvExport') }}</li>
          <li><span class="muted">✗</span> {{ t('recharge.slaExclusive') }}</li>
        </ul>
        <div class="plan-redeem-tip">{{ t('recharge.proRedeemTip') }}</div>
      </div>

      <!-- 企业版 -->
      <div class="plan-card enterprise">
        <div class="plan-badge premium">{{ t('recharge.premium') }}</div>
        <div class="plan-icon">👑</div>
        <div class="plan-name">{{ t('recharge.planEnterprise') }}</div>
        <div class="plan-price">
          <span class="price-num">{{ t('recharge.entPrice') }}</span>
          <span class="price-unit">{{ t('recharge.perMonth') }}</span>
        </div>
        <div class="price-note">{{ t('recharge.entPriceNote') }}</div>
        <div class="plan-bonus">{{ t('recharge.entBonus') }}</div>
        <ul class="plan-features">
          <li><span class="ok">✓</span> <span v-html="t('recharge.entLi1')"></span></li>
          <li><span class="ok">✓</span> <span v-html="t('recharge.entLi2')"></span></li>
          <li><span class="ok">✓</span> <span v-html="t('recharge.entLi3')"></span></li>
          <li><span class="ok">✓</span> <span v-html="t('recharge.entLi4')"></span></li>
          <li><span class="ok">✓</span> {{ t('recharge.budgetAlert') }}</li>
          <li><span class="ok">✓</span> {{ t('recharge.csvExport') }}</li>
          <li><span class="ok">✓</span> <b>SLA 99.5%</b></li>
          <li><span class="ok">✓</span> {{ t('recharge.prioritySupport') }}</li>
        </ul>
        <div class="plan-redeem-tip">{{ t('recharge.entRedeemTip') }}</div>
      </div>
    </div>

    <!-- 说明 -->
    <div class="note-card">
      <div class="note-title">{{ t('recharge.notesTitle') }}</div>
      <ul class="note-list">
        <li>{{ t('recharge.note1') }}</li>
        <li>{{ t('recharge.note2') }}</li>
        <li>{{ t('recharge.note3') }}</li>
        <li>{{ t('recharge.note4') }}：<b>SIXWEI_</b></li>
      </ul>
    </div>

    <!-- 充值记录 -->
    <div class="data-card">
      <div class="card-header"><span class="card-title">{{ t('recharge.ordersTitle') }}</span></div>
      <div v-if="loadingOrders" class="empty-tip">{{ t('recharge.loading') }}</div>
      <div v-else-if="orders.length === 0" class="empty-tip">{{ t('recharge.noOrders') }}</div>
      <div v-else class="order-list">
        <div v-for="o in orders" :key="o.order_no" class="order-item">
          <div class="order-left">
            <div class="order-amount">${{ o.amount?.toFixed(2) }}</div>
            <div class="order-meta">{{ dayjs(o.created_at).format('YYYY-MM-DD HH:mm') }}</div>
            <div class="order-no">{{ o.order_no }}</div>
          </div>
          <span class="order-status" :class="o.payment_status">
            {{ { paid: t('recharge.statusPaid'), pending: t('recharge.statusPending'), failed: t('recharge.statusFailed') }[o.payment_status] || o.payment_status }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
const { t, locale } = useI18n()
import { ref, computed, watch, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { rechargeAPI, dashboardAPI } from '@/utils/api'
import { useAuthStore } from '@/stores/auth'
import api from '@/utils/api'
import dayjs from 'dayjs'

// 用户信息
const me = ref(null)
const orders = ref([])
const loadingOrders = ref(true)

function tierLabel(tier) {
  const m = { free: t('recharge.planFree'), pro: t('recharge.planPro'), enterprise: t('recharge.planEnterprise') }
  return m[tier] || t('recharge.planFree')
}

async function fetchUserInfo() {
  try {
    const data = await dashboardAPI.stats()
    me.value = data
  } catch {}
}

async function fetchOrders() {
  loadingOrders.value = true
  try {
    const data = await rechargeAPI.listOrders()
    orders.value = data.orders || []
  } catch {
    orders.value = []
  } finally {
    loadingOrders.value = false
  }
}

// 兑换码
const auth = useAuthStore()
const redeemCode = ref('')
const redeeming = ref(false)
const redeemMsg = ref('')
const redeemOk = ref(false)


// 兑换码实时预览
const previewInfo = ref(null)
const previewing = ref(false)
let previewTimer = null

// onCodeInput removed, using watch instead


// 监听兑换码变化
const previewDisplay = ref('')
const previewDisplayColor = ref('#999')

watch(redeemCode, (val) => {
  previewInfo.value = null
  previewDisplay.value = ''
  if (!redeemOk.value) redeemMsg.value = ''
  if (previewTimer) clearTimeout(previewTimer)
  const code = (val || '').trim().toUpperCase()
  if (code.length < 19) return
  previewing.value = true
  previewDisplay.value = t('recharge.searching')
  previewDisplayColor.value = '#999'
  previewTimer = setTimeout(async () => {
    try {
      const res = await api.get('/user/redeem/preview', { params: { code } })
      previewInfo.value = res
      const d = res
      if (!d.valid) {
        previewDisplay.value = d.error || t('recharge.invalidCode')
        previewDisplayColor.value = '#f56c6c'
      } else if (d.type === 'membership') {
        const tm = { pro: t('recharge.planPro'), enterprise: t('recharge.planEnterprise') }
        let txt = t('recharge.activatePreview', { tier: (tm[d.membership_tier] || d.membership_tier), days: d.membership_days })
        if (d.balance_amount > 0) txt += t('recharge.plusBalance', { amount: d.balance_amount.toFixed(2) })
        previewDisplay.value = txt
        previewDisplayColor.value = '#67c23a'
      } else {
        let txt = t('recharge.balancePreview', { amount: d.balance_amount.toFixed(2) })
        if (d.is_first_recharge && d.first_bonus > 0) txt += t('recharge.plusFirstBonus', { amount: d.first_bonus.toFixed(2) })
        previewDisplay.value = txt
        previewDisplayColor.value = '#67c23a'
      }
    } catch {
      previewDisplay.value = t('recharge.invalidCode')
      previewDisplayColor.value = '#f56c6c'
    } finally {
      previewing.value = false
    }
  }, 400)
})

const previewText = computed(() => {
  if (!previewInfo.value) return ''
  const p = previewInfo.value
  if (!p.valid) return p.error || t('recharge.invalidCode')
  if (p.type === 'membership') {
    const tierMap = { pro: t('recharge.planPro'), enterprise: t('recharge.planEnterprise') }
    const parts = [t('recharge.activateText', { tier: (tierMap[p.membership_tier] || p.membership_tier), days: p.membership_days })]
    if (p.balance_amount > 0) parts.push(t('recharge.balanceText', { amount: p.balance_amount.toFixed(2) }))
    return '✅ ' + parts.join(' + ')
  }
  // balance 类型
  const parts = [t('recharge.balanceText', { amount: p.balance_amount.toFixed(2) })]
  if (p.is_first_recharge && p.first_bonus > 0) {
    parts.push(t('recharge.firstBonusText', { amount: p.first_bonus.toFixed(2) }))
  }
  return '✅ ' + parts.join(' + ')
})

const previewColor = computed(() => {
  if (!previewInfo.value) return ''
  return previewInfo.value.valid ? '#67c23a' : '#f56c6c'
})

const doRedeem = async () => {
  if (!redeemCode.value.trim()) return
  redeeming.value = true
  redeemMsg.value = ''
  try {
    const res = await api.post('/user/redeem', { code: redeemCode.value.trim().toUpperCase() })
    redeemOk.value = true
    const msg = res.message || t('recharge.redeemSuccess')
    redeemMsg.value = msg
    ElMessage.success(msg)
    previewDisplay.value = ''
    redeemCode.value = ''
    await fetchUserInfo()
    await auth.fetchMe()
    window.dispatchEvent(new Event('balance-changed'))
    await fetchOrders()
  } catch (e) {
    redeemOk.value = false
    const errMsg = e.response?.data?.error || t('recharge.redeemFail')
    redeemMsg.value = errMsg
    ElMessage.error(errMsg)
  } finally {
    redeeming.value = false
  }
}

// === Tab 切换 + Stripe 支付 ===
const activeTab = ref('pay')
const stripeEnabled = ref(false)
const stripeLoading = ref(false)
const stripeTiers = [
  { id: '100',  cny: 100,  usd: '14.29',  balance: 108,  bonusPct: 8 },
  { id: '300',  cny: 300,  usd: '42.86',  balance: 330,  bonusPct: 10 },
  { id: '500',  cny: 500,  usd: '71.43',  balance: 575,  bonusPct: 15 },
  { id: '1000', cny: 1000, usd: '142.86', balance: 1200, bonusPct: 20 },
  { id: '3000', cny: 3000, usd: '428.57', balance: 3750, bonusPct: 25, recommended: true },
]
const stripeMembershipTiers = [
  { id: 'pro',        cny: 99,  usd: '14.14', balance: 120, tier: 'pro' },
  { id: 'enterprise', cny: 499, usd: '71.29', balance: 600, tier: 'enterprise' },
]
async function fetchStripeStatus() {
  try {
    const r = await api.get('/recharge/stripe/status')
    stripeEnabled.value = !!r?.enabled
  } catch (e) { stripeEnabled.value = false }
}
async function payStripe(tierId, customAmount) {
  stripeLoading.value = true
  try {
    // 计算 CNY 金额
    let amountCNY = 0
    if (tierId === 'custom') {
      amountCNY = Number(customAmount)
    } else {
      const tier = stripeTiers.find(t => t.id === tierId)
      if (tier) amountCNY = tier.cny
      else {
        const mem = stripeMembershipTiers.find(t => t.id === tierId)
        if (mem) amountCNY = mem.cny
      }
    }
    if (!amountCNY || amountCNY < 1) {
      ElMessage.error('金额无效')
      return
    }
    const r = await api.post('/user/zhifux/checkout', { amount: amountCNY, pay_type: 'aloop', tier_id: tierId })
    if (r?.pay_url) window.location.href = r.pay_url
    else ElMessage.error('未获取到支付链接')
  } catch (e) {
    ElMessage.error('支付失败: ' + (e?.response?.data?.error || e.message))
  } finally { stripeLoading.value = false }
}

// 自定义金额 (线性插值, 与后端 computeCustomTier 保持一致)
const customAmount = ref(null)
// 自定义金额: zh 输入 CNY (10-10000), en 输入 USD (1.43-1428)
// 内部统一用 amt_cny 算赠送/balance, 提交后端
const customBonus = computed(() => {
  const raw = parseFloat(customAmount.value) || 0
  if (raw <= 0) return null
  // 强制整数 CNY (后端 amount_cny 字段是 int, 小数会 400)
  const amtCNY = isEn.value ? Math.round(raw * 7.06) : Math.round(raw)
  if (amtCNY < 10 || amtCNY > 10000) return null
  let pct = 0
  if (amtCNY < 100) pct = 0
  else if (amtCNY < 300) pct = 8 + (amtCNY-100)/200*(10-8)
  else if (amtCNY < 500) pct = 10 + (amtCNY-300)/200*(15-10)
  else if (amtCNY < 1000) pct = 15 + (amtCNY-500)/500*(20-15)
  else if (amtCNY < 3000) pct = 20 + (amtCNY-1000)/2000*(25-20)
  else pct = 25
  const balanceUSD = amtCNY + amtCNY * pct / 100
  return {
    amt: amtCNY,
    amtDisplay: isEn.value ? (amtCNY / 7.06).toFixed(2) : amtCNY,
    pct: pct.toFixed(1),
    balance: balanceUSD.toFixed(2),
    eligible: amtCNY >= 100,
  }
})
function payCustom() {
  const cb = customBonus.value
  if (!cb) { ElMessage.warning(t('recharge.stripe.customInvalid')); return }
  payStripe('custom', cb.amt)
}

const selectedTier = ref(null)
const isEn = computed(() => String(locale.value).toLowerCase().startsWith('en'))
const canPay = computed(() => {
  if (!selectedTier.value) return false
  if (selectedTier.value === 'custom') return !!customBonus.value
  if (stripeTiers.some(t => t.id === selectedTier.value)) return true
  return stripeMembershipTiers.some(m => m.id === selectedTier.value)
})
const payAmount = computed(() => {
  if (selectedTier.value === 'custom') {
    return customBonus.value?.amtDisplay || 0
  }
  const tier = stripeTiers.find(x => x.id === selectedTier.value)
  if (tier) return isEn.value ? tier.usd : tier.cny
  const m = stripeMembershipTiers.find(x => x.id === selectedTier.value)
  if (m) return isEn.value ? m.usd : m.cny
  return 0
})
function handlePay() {
  if (!canPay.value) return
  if (selectedTier.value === 'custom') payCustom()
  else payStripe(selectedTier.value)
}

onMounted(() => {
  fetchUserInfo()
  fetchOrders()
  fetchStripeStatus()
})
</script>

<style scoped>
.page {
  padding: 16px;
  max-width: 480px;
  margin: 0 auto;
  overflow-x: hidden;
  touch-action: pan-y;
}
.page * { max-width: 100%; }
@media (min-width: 769px) {
  .page > * { max-width: none; }
}

/* Hero */
.recharge-hero {
  position: relative; overflow: hidden;
  background: linear-gradient(135deg, #6366f1, #8b5cf6);
  border-radius: 20px; padding: 28px 24px 24px;
  color: #fff; text-align: center; margin-bottom: 16px;
}
.hero-bg-shape {
  position: absolute; top: -30px; right: -30px;
  width: 120px; height: 120px; border-radius: 50%;
  background: rgba(255,255,255,0.1);
}
.hero-emoji { font-size: 36px; margin-bottom: 8px; }
.hero-title { font-size: 22px; font-weight: 700; }
.hero-sub { font-size: 13px; opacity: 0.9; margin-top: 4px; }

/* 当前等级 */
.current-card {
  background: #fff; border-radius: 16px;
  padding: 16px; margin-bottom: 16px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.06);
  display: flex; align-items: center; gap: 12px;
}
.current-label { font-size: 13px; color: #999; }
.tier-badge {
  padding: 4px 12px; border-radius: 20px; font-size: 13px; font-weight: 600;
}
.tier-badge.free { background: #f3f4f6; color: #6b7280; }
.tier-badge.pro { background: #eef2ff; color: #4338ca; }
.tier-badge.enterprise { background: #fef3c7; color: #92400e; }
.current-expire { font-size: 12px; color: #999; margin-left: auto; }

/* 兑换码卡片 */
.redeem-top-card {
  background: #fff; border-radius: 16px;
  padding: 20px; margin-bottom: 16px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.06);
  text-align: center;
}
.redeem-header { font-size: 18px; font-weight: 700; margin-bottom: 6px; }
.redeem-sub { font-size: 13px; color: #999; margin-bottom: 16px; }
.redeem-row { display: flex; gap: 10px; align-items: center; }
.redeem-input { flex: 1; }
.redeem-btn {
  min-width: 90px;
  background: linear-gradient(135deg, #667eea, #764ba2) !important;
  border: none !important;
  color: #fff !important;
}
.redeem-ok { color: #67c23a; margin-top: 10px; font-size: 14px; font-weight: 600; }
.redeem-err { color: #f56c6c; margin-top: 10px; font-size: 14px; }

/* Tab 切换 */
.tab-switch {
  display: flex; gap: 4px;
  background: #f3f4f6; padding: 4px;
  border-radius: 12px; margin-bottom: 16px;
}
.tab-switch button {
  flex: 1; padding: 10px 16px;
  border: none; background: transparent;
  border-radius: 8px; cursor: pointer;
  font-size: 14px; font-weight: 600;
  color: #6b7280; transition: all 0.2s;
}
.tab-switch button.active {
  background: linear-gradient(135deg, #635bff, #4b41e0);
  color: #fff;
  box-shadow: 0 2px 4px rgba(99,91,255,0.3);
}

/* Stripe 档位按钮内部 */
.tier-cny { font-size: 13px; font-weight: 600; color: #fff; }
.tier-usd { font-size: 18px; font-weight: 700; color: #fff; }
.tier-balance { font-size: 11px; opacity: 0.9; color: #fff; }

/* Stripe 支付卡片 */
.stripe-pay-c1 {
  background: #fff; border-radius: 16px;
  padding: 20px; margin-bottom: 16px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.06);
}
.stripe-header { font-size: 18px; font-weight: 700; margin-bottom: 6px; text-align: center; }
.stripe-sub { font-size: 13px; color: #666; text-align: center; margin-bottom: 16px; }
.stripe-tiers {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
  margin-bottom: 12px;
}
.stripe-tiers > .stripe-tier-btn { grid-column: span 1; }
/* 自定义卡占 2 格 (第 3 行最后 2 格) */
.stripe-tiers > .stripe-tier-btn.stripe-custom-card { grid-column: span 2; }

/* 玻璃磨砂渐变卡片 (统一: 充值/自定义/会员) */
.stripe-tier-btn {
  position: relative;
  background: linear-gradient(135deg, rgba(255,255,255,0.85) 0%, rgba(243,244,255,0.65) 100%);
  backdrop-filter: blur(12px) saturate(1.4);
  -webkit-backdrop-filter: blur(12px) saturate(1.4);
  border: 1.5px solid rgba(99, 102, 241, 0.22);
  color: #1e293b;
  border-radius: 14px;
  padding: 14px 8px 10px;
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-height: 96px;
  text-align: center;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.04), inset 0 1px 0 rgba(255,255,255,0.5);
  overflow: hidden;
}
.stripe-tier-btn::before {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at top right, rgba(99,102,241,0.08), transparent 60%);
  pointer-events: none;
}
.stripe-tier-btn:hover:not(:disabled):not(.selected) {
  border-color: rgba(99, 102, 241, 0.5);
  transform: translateY(-2px);
  box-shadow: 0 6px 16px rgba(79, 70, 229, 0.12), inset 0 1px 0 rgba(255,255,255,0.5);
}
.stripe-tier-btn.selected {
  border-color: #4338ca;
  border-width: 2px;
  box-shadow: 0 0 0 3px rgba(67, 56, 202, 0.2), 0 8px 20px rgba(67,56,202,0.15);
  transform: translateY(-2px);
}
.stripe-tier-btn.recommended {
  background: linear-gradient(135deg, rgba(255,247,237,0.9) 0%, rgba(254,243,199,0.7) 100%);
  border-color: rgba(245, 158, 11, 0.35);
}
.stripe-tier-btn.recommended::before {
  background: radial-gradient(circle at top right, rgba(234,88,12,0.1), transparent 60%);
}
.stripe-tier-btn.recommended.selected {
  border-color: #ea580c;
  box-shadow: 0 0 0 3px rgba(234, 88, 12, 0.25), 0 8px 20px rgba(234,88,12,0.18);
}

/* 会员卡: pro 蓝紫, enterprise 金橙, 玻璃渐变 */
.stripe-tier-btn.mtier-pro {
  background: linear-gradient(135deg, rgba(238,242,255,0.9) 0%, rgba(224,231,255,0.65) 100%);
  border-color: rgba(99, 102, 241, 0.35);
}
.stripe-tier-btn.mtier-pro::before {
  background: radial-gradient(circle at top right, rgba(99,102,241,0.14), transparent 60%);
}
.stripe-tier-btn.mtier-pro.selected {
  border-color: #4338ca;
}
.stripe-tier-btn.mtier-enterprise {
  background: linear-gradient(135deg, rgba(255,251,235,0.92) 0%, rgba(254,243,199,0.7) 100%);
  border-color: rgba(245, 158, 11, 0.4);
}
.stripe-tier-btn.mtier-enterprise::before {
  background: radial-gradient(circle at top right, rgba(245,158,11,0.16), transparent 60%);
}
.stripe-tier-btn.mtier-enterprise.selected {
  border-color: #d97706;
  box-shadow: 0 0 0 3px rgba(217, 119, 6, 0.25), 0 8px 20px rgba(217,119,6,0.18);
}

.membership-card .m-icon {
  font-size: 22px;
  margin-top: 4px;
  line-height: 1;
}
.membership-card .m-label {
  margin-top: 4px !important;
  color: #4338ca !important;
  font-weight: 700;
}
.stripe-tier-btn.mtier-enterprise .m-label { color: #c2410c !important; }
.membership-card .m-period {
  font-size: 11px;
  font-weight: 500;
  margin-left: 2px;
  opacity: 0.7;
}
.stripe-tier-btn.disabled,
.stripe-tier-btn:disabled {
  background: #f3f4f6;
  border-color: #e5e7eb;
  color: #9ca3af;
  cursor: not-allowed;
  opacity: 0.6;
}

.hot-tag {
  position: absolute;
  top: 4px;
  left: 6px;
  font-size: 11px;
  font-weight: 700;
}
.bonus-tag {
  position: absolute;
  top: 4px;
  right: 6px;
  font-size: 10px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 8px;
}
.tier-cny {
  font-size: 11px;
  font-weight: 600;
  margin-top: 10px;
  color: #6b7280;
}
.tier-usd {
  font-size: 11px;
  font-weight: 600;
  color: #6b7280;
  letter-spacing: 0;
}
.tier-cny.primary,
.tier-usd.primary {
  font-size: 17px;
  font-weight: 800;
  color: #1e293b;
  letter-spacing: -0.3px;
}
.tier-balance {
  font-size: 10px;
  font-weight: 500;
  color: #4338ca;
}
.stripe-tier-btn.recommended .tier-balance { color: #c2410c; }
.bonus-tag {
  color: #4338ca;
  background: #eef2ff;
}
.stripe-tier-btn.recommended .bonus-tag {
  color: #c2410c;
  background: #fff7ed;
}

/* 自定义金额卡 (跟普通卡同框架, 内含输入框) */
.stripe-custom-card {
  text-align: left;
  padding: 12px 14px 10px;
  min-height: 96px;
}
.custom-subtitle {
  font-size: 10px;
  color: #6b7280;
  font-weight: 500;
  margin-top: 2px;
  margin-bottom: 6px;
  opacity: 0.85;
}
.stripe-custom-card .custom-header {
  font-size: 12px;
}
.stripe-custom-card .custom-input {
  padding: 7px 8px 7px 22px;
  font-size: 14px;
}
.stripe-custom-card .custom-prefix {
  font-size: 14px;
  left: 10px;
}
.stripe-custom-card .custom-preview {
  font-size: 11px;
  margin-top: 5px;
}
.stripe-custom-card .custom-header {
  font-size: 11px;
  font-weight: 700;
  margin-bottom: 4px;
}
.custom-input-wrap {
  position: relative;
  display: flex;
  align-items: center;
}
.custom-prefix {
  position: absolute;
  left: 8px;
  font-size: 13px;
  font-weight: 700;
  pointer-events: none;
}
.custom-input {
  width: 100%;
  padding: 5px 6px 5px 20px;
  border: 1.5px solid #e5e7eb;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 700;
  background: #fff;
  color: #1e293b;
  outline: none;
  transition: border-color 0.15s;
}
.stripe-custom-card.selected .custom-input { border-color: #6366f1; }
.stripe-custom-card .custom-header { color: #4338ca; }
.stripe-custom-card .custom-prefix { color: #6366f1; }
.stripe-custom-card .custom-preview { color: #059669; }
.stripe-custom-card .custom-preview.placeholder { color: #94a3b8; }
.stripe-custom-card .custom-no-bonus { color: #94a3b8; }
.custom-input:focus { border-color: #6366f1; }
.custom-input::-webkit-outer-spin-button,
.custom-input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
.custom-input[type=number] { -moz-appearance: textfield; }
.custom-preview {
  font-size: 10px;
  font-weight: 600;
  margin-top: 4px;
  min-height: 13px;
  line-height: 13px;
}
.custom-preview.placeholder { opacity: 0.6; font-weight: 500; }
.custom-no-bonus { opacity: 0.7; }

/* 大支付按钮 */
.big-pay-btn {
  width: 100%;
  padding: 13px;
  background: linear-gradient(135deg, #6366f1, #4338ca);
  color: #fff;
  border: none;
  border-radius: 12px;
  font-size: 15px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.18s ease;
  margin-bottom: 6px;
  box-shadow: 0 4px 12px rgba(79, 70, 229, 0.28);
}
.big-pay-btn:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 6px 18px rgba(79, 70, 229, 0.42);
}
.big-pay-btn:active:not(:disabled) { transform: translateY(0); }
.big-pay-btn:disabled {
  background: #d1d5db;
  color: #6b7280;
  cursor: not-allowed;
  box-shadow: none;
}

/* 套餐对比卡 (玻璃磨砂渐变, 深字) */
.plan-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-bottom: 16px; }
.plan-card {
  border-radius: 16px;
  padding: 18px 14px;
  position: relative;
  overflow: hidden;
  backdrop-filter: blur(12px) saturate(1.4);
  -webkit-backdrop-filter: blur(12px) saturate(1.4);
  border: 1.5px solid;
  box-shadow: 0 4px 14px rgba(15, 23, 42, 0.06), inset 0 1px 0 rgba(255,255,255,0.5);
}
.plan-card::before {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
}
.plan-card.pro {
  background: linear-gradient(135deg, rgba(238,242,255,0.92) 0%, rgba(224,231,255,0.7) 100%);
  border-color: rgba(99, 102, 241, 0.35);
  color: #1e1b4b;
}
.plan-card.pro::before {
  background: radial-gradient(circle at top right, rgba(99,102,241,0.15), transparent 60%);
}
.plan-card.enterprise {
  background: linear-gradient(135deg, rgba(255,251,235,0.94) 0%, rgba(254,243,199,0.7) 100%);
  border-color: rgba(245, 158, 11, 0.4);
  color: #7c2d12;
}
.plan-card.enterprise::before {
  background: radial-gradient(circle at top right, rgba(245,158,11,0.18), transparent 60%);
}

.plan-badge {
  font-size: 10px; font-weight: 700; padding: 3px 10px;
  border-radius: 10px;
  background: rgba(99, 102, 241, 0.18);
  color: #4338ca;
  display: inline-block; margin-bottom: 10px;
  letter-spacing: 0.3px;
}
.plan-card.enterprise .plan-badge {
  background: rgba(217, 119, 6, 0.18);
  color: #c2410c;
}
.plan-badge.premium {
  background: rgba(217, 119, 6, 0.2);
  color: #c2410c;
}

.plan-icon { font-size: 32px; margin-bottom: 8px; line-height: 1; }
.plan-name {
  font-size: 16px; font-weight: 700;
  color: #1e1b4b;
  margin-bottom: 6px;
}
.plan-card.enterprise .plan-name { color: #7c2d12; }

.plan-price { margin-bottom: 6px; display: flex; align-items: baseline; gap: 2px; }
.price-num {
  font-size: 28px; font-weight: 800;
  color: #4338ca;
  letter-spacing: -0.5px;
}
.plan-card.enterprise .price-num { color: #c2410c; }
.price-unit { font-size: 12px; color: rgba(67, 56, 202, 0.7); font-weight: 600; }
.plan-card.enterprise .price-unit { color: rgba(194, 65, 12, 0.7); }

.price-note { font-size: 10px; color: #6b7280; margin: -2px 0 8px; opacity: 0.85; line-height: 1.3; }
.plan-bonus {
  font-size: 12px; font-weight: 600;
  color: #059669;
  margin-bottom: 12px;
}

.plan-features { list-style: none; padding: 0; margin: 0 0 14px; }
.plan-features li {
  font-size: 12.5px;
  color: #1e293b;
  margin-bottom: 6px;
  display: flex;
  gap: 6px;
  line-height: 1.5;
}
.ok { color: #059669; font-weight: 700; flex-shrink: 0; }
.muted { color: #9ca3af; }

.plan-redeem-tip {
  font-size: 11px;
  color: #4338ca;
  text-align: center;
  background: rgba(99, 102, 241, 0.1);
  border: 1px solid rgba(99, 102, 241, 0.2);
  border-radius: 8px;
  padding: 8px;
  margin-top: 6px;
  font-weight: 500;
}
.plan-card.enterprise .plan-redeem-tip {
  color: #c2410c;
  background: rgba(217, 119, 6, 0.1);
  border-color: rgba(217, 119, 6, 0.22);
}

/* 说明 */
.note-card {
  background: #f9fafb; border-radius: 16px;
  padding: 16px; margin-bottom: 16px;
}
.note-title { font-size: 14px; font-weight: 600; margin-bottom: 8px; }
.note-list { padding-left: 16px; margin: 0; }
.note-list li { font-size: 12px; color: #6b7280; margin-bottom: 6px; line-height: 1.5; }
.link { color: #6366f1; text-decoration: none; }

/* 充值记录 */
.data-card {
  background: #fff; border-radius: 16px;
  padding: 16px; margin-bottom: 16px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.06);
}
.card-header { display: flex; align-items: center; margin-bottom: 12px; }
.card-title { font-size: 15px; font-weight: 600; }
.empty-tip { text-align: center; color: #999; padding: 24px 0; font-size: 14px; }
.order-list { display: flex; flex-direction: column; gap: 12px; }
.order-item {
  display: flex; justify-content: space-between; align-items: flex-start;
  padding-bottom: 12px; border-bottom: 1px solid #f3f4f6;
  gap: 12px;
}
.order-left {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
}
.order-item:last-child { border-bottom: none; padding-bottom: 0; }
.order-amount { font-size: 16px; font-weight: 700; color: #1f2937; }
.order-meta { font-size: 12px; color: #9ca3af; margin-top: 2px; }
.order-no { font-size: 11px; color: #d1d5db; margin-top: 2px; word-break: break-all; overflow-wrap: anywhere; max-width: 100%; }
.order-status {
  font-size: 12px; font-weight: 600; padding: 4px 10px;
  border-radius: 20px;
  white-space: nowrap;
  flex-shrink: 0;
}
.order-status.paid { background: #d1fae5; color: #059669; }
.order-status.pending { background: #fef3c7; color: #d97706; }
.order-status.failed { background: #fee2e2; color: #dc2626; }
.redeem-preview { margin-top: 8px; font-size: 13px; font-weight: 500; }
.query-btn { min-width: 60px; }

@media (min-width: 769px) {
  .page {
    max-width: 1100px;
    width: 100%;
    margin-left: auto !important;
    margin-right: auto !important;
    justify-self: center;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px 20px;
    align-items: start;
  }
  .recharge-hero,
  .current-card,
  .tab-switch,
  .stripe-pay-c1,
  .plan-grid {
    grid-column: 1 / -1 !important;
    margin: 0 !important;
  }
  /* 兜底: 通过位置选择器强制前几个卡片满宽 (压缩器 bug workaround) */
  .page > .data-card { grid-column: 1 / -1 !important; }
  .redeem-top-card,
  .data-card,
  .note-card {
    grid-column: span 1;
    margin: 0 !important;
  }
  /* 桌面: 5 档 + 自定义卡 6 列 */
  .stripe-tiers { grid-template-columns: repeat(3, 1fr); }
}



.stripe-section-title { font-size: 13px; font-weight: 600; color: #999; margin: 18px 0 10px; padding-left: 4px; }
.stripe-section-title.membership-title { color: #6366f1; }


/* 首充赠送 banner */
.first-recharge-banner {
  background: linear-gradient(135deg, #fef3c7 0%, #fde68a 100%);
  border: 1px solid #fbbf24;
  border-radius: 12px;
  padding: 12px 14px;
  margin-bottom: 14px;
}
.first-recharge-banner .frb-title {
  font-size: 14px; font-weight: 700; color: #92400e;
}
.first-recharge-banner .frb-sub {
  font-size: 12px; color: #b45309; margin-top: 2px;
}

</style>
