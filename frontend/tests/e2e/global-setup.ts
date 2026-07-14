import { createServer, type Server } from 'node:http'
import type { FullConfig } from '@playwright/test'

type LoginResponse = {
  token: string
}

type ModelProvider = {
  id: string
  name: string
  provider_type: string
}

type AIModel = {
  id: string
  provider_id: string
  model_key: string
}

type Project = {
  id: string
}

const mockPort = Number(process.env.PLAYWRIGHT_MOCK_AI_PORT || 18081)
const mockProviderName = 'Playwright Business AI'
const mockModelKey = 'playwright-business-ai'

function analysisForStage(stage: string) {
  const proposals: Record<string, { action: string; tool_name: string; arguments: Record<string, unknown>; requires_approval: boolean }> = {
    plan: { action: 'Estimate the project cost', tool_name: 'project.estimate_cost', arguments: {}, requires_approval: true },
    do: { action: 'Estimate the delivery cost from current evidence', tool_name: 'project.estimate_cost', arguments: {}, requires_approval: true },
    change: { action: 'Move the project back to active delivery', tool_name: 'project.update_status', arguments: { status: 'active', note: 'Playwright change analysis' }, requires_approval: true },
    accept: { action: 'Re-estimate cost before acceptance', tool_name: 'project.estimate_cost', arguments: {}, requires_approval: true },
    learn: {
      action: 'Capture delivery learning',
      tool_name: 'evolution.create_knowledge',
      arguments: { title: 'Playwright delivery learning', content: 'Verified delivery evidence should be retained for the next planning cycle.', tags: ['playwright'] },
      requires_approval: true,
    },
  }
  return {
    summary: `Playwright ${stage} analysis completed from verified project context.`,
    findings: [{ title: 'Verified context available', evidence: '$.project', impact: 'The next action can be proposed without invented facts.' }],
    recommendations: [{ title: 'Continue through governed execution', rationale: 'The proposal remains approval-gated.', priority: 'high' }],
    risks: [{ title: 'Unreviewed automation', probability: 'low', impact: 'A direct write could bypass human judgment.', mitigation: 'Require tool approval before execution.' }],
    proposal: proposals[stage] || proposals.plan,
    confidence: 0.91,
    evidence_refs: ['$.project', '$.stage'],
  }
}

async function startMockAI(): Promise<Server> {
  const server = createServer((request, response) => {
    if (request.method !== 'POST' || request.url !== '/v1/chat/completions') {
      response.writeHead(404).end()
      return
    }
    const chunks: Buffer[] = []
    request.on('data', (chunk) => chunks.push(Buffer.from(chunk)))
    request.on('end', () => {
      const payload = JSON.parse(Buffer.concat(chunks).toString('utf8')) as { model?: string; messages?: Array<{ content?: string }> }
      const prompt = payload.messages?.map((message) => message.content || '').join('\n') || ''
      const stage = prompt.match(/stage "(plan|do|change|accept|learn)"/)?.[1] || 'plan'
      response.writeHead(200, { 'Content-Type': 'application/json' })
      response.end(JSON.stringify({
        id: `playwright-${stage}-${Date.now()}`,
        object: 'chat.completion',
        created: Math.floor(Date.now() / 1000),
        model: payload.model || mockModelKey,
        choices: [{ index: 0, message: { role: 'assistant', content: JSON.stringify(analysisForStage(stage)) }, finish_reason: 'stop' }],
        usage: { prompt_tokens: 120, completion_tokens: 90, total_tokens: 210 },
      }))
    })
  })
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject)
    server.listen(mockPort, '0.0.0.0', resolve)
  })
  return server
}

async function apiRequest<T>(apiBase: string, path: string, token: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`${apiBase}${path}`, {
    ...init,
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json', ...init.headers },
  })
  if (!response.ok) {
    throw new Error(`${init.method || 'GET'} ${path} failed: ${response.status} ${await response.text()}`)
  }
  return response.json() as Promise<T>
}

export default async function globalSetup(_config: FullConfig) {
  const apiBase = process.env.PLAYWRIGHT_API_URL || 'http://127.0.0.1:8080/api/v1'
  const email = process.env.PLAYWRIGHT_PLATFORM_EMAIL || 'platform-admin@local.test'
  const password = process.env.PLAYWRIGHT_PLATFORM_PASSWORD || 'MetaOrgSaasDev!2026'
  const defaultMockProviderHost = process.env.CI ? '127.0.0.1' : 'host.docker.internal'
  const mockProviderURL = process.env.PLAYWRIGHT_MOCK_AI_PROVIDER_URL || `http://${defaultMockProviderHost}:${mockPort}`
  const managementBase = '/platform/admin'
  const mockServer = await startMockAI()

  const loginResponse = await fetch(`${apiBase}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })
  if (!loginResponse.ok) {
    mockServer.close()
    throw new Error(`Playwright platform login setup failed: ${loginResponse.status}`)
  }
  const login = await loginResponse.json() as LoginResponse
  const providers = await apiRequest<ModelProvider[]>(apiBase, `${managementBase}/model-providers?limit=100`, login.token)
  let provider = providers.find((item) => item.provider_type === 'openai' && item.name === mockProviderName)
  if (provider) {
    provider = await apiRequest<ModelProvider>(apiBase, `${managementBase}/model-providers/${provider.id}`, login.token, {
      method: 'PATCH',
      body: JSON.stringify({ base_url: mockProviderURL, status: 'active', timeout_ms: 10000, retry_count: 0, risk_level: 'low' }),
    })
  } else {
    provider = await apiRequest<ModelProvider>(apiBase, `${managementBase}/model-providers`, login.token, {
      method: 'POST',
      body: JSON.stringify({
        name: mockProviderName,
        provider_type: 'openai',
        base_url: mockProviderURL,
        api_key: 'playwright-local-key',
        status: 'active',
        timeout_ms: 10000,
        retry_count: 0,
        risk_level: 'low',
        tags: ['playwright', 'local-only'],
        metadata: { source: 'playwright_global_setup' },
      }),
    })
  }
  const models = await apiRequest<AIModel[]>(apiBase, `${managementBase}/models?provider_id=${provider.id}&limit=100`, login.token)
  const model = models.find((item) => item.model_key === mockModelKey)
  if (model) {
    await apiRequest<AIModel>(apiBase, `${managementBase}/models/${model.id}`, login.token, {
      method: 'PATCH',
      body: JSON.stringify({ display_name: 'Playwright Business AI', status: 'active', capabilities: ['chat', 'business_stage_analysis'] }),
    })
  } else {
    await apiRequest<AIModel>(apiBase, `${managementBase}/models`, login.token, {
      method: 'POST',
      body: JSON.stringify({
        provider_id: provider.id,
        model_key: mockModelKey,
        display_name: 'Playwright Business AI',
        context_window: 32000,
        max_output_tokens: 4096,
        capabilities: ['chat', 'business_stage_analysis'],
        status: 'active',
        currency: 'CNY',
        metadata: { source: 'playwright_global_setup' },
      }),
    })
  }
  const sampleResponse = await fetch(`${apiBase}/platform/admin/sample-tenants/business-closure`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${login.token}`,
      'Content-Type': 'application/json',
    },
    body: '{}',
  })
  if (!sampleResponse.ok) {
    mockServer.close()
    throw new Error(`Playwright sample tenant setup failed: ${sampleResponse.status}`)
  }

  const tenantEmail = process.env.PLAYWRIGHT_TENANT_EMAIL || 'demo@local.com'
  const tenantPassword = process.env.PLAYWRIGHT_TENANT_PASSWORD || 'MetaOrgSampleTenant!2026'
  let tenantReady = false
  for (let attempt = 0; attempt < 90; attempt += 1) {
    const tenantLoginResponse = await fetch(`${apiBase}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: tenantEmail, password: tenantPassword }),
    })
    if (tenantLoginResponse.ok) {
      const tenantLogin = await tenantLoginResponse.json() as LoginResponse
      const projectResponse = await fetch(`${apiBase}/projects?limit=100`, {
        headers: { Authorization: `Bearer ${tenantLogin.token}` },
      })
      if (projectResponse.ok) {
        const projects = await projectResponse.json() as Project[]
        if (projects.length > 0) {
          tenantReady = true
          break
        }
      }
    }
    await new Promise((resolve) => setTimeout(resolve, 2000))
  }
  if (!tenantReady) {
    mockServer.close()
    throw new Error('Playwright sample tenant provisioning did not produce a project')
  }

  return async () => {
    await apiRequest<ModelProvider>(apiBase, `${managementBase}/model-providers/${provider.id}`, login.token, {
      method: 'PATCH',
      body: JSON.stringify({ status: 'disabled' }),
    }).catch(() => undefined)
    await new Promise<void>((resolve) => mockServer.close(() => resolve()))
  }
}
