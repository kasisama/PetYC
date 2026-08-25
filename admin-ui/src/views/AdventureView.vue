<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { IconAlertTriangle, IconCheck, IconDeviceFloppy, IconRefresh } from '@tabler/icons-vue'
import { ApiError } from '../api/client'
import { getAdventureCatalog, getAdventureRuntime, saveAdventureCatalog, validateAdventureCatalog, type AdventureCatalog, type AdventureRuntime } from '../api/adventure'
import PageHeader from '../components/ui/PageHeader.vue'
import UiState from '../components/ui/UiState.vue'
import { useToast } from '../composables/useToast'
import { useUnsavedChanges } from '../composables/useUnsavedChanges'

const toast = useToast()
const loading = ref(true), saving = ref(false), error = ref('')
const catalog = ref<AdventureCatalog>({}), snapshot = ref(''), runtime = ref<AdventureRuntime|null>(null)
const selected = ref('maps'), draft = ref('[]'), draftError = ref('')
const dirty = computed(() => snapshot.value !== '' && JSON.stringify(catalog.value) !== snapshot.value)
useUnsavedChanges(dirty)

const sections = [
  ['maps','大地图','世界入口、推荐等级、封面与开放状态'],['zones','区域','地图内区域、难度、体力消耗与远征解锁目标'],['prerequisites','区域前置','区域之间的探索解锁顺序'],['objectives','探索目标','击杀、地标、首领等目标及探索度权重'],
  ['encounters','遭遇','探索时可能遇到的怪物、地标或安全事件'],['monsters','怪物','等级、生命、攻防、经验和掉落池'],['skills','战斗技能','宠物与怪物共用的技能参数'],['monster_skills','怪物技能','怪物可用技能与随机权重'],
  ['loot_pools','掉落池','固定与随机奖励的抽取规则'],['loot_entries','掉落内容','物品、货币、装备和蓝图碎片'],['expeditions','挂机远征','手动探索解锁后的派遣时长、消耗和奖励'],['bosses','限时首领','刷新周期、出现时长、属性、次数和奖励池'],
  ['boss_reward_tiers','首领档位奖励','按个人贡献追加的奖励档位'],['equipment_templates','装备','武器、防具、秘宝的基础属性与词条池'],['equipment_affixes','随机词条','装备可生成的附加属性'],['equipment_recipes','装备配方','蓝图碎片、货币与合成开关'],['equipment_recipe_materials','配方材料','高阶装备合成所需材料'],
] as const
const currentInfo = computed(() => sections.find(row => row[0] === selected.value) ?? sections[0])
const counts = computed(() => Object.fromEntries(sections.map(row => [row[0], catalog.value[row[0]]?.length ?? 0])))

function selectSection(key:string){if(!commitDraft())return;selected.value=key;draft.value=JSON.stringify(catalog.value[key]??[],null,2);draftError.value=''}
function commitDraft(){try{const value=JSON.parse(draft.value);if(!Array.isArray(value))throw new Error('本分区必须是 JSON 数组');catalog.value[selected.value]=value;draftError.value='';return true}catch(reason){draftError.value=reason instanceof Error?reason.message:'JSON 格式错误';return false}}
async function load(){loading.value=true;error.value='';try{const [data,state]=await Promise.all([getAdventureCatalog(),getAdventureRuntime()]);catalog.value=data;runtime.value=state;snapshot.value=JSON.stringify(data);draft.value=JSON.stringify(data[selected.value]??[],null,2)}catch(reason){error.value=reason instanceof ApiError?reason.message:'冒险配置读取失败'}finally{loading.value=false}}
async function validate(){if(!commitDraft())return;try{const result=await validateAdventureCatalog(catalog.value);toast.success(`校验通过：${result.summary.maps??0} 张地图、${result.summary.zones??0} 个区域`)}catch(reason){toast.error(reason instanceof Error?reason.message:'配置校验失败')}}
async function save(){if(!commitDraft())return;saving.value=true;try{await saveAdventureCatalog(catalog.value);snapshot.value=JSON.stringify(catalog.value);toast.success('冒险世界已原子保存；玩家数据未被修改')}catch(reason){toast.error(reason instanceof Error?reason.message:'保存失败')}finally{saving.value=false}}
onMounted(load)
</script>

<template><section class="adventure-page">
  <PageHeader eyebrow="Adventure world" title="冒险世界" description="地图不是写死的。这里统一维护大地图、区域探索、怪物战斗、限时首领、掉落与装备合成；玩家必须先手动探索并完成目标，才能解锁对应区域的挂机远征。">
    <template #actions><button class="btn btn-ghost" :disabled="loading" @click="load"><IconRefresh :size="17"/>刷新</button><button class="btn btn-ghost" :disabled="loading" @click="validate"><IconCheck :size="17"/>完整校验</button><button class="btn btn-primary" :disabled="saving||!dirty" @click="save"><IconDeviceFloppy :size="17"/>{{saving?'保存中…':'保存世界配置'}}</button></template>
  </PageHeader>
  <UiState v-if="error" tone="error" title="冒险配置加载失败" :description="error" action-label="重试" @action="load"/>
  <UiState v-else-if="loading" tone="loading" title="正在读取冒险世界" description="正在关联地图、怪物、奖励和装备配置。"/>
  <template v-else>
    <div class="flow" aria-label="玩家冒险流程"><div v-for="(step,index) in ['选择大地图与区域','手动探索并触发遭遇','回合战斗与目标推进','探索度达标解锁远征','挂机获取素材与装备','挑战限时群首领']" :key="step"><b>{{index+1}}</b><span>{{step}}</span></div></div>
    <div class="runtime"><article><span>探索中</span><strong>{{runtime?.counts.active_explorations??0}}</strong></article><article><span>战斗中</span><strong>{{runtime?.counts.active_combats??0}}</strong></article><article><span>远征中</span><strong>{{runtime?.counts.running_expeditions??0}}</strong></article><article><span>当前首领</span><strong>{{runtime?.counts.active_bosses??0}}</strong></article></div>
    <div class="explain"><IconAlertTriangle :size="18"/><div><strong>配置关系提示</strong><span>活动不会创建地图或图鉴。永久地图与图鉴由本页维护；活动只选择“全部远征”或指定区域作为活动进度来源。删除宠物、物品、活动引用或正在使用的内容会被方案兼容检查拦截。</span></div></div>
    <div class="workspace">
      <nav><button v-for="row in sections" :key="row[0]" :class="{active:selected===row[0]}" @click="selectSection(row[0])"><span><b>{{row[1]}}</b><small>{{row[2]}}</small></span><em>{{counts[row[0]]}}</em></button></nav>
      <main><header><div><span>当前分区</span><h2>{{currentInfo[1]}}</h2><p>{{currentInfo[2]}}</p></div><span :class="['state',{dirty}]">{{dirty?'有未保存修改':'已与数据库同步'}}</span></header>
        <label class="json-editor"><span>结构化配置（JSON 数组）</span><textarea v-model="draft" spellcheck="false" @blur="commitDraft"/><small v-if="draftError" class="error">{{draftError}}</small><small v-else>保存前会检查所有地图、区域、怪物、奖励池、装备、配方和图片引用；任一关系错误都不会写入数据库。</small></label>
      </main>
    </div>
  </template>
</section></template>

<style scoped>
.adventure-page{display:grid;gap:14px}.flow{display:grid;grid-template-columns:repeat(6,1fr);overflow:hidden;border:1px solid var(--border-color);border-radius:var(--radius-card);background:var(--bg-surface)}.flow div{position:relative;display:grid;gap:7px;padding:16px;border-right:1px solid var(--border-color)}.flow div:last-child{border:0}.flow b{display:grid;place-items:center;width:25px;height:25px;border-radius:50%;background:var(--accent-soft);color:var(--accent);font-size:12px}.flow span{font-size:12px;font-weight:650}.runtime{display:grid;grid-template-columns:repeat(4,1fr);border:1px solid var(--border-color);border-radius:var(--radius-card);background:var(--bg-surface)}.runtime article{display:grid;gap:3px;padding:14px 17px;border-right:1px solid var(--border-color)}.runtime article:last-child{border:0}.runtime span{color:var(--text-muted);font-size:11px}.runtime strong{font-size:22px}.explain{display:flex;gap:11px;padding:13px 15px;border:1px solid var(--border-color);border-radius:12px;background:var(--accent-soft);color:var(--accent)}.explain div{display:grid}.explain span{color:var(--text-muted);font-size:12px}.workspace{display:grid;grid-template-columns:300px minmax(0,1fr);min-height:610px;overflow:hidden;border:1px solid var(--border-color);border-radius:var(--radius-card);background:var(--bg-surface)}nav{align-content:start;display:grid;max-height:690px;padding:8px;overflow:auto;border-right:1px solid var(--border-color);background:var(--bg-subtle)}nav button{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:10px;border:0;border-radius:9px;background:transparent;color:inherit;text-align:left;cursor:pointer}nav button:hover,nav button.active{background:var(--bg-surface)}nav button.active{box-shadow:var(--shadow-card)}nav button span{display:grid;gap:2px}nav small{color:var(--text-muted);font-size:10px}nav em{min-width:25px;padding:3px 6px;border-radius:999px;background:var(--bg-elevated);color:var(--accent);font-size:11px;font-style:normal;text-align:center}main{display:grid;grid-template-rows:auto 1fr;gap:14px;padding:20px;min-width:0}main header{display:flex;justify-content:space-between;gap:16px}main header span{color:var(--text-muted);font-size:11px}main h2{margin:2px 0 0}main p{margin:4px 0 0;color:var(--text-muted);font-size:12px}.state.dirty{color:var(--warning-strong)}.json-editor{display:grid;grid-template-rows:auto 1fr auto;gap:7px;color:var(--text-muted);font-size:12px}.json-editor textarea{width:100%;min-height:480px;padding:14px;resize:vertical;border:1px solid var(--border-color);border-radius:12px;outline:0;background:var(--bg-base);color:var(--text-main);font:12px/1.55 ui-monospace,SFMono-Regular,Consolas,monospace;tab-size:2}.json-editor textarea:focus{border-color:var(--accent)}.json-editor .error{color:var(--danger)}
@media(max-width:1050px){.flow{grid-template-columns:repeat(3,1fr)}.runtime{grid-template-columns:repeat(2,1fr)}.workspace{grid-template-columns:240px 1fr}}@media(max-width:700px){.flow{grid-template-columns:1fr 1fr}.workspace{grid-template-columns:1fr}.workspace nav{display:flex;max-height:none;overflow:auto;border-right:0;border-bottom:1px solid var(--border-color)}nav button{min-width:180px}.json-editor textarea{min-height:420px}}
</style>
