import { api, ApiError } from './client'

export const CONFIG_STATUS_CHANGED_EVENT = 'admin:config-status-changed'

function notifyConfigStatusChanged() {
  window.dispatchEvent(new Event(CONFIG_STATUS_CHANGED_EVENT))
}

/** 配置 schema 白名单（与 admin/api_config.go 一致） */
export type ConfigSchema =
  | 'system'
  | 'commands'
  | 'pet_species'
  | 'items'
  | 'shop_items'
  | 'work_settings'
  | 'menus'
  | 'images'
  | 'checkin_rewards'
  | 'live_events'
  | 'reward_tracks'
	| 'growth_roles'
	| 'growth_stances'
	| 'personality_rules'
	| 'codex_catalog'

export const CONFIG_SCHEMAS: ConfigSchema[] = [
  'system',
  'commands',
  'pet_species',
  'items',
  'shop_items',
  'work_settings',
  'menus',
  'images',
  'checkin_rewards',
	'growth_roles',
	'growth_stances',
	'personality_rules',
	'codex_catalog',
]

export const SCHEMA_LABELS: Partial<Record<ConfigSchema, string>> = {
	 live_events: '活动日历',
	 reward_tracks: '奖励轨道',
	 growth_roles: '宠物定位',
	 growth_stances: '远征姿态',
	 personality_rules: '性格规则',
	 codex_catalog: '图鉴目录',
  system: '系统参数',
  commands: '自定义指令',
  checkin_rewards: '签到奖励',
  work_settings: '挂机打工',
  shop_items: '商店货架',
  items: '游戏道具',
  pet_species: '宠物种类',
  menus: '菜单回复',
  images: '图片映射',
}

/** 删除接口 path 段 type 与主键字段（旧 DELETE /configs/:type/:key） */
type DeletableConfigSchema=Exclude<ConfigSchema,'system'|'growth_roles'|'growth_stances'|'personality_rules'|'codex_catalog'>
export const DELETE_TYPE_MAP: Record<
  DeletableConfigSchema,
  { type: string; keyOf: (row: Record<string, unknown>) => string }
> = {
	 live_events: { type: 'live_event', keyOf: (r) => String(r.key ?? r.Key ?? '') },
	 reward_tracks: { type: 'reward_track', keyOf: (r) => String(r.id ?? r.ID ?? '') },
  commands: { type: 'command', keyOf: (r) => String(r.FuncName ?? r.func_name ?? '') },
  pet_species: { type: 'pet_species', keyOf: (r) => String(r.Name ?? r.name ?? '') },
  items: { type: 'item', keyOf: (r) => String(r.Name ?? r.name ?? '') },
  shop_items: {
    type: 'shop_item',
    keyOf: (r) => String(r.ID ?? r.id ?? r.Name ?? r.name ?? ''),
  },
  work_settings: { type: 'work_setting', keyOf: (r) => String(r.Name ?? r.name ?? '') },
  menus: { type: 'menu', keyOf: (r) => String(r.Name ?? r.name ?? '') },
  images: { type: 'image', keyOf: (r) => String(r.Name ?? r.name ?? '') },
  checkin_rewards: {
    type: 'checkin_reward',
    keyOf: (r) => String(r.ID ?? r.id ?? ''),
  },
}

export interface SystemConfigRow {
  Key: string
  Value: string
}

export interface CommandConfigRow {
  FuncName: string
  Command: string
  DisplayName: string
  Category: string
  Description: string
  Enabled: boolean
  SortOrder: number
}

export type ItemStatus = 'active' | 'limited' | 'hidden' | 'disabled'

export interface PetSpeciesConfigRow {
  Name: string
  Image: string
  AdoptImage: string
  TrainStartImg: string
  TrainEndImg: string
  StudyStartImg: string
  StudyEndImg: string
  FitnessStartImg: string
  FitnessEndImg: string
  FavoriteFood: string
  FavoriteGift: string
  Health: number
  Wisdom: number
  Strength: number
  Defense: number
  Hunger: number
  Description: string
  EvolutionBranch: number
  Evolution: string
  EvolutionGrowth: number
  EvolutionAffect: number
  EvolutionImage: string
  Awaken: string
  AwakenGrowth: number
  AwakenAffect: number
  AwakenItems: string
  AwakenImage: string
  HealthMax: number
  WisdomMax: number
  StrengthMax: number
  DefenseMax: number
  HungerMax: number
  AffectionBonus: number
  GrowthBonus: number
  AttributeBonus: number
  CurrencyBonus: number
}

export interface ItemConfigRow {
  Name: string
  Status: ItemStatus
  Type: string
  RewardType: string
  ObtainType: number
  OpenReq: string
  Effect: string
  Time: number
  Image: string
  Description: string
  SellPrice: number
}

export interface ShopItemConfigRow {
  ID: number
  ShopType: string
  Name: string
  Image: string
  Stock: number
  RestockTarget: number
  Price: number
  Description: string
}

export interface CheckinRewardConfigRow {
  ID: number
  Type: string
  Day: string
  Currency: number
  Affection: number
  Items: string
  Image: string
}

export interface WorkSettingConfigRow {
  Name: string
  Time: number
  HungerCost: number
  RewardCoin: number
  RewardItems: string
  ReplyQuotes: string
  StartImage: string
  EndImage: string
}

export interface MenuConfigRow {
  Name: string
  Reply: string
}

export interface ImageConfigRow {
  Name: string
  Path: string
}

export interface UploadImageResult {
  message: string
  path: string
  url: string
}

export interface ConfigStatus {
  db_revision: number
  loaded_revision: number
  pending_reload: boolean
  saved_at: string | null
  loaded_at: string | null
}

export interface ContentEventRow {
  id?: number
  key: string
  name: string
  region: string
  story_choices: string
  starts_at: string
  ends_at: string
  active: boolean
}

export interface ContentRewardRow {
  id?: number
  event_key: string
  milestone: number
  item_name: string
  quantity: number
  description: string
}

export type GameSettingType = 'boolean' | 'number' | 'list' | 'text'
export type GameSettingValue = boolean | number | string | string[]

export interface GameSettingRow {
  key: string
  label: string
  group: string
  type: GameSettingType
  unit?: string
  description: string
  value: GameSettingValue
}

export interface GameSettingUpdate {
  key: string
  value: GameSettingValue
}

export interface BulkContentResult<T> {
  updated: number
  items: T[]
}

function pick(raw: Record<string, unknown>, ...keys: string[]): unknown {
  for (const k of keys) {
    if (raw[k] !== undefined && raw[k] !== null) return raw[k]
  }
  return undefined
}

function asString(v: unknown, fallback = ''): string {
  if (v == null) return fallback
  return String(v)
}

function asNumber(v: unknown, fallback = 0): number {
  if (typeof v === 'number' && !Number.isNaN(v)) return v
  if (typeof v === 'string' && v !== '' && !Number.isNaN(Number(v))) return Number(v)
  return fallback
}

function asBoolean(v: unknown, fallback = false): boolean {
  if (typeof v === 'boolean') return v
  if (typeof v === 'number') return v !== 0
  if (typeof v === 'string') {
    const normalized = v.trim().toLowerCase()
    if (['1', 'true', 'yes', 'on'].includes(normalized)) return true
    if (['0', 'false', 'no', 'off'].includes(normalized)) return false
  }
  return fallback
}

function messageFrom(payload: unknown, fallback: string): string {
  if (payload && typeof payload === 'object') {
    const o = payload as Record<string, unknown>
    if (typeof o.message === 'string' && o.message) return o.message
    if (typeof o.msg === 'string' && o.msg) return o.msg
  }
  return fallback
}

function asArray(res: unknown): unknown[] {
  if (Array.isArray(res)) return res
  if (res && typeof res === 'object' && Array.isArray((res as { data?: unknown }).data)) {
    return (res as { data: unknown[] }).data
  }
  return []
}

export function normalizeSystem(raw: unknown): SystemConfigRow {
  const o = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  return {
    Key: asString(pick(o, 'Key', 'key')),
    Value: asString(pick(o, 'Value', 'value')),
  }
}

export function normalizeCommand(raw: unknown): CommandConfigRow {
  const o = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  return {
    FuncName: asString(pick(o, 'FuncName', 'func_name', 'funcName')),
    Command: asString(pick(o, 'Command', 'command')),
    DisplayName: asString(pick(o, 'DisplayName', 'display_name', 'displayName'), asString(pick(o, 'Command', 'command'))),
    Category: asString(pick(o, 'Category', 'category'), '其他命令'),
    Description: asString(pick(o, 'Description', 'description')),
    Enabled: asBoolean(pick(o, 'Enabled', 'enabled'), true),
    SortOrder: asNumber(pick(o, 'SortOrder', 'sort_order', 'sortOrder')),
  }
}

export function normalizePetSpecies(raw: unknown): PetSpeciesConfigRow {
  const o = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  return {
    Name: asString(pick(o, 'Name', 'name')),
    Image: asString(pick(o, 'Image', 'image')),
    AdoptImage: asString(pick(o, 'AdoptImage', 'adopt_image', 'AdoptImg')),
    TrainStartImg: asString(pick(o, 'TrainStartImg', 'train_start_img')),
    TrainEndImg: asString(pick(o, 'TrainEndImg', 'train_end_img')),
    StudyStartImg: asString(pick(o, 'StudyStartImg', 'study_start_img')),
    StudyEndImg: asString(pick(o, 'StudyEndImg', 'study_end_img')),
    FitnessStartImg: asString(pick(o, 'FitnessStartImg', 'fitness_start_img')),
    FitnessEndImg: asString(pick(o, 'FitnessEndImg', 'fitness_end_img')),
    FavoriteFood: asString(pick(o, 'FavoriteFood', 'favorite_food')),
    FavoriteGift: asString(pick(o, 'FavoriteGift', 'favorite_gift')),
    Health: asNumber(pick(o, 'Health', 'health')),
    Wisdom: asNumber(pick(o, 'Wisdom', 'wisdom')),
    Strength: asNumber(pick(o, 'Strength', 'strength')),
    Defense: asNumber(pick(o, 'Defense', 'defense')),
    Hunger: asNumber(pick(o, 'Hunger', 'hunger')),
    Description: asString(pick(o, 'Description', 'description')),
    EvolutionBranch: asNumber(pick(o, 'EvolutionBranch', 'evolution_branch')),
    Evolution: asString(pick(o, 'Evolution', 'evolution')),
    EvolutionGrowth: asNumber(pick(o, 'EvolutionGrowth', 'evolution_growth')),
    EvolutionAffect: asNumber(pick(o, 'EvolutionAffect', 'evolution_affect')),
    EvolutionImage: asString(pick(o, 'EvolutionImage', 'evolution_image')),
    Awaken: asString(pick(o, 'Awaken', 'awaken')),
    AwakenGrowth: asNumber(pick(o, 'AwakenGrowth', 'awaken_growth')),
    AwakenAffect: asNumber(pick(o, 'AwakenAffect', 'awaken_affect')),
    AwakenItems: asString(pick(o, 'AwakenItems', 'awaken_items')),
    AwakenImage: asString(pick(o, 'AwakenImage', 'awaken_image')),
    HealthMax: asNumber(pick(o, 'HealthMax', 'health_max')),
    WisdomMax: asNumber(pick(o, 'WisdomMax', 'wisdom_max')),
    StrengthMax: asNumber(pick(o, 'StrengthMax', 'strength_max')),
    DefenseMax: asNumber(pick(o, 'DefenseMax', 'defense_max')),
    HungerMax: asNumber(pick(o, 'HungerMax', 'hunger_max')),
    AffectionBonus: asNumber(pick(o, 'AffectionBonus', 'affection_bonus')),
    GrowthBonus: asNumber(pick(o, 'GrowthBonus', 'growth_bonus')),
    AttributeBonus: asNumber(pick(o, 'AttributeBonus', 'attribute_bonus')),
    CurrencyBonus: asNumber(pick(o, 'CurrencyBonus', 'currency_bonus')),
  }
}

export function normalizeItem(raw: unknown): ItemConfigRow {
  const o = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  const rawStatus = asString(pick(o, 'Status', 'status'), 'active').toLowerCase()
  const status: ItemStatus = ['active', 'limited', 'hidden', 'disabled'].includes(rawStatus)
    ? rawStatus as ItemStatus
    : 'active'
  return {
    Name: asString(pick(o, 'Name', 'name')),
    Status: status,
    Type: asString(pick(o, 'Type', 'type')),
    RewardType: asString(pick(o, 'RewardType', 'reward_type')),
    ObtainType: asNumber(pick(o, 'ObtainType', 'obtain_type')),
    OpenReq: asString(pick(o, 'OpenReq', 'open_req')),
    Effect: asString(pick(o, 'Effect', 'effect')),
    Time: asNumber(pick(o, 'Time', 'time')),
    Image: asString(pick(o, 'Image', 'image')),
    Description: asString(pick(o, 'Description', 'description')),
    SellPrice: asNumber(pick(o, 'SellPrice', 'sell_price')),
  }
}

export function normalizeShopItem(raw: unknown): ShopItemConfigRow {
  const o = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  const stock = asNumber(pick(o, 'Stock', 'stock'), -1)
  return {
    ID: asNumber(pick(o, 'ID', 'id')),
    ShopType: asString(pick(o, 'ShopType', 'shop_type'), 'shop_normal'),
    Name: asString(pick(o, 'Name', 'name')),
    Image: asString(pick(o, 'Image', 'image')),
    Stock: stock,
    RestockTarget: asNumber(pick(o, 'RestockTarget', 'restock_target'), stock),
    Price: asNumber(pick(o, 'Price', 'price')),
    Description: asString(pick(o, 'Description', 'description')),
  }
}

export function normalizeCheckin(raw: unknown): CheckinRewardConfigRow {
  const o = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  return {
    ID: asNumber(pick(o, 'ID', 'id')),
    Type: asString(pick(o, 'Type', 'type')),
    Day: asString(pick(o, 'Day', 'day')),
    Currency: asNumber(pick(o, 'Currency', 'currency')),
    Affection: asNumber(pick(o, 'Affection', 'affection')),
    Items: asString(pick(o, 'Items', 'items')),
    Image: asString(pick(o, 'Image', 'image')),
  }
}

export function normalizeWork(raw: unknown): WorkSettingConfigRow {
  const o = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  return {
    Name: asString(pick(o, 'Name', 'name')),
    Time: asNumber(pick(o, 'Time', 'time')),
    HungerCost: asNumber(pick(o, 'HungerCost', 'hunger_cost')),
    RewardCoin: asNumber(pick(o, 'RewardCoin', 'reward_coin')),
    RewardItems: asString(pick(o, 'RewardItems', 'reward_items')),
    ReplyQuotes: asString(pick(o, 'ReplyQuotes', 'reply_quotes')),
    StartImage: asString(pick(o, 'StartImage', 'start_image')),
    EndImage: asString(pick(o, 'EndImage', 'end_image')),
  }
}

export function normalizeMenu(raw: unknown): MenuConfigRow {
  const o = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  return {
    Name: asString(pick(o, 'Name', 'name')),
    Reply: asString(pick(o, 'Reply', 'reply')),
  }
}

export function normalizeImage(raw: unknown): ImageConfigRow {
  const o = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  return {
    Name: asString(pick(o, 'Name', 'name')),
    Path: asString(pick(o, 'Path', 'path')),
  }
}

/** GET /api/admin/config/schemas */
export async function fetchConfigSchemas(): Promise<string[]> {
  const res = await api.get<unknown>('/api/admin/config/schemas')
  if (Array.isArray(res)) return res.map(String)
  return [...CONFIG_SCHEMAS]
}

/** GET /api/admin/config/status */
export function fetchConfigStatus(): Promise<ConfigStatus> {
  return api.get<ConfigStatus>('/api/admin/config/status')
}

/** GET /api/admin/config/:schema — 标准 {code,msg,data} 或直接数组 */
export async function fetchConfig(schema: ConfigSchema): Promise<unknown[]> {
  const res = await api.get<unknown>(`/api/admin/config/${schema}`)
  return asArray(res)
}

/** PUT /api/admin/config/:schema — body 为配置行数组 */
export async function saveConfig(schema: ConfigSchema, rows: unknown[]): Promise<void> {
  await api.put(`/api/admin/config/${schema}`, rows)
}

/** PUT /api/admin/content/events/:key — 活动与里程碑奖励事务保存 */
export function saveEventBundle(
  key: string,
  event: Partial<ContentEventRow>,
  rewards: Array<Partial<ContentRewardRow>>,
): Promise<{ event: ContentEventRow; rewards: ContentRewardRow[] }> {
  return api.put(`/api/admin/content/events/${encodeURIComponent(key)}`, { event, rewards })
}

/** DELETE /api/admin/content/events/:key — 同时删除活动与所属奖励 */
export function deleteEventBundle(key: string): Promise<{ deleted_rewards: number }> {
  return api.delete(`/api/admin/content/events/${encodeURIComponent(key)}`)
}

/** POST /api/admin/content/items/bulk — 单请求批量更新物品 */
export async function bulkItems(
  names: string[],
  action: 'delete' | 'set_status',
  status?: ItemStatus,
): Promise<BulkContentResult<ItemConfigRow>> {
  const body: { names: string[]; action: string; status?: ItemStatus } = { names, action }
  if (status !== undefined) body.status = status
  const result = await api.post<BulkContentResult<unknown>>('/api/admin/content/items/bulk', body)
  return { ...result, items: result.items.map(normalizeItem) }
}

/** POST /api/admin/content/shop-items/bulk — 单请求批量补货、设目标或删除 */
export async function bulkShopItems(
  ids: number[],
  action: 'delete' | 'restock' | 'set_target',
  value?: number,
): Promise<BulkContentResult<ShopItemConfigRow>> {
  const body: { ids: number[]; action: string; value?: number } = { ids, action }
  if (value !== undefined) body.value = value
  const result = await api.post<BulkContentResult<unknown>>('/api/admin/content/shop-items/bulk', body)
  return { ...result, items: result.items.map(normalizeShopItem) }
}

/** GET /api/admin/settings/game — 带中文元数据的可编辑游戏参数 */
export function fetchGameSettings(): Promise<GameSettingRow[]> {
  return api.get<GameSettingRow[]>('/api/admin/settings/game')
}

/** PUT /api/admin/settings/game — 批量保存游戏参数 */
export function saveGameSettings(rows: GameSettingUpdate[]): Promise<GameSettingRow[]> {
  return api.put<GameSettingRow[]>('/api/admin/settings/game', rows)
}

/** POST /api/admin/configs/reload — 旧接口可能返回 {message} */
export async function reloadConfigs(): Promise<string> {
  const res = await api.post<unknown>('/api/admin/configs/reload')
  notifyConfigStatusChanged()
  return messageFrom(res, '所有配置热重载同步成功')
}

/** POST /api/admin/configs/reset — 恢复出厂配置并自动重载；旧接口可能返回 {message}/{error} */
export async function resetConfigs(): Promise<string> {
  const res = await api.post<unknown>('/api/admin/configs/reset')
  notifyConfigStatusChanged()
  return messageFrom(res, '配置数据已重置为系统默认出厂配置并重载成功')
}

/** DELETE /api/admin/configs/:type/:key */
export async function deleteConfigItem(type: string, key: string): Promise<string> {
  const encoded = encodeURIComponent(key)
  const res = await api.delete<unknown>(`/api/admin/configs/${type}/${encoded}`)
  return messageFrom(res, '配置项删除成功')
}

/** POST /api/admin/upload — multipart field "file" */
export async function uploadImage(file: File): Promise<UploadImageResult> {
  const form = new FormData()
  form.append('file', file)
  const response = await fetch('/api/admin/upload', {
    method: 'POST',
    credentials: 'same-origin',
    body: form,
  })
  if (response.status === 401) {
    throw new ApiError(401, '登录已失效，请重新登录')
  }
  let payload: unknown
  try {
    payload = await response.json()
  } catch {
    throw new ApiError(response.status, '上传响应不是合法 JSON')
  }
  if (!response.ok) {
    const msg =
      payload && typeof payload === 'object' && typeof (payload as { error?: string }).error === 'string'
        ? (payload as { error: string }).error
        : `上传失败（HTTP ${response.status}）`
    throw new ApiError(response.status, msg)
  }
  if (payload && typeof payload === 'object' && 'code' in payload) {
    const std = payload as { code: number; msg: string; data?: UploadImageResult }
    if (std.code !== 0) throw new ApiError(std.code, std.msg || '上传失败')
    if (std.data) return std.data
  }
  const o = (payload && typeof payload === 'object' ? payload : {}) as Record<string, unknown>
  if (typeof o.error === 'string' && o.error) {
    throw new ApiError(1, o.error)
  }
  return {
    message: asString(pick(o, 'message', 'msg'), '图片上传成功'),
    path: asString(pick(o, 'path', 'Path')),
    url: asString(pick(o, 'url', 'Url')),
  }
}

/** 将配置中的相对路径转为可访问 URL */
export function imagePreviewUrl(path: string): string {
  if (!path) return ''
  if (path.startsWith('http://') || path.startsWith('https://') || path.startsWith('/')) {
    return path
  }
  // 后端静态目录 /images/ + 相对路径（兼容反斜杠）
  const normalized = path.replace(/\\/g, '/')
  return `/images/${normalized}`
}

/** 系统参数中文说明（键不在表中时退回键名） */
export const SYSTEM_KEY_META: Record<string, { label: string; group: string }> = {
  'Core.InitialPets': { label: '初始可领养宠物', group: '基础设置' },
  'Core.CoinName': { label: '货币名称', group: '基础设置' },
  'Core.InitialCoin': { label: '初始货币', group: '基础设置' },
  'Core.RenameCost': { label: '改名费用', group: '基础设置' },
  'Core.RenameBlocklist': { label: '改名屏蔽词', group: '基础设置' },
  'Core.MasterQQ': { label: '主人 QQ', group: '基础设置' },
  'Core.NotifyQQ': { label: '系统通知 QQ', group: '基础设置' },
  'Core.ImageHost': { label: '图片服务器地址', group: '基础设置' },
  'Core.TreatCost': { label: '治疗费用', group: '生命状态' },
  'Core.DyingSaveTime': { label: '濒死抢救时限', group: '生命状态' },
  'Core.DyingProtectTime': { label: '抢救成功保护时限', group: '生命状态' },
  'Core.EscapeFindTime': { label: '逃跑寻回时限', group: '生命状态' },
  'Core.LostCooldown': { label: '失去宠物后重新领养冷却', group: '生命状态' },
  'Core.CheckinLike': { label: '签到是否点赞', group: '签到与交易' },
  'Core.TxFee': { label: '转账/交易手续费', group: '签到与交易' },
  'Core.CurrencySync': { label: '是否开启外部货币同步', group: '货币对接' },
  'Core.CurrencySyncPath': { label: '外部 INI 文件路径', group: '货币对接' },
  'Core.CurrencySyncSection': { label: 'INI 配置节', group: '货币对接' },
  'Core.CurrencySyncKey': { label: 'INI 键名', group: '货币对接' },
  'Interaction.StudyGrowth': { label: '学习成长值', group: '学习训练' },
  'Interaction.StudyLimit': { label: '学习每日次数上限', group: '学习训练' },
  'Interaction.StudyHungerCost': { label: '学习饱食消耗', group: '学习训练' },
  'Interaction.TrainGrowth': { label: '锻炼成长值', group: '学习训练' },
  'Interaction.TrainLimit': { label: '锻炼每日次数上限', group: '学习训练' },
  'Interaction.TrainHungerCost': { label: '锻炼饱食消耗', group: '学习训练' },
  'Interaction.FitnessGrowth': { label: '健身成长值', group: '学习训练' },
  'Interaction.FitnessLimit': { label: '健身每日次数上限', group: '学习训练' },
  'Interaction.FitnessHungerCost': { label: '健身饱食消耗', group: '学习训练' },
  'Interaction.WashGrowth': { label: '洗澡成长', group: '日常互动' },
  'Interaction.WashAffection': { label: '洗澡好感', group: '日常互动' },
  'Interaction.WashHungerCost': { label: '洗澡饱食消耗', group: '日常互动' },
  'Interaction.WalkGrowth': { label: '散步成长', group: '日常互动' },
  'Interaction.WalkAffection': { label: '散步好感', group: '日常互动' },
  'Interaction.WalkGrowthLimit': { label: '散步成长每日上限', group: '日常互动' },
  'Interaction.WalkAffectLimit': { label: '散步好感每日上限', group: '日常互动' },
  'Interaction.WalkInterval': { label: '散步冷却', group: '日常互动' },
  'Interaction.WalkHungerCost': { label: '散步饱食消耗', group: '日常互动' },
  'Interaction.TouchGrowth': { label: '摸头成长', group: '日常互动' },
  'Interaction.TouchAffection': { label: '摸头好感', group: '日常互动' },
  'Interaction.TouchGrowthLimit': { label: '摸头成长每日上限', group: '日常互动' },
  'Interaction.TouchAffectLimit': { label: '摸头好感每日上限', group: '日常互动' },
  'Interaction.TouchInterval': { label: '摸头冷却', group: '日常互动' },
  'Interaction.TouchHungerCost': { label: '摸头饱食消耗', group: '日常互动' },
  'Interaction.RpsAffection': { label: '猜拳好感', group: '日常互动' },
  'Interaction.RpsAffectLimit': { label: '猜拳每日上限', group: '日常互动' },
  'Interaction.RpsInterval': { label: '猜拳冷却', group: '日常互动' },
  'Interaction.RpsHungerCost': { label: '猜拳饱食消耗', group: '日常互动' },
  'Interaction.GiftLimit': { label: '送礼每日上限', group: '日常互动' },
  'Interaction.HungerMoodFlush': { label: '心情刷新间隔', group: '日常互动' },
  'Interaction.SnackHungerCost': { label: '偷袭饱食消耗', group: '战斗互动' },
  'Interaction.SnackSuccess': { label: '偷袭成功率', group: '战斗互动' },
  'Interaction.SnackInterval': { label: '偷袭冷却', group: '战斗互动' },
  'Interaction.SnackProtect': { label: '偷袭保护时间', group: '战斗互动' },
  'Interaction.CounterHunger': { label: '反击饱食消耗', group: '战斗互动' },
  'Interaction.CounterSuccess': { label: '反击成功率', group: '战斗互动' },
  'Interaction.CreateFamilyCoin': { label: '创建家族所需货币', group: '家族' },
  'Interaction.CreateFamilyItem': { label: '创建家族所需物品', group: '家族' },
  'Interaction.FamilySizeLimit': { label: '家族人数上限', group: '家族' },
  'Interaction.TreeResultNutri': { label: '神树成熟所需养分', group: '家族' },
  'Interaction.TreeRewardItems': { label: '神树奖励物品池', group: '家族' },
  'Interaction.FishHungerCost': { label: '钓鱼饱食消耗', group: '钓鱼抽奖' },
  'Interaction.FishSuccessRate': { label: '钓鱼成功率', group: '钓鱼抽奖' },
  'Interaction.FishSpecies': { label: '可钓鱼类', group: '钓鱼抽奖' },
  'Interaction.LotteryItem': { label: '抽奖所需物品', group: '钓鱼抽奖' },
  'Interaction.LotteryRewardStr': { label: '抽奖奖励概率配置', group: '钓鱼抽奖' },
  'Interaction.WorkTime': { label: '默认打工时间', group: '打工商店' },
  'Interaction.WorkRewardCoin': { label: '默认打工货币奖励', group: '打工商店' },
  'Interaction.WorkRewardItems': { label: '默认打工物品奖励', group: '打工商店' },
  'Interaction.WorkHungerCost': { label: '默认打工饱食消耗', group: '打工商店' },
  'Interaction.BuyLimit': { label: '单次购买数量限制', group: '打工商店' },
  'Interaction.SellNoPriceGrowth': { label: '无售价物品出售成长奖励', group: '打工商店' },
}

export const SYSTEM_GROUP_ORDER = [
  '基础设置',
  '生命状态',
  '签到与交易',
  '货币对接',
  '学习训练',
  '日常互动',
  '战斗互动',
  '家族',
  '钓鱼抽奖',
  '打工商店',
  '其他',
]

export function systemKeyLabel(key: string): string {
  return SYSTEM_KEY_META[key]?.label ?? key
}

export function systemKeyGroup(key: string): string {
  return SYSTEM_KEY_META[key]?.group ?? '其他'
}

export const ITEM_TYPES = [
  '礼包',
  '血量',
  '饱食',
  '好感',
  '智慧',
  '力量',
  '防御',
  '成长',
  '鱼类',
  '家族物品',
  '抽奖物品',
  '觉醒物品',
]

export const REWARD_TYPES = ['固定奖励', '随机奖励', '']

export const SHOP_TYPES = [
  { value: 'shop_normal', label: '普通商店' },
  { value: 'shop_affection', label: '好感商店' },
]

export const CHECKIN_TYPES = [
  { value: 'checkin_newbie', label: '新手七日' },
  { value: 'checkin_weekly', label: '每周循环' },
]

export function emptyPetSpecies(name = ''): PetSpeciesConfigRow {
  return {
    Name: name,
    Image: '',
    AdoptImage: '',
    TrainStartImg: '',
    TrainEndImg: '',
    StudyStartImg: '',
    StudyEndImg: '',
    FitnessStartImg: '',
    FitnessEndImg: '',
    FavoriteFood: '',
    FavoriteGift: '',
    Health: 100,
    Wisdom: 0,
    Strength: 0,
    Defense: 0,
    Hunger: 100,
    Description: '',
    EvolutionBranch: 0,
    Evolution: '',
    EvolutionGrowth: 0,
    EvolutionAffect: 0,
    EvolutionImage: '',
    Awaken: '',
    AwakenGrowth: 0,
    AwakenAffect: 0,
    AwakenItems: '',
    AwakenImage: '',
    HealthMax: 100,
    WisdomMax: 100,
    StrengthMax: 100,
    DefenseMax: 100,
    HungerMax: 100,
    AffectionBonus: 0,
    GrowthBonus: 0,
    AttributeBonus: 0,
    CurrencyBonus: 0,
  }
}

export function emptyItem(name = ''): ItemConfigRow {
  return {
    Name: name,
    Status: 'active',
    Type: '礼包',
    RewardType: '',
    ObtainType: 0,
    OpenReq: '',
    Effect: '',
    Time: 0,
    Image: '',
    Description: '',
    SellPrice: 0,
  }
}

export function emptyShopItem(shopType = 'shop_normal'): ShopItemConfigRow {
  return {
    ID: 0,
    ShopType: shopType,
    Name: '',
    Image: '',
    Stock: -1,
    RestockTarget: 0,
    Price: 0,
    Description: '',
  }
}

export function emptyWork(name = ''): WorkSettingConfigRow {
  return {
    Name: name,
    Time: 60,
    HungerCost: 0,
    RewardCoin: 0,
    RewardItems: '',
    ReplyQuotes: '',
    StartImage: '',
    EndImage: '',
  }
}

export function emptyMenu(name = ''): MenuConfigRow {
  return { Name: name, Reply: '' }
}

export function emptyImage(name = ''): ImageConfigRow {
  return { Name: name, Path: '' }
}

export function emptyCommand(funcName = ''): CommandConfigRow {
  return {
    FuncName: funcName,
    Command: '',
    DisplayName: '',
    Category: '基础命令',
    Description: '',
    Enabled: true,
    SortOrder: 0,
  }
}

export function emptyCheckin(type = 'checkin_weekly', day = '1'): CheckinRewardConfigRow {
  return {
    ID: 0,
    Type: type,
    Day: day,
    Currency: 0,
    Affection: 0,
    Items: '',
    Image: '',
  }
}
