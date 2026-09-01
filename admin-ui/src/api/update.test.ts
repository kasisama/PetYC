import { afterEach, describe, expect, it, vi } from 'vitest'
import { installUpdate } from './update'

afterEach(() => vi.unstubAllGlobals())

describe('installUpdate', () => {
  it('sends reason and confirmation before starting an install', async () => {
    const fetchMock = vi.fn(async () =>
      new Response(JSON.stringify({ code: 0, msg: 'success', data: { state: 'checking' } }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)
    await installUpdate('安装最新稳定版', '安装更新')
    expect(fetchMock).toHaveBeenCalledWith('/api/admin/updates/install', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ reason: '安装最新稳定版', confirmation: '安装更新' }),
    }))
  })
})
