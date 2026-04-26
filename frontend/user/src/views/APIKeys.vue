<template>
  <div>
    <!-- Create Key -->
    <el-card shadow="hover" class="mb-6">
      <template #header>
        <span class="font-medium">创建 API Key</span>
      </template>
      <el-form :inline="true" :model="createForm">
        <el-form-item label="名称">
          <el-input v-model="createForm.name" placeholder="例如: 开发环境" />
        </el-form-item>
        <el-form-item label="RPM 限制">
          <el-input-number v-model="createForm.rpm_limit" :min="0" :max="10000" placeholder="每分钟请求数" />
        </el-form-item>
        <el-form-item label="TPM 限制">
          <el-input-number v-model="createForm.tpm_limit" :min="0" :max="1000000" placeholder="每分钟 Token 数" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="creating" @click="handleCreate">
            <el-icon><Plus /></el-icon>创建
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Key List -->
    <el-card shadow="hover">
      <template #header>
        <span class="font-medium">我的 API Key ({{ total }})</span>
      </template>
      <el-table :data="keys" v-loading="loading" empty-text="暂无 API Key">
        <el-table-column prop="name" label="名称" width="120" />
        <el-table-column prop="prefix" label="前缀" width="120">
          <template #default="{ row }">
            <code class="bg-gray-100 px-2 py-1 rounded text-sm">sk-{{ row.prefix }}...</code>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.is_active ? 'success' : 'danger'" size="small">
              {{ row.is_active ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="total_used" label="使用次数" width="90" />
        <el-table-column label="最近使用" width="160">
          <template #default="{ row }">
            <span class="text-xs text-gray-400">
              {{ row.last_used_at ? dayjs(row.last_used_at).format('YYYY-MM-DD HH:mm') : '从未使用' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="160">
          <template #default="{ row }">
            <span class="text-xs text-gray-400">{{ dayjs(row.created_at).format('YYYY-MM-DD HH:mm') }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button text size="small" :type="row.is_active ? 'warning' : 'success'" @click="handleToggle(row)">
              {{ row.is_active ? '禁用' : '启用' }}
            </el-button>
            <el-popconfirm title="确定删除这个 API Key？" @confirm="handleDelete(row.id)">
              <template #reference>
                <el-button text size="small" type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- New Key Dialog -->
    <el-dialog v-model="showNewKey" title="API Key 创建成功" width="500px">
      <el-alert type="warning" :closable="false" class="mb-4">
        <template #title>
          请立即复制并安全保存此 Key，关闭后将无法再次查看完整 Key！
        </template>
      </el-alert>
      <el-input v-model="newKeyValue" type="textarea" :rows="3" readonly />
      <div class="mt-2">
        <el-button type="primary" @click="copyKey">复制 Key</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { apiKeysAPI } from '@/utils/api'
import dayjs from 'dayjs'

const loading = ref(true)
const creating = ref(false)
const keys = ref([])
const total = ref(0)
const showNewKey = ref(false)
const newKeyValue = ref('')

const createForm = ref({
  name: '',
  rpm_limit: 0,
  tpm_limit: 0,
})

onMounted(fetchKeys)

async function fetchKeys() {
  loading.value = true
  try {
    const data = await apiKeysAPI.list()
    keys.value = data
    total.value = data.length
  } catch {
    // handled
  } finally {
    loading.value = false
  }
}

async function handleCreate() {
  if (!createForm.value.name) {
    ElMessage.warning('请输入名称')
    return
  }
  creating.value = true
  try {
    const data = await apiKeysAPI.create({
      name: createForm.value.name,
      rpm_limit: createForm.value.rpm_limit > 0 ? createForm.value.rpm_limit : undefined,
      tpm_limit: createForm.value.tpm_limit > 0 ? createForm.value.tpm_limit : undefined,
    })
    newKeyValue.value = data.key
    showNewKey.value = true
    createForm.value.name = ''
    ElMessage.success('创建成功')
    await fetchKeys()
  } catch {
    // handled
  } finally {
    creating.value = false
  }
}

async function handleToggle(row) {
  try {
    await apiKeysAPI.toggle(row.id)
    ElMessage.success(row.is_active ? '已禁用' : '已启用')
    await fetchKeys()
  } catch {
    // handled
  }
}

async function handleDelete(id) {
  try {
    await apiKeysAPI.delete(id)
    ElMessage.success('已删除')
    await fetchKeys()
  } catch {
    // handled
  }
}

function copyKey() {
  navigator.clipboard.writeText(newKeyValue.value).then(() => {
    ElMessage.success('已复制')
  })
}
</script>
