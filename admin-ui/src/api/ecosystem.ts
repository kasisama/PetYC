import { api } from './client'

export interface Overview {
  range: '7d' | '30d'
  players: number
  pets: number
  active_expeditions: number
  completed_expeditions: number
  active_communities: number
  boss_participants: number
  overdue_expeditions: number
  command_success_rate: number
  command_total: number
  today_completed_expeditions: number
  config_pending_reload: boolean
  platform_error_count: number
  active_explorations: number
  active_combats: number
  active_adventure_expeditions: number
  active_adventure_bosses: number
  adventure_shop_transactions: number
  journey_badges_earned: number
  journey_badges_spent: number
  adventure_reference_errors: number
  content_counts: { maps?:number; zones?:number; monsters?:number; items?:number; equipment?:number; events?:number }
  generated_at: string
}

export interface PlayerSummary {
  account_id: string
  pet_name: string
  pet_type: string
  pet_image: string
  role: string
  growth: number
  bond_level: number
  identity_count: number
  community_count: number
  expedition_id: string
  expedition_status: string
  last_active_at: string | null
}

export interface ExpeditionSummary { running: number; today_completed: number; overdue: number; cancelled: number }
export interface PageResult<T> { items: T[]; total: number; page: number; limit: number; summary?: ExpeditionSummary }

export interface GrowthRuleRole { name:string;description:string;skill_1:string;skill_2:string;skill_3:string;enabled:boolean;sort_order:number }
export interface GrowthRuleStance { name:string;description:string;enabled:boolean;sort_order:number }
export interface PersonalityRule { name:string;dimension:'care'|'explore'|'support';min_threshold:number;description:string;enabled:boolean;sort_order:number }
export interface CodexCatalogRule { id?:number;category:string;entry_key:string;region:string;source_type:string;source_key:string;description:string;enabled:boolean;sort_order:number }
export interface GameplayDistribution { name:string;count:number;percentage:number;description?:string;enabled:boolean;skills?:string }
export interface GameplayGrowth {
  summary:{player_count:number;role_coverage_rate:number;personality_formation_rate:number;personality_unformed:number;configured_rule_count:number;configuration_complete:boolean}
  roles:GameplayDistribution[];stances:GameplayDistribution[];personalities:GameplayDistribution[];skills:GameplayDistribution[]
  rules:{roles:GrowthRuleRole[];stances:GrowthRuleStance[];personalities:PersonalityRule[];codex:CodexCatalogRule[]};warnings:string[]
}
export interface CodexCatalogItem extends CodexCatalogRule { id:number;discovered_players:number;completed_players:number;average_progress:number }
export interface GameplayCodex {summary:{catalog_count:number;discovered_entries:number;discovery_rate:number};items:CodexCatalogItem[];warnings:string[]}

export interface PlayerDetail {
  account: { ID: string; CreatedAt: string; UpdatedAt: string; active_pet_id?: string }
  pet: Record<string, unknown>
  pets?: Array<Record<string, unknown>>
  active_pet_id?: string
  pet_image: string
  inventory: Array<Record<string, unknown>>
  codex: Array<Record<string, unknown>>
  identities: Array<{ id: number; platform: string; scene_type: string; app_id: string; scope_id: string; subject_id: string; created_at: string }>
  expeditions: Array<Record<string, unknown>>
  communities: Array<Record<string, unknown>>
  notifications: { Enabled: boolean }
  adventure_inventory: Array<Record<string, unknown>>
  adventure_equipment: Array<Record<string, unknown>>
  adventure_blueprints: Array<Record<string, unknown>>
  adventure_wallet: Record<string, unknown>
  adventure_ledger: Array<Record<string, unknown>>
}

export interface CommunitySummary {
  ID: string
  Platform: string
  SceneType: string
  Level: number
  Materials: number
  NotificationsEnabled: boolean
  member_count: number
  squad_count: number
  open_help_count: number
  UpdatedAt: string
}

export interface PlatformStatus {
  onebot: Record<string, unknown>
  qq_official: Record<string, unknown> & { capabilities?: Record<string, boolean> }
}

export interface PlatformRuntimeConfig {
  listen_address: string
  port: number
  onebot: { token_configured: boolean }
  qq_official: {
    app_id: string
    app_secret_configured: boolean
    api_base: string
    token_url: string
    shard_count: number
    markdown_enabled: boolean
    keyboard_enabled: boolean
    interaction_enabled: boolean
    audit_enabled: boolean
    group_events_enabled: boolean
    guild_events_enabled: boolean
  }
  port_handoff?: { address: string; confirmation_token: string; expires_at: string }
}

export interface PlatformRuntimeUpdate {
  listen_address?: string
  port?: number
  onebot?: { token?: string }
  qq_official?: Partial<{
    app_id: string
    app_secret: string
    api_base: string
    token_url: string
    shard_count: number
    markdown_enabled: boolean
    keyboard_enabled: boolean
    interaction_enabled: boolean
    audit_enabled: boolean
    group_events_enabled: boolean
    guild_events_enabled: boolean
  }>
}

function arrayOrEmpty<T>(value: unknown): T[] {
  return Array.isArray(value) ? value as T[] : []
}

function numberOr(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}

export function normalizePlayerPage(raw: unknown): PageResult<PlayerSummary> {
  const value = raw && typeof raw === 'object' ? raw as Record<string, unknown> : {}
  return {
    items: arrayOrEmpty<PlayerSummary>(value.items),
    total: numberOr(value.total, 0),
    page: numberOr(value.page, 1),
    limit: numberOr(value.limit, 20),
  }
}

export function normalizePlayerDetail(raw: unknown): PlayerDetail {
  const value = raw && typeof raw === 'object' ? raw as Record<string, unknown> : {}
  const notifications = value.notifications && typeof value.notifications === 'object'
    ? value.notifications as { Enabled?: boolean }
    : {}
  return {
    account: (value.account ?? { ID: '', CreatedAt: '', UpdatedAt: '' }) as PlayerDetail['account'],
    pet: value.pet && typeof value.pet === 'object' ? value.pet as Record<string, unknown> : {},
    pets: arrayOrEmpty<Record<string, unknown>>(value.pets),
    active_pet_id: typeof value.active_pet_id === 'string' ? value.active_pet_id : '',
    pet_image: typeof value.pet_image === 'string' ? value.pet_image : '',
    inventory: arrayOrEmpty<Record<string, unknown>>(value.inventory),
    codex: arrayOrEmpty<Record<string, unknown>>(value.codex),
    identities: arrayOrEmpty<PlayerDetail['identities'][number]>(value.identities),
    expeditions: arrayOrEmpty<Record<string, unknown>>(value.expeditions),
    communities: arrayOrEmpty<Record<string, unknown>>(value.communities),
    notifications: { Enabled: typeof notifications.Enabled === 'boolean' ? notifications.Enabled : true },
    adventure_inventory: arrayOrEmpty<Record<string, unknown>>(value.adventure_inventory),
    adventure_equipment: arrayOrEmpty<Record<string, unknown>>(value.adventure_equipment),
    adventure_blueprints: arrayOrEmpty<Record<string, unknown>>(value.adventure_blueprints),
    adventure_wallet: value.adventure_wallet && typeof value.adventure_wallet === 'object' ? value.adventure_wallet as Record<string, unknown> : {},
    adventure_ledger: arrayOrEmpty<Record<string, unknown>>(value.adventure_ledger),
  }
}

export function getOverview(range: '7d' | '30d') { return api.get<Overview>(`/api/admin/overview?range=${range}`) }
export async function getPlayers(query: URLSearchParams) { return normalizePlayerPage(await api.get<unknown>(`/api/admin/players?${query}`)) }
export async function getPlayer(accountId: string) { return normalizePlayerDetail(await api.get<unknown>(`/api/admin/players/${accountId}`)) }
export function grantItem(accountId: string, body: { item_name: string; quantity: number; reason: string; idempotency_key: string }) { return api.post(`/api/admin/players/${accountId}/grants`, body) }
export function setActivePet(accountId: string, petId: string, reason: string) { return api.post(`/api/admin/players/${accountId}/active_pet`, { pet_id: petId, reason }) }
export function resetSeason(eventKey: string, body: { reason: string; confirmation: string; season_key?: string }) {
  return api.post(`/api/admin/seasons/${encodeURIComponent(eventKey)}/reset`, body)
}
export function cancelExpedition(id: string, reason: string) { return api.post(`/api/admin/expeditions/${id}/cancel`, { reason, expected_status: 'running' }) }
export function reconcileExpedition(id: string, reason: string) { return api.post(`/api/admin/expeditions/${id}/reconcile`, { reason }) }
export async function getExpeditions(query: URLSearchParams) {
  const result = await api.get<PageResult<Record<string, unknown>>>(`/api/admin/expeditions?${query}`)
  return {...result,items:arrayOrEmpty<Record<string,unknown>>(result.items),summary:result.summary??{running:0,today_completed:0,overdue:0,cancelled:0}}
}
export function getGameplayDistributions() { return api.get<Record<string, Array<{name:string;count:number}>>>('/api/admin/gameplay/distributions') }
export async function getGameplayGrowth() {
  const result=await api.get<GameplayGrowth>('/api/admin/gameplay/growth')
  return {...result,roles:arrayOrEmpty<GameplayDistribution>(result.roles),stances:arrayOrEmpty<GameplayDistribution>(result.stances),personalities:arrayOrEmpty<GameplayDistribution>(result.personalities),skills:arrayOrEmpty<GameplayDistribution>(result.skills),warnings:arrayOrEmpty<string>(result.warnings),rules:{roles:arrayOrEmpty<GrowthRuleRole>(result.rules?.roles),stances:arrayOrEmpty<GrowthRuleStance>(result.rules?.stances),personalities:arrayOrEmpty<PersonalityRule>(result.rules?.personalities),codex:arrayOrEmpty<CodexCatalogRule>(result.rules?.codex)}}
}
export async function getGameplayCodex(query=new URLSearchParams()) {
  const result=await api.get<GameplayCodex>(`/api/admin/gameplay/codex?${query}`)
  return {...result,items:arrayOrEmpty<CodexCatalogItem>(result.items),warnings:arrayOrEmpty<string>(result.warnings)}
}
export function setPlayerNotifications(accountId: string, enabled: boolean, reason: string) { return api.put(`/api/admin/players/${accountId}/notifications`, { enabled, reason }) }
export function deletePlayerIdentity(accountId: string, identityId: number, reason: string) { return api.delete(`/api/admin/players/${accountId}/identities/${identityId}`, { reason }) }
export function deletePlayer(accountId: string, confirmation: string, reason: string) { return api.delete(`/api/admin/players/${accountId}`, { confirmation, reason }) }
export function getCommunities(query: URLSearchParams) { return api.get<PageResult<CommunitySummary>>(`/api/admin/communities?${query}`) }
export function getCommunity(id: string) { return api.get<Record<string, unknown>>(`/api/admin/communities/${encodeURIComponent(id)}`) }
export function updateFacility(communityId:string, facilityId:number, body:{level:number;progress:number;reason:string;expected_updated_at:string}) { return api.put(`/api/admin/communities/${encodeURIComponent(communityId)}/facilities/${facilityId}`, body) }
export function resetCommunityBoss(communityId:string, reason:string, confirmation:string) { return api.post(`/api/admin/communities/${encodeURIComponent(communityId)}/boss/reset`, {reason,confirmation}) }
export function closeHelpRequest(code:string, reason:string) { return api.post(`/api/admin/help-requests/${code}/close`, {reason}) }
export function disbandSquad(id:string, reason:string, confirmation:string) { return api.post(`/api/admin/squads/${id}/disband`, {reason,confirmation}) }
export function setCommunityNotifications(id:string, enabled:boolean, reason:string) { return api.put(`/api/admin/communities/${encodeURIComponent(id)}/notifications`, {enabled,reason}) }
export function getPlatformStatus() { return api.get<PlatformStatus>('/api/admin/platforms/status') }
export function getPlatformConfig() { return api.get<PlatformRuntimeConfig>('/api/admin/platforms/config') }
export function savePlatformConfig(body: PlatformRuntimeUpdate) { return api.put<PlatformRuntimeConfig>('/api/admin/platforms/config', body) }
export function confirmPlatformPort(confirmationToken: string) { return api.post<{confirmed:boolean}>('/api/admin/platforms/port/confirm', { confirmation_token: confirmationToken }) }
export function buildPortHandoffURL(address: string, confirmationToken: string) {
  return `${address.replace(/\/$/, '')}/admin/platforms#port-handoff=${encodeURIComponent(confirmationToken)}`
}
export function reconnectQQ(reason: string) { return api.post('/api/admin/platforms/qq/reconnect', { reason }) }
export function getQQEnvTemplate() { return api.get<{ template: string }>('/api/admin/platforms/qq/env-template') }
export function getAuditLogs(query: URLSearchParams) { return api.get<PageResult<Record<string, unknown>>>(`/api/admin/audit-logs?${query}`) }
