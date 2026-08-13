import { beforeEach, describe, expect, it, vi } from 'vitest'

describe('useToast', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.resetModules()
  })

  it('推送消息并在超时后自动移除', async () => {
    const { useToast } = await import('./useToast')
    const toast = useToast()

    toast.success('保存成功', 1200)
    expect(toast.toasts.value).toHaveLength(1)
    expect(toast.toasts.value[0]?.message).toBe('保存成功')

    vi.advanceTimersByTime(1200)
    expect(toast.toasts.value).toHaveLength(0)
  })

  it('允许手动关闭消息', async () => {
    const { useToast } = await import('./useToast')
    const toast = useToast()

    const id = toast.error('请求失败')
    toast.dismiss(id)

    expect(toast.toasts.value).toHaveLength(0)
  })
})
