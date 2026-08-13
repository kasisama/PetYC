import { api } from './client'

/** 后端 UserPet 无 json tag，默认以 Go 字段名序列化；同时兼容可能的 snake_case。 */
export interface UserPet {
  id: number
  user_id: number
  group_id: number
  pet_type: string
  name: string
  image: string
  status: string
  mood: string
  mood_points: number
  affection: number
  growth: number
  health: number
  wisdom: number
  strength: number
  defense: number
  hunger: number
  family: string
  family_score: number
  newbie_check: number
  currency: number
  last_checkin: string | null
  study_time: string | null
  study_item: string
  train_time: string | null
  train_item: string
  work_time: string | null
  work_type: string
  fitness_time: string | null
  fitness_item: string
  dying_time: string | null
  escape_time: string | null
  lost_time: string | null
  bind_key: string
  updated_at: string
}

export interface PetsListParams {
  user_id?: string
  group_id?: string
  name?: string
  status?: string
  pet_type?: string
  page?: number
  limit?: number
}

export interface PetsListResult {
  total: number
  page: number
  limit: number
  data: UserPet[]
}

/** 与 admin.PetUpdateRequest 对齐（有 json tag，snake_case）。 */
export interface PetUpdatePayload {
  name?: string
  status?: string
  mood?: string
  mood_points?: number
  affection?: number
  growth?: number
  health?: number
  wisdom?: number
  strength?: number
  defense?: number
  hunger?: number
  currency?: number
  family?: string
  family_score?: number
}

export type PetOperateAction = 'revive' | 'recall' | 'clear_cooldown'

export interface ItemGivePayload {
  user_id: number
  group_id: number
  item_name: string
  quantity: number
}

function pick(raw: Record<string, unknown>, ...keys: string[]): unknown {
  for (const k of keys) {
    if (raw[k] !== undefined && raw[k] !== null) return raw[k]
  }
  return undefined
}

function asNumber(v: unknown, fallback = 0): number {
  if (typeof v === 'number' && !Number.isNaN(v)) return v
  if (typeof v === 'string' && v !== '' && !Number.isNaN(Number(v))) return Number(v)
  return fallback
}

function asString(v: unknown, fallback = ''): string {
  if (v == null) return fallback
  return String(v)
}

function asTime(v: unknown): string | null {
  if (v == null || v === '') return null
  return String(v)
}

/** 将后端宠物记录规范为前端统一 snake_case。 */
export function normalizePet(raw: unknown): UserPet {
  const o = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  return {
    id: asNumber(pick(o, 'id', 'ID')),
    user_id: asNumber(pick(o, 'user_id', 'UserID')),
    group_id: asNumber(pick(o, 'group_id', 'GroupID')),
    pet_type: asString(pick(o, 'pet_type', 'PetType')),
    name: asString(pick(o, 'name', 'Name')),
    image: asString(pick(o, 'image', 'Image')),
    status: asString(pick(o, 'status', 'Status')),
    mood: asString(pick(o, 'mood', 'Mood')),
    mood_points: asNumber(pick(o, 'mood_points', 'MoodPoints')),
    affection: asNumber(pick(o, 'affection', 'Affection')),
    growth: asNumber(pick(o, 'growth', 'Growth')),
    health: asNumber(pick(o, 'health', 'Health')),
    wisdom: asNumber(pick(o, 'wisdom', 'Wisdom')),
    strength: asNumber(pick(o, 'strength', 'Strength')),
    defense: asNumber(pick(o, 'defense', 'Defense')),
    hunger: asNumber(pick(o, 'hunger', 'Hunger')),
    family: asString(pick(o, 'family', 'Family')),
    family_score: asNumber(pick(o, 'family_score', 'FamilyScore')),
    newbie_check: asNumber(pick(o, 'newbie_check', 'NewbieCheck')),
    currency: asNumber(pick(o, 'currency', 'Currency')),
    last_checkin: asTime(pick(o, 'last_checkin', 'LastCheckin')),
    study_time: asTime(pick(o, 'study_time', 'StudyTime')),
    study_item: asString(pick(o, 'study_item', 'StudyItem')),
    train_time: asTime(pick(o, 'train_time', 'TrainTime')),
    train_item: asString(pick(o, 'train_item', 'TrainItem')),
    work_time: asTime(pick(o, 'work_time', 'WorkTime')),
    work_type: asString(pick(o, 'work_type', 'WorkType')),
    fitness_time: asTime(pick(o, 'fitness_time', 'FitnessTime')),
    fitness_item: asString(pick(o, 'fitness_item', 'FitnessItem')),
    dying_time: asTime(pick(o, 'dying_time', 'DyingTime')),
    escape_time: asTime(pick(o, 'escape_time', 'EscapeTime')),
    lost_time: asTime(pick(o, 'lost_time', 'LostTime')),
    bind_key: asString(pick(o, 'bind_key', 'BindKey')),
    updated_at: asString(pick(o, 'updated_at', 'UpdatedAt')),
  }
}

export async function fetchPets(params: PetsListParams): Promise<PetsListResult> {
  const q = new URLSearchParams()
  if (params.user_id) q.set('user_id', params.user_id)
  if (params.group_id) q.set('group_id', params.group_id)
  if (params.name) q.set('name', params.name)
  if (params.status) q.set('status', params.status)
  if (params.pet_type) q.set('pet_type', params.pet_type)
  q.set('page', String(params.page ?? 1))
  q.set('limit', String(params.limit ?? 10))

  const raw = await api.get<{
    total?: number
    page?: number
    limit?: number
    data?: unknown[]
  }>(`/api/admin/pets?${q.toString()}`)

  const list = Array.isArray(raw?.data) ? raw.data.map(normalizePet) : []
  return {
    total: Number(raw?.total ?? 0),
    page: Number(raw?.page ?? params.page ?? 1),
    limit: Number(raw?.limit ?? params.limit ?? 10),
    data: list,
  }
}

export async function updatePet(id: number, body: PetUpdatePayload): Promise<UserPet> {
  const raw = await api.put<{ message?: string; data?: unknown }>(`/api/admin/pets/${id}`, body)
  // 旧接口返回 {message, data}；兼容层会整包返回
  if (raw && typeof raw === 'object' && 'data' in raw) {
    return normalizePet((raw as { data: unknown }).data)
  }
  return normalizePet(raw)
}

export async function deletePet(id: number): Promise<string> {
  const raw = await api.delete<{ message?: string } | string>(`/api/admin/pets/${id}`)
  if (typeof raw === 'string') return raw
  if (raw && typeof raw === 'object' && 'message' in raw) {
    return (raw as { message?: string }).message || '宠物存档已成功删除'
  }
  return '宠物存档已成功删除'
}

export async function operatePet(id: number, action: PetOperateAction): Promise<UserPet> {
  const raw = await api.post<{ message?: string; data?: unknown }>(
    `/api/admin/pets/${id}/operate`,
    { action },
  )
  if (raw && typeof raw === 'object' && 'data' in raw) {
    return normalizePet((raw as { data: unknown }).data)
  }
  return normalizePet(raw)
}

export async function giveItem(body: ItemGivePayload): Promise<string> {
  const raw = await api.post<{ message?: string; quantity?: number; data?: unknown }>(
    '/api/admin/items/give',
    body,
  )
  if (raw && typeof raw === 'object' && 'message' in raw) {
    return (raw as { message?: string }).message || '物品分发更新成功'
  }
  return '物品分发更新成功'
}

/** 尝试加载种类名列表（配置中心标准接口）；失败时返回空。 */
export async function fetchSpeciesNames(): Promise<string[]> {
  try {
    const rows = await api.get<unknown[]>('/api/admin/config/pet_species')
    if (!Array.isArray(rows)) return []
    return rows
      .map((r) => {
        const o = r as Record<string, unknown>
        return String(o.Name ?? o.name ?? '')
      })
      .filter(Boolean)
  } catch {
    return []
  }
}

/** 尝试加载道具名列表，供背包调整下拉参考。 */
export async function fetchItemNames(): Promise<string[]> {
  try {
    const rows = await api.get<unknown[]>('/api/admin/config/items')
    if (!Array.isArray(rows)) return []
    return rows
      .map((r) => {
        const o = r as Record<string, unknown>
        return String(o.Name ?? o.name ?? '')
      })
      .filter(Boolean)
  } catch {
    return []
  }
}
