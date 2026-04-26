<template>
  <div>
    <el-card shadow="hover" class="mb-4">
      <div class="flex justify-between items-center">
        <span class="text-base font-medium">模型管理</span>
        <el-button type="primary" @click="openCreate">添加模型</el-button>
      </div>
    </el-card>

    <el-card shadow="hover">
      <el-table :data="models" v-loading="loading" stripe style="width: 100%">
        <el-table-column prop="display_name" label="模型名称" width="160" />
        <el-table-column prop="name" label="标识" width="160" />
        <el-table-column prop="provider" label="提供商" width="80">
          <template #default="{ row }">
            <el-tag size="small">{{ row.provider }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="input_price" label="输入单价" width="100" align="right">
          <template #default="{ row }">${{ row.input_price.toFixed(4) }}</template>
        </el-table-column>
        <el-table-column prop="output_price" label="输出单价" width="100" align="right">
          <template #default="{ row }">${{ row.output_price.toFixed(4) }}</template>
        </el-table-column>
        <el-table-column prop="multiplier" label="倍率" width="70" align="center" />
        <el-table-column prop="context_length" label="上下文" width="80" align="right">
          <template #default="{ row }">{{ row.context_length.toLocaleString() }}</template>
        </el-table-column>
        <el-table-column prop="is_public" label="公开" width="60" align="center">
          <template #default="{ row }">
            <el-tag :type="row.is_public ? 'success' : 'info'" size="small">{{ row.is_public ? '是' : '否' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="is_enabled" label="启用" width="60" align="center">
          <template #default="{ row }">
            <el-switch :model-value="row.is_enabled" disabled size="small" />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openEdit(row)">编辑</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create/Edit Dialog -->
    <el-dialog v-model="dialogVisible" :title="isEditing ? '编辑模型' : '添加模型'" width="550px">
      <el-form :model="form" :rules="rules" ref="formRef" label-width="120px">
        <el-form-item label="模型标识" prop="name">
          <el-input v-model="form.name" :disabled="isEditing" placeholder="例如: gpt-4" />
        </el-form-item>
        <el-form-item label="显示名称" prop="display_name">
          <el-input v-model="form.display_name" placeholder="例如: GPT-4" />
        </el-form-item>
        <el-form-item label="提供商" prop="provider">
          <el-select v-model="form.provider" style="width:100%" :disabled="isEditing">
            <el-option label="OpenAI" value="openai" />
            <el-option label="Anthropic" value="anthropic" />
            <el-option label="Google" value="google" />
            <el-option label="Qwen" value="qwen" />
            <el-option label="DeepSeek" value="deepseek" />
          </el-select>
        </el-form-item>
        <el-form-item label="上下文长度">
          <el-input-number v-model="form.context_length" :min="1024" :step="4096" />
        </el-form-item>
        <el-row :gutter="10">
          <el-col :span="12">
            <el-form-item label="输入单价" prop="input_price">
              <el-input-number v-model="form.input_price" :precision="4" :step="0.001" :min="0" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="输出单价" prop="output_price">
              <el-input-number v-model="form.output_price" :precision="4" :step="0.001" :min="0" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="倍率">
          <el-input-number v-model="form.multiplier" :precision="2" :step="0.1" :min="0.5" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" />
        </el-form-item>
        <el-row :gutter="10">
          <el-col :span="12">
            <el-form-item label="公开">
              <el-switch v-model="form.is_public" />
            </el-form-item>
          </el-col>
          <el-col :span="12" v-if="isEditing">
            <el-form-item label="启用">
              <el-switch v-model="form.is_enabled" />
            </el-form-item>
          </el-col>
        </el-row>
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
import { modelsAPI } from '@/utils/api'

const models = ref([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEditing = ref(false)
const formRef = ref(null)
const editingId = ref(null)

const form = ref({
  name: '',
  display_name: '',
  provider: 'openai',
  context_length: 4096,
  input_price: 0,
  output_price: 0,
  multiplier: 1.0,
  description: '',
  is_public: true,
  is_enabled: true,
})

const rules = {
  name: [{ required: true, message: '请输入模型标识', trigger: 'blur' }],
  display_name: [{ required: true, message: '请输入显示名称', trigger: 'blur' }],
  provider: [{ required: true, message: '请选择提供商', trigger: 'change' }],
  input_price: [{ required: true, message: '请输入输入单价', trigger: 'blur' }],
  output_price: [{ required: true, message: '请输入输出单价', trigger: 'blur' }],
}

async function fetchData() {
  loading.value = true
  try {
    const res = await modelsAPI.list()
    models.value = res.items
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isEditing.value = false
  editingId.value = null
  form.value = { name: '', display_name: '', provider: 'openai', context_length: 4096, input_price: 0, output_price: 0, multiplier: 1.0, description: '', is_public: true, is_enabled: true }
  dialogVisible.value = true
}

function openEdit(row) {
  isEditing.value = true
  editingId.value = row.id
  form.value = { ...row }
  dialogVisible.value = true
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    const data = {
      display_name: form.value.display_name,
      input_price: form.value.input_price,
      output_price: form.value.output_price,
      multiplier: form.value.multiplier,
      is_public: form.value.is_public,
      description: form.value.description || null,
    }
    if (isEditing.value) {
      data.is_enabled = form.value.is_enabled
      await modelsAPI.update(editingId.value, data)
      ElMessage.success('更新成功')
    } else {
      await modelsAPI.create({
        name: form.value.name,
        display_name: form.value.display_name,
        provider: form.value.provider,
        context_length: form.value.context_length,
        input_price: form.value.input_price,
        output_price: form.value.output_price,
        multiplier: form.value.multiplier,
        is_public: form.value.is_public,
        description: form.value.description || null,
      })
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    await fetchData()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(`确定要删除模型 "${row.display_name}" 吗？`, '提示')
    await modelsAPI.delete(row.id)
    ElMessage.success('已删除')
    await fetchData()
  } catch { /* cancelled */ }
}

onMounted(fetchData)
</script>
