import { describe, expect, it } from 'vitest'
import type { PetEvolutionRuleConfigRow, PetSpeciesConfigRow } from '../api/config'
import { buildPetLineages } from './petLineage'

function pet(overrides: Partial<PetSpeciesConfigRow>): PetSpeciesConfigRow {
  return {
    Key: '', FamilyKey: 'lumi', Stage: 'base', PreviousFormKey: '', Adoptable: true, Archetype: 'balanced', CodexEntryKey: '', Name: '', Image: '', AdoptImage: '', TrainStartImg: '', TrainEndImg: '', StudyStartImg: '', StudyEndImg: '', FitnessStartImg: '', FitnessEndImg: '', FavoriteFood: '', FavoriteGift: '', Health: 0, Wisdom: 0, Strength: 0, Defense: 0, Hunger: 0, Description: '', EvolutionBranch: 0, Evolution: '', EvolutionGrowth: 0, EvolutionAffect: 0, EvolutionImage: '', Awaken: '', AwakenGrowth: 0, AwakenAffect: 0, AwakenItems: '', AwakenImage: '', HealthMax: 0, WisdomMax: 0, StrengthMax: 0, DefenseMax: 0, HungerMax: 0, AffectionBonus: 0, GrowthBonus: 0, AttributeBonus: 0, CurrencyBonus: 0,
    ...overrides,
  }
}

function rule(overrides: Partial<PetEvolutionRuleConfigRow>): PetEvolutionRuleConfigRow {
  return { Key: '', FromFormKey: '', ToFormKey: '', RequiredGrowth: 0, RequiredAffection: 0, BranchLabel: '', Enabled: true, SortOrder: 0, ...overrides }
}

describe('buildPetLineages', () => {
  it('将乱序多分支形态按进化规则排序为稳定链路', () => {
    const result = buildPetLineages([
      pet({ Key: 'awaken-b', Name: '月荫灵', Stage: 'awakened', PreviousFormKey: 'evolved' }),
      pet({ Key: 'base', Name: '光芽兽' }),
      pet({ Key: 'awaken-a', Name: '曦冠灵', Stage: 'awakened', PreviousFormKey: 'evolved' }),
      pet({ Key: 'evolved', Name: '曜叶兽', Stage: 'evolved', PreviousFormKey: 'base' }),
    ], [
      rule({ Key: 'standard', FromFormKey: 'base', ToFormKey: 'evolved', SortOrder: 10 }),
      rule({ Key: 'moon', FromFormKey: 'evolved', ToFormKey: 'awaken-b', SortOrder: 30 }),
      rule({ Key: 'sun', FromFormKey: 'evolved', ToFormKey: 'awaken-a', SortOrder: 20, BranchLabel: '曦光路线' }),
    ])

    expect(result.issues).toEqual([])
    expect(result.lineages).toHaveLength(1)
    expect(result.lineages[0]?.nodes.map((node) => node.pet.Key)).toEqual(['base', 'evolved', 'awaken-a', 'awaken-b'])
    expect(result.lineages[0]?.nodes[2]?.rule?.BranchLabel).toBe('曦光路线')
  })

  it('将断链和无基础形态归入异常谱系', () => {
    const result = buildPetLineages([pet({ Key: 'orphan', Name: '孤立觉醒', Stage: 'awakened', PreviousFormKey: 'missing' })], [])

    expect(result.lineages).toEqual([])
    expect(result.issues[0]?.message).toContain('基础形态')
    expect(result.issues[0]?.message).toContain('前置形态不存在')
  })
})
