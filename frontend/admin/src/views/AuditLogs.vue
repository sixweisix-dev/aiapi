<template>
  <div>
    <el-card shadow="hover">
      <el-table :data="logs" v-loading="loading" stripe style="width: 100%">
        <el-table-column prop="action" label="操作" width="150">
          <template #default="{ row }">{{ actionLabel(row.action) }}</template>
        </el-table-column>
        <el-table-column prop="resource_type" label="资源类型" width="120" />
        <el-table-column prop="resource_id" label="资源ID" width="120" show-overflow-tooltip>
          <template #default="{ row }">{{ row.resource_id ? row.resource_id.slice(0, 12) + '...' : '-' }}</template>
        </el-table-column>
        <el-table-column prop="user_id" label="操作人" width="120" show-overflow-tooltip>
          <template #default="{ row }">{{ row.user_id ? row.user_id.slice(0, 12) + '...' : '-' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="时间" width="170">
          <template #default="{ row }">{{ dayjs(row.created_at).format('YYYY-MM-DD HH:mm:ss') }}</template>
        </el-table-column>
      </el-table>
      <div class="flex justify-center mt-4">
        <el-pagination
          v-model:current-page="page"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next"
          @current-change="fetchData"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { auditAPI } from '@/utils/api'
import dayjs from 'dayjs'

const logs = ref([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(30)
const total = ref(0)

function actionLabel(action) {
  const map = {
    update_user: '编辑用户',
    create_channel: '添加渠道',
    update_channel: '编辑渠道',
    delete_channel: '删除渠道',
    create_model: '添加模型',
    update_model: '编辑模型',
    delete_model: '删除模型',
  }
  return map[action] || action
}

async function fetchData() {
  loading.value = true
  try {
    const res = await auditAPI.list({ page: page.value, page_size: pageSize.value })
    logs.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

onMounted(fetchData)
</script>
