<template>
  <div>
    <!-- Search bar -->
    <el-card shadow="hover" class="mb-4">
      <el-form :inline="true" :model="query" @submit.prevent="fetchData">
        <el-form-item label="搜索">
          <el-input v-model="query.search" placeholder="邮箱/用户名" clearable @clear="fetchData" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="query.role" placeholder="全部" clearable @change="fetchData">
            <el-option label="全部" value="" />
            <el-option label="普通" value="user" />
            <el-option label="VIP" value="vip" />
            <el-option label="管理员" value="admin" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="fetchData">搜索</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Users table -->
    <el-card shadow="hover">
      <el-table :data="users" v-loading="loading" stripe style="width: 100%">
        <el-table-column prop="email" label="邮箱" min-width="200" />
        <el-table-column prop="username" label="用户名" width="120" />
        <el-table-column prop="role" label="角色" width="80">
          <template #default="{ row }">
            <el-tag :type="roleTagType(row.role)" size="small">{{ row.role }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="balance" label="余额" width="120" align="right">
          <template #default="{ row }">¥{{ row.balance.toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="total_spent" label="总消费" width="120" align="right">
          <template #default="{ row }">¥{{ row.total_spent.toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="request_count" label="请求数" width="90" align="right" />
        <el-table-column prop="is_active" label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.is_active ? 'success' : 'danger'" size="small">
              {{ row.is_active ? '正常' : '封禁' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="membership_tier" label="会员等级" width="100">
          <template #default="{ row }">
            <el-tag v-if="row.membership_tier === 'pro'" type="warning" size="small">专业版</el-tag>
            <el-tag v-else-if="row.membership_tier === 'enterprise'" type="danger" size="small">企业版</el-tag>
            <span v-else style="color:#9ca3af;font-size:12px">免费版</span>
          </template>
        </el-table-column>
        <el-table-column prop="membership_expires_at" label="会员到期" width="120">
          <template #default="{ row }">
            <span v-if="row.membership_expires_at" style="font-size:12px">
              {{ dayjs(row.membership_expires_at).format('YYYY-MM-DD') }}
            </span>
            <span v-else style="color:#9ca3af;font-size:12px">—</span>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="注册时间" width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openEdit(row)">编辑</el-button>
            <el-button
              size="small"
              :type="row.is_active ? 'warning' : 'success'"
              @click="toggleActive(row)"
            >
              {{ row.is_active ? '封禁' : '解封' }}
            </el-button>
          </template>
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

    <!-- Edit User Dialog -->
    <el-dialog v-model="dialogVisible" title="编辑用户" width="450px">
      <el-form :model="editForm" label-width="100px">
        <el-form-item label="角色">
          <el-select v-model="editForm.role">
            <el-option label="普通用户" value="user" />
            <el-option label="管理员" value="admin" />
          </el-select>
        </el-form-item>
        <el-form-item label="调整余额">
          <el-input-number v-model="editForm.balance_adjust" :precision="2" :step="10" />
          <span class="text-xs text-gray-400 ml-2">正数增加，负数扣除</span>
        </el-form-item>
        <el-form-item label="邮箱验证">
          <el-switch v-model="editForm.email_verified" />
        </el-form-item>
        <el-form-item label="会员等级">
          <el-select v-model="editForm.membership_tier" style="width:140px">
            <el-option label="免费版" value="free" />
            <el-option label="专业版 (Pro)" value="pro" />
            <el-option label="企业版 (Enterprise)" value="enterprise" />
          </el-select>
        </el-form-item>
        <el-form-item label="会员天数" v-if="editForm.membership_tier !== 'free'">
          <el-input-number v-model="editForm.membership_days" :min="0" :max="3650" :step="30" />
          <div style="font-size:11px;color:#9ca3af;margin-top:4px">0=清除会员；>0=从现在起顺延N天</div>
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
import { usersAPI } from '@/utils/api'
import dayjs from 'dayjs'

const users = ref([])
const loading = ref(false)
const saving = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const dialogVisible = ref(false)

const query = ref({ search: '', role: '' })
const editForm = ref({ role: 'user', balance_adjust: 0, email_verified: false, membership_tier: 'free', membership_days: 30 })
const editingUser = ref(null)

function formatTime(t) {
  return t ? dayjs(t).format('YYYY-MM-DD HH:mm') : '-'
}

function roleTagType(role) {
  const map = { admin: 'danger', vip: 'warning', user: 'info', guest: '' }
  return map[role] || 'info'
}

async function fetchData() {
  loading.value = true
  try {
    const res = await usersAPI.list({ page: page.value, page_size: pageSize.value, ...query.value })
    users.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

function openEdit(row) {
  editingUser.value = row
  editForm.value = {
    role: row.role,
    balance_adjust: 0,
    email_verified: row.email_verified,
    membership_tier: row.membership_tier || 'free',
    membership_days: 30,
  }
  dialogVisible.value = true
}

async function handleSave() {
  if (!editingUser.value) return
  saving.value = true
  try {
    const data = {}
    if (editForm.value.role !== editingUser.value.role) data.role = editForm.value.role
    if (editForm.value.balance_adjust !== 0) data.balance_adjust = editForm.value.balance_adjust
    if (editForm.value.email_verified !== editingUser.value.email_verified) data.email_verified = editForm.value.email_verified
    const origTier = editingUser.value.membership_tier || 'free'
    if (editForm.value.membership_tier !== origTier) {
      data.membership_tier = editForm.value.membership_tier
      data.membership_days = editForm.value.membership_tier === 'free' ? 0 : (editForm.value.membership_days || 30)
    } else if (editForm.value.membership_tier !== 'free' && editForm.value.membership_days > 0) {
      data.membership_days = editForm.value.membership_days
    }
    if (Object.keys(data).length === 0) {
      ElMessage.info('没有修改')
      return
    }
    await usersAPI.update(editingUser.value.id, data)
    ElMessage.success('更新成功')
    dialogVisible.value = false
    await fetchData()
  } finally {
    saving.value = false
  }
}

async function toggleActive(row) {
  try {
    await ElMessageBox.confirm(
      row.is_active ? '确定要封禁该用户吗？' : '确定要解封该用户吗？',
      '提示'
    )
    await usersAPI.update(row.id, { is_active: !row.is_active })
    ElMessage.success(row.is_active ? '已封禁' : '已解封')
    await fetchData()
  } catch { /* cancelled */ }
}

onMounted(fetchData)
</script>
