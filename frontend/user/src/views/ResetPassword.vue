<template>
  <div class="auth-page">
    <div class="auth-card">
      <div class="auth-logo">🔐</div>
      <h1 class="auth-brand">重置密码</h1>
      <p class="auth-tagline">设置一个新的密码</p>

      <div v-if="!done">
        <el-form ref="formRef" :model="form" :rules="rules" class="auth-form">
          <el-form-item prop="new_password">
            <el-input v-model="form.new_password" type="password" placeholder="🔒 新密码（8位+大小写+数字）" size="large" show-password />
          </el-form-item>
          <el-form-item prop="confirm_password">
            <el-input v-model="form.confirm_password" type="password" placeholder="🔒 确认新密码" size="large" show-password />
          </el-form-item>
          <button class="auth-btn" :disabled="loading || !token" @click="handleSubmit">
            {{ loading ? '提交中...' : '重置密码' }}
          </button>
        </el-form>
        <p v-if="!token" class="error-tip">⚠️ 链接无效或已过期</p>
      </div>

      <div v-else class="success-block">
        <div class="success-emoji">✅</div>
        <div class="success-title">密码重置成功</div>
        <button class="auth-btn" style="margin-top:18px" @click="$router.push('/login')">去登录</button>
      </div>

      <div v-if="!done" class="auth-links">
        <router-link to="/login" class="auth-link">返回登录</router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import api from '@/utils/api'

const route = useRoute()
const formRef = ref(null)
const loading = ref(false)
const done = ref(false)
const token = ref('')
const form = reactive({ new_password: '', confirm_password: '' })

onMounted(() => { token.value = route.query.token || '' })

const passwordStrength = (v) => {
  if (v.length < 8) return '至少8位'
  if (!/[a-z]/.test(v)) return '需含小写字母'
  if (!/[A-Z]/.test(v)) return '需含大写字母'
  if (!/[0-9]/.test(v)) return '需含数字'
  return null
}
const rules = {
  new_password: [
    { required: true, trigger: 'blur' },
    { validator: (r, v, cb) => { const e = passwordStrength(v); e ? cb(new Error(e)) : cb() }, trigger: 'blur' }
  ],
  confirm_password: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    { validator: (r, v, cb) => v !== form.new_password ? cb(new Error('两次密码不一致')) : cb(), trigger: 'blur' }
  ]
}

async function handleSubmit() {
  if (!token.value) return
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  loading.value = true
  try {
    await api.post('/auth/reset-password', { token: token.value, new_password: form.new_password })
    done.value = true
  } catch {} finally { loading.value = false }
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
.auth-form { margin-bottom: 12px; }
.auth-btn {
  width: 100%; height: 48px; border: none; border-radius: 14px;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff; font-size: 16px; font-weight: 700; cursor: pointer;
  box-shadow: 0 6px 16px rgba(102,126,234,0.35);
}
.auth-btn:active { transform: scale(0.98); }
.auth-btn:disabled { opacity: 0.6; }
.error-tip { color: #ef4444; font-size: 13px; text-align: center; margin: 12px 0 0; }
.auth-links { text-align: center; font-size: 13px; color: #9ca3af; margin-top: 14px; }
.auth-link { color: #667eea; text-decoration: none; font-weight: 600; }
.success-block { text-align: center; padding: 16px 0 8px; }
.success-emoji { font-size: 56px; margin-bottom: 12px; }
.success-title { font-size: 18px; color: #1f2937; font-weight: 700; }
</style>
