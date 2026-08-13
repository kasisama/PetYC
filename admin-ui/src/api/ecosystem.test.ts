import { afterEach, describe, expect, it, vi } from 'vitest'
import { buildPortHandoffURL, grantItem, reconnectQQ, savePlatformConfig } from './ecosystem'

afterEach(() => vi.unstubAllGlobals())

describe('ecosystem operations', () => {
  it('sends a grant as one idempotent request', async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => new Response(JSON.stringify({ code: 0, msg: 'success', data: { replayed: false } }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)
    await grantItem('account-1', { item_name: '调查记录', quantity: 10, reason: '异常补偿', idempotency_key: 'grant-123456' })
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/admin/players/account-1/grants')
  })

  it('sends gateway reconnect once with an audit reason', async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => new Response(JSON.stringify({ code: 0, msg: 'success', data: { accepted: true } }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)
    await reconnectQQ('排查连接异常')
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('saves platform runtime config in one request', async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => new Response(JSON.stringify({ code: 0, msg: 'success', data: { port: 8090 } }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)
    await savePlatformConfig({ port: 8090, qq_official: { app_id: '123456', markdown_enabled: true } })
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/admin/platforms/config')
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe('PUT')
  })

  it('builds a same-page handoff URL without exposing token in the query string', () => {
    const target = buildPortHandoffURL('http://127.0.0.1:8090', 'secret token')
    expect(target).toBe('http://127.0.0.1:8090/admin/platforms#port-handoff=secret%20token')
  })
})
