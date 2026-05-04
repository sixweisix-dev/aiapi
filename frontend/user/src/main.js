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


// 移动端调试: 全局错误显示在页面上
window.addEventListener('error', (e) => {
  const div = document.createElement('div')
  div.style.cssText = 'position:fixed;top:0;left:0;right:0;background:red;color:white;padding:12px;z-index:99999;font-size:12px;font-family:monospace;white-space:pre-wrap;word-break:break-all;'
  div.textContent = 'ERROR: ' + e.message + ' @ ' + e.filename + ':' + e.lineno + ':' + e.colno
  document.body.appendChild(div)
})
window.addEventListener('unhandledrejection', (e) => {
  const div = document.createElement('div')
  div.style.cssText = 'position:fixed;top:50px;left:0;right:0;background:orange;color:black;padding:12px;z-index:99999;font-size:12px;font-family:monospace;white-space:pre-wrap;word-break:break-all;'
  div.textContent = 'PROMISE: ' + (e.reason?.message || e.reason)
  document.body.appendChild(div)
})

const app = createApp(App)

for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

app.use(createPinia())
app.use(router)
app.use(ElementPlus, { locale: zhCn })
app.use(i18n).mount('#app')
