import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import UpdateCard from './UpdateCard.vue'

function response(data: unknown) {
  return new Response(JSON.stringify({ code: 0, msg: 'success', data }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('UpdateCard', () => {
  it('shows an install action for a portable update', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => response({
      currentVersion: '1.0.0', latestVersion: '1.1.0', available: true,
      notes: '安全更新', publishedAt: '2026-08-27T00:00:00Z', canAutoUpdate: true,
      installMode: 'portable', reason: '', releaseUrl: 'https://example.com/release',
    })))
    const wrapper = mount(UpdateCard)
    await flushPromises()
    expect(wrapper.text()).toContain('发现新版本 v1.1.0')
    expect(wrapper.text()).toContain('安全更新')
    expect(wrapper.findAll('button').some((button) => button.text().includes('立即更新'))).toBe(true)
  })

  it('falls back to a release link when automatic replacement is unavailable', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => response({
      currentVersion: '1.0.0', latestVersion: '1.1.0', available: true,
      notes: '', publishedAt: '', canAutoUpdate: false, installMode: 'manual',
      reason: 'systemd 服务请通过部署命令更新', releaseUrl: 'https://example.com/release',
    })))
    const wrapper = mount(UpdateCard)
    await flushPromises()
    expect(wrapper.text()).toContain('systemd 服务请通过部署命令更新')
    const link = wrapper.get('a')
    expect(link.text()).toContain('手动下载')
    expect(link.attributes('href')).toBe('https://example.com/release')
  })
})

