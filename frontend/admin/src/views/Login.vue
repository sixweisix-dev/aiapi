<template>
  <div class="auth-page">
    <div class="auth-card">
      <div class="auth-logo">⚡</div>
      <h1 class="auth-brand">TransitAI</h1>
      <p class="auth-tagline">管理后台 · Admin Console</p>

      <el-form ref="formRef" :model="form" :rules="rules" class="auth-form">
        <el-form-item prop="email">
          <el-input v-model="form.email" placeholder="📧 管理员邮箱" size="large" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="form.password" type="password" placeholder="🔒 密码" size="large" show-password
            @keyup.enter="handleLogin" />
        </el-form-item>
        <button class="auth-btn" :disabled="loading" @click="handleLogin">
          {{ loading ? '登录中...' : '登 录' }}
        </button>
      </el-form>

      <div class="auth-footer">
        <span class="footer-tag">⚙️ 仅限管理员访问</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()
const formRef = ref(null)
const loading = ref(false)
const form = reactive({ email: '', password: '' })
const rules = {
  email: [{ required: true, message: '请输入邮箱', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

async function handleLogin() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
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
  content: ''; position: absolute; top: -100px; right: -100px;
  width: 300px; height: 300px;
  background: rgba(255, 255, 255, 0.08); border-radius: 50%;
}
.auth-page::after {
  content: ''; position: absolute; bottom: -80px; left: -80px;
  width: 240px; height: 240px;
  background: rgba(255, 255, 255, 0.06); border-radius: 50%;
}
.auth-card {
  width: 100%; max-width: 380px;
  background: #fff; border-radius: 24px; padding: 36px 28px;
  box-shadow: 0 20px 60px rgba(0,0,0,0.2);
  position: relative; z-index: 1;
}
.auth-logo { font-size: 48px; text-align: center; margin-bottom: 8px; }
.auth-brand {
  font-size: 28px; font-weight: 800; text-align: center;
  background: linear-gradient(135deg, #667eea, #764ba2);
  -webkit-background-clip: text; -webkit-text-fill-color: transparent;
  background-clip: text; margin: 0;
}
.auth-tagline {
  text-align: center; color: #9ca3af;
  font-size: 13px; margin: 6px 0 24px;
}
.auth-form { margin-bottom: 16px; }
.auth-btn {
  width: 100%; height: 48px; border: none; border-radius: 14px;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff; font-size: 16px; font-weight: 700; cursor: pointer;
  margin-top: 8px;
  box-shadow: 0 6px 16px rgba(102,126,234,0.35);
}
.auth-btn:active { transform: scale(0.98); }
.auth-btn:disabled { opacity: 0.6; }
.auth-footer { text-align: center; margin-top: 12px; }
.footer-tag {
  font-size: 12px; color: #9ca3af;
  background: #f3f4f6;
  padding: 4px 12px; border-radius: 10px;
}
</style>
