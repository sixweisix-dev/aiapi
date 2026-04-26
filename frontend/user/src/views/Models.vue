<template>
  <div class="page">
    <div class="hero">
      <div class="hero-bg"></div>
      <div class="hero-emoji">🤖</div>
      <div class="hero-title">模型与价格</div>
      <div class="hero-sub">Claude 系列模型 · OpenAI 兼容接口</div>
    </div>

    <div class="data-card">
      <div class="card-header">
        <span class="card-title">📋 可用模型</span>
        <span class="card-tag">{{ models.length }} 个</span>
      </div>
      <div v-if="loading" class="empty-tip">加载中...</div>
      <div v-else-if="models.length === 0" class="empty-tip">暂无可用模型</div>
      <div v-else class="model-list">
        <div v-for="m in models" :key="m.id" class="model-card">
          <div class="model-head">
            <div class="model-name-block">
              <div class="model-name">{{ m.display_name }}</div>
              <code class="model-id">{{ m.name }}</code>
            </div>
            <span class="provider-tag">{{ m.provider }}</span>
          </div>
          <div class="price-grid">
            <div class="price-block">
              <div class="price-label">📥 输入</div>
              <div class="price-value">¥{{ Number(finalPrice(m.input_price, m.multiplier)).toFixed(4) }}</div>
              <div class="price-unit">/ 1K tokens</div>
            </div>
            <div class="price-block">
              <div class="price-label">📤 输出</div>
              <div class="price-value">¥{{ Number(finalPrice(m.output_price, m.multiplier)).toFixed(4) }}</div>
              <div class="price-unit">/ 1K tokens</div>
            </div>
          </div>
          <div class="model-meta">
            <span>📐 上下文 {{ (m.context_length / 1000).toFixed(0) }}K</span>
            <span>·</span>
            <span>倍率 {{ m.multiplier }}x</span>
          </div>
          <div v-if="m.description" class="model-desc">{{ m.description }}</div>
        </div>
      </div>
    </div>

    <div class="data-card tip-card">
      <div class="tip-emoji">💡</div>
      <div class="tip-text">
        <div class="tip-title">价格说明</div>
        <div class="tip-content">
          所有价格以人民币计价 · 按实际 tokens 用量扣费 · 余额不足时返回 402
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { userModelsAPI } from '@/utils/api'

const loading = ref(true)
const models = ref([])

const finalPrice = (base, multiplier) => (base || 0) * (multiplier || 1)

onMounted(async () => {
  try {
    const data = await userModelsAPI.list()
    models.value = data.items || []
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
</style>
