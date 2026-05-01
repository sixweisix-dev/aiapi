<template>
  <div class="admin-layout">
    <!-- 顶部栏（移动端显示汉堡，桌面端隐藏） -->
    <header class="topbar">
      <button class="hamburger" @click="drawerVisible = true" aria-label="菜单">
        <el-icon size="22"><Menu /></el-icon>
      </button>
      <div class="brand">
        <span class="brand-icon">⚡</span>
        <span class="brand-name">TransitAI 管理后台</span>
      </div>
      <div class="page-title-mobile">{{ pageTitle }}</div>
    </header>

    <div class="layout-body">
      <!-- 桌面端侧边栏 -->
      <aside class="sidebar-desktop">
        <div class="sidebar-header">
          <span class="brand-icon">⚡</span>
          <span class="brand-name">TransitAI</span>
        </div>
        <el-menu :default-active="currentRoute" router class="sidebar-menu">
          <el-menu-item v-for="item in menuItems" :key="item.path" :index="item.path">
            <span class="menu-emoji">{{ item.emoji }}</span>
            <span>{{ item.label }}</span>
          </el-menu-item>
        </el-menu>
        <div class="sidebar-footer">
          <div class="user-info">
            <div class="user-avatar">{{ avatarLetter }}</div>
            <div class="user-meta">
              <div class="user-email">{{ user?.email }}</div>
              <div class="user-role">管理员</div>
            </div>
          </div>
          <el-button class="logout-btn" @click="handleLogout" plain>退出登录</el-button>
        </div>
      </aside>

      <!-- 移动端抽屉 -->
      <el-drawer
        v-model="drawerVisible"
        direction="ltr"
        :with-header="false"
        size="80%"
        class="mobile-drawer"
      >
        <div class="drawer-content">
          <div class="drawer-header">
            <div class="brand-block">
              <span class="brand-icon-large">⚡</span>
              <div>
                <div class="drawer-brand-name">TransitAI</div>
                <div class="drawer-brand-sub">管理后台</div>
              </div>
            </div>
          </div>

          <div class="drawer-user-card">
            <div class="user-avatar-large">{{ avatarLetter }}</div>
            <div class="drawer-user-meta">
              <div class="drawer-user-email">{{ user?.email }}</div>
              <div class="drawer-user-role">超级管理员</div>
            </div>
          </div>

          <div class="drawer-menu">
            <div
              v-for="item in menuItems"
              :key="item.path"
              class="drawer-menu-item"
              :class="{ active: currentRoute === item.path }"
              @click="navigateTo(item.path)"
            >
              <span class="menu-emoji-large">{{ item.emoji }}</span>
              <span class="menu-label">{{ item.label }}</span>
              <el-icon class="menu-arrow"><ArrowRight /></el-icon>
            </div>
          </div>

          <div class="drawer-footer">
            <el-button class="drawer-logout" @click="handleLogout" type="danger" plain size="large">
              退出登录
            </el-button>
          </div>
        </div>
      </el-drawer>

      <!-- 主内容 -->
      <main class="main-content">
        <header class="content-header-desktop">
          <h2 class="content-title">{{ pageTitle }}</h2>
        </header>
        <div class="content-scroll">
          <router-view />
        </div>
      </main>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { Menu, ArrowRight } from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const drawerVisible = ref(false)

const menuItems = [
  { path: '/dashboard',   emoji: '📊', label: '仪表盘' },
  { path: '/profit',      emoji: '💰', label: '利润看板' },
  { path: '/users',       emoji: '👥', label: '用户管理' },
  { path: '/channels',    emoji: '🔌', label: '上游渠道' },
  { path: '/models',      emoji: '🤖', label: '模型管理' },
  { path: '/logs',        emoji: '📝', label: '请求日志' },
  { path: '/recharge',    emoji: '💳', label: '充值记录' },
  { path: '/audit-logs',  emoji: '🔍', label: '操作日志' },
  { path: '/settings',    emoji: '⚙️', label: '系统设置' },
  { path: '/redeem-codes', emoji: '🎁', label: '兑换码管理' },
]

const currentRoute = computed(() => route.path)
const user = computed(() => auth.user)
const avatarLetter = computed(() => (user.value?.email?.[0] || 'A').toUpperCase())

const pageTitle = computed(() => {
  const found = menuItems.find(m => m.path === route.path)
  return found ? found.label : 'TransitAI'
})

watch(() => route.path, () => { drawerVisible.value = false })

const navigateTo = (path) => {
  drawerVisible.value = false
  if (route.path !== path) router.push(path)
}

const handleLogout = () => {
  auth.logout()
  router.push('/login')
}
</script>

<style scoped>
/* === 整体布局 === */
.admin-layout {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: #f5f7fa;
}
.layout-body {
  flex: 1;
  display: flex;
  overflow: hidden;
}

/* === 顶部栏（默认隐藏，移动端显示） === */
.topbar { display: none; }

/* === 桌面端侧边栏 === */
.sidebar-desktop {
  width: 240px;
  background: #fff;
  border-right: 1px solid #e5e7eb;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}
.sidebar-header {
  height: 64px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 20px;
  border-bottom: 1px solid #e5e7eb;
}
.brand-icon {
  font-size: 22px;
  filter: drop-shadow(0 2px 4px rgba(102, 126, 234, 0.4));
}
.brand-name {
  font-size: 17px;
  font-weight: 700;
  background: linear-gradient(135deg, #667eea, #764ba2);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}
.sidebar-menu {
  border-right: none !important;
  flex: 1;
  padding: 8px 0;
}
.sidebar-menu :deep(.el-menu-item) {
  height: 46px;
  line-height: 46px;
  margin: 2px 8px;
  border-radius: 8px;
  color: #4b5563;
}
.sidebar-menu :deep(.el-menu-item.is-active) {
  background: linear-gradient(135deg, #667eea15, #764ba215);
  color: #667eea;
  font-weight: 600;
}
.sidebar-menu :deep(.el-menu-item:hover) {
  background: #f3f4f6;
}
.menu-emoji {
  margin-right: 12px;
  font-size: 16px;
}
.sidebar-footer {
  padding: 16px;
  border-top: 1px solid #e5e7eb;
}
.user-info {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}
.user-avatar {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 15px;
  flex-shrink: 0;
}
.user-meta { flex: 1; min-width: 0; }
.user-email {
  font-size: 13px;
  font-weight: 600;
  color: #1f2937;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.user-role {
  font-size: 11px;
  color: #9ca3af;
}
.logout-btn { width: 100%; }

/* === 主内容 === */
.main-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.content-header-desktop {
  height: 64px;
  background: #fff;
  border-bottom: 1px solid #e5e7eb;
  display: flex;
  align-items: center;
  padding: 0 24px;
  flex-shrink: 0;
}
.content-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: #1f2937;
}
.content-scroll {
  flex: 1;
  overflow: auto;
  padding: 20px;
}

/* === 移动端抽屉样式 === */
.mobile-drawer :deep(.el-drawer) {
  background: #fff;
}
.drawer-content {
  display: flex;
  flex-direction: column;
  height: 100%;
}
.drawer-header {
  padding: 24px 20px 20px;
  border-bottom: 1px solid #f3f4f6;
}
.brand-block {
  display: flex;
  align-items: center;
  gap: 12px;
}
.brand-icon-large { font-size: 32px; }
.drawer-brand-name {
  font-size: 20px;
  font-weight: 800;
  background: linear-gradient(135deg, #667eea, #764ba2);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}
.drawer-brand-sub {
  font-size: 12px;
  color: #9ca3af;
  margin-top: 2px;
}
.drawer-user-card {
  margin: 16px;
  padding: 16px;
  border-radius: 14px;
  background: linear-gradient(135deg, #667eea, #764ba2);
  display: flex;
  align-items: center;
  gap: 12px;
  color: #fff;
}
.user-avatar-large {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.25);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: 700;
}
.drawer-user-meta { flex: 1; min-width: 0; }
.drawer-user-email {
  font-weight: 600;
  font-size: 14px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.drawer-user-role {
  font-size: 12px;
  opacity: 0.85;
  margin-top: 2px;
}
.drawer-menu {
  flex: 1;
  padding: 8px 16px;
  overflow-y: auto;
}
.drawer-menu-item {
  display: flex;
  align-items: center;
  padding: 14px 12px;
  margin-bottom: 4px;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.drawer-menu-item:active { background: #f3f4f6; }
.drawer-menu-item.active {
  background: linear-gradient(135deg, #667eea15, #764ba215);
  color: #667eea;
}
.drawer-menu-item.active .menu-label { font-weight: 600; }
.menu-emoji-large { font-size: 22px; margin-right: 14px; }
.menu-label {
  flex: 1;
  font-size: 15px;
  color: #374151;
}
.drawer-menu-item.active .menu-label { color: #667eea; }
.menu-arrow { color: #d1d5db; font-size: 14px; }
.drawer-footer {
  padding: 16px;
  border-top: 1px solid #f3f4f6;
}
.drawer-logout { width: 100%; }

/* === 响应式：移动端 === */
@media (max-width: 768px) {
  .sidebar-desktop { display: none; }
  .content-header-desktop { display: none; }
  .topbar {
    display: flex;
    align-items: center;
    height: 56px;
    background: #fff;
    border-bottom: 1px solid #e5e7eb;
    padding: 0 12px;
    gap: 8px;
    flex-shrink: 0;
  }
  .hamburger {
    background: none;
    border: none;
    padding: 8px;
    border-radius: 8px;
    cursor: pointer;
    color: #4b5563;
    display: flex;
    align-items: center;
  }
  .hamburger:active { background: #f3f4f6; }
  .topbar .brand { display: none; }
  .page-title-mobile {
    flex: 1;
    font-size: 16px;
    font-weight: 600;
    color: #1f2937;
    text-align: center;
    padding-right: 38px;
  }
  .content-scroll { padding: 12px; }
}

/* 桌面端隐藏顶栏内的标题 */
@media (min-width: 769px) {
  .topbar .brand { display: none; }
  .page-title-mobile { display: none; }
}
</style>
