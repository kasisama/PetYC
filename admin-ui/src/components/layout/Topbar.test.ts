import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useSession } from '../../composables/useSession'
import Topbar from './Topbar.vue'
import topbarSource from './Topbar.vue?raw'

const mocks = vi.hoisted(() => ({
  logout: vi.fn<() => Promise<void>>(),
  reloadConfigs: vi.fn<() => Promise<string>>(),
  replace: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ meta: { title: '玩家管理' } }),
  useRouter: () => ({ replace: mocks.replace }),
}))

vi.mock('../../api/auth', () => ({ logout: mocks.logout }))
vi.mock('../../api/config', () => ({ reloadConfigs: mocks.reloadConfigs }))

function mountTopbar() {
  return mount(Topbar, { attachTo: document.body })
}

describe('Topbar', () => {
  beforeEach(() => {
    mocks.logout.mockReset().mockResolvedValue()
    mocks.reloadConfigs.mockReset().mockResolvedValue('重载成功')
    mocks.replace.mockReset()
    useSession().setSession('operator')
  })

  it('桌面操作保持单行 40px，并提供主题、密度、热重载和账号入口', () => {
    const wrapper = mountTopbar()

    expect(wrapper.find('.page-title').text()).toBe('玩家管理')
    expect(wrapper.findAll('.theme-option')).toHaveLength(3)
    expect(wrapper.find('.density-button').exists()).toBe(true)
    expect(wrapper.find('.reload-btn').exists()).toBe(true)
    expect(wrapper.find('.account-trigger').exists()).toBe(true)
    expect(wrapper.findAll('button.topbar-action')).toHaveLength(7)
    expect(topbarSource).toMatch(/\.topbar-action\s*\{[^}]*height:\s*40px;[^}]*white-space:\s*nowrap;/s)

    wrapper.unmount()
  })

  it('平板隐藏操作文字，手机仅保留菜单、页面标题和账号入口', () => {
    expect(topbarSource).toMatch(
      /@media \(max-width:\s*1024px\)[\s\S]*?\.theme-switch\s*\{\s*display:\s*none;\s*\}[\s\S]*?\.action-label,[\s\S]*?display:\s*none;/,
    )
    expect(topbarSource).toMatch(
      /@media \(max-width:\s*700px\)[\s\S]*?\.density-button,[\s\S]*?\.reload-btn\s*\{\s*display:\s*none;\s*\}/,
    )
  })

  it('可用键盘打开账号菜单，并用 Escape 关闭后恢复焦点', async () => {
    const wrapper = mountTopbar()
    const trigger = wrapper.get('.account-trigger')

    expect(trigger.attributes('aria-expanded')).toBe('false')
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)

    await trigger.trigger('keydown', { key: 'ArrowDown' })
    await nextTick()

    const logoutItem = wrapper.get('.logout-item')
    expect(trigger.attributes('aria-expanded')).toBe('true')
    expect(document.activeElement).toBe(logoutItem.element)

    await logoutItem.trigger('keydown', { key: 'Escape' })
    await nextTick()

    expect(wrapper.find('[role="menu"]').exists()).toBe(false)
    expect(document.activeElement).toBe(trigger.element)

    wrapper.unmount()
  })

  it('点击账号区域外部会关闭下拉', async () => {
    const wrapper = mountTopbar()

    await wrapper.get('.account-trigger').trigger('click')
    expect(wrapper.find('[role="menu"]').exists()).toBe(true)

    document.body.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true }))
    await nextTick()

    expect(wrapper.find('[role="menu"]').exists()).toBe(false)

    wrapper.unmount()
  })

  it('退出操作只位于账号下拉，并完成会话清理与跳转', async () => {
    const wrapper = mountTopbar()

    expect(wrapper.find('.logout-item').exists()).toBe(false)
    await wrapper.get('.account-trigger').trigger('click')
    await wrapper.get('.logout-item').trigger('click')
    await flushPromises()

    expect(mocks.logout).toHaveBeenCalledOnce()
    expect(useSession().username.value).toBe('')
    expect(mocks.replace).toHaveBeenCalledWith({ name: 'login' })

    wrapper.unmount()
  })
})
