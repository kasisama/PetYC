<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ApiError } from '../api/client'
import {
  CHECKIN_TYPES,
  CONFIG_SCHEMAS,
  CONFIG_STATUS_CHANGED_EVENT,
  DELETE_TYPE_MAP,
  ITEM_TYPES,
  REWARD_TYPES,
  SCHEMA_LABELS,
  SHOP_TYPES,
  SYSTEM_GROUP_ORDER,
  type CheckinRewardConfigRow,
  type CommandConfigRow,
  type ConfigSchema,
  type ImageConfigRow,
  type ItemConfigRow,
  type MenuConfigRow,
  type PetSpeciesConfigRow,
  type ShopItemConfigRow,
  type SystemConfigRow,
  type WorkSettingConfigRow,
  deleteConfigItem,
  emptyCheckin,
  emptyCommand,
  emptyImage,
  emptyItem,
  emptyMenu,
  emptyPetSpecies,
  emptyShopItem,
  emptyWork,
  fetchConfig,
  fetchConfigStatus,
  imagePreviewUrl,
  normalizeCheckin,
  normalizeCommand,
  normalizeImage,
  normalizeItem,
  normalizeMenu,
  normalizePetSpecies,
  normalizeShopItem,
  normalizeSystem,
  normalizeWork,
  reloadConfigs,
  saveConfig,
  systemKeyGroup,
  systemKeyLabel,
  uploadImage,
  type ConfigStatus,
} from '../api/config'
import PageHeader from '../components/ui/PageHeader.vue'
import UiModal from '../components/ui/UiModal.vue'
import UiState from '../components/ui/UiState.vue'
import { useToast } from '../composables/useToast'
import { confirmUnsavedNavigation, useUnsavedChanges } from '../composables/useUnsavedChanges'

type SyncState = 'clean' | 'dirty' | 'saved' | 'reloaded'

const schema = ref<ConfigSchema>('system')
const loading = ref(false)
const saving = ref(false)
const reloading = ref(false)
const error = ref('')
const search = ref('')
const syncState = ref<SyncState>('clean')
const configStatus = ref<ConfigStatus | null>(null)
const toast = useToast()
const hasUnsavedChanges = computed(() => syncState.value === 'dirty')
useUnsavedChanges(hasUnsavedChanges)

// 各 schema 本地草稿
const systemRows = ref<SystemConfigRow[]>([])
const commandRows = ref<CommandConfigRow[]>([])
const petRows = ref<PetSpeciesConfigRow[]>([])
const itemRows = ref<ItemConfigRow[]>([])
const shopRows = ref<ShopItemConfigRow[]>([])
const workRows = ref<WorkSettingConfigRow[]>([])
const menuRows = ref<MenuConfigRow[]>([])
const imageRows = ref<ImageConfigRow[]>([])
const checkinRows = ref<CheckinRewardConfigRow[]>([])
const persistedKeys = new Map<ConfigSchema, Set<string>>()

// 折叠 / 子 Tab
const collapsedGroups = ref<Record<string, boolean>>({})
const shopTab = ref<'shop_normal' | 'shop_affection'>('shop_normal')
const checkinTab = ref<'checkin_newbie' | 'checkin_weekly'>('checkin_newbie')
const petSelected = ref(0)
const petFormTab = ref(0)
const petSearch = ref('')
const itemPage = ref(1)
const shopPage = ref(1)
const PAGE_SIZE = 12
const PET_FORM_TABS = ['概况', '属性与成长', '进化 · 觉醒 · 资源']

/** 属性对照表：初始 / 上限 */
const PET_ATTR_ROWS = [
  { label: '生命', init: 'Health', max: 'HealthMax' },
  { label: '饱食', init: 'Hunger', max: 'HungerMax' },
  { label: '智慧', init: 'Wisdom', max: 'WisdomMax' },
  { label: '力量', init: 'Strength', max: 'StrengthMax' },
  { label: '防御', init: 'Defense', max: 'DefenseMax' },
] as const

/** 概况页图片 */
const PET_PROFILE_IMAGES = [
  { k: 'Image', l: '宠物图片' },
  { k: 'AdoptImage', l: '领养图' },
] as const

/** 动作与场景图（进化/觉醒图在各自卡片内） */
const PET_ACTION_IMAGES = [
  { k: 'TrainStartImg', l: '锻炼开始' },
  { k: 'TrainEndImg', l: '锻炼结束' },
  { k: 'StudyStartImg', l: '学习开始' },
  { k: 'StudyEndImg', l: '学习结束' },
  { k: 'FitnessStartImg', l: '健身开始' },
  { k: 'FitnessEndImg', l: '健身结束' },
] as const

// 删除确认
const deleteOpen = ref(false)
const deleteLabel = ref('')
const deleteBusy = ref(false)
let deleteAction: (() => Promise<void>) | null = null

// 图片预览
const lightboxUrl = ref('')

// 上传中字段标记
const uploadingKey = ref('')

const schemaNav = CONFIG_SCHEMAS.map((id) => ({
  id,
  label: SCHEMA_LABELS[id],
}))

const syncLabel = computed(() => {
  switch (syncState.value) {
    case 'dirty':
      return '已修改未保存'
    case 'saved':
      return '已保存到数据库，尚未热重载'
    case 'reloaded':
      return '已热重载生效'
    default:
      return '与服务器一致'
  }
})

const syncClass = computed(() => {
  switch (syncState.value) {
    case 'dirty':
      return 'status-dirty'
    case 'saved':
      return 'status-saved'
    case 'reloaded':
      return 'status-ok'
    default:
      return 'status-ok'
  }
})

function errMsg(e: unknown, fallback: string) {
  if (e instanceof ApiError) return e.message
  if (e instanceof Error) return e.message
  return fallback
}

function formatTime(value: string | null | undefined) {
  if (!value) return '暂无记录'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

async function loadStatus() {
  configStatus.value = await fetchConfigStatus()
  if (syncState.value !== 'dirty') {
    syncState.value = configStatus.value.pending_reload ? 'saved' : 'reloaded'
  }
}

function markDirty() {
  if (syncState.value !== 'dirty') syncState.value = 'dirty'
}

function configRowKey(currentSchema: ConfigSchema, row: unknown): string {
  const value = row as Record<string, unknown>
  switch (currentSchema) {
    case 'system':
      return String(value.Key ?? '')
    case 'commands':
      return String(value.FuncName ?? '')
    case 'shop_items':
    case 'checkin_rewards':
      return String(value.ID ?? '')
    default:
      return String(value.Name ?? '')
  }
}

function rememberPersistedKeys(currentSchema: ConfigSchema, rows: unknown[]) {
  persistedKeys.set(
    currentSchema,
    new Set(rows.map((row) => configRowKey(currentSchema, row)).filter(Boolean)),
  )
}

function isPersisted(currentSchema: ConfigSchema, key: string | number) {
  return persistedKeys.get(currentSchema)?.has(String(key)) ?? false
}

function forgetPersistedKey(currentSchema: ConfigSchema, key: string | number) {
  persistedKeys.get(currentSchema)?.delete(String(key))
}

function toggleGroup(g: string) {
  collapsedGroups.value[g] = !collapsedGroups.value[g]
}

// —— 系统参数分组 ——
const systemGroups = computed(() => {
  const q = search.value.trim().toLowerCase()
  const map = new Map<string, SystemConfigRow[]>()
  for (const row of systemRows.value) {
    const label = systemKeyLabel(row.Key)
    if (
      q &&
      !row.Key.toLowerCase().includes(q) &&
      !label.toLowerCase().includes(q) &&
      !row.Value.toLowerCase().includes(q)
    ) {
      continue
    }
    const g = systemKeyGroup(row.Key)
    if (!map.has(g)) map.set(g, [])
    map.get(g)!.push(row)
  }
  const ordered: { name: string; rows: SystemConfigRow[] }[] = []
  for (const name of SYSTEM_GROUP_ORDER) {
    const rows = map.get(name)
    if (rows?.length) ordered.push({ name, rows })
  }
  for (const [name, rows] of map) {
    if (!SYSTEM_GROUP_ORDER.includes(name)) ordered.push({ name, rows })
  }
  return ordered
})

const filteredCommands = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return commandRows.value
  return commandRows.value.filter(
    (r) =>
      r.FuncName.toLowerCase().includes(q) || r.Command.toLowerCase().includes(q),
  )
})

const filteredImages = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return imageRows.value
  return imageRows.value.filter(
    (r) => r.Name.toLowerCase().includes(q) || r.Path.toLowerCase().includes(q),
  )
})

const filteredMenus = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return menuRows.value
  return menuRows.value.filter(
    (r) => r.Name.toLowerCase().includes(q) || r.Reply.toLowerCase().includes(q),
  )
})

const filteredItems = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return itemRows.value
  return itemRows.value.filter(
    (r) =>
      r.Name.toLowerCase().includes(q) ||
      r.Type.toLowerCase().includes(q) ||
      r.Description.toLowerCase().includes(q) ||
      String(r.Effect ?? '')
        .toLowerCase()
        .includes(q),
  )
})

const filteredWorks = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return workRows.value
  return workRows.value.filter((r) => r.Name.toLowerCase().includes(q))
})

const shopFiltered = computed(() => {
  const byTab = shopRows.value.filter((r) => r.ShopType === shopTab.value)
  const q = search.value.trim().toLowerCase()
  if (!q) return byTab
  return byTab.filter(
    (r) =>
      r.Name.toLowerCase().includes(q) ||
      String(r.Description ?? '')
        .toLowerCase()
        .includes(q) ||
      String(r.ID).includes(q),
  )
})

const checkinFiltered = computed(() =>
  checkinRows.value.filter((r) => r.Type === checkinTab.value),
)

/** 种类列表：保留原始下标，避免筛选后选错行 */
const filteredPets = computed(() => {
  const q = petSearch.value.trim().toLowerCase()
  const rows = petRows.value.map((p, index) => ({ p, index }))
  if (!q) return rows
  return rows.filter(
    ({ p }) =>
      (p.Name || '').toLowerCase().includes(q) ||
      (p.Description || '').toLowerCase().includes(q),
  )
})

const currentPet = computed(() => petRows.value[petSelected.value] ?? null)

function paginate<T>(list: T[], page: number, size: number) {
  const total = list.length
  const pages = Math.max(1, Math.ceil(total / size) || 1)
  const safePage = Math.min(Math.max(1, page), pages)
  const start = (safePage - 1) * size
  return {
    rows: list.slice(start, start + size),
    total,
    pages,
    page: safePage,
    from: total === 0 ? 0 : start + 1,
    to: Math.min(start + size, total),
  }
}

const pagedItems = computed(() => paginate(filteredItems.value, itemPage.value, PAGE_SIZE))
const pagedShop = computed(() => paginate(shopFiltered.value, shopPage.value, PAGE_SIZE))

watch(search, () => {
  itemPage.value = 1
  shopPage.value = 1
})
watch(shopTab, () => {
  shopPage.value = 1
})
watch(schema, () => {
  itemPage.value = 1
  shopPage.value = 1
  petSearch.value = ''
})
watch(petSearch, () => {
  const stillVisible = filteredPets.value.some(({ index }) => index === petSelected.value)
  if (!stillVisible && filteredPets.value.length) {
    petSelected.value = filteredPets.value[0].index
  }
})

async function loadSchema(s: ConfigSchema) {
  loading.value = true
  error.value = ''
  try {
    const raw = await fetchConfig(s)
    switch (s) {
      case 'system':
        systemRows.value = raw.map(normalizeSystem)
        break
      case 'commands':
        commandRows.value = raw.map(normalizeCommand)
        break
      case 'pet_species':
        petRows.value = raw.map(normalizePetSpecies)
        petSelected.value = 0
        break
      case 'items':
        itemRows.value = raw.map(normalizeItem)
        break
      case 'shop_items':
        shopRows.value = raw.map(normalizeShopItem)
        break
      case 'work_settings':
        workRows.value = raw.map(normalizeWork)
        break
      case 'menus':
        menuRows.value = raw.map(normalizeMenu)
        break
      case 'images':
        imageRows.value = raw.map(normalizeImage)
        break
      case 'checkin_rewards':
        checkinRows.value = raw.map(normalizeCheckin)
        break
    }
    rememberPersistedKeys(s, currentRows())
  } catch (e) {
    error.value = errMsg(e, '加载配置失败')
  } finally {
    loading.value = false
  }
}

function currentRows(): unknown[] {
  switch (schema.value) {
    case 'system':
      return systemRows.value
    case 'commands':
      return commandRows.value
    case 'pet_species':
      return petRows.value
    case 'items':
      return itemRows.value
    case 'shop_items':
      return shopRows.value
    case 'work_settings':
      return workRows.value
    case 'menus':
      return menuRows.value
    case 'images':
      return imageRows.value
    case 'checkin_rewards':
      return checkinRows.value
	  default:
	    return []
  }
}

async function handleSave() {
  if (saving.value) return
  saving.value = true
  error.value = ''
  try {
    await saveConfig(schema.value, currentRows())
    rememberPersistedKeys(schema.value, currentRows())
    syncState.value = 'saved'
    toast.success('已保存到数据库，热重载后生效')
  } catch (e) {
    error.value = errMsg(e, '保存失败')
    return
  } finally {
    saving.value = false
  }
  try {
    await loadStatus()
  } catch {
    toast.warning('保存已成功，但配置状态刷新失败，请稍后重试')
  }
}

async function handleReload() {
  if (reloading.value) return
  reloading.value = true
  error.value = ''
  try {
    const msg = await reloadConfigs()
    syncState.value = 'reloaded'
    toast.success(msg)
  } catch (e) {
    error.value = errMsg(e, '热重载失败')
    return
  } finally {
    reloading.value = false
  }
  try {
    await loadStatus()
  } catch {
    toast.warning('热重载已成功，但配置状态刷新失败，请稍后重试')
  }
}

function askDelete(label: string, action: () => Promise<void>) {
  deleteLabel.value = label
  deleteAction = action
  deleteOpen.value = true
}

async function confirmDelete() {
  if (!deleteAction || deleteBusy.value) return
  deleteBusy.value = true
  error.value = ''
  try {
    await deleteAction()
    deleteOpen.value = false
    syncState.value = 'saved'
    toast.success('配置项已删除，热重载后生效')
  } catch (e) {
    error.value = errMsg(e, '删除失败')
    return
  } finally {
    deleteBusy.value = false
  }
  try {
    await loadStatus()
  } catch {
    toast.warning('删除已成功，但配置状态刷新失败，请稍后重试')
  }
}

function cancelDelete() {
  deleteOpen.value = false
  deleteAction = null
}

async function doUpload(file: File | undefined, apply: (path: string) => void) {
  if (!file) return
  uploadingKey.value = file.name
  error.value = ''
  try {
    const res = await uploadImage(file)
    apply(res.path)
    markDirty()
    toast.success(res.message || '图片上传成功')
  } catch (e) {
    error.value = errMsg(e, '上传失败')
  } finally {
    uploadingKey.value = ''
  }
}

function openLightbox(path: string) {
  const url = imagePreviewUrl(path)
  if (url) lightboxUrl.value = url
}

// —— 增删行 ——
function addCommand() {
  commandRows.value.push(emptyCommand('新功能'))
  markDirty()
}

function removeCommand(row: CommandConfigRow, idx: number) {
  askDelete(`自定义指令「${row.FuncName}」`, async () => {
    if (isPersisted('commands', row.FuncName)) {
      await deleteConfigItem(DELETE_TYPE_MAP.commands.type, row.FuncName)
      forgetPersistedKey('commands', row.FuncName)
    }
    commandRows.value.splice(idx, 1)
  })
}

function addItem() {
  itemRows.value.push(emptyItem('新道具'))
  itemPage.value = Math.max(1, Math.ceil(itemRows.value.length / PAGE_SIZE))
  markDirty()
}

function removeItem(row: ItemConfigRow, idx: number) {
  askDelete(`道具「${row.Name}」`, async () => {
    if (isPersisted('items', row.Name)) {
      await deleteConfigItem(DELETE_TYPE_MAP.items.type, row.Name)
      forgetPersistedKey('items', row.Name)
    }
    itemRows.value.splice(idx, 1)
  })
}

function addShop() {
  shopRows.value.push(emptyShopItem(shopTab.value))
  // 跳到当前货架最后一页
  const count = shopRows.value.filter((r) => r.ShopType === shopTab.value).length
  shopPage.value = Math.max(1, Math.ceil(count / PAGE_SIZE))
  markDirty()
}

function removeShop(row: ShopItemConfigRow) {
  const key = row.ID ? String(row.ID) : row.Name
  askDelete(`商品「${row.Name || key}」`, async () => {
    if (key) {
      await deleteConfigItem(DELETE_TYPE_MAP.shop_items.type, key)
    }
    shopRows.value = shopRows.value.filter((r) => r !== row)
  })
}

function addWork() {
  workRows.value.push(emptyWork('新打工'))
  markDirty()
}

function removeWork(row: WorkSettingConfigRow, idx: number) {
  askDelete(`打工「${row.Name}」`, async () => {
    if (isPersisted('work_settings', row.Name)) {
      await deleteConfigItem(DELETE_TYPE_MAP.work_settings.type, row.Name)
      forgetPersistedKey('work_settings', row.Name)
    }
    workRows.value.splice(idx, 1)
  })
}

function addMenu() {
  menuRows.value.push(emptyMenu('新菜单'))
  markDirty()
}

function removeMenu(row: MenuConfigRow, idx: number) {
  askDelete(`菜单「${row.Name}」`, async () => {
    if (isPersisted('menus', row.Name)) {
      await deleteConfigItem(DELETE_TYPE_MAP.menus.type, row.Name)
      forgetPersistedKey('menus', row.Name)
    }
    menuRows.value.splice(idx, 1)
  })
}

function addImage() {
  imageRows.value.push(emptyImage('新图片键'))
  markDirty()
}

function removeImage(row: ImageConfigRow, idx: number) {
  askDelete(`图片映射「${row.Name}」`, async () => {
    if (isPersisted('images', row.Name)) {
      await deleteConfigItem(DELETE_TYPE_MAP.images.type, row.Name)
      forgetPersistedKey('images', row.Name)
    }
    imageRows.value.splice(idx, 1)
  })
}

function addCheckin() {
  checkinRows.value.push(emptyCheckin(checkinTab.value, checkinTab.value === 'checkin_newbie' ? '1' : '周一'))
  markDirty()
}

function removeCheckin(row: CheckinRewardConfigRow) {
  askDelete(`签到奖励 #${row.ID || '新'}（${row.Day}）`, async () => {
    if (row.ID) {
      await deleteConfigItem(DELETE_TYPE_MAP.checkin_rewards.type, String(row.ID))
    }
    checkinRows.value = checkinRows.value.filter((r) => r !== row)
  })
}

function addPet() {
  petRows.value.push(emptyPetSpecies('新种类'))
  petSelected.value = petRows.value.length - 1
  petSearch.value = ''
  markDirty()
}

function removePet(row: PetSpeciesConfigRow) {
  askDelete(`宠物种类「${row.Name}」`, async () => {
    if (isPersisted('pet_species', row.Name)) {
      await deleteConfigItem(DELETE_TYPE_MAP.pet_species.type, row.Name)
      forgetPersistedKey('pet_species', row.Name)
    }
    const idx = petRows.value.indexOf(row)
    petRows.value = petRows.value.filter((r) => r !== row)
    if (petSelected.value >= petRows.value.length) {
      petSelected.value = Math.max(0, petRows.value.length - 1)
    } else if (idx >= 0 && petSelected.value > idx) {
      petSelected.value -= 1
    }
  })
}

watch(schema, (s) => {
  search.value = ''
  loadSchema(s)
})

function selectSchema(nextSchema: ConfigSchema) {
  if (nextSchema === schema.value) return
  if (!confirmUnsavedNavigation(hasUnsavedChanges)) return
  syncState.value = configStatus.value?.pending_reload ? 'saved' : 'reloaded'
  schema.value = nextSchema
}

onMounted(() => {
  window.addEventListener(CONFIG_STATUS_CHANGED_EVENT, handleConfigStatusChanged)
  loadSchema(schema.value)
  loadStatus().catch((e) => {
    error.value = errMsg(e, '读取配置运行状态失败')
  })
})

function handleConfigStatusChanged() {
  void loadStatus().catch(() => {
    toast.warning('配置已变更，但运行状态刷新失败，请稍后重试')
  })
}

onBeforeUnmount(() => {
  window.removeEventListener(CONFIG_STATUS_CHANGED_EVENT, handleConfigStatusChanged)
})
</script>

<template>
  <section class="config-page">
    <PageHeader
      eyebrow="Configuration"
      title="配置中心"
      description="按配置域维护游戏规则；保存写入数据库，热重载后机器人内存才会使用最新版本。"
    />

    <div class="status-bar card" :class="syncClass">
      <div class="status-left">
        <span class="status-dot" aria-hidden="true" />
        <span class="status-copy">
          <span class="status-text">{{ syncLabel }}</span>
          <small v-if="configStatus">
            数据库 v{{ configStatus.db_revision }} · 内存 v{{ configStatus.loaded_revision }} ·
            最近保存 {{ formatTime(configStatus.saved_at) }} · 最近生效 {{ formatTime(configStatus.loaded_at) }}
          </small>
        </span>
      </div>
      <div class="status-actions">
        <button type="button" class="btn" :disabled="saving || loading" @click="handleSave">
          {{ saving ? '保存中…' : '保存到数据库' }}
        </button>
        <button
          type="button"
          class="btn btn-success"
          :disabled="reloading || loading"
          title="将数据库配置同步到机器人运行内存"
          @click="handleReload"
        >
          {{ reloading ? '重载中…' : '热重载' }}
        </button>
      </div>
    </div>

    <p v-if="error && currentRows().length" class="page-error" role="alert">{{ error }}</p>

    <div class="config-layout">
      <nav class="subnav card" aria-label="配置类型">
        <button
          v-for="item in schemaNav"
          :key="item.id"
          type="button"
          class="subnav-item"
          :class="{ active: schema === item.id }"
          @click="selectSchema(item.id)"
        >
          {{ item.label }}
        </button>
      </nav>

      <div class="config-main">
        <UiState v-if="loading" tone="loading" title="正在加载配置" description="正在读取当前配置域的数据。" />
        <UiState
          v-else-if="error && !currentRows().length"
          tone="error"
          title="配置加载失败"
          :description="error"
          action-label="重试"
          @action="loadSchema(schema)"
        />

        <!-- 系统参数 -->
        <div v-else-if="schema === 'system'" class="panel">
          <div class="toolbar">
            <input
              v-model="search"
              class="field-input search"
              type="search"
              placeholder="搜索键名或中文说明…"
            />
          </div>
          <div
            v-for="group in systemGroups"
            :key="group.name"
            class="group-card card"
          >
            <button type="button" class="group-head" @click="toggleGroup(group.name)">
              <span>{{ group.name }}</span>
              <span class="group-meta">{{ group.rows.length }} 项 · {{ collapsedGroups[group.name] ? '展开' : '收起' }}</span>
            </button>
            <div v-show="!collapsedGroups[group.name]" class="group-body">
              <label v-for="row in group.rows" :key="row.Key" class="kv-row">
                <span class="kv-label">
                  <span class="kv-title">{{ systemKeyLabel(row.Key) }}</span>
                  <span class="kv-key">{{ row.Key }}</span>
                </span>
                <input
                  v-model="row.Value"
                  class="field-input"
                  type="text"
                  @input="markDirty"
                />
              </label>
            </div>
          </div>
          <p v-if="!systemGroups.length" class="empty">没有匹配的系统参数</p>
        </div>

        <!-- 自定义指令 -->
        <div v-else-if="schema === 'commands'" class="panel">
          <div class="toolbar">
            <input
              v-model="search"
              class="field-input search"
              type="search"
              placeholder="搜索功能名或指令…"
            />
            <button type="button" class="btn btn-ghost" @click="addCommand">新增指令</button>
          </div>
          <div class="table-wrap card">
            <table class="data-table">
              <thead>
                <tr>
                  <th>功能名称</th>
                  <th>玩家触发指令</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="!filteredCommands.length">
                  <td colspan="3" class="empty-cell">暂无指令配置</td>
                </tr>
                <tr v-for="(row, idx) in filteredCommands" :key="idx">
                  <td data-label="功能名称">
                    <input
                      v-model="row.FuncName"
                      class="field-input cell-input"
                      type="text"
                      :disabled="isPersisted('commands', row.FuncName)"
                      :title="isPersisted('commands', row.FuncName) ? '已保存配置的主键不可修改；如需更名请删除后新增' : ''"
                      @input="markDirty"
                    />
                  </td>
                  <td data-label="玩家触发指令">
                    <input
                      v-model="row.Command"
                      class="field-input cell-input"
                      type="text"
                      @input="markDirty"
                    />
                  </td>
                  <td data-label="操作">
                    <button type="button" class="link-btn danger" @click="removeCommand(row, commandRows.indexOf(row))">
                      删除
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- 签到奖励 -->
        <div v-else-if="schema === 'checkin_rewards'" class="panel">
          <div class="toolbar">
            <div class="tabs">
              <button
                v-for="t in CHECKIN_TYPES"
                :key="t.value"
                type="button"
                class="tab"
                :class="{ active: checkinTab === t.value }"
                @click="checkinTab = t.value as 'checkin_newbie' | 'checkin_weekly'"
              >
                {{ t.label }}
              </button>
            </div>
            <button type="button" class="btn btn-ghost" @click="addCheckin">新增条目</button>
          </div>
          <div class="card-grid">
            <div v-for="row in checkinFiltered" :key="row.ID + '-' + row.Day + row.Type" class="mini-card card">
              <div class="mini-head">
                <strong>{{ row.Day || '（未填日期）' }}</strong>
                <button type="button" class="link-btn danger" @click="removeCheckin(row)">删除</button>
              </div>
              <label class="f-field">
                <span>日期 / 星期</span>
                <input v-model="row.Day" class="field-input" type="text" @input="markDirty" />
              </label>
              <label class="f-field">
                <span>类型</span>
                <select v-model="row.Type" class="field-input" @change="markDirty">
                  <option v-for="t in CHECKIN_TYPES" :key="t.value" :value="t.value">{{ t.label }}</option>
                </select>
              </label>
              <div class="f-row">
                <label class="f-field">
                  <span>奖励货币</span>
                  <input v-model.number="row.Currency" class="field-input" type="number" @input="markDirty" />
                </label>
                <label class="f-field">
                  <span>奖励好感</span>
                  <input v-model.number="row.Affection" class="field-input" type="number" @input="markDirty" />
                </label>
              </div>
              <label class="f-field">
                <span>奖励物品</span>
                <input v-model="row.Items" class="field-input" type="text" placeholder="物品*数量#…" @input="markDirty" />
              </label>
              <label class="f-field">
                <span>奖励图片</span>
                <div class="img-field">
                  <input v-model="row.Image" class="field-input" type="text" @input="markDirty" />
                  <label class="btn btn-ghost file-btn">
                    上传
                    <input
                      type="file"
                      accept="image/jpeg,image/png,image/gif,image/webp"
                      hidden
                      @change="doUpload(($event.target as HTMLInputElement).files?.[0], (p) => { row.Image = p })"
                    />
                  </label>
                  <button
                    v-if="row.Image"
                    type="button"
                    class="thumb-btn"
                    @click="openLightbox(row.Image)"
                  >
                    <img :src="imagePreviewUrl(row.Image)" alt="" class="thumb" />
                  </button>
                </div>
              </label>
            </div>
          </div>
          <p v-if="!checkinFiltered.length" class="empty">当前类型暂无签到奖励，可新增</p>
        </div>

        <!-- 挂机打工 -->
        <div v-else-if="schema === 'work_settings'" class="panel">
          <div class="toolbar">
            <input v-model="search" class="field-input search" type="search" placeholder="搜索打工名称…" />
            <button type="button" class="btn btn-ghost" @click="addWork">新增打工</button>
          </div>
          <div class="stack">
            <div v-for="(row, idx) in filteredWorks" :key="idx" class="form-card card">
              <div class="mini-head">
                <input
                  v-model="row.Name"
                  class="field-input title-input"
                  type="text"
                  placeholder="打工名称"
                  :disabled="isPersisted('work_settings', row.Name)"
                  :title="isPersisted('work_settings', row.Name) ? '已保存配置的主键不可修改；如需更名请删除后新增' : ''"
                  @input="markDirty"
                />
                <button type="button" class="link-btn danger" @click="removeWork(row, workRows.indexOf(row))">
                  删除
                </button>
              </div>
              <div class="form-grid">
                <label class="f-field">
                  <span>时长（分钟）</span>
                  <input v-model.number="row.Time" class="field-input" type="number" @input="markDirty" />
                </label>
                <label class="f-field">
                  <span>饱食消耗</span>
                  <input v-model.number="row.HungerCost" class="field-input" type="number" @input="markDirty" />
                </label>
                <label class="f-field">
                  <span>奖励货币</span>
                  <input v-model.number="row.RewardCoin" class="field-input" type="number" @input="markDirty" />
                </label>
                <label class="f-field">
                  <span>概率奖励物品</span>
                  <input v-model="row.RewardItems" class="field-input" type="text" @input="markDirty" />
                </label>
                <label class="f-field wide">
                  <span>回复语（逗号分隔）</span>
                  <textarea v-model="row.ReplyQuotes" class="field-input area" rows="2" @input="markDirty" />
                </label>
                <label class="f-field">
                  <span>开始图</span>
                  <div class="img-field">
                    <input v-model="row.StartImage" class="field-input" type="text" @input="markDirty" />
                    <label class="btn btn-ghost file-btn">
                      上传
                      <input
                        type="file"
                        accept="image/jpeg,image/png,image/gif,image/webp"
                        hidden
                        @change="doUpload(($event.target as HTMLInputElement).files?.[0], (p) => { row.StartImage = p })"
                      />
                    </label>
                    <button v-if="row.StartImage" type="button" class="thumb-btn" @click="openLightbox(row.StartImage)">
                      <img :src="imagePreviewUrl(row.StartImage)" alt="" class="thumb" />
                    </button>
                  </div>
                </label>
                <label class="f-field">
                  <span>结束图</span>
                  <div class="img-field">
                    <input v-model="row.EndImage" class="field-input" type="text" @input="markDirty" />
                    <label class="btn btn-ghost file-btn">
                      上传
                      <input
                        type="file"
                        accept="image/jpeg,image/png,image/gif,image/webp"
                        hidden
                        @change="doUpload(($event.target as HTMLInputElement).files?.[0], (p) => { row.EndImage = p })"
                      />
                    </label>
                    <button v-if="row.EndImage" type="button" class="thumb-btn" @click="openLightbox(row.EndImage)">
                      <img :src="imagePreviewUrl(row.EndImage)" alt="" class="thumb" />
                    </button>
                  </div>
                </label>
              </div>
            </div>
          </div>
          <p v-if="!filteredWorks.length" class="empty">暂无打工配置</p>
        </div>

        <!-- 商店：表格 + 分页 -->
        <div v-else-if="schema === 'shop_items'" class="panel">
          <div class="toolbar">
            <div class="tabs">
              <button
                v-for="t in SHOP_TYPES"
                :key="t.value"
                type="button"
                class="tab"
                :class="{ active: shopTab === t.value }"
                @click="shopTab = t.value as 'shop_normal' | 'shop_affection'"
              >
                {{ t.label }}
              </button>
            </div>
            <input
              v-model="search"
              class="field-input search"
              type="search"
              placeholder="搜索名称 / 描述 / ID…"
            />
            <button type="button" class="btn btn-ghost" @click="addShop">新增商品</button>
          </div>
          <div class="table-wrap card">
            <table class="data-table dense-table">
              <thead>
                <tr>
                  <th class="col-id">ID</th>
                  <th class="col-name">名称</th>
                  <th class="col-img">图片</th>
                  <th class="col-num">库存</th>
                  <th class="col-num">价格</th>
                  <th>描述</th>
                  <th class="col-shop">货架</th>
                  <th class="col-ops">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="!pagedShop.rows.length">
                  <td colspan="8" class="empty-cell">当前货架暂无商品</td>
                </tr>
                <tr v-for="row in pagedShop.rows" :key="(row.ID || 0) + '-' + row.Name">
                  <td class="num" data-label="ID">{{ row.ID || '新' }}</td>
                  <td data-label="名称">
                    <input v-model="row.Name" class="field-input cell-input" type="text" @input="markDirty" />
                  </td>
                  <td data-label="图片">
                    <div class="img-field compact">
                      <input
                        v-model="row.Image"
                        class="field-input cell-input"
                        type="text"
                        placeholder="路径"
                        @input="markDirty"
                      />
                      <label class="btn btn-ghost file-btn sm">
                        传
                        <input
                          type="file"
                          accept="image/jpeg,image/png,image/gif,image/webp"
                          hidden
                          @change="doUpload(($event.target as HTMLInputElement).files?.[0], (p) => { row.Image = p })"
                        />
                      </label>
                      <button v-if="row.Image" type="button" class="thumb-btn" @click="openLightbox(row.Image)">
                        <img :src="imagePreviewUrl(row.Image)" alt="" class="thumb" />
                      </button>
                    </div>
                  </td>
                  <td data-label="库存">
                    <input
                      v-model.number="row.Stock"
                      class="field-input cell-input num-in"
                      type="number"
                      title="-1 表示无限"
                      @input="markDirty"
                    />
                  </td>
                  <td data-label="价格">
                    <input
                      v-model.number="row.Price"
                      class="field-input cell-input num-in"
                      type="number"
                      @input="markDirty"
                    />
                  </td>
                  <td data-label="描述">
                    <input
                      v-model="row.Description"
                      class="field-input cell-input"
                      type="text"
                      @input="markDirty"
                    />
                  </td>
                  <td data-label="货架">
                    <select v-model="row.ShopType" class="field-input cell-input" @change="markDirty">
                      <option v-for="t in SHOP_TYPES" :key="t.value" :value="t.value">{{ t.label }}</option>
                    </select>
                  </td>
                  <td class="ops-cell" data-label="操作">
                    <button type="button" class="link-btn danger" @click="removeShop(row)">下架</button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="pager">
            <span class="pager-info">
              共 {{ pagedShop.total }} 条
              <template v-if="pagedShop.total"> · 第 {{ pagedShop.from }}–{{ pagedShop.to }} 条</template>
            </span>
            <div class="pager-btns">
              <button
                type="button"
                class="btn btn-ghost sm-btn"
                :disabled="pagedShop.page <= 1"
                @click="shopPage = pagedShop.page - 1"
              >
                上一页
              </button>
              <span class="pager-page">{{ pagedShop.page }} / {{ pagedShop.pages }}</span>
              <button
                type="button"
                class="btn btn-ghost sm-btn"
                :disabled="pagedShop.page >= pagedShop.pages"
                @click="shopPage = pagedShop.page + 1"
              >
                下一页
              </button>
            </div>
          </div>
        </div>

        <!-- 道具：表格 + 分页 -->
        <div v-else-if="schema === 'items'" class="panel">
          <div class="toolbar">
            <input v-model="search" class="field-input search" type="search" placeholder="搜索名称 / 类型 / 效果 / 描述…" />
            <button type="button" class="btn btn-ghost" @click="addItem">新增道具</button>
          </div>
          <div class="table-wrap card">
            <table class="data-table dense-table">
              <thead>
                <tr>
                  <th class="col-name">名称</th>
                  <th class="col-type">类型</th>
                  <th class="col-type">礼包</th>
                  <th class="col-type">获得</th>
                  <th class="col-req">开启所需</th>
                  <th class="col-num">效果</th>
                  <th class="col-num">时长</th>
                  <th class="col-num">售价</th>
                  <th class="col-img">图片</th>
                  <th>描述</th>
                  <th class="col-ops">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="!pagedItems.rows.length">
                  <td colspan="11" class="empty-cell">暂无道具</td>
                </tr>
                <tr v-for="row in pagedItems.rows" :key="row.Name + itemRows.indexOf(row)">
                  <td data-label="名称">
                    <input
                      v-model="row.Name"
                      class="field-input cell-input"
                      type="text"
                      placeholder="名称"
                      :disabled="isPersisted('items', row.Name)"
                      :title="isPersisted('items', row.Name) ? '已保存配置的主键不可修改；如需更名请删除后新增' : ''"
                      @input="markDirty"
                    />
                  </td>
                  <td data-label="类型">
                    <select v-model="row.Type" class="field-input cell-input" @change="markDirty">
                      <option v-for="t in ITEM_TYPES" :key="t" :value="t">{{ t }}</option>
                      <option v-if="row.Type && !ITEM_TYPES.includes(row.Type)" :value="row.Type">
                        {{ row.Type }}
                      </option>
                    </select>
                  </td>
                  <td data-label="礼包">
                    <select v-model="row.RewardType" class="field-input cell-input" @change="markDirty">
                      <option v-for="t in REWARD_TYPES" :key="t || 'empty'" :value="t">
                        {{ t || '（无）' }}
                      </option>
                      <option
                        v-if="row.RewardType && !REWARD_TYPES.includes(row.RewardType)"
                        :value="row.RewardType"
                      >
                        {{ row.RewardType }}
                      </option>
                    </select>
                  </td>
                  <td data-label="获得">
                    <select v-model.number="row.ObtainType" class="field-input cell-input" @change="markDirty">
                      <option :value="0">多次</option>
                      <option :value="1">单次</option>
                    </select>
                  </td>
                  <td data-label="开启所需">
                    <input
                      v-model="row.OpenReq"
                      class="field-input cell-input"
                      type="text"
                      placeholder="—"
                      @input="markDirty"
                    />
                  </td>
                  <td data-label="效果">
                    <input
                      v-model="row.Effect"
                      class="field-input cell-input num-in"
                      type="text"
                      @input="markDirty"
                    />
                  </td>
                  <td data-label="时长">
                    <input
                      v-model.number="row.Time"
                      class="field-input cell-input num-in"
                      type="number"
                      @input="markDirty"
                    />
                  </td>
                  <td data-label="售价">
                    <input
                      v-model.number="row.SellPrice"
                      class="field-input cell-input num-in"
                      type="number"
                      @input="markDirty"
                    />
                  </td>
                  <td data-label="图片">
                    <div class="img-field compact">
                      <input
                        v-model="row.Image"
                        class="field-input cell-input"
                        type="text"
                        placeholder="路径"
                        @input="markDirty"
                      />
                      <label class="btn btn-ghost file-btn sm">
                        传
                        <input
                          type="file"
                          accept="image/jpeg,image/png,image/gif,image/webp"
                          hidden
                          @change="doUpload(($event.target as HTMLInputElement).files?.[0], (p) => { row.Image = p })"
                        />
                      </label>
                      <button v-if="row.Image" type="button" class="thumb-btn" @click="openLightbox(row.Image)">
                        <img :src="imagePreviewUrl(row.Image)" alt="" class="thumb" />
                      </button>
                    </div>
                  </td>
                  <td data-label="描述">
                    <input
                      v-model="row.Description"
                      class="field-input cell-input"
                      type="text"
                      @input="markDirty"
                    />
                  </td>
                  <td class="ops-cell" data-label="操作">
                    <button
                      type="button"
                      class="link-btn danger"
                      @click="removeItem(row, itemRows.indexOf(row))"
                    >
                      删除
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="pager">
            <span class="pager-info">
              共 {{ pagedItems.total }} 条
              <template v-if="pagedItems.total"> · 第 {{ pagedItems.from }}–{{ pagedItems.to }} 条</template>
            </span>
            <div class="pager-btns">
              <button
                type="button"
                class="btn btn-ghost sm-btn"
                :disabled="pagedItems.page <= 1"
                @click="itemPage = pagedItems.page - 1"
              >
                上一页
              </button>
              <span class="pager-page">{{ pagedItems.page }} / {{ pagedItems.pages }}</span>
              <button
                type="button"
                class="btn btn-ghost sm-btn"
                :disabled="pagedItems.page >= pagedItems.pages"
                @click="itemPage = pagedItems.page + 1"
              >
                下一页
              </button>
            </div>
          </div>
        </div>

        <!-- 宠物种类 -->
        <div v-else-if="schema === 'pet_species'" class="panel pet-panel">
          <aside class="pet-list card">
            <div class="pet-list-head">
              <span>种类列表</span>
              <button type="button" class="btn btn-ghost sm-btn" @click="addPet">新增</button>
            </div>
            <input
              v-model="petSearch"
              class="field-input pet-search"
              type="search"
              placeholder="搜索种类…"
            />
            <button
              v-for="{ p, index } in filteredPets"
              :key="p.Name + index"
              type="button"
              class="pet-list-item"
              :class="{ active: petSelected === index }"
              @click="petSelected = index"
            >
              <img
                v-if="p.Image"
                :src="imagePreviewUrl(p.Image)"
                alt=""
                class="pet-avatar"
              />
              <span v-else class="pet-avatar placeholder">宠</span>
              <span class="pet-name">{{ p.Name || '（未命名）' }}</span>
            </button>
            <p v-if="!petRows.length" class="empty small">暂无种类</p>
            <p v-else-if="!filteredPets.length" class="empty small">无匹配种类</p>
          </aside>
          <div v-if="currentPet" class="pet-form card">
            <div class="mini-head">
              <strong>编辑：{{ currentPet.Name || '新种类' }}</strong>
              <button type="button" class="link-btn danger" @click="removePet(currentPet)">删除种类</button>
            </div>
            <div class="tabs pet-tabs">
              <button
                v-for="(t, i) in PET_FORM_TABS"
                :key="t"
                type="button"
                class="tab"
                :class="{ active: petFormTab === i }"
                @click="petFormTab = i"
              >
                {{ t }}
              </button>
            </div>

            <!-- Tab0 概况：文案在上，图片在下，避免横向撑破 -->
            <div v-show="petFormTab === 0" class="pet-section">
              <div class="form-grid">
                <label class="f-field">
                  <span>种类名称</span>
                  <input
                    v-model="currentPet.Name"
                    class="field-input"
                    type="text"
                    :disabled="isPersisted('pet_species', currentPet.Name)"
                    :title="isPersisted('pet_species', currentPet.Name) ? '已保存配置的主键不可修改；如需更名请删除后新增' : ''"
                    @input="markDirty"
                  />
                </label>
                <label class="f-field">
                  <span>喜欢的食物</span>
                  <input v-model="currentPet.FavoriteFood" class="field-input" type="text" @input="markDirty" />
                </label>
                <label class="f-field">
                  <span>喜欢的礼物</span>
                  <input v-model="currentPet.FavoriteGift" class="field-input" type="text" @input="markDirty" />
                </label>
                <label class="f-field wide">
                  <span>描述</span>
                  <textarea v-model="currentPet.Description" class="field-input area" rows="3" @input="markDirty" />
                </label>
              </div>
              <div class="section-block">
                <div class="section-title">图片</div>
                <div class="pet-profile-imgs">
                  <div v-for="field in PET_PROFILE_IMAGES" :key="field.k" class="img-card">
                    <div class="img-card-head">
                      <span>{{ field.l }}</span>
                      <button
                        v-if="(currentPet as any)[field.k]"
                        type="button"
                        class="link-btn"
                        @click="openLightbox((currentPet as any)[field.k])"
                      >
                        预览
                      </button>
                    </div>
                    <button
                      v-if="(currentPet as any)[field.k]"
                      type="button"
                      class="img-card-preview"
                      @click="openLightbox((currentPet as any)[field.k])"
                    >
                      <img :src="imagePreviewUrl((currentPet as any)[field.k])" alt="" />
                    </button>
                    <div v-else class="img-card-preview empty">暂无图片</div>
                    <div class="img-field">
                      <input
                        v-model="(currentPet as any)[field.k]"
                        class="field-input"
                        type="text"
                        placeholder="路径或上传"
                        @input="markDirty"
                      />
                      <label class="btn btn-ghost file-btn">
                        上传
                        <input
                          type="file"
                          accept="image/jpeg,image/png,image/gif,image/webp"
                          hidden
                          @change="
                            doUpload(($event.target as HTMLInputElement).files?.[0], (p) => {
                              ;(currentPet as any)[field.k] = p
                            })
                          "
                        />
                      </label>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Tab1 属性与成长 -->
            <div v-show="petFormTab === 1" class="pet-section">
              <div class="section-block">
                <div class="section-title">属性对照</div>
                <div class="attr-table" role="table">
                  <div class="attr-row head" role="row">
                    <span role="columnheader">属性</span>
                    <span role="columnheader">初始</span>
                    <span role="columnheader">上限</span>
                  </div>
                  <div v-for="row in PET_ATTR_ROWS" :key="row.init" class="attr-row" role="row">
                    <span class="attr-label" role="cell">{{ row.label }}</span>
                    <label class="attr-cell" role="cell">
                      <input
                        v-model.number="(currentPet as any)[row.init]"
                        class="field-input"
                        type="number"
                        @input="markDirty"
                      />
                    </label>
                    <label class="attr-cell" role="cell">
                      <input
                        v-model.number="(currentPet as any)[row.max]"
                        class="field-input"
                        type="number"
                        @input="markDirty"
                      />
                    </label>
                  </div>
                </div>
              </div>
              <div class="section-block">
                <div class="section-title">加成比率</div>
                <div class="form-grid bonus-grid">
                  <label class="f-field">
                    <span>好感加成</span>
                    <input v-model.number="currentPet.AffectionBonus" class="field-input" type="number" step="any" @input="markDirty" />
                  </label>
                  <label class="f-field">
                    <span>成长加成</span>
                    <input v-model.number="currentPet.GrowthBonus" class="field-input" type="number" step="any" @input="markDirty" />
                  </label>
                  <label class="f-field">
                    <span>属性加成</span>
                    <input v-model.number="currentPet.AttributeBonus" class="field-input" type="number" step="any" @input="markDirty" />
                  </label>
                  <label class="f-field">
                    <span>货币加成</span>
                    <input v-model.number="currentPet.CurrencyBonus" class="field-input" type="number" step="any" @input="markDirty" />
                  </label>
                </div>
              </div>
            </div>

            <!-- Tab2 进化 · 觉醒 · 资源 -->
            <div v-show="petFormTab === 2" class="pet-section">
              <div class="pet-dual-cards">
                <div class="stage-card">
                  <div class="section-title">进化</div>
                  <div class="form-grid stage-grid">
                    <label class="f-field">
                      <span>进化分支</span>
                      <input v-model.number="currentPet.EvolutionBranch" class="field-input" type="number" @input="markDirty" />
                    </label>
                    <label class="f-field">
                      <span>进化后名称</span>
                      <input v-model="currentPet.Evolution" class="field-input" type="text" @input="markDirty" />
                    </label>
                    <label class="f-field">
                      <span>所需成长</span>
                      <input v-model.number="currentPet.EvolutionGrowth" class="field-input" type="number" @input="markDirty" />
                    </label>
                    <label class="f-field">
                      <span>所需好感</span>
                      <input v-model.number="currentPet.EvolutionAffect" class="field-input" type="number" @input="markDirty" />
                    </label>
                    <label class="f-field wide">
                      <span>进化图片</span>
                      <div class="img-field">
                        <input v-model="currentPet.EvolutionImage" class="field-input" type="text" @input="markDirty" />
                        <label class="btn btn-ghost file-btn">
                          上传
                          <input
                            type="file"
                            accept="image/jpeg,image/png,image/gif,image/webp"
                            hidden
                            @change="doUpload(($event.target as HTMLInputElement).files?.[0], (p) => { currentPet!.EvolutionImage = p })"
                          />
                        </label>
                        <button
                          v-if="currentPet.EvolutionImage"
                          type="button"
                          class="thumb-btn"
                          @click="openLightbox(currentPet.EvolutionImage)"
                        >
                          <img :src="imagePreviewUrl(currentPet.EvolutionImage)" alt="" class="thumb" />
                        </button>
                      </div>
                    </label>
                  </div>
                </div>
                <div class="stage-card">
                  <div class="section-title">觉醒</div>
                  <div class="form-grid stage-grid">
                    <label class="f-field">
                      <span>觉醒后名称</span>
                      <input v-model="currentPet.Awaken" class="field-input" type="text" @input="markDirty" />
                    </label>
                    <label class="f-field">
                      <span>所需物品</span>
                      <input v-model="currentPet.AwakenItems" class="field-input" type="text" @input="markDirty" />
                    </label>
                    <label class="f-field">
                      <span>所需成长</span>
                      <input v-model.number="currentPet.AwakenGrowth" class="field-input" type="number" @input="markDirty" />
                    </label>
                    <label class="f-field">
                      <span>所需好感</span>
                      <input v-model.number="currentPet.AwakenAffect" class="field-input" type="number" @input="markDirty" />
                    </label>
                    <label class="f-field wide">
                      <span>觉醒图片</span>
                      <div class="img-field">
                        <input v-model="currentPet.AwakenImage" class="field-input" type="text" @input="markDirty" />
                        <label class="btn btn-ghost file-btn">
                          上传
                          <input
                            type="file"
                            accept="image/jpeg,image/png,image/gif,image/webp"
                            hidden
                            @change="doUpload(($event.target as HTMLInputElement).files?.[0], (p) => { currentPet!.AwakenImage = p })"
                          />
                        </label>
                        <button
                          v-if="currentPet.AwakenImage"
                          type="button"
                          class="thumb-btn"
                          @click="openLightbox(currentPet.AwakenImage)"
                        >
                          <img :src="imagePreviewUrl(currentPet.AwakenImage)" alt="" class="thumb" />
                        </button>
                      </div>
                    </label>
                  </div>
                </div>
              </div>

              <div class="section-block">
                <div class="section-title">动作与场景图</div>
                <div class="img-gallery">
                  <div v-for="field in PET_ACTION_IMAGES" :key="field.k" class="img-card compact">
                    <div class="img-card-head">
                      <span>{{ field.l }}</span>
                      <button
                        v-if="(currentPet as any)[field.k]"
                        type="button"
                        class="link-btn"
                        @click="openLightbox((currentPet as any)[field.k])"
                      >
                        预览
                      </button>
                    </div>
                    <button
                      v-if="(currentPet as any)[field.k]"
                      type="button"
                      class="img-card-preview sm"
                      @click="openLightbox((currentPet as any)[field.k])"
                    >
                      <img :src="imagePreviewUrl((currentPet as any)[field.k])" alt="" />
                    </button>
                    <div v-else class="img-card-preview sm empty">暂无</div>
                    <div class="img-field">
                      <input
                        v-model="(currentPet as any)[field.k]"
                        class="field-input"
                        type="text"
                        placeholder="路径"
                        @input="markDirty"
                      />
                      <label class="btn btn-ghost file-btn sm">
                        上传
                        <input
                          type="file"
                          accept="image/jpeg,image/png,image/gif,image/webp"
                          hidden
                          @change="
                            doUpload(($event.target as HTMLInputElement).files?.[0], (p) => {
                              ;(currentPet as any)[field.k] = p
                            })
                          "
                        />
                      </label>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <p v-else class="empty">请选择或新增宠物种类</p>
        </div>

        <!-- 菜单 -->
        <div v-else-if="schema === 'menus'" class="panel">
          <div class="toolbar">
            <input v-model="search" class="field-input search" type="search" placeholder="搜索菜单…" />
            <button type="button" class="btn btn-ghost" @click="addMenu">新增菜单</button>
          </div>
          <p class="hint-line">可用变量：[@QQ]、[qq]、图片标记、换行</p>
          <div class="stack">
            <div v-for="row in filteredMenus" :key="row.Name" class="form-card card">
              <div class="mini-head">
                <input
                  v-model="row.Name"
                  class="field-input title-input"
                  type="text"
                  placeholder="菜单触发名称"
                  :disabled="isPersisted('menus', row.Name)"
                  :title="isPersisted('menus', row.Name) ? '已保存配置的主键不可修改；如需更名请删除后新增' : ''"
                  @input="markDirty"
                />
                <button type="button" class="link-btn danger" @click="removeMenu(row, menuRows.indexOf(row))">
                  删除
                </button>
              </div>
              <label class="f-field wide">
                <span>回复内容</span>
                <textarea v-model="row.Reply" class="field-input area" rows="4" @input="markDirty" />
              </label>
            </div>
          </div>
          <p v-if="!filteredMenus.length" class="empty">暂无菜单</p>
        </div>

        <!-- 图片映射 -->
        <div v-else-if="schema === 'images'" class="panel">
          <div class="toolbar">
            <input v-model="search" class="field-input search" type="search" placeholder="搜索功能名或路径…" />
            <button type="button" class="btn btn-ghost" @click="addImage">新增映射</button>
          </div>
          <div class="table-wrap card">
            <table class="data-table">
              <thead>
                <tr>
                  <th>功能名称</th>
                  <th>相对路径</th>
                  <th>预览</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="!filteredImages.length">
                  <td colspan="4" class="empty-cell">暂无图片映射</td>
                </tr>
                <tr v-for="row in filteredImages" :key="row.Name">
                  <td data-label="功能名称">
                    <input
                      v-model="row.Name"
                      class="field-input cell-input"
                      type="text"
                      :disabled="isPersisted('images', row.Name)"
                      :title="isPersisted('images', row.Name) ? '已保存配置的主键不可修改；如需更名请删除后新增' : ''"
                      @input="markDirty"
                    />
                  </td>
                  <td data-label="相对路径">
                    <div class="img-field compact">
                      <input v-model="row.Path" class="field-input cell-input" type="text" @input="markDirty" />
                      <label class="btn btn-ghost file-btn sm">
                        上传
                        <input
                          type="file"
                          accept="image/jpeg,image/png,image/gif,image/webp"
                          hidden
                          @change="doUpload(($event.target as HTMLInputElement).files?.[0], (p) => { row.Path = p })"
                        />
                      </label>
                    </div>
                  </td>
                  <td data-label="预览">
                    <button v-if="row.Path" type="button" class="thumb-btn" @click="openLightbox(row.Path)">
                      <img :src="imagePreviewUrl(row.Path)" alt="" class="thumb" />
                    </button>
                    <span v-else class="muted">—</span>
                  </td>
                  <td data-label="操作">
                    <button
                      type="button"
                      class="link-btn danger"
                      @click="removeImage(row, imageRows.indexOf(row))"
                    >
                      删除
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-if="uploadingKey" class="page-hint">正在上传 {{ uploadingKey }}…</p>
        </div>
      </div>
    </div>

    <!-- 删除确认 -->
    <UiModal
      id="delete-config"
      :open="deleteOpen"
      title="确认删除配置项"
      description="删除后数据库会立即更新，但需热重载后运行中机器人才能感知变化。"
      :busy="deleteBusy"
      size="small"
      @close="cancelDelete"
    >
        <p class="modal-body">
          即将删除 <strong>{{ deleteLabel }}</strong>。删除可能影响正在运行的游戏功能，且需热重载后内存侧才会去掉该项。
        </p>
        <template #footer>
          <button type="button" class="btn btn-ghost" :disabled="deleteBusy" @click="cancelDelete">取消</button>
          <button type="button" class="btn btn-danger" :disabled="deleteBusy" @click="confirmDelete">
            {{ deleteBusy ? '删除中…' : '确认删除' }}
          </button>
        </template>
    </UiModal>

    <!-- 图片放大 -->
    <div v-if="lightboxUrl" class="modal-mask lightbox" @click="lightboxUrl = ''">
      <img :src="lightboxUrl" alt="预览" class="lightbox-img" @click.stop />
    </div>
  </section>
</template>

<style scoped>
.config-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.page-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.page-title {
  margin: 0 0 4px;
  font-size: 20px;
  font-weight: 600;
}

.page-hint {
  margin: 0;
  color: var(--text-muted);
  font-size: 13px;
}

.page-error {
  margin: 0;
  padding: 10px 14px;
  border-radius: var(--radius-input);
  background: var(--danger-soft);
  color: var(--danger);
}

.toast {
  margin: 0;
  padding: 8px 12px;
  border-radius: var(--radius-input);
  background: var(--success-soft);
  color: var(--text-main);
  font-size: 13px;
}

.status-bar {
  position: sticky;
  top: 74px;
  z-index: 12;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
}

.status-copy {
  display: grid;
  gap: 2px;
}

.status-copy small {
  color: var(--text-muted);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.status-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.status-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--success);
}

.status-dirty .status-dot {
  background: var(--warning);
}

.status-saved .status-dot {
  background: var(--warning);
}

.status-ok .status-dot {
  background: var(--success);
}

.status-text {
  font-size: 13px;
  font-weight: 500;
}

.status-dirty {
  border-color: color-mix(in srgb, var(--warning) 45%, var(--border-color));
  background: color-mix(in srgb, var(--warning-soft) 70%, var(--bg-surface));
}

.status-saved {
  border-color: color-mix(in srgb, var(--warning) 40%, var(--border-color));
  background: color-mix(in srgb, var(--warning-soft) 55%, var(--bg-surface));
}

.status-ok {
  border-color: color-mix(in srgb, var(--success) 35%, var(--border-color));
}

.status-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.config-layout {
  display: grid;
  grid-template-columns: 168px minmax(0, 1fr);
  gap: 14px;
  align-items: start;
}

.subnav {
  display: flex;
  flex-direction: column;
  padding: 8px;
  gap: 2px;
  position: sticky;
  top: 78px;
}

.subnav-item {
  text-align: left;
  padding: 9px 12px;
  border: none;
  border-radius: 10px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 13px;
}

.subnav-item:hover {
  background: var(--accent-soft);
  color: var(--text-main);
}

.subnav-item.active {
  background: var(--accent-soft);
  color: var(--accent);
  font-weight: 600;
  box-shadow: inset 3px 0 0 var(--accent);
}

.config-main {
  min-width: 0;
}

.panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}

.search {
  flex: 1;
  min-width: 180px;
  max-width: 360px;
}

.group-card {
  overflow: hidden;
}

.group-head {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border: none;
  background: var(--bg-elevated);
  color: var(--text-main);
  cursor: pointer;
  font-weight: 600;
  font-size: 14px;
}

.group-meta {
  font-weight: 400;
  font-size: 12px;
  color: var(--text-muted);
}

.group-body {
  padding: 8px 16px 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.kv-row {
  display: grid;
  grid-template-columns: minmax(140px, 240px) 1fr;
  gap: 12px;
  align-items: center;
}

.kv-label {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.kv-title {
  font-size: 13px;
  color: var(--text-main);
}

.kv-key {
  font-size: 11px;
  color: var(--text-muted);
  font-family: ui-monospace, Consolas, monospace;
  word-break: break-all;
}

.table-wrap {
  overflow: auto;
  max-width: 100%;
}

.dense-table .cell-input {
  min-width: 0;
  width: 100%;
  padding: 6px 8px;
  font-size: 12px;
}

.dense-table .num-in {
  max-width: 72px;
}

.dense-table th.col-id {
  width: 48px;
}

.dense-table th.col-name {
  min-width: 96px;
}

.dense-table th.col-type {
  min-width: 72px;
  width: 88px;
}

.dense-table th.col-num {
  width: 72px;
}

.dense-table th.col-img,
.dense-table td:has(.img-field) {
  width: 118px;
  max-width: 118px;
}

/* 表格内图片路径框缩短：完整路径悬停 title 可见，上传+缩略图保留 */
.dense-table .img-field.compact {
  flex-wrap: nowrap;
  max-width: 110px;
  gap: 4px;
}

.dense-table .img-field.compact .field-input {
  flex: 1 1 auto;
  width: 52px;
  min-width: 44px;
  max-width: 56px;
  padding: 5px 6px;
}

.dense-table th.col-req {
  min-width: 80px;
}

.dense-table th.col-shop {
  width: 110px;
}

.dense-table th.col-ops {
  width: 56px;
}

.ops-cell {
  white-space: nowrap;
  text-align: center;
}

.pager {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding: 4px 2px;
}

.pager-info {
  font-size: 12px;
  color: var(--text-muted);
}

.pager-btns {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pager-page {
  font-size: 12px;
  color: var(--text-muted);
  min-width: 4.5em;
  text-align: center;
}

.pet-search {
  margin: 0 8px 8px;
  width: calc(100% - 16px);
  padding: 7px 10px;
  font-size: 12px;
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.data-table th,
.data-table td {
  padding: 8px 10px;
  border-bottom: 1px solid var(--border-color);
  text-align: left;
  vertical-align: middle;
}

.data-table th {
  color: var(--text-muted);
  font-weight: 600;
  white-space: nowrap;
  background: var(--bg-elevated);
}

.data-table tbody tr:hover {
  background: var(--accent-soft);
}

.empty-cell,
.empty {
  color: var(--text-muted);
  text-align: center;
  padding: 20px;
}

.empty.small {
  padding: 12px;
  font-size: 12px;
}

.cell-input {
  width: 100%;
  min-width: 80px;
  padding: 6px 8px;
  font-size: 13px;
}

.num-in {
  max-width: 96px;
}

.num {
  font-variant-numeric: tabular-nums;
  color: var(--text-muted);
}

.link-btn {
  border: none;
  background: none;
  color: var(--accent);
  cursor: pointer;
  padding: 2px 6px;
  font-size: 13px;
}

.link-btn.danger {
  color: var(--danger);
}

.link-btn.muted {
  color: var(--text-muted);
}

.tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  padding: 3px;
  border-radius: 12px;
  border: 1px solid var(--border-color);
  background: var(--bg-base);
}

.tab {
  border: none;
  background: transparent;
  padding: 6px 12px;
  border-radius: 9px;
  cursor: pointer;
  color: var(--text-muted);
  font-size: 13px;
}

.tab.active {
  background: var(--accent);
  color: var(--accent-ink);
}

.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 12px;
}

.mini-card,
.form-card {
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.mini-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 12px;
}

.f-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: var(--text-muted);
}

.f-field.wide {
  grid-column: 1 / -1;
}

.f-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.area {
  resize: vertical;
  min-height: 56px;
}

.title-input {
  font-weight: 600;
  font-size: 14px;
  flex: 1;
}

.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.img-field {
  display: flex;
  align-items: center;
  gap: 6px;
}

.img-field .field-input {
  flex: 1;
  min-width: 0;
}

.img-field.compact {
  flex-wrap: wrap;
}

.file-btn {
  padding: 6px 10px;
  font-size: 12px;
  flex-shrink: 0;
  cursor: pointer;
}

.file-btn.sm {
  padding: 4px 8px;
}

.thumb-btn {
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 0;
  background: var(--bg-base);
  cursor: pointer;
  overflow: hidden;
  width: 40px;
  height: 40px;
  flex-shrink: 0;
}

.thumb {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.hint-line {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
}

.muted {
  color: var(--text-muted);
}

/* 宠物双栏 */
.pet-panel {
  display: grid;
  grid-template-columns: 200px minmax(0, 1fr);
  gap: 12px;
  align-items: start;
  min-width: 0;
  overflow-x: hidden;
}

.pet-list {
  padding: 8px;
  max-height: 70vh;
  overflow: auto;
  /* 与主题滚动条一致，避免系统默认白条 */
  scrollbar-gutter: stable;
}

.pet-list-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 8px 10px;
  font-weight: 600;
  font-size: 13px;
}

.sm-btn {
  padding: 4px 10px;
  font-size: 12px;
}

.pet-list-item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  border: none;
  border-radius: 10px;
  background: transparent;
  cursor: pointer;
  color: var(--text-main);
  text-align: left;
}

.pet-list-item:hover {
  background: var(--accent-soft);
}

.pet-list-item.active {
  background: var(--accent-soft);
  box-shadow: inset 3px 0 0 var(--accent);
}

.pet-avatar {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  object-fit: cover;
  background: var(--bg-elevated);
  border: 1px solid var(--border-color);
}

.pet-avatar.placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  color: var(--text-muted);
}

.pet-name {
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pet-form {
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.pet-tabs {
  overflow-x: auto;
  flex-shrink: 0;
}

.pet-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.section-block {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.section-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-main);
  letter-spacing: 0.02em;
}

/* 概况：图片在文案下方，两列且不撑出横向滚动 */
.pet-profile-imgs {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  min-width: 0;
}

.pet-form,
.pet-section,
.section-block,
.img-card,
.img-field {
  min-width: 0;
}

.img-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px;
  border: 1px solid var(--border-color);
  border-radius: 12px;
  background: var(--bg-base);
}

.img-card.compact {
  padding: 8px;
}

.img-card-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
  color: var(--text-muted);
}

.img-card-preview {
  display: block;
  /* 固定方形：避免 width:100% + max-height 被压成横条 */
  width: min(100%, 168px);
  aspect-ratio: 1;
  height: auto;
  margin: 0 auto;
  border: 1px solid var(--border-color);
  border-radius: 10px;
  overflow: hidden;
  padding: 0;
  background: var(--bg-elevated);
  cursor: pointer;
}

.img-card-preview.sm {
  width: min(100%, 120px);
  aspect-ratio: 1;
}

.img-card-preview img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  display: block;
}

.img-card-preview.empty {
  display: grid;
  place-items: center;
  font-size: 12px;
  color: var(--text-muted);
  cursor: default;
}

/* 属性对照表 */
.attr-table {
  border: 1px solid var(--border-color);
  border-radius: 12px;
  overflow: hidden;
  background: var(--bg-base);
}

.attr-row {
  display: grid;
  grid-template-columns: 72px 1fr 1fr;
  gap: 10px;
  align-items: center;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-color);
}

.attr-row:last-child {
  border-bottom: none;
}

.attr-row.head {
  background: var(--bg-elevated);
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  padding: 10px 12px;
}

.attr-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-main);
}

.attr-cell {
  min-width: 0;
}

.attr-cell .field-input {
  width: 100%;
}

.bonus-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

/* 进化 / 觉醒双栏 */
.pet-dual-cards {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.stage-card {
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: 12px;
  background: var(--bg-base);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.stage-grid {
  grid-template-columns: 1fr 1fr;
}

.img-gallery {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 10px;
}

.modal-mask {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: color-mix(in srgb, var(--bg-base) 35%, rgba(20, 10, 16, 0.55));
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.modal {
  width: min(420px, 100%);
  padding: 20px;
}

.modal-title {
  margin: 0 0 10px;
  font-size: 17px;
}

.modal-body {
  margin: 0 0 18px;
  color: var(--text-muted);
  font-size: 13px;
  line-height: 1.6;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.lightbox {
  background: rgba(10, 6, 10, 0.75);
}

.lightbox-img {
  max-width: min(90vw, 900px);
  max-height: 85vh;
  border-radius: var(--radius-card);
  box-shadow: var(--shadow-soft);
  border: 1px solid var(--border-color);
}

@media (max-width: 900px) {
  .config-layout {
    grid-template-columns: 1fr;
  }

  .subnav {
    position: static;
    flex-direction: row;
    flex-wrap: nowrap;
    overflow-x: auto;
    scroll-snap-type: x proximity;
  }

  .subnav-item { flex: 0 0 auto; scroll-snap-align: start; }

  .kv-row {
    grid-template-columns: 1fr;
  }

  .pet-panel {
    grid-template-columns: 1fr;
  }

  .pet-profile-imgs {
    grid-template-columns: 1fr;
  }

  .pet-dual-cards {
    grid-template-columns: 1fr;
  }

  .bonus-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 700px) {
  .status-bar { top: 64px; align-items: stretch; }
  .status-left { align-items: flex-start; }
  .status-actions { display: grid; grid-template-columns: 1fr 1fr; }
  .status-actions .btn { width: 100%; }
  .table-wrap { overflow: visible; border: 0; background: transparent; box-shadow: none; }
  .data-table,
  .data-table tbody,
  .data-table tr,
  .data-table td { display: block; width: 100%; }
  .data-table thead { display: none; }
  .data-table tr { margin-bottom: 12px; padding: 10px; border: 1px solid var(--border-color); border-radius: 14px; background: var(--bg-surface); box-shadow: var(--shadow-card); }
  .data-table td { padding: 6px; border: 0; }
  .data-table td[data-label] {
    display: grid;
    grid-template-columns: 86px minmax(0, 1fr);
    gap: 10px;
    align-items: start;
  }
  .data-table td[data-label]::before {
    content: attr(data-label);
    padding-top: 9px;
    color: var(--text-muted);
    font-size: 11px;
    font-weight: 650;
  }
  .data-table td[data-label].num { text-align: left; }
  .data-table td:empty { display: none; }
  .dense-table .cell-input,
  .dense-table .num-in { width: 100%; min-width: 0; }
  .bonus-grid { grid-template-columns: 1fr; }
}
</style>
