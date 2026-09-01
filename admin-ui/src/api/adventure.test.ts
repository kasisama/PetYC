import { describe, expect, it } from 'vitest'
import { normalizeAdventureCatalog } from './adventure'

describe('normalizeAdventureCatalog', () => {
  it('fills absent legacy collections with empty arrays without losing known rows', () => {
    const catalog = normalizeAdventureCatalog({
      maps: [{ key: 'sunlit', name: '灿光原野', region: '晨光', description: '', image: '', recommended_level: 1, enabled: true, sort_order: 10 }],
      zones: null as unknown as [],
    })

    expect(catalog.maps).toHaveLength(1)
    expect(catalog.zones).toEqual([])
    expect(catalog.shop_items).toEqual([])
    expect(catalog.equipment_recipe_materials).toEqual([])
  })
})
