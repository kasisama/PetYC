<script setup lang="ts">
import { IconCopy, IconPlugConnected, IconRefresh, IconSettings } from '@tabler/icons-vue'
import { computed, onMounted, reactive, ref } from 'vue'
import {
  buildPortHandoffURL,
  confirmPlatformPort,
  getPlatformConfig,
  getPlatformStatus,
  getQQEnvTemplate,
  reconnectQQ,
  savePlatformConfig,
  type PlatformRuntimeConfig,
  type PlatformStatus,
} from '../api/ecosystem'
import { bulkUpdateGroupState, fetchGroups, updateGroup, type GroupSwitch } from '../api/ops'
import PageHeader from '../components/ui/PageHeader.vue'
import UiDrawer from '../components/ui/UiDrawer.vue'
import UiModal from '../components/ui/UiModal.vue'
import { useToast } from '../composables/useToast'

const toast = useToast()
const status = ref<PlatformStatus | null>(null)
const groups = ref<GroupSwitch[]>([])
const loading = ref(false)
const reconnectOpen = ref(false)
const configOpen = ref(false)
const reason = ref('')
const busy = ref(false)
const configLoading = ref(false)
const savedConfig = ref<PlatformRuntimeConfig | null>(null)
const form = reactive({
	listenAddress: '127.0.0.1',
  port: 8080,
  onebotToken: '',
  appId: '',
  appSecret: '',
  apiBase: '',
  tokenUrl: '',
  shardCount: 0,
  markdownEnabled: false,
  keyboardEnabled: false,
  interactionEnabled: false,
  auditEnabled: false,
  groupEventsEnabled: true,
  guildEventsEnabled: true,
})

const qq = computed<Record<string, any>>(() => status.value?.qq_official ?? {})
const onebot = computed<Record<string, any>>(() => status.value?.onebot ?? {})
const capabilityLabels:Record<string,{label:string;impact:string}>={group:{label:'官方群消息',impact:'关闭后不接收官方群命令'},guild:{label:'频道消息',impact:'关闭后不接收频道命令'},markdown:{label:'富文本消息',impact:'不可用时自动发送纯文本'},keyboard:{label:'消息按钮',impact:'不可用时玩家仍可输入命令'},interaction:{label:'按钮互动事件',impact:'仅影响按钮回调'},audit:{label:'消息审核事件',impact:'仅影响平台审核回执'}}
const capabilities = computed(()=>Object.entries(qq.value.capabilities??{}).map(([key,enabled])=>({key,enabled:Boolean(enabled),label:capabilityLabels[key]?.label??key,impact:capabilityLabels[key]?.impact??'不影响核心文本玩法'})))
const onebotGroups = computed(() => groups.value.filter((row) => row.platform === 'onebot'))
const officialGroups = computed(() => groups.value.filter((row) => row.platform === 'qq_group' || row.platform === 'qq_guild'))

async function load() {
  loading.value = true
  try {
    ;[status.value, groups.value] = await Promise.all([getPlatformStatus(), fetchGroups()])
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '平台状态加载失败')
  } finally {
    loading.value = false
  }
}

async function reconnect() {
  if (!reason.value.trim()) return toast.error('请填写重连原因')
  busy.value = true
  try {
    await reconnectQQ(reason.value)
    toast.success('网关重连已发起')
    reconnectOpen.value = false
    await load()
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '重连失败')
  } finally {
    busy.value = false
  }
}

async function copyTemplate() {
  try {
    const result = await getQQEnvTemplate()
    await navigator.clipboard.writeText(result.template)
    toast.success('环境变量模板已复制')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '复制失败')
  }
}

async function bulk(platform: 'onebot' | 'official', enabled: boolean) {
  try {
    const target = platform === 'onebot' ? onebotGroups.value : officialGroups.value
    if (!target.length) return toast.warning('暂无已登记的场景；收到第一条消息后会自动出现')
    const result = await bulkUpdateGroupState(target.map((row) => row.group_id), enabled)
    groups.value = result.groups
    toast.success(`已更新 ${result.updated} 个群开关`)
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '批量更新失败')
  }
}

async function toggleGroup(row: GroupSwitch) {
  try {
    const result = await updateGroup(row.group_id, { is_active: !row.is_active })
    groups.value.splice(groups.value.indexOf(row), 1, result.data)
    toast.success(result.data.is_active ? '该场景已启用' : '该场景已停用')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '群开关更新失败')
  }
}

function platformLabel(platform: GroupSwitch['platform']) {
  return platform === 'qq_group' ? 'QQ 官方群' : platform === 'qq_guild' ? 'QQ 官方频道' : 'OneBot 群'
}

function applyConfig(config: PlatformRuntimeConfig) {
  savedConfig.value = config
	form.listenAddress = config.listen_address || '127.0.0.1'
  form.port = config.port
  form.onebotToken = ''
  form.appId = config.qq_official.app_id ?? ''
  form.appSecret = ''
  form.apiBase = config.qq_official.api_base ?? ''
  form.tokenUrl = config.qq_official.token_url ?? ''
  form.shardCount = config.qq_official.shard_count ?? 0
  form.markdownEnabled = Boolean(config.qq_official.markdown_enabled)
  form.keyboardEnabled = Boolean(config.qq_official.keyboard_enabled)
  form.interactionEnabled = Boolean(config.qq_official.interaction_enabled)
  form.auditEnabled = Boolean(config.qq_official.audit_enabled)
  form.groupEventsEnabled = Boolean(config.qq_official.group_events_enabled)
  form.guildEventsEnabled = Boolean(config.qq_official.guild_events_enabled)
}

async function openConfig() {
  configOpen.value = true
  configLoading.value = true
  try {
    applyConfig(await getPlatformConfig())
  } catch (error) {
    configOpen.value = false
    toast.error(error instanceof Error ? error.message : '运行配置加载失败')
  } finally {
    configLoading.value = false
  }
}

async function saveConfig() {
  if (form.port < 1 || form.port > 65535) return toast.error('后台端口必须在 1 到 65535 之间')
  if (!form.appId.trim() && !savedConfig.value?.qq_official.app_secret_configured && !form.appSecret.trim()) {
    form.appSecret = ''
  }
  busy.value = true
  try {
    const qqOfficial: Record<string, string | number | boolean> = {
      app_id: form.appId.trim(),
      api_base: form.apiBase.trim(),
      token_url: form.tokenUrl.trim(),
      shard_count: Number(form.shardCount),
      markdown_enabled: form.markdownEnabled,
      keyboard_enabled: form.keyboardEnabled,
      interaction_enabled: form.interactionEnabled,
      audit_enabled: form.auditEnabled,
      group_events_enabled: form.groupEventsEnabled,
      guild_events_enabled: form.guildEventsEnabled,
    }
    if (form.appSecret.trim()) qqOfficial.app_secret = form.appSecret.trim()
    const onebot = form.onebotToken.trim() ? { token: form.onebotToken.trim() } : undefined
    const result = await savePlatformConfig({ listen_address: form.listenAddress.trim(), port: Number(form.port), onebot, qq_official: qqOfficial })
    if (result.port_handoff) {
      window.location.assign(buildPortHandoffURL(result.port_handoff.address, result.port_handoff.confirmation_token))
      return
    }
    applyConfig(result)
    configOpen.value = false
    toast.success('运行配置已保存，QQ 网关配置已自动应用')
    await load()
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '运行配置保存失败')
  } finally {
    busy.value = false
  }
}

async function confirmArrivedHandoff() {
  const prefix = '#port-handoff='
  if (!window.location.hash.startsWith(prefix)) return
  const token = decodeURIComponent(window.location.hash.slice(prefix.length))
  try {
    await confirmPlatformPort(token)
    window.history.replaceState(null, '', window.location.pathname + window.location.search)
    toast.success('新端口已接管，旧端口正在安全关闭')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '端口交接确认失败，请返回原端口')
  }
}

onMounted(async () => {
  await confirmArrivedHandoff()
  await load()
})
</script>

<template>
  <section>
    <PageHeader eyebrow="Platforms" title="机器人平台状态" description="查看连接、能力与运行参数；密钥只写入本机安全配置，不会从接口回显。">
      <template #actions>
        <button class="btn btn-ghost" :disabled="loading" @click="load"><IconRefresh :size="17" />刷新</button>
        <button class="btn btn-primary" @click="openConfig"><IconSettings :size="17" />运行配置</button>
      </template>
    </PageHeader>

    <div class="platform-grid">
      <article class="platform-card">
        <header><div class="platform-icon"><IconPlugConnected /></div><div><span>OneBot / NapCat</span><h2>{{ onebot.connected ? '在线' : '离线' }}</h2></div><i :class="onebot.connected ? 'online' : 'offline'" /></header>
        <dl><div><dt>在线连接</dt><dd>{{ onebot.connection_count ?? 0 }}</dd></div><div><dt>最近消息</dt><dd>{{ onebot.last_message_at ?? '暂无' }}</dd></div><div><dt>群开关</dt><dd>{{ onebotGroups.filter(row => row.is_active).length }} / {{ onebotGroups.length }}</dd></div></dl>
        <div class="actions"><button class="btn btn-primary" @click="bulk('onebot', true)">全部启用</button><button class="btn btn-ghost" @click="bulk('onebot', false)">全部停用</button></div>
      </article>
      <article class="platform-card">
        <header><div class="platform-icon qq">QQ</div><div><span>QQ 官方机器人</span><h2>{{ qq.connected ? '网关在线' : qq.configured ? '等待连接' : '未配置' }}</h2></div><i :class="qq.connected ? 'online' : 'offline'" /></header>
        <dl><div><dt>AppID</dt><dd>{{ qq.masked_app_id || '未配置' }}</dd></div><div><dt>AppSecret</dt><dd>{{ qq.app_secret_configured ? '已安全配置' : '未配置' }}</dd></div><div><dt>Session</dt><dd>{{ qq.session_state ?? '—' }}</dd></div><div><dt>分片</dt><dd>{{ qq.connected_shards ?? 0 }} / {{ qq.recommended_shards ?? 0 }}</dd></div><div><dt>场景开关</dt><dd>{{ officialGroups.filter(row => row.is_active).length }} / {{ officialGroups.length }}</dd></div><div><dt>发送队列</dt><dd>{{ qq.queue_depth ?? 0 }}</dd></div><div><dt>最近错误</dt><dd>{{ qq.last_error || '无' }}</dd></div></dl>
        <div class="actions"><button class="btn btn-primary" :disabled="!qq.configured" @click="reconnectOpen = true">重新连接网关</button><button class="btn btn-ghost" @click="copyTemplate"><IconCopy :size="16" />复制部署模板</button><button class="btn btn-ghost" @click="bulk('official', false)">停用全部场景</button></div>
      </article>
    </div>

    <article class="capability-panel"><div><span class="eyebrow">能力降级</span><h2>官方能力矩阵</h2><p>每项能力都说明关闭后的真实影响；核心玩法始终保留文本入口。</p></div><div class="capability-grid"><span v-for="item in capabilities" :key="item.key" :class="{ enabled:item.enabled }"><i />{{ item.label }}<strong>{{ item.enabled ? '可用' : item.impact }}</strong></span></div></article>

    <article class="scene-panel">
      <header><div><span class="eyebrow">统一场景开关</span><h2>群与频道</h2><p>OneBot 群、QQ 官方群和频道使用同一套开关；首次收到消息后自动登记。</p></div></header>
      <div v-if="groups.length" class="scene-list">
        <button v-for="row in groups" :key="row.group_id" :class="{ active: row.is_active }" @click="toggleGroup(row)">
          <span>{{ platformLabel(row.platform) }}</span><strong>{{ row.group_name || row.space_id }}</strong><small>{{ row.is_active ? '已启用，点击停用' : '已停用，点击启用' }}</small>
        </button>
      </div>
      <p v-else class="empty-scenes">暂无已登记场景；收到第一条群或频道消息后会自动出现。</p>
    </article>

    <UiDrawer :open="configOpen" title="运行配置" description="保存后自动应用 QQ 参数；修改端口会先启动新端口，网页接管成功后再关闭旧端口。" :busy="busy" @close="configOpen = false">
      <div v-if="configLoading" class="drawer-loading">正在读取本机配置…</div>
      <form v-else class="runtime-form" @submit.prevent="saveConfig">
        <section class="form-section"><header><h3>后台服务</h3><p>监听地址或端口占用时不会保存，也不会中断当前网页。修改监听地址时建议同时选择一个新端口。</p></header><div class="form-grid"><label>监听地址<select v-model="form.listenAddress"><option value="0.0.0.0">所有网卡（0.0.0.0）</option><option value="127.0.0.1">仅本机（127.0.0.1）</option></select></label><label>后台端口<input v-model.number="form.port" type="number" min="1" max="65535" /></label><label class="wide">OneBot WebSocket 令牌<input v-model="form.onebotToken" type="password" autocomplete="new-password" :placeholder="savedConfig?.onebot.token_configured ? '已配置，留空保持不变' : '输入新令牌'" /></label></div></section>
        <section class="form-section"><header><h3>QQ 官方机器人</h3><p>AppSecret 需要以可恢复形式保存在当前电脑的应用数据目录，保存后不会从接口回显。请限制该目录的系统账号访问权限；更高安全要求可改用系统凭据库。</p></header><div class="form-grid"><label>QQ AppID<input v-model="form.appId" autocomplete="off" placeholder="机器人 AppID" /></label><label>QQ AppSecret<input v-model="form.appSecret" type="password" autocomplete="new-password" :placeholder="savedConfig?.qq_official.app_secret_configured ? '已配置，留空保持不变' : '输入 AppSecret'" /></label><label class="wide">API 地址<input v-model="form.apiBase" placeholder="https://api.sgroup.qq.com" /></label><label class="wide">Token 地址<input v-model="form.tokenUrl" placeholder="https://bots.qq.com/app/getAppAccessToken" /></label><label>固定分片数<input v-model.number="form.shardCount" type="number" min="0" /><small>填 0 时使用平台推荐值</small></label></div></section>
        <section class="form-section"><header><h3>事件订阅与增强能力</h3><p>群与频道事件决定机器人接收哪些场景消息；增强能力未获权限时请保持关闭。</p></header><div class="switch-grid"><label><input v-model="form.groupEventsEnabled" type="checkbox" /><span>官方群事件</span></label><label><input v-model="form.guildEventsEnabled" type="checkbox" /><span>官方频道事件</span></label><label><input v-model="form.markdownEnabled" type="checkbox" /><span>Markdown 消息</span></label><label><input v-model="form.keyboardEnabled" type="checkbox" /><span>消息按钮</span></label><label><input v-model="form.interactionEnabled" type="checkbox" /><span>互动事件</span></label><label><input v-model="form.auditEnabled" type="checkbox" /><span>消息审核事件</span></label></div></section>
      </form>
      <template #footer><div class="drawer-actions"><button class="btn btn-ghost" :disabled="busy" @click="configOpen = false">取消</button><button class="btn btn-primary" :disabled="busy || configLoading" @click="saveConfig">{{ busy ? '正在应用…' : '保存并应用' }}</button></div></template>
    </UiDrawer>

    <UiModal :open="reconnectOpen" title="重新连接 QQ 网关" description="不会修改凭据，只会关闭当前连接并重新建立 Session。" :busy="busy" @close="reconnectOpen = false"><form class="operation-box" @submit.prevent="reconnect"><label>操作原因<textarea v-model="reason" rows="4" /></label><div class="actions"><button type="button" class="btn btn-ghost" @click="reconnectOpen = false">取消</button><button class="btn btn-danger" :disabled="busy">确认重连</button></div></form></UiModal>
  </section>
</template>

<style scoped>
.platform-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}.platform-card,.capability-panel{padding:20px;border:1px solid var(--border-color);border-radius:var(--radius-card);background:var(--bg-surface)}.platform-card header{display:flex;align-items:center;gap:12px}.platform-card header h2{margin:2px 0}.platform-card header span{color:var(--text-muted)}.platform-card header>i{margin-left:auto;width:10px;height:10px;border-radius:50%}.online{background:var(--success);box-shadow:0 0 0 5px var(--success-soft)}.offline{background:var(--text-muted)}.platform-icon{display:grid;place-items:center;width:42px;height:42px;border-radius:12px;background:var(--accent-soft);color:var(--accent)}.platform-icon.qq{font-weight:800}.platform-card dl{display:grid;grid-template-columns:1fr 1fr;margin:18px 0}.platform-card dl div{padding:10px;border-top:1px solid var(--border-color)}.platform-card dt{font-size:11px;color:var(--text-muted)}.platform-card dd{margin:3px 0;word-break:break-word}.capability-panel{display:grid;grid-template-columns:260px 1fr;gap:18px;margin-top:14px}.capability-panel h2{margin:2px 0 6px}.capability-panel p{margin:0;color:var(--text-muted)}.capability-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:8px}.capability-grid span{display:grid;grid-template-columns:auto 1fr;align-items:center;gap:8px;padding:11px;background:var(--bg-elevated);border-radius:10px}.capability-grid i{width:8px;height:8px;border-radius:50%;background:var(--text-muted)}.capability-grid .enabled i{background:var(--success)}.capability-grid strong{grid-column:2;font-size:11px;color:var(--text-muted)}.runtime-form{display:grid;gap:18px}.form-section{padding:18px;border:1px solid var(--border-color);border-radius:14px;background:var(--bg-elevated)}.form-section>header h3{margin:0}.form-section>header p{margin:5px 0 16px;color:var(--text-muted)}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:14px}.form-grid label{display:grid;gap:7px;color:var(--text-muted)}.form-grid .wide{grid-column:1/-1}.form-grid input,.form-grid select,.operation-box textarea{width:100%;padding:11px 12px;border:1px solid var(--border-color);border-radius:10px;background:var(--bg-base);color:var(--text-primary)}.form-grid small{font-size:11px}.switch-grid{display:grid;grid-template-columns:1fr 1fr;gap:8px}.switch-grid label{display:flex;align-items:center;gap:9px;padding:11px;border:1px solid var(--border-color);border-radius:10px}.drawer-actions{display:flex;justify-content:flex-end;gap:10px}.drawer-loading{padding:30px;text-align:center;color:var(--text-muted)}
@media(max-width:900px){.platform-grid,.capability-panel{grid-template-columns:1fr}.capability-grid{grid-template-columns:repeat(2,1fr)}}@media(max-width:560px){.capability-grid,.form-grid,.switch-grid{grid-template-columns:1fr}.form-grid .wide{grid-column:auto}.platform-card dl{grid-template-columns:1fr}}
.scene-panel{margin-top:14px;padding:20px;border:1px solid var(--border-color);border-radius:var(--radius-card);background:var(--bg-surface)}.scene-panel h2{margin:2px 0 6px}.scene-panel p{margin:0;color:var(--text-muted)}.scene-list{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:8px;margin-top:16px}.scene-list button{display:grid;gap:4px;padding:12px;border:1px solid var(--border-color);border-radius:10px;background:var(--bg-elevated);color:var(--text-main);text-align:left;cursor:pointer}.scene-list button.active{border-color:color-mix(in srgb,var(--success) 55%,var(--border-color));background:var(--success-soft)}.scene-list span,.scene-list small{color:var(--text-muted);font-size:11px}.scene-list strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.empty-scenes{margin-top:14px!important;padding:16px;border:1px dashed var(--border-color);border-radius:10px;text-align:center}@media(max-width:900px){.scene-list{grid-template-columns:repeat(2,1fr)}}@media(max-width:560px){.scene-list{grid-template-columns:1fr}}
</style>
