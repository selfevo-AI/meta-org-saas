import type { FullConfig } from '@playwright/test'

type LoginResponse = {
  token: string
}

export default async function globalSetup(_config: FullConfig) {
  const apiBase = process.env.PLAYWRIGHT_API_URL || 'http://127.0.0.1:8080/api/v1'
  const email = process.env.PLAYWRIGHT_PLATFORM_EMAIL || 'platform-admin@local.test'
  const password = process.env.PLAYWRIGHT_PLATFORM_PASSWORD || 'MetaOrgSaasDev!2026'

  const loginResponse = await fetch(`${apiBase}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })
  if (!loginResponse.ok) {
    throw new Error(`Playwright platform login setup failed: ${loginResponse.status}`)
  }
  const login = await loginResponse.json() as LoginResponse
  const sampleResponse = await fetch(`${apiBase}/platform/admin/sample-tenants/business-closure`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${login.token}`,
      'Content-Type': 'application/json',
    },
    body: '{}',
  })
  if (!sampleResponse.ok) {
    throw new Error(`Playwright sample tenant setup failed: ${sampleResponse.status}`)
  }
}
