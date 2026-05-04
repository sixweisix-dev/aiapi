<template>
  <div class="legal-page">
    <header class="legal-header">
      <button class="back-btn" @click="$router.back()">{{ t('legal.back') }}</button>
      <span class="legal-title">{{ t('legal.privacyTitle') }}</span>
    </header>

    <div class="legal-content" v-html="t('legal.privacyBody')"></div>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
</script>

<style scoped>
.legal-page { min-height: 100vh; background: #f5f7fa; }
.legal-header {
  position: sticky;
  top: 0;
  background: #fff;
  height: 56px;
  display: flex;
  align-items: center;
  padding: 0 16px;
  gap: 12px;
  border-bottom: 1px solid #f3f4f6;
  z-index: 10;
}
.back-btn {
  background: none; border: none;
  font-size: 14px; color: #667eea;
  cursor: pointer; font-weight: 600;
}
.legal-title { font-size: 16px; font-weight: 700; color: #1f2937; }
.legal-content {
  max-width: 720px;
  margin: 0 auto;
  padding: 20px 18px 40px;
  background: #fff;
  font-size: 14px; line-height: 1.75; color: #374151;
}
.legal-content h2 { font-size: 20px; font-weight: 800; color: #1f2937; margin: 0 0 6px; }
.legal-meta { color: #9ca3af; font-size: 12px; margin: 0 0 18px; }
.legal-content h3 {
  font-size: 15px; font-weight: 700; color: #1f2937;
  margin: 22px 0 10px; padding-left: 8px;
  border-left: 3px solid #667eea;
}
.legal-content p { margin: 0 0 10px; }
.legal-content strong { color: #ef4444; }
</style>
