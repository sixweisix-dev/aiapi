<template>
  <div>
    <el-card shadow="hover" class="mb-4">
      <div class="flex justify-between items-center">
        <span class="text-base font-medium">上游渠道</span>
        <el-button type="primary" @click="openCreate">添加渠道</el-button>
      </div>
    </el-card>

    <el-card shadow="hover">
      <el-table :data="channels" v-loading="loading" stripe style="width: 100%">
        <el-table-column prop="name" label="名称" width="150" />
        <el-table-column prop="provider" label="提供商" width="100">
          <template #default="{ row }">
            <el-tag size="small">{{ row.provider }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="weight" label="权重" width="70" align="center" />
        <el-table-column prop="health_status" label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="healthTagType(row.health_status)" size="small">
              {{ healthLabel(row.health_status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="is_enabled" label="启用" width="70" align="center">
          <template #default="{ row }">
            <el-switch :model-value="row.is_enabled" disabled size="small" />
          </template>
        </el-table-column>
        <el-table-column prop="total_requests" label="总请求" width="80" align="right" />
        <el-table-column prop="total_tokens" label="总Tokens" width="100" align="right" />
        <el-table-column prop="error_count" label="错误" width="70" align="right" />
        <el-table-column prop="last_health_check" label="最后检测" width="160">
          <template #default="{ row }">{{ row.last_health_check ? dayjs(row.last_health_check).format('MM-DD HH:mm') : '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openEdit(row)">编辑</el-button>
            <el-button size="small" type="success" :loading="testingId === row.id" @click="handleTest(row)">测试</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create/Edit Dialog -->
    <el-dialog v-model="dialogVisible" :title="isEditing ? '编辑渠道' : '添加渠道'" width="500px">
      <el-form :model="form" :rules="rules" ref="formRef" label-width="100px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="例如: OpenAI 主账号" />
        </el-form-item>
        <el-form-item label="提供商" prop="provider">
          <el-select v-model="form.provider" style="width:100%">
            <el-option label="OpenAI" value="openai" />
            <el-option label="Anthropic" value="anthropic" />
            <el-option label="Google" value="google" />
            <el-option label="Qwen" value="qwen" />
            <el-option label="DeepSeek" value="deepseek" />
          </el-select>
        </el-form-item>
        <el-form-item label="API Key" prop="api_key">
          <el-input v-model="form.api_key" type="password" show-password :placeholder="isEditing ? '留空则不修改' : ''" />
        </el-form-item>
        <el-form-item label="自定义URL">
          <el-input v-model="form.base_url" placeholder="留空使用默认 (如 https://api.openai.com)" />
        </el-form-item>
        <el-form-item label="权重">
          <el-input-number v-model="form.weight" :min="1" :max="100" />
        </el-form-item>
        <el-form-item label="启用" v-if="isEditing">
          <el-switch v-model="form.is_enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { channelsAPI } from '@/utils/api'
import dayjs from 'dayjs'

const channels = ref([])
const loading = ref(false)
const saving = ref(false)
const testingId = ref(null)
const dialogVisible = ref(false)
const isEditing = ref(false)
const formRef = ref(null)
const editingId = ref(null)

const form = ref({
  name: '',
  provider: 'openai',
  api_key: '',
  base_url: '',
  weight: 1,
  is_enabled: true,
})

const rules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  provider: [{ required: true, message: '请选择提供商', trigger: 'change' }],
  api_key: [{ required: true, message: '请输入API Key', trigger: 'blur' }],
}

function healthTagType(status) {
  return { healthy: 'success', unhealthy: 'danger', unknown: 'info' }[status] || 'info'
}
function healthLabel(status) {
  return { healthy: '正常', unhealthy: '异常', unknown: '未知' }[status] || status
}

async function fetchData() {
  loading.value = true
  try {
    const res = await channelsAPI.list()
    channels.value = res.items
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isEditing.value = false
  editingId.value = null
  form.value = { name: '', provider: 'openai', api_key: '', base_url: '', weight: 1, is_enabled: true }
  dialogVisible.value = true
}

function openEdit(row) {
  isEditing.value = true
  editingId.value = row.id
  form.value = {
    name: row.name,
    provider: row.provider,
    api_key: '',
    base_url: row.base_url || '',
    weight: row.weight,
    is_enabled: row.is_enabled,
  }
  dialogVisible.value = true
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    if (isEditing.value) {
      const data = { name: form.value.name, base_url: form.value.base_url || null, weight: form.value.weight, is_enabled: form.value.is_enabled }
      if (form.value.api_key) data.api_key = form.value.api_key
      await channelsAPI.update(editingId.value, data)
      ElMessage.success('更新成功')
    } else {
      await channelsAPI.create({ ...form.value, base_url: form.value.base_url || null, weight: form.value.weight })
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    await fetchData()
  } finally {
    saving.value = false
  }
}

async function handleTest(row) {
  testingId.value = row.id
  try {
    const res = await channelsAPI.test(row.id)
    ElMessage.success(res.healthy ? '连接正常' : '连接失败')
    await fetchData()
  } finally {
    testingId.value = null
  }
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(`确定要删除渠道 "${row.name}" 吗？`, '提示')
    await channelsAPI.delete(row.id)
    ElMessage.success('已删除')
    await fetchData()
  } catch { /* cancelled */ }
}

onMounted(fetchData)
</script>
