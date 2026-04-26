<template>
  <div style="min-height:100vh;display:flex;align-items:center;justify-content:center;background:#e5e7eb;padding:16px;">
    <div style="width:100%;max-width:380px;background:white;border-radius:16px;box-shadow:0 4px 20px rgba(0,0,0,0.1);padding:32px;">
      <h2 class="text-2xl font-semibold text-center text-gray-800 mb-2">找回密码</h2>
      <p class="text-center text-sm text-gray-400 mb-6">输入注册邮箱，我们将发送重置链接</p>

      <div v-if="!sent">
        <el-form ref="formRef" :model="form" :rules="rules">
          <el-form-item prop="email">
            <el-input v-model="form.email" placeholder="注册邮箱" size="large"
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
            <el-button type="primary" size="large" round style="width:100%" :loading="loading" @click="handleSubmit">
              发送重置链接
            </el-button>
          </el-form-item>
        </el-form>
      </div>

      <div v-else class="text-center py-6">
        <div class="text-5xl mb-4">✉️</div>
        <p class="text-gray-700 mb-1">重置链接已发送</p>
        <p class="text-sm text-gray-400">请检查收件箱（包括垃圾邮件）</p>
      </div>

      <div class="text-center text-sm text-gray-400 mt-4">
        <router-link to="/login" class="hover:text-gray-600">返回登录</router-link>
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
