import { api } from './client'

export type AdventureCatalog = Record<string, Array<Record<string, unknown>>>
export interface AdventureRuntime { as_of: string; counts: Record<string, number> }
export interface AdventureValidation { valid: boolean; summary: Record<string, number> }

export const getAdventureCatalog = () => api.get<AdventureCatalog>('/api/admin/adventure/catalog')
export const validateAdventureCatalog = (catalog: AdventureCatalog) => api.post<AdventureValidation>('/api/admin/adventure/catalog/validate', catalog)
export const saveAdventureCatalog = (catalog: AdventureCatalog) => api.put<AdventureValidation>('/api/admin/adventure/catalog', catalog)
export const getAdventureRuntime = () => api.get<AdventureRuntime>('/api/admin/adventure/runtime')
