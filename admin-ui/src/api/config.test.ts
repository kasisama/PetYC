import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  CONFIG_STATUS_CHANGED_EVENT,
  fetchConfigStatus,
  normalizeCommand,
  normalizeItem,
  normalizeShopItem,
  reloadConfigs,
} from './config'

afterEach(() => vi.unstubAllGlobals())

describe('fetchConfigStatus', () => {
  it('读取数据库与内存版本状态', async () => {
    const fetchMock = vi.fn(async () =>
      new Response(
        JSON.stringify({
          code: 0,
          msg: 'success',
          data: {
            db_revision: 7,
            loaded_revision: 6,
            pending_reload: true,
            saved_at: '2026-08-09T05:00:00Z',
            loaded_at: '2026-08-09T04:50:00Z',
          },
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    const status = await fetchConfigStatus()

    expect(status.pending_reload).toBe(true)
    expect(status.db_revision).toBe(7)
    expect(fetchMock).toHaveBeenCalledWith('/api/admin/config/status', expect.any(Object))
  })

  it('热重载成功后广播配置状态变化', async () => {
    const fetchMock = vi.fn(async () =>
      new Response(JSON.stringify({ code: 0, msg: '重载成功', data: null }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)
    const listener = vi.fn()
    window.addEventListener(CONFIG_STATUS_CHANGED_EVENT, listener)

    await reloadConfigs()

    expect(listener).toHaveBeenCalledOnce()
    window.removeEventListener(CONFIG_STATUS_CHANGED_EVENT, listener)
  })
})

describe('内容工作台数据兼容', () => {
  it('兼容命令新字段的蛇形和大驼峰命名', () => {
    expect(
      normalizeCommand({
        func_name: 'pet_status',
        command: '状态',
        display_name: '查看状态',
        category: '宠物管理',
        description: '查看当前宠物状态',
        enabled: false,
        sort_order: 12,
      }),
    ).toEqual({
      FuncName: 'pet_status',
      Command: '状态',
      DisplayName: '查看状态',
      Category: '宠物管理',
      Description: '查看当前宠物状态',
      Enabled: false,
      SortOrder: 12,
    })

    expect(
      normalizeCommand({
        FuncName: 'daily',
        Command: '签到',
        DisplayName: '每日签到',
        Category: '日常玩法',
        Description: '领取每日奖励',
        Enabled: true,
        SortOrder: 3,
      }),
    ).toMatchObject({ DisplayName: '每日签到', Enabled: true, SortOrder: 3 })
  })

  it('补齐物品状态和商店目标库存', () => {
    expect(normalizeItem({ name: '绷带', status: 'limited' })).toMatchObject({
      Name: '绷带',
      Status: 'limited',
    })
    expect(normalizeItem({ Name: '木材' })).toMatchObject({ Status: 'active' })
    expect(normalizeShopItem({ id: 7, name: '绷带', restock_target: 50 })).toMatchObject({
      ID: 7,
      RestockTarget: 50,
    })
  })
})

describe('内容工作台专用接口', () => {
  it('活动和奖励通过一个 PUT 请求保存', async () => {
    const fetchMock = vi.fn(async () =>
      new Response(JSON.stringify({ code: 0, msg: 'success', data: {} }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)
    const config = (await import('./config')) as unknown as Record<string, (...args: any[]) => Promise<unknown>>

    expect(typeof config.saveEventBundle).toBe('function')
    await config.saveEventBundle(
      'forest-week',
      { key: 'forest-week', name: '森林调查' },
      [{ milestone: 100, item_name: '木材', quantity: 2 }],
    )

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/content/events/forest-week',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({
          event: { key: 'forest-week', name: '森林调查' },
          rewards: [{ milestone: 100, item_name: '木材', quantity: 2 }],
        }),
      }),
    )
  })

  it('批量物品和商店操作各自只发送一个请求', async () => {
    const fetchMock = vi.fn(async () =>
      new Response(JSON.stringify({ code: 0, msg: 'success', data: { updated: 2, items: [] } }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)
    const config = (await import('./config')) as unknown as Record<string, (...args: any[]) => Promise<unknown>>

    expect(typeof config.bulkItems).toBe('function')
    expect(typeof config.bulkShopItems).toBe('function')
    await config.bulkItems(['木材', '绷带'], 'set_status', 'hidden')
    await config.bulkShopItems([3, 7], 'set_target', 80)

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      '/api/admin/content/items/bulk',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ names: ['木材', '绷带'], action: 'set_status', status: 'hidden' }),
      }),
    )
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      '/api/admin/content/shop-items/bulk',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ ids: [3, 7], action: 'set_target', value: 80 }),
      }),
    )
  })

  it('读取和保存中文游戏参数元数据', async () => {
    const response = [
      {
        key: 'Core.CheckinLike',
        label: '每日陪伴点赞',
        group: '通知能力',
        type: 'boolean',
        description: '每日陪伴后请求点赞',
        value: true,
      },
    ]
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ code: 0, msg: 'success', data: response }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ code: 0, msg: 'success', data: response }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    vi.stubGlobal('fetch', fetchMock)
    const config = (await import('./config')) as unknown as Record<string, (...args: any[]) => Promise<unknown>>

    expect(typeof config.fetchGameSettings).toBe('function')
    expect(typeof config.saveGameSettings).toBe('function')
    await expect(config.fetchGameSettings()).resolves.toEqual(response)
    await config.saveGameSettings([{ key: 'Core.CheckinLike', value: false }])

    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      '/api/admin/settings/game',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify([{ key: 'Core.CheckinLike', value: false }]),
      }),
    )
  })
})
