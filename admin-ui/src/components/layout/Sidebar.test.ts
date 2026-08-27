import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { __resetPlatformStatusForTests } from '../../composables/usePlatformStatus'
import Sidebar from './Sidebar.vue'

function response(data: unknown) {
  return new Response(JSON.stringify({ code: 0, msg: 'success', data }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

afterEach(() => {
  __resetPlatformStatusForTests()
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('Sidebar platform status', () => {
  it('switches between offline, online, and unknown without remounting', async () => {
    vi.useFakeTimers()
    let connected = false
    let statusFails = false
    vi.stubGlobal('fetch', vi.fn(async () => {
      if (statusFails) return new Response(JSON.stringify({ message: 'temporary failure' }), { status: 503, headers: { 'Content-Type': 'application/json' } })
      return response({ onebot: { connected }, qq_official: { connected: false, capabilities: {} } })
    }))
    const routeNames = ['dashboard', 'players', 'gameplay', 'adventure', 'communities', 'content', 'profiles', 'platforms', 'system']
    const router = createRouter({
      history: createMemoryHistory(),
      routes: routeNames.map((name, index) => ({ path: index === 0 ? '/' : `/${name}`, name, component: { template: '<div />' } })),
    })
    await router.push('/')
    await router.isReady()

    const wrapper = mount(Sidebar, { global: { plugins: [router] } })
    await flushPromises()
    expect(wrapper.text()).toContain('机器人暂未连接')

    connected = true
    await vi.advanceTimersByTimeAsync(5_000)
    await flushPromises()
    expect(wrapper.text()).toContain('机器人在线')

    statusFails = true
    await vi.advanceTimersByTimeAsync(5_000)
    await flushPromises()
    expect(wrapper.text()).toContain('机器人状态未知')
    wrapper.unmount()
  })
})
