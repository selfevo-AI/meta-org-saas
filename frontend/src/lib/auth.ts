const TOKEN_KEY = 'meta_org.token'
const USER_KEY = 'meta_org.user'
const ORGANIZATION_KEY = 'meta_org.organization_id'
const LEGACY_TOKEN_KEY = 'harness_token'
const LEGACY_USER_KEY = 'harness_user'

export interface SessionOrganization {
  id: string
  name: string
  description?: string
  membership_id?: string
  authority_tier?: string
  is_owner?: boolean
}

export interface SessionUser {
  id: string
  type: string
  onboarding_required?: boolean
  default_organization_id?: string
  platform_role?: string
  organizations?: SessionOrganization[]
  enabled_modules?: Record<string, boolean>
}

function migrateLegacySession(): void {
  if (typeof window === 'undefined') return
  const token = localStorage.getItem(TOKEN_KEY) || localStorage.getItem(LEGACY_TOKEN_KEY)
  const user = localStorage.getItem(USER_KEY) || localStorage.getItem(LEGACY_USER_KEY)
  if (token) localStorage.setItem(TOKEN_KEY, token)
  if (user) localStorage.setItem(USER_KEY, user)
  localStorage.removeItem(LEGACY_TOKEN_KEY)
  localStorage.removeItem(LEGACY_USER_KEY)
}

export function setSession(token: string, userId: string, userType: string, details: Partial<SessionUser> = {}): void {
  if (typeof window === 'undefined') return
  const user: SessionUser = { id: userId, type: userType, ...details }
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(USER_KEY, JSON.stringify(user))
  if (user.platform_role) {
    localStorage.removeItem(ORGANIZATION_KEY)
    return
  }
  const nextOrgID = user.default_organization_id || user.organizations?.[0]?.id
  if (nextOrgID) {
    localStorage.setItem(ORGANIZATION_KEY, nextOrgID)
  } else {
    localStorage.removeItem(ORGANIZATION_KEY)
  }
}

export function getToken(): string | null {
  if (typeof window === 'undefined') return null
  migrateLegacySession()
  return localStorage.getItem(TOKEN_KEY)
}

export function getSessionUser(): SessionUser | null {
  if (typeof window === 'undefined') return null
  migrateLegacySession()
  const raw = localStorage.getItem(USER_KEY)
  if (!raw) return null

  try {
    return JSON.parse(raw) as SessionUser
  } catch {
    clearSession()
    return null
  }
}

export function clearSession(): void {
  if (typeof window === 'undefined') return
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
  localStorage.removeItem(ORGANIZATION_KEY)
  localStorage.removeItem(LEGACY_TOKEN_KEY)
  localStorage.removeItem(LEGACY_USER_KEY)
}

export function isAuthenticated(): boolean {
  return !!getToken()
}

export function getCurrentOrganizationId(): string | null {
  if (typeof window === 'undefined') return null
  return localStorage.getItem(ORGANIZATION_KEY)
}

export function setCurrentOrganizationId(organizationId: string | null): void {
  if (typeof window === 'undefined') return
  if (organizationId) {
    localStorage.setItem(ORGANIZATION_KEY, organizationId)
  } else {
    localStorage.removeItem(ORGANIZATION_KEY)
  }
}
