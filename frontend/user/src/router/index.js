import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes = [
  {
    path: '/terms',
    name: 'Terms',
    component: () => import('@/views/Terms.vue'),
    meta: { noAuth: true },
  },
  {
    path: '/privacy',
    name: 'Privacy',
    component: () => import('@/views/Privacy.vue'),
    meta: { noAuth: true },
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { noAuth: true },
  },
  {
    path: '/reset-password',
    name: 'ResetPassword',
    component: () => import('@/views/ResetPassword.vue'),
    meta: { noAuth: true },
  },
  {
    path: '/forgot-password',
    name: 'ForgotPassword',
    component: () => import('@/views/ForgotPassword.vue'),
    meta: { noAuth: true },
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/views/Register.vue'),
    meta: { noAuth: true },
  },
  {
    path: '/landing',
    name: 'Landing',
    component: () => import('@/views/Landing.vue'),
    meta: { noAuth: true },
  },
  {
    path: '/pricing',
    name: 'Pricing',
    component: () => import('@/views/Landing.vue'),
    meta: { noAuth: true },
  },
  {
    path: '/',
    component: () => import('@/views/Layout.vue'),
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', name: 'Dashboard', component: () => import('@/views/Dashboard.vue') },
      { path: 'api-keys', name: 'APIKeys', component: () => import('@/views/APIKeys.vue') },
      { path: 'recharge', name: 'Recharge', component: () => import('@/views/Recharge.vue') },
      { path: 'membership', redirect: '/recharge' },
      { path: 'billing', name: 'Billing', component: () => import('@/views/Billing.vue') },
      { path: 'models', name: 'Models', component: () => import('@/views/Models.vue') },
      { path: 'api-docs', name: 'ApiDocs', component: () => import('@/views/ApiDocs.vue') },
      { path: 'playground', name: 'Playground', component: () => import('@/views/Playground.vue') },
      { path: 'change-password', name: 'ChangePassword', component: () => import('@/views/ChangePassword.vue') },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to, from, next) => {
  const store = useAuthStore()

  // 公开页：未登录可访问
  if (to.meta.noAuth) {
    // 已登录访问 /landing 或 /pricing 也可以正常看，这里不强跳
    return next()
  }

  // 受保护页：未登录跳到 Landing（首次访客看公开介绍页）
  if (!store.isLoggedIn) {
    // 如果是访问 / 或 /dashboard，去 Landing；其他敏感页跳 Login
    if (to.path === '/' || to.path === '/dashboard') {
      return next('/landing')
    }
    return next('/login')
  }
  next()
})

export default router
