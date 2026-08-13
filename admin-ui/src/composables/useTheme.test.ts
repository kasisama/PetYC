import { beforeEach, describe, expect, it, vi } from 'vitest'

describe('useTheme', () => {
  beforeEach(() => {
    vi.resetModules()
    localStorage.clear()
    document.documentElement.removeAttribute('data-theme')
  })

  it('使用晨光、月影和异梦作为三套主题的展示名', async () => {
    const { THEMES } = await import('./useTheme')

    expect(THEMES).toEqual([
      { id: 'mita-day', label: '晨光' },
      { id: 'mita-night', label: '月影' },
      { id: 'mita-other', label: '异梦' },
    ])
  })
})
