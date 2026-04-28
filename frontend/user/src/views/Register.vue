<template>
  <div class="auth-page">
    <div class="auth-card">
      <div class="auth-logo">⚡</div>
      <h1 class="auth-brand">创建账号</h1>
      <p class="auth-tagline">几秒钟完成注册，立即开始</p>

      <el-form ref="formRef" :model="form" :rules="rules" class="auth-form">
        <el-form-item prop="email">
          <el-input v-model="form.email" placeholder="📧 邮箱" size="large" :disabled="codeSent" />
        </el-form-item>
        <el-form-item prop="emailCode">
          <div style="display:flex;gap:8px;width:100%">
            <el-input v-model="form.emailCode" placeholder="✉️ 邮箱验证码（6位）" size="large" maxlength="6" style="flex:1" />
            <button type="button" class="code-btn" :disabled="codeBtnDisabled" @click="onClickSendCode">
              {{ codeBtnText }}
            </button>
          </div>
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="form.password" type="password" placeholder="🔒 密码（8位+大小写+数字）" size="large" show-password />
        </el-form-item>
        <el-form-item prop="confirmPassword">
          <el-input v-model="form.confirmPassword" type="password" placeholder="🔒 确认密码" size="large" show-password />
        </el-form-item>

        <!-- Turnstile 在弹窗里按需渲染 -->
        <el-dialog v-model="tsDialogVisible" title="人机验证" width="340px" :close-on-click-modal="false" align-center>
          <div style="display:flex;justify-content:center;padding:8px 0 4px">
            <TurnstileWidget v-if="tsDialogVisible" ref="tsRef" v-model="turnstileToken" />
          </div>
          <p style="text-align:center;color:#9ca3af;font-size:12px;margin:0">完成验证后将自动发送邮件</p>
        </el-dialog>

        <div class="agree-row">
          <el-checkbox v-model="agreed" />
          <span class="agree-text">
            我已阅读并同意
            <router-link to="/terms" class="agree-link">《用户协议》</router-link>
            和
            <router-link to="/privacy" class="agree-link">《隐私政策》</router-link>
          </span>
        </div>
        <button type="button" class="auth-btn" :disabled="loading" @click="handleRegister">
          {{ loading ? '注册中...' : '注 册' }}
        </button>
      </el-form>

      <div class="auth-links">
        已有账号？<router-link to="/login" class="auth-link">立即登录</router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import api from '@/utils/api'
import TurnstileWidget from '@/components/TurnstileWidget.vue'

const router = useRouter()
const auth = useAuthStore()
const formRef = ref(null)
const tsRef = ref(null)
const loading = ref(false)
const agreed = ref(false)
const codeSent = ref(false)
const cooldown = ref(0)
const tsDialogVisible = ref(false)
const turnstileToken = ref('')
const form = reactive({ email: '', emailCode: '', password: '', confirmPassword: '' })

const passwordStrength = (v) => {
  if (v.length < 8) return '至少8位'
  if (!/[a-z]/.test(v)) return '需含小写字母'
  if (!/[A-Z]/.test(v)) return '需含大写字母'
  if (!/[0-9]/.test(v)) return '需含数字'
  return null
}

const rules = {
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '邮箱格式不正确', trigger: 'blur' },
  ],
  emailCode: [
    { required: true, message: '请输入邮箱验证码', trigger: 'blur' },
    { len: 6, message: '验证码为 6 位', trigger: 'blur' },
  ],
  password: [
    { required: true, trigger: 'blur' },
    { validator: (r, v, cb) => { const e = passwordStrength(v); e ? cb(new Error(e)) : cb() }, trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    { validator: (r, v, cb) => v !== form.password ? cb(new Error('两次密码不一致')) : cb(), trigger: 'blur' }
  ],
}

const codeBtnDisabled = computed(() => cooldown.value > 0 || loading.value)
const codeBtnText = computed(() => {
  if (cooldown.value > 0) return cooldown.value + 's'
  return codeSent.value ? '重新发送' : '发送验证码'
})

function onClickSendCode() {
  // 只校验邮箱字段
  formRef.value?.validateField('email').then(() => {
    turnstileToken.value = ''
    tsDialogVisible.value = true
  }).catch(() => {})
}

// 监听 Turnstile token，拿到就立刻调发送接口
watch(turnstileToken, async (tok) => {
  if (!tok || !tsDialogVisible.value) return
  try {
    await api.post('/auth/send-code', {
      email: form.email,
      turnstile_token: tok,
      purpose: 'register',
    })
    ElMessage.success('验证码已发送，请查收邮件')
    codeSent.value = true
    cooldown.value = 60
    const timer = setInterval(() => {
      cooldown.value--
      if (cooldown.value <= 0) clearInterval(timer)
    }, 1000)
    tsDialogVisible.value = false
  } catch {
    tsRef.value?.reset()
  }
})

async function handleRegister() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  if (!agreed.value) {
    return ElMessage.warning('请先阅读并同意用户协议和隐私政策')
  }
  loading.value = true
  try {
    await auth.register(form.email, form.password, undefined, form.emailCode)
    ElMessage.success('注册成功')
    router.push('/dashboard')
  } catch {
    // 错误消息已在拦截器中弹出
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.auth-page { min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 20px; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); position: relative; overflow: hidden; }
.auth-page::before { content: ''; position: absolute; top: -100px; right: -100px; width: 300px; height: 300px; background: rgba(255, 255, 255, 0.08); border-radius: 50%; }
.auth-page::after { content: ''; position: absolute; bottom: -80px; left: -80px; width: 240px; height: 240px; background: rgba(255, 255, 255, 0.06); border-radius: 50%; }
.auth-card { width: 100%; max-width: 400px; background: #fff; border-radius: 24px; padding: 32px 26px; box-shadow: 0 20px 60px rgba(0,0,0,0.2); position: relative; z-index: 1; }
.auth-logo { font-size: 44px; text-align: center; margin-bottom: 6px; }
.auth-brand { font-size: 24px; font-weight: 800; text-align: center; background: linear-gradient(135deg, #667eea, #764ba2); -webkit-background-clip: text; -webkit-text-fill-color: transparent; background-clip: text; margin: 0; }
.auth-tagline { text-align: center; color: #9ca3af; font-size: 13px; margin: 6px 0 20px; }
.auth-form { margin-bottom: 14px; }
.auth-btn { width: 100%; height: 48px; border: none; border-radius: 14px; background: linear-gradient(135deg, #667eea, #764ba2); color: #fff; font-size: 16px; font-weight: 700; cursor: pointer; margin-top: 4px; box-shadow: 0 6px 16px rgba(102,126,234,0.35); }
.auth-btn:active { transform: scale(0.98); }
.auth-btn:disabled { opacity: 0.6; }
.code-btn { height: 40px; min-width: 110px; padding: 0 12px; border: 1px solid #e5e7eb; border-radius: 8px; background: #f9fafb; color: #4b5563; font-size: 13px; font-weight: 600; cursor: pointer; }
.code-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.code-btn:not(:disabled):hover { border-color: #667eea; color: #667eea; }
.auth-links { text-align: center; font-size: 13px; color: #9ca3af; }
.auth-link { color: #667eea; text-decoration: none; font-weight: 600; margin-left: 4px; }
.agree-row { display: flex; align-items: flex-start; gap: 8px; margin: 4px 0 14px; padding: 0 4px; }
.agree-text { font-size: 12px; color: #6b7280; line-height: 1.6; flex: 1; }
.agree-link { color: #667eea; text-decoration: none; font-weight: 500; }
</style>
