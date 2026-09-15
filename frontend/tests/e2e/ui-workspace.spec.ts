import { expect, test, type Page } from '@playwright/test'

const organizationID = 'f1371000-9200-4000-8000-000000000001'
const userID = 'f1371000-9200-4000-8000-000000000002'
const projectID = 'f1371000-9200-4000-8000-000000000003'
const projectRecord = { PrjCode: 'PRJ-01', Name: 'Warehouse rollout', Status: 'active', Description: 'Prepare the new warehouse team.' }
const tenantPath = `/tenant/${organizationID}`
type RecordData = Record<string, unknown>
type Write = { method: string; path: string; body: RecordData; organization: string | undefined }

async function mockWorkspace(page: Page, scope: 'tenant' | 'platform' = 'tenant') {
  const state = {
    writes: [] as Write[],
    failNextSave: false,
    users: [{ user_id: 'staff-01', name: 'Jane Liu', email: 'jane@example.test', roles: ['operator'], account_status: 'active' }],
    aiRun: {
      id: 'ai-run-01', stage: 'plan', status: 'completed', proposal_status: 'not_submitted', tool_approval_id: '', resolved_model: 'local-ui-model', cost_amount: 0, currency: 'CNY', input_tokens: 20, output_tokens: 40, proposal_result: {},
      analysis: { summary: 'Prepare the warehouse rollout plan', confidence: 0.9, findings: [], recommendations: [], risks: [], evidence_refs: [], proposal: { action: 'Assign the warehouse rollout team', tool_name: 'project.match_members', arguments: { project_id: projectID, strategy: 'Internal warehouse staff' }, requires_approval: true } },
    },
    records: new Map<string, RecordData>([
      ['PO-1001', { DocEntry: 'PO-1001', CardName: 'Northwind Components', CardCode: 'SUP-01', DocDate: '2026-09-14', DocDueDate: '2026-09-20', DocCur: 'CNY', DocTotal: 1446.4, VatSum: 166.4, DocStatus: 'O', WddStatus: 'N', Posted: 'N', Comments: 'Deliver to the main warehouse.' }],
      ['PO-1002', { DocEntry: 'PO-1002', CardName: 'Contoso Supplies', CardCode: 'SUP-02', DocDate: '2026-09-14', DocCur: 'CNY', DocTotal: 520, DocStatus: 'O', WddStatus: 'N', Posted: 'N' }],
      ['PO-LOCKED', { DocEntry: 'PO-LOCKED', CardName: 'Approved purchase', CardCode: 'SUP-01', DocDate: '2026-09-13', DocCur: 'CNY', DocTotal: 720, DocStatus: 'O', WddStatus: 'A', Posted: 'N' }],
    ]),
    lines: new Map<string, RecordData[]>([
      ['PO-1001', [{ key: '1', LineNum: 1, ItemCode: 'ITEM-01', Dscription: 'Standard component', Quantity: 10, Price: 128, TaxRate: 13, WhsCode: 'WHS-01' }]],
    ]),
    inbox: [{ id: 'approval-01', type: 'tool_approval', title: 'Review the supplier onboarding request', status: 'pending', priority: 'high', source: 'Procurement', created_at: '2026-09-14T02:00:00Z' }],
  }
  await page.addInitScript(({ organizationID, userID, scope }) => {
    localStorage.setItem('meta_org.language.v1', 'en')
    localStorage.setItem(`meta_org.${scope}.token`, 'ui-test-session')
    localStorage.setItem(`meta_org.${scope}.user`, JSON.stringify({
      id: userID, type: 'human', onboarding_required: false,
      ...(scope === 'platform' ? { platform_role: 'system_owner' } : {
        default_organization_id: organizationID,
        organizations: [{ id: organizationID, name: 'Review workspace', is_owner: true, authority_tier: 'owner' }],
      }),
    }))
    if (scope === 'tenant') localStorage.setItem('meta_org.tenant.organization_id', organizationID)
    sessionStorage.setItem('meta_org.active_surface', scope)
  }, { organizationID, userID, scope })

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request()
    const path = new URL(request.url()).pathname.replace('/api/v1', '')
    const method = request.method()
    const payload = request.postData() ? request.postDataJSON() as RecordData : {}
    const respond = (json: unknown, status = 200) => route.fulfill({ status, json })
    if (path === '/auth/me') return respond({
      id: userID, name: 'Review employee', email: 'ui@example.test', account_status: 'active', onboarding_required: false,
      ...(scope === 'platform' ? { platform_role: 'system_owner', organizations: [] } : {
        default_organization_id: organizationID,
        organizations: [{ id: organizationID, name: 'Review workspace', is_owner: true, authority_tier: 'owner' }],
      }),
    })
    if (path.endsWith('/meta-org/overview')) return respond({
      generated_at: '2026-09-14T02:00:00Z',
      health: { open_requirements: 2, active_projects: 3, active_agents: 2, pending_approvals: state.inbox.length, unexported_cost: 0, currency: 'CNY' },
      projects: { by_status: { active: 3, completed: 2 }, over_budget: 0 },
      agents: { active: 2, total: 4, by_risk_level: {} },
      cost: { today: 0, month_to_date: 0, unexported: 0, currency: 'CNY', by_provider: {} },
      risks: [], activity: [
        { id: 'event-1', type: 'project', title: 'Quarterly delivery reviewed', status: 'completed', created_at: '2026-09-14T01:30:00Z' },
        { id: 'event-2', type: 'ai_invocation', title: 'business_stage_ai model call', status: 'completed', created_at: '2026-09-14T01:00:00Z' },
      ],
    })
    if (path.endsWith('/meta-org/inbox')) return respond(state.inbox)
    if (path.includes('/preferences/')) return respond({ value: { menu: 248, business: 280, status: 300 } })
    if (path === '/modules') return respond(['project', 'procurement', 'inventory', 'sales', 'finance', 'organization'].map((module_key) => ({ module_key, display_name: module_key, category: 'business', enabled_default: true, license_scope: 'mit', metadata: {} })))
    if (path === '/model-providers') return respond([{ id: 'provider-01', provider_type: 'openai', name: 'Local UI fixture', status: 'active' }])
    if (path === '/models') return respond([{ id: 'model-01', provider_id: 'provider-01', model_key: 'local-ui-model', display_name: 'Local UI model', status: 'active' }])
    if (path === '/projects') return respond([{ id: projectID, master_key: projectRecord.PrjCode, name: projectRecord.Name }])
    if (path === `/projects/${projectID}/ai-analyses`) {
      if (method === 'GET') return respond([state.aiRun])
      state.writes.push({ method, path, body: payload, organization: request.headers()['x-organization-id'] })
      state.aiRun.id = 'ai-run-02'
      state.aiRun.analysis.summary = 'Updated plan using internal warehouse staff'
      return respond(state.aiRun, 201)
    }
    if (path.endsWith('/submit-proposal')) {
      state.writes.push({ method, path, body: payload, organization: request.headers()['x-organization-id'] })
      state.aiRun.proposal_status = 'approval_required'
      state.aiRun.tool_approval_id = 'ai-approval-01'
      return respond(state.aiRun, 202)
    }
    if (path === '/assistant/sessions') return respond({ id: 'assistant-session-01', status: 'active' }, 201)
    if (path === '/assistant/sessions/assistant-session-01/runs' || path === '/assistant/sessions/assistant-session-01/resume') {
      const event = path.endsWith('/resume') ? { delta: 'The warehouse team has been assigned.', done: true } : {
        delta: 'Please review the proposed warehouse team assignment.',
        step: { id: 'assistant-step-01', step_type: 'tool_approval', status: 'approval_required', summary: 'Assign the internal warehouse team', tool_approval_id: 'assistant-approval-01', data: {}, turn: 1, created_at: '2026-09-14T02:00:00Z' },
        done: true,
      }
      return route.fulfill({ status: 200, contentType: 'text/event-stream', body: `event: update\ndata: ${JSON.stringify(event)}\n\n` })
    }
    if (path === '/platform/admin/me/permissions') return respond({ permissions: { 'platform.read': true, 'platform.user.manage': true, 'platform.rbac.manage': true }, roles: ['owner'] })
    if (path === '/platform/admin/users' && method === 'GET') return respond(state.users)
    if (path === '/platform/admin/users/staff-01/disable') {
      state.writes.push({ method, path, body: payload, organization: request.headers()['x-organization-id'] })
      state.users[0].account_status = 'disabled'
      return respond(state.users[0])
    }
    if (path.includes('/tool-approvals/') && method === 'POST') {
      state.writes.push({ method, path, body: payload, organization: request.headers()['x-organization-id'] })
      state.inbox = []
      if (path.includes('/ai-approval-01/')) state.aiRun.proposal_status = path.endsWith('/reject') ? 'rejected' : 'completed'
      return respond({ approval: { id: path.split('/').at(-2), status: 'approved' }, execution: { status: 'completed' } })
    }
    if (path === '/auth/me/password' && method === 'POST') return respond({ error: 'invalid current password', code: 'authentication_required' }, 401)
    if (path.startsWith('/ontology/types/')) {
      const type = path.split('/').at(-1)
      return respond({ key: type, table_code: type === 'purchase_order' ? 'MPOR' : 'MPRJ', primary_key: type === 'project' ? 'PrjCode' : 'DocEntry',
        label: { en: 'Purchase order', zh: '采购订单' },
        properties: Object.keys(type === 'project' ? projectRecord : state.records.get('PO-1001')!).map((key) => ({ key, source_field: key, data_type: 'string', label: { en: key, zh: key } })),
        actions: ['submit', 'approve', 'receive'].map((key) => ({ key, label: { en: key, zh: key }, requires_approval: true })),
      })
    }
    if (path.endsWith('/query')) {
      if (path.includes('/project/')) return respond({ objects: [{ type: 'project', table_code: 'MPRJ', key: projectRecord.PrjCode, title: projectRecord.Name, properties: projectRecord }] })
      if (!path.includes('/purchase_order/')) return respond({ objects: [] })
      const search = String(payload.search ?? '').toLowerCase()
      return respond({ objects: [...state.records].filter(([key, data]) => (key + JSON.stringify(data)).toLowerCase().includes(search)).map(([key, properties]) => ({ type: 'purchase_order', table_code: 'MPOR', key, title: properties.CardName, properties })) })
    }
    if (path.endsWith('/links')) return respond({ links: [], truncated: false })
    if (path.endsWith('/history')) return respond({ executions: [] })
    if (path.includes('/actions/') && method === 'POST') {
      state.writes.push({ method, path, body: payload, organization: request.headers()['x-organization-id'] })
      const parts = path.split('/')
      const record = state.records.get(parts[4])
      if (record && parts.at(-1) === 'submit') record.DocStatus = 'S'
      if (record && parts.at(-1) === 'approve') record.WddStatus = 'A'
      return respond({ object: {}, execution: { status: 'succeeded' } })
    }
    const parts = path.split('/').filter(Boolean)
    if (parts[0] === 'erp' && parts[1] === 'MPRJ') return respond(parts[3] ? { records: [] } : { key: projectRecord.PrjCode, table_code: 'MPRJ', data: projectRecord })
    if (parts[0] === 'erp' && parts[1] === 'MPOR') {
      const key = parts[2]
      if (method === 'GET') {
        if (parts[3] === 'POR1') return respond({ records: (state.lines.get(key) ?? []).map((data) => ({ key: String(data.key), data })) })
        if (key) return respond({ key, table_code: 'MPOR', data: state.records.get(key) })
        return respond({ records: [...state.records].map(([key, data]) => ({ key, data })) })
      }
      state.writes.push({ method, path, body: payload, organization: request.headers()['x-organization-id'] })
      if (state.failNextSave) {
        state.failNextSave = false
        return respond({ error: 'forbidden', code: 'forbidden', request_id: 'ui-permission-check' }, 403)
      }
      if (parts[3] === 'POR1') {
        const lines = state.lines.get(key) ?? []
        const data = payload.data as RecordData
        if (method === 'POST') lines.push({ ...data, key: String(payload.key), LineNum: Number(payload.key) })
        if (method === 'PATCH') Object.assign(lines.find((line) => line.key === parts[4])!, data)
        if (method === 'DELETE') state.lines.set(key, lines.filter((line) => line.key !== parts[4]))
        else state.lines.set(key, lines)
        return respond({ key: payload.key ?? parts[4], data }, method === 'POST' ? 201 : 200)
      }
      if (method === 'POST') state.records.set(String(payload.key), { ...(payload.data as RecordData), DocEntry: payload.key, DocStatus: 'O', WddStatus: 'N', Posted: 'N' })
      if (method === 'PATCH') Object.assign(state.records.get(key)!, payload.data)
      if (method === 'DELETE') state.records.delete(key)
      return respond({ key: key ?? payload.key, data: state.records.get(key ?? String(payload.key)) }, method === 'POST' ? 201 : 200)
    }
    if (path === '/erp/MCRD') return respond({ records: [{ key: 'SUP-01', data: { CardName: 'Northwind Components', CardType: 'S' } }, { key: 'SUP-02', data: { CardName: 'Contoso Supplies', CardType: 'S' } }] })
    if (path === '/erp/MITM') return respond({ records: [{ key: 'ITEM-01', data: { ItemName: 'Standard component' } }] })
    if (path === '/erp/MWHS') return respond({ records: [{ key: 'WHS-01', data: { WhsName: 'Main warehouse' } }] })
    // Optional catalogs and assistant libraries are empty in this isolated UI fixture.
    return respond([])
  })
  return state
}

async function openDocument(page: Page) {
  const state = await mockWorkspace(page)
  await page.goto(`${tenantPath}/procurement`)
  await expect(page.getByTestId('field-value-DocEntry')).toHaveText('PO-1001')
  await expect(page.getByRole('button', { name: 'Edit record', exact: true })).toBeEnabled()
  return state
}

async function openNavigation(page: Page) {
  if ((page.viewportSize()?.width ?? 1440) < 1024) await page.getByTestId('mobile-menu-open').click()
}

async function assertLayout(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1)
  await expect(page.getByTestId('session-scope')).toBeVisible()
  await expect(page.locator('body')).not.toContainText(/ui\.(document|home|shell|search|unsaved|review)\./)
}

test('workspace uses clear controls in both languages and both themes without overflow', async ({ page }, testInfo) => {
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  await openDocument(page)
  await expect(page.locator('main')).toHaveClass(/theme-light/)
  await expect(page.getByRole('button', { name: 'New record', exact: true })).toBeVisible()
  await expect(page.getByTestId('document-header-form').locator('input:enabled')).toHaveCount(0)
  await assertLayout(page)
  await page.screenshot({ path: testInfo.outputPath('workspace-light.png'), fullPage: true })
  await page.getByRole('button', { name: '中文', exact: true }).click()
  await expect(page.getByRole('button', { name: '编辑单据', exact: true })).toBeVisible()
  await assertLayout(page)
  await page.screenshot({ path: testInfo.outputPath('reference-layout-zh.png'), fullPage: true })
  await page.getByRole('button', { name: 'EN', exact: true }).click()
  await page.getByRole('button', { name: /dark theme/i }).click()
  await expect(page.locator('main')).not.toHaveClass(/theme-light/)
  await assertLayout(page)
  const contrast = await page.getByRole('button', { name: 'New record', exact: true }).evaluate((element) => {
    const style = getComputedStyle(element)
    const luminance = (color: string) => {
      const channels = (color.match(/[\d.]+/g) ?? []).slice(0, 3).map(Number).map((value) => {
        const linear = value / 255
        return linear <= 0.04045 ? linear / 12.92 : ((linear + 0.055) / 1.055) ** 2.4
      })
      return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2]
    }
    const a = luminance(style.color), b = luminance(style.backgroundColor)
    return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05)
  })
  expect(contrast).toBeGreaterThanOrEqual(4.5)
  await page.screenshot({ path: testInfo.outputPath('workspace-dark.png'), fullPage: true })
  await page.reload()
  await expect(page.getByTestId('document-workbench')).toBeVisible()
  await expect(page.locator('main')).not.toHaveClass(/theme-light/)
  expect(errors).toEqual([])
})

test('document form leads with fields and keeps totals below the line grid', async ({ page }, testInfo) => {
  const state = await openDocument(page)
  const form = page.getByTestId('document-header-form')
  const summary = page.getByTestId('document-summary')
  await expect(page.getByTestId('document-register')).not.toBeVisible()
  await expect(summary.getByTestId('field-value-DocTotal')).toHaveText('1,446.40')
  await expect(summary.getByTestId('field-value-WddStatus')).toBeVisible()
  await expect(form.getByTestId('field-value-DocTotal')).toHaveCount(0)
  const positions = await page.evaluate(() => {
    const lines = document.querySelector('.document-tab-panel')!.getBoundingClientRect()
    return {
      header: document.querySelector('[data-testid="document-header-form"]')!.getBoundingClientRect().bottom,
      linesTop: lines.top,
      linesBottom: lines.bottom,
      footer: document.querySelector('[data-testid="document-summary"]')!.getBoundingClientRect().top,
    }
  })
  expect(positions.header).toBeLessThanOrEqual(positions.linesTop)
  expect(positions.linesBottom).toBeLessThanOrEqual(positions.footer)
  await page.getByRole('button', { name: 'Edit record', exact: true }).click()
  for (const field of await form.locator('.document-fields-compact .ui-field').all()) {
    const bounds = await field.evaluate((element) => {
      const label = element.querySelector('.ui-field-label')!.getBoundingClientRect()
      const value = element.querySelector('input, select, output')!.getBoundingClientRect()
      return { labelRight: label.right, valueLeft: value.left, valueRight: value.right, fieldRight: element.getBoundingClientRect().right }
    })
    expect(bounds.labelRight).toBeLessThanOrEqual(bounds.valueLeft)
    expect(bounds.valueRight).toBeLessThanOrEqual(bounds.fieldRight + 1)
  }
  await assertLayout(page)
  await page.screenshot({ path: testInfo.outputPath('reference-layout-edit.png'), fullPage: true })
  expect(state.writes).toHaveLength(0)
})

test('record navigation supports previous, next, and a searchable register', async ({ page }) => {
  const state = await openDocument(page)
  const previous = page.getByRole('button', { name: 'Previous record', exact: true })
  const next = page.getByRole('button', { name: 'Next record', exact: true })
  await expect(previous).toBeDisabled()
  await next.click()
  await expect(page.getByTestId('field-value-DocEntry')).toHaveText('PO-1002')
  await previous.click()
  await expect(page.getByTestId('field-value-DocEntry')).toHaveText('PO-1001')
  await page.getByTestId('document-register-toggle').click()
  await expect(page.getByTestId('document-register')).toBeVisible()
  await page.locator('.document-search input').fill('PO-1002')
  await page.locator('.document-search').getByRole('button').click()
  await expect(page.getByTestId('document-register').locator('tbody tr')).toHaveCount(1)
  await page.locator('[data-record-key="PO-1002"]').click()
  await expect(page.getByTestId('field-value-DocEntry')).toHaveText('PO-1002')
  await expect(page.getByTestId('document-register')).not.toBeVisible()
  await expect(previous).toBeDisabled()
  await expect(next).toBeDisabled()
  await page.getByTestId('document-register-toggle').click()
  await page.locator('.document-search input').fill('NO-MATCH')
  await page.locator('.document-search').getByRole('button').click()
  await expect(page.getByTestId('document-register').locator('tbody tr')).toHaveCount(0)
  await expect(page.getByTestId('document-register-toggle')).toBeDisabled()
  await expect(previous).toBeDisabled()
  await expect(next).toBeDisabled()
  await expect(page.getByRole('button', { name: 'New record', exact: true })).toBeEnabled()
  await page.locator('.document-search input').fill('')
  await page.locator('.document-search').getByRole('button').click()
  await expect(page.getByTestId('document-register').locator('tbody tr')).toHaveCount(3)
  await page.locator('[data-record-key="PO-1001"]').click()
  await expect(page.getByTestId('field-value-DocEntry')).toHaveText('PO-1001')
  expect(state.writes).toHaveLength(0)
})

test('menu groups fold independently and filtering reveals document destinations', async ({ page }, testInfo) => {
  const state = await openDocument(page)
  await openNavigation(page)
  const group = page.getByTestId('navigation-group-supplyChain')
  const heading = group.getByRole('button', { name: 'Supply Chain', exact: true })
  await expect(heading).toHaveAttribute('aria-expanded', 'true')
  await heading.click()
  await expect(group.getByTestId('domain-nav-Procurement')).not.toBeVisible()
  await expect(page).toHaveURL(/\/procurement$/)
  await page.getByRole('textbox', { name: 'Filter navigation', exact: true }).fill('purchase')
  await expect(group.getByTestId('domain-nav-Procurement')).toBeVisible()
  await expect(group.locator('.sidebar-submenu button').first()).toBeVisible()
  await page.getByRole('textbox', { name: 'Filter navigation', exact: true }).fill('')
  await expect(heading).toHaveAttribute('aria-expanded', 'false')
  await heading.click()
  await page.screenshot({ path: testInfo.outputPath('reference-layout-menu.png'), fullPage: false })
  expect(state.writes).toHaveLength(0)
})

test('record edits require a review and send only the changed writable fields', async ({ page }) => {
  const state = await openDocument(page)
  await page.getByRole('button', { name: 'Edit record', exact: true }).click()
  const form = page.getByTestId('document-header-form')
  await form.locator('[name="Comments"]').fill('Confirm delivery with the warehouse team.')
  await form.locator('[name="DocDueDate"]').fill('2026-09-25')
  await form.getByRole('button', { name: 'Save changes', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Review your changes', exact: true })
  await expect(dialog).toContainText('PO-1001')
  await expect(dialog).toContainText('Deliver to the main warehouse.')
  await expect(dialog).toContainText('Confirm delivery with the warehouse team.')
  await expect(dialog).toContainText('2026-09-20')
  expect(state.writes).toHaveLength(0)
  await expect(dialog.getByRole('button', { name: 'Keep editing', exact: true })).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(form.locator('[name="Comments"]')).toHaveValue('Confirm delivery with the warehouse team.')
  await form.getByRole('button', { name: 'Save changes', exact: true }).click()
  await dialog.getByRole('button', { name: 'Confirm save', exact: true }).click()
  await expect(page.getByTestId('field-value-Comments')).toHaveText('Confirm delivery with the warehouse team.')
  expect(state.writes).toEqual([{ method: 'PATCH', path: '/erp/MPOR/PO-1001', body: { data: { DocDueDate: '2026-09-25', Comments: 'Confirm delivery with the warehouse team.' } }, organization: organizationID }])
  await expect(dialog).toHaveCount(0)
})

test('switching records or modules preserves edits until the employee discards them', async ({ page }) => {
  const state = await openDocument(page)
  await page.getByRole('button', { name: 'Edit record', exact: true }).click()
  const remarks = page.getByTestId('document-header-form').locator('[name="Comments"]')
  await remarks.fill('An unsaved delivery note')
  await page.getByRole('button', { name: 'Next record', exact: true }).click()
  const guard = page.getByRole('dialog', { name: 'You have unsaved changes', exact: true })
  await guard.getByRole('button', { name: 'Keep editing', exact: true }).click()
  await expect(remarks).toHaveValue('An unsaved delivery note')
  await openNavigation(page)
  await page.getByTestId('domain-nav-Project').click()
  await expect(guard).toBeVisible()
  await guard.getByRole('button', { name: 'Keep editing', exact: true }).click()
  await expect(page).toHaveURL(/\/procurement$/)
  await expect(remarks).toHaveValue('An unsaved delivery note')
  await page.getByRole('button', { name: 'Next record', exact: true }).click()
  await guard.getByRole('button', { name: 'Discard changes', exact: true }).click()
  await expect(page.getByTestId('field-value-DocEntry')).toHaveText('PO-1002')
  expect(state.writes).toHaveLength(0)
})

test('line editing has context, a change review, and restores focus when closed', async ({ page }) => {
  const state = await openDocument(page)
  const edit = page.getByRole('button', { name: 'Edit line item 1', exact: true })
  await edit.click()
  const dialog = page.getByRole('dialog', { name: 'Edit line item', exact: true })
  await expect(dialog).toContainText('PO-1001')
  await dialog.locator('[name="Quantity"]').fill('12')
  await dialog.getByRole('button', { name: 'Save Line', exact: true }).click()
  const review = page.getByRole('dialog', { name: 'Review your changes', exact: true })
  await expect(review).toContainText('Line 1')
  await expect(review).toContainText('10')
  await expect(review).toContainText('12')
  expect(state.writes).toHaveLength(0)
  await review.getByRole('button', { name: 'Confirm save', exact: true }).click()
  await expect(dialog).toHaveCount(0)
  await expect(page.getByRole('cell', { name: '12', exact: true })).toBeVisible()
  expect(state.writes[0]).toMatchObject({ method: 'PATCH', path: '/erp/MPOR/PO-1001/POR1/1', body: { data: { Quantity: 12 } } })
  expect(await page.evaluate(() => document.body.style.overflow)).not.toBe('hidden')
  await edit.click()
  await page.keyboard.press('Escape')
  await expect(dialog).toHaveCount(0)
  await expect(edit).toBeFocused()
})

test('new records are reviewed before creation and cancelled deletion makes no write', async ({ page }) => {
  const state = await openDocument(page)
  await page.getByRole('button', { name: 'New record', exact: true }).click()
  const form = page.getByTestId('document-header-form')
  await form.locator('[name="DocEntry"]').fill('PO-NEW')
  await form.locator('[name="CardCode"]').fill('SUP-01')
  await form.getByRole('button', { name: 'Create', exact: true }).click()
  const review = page.getByRole('dialog', { name: 'Review the new record', exact: true })
  await expect(review).toContainText('PO-NEW')
  expect(state.writes).toHaveLength(0)
  await review.getByRole('button', { name: 'Confirm create', exact: true }).click()
  await expect(page.getByTestId('field-value-DocEntry')).toHaveText('PO-NEW')
  await page.getByTestId('document-more-actions').click()
  await page.getByRole('menuitem', { name: 'Delete record', exact: true }).click()
  const deletion = page.getByRole('dialog', { name: 'Delete this record?', exact: true })
  await expect(deletion).toContainText('PO-NEW')
  await expect(deletion.getByRole('button', { name: 'Cancel', exact: true })).toBeFocused()
  await deletion.getByRole('button', { name: 'Cancel', exact: true }).click()
  expect(state.writes.filter((write) => write.method === 'DELETE')).toHaveLength(0)
  expect(state.records.has('PO-NEW')).toBe(true)
  await page.getByTestId('document-more-actions').click()
  await page.getByRole('menuitem', { name: 'Delete record', exact: true }).click()
  await deletion.getByRole('button', { name: 'Confirm Deletion', exact: true }).click()
  await expect(deletion).toHaveCount(0)
  expect(state.writes.filter((write) => write.method === 'DELETE')).toMatchObject([{ path: '/erp/MPOR/PO-NEW' }])
  expect(state.records.has('PO-NEW')).toBe(false)
})

test('locked records cannot be edited or deleted but retain available workflow actions', async ({ page }) => {
  const state = await openDocument(page)
  await page.getByTestId('document-register-toggle').click()
  await page.locator('[data-record-key="PO-LOCKED"]').click()
  await expect(page.getByTestId('field-value-DocEntry')).toHaveText('PO-LOCKED')
  await expect(page.getByRole('button', { name: 'Edit record', exact: true })).toBeDisabled()
  await expect(page.getByRole('button', { name: 'Add line item', exact: true })).toBeDisabled()
  await page.getByTestId('document-more-actions').click()
  await expect(page.getByRole('menuitem', { name: 'Delete record', exact: true })).toBeDisabled()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('button', { name: 'Receive', exact: true })).toBeEnabled()
  await page.getByRole('button', { name: 'Receive', exact: true }).click()
  const action = page.getByRole('dialog', { name: 'Receive', exact: true })
  await expect(action).toContainText('PO-LOCKED')
  await action.getByRole('button', { name: 'Cancel', exact: true }).click()
  expect(state.writes).toHaveLength(0)
})

test('failed saves keep the review and input intact for correction or retry', async ({ page }) => {
  const state = await openDocument(page)
  state.failNextSave = true
  await page.getByRole('button', { name: 'Edit record', exact: true }).click()
  await page.getByTestId('document-header-form').locator('[name="Comments"]').fill('Retain this change after failure')
  await page.getByRole('button', { name: 'Save changes', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Review your changes', exact: true })
  await dialog.getByRole('button', { name: 'Confirm save', exact: true }).click()
  await expect(dialog.getByRole('alert')).toContainText(/permission/i)
  await expect(dialog).toContainText('Retain this change after failure')
  expect(state.records.get('PO-1001')?.Comments).toBe('Deliver to the main warehouse.')
  await dialog.getByRole('button', { name: 'Confirm save', exact: true }).click()
  await expect(page.getByTestId('field-value-Comments')).toHaveText('Retain this change after failure')
  expect(state.writes).toHaveLength(2)
})

test('quick navigation supports keyboard search and employee review notes reach the API', async ({ page }) => {
  const state = await mockWorkspace(page)
  await page.goto(`${tenantPath}/overview`)
  await expect(page.getByTestId('employee-home')).toBeVisible()
  await expect(page.locator('.home-metric').filter({ hasText: 'Open requirements' }).locator('strong')).toHaveText('2')
  await expect(page.locator('.home-activity').getByRole('listitem').filter({ hasText: 'Project AI analysis' })).toBeVisible()
  await expect(page.locator('body')).not.toContainText('business_stage_ai')
  await page.getByRole('button', { name: 'Review request', exact: true }).click()
  const review = page.getByRole('dialog', { name: 'Review request', exact: true })
  await expect(review).toContainText('Review the supplier onboarding request')
  await review.getByLabel('Review note', { exact: true }).fill('Verified the supplier details with purchasing.')
  expect(state.writes).toHaveLength(0)
  await review.getByRole('button', { name: 'Approve request', exact: true }).click()
  await expect(review).toHaveCount(0)
  expect(state.writes[0]).toMatchObject({ path: '/tool-approvals/approval-01/approve', body: { reason: 'Verified the supplier details with purchasing.' }, organization: organizationID })
  await page.keyboard.press('Control+k')
  const search = page.getByRole('dialog', { name: 'Quick navigation', exact: true })
  await search.getByRole('combobox').fill('purchase order')
  await expect(search.getByRole('option').first()).toContainText(/purchase order/i)
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(/\/procurement$/)
  await expect(page.getByTestId('document-workbench')).toBeVisible()
  await assertLayout(page)
})

test('platform navigation contains platform shortcuts and the account menu works with a keyboard', async ({ page }) => {
  await mockWorkspace(page, 'platform')
  await page.goto('/platform/overview')
  await expect(page.getByTestId('employee-home')).toBeVisible()
  await expect(page.getByTestId('quick-access-PlatformAdmin:saas')).toBeVisible()
  await expect(page.getByTestId('quick-access-Procurement')).toHaveCount(0)
  await page.getByTestId('account-menu').focus()
  await page.keyboard.press('ArrowDown')
  const password = page.getByRole('menuitem', { name: 'Change password', exact: true })
  await expect(password).toBeFocused()
  await page.keyboard.press('Enter')
  const dialog = page.getByRole('dialog', { name: 'Change password', exact: true })
  await expect(dialog).toBeVisible()
  await expect(dialog.getByLabel('Current password', { exact: true })).toBeFocused()
  await dialog.getByLabel('Current password', { exact: true }).fill('incorrect-password')
  await dialog.getByLabel('New password', { exact: true }).fill('LocalUiReview!2026')
  await dialog.getByLabel('Confirm new password', { exact: true }).fill('LocalUiReview!2026')
  await dialog.getByRole('button', { name: 'Save', exact: true }).click()
  await expect(dialog.getByRole('alert')).toContainText('The current password is incorrect')
  await expect(dialog.getByLabel('New password', { exact: true })).toHaveValue('LocalUiReview!2026')
  await page.keyboard.press('Escape')
  await expect(dialog).toHaveCount(0)
  await assertLayout(page)
})

test('AI proposals can be revised and require a reviewed decision before execution', async ({ page }) => {
  const state = await mockWorkspace(page)
  await page.goto(`${tenantPath}/projects`)
  const workbench = page.getByTestId('business-ai-workbench')
  await expect(workbench.getByText(state.aiRun.analysis.summary, { exact: true })).toBeVisible()
  await workbench.getByTestId('business-ai-submit-proposal').click()
  const review = page.getByRole('dialog', { name: 'Review the proposed action', exact: true })
  await expect(review).toContainText(projectRecord.Name)
  await expect(review).toContainText('Internal warehouse staff')
  expect(state.writes).toHaveLength(0)
  await review.getByRole('button', { name: 'Adjust the request', exact: true }).click()
  await expect(workbench.getByLabel('Analysis focus', { exact: true })).toBeFocused()
  await workbench.getByLabel('Analysis focus', { exact: true }).fill('Use our internal warehouse staff only.')
  await expect(workbench.getByTestId('business-ai-submit-proposal')).toBeDisabled()
  await workbench.getByTestId('business-ai-analyze').click()
  await expect(workbench.getByText('Updated plan using internal warehouse staff', { exact: true })).toBeVisible()
  expect(state.writes[0].body).toMatchObject({ focus: 'Use our internal warehouse staff only.' })
  await workbench.getByTestId('business-ai-submit-proposal').click()
  await review.getByRole('button', { name: 'Confirm', exact: true }).click()
  await expect(workbench.getByTestId('business-ai-proposal-status')).toHaveText('Awaiting approval')
  expect(state.writes[1].path).toBe(`/projects/${projectID}/ai-analyses/ai-run-02/submit-proposal`)
  await workbench.getByRole('button', { name: 'Reject proposal', exact: true }).click()
  await review.getByRole('button', { name: 'Cancel', exact: true }).click()
  expect(state.writes).toHaveLength(2)
  await workbench.getByTestId('business-ai-approve-proposal').click()
  await review.getByLabel('Review note', { exact: true }).fill('Checked staffing and delivery responsibilities.')
  await review.getByRole('button', { name: 'Confirm', exact: true }).click()
  await expect(workbench.getByTestId('business-ai-proposal-status')).toHaveText('Executed')
  expect(state.writes[2]).toMatchObject({ path: '/tool-approvals/ai-approval-01/approve', body: { reason: 'Checked staffing and delivery responsibilities.' } })
})

test('assistant approval preserves the drawer and releases nested dialogs after review', async ({ page }) => {
  const state = await openDocument(page)
  await page.getByRole('button', { name: 'AI Assistant', exact: true }).click()
  const drawer = page.getByRole('dialog', { name: 'Your AI assistant', exact: true })
  await drawer.locator('textarea').fill('Prepare the warehouse team assignment')
  await drawer.getByRole('button', { name: 'Send', exact: true }).click()
  await drawer.getByRole('button', { name: 'Approve and continue', exact: true }).click()
  const review = page.getByRole('dialog', { name: 'Review the proposed action', exact: true })
  await expect(review).toContainText('Assign the internal warehouse team')
  await review.getByLabel('Review note', { exact: true }).fill('The staff list is correct.')
  expect(state.writes).toHaveLength(0)
  await review.getByRole('button', { name: 'Approve request', exact: true }).click()
  await expect(review).toHaveCount(0)
  await expect(drawer.getByText('The warehouse team has been assigned.', { exact: true })).toBeVisible()
  expect(state.writes[0]).toMatchObject({ path: '/tool-approvals/assistant-approval-01/approve', body: { reason: 'The staff list is correct.' } })
  expect(await page.evaluate(() => document.body.style.overflow)).toBe('hidden')
  await page.keyboard.press('Escape')
  await expect(drawer).toHaveCount(0)
  expect(await page.evaluate(() => document.body.style.overflow)).not.toBe('hidden')
  await assertLayout(page)
})

test('platform user actions identify the person and only write after explicit confirmation', async ({ page }) => {
  const state = await mockWorkspace(page, 'platform')
  await page.goto('/platform/users')
  const workspace = page.getByTestId('system-admin-workspace')
  await workspace.getByRole('textbox', { name: 'Search people by name or email', exact: true }).fill('jane@example.test')
  const row = workspace.getByRole('row').filter({ hasText: 'jane@example.test' })
  await row.getByRole('button', { name: 'Actions', exact: true }).click()
  const reset = page.getByRole('menuitem', { name: 'Reset password', exact: true })
  await expect(reset).toBeVisible()
  await reset.click()
  const resetReview = page.getByRole('dialog', { name: 'Reset password', exact: true })
  await expect(resetReview).toContainText('Jane Liu')
  await expect(resetReview).toContainText('jane@example.test')
  await resetReview.getByRole('button', { name: 'Cancel', exact: true }).click()
  expect(state.writes).toHaveLength(0)
  await row.getByRole('button', { name: 'Actions', exact: true }).click()
  await page.getByRole('menuitem', { name: 'Disable', exact: true }).click()
  const disableReview = page.getByRole('dialog', { name: 'Disable', exact: true })
  await expect(disableReview.getByRole('button', { name: 'Cancel', exact: true })).toBeFocused()
  await disableReview.getByRole('button', { name: 'Confirm action', exact: true }).click()
  await expect(disableReview).toHaveCount(0)
  await expect(row).toContainText('Disabled')
  expect(state.writes).toMatchObject([{ path: '/platform/admin/users/staff-01/disable' }])
  await workspace.locator('.admin-related-pages > summary').click()
  await workspace.locator('.admin-related-pages').getByRole('button', { name: /^Permissions/ }).click()
  await expect(page).toHaveURL(/\/platform\/permissions$/)
  await assertLayout(page)
})
