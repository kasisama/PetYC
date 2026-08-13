import { describe, expect, it } from 'vitest'
import { normalizePlayerDetail, normalizePlayerPage } from './ecosystem'

describe('ecosystem response normalization', () => {
  it('normalizes null player items to an empty array', () => {
    expect(normalizePlayerPage({ items: null, total: null, page: null, limit: null }).items).toEqual([])
  })

  it('normalizes every nullable player detail collection', () => {
    const detail = normalizePlayerDetail({
      account: { ID: 'account-1' },
      inventory: null,
      codex: null,
      identities: null,
      expeditions: null,
      communities: null,
      notifications: null,
    })
    expect(detail.inventory).toEqual([])
    expect(detail.codex).toEqual([])
    expect(detail.identities).toEqual([])
    expect(detail.expeditions).toEqual([])
    expect(detail.communities).toEqual([])
    expect(detail.notifications.Enabled).toBe(true)
  })
})
