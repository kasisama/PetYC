<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  IconAdjustments,
  IconBolt,
  IconBox,
  IconCalendarEvent,
  IconCheck,
  IconChevronRight,
  IconDeviceFloppy,
  IconMessageCircle,
  IconPackage,
  IconPaw,
  IconPhoto,
  IconPlus,
  IconRefresh,
  IconReload,
  IconSearch,
  IconShoppingBag,
  IconSparkles,
  IconTrash,
} from '@tabler/icons-vue'
import {
  bulkItems,
  bulkShopItems,
  deleteConfigItem,
  deleteEventBundle,
  emptyCommand,
  emptyItem,
  emptyImage,
  emptyMenu,
  emptyPetSpecies,
  emptyShopItem,
  fetchConfig,
  fetchConfigStatus,
  fetchGameSettings,
  normalizeCommand,
  normalizeItem,
  normalizeImage,
  normalizeMenu,
  normalizePetSpecies,
  normalizeShopItem,
  reloadConfigs,
  imagePreviewUrl,
  saveConfig,
  saveEventBundle,
  saveGameSettings,
  uploadImage,
  type CommandConfigRow,
  type ConfigStatus,
  type ContentEventRow,
  type ContentRewardRow,
  type GameSettingRow,
  type ItemConfigRow,
  type ItemStatus,
  type ImageConfigRow,
  type MenuConfigRow,
  type PetSpeciesConfigRow,
  type ShopItemConfigRow,
} from '../api/config'
import PageHeader from '../components/ui/PageHeader.vue'
import UiDrawer from '../components/ui/UiDrawer.vue'
import UiState from '../components/ui/UiState.vue'
import { useToast } from '../composables/useToast'
import { useUnsavedChanges } from '../composables/useUnsavedChanges'
import { cloneConfigValue } from '../utils/configClone'

type MainTab = 'events' | 'assets' | 'text' | 'game'
type AssetTab = 'pets' | 'items' | 'shop' | 'images'
type TextTab = 'commands' | 'menus'
type EditorKind = 'event' | 'pet' | 'item' | 'shop' | 'image' | 'command' | 'menu'
type RewardItemDraft = { item_name: string; quantity: number }
type RewardGroupDraft = { milestone: number; description: string; items: RewardItemDraft[] }
type EventDraft = Omit<ContentEventRow, 'story_choices'> & {
  choices: string[]
  rewardGroups: RewardGroupDraft[]
}

const route = useRoute()
const router = useRouter()
const toast = useToast()
const allowedTabs: MainTab[] = ['events', 'assets', 'text', 'game']
const initialTab = allowedTabs.includes(route.query.tab as MainTab) ? route.query.tab as MainTab : 'events'

const tab = ref<MainTab>(initialTab)
const assetTab = ref<AssetTab>('pets')
const textTab = ref<TextTab>('commands')
const search = ref('')
const loading = ref(false)
const saving = ref(false)
const status = ref<ConfigStatus | null>(null)
const events = ref<ContentEventRow[]>([])
const rewards = ref<ContentRewardRow[]>([])
const pets = ref<PetSpeciesConfigRow[]>([])
const items = ref<ItemConfigRow[]>([])
const shops = ref<ShopItemConfigRow[]>([])
const images = ref<ImageConfigRow[]>([])
const commands = ref<CommandConfigRow[]>([])
const menus = ref<MenuConfigRow[]>([])
const gameSettings = ref<GameSettingRow[]>([])
const snapshot = ref('')
const editorSnapshot = ref('')
const selectedItems = ref<string[]>([])
const selectedShops = ref<number[]>([])
const bulkItemStatus = ref<ItemStatus>('active')
const bulkShopTarget = ref(50)
const editor = reactive<{ kind: EditorKind | null; index: number; draft: any }>({
  kind: null,
  index: -1,
  draft: null,
})
const editorTitle = computed(() => {
  if (editor.kind === 'event') return editor.index < 0 ? '新建活动' : '编辑活动'
  const labels: Record<string, string> = { pet: '宠物种类', item: '物品', shop: '商品', image: '图片资产', command: '命令', menu: '菜单场景' }
  return `${editor.index < 0 ? '新增' : '编辑'}${labels[String(editor.kind)] ?? '内容'}`
})

const editableState = computed(() => ({
  pets: pets.value,
  items: items.value,
  shops: shops.value,
  images: images.value,
  commands: commands.value,
  menus: menus.value,
  gameSettings: gameSettings.value,
}))
const dirty = computed(() => snapshot.value !== '' && JSON.stringify(editableState.value) !== snapshot.value)
const editorDirty = computed(() => editor.kind !== null && editorSnapshot.value !== '' && JSON.stringify(editor.draft) !== editorSnapshot.value)
const hasUnsavedChanges = computed(() => dirty.value || editorDirty.value)
useUnsavedChanges(hasUnsavedChanges)

const query = computed(() => search.value.trim().toLowerCase())
const visibleEvents = computed(() => events.value.filter((row) => `${row.name} ${row.region}`.toLowerCase().includes(query.value)))
const visiblePets = computed(() => pets.value.filter((row) => `${row.Name} ${row.Description} ${row.FavoriteFood}`.toLowerCase().includes(query.value)))
const visibleItems = computed(() => items.value.filter((row) => `${row.Name} ${row.Type} ${row.Description}`.toLowerCase().includes(query.value)))
const visibleShops = computed(() => shops.value.filter((row) => `${row.Name} ${shopTypeLabel(row.ShopType)} ${row.Description}`.toLowerCase().includes(query.value)))
const visibleImages = computed(() => images.value.filter((row) => `${row.Name} ${row.Path}`.toLowerCase().includes(query.value)))
const commandGroups = computed(() => {
  const groups = new Map<string, CommandConfigRow[]>()
  commands.value
    .filter((row) => `${row.DisplayName} ${row.Command} ${row.Category} ${row.Description}`.toLowerCase().includes(query.value))
    .sort((left, right) => left.Category.localeCompare(right.Category, 'zh-CN') || left.SortOrder - right.SortOrder)
    .forEach((row) => {
      const category = row.Category || '其他命令'
      const rows = groups.get(category) ?? []
      rows.push(row)
      groups.set(category, rows)
    })
  return [...groups.entries()].map(([category, rows]) => ({ category, rows }))
})
const visibleMenus = computed(() => menus.value.filter((row) => `${menuScene(row.Name)} ${row.Reply}`.toLowerCase().includes(query.value)))
const gameGroups = computed(() => {
  const groups = new Map<string, GameSettingRow[]>()
  gameSettings.value.forEach((row) => {
    const rows = groups.get(row.group) ?? []
    rows.push(row)
    groups.set(row.group, rows)
  })
  return [...groups.entries()].map(([group, rows]) => ({ group, rows }))
})

const itemStatusOptions: Array<{ value: ItemStatus; label: string }> = [
  { value: 'active', label: '正常上架' },
  { value: 'limited', label: '限时供应' },
  { value: 'hidden', label: '仅隐藏' },
  { value: 'disabled', label: '已停用' },
]

function shopTypeLabel(value: string) {
  return value === 'shop_affection' ? '羁绊商店' : '普通商店'
}

function itemStatusLabel(value: ItemStatus) {
  return itemStatusOptions.find((option) => option.value === value)?.label ?? '正常上架'
}

function menuScene(name: string) {
  const labels: Record<string, string> = {
    main: '主菜单',
    adopt: '领养引导',
    status: '状态查询',
    shop: '商店入口',
    help: '帮助说明',
    主菜单: '主菜单',
    今日与状态: '今日与状态',
    远征指南: '远征指南',
    成长与图鉴: '成长与图鉴',
    陪伴互动: '陪伴互动',
    营地与小队: '营地与小队',
    账号与隐私: '账号与隐私',
  }
  return labels[name] ?? labels[name.toLowerCase()] ?? name
}

function formatDate(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '未设置' : date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function eventStatus(row: ContentEventRow) {
  if (!row.active) return '已停用'
  const now = Date.now()
  if (now < new Date(row.starts_at).getTime()) return '待开始'
  if (now >= new Date(row.ends_at).getTime()) return '已结束'
  return '进行中'
}

function eventRewardCount(key: string) {
  return new Set(rewards.value.filter((row) => row.event_key === key).map((row) => row.milestone)).size
}

function parseChoices(value: string) {
  try {
    const parsed = JSON.parse(value)
    if (Array.isArray(parsed)) return parsed.map(String)
  } catch {
    return []
  }
  return []
}

function rewardGroupsFor(key: string): RewardGroupDraft[] {
  const groups = new Map<number, RewardGroupDraft>()
  rewards.value
    .filter((row) => row.event_key === key)
    .sort((left, right) => left.milestone - right.milestone)
    .forEach((row) => {
      const group = groups.get(row.milestone) ?? {
        milestone: row.milestone,
        description: row.description,
        items: [],
      }
      group.items.push({ item_name: row.item_name, quantity: row.quantity })
      groups.set(row.milestone, group)
    })
  return [...groups.values()]
}

function blankEvent(): EventDraft {
  const start = new Date()
  const end = new Date(Date.now() + 7 * 86400000)
  return {
    key: '',
    name: '',
    region: '森林',
    choices: ['记录线索', '继续调查', '呼叫支援'],
    starts_at: start.toISOString().slice(0, 16),
    ends_at: end.toISOString().slice(0, 16),
    active: true,
    rewardGroups: [{ milestone: 100, description: '首个调查节点', items: [{ item_name: items.value[0]?.Name ?? '', quantity: 1 }] }],
  }
}

function openEvent(row?: ContentEventRow) {
  const draft = row
    ? {
        ...cloneConfigValue(row),
        choices: parseChoices(row.story_choices),
        starts_at: String(row.starts_at).slice(0, 16),
        ends_at: String(row.ends_at).slice(0, 16),
        rewardGroups: rewardGroupsFor(row.key),
      }
    : blankEvent()
  editor.kind = 'event'
  editor.index = row ? events.value.indexOf(row) : -1
  editor.draft = draft
  editorSnapshot.value = JSON.stringify(draft)
}

function openEditor(kind: Exclude<EditorKind, 'event'>, row?: any) {
  const collection = kind === 'pet' ? pets.value
    : kind === 'item' ? items.value
      : kind === 'shop' ? shops.value
        : kind === 'image' ? images.value
          : kind === 'command' ? commands.value
            : menus.value
  editor.kind = kind
  editor.index = row ? collection.indexOf(row) : -1
  editor.draft = row ? cloneConfigValue(row)
    : kind === 'pet' ? emptyPetSpecies('')
      : kind === 'item' ? emptyItem('')
        : kind === 'shop' ? emptyShopItem('shop_normal')
          : kind === 'image' ? emptyImage('')
            : kind === 'command' ? emptyCommand('')
              : emptyMenu('')
  editorSnapshot.value = JSON.stringify(editor.draft)
}

function closeEditor() {
  if (saving.value) return
  if (editorDirty.value && !confirm('当前编辑内容尚未应用，确定关闭吗？')) return
  editor.kind = null
  editorSnapshot.value = ''
}

function fillEventSample() {
  if (editor.kind !== 'event') return
  const start = new Date(Date.now() + 86400000)
  const end = new Date(Date.now() + 8 * 86400000)
  editor.draft = {
    ...editor.draft,
    key: editor.index >= 0 ? editor.draft.key : 'forest-field-test',
    name: '森林生态联合调查',
    region: '森林',
    choices: ['记录足迹', '采集样本', '呼叫巡护员'],
    starts_at: start.toISOString().slice(0, 16),
    ends_at: end.toISOString().slice(0, 16),
    active: true,
    rewardGroups: [
      { milestone: 100, description: '完成初步调查', items: [{ item_name: items.value[0]?.Name ?? '木材', quantity: 2 }, { item_name: items.value[1]?.Name ?? '绷带', quantity: 1 }] },
      { milestone: 300, description: '完成区域调查', items: [{ item_name: items.value[0]?.Name ?? '木材', quantity: 5 }] },
    ],
  }
}

function addRewardGroup() {
  const groups = editor.draft.rewardGroups as RewardGroupDraft[]
  const last = groups.at(-1)?.milestone ?? 0
  groups.push({ milestone: last + 100, description: '', items: [{ item_name: items.value[0]?.Name ?? '', quantity: 1 }] })
}

function addRewardItem(group: RewardGroupDraft) {
  group.items.push({ item_name: items.value[0]?.Name ?? '', quantity: 1 })
}

function removeRewardGroup(index: string | number) {
  editor.draft.rewardGroups.splice(Number(index), 1)
}

function cumulativeRewards(index: string | number) {
  const totals = new Map<string, number>()
  const groups = [...editor.draft.rewardGroups].sort((left: RewardGroupDraft, right: RewardGroupDraft) => left.milestone - right.milestone)
  groups.slice(0, Number(index) + 1).forEach((group: RewardGroupDraft) => {
    group.items.forEach((item) => totals.set(item.item_name, (totals.get(item.item_name) ?? 0) + Number(item.quantity || 0)))
  })
  return [...totals.entries()].filter(([name]) => name).map(([name, quantity]) => `${name} × ${quantity}`)
}

async function saveCurrentEvent() {
  if (editor.kind !== 'event') return
  const draft = editor.draft as EventDraft
  if (!draft.key.trim() || !draft.name.trim()) return toast.error('请填写活动键和活动名称')
  if (draft.choices.length < 2 || draft.choices.some((choice) => !choice.trim())) return toast.error('请填写至少两个完整的故事选项')
  if (new Date(draft.ends_at).getTime() <= new Date(draft.starts_at).getTime()) return toast.error('结束时间必须晚于开始时间')
  if (draft.rewardGroups.some((group) => group.milestone <= 0 || group.items.length === 0 || group.items.some((item) => !item.item_name || item.quantity <= 0))) {
    return toast.error('里程碑、奖励物品和数量必须填写完整')
  }
  const event: ContentEventRow = {
    ...(draft.id ? { id: draft.id } : {}),
    key: draft.key.trim(),
    name: draft.name.trim(),
    region: draft.region.trim(),
    story_choices: JSON.stringify(draft.choices.map((choice) => choice.trim())),
    starts_at: new Date(draft.starts_at).toISOString(),
    ends_at: new Date(draft.ends_at).toISOString(),
    active: draft.active,
  }
  const eventRewards = draft.rewardGroups.flatMap((group) => group.items.map((item) => ({
    event_key: event.key,
    milestone: Number(group.milestone),
    item_name: item.item_name,
    quantity: Number(item.quantity),
    description: group.description,
  })))
  saving.value = true
  try {
    const saved = await saveEventBundle(event.key, event, eventRewards)
    const savedEvent = saved.event ?? event
    const savedRewards = saved.rewards ?? eventRewards
    if (editor.index < 0) events.value.push(savedEvent)
    else events.value.splice(editor.index, 1, savedEvent)
    rewards.value = [...rewards.value.filter((row) => row.event_key !== event.key), ...savedRewards]
    status.value = await fetchConfigStatus()
    editor.kind = null
    editorSnapshot.value = ''
    toast.success('活动与奖励已一次保存')
  } catch (reason) {
    toast.error(reason instanceof Error ? reason.message : '活动保存失败')
  } finally {
    saving.value = false
  }
}

async function removeCurrentEvent() {
  if (editor.kind !== 'event' || editor.index < 0 || !confirm('确定删除该活动及全部里程碑奖励吗？')) return
  saving.value = true
  try {
    const key = editor.draft.key as string
    await deleteEventBundle(key)
    events.value.splice(editor.index, 1)
    rewards.value = rewards.value.filter((row) => row.event_key !== key)
    editor.kind = null
    editorSnapshot.value = ''
    toast.success('活动与关联奖励已删除')
  } catch (reason) {
    toast.error(reason instanceof Error ? reason.message : '活动删除失败')
  } finally {
    saving.value = false
  }
}

function commitEditor() {
  if (!editor.kind || editor.kind === 'event') return
  const collection = editor.kind === 'pet' ? pets.value
    : editor.kind === 'item' ? items.value
      : editor.kind === 'shop' ? shops.value
        : editor.kind === 'image' ? images.value
          : editor.kind === 'command' ? commands.value
            : menus.value
  if (editor.kind === 'command' && !editor.draft.DisplayName) editor.draft.DisplayName = editor.draft.Command
  if (editor.index < 0) collection.push(cloneConfigValue(editor.draft))
  else collection.splice(editor.index, 1, cloneConfigValue(editor.draft))
  editor.kind = null
  editorSnapshot.value = ''
}

async function deleteCurrentConfig() {
  if (!editor.kind || editor.kind === 'event' || editor.index < 0 || !confirm('确定删除当前配置吗？')) return
  const kind = editor.kind
  const collection = kind === 'pet' ? pets.value : kind === 'image' ? images.value : kind === 'command' ? commands.value : menus.value
  const type = kind === 'pet' ? 'pet_species' : kind === 'image' ? 'image' : kind === 'command' ? 'command' : 'menu'
  const key = kind === 'pet' || kind === 'image' ? editor.draft.Name : kind === 'command' ? editor.draft.FuncName : editor.draft.Name
  saving.value = true
  try {
    await deleteConfigItem(type, key)
    collection.splice(editor.index, 1)
    editor.kind = null
    editorSnapshot.value = ''
    syncSnapshot()
    toast.success('配置已删除')
  } catch (reason) {
    toast.error(reason instanceof Error ? reason.message : '删除失败')
  } finally {
    saving.value = false
  }
}

function toggleAllItems(checked: boolean) {
  selectedItems.value = checked ? visibleItems.value.map((row) => row.Name) : []
}

function toggleAllShops(checked: boolean) {
  selectedShops.value = checked ? visibleShops.value.map((row) => row.ID).filter(Boolean) : []
}

async function applyItemBulk(action: 'delete' | 'set_status') {
  if (!selectedItems.value.length) return toast.warning('请先选择物品')
  if (action === 'delete' && !confirm('确定删除所选物品吗？被引用的物品会由后端阻止删除。')) return
  saving.value = true
  try {
    const result = await bulkItems(selectedItems.value, action, action === 'set_status' ? bulkItemStatus.value : undefined)
    items.value = result.items
    selectedItems.value = []
    syncSnapshot()
    toast.success(action === 'delete' ? '所选物品已删除' : '物品状态已批量更新')
  } catch (reason) {
    toast.error(reason instanceof Error ? reason.message : '批量更新物品失败')
  } finally {
    saving.value = false
  }
}

async function applyShopBulk(action: 'delete' | 'restock' | 'set_target') {
  if (!selectedShops.value.length) return toast.warning('请先选择商品')
  if (action === 'delete' && !confirm('确定删除所选商品吗？')) return
  saving.value = true
  try {
    const result = await bulkShopItems(selectedShops.value, action, action === 'set_target' ? bulkShopTarget.value : undefined)
    shops.value = result.items
    selectedShops.value = []
    syncSnapshot()
    toast.success(action === 'restock' ? '所选商品已补货' : action === 'set_target' ? '目标库存已批量更新' : '所选商品已删除')
  } catch (reason) {
    toast.error(reason instanceof Error ? reason.message : '批量更新商品失败')
  } finally {
    saving.value = false
  }
}

async function saveAssets() {
  saving.value = true
  try {
    if (assetTab.value === 'pets') await saveConfig('pet_species', pets.value)
    if (assetTab.value === 'items') await saveConfig('items', items.value)
    if (assetTab.value === 'shop') await saveConfig('shop_items', shops.value)
    if (assetTab.value === 'images') await saveConfig('images', images.value)
    status.value = await fetchConfigStatus()
    syncSnapshot()
    toast.success('当前配置已保存')
  } catch (reason) {
    toast.error(reason instanceof Error ? reason.message : '保存失败')
  } finally {
    saving.value = false
  }
}

async function uploadAsset(event: Event, intoEditor = false) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  saving.value = true
  try {
    const result = await uploadImage(file)
    if (intoEditor && editor.kind === 'image') {
      editor.draft.Path = result.path
      if (!editor.draft.Name) editor.draft.Name = file.name.replace(/\.[^.]+$/, '')
    } else {
      images.value.push({ Name: file.name.replace(/\.[^.]+$/, ''), Path: result.path })
    }
    toast.success('图片已上传，请确认名称后保存配置')
  } catch (reason) {
    toast.error(reason instanceof Error ? reason.message : '图片上传失败')
  } finally {
    saving.value = false
    input.value = ''
  }
}

async function copyImagePath(path: string) {
  try {
    await navigator.clipboard.writeText(path)
    toast.success('图片路径已复制')
  } catch {
    toast.error('无法访问剪贴板，请手动复制路径')
  }
}

async function saveText() {
  saving.value = true
  try {
    if (textTab.value === 'commands') await saveConfig('commands', commands.value)
    else await saveConfig('menus', menus.value)
    status.value = await fetchConfigStatus()
    syncSnapshot()
    toast.success('文本与命令已保存')
  } catch (reason) {
    toast.error(reason instanceof Error ? reason.message : '保存失败')
  } finally {
    saving.value = false
  }
}

async function saveGame() {
  saving.value = true
  try {
    gameSettings.value = await saveGameSettings(gameSettings.value.map((row) => ({ key: row.key, value: row.value })))
    status.value = await fetchConfigStatus()
    syncSnapshot()
    toast.success('游戏参数已保存')
  } catch (reason) {
    toast.error(reason instanceof Error ? reason.message : '游戏参数保存失败')
  } finally {
    saving.value = false
  }
}

function updateListSetting(row: GameSettingRow, event: Event) {
  row.value = (event.target as HTMLInputElement).value
    .split(/[,，、]/)
    .map((value) => value.trim())
    .filter(Boolean)
}

function listSettingText(row: GameSettingRow) {
  return Array.isArray(row.value) ? row.value.join('，') : String(row.value ?? '')
}

function setTab(value: MainTab) {
  if (hasUnsavedChanges.value && !confirm('当前页面有未保存修改，确定切换吗？')) return
  tab.value = value
  search.value = ''
  router.replace({ query: { tab: value } })
}

function setAssetTab(value: AssetTab) {
  assetTab.value = value
  search.value = ''
}

function setTextTab(value: TextTab) {
  textTab.value = value
  search.value = ''
}

function syncSnapshot() {
  snapshot.value = JSON.stringify(editableState.value)
}

async function load() {
  loading.value = true
  try {
    const [eventRows, rewardRows, petRows, itemRows, shopRows, imageRows, commandRows, menuRows, settings, currentStatus] = await Promise.all([
      fetchConfig('live_events'),
      fetchConfig('reward_tracks'),
      fetchConfig('pet_species'),
      fetchConfig('items'),
      fetchConfig('shop_items'),
      fetchConfig('images'),
      fetchConfig('commands'),
      fetchConfig('menus'),
      fetchGameSettings(),
      fetchConfigStatus(),
    ])
    events.value = eventRows as ContentEventRow[]
    rewards.value = rewardRows as ContentRewardRow[]
    pets.value = petRows.map(normalizePetSpecies)
    items.value = itemRows.map(normalizeItem)
    shops.value = shopRows.map(normalizeShopItem)
    images.value = imageRows.map(normalizeImage)
    commands.value = commandRows.map(normalizeCommand)
    menus.value = menuRows.map(normalizeMenu)
    gameSettings.value = settings
    status.value = currentStatus
    selectedItems.value = []
    selectedShops.value = []
    syncSnapshot()
  } catch (reason) {
    toast.error(reason instanceof Error ? reason.message : '内容工作台加载失败')
  } finally {
    loading.value = false
  }
}

async function reload() {
  try {
    await reloadConfigs()
    status.value = await fetchConfigStatus()
    toast.success('配置已重载生效')
  } catch (reason) {
    toast.error(reason instanceof Error ? reason.message : '重载失败')
  }
}

onMounted(load)
</script>

<template>
  <section class="content-page">
    <PageHeader
      eyebrow="Content Studio"
      title="内容工作台"
      description="把活动、宠物资产、玩家文案与游戏规则放在同一条运营链路中维护。"
    >
      <template #actions>
        <button class="btn btn-ghost" :disabled="loading || saving" @click="load"><IconRefresh :size="17" />刷新数据</button>
        <button class="btn btn-ghost" :disabled="loading || saving" @click="reload"><IconReload :size="17" />重载生效</button>
      </template>
    </PageHeader>

    <nav class="page-tabs" aria-label="内容工作台导航">
      <button :class="{ active: tab === 'events' }" @click="setTab('events')"><IconCalendarEvent :size="17" />活动运营</button>
      <button :class="{ active: tab === 'assets' }" @click="setTab('assets')"><IconPackage :size="17" />宠物与物品</button>
      <button :class="{ active: tab === 'text' }" @click="setTab('text')"><IconMessageCircle :size="17" />文本与命令</button>
      <button :class="{ active: tab === 'game' }" @click="setTab('game')"><IconAdjustments :size="17" />游戏参数</button>
    </nav>

    <div v-if="status" class="revision-bar" :class="{ pending: status.pending_reload }">
      <span><i />{{ status.pending_reload ? '有配置等待重载' : '当前配置已生效' }}</span>
      <small>数据库版本 {{ status.db_revision }} · 已加载 {{ status.loaded_revision }}</small>
    </div>

    <UiState v-if="loading" tone="loading" title="正在准备内容工作台" description="活动、物品与游戏参数正在并行读取。" />

    <template v-else-if="tab === 'events'">
      <div class="section-heading">
        <div><span class="section-kicker">活动排期与奖励</span><h2>活动运营</h2><p>一个抽屉完成故事分支、里程碑和奖励配置，保存时由后端事务统一提交。</p></div>
        <button class="btn btn-primary" @click="openEvent()"><IconPlus :size="17" />新建活动</button>
      </div>
      <div class="toolbar">
        <label class="searchbox"><IconSearch :size="17" /><input v-model="search" placeholder="搜索活动名称或区域" /></label>
        <span>{{ visibleEvents.length }} 场活动 · {{ rewards.length }} 条物品奖励</span>
      </div>
      <UiState v-if="visibleEvents.length === 0" title="还没有活动" description="创建第一场活动，并在同一抽屉中配置奖励。" action-label="新建活动" @action="openEvent()" />
      <div v-else class="event-list">
        <article v-for="row in visibleEvents" :key="row.key" class="event-card">
          <div class="event-date"><strong>{{ new Date(row.starts_at).getDate() }}</strong><span>{{ new Date(row.starts_at).toLocaleDateString('zh-CN', { month: 'short' }) }}</span></div>
          <div class="event-copy"><div class="title-line"><h3>{{ row.name }}</h3><span class="status-mark" :data-status="eventStatus(row)">{{ eventStatus(row) }}</span></div><p>{{ row.region }}区域 · {{ formatDate(row.starts_at) }} 至 {{ formatDate(row.ends_at) }}</p></div>
          <div class="event-metric"><span>奖励节点</span><strong>{{ eventRewardCount(row.key) }}</strong></div>
          <button class="btn btn-ghost" @click="openEvent(row)">编辑活动<IconChevronRight :size="16" /></button>
        </article>
      </div>
    </template>

    <template v-else-if="tab === 'assets'">
      <div class="section-heading compact">
        <div><span class="section-kicker">可运营资产</span><h2>宠物与物品</h2><p>维护宠物资料、物品可见性与商店补货策略。</p></div>
        <button class="btn btn-primary" :disabled="saving || !dirty" @click="saveAssets"><IconDeviceFloppy :size="17" />保存当前配置</button>
      </div>
      <nav class="sub-tabs" aria-label="宠物与物品分类">
        <button :class="{ active: assetTab === 'pets' }" @click="setAssetTab('pets')"><IconPaw :size="16" />宠物</button>
        <button :class="{ active: assetTab === 'items' }" @click="setAssetTab('items')"><IconBox :size="16" />物品</button>
        <button :class="{ active: assetTab === 'shop' }" @click="setAssetTab('shop')"><IconShoppingBag :size="16" />商店</button>
        <button :class="{ active: assetTab === 'images' }" @click="setAssetTab('images')"><IconPhoto :size="16" />图片资产</button>
      </nav>
      <div class="toolbar">
        <label class="searchbox"><IconSearch :size="17" /><input v-model="search" :placeholder="assetTab === 'pets' ? '搜索宠物' : assetTab === 'items' ? '搜索物品' : assetTab === 'shop' ? '搜索商品' : '搜索图片名称或路径'" /></label>
        <div class="toolbar-actions"><label v-if="assetTab === 'images'" class="btn btn-ghost upload-button"><IconPhoto :size="16" />上传图片<input type="file" accept="image/png,image/jpeg,image/gif,image/webp" @change="uploadAsset($event)" /></label><button class="btn btn-ghost" @click="openEditor(assetTab === 'pets' ? 'pet' : assetTab === 'items' ? 'item' : assetTab === 'shop' ? 'shop' : 'image')"><IconPlus :size="16" />新增</button></div>
      </div>

      <div v-if="assetTab === 'pets'" class="pet-grid">
        <article v-for="row in visiblePets" :key="row.Name" class="pet-card" @click="openEditor('pet', row)">
          <div class="pet-avatar"><img v-if="row.Image" :src="row.Image" :alt="`${row.Name}形象`" /><IconPaw v-else :size="28" /></div>
          <div><h3>{{ row.Name }}</h3><p>{{ row.Description || '尚未填写宠物介绍' }}</p><small>偏爱食物：{{ row.FavoriteFood || '未设置' }}</small></div>
          <IconChevronRight :size="18" />
        </article>
      </div>

      <template v-else-if="assetTab === 'items'">
        <div class="bulk-bar">
          <label class="check-all"><input type="checkbox" :checked="selectedItems.length === visibleItems.length && visibleItems.length > 0" @change="toggleAllItems(($event.target as HTMLInputElement).checked)" />选择全部</label>
          <span>已选 {{ selectedItems.length }} 项</span>
          <select v-model="bulkItemStatus"><option v-for="option in itemStatusOptions" :key="option.value" :value="option.value">{{ option.label }}</option></select>
          <button class="btn btn-ghost" :disabled="saving" @click="applyItemBulk('set_status')">批量设置状态</button>
          <button class="btn btn-danger" :disabled="saving" @click="applyItemBulk('delete')"><IconTrash :size="15" />批量删除</button>
        </div>
        <div class="responsive-list">
          <article v-for="row in visibleItems" :key="row.Name" class="asset-row">
            <input v-model="selectedItems" type="checkbox" :value="row.Name" :aria-label="`选择${row.Name}`" />
            <div class="asset-main"><strong>{{ row.Name }}</strong><small>{{ row.Type || '未分类' }} · {{ row.Description || '暂无说明' }}</small></div>
            <span class="status-mark" :data-status="row.Status">{{ itemStatusLabel(row.Status) }}</span>
            <span class="price-copy">回收价 <b>{{ row.SellPrice }}</b></span>
            <button class="btn btn-ghost btn-small" @click="openEditor('item', row)">详情</button>
          </article>
        </div>
      </template>

      <template v-else-if="assetTab === 'shop'">
        <div class="bulk-bar">
          <label class="check-all"><input type="checkbox" :checked="selectedShops.length === visibleShops.length && visibleShops.length > 0" @change="toggleAllShops(($event.target as HTMLInputElement).checked)" />选择全部</label>
          <span>已选 {{ selectedShops.length }} 项</span>
          <button class="btn btn-ghost" :disabled="saving" @click="applyShopBulk('restock')">一键补货</button>
          <input v-model.number="bulkShopTarget" type="number" min="-1" aria-label="批量目标库存" />
          <button class="btn btn-ghost" :disabled="saving" @click="applyShopBulk('set_target')">设置目标</button>
          <button class="btn btn-danger" :disabled="saving" @click="applyShopBulk('delete')"><IconTrash :size="15" />批量删除</button>
        </div>
        <div class="responsive-list shop-list">
          <article v-for="row in visibleShops" :key="row.ID || row.Name" class="asset-row shop-row">
            <input v-model="selectedShops" type="checkbox" :value="row.ID" :aria-label="`选择${row.Name}`" />
            <div class="asset-main"><strong>{{ row.Name }}</strong><small>{{ shopTypeLabel(row.ShopType) }} · {{ row.Description || '暂无说明' }}</small></div>
            <label><span>价格</span><input v-model.number="row.Price" type="number" min="0" :aria-label="`${row.Name}价格`" /></label>
            <label><span>库存</span><input v-model.number="row.Stock" type="number" min="-1" :aria-label="`${row.Name}库存`" /></label>
            <label><span>目标库存</span><input v-model.number="row.RestockTarget" type="number" min="-1" :aria-label="`${row.Name}目标库存`" /></label>
            <button class="btn btn-ghost btn-small" @click="openEditor('shop', row)">详情</button>
          </article>
        </div>
      </template>
      <div v-else class="image-grid">
        <article v-for="row in visibleImages" :key="row.Name" class="image-card">
          <div class="image-preview"><img v-if="row.Path" :src="imagePreviewUrl(row.Path)" :alt="row.Name" /><IconPhoto v-else :size="30" /></div>
          <div><strong>{{ row.Name }}</strong><small>{{ row.Path || '尚未选择图片' }}</small></div>
          <div class="image-actions"><button class="btn btn-ghost btn-small" @click="copyImagePath(row.Path)">复制路径</button><button class="btn btn-ghost btn-small" @click="openEditor('image', row)">编辑</button></div>
        </article>
      </div>
    </template>

    <template v-else-if="tab === 'text'">
      <div class="section-heading compact">
        <div><span class="section-kicker">玩家看到的语言</span><h2>文本与命令</h2><p>按中文业务分类维护命令，并用聊天气泡检查菜单回复。</p></div>
        <button class="btn btn-primary" :disabled="saving || !dirty" @click="saveText"><IconDeviceFloppy :size="17" />保存当前配置</button>
      </div>
      <nav class="sub-tabs" aria-label="文本与命令分类">
        <button :class="{ active: textTab === 'commands' }" @click="setTextTab('commands')"><IconBolt :size="16" />命令</button>
        <button :class="{ active: textTab === 'menus' }" @click="setTextTab('menus')"><IconMessageCircle :size="16" />菜单场景</button>
      </nav>
      <div class="toolbar">
        <label class="searchbox"><IconSearch :size="17" /><input v-model="search" :placeholder="textTab === 'commands' ? '搜索名称、触发词或分类' : '搜索场景或回复内容'" /></label>
        <button class="btn btn-ghost" @click="openEditor(textTab === 'commands' ? 'command' : 'menu')"><IconPlus :size="16" />新增</button>
      </div>

      <div v-if="textTab === 'commands'" class="command-groups">
        <section v-for="group in commandGroups" :key="group.category" class="command-group">
          <header><div><span>{{ group.rows.length }} 条命令</span><h3>{{ group.category }}</h3></div></header>
          <button v-for="row in group.rows" :key="row.FuncName" class="command-row" @click="openEditor('command', row)">
            <span class="command-icon"><IconBolt :size="18" /></span>
            <span class="command-copy"><strong>{{ row.DisplayName || row.Command }}</strong><small>{{ row.Description || '暂无使用说明' }}</small></span>
            <span class="command-trigger">发送“{{ row.Command }}”</span>
            <span class="status-mark" :data-status="row.Enabled ? 'active' : 'disabled'">{{ row.Enabled ? '已启用' : '已停用' }}</span>
            <IconChevronRight :size="17" />
          </button>
        </section>
      </div>

      <div v-else class="menu-grid">
        <article v-for="row in visibleMenus" :key="row.Name" class="menu-card">
          <header><div><span>QQ 菜单场景</span><h3>{{ menuScene(row.Name) }}</h3></div><button class="btn btn-ghost btn-small" @click="openEditor('menu', row)">编辑</button></header>
          <div class="qq-preview"><div class="qq-avatar"><IconPaw :size="19" /></div><div class="qq-message">{{ row.Reply || '这里会显示机器人发送给玩家的菜单回复。' }}</div></div>
        </article>
      </div>
    </template>

    <template v-else>
      <div class="section-heading compact">
        <div><span class="section-kicker">核心规则</span><h2>游戏参数</h2><p>所有参数使用中文名称与类型化控件，不直接暴露内部配置键。</p></div>
        <button class="btn btn-primary" :disabled="saving || !dirty" @click="saveGame"><IconDeviceFloppy :size="17" />保存游戏参数</button>
      </div>
      <div class="game-groups">
        <section v-for="group in gameGroups" :key="group.group" class="game-group">
          <header><span>{{ group.rows.length }} 项</span><h3>{{ group.group }}</h3></header>
          <div class="setting-grid">
            <article v-for="row in group.rows" :key="row.key" class="setting-card">
              <div class="setting-copy"><strong>{{ row.label }}</strong><p>{{ row.description }}</p></div>
              <label v-if="row.type === 'boolean'" class="switch-control">
                <input v-model="row.value" type="checkbox" />
                <span class="switch-track"><i /></span>
                <b>{{ row.value ? '已开启' : '已关闭' }}</b>
              </label>
              <label v-else-if="row.type === 'number'" class="number-control"><input v-model.number="row.value" type="number" /><span v-if="row.unit">{{ row.unit }}</span></label>
              <div v-else-if="row.type === 'list'" class="list-control">
                <div class="tag-list"><span v-for="value in Array.isArray(row.value) ? row.value : []" :key="value">{{ value }}</span></div>
                <input :value="listSettingText(row)" placeholder="使用逗号分隔多个选项" @input="updateListSetting(row, $event)" />
              </div>
              <input v-else v-model="row.value" class="text-control" />
            </article>
          </div>
        </section>
      </div>
    </template>

    <UiDrawer :open="editor.kind !== null" :title="editorTitle" :description="editor.kind === 'event' ? '活动和奖励将在一次请求中保存。' : '修改后返回列表检查并保存当前配置。'" :busy="saving" @close="closeEditor">
      <template v-if="editor.kind === 'event' && editor.draft">
        <div class="drawer-help"><IconSparkles :size="19" /><div><strong>使用说明</strong><p>先设置活动时间与故事选项，再按里程碑添加一个或多个物品。累计预览会展示玩家到达每个节点时已获得的总奖励。</p></div><button class="btn btn-ghost btn-small" @click="fillEventSample">填充测试示例</button></div>
        <div class="form-grid two">
          <label><span>活动键</span><input v-model="editor.draft.key" :disabled="editor.index >= 0" placeholder="例如 forest-week" /></label>
          <label><span>活动名称</span><input v-model="editor.draft.name" /></label>
          <label><span>活动区域</span><input v-model="editor.draft.region" /></label>
          <label class="switch-field"><span>运营状态</span><input v-model="editor.draft.active" type="checkbox" /><b>{{ editor.draft.active ? '启用活动' : '暂不启用' }}</b></label>
          <label><span>开始时间</span><input v-model="editor.draft.starts_at" type="datetime-local" /></label>
          <label><span>结束时间</span><input v-model="editor.draft.ends_at" type="datetime-local" /></label>
        </div>
        <section class="drawer-section"><header><div><span>玩家分支</span><h3>故事选项</h3></div><button class="text-button" @click="editor.draft.choices.push('')"><IconPlus :size="15" />添加选项</button></header><div class="choice-list"><label v-for="(_, index) in editor.draft.choices" :key="index"><b>{{ Number(index) + 1 }}</b><input v-model="editor.draft.choices[index]" /><button v-if="editor.draft.choices.length > 2" aria-label="删除故事选项" @click="editor.draft.choices.splice(Number(index), 1)"><IconTrash :size="15" /></button></label></div></section>
        <section class="drawer-section rewards-editor"><header><div><span>事务保存</span><h3>里程碑奖励</h3></div><button class="text-button" @click="addRewardGroup"><IconPlus :size="15" />添加里程碑</button></header>
          <article v-for="(group, groupIndex) in editor.draft.rewardGroups" :key="groupIndex" class="reward-group">
            <header><strong>里程碑 {{ group.milestone }}</strong><button aria-label="删除里程碑" @click="removeRewardGroup(groupIndex)"><IconTrash :size="16" /></button></header>
            <div class="reward-meta"><label><span>达成数值</span><input v-model.number="group.milestone" type="number" min="1" /></label><label><span>节点说明</span><input v-model="group.description" /></label></div>
            <div class="reward-items"><div v-for="(item, itemIndex) in group.items" :key="itemIndex"><select v-model="item.item_name"><option value="">选择物品</option><option v-for="option in items" :key="option.Name" :value="option.Name">{{ option.Name }}</option></select><input v-model.number="item.quantity" type="number" min="1" aria-label="奖励数量" /><button v-if="group.items.length > 1" aria-label="删除奖励物品" @click="group.items.splice(itemIndex, 1)"><IconTrash :size="15" /></button></div><button class="text-button" @click="addRewardItem(group)"><IconPlus :size="14" />同里程碑添加物品</button></div>
            <div class="cumulative-preview"><span>累计奖励预览</span><div><b v-for="preview in cumulativeRewards(groupIndex)" :key="preview">{{ preview }}</b></div></div>
          </article>
        </section>
      </template>

      <div v-else-if="editor.kind === 'pet' && editor.draft" class="form-grid">
        <label><span>宠物名称</span><input v-model="editor.draft.Name" :disabled="editor.index >= 0" /></label><label><span>形象路径</span><input v-model="editor.draft.Image" /></label><label><span>偏爱食物</span><input v-model="editor.draft.FavoriteFood" /></label><label><span>偏爱礼物</span><input v-model="editor.draft.FavoriteGift" /></label><label><span>宠物介绍</span><textarea v-model="editor.draft.Description" rows="4" /></label>
      </div>
      <div v-else-if="editor.kind === 'item' && editor.draft" class="form-grid">
        <label><span>物品名称</span><input v-model="editor.draft.Name" :disabled="editor.index >= 0" /></label><label><span>运营状态</span><select v-model="editor.draft.Status"><option v-for="option in itemStatusOptions" :key="option.value" :value="option.value">{{ option.label }}</option></select></label><label><span>物品类型</span><input v-model="editor.draft.Type" /></label><label><span>回收价格</span><input v-model.number="editor.draft.SellPrice" type="number" min="0" /></label><label><span>形象路径</span><input v-model="editor.draft.Image" /></label><label><span>物品说明</span><textarea v-model="editor.draft.Description" rows="4" /></label>
      </div>
      <div v-else-if="editor.kind === 'shop' && editor.draft" class="form-grid">
        <label><span>商品名称</span><input v-model="editor.draft.Name" /></label><label><span>商店类型</span><select v-model="editor.draft.ShopType"><option value="shop_normal">普通商店</option><option value="shop_affection">羁绊商店</option></select></label><label><span>价格</span><input v-model.number="editor.draft.Price" type="number" min="0" /></label><label><span>当前库存</span><input v-model.number="editor.draft.Stock" type="number" min="-1" /></label><label><span>目标库存</span><input v-model.number="editor.draft.RestockTarget" type="number" min="-1" /></label><label><span>商品说明</span><textarea v-model="editor.draft.Description" rows="4" /></label>
      </div>
      <div v-else-if="editor.kind === 'image' && editor.draft" class="form-grid">
        <label><span>图片名称</span><input v-model="editor.draft.Name" :disabled="editor.index >= 0" /></label><label><span>图片路径</span><input v-model="editor.draft.Path" /></label><label class="upload-field"><span>上传或替换</span><input type="file" accept="image/png,image/jpeg,image/gif,image/webp" @change="uploadAsset($event, true)" /></label><div class="image-preview large"><img v-if="editor.draft.Path" :src="imagePreviewUrl(editor.draft.Path)" :alt="editor.draft.Name" /><IconPhoto v-else :size="38" /></div>
      </div>
      <div v-else-if="editor.kind === 'command' && editor.draft" class="form-grid">
        <label><span>显示名称</span><input v-model="editor.draft.DisplayName" /></label><label><span>中文分类</span><input v-model="editor.draft.Category" /></label><label><span>玩家触发词</span><input v-model="editor.draft.Command" /></label><label><span>排序</span><input v-model.number="editor.draft.SortOrder" type="number" /></label><label class="switch-field"><span>命令状态</span><input v-model="editor.draft.Enabled" type="checkbox" /><b>{{ editor.draft.Enabled ? '启用命令' : '停用命令' }}</b></label><label><span>使用说明</span><textarea v-model="editor.draft.Description" rows="4" /></label><details class="technical-field"><summary>高级设置</summary><label><span>稳定功能标识</span><input v-model="editor.draft.FuncName" :disabled="editor.index >= 0" /></label></details>
      </div>
      <div v-else-if="editor.kind === 'menu' && editor.draft" class="form-grid">
        <label><span>场景标识</span><input v-model="editor.draft.Name" :disabled="editor.index >= 0" /></label><label><span>机器人回复</span><textarea v-model="editor.draft.Reply" rows="10" /></label><div class="qq-preview large"><div class="qq-avatar"><IconPaw :size="19" /></div><div class="qq-message">{{ editor.draft.Reply || '输入文案后在这里检查 QQ 消息效果。' }}</div></div>
      </div>

      <template #footer>
        <div class="drawer-actions">
          <button v-if="editor.index >= 0 && ['event', 'pet', 'image', 'command', 'menu'].includes(String(editor.kind))" class="btn btn-danger" :disabled="saving" @click="editor.kind === 'event' ? removeCurrentEvent() : deleteCurrentConfig()"><IconTrash :size="16" />删除</button>
          <span />
          <button class="btn btn-ghost" :disabled="saving" @click="closeEditor">取消</button>
          <button v-if="editor.kind === 'event'" class="btn btn-primary" :disabled="saving" @click="saveCurrentEvent"><IconCheck :size="16" />{{ saving ? '保存中' : '一次保存活动与奖励' }}</button>
          <button v-else class="btn btn-primary" :disabled="saving" @click="commitEditor"><IconCheck :size="16" />应用到列表</button>
        </div>
      </template>
    </UiDrawer>
  </section>
</template>

<style scoped>
.content-page{display:grid;gap:16px;max-width:1440px;margin:0 auto;padding-bottom:48px}.page-tabs button{display:inline-flex;align-items:center;gap:7px}.revision-bar{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:10px 14px;border:1px solid color-mix(in srgb,var(--success) 45%,var(--border-color));border-radius:var(--radius-input);background:var(--success-soft);color:var(--success-strong)}.revision-bar.pending{border-color:color-mix(in srgb,var(--warning) 45%,var(--border-color));background:var(--warning-soft);color:var(--warning-strong)}.revision-bar span{display:flex;align-items:center;gap:8px;font-weight:600}.revision-bar i{width:7px;height:7px;border-radius:50%;background:currentColor}.revision-bar small{color:var(--text-muted)}.section-heading{display:flex;align-items:end;justify-content:space-between;gap:24px;padding:24px 0 6px}.section-heading.compact{padding-top:10px}.section-heading h2{margin:2px 0 4px;font-size:clamp(24px,3vw,34px);line-height:1.15;letter-spacing:-.03em}.section-heading p{max-width:680px;margin:0;color:var(--text-muted)}.section-kicker{color:var(--accent);font-size:12px;font-weight:700;letter-spacing:.08em}.toolbar{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:12px 14px;border:1px solid var(--border-color);border-radius:var(--radius-card);background:var(--bg-surface)}.toolbar>span{color:var(--text-muted);font-size:12px}.searchbox{display:flex;align-items:center;gap:8px;min-width:min(420px,100%);padding:0 11px;border:1px solid var(--border-color);border-radius:var(--radius-input);background:var(--bg-base);color:var(--text-muted)}.searchbox input{width:100%;min-height:38px;border:0;outline:0;background:transparent;color:var(--text-main);font:inherit}.event-list{display:grid;gap:10px}.event-card{display:grid;grid-template-columns:auto minmax(0,1fr) auto auto;align-items:center;gap:18px;padding:16px;border:1px solid var(--border-color);border-radius:var(--radius-card);background:var(--bg-surface);box-shadow:var(--shadow-card);transition:transform .2s var(--ease-out),border-color .2s ease}.event-card:hover{transform:translateY(-1px);border-color:var(--border-strong)}.event-date{display:grid;place-items:center;width:58px;height:64px;border-radius:12px;background:var(--accent-soft);color:var(--accent)}.event-date strong{font-size:24px;line-height:1}.event-date span{font-size:11px}.title-line{display:flex;align-items:center;gap:9px}.title-line h3,.pet-card h3,.menu-card h3,.command-group h3,.game-group h3{margin:0}.event-copy p{margin:4px 0 0;color:var(--text-muted);font-size:12px}.event-metric{display:grid;text-align:center}.event-metric span{color:var(--text-muted);font-size:11px}.event-metric strong{font-size:22px;font-variant-numeric:tabular-nums}.status-mark{display:inline-flex;width:max-content;align-items:center;padding:3px 8px;border-radius:6px;background:var(--bg-elevated);color:var(--text-muted);font-size:11px;font-weight:700}.status-mark[data-status='进行中'],.status-mark[data-status='active']{background:var(--success-soft);color:var(--success-strong)}.status-mark[data-status='待开始'],.status-mark[data-status='limited']{background:var(--warning-soft);color:var(--warning-strong)}.status-mark[data-status='hidden']{background:var(--accent-soft);color:var(--accent)}.status-mark[data-status='disabled'],.status-mark[data-status='已停用']{background:var(--danger-soft);color:var(--danger)}.sub-tabs{display:flex;gap:6px}.sub-tabs button{display:inline-flex;align-items:center;gap:6px;padding:8px 13px;border:1px solid transparent;border-radius:var(--radius-btn);background:transparent;color:var(--text-muted);font:inherit;cursor:pointer}.sub-tabs button.active{border-color:var(--border-color);background:var(--bg-surface);color:var(--text-main);box-shadow:var(--shadow-card)}.pet-grid,.menu-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.pet-card{display:grid;grid-template-columns:auto minmax(0,1fr) auto;align-items:center;gap:14px;padding:16px;border:1px solid var(--border-color);border-radius:var(--radius-card);background:var(--bg-surface);color:inherit;text-align:left;cursor:pointer}.pet-card p{margin:3px 0;color:var(--text-muted)}.pet-card small{color:var(--text-muted)}.pet-avatar{display:grid;place-items:center;width:54px;height:54px;overflow:hidden;border-radius:15px;background:var(--accent-soft);color:var(--accent)}.pet-avatar img{width:100%;height:100%;object-fit:cover}.bulk-bar{display:flex;align-items:center;gap:9px;flex-wrap:wrap;padding:12px 14px;border:1px solid var(--border-color);border-radius:var(--radius-card);background:var(--bg-elevated)}.bulk-bar>span{margin-right:auto;color:var(--text-muted)}.bulk-bar select,.bulk-bar>input{min-height:36px;padding:6px 9px;border:1px solid var(--border-color);border-radius:var(--radius-input);background:var(--bg-base);color:var(--text-main)}.check-all{display:flex;align-items:center;gap:7px;font-weight:600}.responsive-list{display:grid;gap:8px}.asset-row{display:grid;grid-template-columns:auto minmax(190px,1fr) auto auto auto;align-items:center;gap:14px;padding:13px 15px;border:1px solid var(--border-color);border-radius:12px;background:var(--bg-surface)}.asset-main{display:grid;min-width:0}.asset-main small{overflow:hidden;color:var(--text-muted);text-overflow:ellipsis;white-space:nowrap}.price-copy{color:var(--text-muted);font-size:12px}.price-copy b{color:var(--text-main);font-size:15px}.shop-row{grid-template-columns:auto minmax(180px,1fr) repeat(3,minmax(90px,120px)) auto}.shop-row label{display:grid;gap:3px}.shop-row label span{color:var(--text-muted);font-size:10px}.shop-row label input{width:100%;min-height:34px;padding:5px 8px;border:1px solid var(--border-color);border-radius:7px;background:var(--bg-base);color:var(--text-main)}.command-groups,.game-groups{display:grid;gap:16px}.command-group,.game-group{overflow:hidden;border:1px solid var(--border-color);border-radius:var(--radius-card);background:var(--bg-surface)}.command-group>header,.game-group>header{padding:14px 16px;background:var(--bg-elevated)}.command-group header span,.game-group header span{color:var(--text-muted);font-size:11px}.command-row{display:grid;width:100%;grid-template-columns:auto minmax(0,1fr) auto auto auto;align-items:center;gap:14px;padding:13px 16px;border:0;border-top:1px solid var(--border-color);background:transparent;color:inherit;text-align:left;cursor:pointer}.command-row:hover{background:var(--bg-hover)}.command-icon{display:grid;place-items:center;width:34px;height:34px;border-radius:9px;background:var(--accent-soft);color:var(--accent)}.command-copy{display:grid}.command-copy small{color:var(--text-muted)}.command-trigger{padding:5px 8px;border-radius:7px;background:var(--bg-base);color:var(--text-muted);font-size:12px}.menu-card{overflow:hidden;border:1px solid var(--border-color);border-radius:var(--radius-card);background:var(--bg-surface)}.menu-card>header{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:14px 16px;border-bottom:1px solid var(--border-color)}.menu-card header span{color:var(--text-muted);font-size:10px}.qq-preview{display:flex;align-items:flex-start;gap:10px;min-height:150px;padding:18px;background:color-mix(in srgb,var(--bg-base) 84%,#95afc7)}.qq-preview.large{min-height:190px;border-radius:12px}.qq-avatar{display:grid;flex:0 0 auto;place-items:center;width:38px;height:38px;border-radius:11px;background:var(--accent);color:var(--accent-ink)}.qq-message{position:relative;max-width:78%;padding:10px 12px;border-radius:3px 12px 12px;background:var(--bg-surface);box-shadow:var(--shadow-soft);white-space:pre-wrap}.setting-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:1px;background:var(--border-color)}.setting-card{display:grid;align-content:space-between;gap:16px;min-height:150px;padding:17px;background:var(--bg-surface)}.setting-copy p{margin:4px 0 0;color:var(--text-muted);font-size:12px}.switch-control{display:flex;align-items:center;gap:9px;cursor:pointer}.switch-control>input{position:absolute;opacity:0}.switch-track{position:relative;width:42px;height:24px;border-radius:12px;background:var(--border-strong);transition:background .2s}.switch-track i{position:absolute;top:3px;left:3px;width:18px;height:18px;border-radius:50%;background:var(--bg-surface);box-shadow:0 2px 6px rgba(0,0,0,.18);transition:transform .2s}.switch-control input:checked+.switch-track{background:var(--accent)}.switch-control input:checked+.switch-track i{transform:translateX(18px)}.switch-control input:focus-visible+.switch-track{outline:2px solid var(--accent);outline-offset:2px}.switch-control b{font-size:12px}.number-control{display:flex;align-items:center;width:min(220px,100%);border:1px solid var(--border-color);border-radius:var(--radius-input);background:var(--bg-base)}.number-control input{width:100%;min-width:0;padding:8px 10px;border:0;outline:0;background:transparent;color:var(--text-main);font:inherit;font-variant-numeric:tabular-nums}.number-control span{padding-right:10px;color:var(--text-muted);font-size:12px}.list-control{display:grid;gap:9px}.tag-list{display:flex;gap:6px;flex-wrap:wrap}.tag-list span{padding:3px 8px;border-radius:6px;background:var(--accent-soft);color:var(--accent);font-size:11px;font-weight:600}.list-control input,.text-control{width:100%;min-height:38px;padding:7px 9px;border:1px solid var(--border-color);border-radius:var(--radius-input);background:var(--bg-base);color:var(--text-main);font:inherit}.drawer-help{display:grid;grid-template-columns:auto minmax(0,1fr) auto;align-items:start;gap:11px;padding:14px;border-radius:12px;background:var(--accent-soft);color:var(--accent)}.drawer-help p{margin:3px 0 0;color:var(--text-muted);font-size:12px}.form-grid{display:grid;gap:14px;margin-top:16px}.form-grid.two{grid-template-columns:repeat(2,minmax(0,1fr))}.form-grid label,.reward-meta label{display:grid;gap:5px}.form-grid label>span,.reward-meta label>span{color:var(--text-muted);font-size:11px}.form-grid input,.form-grid select,.form-grid textarea,.reward-meta input,.reward-items input,.reward-items select{width:100%;min-height:40px;padding:8px 10px;border:1px solid var(--border-color);border-radius:var(--radius-input);background:var(--bg-base);color:var(--text-main);font:inherit}.form-grid textarea{resize:vertical}.switch-field{grid-template-columns:auto 1fr;align-items:center}.switch-field>span{grid-column:1/-1}.switch-field input{width:auto;min-height:auto}.switch-field b{font-size:12px}.drawer-section{display:grid;gap:11px;margin-top:22px}.drawer-section>header{display:flex;align-items:end;justify-content:space-between;gap:12px;padding-bottom:8px;border-bottom:1px solid var(--border-color)}.drawer-section h3{margin:0}.drawer-section header span{color:var(--text-muted);font-size:10px}.text-button{display:inline-flex;align-items:center;gap:5px;padding:5px;border:0;background:transparent;color:var(--accent);font:inherit;font-size:12px;font-weight:600;cursor:pointer}.choice-list{display:grid;gap:7px}.choice-list label{display:grid;grid-template-columns:28px 1fr auto;align-items:center;gap:7px}.choice-list label b{display:grid;place-items:center;width:28px;height:28px;border-radius:8px;background:var(--accent-soft);color:var(--accent)}.choice-list input{min-height:38px;padding:7px 9px;border:1px solid var(--border-color);border-radius:var(--radius-input);background:var(--bg-base);color:var(--text-main)}.choice-list button,.reward-group button,.reward-items button{border:0;background:transparent;color:var(--text-muted);cursor:pointer}.reward-group{display:grid;gap:10px;padding:13px;border:1px solid var(--border-color);border-radius:12px}.reward-group>header{display:flex;align-items:center;justify-content:space-between}.reward-meta{display:grid;grid-template-columns:130px 1fr;gap:9px}.reward-items{display:grid;gap:7px}.reward-items>div{display:grid;grid-template-columns:1fr 90px auto;gap:7px}.cumulative-preview{display:grid;gap:6px;padding:10px;border-radius:9px;background:var(--bg-elevated)}.cumulative-preview>span{color:var(--text-muted);font-size:10px}.cumulative-preview>div{display:flex;gap:6px;flex-wrap:wrap}.cumulative-preview b{padding:3px 7px;border-radius:6px;background:var(--bg-surface);font-size:11px}.technical-field{padding:11px;border:1px dashed var(--border-color);border-radius:10px}.technical-field summary{cursor:pointer;color:var(--text-muted)}.technical-field label{margin-top:10px}.drawer-actions{display:flex;align-items:center;gap:8px}.drawer-actions>span{flex:1}.btn-small{min-height:32px!important;padding:5px 9px!important;font-size:12px!important}
.toolbar-actions{display:flex;gap:8px}.upload-button{position:relative;overflow:hidden}.upload-button input{position:absolute;inset:0;opacity:0;cursor:pointer}.image-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}.image-card{display:grid;grid-template-columns:84px minmax(0,1fr);gap:12px;padding:13px;border:1px solid var(--border-color);border-radius:var(--radius-card);background:var(--bg-surface)}.image-card>div:nth-child(2){display:grid;align-content:center;min-width:0}.image-card small{overflow:hidden;color:var(--text-muted);text-overflow:ellipsis;white-space:nowrap}.image-preview{display:grid;place-items:center;min-height:76px;overflow:hidden;border-radius:11px;background:var(--bg-elevated);color:var(--text-muted)}.image-preview img{width:100%;height:100%;object-fit:cover}.image-preview.large{min-height:240px}.image-actions{display:flex;grid-column:1/-1;gap:7px}.upload-field input{padding:10px}
@media(max-width:900px){.shop-row{grid-template-columns:auto minmax(160px,1fr) repeat(3,86px) auto}.pet-grid,.menu-grid{grid-template-columns:1fr}.image-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}
@media(max-width:700px){.content-page{gap:12px}.section-heading{align-items:stretch;flex-direction:column}.section-heading>.btn{width:100%}.revision-bar,.toolbar{align-items:stretch;flex-direction:column}.searchbox{min-width:0;width:100%}.event-card{grid-template-columns:auto minmax(0,1fr)}.event-metric{display:none}.event-card>.btn{grid-column:1/-1;width:100%}.bulk-bar{align-items:stretch}.bulk-bar>*{width:100%}.responsive-list{gap:10px}.asset-row,.shop-row{grid-template-columns:auto minmax(0,1fr);align-items:start}.asset-row>.status-mark,.asset-row>.price-copy,.asset-row>.btn,.shop-row>label{grid-column:2}.shop-row>label{display:grid;grid-template-columns:90px minmax(0,1fr);align-items:center}.command-row{grid-template-columns:auto minmax(0,1fr) auto}.command-trigger{grid-column:2}.command-row>.status-mark{grid-column:2}.setting-grid{grid-template-columns:1fr}.form-grid.two,.reward-meta{grid-template-columns:1fr}.drawer-help{grid-template-columns:auto 1fr}.drawer-help>.btn{grid-column:1/-1}.reward-items>div{grid-template-columns:minmax(0,1fr) 72px auto}.drawer-actions{flex-wrap:wrap}.drawer-actions>span{display:none}.drawer-actions .btn{flex:1 1 auto}}
@media(max-width:600px){.image-grid{grid-template-columns:1fr}.toolbar-actions{width:100%}.toolbar-actions>*{flex:1}}
</style>
