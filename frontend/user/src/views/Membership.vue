<template>
  <div class="page">
    <!-- 页头 -->
    <div class="hero">
      <div class="hero-emoji">⭐</div>
      <div class="hero-title">会员中心</div>
      <div class="hero-sub">解锁更高速率、更多 API Key、专业服务</div>
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
        到期时间：{{ formatDate(me.membership_expires_at) }}
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
          <li><span class="ok">✓</span> 发票支持</li>
          <li><span class="muted">✗</span> SLA 保障（企业版独享）</li>
        </ul>
        <button class="plan-btn pro-btn" :disabled="submitting" @click="upgrade('membership_pro', 99)">
          {{ submitting === 'membership_pro' ? '处理中...' : '立即开通专业版' }}
        </button>
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
          <li><span class="ok">✓</span> 发票支持</li>
          <li><span class="ok"></span> <b>SLA 99.5%</b></li>
          <li><span class="ok"></span> 优先技术支持</li>
        </ul>
        <button class="plan-btn ent-btn" :disabled="submitting" @click="upgrade('membership_enterprise', 499)">
          {{ submitting === 'membership_enterprise' ? '处理中...' : '立即开通企业版' }}
        </button>
      </div>
    </div>

    <!-- 说明 -->
    <div class="note-card">
      <div class="note-title">💡 说明</div>
      <ul class="note-list">
        <li>会员有效期 30 天，到期后自动恢复免费版速率（已充值余额不受影响）</li>
        <li>会员套餐与<router-link to="/recharge" class="link">充值优惠活动</router-link>独立，套餐金额按官方专属福利计算，不享受阶梯赠送</li>
        <li>未到期续费会自动叠加时长，多月连续续费不打折但额度叠加</li>
        <li>如需企业开票或定制 SLA，请联系：<a href="mailto:sixweisix@gmail.com" class="link">sixweisix@gmail.com</a></li>
      </ul>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { rechargeAPI, dashboardAPI } from '@/utils/api'
import dayjs from 'dayjs'

const me = ref(null)
const submitting = ref('')

function tierLabel(t) {
  const m = { free: '免费版', pro: '专业版', enterprise: '企业版' }
  return m[t] || '免费版'
}

function formatDate(s) {
  if (!s) return '-'
  return dayjs(s).format('YYYY-MM-DD HH:mm')
}

async function loadMe() {
  try {
    const data = await dashboardAPI.stats()
    me.value = data
  } catch {}
}

async function upgrade(intent, amount) {
  submitting.value = intent
  try {
    const data = await rechargeAPI.createOrder(amount, intent)
    if (data.pay_url) {
      window.open(data.pay_url, '_blank')
      ElMessage.success('订单已创建，正在跳转支付...')
    }
  } catch (e) {
    // 拦截器已弹错误
  } finally {
    submitting.value = ''
  }
}

onMounted(loadMe)
</script>

<style scoped>
.page { padding-bottom: 30px; }

.hero {
  background: linear-gradient(135deg, #fbbf24, #f59e0b);
  border-radius: 20px; padding: 28px 20px; color: #fff;
  margin-bottom: 14px; text-align: center;
  box-shadow: 0 10px 30px rgba(245,158,11,0.3);
}
.hero-emoji { font-size: 40px; margin-bottom: 6px; }
.hero-title { font-size: 22px; font-weight: 800; }
.hero-sub { font-size: 13px; opacity: 0.95; margin-top: 4px; }

.current-card {
  background: #fff; border-radius: 14px; padding: 16px;
  margin-bottom: 14px; box-shadow: 0 2px 8px rgba(0,0,0,0.04);
  display: flex; align-items: center; gap: 12px;
}
.current-label { font-size: 13px; color: #9ca3af; }
.tier-badge {
  display: inline-block; padding: 4px 14px; border-radius: 12px;
  font-size: 13px; font-weight: 700;
}
.tier-badge.free { background: #f3f4f6; color: #6b7280; }
.tier-badge.pro { background: linear-gradient(135deg, #818cf8, #6366f1); color: #fff; }
.tier-badge.enterprise { background: linear-gradient(135deg, #fbbf24, #f59e0b); color: #fff; }
.current-expire { font-size: 12px; color: #9ca3af; margin-left: auto; }

.plan-grid { display: grid; gap: 14px; }
.plan-card {
  position: relative;
  background: #fff; border-radius: 18px; padding: 24px 20px;
  box-shadow: 0 8px 24px rgba(0,0,0,0.06);
  border: 2px solid transparent;
  overflow: hidden;
}
.plan-card.pro { border-color: #818cf8; }
.plan-card.enterprise {
  background: linear-gradient(135deg, #fffbeb, #fef3c7);
  border-color: #f59e0b;
}
.plan-badge {
  position: absolute; top: 12px; right: 12px;
  background: #6366f1; color: #fff;
  padding: 3px 10px; border-radius: 10px;
  font-size: 11px; font-weight: 600;
}
.plan-badge.premium { background: linear-gradient(135deg, #f59e0b, #d97706); }

.plan-icon { font-size: 36px; margin-bottom: 4px; }
.plan-name { font-size: 22px; font-weight: 800; color: #1f2937; }
.plan-price { margin: 8px 0 4px; }
.price-num {
  font-size: 36px; font-weight: 800;
  background: linear-gradient(135deg, #6366f1, #8b5cf6);
  -webkit-background-clip: text; -webkit-text-fill-color: transparent;
  background-clip: text;
}
.enterprise .price-num { background: linear-gradient(135deg, #f59e0b, #d97706); -webkit-background-clip: text; background-clip: text; -webkit-text-fill-color: transparent; }
.price-unit { color: #9ca3af; font-size: 14px; margin-left: 4px; }
.plan-bonus { font-size: 13px; color: #059669; font-weight: 600; margin-bottom: 14px; }
.plan-features { list-style: none; padding: 0; margin: 14px 0; }
.plan-features li { padding: 6px 0; font-size: 14px; color: #374151; display: flex; gap: 10px; align-items: center; }
.plan-features li b { color: #1f2937; }
.plan-features li .ok { color: #10b981; font-weight: 700; }
.plan-features li .muted { color: #d1d5db; }

.plan-btn {
  width: 100%; height: 48px; border: none; border-radius: 14px;
  font-size: 15px; font-weight: 700; cursor: pointer;
  margin-top: 8px;
  transition: transform 0.15s;
}
.plan-btn:active { transform: scale(0.98); }
.plan-btn:disabled { opacity: 0.6; }
.pro-btn { background: linear-gradient(135deg, #6366f1, #8b5cf6); color: #fff; box-shadow: 0 6px 16px rgba(99,102,241,0.35); }
.ent-btn { background: linear-gradient(135deg, #f59e0b, #d97706); color: #fff; box-shadow: 0 6px 16px rgba(245,158,11,0.4); }

.note-card {
  margin-top: 14px; background: #f9fafb;
  border-radius: 14px; padding: 16px;
}
.note-title { font-size: 14px; font-weight: 600; color: #374151; margin-bottom: 8px; }
.note-list { list-style: none; padding: 0; margin: 0; }
.note-list li { padding: 5px 0; font-size: 12px; color: #6b7280; line-height: 1.6; }
.link { color: #6366f1; text-decoration: none; }
</style>
