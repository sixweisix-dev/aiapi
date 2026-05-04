import { createI18n } from 'vue-i18n'
import zh from '@/locales/zh.json'
import en from '@/locales/en.json'

const stored = localStorage.getItem('user_lang') || 'zh'

export const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: stored,
  fallbackLocale: 'zh',
  messages: { zh, en },
})

export function setLocale(lang) {
  i18n.global.locale.value = lang
  localStorage.setItem('user_lang', lang)
}

export function currentLocale() {
  return i18n.global.locale.value
}
