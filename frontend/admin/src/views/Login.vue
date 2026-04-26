<template>
  <div class="min-h-screen flex items-center justify-center bg-gray-100">
    <div class="w-96 bg-white rounded-lg shadow p-8">
      <h2 class="text-2xl font-bold text-center mb-2">管理后台登录</h2>
      <p class="text-sm text-gray-500 text-center mb-6">AI API Gateway</p>
      <el-form ref="formRef" :model="form" :rules="rules" @submit.prevent="handleLogin">
        <el-form-item prop="email">
          <el-input v-model="form.email" placeholder="邮箱" size="large" prefix-icon="User" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="form.password" type="password" placeholder="密码" size="large" prefix-icon="Lock" show-password />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" size="large" class="w-full" :loading="loading" @click="handleLogin">
            {{ loading ? '登录中...' : '登 录' }}
          </el-button>
        </el-form-item>
      </el-form>
      <p class="text-xs text-gray-400 text-center mt-4">
        默认管理员: admin@example.com / admin123
      </p>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()
const formRef = ref(null)
const loading = ref(false)

const form = reactive({
  email: '',
  password: '',
})

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
    if (auth.user?.role !== 'admin') {
      auth.logout()
      ElMessage.error('该账号不是管理员，无法登录管理后台')
      return
    }
    ElMessage.success('登录成功')
    router.push('/dashboard')
  } catch (err) {
    // Error already handled by interceptor
  } finally {
    loading.value = false
  }
}
</script>
