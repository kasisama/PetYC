import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import PlatformsView from './PlatformsView.vue'

function response(data: unknown) {
  return new Response(JSON.stringify({ code: 0, msg: 'success', data }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

afterEach(() => {
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
})

describe('PlatformsView', () => {
  it('uses a runtime config drawer and never displays stored secrets', async () => {
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      const path = String(input)
      if (path === '/api/admin/platforms/status') return response({ onebot: {}, qq_official: { capabilities: {} } })
      if (path === '/api/admin/groups') return response([])
      if (path === '/api/admin/platforms/config') return response({
        listen_address: '0.0.0.0',
        port: 8080,
        onebot: { token_configured: true },
        qq_official: {
          app_id: '1029384756', app_secret_configured: true,
          api_base: 'https://api.sgroup.qq.com', token_url: 'https://bots.qq.com/app/getAppAccessToken',
          shard_count: 1, markdown_enabled: true, keyboard_enabled: false,
          interaction_enabled: false, audit_enabled: false,
          group_events_enabled: true, guild_events_enabled: true,
        },
      })
      return response({ items: [] })
    }))

    const wrapper = mount(PlatformsView, { attachTo: document.body })
    await flushPromises()
    const configButton = wrapper.findAll('button').find((button) => button.text().includes('运行配置'))
    expect(configButton).toBeTruthy()
    await configButton!.trigger('click')
    await flushPromises()

    const text = document.body.textContent ?? ''
    expect(text).toContain('后台端口')
    expect(text).toContain('监听地址')
    expect(text).toContain('官方群事件')
    expect(text).toContain('QQ AppSecret')
    const secretInputs = Array.from(document.body.querySelectorAll<HTMLInputElement>('input[type="password"]'))
    expect(secretInputs.some((input) => input.placeholder === '已配置，留空保持不变')).toBe(true)
    expect(text).not.toContain('true')
    expect(text).not.toContain('false')
    wrapper.unmount()
  })
})
