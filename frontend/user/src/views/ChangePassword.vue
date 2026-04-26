<template>
  <div class="max-w-md mx-auto">
    <el-card shadow="hover">
      <template #header>
        <span class="font-medium">修改密码</span>
      </template>
      <el-form :model="form" :rules="rules" ref="formRef" label-width="100px">
        <el-form-item label="当前密码" prop="old_password">
          <el-input v-model="form.old_password" type="password" show-password placeholder="请输入当前密码" />
        </el-form-item>
        <el-form-item label="新密码" prop="new_password">
          <el-input v-model="form.new_password" type="password" show-password placeholder="8位以上，含大小写字母和数字" />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirm_password">
          <el-input v-model="form.confirm_password" type="password" show-password placeholder="再次输入新密码" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="loading" @click="handleSubmit">确认修改</el-button>
          <el-button @click="$router.push('/')">取消</el-button>
        </el-form-item>
      </el-form>
    </el-card>
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
    {
      validator: (rule, value, callback) => {
        const err = passwordStrength(value)
        err ? callback(new Error(err)) : callback()
      },
      trigger: 'blur'
    }
  ],
  confirm_password: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        value !== form.value.new_password ? callback(new Error('两次密码不一致')) : callback()
      },
      trigger: 'blur'
    }
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
  } catch {
  } finally {
    loading.value = false
  }
}
</script>
