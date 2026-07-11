<template>
  <div class="soft-backdrop" aria-hidden="true">
    <!-- 底层渐变斑 (保留) -->
    <div class="blob b1"></div>
    <div class="blob b2"></div>
    <div class="blob b3"></div>
    <div class="blob b4"></div>
    <!-- 网格 (保留) -->
    <div class="grid"></div>
    <!-- 星空连线 canvas -->
    <canvas ref="cvs" class="net-canvas"></canvas>
    <!-- 独立闪烁星点 -->
    <div v-for="(s, i) in stars" :key="i" class="star"
         :style="{ top: s.y + '%', left: s.x + '%', width: s.size + 'px', height: s.size + 'px', animationDelay: s.delay + 's', animationDuration: s.dur + 's' }"></div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'

const cvs = ref(null)
const stars = ref([])
let raf = 0
let particles = []
let ctx = null
let W = 0, H = 0

function resize() {
  const el = cvs.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  const dpr = window.devicePixelRatio || 1
  el.width = rect.width * dpr
  el.height = rect.height * dpr
  W = rect.width
  H = rect.height
  ctx.scale(dpr, dpr)
}

function initParticles() {
  const isMobile = window.innerWidth < 640
  const count = isMobile ? 32 : 60
  particles = []
  for (let i = 0; i < count; i++) {
    particles.push({
      x: Math.random() * W,
      y: Math.random() * H,
      vx: (Math.random() - 0.5) * 0.35,
      vy: (Math.random() - 0.5) * 0.35,
      r: 1.2 + Math.random() * 1.6,
    })
  }
}

function draw() {
  if (!ctx) return
  ctx.clearRect(0, 0, W, H)

  // 更新位置
  for (const p of particles) {
    p.x += p.vx
    p.y += p.vy
    if (p.x < 0 || p.x > W) p.vx *= -1
    if (p.y < 0 || p.y > H) p.vy *= -1
  }

  // 连线 (距离 < 130px 才连, 越近越明显)
  const LINK = 130
  for (let i = 0; i < particles.length; i++) {
    for (let j = i + 1; j < particles.length; j++) {
      const dx = particles[i].x - particles[j].x
      const dy = particles[i].y - particles[j].y
      const dist = Math.sqrt(dx * dx + dy * dy)
      if (dist < LINK) {
        const alpha = (1 - dist / LINK) * 0.22
        ctx.strokeStyle = `rgba(102, 126, 234, ${alpha * 1.1})`
        ctx.lineWidth = 0.8
        ctx.beginPath()
        ctx.moveTo(particles[i].x, particles[i].y)
        ctx.lineTo(particles[j].x, particles[j].y)
        ctx.stroke()
      }
    }
  }

  // 画点
  for (const p of particles) {
    ctx.fillStyle = 'rgba(102, 126, 234, 0.75)'
    ctx.beginPath()
    ctx.arc(p.x, p.y, p.r, 0, Math.PI * 2)
    ctx.fill()
  }

  raf = requestAnimationFrame(draw)
}

let resizeHandler = null

onMounted(() => {
  ctx = cvs.value.getContext('2d')
  resize()
  initParticles()
  draw()

  resizeHandler = () => {
    resize()
    initParticles()
  }
  window.addEventListener('resize', resizeHandler)

  // 生成闪烁星点
  const arr = []
  const cnt = window.innerWidth < 640 ? 40 : 70
  for (let i = 0; i < cnt; i++) {
    arr.push({
      x: Math.random() * 100,
      y: Math.random() * 100,
      size: 1 + Math.random() * 2.5,
      delay: Math.random() * 6,
      dur: 3 + Math.random() * 5,
    })
  }
  stars.value = arr
})

onUnmounted(() => {
  cancelAnimationFrame(raf)
  if (resizeHandler) window.removeEventListener('resize', resizeHandler)
})
</script>

<style scoped>
.soft-backdrop {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
  z-index: 0;
}

.net-canvas {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

/* 渐变斑 */
.blob {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.4;
  animation: drift 20s ease-in-out infinite;
  will-change: transform;
}
.b1 { width: 480px; height: 480px; top: -100px; left: -120px; background: radial-gradient(circle, #a5b4fc 0%, transparent 70%); }
.b2 { width: 420px; height: 420px; top: 120px; right: -100px; background: radial-gradient(circle, #fbcfe8 0%, transparent 70%); animation-delay: -5s; }
.b3 { width: 380px; height: 380px; bottom: -80px; left: 30%; background: radial-gradient(circle, #bae6fd 0%, transparent 70%); animation-delay: -10s; }
.b4 { width: 340px; height: 340px; top: 40%; right: 20%; background: radial-gradient(circle, #ddd6fe 0%, transparent 70%); animation-delay: -15s; }

@keyframes drift {
  0%, 100% { transform: translate(0, 0) scale(1); }
  33% { transform: translate(40px, -30px) scale(1.05); }
  66% { transform: translate(-30px, 40px) scale(0.95); }
}

/* 网格 */
.grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(99, 102, 241, 0.04) 1px, transparent 1px),
    linear-gradient(90deg, rgba(99, 102, 241, 0.04) 1px, transparent 1px);
  background-size: 48px 48px;
  mask-image: radial-gradient(ellipse at center, black 30%, transparent 80%);
  -webkit-mask-image: radial-gradient(ellipse at center, black 30%, transparent 80%);
}

/* 闪烁星点 */
.star {
  position: absolute;
  border-radius: 50%;
  background: rgba(102, 126, 234, 0.9);
  box-shadow: 0 0 6px rgba(102, 126, 234, 0.55);
  animation: twinkle ease-in-out infinite;
}
@keyframes twinkle {
  0%, 100% { opacity: 0.15; transform: scale(0.8); }
  50% { opacity: 1; transform: scale(1.4); }
}

@media (max-width: 640px) {
  .blob { filter: blur(50px); }
}
</style>
