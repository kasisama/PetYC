import { api } from './client'

export interface OnboardingStatus {
  setup_completed: boolean
  tour_version_completed: number
  current_tour_version: number
}

export const fetchOnboardingStatus = () => api.get<OnboardingStatus>('/api/admin/onboarding/status')
export const completeSetup = () => api.post<{ message: string }>('/api/admin/onboarding/setup-complete')
export const completeTour = (version: number) => api.put<{ message: string }>('/api/admin/onboarding/tour', { version })
