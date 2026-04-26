<template>
  <div style="min-height:100vh;display:flex;align-items:center;justify-content:center;background:#e5e7eb;padding:16px;">
    <div style="width:100%;max-width:380px;background:white;border-radius:16px;box-shadow:0 4px 20px rgba(0,0,0,0.1);padding:32px;">
      <h2 class="text-2xl font-semibold text-center text-gray-800 mb-6">创建账号</h2>
      <el-form ref="formRef" :model="form" :rules="rules">
        <el-form-item prop="email">
          <el-input v-model="form.email" placeholder="邮箱" size="large"
            style="--el-input-bg-color:#f3f4f6;--el-input-border-color:transparent" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="form.password" type="password" placeholder="密码（8位以上含大小写和数字）" size="large" show-password
            style="--el-input-bg-color:#f3f4f6;--el-input-border-color:transparent" />
        </el-form-item>
        <el-form-item prop="confirmPassword">
          <el-input v-model="form.confirmPassword" type="password" placeholder="确认密码" size="large" show-password
            style="--el-input-bg-color:#f3f4f6;--el-input-border-color:transparent" />
        </el-form-item>
        <el-form-item prop="captcha">
          <div class="flex gap-2 w-full">
            <el-input v-model="form.captcha" placeholder="图片验证码" size="large" class="flex-1"
              style="--el-input-bg-color:#f3f4f6;--el-input-border-color:transparent" />
            <img :src="captchaUrl" @click="refreshCaptcha" class="h-10 rounded-lg cursor-pointer" style="min-width:110px;object-fit:cover" />
          </div>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" size="large" round style="width:100%" :loading="loading" @click="handleRegister">
            注册
          </el-button>
        </el-form-item>
      </el-form>
      <div class="text-center text-sm text-gray-400 mt-2">
        <router-link to="/login" class="hover:text-gray-600">返回登录</router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import api from '@/utils/api'

const router = useRouter()
const auth = useAuthStore()
const formRef = ref(null)
const loading = ref(false)
const captchaId = ref('')
const captchaUrl = ref('')
const form = reactive({ email: '', password: '', confirmPassword: '', captcha: '' })

const passwordStrength = (v) => {
  if (v.length < 8) return '至少8位'
  if (!/[a-z]/.test(v)) return '需含小写字母'
  if (!/[A-Z]/.test(v)) return '需含大写字母'
  if (!/[0-9]/.test(v)) return '需含数字'
  return null
}

const rules = {
  email: [{ required: true, message: '请输入邮箱', trigger: 'blur' }],
  password: [
    { required: true, trigger: 'blur' },
    { validator: (r, v, cb) => { const e = passwordStrength(v); e ? cb(new Error(e)) : cb() }, trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    { validator: (r, v, cb) => v !== form.password ? cb(new Error('两次密码不一致')) : cb(), trigger: 'blur' }
  ],
  captcha: [{ required: true, message: '请输入验证码', trigger: 'blur' }],
}

async function refreshCaptcha() {
  const res = await api.get('/auth/captcha/new')
  captchaId.value = res.captcha_id
  captchaUrl.value = res.captcha_url
}

onMounted(refreshCaptcha)

async function handleRegister() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  loading.value = true
  try {
    await auth.register(form.email, form.password, undefined, captchaId.value, form.captcha)
    ElMessage.success('注册成功')
    router.push('/dashboard')
  } catch { refreshCaptcha() } finally { loading.value = false }
}
</script>
