<template>
  <div class="h-screen flex bg-gray-50">
    <!-- Sidebar -->
    <div class="w-60 bg-white border-r border-gray-200 flex flex-col flex-shrink-0">
      <div class="h-14 flex items-center px-5 border-b border-gray-200">
        <h1 class="text-lg font-bold text-gray-800">AI API 管理</h1>
      </div>
      <el-menu
        :default-active="currentRoute"
        router
        class="border-r-0 flex-1"
      >
        <el-menu-item index="/dashboard">
          <el-icon><Odometer /></el-icon>
          <span>仪表盘</span>
        </el-menu-item>
        <el-menu-item index="/users">
          <el-icon><User /></el-icon>
          <span>用户管理</span>
        </el-menu-item>
        <el-menu-item index="/channels">
          <el-icon><Connection /></el-icon>
          <span>上游渠道</span>
        </el-menu-item>
        <el-menu-item index="/models">
          <el-icon><Grid /></el-icon>
          <span>模型管理</span>
        </el-menu-item>
        <el-menu-item index="/logs">
          <el-icon><Document /></el-icon>
          <span>请求日志</span>
        </el-menu-item>
        <el-menu-item index="/recharge">
          <el-icon><Coin /></el-icon>
          <span>充值记录</span>
        </el-menu-item>
        <el-menu-item index="/audit-logs">
          <el-icon><List /></el-icon>
          <span>操作日志</span>
        </el-menu-item>
      </el-menu>
      <div class="p-4 border-t border-gray-200">
        <div class="flex items-center justify-between">
          <span class="text-sm text-gray-600 truncate">{{ user?.email }}</span>
          <el-button text size="small" @click="handleLogout">退出</el-button>
        </div>
      </div>
    </div>
    <!-- Main content -->
    <div class="flex-1 flex flex-col overflow-hidden">
      <header class="h-14 bg-white border-b border-gray-200 flex items-center px-6 flex-shrink-0">
        <h2 class="text-base font-medium text-gray-700">{{ pageTitle }}</h2>
      </header>
      <main class="flex-1 overflow-auto p-6 bg-gray-50">
        <router-view />
      </main>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const currentRoute = computed(() => route.path)
const user = computed(() => auth.user)

const pageTitle = computed(() => {
  const map = {
    '/dashboard': '仪表盘',
    '/users': '用户管理',
    '/channels': '上游渠道',
    '/models': '模型管理',
    '/logs': '请求日志',
    '/recharge': '充值记录',
    '/audit-logs': '操作日志',
  }
  return map[route.path] || 'AI API 管理后台'
})

function handleLogout() {
  auth.logout()
  router.push('/login')
}
</script>
