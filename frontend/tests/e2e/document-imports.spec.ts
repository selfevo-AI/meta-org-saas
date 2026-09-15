import { expect, test, type Page } from '@playwright/test'

import type { DocumentImport } from '../../src/lib/document-import'

const organizationID = 'e1131000-9200-4000-8000-000000000001'
const actorID = 'e1131000-9200-4000-8000-000000000002'
const importID = 'e1131000-9200-4000-8000-000000000003'
const fileID = 'e1131000-9200-4000-8000-000000000004'
const fieldMap: Record<string, string> = {
  key: 'DocEntry', partner: 'CardCode', date: 'DocDate', due_date: 'DocDueDate', currency: 'DocCur',
  total: 'DocTotal', tax: 'VatSum', status: 'DocStatus', approval_status: 'WddStatus', posted: 'Posted',
  external_number: 'NumAtCard', note: 'Comments',
}

async function fixture(page: Page, source?: { name: string; mediaType: string; bytes: Buffer }) {
  const base64 = await page.evaluate(() => {
    const canvas = document.createElement('canvas')
    canvas.width = 760
    canvas.height = 980
    const context = canvas.getContext('2d')!
    context.fillStyle = '#ffffff'
    context.fillRect(0, 0, 760, 980)
    context.fillStyle = '#18766e'
    context.fillRect(44, 50, 8, 70)
    context.fillStyle = '#1d292f'
    context.font = 'bold 29px Arial'
    context.fillText('NORTHWIND COMPONENTS', 70, 81)
    context.font = '18px Arial'
    context.fillText('Supplier purchase confirmation', 70, 111)
    context.font = 'bold 34px Arial'
    context.fillText('PURCHASE ORDER', 44, 190)
    context.font = '18px Arial'
    const lines = ['Reference: EXT-2026-0915', 'Date: 2026-09-15', 'Supplier: SUP-01', 'Currency: CNY', '',
      'Item             Qty         Price          Tax', 'ITEM-01          10          12.50          13%', '',
      'Warehouse: WHS-01', '', 'Net amount: 125.00', 'Tax: 16.25', 'Total: CNY 141.25']
    lines.forEach((line, index) => context.fillText(line, 44, 260 + index * 37))
    context.strokeStyle = '#cad1d5'
    context.beginPath(); context.moveTo(44, 420); context.lineTo(710, 420); context.stroke()
    return canvas.toDataURL('image/png').split(',')[1]
  })
  const original = source?.bytes ?? Buffer.from(base64, 'base64')
  const mediaType = source?.mediaType ?? 'image/png'
  const sourceName = source?.name ?? 'supplier-original.png'
  const state = {
    queries: [] as Record<string, unknown>[],
    confirmations: 0,
    erpWrites: [] as string[],
    failSave: false,
    recognitionMode: 'success' as 'success' | 'unavailable' | 'stalled',
    item: null as DocumentImport | null,
    records: new Map<string, Record<string, unknown>>(Array.from({ length: 12 }, (_, index) => {
      const key = `PO-${1001 + index}`
      return [key, { DocEntry: key, CardCode: 'SUP-01', DocDate: '2026-09-15', DocCur: 'CNY', DocTotal: 141.25 + index,
        DocStatus: 'O', WddStatus: index > 8 ? 'A' : 'N', NumAtCard: `EXT-${1001 + index}`, Posted: 'N' }]
    })),
  }
  await page.addInitScript(({ organizationID, actorID }) => {
    localStorage.setItem('meta_org.language.v1', 'en')
    localStorage.setItem('meta_org.tenant.token', 'import-ui-fixture')
    localStorage.setItem('meta_org.tenant.organization_id', organizationID)
    localStorage.setItem('meta_org.tenant.user', JSON.stringify({ id: actorID, type: 'human', onboarding_required: false,
      default_organization_id: organizationID, organizations: [{ id: organizationID, name: 'Import verification', is_owner: true, authority_tier: 'organization_admin' }] }))
    sessionStorage.setItem('meta_org.active_surface', 'tenant')
  }, { organizationID, actorID })
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const path = url.pathname.replace('/api/v1', '')
    const method = request.method()
    const multipart = request.headers()['content-type']?.includes('multipart/form-data')
    const body = request.postData() && !multipart ? request.postDataJSON() as Record<string, unknown> : {}
    const respond = (json: unknown, status = 200) => route.fulfill({ status, json })
    if (path === '/auth/me') return respond({ id: actorID, name: 'Import reviewer', email: 'import@example.test', account_status: 'active', onboarding_required: false,
      default_organization_id: organizationID, organizations: [{ id: organizationID, name: 'Import verification', is_owner: true, authority_tier: 'organization_admin' }] })
    if (path === '/modules') return respond(['procurement', 'inventory', 'finance', 'sales', 'project'].map((module_key) => ({ module_key, enabled_default: true, metadata: {} })))
    if (path.includes('/preferences/')) return respond({ value: { menu: 248, business: 280, status: 300 } })
    if (path.endsWith('/meta-org/overview')) return respond({ health: {}, projects: { by_status: {} }, agents: {}, cost: {}, risks: [], activity: [] })
    if (path === '/ontology/types/purchase_order') return respond({ key: 'purchase_order', table_code: 'MPOR', primary_key: 'DocEntry', importable: true, schema_version: 1,
      label: { zh: '采购订单', en: 'Purchase order' }, properties: Object.entries(fieldMap).map(([key, source_field]) => ({ key, source_field, data_type: key === 'total' ? 'decimal' : 'string', label: { en: key, zh: key } })),
      actions: ['submit', 'approve', 'receive'].map((key) => ({ key, requires_approval: true, label: { zh: key, en: key } })) })
    if (path.endsWith('/query')) {
      state.queries.push(body)
      const filtered = [...state.records].filter(([key, data]) => {
        const status = data.WddStatus === 'A' ? 'approved' : 'draft'
        return (!body.status || body.status === 'all' || body.status === status) && (!body.search || key.includes(String(body.search)))
      })
      const column = fieldMap[String(body.sort || 'key')]
      filtered.sort((a, b) => {
        const comparison = column === 'DocTotal' ? Number(a[1][column]) - Number(b[1][column]) : String(a[1][column]).localeCompare(String(b[1][column]))
        return body.direction === 'desc' ? -comparison : comparison
      })
      return respond({ total: filtered.length, objects: filtered.map(([key, data]) => ({ type: 'purchase_order', key, table_code: 'MPOR', title: key,
        properties: Object.fromEntries(Object.entries(fieldMap).map(([property, field]) => [property, data[field]])) })) })
    }
    if (path.endsWith('/links')) return respond({ links: [], truncated: false })
    if (path.endsWith('/history')) return respond({ executions: [] })
    if (path === '/erp/MCRD') return respond({ records: [{ key: 'SUP-01', data: { CardName: 'Northwind Components', CardType: 'S' } }, { key: 'SUP-02', data: { CardName: 'Contoso Parts', CardType: 'S' } }] })
    if (path === '/erp/MITM') return respond({ records: [{ key: 'ITEM-01', data: { ItemName: 'Standard component' } }] })
    if (path === '/erp/MWHS') return respond({ records: [{ key: 'WHS-01', data: { WhsName: 'Main warehouse' } }] })
    if (path.startsWith('/erp/')) {
      if (method !== 'GET') state.erpWrites.push(path)
      const key = path.split('/')[3]
      return respond(path.endsWith('/POR1') ? { records: [] } : { key, table_code: 'MPOR', data: state.records.get(key) })
    }
    if (path === '/document-imports' && method === 'GET') return respond({ imports: state.item ? [state.item] : [] })
    if (path === '/document-imports' && method === 'POST') {
      expect(request.headers()['x-organization-id']).toBe(organizationID)
      state.item = { id: importID, object_type: 'purchase_order', status: 'uploaded', version: 1, created_at: '2026-09-15T02:00:00Z',
        draft: { key: 'IMPORT-9001', properties: {}, lines: [] }, extraction: {}, recognition_method: '',
        files: [{ id: fileID, import_id: importID, name: sourceName, media_type: mediaType, byte_size: original.length, sha256: 'a'.repeat(64) }] }
      return respond(state.item)
    }
    if (path === `/document-imports/${importID}/recognize`) {
      if (state.recognitionMode !== 'success') {
        state.item = { ...state.item!, version: state.item!.version + 2,
          status: state.recognitionMode === 'unavailable' ? 'failed' : 'recognizing',
          error_code: state.recognitionMode === 'unavailable' ? 'recognition_unavailable' : undefined,
          recognition_started_at: new Date(Date.now() - 240_000).toISOString() }
        return respond(state.item)
      }
      const draft = { key: 'IMPORT-9001', properties: { external_number: 'EXT-2026-0915', partner: 'SUP-01', date: '2026-09-15', currency: 'CNY', total: '141.25', tax: '16.25' },
        lines: [{ item: 'ITEM-01', warehouse: 'WHS-01', quantity: '10', unit_price: '12.5', tax_rate: '13' }] }
      state.item = { ...state.item!, version: 3, status: 'needs_review', draft, recognition_method: 'ai_gateway',
        extraction: { draft: structuredClone(draft), evidence: [{ path: 'properties.partner', source_id: fileID, quote: 'Supplier: SUP-01', confidence: 0.72 }], warnings: [] } }
      return respond(state.item)
    }
    if (path === `/document-imports/${importID}/review`) {
      if (state.failSave) return respond({ error: 'conflict', code: 'document_import.conflict' }, 409)
      state.item = { ...state.item!, version: state.item!.version + 1, draft: body.draft as DocumentImport['draft'], reviewed_by: actorID }
      return respond(state.item)
    }
    if (path === `/document-imports/${importID}/confirm`) {
      expect(body.confirmed).toBe(true)
      expect(body.version).toBe(state.item!.version)
      state.confirmations++
      const draft = body.draft as DocumentImport['draft']
      state.item = { ...state.item!, status: 'confirmed', version: state.item!.version + 1, draft, confirmed_key: draft.key, reviewed_by: actorID }
      state.records.set(draft.key, { ...Object.fromEntries(Object.entries(draft.properties).map(([key, value]) => [fieldMap[key], value])),
        DocEntry: draft.key, DocStatus: 'O', WddStatus: 'N', Posted: 'N', provenance: { import_id: importID } })
      return respond(state.item)
    }
    if (path === `/document-imports/${importID}/files/${fileID}`) return route.fulfill({ status: 200, contentType: mediaType, body: original })
    if (path === `/document-imports/${importID}`) return respond(state.item)
    return respond([])
  })
  await page.goto(`/tenant/${organizationID}/procurement`)
  await expect(page.getByTestId('document-header-form')).toBeVisible()
  return { state, original }
}

async function beginUpload(page: Page, original: Buffer, name = 'supplier-original.png', mimeType = 'image/png') {
  await page.getByRole('button', { name: 'Import documents', exact: true }).click()
  await page.getByTestId('import-file-input').setInputFiles({ name, mimeType, buffer: original })
  await page.getByRole('button', { name: 'Upload and recognize', exact: true }).click()
}

async function upload(page: Page, original: Buffer) {
  await beginUpload(page, original)
  await expect(page.getByTestId('document-import-workspace').locator('[name="properties.partner"]')).toHaveValue('SUP-01')
}

async function assertLayout(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1)
  await expect(page.locator('body')).not.toContainText(/import\.(field|issue|status|error|tab)\./)
  const buttons = page.getByRole('dialog').locator('.ui-button')
  for (const button of await buttons.all()) {
    if (await button.isVisible()) expect(await button.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1)
  }
}

test('external originals require human review before creating one draft', async ({ page }, testInfo) => {
  test.setTimeout(90_000)
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  const { state, original } = await fixture(page)
  await page.screenshot({ path: testInfo.outputPath('document-form-en.png'), fullPage: true })
  await upload(page, original)
  const workbench = page.getByTestId('document-import-workspace')
  await expect(workbench.locator('[data-low-confidence="true"]')).toHaveCount(1)
  const image = workbench.locator('img')
  await expect.poll(() => image.evaluate((element: HTMLImageElement) => element.complete && element.naturalWidth)).toBe(760)
  expect(state.confirmations).toBe(0)
  expect(state.erpWrites).toEqual([])
  await assertLayout(page)
  await page.screenshot({ path: testInfo.outputPath('document-review-en.png'), fullPage: true })
  await workbench.locator('[name="properties.partner"]').fill('SUP-02')
  await page.getByRole('dialog', { name: /External Documents/ }).getByRole('button', { name: 'Confirm document', exact: true }).click()
  const confirmation = page.getByRole('dialog', { name: 'Confirm import review', exact: true })
  await expect(confirmation).toContainText('SUP-01')
  await expect(confirmation).toContainText('SUP-02')
  const confirm = confirmation.getByRole('button', { name: 'Confirm and create draft', exact: true })
  await expect(confirm).toBeDisabled()
  expect(state.confirmations).toBe(0)
  await confirmation.getByRole('checkbox').check()
  await confirm.click()
  await expect(confirmation).toHaveCount(0)
  expect(state.confirmations).toBe(1)
  expect(state.erpWrites).toEqual([])
  await page.getByRole('button', { name: 'Open document', exact: true }).click()
  await expect(page.getByTestId('field-value-DocEntry')).toHaveText('IMPORT-9001')
  await page.getByRole('button', { name: '中文', exact: true }).click()
  await page.getByRole('button', { name: '来源文件', exact: true }).click()
  await expect(page.getByTestId('document-import-workspace').locator('[name="properties.partner"]')).toBeDisabled()
  await assertLayout(page)
  await page.screenshot({ path: testInfo.outputPath('document-source-zh.png'), fullPage: true })
  expect(errors).toEqual([])
})

test('stale review keeps corrections and does not create a document', async ({ page }) => {
  const { state, original } = await fixture(page)
  await upload(page, original)
  const workbench = page.getByTestId('document-import-workspace')
  await workbench.locator('[name="properties.note"]').fill('Operator correction retained')
  state.failSave = true
  await page.getByRole('button', { name: 'Save review', exact: true }).click()
  await expect(workbench.getByRole('alert')).toContainText('Refresh and review')
  await expect(workbench.locator('[name="properties.note"]')).toHaveValue('Operator correction retained')
  expect(state.confirmations).toBe(0)
  state.failSave = false
  await page.getByRole('button', { name: 'Save review', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Save review', exact: true })).toBeDisabled()
  await expect(workbench.locator('[name="properties.note"]')).toHaveValue('Operator correction retained')
})

test('register sends status and semantic sort to the server', async ({ page }) => {
  const { state } = await fixture(page)
  await page.getByTestId('document-register-toggle').click()
  const table = page.getByTestId('document-register')
  await table.getByRole('button', { name: 'Total', exact: true }).click()
  await expect.poll(() => state.queries.at(-1)?.sort).toBe('total')
  await table.getByRole('button', { name: 'Total', exact: true }).click()
  await expect.poll(() => state.queries.at(-1)?.direction).toBe('desc')
  await page.getByTestId('document-workbench').getByRole('combobox').selectOption('approved')
  await expect.poll(() => state.queries.at(-1)?.status).toBe('approved')
  await expect(table.locator('tbody tr')).toHaveCount(3)
  await assertLayout(page)
})

test('PDF originals render locally with working pagination and zoom', async ({ page }, testInfo) => {
  const sourcePage = await page.context().newPage()
  await sourcePage.setContent('<html><head><style>body { font: 20px Arial; padding: 40px } h1 { color: #18766e } section { break-before: page }</style></head><body><h1>Supplier Invoice</h1><p>Reference EXT-2026-0915</p><p>Supplier SUP-01</p><p>ITEM-01 / 10 x CNY 12.50</p><h2>Total: CNY 141.25</h2><section><h1>Delivery details</h1><p>Warehouse WHS-01</p><p>10 standard components</p></section></body></html>')
  const bytes = await sourcePage.pdf({ format: 'A4', printBackground: true })
  await sourcePage.close()
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  await fixture(page, { name: 'supplier-original.pdf', mediaType: 'application/pdf', bytes })
  await beginUpload(page, bytes, 'supplier-original.pdf', 'application/pdf')
  const canvas = page.getByTestId('import-pdf-canvas')
  await expect(canvas).toHaveAttribute('data-rendered', '1', { timeout: 60_000 })
  expect(await canvas.evaluate((element: HTMLCanvasElement) => {
    const pixels = element.getContext('2d')!.getImageData(0, 0, element.width, element.height).data
    let ink = 0
    for (let index = 0; index < pixels.length; index += 32) {
      if (pixels[index + 3] > 0 && pixels[index] < 210 && pixels[index + 1] < 210) ink++
    }
    return ink
  })).toBeGreaterThan(30)
  await expect(page.getByRole('button', { name: 'Previous page', exact: true })).toBeDisabled()
  await page.getByRole('button', { name: 'Next page', exact: true }).click()
  await expect(canvas).toHaveAttribute('data-rendered', '2')
  await expect(page.getByRole('button', { name: 'Next page', exact: true })).toBeDisabled()
  const width = await canvas.evaluate((element: HTMLCanvasElement) => element.width)
  await page.getByRole('button', { name: 'Zoom in', exact: true }).click()
  await expect.poll(() => canvas.evaluate((element: HTMLCanvasElement) => element.width)).toBeGreaterThan(width)
  await expect(canvas).toHaveAttribute('data-rendered', '2')
  await assertLayout(page)
  await page.screenshot({ path: testInfo.outputPath('document-pdf-preview.png'), fullPage: true })
  expect(errors).toEqual([])
})

test('recognition failure retains originals and allows manual review', async ({ page }) => {
  const { state, original } = await fixture(page)
  state.recognitionMode = 'unavailable'
  await beginUpload(page, original)
  const workbench = page.getByTestId('document-import-workspace')
  await expect(workbench).toContainText('No recognition model is available')
  await expect(workbench.locator('[name="properties.partner"]')).toBeEnabled()
  await workbench.locator('[name="properties.partner"]').fill('SUP-01')
  await workbench.locator('[name="properties.note"]').fill('Checked manually against the original')
  await page.getByRole('button', { name: 'Save review', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Save review', exact: true })).toBeDisabled()
  await expect.poll(() => workbench.locator('img').evaluate((element: HTMLImageElement) => element.naturalWidth)).toBe(760)
  expect(state.item?.draft.properties.note).toBe('Checked manually against the original')
  expect(state.confirmations).toBe(0)
})

test('an expired recognition can be retried from the review interface', async ({ page }) => {
  const { state, original } = await fixture(page)
  state.recognitionMode = 'stalled'
  await beginUpload(page, original)
  const workbench = page.getByTestId('document-import-workspace')
  await expect(workbench).toContainText('Recognition timed out')
  await expect(workbench.locator('[name="properties.partner"]')).toBeDisabled()
  state.recognitionMode = 'success'
  await page.getByRole('button', { name: 'Recognize again', exact: true }).click()
  await expect(workbench.locator('[name="properties.partner"]')).toHaveValue('SUP-01')
  await expect(workbench.locator('[name="properties.partner"]')).toBeEnabled()
  expect(state.confirmations).toBe(0)
})
