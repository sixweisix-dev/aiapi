<template>
  <div class="cb-page">
    <div class="cb-card">
      <div class="cb-spinner">⚡</div>
      <div class="cb-text">{{ msg }}</div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const msg = ref('正在登录...')

onMounted(async () => {
  const token = route.query.token
  if (!token) {
    msg.value = '登录失败：缺少 token'
    setTimeout(() => router.replace('/login'), 1500)
    return
  }
  localStorage.setItem('user_token', String(token))
  auth.token = String(token)
  try {
    await auth.fetchMe()
    ElMessage.success('登录成功')
    router.replace('/dashboard')
  } catch {
    msg.value = '登录失败'
    setTimeout(() => router.replace('/login'), 1500)
  }
})
</script>

<style scoped>
.cb-page {
  min-height: 100vh; display: flex; align-items: center; justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}
.cb-card {
  background: #fff; border-radius: 20px; padding: 40px 32px; text-align: center;
  box-shadow: 0 20px 60px rgba(0,0,0,0.2); min-width: 260px;
}
.cb-spinner { font-size: 44px; animation: pulse 1s ease-in-out infinite; }
@keyframes pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.3; } }
.cb-text { color: #6b7280; font-size: 14px; margin-top: 16px; }
</style>
