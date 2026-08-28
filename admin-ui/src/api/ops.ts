import { api } from './client'

/** 与 models.GroupSwitch 对齐；后端无 json tag 时可能是 PascalCase。 */
export interface GroupSwitch {
  group_id: number
  platform: 'onebot' | 'qq_group' | 'qq_guild'
  space_id: string
  group_name: string
  is_active: boolean
}

export interface BulkGroupStateResult {
  updated: number
  groups: GroupSwitch[]
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

function asBool(v: unknown, fallback = true): boolean {
  if (typeof v === 'boolean') return v
  if (v === 1 || v === '1' || v === 'true') return true
  if (v === 0 || v === '0' || v === 'false') return false
  return fallback
}

export function normalizeGroup(raw: unknown): GroupSwitch {
  const o = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  return {
    group_id: asNumber(pick(o, 'group_id', 'GroupID')),
    platform: (asString(pick(o, 'platform', 'Platform')) || 'onebot') as GroupSwitch['platform'],
    space_id: asString(pick(o, 'space_id', 'SpaceID'), String(asNumber(pick(o, 'group_id', 'GroupID')))),
    group_name: asString(pick(o, 'group_name', 'GroupName')),
    is_active: asBool(pick(o, 'is_active', 'IsActive'), true),
  }
}

function messageFrom(payload: unknown, fallback: string): string {
  if (payload && typeof payload === 'object') {
    const o = payload as Record<string, unknown>
    if (typeof o.message === 'string' && o.message) return o.message
    if (typeof o.msg === 'string' && o.msg) return o.msg
  }
  return fallback
}

/** GET /api/admin/groups — 标准响应已由 api/client.ts 解包 */
export async function fetchGroups(): Promise<GroupSwitch[]> {
  const res = await api.get<unknown>('/api/admin/groups')
  if (!Array.isArray(res)) throw new Error('群开关接口 data 字段必须是数组')
  return res.map(normalizeGroup)
}

/** PUT /api/admin/groups/:id — body: group_name? / is_active? */
export async function updateGroup(
  groupId: number,
  body: { group_name?: string; is_active?: boolean },
): Promise<{ message: string; data: GroupSwitch }> {
  const res = await api.put<unknown>(`/api/admin/groups/${groupId}`, body)
  return { message: '群开关状态更新成功', data: normalizeGroup(res) }
}

/** PUT /api/admin/groups/bulk-state — groupIds=null 表示全部群组 */
export async function bulkUpdateGroupState(
  groupIds: number[] | null,
  isActive: boolean,
): Promise<BulkGroupStateResult> {
  const res = await api.put<{ updated: number; groups: unknown[] }>('/api/admin/groups/bulk-state', {
    group_ids: groupIds,
    is_active: isActive,
  })
  return {
    updated: asNumber(res.updated),
    groups: Array.isArray(res.groups) ? res.groups.map(normalizeGroup) : [],
  }
}

/** DELETE /api/admin/groups/:id */
export async function deleteGroup(groupId: number): Promise<string> {
  const res = await api.delete<unknown>(`/api/admin/groups/${groupId}`)
  return messageFrom(res, '群组记录已删除，将恢复默认全部开启状态')
}
