<template>
  <div>
    <el-card shadow="hover">
      <template #header>
        <div class="flex items-center justify-between">
          <span class="font-medium">模型与价格</span>
          <el-tag type="info">价格单位: USD / 1K tokens</el-tag>
        </div>
      </template>
      <el-table :data="models" v-loading="loading" empty-text="暂无可用模型">
        <el-table-column prop="display_name" label="模型名称" min-width="160" />
        <el-table-column prop="name" label="标识" width="200">
          <template #default="{ row }">
            <code class="bg-gray-100 px-2 py-0.5 rounded text-sm">{{ row.name }}</code>
          </template>
        </el-table-column>
        <el-table-column prop="provider" label="提供商" width="100">
          <template #default="{ row }">
            <el-tag>{{ row.provider }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="context_length" label="上下文" width="90">
          <template #default="{ row }">
            {{ (row.context_length / 1000).toFixed(0) }}K
          </template>
        </el-table-column>
        <el-table-column label="输入价格" width="120">
          <template #default="{ row }">
            ${{ row.input_price?.toFixed(6) }}
          </template>
        </el-table-column>
        <el-table-column label="输出价格" width="120">
          <template #default="{ row }">
            ${{ row.output_price?.toFixed(6) }}
          </template>
        </el-table-column>
        <el-table-column label="倍率" width="70">
          <template #default="{ row }">
            {{ row.multiplier }}x
          </template>
        </el-table-column>
        <el-table-column prop="description" label="说明" min-width="200" show-overflow-tooltip />
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { userModelsAPI } from '@/utils/api'

const loading = ref(true)
const models = ref([])

onMounted(async () => {
  try {
    const data = await userModelsAPI.list()
    models.value = data.items || []
  } catch {
    // handled
  } finally {
    loading.value = false
  }
})
</script>
