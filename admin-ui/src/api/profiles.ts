import { api, ApiError } from './client'

export interface ConfigProfile {
  id: string
  name: string
  description: string
  source: string
  schema_version: number
  app_version: string
  builtin: boolean
  active: boolean
  dirty: boolean
  summary: { schemas: number; rows: number }
  created_at: string
  updated_at: string
}

export interface ProfileConflict { kind: string; missing_keys: string[]; affected_count: number }
export interface ProfileList { items: ConfigProfile[]; active_profile_id: string; dirty: boolean }
export interface ProfileValidation { valid: boolean; conflicts: ProfileConflict[]; summary: { schemas: number; rows: number } }

export const getProfiles = () => api.get<ProfileList>('/api/admin/config/profiles')
export const createProfile = (name: string, description: string) => api.post<ConfigProfile>('/api/admin/config/profiles', { name, description })
export const captureProfile = (id: string) => api.post<{ message: string }>(`/api/admin/config/profiles/${id}/capture`)
export const validateProfile = (id: string) => api.post<ProfileValidation>(`/api/admin/config/profiles/${id}/validate`)
export const activateProfile = (id: string, discardChanges = false) => api.post<{ message: string }>(`/api/admin/config/profiles/${id}/activate`, { discard_changes: discardChanges })
export const deleteProfile = (id: string) => api.delete<{ message: string }>(`/api/admin/config/profiles/${id}`)

async function responseMessage(response: Response) {
  try { const body = await response.json(); return body?.msg || body?.error || `请求失败（HTTP ${response.status}）` }
  catch { return `请求失败（HTTP ${response.status}）` }
}

export async function importProfile(file: File): Promise<{ message: string; profile: ConfigProfile; conflicts: ProfileConflict[] }> {
  const body = new FormData(); body.append('file', file)
  const response = await fetch('/api/admin/config/profiles/import', { method: 'POST', credentials: 'same-origin', body })
  if (!response.ok) throw new ApiError(response.status, await responseMessage(response))
  const payload = await response.json()
  if (payload.code !== 0) throw new ApiError(payload.code, payload.msg || '导入失败')
  return payload.data
}

export async function exportProfile(profile: ConfigProfile) {
  const response = await fetch(`/api/admin/config/profiles/${profile.id}/export`, { credentials: 'same-origin' })
  if (!response.ok) throw new ApiError(response.status, await responseMessage(response))
  const blob = await response.blob(); const url = URL.createObjectURL(blob); const anchor = document.createElement('a')
  const disposition = response.headers.get('content-disposition') || ''
  const match = disposition.match(/filename="?([^";]+)"?/i)
  const safeName = profile.name.replace(/[<>:"/\\|?*\u0000-\u001f]/g, '-').trim() || 'qqpet-config'
  anchor.href = url; anchor.download = match?.[1] || `${safeName}_${new Date().toISOString().replace(/[-:]/g, '').slice(0, 15)}Z.qqpet-config`; anchor.click(); URL.revokeObjectURL(url)
}
