import { AuthResponse, RegisterUserInput, UserProfile } from './types'
import { apiRequest } from './core'
import type { SessionScope } from '@/lib/auth'
export async function login(email: string, password: string): Promise<AuthResponse> {
  return apiRequest<AuthResponse>('/auth/login', {
    method: 'POST',
    body: { email, password },
  })
}

export async function registerUser(input: RegisterUserInput): Promise<AuthResponse> {
  return apiRequest<AuthResponse>('/auth/register', {
    method: 'POST',
    body: input,
  })
}

export async function getMe(token: string, scope: SessionScope = 'tenant'): Promise<UserProfile> {
  return apiRequest<UserProfile>('/auth/me', { token, scope })
}

export async function changeOwnPassword(token: string, currentPassword: string, newPassword: string): Promise<{ status: string; token?: string; expires_at?: number }> {
  return apiRequest<{ status: string; token?: string; expires_at?: number }>('/auth/me/password', {
    method: 'POST',
    token,
    body: { current_password: currentPassword, new_password: newPassword },
  })
}
