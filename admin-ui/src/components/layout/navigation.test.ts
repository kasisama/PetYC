import { describe, expect, it } from 'vitest'
import { adminNavItems } from './navigation'

describe('admin navigation', () => {
  it('contains every expedition ecosystem domain', () => {
    expect(adminNavItems.map((item) => item.name)).toEqual([
      'dashboard', 'players', 'gameplay', 'communities', 'content', 'platforms', 'system',
    ])
  })
})
