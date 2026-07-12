import { expect, test } from '@playwright/test'

const tenantEmail = process.env.PLAYWRIGHT_TENANT_EMAIL || 'demo@local.com'
const tenantPassword = process.env.PLAYWRIGHT_TENANT_PASSWORD || 'MetaOrgSampleTenant!2026'

test('project workbench runs evidence-based AI analysis', async ({ page }) => {
  await page.goto('/')
  await page.getByTestId('login-surface-tenant').click()
  await page.getByTestId('auth-email').fill(tenantEmail)
  await page.getByTestId('auth-password').fill(tenantPassword)
  await page.getByTestId('auth-submit').click()

  await expect(page.getByTestId('session-scope')).toHaveAttribute('data-scope', 'tenant')
  if ((page.viewportSize()?.width ?? 1440) < 1024) {
    await page.getByTestId('mobile-menu-open').click()
  }
  await page.getByTestId('domain-nav-Project').click()
  await page.getByTestId('erp-document-project').click()

  const workbench = page.getByTestId('business-ai-workbench')
  await expect(workbench).toBeVisible()
  await workbench.getByTestId('business-ai-stage-do').click()
  const responsePromise = page.waitForResponse((response) => (
    response.request().method() === 'POST' && response.url().includes('/projects/') && response.url().endsWith('/ai-analyses')
  ))
  await workbench.getByTestId('business-ai-analyze').click()
  const response = await responsePromise
  expect(response.status()).toBe(201)
  const run = await response.json() as { stage: string; status: string; invocation_id: string; analysis: { summary: string } }
  expect(run.stage).toBe('do')
  expect(run.status).toBe('completed')
  await expect(workbench.getByText(run.analysis.summary)).toBeVisible({ timeout: 30_000 })
  await expect(workbench.getByText(new RegExp(run.invocation_id))).toBeVisible()
  await expect(workbench.getByText(/project\.bind_workflow/)).toBeVisible()
})
