import { defineStore } from 'pinia'
import { authAPI } from '@/utils/api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('user_token') || '',
    user: JSON.parse(localStorage.getItem('user_user') || 'null'),
  }),
  getters: {
    isLoggedIn: (state) => !!state.token,
  },
  actions: {
    async login(email, password) {
      const res = await authAPI.login(email, password)
      this.token = res.token
      this.user = res.user
      localStorage.setItem('user_token', res.token)
      localStorage.setItem('user_user', JSON.stringify(res.user))
      return res
    },
    async register(email, password, username, emailCode) {
      const res = await authAPI.register(email, password, username, emailCode)
      this.token = res.token
      this.user = res.user
      localStorage.setItem('user_token', res.token)
      localStorage.setItem('user_user', JSON.stringify(res.user))
      return res
    },
    async fetchMe() {
      try {
        const res = await authAPI.me()
        this.user = res
        localStorage.setItem('user_user', JSON.stringify(res))
      } catch {
        this.logout()
      }
    },
    logout() {
      this.token = ''
      this.user = null
      localStorage.removeItem('user_token')
      localStorage.removeItem('user_user')
    },
  },
})
