<template>
  <div class="user-layout">
    <!-- 顶部栏 -->
    <header class="topbar">
      <button class="hamburger" @click="drawerOpen = true">
        <el-icon size="22"><Expand /></el-icon>
      </button>
      <div class="brand">
        <span class="brand-icon">⚡</span>
        <span class="brand-name">TransitAI</span>
      </div>
      <button class="lang-toggle" @click="toggleLang">{{ lang === 'zh' ? 'EN' : '中' }}</button>
      <div class="balance-pill" :class="{ negative: balance < 0 }">
        ¥{{ balance.toFixed(2) }}
      </div>
    </header>

    <!-- 抽屉菜单 -->
    <el-drawer v-model="drawerOpen" direction="ltr" size="80%" :with-header="false" class="user-drawer">
      <div class="drawer-content">
        <div class="drawer-header">
          <div class="brand-block">
            <span class="brand-icon-large">⚡</span>
            <div>
              <div class="drawer-brand-name">{{ t('brand.name') }}</div>
              <div class="drawer-brand-sub">{{ t('brand.sub') }}</div>
            </div>
          </div>
        </div>

        <!-- 用户卡片 -->
        <div class="user-card">
          <div class="user-card-top">
            <div class="user-avatar">{{ avatarLetter }}</div>
            <div class="user-meta">
              <div class="user-email">{{ user?.email }}</div>
              <div class="user-tag" :class="tierClass">{{ tierLabel }}</div>
            </div>
          </div>
          <div class="balance-block">
            <div class="balance-label">{{ t('common.balance') }}</div>
            <div class="balance-value">¥{{ balance.toFixed(4) }}</div>
          </div>
        </div>

        <!-- 菜单 -->
        <div class="drawer-menu">
          <div
            v-for="item in menuItems"
            :key="item.path"
            class="drawer-item"
            :class="{ active: currentRoute === item.path }"
            @click="navigateTo(item.path)"
          >
            <span class="item-emoji">{{ item.emoji }}</span>
            <span class="item-label">{{ item.label }}</span>
            <el-icon class="item-arrow"><ArrowRight /></el-icon>
          </div>
        </div>

        <div class="drawer-footer">
          <el-button class="logout-btn" @click="handleLogout" type="danger" plain size="large">
            退出登录
          </el-button>
        </div>
      </div>
    </el-drawer>

    <!-- PC 侧边栏 (>= 769px) -->
    <aside class="pc-sidebar">
      <div class="pc-sidebar-header">
        <span class="brand-icon-large">⚡</span>
        <div>
          <div class="drawer-brand-name">{{ t('brand.name') }}</div>
          <div class="drawer-brand-sub">{{ t('brand.sub') }}</div>
        </div>
      </div>

      <div class="user-card">
        <div class="user-card-top">
          <div class="user-avatar">{{ avatarLetter }}</div>
          <div class="user-meta">
            <div class="user-email">{{ user?.email }}</div>
            <div class="user-tag" :class="tierClass">{{ tierLabel }}</div>
          </div>
        </div>
        <div class="balance-block">
          <div class="balance-label">{{ t('common.balance') }}</div>
          <div class="balance-value">¥{{ balance.toFixed(4) }}</div>
        </div>
      </div>

      <div class="drawer-menu">
        <div
          v-for="item in menuItems"
          :key="'pc-' + item.path"
          class="drawer-item"
          :class="{ active: currentRoute === item.path }"
          @click="navigateTo(item.path)"
        >
          <span class="item-emoji">{{ item.emoji }}</span>
          <span class="item-label">{{ item.label }}</span>
        </div>
      </div>

      <div class="drawer-footer">
        <button class="pc-lang-toggle" @click="toggleLang">🌐 {{ lang === 'zh' ? 'English' : '中文' }}</button>
        <el-button class="logout-btn" @click="handleLogout" type="danger" plain size="large">
          退出登录
        </el-button>
      </div>
    </aside>

    <!-- 主内容 -->
    <main class="main-area">
      <router-view />
    </main>
  </div>
</template>

<script setup>
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { i18n, setLocale, currentLocale } from '@/i18n'
import { dashboardAPI } from '@/utils/api'
import { Expand, ArrowRight } from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const t = (key) => i18n.global.t(key)
const lang = ref(currentLocale())
function toggleLang() {
  const next = lang.value === 'zh' ? 'en' : 'zh'
  setLocale(next)
  lang.value = next
}
const balance = ref(0)
const drawerOpen = ref(false)

const menuItemsRaw = [
  { path: '/dashboard',       emoji: '🏠', key: 'menu.home' },
  { path: '/api-keys',        emoji: '🔑', key: 'menu.apiKeys' },
  { path: '/recharge',        emoji: '💳', key: 'menu.recharge' },
  { path: '/billing',         emoji: '📋', key: 'menu.billing' },
  { path: '/models',          emoji: '🤖', key: 'menu.models' },
  { path: '/playground',      emoji: '🎮', key: 'menu.playground' },
  { path: '/api-docs',        emoji: '📖', key: 'menu.apiDocs' },
  { path: '/change-password', emoji: '🔒', key: 'menu.changePassword' },
]
const menuItems = computed(() => menuItemsRaw.map(m => ({ ...m, label: t(m.key) })))

const currentRoute = computed(() => route.path)
const user = computed(() => auth.user)
const avatarLetter = computed(() => (user.value?.email?.[0] || 'U').toUpperCase())
const tierLabel = computed(() => {
  const t = user.value?.membership_tier
  const exp = user.value?.membership_expires_at
  if (t === 'pro' && exp && new Date(exp) > new Date()) return '⭐ 专业版'
  if (t === 'enterprise' && exp && new Date(exp) > new Date()) return '💎 企业版'
  return '普通用户'
})
const tierClass = computed(() => {
  const t = user.value?.membership_tier
  const exp = user.value?.membership_expires_at
  if (t === 'pro' && exp && new Date(exp) > new Date()) return 'tag-pro'
  if (t === 'enterprise' && exp && new Date(exp) > new Date()) return 'tag-enterprise'
  return ''
})

function navigateTo(path) {
  drawerOpen.value = false
  if (route.path !== path) router.push(path)
}

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

// 锁视口：避免登录后页面出现双滚动条
onMounted(() => document.body.classList.add('locked-viewport'))
onUnmounted(() => document.body.classList.remove('locked-viewport'))

fetchBalance()

// 全局事件：任何页面 dispatch 'balance-changed' 都会触发右上角余额刷新
const balanceListener = () => fetchBalance()
window.addEventListener('balance-changed', balanceListener)
onUnmounted(() => window.removeEventListener('balance-changed', balanceListener))

watch(() => route.path, () => {
  if (route.path === '/dashboard') fetchBalance()
})

// 每次加载刷新用户信息（会员状态等，仅已登录时）
if (auth.isLoggedIn) auth.fetchMe()
</script>

<style scoped>
.user-layout {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: #f5f7fa;
}
/* 顶部栏 */
.topbar {
  height: 56px;
  background: #fff;
  display: flex;
  align-items: center;
  padding: 0 12px;
  gap: 10px;
  box-shadow: 0 1px 3px rgba(0,0,0,0.06);
  flex-shrink: 0;
  z-index: 100;
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
.brand {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
}
.brand-icon { font-size: 20px; }
.brand-name {
  font-size: 17px;
  font-weight: 800;
  background: linear-gradient(135deg, #667eea, #764ba2);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}
.balance-pill {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  font-weight: 700;
  font-size: 13px;
  padding: 6px 12px;
  border-radius: 14px;
  box-shadow: 0 2px 8px rgba(102, 126, 234, 0.3);
}
.balance-pill.negative {
  background: linear-gradient(135deg, #f5576c, #f093fb);
  box-shadow: 0 2px 8px rgba(245, 87, 108, 0.3);
}

/* 抽屉 */
.user-drawer :deep(.el-drawer) { background: #fff; }
.drawer-content {
  display: flex;
  flex-direction: column;
  height: 100%;
}
.drawer-header {
  padding: 24px 20px 16px;
  border-bottom: 1px solid #f3f4f6;
}
.brand-block { display: flex; align-items: center; gap: 12px; }
.brand-icon-large { font-size: 32px; }
.drawer-brand-name {
  font-size: 22px;
  font-weight: 800;
  background: linear-gradient(135deg, #667eea, #764ba2);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  line-height: 1.2;
}
.drawer-brand-sub {
  font-size: 11px;
  color: #9ca3af;
  margin-top: 2px;
}

/* 用户卡片 */
.user-card {
  margin: 16px;
  padding: 18px;
  border-radius: 16px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
  box-shadow: 0 8px 24px rgba(102, 126, 234, 0.35);
}
.user-card-top {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-bottom: 14px;
  border-bottom: 1px solid rgba(255,255,255,0.2);
  margin-bottom: 14px;
}
.user-avatar {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background: rgba(255,255,255,0.25);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  font-weight: 700;
}
.user-meta { flex: 1; min-width: 0; }
.user-email {
  font-size: 14px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.user-tag {
  font-size: 11px;
  opacity: 0.85;
  margin-top: 2px;
}
.balance-label {
  font-size: 12px;
  opacity: 0.85;
  margin-bottom: 4px;
}
.balance-value {
  font-size: 24px;
  font-weight: 800;
  letter-spacing: -0.5px;
}

/* 菜单 */
.drawer-menu {
  flex: 1;
  padding: 4px 12px 12px;
  overflow-y: auto;
}
.drawer-item {
  display: flex;
  align-items: center;
  padding: 13px 12px;
  margin-bottom: 4px;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.drawer-item:active { background: #f3f4f6; }
.drawer-item.active {
  background: linear-gradient(135deg, rgba(102,126,234,0.08), rgba(118,75,162,0.08));
}
.item-emoji {
  font-size: 20px;
  margin-right: 14px;
  width: 24px;
  text-align: center;
}
.item-label {
  flex: 1;
  font-size: 15px;
  color: #374151;
}
.drawer-item.active .item-label {
  color: #667eea;
  font-weight: 600;
}
.item-arrow { color: #d1d5db; font-size: 14px; }

/* 底部 */
.drawer-footer {
  padding: 16px;
  border-top: 1px solid #f3f4f6;
}
.logout-btn { width: 100%; }

/* 主内容 */
.main-area {
  flex: 1;
  overflow-y: auto;
  padding: 14px;
  box-sizing: border-box;
}
.pc-sidebar { display: none; }

@media (min-width: 769px) {
  .topbar { display: none; }
  .pc-sidebar {
    position: fixed;
    top: 0;
    left: 0;
    width: 240px;
    height: 100vh;
    background: #fff;
    border-right: 1px solid #e5e7eb;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    padding: 24px 16px;
    box-sizing: border-box;
    z-index: 100;
  }
  .pc-sidebar:hover { overflow-y: auto; }
  .pc-sidebar-header {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 4px 8px 16px;
    border-bottom: 1px solid #f3f4f6;
    margin-bottom: 16px;
  }
  .pc-sidebar .user-card { margin: 0 0 16px; }
  .pc-sidebar .drawer-menu { flex: 1; padding: 0; }
  .pc-sidebar .drawer-item .item-arrow { display: none; }
  .pc-sidebar .drawer-footer { padding: 12px 0 0; border-top: 1px solid #f3f4f6; }
  .main-area {
    margin-left: 240px;
    padding: 32px 48px;
    width: calc(100vw - 240px);
    box-sizing: border-box;
  }

  .user-layout > .el-drawer { display: none !important; }

  /* PC 端通用网格美化: 列表类卡片自动多列 */
  .main-area .key-list,
  .main-area .model-list,
  .main-area .order-list,
  .main-area .record-list,
  .main-area .bill-list,
  .main-area .note-list {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
    gap: 16px;
  }

  /* 余额卡 + 快捷入口左右排 */
  .main-area .quick-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 16px;
  }

  /* 套餐对比 PC 横向 */
  .main-area .plan-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 24px;
  }

  /* 价格网格 PC 横向 */
  .main-area .price-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 12px;
  }

  /* 表单 2 列 */
  .main-area .form-grid-2 {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 16px;
  }

  /* 余额统计横排 */
  .main-area .balance-stats {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 12px;
  }

  /* 卡片 hover 效果 */
  .main-area .data-card:hover,
  .main-area .model-card:hover,
  .main-area .plan-card:hover {
    transform: translateY(-2px);
    box-shadow: 0 8px 24px rgba(102,126,234,0.12);
    transition: all 0.2s;
  }
}

.lang-toggle {
  margin-right: 8px;
  background: rgba(102,126,234,0.08);
  color: #667eea;
  border: none;
  border-radius: 8px;
  padding: 6px 10px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}
.pc-lang-toggle {
  width: 100%;
  background: #f9fafb;
  color: #4b5563;
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  padding: 8px;
  font-size: 13px;
  cursor: pointer;
  margin-bottom: 8px;
}
.pc-lang-toggle:hover { border-color: #667eea; color: #667eea; }
</style>
