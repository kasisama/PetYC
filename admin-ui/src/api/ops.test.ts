import { afterEach, describe, expect, it, vi } from 'vitest'
import { bulkUpdateGroupState } from './ops'

afterEach(() => vi.unstubAllGlobals())

describe('bulkUpdateGroupState', () => {
  it('一次请求返回更新数量与标准化群组', async () => {
    const fetchMock = vi.fn(async () =>
      new Response(
        JSON.stringify({
          code: 0,
          msg: 'success',
          data: {
            updated: 2,
            groups: [{ GroupID: 1001, GroupName: '测试群', IsActive: false }],
          },
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    const result = await bulkUpdateGroupState(null, false)

    expect(result.updated).toBe(2)
    expect(result.groups[0]).toEqual({ group_id: 1001, platform: 'onebot', space_id: '1001', group_name: '测试群', is_active: false })
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })
})
