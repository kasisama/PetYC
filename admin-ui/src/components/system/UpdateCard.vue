<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  checkForUpdates,
  fetchUpdateStatus,
  installUpdate,
  type UpdateInfo,
  type UpdateStatus,
} from '../../api/update'
import UiModal from '../ui/UiModal.vue'

const info = ref<UpdateInfo | null>(null)
const status = ref<UpdateStatus | null>(null)
const checking = ref(false)
const error = ref('')
let pollTimer: ReturnType<typeof setTimeout> | null = null

const busy = computed(() => {
  const state = status.value?.state
  return state === 'checking' || state === 'downloading' || state === 'verifying' || state === 'restarting'
})

const statusText = computed(() => {
  switch (status.value?.state) {
    case 'checking': return '正在确认更新…'
    case 'downloading': return `正在下载 ${status.value.progress}%`
    case 'verifying': return '正在验证签名与文件完整性…'
    case 'restarting': return '更新已就绪，服务正在重启…'
    case 'failed': return status.value.error || '更新失败'
    default: return ''
  }
})

async function runCheck(force = false) {
  if (checking.value || busy.value) return
  checking.value = true
  error.value = ''
  try {
    info.value = await checkForUpdates(force)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '检查更新失败'
  } finally {
    checking.value = false
  }
}

const installOpen = ref(false)
const installBusy = ref(false)
const installReason = ref('')
const installConfirm = ref('')

function openInstallDialog() {
  if (!info.value?.canAutoUpdate || busy.value) return
  installReason.value = ''
  installConfirm.value = ''
  error.value = ''
  installOpen.value = true
}

async function confirmInstall() {
  if (installBusy.value || !info.value?.canAutoUpdate) return
  if (!installReason.value.trim()) {
    error.value = '请填写操作原因'
    return
  }
  if (installConfirm.value.trim() !== '安装更新') {
    error.value = '请输入「安装更新」以确认操作'
    return
  }
  installBusy.value = true
  error.value = ''
  try {
    status.value = await installUpdate(installReason.value.trim(), installConfirm.value.trim())
    installOpen.value = false
    schedulePoll()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '启动更新失败'
  } finally {
    installBusy.value = false
  }
}

function schedulePoll() {
  if (pollTimer) clearTimeout(pollTimer)
  pollTimer = setTimeout(pollStatus, 900)
}

async function pollStatus() {
  try {
    status.value = await fetchUpdateStatus()
    if (status.value.state === 'restarting') {
      await waitForRestart()
      return
    }
    if (status.value.state !== 'failed' && status.value.state !== 'idle') schedulePoll()
  } catch {
    if (status.value?.state === 'restarting') {
      await waitForRestart()
    } else {
      schedulePoll()
    }
  }
}

async function waitForRestart() {
  const expected = info.value?.latestVersion
  const deadline = Date.now() + 90_000
  while (Date.now() < deadline) {
    try {
      const response = await fetch('/healthz', { cache: 'no-store' })
      const payload = await response.json() as { status?: string; version?: string }
      if (response.ok && payload.status === 'ok' && (!expected || payload.version === expected)) {
        window.location.reload()
        return
      }
    } catch {
      // A connection error is expected while the executable is being replaced.
    }
    await new Promise((resolve) => setTimeout(resolve, 1_500))
  }
  error.value = '服务重启等待超时，请手动刷新页面或检查更新日志'
}

onMounted(() => void runCheck(false))
onBeforeUnmount(() => {
  if (pollTimer) clearTimeout(pollTimer)
})
</script>

<template>
  <div class="card update-card">
    <div class="update-heading">
      <div>
        <p class="eyebrow">PetYC Update</p>
        <h2 class="card-title">程序更新</h2>
      </div>
      <span class="version-badge">{{ info?.currentVersion === 'dev' ? 'dev' : info?.currentVersion ? `v${info.currentVersion}` : '—' }}</span>
    </div>

    <p v-if="checking && !info" class="card-hint">正在检查新版本…</p>
    <template v-else-if="info">
      <div v-if="info.available" class="available-block">
        <strong>发现新版本 v{{ info.latestVersion }}</strong>
        <p>{{ info.notes || '完整更新说明请查看 GitHub Release。' }}</p>
      </div>
      <p v-else-if="info.currentVersion === 'dev'" class="card-hint">当前为开发构建，不支持在线更新。</p>
      <p v-else class="card-hint">当前已是最新版本。</p>

      <div v-if="status?.state === 'downloading'" class="progress" aria-live="polite">
        <div class="progress-track"><span :style="{ width: `${status.progress}%` }" /></div>
        <span>{{ status.progress }}%</span>
      </div>
      <p v-if="statusText" class="status-text" :class="{ failed: status?.state === 'failed' }" role="status">{{ statusText }}</p>
      <p v-if="info.available && !info.canAutoUpdate" class="manual-reason">{{ info.reason }}</p>

      <div class="actions">
        <button class="btn btn-ghost" type="button" :disabled="checking || busy" @click="runCheck(true)">
          {{ checking ? '检查中…' : '检查更新' }}
        </button>
        <button v-if="info.available && info.canAutoUpdate" class="btn" type="button" :disabled="busy" @click="openInstallDialog">
          {{ busy ? '更新处理中…' : '立即更新' }}
        </button>
        <a v-else-if="info.available" class="btn" :href="info.releaseUrl" target="_blank" rel="noreferrer">手动下载</a>
      </div>
    </template>
    <div v-else class="actions">
      <button class="btn" type="button" :disabled="checking" @click="runCheck(true)">重新检查</button>
    </div>
    <p v-if="error && !installOpen" class="form-message is-error" role="alert">{{ error }}</p>
    <UiModal :open="installOpen" title="确认安装更新" description="下载并替换当前程序后服务会自动重启。" :busy="installBusy" size="small" @close="installOpen=false">
      <p class="card-hint">即将安装 v{{ info?.latestVersion }}。请填写原因并输入确认词。</p>
      <label class="field"><span class="field-label">操作原因</span><textarea v-model="installReason" class="field-input" rows="3" :disabled="installBusy"/></label>
      <label class="field"><span class="field-label">请输入「安装更新」</span><input v-model="installConfirm" class="field-input" :disabled="installBusy"/></label>
      <p v-if="error" class="form-message is-error" role="alert">{{ error }}</p>
      <template #footer>
        <button type="button" class="btn btn-ghost" :disabled="installBusy" @click="installOpen=false">取消</button>
        <button type="button" class="btn" :disabled="installBusy || !installReason.trim() || installConfirm.trim() !== '安装更新'" @click="confirmInstall">{{ installBusy ? '处理中…' : '确认安装' }}</button>
      </template>
    </UiModal>
  </div>
</template>

<style scoped>
.card { display: flex; flex-direction: column; gap: 14px; padding: 20px; background: var(--bg-surface); border: 1px solid var(--border-color); border-radius: var(--radius-card, 14px); box-shadow: var(--shadow-soft); }
.field { display: grid; gap: 6px; margin-top: 12px; }
.field-label { color: var(--text-muted); font-size: 12px; }
.field-input { padding: 9px 11px; border: 1px solid var(--border-color); border-radius: 9px; background: var(--bg-base); color: var(--text-main); }
.card-title { margin: 0; color: var(--text-main); font-size: 15px; font-weight: 600; }
.card-hint { margin: 0; color: var(--text-muted); font-size: 13px; line-height: 1.55; }
.form-message { margin: 0; padding: 8px 12px; border-radius: 8px; font-size: 13px; }
.form-message.is-error { color: var(--danger); background: var(--danger-soft); }
.update-card { grid-column: 1 / -1; }
.update-heading { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.eyebrow { margin: 0 0 5px; color: var(--accent); font-size: 10px; font-weight: 750; letter-spacing: .12em; text-transform: uppercase; }
.version-badge { padding: 5px 9px; border-radius: 8px; background: var(--bg-subtle); color: var(--text-muted); font-size: 12px; font-variant-numeric: tabular-nums; }
.available-block { padding: 14px 16px; border: 1px solid color-mix(in srgb, var(--accent) 35%, var(--border-color)); border-radius: 11px; background: color-mix(in srgb, var(--accent) 7%, var(--bg-surface)); }
.available-block strong { color: var(--text-main); }
.available-block p { margin: 6px 0 0; color: var(--text-muted); font-size: 13px; line-height: 1.6; white-space: pre-wrap; }
.actions { display: flex; flex-wrap: wrap; gap: 10px; }
.actions .btn { text-decoration: none; }
.manual-reason, .status-text { margin: 0; color: var(--text-muted); font-size: 13px; }
.status-text.failed { color: var(--danger); }
.progress { display: flex; align-items: center; gap: 10px; color: var(--text-muted); font-size: 12px; }
.progress-track { flex: 1; height: 7px; overflow: hidden; border-radius: 999px; background: var(--bg-subtle); }
.progress-track span { display: block; height: 100%; border-radius: inherit; background: var(--accent); transition: width .2s ease; }
@media (max-width: 800px) { .update-card { grid-column: auto; } }
</style>
