<template>
  <div class="ambient-backdrop" aria-hidden="true">
    <div class="blob b1"></div>
    <div class="blob b2"></div>
    <div class="blob b3"></div>
  </div>
</template>

<style scoped>
.ambient-backdrop {
  position: fixed;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
  z-index: 0;
}
.blob {
  position: absolute;
  border-radius: 50%;
  filter: blur(100px);
  opacity: 0.55;
  animation: slowDrift 35s ease-in-out infinite;
  will-change: transform;
}
.b1 {
  width: 520px; height: 520px;
  top: -140px; left: -160px;
  background: radial-gradient(circle, #6366f1 0%, transparent 70%);
}
.b2 {
  width: 460px; height: 460px;
  bottom: -160px; right: -140px;
  background: radial-gradient(circle, #ec4899 0%, transparent 70%);
  animation-delay: -12s;
}
.b3 {
  width: 400px; height: 400px;
  top: 50%; left: 55%;
  background: radial-gradient(circle, #06b6d4 0%, transparent 70%);
  animation-delay: -22s;
}
@keyframes slowDrift {
  0%, 100% { transform: translate(0, 0) scale(1); }
  50% { transform: translate(30px, -20px) scale(1.08); }
}
@media (max-width: 640px) {
  .blob { filter: blur(60px); opacity: 0.4; }
}
</style>
