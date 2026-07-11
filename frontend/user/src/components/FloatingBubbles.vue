<template>
  <div class="floating-bg" aria-hidden="true">
    <div v-for="(b, i) in bubbles" :key="i"
         class="bubble"
         :style="{
           left: b.x + '%',
           top: b.y + '%',
           width: b.size + 'px',
           height: b.size + 'px',
           background: b.bg,
           color: b.color,
           animationDuration: b.dur + 's',
           animationDelay: b.delay + 's',
         }">
      <div v-if="b.svg" class="logo-wrap" v-html="b.svg" :style="{ width: b.size * 0.55 + 'px', height: b.size * 0.55 + 'px' }"></div>
      <span v-else class="text-icon" :style="{ fontSize: b.size * 0.42 + 'px' }">{{ b.text }}</span>
    </div>
    <div class="glow g1"></div>
    <div class="glow g2"></div>
    <div class="glow g3"></div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import {
  siAnthropic,
  siGooglegemini,
  siMeta,
  siHuggingface,
  siPerplexity,
  siMistralai,
  siAlibabacloud,
  siBaidu,
  siGoogle,
  siDeepseek,
  siQwen,
} from 'simple-icons'

function makeSvg(icon) {
  return `<svg viewBox="0 0 24 24" fill="currentColor" width="100%" height="100%"><path d="${icon.path}"/></svg>`
}

// OpenAI simple-icons 没有,内嵌官方 SVG
const OPENAI_SVG = `<svg viewBox="0 0 24 24" fill="currentColor" width="100%" height="100%"><path d="M22.282 9.821a5.985 5.985 0 0 0-.516-4.91 6.046 6.046 0 0 0-6.51-2.9A6.065 6.065 0 0 0 4.981 4.18a5.985 5.985 0 0 0-3.998 2.9 6.046 6.046 0 0 0 .743 7.097 5.98 5.98 0 0 0 .51 4.911 6.051 6.051 0 0 0 6.515 2.9A5.985 5.985 0 0 0 13.26 24a6.056 6.056 0 0 0 5.772-4.206 5.99 5.99 0 0 0 3.997-2.9 6.056 6.056 0 0 0-.747-7.073zM13.26 22.43a4.476 4.476 0 0 1-2.876-1.04l.141-.081 4.774-2.759a.795.795 0 0 0 .392-.681v-6.737l2.02 1.168a.071.071 0 0 1 .038.052v5.583a4.504 4.504 0 0 1-4.489 4.495zM3.6 18.304a4.47 4.47 0 0 1-.535-3.014l.142.085 4.783 2.759a.771.771 0 0 0 .78 0l5.843-3.369v2.332a.08.08 0 0 1-.033.062L9.74 19.95a4.5 4.5 0 0 1-6.14-1.646zM2.34 7.896a4.485 4.485 0 0 1 2.366-1.973V11.6a.766.766 0 0 0 .388.676l5.815 3.355-2.02 1.168a.076.076 0 0 1-.071 0l-4.83-2.786A4.504 4.504 0 0 1 2.34 7.872zm16.597 3.855-5.833-3.387L15.119 7.2a.076.076 0 0 1 .071 0l4.83 2.791a4.494 4.494 0 0 1-.676 8.105v-5.678a.79.79 0 0 0-.407-.667zm2.01-3.023-.141-.085-4.774-2.782a.776.776 0 0 0-.785 0L9.409 9.23V6.897a.066.066 0 0 1 .028-.061l4.83-2.787a4.5 4.5 0 0 1 6.68 4.66zm-12.64 4.135-2.02-1.164a.08.08 0 0 1-.038-.057V6.075a4.5 4.5 0 0 1 7.375-3.453l-.142.08L8.704 5.46a.79.79 0 0 0-.393.681zm1.097-2.365 2.602-1.5 2.607 1.5v2.999l-2.597 1.5-2.607-1.5Z"/></svg>`

const BRANDS = [
  { svg: OPENAI_SVG,               bg: 'linear-gradient(135deg, #10a37f, #1a7f5a)', color: '#fff' },
  { svg: makeSvg(siAnthropic),     bg: 'linear-gradient(135deg, #d97757, #cc785c)', color: '#fff' },
  { svg: makeSvg(siGooglegemini),  bg: 'linear-gradient(135deg, #4285f4, #9b72f2)', color: '#fff' },
  { svg: makeSvg(siGoogle),        bg: 'linear-gradient(135deg, #ea4335, #34a853)', color: '#fff' },
  { svg: makeSvg(siMeta),          bg: 'linear-gradient(135deg, #0866ff, #0064e0)', color: '#fff' },
  { svg: makeSvg(siHuggingface),   bg: 'linear-gradient(135deg, #ffd21e, #ff9d00)', color: '#1a1a1a' },
  { svg: makeSvg(siPerplexity),    bg: 'linear-gradient(135deg, #20808d, #1a6b76)', color: '#fff' },
  { svg: makeSvg(siMistralai),     bg: 'linear-gradient(135deg, #ff7000, #f04e00)', color: '#fff' },
  { svg: makeSvg(siAlibabacloud),  bg: 'linear-gradient(135deg, #ff6a00, #ee0a24)', color: '#fff' },
  { svg: makeSvg(siBaidu),         bg: 'linear-gradient(135deg, #2932e1, #1e2599)', color: '#fff' },
  // 国内 AI 厂商 (无官方 SVG, 文字 fallback)
  { svg: makeSvg(siDeepseek),    bg: 'linear-gradient(135deg, #4d6bfe, #2b4bde)', color: '#fff' },  // DeepSeek
  { svg: makeSvg(siQwen),        bg: 'linear-gradient(135deg, #615ced, #8b5cf6)', color: '#fff' },  // 通义千问 Qwen
  { text: 'Kimi', bg: 'linear-gradient(135deg, #ff5757, #d94848)', color: '#fff' },  // Kimi
  { text: '智',   bg: 'linear-gradient(135deg, #6366f1, #4f46e5)', color: '#fff' },  // 智谱 GLM
  { text: 'X',    bg: 'linear-gradient(135deg, #1a1a1a, #333333)', color: '#fff' },  // xAI Grok
  { text: 'M',    bg: 'linear-gradient(135deg, #00c9ff, #92fe9d)', color: '#1a1a1a' },// MiniMax
]

const bubbles = ref([])

onMounted(() => {
  const list = []
  const count = 16
  for (let i = 0; i < count; i++) {
    const b = BRANDS[i % BRANDS.length]
    list.push({
      x: Math.random() * 90 + 2,
      y: Math.random() * 85 + 2,
      size: 52 + Math.random() * 40,
      bg: b.bg,
      color: b.color,
      svg: b.svg,
      text: b.text,
      dur: 14 + Math.random() * 12,
      delay: -Math.random() * 20,
    })
  }
  bubbles.value = list
})
</script>

<style scoped>
.floating-bg {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
  z-index: 0;
}

.bubble {
  position: absolute;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow:
    0 12px 30px rgba(0,0,0,0.2),
    inset -6px -6px 12px rgba(0,0,0,0.2),
    inset 6px 6px 12px rgba(255,255,255,0.35);
  animation: float linear infinite;
  will-change: transform;
  opacity: 0.9;
  font-weight: 900;
  font-family: -apple-system, sans-serif;
}
.logo-wrap {
  filter: drop-shadow(0 2px 4px rgba(0,0,0,0.25));
  display: flex;
  align-items: center;
  justify-content: center;
}
.text-icon {
  filter: drop-shadow(0 2px 4px rgba(0,0,0,0.25));
  user-select: none;
  font-weight: 900;
}

@keyframes float {
  0%   { transform: translate(0, 0) rotate(0deg); }
  25%  { transform: translate(30px, -40px) rotate(90deg); }
  50%  { transform: translate(-20px, -70px) rotate(180deg); }
  75%  { transform: translate(-40px, -30px) rotate(270deg); }
  100% { transform: translate(0, 0) rotate(360deg); }
}

.glow {
  position: absolute;
  border-radius: 50%;
  filter: blur(60px);
  opacity: 0.5;
  animation: pulse 8s ease-in-out infinite;
}
.g1 { width: 320px; height: 320px; top: -80px; left: -80px; background: radial-gradient(circle, #10a37f, transparent 70%); }
.g2 { width: 400px; height: 400px; bottom: -120px; right: -100px; background: radial-gradient(circle, #4285f4, transparent 70%); animation-delay: -3s; }
.g3 { width: 260px; height: 260px; top: 40%; left: 60%; background: radial-gradient(circle, #d97757, transparent 70%); animation-delay: -5s; }

@keyframes pulse {
  0%, 100% { opacity: 0.4; transform: scale(1); }
  50% { opacity: 0.7; transform: scale(1.1); }
}

@media (max-width: 640px) {
  .bubble:nth-child(n+13) { display: none; }
  .glow { filter: blur(40px); }
}
</style>
