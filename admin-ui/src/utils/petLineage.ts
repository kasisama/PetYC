import type { PetEvolutionRuleConfigRow, PetSpeciesConfigRow } from '../api/config'

export type PetLineageNode = {
  pet: PetSpeciesConfigRow
  depth: number
  rule?: PetEvolutionRuleConfigRow
}

export type PetLineage = {
  familyKey: string
  title: string
  nodes: PetLineageNode[]
}

export type PetLineageIssue = {
  familyKey: string
  message: string
  pets: PetSpeciesConfigRow[]
}

export type PetLineageResult = {
  lineages: PetLineage[]
  issues: PetLineageIssue[]
}

const stageRank: Record<string, number> = { base: 0, evolved: 1, awakened: 2 }

function compareText(left: string, right: string) {
  return left.localeCompare(right, 'zh-CN')
}

function petOrder(left: PetSpeciesConfigRow, right: PetSpeciesConfigRow) {
  return (stageRank[left.Stage] ?? 99) - (stageRank[right.Stage] ?? 99)
    || compareText(left.Name, right.Name)
    || compareText(left.Key, right.Key)
}

function groupTitle(pets: PetSpeciesConfigRow[]) {
  return pets.find((pet) => pet.Stage === 'base')?.Name || pets[0]?.Name || '未命名谱系'
}

export function stageLabel(stage: string) {
  return ({ base: '基础形态', evolved: '标准进化', awakened: '觉醒形态' } as Record<string, string>)[stage] || '未知阶段'
}

export function buildPetLineages(pets: PetSpeciesConfigRow[], rules: PetEvolutionRuleConfigRow[]): PetLineageResult {
  const groups = new Map<string, PetSpeciesConfigRow[]>()
  const issues: PetLineageIssue[] = []
  const ruleByEdge = new Map<string, PetEvolutionRuleConfigRow>()
  for (const rule of rules) {
    const key = `${rule.FromFormKey}\u0000${rule.ToFormKey}`
    const previous = ruleByEdge.get(key)
    if (!previous || rule.SortOrder < previous.SortOrder || (rule.SortOrder === previous.SortOrder && compareText(rule.Key, previous.Key) < 0)) ruleByEdge.set(key, rule)
  }
  for (const pet of pets) {
    const key = pet.FamilyKey.trim()
    const group = groups.get(key) ?? []
    group.push(pet)
    groups.set(key, group)
  }

  const lineages: PetLineage[] = []
  for (const [familyKey, group] of [...groups.entries()].sort(([left], [right]) => compareText(left, right))) {
    const sortedPets = [...group].sort(petOrder)
    const petByKey = new Map(sortedPets.map((pet) => [pet.Key, pet]))
    const familyIssues: string[] = []
    if (!familyKey) familyIssues.push('缺少谱系标识')
    const bases = sortedPets.filter((pet) => pet.Stage === 'base')
    if (bases.length !== 1) familyIssues.push(`应有且仅有一个基础形态，当前为 ${bases.length} 个`)

    for (const pet of sortedPets) {
      const rank = stageRank[pet.Stage]
      if (rank === undefined) {
        familyIssues.push(`“${pet.Name || pet.Key}”的阶段无效`)
        continue
      }
      if (pet.Stage === 'base') {
        if (pet.PreviousFormKey) familyIssues.push(`基础形态“${pet.Name || pet.Key}”不应配置前置形态`)
        continue
      }
      if (!pet.PreviousFormKey) {
        familyIssues.push(`“${pet.Name || pet.Key}”缺少前置形态`)
        continue
      }
      const previous = petByKey.get(pet.PreviousFormKey)
      if (!previous) {
        familyIssues.push(`“${pet.Name || pet.Key}”引用的前置形态不存在或不属于本谱系`)
      } else if ((stageRank[previous.Stage] ?? 99) >= rank) {
        familyIssues.push(`“${pet.Name || pet.Key}”的前置形态阶段不低于自身`)
      }
    }

    const children = new Map<string, PetSpeciesConfigRow[]>()
    for (const pet of sortedPets) {
      if (!pet.PreviousFormKey) continue
      const list = children.get(pet.PreviousFormKey) ?? []
      list.push(pet)
      children.set(pet.PreviousFormKey, list)
    }
    const compareChild = (parentKey: string) => (left: PetSpeciesConfigRow, right: PetSpeciesConfigRow) => {
      const leftRule = ruleByEdge.get(`${parentKey}\u0000${left.Key}`)
      const rightRule = ruleByEdge.get(`${parentKey}\u0000${right.Key}`)
      return (leftRule?.SortOrder ?? Number.MAX_SAFE_INTEGER) - (rightRule?.SortOrder ?? Number.MAX_SAFE_INTEGER)
        || petOrder(left, right)
    }

    const nodes: PetLineageNode[] = []
    const visited = new Set<string>()
    const visit = (pet: PetSpeciesConfigRow, depth: number) => {
      if (visited.has(pet.Key)) {
        familyIssues.push(`“${pet.Name || pet.Key}”存在循环引用`)
        return
      }
      visited.add(pet.Key)
      nodes.push({ pet, depth, rule: pet.PreviousFormKey ? ruleByEdge.get(`${pet.PreviousFormKey}\u0000${pet.Key}`) : undefined })
      for (const child of [...(children.get(pet.Key) ?? [])].sort(compareChild(pet.Key))) visit(child, depth + 1)
    }
    for (const base of bases.sort(petOrder)) visit(base, 0)
    if (visited.size !== sortedPets.length && familyIssues.length === 0) familyIssues.push('存在无法从基础形态到达的循环或断链')

    if (familyIssues.length > 0) {
      issues.push({ familyKey: familyKey || '未分组', message: [...new Set(familyIssues)].join('；'), pets: sortedPets })
    } else {
      lineages.push({ familyKey, title: groupTitle(sortedPets), nodes })
    }
  }
  return { lineages, issues }
}
