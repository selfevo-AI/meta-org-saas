import { expect, test } from '@playwright/test'

const platformEmail = process.env.PLAYWRIGHT_PLATFORM_EMAIL || 'platform-admin@local.test'
const platformPassword = process.env.PLAYWRIGHT_PLATFORM_PASSWORD || 'MetaOrgSaasDev!2026'
const tenantEmail = process.env.PLAYWRIGHT_TENANT_EMAIL || 'demo@local.com'
const tenantPassword = process.env.PLAYWRIGHT_TENANT_PASSWORD || 'MetaOrgSampleTenant!2026'

async function expectNoHorizontalOverflow(page: import('@playwright/test').Page) {
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)
  expect(overflow).toBeLessThanOrEqual(1)
}

async function openNavigationIfNeeded(page: import('@playwright/test').Page) {
  if ((page.viewportSize()?.width ?? 1440) < 1024) {
    await page.getByTestId('mobile-menu-open').click()
  }
}

test('login surfaces remain usable without horizontal overflow', async ({ page }) => {
  await page.goto('/')

  await expect(page.getByTestId('login-shell')).toBeVisible()
  await expect(page.getByTestId('login-surface-tenant')).toBeVisible()
  await expect(page.getByTestId('login-surface-platform')).toBeVisible()
  await page.getByTestId('login-surface-platform').click()
  await expect(page.getByTestId('auth-email')).toBeVisible()
  await expect(page.getByTestId('auth-password')).toBeVisible()

  await expectNoHorizontalOverflow(page)
})

test('platform administrator session is explicitly scoped', async ({ page }) => {
  await page.goto('/')
  await page.getByTestId('login-surface-platform').click()
  await page.getByTestId('auth-email').fill(platformEmail)
  await page.getByTestId('auth-password').fill(platformPassword)
  await page.getByTestId('auth-submit').click()

  await expect(page.getByTestId('session-scope')).toHaveAttribute('data-scope', 'platform')
  await expect(page.getByTestId('session-scope')).toBeVisible()
  await expect(page.getByTestId('login-shell')).toHaveCount(0)
  await openNavigationIfNeeded(page)
  await page.getByTestId('domain-nav-PlatformAdmin:models').click()
  await expect(page.getByTestId('system-admin-workspace')).toBeVisible({ timeout: 30_000 })
  await expectNoHorizontalOverflow(page)
})

test('tenant session is explicitly scoped', async ({ page }) => {
  await page.goto('/')
  await page.getByTestId('login-surface-tenant').click()
  await page.getByTestId('auth-email').fill(tenantEmail)
  await page.getByTestId('auth-password').fill(tenantPassword)
  await page.getByTestId('auth-submit').click()

  await expect(page.getByTestId('session-scope')).toHaveAttribute('data-scope', 'tenant')
  await expect(page.getByTestId('session-scope')).toBeVisible()
  await expect(page.getByTestId('login-shell')).toHaveCount(0)
  await openNavigationIfNeeded(page)
  await page.getByTestId('domain-nav-Project').click()
  await expect(page.getByTestId('erp-business-module-workspace')).toBeVisible({ timeout: 30_000 })
  await expectNoHorizontalOverflow(page)
})
