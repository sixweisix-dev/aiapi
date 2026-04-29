<template>
  <div class="page">
    <div class="page-header">
      <h1>⚙️ 系统设置</h1>
      <p class="muted">运营配置（修改后立即生效）</p>
    </div>

    <div v-if="loaded">
      <!-- 注册赠送 -->
      <div class="settings-card">
        <h2 class="card-title">🎁 注册赠送</h2>
        <el-form label-position="top">
          <el-form-item>
            <template #label>
              <div class="lbl">注册赠送余额（¥）</div>
              <div class="lbl-desc">新用户注册成功后自动到账。同 IP 每日限领 1 次。设为 0 关闭。</div>
            </template>
            <el-input-number v-model="signupBonus" :min="0" :max="1000" :precision="2" :step="1" size="large" style="width: 240px" />
          </el-form-item>
        </el-form>
      </div>

      <!-- 充值赠送活动 -->
      <div class="settings-card">
        <h2 class="card-title">💰 充值赠送活动</h2>

        <el-form label-position="top">
          <el-form-item>
            <template #label>
              <div class="lbl">活动总开关</div>
              <div class="lbl-desc">关闭后所有充值都按本金到账（会员套餐 ¥99/¥499 不受影响）</div>
            </template>
            <el-switch v-model="promoEnabled" />
          </el-form-item>

          <el-form-item>
            <template #label>
              <div class="lbl">阶梯赠送规则</div>
              <div class="lbl-desc">
                按"充值金额 ≥ 门槛"匹配最高档赠送。会员套餐 ¥99/¥499 不会触发阶梯。
                <span style="color:#ef4444">⚠️ 单档赠送 ÷ 充值金额建议 ≤ 28%（避免负毛利）</span>
              </div>
            </template>
            <div class="tier-table">
              <div class="tier-head">
                <span>充值门槛 (¥)</span>
                <span>额外赠送 (¥)</span>
                <span>赠送占比</span>
                <span>毛利率(估)</span>
                <span></span>
              </div>
              <div v-for="(t, idx) in tiers" :key="idx" class="tier-row">
                <el-input-number v-model="t.min" :min="1" :precision="0" controls-position="right" />
                <el-input-number v-model="t.bonus" :min="0" :precision="2" controls-position="right" />
                <span class="ratio" :class="ratioClass(t)">{{ ratio(t) }}</span>
                <span class="ratio" :class="profitClass(t)">{{ profit(t) }}</span>
                <el-button type="danger" link @click="tiers.splice(idx, 1)">删除</el-button>
              </div>
              <el-button type="primary" plain size="small" @click="addTier">+ 添加档位</el-button>
            </div>
          </el-form-item>

          <el-form-item>
            <template #label>
              <div class="lbl">新人首充额外礼（¥）</div>
              <div class="lbl-desc">仅第一次充值时叠加在阶梯之上，不论充多少都送这个固定金额。设为 0 关闭。</div>
            </template>
            <el-input-number v-model="firstRechargeBonus" :min="0" :max="500" :precision="2" :step="10" size="large" style="width: 240px" />
          </el-form-item>

          <el-alert type="info" :closable="false" style="margin-top: 12px">
            <template #title>
              <div style="font-size:13px;line-height:1.7">
                <div>预览：{{ previewText }}</div>
              </div>
            </template>
          </el-alert>
        </el-form>
      </div>

      <!-- 成本告警 -->
      <div class="settings-card">
        <h2 class="card-title">📧 每日成本告警</h2>
        <p class="muted" style="margin: -8px 0 14px">每天凌晨 01:00 自动检查上游成本，超阈值发邮件。</p>
        <el-form label-position="top">
          <el-form-item>
            <template #label>
              <div class="lbl">告警邮箱</div>
              <div class="lbl-desc">留空则不发邮件（仅记录日志）</div>
            </template>
            <el-input v-model="alertEmail" placeholder="admin@example.com" size="large" style="max-width: 360px" />
          </el-form-item>
          <el-form-item>
            <template #label>
              <div class="lbl">⚠️ 警告阈值（¥）</div>
              <div class="lbl-desc">昨日上游成本超过此值发普通告警邮件</div>
            </template>
            <el-input-number v-model="alertWarn" :min="0" :max="100000" :precision="0" :step="50" size="large" style="width: 240px" />
          </el-form-item>
          <el-form-item>
            <template #label>
              <div class="lbl">🚨 紧急阈值（¥）</div>
              <div class="lbl-desc">超过此值邮件标题加 🚨 前缀</div>
            </template>
            <el-input-number v-model="alertCritical" :min="0" :max="100000" :precision="0" :step="100" size="large" style="width: 240px" />
          </el-form-item>
        </el-form>
      </div>

      <!-- 其他 -->
      <div class="settings-card">
        <h2 class="card-title">🛠 其他</h2>
        <el-form label-position="top">
          <el-form-item>
            <template #label><div class="lbl">公告</div><div class="lbl-desc">显示在用户前台 Dashboard 顶部</div></template>
            <el-input v-model="announcement" type="textarea" :rows="3" placeholder="例如：周末维护通知" />
          </el-form-item>
          <el-form-item>
            <template #label><div class="lbl">允许新用户注册</div><div class="lbl-desc">关闭后注册接口拒绝新请求</div></template>
            <el-switch v-model="allowRegistration" />
          </el-form-item>
        </el-form>
      </div>

      <div class="save-bar">
        <el-button type="primary" size="large" :loading="saving" @click="save">保存全部修改</el-button>
      </div>
    </div>

    <div v-else class="loading-block">加载中...</div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/utils/api'

const loaded = ref(false)
const saving = ref(false)
const signupBonus = ref(0)
const announcement = ref('')
const allowRegistration = ref(true)
const promoEnabled = ref(true)
const firstRechargeBonus = ref(0)
const tiers = ref([])
const alertEmail = ref('')
const alertWarn = ref(100)
const alertCritical = ref(500)

function ratio(t) {
  if (!t.min || t.min <= 0) return '-'
  return ((t.bonus / t.min) * 100).toFixed(1) + '%'
}
function ratioClass(t) {
  if (!t.min) return ''
  const r = t.bonus / t.min
  if (r > 0.28) return 'bad'
  if (r > 0.20) return 'warn'
  return 'ok'
}
// 平台毛利率 33.3%(1.5x), 赠送等比例侵蚀
function profit(t) {
  if (!t.min || t.min <= 0) return '-'
  const grossMargin = 1/3 // 33.3%
  const ratio = t.bonus / t.min
  // 用户拿到本金*(1+ratio)的额度,实际成本=本金*(1+ratio)*(1-grossMargin),实际收入=本金
  // 毛利=本金 - 本金*(1+ratio)*(2/3) = 本金 * (1 - (2/3)*(1+ratio))
  const realMargin = 1 - (1 - grossMargin) * (1 + ratio)
  return (realMargin * 100).toFixed(1) + '%'
}
function profitClass(t) {
  if (!t.min) return ''
  const grossMargin = 1/3
  const ratio = t.bonus / t.min
  const realMargin = 1 - (1 - grossMargin) * (1 + ratio)
  if (realMargin < 0) return 'bad'
  if (realMargin < 0.10) return 'warn'
  return 'ok'
}

function addTier() {
  tiers.value.push({ min: 100, bonus: 0 })
  tiers.value.sort((a, b) => a.min - b.min)
}

const previewText = computed(() => {
  if (!promoEnabled.value) return '活动已关闭，所有充值按本金到账。'
  const sorted = [...tiers.value].filter(t => t.min > 0).sort((a, b) => a.min - b.min)
  const examples = []
  // 选一个中等档位作为示例
  const sampleMin = sorted.length > 0 ? sorted[Math.floor(sorted.length / 2)].min : 100
  // 找首充示例 = 命中最高档
  let bonus = 0
  for (const t of sorted) {
    if (sampleMin >= t.min && t.bonus > bonus) bonus = t.bonus
  }
  const fb = firstRechargeBonus.value || 0
  examples.push(`首充 ¥${sampleMin} → 实际到账 ¥${(sampleMin + bonus + fb).toFixed(2)}（本金 ¥${sampleMin} + 阶梯 ¥${bonus} + 新人 ¥${fb}）`)
  examples.push(`第二次充 ¥${sampleMin} → 实际到账 ¥${(sampleMin + bonus).toFixed(2)}（仅享阶梯）`)
  return examples.join(' / ')
})

async function load() {
  try {
    const res = await api.get('/admin/settings')
    signupBonus.value = parseFloat(res.signup_bonus || '0')
    announcement.value = res.announcement || ''
    allowRegistration.value = (res.allow_registration ?? 'true') === 'true'
    promoEnabled.value = (res.recharge_promo_enabled ?? 'true') === 'true'
    firstRechargeBonus.value = parseFloat(res.first_recharge_bonus || '0')
    try {
      const parsed = JSON.parse(res.recharge_tiers || '[]')
      tiers.value = Array.isArray(parsed) ? parsed.map(t => ({ min: Number(t.min) || 0, bonus: Number(t.bonus) || 0 })) : []
    } catch {
      tiers.value = []
    }
    alertEmail.value = res.alert_email || ''
    alertWarn.value = parseFloat(res.alert_warn_threshold || '100')
    alertCritical.value = parseFloat(res.alert_critical_threshold || '500')
    loaded.value = true
  } catch (e) {}
}

async function save() {
  // 简单合法性检查
  for (const t of tiers.value) {
    if (t.min <= 0) {
      return ElMessage.warning('阶梯门槛必须大于 0')
    }
    if (t.bonus < 0) {
      return ElMessage.warning('赠送金额不能为负')
    }
  }
  saving.value = true
  try {
    await api.put('/admin/settings', {
      signup_bonus: String(signupBonus.value),
      announcement: announcement.value,
      allow_registration: allowRegistration.value ? 'true' : 'false',
      recharge_promo_enabled: promoEnabled.value ? 'true' : 'false',
      first_recharge_bonus: String(firstRechargeBonus.value),
      recharge_tiers: JSON.stringify(
        [...tiers.value].sort((a, b) => a.min - b.min).filter(t => t.min > 0)
      ),
      alert_email: alertEmail.value,
      alert_warn_threshold: String(alertWarn.value),
      alert_critical_threshold: String(alertCritical.value),
    })
    ElMessage.success('已保存')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.page { padding: 24px; max-width: 800px; margin: 0 auto; padding-bottom: 100px; }
.page-header { margin-bottom: 24px; }
.page-header h1 { font-size: 22px; margin: 0 0 4px; }
.muted { color: #9ca3af; font-size: 13px; margin: 0; }

.settings-card {
  background: #fff;
  border-radius: 16px;
  padding: 24px;
  box-shadow: 0 4px 16px rgba(0,0,0,0.04);
  margin-bottom: 16px;
}
.card-title { font-size: 16px; margin: 0 0 16px; color: #1f2937; }

.lbl { font-size: 14px; font-weight: 600; color: #1f2937; }
.lbl-desc { font-size: 12px; color: #9ca3af; font-weight: normal; margin-top: 2px; line-height: 1.6; }

.tier-table { background: #f9fafb; border-radius: 10px; padding: 12px; }
.tier-head, .tier-row {
  display: grid;
  grid-template-columns: 1.4fr 1.4fr 0.8fr 0.8fr 60px;
  gap: 8px;
  align-items: center;
}
.tier-head { font-size: 12px; color: #6b7280; padding: 4px 8px 8px; border-bottom: 1px solid #e5e7eb; margin-bottom: 8px; }
.tier-row { padding: 4px 0; }
.tier-row :deep(.el-input-number) { width: 100% !important; }
.ratio { font-size: 13px; padding: 0 4px; font-variant-numeric: tabular-nums; }
.ratio.ok { color: #10b981; }
.ratio.warn { color: #f59e0b; }
.ratio.bad { color: #ef4444; font-weight: 700; }

.save-bar {
  position: sticky;
  bottom: 0;
  text-align: center;
  padding: 16px;
  background: linear-gradient(180deg, transparent, #f5f7fa 30%);
}

.loading-block { text-align: center; padding: 60px 0; color: #9ca3af; }

@media (max-width: 600px) {
  .tier-head, .tier-row { grid-template-columns: 1fr 1fr 50px; }
  .tier-head span:nth-child(3), .tier-head span:nth-child(4),
  .tier-row .ratio:nth-child(3), .tier-row .ratio:nth-child(4) { display: none; }
}
</style>
