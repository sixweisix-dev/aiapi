<template>
  <div class="auth-page">
    <div class="auth-card">
      <div class="auth-logo">📮</div>
      <h1 class="auth-brand">找回密码</h1>
      <p class="auth-tagline">输入注册邮箱，我们将发送重置链接</p>

      <div v-if="!sent">
        <el-form ref="formRef" :model="form" :rules="rules" class="auth-form">
          <el-form-item prop="email">
            <el-input v-model="form.email" placeholder="📧 注册邮箱" size="large" />
          </el-form-item>
          <el-form-item prop="captcha">
            <div style="display:flex;gap:8px;width:100%">
              <el-input v-model="form.captcha" placeholder="🛡️ 验证码" size="large" style="flex:1" />
              <img :src="captchaUrl" @click="refreshCaptcha"
                style="height:40px;border-radius:8px;cursor:pointer;min-width:110px;object-fit:cover" />
            </div>
          </el-form-item>
          <button class="auth-btn" :disabled="loading" @click="handleSubmit">
            {{ loading ? '发送中...' : '发送重置链接' }}
          </button>
        </el-form>
      </div>

      <div v-else class="success-block">
        <div class="success-emoji">✉️</div>
        <div class="success-title">重置链接已发送</div>
        <div class="success-sub">请检查收件箱（包括垃圾邮件）</div>
      </div>

      <div class="auth-links">
        <router-link to="/login" class="auth-link">返回登录</router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import api from '@/utils/api'

const formRef = ref(null)
const loading = ref(false)
const sent = ref(false)
const captchaId = ref('')
const captchaUrl = ref('')
const form = reactive({ email: '', captcha: '' })
const rules = {
  email: [{ required: true, message: '请输入邮箱', trigger: 'blur' }],
  captcha: [{ required: true, message: '请输入验证码', trigger: 'blur' }],
}

async function refreshCaptcha() {
  const res = await api.get('/auth/captcha/new')
  captchaId.value = res.captcha_id
  captchaUrl.value = res.captcha_url
}
onMounted(refreshCaptcha)

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  loading.value = true
  try {
    await api.post('/auth/forgot-password', { email: form.email, captcha_id: captchaId.value, captcha_answer: form.captcha })
    sent.value = true
  } catch { refreshCaptcha(); form.captcha = '' } finally { loading.value = false }
}
</script>

<style scoped>
.auth-page {
  min-height: 100vh; display: flex; align-items: center; justify-content: center;
  padding: 20px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  position: relative; overflow: hidden;
}
.auth-page::before {
  content: ''; position: absolute; top: -100px; right: -100px;
  width: 300px; height: 300px; background: rgba(255,255,255,0.08); border-radius: 50%;
}
.auth-page::after {
  content: ''; position: absolute; bottom: -80px; left: -80px;
  width: 240px; height: 240px; background: rgba(255,255,255,0.06); border-radius: 50%;
}
.auth-card {
  width: 100%; max-width: 380px;
  background: #fff; border-radius: 24px; padding: 36px 26px;
  box-shadow: 0 20px 60px rgba(0,0,0,0.2);
  position: relative; z-index: 1;
}
.auth-logo { font-size: 44px; text-align: center; }
.auth-brand {
  font-size: 24px; font-weight: 800; text-align: center;
  background: linear-gradient(135deg, #667eea, #764ba2);
  -webkit-background-clip: text; -webkit-text-fill-color: transparent;
  background-clip: text; margin: 4px 0 0;
}
.auth-tagline { text-align: center; color: #9ca3af; font-size: 13px; margin: 6px 0 24px; }
.auth-form { margin-bottom: 16px; }
.auth-btn {
  width: 100%; height: 48px; border: none; border-radius: 14px;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff; font-size: 16px; font-weight: 700; cursor: pointer;
  box-shadow: 0 6px 16px rgba(102,126,234,0.35);
}
.auth-btn:active { transform: scale(0.98); }
.auth-btn:disabled { opacity: 0.6; }
.auth-links { text-align: center; font-size: 13px; color: #9ca3af; margin-top: 8px; }
.auth-link { color: #667eea; text-decoration: none; font-weight: 600; }

.success-block { text-align: center; padding: 24px 0 16px; }
.success-emoji { font-size: 56px; margin-bottom: 12px; }
.success-title { font-size: 16px; color: #1f2937; font-weight: 600; margin-bottom: 4px; }
.success-sub { font-size: 13px; color: #9ca3af; }
</style>
