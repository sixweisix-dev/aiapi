import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '@/router'

const api = axios.create({
  baseURL: '/v1',
  timeout: 15000,
})

// Request interceptor: attach JWT token
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('admin_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Response interceptor: handle errors globally
api.interceptors.response.use(
  (response) => {
    // 滑动续期
    const newToken = response.headers?.['x-refresh-token']
    if (newToken) {
      localStorage.setItem('admin_token', newToken)
    }
    return response.data
  },
  (error) => {
    const status = error.response?.status
    const msg = error.response?.data?.error || error.message

    if (status === 401) {
      localStorage.removeItem('admin_token')
      localStorage.removeItem('admin_user')
      router.push('/login')
      ElMessage.error('登录已过期，请重新登录')
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
  me() {
    return api.get('/auth/me')
  },
}

// ---- Dashboard ----
export const dashboardAPI = {
  stats() {
    return api.get('/admin/dashboard')
  },
}

// ---- Users ----
export const usersAPI = {
  list(params) {
    return api.get('/admin/users', { params })
  },
  get(id) {
    return api.get(`/admin/users/${id}`)
  },
  update(id, data) {
    return api.patch(`/admin/users/${id}`, data)
  },
}

// ---- Channels ----
export const channelsAPI = {
  list() {
    return api.get('/admin/channels')
  },
  create(data) {
    return api.post('/admin/channels', data)
  },
  update(id, data) {
    return api.put(`/admin/channels/${id}`, data)
  },
  delete(id) {
    return api.delete(`/admin/channels/${id}`)
  },
  test(id) {
    return api.post(`/admin/channels/${id}/test`)
  },
}

// ---- Models ----
export const modelsAPI = {
  list() {
    return api.get('/admin/models')
  },
  create(data) {
    return api.post('/admin/models', data)
  },
  update(id, data) {
    return api.put(`/admin/models/${id}`, data)
  },
  delete(id) {
    return api.delete(`/admin/models/${id}`)
  },
}

// ---- Logs ----
export const logsAPI = {
  list(params) {
    return api.get('/admin/logs', { params })
  },
}

// ---- Audit Logs ----
export const auditAPI = {
  list(params) {
    return api.get('/admin/audit-logs', { params })
  },
}

// ---- Recharge Orders ----
export const rechargeAPI = {
  list(params) {
    return api.get('/admin/recharge-orders', { params })
  },
}
