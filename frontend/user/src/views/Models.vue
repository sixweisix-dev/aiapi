<template>
  <div class="page">
    <div class="hero">
      <div class="hero-bg"></div>
      <div class="hero-emoji">🤖</div>
      <div class="hero-title">{{ t('models.heroTitle') }}</div>
      <div class="hero-sub">{{ t('models.heroSub') }}</div>
    </div>

    <div v-if="loading" class="data-card empty-tip">{{ t('models.loading') }}</div>
    <div v-else-if="models.length === 0" class="data-card empty-tip">{{ t('models.noModels') }}</div>
    <template v-else>
      <div class="group-tabs">
        <button v-for="g in groups" :key="g.slug" :class="['group-tab', activeGroup === g.slug ? 'active' : '']" @click="activeGroup = g.slug">
          {{ g.name }}
          <span class="tab-rate" v-if="Number(g.multiplier) !== 1">{{ Number(g.multiplier).toFixed(2) }}×</span>
        </button>
      </div>
      <div v-if="activeGroupDesc" class="group-desc-card">
        <span class="group-desc-emoji">📦</span>
        <span>{{ activeGroupDesc }}</span>
      </div>
      <div class="data-card">
        <div class="card-header">
          <span class="card-title">{{ activeGroupName }}</span>
          <span class="card-tag">{{ filteredModels.length }} {{ t('models.countUnit') }}</span>
        </div>
        <div class="model-list">
          <div v-for="m in filteredModels" :key="m.id" class="model-card">
          <div class="model-head">
            <div class="model-name-block">
              <div class="model-name">{{ locale === 'en' && m.display_name_en ? m.display_name_en : m.display_name }}</div>
              <code class="model-id">{{ m.name }}</code>
            </div>
            <span class="provider-tag">{{ m.provider }}</span>
          </div>
          <!-- 按次计费模型 -->
          <div v-if="m.cost_per_call > 0" class="price-grid one-col">
            <div class="price-block image-price">
              <div class="price-label">{{ t('models.perCall') }}</div>
              <div class="price-value">${{ Number(m.cost_per_call).toFixed(4) }}</div>
              <div class="price-unit">{{ t('models.perImage') }}</div>
            </div>
          </div>
          <div v-else class="price-grid four-col">
            <div class="price-block">
              <div class="price-label">{{ t('models.input') }}</div>
              <div class="price-value">${{ Number(finalPrice(m.input_price, m.multiplier, m.group_multiplier) * 1000).toFixed(4) }}</div>
              <div class="price-unit">{{ t('models.perMillion') }}</div>
            </div>
            <div class="price-block">
              <div class="price-label">{{ t('models.output') }}</div>
              <div class="price-value">${{ Number(finalPrice(m.output_price, m.multiplier, m.group_multiplier) * 1000).toFixed(4) }}</div>
              <div class="price-unit">{{ t('models.perMillion') }}</div>
            </div>
            <div class="price-block">
              <div class="price-label">{{ t('models.cacheRead') }}</div>
              <div class="price-value">${{ Number(finalPrice(m.input_price, m.multiplier, m.group_multiplier) * 1000 * 0.1).toFixed(4) }}</div>
              <div class="price-unit">{{ t('models.perMillion') }}</div>
            </div>
            <div class="price-block">
              <div class="price-label">{{ t('models.cacheWrite') }}</div>
              <div class="price-value">${{ Number(finalPrice(m.input_price, m.multiplier, m.group_multiplier) * 1000 * 1.25).toFixed(4) }}</div>
              <div class="price-unit">{{ t('models.perMillion') }}</div>
            </div>
          </div>
          <div class="model-meta">
            <span>{{ t('models.context') }} {{ (m.context_length / 1000).toFixed(0) }}K</span>
            <span>·</span>
            <span v-if="Number((m.multiplier || 1) * (m.group_multiplier || 1)) !== 1">{{ t('models.multiplier') }} {{ Number((m.multiplier || 1) * (m.group_multiplier || 1)).toFixed(2) }}×</span>
          </div>
            <div v-if="m.description" class="model-desc">{{ m.description }}</div>
          </div>
        </div>
      </div>
    </template>

    <div class="data-card tip-card">
      <div class="tip-emoji">💡</div>
      <div class="tip-text">
        <div class="tip-title">{{ t('models.tipTitle') }}</div>
        <div class="tip-content">
          {{ t('models.tipText') }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
const { t, locale } = useI18n()
import { ref, computed, onMounted } from 'vue'
import { userModelsAPI } from '@/utils/api'

const loading = ref(true)
const models = ref([])
const activeGroup = ref('')

// final = base × model_multiplier × group_multiplier
const finalPrice = (base, mMul, gMul) => (base || 0) * (mMul || 1) * (gMul || 1)

const groups = computed(() => {
  const map = new Map()
  for (const m of models.value) {
    const slug = m.group_slug || 'default'
    if (!map.has(slug)) {
      const isEn = locale.value && locale.value.startsWith('en')
      map.set(slug, {
        slug,
        name: (isEn && m.group_name_en) ? m.group_name_en : (m.group_name || '默认分组'),
        multiplier: m.group_multiplier || 1,
        description: (isEn && m.group_description_en) ? m.group_description_en : (m.group_description || ''),
      })
    }
  }
  return Array.from(map.values()).sort((a, b) => (b.multiplier - a.multiplier))
})

const filteredModels = computed(() => {
  if (!activeGroup.value) return models.value
  return models.value.filter(m => (m.group_slug || 'default') === activeGroup.value)
})

const activeGroupName = computed(() => {
  const g = groups.value.find(x => x.slug === activeGroup.value)
  return g?.name || ''
})

const activeGroupDesc = computed(() => {
  const g = groups.value.find(x => x.slug === activeGroup.value)
  return g?.description || ''
})

onMounted(async () => {
  try {
    const data = await userModelsAPI.list()
    models.value = data.items || []
    if (groups.value.length > 0) activeGroup.value = groups.value[0].slug
  } catch {} finally { loading.value = false }
})
</script>

<style scoped>
.page { padding-bottom: 20px; }
.hero {
  position: relative;
  background: linear-gradient(135deg, #4facfe, #00f2fe);
  border-radius: 20px;
  padding: 24px 20px;
  color: #fff;
  margin-bottom: 14px;
  text-align: center;
  box-shadow: 0 10px 30px rgba(79,172,254,0.3);
  overflow: hidden;
}
.hero-bg {
  position: absolute;
  top: -40px;
  right: -40px;
  width: 140px;
  height: 140px;
  background: rgba(255,255,255,0.12);
  border-radius: 50%;
}
.hero-emoji { font-size: 36px; position: relative; z-index: 1; }
.hero-title { font-size: 22px; font-weight: 800; margin-top: 6px; position: relative; z-index: 1; }
.hero-sub { font-size: 13px; opacity: 0.95; margin-top: 4px; position: relative; z-index: 1; }

.data-card {
  background: #fff;
  border-radius: 14px;
  padding: 16px;
  margin-bottom: 14px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.04);
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 14px;
}
.card-title { font-size: 15px; font-weight: 600; color: #1f2937; }
.card-tag {
  background: #eef2ff;
  color: #6366f1;
  padding: 2px 10px;
  border-radius: 10px;
  font-size: 12px;
}
.empty-tip { text-align: center; color: #9ca3af; padding: 30px 0; font-size: 13px; }

.model-list { display: flex; flex-direction: column; gap: 12px; }
.model-card {
  border: 1px solid #f3f4f6;
  border-radius: 14px;
  padding: 14px;
  background: linear-gradient(135deg, #fafbfc, #fff);
}
.model-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 12px;
  gap: 8px;
}
.model-name-block { flex: 1; min-width: 0; }
.model-name {
  font-size: 16px;
  font-weight: 700;
  color: #1f2937;
  margin-bottom: 4px;
}
.model-id {
  font-family: 'SF Mono', monospace;
  background: #f3f4f6;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 11px;
  color: #6b7280;
}
.provider-tag {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  font-size: 10px;
  font-weight: 600;
  padding: 3px 10px;
  border-radius: 8px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  white-space: nowrap;
}

.price-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin-bottom: 10px;
}
.price-block {
  background: #fff;
  border: 1px solid #f3f4f6;
  border-radius: 10px;
  padding: 10px;
  text-align: center;
}
.price-label { font-size: 11px; color: #6b7280; margin-bottom: 4px; }
.price-value { font-size: 16px; font-weight: 700; color: #667eea; }
.price-unit { font-size: 10px; color: #9ca3af; margin-top: 2px; }

.model-meta {
  display: flex;
  gap: 6px;
  font-size: 11px;
  color: #9ca3af;
  margin-bottom: 6px;
  flex-wrap: wrap;
}
.model-desc {
  font-size: 12px;
  color: #6b7280;
  line-height: 1.5;
  padding-top: 8px;
  border-top: 1px dashed #f3f4f6;
}

.tip-card {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  background: linear-gradient(135deg, #fef3c7, #fde68a);
}
.tip-emoji { font-size: 22px; }
.tip-text { flex: 1; }
.tip-title { font-size: 13px; font-weight: 700; color: #78350f; margin-bottom: 4px; }
.tip-content { font-size: 12px; color: #92400e; line-height: 1.5; }
.price-grid.four-col { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
@media (min-width: 600px) { .price-grid.four-col { grid-template-columns: repeat(4, 1fr); } }

.group-tabs {
  display: flex;
  gap: 8px;
  margin: 12px 16px;
  overflow-x: auto;
  padding-bottom: 4px;
}
.group-tab {
  flex-shrink: 0;
  padding: 8px 16px;
  border-radius: 12px;
  border: 1.5px solid var(--el-border-color, #d4d8de);
  background: var(--el-bg-color, #fff);
  font-size: 14px;
  font-weight: 500;
  color: var(--el-text-color-primary, #2c3e50);
  cursor: pointer;
  transition: all 0.18s;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.group-tab.active {
  background: linear-gradient(135deg, #4facfe, #00f2fe);
  border-color: #4facfe;
  color: #fff;
  box-shadow: 0 2px 8px rgba(79,172,254,0.35);
}
.tab-rate {
  font-size: 12px;
  font-weight: 600;
  opacity: 0.85;
}
.group-desc-card {
  margin: 0 16px 12px;
  padding: 12px 14px;
  background: rgba(79,172,254,0.08);
  border-left: 3px solid #4facfe;
  border-radius: 8px;
  font-size: 13px;
  color: var(--el-text-color-regular, #555);
  line-height: 1.5;
  display: flex;
  align-items: flex-start;
  gap: 10px;
}
.group-desc-emoji {
  font-size: 18px;
  line-height: 1.2;
}

.price-grid.one-col {
  grid-template-columns: 1fr !important;
}

</style>
