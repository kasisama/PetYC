import { beforeEach, describe, expect, it, vi } from 'vitest'

describe('useShellLayout', () => {
  beforeEach(() => {
    vi.resetModules()
  })

  it('记住侧栏折叠状态', async () => {
    const { useShellLayout } = await import('./useShellLayout')
    const shell = useShellLayout()

    shell.toggleCollapsed()

    expect(shell.collapsed.value).toBe(true)
    expect(localStorage.getItem('adminSidebarCollapsed')).toBe('true')
  })

  it('关闭移动端抽屉不会改变桌面折叠状态', async () => {
    const { useShellLayout } = await import('./useShellLayout')
    const shell = useShellLayout()

    shell.openDrawer()
    shell.closeDrawer()

    expect(shell.drawerOpen.value).toBe(false)
    expect(shell.collapsed.value).toBe(false)
  })
})
