import { UserPreference } from './types'
import { apiRequest } from './core'
export async function getUserPreference(token: string, key: string): Promise<UserPreference> {
  return apiRequest<UserPreference>(`/preferences/${encodeURIComponent(key)}`, { token })
}

export async function saveUserPreference(token: string, key: string, value: Record<string, unknown>): Promise<UserPreference> {
  return apiRequest<UserPreference>(`/preferences/${encodeURIComponent(key)}`, {
    method: 'PUT',
    token,
    body: { value },
  })
}
