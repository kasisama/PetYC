import { ref, watch } from 'vue'

export const DENSITIES = [
  { id: 'comfortable', label: '舒适' },
  { id: 'compact', label: '紧凑' },
] as const

export type DensityId = (typeof DENSITIES)[number]['id']

const STORAGE_KEY = 'adminDensity'

function readStoredDensity(): DensityId {
  try {
    return localStorage.getItem(STORAGE_KEY) === 'compact' ? 'compact' : 'comfortable'
  } catch {
    return 'comfortable'
  }
}

const density = ref<DensityId>(readStoredDensity())

watch(
  density,
  (value) => {
    document.documentElement.dataset.density = value
    try {
      localStorage.setItem(STORAGE_KEY, value)
    } catch {
      // 存储不可用时仍保留当前会话内的显示偏好。
    }
  },
  { immediate: true, flush: 'sync' },
)

export function useDensity() {
  return {
    density,
    densities: DENSITIES,
    setDensity(value: DensityId) {
      density.value = value
    },
    toggleDensity() {
      density.value = density.value === 'comfortable' ? 'compact' : 'comfortable'
    },
  }
}
