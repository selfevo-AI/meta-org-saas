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
  const providerCatalogResponse = page.waitForResponse((response) => (
    response.request().method() === 'GET' && response.url().endsWith('/api/v1/model-providers')
  ))
  const modelCatalogResponse = page.waitForResponse((response) => (
    response.request().method() === 'GET' && response.url().endsWith('/api/v1/models')
  ))
  await page.getByTestId('domain-nav-Project').click()
  await page.getByTestId('erp-document-project').click()

  const workbench = page.getByTestId('business-ai-workbench')
  await expect(workbench).toBeVisible()
  const providers = await (await providerCatalogResponse).json() as Array<Record<string, unknown>>
  const models = await (await modelCatalogResponse).json() as Array<Record<string, unknown>>
  expect(providers.length).toBeGreaterThan(0)
  expect(models.length).toBeGreaterThan(0)
  for (const provider of providers) {
    for (const forbidden of ['base_url', 'masked_api_key', 'last_test_error', 'metadata', 'tags']) {
      expect(provider).not.toHaveProperty(forbidden)
    }
  }
  for (const model of models) {
    for (const forbidden of ['metadata', 'created_at', 'updated_at']) {
      expect(model).not.toHaveProperty(forbidden)
    }
  }
  const stages = [
    ['plan', /project\.(match_members|bind_workflow|estimate_cost)/],
    ['change', /project\.(update_status|bind_workflow|match_members)/],
    ['accept', /project\.(accept_deliverable|close_feedback|estimate_cost)/],
    ['learn', /evolution\.(create_knowledge|create_signal|propose_experiment)/],
    ['do', /project\.(create_deliverable|create_cost_entry|estimate_cost|update_status)/],
  ] as const
  for (const [stage, allowedTool] of stages) {
    await workbench.getByTestId(`business-ai-stage-${stage}`).click()
    const responsePromise = page.waitForResponse((response) => (
      response.request().method() === 'POST' && response.url().includes('/projects/') && response.url().endsWith('/ai-analyses')
    ))
    await workbench.getByTestId('business-ai-analyze').click()
    const response = await responsePromise
    expect(response.status()).toBe(201)
    const run = await response.json() as { id: string; project_id: string; stage: string; status: string; invocation_id: string; analysis: { summary: string } }
    expect(run.stage).toBe(stage)
    expect(run.status).toBe('completed')
    await expect(workbench.getByText(run.analysis.summary)).toBeVisible({ timeout: 30_000 })
    await expect(workbench.getByText(new RegExp(run.invocation_id))).toBeVisible()
    await expect(workbench.getByText(allowedTool)).toBeVisible()
    if (stage === 'plan') {
      const requestHeaders = response.request().headers()
      const apiRoot = `${new URL(response.url()).origin}/api/v1`
      const headers = {
        Authorization: requestHeaders.authorization,
        'X-Organization-ID': requestHeaders['x-organization-id'],
      }
      const projectResponse = await page.request.get(`${apiRoot}/projects/${run.project_id}`, { headers })
      expect(projectResponse.status()).toBe(200)
      const project = await projectResponse.json() as { description: string }
      const changedDescription = `${project.description || ''} [business-ai-stale-context-check]`
      const updateResponse = await page.request.patch(`${apiRoot}/projects/${run.project_id}`, {
        headers,
        data: { description: changedDescription },
      })
      expect(updateResponse.status()).toBe(200)
      const staleProposalResponse = await page.request.post(`${apiRoot}/projects/${run.project_id}/ai-analyses/${run.id}/submit-proposal`, {
        headers,
        data: {},
      })
      expect(staleProposalResponse.status()).toBe(409)
      expect(await staleProposalResponse.text()).toContain('project context changed')
      const restoreResponse = await page.request.patch(`${apiRoot}/projects/${run.project_id}`, {
        headers,
        data: { description: project.description || '' },
      })
      expect(restoreResponse.status()).toBe(200)
    }
  }
  const proposalResponse = page.waitForResponse((candidate) => (
    candidate.request().method() === 'POST' && candidate.url().endsWith('/submit-proposal')
  ))
  await workbench.getByTestId('business-ai-submit-proposal').click()
  expect((await proposalResponse).status()).toBe(202)
  await expect(workbench.getByTestId('business-ai-proposal-status')).toContainText(/Awaiting approval|等待审批/)
  const approvalResponse = page.waitForResponse((candidate) => (
    candidate.request().method() === 'POST' && candidate.url().includes('/tool-approvals/') && candidate.url().endsWith('/approve')
  ))
  await workbench.getByTestId('business-ai-approve-proposal').click()
  expect((await approvalResponse).status()).toBe(200)
  await expect(workbench.getByTestId('business-ai-proposal-status')).toContainText(/Executed|已执行/, { timeout: 30_000 })
})
