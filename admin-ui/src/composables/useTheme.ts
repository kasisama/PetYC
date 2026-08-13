import { ref, watch } from 'vue'

// 三套米塔主题，与 styles/theme.css 中 [data-theme] 一一对应。
export const THEMES = [
  { id: 'mita-day', label: '晨光' },
  { id: 'mita-night', label: '月影' },
  { id: 'mita-other', label: '异梦' },
] as const

export type ThemeId = (typeof THEMES)[number]['id']

const STORAGE_KEY = 'adminTheme'
const DEFAULT_THEME: ThemeId = 'mita-day'

/** 旧主题 id → 新米塔主题，保证升级后不丢偏好。 */
const LEGACY_THEME_MAP: Record<string, ThemeId> = {
  monochrome: 'mita-night',
  violet: 'mita-night',
  vermilion: 'mita-other',
}

function isThemeId(value: string | null): value is ThemeId {
  return THEMES.some((theme) => theme.id === value)
}

function readStoredTheme(): ThemeId {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (isThemeId(stored)) {
      return stored
    }
    if (stored && stored in LEGACY_THEME_MAP) {
      return LEGACY_THEME_MAP[stored]
    }
    return DEFAULT_THEME
  } catch {
    // 隐私模式下 localStorage 可能不可用，退回默认主题。
    return DEFAULT_THEME
  }
}

// 模块级单例：所有组件共享同一份主题状态。
const theme = ref<ThemeId>(readStoredTheme())

watch(
  theme,
  (value) => {
    document.documentElement.setAttribute('data-theme', value)
    try {
      localStorage.setItem(STORAGE_KEY, value)
    } catch {
      // 写入失败只影响下次记忆，不影响本次切换。
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
