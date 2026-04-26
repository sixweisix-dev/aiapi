<template>
  <div class="min-h-screen flex items-center justify-center bg-gray-100">
    <div class="w-full max-w-sm bg-white rounded-2xl shadow-lg p-8">
      <div class="text-center mb-6">
        <div class="w-12 h-12 bg-blue-600 rounded-xl flex items-center justify-center mx-auto mb-3">
          <span class="text-white text-xl font-bold">AI</span>
        </div>
        <h1 class="text-2xl font-bold text-gray-800">重置密码</h1>
      </div>

      <div v-if="!done">
        <el-form ref="formRef" :model="form" :rules="rules">
          <el-form-item prop="new_password">
            <el-input v-model="form.new_password" type="password" placeholder="新密码（8位以上含大小写和数字）" size="large" show-password />
          </el-form-item>
          <el-form-item prop="confirm_password">
            <el-input v-model="form.confirm_password" type="password" placeholder="确认新密码" size="large" show-password />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" size="large" class="w-full" :loading="loading" @click="handleSubmit">
              重置密码
            </el-button>
          </el-form-item>
        </el-form>
        <p v-if="!token" class="text-red-500 text-sm text-center">链接无效或已过期</p>
      </div>

      <div v-else class="text-center py-4">
        <el-icon class="text-5xl text-green-500 mb-4"><CircleCheck /></el-icon>
        <p class="text-gray-700 mb-4">密码重置成功！</p>
        <el-button type="primary" @click="$router.push('/login')">去登录</el-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import api from '@/utils/api'

const route = useRoute()
const formRef = ref(null)
const loading = ref(false)
const done = ref(false)
const token = ref('')
const form = reactive({ new_password: '', confirm_password: '' })

onMounted(() => {
  token.value = route.query.token || ''
})

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
    await api.post('/auth/reset-password', {
      token: token.value,
      new_password: form.new_password
    })
    done.value = true
  } catch {
  } finally {
    loading.value = false
  }
}
</script>
