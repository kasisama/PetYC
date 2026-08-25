<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { IconCheck, IconDatabaseExport, IconDatabaseImport, IconPlus, IconRefresh, IconTrash } from '@tabler/icons-vue'
import PageHeader from '../components/ui/PageHeader.vue'
import UiModal from '../components/ui/UiModal.vue'
import {
  activateProfile, captureProfile, createProfile, deleteProfile, exportProfile, getProfiles,
  importProfile, validateProfile, type ConfigProfile, type ProfileConflict,
} from '../api/profiles'

const profiles = ref<ConfigProfile[]>([])
const dirty = ref(false)
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const notice = ref('')
const createOpen = ref(false)
const name = ref('')
const description = ref('')
const pending = ref<ConfigProfile | null>(null)
const confirmMode = ref<'activate' | 'delete' | null>(null)
const conflicts = ref<ProfileConflict[]>([])
const fileInput = ref<HTMLInputElement | null>(null)
const active = computed(() => profiles.value.find((profile) => profile.active))

function formatTime(value: string) {
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}
function sourceLabel(profile: ConfigProfile) {
  if (profile.builtin) return '官方内置'
  return profile.source === 'import' ? '导入' : '用户创建'
}
async function load() {
  loading.value = true
  try { const result = await getProfiles(); profiles.value = result.items; dirty.value = result.dirty }
  catch (reason) { error.value = reason instanceof Error ? reason.message : '读取配置方案失败' }
  finally { loading.value = false }
}
function clearMessages() { error.value = ''; notice.value = ''; conflicts.value = [] }
async function saveAs() {
  if (!name.value.trim()) return
  busy.value = true; clearMessages()
  try { await createProfile(name.value, description.value); createOpen.value = false; name.value = ''; description.value = ''; notice.value = '已将当前配置另存为新方案'; await load() }
  catch (reason) { error.value = reason instanceof Error ? reason.message : '创建方案失败' }
  finally { busy.value = false }
}
async function capture(profile: ConfigProfile) {
  busy.value = true; clearMessages()
  try { await captureProfile(profile.id); notice.value = '当前修改已保存到方案'; await load() }
  catch (reason) { error.value = reason instanceof Error ? reason.message : '更新方案失败' }
  finally { busy.value = false }
}
async function requestActivate(profile: ConfigProfile) {
  clearMessages(); busy.value = true
  try {
    const validation = await validateProfile(profile.id)
    if (!validation.valid) { conflicts.value = validation.conflicts; pending.value = profile; return }
    pending.value = profile; confirmMode.value = 'activate'
  } catch (reason) { error.value = reason instanceof Error ? reason.message : '方案校验失败' }
  finally { busy.value = false }
}
async function confirmActivate(discardChanges: boolean) {
  if (!pending.value) return
  busy.value = true; clearMessages()
  try { await activateProfile(pending.value.id, discardChanges); notice.value = `已应用“${pending.value.name}”并完成热重载`; confirmMode.value = null; pending.value = null; await load() }
  catch (reason) { error.value = reason instanceof Error ? reason.message : '切换方案失败' }
  finally { busy.value = false }
}
async function removeProfile() {
  if (!pending.value) return
  busy.value = true; clearMessages()
  try { await deleteProfile(pending.value.id); notice.value = '方案已删除'; confirmMode.value = null; pending.value = null; await load() }
  catch (reason) { error.value = reason instanceof Error ? reason.message : '删除方案失败' }
  finally { busy.value = false }
}
async function chooseFile(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  busy.value = true; clearMessages()
  try { const result = await importProfile(file); conflicts.value = result.conflicts || []; notice.value = conflicts.value.length ? `${result.profile.name} 已导入，但与现有玩家数据存在冲突，暂时不能应用` : `${result.profile.name} 已导入，校验后可主动应用`; await load() }
  catch (reason) { error.value = reason instanceof Error ? reason.message : '导入失败' }
  finally { busy.value = false; if (fileInput.value) fileInput.value.value = '' }
}
async function exportOne(profile: ConfigProfile) {
  clearMessages(); busy.value = true
  try { await exportProfile(profile); notice.value = `已导出“${profile.name}”` }
  catch (reason) { error.value = reason instanceof Error ? reason.message : '导出失败' }
  finally { busy.value = false }
}
onMounted(load)
</script>

<template>
  <section class="profiles-page">
    <PageHeader eyebrow="Configuration profiles" title="配置方案" description="只迁移玩法与内容配置，不包含玩家、管理员、平台凭据或本机运行参数。">
      <template #actions>
        <input ref="fileInput" class="sr-only" type="file" accept=".qqpet-config" @change="chooseFile" />
        <button class="btn btn-ghost" :disabled="busy" @click="fileInput?.click()"><IconDatabaseImport :size="17" />导入方案</button>
        <button class="btn" :disabled="busy" @click="createOpen=true"><IconPlus :size="17" />另存为新方案</button>
      </template>
    </PageHeader>

    <div v-if="active" class="current-strip">
      <span><small>当前运行方案</small><strong>{{ active.name }}</strong></span>
      <span class="status" :class="{ dirty }">{{ dirty ? '有未保存到方案的修改' : '方案内容已同步' }}</span>
      <button v-if="dirty && !active.builtin" class="btn btn-ghost" :disabled="busy" @click="capture(active)"><IconRefresh :size="16" />更新当前方案</button>
      <button v-if="dirty && active.builtin" class="btn btn-ghost" @click="createOpen=true">另存后保留修改</button>
    </div>
    <p v-if="error" class="message error" role="alert">{{ error }}</p>
    <p v-if="notice" class="message success" role="status">{{ notice }}</p>
    <section v-if="conflicts.length" class="conflict-panel" role="alert">
      <h2>无法切换：现有玩家数据仍在引用目标方案缺少的内容</h2>
      <div v-for="item in conflicts" :key="item.kind"><strong>{{ item.kind }}</strong><span>影响 {{ item.affected_count }} 条：{{ item.missing_keys.join('、') }}</span></div>
    </section>

    <div v-if="loading" class="empty">正在读取方案…</div>
    <div v-else class="profile-grid">
      <article v-for="profile in profiles" :key="profile.id" class="profile-card" :class="{ active: profile.active }">
        <header><div><span class="source">{{ sourceLabel(profile) }}</span><h2>{{ profile.name }}</h2></div><span v-if="profile.active" class="active-badge"><IconCheck :size="14" />正在使用</span></header>
        <p>{{ profile.description || '暂无方案说明' }}</p>
        <dl><div><dt>配置项</dt><dd>{{ profile.summary.rows }}</dd></div><div><dt>格式 / 应用</dt><dd>v{{ profile.schema_version }} / {{ profile.app_version }}</dd></div><div><dt>更新时间</dt><dd>{{ formatTime(profile.updated_at) }}</dd></div></dl>
        <footer>
          <button class="btn btn-ghost" :disabled="busy" @click="exportOne(profile)"><IconDatabaseExport :size="16" />导出</button>
          <button v-if="!profile.active" class="btn" :disabled="busy" @click="requestActivate(profile)">校验并应用</button>
          <button v-if="!profile.builtin && !profile.active" class="icon-danger" :disabled="busy" title="删除方案" aria-label="删除方案" @click="pending=profile;confirmMode='delete'"><IconTrash :size="17" /></button>
        </footer>
      </article>
    </div>

    <UiModal :open="createOpen" title="另存为新方案" description="捕获当前配置，并立即将新方案设为当前方案。" :busy="busy" @close="createOpen=false">
      <label class="field">方案名称<input v-model="name" maxlength="80" /></label><label class="field">方案说明<textarea v-model="description" rows="3" maxlength="300" /></label>
      <template #footer><button class="btn btn-ghost" @click="createOpen=false">取消</button><button class="btn" :disabled="busy||!name.trim()" @click="saveAs">保存方案</button></template>
    </UiModal>
    <UiModal :open="confirmMode==='activate'" title="应用配置方案" :description="`即将切换到“${pending?.name || ''}”，通过后会立即热重载。`" :busy="busy" @close="confirmMode=null">
      <p v-if="dirty">当前方案存在未保存修改。你可以先关闭窗口更新/另存，也可以放弃修改后切换。</p><p v-else>校验已通过。玩家数据、本机参数和平台密钥不会被修改。</p>
      <template #footer><button class="btn btn-ghost" @click="confirmMode=null">取消</button><button class="btn" :class="{'btn-danger':dirty}" :disabled="busy" @click="confirmActivate(dirty)">{{dirty?'放弃修改并应用':'确认应用'}}</button></template>
    </UiModal>
    <UiModal :open="confirmMode==='delete'" title="删除配置方案" :description="`删除“${pending?.name || ''}”后无法恢复。`" :busy="busy" @close="confirmMode=null"><p>只会删除方案快照，不会删除当前配置、玩家数据或图片资源。</p><template #footer><button class="btn btn-ghost" @click="confirmMode=null">取消</button><button class="btn btn-danger" :disabled="busy" @click="removeProfile">确认删除</button></template></UiModal>
  </section>
</template>

<style scoped>
.profiles-page{max-width:1180px;margin:0 auto}.current-strip{display:flex;align-items:center;gap:18px;margin-bottom:16px;padding:15px 18px;border:1px solid var(--border-strong);border-radius:14px;background:var(--bg-surface)}.current-strip>span:first-child{display:grid}.current-strip small{color:var(--text-muted)}.status{margin-left:auto;padding:5px 9px;border-radius:8px;background:var(--success-soft);color:var(--success-strong);font-size:12px}.status.dirty{background:var(--warning-soft);color:var(--warning-strong)}.profile-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}.profile-card{display:grid;gap:16px;padding:19px;border:1px solid var(--border-color);border-radius:16px;background:var(--bg-surface)}.profile-card.active{border-color:color-mix(in srgb,var(--accent) 55%,var(--border-color));box-shadow:inset 3px 0 0 var(--accent)}.profile-card header,.profile-card footer{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}.profile-card h2{margin:4px 0 0;font-size:18px}.profile-card p{margin:0;color:var(--text-muted)}.source{color:var(--accent);font-size:11px;font-weight:700}.active-badge{display:flex;align-items:center;gap:4px;padding:4px 8px;border-radius:8px;background:var(--accent-soft);color:var(--accent);font-size:11px}.profile-card dl{display:grid;grid-template-columns:.6fr 1fr 1.4fr;gap:8px;margin:0}.profile-card dl div{padding:10px;border-radius:10px;background:var(--bg-elevated)}dt{color:var(--text-muted);font-size:11px}dd{margin:3px 0 0;font-size:12px}.profile-card footer{align-items:center;justify-content:flex-end;margin-top:auto}.icon-danger{display:grid;place-items:center;width:38px;height:38px;border:0;border-radius:10px;background:transparent;color:var(--danger);cursor:pointer}.icon-danger:hover{background:var(--danger-soft)}.field{display:grid;gap:7px;margin-bottom:14px;color:var(--text-muted)}.field input,.field textarea{padding:10px 12px;border:1px solid var(--border-color);border-radius:10px;background:var(--bg-base);color:var(--text-main);font:inherit}.message,.conflict-panel,.empty{padding:13px 15px;border-radius:12px}.message.error,.conflict-panel{background:var(--danger-soft);color:var(--danger)}.message.success{background:var(--success-soft);color:var(--success-strong)}.conflict-panel{display:grid;gap:8px;margin-bottom:14px}.conflict-panel h2{margin:0;font-size:14px}.conflict-panel div{display:grid}.conflict-panel span{font-size:12px}.empty{text-align:center;color:var(--text-muted);background:var(--bg-surface)}.sr-only{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0,0,0,0)}@media(max-width:800px){.profile-grid{grid-template-columns:1fr}.current-strip{align-items:flex-start;flex-wrap:wrap}.status{margin-left:0}.profile-card dl{grid-template-columns:1fr}}
</style>
