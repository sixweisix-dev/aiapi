<template>
  <div class="page">
    <!-- 页头 -->
    <div class="recharge-hero">
      <div class="hero-bg-shape"></div>
      <div class="hero-emoji">💳</div>
      <div class="hero-title">充值 & 会员</div>
      <div class="hero-sub">兑换码直接到账 · 会员解锁更高速率</div>
    </div>

    <!-- 当前等级 -->
    <div v-if="me" class="current-card">
      <div class="current-label">当前等级</div>
      <div class="current-tier">
        <span class="tier-badge" :class="me.membership_tier || 'free'">
          {{ tierLabel(me.membership_tier) }}
        </span>
      </div>
      <div v-if="me.membership_expires_at && me.membership_tier !== 'free'" class="current-expire">
        到期时间：{{ dayjs(me.membership_expires_at).format('YYYY-MM-DD HH:mm') }}
      </div>
    </div>

    <!-- 兑换码（置顶） -->
    <div class="data-card redeem-top-card">
      <div class="redeem-header">🎁 兑换码</div>
      <div class="redeem-sub">输入购买的兑换码，余额或会员权益即刻到账</div>
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
        >立即兑换</el-button>
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
        <div class="plan-badge">最受欢迎</div>
        <div class="plan-icon">💼</div>
        <div class="plan-name">专业版</div>
        <div class="plan-price">
          <span class="price-num">¥99</span>
          <span class="price-unit">/ 月</span>
        </div>
        <div class="plan-bonus">充 ¥99 → 到账 ¥120（送 ¥21）</div>
        <ul class="plan-features">
          <li><span class="ok">✓</span> 充值到账 <b>¥120</b></li>
          <li><span class="ok">✓</span> RPM <b>60</b>（10 倍提速）</li>
          <li><span class="ok">✓</span> TPM <b>10 万</b></li>
          <li><span class="ok">✓</span> API Key 数量 <b>5 个</b></li>
          <li><span class="ok">✓</span> 预算告警</li>
          <li><span class="ok">✓</span> CSV 账单导出</li>
          <li><span class="muted">✗</span> SLA 保障（企业版独享）</li>
        </ul>
        <div class="plan-redeem-tip">购买专业版兑换码后在上方输入</div>
      </div>

      <!-- 企业版 -->
      <div class="plan-card enterprise">
        <div class="plan-badge premium">尊享旗舰</div>
        <div class="plan-icon">👑</div>
        <div class="plan-name">企业版</div>
        <div class="plan-price">
          <span class="price-num">¥499</span>
          <span class="price-unit">/ 月</span>
        </div>
        <div class="plan-bonus">充 ¥499 → 到账 ¥600（送 ¥101）</div>
        <ul class="plan-features">
          <li><span class="ok">✓</span> 充值到账 <b>¥600</b></li>
          <li><span class="ok">✓</span> RPM <b>600</b>（100 倍提速）</li>
          <li><span class="ok">✓</span> TPM <b>100 万</b></li>
          <li><span class="ok">✓</span> API Key <b>不限数量</b></li>
          <li><span class="ok">✓</span> 预算告警</li>
          <li><span class="ok">✓</span> CSV 账单导出</li>
          <li><span class="ok">✓</span> <b>SLA 99.5%</b></li>
          <li><span class="ok">✓</span> 优先技术支持</li>
        </ul>
        <div class="plan-redeem-tip">购买企业版兑换码后在上方输入</div>
      </div>
    </div>

    <!-- 说明 -->
    <div class="note-card">
      <div class="note-title">💡 说明</div>
      <ul class="note-list">
        <li>会员有效期 30 天，到期后自动恢复免费版速率（已充值余额不受影响）</li>
        <li>在闲鱼购买兑换码后，输入上方兑换框即可激活会员和余额</li>
        <li>未到期续费会自动叠加时长，多月连续续费额度叠加</li>
        <li>企业开票或定制 SLA，请联系：<a href="mailto:sixweisix@gmail.com" class="link">sixweisix@gmail.com</a></li>
      </ul>
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
            {{ { paid: '已到账', pending: '待支付', failed: '失败' }[o.payment_status] || o.payment_status }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { rechargeAPI, dashboardAPI } from '@/utils/api'
import api from '@/utils/api'
import dayjs from 'dayjs'

// 用户信息
const me = ref(null)
const orders = ref([])
const loadingOrders = ref(true)

function tierLabel(t) {
  const m = { free: '免费版', pro: '专业版', enterprise: '企业版' }
  return m[t] || '免费版'
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
  previewDisplay.value = '查询中...'
  previewDisplayColor.value = '#999'
  previewTimer = setTimeout(async () => {
    try {
      const res = await api.get('/user/redeem/preview', { params: { code } })
      previewInfo.value = res
      const d = res
      if (!d.valid) {
        previewDisplay.value = d.error || '兑换码无效'
        previewDisplayColor.value = '#f56c6c'
      } else if (d.type === 'membership') {
        const tm = { pro: '专业版', enterprise: '企业版' }
        let txt = '✅ 开通 ' + (tm[d.membership_tier] || d.membership_tier) + ' ' + d.membership_days + ' 天'
        if (d.balance_amount > 0) txt += ' + 余额 +¥' + d.balance_amount.toFixed(2)
        previewDisplay.value = txt
        previewDisplayColor.value = '#67c23a'
      } else {
        let txt = '✅ 余额 +¥' + d.balance_amount.toFixed(2)
        if (d.is_first_recharge && d.first_bonus > 0) txt += ' + 首充礼 +¥' + d.first_bonus.toFixed(2)
        previewDisplay.value = txt
        previewDisplayColor.value = '#67c23a'
      }
    } catch {
      previewDisplay.value = '兑换码无效'
      previewDisplayColor.value = '#f56c6c'
    } finally {
      previewing.value = false
    }
  }, 400)
})

const previewText = computed(() => {
  if (!previewInfo.value) return ''
  const p = previewInfo.value
  if (!p.valid) return p.error || '兑换码无效'
  if (p.type === 'membership') {
    const tierMap = { pro: '专业版', enterprise: '企业版' }
    const parts = [`开通 ${tierMap[p.membership_tier] || p.membership_tier} ${p.membership_days} 天`]
    if (p.balance_amount > 0) parts.push(`余额 +¥${p.balance_amount.toFixed(2)}`)
    return '✅ ' + parts.join(' + ')
  }
  // balance 类型
  const parts = [`余额 +¥${p.balance_amount.toFixed(2)}`]
  if (p.is_first_recharge && p.first_bonus > 0) {
    parts.push(`首充礼 +¥${p.first_bonus.toFixed(2)}`)
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
    const msg = res.message || '兑换成功！'
    redeemMsg.value = msg
    ElMessage.success(msg)
    previewDisplay.value = ''
    redeemCode.value = ''
    await fetchUserInfo()
    await fetchOrders()
  } catch (e) {
    redeemOk.value = false
    const errMsg = e.response?.data?.error || '兑换失败，请检查兑换码'
    redeemMsg.value = errMsg
    ElMessage.error(errMsg)
  } finally {
    redeeming.value = false
  }
}

onMounted(() => {
  fetchUserInfo()
  fetchOrders()
})
</script>

<style scoped>
.page { padding: 16px; max-width: 480px; margin: 0 auto; }

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

/* 套餐卡片 */
.plan-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-bottom: 16px; }
.plan-card {
  border-radius: 16px; padding: 16px 12px;
  position: relative; overflow: hidden;
}
.plan-card.pro { background: linear-gradient(145deg, #667eea 0%, #764ba2 100%); color: #fff; }
.plan-card.enterprise { background: linear-gradient(145deg, #f093fb 0%, #f5576c 100%); color: #fff; }
.plan-badge {
  font-size: 10px; font-weight: 700; padding: 2px 8px;
  border-radius: 10px; background: rgba(255,255,255,0.25); color: #fff;
  display: inline-block; margin-bottom: 8px;
}
.plan-badge.premium { background: rgba(255,255,255,0.25); }
.plan-icon { font-size: 28px; margin-bottom: 6px; }
.plan-name { font-size: 15px; font-weight: 700; color: #fff; margin-bottom: 4px; }
.plan-price { margin-bottom: 4px; }
.price-num { font-size: 26px; font-weight: 800; color: #fff; }
.plan-card.enterprise .price-num { color: #fff; }
.price-unit { font-size: 12px; color: rgba(255,255,255,0.8); }
.plan-bonus { font-size: 11px; color: rgba(255,255,255,0.9); font-weight: 600; margin-bottom: 10px; }
.plan-features { list-style: none; padding: 0; margin: 0 0 12px; }
.plan-features li { font-size: 12px; color: rgba(255,255,255,0.9); margin-bottom: 4px; display: flex; gap: 4px; }
.ok { color: #a7f3d0; }
.muted { color: rgba(255,255,255,0.4); }
.plan-redeem-tip {
  font-size: 11px; color: rgba(255,255,255,0.8); text-align: center;
  background: rgba(255,255,255,0.15); border-radius: 8px;
  padding: 6px; margin-top: 4px;
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
  display: flex; justify-content: space-between; align-items: center;
  padding-bottom: 12px; border-bottom: 1px solid #f3f4f6;
}
.order-item:last-child { border-bottom: none; padding-bottom: 0; }
.order-amount { font-size: 16px; font-weight: 700; color: #1f2937; }
.order-meta { font-size: 12px; color: #9ca3af; margin-top: 2px; }
.order-no { font-size: 11px; color: #d1d5db; margin-top: 2px; }
.order-status {
  font-size: 12px; font-weight: 600; padding: 4px 10px;
  border-radius: 20px;
}
.order-status.paid { background: #d1fae5; color: #059669; }
.order-status.pending { background: #fef3c7; color: #d97706; }
.order-status.failed { background: #fee2e2; color: #dc2626; }
.redeem-preview { margin-top: 8px; font-size: 13px; font-weight: 500; }
.query-btn { min-width: 60px; }
</style>
