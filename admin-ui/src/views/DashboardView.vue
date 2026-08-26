<script setup lang="ts">
import { IconAlertTriangle, IconRefresh } from '@tabler/icons-vue'
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ApiError } from '../api/client'
import { getOverview, type Overview } from '../api/ecosystem'
import PageHeader from '../components/ui/PageHeader.vue'
import UiState from '../components/ui/UiState.vue'

const router = useRouter()
const range = ref<'7d' | '30d'>('7d')
const data = ref<Overview | null>(null)
const loading = ref(true)
const error = ref('')

const cards = computed(() => data.value ? [
  ['全局玩家', data.value.players, { name: 'players' }],
  ['宠物存档', data.value.pets, { name: 'players' }],
  ['进行中远征', data.value.active_expeditions, { name: 'gameplay', query: { status: 'running' } }],
  ['周期完成远征', data.value.completed_expeditions, { name: 'gameplay', query: { status: 'claimed' } }],
  ['活跃社区', data.value.active_communities, { name: 'communities' }],
  ['首领参与玩家', data.value.boss_participants, { name: 'communities', query: { panel: 'boss' } }],
] : [])

async function load() {
  loading.value = true
  error.value = ''
  try { data.value = await getOverview(range.value) }
  catch (reason) { error.value = reason instanceof ApiError ? reason.message : '总览数据加载失败' }
  finally { loading.value = false }
}

onMounted(load)
</script>

<template>
  <section>
    <PageHeader eyebrow="Overview" title="宠物远征运营总览" description="玩家、远征、社群与平台运行情况均来自真实业务数据。">
      <template #actions>
        <select v-model="range" class="field-control" aria-label="统计范围" @change="load"><option value="7d">最近 7 天</option><option value="30d">最近 30 天</option></select>
        <button class="btn btn-ghost" type="button" :disabled="loading" @click="load"><IconRefresh :size="17" />刷新</button>
      </template>
    </PageHeader>
    <div v-if="loading" class="ops-grid"><span v-for="n in 6" :key="n" class="ops-skeleton" /></div>
    <UiState v-else-if="error" tone="error" title="总览加载失败" :description="error" action-label="重试" @action="load" />
    <template v-else-if="data">
      <div class="ops-grid">
        <button v-for="card in cards" :key="String(card[0])" class="metric-card" type="button" @click="router.push(card[2] as never)">
          <span>{{ card[0] }}</span><strong>{{ card[1] }}</strong><small>查看详情 →</small>
        </button>
      </div>
      <div class="ops-columns">
        <article class="ops-panel adventure-ops">
          <div class="ops-panel-head"><div><span class="eyebrow">Adventure live</span><h2>冒险世界实时状态</h2></div><button class="text-link" type="button" @click="router.push({name:'adventure'})">管理配置 →</button></div>
          <div class="live-grid"><div><span>探索中</span><strong>{{data.active_explorations}}</strong></div><div><span>战斗中</span><strong>{{data.active_combats}}</strong></div><div><span>远征中</span><strong>{{data.active_adventure_expeditions}}</strong></div><div><span>当前首领</span><strong>{{data.active_adventure_bosses}}</strong></div></div>
          <div class="economy-row"><span>远征商店交易 <b>{{data.adventure_shop_transactions}}</b></span><span>旅途徽章产出 <b>+{{data.journey_badges_earned}}</b></span><span>旅途徽章消耗 <b>-{{data.journey_badges_spent}}</b></span></div>
        </article>
        <article class="ops-panel">
          <div class="ops-panel-head"><div><span class="eyebrow">Command health</span><h2>命令执行健康度</h2></div><strong class="rate">{{ (data.command_success_rate * 100).toFixed(1) }}%</strong></div>
          <div class="progress"><span :style="{ width: `${data.command_success_rate * 100}%` }" /></div>
          <p class="muted">统计范围内共处理 {{ data.command_total }} 条已聚合命令，不包含 OpenID 与消息正文。</p>
        </article>
        <article class="ops-panel" :class="{ warning: data.overdue_expeditions > 0 }">
          <div class="ops-panel-head"><div><span class="eyebrow">Exception queue</span><h2>异常处理队列</h2></div><IconAlertTriangle :size="24" /></div>
          <button class="exception-row" type="button" @click="router.push({ name: 'gameplay', query: { status: 'overdue' } })"><span>到期未结算远征</span><strong>{{ data.overdue_expeditions }}</strong></button>
          <button class="exception-row" type="button" @click="router.push({ name: 'content' })"><span>配置待重载</span><strong>{{ data.config_pending_reload ? 1 : 0 }}</strong></button>
          <button class="exception-row" type="button" @click="router.push({ name: 'adventure' })"><span>冒险引用异常</span><strong>{{ data.adventure_reference_errors ?? 0 }}</strong></button>
          <button class="exception-row" type="button" @click="router.push({ name: 'platforms' })"><span>平台最近错误</span><strong>{{ data.platform_error_count ?? 0 }}</strong></button>
        </article>
      </div>
    </template>
  </section>
</template>

<style scoped>
.ops-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px;margin-bottom:16px}.metric-card,.ops-panel{border:1px solid var(--border-color);background:var(--bg-surface);border-radius:var(--radius-card);box-shadow:var(--shadow-card)}.metric-card{padding:18px;text-align:left;color:inherit;cursor:pointer}.metric-card span,.metric-card small,.muted{color:var(--text-muted)}.metric-card strong{display:block;margin:8px 0;font-size:30px;color:var(--accent)}.ops-columns{display:grid;grid-template-columns:1fr 1fr;gap:14px}.ops-panel{padding:20px}.ops-panel.warning{border-color:color-mix(in srgb,var(--warning) 45%,var(--border-color))}.ops-panel-head,.exception-row{display:flex;align-items:center;justify-content:space-between;gap:16px}.ops-panel h2{margin:2px 0 14px;font-size:17px}.eyebrow{font-size:11px;text-transform:uppercase;letter-spacing:.12em;color:var(--text-muted)}.rate{font-size:28px}.progress{height:8px;background:var(--bg-elevated);border-radius:999px;overflow:hidden}.progress span{display:block;height:100%;background:var(--success);border-radius:inherit}.exception-row{width:100%;padding:12px 0;border:0;border-top:1px solid var(--border-color);background:none;color:inherit;cursor:pointer}.ops-skeleton{height:126px;border-radius:var(--radius-card);background:var(--bg-elevated);animation:pulse 1.2s infinite alternate}.adventure-ops{grid-column:1/-1}.text-link{border:0;background:none;color:var(--accent);font:inherit;font-size:12px;cursor:pointer}.live-grid{display:grid;grid-template-columns:repeat(4,1fr);margin-top:4px;border:1px solid var(--border-color);border-radius:11px;overflow:hidden}.live-grid div{display:grid;gap:5px;padding:13px;border-right:1px solid var(--border-color)}.live-grid div:last-child{border:0}.live-grid span,.economy-row{color:var(--text-muted);font-size:11px}.live-grid strong{font-size:21px}.economy-row{display:flex;gap:20px;flex-wrap:wrap;margin-top:13px}.economy-row b{margin-left:4px;color:var(--text-main)}@keyframes pulse{to{opacity:.45}}@media(max-width:900px){.ops-grid{grid-template-columns:repeat(2,1fr)}.ops-columns{grid-template-columns:1fr}}@media(max-width:560px){.ops-grid{grid-template-columns:1fr}.metric-card{padding:15px}.metric-card strong{font-size:24px}.live-grid{grid-template-columns:repeat(2,1fr)}}
</style>
