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
      { id: 1, event_key: 'forest-week', milestone: 100, reward_type: 'item', reward_key: 'wood', reward_name: '木材', quantity: 2, description: '基础补给' },
      { id: 2, event_key: 'forest-week', milestone: 100, reward_type: 'item', reward_key: 'bandage', reward_name: '绷带', quantity: 1, description: '基础补给' },
    ],
	pet_species: [
	  { Key: 'tuanzhi-base', FamilyKey: 'tuanzhi', Stage: 'base', Name: '团子', Image: '宠物图片\\团子.png', Description: '活泼的初始宠物' },
	  { Key: 'tuanzhi-evolved', FamilyKey: 'tuanzhi', Stage: 'evolved', PreviousFormKey: 'tuanzhi-base', Name: '团子进化', Description: '成长后的形态' },
	  { Key: 'tuanzhi-awaken-moon', FamilyKey: 'tuanzhi', Stage: 'awakened', PreviousFormKey: 'tuanzhi-evolved', Name: '团子觉醒月', Description: '月光路线' },
	  { Key: 'tuanzhi-awaken-sun', FamilyKey: 'tuanzhi', Stage: 'awakened', PreviousFormKey: 'tuanzhi-evolved', Name: '团子觉醒日', Description: '日光路线' },
	],
	pet_evolution_rules: [
	  { Key: 'tuanzhi-standard', FromFormKey: 'tuanzhi-base', ToFormKey: 'tuanzhi-evolved', RequiredGrowth: 10, RequiredAffection: 5, BranchLabel: '标准进化', Enabled: true, SortOrder: 10 },
	  { Key: 'tuanzhi-moon', FromFormKey: 'tuanzhi-evolved', ToFormKey: 'tuanzhi-awaken-moon', RequiredGrowth: 20, RequiredAffection: 10, BranchLabel: '月光路线', Enabled: true, SortOrder: 30 },
	  { Key: 'tuanzhi-sun', FromFormKey: 'tuanzhi-evolved', ToFormKey: 'tuanzhi-awaken-sun', RequiredGrowth: 20, RequiredAffection: 10, BranchLabel: '日光路线', Enabled: true, SortOrder: 20 },
	],
	items: [
		{ Name: '木材', Status: 'active', Type: '材料', Image: '物品图片\\木材.png', Description: '森林采集材料' },
		{ Name: '绷带', Status: 'limited', Type: '消耗品', Image: '物品图片\\绷带.png', Description: '恢复状态' },
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
    menus: [{ Name: 'main', Reply: '欢迎回来，请选择要进行的操作。', Markdown: '# 欢迎回来\n\n**请选择操作**', Image: '上传/main.webp' }],
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
	if (path === '/api/admin/upload') return json({ message: '图片上传成功', path: '上传/new-pet.png', url: '/images/上传/new-pet.png' })
    if (path === '/api/admin/adventure/catalog') return json({ revision: 1, catalog: { maps: [], zones: [] } })
    if (path === '/api/admin/content/messages') return json([])
    if (path.startsWith('/api/admin/config/') && path.endsWith('/meta')) {
      return json({ schema: path.split('/').at(-2), consumers: ['统一领域服务'], effective_revision: 2, db_revision: 2, pending_reload: false })
    }
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
  const button = [...document.body.querySelectorAll('button')].find((item) => item.textContent?.includes(text))
  expect(button, `未找到按钮：${text}`).toBeTruthy()
  button!.click()
  await flushPromises()
}

async function clickDrawerPrimary(text: string) {
  const button = document.body.querySelector('.drawer-actions .btn-primary') as HTMLButtonElement | null
  expect(button?.textContent, `未找到抽屉主按钮：${text}`).toContain(text)
  button!.click()
  await flushPromises()
}

async function clickSaveCurrentConfig() {
  const button = [...document.body.querySelectorAll('button')].find((item) => item.textContent?.includes('保存当前配置') && !(item as HTMLButtonElement).disabled)
  expect(button, `未找到保存当前配置，现有按钮：${[...document.body.querySelectorAll('button')].map((item) => item.textContent?.replace(/\s+/g, ' ').trim()).join(' | ')}`).toBeTruthy()
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

  it('活动抽屉按步骤解释时间、进度来源、故事选项和多物品奖励', async () => {
    const wrapper = await mountView()
    await clickByText(wrapper, '编辑活动')
    const drawerText = document.body.textContent ?? ''

    expect(drawerText).toContain('基本资料')
    expect(drawerText).toContain('生效时间')
    expect(drawerText).toContain('活动只统计所选区域的远征进度')
    expect(drawerText).toContain('填充测试示例')
    expect(drawerText).toContain('故事选项')
    expect(drawerText).toContain('里程碑 100')
    expect(drawerText).toContain('木材 × 2')
    expect(drawerText).toContain('绷带 × 1')
    expect(drawerText).toContain('累计奖励预览')

    wrapper.unmount()
  })

  it('关闭有修改的活动抽屉前使用应用内确认弹窗', async () => {
    const wrapper = await mountView()

    await clickByText(wrapper, '新建活动')
    await clickDocumentButton('填充测试示例')
    await clickDocumentButton('取消')

    expect(document.body.textContent).toContain('放弃当前编辑？')
    expect(document.body.textContent).toContain('尚未应用到列表的修改将会丢失。')
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
    await flushPromises()
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

  it('菜单场景可预览、上传和清除配图', async () => {
    const wrapper = await mountView()
    const fetchMock = globalThis.fetch as ReturnType<typeof vi.fn>
    await clickByText(wrapper, '文本与命令')
    await clickByText(wrapper, '菜单场景')

    expect(wrapper.get('.menu-card img[alt="主菜单图片"]').attributes('src')).toBe('/images/上传/main.webp')
    expect(wrapper.text()).toContain('已配置 Markdown')
    await clickByText(wrapper, '编辑')
    expect(document.body.textContent).toContain('配图仅可在上方直接上传')
    expect(document.body.textContent).toContain('Markdown 回复（可选）')
    expect(document.body.textContent).toContain('文本、Markdown 和图片立即对玩家生效')
    const dropzoneRoot = document.body.querySelector('[aria-label="图片上传与预览"]') as HTMLElement
    const qqPreview = document.body.querySelector('.qq-preview.large') as HTMLElement
    expect(dropzoneRoot.querySelector('img[alt="主菜单图片"]')?.getAttribute('src')).toBe('/images/上传/main.webp')
    expect(qqPreview.querySelector('img[alt="主菜单图片"]')?.getAttribute('src')).toBe('/images/上传/main.webp')
    expect(qqPreview.textContent).toContain('欢迎回来，请选择要进行的操作。')
    const textareas = [...document.body.querySelectorAll('textarea')] as HTMLTextAreaElement[]
    expect(textareas).toHaveLength(2)
    expect(textareas[1].value).toContain('# 欢迎回来')
    expect(document.body.querySelector('.markdown-source')?.textContent).toContain('**请选择操作**')

    const dropzone = document.body.querySelector('.image-dropzone') as HTMLElement
    const file = new File([new Uint8Array([137, 80, 78, 71])], 'new-menu.png', { type: 'image/png' })
    const drop = new Event('drop', { bubbles: true, cancelable: true })
    Object.defineProperty(drop, 'dataTransfer', { value: { files: [file] } })
    dropzone.dispatchEvent(drop)
    await flushPromises()

    expect(dropzoneRoot.querySelector('img[alt="主菜单图片"]')?.getAttribute('src')).toBe('/images/上传/new-pet.png')
    expect(qqPreview.querySelector('img[alt="主菜单图片"]')?.getAttribute('src')).toBe('/images/上传/new-pet.png')

    await clickDrawerPrimary('应用到列表')
    expect(wrapper.get('.menu-card img[alt="主菜单图片"]').attributes('src')).toBe('/images/上传/new-pet.png')
    await clickSaveCurrentConfig()
    const uploaded = fetchMock.mock.calls.find(([url, init]) => String(url).includes('/api/admin/config/menus') && (init as RequestInit)?.method === 'PUT')
    expect(JSON.parse(String((uploaded?.[1] as RequestInit).body))[0].Image).toBe('上传/new-pet.png')
    expect(JSON.parse(String((uploaded?.[1] as RequestInit).body))[0].Markdown).toContain('# 欢迎回来')

    await clickByText(wrapper, '编辑')
    await clickDocumentButton('移除当前图片')
    const clearedDropzone = document.body.querySelector('[aria-label="图片上传与预览"]') as HTMLElement
    const clearedPreview = document.body.querySelector('.qq-preview.large') as HTMLElement
    expect(clearedDropzone.querySelector('img')).toBeNull()
    expect(clearedDropzone.querySelector('[aria-label="主菜单暂无可用图片"]')).toBeTruthy()
    expect(clearedPreview.querySelector('img')).toBeNull()
    expect(clearedPreview.textContent).toContain('欢迎回来，请选择要进行的操作。')

    await clickDrawerPrimary('应用到列表')
    expect(wrapper.find('.menu-card img[alt="主菜单图片"]').exists()).toBe(false)
    await clickSaveCurrentConfig()
    const puts = fetchMock.mock.calls.filter(([url, init]) => String(url).includes('/api/admin/config/menus') && (init as RequestInit)?.method === 'PUT')
    expect(JSON.parse(String((puts.at(-1)?.[1] as RequestInit).body))[0].Image).toBe('')

    wrapper.unmount()
  })

  it('菜单场景缺少回复时阻止应用并直接提示', async () => {
    const wrapper = await mountView()
    const fetchMock = globalThis.fetch as ReturnType<typeof vi.fn>
    await clickByText(wrapper, '文本与命令')
    await clickByText(wrapper, '菜单场景')
    await clickByText(wrapper, '编辑')

    const reply = document.body.querySelector('textarea') as HTMLTextAreaElement
    reply.value = '   '
    reply.dispatchEvent(new Event('input', { bubbles: true }))
    await clickDrawerPrimary('应用到列表')

    expect(document.body.textContent).toContain('编辑菜单场景')
    const { useToast } = await import('../composables/useToast')
    expect(useToast().toasts.value.at(-1)?.message).toContain('缺少机器人回复')
    const menuPuts = fetchMock.mock.calls.filter(([url, init]) => String(url).includes('/api/admin/config/menus') && (init as RequestInit)?.method === 'PUT')
    expect(menuPuts).toHaveLength(0)

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
		expect(wrapper.text()).not.toContain('批量设置状态')
		expect(wrapper.get('img[alt="木材图片"]').attributes('src')).toBe('/images/物品图片/木材.png')
		await wrapper.get('.desktop-table input[aria-label="选择木材"]').setValue(true)
		expect(wrapper.text()).toContain('批量设置状态')
		expect(wrapper.text()).toContain('批量删除')

    await clickByText(wrapper, '商店')
    expect(wrapper.text()).not.toContain('一键补货')
    expect(wrapper.find('input[aria-label="绷带价格"]').exists()).toBe(true)
    expect(wrapper.find('input[aria-label="绷带库存"]').exists()).toBe(true)
		expect(wrapper.find('input[aria-label="绷带目标库存"]').exists()).toBe(true)
		expect(wrapper.get('img[alt="绷带图片"]').attributes('src')).toBe('/images/物品图片/绷带.png')

    wrapper.unmount()
	})

  it('物品类型筛选会收缩列表，且未修改时不出现主保存按钮', async () => {
    const wrapper = await mountView()
    await clickByText(wrapper, '宠物与物品')
    await clickByText(wrapper, '物品')

    expect(wrapper.text()).not.toContain('保存当前配置')
    expect(wrapper.text()).toContain('木材')
    expect(wrapper.text()).toContain('绷带')
    await wrapper.get('select[aria-label="物品类型"]').setValue('材料')
    expect(wrapper.text()).toContain('木材')
    expect(wrapper.text()).not.toContain('绷带')

    wrapper.unmount()
  })

	it('宠物工作台可预览并通过拖拽上传替换当前形态图片', async () => {
		const wrapper = await mountView()
		await clickByText(wrapper, '宠物与物品')
		expect(wrapper.get('img[alt="团子图片"]').attributes('src')).toBe('/images/宠物图片/团子.png')

		expect(document.body.textContent).toContain('点击选择或拖拽图片到这里')
		const dropzone = document.body.querySelector('.image-dropzone') as HTMLElement
		const file = new File([new Uint8Array([137, 80, 78, 71])], 'new-pet.png', { type: 'image/png' })
		const drop = new Event('drop', { bubbles: true, cancelable: true })
		Object.defineProperty(drop, 'dataTransfer', { value: { files: [file] } })
		dropzone.dispatchEvent(drop)
		await flushPromises()

		const preview = document.body.querySelector('.image-editor img[alt="团子图片"]') as HTMLImageElement
		expect(preview.getAttribute('src')).toBe('/images/上传/new-pet.png')
		expect(vi.mocked(fetch).mock.calls.some(([path]) => String(path) === '/api/admin/upload')).toBe(true)
		wrapper.unmount()
	})

	it('宠物工作台按谱系展示进化链，分支按规则顺序排列且搜索保留上下文', async () => {
		const wrapper = await mountView()
		await clickByText(wrapper, '宠物与物品')

		expect(wrapper.findAll('.lineage-studio')).toHaveLength(1)
		expect(wrapper.text()).toContain('团子4 个形态')
		expect(wrapper.find('.evolution-track').exists()).toBe(true)
		expect(wrapper.findAll('.track-connector')).toHaveLength(2)
		expect(wrapper.findAll('.stage-awakened .evolution-node')).toHaveLength(2)
		const nodeTexts = wrapper.findAll('.evolution-node').map((node) => node.text())
		expect(nodeTexts.join(' ')).toContain('基础形态团子')
		expect(nodeTexts.join(' ')).toContain('标准进化团子进化')
		expect(nodeTexts.join(' ')).toContain('觉醒形态团子觉醒日')
		expect(nodeTexts.join(' ')).toContain('觉醒形态团子觉醒月')
		expect(nodeTexts.findIndex((text) => text.includes('团子觉醒日')))
			.toBeLessThan(nodeTexts.findIndex((text) => text.includes('团子觉醒月')))
		expect(wrapper.find('[aria-label="团子进化暂无可用图片"]').exists()).toBe(true)

		await wrapper.get('.toolbar .searchbox input').setValue('团子觉醒月')
		expect(wrapper.findAll('.catalog-item')).toHaveLength(1)
		expect(wrapper.findAll('.evolution-node')).toHaveLength(4)
		expect(wrapper.text()).toContain('团子进化')
		wrapper.unmount()
	})

  it('图片资产在当前工作台直接预览、上传和编辑', async () => {
    const wrapper = await mountView()
    await clickByText(wrapper, '宠物与物品')
    await clickByText(wrapper, '图片资产')

    expect(wrapper.text()).toContain('森林背景')
    expect(wrapper.text()).toContain('上传图片')
		expect(wrapper.find('img[alt="森林背景图片"]').attributes('src')).toContain('/images/')
    wrapper.unmount()
  })
})
