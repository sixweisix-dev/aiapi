<template>
  <div class="auth-page">
    <FloatingBubbles />
    <button class="auth-lang-toggle" type="button" @click="toggleAuthLang">{{ authPageLang === 'zh' ? 'EN' : '中' }}</button>
    <div class="auth-card">
      <div class="auth-logo">📮</div>
      <h1 class="auth-brand">{{ t('forgot.title') }}</h1>
      <p class="auth-tagline">{{ t('forgot.subtitle') }}</p>

      <div v-if="!sent">
        <el-form ref="formRef" :model="form" :rules="rules" class="auth-form">
          <el-form-item prop="email">
            <el-input v-model="form.email" :placeholder="t('forgot.emailPlaceholder')" size="large" />
          </el-form-item>
          <TurnstileWidget ref="tsRef" v-model="turnstileToken" />
          <button type="button" class="auth-btn" :disabled="loading" @click="handleSubmit">
            {{ loading ? t('forgot.submitting') : t('forgot.submitBtn') }}
          </button>
        </el-form>
      </div>

      <div v-else class="success-block">
        <div class="success-emoji">✉️</div>
        <div class="success-title">{{ t('forgot.successTitle') }}</div>
        <div class="success-sub">请检查收件箱（包括垃圾邮件）</div>
      </div>

      <div class="auth-links">
        <router-link to="/login" class="auth-link">{{ t('forgot.backLogin') }}</router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import FloatingBubbles from '@/components/FloatingBubbles.vue'
import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { setLocale, currentLocale } from '@/i18n'
import { useI18n } from 'vue-i18n'
import api from '@/utils/api'
import TurnstileWidget from '@/components/TurnstileWidget.vue'

const authPageLang = ref(currentLocale())
function toggleAuthLang() {
  const next = authPageLang.value === 'zh' ? 'en' : 'zh'
  setLocale(next)
  authPageLang.value = next
}

const formRef = ref(null)
const tsRef = ref(null)
const { t } = useI18n()
const loading = ref(false)
const sent = ref(false)
const turnstileToken = ref('')
const form = reactive({ email: '' })
const rules = {
  email: [{ required: true, message: '请输入邮箱', trigger: 'blur' }],
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  if (!turnstileToken.value) {
    return ElMessage.warning(t('forgot.completeTs'))
  }
  loading.value = true
  try {
    await api.post('/auth/forgot-password', { email: form.email, turnstile_token: turnstileToken.value })
    sent.value = true
  } catch {
    tsRef.value?.reset()
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.auth-page {
  position: fixed;
  inset: 0;
  width: 100vw;
  height: 100vh;
  height: 100dvh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: linear-gradient(135deg, #f8f7ff 0%, #eef2ff 50%, #faf5ff 100%);
  overflow: hidden;
  box-sizing: border-box;
}
.auth-page::before { content: ''; position: absolute; top: -100px; right: -100px; width: 300px; height: 300px; background: rgba(255,255,255,0.08); border-radius: 50%; }
.auth-page::after { content: ''; position: absolute; bottom: -80px; left: -80px; width: 240px; height: 240px; background: rgba(255,255,255,0.06); border-radius: 50%; }
.auth-card { width: 100%; max-width: 380px; background: #fff; border-radius: 24px; padding: 36px 26px; box-shadow: 0 20px 60px rgba(0,0,0,0.2); position: relative; z-index: 1; }
.auth-logo { font-size: 44px; text-align: center; }
.auth-brand { font-size: 24px; font-weight: 800; text-align: center; background: linear-gradient(135deg, #667eea, #764ba2); -webkit-background-clip: text; -webkit-text-fill-color: transparent; background-clip: text; margin: 4px 0 0; }
.auth-tagline { text-align: center; color: #9ca3af; font-size: 13px; margin: 6px 0 24px; }
.auth-form { margin-bottom: 16px; }
.auth-btn { width: 100%; height: 48px; border: none; border-radius: 14px; background: linear-gradient(135deg, #667eea, #764ba2); color: #fff; font-size: 16px; font-weight: 700; cursor: pointer; box-shadow: 0 6px 16px rgba(102,126,234,0.35); }
.auth-btn:active { transform: scale(0.98); }
.auth-btn:disabled { opacity: 0.6; }
.auth-links { text-align: center; font-size: 13px; color: #9ca3af; margin-top: 8px; }
.auth-link { color: #667eea; text-decoration: none; font-weight: 600; }
.success-block { text-align: center; padding: 24px 0 16px; }
.success-emoji { font-size: 56px; margin-bottom: 12px; }
.success-title { font-size: 16px; color: #1f2937; font-weight: 600; margin-bottom: 4px; }
.success-sub { font-size: 13px; color: #9ca3af; }

.auth-lang-toggle {
  position: fixed;
  top: 20px;
  right: 20px;
  z-index: 100;
  background: rgba(255,255,255,0.2);
  backdrop-filter: blur(8px);
  color: #fff;
  border: 1px solid rgba(255,255,255,0.3);
  border-radius: 10px;
  padding: 8px 14px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.2s;
}
.auth-lang-toggle:hover { background: rgba(255,255,255,0.35); }

/* ==== 磨砂玻璃卡片 (覆盖上面 background: #fff) ==== */
.auth-card {
  background: rgba(255, 255, 255, 0.20) !important;
  backdrop-filter: blur(10px) saturate(1.2);
  -webkit-backdrop-filter: blur(10px) saturate(1.2);
  border: 1px solid rgba(255, 255, 255, 0.6);
  box-shadow:
    0 20px 60px rgba(31, 38, 135, 0.25),
    inset 0 1px 0 rgba(255, 255, 255, 0.7) !important;
}

/* ==== 磨砂玻璃上文字清晰度增强 ==== */
.auth-tagline { color: #1f2937 !important; font-weight: 500; text-shadow: 0 1px 2px rgba(255,255,255,0.6); }
.auth-links { color: #1f2937 !important; font-weight: 500; }
.auth-link { color: #312e81 !important; font-weight: 700; }
.agree-text { color: #111827 !important; font-weight: 500; }
.agree-link { color: #312e81 !important; font-weight: 700; }
.auth-card :deep(.el-input__wrapper) {
  background: rgba(255, 255, 255, 0.7) !important;
  box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.08) inset !important;
}
.auth-card :deep(.el-input__inner) {
  color: #1f2937;
}

/* auth-brand 加深适配磨砂玻璃 */
.auth-brand {
  background: linear-gradient(135deg, #312e81, #5b21b6) !important;
  -webkit-background-clip: text !important;
  background-clip: text !important;
  -webkit-text-fill-color: transparent !important;
  color: transparent !important;
  filter: drop-shadow(0 1px 2px rgba(49, 46, 129, 0.35));
}
.auth-logo {
  filter: drop-shadow(0 2px 6px rgba(99, 102, 241, 0.4));
}
</style>
