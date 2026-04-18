import { createI18n } from 'vue-i18n'
import en from './en.json'
import zhCN from './zh-CN.json'

function getDefaultLocale() {
  const saved = localStorage.getItem('locale')
  if (saved) return saved
  const browser = navigator.language
  if (browser.startsWith('zh')) return 'zh-CN'
  return 'en'
}

const i18n = createI18n({
  legacy: false,
  locale: getDefaultLocale(),
  fallbackLocale: 'en',
  messages: { en, 'zh-CN': zhCN },
})

export default i18n
