<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ApiError } from '../api/client'
import {
  deletePet,
  fetchItemNames,
  fetchPets,
  fetchSpeciesNames,
  giveItem,
  operatePet,
  type PetOperateAction,
  type PetUpdatePayload,
  type UserPet,
  updatePet,
} from '../api/pets'
import PageHeader from '../components/ui/PageHeader.vue'
import UiModal from '../components/ui/UiModal.vue'
import UiState from '../components/ui/UiState.vue'
import { useToast } from '../composables/useToast'
import { confirmUnsavedNavigation, useUnsavedChanges } from '../composables/useUnsavedChanges'

const STATUS_OPTIONS = [
  '空闲',
  '打工',
  '学习',
  '锻炼',
  '健身',
  '濒死',
  '逃跑',
  '失去宠物',
] as const

const route = useRoute()
const router = useRouter()

const filters = reactive({
  user_id: '',
  group_id: '',
  name: '',
  status: '',
  pet_type: '',
})

const page = ref(1)
const limit = ref(10)
const total = ref(0)
const rows = ref<UserPet[]>([])
const loading = ref(false)
const error = ref('')
const toast = useToast()

const speciesOptions = ref<string[]>([])
const itemOptions = ref<string[]>([])

// 编辑抽屉
const drawerOpen = ref(false)
const editing = ref<UserPet | null>(null)
const form = reactive<PetUpdatePayload>({
  name: '',
  status: '',
  mood: '',
  mood_points: 0,
  affection: 0,
  growth: 0,
  health: 0,
  wisdom: 0,
  strength: 0,
  defense: 0,
  hunger: 0,
  currency: 0,
  family: '',
  family_score: 0,
})
const saving = ref(false)
const drawerError = ref('')
const drawerNotice = ref('')
const drawerSnapshot = ref('')

// 删除确认
const deleteTarget = ref<UserPet | null>(null)
const deleting = ref(false)

// 背包调整
const bagOpen = ref(false)
const bagForm = reactive({
  user_id: '',
  group_id: '',
  item_name: '',
  quantity: 1 as number | string,
})
const bagSubmitting = ref(false)
const bagError = ref('')
const bagNotice = ref('')

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / limit.value)))

const bagQty = computed(() => Number(bagForm.quantity) || 0)
const bagDirection = computed(() => {
  if (bagQty.value > 0) return '增加'
  if (bagQty.value < 0) return '扣除'
  return '不变'
})

function showToast(msg: string) { toast.success(msg) }

function currentPayload(): PetUpdatePayload {
  return {
    name: form.name,
    status: form.status,
    mood: form.mood,
    mood_points: Number(form.mood_points),
    affection: Number(form.affection),
    growth: Number(form.growth),
    health: Number(form.health),
    wisdom: Number(form.wisdom),
    strength: Number(form.strength),
    defense: Number(form.defense),
    hunger: Number(form.hunger),
    currency: Number(form.currency),
    family: form.family,
    family_score: Number(form.family_score),
  }
}

const drawerDirty = computed(
  () => drawerOpen.value && drawerSnapshot.value !== JSON.stringify(currentPayload()),
)
useUnsavedChanges(drawerDirty)

function formatTime(v: string | null | undefined) {
  if (!v) return '—'
  try {
    const d = new Date(v)
    if (Number.isNaN(d.getTime())) return v
    return d.toLocaleString('zh-CN', { hour12: false })
  } catch {
    return v
  }
}

function statusClass(status: string) {
  if (status === '濒死' || status === '逃跑' || status === '失去宠物') return 'badge badge-danger'
  if (status === '空闲') return 'badge badge-success'
  return 'badge'
}

function applyQueryFromRoute() {
  const q = route.query
  if (typeof q.user_id === 'string') filters.user_id = q.user_id
  if (typeof q.group_id === 'string') filters.group_id = q.group_id
  if (typeof q.name === 'string') filters.name = q.name
  if (typeof q.status === 'string') filters.status = q.status
  if (typeof q.pet_type === 'string') filters.pet_type = q.pet_type
  if (typeof q.page === 'string' && Number(q.page) > 0) page.value = Number(q.page)
}

function syncRouteQuery() {
  const query: Record<string, string> = {}
  if (filters.user_id) query.user_id = filters.user_id
  if (filters.group_id) query.group_id = filters.group_id
  if (filters.name) query.name = filters.name
  if (filters.status) query.status = filters.status
  if (filters.pet_type) query.pet_type = filters.pet_type
  if (page.value > 1) query.page = String(page.value)
  router.replace({ name: 'pets', query })
}

async function loadList() {
  loading.value = true
  error.value = ''
  try {
    const res = await fetchPets({
      user_id: filters.user_id.trim() || undefined,
      group_id: filters.group_id.trim() || undefined,
      name: filters.name.trim() || undefined,
      status: filters.status || undefined,
      pet_type: filters.pet_type || undefined,
      page: page.value,
      limit: limit.value,
    })
    rows.value = res.data
    total.value = res.total
    page.value = res.page
    limit.value = res.limit
    const targetPetId = Number(route.query.pet_id)
    syncRouteQuery()
    if (targetPetId) {
      const target = rows.value.find((pet) => pet.id === targetPetId)
      if (target) openEdit(target)
    }
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : '加载宠物列表失败'
    rows.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function onSearch() {
  page.value = 1
  void loadList()
}

function onReset() {
  filters.user_id = ''
  filters.group_id = ''
  filters.name = ''
  filters.status = ''
  filters.pet_type = ''
  page.value = 1
  void loadList()
}

function goPage(p: number) {
  if (p < 1 || p > totalPages.value || p === page.value) return
  page.value = p
  void loadList()
}

function openEdit(pet: UserPet) {
  editing.value = pet
  form.name = pet.name
  form.status = pet.status
  form.mood = pet.mood
  form.mood_points = pet.mood_points
  form.affection = pet.affection
  form.growth = pet.growth
  form.health = pet.health
  form.wisdom = pet.wisdom
  form.strength = pet.strength
  form.defense = pet.defense
  form.hunger = pet.hunger
  form.currency = pet.currency
  form.family = pet.family
  form.family_score = pet.family_score
  drawerError.value = ''
  drawerNotice.value = ''
  drawerSnapshot.value = JSON.stringify(currentPayload())
  drawerOpen.value = true
}

function closeDrawer() {
  if (!confirmUnsavedNavigation(drawerDirty)) return
  drawerOpen.value = false
  editing.value = null
  drawerSnapshot.value = ''
}

async function saveEdit() {
  if (!editing.value || saving.value) return
  saving.value = true
  drawerError.value = ''
  drawerNotice.value = ''
  try {
    const payload = currentPayload()
    const updated = await updatePet(editing.value.id, payload)
    editing.value = updated
    drawerSnapshot.value = JSON.stringify(payload)
    drawerNotice.value = '宠物属性更新成功'
    showToast('宠物属性更新成功')
    // 同步表格行
    const idx = rows.value.findIndex((r) => r.id === updated.id)
    if (idx >= 0) rows.value[idx] = updated
  } catch (err) {
    drawerError.value = err instanceof ApiError ? err.message : '保存失败'
  } finally {
    saving.value = false
  }
}

async function doOperate(pet: UserPet, action: PetOperateAction) {
  const labels: Record<PetOperateAction, string> = {
    revive: '复活',
    recall: '召回',
    clear_cooldown: '清冷却',
  }
  try {
    const updated = await operatePet(pet.id, action)
    showToast(`${labels[action]}成功`)
    const idx = rows.value.findIndex((r) => r.id === updated.id)
    if (idx >= 0) rows.value[idx] = updated
    if (editing.value?.id === updated.id) {
      editing.value = updated
      form.status = updated.status
      form.health = updated.health
      drawerSnapshot.value = JSON.stringify(currentPayload())
    }
  } catch (err) {
    const msg = err instanceof ApiError ? err.message : `${labels[action]}失败`
    toast.error(msg)
    if (drawerOpen.value) drawerError.value = msg
  }
}

function askDelete(pet: UserPet) {
  deleteTarget.value = pet
}

function cancelDelete() {
  deleteTarget.value = null
}

async function confirmDelete() {
  if (!deleteTarget.value || deleting.value) return
  deleting.value = true
  try {
    const msg = await deletePet(deleteTarget.value.id)
    showToast(msg)
    if (editing.value?.id === deleteTarget.value.id) {
      drawerSnapshot.value = JSON.stringify(currentPayload())
      closeDrawer()
    }
    deleteTarget.value = null
    await loadList()
  } catch (err) {
    toast.error(err instanceof ApiError ? err.message : '删除失败')
  } finally {
    deleting.value = false
  }
}

function openBag(pet?: UserPet) {
  bagError.value = ''
  bagNotice.value = ''
  if (pet) {
    bagForm.user_id = String(pet.user_id)
    bagForm.group_id = String(pet.group_id)
  } else if (editing.value) {
    bagForm.user_id = String(editing.value.user_id)
    bagForm.group_id = String(editing.value.group_id)
  }
  bagForm.item_name = ''
  bagForm.quantity = 1
  bagOpen.value = true
}

function closeBag() {
  bagOpen.value = false
}

async function submitBag() {
  if (bagSubmitting.value) return
  bagError.value = ''
  bagNotice.value = ''
  const uid = Number(bagForm.user_id)
  const gid = Number(bagForm.group_id)
  const qty = Number(bagForm.quantity)
  if (!uid || !bagForm.item_name.trim()) {
    bagError.value = '玩家 QQ 与物品名称不能为空'
    return
  }
  if (!qty || Number.isNaN(qty)) {
    bagError.value = '请填写非零的调整数量'
    return
  }
  bagSubmitting.value = true
  try {
    const msg = await giveItem({
      user_id: uid,
      group_id: Number.isNaN(gid) ? 0 : gid,
      item_name: bagForm.item_name.trim(),
      quantity: qty,
    })
    bagNotice.value = msg
    showToast(msg)
  } catch (err) {
    bagError.value = err instanceof ApiError ? err.message : '调整失败'
  } finally {
    bagSubmitting.value = false
  }
}

onMounted(async () => {
  applyQueryFromRoute()
  const listPromise = loadList()
  const [species, items] = await Promise.allSettled([fetchSpeciesNames(), fetchItemNames()])
  if (species.status === 'fulfilled') speciesOptions.value = species.value
  else toast.warning('宠物种类选项加载失败，列表仍可正常使用')
  if (items.status === 'fulfilled') itemOptions.value = items.value
  else toast.warning('道具选项加载失败，背包调整可稍后重试')
  await listPromise
})

watch(
  () => route.query.status,
  (st) => {
    if (typeof st === 'string' && st !== filters.status) {
      filters.status = st
      page.value = 1
      void loadList()
    }
  },
)
</script>

<template>
  <section class="pets-page">
    <PageHeader eyebrow="Players & Pets" title="玩家与宠物" description="查询存档、编辑属性、处理异常状态并预览背包调整。">
      <template #actions><button type="button" class="btn btn-ghost" @click="openBag()">背包调整</button></template>
    </PageHeader>

    <!-- 筛选 -->
    <form class="filter-bar card" @submit.prevent="onSearch">
      <label class="f-field">
        <span>玩家 QQ</span>
        <input v-model="filters.user_id" class="field-input" type="text" inputmode="numeric" placeholder="完整 QQ 号" />
      </label>
      <label class="f-field">
        <span>群号</span>
        <input v-model="filters.group_id" class="field-input" type="text" inputmode="numeric" placeholder="QQ 群号" />
      </label>
      <label class="f-field">
        <span>昵称</span>
        <input v-model="filters.name" class="field-input" type="text" placeholder="模糊搜索" />
      </label>
      <label class="f-field">
        <span>状态</span>
        <select v-model="filters.status" class="field-input">
          <option value="">全部</option>
          <option v-for="s in STATUS_OPTIONS" :key="s" :value="s">{{ s }}</option>
        </select>
      </label>
      <label class="f-field">
        <span>种类</span>
        <select v-if="speciesOptions.length" v-model="filters.pet_type" class="field-input">
          <option value="">全部</option>
          <option v-for="sp in speciesOptions" :key="sp" :value="sp">{{ sp }}</option>
        </select>
        <input
          v-else
          v-model="filters.pet_type"
          class="field-input"
          type="text"
          placeholder="种类名称"
        />
      </label>
      <div class="filter-actions">
        <button type="submit" class="btn" :disabled="loading">查询</button>
        <button type="button" class="btn btn-ghost" :disabled="loading" @click="onReset">重置</button>
      </div>
    </form>

    <UiState v-if="loading" tone="loading" title="正在加载宠物列表" description="正在按当前筛选条件读取存档。" />
    <UiState v-else-if="error" tone="error" title="宠物列表加载失败" :description="error" action-label="重试" @action="loadList" />
    <UiState v-else-if="!rows.length" tone="empty" title="没有匹配的宠物" description="调整筛选条件后再试，或重置为全部记录。" action-label="重置筛选" @action="onReset" />

    <!-- 表格 -->
    <div v-else class="table-wrap card">
      <table class="data-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>玩家 QQ</th>
            <th>群号</th>
            <th>种类</th>
            <th>昵称</th>
            <th>状态</th>
            <th>心情</th>
            <th>货币</th>
            <th>成长</th>
            <th>好感</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="pet in rows" :key="pet.id">
            <td class="num" data-label="ID">{{ pet.id }}</td>
            <td class="num" data-label="玩家 QQ">{{ pet.user_id }}</td>
            <td class="num" data-label="群号">{{ pet.group_id }}</td>
            <td data-label="种类">{{ pet.pet_type || '—' }}</td>
            <td data-label="昵称">{{ pet.name || '未命名' }}</td>
            <td data-label="状态">
              <span :class="statusClass(pet.status)">{{ pet.status || '未知' }}</span>
            </td>
            <td data-label="心情">{{ pet.mood || '—' }}</td>
            <td class="num" data-label="货币">{{ pet.currency }}</td>
            <td class="num" data-label="成长">{{ pet.growth }}</td>
            <td class="num" data-label="好感">{{ pet.affection }}</td>
            <td class="ops" data-label="操作">
              <button type="button" class="link-btn" @click="openEdit(pet)">编辑</button>
              <button type="button" class="link-btn" @click="doOperate(pet, 'revive')">复活</button>
              <button type="button" class="link-btn" @click="doOperate(pet, 'recall')">召回</button>
              <button type="button" class="link-btn" @click="doOperate(pet, 'clear_cooldown')">清冷却</button>
              <button type="button" class="link-btn danger" @click="askDelete(pet)">删除</button>
              <button type="button" class="link-btn" @click="openBag(pet)">背包</button>
            </td>
          </tr>
        </tbody>
      </table>

      <div class="pager">
        <span class="pager-meta">共 {{ total }} 条 · 第 {{ page }} / {{ totalPages }} 页</span>
        <div class="pager-btns">
          <button type="button" class="btn btn-ghost" :disabled="page <= 1 || loading" @click="goPage(page - 1)">
            上一页
          </button>
          <button
            type="button"
            class="btn btn-ghost"
            :disabled="page >= totalPages || loading"
            @click="goPage(page + 1)"
          >
            下一页
          </button>
        </div>
      </div>
    </div>

    <!-- 编辑抽屉 -->
    <div v-if="drawerOpen && editing" class="drawer-mask" @click.self="closeDrawer">
      <aside class="drawer" role="dialog" aria-modal="true" aria-labelledby="drawer-title" tabindex="-1" @keydown.esc="closeDrawer">
        <header class="drawer-head">
          <div>
            <h2 id="drawer-title" class="drawer-title">编辑宠物存档</h2>
            <p class="page-hint">ID {{ editing.id }} · {{ editing.name || '未命名' }} <span v-if="drawerDirty" class="dirty-badge">有未保存修改</span></p>
          </div>
          <button type="button" class="btn btn-ghost" @click="closeDrawer">{{ drawerDirty ? '放弃修改' : '关闭' }}</button>
        </header>

        <div class="drawer-body">
          <section class="group readonly">
            <h3 class="group-title">身份（只读）</h3>
            <div class="readonly-grid">
              <div><span class="k">玩家 QQ</span><strong>{{ editing.user_id }}</strong></div>
              <div><span class="k">群号</span><strong>{{ editing.group_id }}</strong></div>
              <div><span class="k">种类</span><strong>{{ editing.pet_type || '—' }}</strong></div>
              <div><span class="k">绑定密钥</span><strong class="mono">{{ editing.bind_key || '—' }}</strong></div>
              <div class="img-row">
                <span class="k">当前图</span>
                <span class="mono">{{ editing.image || '—' }}</span>
              </div>
              <div><span class="k">最近更新</span><strong>{{ formatTime(editing.updated_at) }}</strong></div>
            </div>
          </section>

          <section class="group">
            <h3 class="group-title">状态</h3>
            <div class="form-grid">
              <label class="f-field">
                <span>昵称</span>
                <input v-model="form.name" class="field-input" type="text" />
              </label>
              <label class="f-field">
                <span>状态</span>
                <select v-model="form.status" class="field-input">
                  <option v-for="s in STATUS_OPTIONS" :key="s" :value="s">{{ s }}</option>
                  <option v-if="form.status && !(STATUS_OPTIONS as readonly string[]).includes(form.status)" :value="form.status">
                    {{ form.status }}
                  </option>
                </select>
              </label>
              <label class="f-field">
                <span>心情文案</span>
                <input v-model="form.mood" class="field-input" type="text" />
              </label>
              <label class="f-field">
                <span>心情点</span>
                <input v-model.number="form.mood_points" class="field-input" type="number" />
              </label>
            </div>
          </section>

          <section class="group">
            <h3 class="group-title">生存</h3>
            <div class="form-grid">
              <label class="f-field">
                <span>生命</span>
                <input v-model.number="form.health" class="field-input" type="number" />
              </label>
              <label class="f-field">
                <span>饱食</span>
                <input v-model.number="form.hunger" class="field-input" type="number" />
              </label>
            </div>
          </section>

          <section class="group">
            <h3 class="group-title">成长</h3>
            <div class="form-grid">
              <label class="f-field">
                <span>好感</span>
                <input v-model.number="form.affection" class="field-input" type="number" />
              </label>
              <label class="f-field">
                <span>成长</span>
                <input v-model.number="form.growth" class="field-input" type="number" />
              </label>
              <label class="f-field">
                <span>智慧</span>
                <input v-model.number="form.wisdom" class="field-input" type="number" />
              </label>
              <label class="f-field">
                <span>力量</span>
                <input v-model.number="form.strength" class="field-input" type="number" />
              </label>
              <label class="f-field">
                <span>防御</span>
                <input v-model.number="form.defense" class="field-input" type="number" />
              </label>
            </div>
          </section>

          <section class="group">
            <h3 class="group-title">经济与家族</h3>
            <div class="form-grid">
              <label class="f-field">
                <span>货币</span>
                <input v-model.number="form.currency" class="field-input" type="number" />
              </label>
              <label class="f-field">
                <span>家族名</span>
                <input v-model="form.family" class="field-input" type="text" />
              </label>
              <label class="f-field">
                <span>家族积分</span>
                <input v-model.number="form.family_score" class="field-input" type="number" />
              </label>
            </div>
          </section>

          <section class="group readonly">
            <h3 class="group-title">任务与异常（只读）</h3>
            <div class="readonly-grid">
              <div>
                <span class="k">学习</span>
                <strong>{{ formatTime(editing.study_time) }} / {{ editing.study_item || '—' }}</strong>
              </div>
              <div>
                <span class="k">锻炼</span>
                <strong>{{ formatTime(editing.train_time) }} / {{ editing.train_item || '—' }}</strong>
              </div>
              <div>
                <span class="k">打工</span>
                <strong>{{ formatTime(editing.work_time) }} / {{ editing.work_type || '—' }}</strong>
              </div>
              <div>
                <span class="k">健身</span>
                <strong>{{ formatTime(editing.fitness_time) }} / {{ editing.fitness_item || '—' }}</strong>
              </div>
              <div>
                <span class="k">濒死</span>
                <strong>{{ formatTime(editing.dying_time) }}</strong>
              </div>
              <div>
                <span class="k">逃跑</span>
                <strong>{{ formatTime(editing.escape_time) }}</strong>
              </div>
              <div>
                <span class="k">失去宠物</span>
                <strong>{{ formatTime(editing.lost_time) }}</strong>
              </div>
              <div>
                <span class="k">新手签到天数</span>
                <strong>{{ editing.newbie_check }}</strong>
              </div>
              <div>
                <span class="k">最近签到</span>
                <strong>{{ formatTime(editing.last_checkin) }}</strong>
              </div>
            </div>
          </section>

          <p v-if="drawerError" class="page-error" role="alert">{{ drawerError }}</p>
          <p v-else-if="drawerNotice" class="page-ok" role="status">{{ drawerNotice }}</p>
        </div>

        <footer class="drawer-foot">
          <button type="button" class="btn" :disabled="saving" @click="saveEdit">
            {{ saving ? '保存中…' : '保存' }}
          </button>
          <button type="button" class="btn btn-ghost" @click="doOperate(editing, 'revive')">复活</button>
          <button type="button" class="btn btn-ghost" @click="doOperate(editing, 'recall')">召回</button>
          <button type="button" class="btn btn-ghost" @click="doOperate(editing, 'clear_cooldown')">清冷却</button>
          <button type="button" class="btn btn-ghost" @click="openBag(editing)">背包调整</button>
          <button type="button" class="btn btn-danger" @click="askDelete(editing)">删除宠物存档</button>
        </footer>
      </aside>
    </div>

    <!-- 删除确认 -->
    <UiModal :open="Boolean(deleteTarget)" title="删除宠物存档" description="此操作将永久删除宠物记录且不可恢复。" :busy="deleting" size="small" @close="cancelDelete">
        <p class="modal-desc">
          此操作将永久删除该宠物记录，且不可恢复。请确认以下信息无误后再继续。
        </p>
        <ul class="del-meta">
          <li><span>宠物昵称</span><strong>{{ deleteTarget?.name || '未命名' }}</strong></li>
          <li><span>玩家 QQ</span><strong>{{ deleteTarget?.user_id }}</strong></li>
          <li><span>群号</span><strong>{{ deleteTarget?.group_id }}</strong></li>
          <li><span>宠物 ID</span><strong>{{ deleteTarget?.id }}</strong></li>
        </ul>
        <template #footer>
          <button type="button" class="btn btn-ghost" :disabled="deleting" @click="cancelDelete">取消</button>
          <button type="button" class="btn btn-danger" :disabled="deleting" @click="confirmDelete">
            {{ deleting ? '删除中…' : '删除宠物存档' }}
          </button>
        </template>
    </UiModal>

    <!-- 背包调整 -->
    <UiModal :open="bagOpen" title="玩家背包调整" description="先预览目标和数量变化，再确认提交。" :busy="bagSubmitting" @close="closeBag">
        <p class="modal-desc">数量为正数时增加，为负数时扣除；扣至 0 及以下将删除该背包记录。</p>

        <div class="form-grid bag-form">
          <label class="f-field">
            <span>玩家 QQ</span>
            <input v-model="bagForm.user_id" class="field-input" type="text" inputmode="numeric" />
          </label>
          <label class="f-field">
            <span>QQ 群号</span>
            <input v-model="bagForm.group_id" class="field-input" type="text" inputmode="numeric" />
          </label>
          <label class="f-field wide">
            <span>物品名称</span>
            <input
              v-model="bagForm.item_name"
              class="field-input"
              type="text"
              list="item-name-list"
              placeholder="与游戏内道具名一致"
            />
            <datalist id="item-name-list">
              <option v-for="n in itemOptions" :key="n" :value="n" />
            </datalist>
          </label>
          <label class="f-field">
            <span>调整数量</span>
            <input v-model="bagForm.quantity" class="field-input" type="number" />
          </label>
        </div>

        <div class="bag-summary">
          <span
            class="dir-tag"
            :class="{
              'dir-plus': bagQty > 0,
              'dir-minus': bagQty < 0,
            }"
          >
            本次：{{ bagDirection }}
          </span>
          <p>
            目标玩家 <strong>{{ bagForm.user_id || '—' }}</strong>
            · 群 <strong>{{ bagForm.group_id || '0' }}</strong>
            · 物品 <strong>{{ bagForm.item_name || '—' }}</strong>
            · 数量 <strong>{{ bagForm.quantity }}</strong>
          </p>
        </div>

        <p v-if="bagError" class="page-error" role="alert">{{ bagError }}</p>
        <p v-else-if="bagNotice" class="page-ok" role="status">{{ bagNotice }}</p>

        <template #footer>
          <button type="button" class="btn btn-ghost" :disabled="bagSubmitting" @click="closeBag">取消</button>
          <button type="button" class="btn" :disabled="bagSubmitting" @click="submitBag">
            {{ bagSubmitting ? '提交中…' : '确认调整' }}
          </button>
        </template>
    </UiModal>
  </section>
</template>

<style scoped>
.pets-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.page-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.page-title {
  margin: 0 0 6px;
  font-size: 22px;
  font-weight: 700;
}

.page-hint {
  margin: 0;
  color: var(--text-muted);
  font-size: 13px;
}

.page-error {
  margin: 0;
  color: var(--danger);
  font-size: 13px;
}

.page-ok {
  margin: 0;
  color: var(--success);
  font-size: 13px;
}

.toast {
  margin: 0;
  padding: 10px 14px;
  border-radius: var(--radius-input);
  background: var(--accent-soft);
  color: var(--text-main);
  border: 1px solid var(--border-color);
  font-size: 13px;
}

.filter-bar {
  position: sticky;
  top: 74px;
  z-index: 10;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 12px;
  padding: 14px 16px;
  align-items: end;
}

.dirty-badge { display: inline-flex; margin-left: 6px; padding: 1px 7px; border-radius: 7px; background: var(--warning-soft); color: var(--warning-strong); font-size: 11px; }

.f-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  color: var(--text-muted);
}

.f-field.wide {
  grid-column: 1 / -1;
}

.filter-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.table-wrap {
  padding: 0;
  overflow: auto;
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.data-table th,
.data-table td {
  padding: 10px 12px;
  text-align: left;
  border-bottom: 1px solid var(--border-color);
  white-space: nowrap;
}

.data-table th {
  font-weight: 600;
  color: var(--text-muted);
  background: var(--bg-elevated);
  position: sticky;
  top: 0;
}

.data-table tbody tr:nth-child(even) {
  background: var(--accent-soft);
}

.data-table tbody tr:hover {
  background: var(--bg-elevated);
}

.num {
  font-variant-numeric: tabular-nums;
}

.empty-cell {
  text-align: center;
  color: var(--text-muted);
  padding: 28px 12px !important;
}

.ops {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 8px;
}

.link-btn {
  border: none;
  background: transparent;
  color: var(--accent);
  cursor: pointer;
  padding: 0;
  font-size: 12px;
}

.link-btn:hover {
  text-decoration: underline;
}

.link-btn.danger {
  color: var(--danger);
}

.pager {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  flex-wrap: wrap;
}

.pager-meta {
  font-size: 13px;
  color: var(--text-muted);
}

.pager-btns {
  display: flex;
  gap: 8px;
}

/* 抽屉 */
.drawer-mask {
  position: fixed;
  inset: 0;
  z-index: 40;
  background: rgba(40, 24, 32, 0.45);
  display: flex;
  justify-content: flex-end;
}

.drawer {
  width: min(560px, 100vw);
  height: 100%;
  background: var(--bg-surface);
  border-left: 1px solid var(--border-color);
  box-shadow: var(--shadow-soft);
  display: flex;
  flex-direction: column;
  animation: slide-in 0.22s ease;
}

@keyframes slide-in {
  from {
    transform: translateX(16px);
    opacity: 0.6;
  }
  to {
    transform: translateX(0);
    opacity: 1;
  }
}

.drawer-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  padding: 18px 18px 12px;
  border-bottom: 1px solid var(--border-color);
}

.drawer-title {
  margin: 0 0 4px;
  font-size: 17px;
  font-weight: 700;
}

.drawer-body {
  flex: 1;
  overflow: auto;
  padding: 14px 18px 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.drawer-foot {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 12px 18px 18px;
  border-top: 1px solid var(--border-color);
  background: var(--bg-elevated);
}

.group {
  padding: 12px 14px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-card);
  background: var(--bg-base);
}

.group.readonly {
  background: var(--bg-elevated);
}

.group-title {
  margin: 0 0 10px;
  font-size: 13px;
  font-weight: 600;
  color: var(--accent);
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.readonly-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px 14px;
  font-size: 13px;
}

.readonly-grid .k {
  display: block;
  font-size: 11px;
  color: var(--text-muted);
  margin-bottom: 2px;
}

.readonly-grid strong {
  font-weight: 600;
  word-break: break-all;
}

.mono {
  font-family: ui-monospace, 'Cascadia Code', Consolas, monospace;
  font-size: 12px;
}

.img-row {
  grid-column: 1 / -1;
}

/* 弹窗 */
.modal-mask {
  position: fixed;
  inset: 0;
  z-index: 50;
  background: rgba(40, 24, 32, 0.5);
  display: grid;
  place-items: center;
  padding: 16px;
}

.modal {
  width: min(420px, 100%);
  padding: 20px;
  background: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-card);
  box-shadow: var(--shadow-soft);
}

.danger-modal {
  border-color: var(--danger);
  box-shadow: 0 0 0 1px var(--danger-soft), var(--shadow-soft);
}

.bag-modal {
  width: min(480px, 100%);
}

.modal-title {
  margin: 0 0 8px;
  font-size: 17px;
  font-weight: 700;
}

.modal-desc {
  margin: 0 0 14px;
  font-size: 13px;
  color: var(--text-muted);
}

.del-meta {
  list-style: none;
  margin: 0 0 16px;
  padding: 12px;
  border-radius: var(--radius-input);
  background: var(--danger-soft);
  border: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.del-meta li {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
}

.del-meta span {
  color: var(--text-muted);
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
}

.bag-form {
  margin-bottom: 12px;
}

.bag-summary {
  margin-bottom: 12px;
  padding: 10px 12px;
  border-radius: var(--radius-input);
  background: var(--accent-soft);
  border: 1px solid var(--border-color);
  font-size: 13px;
}

.bag-summary p {
  margin: 8px 0 0;
  color: var(--text-muted);
}

.dir-tag {
  display: inline-flex;
  padding: 2px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  background: var(--info);
  color: var(--text-main);
}

.dir-plus {
  background: var(--success-soft);
  color: var(--success);
}

.dir-minus {
  background: var(--danger-soft);
  color: var(--danger);
}

@media (max-width: 640px) {
  .filter-bar { position: static; grid-template-columns: 1fr; }
  .filter-actions { grid-column: 1 / -1; display: grid; grid-template-columns: 1fr 1fr; }
  .filter-actions .btn { width: 100%; }
  .table-wrap { overflow: visible; border: 0; background: transparent; box-shadow: none; }
  .data-table,
  .data-table tbody,
  .data-table tr,
  .data-table td { display: block; width: 100%; }
  .data-table thead { display: none; }
  .data-table tr { margin-bottom: 12px; padding: 12px; border: 1px solid var(--border-color); border-radius: 14px; background: var(--bg-surface); box-shadow: var(--shadow-card); }
  .data-table td { display: grid; grid-template-columns: 84px minmax(0, 1fr); gap: 10px; align-items: center; padding: 6px; border: 0; white-space: normal; text-align: right; }
  .data-table td::before { content: attr(data-label); color: var(--text-muted); font-size: 11px; text-align: left; }
  .data-table td.ops { align-items: start; text-align: left; }
  .data-table td.ops::before { padding-top: 3px; }
  .pager { flex-direction: column; align-items: stretch; padding: 14px 0 0; }
  .pager-btns { display: grid; grid-template-columns: 1fr 1fr; }
  .form-grid,
  .readonly-grid {
    grid-template-columns: 1fr;
  }

  .drawer {
    width: 100vw;
  }
}
</style>
