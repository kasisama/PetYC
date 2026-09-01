<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { IconArrowDown, IconArrowRight, IconArrowUp, IconCheck, IconChevronRight, IconDeviceFloppy, IconPlus, IconRefresh, IconTrash } from '@tabler/icons-vue'
import { emptyPetSpecies, savePetLineage, uploadImage, type PetEvolutionRuleConfigRow, type PetLineagePayload, type PetSpeciesConfigRow } from '../../api/config'
import { buildPetLineages, stageLabel, type PetLineageNode } from '../../utils/petLineage'
import { cloneConfigValue } from '../../utils/configClone'
import { useToast } from '../../composables/useToast'
import AssetThumbnail from './AssetThumbnail.vue'
import ImageDropzone from './ImageDropzone.vue'
import UiModal from '../ui/UiModal.vue'

const props = defineProps<{
  pets: PetSpeciesConfigRow[]
  rules: PetEvolutionRuleConfigRow[]
  query: string
  busy?: boolean
}>()

const emit = defineEmits<{
  saved: [familyKey: string, payload: PetLineagePayload]
  reload: []
}>()

const toast = useToast()
const selectedFamilyKey = ref('')
const selectedFormKey = ref('')
const draftPets = ref<PetSpeciesConfigRow[]>([])
const draftRules = ref<PetEvolutionRuleConfigRow[]>([])
const snapshot = ref('')
const saving = ref(false)
const uploading = ref(false)
const pendingFamily = ref('')
const discardOpen = ref(false)
const deleteOpen = ref(false)

function cloneRows<T>(rows: T[]): T[] {
  return cloneConfigValue(rows)
}

const effectivePets = computed(() => selectedFamilyKey.value
  ? [...props.pets.filter((pet) => pet.FamilyKey !== selectedFamilyKey.value), ...draftPets.value]
  : props.pets)
const originalFormKeys = computed(() => new Set(props.pets.filter((pet) => pet.FamilyKey === selectedFamilyKey.value).map((pet) => pet.Key)))
const effectiveRules = computed(() => selectedFamilyKey.value
  ? [...props.rules.filter((rule) => !originalFormKeys.value.has(rule.FromFormKey) && !originalFormKeys.value.has(rule.ToFormKey)), ...draftRules.value]
  : props.rules)
const lineageResult = computed(() => buildPetLineages(effectivePets.value, effectiveRules.value))

function searchMatch(pet: PetSpeciesConfigRow) {
  const query = props.query.trim().toLocaleLowerCase()
  if (!query) return true
  return `${pet.Name} ${pet.Key} ${pet.FamilyKey} ${pet.Description} ${pet.Archetype}`.toLocaleLowerCase().includes(query)
}

const lineageList = computed(() => {
  const grouped = new Map<string, PetSpeciesConfigRow[]>()
  effectivePets.value.forEach((pet) => {
    const key = pet.FamilyKey || `invalid-${pet.Key}`
    grouped.set(key, [...(grouped.get(key) ?? []), pet])
  })
  const issueMap = new Map(lineageResult.value.issues.map((issue) => [issue.familyKey, issue.message]))
  return [...grouped.entries()]
    .map(([key, pets]) => {
      const base = pets.find((pet) => pet.Stage === 'base') ?? pets[0]
      return { key, pets, title: base?.Name || key, image: base?.Image || '', issue: issueMap.get(key) || '' }
    })
    .filter((lineage) => lineage.pets.some(searchMatch))
    .sort((left, right) => left.title.localeCompare(right.title, 'zh-CN'))
})

const selectedPets = computed(() => draftPets.value)
const selectedPet = computed(() => selectedPets.value.find((pet) => pet.Key === selectedFormKey.value) ?? null)
const selectedRule = computed(() => selectedPet.value?.PreviousFormKey
  ? draftRules.value.find((rule) => rule.ToFormKey === selectedPet.value?.Key) ?? null
  : null)
const selectedNodes = computed(() => {
  const lineage = lineageResult.value.lineages.find((item) => item.familyKey === selectedFamilyKey.value)
  if (lineage) return lineage.nodes
  const pets = [...selectedPets.value].sort((left, right) => left.Stage.localeCompare(right.Stage, 'zh-CN') || left.Name.localeCompare(right.Name, 'zh-CN'))
  return pets.map((pet) => ({ pet, depth: pet.Stage === 'base' ? 0 : pet.Stage === 'evolved' ? 1 : 2, rule: draftRules.value.find((rule) => rule.ToFormKey === pet.Key) }))
})
type TrackStage = {
  key: 'base' | 'evolved' | 'awakened'
  step: number
  nodes: PetLineageNode[]
}

const horizontalStages = computed<TrackStage[]>(() => {
  const stages: TrackStage[] = [
    { key: 'base', step: 1, nodes: [] },
    { key: 'evolved', step: 2, nodes: [] },
    { key: 'awakened', step: 3, nodes: [] },
  ]
  const byStage = new Map(stages.map((stage) => [stage.key, stage]))
  selectedNodes.value.forEach((node) => byStage.get(node.pet.Stage as TrackStage['key'])?.nodes.push(node))
  return stages.filter((stage) => stage.nodes.length)
})
const isDirty = computed(() => snapshot.value !== '' && JSON.stringify({ pets: draftPets.value, rules: draftRules.value }) !== snapshot.value)
const canAddStage = computed(() => selectedPet.value?.Stage === 'base' || selectedPet.value?.Stage === 'evolved')
const previousOptions = computed(() => {
  const selected = selectedPet.value
  if (!selected) return []
  const rank: Record<string, number> = { base: 0, evolved: 1, awakened: 2 }
  return selectedPets.value.filter((pet) => pet.Key !== selected.Key && (rank[pet.Stage] ?? 99) < (rank[selected.Stage] ?? 99))
})

function startFamily(key: string) {
  const pets = props.pets.filter((pet) => pet.FamilyKey === key)
  const formKeys = new Set(pets.map((pet) => pet.Key))
  selectedFamilyKey.value = key
  draftPets.value = cloneRows(pets)
  draftRules.value = cloneRows(props.rules.filter((rule) => formKeys.has(rule.FromFormKey) || formKeys.has(rule.ToFormKey)))
  snapshot.value = JSON.stringify({ pets: draftPets.value, rules: draftRules.value })
  selectedFormKey.value = pets.find((pet) => pet.Stage === 'base')?.Key || pets[0]?.Key || ''
}

function requestFamily(key: string) {
  if (key === selectedFamilyKey.value) return
  if (isDirty.value) {
    pendingFamily.value = key
    discardOpen.value = true
    return
  }
  startFamily(key)
}

function confirmDiscard() {
  discardOpen.value = false
  if (pendingFamily.value) startFamily(pendingFamily.value)
  pendingFamily.value = ''
}

watch(lineageList, (lineages) => {
  if (!lineages.length) {
    selectedFamilyKey.value = ''
    return
  }
  if (!lineages.some((lineage) => lineage.key === selectedFamilyKey.value) && !isDirty.value) startFamily(lineages[0].key)
}, { immediate: true })

function newStableKey(prefix: string) {
  return `${prefix}-${crypto.randomUUID().slice(0, 8)}`
}

function transitionSummary(nodes: PetLineageNode[]) {
  const rule = nodes.find((node) => node.rule)?.rule
  if (!rule) return nodes.length > 1 ? '选择进化分支' : '进化条件'
  if (nodes.length > 1) return `分支 ${nodes.length} 条`
  return `成长 ≥ ${rule.RequiredGrowth} · 好感 ≥ ${rule.RequiredAffection}`
}

function addLineage() {
  if (isDirty.value) {
    toast.warning('请先保存或放弃当前谱系的修改')
    return
  }
  const key = newStableKey('pet')
  const pet = emptyPetSpecies('新宠物')
  pet.Key = key
  pet.FamilyKey = key
  pet.Stage = 'base'
  pet.PreviousFormKey = ''
  selectedFamilyKey.value = key
  draftPets.value = [pet]
  draftRules.value = []
  selectedFormKey.value = key
  snapshot.value = JSON.stringify({ pets: draftPets.value, rules: draftRules.value })
}

function addStage() {
  const parent = selectedPet.value
  if (!parent || !canAddStage.value) return
  const stage = parent.Stage === 'base' ? 'evolved' : 'awakened'
  const key = newStableKey('pet')
  const pet = emptyPetSpecies('新形态')
  pet.Key = key
  pet.FamilyKey = selectedFamilyKey.value
  pet.Stage = stage
  pet.PreviousFormKey = parent.Key
  pet.Adoptable = false
  const siblingOrders = draftRules.value.filter((rule) => rule.FromFormKey === parent.Key).map((rule) => rule.SortOrder)
  draftPets.value.push(pet)
  draftRules.value.push({ Key: newStableKey('evolution'), FromFormKey: parent.Key, ToFormKey: key, RequiredGrowth: 0, RequiredAffection: 0, BranchLabel: stage === 'evolved' ? '标准进化' : '觉醒分支', Enabled: true, SortOrder: Math.max(0, ...siblingOrders) + 10 })
  selectedFormKey.value = key
}

function syncPreviousRule() {
  const pet = selectedPet.value
  const rule = selectedRule.value
  if (pet && rule) rule.FromFormKey = pet.PreviousFormKey
}

function ensureRule() {
  const pet = selectedPet.value
  if (!pet?.PreviousFormKey || selectedRule.value) return
  draftRules.value.push({ Key: newStableKey('evolution'), FromFormKey: pet.PreviousFormKey, ToFormKey: pet.Key, RequiredGrowth: 0, RequiredAffection: 0, BranchLabel: '进化分支', Enabled: true, SortOrder: 10 })
}

function descendantsOf(key: string) {
  const deleted = new Set<string>([key])
  let changed = true
  while (changed) {
    changed = false
    draftPets.value.forEach((pet) => {
      if (pet.PreviousFormKey && deleted.has(pet.PreviousFormKey) && !deleted.has(pet.Key)) {
        deleted.add(pet.Key)
        changed = true
      }
    })
  }
  return deleted
}

function deleteSelected() {
  const pet = selectedPet.value
  if (!pet) return
  const removed = descendantsOf(pet.Key)
  draftPets.value = draftPets.value.filter((item) => !removed.has(item.Key))
  draftRules.value = draftRules.value.filter((rule) => !removed.has(rule.FromFormKey) && !removed.has(rule.ToFormKey))
  deleteOpen.value = false
  selectedFormKey.value = draftPets.value.find((item) => item.Stage === 'base')?.Key || draftPets.value[0]?.Key || ''
}

function moveBranch(direction: -1 | 1) {
  const rule = selectedRule.value
  if (!rule) return
  const siblings = draftRules.value.filter((item) => item.FromFormKey === rule.FromFormKey).sort((left, right) => left.SortOrder - right.SortOrder || left.Key.localeCompare(right.Key))
  const index = siblings.findIndex((item) => item.Key === rule.Key)
  const swap = siblings[index + direction]
  if (!swap) return
  const original = rule.SortOrder
  rule.SortOrder = swap.SortOrder
  swap.SortOrder = original
}

async function uploadPetImage(file: File) {
  const pet = selectedPet.value
  if (!pet) return
  const supported = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp'])
  if (!supported.has(file.type)) return toast.error('只支持 JPG、PNG、GIF 或 WEBP 图片')
  if (file.size > 10 * 1024 * 1024) return toast.error('图片不能超过 10MB')
  uploading.value = true
  try {
    pet.Image = (await uploadImage(file)).path
    toast.success('图片已上传，保存谱系后生效')
  } catch (reason) {
    toast.error(reason instanceof Error ? reason.message : '图片上传失败')
  } finally {
    uploading.value = false
  }
}

async function save() {
  if (!selectedFamilyKey.value) return
  saving.value = true
  try {
    const saved = await savePetLineage(selectedFamilyKey.value, { pets: cloneRows(draftPets.value), rules: cloneRows(draftRules.value) })
    draftPets.value = cloneRows(saved.pets)
    draftRules.value = cloneRows(saved.rules)
    snapshot.value = JSON.stringify({ pets: draftPets.value, rules: draftRules.value })
    emit('saved', selectedFamilyKey.value, saved)
    toast.success('谱系已保存，点击重载后应用到运行中服务')
  } catch (reason) {
    toast.error(reason instanceof Error ? reason.message : '保存谱系失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <section class="lineage-studio" aria-label="宠物谱系工作台">
    <header class="studio-head">
      <div class="studio-actions">
        <button class="btn btn-ghost" :disabled="saving || busy" @click="emit('reload')"><IconRefresh :size="16" />重载配置</button>
        <button class="btn btn-ghost" :disabled="saving || uploading" @click="addLineage"><IconPlus :size="16" />新增谱系</button>
        <button class="btn btn-primary" :disabled="saving || uploading || !isDirty" @click="save"><IconDeviceFloppy :size="16" />{{ saving ? '保存中…' : '保存谱系' }}</button>
      </div>
    </header>

    <div v-if="!lineageList.length" class="studio-empty">没有匹配的宠物谱系。可清空搜索条件或新建谱系。</div>
    <div v-else class="studio-grid">
      <aside class="lineage-catalog" aria-label="谱系列表">
        <div class="catalog-head"><span>谱系列表</span><b>{{ lineageList.length }}</b></div>
        <button v-for="lineage in lineageList" :key="lineage.key" class="catalog-item" :class="{ active: lineage.key === selectedFamilyKey }" @click="requestFamily(lineage.key)">
          <AssetThumbnail :path="lineage.image" :label="lineage.title" kind="pet" size="medium" />
          <span><strong>{{ lineage.title }}</strong><small>{{ lineage.pets.length }} 个形态</small></span>
          <i v-if="lineage.issue" title="谱系需要修复">!</i><IconChevronRight v-else :size="15" />
        </button>
      </aside>

      <main class="lineage-canvas">
        <header class="canvas-head"><div><span>进化路径</span><h3>{{ lineageList.find(item => item.key === selectedFamilyKey)?.title || '新谱系' }}</h3></div><small>{{ selectedNodes.length }} 个形态</small></header>
        <div class="canvas-flow">
          <div class="evolution-track" aria-label="横向进化路径">
            <template v-for="(stage, index) in horizontalStages" :key="stage.key">
              <section class="track-stage" :class="`stage-${stage.key}`" :aria-label="stageLabel(stage.key)">
                <article v-for="node in stage.nodes" :key="node.pet.Key" class="evolution-node" :class="{ active: node.pet.Key === selectedFormKey }" @click="selectedFormKey = node.pet.Key">
                  <span class="node-step">{{ stage.step }}</span>
                  <AssetThumbnail :path="node.pet.Image" :label="node.pet.Name || '未命名形态'" kind="pet" size="medium" />
                  <div class="node-copy"><span class="status-mark" :data-stage="node.pet.Stage">{{ stageLabel(node.pet.Stage) }}</span><strong>{{ node.pet.Name || '未命名形态' }}</strong><small v-if="node.rule">{{ node.rule.BranchLabel || '进化分支' }}</small><small v-else>谱系起点</small></div>
                  <dl class="node-stats"><div><dt>生命</dt><dd>{{ node.pet.Health }}</dd></div><div><dt>力量</dt><dd>{{ node.pet.Strength }}</dd></div><div><dt>成长</dt><dd>+{{ node.pet.GrowthBonus }}</dd></div><div><dt>好感</dt><dd>+{{ node.pet.AffectionBonus }}</dd></div></dl>
                </article>
              </section>
              <div v-if="index < horizontalStages.length - 1" class="track-connector" aria-hidden="true"><span>{{ transitionSummary(horizontalStages[index + 1].nodes) }}</span><i></i><IconArrowRight :size="19" /></div>
            </template>
          </div>
          <button v-if="canAddStage" class="add-stage" @click="addStage"><IconPlus :size="18" />添加{{ selectedPet?.Stage === 'base' ? '标准进化阶段' : '觉醒进化分支' }}</button>
        </div>
      </main>

      <aside class="property-panel">
        <template v-if="selectedPet">
          <header class="property-head"><div><span>当前形态</span><h3>{{ selectedPet.Name || '未命名形态' }}</h3></div></header>
          <div class="property-body">
            <ImageDropzone :path="selectedPet.Image" :label="selectedPet.Name || '新形态'" kind="pet" size="medium" :busy="uploading" @file="uploadPetImage" @clear="selectedPet.Image = ''" />
            <section class="field-section"><h4>基础资料</h4><div class="field-grid"><label><span>名称</span><input v-model.trim="selectedPet.Name" maxlength="64" /></label><label><span>阶段</span><select v-model="selectedPet.Stage"><option value="base">基础形态</option><option value="evolved">标准进化</option><option value="awakened">觉醒形态</option></select></label><label class="wide"><span>前置形态</span><select v-model="selectedPet.PreviousFormKey" :disabled="selectedPet.Stage === 'base'" @change="syncPreviousRule"><option value="">{{ selectedPet.Stage === 'base' ? '基础形态无前置' : '请选择前置形态' }}</option><option v-for="pet in previousOptions" :key="pet.Key" :value="pet.Key">{{ pet.Name }} · {{ stageLabel(pet.Stage) }}</option></select></label><label class="wide"><span>描述</span><textarea v-model="selectedPet.Description" rows="3" /></label><label><span>定位类型</span><select v-model="selectedPet.Archetype"><option value="balanced">均衡型</option><option value="attacker">攻击型</option><option value="support">辅助型</option><option value="defender">防御型</option></select></label><label class="toggle"><input v-model="selectedPet.Adoptable" type="checkbox" />允许领养</label></div></section>
            <section class="field-section"><h4>属性与成长</h4><div class="number-grid"><label><span>生命</span><input v-model.number="selectedPet.Health" type="number" min="0" /></label><label><span>智慧</span><input v-model.number="selectedPet.Wisdom" type="number" min="0" /></label><label><span>力量</span><input v-model.number="selectedPet.Strength" type="number" min="0" /></label><label><span>防御</span><input v-model.number="selectedPet.Defense" type="number" min="0" /></label><label><span>成长加成</span><input v-model.number="selectedPet.GrowthBonus" type="number" min="0" /></label><label><span>好感加成</span><input v-model.number="selectedPet.AffectionBonus" type="number" min="0" /></label></div></section>
            <section v-if="selectedPet.Stage !== 'base'" class="field-section evolution-fields"><header><h4>进化条件</h4><button v-if="!selectedRule" class="btn btn-ghost btn-small" @click="ensureRule">补充规则</button></header><template v-if="selectedRule"><div class="field-grid"><label class="wide"><span>分支名称</span><input v-model.trim="selectedRule.BranchLabel" /></label><label><span>成长门槛</span><input v-model.number="selectedRule.RequiredGrowth" type="number" min="0" /></label><label><span>好感门槛</span><input v-model.number="selectedRule.RequiredAffection" type="number" min="0" /></label><label class="toggle"><input v-model="selectedRule.Enabled" type="checkbox" />启用此分支</label><div class="branch-order"><span>同级顺序</span><button class="icon-btn" :disabled="!selectedRule" aria-label="上移分支" @click="moveBranch(-1)"><IconArrowUp :size="15" /></button><button class="icon-btn" :disabled="!selectedRule" aria-label="下移分支" @click="moveBranch(1)"><IconArrowDown :size="15" /></button></div></div></template><p v-else>该形态尚未配置进化条件，无法在玩家端触发进化。</p></section>
          </div>
          <footer class="property-footer"><span v-if="isDirty">当前谱系有未保存修改</span><span v-else><IconCheck :size="15" />已保存</span><button class="btn btn-danger btn-small" :disabled="saving" @click="deleteOpen = true"><IconTrash :size="15" />删除形态</button></footer>
        </template>
      </aside>
    </div>
  </section>

  <UiModal :open="discardOpen" title="放弃当前谱系的修改？" description="未保存的形态、进化条件和排序将被丢弃。" size="small" @close="discardOpen = false"><template #footer><button class="btn btn-ghost" @click="discardOpen = false">继续编辑</button><button class="btn btn-danger" @click="confirmDiscard">放弃并切换</button></template></UiModal>
  <UiModal :open="deleteOpen" title="删除当前形态？" :description="selectedPet?.Stage === 'base' ? '删除基础形态将删除整条谱系及其全部进化规则。' : '将同时删除该形态的所有后继形态和关联进化规则。'" size="small" @close="deleteOpen = false"><template #footer><button class="btn btn-ghost" @click="deleteOpen = false">取消</button><button class="btn btn-danger" @click="deleteSelected">确认删除</button></template></UiModal>
</template>

<style scoped>
.lineage-studio{display:grid;gap:14px}.studio-head,.canvas-head,.property-head,.field-section>header{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}.studio-head{padding:16px 18px;border:1px solid var(--border-color);border-radius:var(--radius-card);background:var(--bg-surface)}.studio-head span,.canvas-head span,.property-head span{color:var(--accent);font-size:11px;font-weight:750;letter-spacing:.06em}.studio-head h2,.canvas-head h3,.property-head h3{margin:4px 0 0}.studio-head h2{font-size:17px}.studio-actions{display:flex;align-items:center;gap:8px;flex-wrap:wrap}.studio-empty{padding:42px;border:1px dashed var(--border-color);border-radius:var(--radius-card);color:var(--text-muted);text-align:center}.studio-grid{display:grid;grid-template-columns:minmax(210px,.72fr) minmax(420px,1.45fr) minmax(310px,.9fr);min-height:660px;overflow:hidden;border:1px solid var(--border-color);border-radius:var(--radius-card);background:var(--bg-surface)}.lineage-catalog,.property-panel{min-width:0;background:var(--bg-elevated)}.lineage-catalog{padding:10px;border-right:1px solid var(--border-color)}.catalog-head{display:flex;align-items:center;justify-content:space-between;padding:6px 8px 10px;color:var(--text-muted);font-size:12px}.catalog-head b{display:grid;place-items:center;min-width:22px;height:22px;border-radius:7px;background:var(--accent-soft);color:var(--accent);font-size:11px}.catalog-item{display:grid;grid-template-columns:auto minmax(0,1fr) auto;align-items:center;gap:9px;width:100%;margin-top:5px;padding:9px;border:1px solid transparent;border-radius:11px;background:transparent;color:var(--text-main);text-align:left;cursor:pointer;transition:background .18s ease,border-color .18s ease,transform .18s var(--ease-out)}.catalog-item:hover{background:var(--bg-hover);transform:translateX(2px)}.catalog-item.active{border-color:color-mix(in srgb,var(--accent) 58%,var(--border-color));background:var(--accent-soft)}.catalog-item span{display:grid;min-width:0;gap:3px}.catalog-item strong,.catalog-item small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.catalog-item strong{font-size:13px}.catalog-item small{color:var(--text-muted);font-size:10px}.catalog-item i{display:grid;place-items:center;width:18px;height:18px;border-radius:50%;background:var(--warning-soft);color:var(--warning-strong);font-size:11px;font-style:normal;font-weight:800}.lineage-canvas{min-width:0;padding:17px;background:var(--bg-surface)}.canvas-head{padding-bottom:14px;border-bottom:1px solid var(--border-color)}.canvas-head small{color:var(--text-muted);font-variant-numeric:tabular-nums}.canvas-flow{display:grid;align-content:start;gap:12px;padding:20px 4px}.evolution-node{position:relative;display:grid;grid-template-columns:auto auto minmax(0,1fr);align-items:center;gap:11px;max-width:480px;margin-left:calc(var(--depth) * 38px);padding:12px;border:1px solid var(--border-color);border-radius:13px;background:var(--bg-elevated);cursor:pointer;transition:border-color .18s ease,background .18s ease,transform .18s var(--ease-out)}.evolution-node:not(.root)::before{position:absolute;top:-13px;left:-24px;width:21px;height:26px;border-bottom:1px solid var(--border-strong);border-left:1px solid var(--border-strong);content:''}.evolution-node:hover{transform:translateY(-1px);border-color:var(--border-strong)}.evolution-node.active{border-color:var(--accent);background:var(--accent-soft);box-shadow:0 10px 22px color-mix(in srgb,var(--accent) 12%,transparent)}.node-step{display:grid;place-items:center;width:22px;height:22px;border-radius:7px;background:var(--bg-base);color:var(--text-muted);font-size:11px;font-weight:800;font-variant-numeric:tabular-nums}.node-copy{display:grid;min-width:0;gap:4px}.node-copy strong,.node-copy small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.node-copy strong{font-size:14px}.node-copy small{color:var(--text-muted);font-size:10px}.status-mark{width:max-content;padding:3px 6px;border-radius:5px;background:var(--bg-base);color:var(--text-muted);font-size:10px;font-weight:750}.status-mark[data-stage=base]{background:var(--success-soft);color:var(--success-strong)}.status-mark[data-stage=evolved]{background:var(--accent-soft);color:var(--accent)}.status-mark[data-stage=awakened]{background:var(--warning-soft);color:var(--warning-strong)}.add-stage{display:flex;align-items:center;justify-content:center;gap:7px;max-width:480px;margin-left:calc((var(--depth, 0) + 1) * 38px);padding:11px;border:1px dashed var(--border-strong);border-radius:11px;background:transparent;color:var(--accent);font:inherit;font-size:12px;font-weight:700;cursor:pointer}.add-stage:hover{border-color:var(--accent);background:var(--accent-soft)}.property-panel{border-left:1px solid var(--border-color)}.property-head{padding:15px;border-bottom:1px solid var(--border-color)}.property-head h3{font-size:16px}.property-head code{max-width:120px;overflow:hidden;color:var(--text-muted);text-overflow:ellipsis;white-space:nowrap;font-size:10px}.property-body{display:grid;gap:13px;max-height:720px;padding:14px;overflow:auto}.field-section{display:grid;gap:10px;padding:12px;border:1px solid var(--border-color);border-radius:12px;background:var(--bg-surface)}.field-section h4{margin:0;font-size:13px}.field-section p{margin:0;color:var(--text-muted);font-size:11px;line-height:1.55}.field-grid,.number-grid{display:grid;grid-template-columns:1fr 1fr;gap:8px}.field-grid label,.number-grid label{display:grid;gap:5px;color:var(--text-muted);font-size:10px}.field-grid .wide{grid-column:1/-1}.field-grid input,.field-grid select,.field-grid textarea,.number-grid input{width:100%;min-height:35px;padding:7px 8px;border:1px solid var(--border-color);border-radius:8px;background:var(--bg-base);color:var(--text-main);font:inherit;font-size:12px}.field-grid textarea{resize:vertical}.field-grid input:focus,.field-grid select:focus,.field-grid textarea:focus,.number-grid input:focus{border-color:var(--accent);outline:0}.field-grid .toggle{display:flex;align-items:center;gap:7px;min-height:35px;padding:0 8px;border:1px solid var(--border-color);border-radius:8px;background:var(--bg-base);color:var(--text-main);font-size:11px}.field-grid .toggle input{width:15px;min-height:auto;padding:0;accent-color:var(--accent)}.branch-order{display:flex;align-items:end;gap:5px;justify-content:flex-end;color:var(--text-muted);font-size:10px}.icon-btn{display:grid;place-items:center;width:29px;height:29px;border:1px solid var(--border-color);border-radius:8px;background:var(--bg-base);color:var(--text-main);cursor:pointer}.icon-btn:hover:not(:disabled){border-color:var(--accent);color:var(--accent)}.icon-btn:disabled{opacity:.45;cursor:not-allowed}.property-footer{display:flex;align-items:center;justify-content:space-between;gap:8px;padding:12px 14px;border-top:1px solid var(--border-color);background:var(--bg-surface)}.property-footer>span{display:flex;align-items:center;gap:5px;color:var(--text-muted);font-size:10px}.btn-small{min-height:32px!important;padding:5px 9px!important;font-size:11px!important}
@media(max-width:1100px){.studio-grid{grid-template-columns:minmax(195px,.7fr) minmax(0,1.3fr)}.property-panel{grid-column:1/-1;border-top:1px solid var(--border-color);border-left:0}.property-body{grid-template-columns:minmax(220px,.7fr) minmax(0,1fr);max-height:none}.property-body>.field-section:last-child{grid-column:1/-1}}
@media(max-width:700px){.studio-head{align-items:stretch;flex-direction:column}.studio-actions>*{flex:1}.studio-grid{grid-template-columns:1fr}.lineage-catalog{border-right:0;border-bottom:1px solid var(--border-color)}.catalog-item{margin-top:4px}.lineage-canvas{padding:14px}.evolution-node{margin-left:calc(var(--depth) * 18px)}.evolution-node:not(.root)::before{left:-13px;width:11px}.add-stage{margin-left:0}.property-panel{grid-column:auto}.property-body{grid-template-columns:1fr}.property-body>.field-section:last-child{grid-column:auto}.field-grid,.number-grid{grid-template-columns:1fr}}
.canvas-flow{min-width:0;padding:24px 4px;overflow-x:auto}.evolution-track{display:flex;align-items:center;min-width:max-content;gap:12px;padding:4px 2px 14px}.track-stage{display:flex;align-items:stretch;gap:10px}.evolution-node{display:grid;grid-template-columns:auto auto minmax(0,1fr);align-items:center;gap:9px;width:142px;min-height:132px;max-width:none;margin-left:0;padding:10px}.evolution-node:not(.root)::before{content:none}.node-copy{grid-column:1/-1}.node-copy strong{font-size:13px}.track-connector{display:grid;grid-template-columns:minmax(32px,1fr) auto;align-items:center;min-width:76px;gap:4px;color:var(--text-muted)}.track-connector span{grid-column:1/-1;max-width:92px;margin:auto;color:var(--text-muted);font-size:10px;line-height:1.35;text-align:center}.track-connector i{height:1px;background:var(--border-strong)}.track-connector svg{color:var(--text-main)}.add-stage{display:flex;flex-direction:column;align-items:center;justify-content:center;gap:7px;width:118px;min-height:132px;max-width:none;margin-left:0;padding:11px}
.lineage-studio{--studio-gap:16px;--catalog-padding:14px;--catalog-row-padding:12px;--node-width:172px;--node-height:308px;--node-image:94px;gap:var(--studio-gap)}
.studio-head{justify-content:flex-end;min-height:0;padding:0;border:0;border-radius:0;background:transparent}.studio-actions{gap:9px}.studio-actions .btn{min-height:38px}
.studio-grid{grid-template-columns:230px minmax(660px,1fr) 350px;min-height:calc(100dvh - 218px);border-radius:16px;box-shadow:var(--shadow-card)}
.lineage-catalog{padding:var(--catalog-padding);background:color-mix(in srgb,var(--bg-elevated) 72%,var(--bg-surface))}.catalog-head{padding:2px 1px 13px;color:var(--text-main);font-size:14px;font-weight:750}.catalog-head b{min-width:26px;height:26px;border-radius:8px}.catalog-item{gap:11px;margin-top:7px;padding:var(--catalog-row-padding);border-radius:12px}.catalog-item :deep(.asset-thumbnail.is-medium){width:52px;height:52px;border-radius:11px}.catalog-item strong{font-size:14px}.catalog-item small{font-size:11px}.catalog-item.active{box-shadow:inset 3px 0 0 var(--accent),0 8px 18px color-mix(in srgb,var(--accent) 10%,transparent)}
.lineage-canvas{display:grid;grid-template-rows:auto minmax(0,1fr);padding:22px 24px}.canvas-head{padding-bottom:17px}.canvas-head h3{font-size:21px;letter-spacing:-.025em}.canvas-head small{padding-top:7px;font-size:12px}.canvas-flow{display:grid;align-content:center;gap:26px;min-height:0;padding:24px 0;overflow-x:auto}.evolution-track{align-items:center;justify-content:center;width:max-content;min-width:100%;gap:16px;padding:0}.track-stage{align-items:center;gap:12px}.evolution-node{position:relative;display:grid;grid-template-columns:1fr;align-content:start;justify-items:start;gap:9px;width:var(--node-width);min-height:var(--node-height);padding:16px 15px;border-radius:13px}.evolution-node :deep(.asset-thumbnail.is-medium){justify-self:center;width:var(--node-image);height:var(--node-image);border-radius:16px}.evolution-node .node-step{position:absolute;top:13px;left:13px;z-index:1;background:var(--bg-base)}.node-copy{gap:5px;width:100%}.node-copy strong{font-size:16px;line-height:1.2}.node-copy small{font-size:11px}.status-mark{padding:4px 7px;font-size:10px}.node-stats{display:grid;grid-template-columns:1fr 1fr;gap:7px 10px;width:100%;margin:4px 0 0;padding-top:10px;border-top:1px solid var(--border-color)}.node-stats div{display:flex;align-items:baseline;justify-content:space-between;gap:6px}.node-stats dt,.node-stats dd{margin:0;font-size:10px}.node-stats dt{color:var(--text-muted)}.node-stats dd{font-variant-numeric:tabular-nums;font-weight:750}.track-connector{grid-template-columns:minmax(36px,1fr) auto;min-width:78px;gap:5px}.track-connector span{max-width:104px;font-size:11px}.track-connector i{background:color-mix(in srgb,var(--text-main) 46%,var(--border-color))}.track-connector svg{color:var(--accent)}.add-stage{display:flex;flex-direction:row;align-items:center;justify-content:center;gap:8px;width:100%;min-height:64px;margin:0;padding:13px;border-radius:12px;font-size:13px;letter-spacing:.01em}.add-stage:hover{box-shadow:inset 0 0 0 1px var(--accent)}
.property-panel{background:color-mix(in srgb,var(--bg-elevated) 82%,var(--bg-surface))}.property-head{padding:18px}.property-head h3{font-size:18px}.property-body{gap:14px;max-height:calc(100dvh - 300px);padding:16px}.property-body :deep(.image-editor){grid-template-columns:auto minmax(0,1fr);align-items:center;column-gap:11px}.property-body :deep(.image-editor .asset-thumbnail.is-medium){width:68px;height:68px;border-radius:14px}.property-body :deep(.image-dropzone){min-height:68px;padding:10px}.property-body :deep(.clear-image){grid-column:2;margin-top:-5px}.field-section{padding:13px}.field-section h4{font-size:13px}.field-grid input,.field-grid select,.field-grid textarea,.number-grid input{min-height:36px}.property-footer{padding:13px 16px}
@media(max-width:1560px){.studio-grid{grid-template-columns:220px minmax(520px,1fr)}.property-panel{grid-column:1/-1;border-top:1px solid var(--border-color);border-left:0}.property-body{grid-template-columns:repeat(2,minmax(0,1fr));max-height:none}.property-body>.field-section:last-child{grid-column:1/-1}}
@media(max-width:700px){.studio-head{align-items:stretch}.studio-actions>*{flex:1}.studio-grid{grid-template-columns:1fr;min-height:0}.lineage-catalog{border-right:0;border-bottom:1px solid var(--border-color)}.lineage-canvas{padding:16px}.evolution-track{justify-content:flex-start}.evolution-node{width:154px;min-height:270px}.property-panel{grid-column:auto}.property-body{grid-template-columns:1fr}.property-body>.field-section:last-child{grid-column:auto}}
</style>

<style>
html[data-density='compact'] .lineage-studio{--studio-gap:10px;--catalog-padding:10px;--catalog-row-padding:8px;--node-width:150px;--node-height:270px;--node-image:76px;font-size:13px}
html[data-density='compact'] .lineage-studio .lineage-canvas{padding:16px 18px}
html[data-density='compact'] .lineage-studio .canvas-flow{gap:18px;padding:16px 0}
html[data-density='compact'] .lineage-studio .catalog-item{margin-top:4px}
html[data-density='compact'] .lineage-studio .catalog-item .asset-thumbnail.is-medium{width:42px;height:42px}
html[data-density='compact'] .lineage-studio .evolution-node{padding:12px}
html[data-density='compact'] .lineage-studio .evolution-node .asset-thumbnail.is-medium{width:var(--node-image);height:var(--node-image)}
html[data-density='compact'] .lineage-studio .node-stats{gap:4px 8px;padding-top:7px}
html[data-density='compact'] .lineage-studio .property-body{gap:10px;padding:12px}
html[data-density='compact'] .lineage-studio .field-section{gap:8px;padding:10px}
</style>
