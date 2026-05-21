import { createI18n } from 'vue-i18n'
import zh from '@/locales/zh.json'
import en from '@/locales/en.json'

const stored = localStorage.getItem('user_lang')

// 初始 locale: 优先 localStorage; 没有则用浏览器语言粗判 (避免渲染时空白)
const browserLang = (navigator.language || 'zh').toLowerCase().startsWith('en') ? 'en' : 'zh'
const initial = stored || browserLang

export const i18n = createI18n({
  legacy: false,
  warnHtmlMessage: false,
  globalInjection: true,
  locale: initial,
  fallbackLocale: 'zh',
  messages: { zh, en },
})

// 首次访问 (没存语言时): 异步调后端按 CF-IPCountry 精确判断, 然后写入 localStorage
if (!stored) {
  fetch('/v1/locale-detect', { credentials: 'omit' })
    .then(r => r.json())
    .then(d => {
      if (d && d.locale && (d.locale === 'zh' || d.locale === 'en')) {
        i18n.global.locale.value = d.locale
        localStorage.setItem('user_lang', d.locale)
      }
    })
    .catch(() => {}) // 失败保持 fallback, 不阻塞
}

export function setLocale(lang) {
  i18n.global.locale.value = lang
  localStorage.setItem('user_lang', lang)
}

export function currentLocale() {
  return i18n.global.locale.value
}
