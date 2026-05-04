import { i18n } from '@/i18n'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import './global.css'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import App from './App.vue'
import router from './router'

const app = createApp(App)

for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

app.use(createPinia())
app.use(router)
app.use(ElementPlus, { locale: zhCn })
app.use(i18n).mount('#app')

// Silence harmless ResizeObserver loop warnings (Element Plus / ECharts)
const _roeMsg = 'ResizeObserver loop'
window.addEventListener('error', (e) => {
  if (e.message && e.message.includes(_roeMsg)) {
    e.stopImmediatePropagation()
    e.preventDefault()
  }
})
window.addEventListener('unhandledrejection', (e) => {
  if (e.reason && String(e.reason).includes(_roeMsg)) {
    e.preventDefault()
  }
})
