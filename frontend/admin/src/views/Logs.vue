<template>
  <div>
    <!-- Filter bar -->
    <el-card shadow="hover" class="mb-4">
      <el-form :inline="true" :model="query" @submit.prevent="fetchData">
        <el-form-item label="用户ID">
          <el-input v-model="query.user_id" placeholder="用户UUID" clearable style="width: 200px" @clear="fetchData" />
        </el-form-item>
        <el-form-item label="模型">
          <el-input v-model="query.model" placeholder="模型名称" clearable style="width: 150px" @clear="fetchData" />
        </el-form-item>
        <el-form-item label="状态码">
          <el-input v-model="query.status_code" placeholder="如 200" clearable style="width: 100px" @clear="fetchData" />
        </el-form-item>
        <el-form-item label="开始">
          <el-date-picker v-model="query.start_date" type="datetime" placeholder="开始时间" value-format="YYYY-MM-DD HH:mm:ss" style="width: 180px" />
        </el-form-item>
        <el-form-item label="结束">
          <el-date-picker v-model="query.end_date" type="datetime" placeholder="结束时间" value-format="YYYY-MM-DD HH:mm:ss" style="width: 180px" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="fetchData">查询</el-button>
          <el-button @click="resetQuery">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Logs table -->
    <el-card shadow="hover">
      <el-table :data="logs" v-loading="loading" stripe style="width: 100%" size="small">
        <el-table-column prop="id" label="ID" width="100" show-overflow-tooltip>
          <template #default="{ row }">{{ row.id.slice(0, 8) }}...</template>
        </el-table-column>
        <el-table-column prop="user_email" label="用户" width="140" show-overflow-tooltip />
        <el-table-column prop="model_name" label="模型" width="130" />
        <el-table-column prop="path" label="路径" width="100" />
        <el-table-column prop="status_code" label="状态" width="70" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status_code === 200 ? 'success' : 'danger'" size="small">{{ row.status_code }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="prompt_tokens" label="输入Token" width="90" align="right" />
        <el-table-column prop="completion_tokens" label="输出Token" width="90" align="right" />
        <el-table-column prop="total_tokens" label="总Token" width="80" align="right" />
        <el-table-column prop="cost" label="费用" width="80" align="right">
          <template #default="{ row }">¥{{ row.cost.toFixed(6) }}</template>
        </el-table-column>
        <el-table-column prop="duration_ms" label="耗时" width="70" align="right">
          <template #default="{ row }">{{ row.duration_ms }}ms</template>
        </el-table-column>
        <el-table-column prop="created_at" label="时间" width="160">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
      </el-table>
      <div class="flex justify-center mt-4">
        <el-pagination
          v-model:current-page="page"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next, total"
          @current-change="fetchData"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { logsAPI } from '@/utils/api'
import dayjs from 'dayjs'

const logs = ref([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

const query = reactive({
  user_id: '',
  model: '',
  status_code: '',
  start_date: '',
  end_date: '',
})

function formatTime(t) {
  return t ? dayjs(t).format('YYYY-MM-DD HH:mm:ss') : '-'
}

function resetQuery() {
  Object.assign(query, { user_id: '', model: '', status_code: '', start_date: '', end_date: '' })
  page.value = 1
  fetchData()
}

async function fetchData() {
  loading.value = true
  try {
    const params = { page: page.value, page_size: pageSize.value }
    if (query.user_id) params.user_id = query.user_id
    if (query.model) params.model = query.model
    if (query.status_code) params.status_code = parseInt(query.status_code)
    if (query.start_date) params.start_date = query.start_date
    if (query.end_date) params.end_date = query.end_date
    const res = await logsAPI.list(params)
    logs.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

onMounted(fetchData)
</script>
