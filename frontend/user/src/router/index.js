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
    path: '/',
    component: () => import('@/views/Layout.vue'),
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', name: 'Dashboard', component: () => import('@/views/Dashboard.vue') },
      { path: 'api-keys', name: 'APIKeys', component: () => import('@/views/APIKeys.vue') },
      { path: 'recharge', name: 'Recharge', component: () => import('@/views/Recharge.vue') },
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
  if (to.meta.noAuth) return next()
  const store = useAuthStore()
  if (!store.isLoggedIn) return next('/login')
  next()
})

export default router
