<template>
  <div ref="container" class="turnstile-wrap"></div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import api from '@/utils/api'

const emit = defineEmits(['update:modelValue'])
const container = ref(null)
let widgetId = null
let siteKey = ''

const SCRIPT_SRC = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'

function loadScript() {
  return new Promise((resolve, reject) => {
    if (window.turnstile) return resolve()
    const existing = document.querySelector('script[data-turnstile]')
    if (existing) {
      existing.addEventListener('load', () => resolve())
      existing.addEventListener('error', reject)
      return
    }
    const s = document.createElement('script')
    s.src = SCRIPT_SRC
    s.async = true
    s.defer = true
    s.dataset.turnstile = '1'
    s.onload = () => resolve()
    s.onerror = reject
    document.head.appendChild(s)
  })
}

async function init() {
  try {
    if (!siteKey) {
      const cfg = await api.get('/auth/config')
      siteKey = cfg.turnstile_site_key || ''
    }
    if (!siteKey) {
      console.warn('[Turnstile] site key empty')
      return
    }
    await loadScript()
    if (!window.turnstile || !container.value) return
    widgetId = window.turnstile.render(container.value, {
      sitekey: siteKey,
      callback: (token) => emit('update:modelValue', token),
      'expired-callback': () => emit('update:modelValue', ''),
      'error-callback': () => emit('update:modelValue', ''),
    })
  } catch (e) {
    console.error('[Turnstile] init failed', e)
  }
}

function reset() {
  if (window.turnstile && widgetId !== null) {
    try { window.turnstile.reset(widgetId) } catch (e) {}
  }
  emit('update:modelValue', '')
}

defineExpose({ reset })

onMounted(init)
onBeforeUnmount(() => {
  if (window.turnstile && widgetId !== null) {
    try { window.turnstile.remove(widgetId) } catch (e) {}
  }
})
</script>

<style scoped>
.turnstile-wrap { display: flex; justify-content: center; margin: 4px 0 14px; min-height: 65px; }
</style>
