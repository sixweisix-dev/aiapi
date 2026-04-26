<template>
  <div class="h-screen flex flex-col bg-gray-50">
    <!-- Top Header -->
    <header class="h-14 bg-white border-b border-gray-200 flex items-center px-4 flex-shrink-0 shadow-sm z-10">
      <el-button text @click="drawerOpen = true" class="mr-3">
        <el-icon size="22"><Expand /></el-icon>
      </el-button>
      <div class="flex items-center gap-2 flex-1">
        <div class="w-7 h-7 bg-blue-600 rounded-lg flex items-center justify-center">
          <span class="text-white text-xs font-bold">AI</span>
        </div>
        <h1 class="text-base font-bold text-gray-800">TransitAI</h1>
      </div>
      <el-tag :type="balance >= 0 ? 'success' : 'danger'" size="small" effect="dark">
        ¥{{ balance.toFixed(2) }}
      </el-tag>
    </header>

    <!-- Drawer -->
    <el-drawer v-model="drawerOpen" direction="ltr" size="72%" :with-header="false">
      <div class="flex flex-col h-full">
        <!-- Drawer Header -->
        <div class="h-14 flex items-center px-4 border-b border-gray-100">
          <div class="flex items-center gap-2 flex-1">
            <div class="w-8 h-8 bg-blue-600 rounded-lg flex items-center justify-center">
              <span class="text-white text-sm font-bold">AI</span>
            </div>
            <h1 class="text-lg font-bold text-gray-800">TransitAI</h1>
          </div>
          <el-button text @click="drawerOpen = false">
            <el-icon><Close /></el-icon>
          </el-button>
        </div>

        <!-- User Info -->
        <div class="px-4 py-3 bg-blue-50 mx-3 mt-3 rounded-xl">
          <div class="flex items-center gap-2 mb-1">
            <div class="w-8 h-8 bg-blue-600 rounded-full flex items-center justify-center">
              <span class="text-white text-sm font-semibold">{{ user?.email?.[0]?.toUpperCase() }}</span>
            </div>
            <span class="text-sm text-gray-700 truncate">{{ user?.email }}</span>
          </div>
          <p class="text-xs text-gray-500">当前余额</p>
          <p class="text-xl font-bold" :class="balance >= 0 ? 'text-blue-600' : 'text-red-500'">
            ¥{{ balance.toFixed(4) }}
          </p>
        </div>

        <!-- Menu -->
        <el-menu :default-active="currentRoute" router class="border-r-0 flex-1 mt-2" @select="drawerOpen = false">
          <el-menu-item index="/dashboard">
            <el-icon><Odometer /></el-icon><span>仪表盘</span>
          </el-menu-item>
          <el-menu-item index="/api-keys">
            <el-icon><Key /></el-icon><span>API Key 管理</span>
          </el-menu-item>
          <el-menu-item index="/recharge">
            <el-icon><Coin /></el-icon><span>充值</span>
          </el-menu-item>
          <el-menu-item index="/billing">
            <el-icon><Document /></el-icon><span>消费明细</span>
          </el-menu-item>
          <el-menu-item index="/models">
            <el-icon><Grid /></el-icon><span>模型与价格</span>
          </el-menu-item>
          <el-menu-item index="/api-docs">
            <el-icon><Reading /></el-icon><span>API 文档</span>
          </el-menu-item>
          <el-menu-item index="/playground">
            <el-icon><Monitor /></el-icon><span>Playground</span>
          </el-menu-item>
          <el-menu-item index="/change-password">
            <el-icon><Lock /></el-icon><span>修改密码</span>
          </el-menu-item>
        </el-menu>

        <!-- Logout -->
        <div class="p-4 border-t border-gray-100">
          <el-button type="danger" plain class="w-full" @click="handleLogout">
            <el-icon><SwitchButton /></el-icon> 退出登录
          </el-button>
        </div>
      </div>
    </el-drawer>

    <!-- Main Content -->
    <main class="flex-1 overflow-auto p-4 bg-gray-50">
      <router-view />
    </main>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { dashboardAPI } from '@/utils/api'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const balance = ref(0)
const drawerOpen = ref(false)

const currentRoute = computed(() => route.path)
const user = computed(() => auth.user)

function handleLogout() {
  drawerOpen.value = false
  auth.logout()
  router.push('/login')
}

async function fetchBalance() {
  try {
    const data = await dashboardAPI.stats()
    balance.value = data.balance || 0
  } catch {}
}

fetchBalance()
watch(() => route.path, () => {
  if (route.path === '/dashboard') fetchBalance()
})
</script>
