import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import ContentView from './ContentView.vue'

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace: vi.fn() }),
  onBeforeRouteLeave: vi.fn(),
}))

function json(data: unknown) {
  return new Response(JSON.stringify({ code: 0, msg: 'success', data }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

function installApiMock() {
  const schemas: Record<string, unknown[]> = {
    live_events: [
      {
        id: 1,
        key: 'forest-week',
        name: '森林调查周',
        region: '森林',
        story_choices: '["记录线索","继续调查","呼叫支援"]',
        starts_at: '2026-08-10T08:00:00Z',
        ends_at: '2026-08-20T08:00:00Z',
        active: true,
      },
    ],
    reward_tracks: [
      { id: 1, event_key: 'forest-week', milestone: 100, item_name: '木材', quantity: 2, description: '基础补给' },
      { id: 2, event_key: 'forest-week', milestone: 100, item_name: '绷带', quantity: 1, description: '基础补给' },
    ],
    pet_species: [{ Name: '团子', Description: '活泼的初始宠物' }],
    items: [
      { Name: '木材', Status: 'active', Type: '材料', Description: '森林采集材料' },
      { Name: '绷带', Status: 'limited', Type: '消耗品', Description: '恢复状态' },
    ],
    shop_items: [
      { ID: 3, ShopType: 'shop_normal', Name: '绷带', Stock: 4, RestockTarget: 50, Price: 12, Description: '应急用品' },
    ],
    images: [{ Name: '森林背景', Path: '上传/forest.webp' }],
    commands: [
      {
        func_name: 'pet_status',
        command: '状态',
        display_name: '查看宠物状态',
        category: '宠物管理',
        description: '查看当前宠物的详细状态',
        enabled: true,
        sort_order: 1,
      },
    ],
    menus: [{ Name: 'main', Reply: '欢迎回来，请选择要进行的操作。' }],
  }
  const gameSettings = [
    {
      key: 'Core.CheckinLike',
      label: '每日陪伴点赞',
      group: '通知能力',
      type: 'boolean',
      description: '每日陪伴完成后是否请求平台点赞能力。',
      value: true,
    },
    {
      key: 'Core.InitialCoin',
      label: '初始货币',
      group: '基础设置',
      type: 'number',
      unit: '枚',
      description: '新玩家建立存档时获得的基础货币。',
      value: 120,
    },
    {
      key: 'Core.InitialPets',
      label: '初始可领养宠物',
      group: '基础设置',
      type: 'list',
      description: '新玩家可以选择的初始宠物名称。',
      value: ['团子', '小白'],
    },
  ]

  return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input)
    if (path === '/api/admin/config/status') {
      return json({ db_revision: 2, loaded_revision: 2, pending_reload: false, saved_at: null, loaded_at: null })
    }
    if (path === '/api/admin/settings/game') return json(gameSettings)
    if (path.startsWith('/api/admin/config/')) {
      return json(schemas[path.replace('/api/admin/config/', '')] ?? [])
    }
    if (path.startsWith('/api/admin/content/events/') && init?.method === 'PUT') {
      const body = JSON.parse(String(init.body)) as { event: Record<string, unknown>; rewards: unknown[] }
      return json({ event: { ...body.event, id: 9 }, rewards: body.rewards })
    }
    if (path === '/api/admin/content/items/bulk') return json({ updated: 2, items: schemas.items })
    if (path === '/api/admin/content/shop-items/bulk') return json({ updated: 1, items: schemas.shop_items })
    return json({})
  })
}

async function mountView() {
  vi.stubGlobal('fetch', installApiMock())
  const wrapper = mount(ContentView, { attachTo: document.body })
  await flushPromises()
  return wrapper
}

async function clickByText(wrapper: VueWrapper, text: string) {
  const button = wrapper.findAll('button').find((item) => item.text().trim() === text)
  expect(button, `未找到按钮：${text}`).toBeTruthy()
  await button!.trigger('click')
  await flushPromises()
}

async function clickDocumentButton(text: string) {
  const button = [...document.body.querySelectorAll('button')].find((item) => item.textContent?.trim() === text)
  expect(button, `未找到按钮：${text}`).toBeTruthy()
  button!.click()
  await flushPromises()
}

afterEach(() => {
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
})

describe('ContentView 内容工作台', () => {
  it('主导航只保留四个业务域', async () => {
    const wrapper = await mountView()
    const tabs = wrapper.findAll('.page-tabs > button').map((button) => button.text().trim())

    expect(tabs).toEqual(['活动运营', '宠物与物品', '文本与命令', '游戏参数'])
    expect(tabs).not.toContain('奖励配置')
    expect(tabs).not.toContain('历史配置')

    wrapper.unmount()
  })

  it('活动抽屉集中编辑说明、示例、故事选项和多物品奖励', async () => {
    const wrapper = await mountView()
    await clickByText(wrapper, '编辑活动')
    const drawerText = document.body.textContent ?? ''

    expect(drawerText).toContain('使用说明')
    expect(drawerText).toContain('填充测试示例')
    expect(drawerText).toContain('故事选项')
    expect(drawerText).toContain('里程碑 100')
    expect(drawerText).toContain('木材 × 2')
    expect(drawerText).toContain('绷带 × 1')
    expect(drawerText).toContain('累计奖励预览')

    wrapper.unmount()
  })

  it('关闭有修改的活动抽屉前要求确认', async () => {
    const wrapper = await mountView()
    const confirm = vi.fn(() => false)
    vi.stubGlobal('confirm', confirm)

    await clickByText(wrapper, '新建活动')
    await clickDocumentButton('填充测试示例')
    await clickDocumentButton('取消')

    expect(confirm).toHaveBeenCalledWith('当前编辑内容尚未应用，确定关闭吗？')
    expect(document.body.textContent).toContain('一次保存活动与奖励')
    wrapper.unmount()
  })

  it('新活动一次保存奖励并在后续编辑沿用服务端编号', async () => {
    const wrapper = await mountView()
    await clickByText(wrapper, '新建活动')
    await clickDocumentButton('填充测试示例')
    await clickDocumentButton('一次保存活动与奖励')

    const createdCard = wrapper.findAll('.event-card').find((card) => card.text().includes('森林生态联合调查'))
    expect(createdCard).toBeTruthy()
    await createdCard!.get('button').trigger('click')
    await clickDocumentButton('一次保存活动与奖励')

    const eventRequests = vi.mocked(fetch).mock.calls.filter(([path, init]) =>
      String(path).startsWith('/api/admin/content/events/') && init?.method === 'PUT',
    )
    expect(eventRequests).toHaveLength(2)
    const firstBody = JSON.parse(String(eventRequests[0]?.[1]?.body)) as { rewards: unknown[] }
    const secondBody = JSON.parse(String(eventRequests[1]?.[1]?.body)) as { event: { id?: number } }
    expect(firstBody.rewards).toHaveLength(3)
    expect(secondBody.event.id).toBe(9)

    wrapper.unmount()
  })

  it('命令按中文分类展示且列表隐藏技术键', async () => {
    const wrapper = await mountView()
    await clickByText(wrapper, '文本与命令')

    expect(wrapper.text()).toContain('宠物管理')
    expect(wrapper.text()).toContain('查看宠物状态')
    expect(wrapper.text()).toContain('查看当前宠物的详细状态')
    expect(wrapper.text()).not.toContain('pet_status')

    wrapper.unmount()
  })

  it('游戏参数使用中文控件而不暴露布尔值和技术键', async () => {
    const wrapper = await mountView()
    await clickByText(wrapper, '游戏参数')

    expect(wrapper.text()).toContain('基础设置')
    expect(wrapper.text()).toContain('每日陪伴点赞')
    expect(wrapper.find('input[type="checkbox"]').exists()).toBe(true)
    expect(wrapper.find('input[type="number"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('团子')
    expect(wrapper.text()).not.toContain('Core.CheckinLike')
    expect(wrapper.text()).not.toMatch(/\btrue\b|\bfalse\b/)

    wrapper.unmount()
  })

  it('物品与商店支持多选和列表内联库存字段', async () => {
    const wrapper = await mountView()
    await clickByText(wrapper, '宠物与物品')
    await clickByText(wrapper, '物品')

    expect(wrapper.findAll('input[type="checkbox"]').length).toBeGreaterThanOrEqual(3)
    expect(wrapper.text()).toContain('批量设置状态')

    await clickByText(wrapper, '商店')
    expect(wrapper.find('input[aria-label="绷带价格"]').exists()).toBe(true)
    expect(wrapper.find('input[aria-label="绷带库存"]').exists()).toBe(true)
    expect(wrapper.find('input[aria-label="绷带目标库存"]').exists()).toBe(true)

    wrapper.unmount()
  })

  it('图片资产在当前工作台直接预览、上传和编辑', async () => {
    const wrapper = await mountView()
    await clickByText(wrapper, '宠物与物品')
    await clickByText(wrapper, '图片资产')

    expect(wrapper.text()).toContain('森林背景')
    expect(wrapper.text()).toContain('上传图片')
    expect(wrapper.find('img[alt="森林背景"]').attributes('src')).toContain('/images/')
    wrapper.unmount()
  })
})
