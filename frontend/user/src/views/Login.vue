<template>
  <div class="auth-page">
    <div class="auth-card">
      <div class="auth-logo">⚡</div>
      <h1 class="auth-brand">TransitAI</h1>
      <p class="auth-tagline">欢迎回来，请登录账号</p>

      <el-form ref="formRef" :model="form" :rules="rules" class="auth-form">
        <el-form-item prop="email">
          <el-input v-model="form.email" placeholder="📧 邮箱" size="large" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="form.password" type="password" placeholder="🔒 密码" size="large" show-password
            @keyup.enter="handleLogin" />
        </el-form-item>
        <div class="agree-row">
          <el-checkbox v-model="agreed" />
          <span class="agree-text">
            登录即代表已阅读并同意
            <router-link to="/terms" class="agree-link">《用户协议》</router-link>
            和
            <router-link to="/privacy" class="agree-link">《隐私政策》</router-link>
          </span>
        </div>
        <button type="button" class="auth-btn" :disabled="loading" @click="handleLogin">
          {{ loading ? '登录中...' : '登 录' }}
        </button>
      </el-form>

      <div class="auth-links">
        <router-link to="/register" class="auth-link">注册账号</router-link>
        <span class="link-divider">·</span>
        <router-link to="/forgot-password" class="auth-link">忘记密码</router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()
const formRef = ref(null)
const loading = ref(false)
const agreed = ref(false)
const form = reactive({ email: '', password: '' })
const rules = {
  email: [{ required: true, message: '请输入邮箱', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

async function handleLogin() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  if (!agreed.value) {
    return ElMessage.warning('请先阅读并同意用户协议和隐私政策')
  }
  loading.value = true
  try {
    await auth.login(form.email, form.password)
    router.push('/dashboard')
  } catch {} finally { loading.value = false }
}
</script>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  position: relative;
  overflow: hidden;
}
.auth-page::before {
  content: '';
  position: absolute;
  top: -100px;
  right: -100px;
  width: 300px;
  height: 300px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 50%;
}
.auth-page::after {
  content: '';
  position: absolute;
  bottom: -80px;
  left: -80px;
  width: 240px;
  height: 240px;
  background: rgba(255, 255, 255, 0.06);
  border-radius: 50%;
}
.auth-card {
  width: 100%;
  max-width: 380px;
  background: #fff;
  border-radius: 24px;
  padding: 36px 28px;
  box-shadow: 0 20px 60px rgba(0,0,0,0.2);
  position: relative;
  z-index: 1;
}
.auth-logo {
  font-size: 48px;
  text-align: center;
  margin-bottom: 8px;
}
.auth-brand {
  font-size: 28px;
  font-weight: 800;
  text-align: center;
  background: linear-gradient(135deg, #667eea, #764ba2);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin: 0;
}
.auth-tagline {
  text-align: center;
  color: #9ca3af;
  font-size: 13px;
  margin: 6px 0 24px;
}
.auth-form { margin-bottom: 16px; }
.auth-btn {
  width: 100%;
  height: 48px;
  border: none;
  border-radius: 14px;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  font-size: 16px;
  font-weight: 700;
  cursor: pointer;
  margin-top: 8px;
  box-shadow: 0 6px 16px rgba(102,126,234,0.35);
  transition: transform 0.15s;
}
.auth-btn:active { transform: scale(0.98); }
.auth-btn:disabled { opacity: 0.6; }
.auth-links {
  text-align: center;
  font-size: 13px;
  color: #9ca3af;
}
.auth-link {
  color: #667eea;
  text-decoration: none;
  font-weight: 500;
}
.auth-link:active { color: #4f46e5; }
.link-divider { margin: 0 10px; }
.agree-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin: 6px 0 14px;
  padding: 0 4px;
}
.agree-text {
  font-size: 12px;
  color: #6b7280;
  line-height: 1.6;
  flex: 1;
}
.agree-link {
  color: #667eea;
  text-decoration: none;
  font-weight: 500;
}
</style>
