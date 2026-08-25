import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'

describe('未保存变更保护', () => {
  it('无改动时允许直接离开', async () => {
    const { confirmUnsavedNavigation } = await import('./useUnsavedChanges')
		const request = vi.fn(async () => false)

		expect(confirmUnsavedNavigation(ref(false), request)).toBe(true)
		expect(request).not.toHaveBeenCalled()
  })

  it('有改动时使用明确文案请求确认', async () => {
    const { confirmUnsavedNavigation } = await import('./useUnsavedChanges')
		const request = vi.fn(async () => false)

		expect(await confirmUnsavedNavigation(ref(true), request)).toBe(false)
		expect(request).toHaveBeenCalledWith('当前页面有未保存的修改，确定要离开吗？')
  })
})
