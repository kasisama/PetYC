import { beforeEach, describe, expect, it, vi } from 'vitest'

describe('useDensity', () => {
  beforeEach(() => {
    vi.resetModules()
  })

  it('默认使用舒适密度并同步到根节点', async () => {
    const { useDensity } = await import('./useDensity')
    const density = useDensity()

    expect(density.density.value).toBe('comfortable')
    expect(document.documentElement.dataset.density).toBe('comfortable')
  })

  it('切换紧凑模式后持久化偏好', async () => {
    const { useDensity } = await import('./useDensity')
    const density = useDensity()

    density.setDensity('compact')

    expect(document.documentElement.dataset.density).toBe('compact')
    expect(localStorage.getItem('adminDensity')).toBe('compact')
  })
})
