<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { IconArrowLeft, IconArrowRight, IconX } from '@tabler/icons-vue'
import { completeTour, fetchOnboardingStatus } from '../../api/onboarding'

const router = useRouter()
const steps = [
  { route: 'dashboard', target: 'nav-dashboard', title: '运营总览', body: '先看在线平台、活跃玩家和运营异常，快速掌握今天的整体状态。' },
  { route: 'players', target: 'nav-players', title: '玩家管理', body: '查询宠物、背包和资产；涉及玩家的数据始终与配置方案隔离。' },
  { route: 'gameplay', target: 'nav-gameplay', title: '玩法运营', body: '管理活动、远征、奖励轨道和概率玩法的运行状态。' },
  { route: 'adventure', target: 'nav-adventure', title: '冒险世界', body: '配置大地图、区域探索、怪物、限时首领、远征商店、装备与合成配方。' },
  { route: 'communities', target: 'nav-communities', title: '社群运营', body: '查看群组、成员和社区开关，处理不同社群的运营需求。' },
  { route: 'content', target: 'nav-content', title: '内容配置', body: '编辑宠物、道具、商店、指令和图片映射；保存后会标记当前方案有修改。' },
  { route: 'profiles', target: 'nav-profiles', title: '配置方案', body: '创建、切换、导入或导出玩法配置。配置包不会携带玩家或平台密钥。' },
  { route: 'platforms', target: 'nav-platforms', title: '平台状态', body: '配置 OneBot / NapCat 与 QQ 官方机器人，并检查实时连接状态。' },
  { route: 'system', target: 'nav-system', title: '系统设置', body: '管理管理员密码、运行配置状态和恢复官方默认方案。' },
  { route: 'system', target: 'hot-reload', title: '热重载', body: '内容保存进数据库后，用热重载同步到机器人运行内存。' },
] as const
const open = ref(false), index = ref(0), version = ref(1)
let highlighted: HTMLElement | null = null

function clearHighlight() { highlighted?.classList.remove('tour-highlight'); highlighted = null }
async function focusStep() {
  clearHighlight()
  const step = steps[index.value]
  await router.push({ name: step.route }); await nextTick()
  highlighted = Array.from(document.querySelectorAll<HTMLElement>(`[data-tour="${step.target}"]`)).find((element) => element.offsetParent !== null) || null
  highlighted?.classList.add('tour-highlight')
  highlighted?.scrollIntoView({ block: 'nearest', behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' })
}
async function start() { index.value = 0; open.value = true; await focusStep() }
async function next() { if (index.value === steps.length - 1) return finish(); index.value += 1; await focusStep() }
async function previous() { if (index.value > 0) { index.value -= 1; await focusStep() } }
async function finish() { clearHighlight(); open.value = false; await completeTour(version.value) }
function keydown(event: KeyboardEvent) { if (!open.value) return; if (event.key === 'Escape') void finish(); if (event.key === 'ArrowRight') void next(); if (event.key === 'ArrowLeft') void previous() }
onMounted(async () => {
  window.addEventListener('keydown', keydown); window.addEventListener('qqpet:replay-tour', start)
  try { const status = await fetchOnboardingStatus(); version.value = status.current_tour_version; if (status.setup_completed && status.tour_version_completed < status.current_tour_version) await start() } catch { /* 不阻塞后台 */ }
})
onBeforeUnmount(() => { clearHighlight(); window.removeEventListener('keydown', keydown); window.removeEventListener('qqpet:replay-tour', start) })
</script>

<template><Teleport to="body"><aside v-if="open" class="tour-panel" role="dialog" aria-modal="false" aria-labelledby="tour-title" aria-live="polite">
  <header><span>功能导览 · {{ index + 1 }}/{{ steps.length }}</span><button type="button" aria-label="跳过导览" @click="finish"><IconX :size="18" /></button></header>
  <h2 id="tour-title">{{ steps[index].title }}</h2><p>{{ steps[index].body }}</p>
  <footer><button class="btn btn-ghost" :disabled="index===0" @click="previous"><IconArrowLeft :size="16" />上一步</button><button class="btn" @click="next">{{index===steps.length-1?'完成':'下一步'}}<IconArrowRight :size="16" /></button></footer>
  <button class="skip" type="button" @click="finish">跳过导览</button>
</aside></Teleport></template>

<style scoped>.tour-panel{position:fixed;right:24px;bottom:24px;z-index:calc(var(--z-modal) - 1);width:min(380px,calc(100vw - 28px));padding:20px;border:1px solid var(--border-strong);border-radius:18px;background:var(--bg-surface);box-shadow:var(--shadow-dialog)}header,footer{display:flex;align-items:center;justify-content:space-between;gap:10px}header span{color:var(--accent);font-size:11px;font-weight:750;letter-spacing:.06em}header button{display:grid;place-items:center;border:0;background:transparent;color:var(--text-muted);cursor:pointer}.tour-panel h2{margin:16px 0 7px;font-size:21px}.tour-panel p{margin:0 0 20px;color:var(--text-muted);line-height:1.55}footer{justify-content:flex-end}.skip{margin-top:11px;border:0;background:transparent;color:var(--text-muted);font-size:12px;cursor:pointer}@media(max-width:700px){.tour-panel{right:14px;bottom:92px}}</style>
<style>.tour-highlight{position:relative;z-index:calc(var(--z-modal) - 2)!important;outline:3px solid var(--accent)!important;outline-offset:3px;box-shadow:0 0 0 8px var(--accent-soft)!important}@media(prefers-reduced-motion:reduce){.tour-highlight{scroll-behavior:auto}.tour-panel{animation:none!important}}</style>
