<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ApiError } from '../api/client'
import {
  bulkUpdateGroupState,
  deleteGroup,
  distributeCompensation,
  fetchGroups,
  syncGroups,
  updateGroup,
  type GroupSwitch,
} from '../api/ops'
import PageHeader from '../components/ui/PageHeader.vue'
import UiModal from '../components/ui/UiModal.vue'
import UiState from '../components/ui/UiState.vue'
import { useToast } from '../composables/useToast'

type OpsTab = 'groups' | 'compensation'

const tab = ref<OpsTab>('groups')
const toast = useToast()
const error = ref('')

// —— 群组开关 ——
const groups = ref<GroupSwitch[]>([])
const groupsLoading = ref(false)
const groupsBusy = ref(false)
const editingNameId = ref<number | null>(null)
const editingName = ref('')
const deleteTarget = ref<GroupSwitch | null>(null)
const deleting = ref(false)
const selectedGroupIds = ref<number[]>([])
const allGroupsSelected = computed(
  () => groups.value.length > 0 && selectedGroupIds.value.length === groups.value.length,
)

// —— 补偿发放 ——
const form = reactive({
  group_id: '0',
  user_ids_text: '',
  coins: 0 as number | string,
  items: '',
  notice: '',
  no_broadcast: false,
})
const submitting = ref(false)
const confirmOpen = ref(false)
const compResult = ref('')

function showToast(msg: string) { toast.success(msg) }

function errMsg(e: unknown, fallback: string) {
  if (e instanceof ApiError) return e.message
  if (e instanceof Error) return e.message
  return fallback
}

// —— 群组 ——
async function loadGroups() {
  groupsLoading.value = true
  error.value = ''
  try {
    groups.value = await fetchGroups()
    selectedGroupIds.value = selectedGroupIds.value.filter((id) =>
      groups.value.some((group) => group.group_id === id),
    )
  } catch (e) {
    error.value = errMsg(e, '加载群组列表失败')
  } finally {
    groupsLoading.value = false
  }
}

async function onSync() {
  groupsBusy.value = true
  error.value = ''
  try {
    const msg = await syncGroups()
    showToast(msg)
    await loadGroups()
  } catch (e) {
    error.value = errMsg(e, '同步活跃群组失败')
  } finally {
    groupsBusy.value = false
  }
}

async function toggleActive(g: GroupSwitch) {
  groupsBusy.value = true
  error.value = ''
  try {
    const res = await updateGroup(g.group_id, { is_active: !g.is_active })
    const idx = groups.value.findIndex((x) => x.group_id === g.group_id)
    if (idx >= 0) groups.value[idx] = res.data
    showToast(res.message)
  } catch (e) {
    error.value = errMsg(e, '更新群开关失败')
  } finally {
    groupsBusy.value = false
  }
}

function startEditName(g: GroupSwitch) {
  editingNameId.value = g.group_id
  editingName.value = g.group_name
}

function cancelEditName() {
  editingNameId.value = null
  editingName.value = ''
}

async function saveName(g: GroupSwitch) {
  const name = editingName.value.trim()
  groupsBusy.value = true
  error.value = ''
  try {
    const res = await updateGroup(g.group_id, { group_name: name })
    const idx = groups.value.findIndex((x) => x.group_id === g.group_id)
    if (idx >= 0) groups.value[idx] = res.data
    showToast(res.message)
    cancelEditName()
  } catch (e) {
    error.value = errMsg(e, '修改群名称失败')
  } finally {
    groupsBusy.value = false
  }
}

function toggleGroupSelection(groupId: number) {
  selectedGroupIds.value = selectedGroupIds.value.includes(groupId)
    ? selectedGroupIds.value.filter((id) => id !== groupId)
    : [...selectedGroupIds.value, groupId]
}

function toggleAllGroups() {
  selectedGroupIds.value = allGroupsSelected.value ? [] : groups.value.map((group) => group.group_id)
}

async function setBulkActive(active: boolean, selectedOnly: boolean) {
  const groupIds = selectedOnly ? selectedGroupIds.value : null
  if (selectedOnly && !groupIds?.length) {
    toast.warning('请先选择需要批量操作的群组')
    return
  }
  if (!groups.value.length) {
    toast.info('当前没有群记录')
    return
  }
  groupsBusy.value = true
  error.value = ''
  try {
    const result = await bulkUpdateGroupState(groupIds, active)
    groups.value = result.groups
    if (selectedOnly) selectedGroupIds.value = []
    showToast(`${active ? '已开启' : '已关闭'} ${result.updated} 个群组`)
  } catch (e) {
    error.value = errMsg(e, '批量更新失败')
  } finally {
    groupsBusy.value = false
  }
}

function askDelete(g: GroupSwitch) {
  deleteTarget.value = g
}

function cancelDelete() {
  deleteTarget.value = null
}

async function confirmDelete() {
  const g = deleteTarget.value
  if (!g) return
  deleting.value = true
  error.value = ''
  try {
    const msg = await deleteGroup(g.group_id)
    groups.value = groups.value.filter((x) => x.group_id !== g.group_id)
    showToast(msg)
    deleteTarget.value = null
  } catch (e) {
    error.value = errMsg(e, '删除群组记录失败')
  } finally {
    deleting.value = false
  }
}

// —— 补偿 ——
const parsedUserIds = computed(() => {
  const raw = form.user_ids_text.trim()
  if (!raw) return [] as number[]
  const parts = raw.split(/[,，\s]+/).map((s) => s.trim()).filter(Boolean)
  const ids: number[] = []
  for (const p of parts) {
    const n = Number(p)
    if (!Number.isFinite(n) || n <= 0 || !Number.isInteger(n)) continue
    ids.push(n)
  }
  return ids
})

const groupIdNum = computed(() => {
  const n = Number(form.group_id)
  return Number.isFinite(n) ? n : 0
})

const coinsNum = computed(() => Number(form.coins) || 0)

const effectiveNotice = computed(() => (form.no_broadcast ? '' : form.notice.trim()))

const noticePreview = computed(() => {
  if (form.no_broadcast) {
    return '（已勾选「只发奖励不广播」，将不发送群消息）'
  }
  let text = form.notice || ''
  if (!text.trim()) {
    return '（广播文案为空：只发放奖励，不发送群消息）'
  }
  const players =
    parsedUserIds.value.length === 0
      ? '本群全体宠物玩家'
      : parsedUserIds.value.map((id) => `@${id}`).join(' ')
  let coinsStr = ''
  if (coinsNum.value > 0) {
    coinsStr = `\n- ${coinsNum.value} 货币`
  }
  let itemsStr = ''
  if (form.items.trim()) {
    itemsStr = `\n- ${form.items.trim().replace(/#/g, '、')}`
  }
  text = text
    .replaceAll('[奖励玩家]', players)
    .replaceAll('[奖励货币]', coinsStr)
    .replaceAll('[奖励物品]', itemsStr)
    .replaceAll('[换行]', '\n')
  return text
})

const summary = computed(() => {
  const groupLabel = groupIdNum.value === 0 ? '所有群' : `群 ${groupIdNum.value}`
  const playersLabel =
    parsedUserIds.value.length === 0
      ? '目标范围内全部宠物玩家'
      : `指定 ${parsedUserIds.value.length} 名玩家：${parsedUserIds.value.join('、')}`
  return {
    group: groupLabel,
    players: playersLabel,
    coins: coinsNum.value,
    items: form.items.trim() || '（无）',
    broadcast: form.no_broadcast || !form.notice.trim() ? '不广播' : '将按群发送广播',
  }
})

const compensationStep = computed(() => {
  if (compResult.value) return 4
  if (confirmOpen.value) return 3
  if (coinsNum.value || form.items.trim()) return 2
  return 1
})

function openConfirm() {
  error.value = ''
  compResult.value = ''
  if (!coinsNum.value && !form.items.trim()) {
    error.value = '请至少填写货币或物品中的一项'
    return
  }
  confirmOpen.value = true
}

function closeConfirm() {
  confirmOpen.value = false
}

async function submitCompensation() {
  submitting.value = true
  error.value = ''
  try {
    const res = await distributeCompensation({
      group_id: groupIdNum.value,
      user_ids: parsedUserIds.value,
      coins: coinsNum.value,
      items: form.items.trim(),
      notice: effectiveNotice.value,
    })
    compResult.value = res.message + (res.count != null ? `（影响 ${res.count} 人）` : '')
    showToast(res.message)
    confirmOpen.value = false
  } catch (e) {
    error.value = errMsg(e, '补偿发放失败')
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  loadGroups()
})
</script>

<template>
  <section class="ops-page">
    <PageHeader
      eyebrow="Operations"
      title="运营工具"
      description="统一处理群组开关与资产补偿；批量写入操作会在确认后一次提交。"
    />

    <p v-if="error" class="page-error" role="alert">{{ error }}</p>

    <div class="tabs" role="tablist">
      <button
        type="button"
        class="tab"
        role="tab"
        :class="{ active: tab === 'groups' }"
        :aria-selected="tab === 'groups'"
        @click="tab = 'groups'"
      >
        群组开关
      </button>
      <button
        type="button"
        class="tab"
        role="tab"
        :class="{ active: tab === 'compensation' }"
        :aria-selected="tab === 'compensation'"
        @click="tab = 'compensation'"
      >
        补偿发放
      </button>
    </div>

    <!-- 群组开关 -->
    <div v-show="tab === 'groups'" class="tab-panel">
      <div class="toolbar card">
        <div class="toolbar-left">
          <button type="button" class="btn" :disabled="groupsBusy || groupsLoading" @click="onSync">
            同步活跃群组
          </button>
          <button
            type="button"
            class="btn btn-ghost"
            :disabled="groupsBusy || groupsLoading || !selectedGroupIds.length"
            @click="setBulkActive(true, true)"
          >
            开启已选（{{ selectedGroupIds.length }}）
          </button>
          <button
            type="button"
            class="btn btn-ghost"
            :disabled="groupsBusy || groupsLoading || !selectedGroupIds.length"
            @click="setBulkActive(false, true)"
          >
            关闭已选
          </button>
          <button
            type="button"
            class="btn btn-ghost"
            :disabled="groupsBusy || groupsLoading || !groups.length"
            @click="setBulkActive(true, false)"
          >全部开启</button>
          <button
            type="button"
            class="btn btn-ghost"
            :disabled="groupsBusy || groupsLoading || !groups.length"
            @click="setBulkActive(false, false)"
          >全部关闭</button>
          <button type="button" class="btn btn-ghost" :disabled="groupsLoading" @click="loadGroups">
            刷新
          </button>
        </div>
        <p class="toolbar-hint">
          关闭后机器人不处理该群消息；删除开关记录后该群恢复默认开启。
        </p>
      </div>

      <UiState v-if="groupsLoading" tone="loading" title="正在加载群组" description="正在同步当前群组开关状态。" />
      <UiState
        v-else-if="!groups.length"
        tone="empty"
        title="还没有群组记录"
        description="同步活跃群组后即可在这里批量启停。"
        action-label="同步活跃群组"
        @action="onSync"
      />

      <div v-else class="table-wrap card">
        <table class="data-table">
          <thead>
            <tr>
              <th class="select-cell"><input type="checkbox" :checked="allGroupsSelected" aria-label="选择全部群组" @change="toggleAllGroups" /></th>
              <th>群号</th>
              <th>群名称</th>
              <th>是否启用</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="g in groups" :key="g.group_id">
              <td class="select-cell"><input type="checkbox" :checked="selectedGroupIds.includes(g.group_id)" :aria-label="`选择群 ${g.group_id}`" @change="toggleGroupSelection(g.group_id)" /></td>
              <td class="num" data-label="群号">{{ g.group_id }}</td>
              <td data-label="群名称">
                <div v-if="editingNameId === g.group_id" class="name-edit">
                  <input v-model="editingName" class="field-input name-input" type="text" />
                  <button type="button" class="link-btn" :disabled="groupsBusy" @click="saveName(g)">
                    保存
                  </button>
                  <button type="button" class="link-btn muted" @click="cancelEditName">取消</button>
                </div>
                <div v-else class="name-cell">
                  <span>{{ g.group_name || '—' }}</span>
                  <button type="button" class="link-btn" :disabled="groupsBusy" @click="startEditName(g)">
                    改名
                  </button>
                </div>
              </td>
              <td data-label="是否启用">
                <button
                  type="button"
                  class="switch"
                  :class="{ on: g.is_active }"
                  :disabled="groupsBusy"
                  :aria-pressed="g.is_active"
                  :title="g.is_active ? '已启用，点击关闭' : '已关闭，点击开启'"
                  @click="toggleActive(g)"
                >
                  <span class="switch-knob" />
                  <span class="switch-label">{{ g.is_active ? '开启' : '关闭' }}</span>
                </button>
              </td>
              <td data-label="操作">
                <button type="button" class="link-btn danger" :disabled="groupsBusy" @click="askDelete(g)">
                  删除
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 补偿发放 -->
    <div v-show="tab === 'compensation'" class="tab-panel comp-layout">
      <ol class="flow-steps card" aria-label="补偿发放步骤">
        <li v-for="(label, index) in ['选择目标', '填写奖励', '预览确认', '发放结果']" :key="label" :class="{ active: compensationStep >= index + 1 }">
          <span>{{ index + 1 }}</span>{{ label }}
        </li>
      </ol>
      <div class="comp-form card">
        <section class="group">
          <h2 class="group-title">目标</h2>
          <div class="form-grid">
            <label class="f-field">
              <span>群号（0 = 所有群）</span>
              <input v-model="form.group_id" class="field-input" type="text" inputmode="numeric" />
            </label>
            <label class="f-field wide">
              <span>玩家 QQ 列表（空 = 群内全部宠物玩家；逗号/空格分隔）</span>
              <textarea
                v-model="form.user_ids_text"
                class="field-input area"
                rows="2"
                placeholder="例如：123456 789012"
              />
            </label>
          </div>
        </section>

        <section class="group">
          <h2 class="group-title">奖励</h2>
          <div class="form-grid">
            <label class="f-field">
              <span>货币（可正可负）</span>
              <input v-model="form.coins" class="field-input" type="number" />
            </label>
            <label class="f-field wide">
              <span>物品（格式：饼干*5#抽奖券*10）</span>
              <input
                v-model="form.items"
                class="field-input"
                type="text"
                placeholder="多个物品用 # 分隔，未写数量默认 1"
              />
            </label>
          </div>
        </section>

        <section class="group">
          <h2 class="group-title">广播</h2>
          <label class="f-field wide">
            <span>广播文案</span>
            <textarea
              v-model="form.notice"
              class="field-input area"
              rows="5"
              :disabled="form.no_broadcast"
              placeholder="支持变量：[奖励玩家] [奖励货币] [奖励物品] [换行]"
            />
          </label>
          <p class="var-hint">
            变量：
            <code>[奖励玩家]</code>
            <code>[奖励货币]</code>
            <code>[奖励物品]</code>
            <code>[换行]</code>
            · 文案为空时只发奖励不广播
          </p>
          <label class="check-row">
            <input v-model="form.no_broadcast" type="checkbox" />
            <span>只发奖励不广播</span>
          </label>
        </section>

        <div class="form-actions">
          <button type="button" class="btn" @click="openConfirm">发放</button>
        </div>

        <p v-if="compResult" class="page-ok" role="status">{{ compResult }}</p>
      </div>

      <aside class="comp-side card">
        <h2 class="group-title">发放摘要</h2>
        <ul class="summary-list">
          <li><span class="k">目标群</span><strong>{{ summary.group }}</strong></li>
          <li><span class="k">目标玩家</span><strong>{{ summary.players }}</strong></li>
          <li><span class="k">货币</span><strong class="num">{{ summary.coins }}</strong></li>
          <li><span class="k">物品</span><strong>{{ summary.items }}</strong></li>
          <li><span class="k">广播</span><strong>{{ summary.broadcast }}</strong></li>
        </ul>
        <h2 class="group-title preview-title">广播预览</h2>
        <pre class="preview">{{ noticePreview }}</pre>
      </aside>
    </div>

    <!-- 删除群确认 -->
    <UiModal :open="Boolean(deleteTarget)" title="删除群组开关记录" :busy="deleting" size="small" @close="cancelDelete">
        <p class="page-hint">
          将删除群 <strong class="num">{{ deleteTarget?.group_id }}</strong>
          （{{ deleteTarget?.group_name || '未命名' }}）的开关记录。删除后该群恢复默认开启。
        </p>
        <template #footer>
          <button type="button" class="btn btn-ghost" :disabled="deleting" @click="cancelDelete">取消</button>
          <button type="button" class="btn btn-danger" :disabled="deleting" @click="confirmDelete">
            {{ deleting ? '删除中…' : '确认删除群组开关' }}
          </button>
        </template>
    </UiModal>

    <!-- 补偿二次确认 -->
    <UiModal :open="confirmOpen" title="确认发放补偿" description="这是批量资产写入操作，请核对目标与奖励内容。" :busy="submitting" @close="closeConfirm">
        <p class="warn-line">这是批量写入操作，将同时修改多名玩家的货币/背包，请再次确认。</p>
        <ul class="summary-list">
          <li><span class="k">目标群</span><strong>{{ summary.group }}</strong></li>
          <li><span class="k">目标玩家</span><strong>{{ summary.players }}</strong></li>
          <li><span class="k">货币</span><strong class="num">{{ summary.coins }}</strong></li>
          <li><span class="k">物品</span><strong>{{ summary.items }}</strong></li>
          <li><span class="k">广播</span><strong>{{ summary.broadcast }}</strong></li>
        </ul>
        <pre class="preview compact">{{ noticePreview }}</pre>
        <template #footer>
          <button type="button" class="btn btn-ghost" :disabled="submitting" @click="closeConfirm">取消</button>
          <button type="button" class="btn" :disabled="submitting" @click="submitCompensation">
            {{ submitting ? '发放中…' : '确认发放补偿' }}
          </button>
        </template>
    </UiModal>
  </section>
</template>

<style scoped>
.ops-page {
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
  margin: 12px 0 0;
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

.tabs {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.flow-steps {
  grid-column: 1 / -1;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
  margin: 0;
  padding: 10px;
  list-style: none;
}

.flow-steps li {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  padding: 9px 10px;
  border-radius: 10px;
  color: var(--text-muted);
  font-size: 12px;
}

.flow-steps li span {
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  flex: 0 0 auto;
  border-radius: 8px;
  background: var(--bg-subtle);
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}

.flow-steps li.active { color: var(--text-main); background: var(--accent-soft); }
.flow-steps li.active span { background: var(--accent); color: var(--accent-ink); }

.select-cell { width: 42px; text-align: center; }
.select-cell input { width: 16px; height: 16px; accent-color: var(--accent); }

.tab {
  border: 1px solid var(--border-color);
  background: var(--bg-surface);
  color: var(--text-muted);
  padding: 8px 16px;
  border-radius: var(--radius-btn);
  cursor: pointer;
  font-size: 14px;
}

.tab:hover {
  color: var(--text-main);
  background: var(--accent-soft);
}

.tab.active {
  color: var(--accent);
  border-color: var(--accent);
  background: var(--accent-soft);
  font-weight: 600;
}

.tab-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.card {
  background: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-card);
  box-shadow: var(--shadow-soft);
}

.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
}

.toolbar-left {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.toolbar-hint {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
  max-width: 420px;
}

.btn {
  border: none;
  background: var(--accent);
  color: var(--accent-ink);
  padding: 8px 14px;
  border-radius: var(--radius-btn);
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
}

.btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.btn-ghost {
  background: transparent;
  color: var(--text-main);
  border: 1px solid var(--border-color);
}

.btn-danger {
  background: var(--danger);
  color: #fff;
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
  white-space: normal !important;
}

.name-cell,
.name-edit {
  display: flex;
  align-items: center;
  gap: 8px;
}

.name-input {
  min-width: 140px;
  max-width: 220px;
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

.link-btn.muted {
  color: var(--text-muted);
}

.switch {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--border-color);
  background: var(--bg-elevated);
  border-radius: 999px;
  padding: 3px 10px 3px 4px;
  cursor: pointer;
  color: var(--text-muted);
  font-size: 12px;
}

.switch.on {
  border-color: var(--success);
  background: var(--success-soft);
  color: var(--text-main);
}

.switch:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.switch-knob {
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: var(--text-muted);
  transition: transform 0.15s ease, background 0.15s ease;
}

.switch.on .switch-knob {
  background: var(--success);
}

.field-input {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid var(--border-color);
  background: var(--bg-base);
  color: var(--text-main);
  border-radius: var(--radius-input);
  padding: 8px 10px;
  font-size: 13px;
}

.field-input.area {
  resize: vertical;
  min-height: 64px;
  font-family: inherit;
  line-height: 1.5;
}

.field-input:disabled {
  opacity: 0.55;
}

/* 补偿布局 */
.comp-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(260px, 0.9fr);
  gap: 14px;
  align-items: start;
}

@media (max-width: 960px) {
  .comp-layout {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 700px) {
  .flow-steps { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .toolbar-left { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); width: 100%; }
  .toolbar-left .btn { width: 100%; }
  .table-wrap { overflow: visible; border: 0; background: transparent; box-shadow: none; }
  .data-table,
  .data-table tbody,
  .data-table tr,
  .data-table td { display: block; width: 100%; }
  .data-table thead { display: none; }
  .data-table tr { position: relative; margin-bottom: 12px; padding: 12px 12px 12px 48px; border: 1px solid var(--border-color); border-radius: 14px; background: var(--bg-surface); box-shadow: var(--shadow-card); }
  .data-table td { padding: 6px; border: 0; }
  .data-table td[data-label] { display: grid; grid-template-columns: 76px minmax(0, 1fr); gap: 10px; align-items: center; }
  .data-table td[data-label]::before { content: attr(data-label); color: var(--text-muted); font-size: 11px; font-weight: 650; }
  .data-table td[data-label].num { text-align: left; }
  .data-table .select-cell { position: absolute; left: 12px; top: 14px; width: 24px; padding: 0; }
}

.comp-form,
.comp-side {
  padding: 16px 18px 18px;
}

.group {
  margin-bottom: 16px;
}

.group-title {
  margin: 0 0 10px;
  font-size: 14px;
  font-weight: 700;
  color: var(--text-main);
}

.preview-title {
  margin-top: 18px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 12px;
}

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

.var-hint {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.6;
}

.var-hint code {
  font-size: 11px;
  padding: 1px 5px;
  border-radius: 4px;
  background: var(--bg-elevated);
  color: var(--accent);
  border: 1px solid var(--border-color);
}

.check-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 10px;
  font-size: 13px;
  color: var(--text-main);
  cursor: pointer;
}

.form-actions {
  margin-top: 8px;
}

.summary-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.summary-list li {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 13px;
}

.summary-list .k {
  font-size: 12px;
  color: var(--text-muted);
}

.summary-list strong {
  font-weight: 600;
  color: var(--text-main);
  word-break: break-all;
  white-space: normal;
}

.preview {
  margin: 0;
  padding: 12px;
  border-radius: var(--radius-input);
  background: var(--bg-elevated);
  border: 1px solid var(--border-color);
  color: var(--text-main);
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 280px;
  overflow: auto;
  font-family: inherit;
}

.preview.compact {
  margin-top: 12px;
  max-height: 160px;
}

.modal-mask {
  position: fixed;
  inset: 0;
  z-index: 50;
  background: rgba(40, 24, 32, 0.45);
  display: grid;
  place-items: center;
  padding: 16px;
}

.modal {
  width: min(440px, 100%);
  padding: 18px 20px;
}

.modal-wide {
  width: min(520px, 100%);
}

.modal-title {
  margin: 0 0 10px;
  font-size: 17px;
  font-weight: 700;
}

.warn-line {
  margin: 0 0 12px;
  padding: 8px 10px;
  border-radius: var(--radius-input);
  background: var(--danger-soft);
  color: var(--danger);
  font-size: 13px;
  border: 1px solid var(--border-color);
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
  flex-wrap: wrap;
}
</style>
