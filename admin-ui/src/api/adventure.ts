import { api } from './client'

export interface AdventureMap { key:string; name:string; region:string; description:string; image:string; recommended_level:number; enabled:boolean; sort_order:number }
export interface AdventureZone { key:string; map_key:string; name:string; description:string; image:string; recommended_level:number; difficulty_permille:number; hunger_cost:number; readiness_cost:number; expedition_unlock_objective_key:string; enabled:boolean; sort_order:number }
export interface ZonePrerequisite { id?:number; zone_key:string; prerequisite_zone_key:string }
export interface AdventureObjective { key:string; zone_key:string; name:string; objective_type:string; target_key:string; required_count:number; weight:number; codex_category:string; codex_entry:string; codex_progress:number; enabled:boolean; sort_order:number }
export interface AdventureMonster { key:string; name:string; description:string; image:string; level:number; max_health:number; attack:number; defense:number; wisdom:number; adventure_xp:number; ai_profile:string; fixed_loot_pool_key:string; random_loot_pool_key:string; elite:boolean; enabled:boolean }
export interface AdventureSkill { key:string; name:string; description:string; power_permille:number; wisdom_permille:number; accuracy_permille:number; cooldown_turns:number; effect_type:string; effect_value:number; enabled:boolean }
export interface MonsterSkill { id?:number; monster_key:string; skill_key:string; weight:number; sort_order:number }
export interface AdventureEncounter { id?:number; zone_key:string; encounter_key:string; encounter_type:string; target_key:string; name:string; description:string; weight:number; enabled:boolean; sort_order:number }
export interface AdventureEncounterEffect { id?:number; encounter_key:string; effect_type:string; target_key:string; min_value:number; max_value:number; weight:number; enabled:boolean }
export interface LootPool { key:string; name:string; rolls:number; allow_duplicates:boolean }
export interface LootEntry { id?:number; pool_key:string; reward_type:string; reward_key:string; min_quantity:number; max_quantity:number; weight:number; guaranteed:boolean; first_clear_only:boolean; sort_order:number }
export interface Currency { key:string; name:string; description:string; image:string; builtin:boolean; enabled:boolean; sort_order:number }
export interface UnifiedItem { key:string; name:string; description:string; image:string; category:string; rarity:string; stackable:boolean; max_stack:number; usage:string; sell_price:number; status:string; type:string; reward_type:string; obtain_type:number; open_req:string; effect:string; time:number }
export interface AdventureShopItem { key:string; name:string; description:string; image:string; product_type:string; product_key:string; quantity:number; price:number; limit_type:string; limit_quantity:number; enabled:boolean; sort_order:number }
export interface AdventureExpedition { zone_key:string; name:string; description:string; duration_minutes:number; hunger_cost:number; readiness_cost:number; required_item:string; required_quantity:number; fixed_loot_pool_key:string; random_loot_pool_key:string; adventure_xp:number; event_progress_points:number; recommended_power:number; start_image:string; end_image:string; enabled:boolean }
export interface AdventureBoss { key:string; map_key:string; zone_key:string; monster_key:string; name:string; description:string; schedule_anchor:string; spawn_interval_minutes:number; active_duration_minutes:number; recommended_level:number; max_health:number; attack:number; defense:number; wisdom:number; challenge_cooldown_minutes:number; challenge_limit:number; minimum_contribution:number; defeated_loot_pool_key:string; expired_loot_pool_key:string; enabled:boolean }
export interface BossRewardTier { id?:number; boss_key:string; threshold:number; loot_pool_key:string; description:string }
export interface EquipmentTemplate { key:string; name:string; description:string; image:string; slot:string; rarity:string; required_level:number; base_attack:number; base_defense:number; base_health:number; base_wisdom:number; affix_pool_key:string; min_affixes:number; max_affixes:number; salvage_item:string; salvage_quantity:number; enabled:boolean }
export interface EquipmentAffix { key:string; pool_key:string; name:string; attribute:string; min_value:number; max_value:number; weight:number; enabled:boolean }
export interface EquipmentRecipe { equipment_key:string; blueprint_fragment_item:string; blueprint_fragments:number; currency_cost:number; enabled:boolean }
export interface EquipmentRecipeMaterial { id?:number; equipment_key:string; item_name:string; quantity:number }

export interface AdventureCatalog {
  maps: AdventureMap[]
  zones: AdventureZone[]
  prerequisites: ZonePrerequisite[]
  objectives: AdventureObjective[]
  monsters: AdventureMonster[]
  skills: AdventureSkill[]
  monster_skills: MonsterSkill[]
  encounters: AdventureEncounter[]
  encounter_effects: AdventureEncounterEffect[]
  loot_pools: LootPool[]
  loot_entries: LootEntry[]
  currencies: Currency[]
  items: UnifiedItem[]
  shop_items: AdventureShopItem[]
  expeditions: AdventureExpedition[]
  bosses: AdventureBoss[]
  boss_reward_tiers: BossRewardTier[]
  equipment_templates: EquipmentTemplate[]
  equipment_affixes: EquipmentAffix[]
  equipment_recipes: EquipmentRecipe[]
  equipment_recipe_materials: EquipmentRecipeMaterial[]
}

export type AdventureEntity = AdventureCatalog[keyof AdventureCatalog][number]
export interface AdventureCatalogEnvelope { revision:number; catalog:AdventureCatalog }
export interface AdventureValidationIssue { module:string; entity_key?:string; field?:string; code:string; message:string; reference?:string }
export interface AdventureValidation { valid:boolean; issues:AdventureValidationIssue[]; summary:Record<string, number> }
export interface AdventureSaveResult { saved:boolean; revision:number; summary:Record<string, number> }

export const getAdventureCatalog = () => api.get<AdventureCatalogEnvelope>('/api/admin/adventure/catalog')
export const validateAdventureCatalog = (catalog: AdventureCatalog) => api.post<AdventureValidation>('/api/admin/adventure/catalog/validate', catalog)
export const saveAdventureCatalog = (revision:number, catalog: AdventureCatalog) => api.put<AdventureSaveResult>('/api/admin/adventure/catalog', { expected_revision:revision, catalog })
