<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { changePassword } from '../api/auth'
import {
  CONFIG_STATUS_CHANGED_EVENT,
  fetchConfigStatus,
  reloadConfigs,
  resetConfigs,
  type ConfigStatus,
} from '../api/config'
import PageHeader from '../components/ui/PageHeader.vue'
import UiModal from '../components/ui/UiModal.vue'
import AuditLogPanel from '../components/AuditLogPanel.vue'
import { useSession } from '../composables/useSession'

const router = useRouter()
const { clearSession } = useSession()
const configStatus = ref<ConfigStatus | null>(null)
const statusLoading = ref(false)

function formatTime(value: string | null | undefined) {
  if (!value) return '暂无记录'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  }).format(new Date(value))
}

async function loadConfigStatus() {
  statusLoading.value = true
  try {
    configStatus.value = await fetchConfigStatus()
  } finally {
    statusLoading.value = false
  }
}

/* —— 修改密码 —— */
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const pwdSubmitting = ref(false)
const pwdError = ref('')
const pwdNotice = ref('')

async function handlePasswordSubmit() {
  if (pwdSubmitting.value) return
  pwdError.value = ''
  pwdNotice.value = ''
  pwdSubmitting.value = true
  try {
    pwdNotice.value = await changePassword(
      currentPassword.value,
      newPassword.value,
      confirmPassword.value,
    )
    // 后端在改密成功后会销毁全部会话，因此这里必须回到登录页重新认证。
    clearSession()
    setTimeout(() => {
      router.replace({ name: 'login' })
    }, 1200)
  } catch (err) {
    pwdError.value = err instanceof Error ? err.message : '修改密码失败'
  } finally {
    pwdSubmitting.value = false
  }
}

/* —— 热重载 —— */
const reloading = ref(false)
const reloadMsg = ref('')
const reloadErr = ref('')

async function handleReload() {
  if (reloading.value) return
  reloading.value = true
  reloadMsg.value = ''
  reloadErr.value = ''
  try {
    reloadMsg.value = await reloadConfigs()
    await loadConfigStatus()
  } catch (err) {
    reloadErr.value = err instanceof Error ? err.message : '热重载失败'
  } finally {
    reloading.value = false
  }
}

/* —— 恢复出厂 —— */
const resetOpen = ref(false)
const resetBusy = ref(false)
const resetMsg = ref('')
const resetErr = ref('')
/** 二次确认：要求用户输入「恢复出厂」以降低误触 */
const resetConfirmText = ref('')

function openResetDialog() {
  resetConfirmText.value = ''
  resetErr.value = ''
  resetOpen.value = true
}

function closeResetDialog() {
  if (resetBusy.value) return
  resetOpen.value = false
  resetConfirmText.value = ''
}

async function confirmReset() {
  if (resetBusy.value) return
  if (resetConfirmText.value.trim() !== '恢复出厂') {
    resetErr.value = '请输入「恢复出厂」以确认操作'
    return
  }
  resetBusy.value = true
  resetErr.value = ''
  resetMsg.value = ''
  try {
    resetMsg.value = await resetConfigs()
    await loadConfigStatus()
    resetOpen.value = false
    resetConfirmText.value = ''
  } catch (err) {
    resetErr.value = err instanceof Error ? err.message : '恢复出厂配置失败'
  } finally {
    resetBusy.value = false
  }
}

onMounted(() => {
  window.addEventListener(CONFIG_STATUS_CHANGED_EVENT, handleConfigStatusChanged)
  loadConfigStatus().catch((err) => {
    reloadErr.value = err instanceof Error ? err.message : '读取配置状态失败'
  })
})

function handleConfigStatusChanged() {
  void loadConfigStatus().catch((err) => {
    reloadErr.value = err instanceof Error ? err.message : '刷新配置状态失败'
  })
}

onBeforeUnmount(() => {
  window.removeEventListener(CONFIG_STATUS_CHANGED_EVENT, handleConfigStatusChanged)
})
</script>

<template>
  <section class="system-page">
    <PageHeader
      eyebrow="System"
      title="系统设置"
      description="管理账号安全、配置运行状态与危险操作；玩家存档和群组开关不受出厂重置影响。"
    />

    <div class="stack">
      <!-- 区块1 账号安全 -->
      <form class="card" @submit.prevent="handlePasswordSubmit">
        <h2 class="card-title">账号安全</h2>
        <p class="card-hint">密码长度至少 6 位；修改成功后全部会话失效，需使用新密码重新登录。</p>

        <label class="field">
          <span class="field-label">当前密码</span>
          <input
            v-model="currentPassword"
            class="field-input"
            type="password"
            autocomplete="current-password"
            required
          />
        </label>

        <label class="field">
          <span class="field-label">新密码</span>
          <input
            v-model="newPassword"
            class="field-input"
            type="password"
            autocomplete="new-password"
            required
          />
        </label>

        <label class="field">
          <span class="field-label">确认新密码</span>
          <input
            v-model="confirmPassword"
            class="field-input"
            type="password"
            autocomplete="new-password"
            required
          />
        </label>

        <p v-if="pwdError" class="form-message is-error" role="alert">{{ pwdError }}</p>
        <p v-else-if="pwdNotice" class="form-message is-success" role="status">{{ pwdNotice }}</p>

        <button class="btn" type="submit" :disabled="pwdSubmitting">
          {{ pwdSubmitting ? '提交中…' : '修改密码' }}
        </button>
      </form>

      <!-- 区块2 配置生效说明 + 热重载 -->
      <div class="card">
        <div class="section-heading">
          <div><h2 class="card-title">配置运行状态</h2><p class="card-hint">数据库版本与机器人当前内存版本的实时关系。</p></div>
          <span v-if="configStatus" class="runtime-badge" :class="{ pending: configStatus.pending_reload }">
            {{ configStatus.pending_reload ? '待热重载' : '运行中已生效' }}
          </span>
        </div>
        <dl v-if="configStatus" class="runtime-grid">
          <div><dt>数据库版本</dt><dd>v{{ configStatus.db_revision }}</dd></div>
          <div><dt>内存版本</dt><dd>v{{ configStatus.loaded_revision }}</dd></div>
          <div><dt>最近保存</dt><dd>{{ formatTime(configStatus.saved_at) }}</dd></div>
          <div><dt>最近生效</dt><dd>{{ formatTime(configStatus.loaded_at) }}</dd></div>
        </dl>
        <p v-else-if="statusLoading" class="card-hint">正在读取运行状态…</p>
        <div class="explain">
          <p>
            在「配置中心」点击<strong>保存</strong>时，只会把内容写入数据库，
            <strong>不会</strong>立刻改变正在运行的机器人行为。
          </p>
          <p>
            需要点击<strong>热重载</strong>（本页或顶栏均可），才会把数据库中的最新配置加载到机器人内存并生效。
          </p>
          <ul class="explain-list">
            <li><span class="tag tag-warn">仅保存</span> 数据库已更新，运行中机器人仍用旧配置</li>
            <li><span class="tag tag-ok">已热重载</span> 机器人内存与数据库一致，新规则生效</li>
          </ul>
        </div>

        <p v-if="reloadErr" class="form-message is-error" role="alert">{{ reloadErr }}</p>
        <p v-else-if="reloadMsg" class="form-message is-success" role="status">{{ reloadMsg }}</p>

        <button
          class="btn btn-reload"
          type="button"
          :disabled="reloading"
          title="将数据库中的配置同步到机器人运行内存"
          @click="handleReload"
        >
          {{ reloading ? '重载中…' : '立即热重载' }}
        </button>
      </div>

      <!-- 区块3 危险区 · 恢复出厂 -->
      <div class="card card-danger">
        <div class="danger-head">
          <h2 class="card-title danger-title">危险区 · 恢复出厂配置</h2>
          <span class="danger-badge">不可逆</span>
        </div>
        <p class="card-hint danger-hint">
          将删除全部自定义游戏配置，并从程序内置种子数据重新写入默认配置，随后自动热重载。
          此操作<strong>不可撤销</strong>。玩家宠物存档、背包与群组开关不会被本接口清除，但商店/种类/指令等配置会回到出厂默认。
        </p>

        <p v-if="resetMsg" class="form-message is-success" role="status">{{ resetMsg }}</p>
        <p v-if="resetErr && !resetOpen" class="form-message is-error" role="alert">{{ resetErr }}</p>

        <button class="btn btn-danger" type="button" @click="openResetDialog">
          恢复出厂配置
        </button>
      </div>
    </div>

    <!-- 二次确认弹窗 -->
    <UiModal :open="resetOpen" title="确认恢复出厂配置" description="此操作不可撤销，并会在重置完成后自动热重载。" :busy="resetBusy" size="small" @close="closeResetDialog">
        <p class="modal-body">
          即将覆盖<strong>全部</strong>自定义配置（系统参数、指令、宠物种类、道具、商店、签到、打工、菜单、图片映射等），
          并从内置种子重新生成默认数据且自动热重载。此操作<strong>不可逆</strong>。
        </p>
        <label class="field">
          <span class="field-label">请输入「恢复出厂」以确认</span>
          <input
            v-model="resetConfirmText"
            class="field-input"
            type="text"
            autocomplete="off"
            placeholder="恢复出厂"
            :disabled="resetBusy"
          />
        </label>
        <p v-if="resetErr" class="form-message is-error" role="alert">{{ resetErr }}</p>
        <template #footer>
          <button type="button" class="btn btn-ghost" :disabled="resetBusy" @click="closeResetDialog">
            取消
          </button>
          <button
            type="button"
            class="btn btn-danger"
            :disabled="resetBusy || resetConfirmText.trim() !== '恢复出厂'"
            @click="confirmReset"
          >
            {{ resetBusy ? '执行中…' : '确认恢复出厂配置' }}
          </button>
        </template>
    </UiModal>
    <AuditLogPanel />
  </section>
</template>

<style scoped>
.system-page {
  max-width: 1040px;
  margin: 0 auto;
}

.page-title {
  margin: 0 0 8px;
  font-size: 20px;
  font-weight: 600;
  color: var(--text-main);
}

.page-desc {
  margin: 0 0 20px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--text-muted);
}

.stack {
  display: grid;
  grid-template-columns: minmax(0, .85fr) minmax(0, 1.15fr);
  gap: 16px;
}

.card-danger { grid-column: 1 / -1; }

.card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 20px;
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-card, 14px);
  box-shadow: var(--shadow-soft);
}

.card-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-main);
}

.section-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.section-heading .card-hint { margin-top: 5px; }
.runtime-badge { flex: 0 0 auto; padding: 4px 9px; border-radius: 8px; background: var(--success-soft); color: var(--success-strong); font-size: 11px; font-weight: 700; }
.runtime-badge.pending { background: var(--warning-soft); color: var(--warning-strong); }
.runtime-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; margin: 0; }
.runtime-grid div { padding: 10px 12px; border-radius: 10px; background: var(--bg-subtle); }
.runtime-grid dt { color: var(--text-muted); font-size: 11px; }
.runtime-grid dd { margin: 3px 0 0; font-weight: 650; font-variant-numeric: tabular-nums; }

.card-hint {
  margin: -6px 0 0;
  font-size: 13px;
  line-height: 1.55;
  color: var(--text-muted);
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field-label {
  font-size: 13px;
  color: var(--text-muted);
}

.field-input {
  padding: 10px 12px;
  font: inherit;
  color: var(--text-main);
  background-color: var(--bg-base);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-input, 9px);
}

.field-input:focus {
  outline: none;
  border-color: var(--accent);
}

.form-message {
  margin: 0;
  padding: 8px 12px;
  font-size: 13px;
  border-radius: 8px;
}

.form-message.is-error {
  color: var(--danger);
  background-color: var(--danger-soft);
}

.form-message.is-success {
  color: var(--text-main);
  background-color: var(--success-soft);
  border: 1px solid color-mix(in srgb, var(--success) 40%, transparent);
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-reload {
  align-self: flex-start;
}

/* 说明区 */
.explain {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 14px;
  font-size: 13px;
  line-height: 1.55;
  color: var(--text-main);
  background: var(--bg-elevated);
  border-radius: 10px;
  border: 1px solid var(--border-color);
}

.explain p {
  margin: 0;
}

.explain-list {
  margin: 4px 0 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.explain-list li {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  color: var(--text-muted);
}

.tag {
  flex-shrink: 0;
  display: inline-block;
  padding: 1px 8px;
  font-size: 12px;
  border-radius: 999px;
  font-weight: 500;
}

.tag-warn {
  color: var(--text-main);
  background: var(--warning-soft);
  border: 1px solid color-mix(in srgb, var(--warning) 45%, transparent);
}

.tag-ok {
  color: var(--text-main);
  background: var(--success-soft);
  border: 1px solid color-mix(in srgb, var(--success) 45%, transparent);
}

/* 危险区 */
.card-danger {
  border-color: color-mix(in srgb, var(--danger) 55%, var(--border-color));
  background: color-mix(in srgb, var(--danger-soft) 70%, var(--bg-surface));
  /* 细裂纹感：对角重复的淡危险色线 */
  background-image: repeating-linear-gradient(
    -12deg,
    transparent,
    transparent 10px,
    color-mix(in srgb, var(--danger) 6%, transparent) 10px,
    color-mix(in srgb, var(--danger) 6%, transparent) 11px
  );
}

.danger-head {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.danger-title {
  color: var(--danger);
}

.danger-badge {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  padding: 2px 8px;
  border-radius: 999px;
  color: var(--danger);
  border: 1px solid color-mix(in srgb, var(--danger) 50%, transparent);
  background: var(--danger-soft);
}

.danger-hint {
  color: var(--text-muted);
}

/* 弹窗 */
.modal-mask {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: color-mix(in srgb, var(--bg-base) 35%, rgba(20, 10, 16, 0.55));
  backdrop-filter: blur(4px);
}

.modal {
  width: min(420px, 100%);
  max-height: min(90vh, 640px);
  overflow: auto;
  box-shadow: var(--shadow-soft);
  border-color: color-mix(in srgb, var(--danger) 40%, var(--border-color));
}

.modal-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--danger);
}

.modal-body {
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--text-muted);
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  flex-wrap: wrap;
}

@media (max-width: 800px) {
  .stack { grid-template-columns: 1fr; }
  .card-danger { grid-column: auto; }
}

@media (max-width: 520px) {
  .runtime-grid { grid-template-columns: 1fr; }
}
</style>
