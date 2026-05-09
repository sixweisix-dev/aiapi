<template>
  <div class="page">
    <div class="header">
      <h2>渠道分组</h2>
      <el-button type="primary" @click="openCreate">新建分组</el-button>
    </div>
    <el-alert type="info" :closable="false" class="mb-4">
      渠道分组 = 路由层 + 倍率层。模型按 group_id 路由到对应渠道, 用户付费 = 模型基础价 × 分组倍率。例如经济版 0.6× / 官方版 2.0×
    </el-alert>
    <el-table :data="rows" v-loading="loading" stripe>
      <el-table-column prop="sort_order" label="排序" width="80" align="center" />
      <el-table-column prop="name" label="名称" min-width="120" />
      <el-table-column prop="slug" label="slug" width="120">
        <template #default="{ row }"><el-tag size="small">{{ row.slug }}</el-tag></template>
      </el-table-column>
      <el-table-column label="倍率" width="100" align="right">
        <template #default="{ row }"><strong>{{ Number(row.multiplier).toFixed(2) }}×</strong></template>
      </el-table-column>
      <el-table-column prop="name_en" label="英文名" width="130" show-overflow-tooltip />
      <el-table-column prop="description" label="说明(中文)" min-width="200" show-overflow-tooltip />
      <el-table-column prop="description_en" label="说明(英文)" min-width="200" show-overflow-tooltip />
      <el-table-column label="渠道数" width="90" align="center">
        <template #default="{ row }">
          <el-tag size="small" :type="row.channels > 0 ? 'success' : 'info'">{{ row.channels }} 个</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="模型数" width="90" align="center">
        <template #default="{ row }">
          <el-tag size="small" :type="row.models > 0 ? 'success' : 'info'">{{ row.models }} 个</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="默认" width="80" align="center">
        <template #default="{ row }">
          <el-tag v-if="row.is_default" type="success" size="small">默认</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-dialog v-model="dialogVisible" :title="isEditing ? '编辑分组' : '新建分组'" width="600px">
      <el-form :model="form" :rules="rules" ref="formRef" label-width="120px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="例如: 经济版 / 官方直连" />
        </el-form-item>
        <el-form-item label="slug" prop="slug">
          <el-input v-model="form.slug" :disabled="isEditing" placeholder="economy / official, 必须英文" />
          <span class="text-xs text-gray-400 ml-2">内部标识, 创建后不可修改</span>
        </el-form-item>
        <el-form-item label="倍率" prop="multiplier">
          <el-input-number v-model="form.multiplier" :min="0.01" :step="0.1" :precision="4" controls-position="right" />
          <span class="ml-2 text-xs text-gray-400">用户付费 = 模型基础价 × 此倍率</span>
        </el-form-item>
        <el-form-item label="说明(中文)">
          <el-input v-model="form.description" type="textarea" :rows="2" placeholder="向用户展示的卖点说明（中文）" />
        </el-form-item>
        <el-form-item label="英文名">
          <el-input v-model="form.name_en" placeholder="English name, e.g. Economy / Official Direct" />
        </el-form-item>
        <el-form-item label="说明(英文)">
          <el-input v-model="form.description_en" type="textarea" :rows="2" placeholder="English description shown to EN users" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort_order" :min="0" :step="1" controls-position="right" />
          <span class="ml-2 text-xs text-gray-400">数字越小越靠前</span>
        </el-form-item>
        <el-form-item label="默认分组">
          <el-switch v-model="form.is_default" />
          <span class="ml-2 text-xs text-gray-400">未指定 group 的模型默认走这里</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSave">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '@/utils/api'

const rows = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const isEditing = ref(false)
const editingId = ref(null)
const formRef = ref()
const form = reactive({
  name: '', slug: '', multiplier: 1.0, description: '', name_en: '', description_en: '',
  sort_order: 0, is_default: false
})
const rules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  slug: [{ required: true, message: '请输入 slug', trigger: 'blur' }],
  multiplier: [{ required: true, message: '请输入倍率', trigger: 'blur' }],
}

async function load() {
  loading.value = true
  try {
    const res = await api.get('/admin/channel-groups')
    rows.value = res.items || []
  } catch (e) {
    ElMessage.error('加载失败: ' + (e?.response?.data?.error || e.message))
  } finally { loading.value = false }
}

function openCreate() {
  isEditing.value = false; editingId.value = null
  Object.assign(form, { name:'', slug:'', multiplier:1.0, description:'', name_en:'', description_en:'', sort_order:0, is_default:false })
  dialogVisible.value = true
}

function openEdit(row) {
  isEditing.value = true; editingId.value = row.id
  Object.assign(form, {
    name: row.name, slug: row.slug, multiplier: Number(row.multiplier),
    description: row.description || '', name_en: row.name_en || '',
    description_en: row.description_en || '', sort_order: row.sort_order || 0,
    is_default: !!row.is_default
  })
  dialogVisible.value = true
}

async function handleSave() {
  await formRef.value?.validate?.().catch(() => null)
  const payload = {
    name: form.name, slug: form.slug,
    multiplier: Number(form.multiplier),
    description: form.description || '',
    name_en: form.name_en || '',
    description_en: form.description_en || '',
    sort_order: Number(form.sort_order) || 0,
    is_default: !!form.is_default,
  }
  try {
    if (isEditing.value) {
      await api.put(`/admin/channel-groups/${editingId.value}`, payload)
      ElMessage.success('已更新')
    } else {
      await api.post('/admin/channel-groups', payload)
      ElMessage.success('已创建')
    }
    dialogVisible.value = false
    load()
  } catch (e) {
    ElMessage.error('保存失败: ' + (e?.response?.data?.error || e.message))
  }
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(`确定删除分组「${row.name}」?`, '确认', { type: 'warning' })
  } catch { return }
  try {
    await api.delete(`/admin/channel-groups/${row.id}`)
    ElMessage.success('已删除')
    load()
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '未知错误'
    if (msg.includes('channels') || msg.includes('models')) {
      ElMessage.error('请先在【模型管理】中将该分组下的模型移到其他分组，再删除')
    } else {
      ElMessage.error('删除失败: ' + msg)
    }
  }
}

onMounted(load)
</script>

<style scoped>
.page { padding: 24px; }
.header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.mb-4 { margin-bottom: 16px; }
.ml-2 { margin-left: 8px; }
</style>
