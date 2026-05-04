import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '@/router'

const api = axios.create({
  baseURL: '/v1',
  timeout: 15000,
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('user_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  (response) => {
    // 滑动续期：后端检测到 token 临近过期会在响应 header 返回新 token
    const newToken = response.headers?.['x-refresh-token']
    if (newToken) {
      localStorage.setItem('user_token', newToken)
    }
    return response.data
  },
  (error) => {
    const status = error.response?.status
    const msg = error.response?.data?.error || error.message

    if (status === 401) {
      // 登录接口密码错误也返回 401，不弹"过期"提示，让页面自己处理
      const isLoginRequest = error.config?.url?.includes('/auth/login')
      if (!isLoginRequest) {
        localStorage.removeItem('user_token')
        localStorage.removeItem('user_user')
        router.push('/login')
        ElMessage.error('登录已过期，请重新登录')
      }
    } else if (status === 403) {
      ElMessage.error('权限不足')
    } else if (status >= 500) {
      ElMessage.error('服务器错误: ' + msg)
    } else {
      ElMessage.error(msg)
    }
    return Promise.reject(error)
  }
)

export default api

// ---- Auth ----
export const authAPI = {
  login(email, password) {
    return api.post('/auth/login', { email, password })
  },
  register(email, password, username, emailCode) {
    return api.post('/auth/register', { email, password, username, email_code: emailCode })
  },
  me() {
    return api.get('/auth/me')
  },
}

// ---- Dashboard ----
export const dashboardAPI = {
  stats() {
    return api.get('/user/dashboard')
  },
  usage() {
    return api.get('/user/usage')
  },
}

// ---- API Keys ----
export const apiKeysAPI = {
  list() {
    return api.get('/api-keys')
  },
  create(data) {
    return api.post('/api-keys', data)
  },
  delete(id) {
    return api.delete(`/api-keys/${id}`)
  },
  update(id, data) {
    return api.patch(`/api-keys/${id}`, data)
  },
  toggle(id) {
    return api.patch(`/api-keys/${id}/toggle`)
  },
}

// ---- Recharge ----
export const rechargeAPI = {
  createOrder(amount, intent) {
    return api.post('/recharge/orders', { amount, payment_method: 'alipay', intent: intent || 'balance' })
  },
  listOrders() {
    return api.get('/recharge/orders')
  },
}

// ---- Billing ----
export const billingAPI = {
  list(params) {
    return api.get('/user/billing', { params })
  },
  exportCSV(params = {}) {
    return api.get('/user/billing/export', { params, responseType: 'blob' })
  },
}

// ---- User Models ----
export const userModelsAPI = {
  list() {
    return api.get('/user/models')
  },
}
