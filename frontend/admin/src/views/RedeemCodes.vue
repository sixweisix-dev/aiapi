<template>
  <div class="page">
    <!-- 顶部操作栏 -->
    <div class="data-card filter-card">
      <div class="filter-title">🎁 兑换码管理</div>
      <div class="filter-row">
        <el-select v-model="filter.status" placeholder="状态" clearable size="small" style="width:110px">
          <el-option label="未使用" value="unused" />
          <el-option label="已使用" value="used" />
        </el-select>
        <el-input v-model="filter.note" placeholder="备注/档位" clearable size="small" style="width:160px" />
        <el-input v-model="filter.code" placeholder="搜索兑换码" clearable size="small" style="width:180px" />
        <el-button type="primary" size="small" @click="fetchCodes">🔍 查询</el-button>
        <el-button size="small" @click="exportCodes">📥 导出未使用</el-button>
        <el-button type="success" size="small" @click="showGenerate = true">+ 生成新码</el-button>
      </div>
    </div>

    <!-- 库存概览 -->
    <div class="stock-grid">
      <div
        v-for="s in stockList" :key="s.note"
        class="stock-card" :class="{ low: s.unused <= s.threshold, expanded: expandedNote === s.note }"
        @click="toggleExpand(s.note)"
      >
        <div class="stock-card-header">
          <div class="stock-note">{{ s.note }}</div>
          <span class="stock-arrow">{{ expandedNote === s.note ? '▲' : '▼' }}</span>
        </div>
        <div class="stock-count">
          <span class="stock-num" :class="{ low: s.unused <= s.threshold }">{{ s.unused }}</span>
          <span class="stock-unit">个可用</span>
        </div>
        <div class="stock-bar">
          <div class="stock-bar-fill" :style="{ width: Math.min(100, s.unused / s.total * 100) + '%', background: s.unused <= s.threshold ? '#f56c6c' : '#67c23a' }"></div>
        </div>
        <div class="stock-meta">总计 {{ s.total }} 个 · 已用 {{ s.used }} 个</div>
        <el-button v-if="s.unused <= s.threshold" size="small" type="danger" @click.stop="triggerRestock">⚠️ 立即补货</el-button>

        <!-- 展开的兑换码列表 -->
        <div v-if="expandedNote === s.note" class="card-code-list" @click.stop>
          <div v-if="cardCodesLoading" class="card-codes-empty">加载中...</div>
          <div v-else-if="groupedCodes[s.note]?.length === 0" class="card-codes-empty">暂无数据</div>
          <div v-else class="code-list">
            <div v-for="c in groupedCodes[s.note]" :key="c.id" class="code-item">
              <div class="code-main">
                <span class="code-text">{{ c.code }}</span>
                <el-tag :type="c.status === 'unused' ? 'success' : 'info'" size="small">{{ c.status === 'unused' ? '未使用' : '已使用' }}</el-tag>
              </div>
              <div class="code-meta">
                <span v-if="c.balance_amount > 0">余额 ¥{{ c.balance_amount }}</span>
                <span v-if="c.membership_tier !== 'free'">{{ c.membership_tier }} {{ c.membership_days }}天</span>
                <span v-if="c.redeemed_at">{{ dayjs(c.redeemed_at).format('MM-DD HH:mm') }} 兑换</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>



    <!-- 生成对话框 -->
    <el-dialog v-model="showGenerate" title="批量生成兑换码" width="360px">
      <el-form label-position="top">
        <el-form-item label="档位">
          <el-select v-model="genForm.preset" placeholder="选择档位（从系统设置阶梯同步）" @change="applyPreset" style="width:100%">
            <el-option v-for="opt in tierOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="数量">
          <el-input-number v-model="genForm.count" :min="1" :max="500" style="width:100%" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="genForm.note" />
        </el-form-item>
        <el-form-item label="有效期（天）">
          <el-input-number v-model="genForm.expiry_days" :min="1" :max="365" style="width:100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showGenerate = false">取消</el-button>
        <el-button type="primary" :loading="generating" @click="doGenerate">生成</el-button>
      </template>
    </el-dialog>

    <!-- 生成结果 -->
    <el-dialog v-model="showResult" title="生成成功" width="400px">
      <div class="result-info">生成了 {{ resultCodes.length }} 个兑换码，请复制到闲鱼自动发货库存：</div>
      <el-input type="textarea" :value="resultCodes.join('\n')" :rows="10" readonly />
      <template #footer>
        <el-button type="primary" @click="copyAll">复制全部</el-button>
        <el-button @click="showResult = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/utils/api'
import dayjs from 'dayjs'

const codes = ref([])
const loading = ref(false)
const showGenerate = ref(false)
const showResult = ref(false)
const generating = ref(false)
const resultCodes = ref([])
const stockList = ref([])
const settingTiers = ref([]) // 从 Settings 读取的阶梯规则
const firstBonus = ref(0)   // 首充礼金额

const filter = ref({ status: 'unused', note: '', code: '' })
const openGroups = ref([])
const expandedNote = ref(null)
const cardCodesLoading = ref(false)

// 加载阶梯规则（用于动态生成档位）
async function loadSettings() {
  try {
    const res = await api.get('/admin/settings')
    try { settingTiers.value = JSON.parse(res.recharge_tiers || '[]') } catch { settingTiers.value = [] }
    firstBonus.value = parseFloat(res.first_recharge_bonus || '0')
  } catch {}
}

// 动态档位列表（基于 settingTiers）
const tierOptions = computed(() => {
  const opts = settingTiers.value.map(t => ({
    value: String(t.min),
    label: `¥${t.min} 充值码（到账 ¥${(t.min + t.bonus).toFixed(0)}）`,
    type: 'balance',
    balance_amount: t.min + t.bonus,
    face_value: t.min,
    note: `闲鱼¥${t.min}充值码`,
    expiry_days: 180,
    count: t.min >= 1000 ? 5 : t.min >= 500 ? 10 : 20,
  }))
  opts.push({ value: 'pro', label: '专业版会员30天', type: 'membership', balance_amount: 120, face_value: 99, membership_tier: 'pro', membership_days: 30, note: '闲鱼专业版30天', expiry_days: 180, count: 10 })
  opts.push({ value: 'enterprise', label: '企业版会员30天', type: 'membership', balance_amount: 600, face_value: 499, membership_tier: 'enterprise', membership_days: 30, note: '闲鱼企业版30天', expiry_days: 180, count: 5 })
  opts.push({ value: 'custom', label: '自定义' })
  return opts
})

async function toggleExpand(note) {
  if (expandedNote.value === note) {
    expandedNote.value = null
    return
  }
  expandedNote.value = note
  // 触发加载该分组数据
  if (!codes.value.length || !groupedCodes.value[note]) {
    cardCodesLoading.value = true
    await fetchCodes()
    cardCodesLoading.value = false
  }
}

const groupedCodes = computed(() => {
  const map = {}
  for (const c of codes.value) {
    const key = c.note || '未分类'
    if (!map[key]) map[key] = []
    map[key].push(c)
  }
  return map
})

const genForm = ref({ preset: '', type: 'balance', balance_amount: 0, face_value: 0, membership_tier: 'free', membership_days: 0, note: '', expiry_days: 180, count: 20 })

function applyPreset(val) {
  const opt = tierOptions.value.find(o => o.value === val)
  if (opt && val !== 'custom') {
    genForm.value.type = opt.type || 'balance'
    genForm.value.balance_amount = opt.balance_amount || 0
    genForm.value.face_value = opt.face_value || opt.balance_amount || 0
    genForm.value.membership_tier = opt.membership_tier || 'free'
    genForm.value.membership_days = opt.membership_days || 0
    genForm.value.note = opt.note || ''
    genForm.value.expiry_days = opt.expiry_days || 180
    genForm.value.count = opt.count || 10
  }
}

async function fetchCodes() {
  loading.value = true
  try {
    const params = {}
    if (filter.value.status) params.status = filter.value.status
    if (filter.value.note) params.batch_id = filter.value.note
    const res = await api.get('/admin/redeem-codes', { params })
    let list = res.codes || []
    if (filter.value.code) list = list.filter(c => c.code.includes(filter.value.code.toUpperCase()))
    if (filter.value.note) list = list.filter(c => (c.note || '').includes(filter.value.note))
    codes.value = list
    // 默认展开所有分组
    openGroups.value = [...new Set(list.map(c => c.note || '未分类'))]
  } catch (e) {
    ElMessage.error('加载失败')
  } finally {
    loading.value = false
  }
}

async function fetchStock() {
  try {
    const all = await api.get('/admin/redeem-codes', { params: {} })
    const list = all.codes || []
    // 按 note 自动分组，动态统计库存
    const map = {}
    for (const c of list) {
      const key = c.note || '未分类'
      if (!map[key]) map[key] = { note: key, unused: 0, used: 0, total: 0, threshold: 3 }
      map[key].total++
      if (c.status === 'unused') map[key].unused++
      else map[key].used++
    }
    // 按档位金额排序（会员放最后）
    stockList.value = Object.values(map).sort((a, b) => {
      const getNum = n => { const m = n.match(/¥(\d+)/); return m ? Number(m[1]) : 99999 }
      return getNum(a.note) - getNum(b.note)
    })
  } catch {}
}

async function triggerRestock() {
  try {
    const res = await api.post('/admin/restock')
    if (res.message) {
      ElMessage.info(res.message)
    } else {
      ElMessage.success(`已补货 ${res.restocked} 个档位，邮件通知将发送到告警邮箱`)
    }
    await fetchStock()
  } catch {
    ElMessage.error('补货失败')
  }
}

async function doGenerate() {
  generating.value = true
  try {
    const payload = {
      count: genForm.value.count,
      type: genForm.value.type,
      balance_amount: genForm.value.balance_amount,
      membership_tier: genForm.value.membership_tier || 'free',
      membership_days: genForm.value.membership_days || 0,
      expiry_days: genForm.value.expiry_days,
      note: genForm.value.note,
    }
    const res = await api.post('/admin/redeem-codes/generate', payload)
    resultCodes.value = res.codes || []
    showGenerate.value = false
    showResult.value = true
    ElMessage.success(`生成 ${resultCodes.value.length} 个兑换码`)
    await fetchStock()
  } catch {
    ElMessage.error('生成失败')
  } finally {
    generating.value = false
  }
}

function copyAll() {
  navigator.clipboard.writeText(resultCodes.value.join('\n'))
  ElMessage.success('已复制到剪贴板')
}

function exportCodes() {
  const unused = codes.value.filter(c => c.status === 'unused').map(c => c.code).join('\n')
  const blob = new Blob([unused], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a'); a.href = url; a.download = 'unused_codes.txt'; a.click()
}

onMounted(async () => {
  await loadSettings()
  fetchCodes()
  fetchStock()
})
</script>

<style scoped>
.page { padding: 24px; max-width: 1200px; margin: 0 auto; }
.filter-title { font-size: 18px; font-weight: 700; color: #1f2937; margin-bottom: 12px; }

.stock-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 16px; margin-bottom: 24px; }
.stock-card { background: #fff; border-radius: 12px; padding: 16px; box-shadow: 0 2px 8px rgba(0,0,0,0.06); border: 1.5px solid #e5e7eb; }
.stock-card.low { border-color: #fca5a5; background: #fff7f7; }
.stock-note { font-size: 13px; color: #6b7280; margin-bottom: 8px; font-weight: 500; }
.stock-count { display: flex; align-items: baseline; gap: 4px; margin-bottom: 8px; }
.stock-num { font-size: 32px; font-weight: 800; color: #1f2937; }
.stock-num.low { color: #dc2626; }
.stock-unit { font-size: 13px; color: #9ca3af; }
.stock-bar { height: 4px; background: #f3f4f6; border-radius: 2px; margin-bottom: 8px; }
.stock-bar-fill { height: 100%; border-radius: 2px; transition: width 0.3s; }
.stock-meta { font-size: 11px; color: #9ca3af; margin-bottom: 8px; }

.filter-card { margin-bottom: 16px; }
.filter-row { display: flex; gap: 10px; flex-wrap: wrap; align-items: center; }

.data-card { background: #fff; border-radius: 12px; padding: 20px; box-shadow: 0 2px 8px rgba(0,0,0,0.06); margin-bottom: 16px; }
.card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.card-title { font-size: 15px; font-weight: 600; }
.card-tag { background: #eef2ff; color: #6366f1; font-size: 12px; padding: 2px 8px; border-radius: 10px; }
.empty { text-align: center; color: #9ca3af; padding: 40px 0; }

.code-list { display: flex; flex-direction: column; gap: 10px; }
.code-item { padding: 12px; border: 1px solid #f3f4f6; border-radius: 8px; }
.code-main { display: flex; align-items: center; gap: 10px; margin-bottom: 6px; }
.code-text { font-family: monospace; font-size: 15px; font-weight: 600; color: #1f2937; letter-spacing: 1px; }
.code-meta { display: flex; gap: 12px; font-size: 12px; color: #9ca3af; flex-wrap: wrap; }

.result-info { margin-bottom: 12px; font-size: 14px; color: #4b5563; }

.code-collapse { border: none; }
.code-collapse :deep(.el-collapse-item__header) {
  background: #f9fafb; border-radius: 10px; padding: 0 16px;
  font-size: 14px; border: 1px solid #e5e7eb; margin-bottom: 8px;
}
.code-collapse :deep(.el-collapse-item__wrap) { border: none; }
.code-collapse :deep(.el-collapse-item__content) { padding: 0 0 12px; }
.group-title { display: flex; align-items: center; justify-content: space-between; width: 100%; }
.group-note { font-weight: 600; color: #1f2937; }
.group-count { display: flex; align-items: center; margin-right: 24px; }

.stock-card { cursor: pointer; transition: box-shadow 0.2s; }
.stock-card:hover { box-shadow: 0 4px 16px rgba(0,0,0,0.1); }
.stock-card.expanded { border-color: #6366f1; }
.stock-card-header { display: flex; justify-content: space-between; align-items: center; }
.stock-arrow { font-size: 10px; color: #9ca3af; }
.card-code-list { margin-top: 14px; border-top: 1px solid #f3f4f6; padding-top: 12px; max-height: 400px; overflow-y: auto; }
.card-codes-empty { text-align: center; color: #9ca3af; padding: 20px 0; font-size: 13px; }
</style>
