<template>
  <div style="min-height:100vh;display:flex;align-items:center;justify-content:center;background:#e5e7eb;padding:16px;">
    <div style="width:100%;max-width:380px;background:white;border-radius:16px;box-shadow:0 4px 20px rgba(0,0,0,0.1);padding:32px;">
      <h2 class="text-2xl font-semibold text-center text-gray-800 mb-6">TransitAI</h2>
      <el-form ref="formRef" :model="form" :rules="rules">
        <el-form-item prop="email">
          <el-input v-model="form.email" placeholder="邮箱" size="large"
            class="rounded-lg bg-gray-100 border-0" style="--el-input-bg-color:#f3f4f6;--el-input-border-color:transparent" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="form.password" type="password" placeholder="密码" size="large" show-password
            class="rounded-lg" style="--el-input-bg-color:#f3f4f6;--el-input-border-color:transparent"
            @keyup.enter="handleLogin" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" size="large" round style="width:100%;font-size:16px;" :loading="loading" @click="handleLogin">
            登录
          </el-button>
        </el-form-item>
      </el-form>
      <div style="display:flex;justify-content:space-between;font-size:14px;color:#9ca3af;margin-top:8px;">
        <router-link to="/register" class="hover:text-gray-600">注册</router-link>
        <router-link to="/forgot-password" class="hover:text-gray-600">忘记密码</router-link>
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
  } catch {} finally {
    loading.value = false
  }
}
</script>
