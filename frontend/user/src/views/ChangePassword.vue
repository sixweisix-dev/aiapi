<template>
  <div class="page">
    <div class="hero">
      <div class="hero-bg"></div>
      <div class="hero-emoji">🔒</div>
      <div class="hero-title">修改密码</div>
      <div class="hero-sub">为账号安全，建议定期更换密码</div>
    </div>

    <div class="data-card">
      <div class="card-header"><span class="card-title">🔑 设置新密码</span></div>
      <el-form :model="form" :rules="rules" ref="formRef" class="form-body">
        <el-form-item prop="old_password">
          <el-input v-model="form.old_password" type="password" placeholder="🔒 当前密码" size="large" show-password />
        </el-form-item>
        <el-form-item prop="new_password">
          <el-input v-model="form.new_password" type="password" placeholder="✨ 新密码（8位+大小写+数字）" size="large" show-password />
        </el-form-item>
        <el-form-item prop="confirm_password">
          <el-input v-model="form.confirm_password" type="password" placeholder="✅ 确认新密码" size="large" show-password />
        </el-form-item>
        <button class="primary-btn" :disabled="loading" @click="handleSubmit">
          {{ loading ? '提交中...' : '确认修改' }}
        </button>
        <button class="secondary-btn" @click="$router.push('/')">取消</button>
      </el-form>
    </div>

    <div class="data-card tip-card">
      <div class="tip-emoji">💡</div>
      <div class="tip-text">
        <div class="tip-title">密码要求</div>
        <div class="tip-content">至少 8 位 · 包含大小写字母 · 包含数字</div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'
import api from '@/utils/api'

const router = useRouter()
const formRef = ref()
const loading = ref(false)
const form = ref({ old_password: '', new_password: '', confirm_password: '' })

const passwordStrength = (value) => {
  if (value.length < 8) return '至少8位'
  if (!/[a-z]/.test(value)) return '需包含小写字母'
  if (!/[A-Z]/.test(value)) return '需包含大写字母'
  if (!/[0-9]/.test(value)) return '需包含数字'
  return null
}

const rules = {
  old_password: [{ required: true, message: '请输入当前密码', trigger: 'blur' }],
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { validator: (rule, value, callback) => { const err = passwordStrength(value); err ? callback(new Error(err)) : callback() }, trigger: 'blur' }
  ],
  confirm_password: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    { validator: (rule, value, callback) => { value !== form.value.new_password ? callback(new Error('两次密码不一致')) : callback() }, trigger: 'blur' }
  ]
}

async function handleSubmit() {
  await formRef.value.validate()
  loading.value = true
  try {
    await api.post('/auth/change-password', {
      old_password: form.value.old_password,
      new_password: form.value.new_password
    })
    ElMessage.success('密码修改成功，请重新登录')
    localStorage.removeItem('user_token')
    localStorage.removeItem('user_user')
    router.push('/login')
  } catch {} finally { loading.value = false }
}
</script>

<style scoped>
.page { padding-bottom: 20px; }
.hero {
  position: relative;
  background: linear-gradient(135deg, #f093fb, #f5576c);
  border-radius: 20px;
  padding: 24px 20px;
  color: #fff;
  margin-bottom: 14px;
  text-align: center;
  box-shadow: 0 10px 30px rgba(245,87,108,0.3);
  overflow: hidden;
}
.hero-bg {
  position: absolute; top: -40px; right: -40px;
  width: 140px; height: 140px;
  background: rgba(255,255,255,0.12); border-radius: 50%;
}
.hero-emoji { font-size: 36px; position: relative; z-index: 1; }
.hero-title { font-size: 22px; font-weight: 800; margin-top: 6px; position: relative; z-index: 1; }
.hero-sub { font-size: 13px; opacity: 0.95; margin-top: 4px; position: relative; z-index: 1; }

.data-card {
  background: #fff;
  border-radius: 14px;
  padding: 16px;
  margin-bottom: 14px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.04);
}
.card-header { display: flex; justify-content: space-between; margin-bottom: 14px; }
.card-title { font-size: 15px; font-weight: 600; color: #1f2937; }
.form-body { display: flex; flex-direction: column; gap: 8px; }

.primary-btn {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff; border: none;
  height: 46px; border-radius: 12px;
  font-size: 15px; font-weight: 600;
  cursor: pointer; width: 100%;
  box-shadow: 0 4px 12px rgba(102,126,234,0.3);
  margin-top: 4px;
}
.primary-btn:active { transform: scale(0.98); }
.primary-btn:disabled { opacity: 0.6; }
.secondary-btn {
  background: #f3f4f6; color: #4b5563;
  border: none;
  height: 42px; border-radius: 10px;
  font-size: 14px; font-weight: 500;
  cursor: pointer; width: 100%;
  margin-top: 4px;
}
.secondary-btn:active { background: #e5e7eb; }

.tip-card {
  display: flex; gap: 12px; align-items: flex-start;
  background: linear-gradient(135deg, #fef3c7, #fde68a);
}
.tip-emoji { font-size: 22px; }
.tip-text { flex: 1; }
.tip-title { font-size: 13px; font-weight: 700; color: #78350f; margin-bottom: 4px; }
.tip-content { font-size: 12px; color: #92400e; line-height: 1.5; }
</style>
