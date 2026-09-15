import { createHash } from 'node:crypto'
import { expect, test, type Page } from '@playwright/test'

import type { DocumentImport } from '../../src/lib/document-import'

const apiBase = process.env.PLAYWRIGHT_API_URL || 'http://127.0.0.1:8080/api/v1'

async function login(page: Page, scope: 'platform' | 'tenant') {
  await page.goto('/')
  await page.getByTestId(`login-surface-${scope}`).click()
  await page.getByTestId('auth-email').fill(scope === 'platform'
    ? process.env.PLAYWRIGHT_PLATFORM_EMAIL || 'platform-admin@local.test'
    : process.env.PLAYWRIGHT_TENANT_EMAIL || 'demo@local.com')
  await page.getByTestId('auth-password').fill(scope === 'platform'
    ? process.env.PLAYWRIGHT_PLATFORM_PASSWORD || 'MetaOrgSaasDev!2026'
    : process.env.PLAYWRIGHT_TENANT_PASSWORD || 'MetaOrgSampleTenant!2026')
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('session-scope')).toHaveAttribute('data-scope', scope, { timeout: 30_000 })
  await page.getByRole('button', { name: 'EN', exact: true }).click()
  return page.evaluate((surface) => ({
    Authorization: `Bearer ${localStorage.getItem(`meta_org.${surface}.token`)}`,
    ...(surface === 'tenant' ? { 'X-Organization-ID': localStorage.getItem('meta_org.tenant.organization_id') || '' } : {}),
  }), scope)
}

test('a provisioned tenant imports one reviewed draft with original evidence over real HTTP', async ({ page }, testInfo) => {
  test.setTimeout(240_000)
  const platform = await login(page, 'platform')
  const setup = await page.request.post(`${apiBase}/platform/admin/sample-tenants/business-closure`, { headers: platform, data: {} })
  expect(setup.status(), await setup.text()).toBe(201)
  const tenant = await setup.json()
  expect(tenant.tenant_database_name).toMatch(/^meta_org_[a-f0-9]{4}$/)
  await page.evaluate(() => { localStorage.clear(); sessionStorage.clear() })
  const headers = await login(page, 'tenant')
  await expect.poll(async () => {
    const response = await page.request.get(`${apiBase}/ontology/types/purchase_order`, { headers })
    return response.status()
  }, { timeout: 120_000, intervals: [2000, 3000, 5000] }).toBe(200)
  const suffix = Date.now().toString()
  const supplier = `IMP-S-${suffix}`, item = `IMP-I-${suffix}`, warehouse = `IMP-W-${suffix}`
  for (const [table, key, data] of [
    ['MCRD', supplier, { CardName: 'Document import supplier', CardType: 'S', ValidFor: 'Y' }],
    ['MITM', item, { ItemName: 'Document import component', InvntItem: 'Y' }],
    ['MWHS', warehouse, { WhsName: 'Document import warehouse' }],
  ] as const) {
    const response = await page.request.post(`${apiBase}/erp/${table}`, { headers, data: { key, data } })
    expect(response.status(), await response.text()).toBe(201)
  }
  const organizationID = headers['X-Organization-ID']!
  await page.goto(`/tenant/${organizationID}/procurement`)
  await page.getByRole('button', { name: 'Import documents', exact: true }).click()
  const csv = Buffer.from(`external_number,partner,date,currency,item,warehouse,quantity,unit_price,tax_rate\nEXT-${suffix},${supplier},2026-09-15,CNY,${item},${warehouse},10,12.5,13\n`)
  await page.getByTestId('import-file-input').setInputFiles({ name: 'supplier-order.csv', mimeType: 'text/csv', buffer: csv })
  const recognition = page.waitForResponse((response) => response.request().method() === 'POST' && response.url().endsWith('/recognize'))
  await page.getByRole('button', { name: 'Upload and recognize', exact: true }).click()
  const recognizedResponse = await recognition
  expect(recognizedResponse.status(), await recognizedResponse.text()).toBe(200)
  const recognized = await recognizedResponse.json() as DocumentImport
  expect(recognized.status).toBe('needs_review')
  expect(recognized.recognition_method).toBe('structured_csv')
  const absent = await page.request.get(`${apiBase}/erp/MPOR/${recognized.draft.key}`, { headers })
  expect(absent.status()).toBe(404)

  const workbench = page.getByTestId('document-import-workspace')
  await expect(workbench.locator('[name="properties.partner"]')).toHaveValue(supplier)
  await expect(workbench.locator('pre')).toContainText(`EXT-${suffix}`)
  await workbench.locator('[name="properties.note"]').fill('Original checked by the human operator')
  const reviewResponse = page.waitForResponse((response) => response.request().method() === 'PATCH' && response.url().endsWith('/review'))
  await page.getByRole('button', { name: 'Save review', exact: true }).click()
  const review = await (await reviewResponse).json() as DocumentImport
  expect(review.version).toBeGreaterThan(recognized.version)
  const stale = await page.request.patch(`${apiBase}/document-imports/${review.id}/review`, {
    headers, data: { version: recognized.version, draft: review.draft },
  })
  expect(stale.status()).toBe(409)
  await page.getByRole('button', { name: 'Confirm document', exact: true }).click()
  const confirmation = page.getByRole('dialog', { name: 'Confirm import review', exact: true })
  await expect(confirmation.getByRole('button', { name: 'Confirm and create draft', exact: true })).toBeDisabled()
  await confirmation.getByRole('checkbox').check()
  const confirmedResponse = page.waitForResponse((response) => response.request().method() === 'POST' && response.url().endsWith('/confirm'))
  await confirmation.getByRole('button', { name: 'Confirm and create draft', exact: true }).click()
  const result = await confirmedResponse
  expect(result.status(), await result.text()).toBe(200)
  const confirmed = await result.json() as DocumentImport
  expect(confirmed.status).toBe('confirmed')
  const replay = await page.request.post(`${apiBase}/document-imports/${review.id}/confirm`, {
    headers, data: { version: review.version, draft: review.draft, confirmed: true },
  })
  expect(replay.status(), await replay.text()).toBe(200)
  expect((await replay.json()).confirmed_key).toBe(confirmed.confirmed_key)
  const query = await page.request.post(`${apiBase}/ontology/objects/purchase_order/query`, {
    headers, data: { filters: { key: confirmed.confirmed_key, total: 141.25 }, status: 'draft', sort: 'total', direction: 'asc' },
  })
  expect(query.status(), await query.text()).toBe(200)
  const objects = await query.json()
  expect(objects.total).toBe(1)
  expect(objects.objects[0].provenance.import_id).toBe(confirmed.id)
  expect(objects.objects[0].properties.posted).not.toBe('Y')
  expect(objects.objects[0].properties.approval_status).not.toBe('A')
  const originalURL = `${apiBase}/document-imports/${confirmed.id}/files/${confirmed.files[0].id}`
  const original = await page.request.get(originalURL, { headers })
  expect(original.status()).toBe(200)
  expect(createHash('sha256').update(await original.body()).digest('hex')).toBe(createHash('sha256').update(csv).digest('hex'))
  expect((await page.request.get(originalURL)).status()).toBe(401)

  await page.getByRole('button', { name: 'Open document', exact: true }).click()
  await expect(page.getByTestId('field-value-DocEntry')).toHaveText(confirmed.confirmed_key!)
  await page.getByRole('button', { name: 'Source files', exact: true }).click()
  await expect(page.getByTestId('document-import-workspace').locator('[name="properties.note"]')).toHaveValue('Original checked by the human operator')
  await page.getByRole('tab', { name: 'Review history', exact: true }).click()
  await expect(page.getByTestId('document-import-workspace')).toContainText('Document confirmed by operator')
  await page.screenshot({ path: testInfo.outputPath('verified-import-history.png'), fullPage: true })
})
