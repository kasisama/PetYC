import { ref, watch } from 'vue'

// 三套配色与 styles/theme.css 中的 [data-theme] 选择器一一对应。
export const THEMES = [
  { id: 'monochrome', label: '墨白' },
  { id: 'violet', label: '雾紫' },
  { id: 'vermilion', label: '朱砂' },
] as const

export type ThemeId = (typeof THEMES)[number]['id']

const STORAGE_KEY = 'adminTheme'
const DEFAULT_THEME: ThemeId = 'monochrome'

function isThemeId(value: string | null): value is ThemeId {
  return THEMES.some((theme) => theme.id === value)
}

function readStoredTheme(): ThemeId {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    return isThemeId(stored) ? stored : DEFAULT_THEME
  } catch {
    // 隐私模式下 localStorage 可能不可用，退回默认主题而不是让页面崩掉。
    return DEFAULT_THEME
  }
}

// 模块级单例：所有组件共享同一份主题状态，切换后整站同步。
const theme = ref<ThemeId>(readStoredTheme())

watch(
  theme,
  (value) => {
    document.documentElement.setAttribute('data-theme', value)
    try {
      localStorage.setItem(STORAGE_KEY, value)
    } catch {
      // 写入失败只影响下次进入时的记忆，不影响本次切换。
    }
  },
  { immediate: true },
)

export function useTheme() {
  return {
    theme,
    themes: THEMES,
    setTheme(value: ThemeId) {
      theme.value = value
    },
  }
}
