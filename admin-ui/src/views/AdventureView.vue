<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { IconBook2, IconCheck, IconDeviceFloppy, IconEdit, IconPlus, IconRefresh, IconSearch, IconTrash } from '@tabler/icons-vue'
import { ApiError } from '../api/client'
import {
  getAdventureCatalog,
  saveAdventureCatalog,
  validateAdventureCatalog,
  type AdventureCatalog,
  type AdventureEntity,
  type AdventureValidationIssue,
} from '../api/adventure'
import { fetchConfig, normalizeImage } from '../api/config'
import PageHeader from '../components/ui/PageHeader.vue'
import UiDrawer from '../components/ui/UiDrawer.vue'
import UiModal from '../components/ui/UiModal.vue'
import UiSearchSelect from '../components/ui/UiSearchSelect.vue'
import UiState from '../components/ui/UiState.vue'
import { useToast } from '../composables/useToast'
import { useUnsavedChanges } from '../composables/useUnsavedChanges'

type CollectionKey = keyof AdventureCatalog
type ModuleKey = 'maps'|'exploration'|'monsters'|'skills'|'loot'|'inventory'|'shop'|'expeditions'|'bosses'|'equipment'
type FieldType = 'text'|'textarea'|'number'|'toggle'|'select'|'datetime'
interface Option { value:string; label:string; group?:string }
interface Field {
  key:string
  label:string
  type:FieldType
  help?:string
  required?:boolean
  min?:number
  max?:number
  step?:number
  options?:Option[]
  source?:CollectionKey
  dynamic?:'objective-target'|'encounter-target'|'reward-target'|'product-target'
  asset?:boolean
  wide?:boolean
  hiddenWhen?: (row:AdventureEntity)=>boolean
}
interface CollectionDefinition {
  key:CollectionKey
  label:string
  singular:string
  description:string
  titleField:string
  keyField?:string
  defaults:object
  fields:Field[]
}
interface ModuleDefinition { key:ModuleKey; label:string; description:string; collections:CollectionKey[] }

const choice = (values:Array<[string,string]>):Option[] => values.map(([value,label])=>({value,label}))
const yesNo = (label='启用'):Field => ({key:'enabled',label,type:'toggle'})
const fields:Record<CollectionKey, CollectionDefinition> = {
  maps:{key:'maps',label:'大地图',singular:'大地图',description:'配置世界入口、地图主题、推荐等级和开放状态。',titleField:'name',keyField:'key',defaults:{name:'',region:'',description:'',image:'',recommended_level:1,enabled:true,sort_order:0},fields:[
    {key:'name',label:'地图名称',type:'text',required:true},{key:'region',label:'世界区域',type:'text',required:true},{key:'description',label:'玩家说明',type:'textarea',wide:true},{key:'image',label:'地图封面',type:'select',asset:true,help:'从内容配置已上传的图片资产中选择',wide:true},{key:'recommended_level',label:'推荐挑战等级',type:'number',min:1},yesNo(),{key:'sort_order',label:'显示顺序',type:'number'}]},
  zones:{key:'zones',label:'区域',singular:'区域',description:'配置地图内的可探索区域、消耗、难度和远征解锁目标。',titleField:'name',keyField:'key',defaults:{map_key:'',name:'',description:'',image:'',recommended_level:1,difficulty_permille:1000,hunger_cost:0,readiness_cost:0,expedition_unlock_objective_key:'',enabled:true,sort_order:0},fields:[
    {key:'map_key',label:'所属大地图',type:'select',source:'maps',required:true},{key:'name',label:'区域名称',type:'text',required:true},{key:'description',label:'区域说明',type:'textarea',wide:true},{key:'image',label:'区域图片',type:'select',asset:true,wide:true},{key:'recommended_level',label:'推荐等级',type:'number',min:1},{key:'difficulty_permille',label:'难度系数',type:'number',min:1,help:'1000 表示标准难度，1200 表示 1.2 倍'},{key:'hunger_cost',label:'单次探索饥饿消耗',type:'number',min:0},{key:'readiness_cost',label:'单次探索体力消耗',type:'number',min:0},{key:'expedition_unlock_objective_key',label:'远征解锁目标',type:'select',source:'objectives',help:'完成该目标后才能挂机远征'},yesNo(),{key:'sort_order',label:'显示顺序',type:'number'}]},
  prerequisites:{key:'prerequisites',label:'区域前置',singular:'区域前置',description:'指定玩家探索某区域前必须完成的区域。',titleField:'zone_key',defaults:{zone_key:'',prerequisite_zone_key:''},fields:[{key:'zone_key',label:'待解锁区域',type:'select',source:'zones',required:true},{key:'prerequisite_zone_key',label:'前置区域',type:'select',source:'zones',required:true}]},
  objectives:{key:'objectives',label:'探索目标',singular:'探索目标',description:'目标贡献探索度，也可记录到图鉴并作为远征解锁条件。',titleField:'name',keyField:'key',defaults:{zone_key:'',name:'',objective_type:'enter',target_key:'',required_count:1,weight:1,codex_category:'',codex_entry:'',codex_progress:0,enabled:true,sort_order:0},fields:[
    {key:'zone_key',label:'所属区域',type:'select',source:'zones',required:true},{key:'name',label:'目标名称',type:'text',required:true},{key:'objective_type',label:'目标类型',type:'select',options:choice([['enter','进入区域'],['monster_kill','击败怪物'],['elite_kill','击败精英'],['landmark','发现地标'],['boss_kill','击败首领']])},{key:'target_key',label:'目标对象',type:'select',dynamic:'objective-target',hiddenWhen:r=>getField(r,'objective_type')==='enter'},{key:'required_count',label:'达成次数',type:'number',min:1},{key:'weight',label:'探索度权重',type:'number',min:1,help:'同一区域所有目标权重共同换算为 100% 探索度'},{key:'codex_category',label:'图鉴分类',type:'select',options:choice([['','不记录图鉴'],['生物','生物'],['遗迹','遗迹'],['生态','生态'],['首领','首领']]),help:'图鉴不是固定三条；只有配置了分类和条目才会推进'},{key:'codex_entry',label:'图鉴条目',type:'text',help:'玩家可见的图鉴条目名称；保存时会校验现有图鉴目录'},{key:'codex_progress',label:'每次图鉴进度',type:'number',min:0},yesNo(),{key:'sort_order',label:'显示顺序',type:'number'}]},
  encounters:{key:'encounters',label:'遭遇',singular:'遭遇',description:'设置区域探索时可能触发的战斗、地标或安全事件。',titleField:'name',keyField:'encounter_key',defaults:{zone_key:'',encounter_type:'monster',target_key:'',name:'',description:'',weight:1,enabled:true,sort_order:0},fields:[
    {key:'zone_key',label:'所属区域',type:'select',source:'zones',required:true},{key:'name',label:'遭遇名称',type:'text',required:true},{key:'encounter_type',label:'遭遇类型',type:'select',options:choice([['monster','普通怪物'],['elite','精英怪物'],['landmark','地标'],['safe','安全事件']])},{key:'target_key',label:'关联对象',type:'select',dynamic:'encounter-target',hiddenWhen:r=>['landmark','safe'].includes(String(getField(r,'encounter_type')))},{key:'description',label:'玩家说明',type:'textarea',wide:true},{key:'weight',label:'出现权重',type:'number',min:1},yesNo(),{key:'sort_order',label:'显示顺序',type:'number'}]},
  encounter_effects:{key:'encounter_effects',label:'遭遇效果',singular:'遭遇效果',description:'地标和安全事件的结构化效果，例如发放物品或恢复准备度。',titleField:'encounter_key',defaults:{encounter_key:'',effect_type:'item',target_key:'',min_value:1,max_value:1,weight:1,enabled:true},fields:[
    {key:'encounter_key',label:'遭遇键',type:'text',required:true},{key:'effect_type',label:'效果类型',type:'select',options:choice([['item','发放物品'],['readiness','恢复准备度'],['currency','发放货币']])},{key:'target_key',label:'目标键',type:'text'},{key:'min_value',label:'最小数值',type:'number',min:0},{key:'max_value',label:'最大数值',type:'number',min:0},{key:'weight',label:'权重',type:'number',min:1},yesNo()]},
  monsters:{key:'monsters',label:'怪物',singular:'怪物',description:'配置等级、战斗属性、AI 与战后奖励。',titleField:'name',keyField:'key',defaults:{name:'',description:'',image:'',level:1,max_health:100,attack:10,defense:5,wisdom:0,adventure_xp:10,ai_profile:'balanced',fixed_loot_pool_key:'',random_loot_pool_key:'',elite:false,enabled:true},fields:[
    {key:'name',label:'怪物名称',type:'text',required:true},{key:'description',label:'怪物说明',type:'textarea',wide:true},{key:'image',label:'怪物图片',type:'select',asset:true,wide:true},{key:'level',label:'等级',type:'number',min:1},{key:'max_health',label:'生命',type:'number',min:1},{key:'attack',label:'攻击',type:'number',min:0},{key:'defense',label:'防御',type:'number',min:0},{key:'wisdom',label:'灵性',type:'number',min:0},{key:'adventure_xp',label:'冒险经验',type:'number',min:0},{key:'ai_profile',label:'战斗 AI',type:'select',options:choice([['balanced','均衡'],['aggressive','进攻'],['defensive','防守'],['tactical','策略']])},{key:'fixed_loot_pool_key',label:'固定奖励池',type:'select',source:'loot_pools'},{key:'random_loot_pool_key',label:'随机奖励池',type:'select',source:'loot_pools'},{key:'elite',label:'精英怪物',type:'toggle'},yesNo()]},
  skills:{key:'skills',label:'战斗技能',singular:'战斗技能',description:'定义倍率、命中、冷却和附加效果。',titleField:'name',keyField:'key',defaults:{name:'',description:'',power_permille:1000,wisdom_permille:0,accuracy_permille:1000,cooldown_turns:0,effect_type:'',effect_value:0,enabled:true},fields:[
    {key:'name',label:'技能名称',type:'text',required:true},{key:'description',label:'技能说明',type:'textarea',wide:true},{key:'power_permille',label:'攻击倍率',type:'number',min:0,help:'1000 表示 100%'},{key:'wisdom_permille',label:'灵性倍率',type:'number',min:0},{key:'accuracy_permille',label:'命中率',type:'number',min:0,max:1000},{key:'cooldown_turns',label:'冷却回合',type:'number',min:0},{key:'effect_type',label:'附加效果',type:'select',options:choice([['','无'],['heal','治疗'],['shield','护盾'],['stun','眩晕'],['defense_down','降低防御']])},{key:'effect_value',label:'效果数值',type:'number',min:0},yesNo()]},
  monster_skills:{key:'monster_skills',label:'怪物技能',singular:'怪物技能',description:'为怪物选择可使用的技能并设置施放权重。',titleField:'monster_key',defaults:{monster_key:'',skill_key:'',weight:1,sort_order:0},fields:[{key:'monster_key',label:'怪物',type:'select',source:'monsters',required:true},{key:'skill_key',label:'技能',type:'select',source:'skills',required:true},{key:'weight',label:'施放权重',type:'number',min:1},{key:'sort_order',label:'优先顺序',type:'number'}]},
  loot_pools:{key:'loot_pools',label:'奖励池',singular:'奖励池',description:'定义奖励抽取次数和是否允许重复抽中。',titleField:'name',keyField:'key',defaults:{name:'',rolls:1,allow_duplicates:false},fields:[{key:'name',label:'奖励池名称',type:'text',required:true},{key:'rolls',label:'随机抽取次数',type:'number',min:0},{key:'allow_duplicates',label:'允许重复抽取',type:'toggle'}]},
  loot_entries:{key:'loot_entries',label:'奖励内容',singular:'奖励内容',description:'只可选择统一物品、账号货币、装备或目标装备的蓝图碎片。',titleField:'reward_key',defaults:{pool_key:'',reward_type:'item',reward_key:'',min_quantity:1,max_quantity:1,weight:1,guaranteed:false,first_clear_only:false,sort_order:0},fields:[
    {key:'pool_key',label:'所属奖励池',type:'select',source:'loot_pools',required:true},{key:'reward_type',label:'奖励类型',type:'select',options:choice([['item','统一物品'],['currency','账号货币'],['equipment','装备成品'],['blueprint_fragment','蓝图碎片']])},{key:'reward_key',label:'奖励对象',type:'select',dynamic:'reward-target',required:true},{key:'min_quantity',label:'最少数量',type:'number',min:1},{key:'max_quantity',label:'最多数量',type:'number',min:1},{key:'weight',label:'随机权重',type:'number',min:0},{key:'guaranteed',label:'固定掉落',type:'toggle'},{key:'first_clear_only',label:'仅首次完成',type:'toggle'},{key:'sort_order',label:'显示顺序',type:'number'}]},
  currencies:{key:'currencies',label:'账号货币',singular:'账号货币',description:'使用稳定英文键统一管理星砂、调查徽章与赛季代币，余额全部进入账号钱包。',titleField:'name',keyField:'key',defaults:{name:'',description:'',image:'',builtin:false,enabled:true,sort_order:0},fields:[{key:'name',label:'显示名称',type:'text',required:true},{key:'description',label:'用途说明',type:'textarea',wide:true},{key:'image',label:'图标',type:'select',asset:true,wide:true},{key:'builtin',label:'内置货币',type:'toggle'},yesNo(),{key:'sort_order',label:'显示顺序',type:'number'}]},
  items:{key:'items',label:'统一物品',singular:'统一物品',description:'陪伴、训练、探索、进化、制造和活动共用同一物品目录与账号背包。',titleField:'name',keyField:'key',defaults:{name:'',description:'',image:'',category:'material',rarity:'common',stackable:true,max_stack:999999,usage:'',sell_price:0,status:'active',type:'材料'},fields:[
    {key:'name',label:'物品名称',type:'text',required:true},{key:'category',label:'物品分类',type:'select',options:choice([['consumable','消耗品'],['gift','礼物'],['material','材料'],['evolution','进化素材'],['event','活动品'],['collectible','收藏品'],['boss_material','首领素材'],['crafting_material','制造材料']])},{key:'rarity',label:'稀有度',type:'select',options:choice([['common','普通'],['fine','优秀'],['rare','稀有'],['epic','史诗'],['legendary','传说']])},{key:'description',label:'玩家说明',type:'textarea',wide:true},{key:'usage',label:'主要用途与来源',type:'textarea',wide:true},{key:'image',label:'物品图片',type:'select',asset:true,wide:true},{key:'stackable',label:'允许堆叠',type:'toggle'},{key:'max_stack',label:'最大堆叠',type:'number',min:1},{key:'sell_price',label:'出售价值',type:'number',min:0},{key:'status',label:'状态',type:'select',options:choice([['active','启用'],['limited','限时'],['hidden','隐藏'],['disabled','停用']])}]},
  shop_items:{key:'shop_items',label:'远征商品',singular:'远征商品',description:'固定商品使用调查徽章结算，并按玩家独立计算限购。',titleField:'name',keyField:'key',defaults:{name:'',description:'',image:'',product_type:'item',product_key:'',quantity:1,price:1,limit_type:'none',limit_quantity:0,enabled:true,sort_order:0},fields:[
    {key:'name',label:'商品名称',type:'text',required:true},{key:'description',label:'玩家说明',type:'textarea',wide:true},{key:'image',label:'商品图片',type:'select',asset:true,wide:true},{key:'product_type',label:'出售类型',type:'select',options:choice([['item','统一物品'],['equipment','装备成品'],['blueprint_fragment','蓝图碎片']])},{key:'product_key',label:'出售对象',type:'select',dynamic:'product-target',required:true},{key:'quantity',label:'单次发放数量',type:'number',min:1},{key:'price',label:'调查徽章价格',type:'number',min:0},{key:'limit_type',label:'限购周期',type:'select',options:choice([['none','不限购'],['daily','每日'],['weekly','每周'],['lifetime','永久']])},{key:'limit_quantity',label:'周期限购数量',type:'number',min:0,hiddenWhen:r=>getField(r,'limit_type')==='none'},yesNo(),{key:'sort_order',label:'显示顺序',type:'number'}]},
  expeditions:{key:'expeditions',label:'挂机远征',singular:'远征规则',description:'仅已完成手动探索解锁的区域可以派遣宠物挂机。',titleField:'name',defaults:{zone_key:'',name:'',description:'',duration_minutes:60,hunger_cost:0,readiness_cost:0,required_item:'',required_quantity:0,fixed_loot_pool_key:'',random_loot_pool_key:'',adventure_xp:0,event_progress_points:0,recommended_power:0,start_image:'',end_image:'',enabled:true},fields:[
    {key:'zone_key',label:'远征区域',type:'select',source:'zones',required:true},{key:'name',label:'远征名称',type:'text',required:true},{key:'description',label:'玩家说明',type:'textarea',wide:true},{key:'duration_minutes',label:'持续分钟',type:'number',min:1},{key:'hunger_cost',label:'饥饿消耗',type:'number',min:0},{key:'readiness_cost',label:'体力消耗',type:'number',min:0},{key:'required_item',label:'消耗材料',type:'select',source:'items'},{key:'required_quantity',label:'材料数量',type:'number',min:0},{key:'recommended_power',label:'推荐战力',type:'number',min:0},{key:'adventure_xp',label:'冒险经验',type:'number',min:0},{key:'event_progress_points',label:'活动进度',type:'number',min:0},{key:'fixed_loot_pool_key',label:'固定奖励池',type:'select',source:'loot_pools'},{key:'random_loot_pool_key',label:'随机奖励池',type:'select',source:'loot_pools'},{key:'start_image',label:'出发图片',type:'select',asset:true,wide:true},{key:'end_image',label:'归来图片',type:'select',asset:true,wide:true},yesNo()]},
  bosses:{key:'bosses',label:'世界首领',singular:'世界首领',description:'设置首领在哪张地图限时出现、刷新频率和挑战规则。',titleField:'name',keyField:'key',defaults:{map_key:'',zone_key:'',monster_key:'',name:'',description:'',schedule_anchor:new Date().toISOString(),spawn_interval_minutes:360,active_duration_minutes:30,recommended_level:1,max_health:1000,attack:50,defense:30,wisdom:0,challenge_cooldown_minutes:5,challenge_limit:0,minimum_contribution:1,defeated_loot_pool_key:'',expired_loot_pool_key:'',enabled:true},fields:[
    {key:'map_key',label:'所属大地图',type:'select',source:'maps',required:true},{key:'zone_key',label:'出现区域',type:'select',source:'zones',required:true},{key:'monster_key',label:'怪物模板',type:'select',source:'monsters',required:true},{key:'name',label:'首领名称',type:'text',required:true},{key:'description',label:'首领说明',type:'textarea',wide:true},{key:'schedule_anchor',label:'刷新基准时间',type:'datetime'},{key:'spawn_interval_minutes',label:'每隔多少分钟出现',type:'number',min:1},{key:'active_duration_minutes',label:'每次持续分钟',type:'number',min:1},{key:'recommended_level',label:'推荐等级',type:'number',min:1},{key:'max_health',label:'生命',type:'number',min:1},{key:'attack',label:'攻击',type:'number',min:0},{key:'defense',label:'防御',type:'number',min:0},{key:'wisdom',label:'灵性',type:'number',min:0},{key:'challenge_cooldown_minutes',label:'挑战冷却分钟',type:'number',min:0},{key:'challenge_limit',label:'周期挑战次数',type:'number',min:0,help:'0 表示不限次数'},{key:'minimum_contribution',label:'最低有效贡献',type:'number',min:1},{key:'defeated_loot_pool_key',label:'击败奖励池',type:'select',source:'loot_pools'},{key:'expired_loot_pool_key',label:'未击败奖励池',type:'select',source:'loot_pools'},yesNo()]},
  boss_reward_tiers:{key:'boss_reward_tiers',label:'贡献奖励',singular:'贡献奖励',description:'按玩家对世界首领的累计贡献追加奖励。',titleField:'boss_key',defaults:{boss_key:'',threshold:1,loot_pool_key:'',description:''},fields:[{key:'boss_key',label:'世界首领',type:'select',source:'bosses',required:true},{key:'threshold',label:'贡献门槛',type:'number',min:1},{key:'loot_pool_key',label:'奖励池',type:'select',source:'loot_pools',required:true},{key:'description',label:'档位说明',type:'text'}]},
  equipment_templates:{key:'equipment_templates',label:'装备',singular:'装备',description:'配置武器、防具与秘宝的基础属性、词条和分解产物。',titleField:'name',keyField:'key',defaults:{name:'',description:'',image:'',slot:'weapon',rarity:'common',required_level:1,base_attack:0,base_defense:0,base_health:0,base_wisdom:0,affix_pool_key:'',min_affixes:0,max_affixes:0,salvage_item:'',salvage_quantity:0,enabled:true},fields:[
    {key:'name',label:'装备名称',type:'text',required:true},{key:'slot',label:'装备槽位',type:'select',options:choice([['weapon','武器'],['armor','防具'],['treasure','秘宝']])},{key:'rarity',label:'稀有度',type:'select',options:choice([['common','普通'],['fine','优秀'],['rare','稀有'],['epic','史诗'],['legendary','传说']])},{key:'description',label:'装备说明',type:'textarea',wide:true},{key:'image',label:'装备图片',type:'select',asset:true,wide:true},{key:'required_level',label:'穿戴等级',type:'number',min:1},{key:'base_attack',label:'基础攻击',type:'number',min:0},{key:'base_defense',label:'基础防御',type:'number',min:0},{key:'base_health',label:'基础生命',type:'number',min:0},{key:'base_wisdom',label:'基础灵性',type:'number',min:0},{key:'affix_pool_key',label:'词条池标识',type:'select',source:'equipment_affixes'},{key:'min_affixes',label:'最少词条',type:'number',min:0},{key:'max_affixes',label:'最多词条',type:'number',min:0},{key:'salvage_item',label:'分解产物',type:'select',source:'items'},{key:'salvage_quantity',label:'分解数量',type:'number',min:0},yesNo()]},
  equipment_affixes:{key:'equipment_affixes',label:'随机词条',singular:'随机词条',description:'配置装备生成时可出现的属性范围和权重。',titleField:'name',keyField:'key',defaults:{pool_key:'default',name:'',attribute:'attack',min_value:1,max_value:1,weight:1,enabled:true},fields:[{key:'name',label:'词条名称',type:'text',required:true},{key:'pool_key',label:'词条池',type:'text',required:true},{key:'attribute',label:'属性',type:'select',options:choice([['attack','攻击'],['defense','防御'],['health','生命'],['wisdom','灵性']])},{key:'min_value',label:'最小值',type:'number'},{key:'max_value',label:'最大值',type:'number'},{key:'weight',label:'出现权重',type:'number',min:1},yesNo()]},
  equipment_recipes:{key:'equipment_recipes',label:'蓝图与锻造',singular:'装备配方',description:'设置高阶装备所需蓝图碎片和旅途徽章。',titleField:'equipment_key',defaults:{equipment_key:'',blueprint_fragment_item:'',blueprint_fragments:1,currency_cost:0,enabled:true},fields:[{key:'equipment_key',label:'目标装备',type:'select',source:'equipment_templates',required:true},{key:'blueprint_fragments',label:'蓝图碎片目标',type:'number',min:1},{key:'currency_cost',label:'旅途徽章费用',type:'number',min:0},yesNo()]},
  equipment_recipe_materials:{key:'equipment_recipe_materials',label:'锻造材料',singular:'锻造材料',description:'锻造材料只允许选择远征物品。',titleField:'equipment_key',defaults:{equipment_key:'',item_name:'',quantity:1},fields:[{key:'equipment_key',label:'目标装备',type:'select',source:'equipment_templates',required:true},{key:'item_name',label:'远征材料',type:'select',source:'items',required:true},{key:'quantity',label:'需要数量',type:'number',min:1}]},
}

const modules:ModuleDefinition[] = [
  {key:'maps',label:'地图管理',description:'大地图、区域图片、等级和开放顺序。',collections:['maps','zones','prerequisites']},
  {key:'exploration',label:'探索机制',description:'探索目标、探索度、遭遇、图鉴与远征解锁。',collections:['objectives','encounters','encounter_effects']},
  {key:'monsters',label:'怪物管理',description:'怪物属性、AI、技能组合和战后奖励。',collections:['monsters','monster_skills']},
  {key:'skills',label:'战斗技能',description:'技能倍率、命中率、冷却和效果。',collections:['skills']},
  {key:'loot',label:'奖励池',description:'固定奖励、随机奖励与概率规则。',collections:['loot_pools','loot_entries']},
  {key:'inventory',label:'远征背包',description:'独立远征物品与旅途徽章定义。',collections:['items','currencies']},
  {key:'shop',label:'远征商店',description:'固定商品、旅途徽章价格和个人限购。',collections:['shop_items']},
  {key:'expeditions',label:'远征配置',description:'区域派遣时长、消耗、战力与奖励。',collections:['expeditions']},
  {key:'bosses',label:'世界首领',description:'限时刷新、挑战限制、贡献档位和奖励。',collections:['bosses','boss_reward_tiers']},
  {key:'equipment',label:'装备与锻造',description:'装备、秘宝、词条、蓝图、材料和分解。',collections:['equipment_templates','equipment_affixes','equipment_recipes','equipment_recipe_materials']},
]

const toast=useToast()
const loading=ref(true), saving=ref(false), error=ref(''), search=ref('')
const revision=ref(0), catalog=ref<AdventureCatalog|null>(null), snapshot=ref('')
const activeModule=ref<ModuleKey>('maps'), activeCollection=ref<CollectionKey>('maps')
const drawerOpen=ref(false), editingIndex=ref(-1), draft=ref<AdventureEntity|null>(null)
const helpOpen=ref(false), deleteIndex=ref<number|null>(null), issues=ref<AdventureValidationIssue[]>([])
const imageOptions=ref<Option[]>([])
const dirty=computed(()=>!!catalog.value && snapshot.value!==JSON.stringify(catalog.value))
useUnsavedChanges(dirty)
const moduleInfo=computed(()=>modules.find(item=>item.key===activeModule.value) ?? modules[0])
const definition=computed(()=>fields[activeCollection.value])
const currentRows=computed<AdventureEntity[]>(()=>catalog.value ? catalog.value[activeCollection.value] as AdventureEntity[] : [])
const visibleRows=computed(()=>{const needle=search.value.trim().toLowerCase();if(!needle)return currentRows.value;return currentRows.value.filter(row=>`${rowTitle(row)} ${rowSubtitle(row)}`.toLowerCase().includes(needle))})

function getField(row:AdventureEntity,key:string):unknown{return Reflect.get(row as object,key)}
function setField(row:AdventureEntity,key:string,value:unknown){Reflect.set(row as object,key,value)}
function optionRows(source:CollectionKey):AdventureEntity[]{return catalog.value ? catalog.value[source] as AdventureEntity[] : []}
function rowKey(row:AdventureEntity){return String(getField(row,definition.value.keyField ?? definition.value.titleField) ?? '')}
function rowTitle(row:AdventureEntity){return String(getField(row,definition.value.titleField) || '未命名')}
function rowSubtitle(row:AdventureEntity){const key=definition.value.keyField ? getField(row,definition.value.keyField) : '';const enabled=getField(row,'enabled');return [key,enabled===false?'已停用':''].filter(Boolean).join(' · ') || definition.value.description}
function sourceOptions(source:CollectionKey):Option[]{return optionRows(source).map(row=>({value:String(getField(row,fields[source].keyField ?? fields[source].titleField) ?? ''),label:rowTitleFor(source,row)}))}
function rowTitleFor(source:CollectionKey,row:AdventureEntity){const def=fields[source];return String(getField(row,def.titleField) || getField(row,def.keyField ?? '') || '未命名')}
function fieldOptions(field:Field,row:AdventureEntity):Option[]{
  if(field.asset)return imageOptions.value
  if(field.options)return field.options
  if(field.source){
    if(field.key==='affix_pool_key'){const seen=new Set<string>();return optionRows('equipment_affixes').flatMap(item=>{const value=String(getField(item,'pool_key')||'');if(!value||seen.has(value))return[];seen.add(value);return[{value,label:value}]})}
    return sourceOptions(field.source)
  }
  if(field.dynamic==='objective-target'){const type=String(getField(row,'objective_type'));if(type==='monster_kill'||type==='elite_kill')return sourceOptions('monsters');if(type==='boss_kill')return sourceOptions('bosses');return[]}
  if(field.dynamic==='encounter-target')return sourceOptions('monsters')
  if(field.dynamic==='reward-target'){const type=String(getField(row,'reward_type'));if(type==='item')return sourceOptions('items');if(type==='currency')return sourceOptions('currencies');return sourceOptions('equipment_templates')}
  if(field.dynamic==='product-target'){const type=String(getField(row,'product_type'));return type==='item'?sourceOptions('items'):sourceOptions('equipment_templates')}
  return[]
}
function safeKey(prefix:string){return `${prefix}-${crypto.randomUUID().slice(0,8)}`}
function createRow(){const def=definition.value;const value={...structuredClone(def.defaults)};if(def.keyField)Reflect.set(value,def.keyField,safeKey(def.key.replaceAll('_','-')));draft.value=value as AdventureEntity;editingIndex.value=-1;drawerOpen.value=true}
function editRow(row:AdventureEntity){editingIndex.value=currentRows.value.indexOf(row);draft.value=structuredClone(row);drawerOpen.value=true}
function saveRow(){if(!draft.value||!catalog.value)return;for(const field of definition.value.fields){if(field.required&&!String(getField(draft.value,field.key)??'').trim()){toast.error(`请填写${field.label}`);return}}
  const rows=catalog.value[activeCollection.value] as AdventureEntity[];if(editingIndex.value<0)rows.push(draft.value);else rows.splice(editingIndex.value,1,draft.value);drawerOpen.value=false;toast.success(`${definition.value.singular}已加入未发布草稿`)}
function requestDelete(row:AdventureEntity){deleteIndex.value=currentRows.value.indexOf(row)}
function confirmDelete(){if(deleteIndex.value===null||!catalog.value)return;const row=currentRows.value[deleteIndex.value];if(Boolean(getField(row,'builtin'))){toast.error('内置配置不可删除，只能停用');deleteIndex.value=null;return}(catalog.value[activeCollection.value] as AdventureEntity[]).splice(deleteIndex.value,1);deleteIndex.value=null;toast.success('已从草稿移除；发布前会检查所有引用关系')}
function selectModule(key:ModuleKey){activeModule.value=key;activeCollection.value=(modules.find(item=>item.key===key)??modules[0]).collections[0];search.value='';issues.value=[]}
function inputValue(event:Event,field:Field){const target=event.target as HTMLInputElement;return field.type==='number'?Number(target.value):field.type==='datetime'?new Date(target.value).toISOString():target.value}
function dateValue(value:unknown){if(!value)return'';const date=new Date(String(value));return Number.isNaN(date.getTime())?'':new Date(date.getTime()-date.getTimezoneOffset()*60000).toISOString().slice(0,16)}
async function load(){loading.value=true;error.value='';try{const [data,imageRows]=await Promise.all([getAdventureCatalog(),fetchConfig('images')]);revision.value=data.revision;catalog.value=data.catalog;snapshot.value=JSON.stringify(data.catalog);imageOptions.value=imageRows.map(normalizeImage).filter(image=>image.Path).map(image=>({value:image.Path,label:image.Name||image.Path,group:'图片资产'}));issues.value=[]}catch(reason){error.value=reason instanceof ApiError?reason.message:'冒险配置读取失败'}finally{loading.value=false}}
async function validate(){if(!catalog.value)return;try{const result=await validateAdventureCatalog(catalog.value);issues.value=result.issues;if(result.valid)toast.success('世界配置校验通过，可以发布');else toast.error(`发现 ${result.issues.length} 项问题，请按提示修正`)}catch(reason){toast.error(reason instanceof Error?reason.message:'配置校验失败')}}
async function publish(){if(!catalog.value)return;saving.value=true;try{const check=await validateAdventureCatalog(catalog.value);issues.value=check.issues;if(!check.valid){toast.error('校验未通过，数据库没有发生任何变化');return}const result=await saveAdventureCatalog(revision.value,catalog.value);revision.value=result.revision;snapshot.value=JSON.stringify(catalog.value);toast.success('全部冒险配置已原子发布')}catch(reason){toast.error(reason instanceof Error?reason.message:'发布失败，数据库没有发生变化')}finally{saving.value=false}}
onMounted(load)
</script>

<template>
  <section class="adventure-page">
    <PageHeader eyebrow="Adventure world" title="冒险世界" description="维护地图探索、战斗资源、远征规则与装备成长。">
      <template #actions>
        <button class="btn btn-ghost" type="button" @click="helpOpen=true"><IconBook2 :size="17"/>玩法说明</button>
        <button class="btn btn-ghost" type="button" :disabled="loading" @click="load"><IconRefresh :size="17"/>刷新</button>
        <button class="btn btn-ghost" type="button" :disabled="loading" @click="validate"><IconCheck :size="17"/>校验世界配置</button>
        <button class="btn btn-primary" type="button" :disabled="saving||!dirty" @click="publish"><IconDeviceFloppy :size="17"/>{{saving?'发布中…':'发布全部修改'}}</button>
      </template>
    </PageHeader>

    <UiState v-if="error" tone="error" title="冒险配置加载失败" :description="error" action-label="重试" @action="load"/>
    <UiState v-else-if="loading" tone="loading" title="正在读取冒险世界" description="正在关联地图、怪物、奖励和装备配置。"/>
    <template v-else-if="catalog">
      <div class="draft-bar" :class="{dirty}"><span class="draft-dot"/><div><b>{{dirty?'有尚未发布的修改':'当前配置已发布'}}</b><small>{{dirty?'修改会跨模块保留，只有点击“发布全部修改”才会写入数据库。':'可以安全切换模块开始编辑。'}}</small></div></div>

      <nav class="module-nav" aria-label="冒险配置模块">
        <button v-for="item in modules" :key="item.key" type="button" :class="{active:activeModule===item.key}" @click="selectModule(item.key)"><b>{{item.label}}</b><small>{{item.description}}</small></button>
      </nav>

      <section class="editor-shell">
        <header class="editor-head">
          <div><span>{{moduleInfo.label}}</span><h2>{{moduleInfo.description}}</h2></div>
          <div class="collection-tabs" role="tablist"><button v-for="key in moduleInfo.collections" :key="key" type="button" :class="{active:activeCollection===key}" @click="activeCollection=key;search='';issues=[]">{{fields[key].label}}</button></div>
        </header>

        <div v-if="issues.length" class="issues" role="alert"><b>发布前需要修正</b><button v-for="issue in issues" :key="issue.message" type="button" @click="issue.module&&selectModule(issue.module as ModuleKey)">{{issue.message}}</button></div>

        <div class="entity-toolbar"><label><IconSearch :size="17"/><input v-model="search" :placeholder="`搜索${definition.label}`"/></label><button class="btn btn-primary" type="button" @click="createRow"><IconPlus :size="17"/>新增{{definition.singular}}</button></div>

        <div v-if="visibleRows.length" class="entity-list">
          <article v-for="row in visibleRows" :key="rowKey(row)">
            <div class="entity-copy"><div v-if="String(getField(row,'image')||'')" class="entity-image"><img :src="String(getField(row,'image'))" alt=""/></div><div><h3>{{rowTitle(row)}}</h3><p>{{rowSubtitle(row)}}</p></div></div>
            <div class="entity-actions"><button class="ui-icon-button" type="button" :aria-label="`编辑${rowTitle(row)}`" @click="editRow(row)"><IconEdit :size="17"/></button><button class="ui-icon-button danger" type="button" :aria-label="`删除${rowTitle(row)}`" @click="requestDelete(row)"><IconTrash :size="17"/></button></div>
          </article>
        </div>
        <UiState v-else tone="empty" :title="search?'没有匹配结果':`还没有${definition.label}`" :description="search?'换一个关键词试试。':`新增后先保存在跨模块草稿中，统一校验并发布。`" :action-label="search?'清除搜索':`新增${definition.singular}`" @action="search?search='':createRow()"/>
      </section>
    </template>

    <UiDrawer :open="drawerOpen" :title="editingIndex<0?`新增${definition.singular}`:`编辑${rowTitle(draft!)}`" :description="definition.description" @close="drawerOpen=false">
      <form v-if="draft" class="entity-form" @submit.prevent="saveRow">
        <template v-for="field in definition.fields" :key="field.key">
          <label v-if="!field.hiddenWhen?.(draft)" :class="{wide:field.wide,toggle:field.type==='toggle'}">
            <template v-if="field.type==='toggle'"><input type="checkbox" :checked="Boolean(getField(draft,field.key))" @change="setField(draft,field.key,($event.target as HTMLInputElement).checked)"/><span><b>{{field.label}}</b><small v-if="field.help">{{field.help}}</small></span></template>
            <template v-else><span>{{field.label}}<i v-if="field.required">必填</i></span>
              <textarea v-if="field.type==='textarea'" :value="String(getField(draft,field.key)??'')" rows="4" @input="setField(draft,field.key,($event.target as HTMLTextAreaElement).value)"/>
              <UiSearchSelect v-else-if="field.type==='select'" :model-value="String(getField(draft,field.key)??'')" :options="fieldOptions(field,draft)" :placeholder="field.asset?'选择图片资产':'请选择或输入名称搜索'" @update:model-value="setField(draft,field.key,$event)"/>
              <input v-else-if="field.type==='datetime'" type="datetime-local" :value="dateValue(getField(draft,field.key))" @input="setField(draft,field.key,inputValue($event,field))"/>
              <input v-else :type="field.type==='number'?'number':'text'" :value="String(getField(draft,field.key)??'')" :min="field.min" :max="field.max" :step="field.step" @input="setField(draft,field.key,inputValue($event,field))"/>
              <small v-if="field.help">{{field.help}}</small>
            </template>
          </label>
        </template>
      </form>
      <template #footer><div class="drawer-actions"><button class="btn btn-ghost" type="button" @click="drawerOpen=false">取消</button><button class="btn btn-primary" type="button" @click="saveRow">保存到草稿</button></div></template>
    </UiDrawer>

    <UiModal :open="deleteIndex!==null" title="从草稿中移除" description="实际删除会在发布时统一执行。" size="small" @close="deleteIndex=null"><p class="confirm-copy">如果该内容仍被地图、怪物、奖励池、商店或配方引用，完整校验会阻止发布，数据库不会产生部分修改。</p><template #footer><button class="btn btn-ghost" @click="deleteIndex=null">取消</button><button class="btn btn-danger" @click="confirmDelete">确认移除</button></template></UiModal>
    <UiModal :open="helpOpen" title="冒险玩法与配置关系" description="玩家从手动探索开始，逐步解锁远征与装备成长。" size="large" @close="helpOpen=false">
      <div class="guide"><ol><li><b>选择大地图和区域</b><span>等级与开放状态决定玩家能否进入。</span></li><li><b>手动探索</b><span>消耗饥饿和体力，随机触发怪物、地标或安全遭遇。</span></li><li><b>完成探索目标</b><span>目标按权重推进区域探索度；配置了图鉴字段的目标同时推进对应图鉴。</span></li><li><b>解锁挂机远征</b><span>只有完成区域指定解锁目标后，玩家才能派宠物远征。</span></li><li><b>获得远征资源</b><span>奖励进入独立远征背包或旅途徽章钱包，不影响普通背包和金币。</span></li><li><b>打造装备与挑战首领</b><span>低阶装备可直接掉落，高阶装备需要蓝图、远征材料和旅途徽章。</span></li></ol><aside><b>活动的关系</b><p>活动只统计所选区域的远征进度，不会创建地图、修改探索度或生成图鉴。</p></aside></div>
    </UiModal>
  </section>
</template>

<style scoped>
.adventure-page{display:grid;gap:16px}.draft-bar{display:flex;align-items:center;gap:11px;padding:12px 15px;border:1px solid var(--border-color);border-radius:12px;background:var(--bg-surface)}.draft-bar.dirty{border-color:color-mix(in srgb,var(--warning-strong) 42%,var(--border-color))}.draft-dot{width:9px;height:9px;border-radius:50%;background:var(--success)}.draft-bar.dirty .draft-dot{background:var(--warning-strong)}.draft-bar div{display:grid;gap:2px}.draft-bar b{font-size:13px}.draft-bar small{color:var(--text-muted)}.module-nav{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:1px;padding:1px;border:1px solid var(--border-color);border-radius:14px;overflow:hidden;background:var(--border-color)}.module-nav button{display:grid;gap:4px;min-height:72px;padding:13px 14px;border:0;background:var(--bg-surface);color:var(--text-main);text-align:left;cursor:pointer}.module-nav button:hover{background:var(--bg-hover)}.module-nav button.active{background:var(--accent-soft);color:var(--accent)}.module-nav b{font-size:13px}.module-nav small{color:var(--text-muted);font-size:10px;line-height:1.35}.editor-shell{display:grid;gap:16px;min-height:480px;padding:20px;border:1px solid var(--border-color);border-radius:var(--radius-card);background:var(--bg-surface)}.editor-head{display:flex;align-items:flex-end;justify-content:space-between;gap:18px;padding-bottom:16px;border-bottom:1px solid var(--border-color)}.editor-head span{color:var(--accent);font-size:11px;font-weight:800;letter-spacing:.08em}.editor-head h2{margin:4px 0 0;font-size:18px}.collection-tabs{display:flex;flex-wrap:wrap;justify-content:flex-end;gap:6px}.collection-tabs button{padding:8px 11px;border:1px solid var(--border-color);border-radius:9px;background:transparent;color:var(--text-muted);font:inherit;font-size:12px;cursor:pointer}.collection-tabs button.active{border-color:var(--accent);background:var(--accent-soft);color:var(--accent);font-weight:700}.issues{display:grid;gap:7px;padding:13px;border:1px solid color-mix(in srgb,var(--danger) 35%,var(--border-color));border-radius:11px;background:color-mix(in srgb,var(--danger) 7%,var(--bg-surface))}.issues b{color:var(--danger)}.issues button{padding:0;border:0;background:none;color:var(--text-main);font:inherit;font-size:12px;text-align:left;cursor:pointer}.issues button:hover{text-decoration:underline}.entity-toolbar{display:flex;align-items:center;justify-content:space-between;gap:12px}.entity-toolbar label{display:flex;align-items:center;gap:9px;width:min(420px,100%);padding:0 12px;border:1px solid var(--border-color);border-radius:10px;background:var(--bg-base);color:var(--text-muted)}.entity-toolbar input{width:100%;height:40px;border:0;outline:0;background:transparent;color:var(--text-main)}.entity-list{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;align-content:start}.entity-list article{display:flex;align-items:center;justify-content:space-between;gap:14px;min-height:88px;padding:13px;border:1px solid var(--border-color);border-radius:12px;background:var(--bg-base)}.entity-list article:hover{border-color:var(--border-strong);background:var(--bg-hover)}.entity-copy{display:flex;align-items:center;gap:12px;min-width:0}.entity-image{display:grid;place-items:center;flex:0 0 62px;width:62px;height:62px;overflow:hidden;border:1px solid var(--border-color);border-radius:10px;background:var(--bg-subtle)}.entity-image img{width:100%;height:100%;object-fit:contain;object-position:center}.entity-copy div:last-child{min-width:0}.entity-copy h3{margin:0;overflow:hidden;font-size:14px;text-overflow:ellipsis;white-space:nowrap}.entity-copy p{margin:5px 0 0;overflow:hidden;color:var(--text-muted);font-size:11px;text-overflow:ellipsis;white-space:nowrap}.entity-actions{display:flex;gap:5px}.entity-actions .danger{color:var(--danger)}.entity-form{display:grid;grid-template-columns:1fr 1fr;gap:16px}.entity-form>label{display:grid;align-content:start;gap:7px}.entity-form>label.wide{grid-column:1/-1}.entity-form>label>span{display:flex;align-items:center;justify-content:space-between;color:var(--text-muted);font-size:12px;font-weight:650}.entity-form i{color:var(--accent);font-size:10px;font-style:normal}.entity-form input:not([type=checkbox]),.entity-form select,.entity-form textarea{width:100%;padding:10px 11px;border:1px solid var(--border-color);border-radius:9px;outline:0;background:var(--bg-base);color:var(--text-main);font:inherit}.entity-form textarea{resize:vertical}.entity-form input:focus,.entity-form select:focus,.entity-form textarea:focus{border-color:var(--accent)}.entity-form small{color:var(--text-muted);font-size:10px;line-height:1.45}.entity-form .toggle{display:flex;align-items:center;gap:10px;min-height:44px;padding:10px 12px;border:1px solid var(--border-color);border-radius:9px;background:var(--bg-base)}.entity-form .toggle input{width:17px;height:17px;accent-color:var(--accent)}.entity-form .toggle span{display:grid;gap:2px}.entity-form .toggle b{font-size:12px}.drawer-actions{display:flex;justify-content:flex-end;gap:8px}.confirm-copy{margin:0;color:var(--text-muted);line-height:1.7}.guide ol{display:grid;gap:0;margin:0;padding:0;list-style:none;counter-reset:step}.guide li{display:grid;grid-template-columns:32px 1fr;gap:2px 12px;padding:13px 0;border-bottom:1px solid var(--border-color);counter-increment:step}.guide li::before{grid-row:1/3;display:grid;place-items:center;width:28px;height:28px;border-radius:50%;background:var(--accent-soft);color:var(--accent);font-weight:800;content:counter(step)}.guide li span{color:var(--text-muted);font-size:12px}.guide aside{margin-top:18px;padding:15px;border-radius:12px;background:var(--accent-soft)}.guide aside p{margin:5px 0 0;color:var(--text-muted)}
@media(max-width:1100px){.module-nav{grid-template-columns:repeat(3,1fr)}.entity-list{grid-template-columns:1fr}}@media(max-width:700px){.module-nav{display:flex;overflow:auto}.module-nav button{min-width:170px}.editor-head,.entity-toolbar{align-items:stretch;flex-direction:column}.collection-tabs{justify-content:flex-start}.entity-form{grid-template-columns:1fr}.entity-form>label.wide{grid-column:auto}.entity-toolbar .btn{justify-content:center}}
</style>
