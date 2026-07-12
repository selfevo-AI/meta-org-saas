const TOKEN_KEY = 'meta_org.token'
const USER_KEY = 'meta_org.user'
const ORGANIZATION_KEY = 'meta_org.organization_id'
const LEGACY_TOKEN_KEY = 'harness_token'
const LEGACY_USER_KEY = 'harness_user'
const ACTIVE_SURFACE_KEY = 'meta_org.active_surface'
const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

export type SessionScope = 'tenant' | 'platform'

interface SessionKeySet {
  token: string
  user: string
  organization?: string
}

const SESSION_KEYS: Record<SessionScope, SessionKeySet> = {
  tenant: {
    token: 'meta_org.tenant.token',
    user: 'meta_org.tenant.user',
    organization: 'meta_org.tenant.organization_id',
  },
  platform: {
    token: 'meta_org.platform.token',
    user: 'meta_org.platform.user',
  },
}

const LEGACY_SESSION_KEYS = [TOKEN_KEY, USER_KEY, ORGANIZATION_KEY, LEGACY_TOKEN_KEY, LEGACY_USER_KEY]

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

interface SetSessionOptions {
  scope?: SessionScope
  activate?: boolean
}

function isSessionScope(value: string | null): value is SessionScope {
  return value === 'tenant' || value === 'platform'
}

function resolveSessionScope(user?: Partial<SessionUser> | null, scope?: SessionScope): SessionScope {
  if (scope) return scope
  return user?.platform_role ? 'platform' : 'tenant'
}

function migrateLegacySession(): void {
  if (typeof window === 'undefined') return
  const token = localStorage.getItem(TOKEN_KEY) || localStorage.getItem(LEGACY_TOKEN_KEY)
  const rawUser = localStorage.getItem(USER_KEY) || localStorage.getItem(LEGACY_USER_KEY)

  if (token && rawUser) {
    try {
      const user = JSON.parse(rawUser) as SessionUser
      const scope = resolveSessionScope(user)
      if (!localStorage.getItem(SESSION_KEYS[scope].token)) {
        localStorage.setItem(SESSION_KEYS[scope].token, token)
      }
      if (!localStorage.getItem(SESSION_KEYS[scope].user)) {
        localStorage.setItem(SESSION_KEYS[scope].user, JSON.stringify(user))
      }
      const organization = SESSION_KEYS.tenant.organization
      const legacyOrganizationID = normalizeOrganizationId(
        localStorage.getItem(ORGANIZATION_KEY) || user.default_organization_id || user.organizations?.[0]?.id,
      )
      if (scope === 'tenant' && organization && legacyOrganizationID && !localStorage.getItem(organization)) {
        localStorage.setItem(organization, legacyOrganizationID)
      }
      if (!isSessionScope(sessionStorage.getItem(ACTIVE_SURFACE_KEY))) {
        sessionStorage.setItem(ACTIVE_SURFACE_KEY, scope)
      }
    } catch {
      // Bad legacy payloads should not poison the scoped session stores.
    }
  }

  for (const key of LEGACY_SESSION_KEYS) {
    localStorage.removeItem(key)
  }
}

export function setActiveSessionScope(scope: SessionScope): void {
  if (typeof window === 'undefined') return
  sessionStorage.setItem(ACTIVE_SURFACE_KEY, scope)
}

export function getActiveSessionScope(): SessionScope {
  if (typeof window === 'undefined') return 'tenant'
  migrateLegacySession()
  const activeScope = sessionStorage.getItem(ACTIVE_SURFACE_KEY)
  if (isSessionScope(activeScope) && localStorage.getItem(SESSION_KEYS[activeScope].token)) {
    return activeScope
  }
  if (localStorage.getItem(SESSION_KEYS.platform.token) && !localStorage.getItem(SESSION_KEYS.tenant.token)) {
    return 'platform'
  }
  return 'tenant'
}

export function setSession(
  token: string,
  userId: string,
  userType: string,
  details: Partial<SessionUser> = {},
  options: SetSessionOptions = {},
): void {
  if (typeof window === 'undefined') return
  migrateLegacySession()
  const user: SessionUser = { id: userId, type: userType, ...details }
  const scope = resolveSessionScope(user, options.scope)
  localStorage.setItem(SESSION_KEYS[scope].token, token)
  localStorage.setItem(SESSION_KEYS[scope].user, JSON.stringify(user))

  if (scope === 'tenant') {
    const organization = SESSION_KEYS.tenant.organization
    const nextOrgID = normalizeOrganizationId(user.default_organization_id || user.organizations?.[0]?.id)
    if (organization && nextOrgID) {
      localStorage.setItem(organization, nextOrgID)
    } else if (organization) {
      localStorage.removeItem(organization)
    }
  }

  if (options.activate !== false) {
    setActiveSessionScope(scope)
  }
}

export function getToken(scope: SessionScope = getActiveSessionScope()): string | null {
  if (typeof window === 'undefined') return null
  migrateLegacySession()
  return localStorage.getItem(SESSION_KEYS[scope].token)
}

export function getSessionUser(scope: SessionScope = getActiveSessionScope()): SessionUser | null {
  if (typeof window === 'undefined') return null
  migrateLegacySession()
  const raw = localStorage.getItem(SESSION_KEYS[scope].user)
  if (!raw) return null

  try {
    return JSON.parse(raw) as SessionUser
  } catch {
    clearSession(scope)
    return null
  }
}

export function clearSession(scope?: SessionScope): void {
  if (typeof window === 'undefined') return
  migrateLegacySession()
  const scopes: SessionScope[] = scope ? [scope] : ['tenant', 'platform']
  for (const item of scopes) {
    localStorage.removeItem(SESSION_KEYS[item].token)
    localStorage.removeItem(SESSION_KEYS[item].user)
    const organization = SESSION_KEYS[item].organization
    if (organization) localStorage.removeItem(organization)
  }
  if (!scope || sessionStorage.getItem(ACTIVE_SURFACE_KEY) === scope) {
    sessionStorage.removeItem(ACTIVE_SURFACE_KEY)
  }
}

export function isAuthenticated(scope: SessionScope = getActiveSessionScope()): boolean {
  return !!getToken(scope)
}

export function getCurrentOrganizationId(scope: SessionScope = 'tenant'): string | null {
  if (typeof window === 'undefined') return null
  migrateLegacySession()
  const organization = SESSION_KEYS[scope].organization
  if (!organization) return null
  const value = normalizeOrganizationId(localStorage.getItem(organization))
  if (!value) {
    localStorage.removeItem(organization)
  }
  return value
}

export function setCurrentOrganizationId(organizationId: string | null, scope: SessionScope = 'tenant'): void {
  if (typeof window === 'undefined') return
  migrateLegacySession()
  const organization = SESSION_KEYS[scope].organization
  if (!organization) return
  const value = normalizeOrganizationId(organizationId)
  if (value) {
    localStorage.setItem(organization, value)
  } else {
    localStorage.removeItem(organization)
  }
}

export function normalizeOrganizationId(organizationId?: string | null): string | null {
  const value = organizationId?.trim()
  if (!value || value === 'null' || value === 'undefined' || !UUID_PATTERN.test(value)) {
    return null
  }
  return value
}
