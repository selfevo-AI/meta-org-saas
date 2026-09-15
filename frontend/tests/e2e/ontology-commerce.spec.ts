import { expect, test, type Page } from '@playwright/test'
import { parseWorkspacePath } from '../../src/lib/workspace-routes'

const apiBase = process.env.PLAYWRIGHT_API_URL || 'http://127.0.0.1:8080/api/v1'

async function login(page: Page, scope: 'tenant' | 'platform' = 'tenant') {
  page.setDefaultTimeout(20_000)
  await page.goto('/')
  await page.getByTestId(`login-surface-${scope}`).click()
  await page.getByTestId('auth-email').fill(scope === 'tenant'
    ? process.env.PLAYWRIGHT_TENANT_EMAIL || 'demo@local.com'
    : process.env.PLAYWRIGHT_PLATFORM_EMAIL || 'platform-admin@local.test')
  await page.getByTestId('auth-password').fill(scope === 'tenant'
    ? process.env.PLAYWRIGHT_TENANT_PASSWORD || 'MetaOrgSampleTenant!2026'
    : process.env.PLAYWRIGHT_PLATFORM_PASSWORD || 'MetaOrgSaasDev!2026')
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('session-scope')).toHaveAttribute('data-scope', scope)
  await page.getByRole('button', { name: 'EN', exact: true }).click()
  await expect(page.getByTestId('session-scope')).toContainText(scope === 'tenant' ? 'Tenant operations' : 'Platform control')
  return page.evaluate((activeScope) => ({
    Authorization: `Bearer ${localStorage.getItem(`meta_org.${activeScope}.token`)}`,
    ...(activeScope === 'tenant' ? { 'X-Organization-ID': localStorage.getItem('meta_org.tenant.organization_id') || '' } : {}),
  }), scope)
}

async function openMenu(page: Page) {
  const button = page.getByTestId('mobile-menu-open')
  if (await button.isVisible()) {
    if (await button.getAttribute('aria-expanded') !== 'true') await button.click()
    await expect(button).toHaveAttribute('aria-expanded', 'true')
  }
}

async function navigate(page: Page, domain: string, document?: string) {
  await openMenu(page)
  await page.getByTestId(`domain-nav-${domain}`).click()
  await expect.poll(() => parseWorkspacePath(new URL(page.url()).pathname)?.view).toBe(`domain:${domain}`)
  if (document) {
    await openMenu(page)
    await page.getByTestId(`tenant-document-${document}`).click()
  }
}

async function createHeader(page: Page, table: string, key: string, values: Record<string, string>) {
  const workbench = page.getByTestId('document-workbench')
  await workbench.getByRole('button', { name: 'New record', exact: true }).click()
  const form = workbench.getByTestId('document-header-form')
  await form.locator('input[name="DocEntry"]').fill(key)
  for (const [field, value] of Object.entries(values)) await form.locator(`[name="${field}"]`).fill(value)
  await form.getByRole('button', { name: 'Create', exact: true }).click()
  const review = page.getByRole('dialog', { name: 'Review the new record' })
  await expect(review).toContainText(key)
  const response = page.waitForResponse((candidate) => candidate.request().method() === 'POST' && candidate.url().endsWith(`/erp/${table}`))
  await review.getByRole('button', { name: 'Confirm create', exact: true }).click()
  const created = await response
  expect(created.status(), await created.text()).toBe(201)
  await expect(form.getByLabel('Key', { exact: true })).toHaveText(key)
}

async function createLine(page: Page, item: string, warehouse: string, quantity: string, price: string) {
  const workbench = page.getByTestId('document-workbench')
  await workbench.getByRole('button', { name: 'Add line item', exact: true }).click()
  const form = workbench.getByTestId('document-line-form')
  for (const [field, value] of Object.entries({ ItemCode: item, WhsCode: warehouse, Quantity: quantity, Price: price, TaxRate: '13' })) {
    await form.locator(`[name="${field}"]`).fill(value)
  }
  await page.getByRole('dialog', { name: 'Add line item', exact: true }).getByRole('button', { name: 'Add line item', exact: true }).click()
  const response = page.waitForResponse((candidate) => candidate.request().method() === 'POST' && /\/(POR1|RDR1)$/.test(candidate.url()))
  await page.getByRole('dialog', { name: 'Review the new record' }).getByRole('button', { name: 'Confirm create', exact: true }).click()
  const created = await response
  expect(created.status(), await created.text()).toBe(201)
  await expect(form).toHaveCount(0)
}

async function action(page: Page, name: string, values: Record<string, string> = {}) {
  await page.getByTestId('document-workbench').getByRole('button', { name, exact: true }).click()
  const dialog = page.getByRole('dialog', { name, exact: true })
  for (const [field, value] of Object.entries(values)) await dialog.locator(`[name="${field}"]`).fill(value)
  const response = page.waitForResponse((candidate) => candidate.request().method() === 'POST' && candidate.url().includes('/ontology/objects/') && candidate.url().includes('/actions/'))
  await dialog.getByRole('button', { name: 'Confirm', exact: true }).click()
  const result = await response
  expect(result.status(), await result.text()).toBe(200)
  await expect(dialog).toHaveCount(0)
  return result.json()
}

async function openLinkedObject(page: Page, key: string) {
  const workbench = page.getByTestId('document-workbench')
  await workbench.getByRole('tab', { name: 'Linked Objects', exact: true }).click()
  await workbench.getByRole('button').filter({ has: page.locator('small', { hasText: key }) }).click()
  await expect(page.getByTestId('document-header-form').getByLabel('Key', { exact: true })).toHaveText(key)
}

async function assertLayout(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1)
  await expect(page.locator('body')).not.toContainText(/ontology\.(field|tab|execution|status)\./)
  await expect(page.getByTestId('erp-business-module-workspace').getByRole('alert')).toHaveCount(0)
  const breadcrumb = await page.getByTestId('workspace-breadcrumb').boundingBox()
  const controls = await page.getByTestId('workspace-controls').boundingBox()
  expect(breadcrumb).not.toBeNull()
  expect(controls).not.toBeNull()
  expect(breadcrumb!.x + breadcrumb!.width <= controls!.x + 1 || breadcrumb!.y + breadcrumb!.height <= controls!.y + 1).toBe(true)
}

test('purchase-to-pay and order-to-cash use one governed ledger', async ({ page }, testInfo) => {
  test.setTimeout(180_000)
  const browserErrors: string[] = []
  page.on('pageerror', (error) => browserErrors.push(error.message))
  page.on('console', (message) => {
    // A fresh tenant has no saved layout preference; the shell uses its default.
    const optionalPreference = message.location().url === `${apiBase}/preferences/workspace.layout.widths.v1`
      && message.text().includes('404 (Not Found)')
    if (message.type() === 'error' && !optionalPreference) browserErrors.push(`${message.text()} ${message.location().url}`)
  })
  const headers = await login(page)
  const suffix = `${Date.now()}-${testInfo.project.name.startsWith('mobile') ? 'M' : 'D'}`
  const supplier = `S-${suffix}`, customer = `C-${suffix}`, item = `I-${suffix}`, warehouse = `W-${suffix}`
  for (const [table, key, data] of [
    ['MCRD', supplier, { CardName: 'Commerce supplier', CardType: 'S', Currency: 'CNY' }],
    ['MCRD', customer, { CardName: 'Commerce customer', CardType: 'C', Currency: 'CNY' }],
    ['MITM', item, { ItemName: 'Commerce item', InvntryUom: 'EA', InvntItem: 'Y' }],
    ['MWHS', warehouse, { WhsName: 'Commerce warehouse' }],
  ] as const) {
    const response = await page.request.post(`${apiBase}/erp/${table}`, { headers, data: { key, data } })
    expect(response.status(), await response.text()).toBe(201)
  }
  const getRecord = async (table: string, key: string) => {
    const response = await page.request.get(`${apiBase}/erp/${table}/${encodeURIComponent(key)}`, { headers })
    expect(response.status(), await response.text()).toBe(200)
    return (await response.json()).data as Record<string, unknown>
  }
  const purchase = `PO-${suffix}`, receipt = `GR-${purchase}`, payable = `AP-${receipt}`
  await navigate(page, 'Procurement')
  await createHeader(page, 'MPOR', purchase, { CardCode: supplier })
  await createLine(page, item, warehouse, '10', '10')
  await action(page, 'Submit')
  await action(page, 'Approve')
  await expect(page.getByTestId('document-workbench').getByRole('button', { name: 'Edit record', exact: true })).toBeDisabled()
  await action(page, 'Receive')
  await openLinkedObject(page, receipt)
  await action(page, 'Approve')
  await action(page, 'Post')
  await openLinkedObject(page, payable)
  await action(page, 'Post')
  await navigate(page, 'FinancePayables', 'finance:outgoing_payment')
  await createHeader(page, 'MVPM', `PAY-${suffix}`, { CardCode: supplier, DocTotal: '113' })
  await action(page, 'Allocate', { TargetKey: payable, Amount: '50' })
  await action(page, 'Allocate', { TargetKey: payable, Amount: '63' })
  await expect(page.getByTestId('document-header-form').getByLabel('Open Balance', { exact: true })).toHaveText('0')
  expect((await getRecord('MPCH', payable)).PaidToDate).toBe(113)
  const stockKey = `${item}|${warehouse}`
  expect(await getRecord('MITW', stockKey)).toMatchObject({ OnHand: 10, InventoryValue: 100 })

  const sales = `SO-${suffix}`, delivery = `DL-${sales}`, receivable = `INV-${delivery}`
  await navigate(page, 'Sales')
  await createHeader(page, 'MRDR', sales, { CardCode: customer })
  await createLine(page, item, warehouse, '4', '25')
  await action(page, 'Confirm')
  await action(page, 'Approve')
  await action(page, 'Create Delivery')
  await openLinkedObject(page, delivery)
  await action(page, 'Approve')
  await action(page, 'Post')
  await openLinkedObject(page, receivable)
  await action(page, 'Post')
  await navigate(page, 'FinanceReceivables', 'finance:incoming_payment')
  await createHeader(page, 'MRCT', `REC-${suffix}`, { CardCode: customer, DocTotal: '113' })
  await action(page, 'Allocate', { TargetKey: receivable, Amount: '113' })
  expect((await getRecord('MINV', receivable)).PaidToDate).toBe(113)
  expect(await getRecord('MITW', stockKey)).toMatchObject({ OnHand: 6, InventoryValue: 60, AvgPrice: 10 })
  await page.getByTestId('document-workbench').getByRole('tab', { name: 'Action History' }).click()
  await expect(page.getByTestId('document-workbench').getByText('Completed', { exact: true })).toBeVisible()
  await assertLayout(page)
  await page.screenshot({ path: testInfo.outputPath('ontology-payment-en.png'), fullPage: true })
  await page.getByRole('button', { name: '中文', exact: true }).click()
  await expect(page.getByTestId('document-workbench').getByRole('tab', { name: '执行历史' })).toBeVisible()
  await assertLayout(page)
  await page.screenshot({ path: testInfo.outputPath('ontology-payment-zh.png'), fullPage: true })
  await page.getByRole('button', { name: 'EN', exact: true }).click()
  await navigate(page, 'FinanceAccounting', 'finance:trial_balance')
  const download = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Export CSV', exact: true }).click()
  expect((await download).suggestedFilename()).toBe('trial-balance.csv')
  const trial = await page.request.get(`${apiBase}/finance/gl/trial-balance?currency=CNY`, { headers })
  expect(trial.status()).toBe(200)
  const balance = await trial.json()
  expect(balance.total_debit).toBeGreaterThan(0)
  expect(balance.total_debit).toBe(balance.total_credit)
  await assertLayout(page)
  await page.screenshot({ path: testInfo.outputPath('ontology-trial-balance.png'), fullPage: true })
  expect(browserErrors).toEqual([])
})

test('business archive is read-only without locking core inventory and accounting', async ({ page }, testInfo) => {
  test.setTimeout(120_000)
  const headers = await login(page)
  const suffix = `${Date.now()}-${testInfo.project.name.startsWith('mobile') ? 'M' : 'D'}`
  const documents = [
    { table: 'MIGE', archive: 'material_issue', domain: 'Inventory', core: 'inventory:goods_issue' },
    { table: 'MIGN', archive: 'finished_goods_receipt', domain: 'Inventory', core: 'inventory:goods_receipt' },
    { table: 'MJDT', archive: 'production_journal', domain: 'FinanceAccounting', core: 'finance:journal_entry' },
  ]
  await openMenu(page)
  await expect(page.getByTestId('domain-nav-Manufacturing')).toHaveCount(0)
  await page.getByRole('button', { name: /Business Archive \(Read Only\)/ }).click()
  for (const document of documents) {
    const key = `ARCHIVE-${document.table}-${suffix}`
    const created = await page.request.post(`${apiBase}/erp/${document.table}`, {
      headers, data: { key, data: { Memo: 'Archive boundary regression', Currency: 'CNY' } },
    })
    expect(created.status(), await created.text()).toBe(201)
    for (const archive of [true, false]) {
      await navigate(page, archive ? 'Manufacturing' : document.domain, archive ? `manufacturing:${document.archive}` : document.core)
      const workbench = page.getByTestId('document-workbench')
      await workbench.getByRole('textbox', { name: 'Search Records', exact: true }).fill(key)
      await workbench.getByRole('button', { name: 'Search Records', exact: true }).click()
      await workbench.locator('aside').getByRole('button').filter({ hasText: key }).click()
      const form = workbench.getByTestId('document-header-form')
      await expect(form.getByLabel('Key', { exact: true })).toHaveText(key)
      if (archive) {
        await expect(workbench.getByText('Read Only', { exact: true })).toBeVisible()
        await expect(form.locator('input:enabled, select:enabled, textarea:enabled')).toHaveCount(0)
        await expect(workbench.getByRole('button', { name: /^(New record|Edit record|Delete record|Add line item|Post)$/ })).toHaveCount(0)
      } else {
        await expect(workbench.getByRole('button', { name: 'New record', exact: true })).toBeEnabled()
        await expect(workbench.getByRole('button', { name: 'Edit record', exact: true })).toBeEnabled()
        await expect(workbench.getByRole('button', { name: 'Post', exact: true })).toBeEnabled()
      }
      await assertLayout(page)
    }
  }
  await navigate(page, 'Retail', 'retail:branch')
  await expect(page.getByTestId('document-workbench').getByRole('button', { name: 'New record', exact: true })).toHaveCount(0)
  await assertLayout(page)
})

test('AI provider presets configure a real gateway connection to the local fixture', async ({ page }, testInfo) => {
  test.setTimeout(90_000)
  const headers = await login(page, 'platform')
  await navigate(page, 'PlatformAdmin:models')
  const preset = page.getByRole('combobox', { name: 'Provider Preset', exact: true })
  await expect(preset).toBeVisible({ timeout: 30_000 })
  await expect(preset).toContainText('DeepSeek')
  const options = await preset.locator('option').allTextContents()
  expect(options.join(' ')).toMatch(/DeepSeek/)
  expect(options.join(' ')).toMatch(/Anthropic/)
  expect(options.join(' ')).toMatch(/Gemini/)
  await preset.selectOption({ label: options.find((option) => option.includes('DeepSeek'))! })
  await expect(page.getByLabel('Base URL', { exact: true })).toHaveValue(/deepseek/)
  await preset.selectOption('')
  const name = `Playwright UI Provider ${Date.now()}`
  await page.getByLabel('Name', { exact: true }).fill(name)
  const mockPort = process.env.PLAYWRIGHT_MOCK_AI_PORT || '18081'
  const mockURL = process.env.PLAYWRIGHT_MOCK_AI_PROVIDER_URL || `http://${process.env.CI ? '127.0.0.1' : 'host.docker.internal'}:${mockPort}`
  await page.getByLabel('Base URL', { exact: true }).fill(`${mockURL.replace(/\/$/, '')}/v1`)
  await page.getByLabel('API Key', { exact: true }).fill('playwright-local-key')
  const response = page.waitForResponse((candidate) => candidate.request().method() === 'POST' && candidate.url().endsWith('/model-providers'))
  await page.getByRole('button', { name: 'Create Provider', exact: true }).click()
  const created = await response
  expect(created.status(), await created.text()).toBe(201)
  const provider = await created.json()
  try {
    await expect(page.getByLabel('API Key', { exact: true })).toHaveValue('')
    await expect(page.getByRole('button', { name: 'Test Provider', exact: true })).toBeDisabled()
    await page.getByLabel('Test Model', { exact: true }).fill('playwright-business-ai')
    const tested = page.waitForResponse((candidate) => candidate.request().method() === 'POST' && candidate.url().includes(`/model-providers/${provider.id}/test`))
    await page.getByRole('button', { name: 'Test Provider', exact: true }).click()
    const result = await tested
    expect(result.status(), await result.text()).toBe(200)
    await expect(page.getByText('Provider tested', { exact: true })).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1)
    await page.screenshot({ path: testInfo.outputPath('ai-provider-configuration.png'), fullPage: true })
  } finally {
    await page.request.patch(`${apiBase}/platform/admin/model-providers/${provider.id}`, { headers, data: { status: 'disabled' } })
  }
})
