import { api } from './client'

export interface UpdateInfo {
  currentVersion: string
  latestVersion: string
  available: boolean
  notes: string
  publishedAt: string
  canAutoUpdate: boolean
  installMode: 'portable' | 'manual'
  reason: string
  releaseUrl: string
}

export type UpdateState =
  | 'idle'
  | 'checking'
  | 'available'
  | 'downloading'
  | 'verifying'
  | 'restarting'
  | 'completed'
  | 'failed'

export interface UpdateStatus {
  state: UpdateState
  progress: number
  downloaded: number
  total: number
  currentVersion: string
  latestVersion: string
  error: string
}

export const checkForUpdates = (force = false) =>
  api.get<UpdateInfo>(`/api/admin/updates/check${force ? '?force=1' : ''}`)

export const installUpdate = (reason: string, confirmation: string) =>
  api.post<UpdateStatus>('/api/admin/updates/install', { reason, confirmation })

export const fetchUpdateStatus = () =>
  api.get<UpdateStatus>('/api/admin/updates/status')

