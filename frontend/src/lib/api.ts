import { getCurrentOrganizationId } from './auth'
import type { ApiOperation } from './operations'

export const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://127.0.0.1:8080/api/v1'

interface RequestOptions {
  method?: string
  body?: unknown
  token?: string
  organizationId?: string | null
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const isFormData = typeof FormData !== 'undefined' && options.body instanceof FormData
  const headers: Record<string, string> = {}

  if (options.token) {
    headers['Authorization'] = `Bearer ${options.token}`
    const organizationId = options.organizationId !== undefined ? options.organizationId : getCurrentOrganizationId()
    if (organizationId) {
      headers['X-Organization-ID'] = organizationId
    }
  }
  if (!isFormData) {
    headers['Content-Type'] = 'application/json'
  }
  const requestBody = options.body ? (isFormData ? (options.body as BodyInit) : JSON.stringify(options.body)) : undefined

  const response = await fetch(`${API_BASE}${path}`, {
    method: options.method || 'GET',
    headers,
    body: requestBody,
  })

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Unknown error' }))
    throw new Error(error.error || `HTTP ${response.status}`)
  }

  return response.json()
}

export async function login(email: string, password: string): Promise<AuthResponse> {
  return apiRequest<AuthResponse>('/auth/login', {
    method: 'POST',
    body: { email, password },
  })
}

export async function registerUser(input: RegisterUserInput): Promise<AuthResponse> {
  return apiRequest<AuthResponse>('/auth/register', {
    method: 'POST',
    body: input,
  })
}

export async function getMe(token: string): Promise<UserProfile> {
  return apiRequest<UserProfile>('/auth/me', { token })
}

export async function listSaaSModules(token: string): Promise<SaaSModule[]> {
  return apiRequest<SaaSModule[]>('/modules', { token })
}

export async function listRuntimeOperations(token: string): Promise<ApiOperation[]> {
  return apiRequest<ApiOperation[]>('/runtime/operations', { token })
}

export async function listPlatformRuntimeOperations(token: string): Promise<ApiOperation[]> {
  return apiRequest<ApiOperation[]>('/platform/admin/runtime/operations', { token })
}

export async function listPlatformOrganizations(token: string, limit = 100): Promise<SessionOrganization[]> {
  const query = limit > 0 ? `?limit=${encodeURIComponent(String(limit))}` : ''
  return apiRequest<SessionOrganization[]>(`/platform/organizations${query}`, { token })
}

export async function getPlatformPermissionProfile(token: string): Promise<PlatformPermissionProfile> {
  return apiRequest<PlatformPermissionProfile>('/platform/admin/me/permissions', { token })
}

export async function closePlatformOrganization(token: string, organizationID: string, reason = ''): Promise<SessionOrganization> {
  return apiRequest<SessionOrganization>(`/platform/admin/organizations/${encodeURIComponent(organizationID)}/close`, {
    method: 'POST',
    token,
    body: { reason },
  })
}

export async function getOrganizationSubscription(token: string, organizationID: string): Promise<OrganizationSubscription> {
  return apiRequest<OrganizationSubscription>(`/organizations/${encodeURIComponent(organizationID)}/subscription`, {
    token,
    organizationId: organizationID,
  })
}

export async function getOrganizationEntitlements(token: string, organizationID: string): Promise<Record<string, boolean>> {
  return apiRequest<Record<string, boolean>>(`/organizations/${encodeURIComponent(organizationID)}/entitlements`, {
    token,
    organizationId: organizationID,
  })
}

export async function updateOrganizationModules(
  token: string,
  organizationID: string,
  enabledModules: string[],
): Promise<Record<string, boolean>> {
  return apiRequest<Record<string, boolean>>(`/organizations/${encodeURIComponent(organizationID)}/modules`, {
    method: 'PATCH',
    token,
    organizationId: organizationID,
    body: { enabled_modules: enabledModules },
  })
}

export async function listOrganizationInvitations(token: string, organizationID: string, limit = 100): Promise<OrganizationInvitation[]> {
  const query = limit > 0 ? `?limit=${encodeURIComponent(String(limit))}` : ''
  return apiRequest<OrganizationInvitation[]>(`/organizations/${encodeURIComponent(organizationID)}/invitations${query}`, {
    token,
    organizationId: organizationID,
  })
}

export async function createOrganizationInvitation(
  token: string,
  organizationID: string,
  input: CreateOrganizationInvitationInput,
): Promise<OrganizationInvitation> {
  return apiRequest<OrganizationInvitation>(`/organizations/${encodeURIComponent(organizationID)}/invitations`, {
    method: 'POST',
    token,
    organizationId: organizationID,
    body: input,
  })
}

export async function completeOnboarding(token: string, input: OnboardingOrganizationInput): Promise<OnboardingOrganizationResponse> {
  return apiRequest<OnboardingOrganizationResponse>('/onboarding/organization', {
    method: 'POST',
    token,
    body: input,
  })
}

export async function listRoles(): Promise<Role[]> {
  return apiRequest<Role[]>('/roles')
}

export async function listPlatformMasters(token: string, moduleKey: string, limit = 100): Promise<PlatformMaster[]> {
  const query = limit > 0 ? `?limit=${encodeURIComponent(String(limit))}` : ''
  return apiRequest<PlatformMaster[]>(`/platform/admin/modules/${encodeURIComponent(moduleKey)}/masters${query}`, { token })
}

export async function listPlatformDetails(token: string, masterKey: string): Promise<PlatformDetail[]> {
  return apiRequest<PlatformDetail[]>(`/platform/admin/masters/${encodeURIComponent(masterKey)}/details`, { token })
}

export async function listOrganizationSchemaTargets(token: string, limit = 100): Promise<OrganizationSchemaTarget[]> {
  const query = limit > 0 ? `?limit=${encodeURIComponent(String(limit))}` : ''
  return apiRequest<OrganizationSchemaTarget[]>(`/platform/admin/schema-targets${query}`, { token })
}

export async function exportOrganizationSchema(token: string, organizationID: string): Promise<SchemaPackage> {
  return apiRequest<SchemaPackage>(`/platform/admin/organizations/${encodeURIComponent(organizationID)}/schema/export`, { token })
}

export async function createOrganizationSchemaChange(
  token: string,
  organizationID: string,
  input: CreateSchemaChangeRequestInput,
): Promise<SchemaChangeRequest> {
  return apiRequest<SchemaChangeRequest>(`/platform/admin/organizations/${encodeURIComponent(organizationID)}/schema/change-requests`, {
    method: 'POST',
    token,
    body: input,
  })
}

export async function approveSchemaChange(token: string, requestID: string, reason = ''): Promise<SchemaChangeRequest> {
  return apiRequest<SchemaChangeRequest>(`/platform/admin/schema-change-requests/${encodeURIComponent(requestID)}/approve`, {
    method: 'POST',
    token,
    body: { reason },
  })
}

export async function applySchemaChange(token: string, requestID: string): Promise<SchemaApplyJob> {
  return apiRequest<SchemaApplyJob>(`/platform/admin/schema-change-requests/${encodeURIComponent(requestID)}/apply`, {
    method: 'POST',
    token,
  })
}

export async function listDataTables(token: string, category?: string): Promise<DataTable[]> {
  const query = category ? `?category=${encodeURIComponent(category)}` : ''
  return apiRequest<DataTable[]>(`/governance/data/tables${query}`, { token })
}

export async function listDataFields(token: string, tableName: string): Promise<DataField[]> {
  return apiRequest<DataField[]>(`/governance/data/tables/${encodeURIComponent(tableName)}/fields`, { token })
}

export async function getUserFieldPreference(token: string, tableName: string): Promise<UserFieldPreference> {
  return apiRequest<UserFieldPreference>(`/governance/data/field-preferences/${encodeURIComponent(tableName)}`, { token })
}

export async function saveUserFieldPreference(token: string, tableName: string, input: SaveUserFieldPreferenceInput): Promise<UserFieldPreference> {
  return apiRequest<UserFieldPreference>(`/governance/data/field-preferences/${encodeURIComponent(tableName)}`, {
    method: 'PUT',
    token,
    body: input,
  })
}

export async function getUserPreference(token: string, key: string): Promise<UserPreference> {
  return apiRequest<UserPreference>(`/preferences/${encodeURIComponent(key)}`, { token })
}

export async function saveUserPreference(token: string, key: string, value: Record<string, unknown>): Promise<UserPreference> {
  return apiRequest<UserPreference>(`/preferences/${encodeURIComponent(key)}`, {
    method: 'PUT',
    token,
    body: { value },
  })
}

export async function createFieldPermissionRule(token: string, input: CreateFieldPermissionRuleInput): Promise<FieldPermissionRule> {
  return apiRequest<FieldPermissionRule>('/governance/data/field-permissions', {
    method: 'POST',
    token,
    body: input,
  })
}

export async function listFieldPermissionRules(token: string, tableName?: string): Promise<FieldPermissionRule[]> {
  const query = tableName ? `?table=${encodeURIComponent(tableName)}` : ''
  return apiRequest<FieldPermissionRule[]>(`/governance/data/field-permissions${query}`, { token })
}

export async function checkFieldAccess(token: string, input: FieldAccessCheckInput): Promise<FieldAccessCheckResult> {
  return apiRequest<FieldAccessCheckResult>('/governance/data/field-access/check', {
    method: 'POST',
    token,
    body: input,
  })
}

export async function getDashboardOverview(token: string): Promise<DashboardOverview> {
  return apiRequest<DashboardOverview>('/dashboard/overview', { token })
}

export async function getMetaOrgOverview(token: string): Promise<MetaOrgOverview> {
  return apiRequest<MetaOrgOverview>('/meta-org/overview', { token })
}

export async function getMetaOrgInbox(token: string): Promise<InboxItem[]> {
  return apiRequest<InboxItem[]>('/meta-org/inbox', { token })
}

export async function getPlatformMetaOrgOverview(token: string): Promise<MetaOrgOverview> {
  return apiRequest<MetaOrgOverview>('/platform/admin/meta-org/overview', { token })
}

export async function getPlatformMetaOrgInbox(token: string): Promise<InboxItem[]> {
  return apiRequest<InboxItem[]>('/platform/admin/meta-org/inbox', { token })
}

export async function listMetaResources(token: string, filter: { resource_type?: string; status?: string } = {}): Promise<MetaResource[]> {
  const params = new URLSearchParams()
  if (filter.resource_type) params.set('resource_type', filter.resource_type)
  if (filter.status) params.set('status', filter.status)
  const query = params.toString() ? `?${params.toString()}` : ''
  return apiRequest<MetaResource[]>(`/meta-resources${query}`, { token })
}

export async function createMetaResource(token: string, input: CreateMetaResourceInput): Promise<MetaResource> {
  return apiRequest<MetaResource>('/meta-resources', { method: 'POST', token, body: input })
}

export async function syncExistingMetaResources(token: string): Promise<Record<string, number>> {
  return apiRequest<Record<string, number>>('/meta-resources/sync-existing', { method: 'POST', token })
}

export async function getMetaResourceSummary(token: string): Promise<MetaResourceSummary> {
  return apiRequest<MetaResourceSummary>('/meta-resources/summary', { token })
}

export async function listDemandProfiles(token: string): Promise<DemandProfile[]> {
  return apiRequest<DemandProfile[]>('/demand-profiles', { token })
}

export async function createDemandProfile(token: string, input: CreateDemandProfileInput): Promise<DemandProfile> {
  return apiRequest<DemandProfile>('/demand-profiles', { method: 'POST', token, body: input })
}

export async function listPDCACycles(token: string): Promise<PDCACycle[]> {
  return apiRequest<PDCACycle[]>('/pdca-cycles', { token })
}

export async function createPDCACycle(token: string, input: CreatePDCACycleInput): Promise<PDCACycle> {
  return apiRequest<PDCACycle>('/pdca-cycles', { method: 'POST', token, body: input })
}

export async function listPDCAEvents(token: string, cycleID?: string): Promise<PDCAEvent[]> {
  const query = cycleID ? `?cycle_id=${encodeURIComponent(cycleID)}` : ''
  return apiRequest<PDCAEvent[]>(`/pdca-events${query}`, { token })
}

export async function createPDCAEvent(token: string, input: CreatePDCAEventInput): Promise<PDCAEvent> {
  return apiRequest<PDCAEvent>('/pdca-events', { method: 'POST', token, body: input })
}

export async function listModelProviders(token: string): Promise<ModelProvider[]> {
  return apiRequest<ModelProvider[]>('/model-providers', { token })
}

export async function listPlatformModelProviders(token: string): Promise<ModelProvider[]> {
  return apiRequest<ModelProvider[]>('/platform/admin/model-providers', { token })
}

export async function createModelProvider(token: string, input: CreateModelProviderInput): Promise<ModelProvider> {
  return apiRequest<ModelProvider>('/model-providers', { method: 'POST', token, body: input })
}

export async function rotateModelProviderKey(token: string, id: string, apiKey: string): Promise<ModelProvider> {
  return apiRequest<ModelProvider>(`/model-providers/${id}/rotate-key`, {
    method: 'POST',
    token,
    body: { api_key: apiKey },
  })
}

export async function testModelProvider(token: string, id: string, model?: string): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/model-providers/${id}/test`, {
    method: 'POST',
    token,
    body: { model },
  })
}

export async function listProviderChannels(token: string, providerID?: string): Promise<ProviderChannel[]> {
  const query = providerID ? `?provider_id=${encodeURIComponent(providerID)}` : ''
  return apiRequest<ProviderChannel[]>(`/model-provider-channels${query}`, { token })
}

export async function createProviderChannel(token: string, providerID: string, input: CreateProviderChannelInput): Promise<ProviderChannel> {
  return apiRequest<ProviderChannel>(`/model-providers/${providerID}/channels`, { method: 'POST', token, body: input })
}

export async function updateProviderChannel(token: string, id: string, input: UpdateProviderChannelInput): Promise<ProviderChannel> {
  return apiRequest<ProviderChannel>(`/model-provider-channels/${id}`, { method: 'PATCH', token, body: input })
}

export async function rotateProviderChannelKey(token: string, id: string, apiKey: string): Promise<ProviderChannel> {
  return apiRequest<ProviderChannel>(`/model-provider-channels/${id}/rotate-key`, {
    method: 'POST',
    token,
    body: { api_key: apiKey },
  })
}

export async function testProviderChannel(token: string, id: string, model?: string): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/model-provider-channels/${id}/test`, {
    method: 'POST',
    token,
    body: { model },
  })
}

export async function listModels(token: string): Promise<ModelCatalogItem[]> {
  return apiRequest<ModelCatalogItem[]>('/models', { token })
}

export async function listPlatformModels(token: string): Promise<ModelCatalogItem[]> {
  return apiRequest<ModelCatalogItem[]>('/platform/admin/models', { token })
}

export async function createModel(token: string, input: CreateModelInput): Promise<ModelCatalogItem> {
  return apiRequest<ModelCatalogItem>('/models', { method: 'POST', token, body: input })
}

export async function listTools(token: string): Promise<ToolDefinition[]> {
  return apiRequest<ToolDefinition[]>('/tools', { token })
}

export async function listInterfaceFiles(token: string): Promise<InterfaceFile[]> {
  return apiRequest<InterfaceFile[]>('/interface-files', { token })
}

export async function getInterfaceFile(token: string, id: string): Promise<InterfaceFile> {
  return apiRequest<InterfaceFile>(`/interface-files/${id}`, { token })
}

export async function createInterfaceFile(token: string, input: CreateInterfaceFileInput): Promise<InterfaceFile> {
  return apiRequest<InterfaceFile>('/interface-files', { method: 'POST', token, body: input })
}

export async function updateInterfaceFile(token: string, id: string, input: UpdateInterfaceFileInput): Promise<InterfaceFile> {
  return apiRequest<InterfaceFile>(`/interface-files/${id}`, { method: 'PATCH', token, body: input })
}

export async function listToolExecutions(token: string): Promise<ToolExecution[]> {
  return apiRequest<ToolExecution[]>('/tool-executions', { token })
}

export async function approveToolApproval(token: string, id: string, reason = 'approved from human review console'): Promise<ToolApprovalReviewResult> {
  return apiRequest<ToolApprovalReviewResult>(`/tool-approvals/${id}/approve`, { method: 'POST', token, body: { reason } })
}

export async function rejectToolApproval(token: string, id: string, reason = 'rejected from human review console'): Promise<ToolApprovalReviewResult> {
  return apiRequest<ToolApprovalReviewResult>(`/tool-approvals/${id}/reject`, { method: 'POST', token, body: { reason } })
}

export async function approvePlatformToolApproval(token: string, id: string, reason = 'approved from platform assistant'): Promise<ToolApprovalReviewResult> {
  return apiRequest<ToolApprovalReviewResult>(`/platform/admin/tool-approvals/${id}/approve`, { method: 'POST', token, body: { reason } })
}

export async function rejectPlatformToolApproval(token: string, id: string, reason = 'rejected from platform assistant'): Promise<ToolApprovalReviewResult> {
  return apiRequest<ToolApprovalReviewResult>(`/platform/admin/tool-approvals/${id}/reject`, { method: 'POST', token, body: { reason } })
}

export async function listInvocations(token: string): Promise<AIInvocation[]> {
  return apiRequest<AIInvocation[]>('/ai-gateway/invocations', { token })
}

export async function getAIInvocation(token: string, id: string): Promise<AIInvocation> {
  return apiRequest<AIInvocation>(`/ai-gateway/invocations/${id}`, { token })
}

export async function getPlatformAIInvocation(token: string, id: string): Promise<AIInvocation> {
  return apiRequest<AIInvocation>(`/platform/admin/ai-gateway/invocations/${id}`, { token })
}

export async function createAssistantSession(token: string, input: CreateAssistantSessionInput): Promise<AssistantSession> {
  return apiRequest<AssistantSession>('/assistant/sessions', { method: 'POST', token, body: input })
}

export async function createPlatformAssistantSession(token: string, input: CreateAssistantSessionInput): Promise<AssistantSession> {
  return apiRequest<AssistantSession>('/platform/admin/assistant/sessions', { method: 'POST', token, body: input })
}

export async function listAssistantSessions(token: string, moduleKey?: string): Promise<AssistantSession[]> {
  const query = moduleKey ? `?module_key=${encodeURIComponent(moduleKey)}` : ''
  return apiRequest<AssistantSession[]>(`/assistant/sessions${query}`, { token })
}

export async function listAssistantSteps(token: string, sessionID: string): Promise<AssistantStep[]> {
  return apiRequest<AssistantStep[]>(`/assistant/sessions/${sessionID}/steps`, { token })
}

export async function listAssistantContextTargets(token: string, moduleKey: string, targetType?: string): Promise<AssistantContextTarget[]> {
  const params = new URLSearchParams()
  if (moduleKey) params.set('module_key', moduleKey)
  if (targetType) params.set('target_type', targetType)
  const query = params.toString() ? `?${params.toString()}` : ''
  return apiRequest<AssistantContextTarget[]>(`/assistant/context-targets${query}`, { token })
}

export async function listPlatformAssistantContextTargets(token: string, moduleKey: string, targetType?: string): Promise<AssistantContextTarget[]> {
  const params = new URLSearchParams()
  if (moduleKey) params.set('module_key', moduleKey)
  if (targetType) params.set('target_type', targetType)
  const query = params.toString() ? `?${params.toString()}` : ''
  return apiRequest<AssistantContextTarget[]>(`/platform/admin/assistant/context-targets${query}`, { token })
}

export async function listAssistantProposals(token: string, sessionID: string): Promise<AssistantProposal[]> {
  return apiRequest<AssistantProposal[]>(`/assistant/sessions/${sessionID}/proposals`, { token })
}

export async function listPlatformAssistantProposals(token: string, sessionID: string): Promise<AssistantProposal[]> {
  return apiRequest<AssistantProposal[]>(`/platform/admin/assistant/sessions/${sessionID}/proposals`, { token })
}

export async function confirmAssistantProposal(token: string, proposalID: string): Promise<AssistantProposal> {
  return apiRequest<AssistantProposal>(`/assistant/proposals/${proposalID}/confirm`, { method: 'POST', token })
}

export async function rejectAssistantProposal(token: string, proposalID: string, reason = ''): Promise<AssistantProposal> {
  return apiRequest<AssistantProposal>(`/assistant/proposals/${proposalID}/reject`, {
    method: 'POST',
    token,
    body: { reason },
  })
}

export async function confirmPlatformAssistantProposal(token: string, proposalID: string): Promise<AssistantProposal> {
  return apiRequest<AssistantProposal>(`/platform/admin/assistant/proposals/${proposalID}/confirm`, { method: 'POST', token })
}

export async function rejectPlatformAssistantProposal(token: string, proposalID: string, reason = ''): Promise<AssistantProposal> {
  return apiRequest<AssistantProposal>(`/platform/admin/assistant/proposals/${proposalID}/reject`, {
    method: 'POST',
    token,
    body: { reason },
  })
}

export async function listAssistantSkills(token: string, moduleKey?: string, targetType?: string): Promise<AssistantBusinessSkill[]> {
  const params = new URLSearchParams()
  if (moduleKey) params.set('module_key', moduleKey)
  if (targetType) params.set('target_type', targetType)
  const query = params.toString() ? `?${params.toString()}` : ''
  return apiRequest<AssistantBusinessSkill[]>(`/assistant/skills${query}`, { token })
}

export async function listPlatformAssistantSkills(token: string, moduleKey?: string, targetType?: string): Promise<AssistantBusinessSkill[]> {
  const params = new URLSearchParams()
  if (moduleKey) params.set('module_key', moduleKey)
  if (targetType) params.set('target_type', targetType)
  const query = params.toString() ? `?${params.toString()}` : ''
  return apiRequest<AssistantBusinessSkill[]>(`/platform/admin/assistant/skills${query}`, { token })
}

export async function createAssistantSkill(
  token: string,
  input: CreateAssistantBusinessSkillInput,
): Promise<AssistantBusinessSkill> {
  return apiRequest<AssistantBusinessSkill>('/assistant/skills', { method: 'POST', token, body: input })
}

export async function createPlatformAssistantSkill(
  token: string,
  input: CreateAssistantBusinessSkillInput,
): Promise<AssistantBusinessSkill> {
  return apiRequest<AssistantBusinessSkill>('/platform/admin/assistant/skills', { method: 'POST', token, body: input })
}

export async function activateAssistantSkill(token: string, skillID: string): Promise<AssistantBusinessSkill> {
  return apiRequest<AssistantBusinessSkill>(`/assistant/skills/${skillID}/activate`, { method: 'POST', token })
}

export async function activatePlatformAssistantSkill(token: string, skillID: string): Promise<AssistantBusinessSkill> {
  return apiRequest<AssistantBusinessSkill>(`/platform/admin/assistant/skills/${skillID}/activate`, { method: 'POST', token })
}

export async function runAssistantSkill(
  token: string,
  skillID: string,
  input: Record<string, unknown>,
): Promise<AssistantSkillRun> {
  return apiRequest<AssistantSkillRun>(`/assistant/skills/${skillID}/run`, { method: 'POST', token, body: input })
}

export async function runPlatformAssistantSkill(
  token: string,
  skillID: string,
  input: Record<string, unknown>,
): Promise<AssistantSkillRun> {
  return apiRequest<AssistantSkillRun>(`/platform/admin/assistant/skills/${skillID}/run`, { method: 'POST', token, body: input })
}

export async function getAICostSummary(token: string): Promise<AICostSummary> {
  return apiRequest<AICostSummary>('/ai-gateway/cost-summary', { token })
}

export async function listRoutingRules(token: string): Promise<AIRoutingRule[]> {
  return apiRequest<AIRoutingRule[]>('/ai-gateway/routing-rules', { token })
}

export async function createRoutingRule(token: string, input: CreateAIRoutingRuleInput): Promise<AIRoutingRule> {
  return apiRequest<AIRoutingRule>('/ai-gateway/routing-rules', { method: 'POST', token, body: input })
}

export async function getAIUsageAnalysis(token: string): Promise<AIUsageAnalysis> {
  return apiRequest<AIUsageAnalysis>('/ai-gateway/usage-analysis', { token })
}

export async function estimateAICost(token: string, input: EstimateAICostInput): Promise<EstimateAICostOutput> {
  return apiRequest<EstimateAICostOutput>('/ai-gateway/estimate-cost', { method: 'POST', token, body: input })
}

export async function listFinanceAdapters(token: string): Promise<FinanceAdapter[]> {
  return apiRequest<FinanceAdapter[]>('/finance/adapters', { token })
}

export async function createFinanceAdapter(token: string, input: CreateFinanceAdapterInput): Promise<FinanceAdapter> {
  return apiRequest<FinanceAdapter>('/finance/adapters', { method: 'POST', token, body: input })
}

export async function testFinanceAdapter(token: string, id: string): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/finance/adapters/${id}/test`, { method: 'POST', token })
}

export async function createFinanceExportBatch(
  token: string,
  input: CreateFinanceExportBatchInput,
): Promise<FinanceExportBatch> {
  return apiRequest<FinanceExportBatch>('/finance/accounting-batches', { method: 'POST', token, body: input })
}

export async function listFinanceExportBatches(token: string): Promise<FinanceExportBatch[]> {
  return apiRequest<FinanceExportBatch[]>('/finance/accounting-batches', { token })
}

export async function getFinanceExportBatch(token: string, id: string): Promise<FinanceExportBatch> {
  return apiRequest<FinanceExportBatch>(`/finance/accounting-batches/${id}`, { token })
}

export async function submitFinanceExportBatch(token: string, id: string): Promise<FinanceExportBatch> {
  return apiRequest<FinanceExportBatch>(`/finance/accounting-batches/${id}/submit`, { method: 'POST', token })
}

export async function listFinanceReconciliation(token: string): Promise<FinanceReconciliationItem[]> {
  return apiRequest<FinanceReconciliationItem[]>('/finance/reconciliation', { token })
}

export async function importFinanceExpenses(token: string, input: ImportFinanceExpensesInput): Promise<FinanceImportResult> {
  return apiRequest<FinanceImportResult>('/finance/imports', { method: 'POST', token, body: input })
}

export async function importFinanceExpenseFile(token: string, adapterID: string, file: File): Promise<FinanceImportResult> {
  const form = new FormData()
  form.append('adapter_id', adapterID)
  form.append('file', file)
  return apiRequest<FinanceImportResult>('/finance/imports/files', { method: 'POST', token, body: form })
}

export async function pullFinanceExpenses(token: string, adapterID: string): Promise<FinanceImportResult> {
  return apiRequest<FinanceImportResult>(`/finance/imports/${adapterID}/pull`, { method: 'POST', token })
}

export async function listFinanceImportBatches(token: string): Promise<FinanceImportBatch[]> {
  return apiRequest<FinanceImportBatch[]>('/finance/import-batches', { token })
}

export async function listFinanceImportRecords(token: string): Promise<FinanceImportRecord[]> {
  return apiRequest<FinanceImportRecord[]>('/finance/import-records', { token })
}

export async function createFinanceSettlementOrder(token: string, input: CreateFinanceSettlementOrderInput): Promise<FinanceSettlementOrder> {
  return apiRequest<FinanceSettlementOrder>('/finance/settlement-orders', { method: 'POST', token, body: input })
}

export async function listFinanceSettlementOrders(token: string): Promise<FinanceSettlementOrder[]> {
  return apiRequest<FinanceSettlementOrder[]>('/finance/settlement-orders', { token })
}

export async function updateFinanceSettlementOrder(token: string, id: string, input: CreateFinanceSettlementOrderInput): Promise<FinanceSettlementOrder> {
  return apiRequest<FinanceSettlementOrder>(`/finance/settlement-orders/${id}`, { method: 'PATCH', token, body: input })
}

export async function postFinanceSettlementOrder(token: string, id: string): Promise<FinanceReceivable> {
  return apiRequest<FinanceReceivable>(`/finance/settlement-orders/${id}/post`, { method: 'POST', token })
}

export async function voidFinanceSettlementOrder(token: string, id: string, reason: string): Promise<FinanceSettlementOrder> {
  return apiRequest<FinanceSettlementOrder>(`/finance/settlement-orders/${id}/void`, { method: 'POST', token, body: { reason } })
}

export async function createFinanceReceivable(token: string, input: CreateFinanceReceivableInput): Promise<FinanceReceivable> {
  return apiRequest<FinanceReceivable>('/finance/receivables', { method: 'POST', token, body: input })
}

export async function listFinanceReceivables(token: string): Promise<FinanceReceivable[]> {
  return apiRequest<FinanceReceivable[]>('/finance/receivables', { token })
}

export async function updateFinanceReceivable(token: string, id: string, input: CreateFinanceReceivableInput): Promise<FinanceReceivable> {
  return apiRequest<FinanceReceivable>(`/finance/receivables/${id}`, { method: 'PATCH', token, body: input })
}

export async function voidFinanceReceivable(token: string, id: string, reason: string): Promise<FinanceReceivable> {
  return apiRequest<FinanceReceivable>(`/finance/receivables/${id}/void`, { method: 'POST', token, body: { reason } })
}

export async function createFinanceReceipt(token: string, input: CreateFinanceReceiptInput): Promise<FinanceReceipt> {
  return apiRequest<FinanceReceipt>('/finance/receipts', { method: 'POST', token, body: input })
}

export async function listFinanceReceipts(token: string): Promise<FinanceReceipt[]> {
  return apiRequest<FinanceReceipt[]>('/finance/receipts', { token })
}

export async function allocateFinanceReceipt(token: string, receiptID: string, input: AllocateFinanceReceiptInput): Promise<FinanceReceiptAllocation> {
  return apiRequest<FinanceReceiptAllocation>(`/finance/receipts/${receiptID}/allocate`, { method: 'POST', token, body: input })
}

export async function createFinancePayable(token: string, input: CreateFinancePayableInput): Promise<FinancePayable> {
  return apiRequest<FinancePayable>('/finance/payables', { method: 'POST', token, body: input })
}

export async function listFinancePayables(token: string): Promise<FinancePayable[]> {
  return apiRequest<FinancePayable[]>('/finance/payables', { token })
}

export async function updateFinancePayable(token: string, id: string, input: CreateFinancePayableInput): Promise<FinancePayable> {
  return apiRequest<FinancePayable>(`/finance/payables/${id}`, { method: 'PATCH', token, body: input })
}

export async function voidFinancePayable(token: string, id: string, reason: string): Promise<FinancePayable> {
  return apiRequest<FinancePayable>(`/finance/payables/${id}/void`, { method: 'POST', token, body: { reason } })
}

export async function createFinancePayment(token: string, input: CreateFinancePaymentInput): Promise<FinancePayment> {
  return apiRequest<FinancePayment>('/finance/payments', { method: 'POST', token, body: input })
}

export async function listFinancePayments(token: string): Promise<FinancePayment[]> {
  return apiRequest<FinancePayment[]>('/finance/payments', { token })
}

export async function updateFinancePayment(token: string, id: string, input: CreateFinancePaymentInput): Promise<FinancePayment> {
  return apiRequest<FinancePayment>(`/finance/payments/${id}`, { method: 'PATCH', token, body: input })
}

export async function voidFinancePayment(token: string, id: string, reason: string): Promise<FinancePayment> {
  return apiRequest<FinancePayment>(`/finance/payments/${id}/void`, { method: 'POST', token, body: { reason } })
}

export async function allocateFinancePayment(token: string, paymentID: string, input: AllocateFinancePaymentInput): Promise<FinancePaymentAllocation> {
  return apiRequest<FinancePaymentAllocation>(`/finance/payments/${paymentID}/allocate`, { method: 'POST', token, body: input })
}

export async function listBusinessPartners(token: string): Promise<BusinessPartner[]> {
  return apiRequest<BusinessPartner[]>('/inventory/partners', { token })
}

export async function createBusinessPartner(token: string, input: CreateBusinessPartnerInput): Promise<BusinessPartner> {
  return apiRequest<BusinessPartner>('/inventory/partners', { method: 'POST', token, body: input })
}

export async function listInventoryItems(token: string): Promise<InventoryItem[]> {
  return apiRequest<InventoryItem[]>('/inventory/items', { token })
}

export async function createInventoryItem(token: string, input: CreateInventoryItemInput): Promise<InventoryItem> {
  return apiRequest<InventoryItem>('/inventory/items', { method: 'POST', token, body: input })
}

export async function listWarehouses(token: string): Promise<Warehouse[]> {
  return apiRequest<Warehouse[]>('/inventory/warehouses', { token })
}

export async function createWarehouse(token: string, input: CreateWarehouseInput): Promise<Warehouse> {
  return apiRequest<Warehouse>('/inventory/warehouses', { method: 'POST', token, body: input })
}

export async function listInventoryBalances(token: string): Promise<InventoryBalance[]> {
  return apiRequest<InventoryBalance[]>('/inventory/balances', { token })
}

export async function listInventoryMovements(token: string): Promise<InventoryMovement[]> {
  return apiRequest<InventoryMovement[]>('/inventory/movements', { token })
}

export async function createInventoryMovement(token: string, input: CreateInventoryMovementInput): Promise<InventoryMovement> {
  return apiRequest<InventoryMovement>('/inventory/movements', { method: 'POST', token, body: input })
}

export async function listInventoryTransfers(token: string): Promise<InventoryTransfer[]> {
  return apiRequest<InventoryTransfer[]>('/inventory/transfers', { token })
}

export async function createInventoryTransfer(token: string, input: CreateInventoryTransferInput): Promise<InventoryTransfer> {
  return apiRequest<InventoryTransfer>('/inventory/transfers', { method: 'POST', token, body: input })
}

export async function listInventoryAdjustments(token: string): Promise<InventoryAdjustment[]> {
  return apiRequest<InventoryAdjustment[]>('/inventory/adjustments', { token })
}

export async function createInventoryAdjustment(token: string, input: CreateInventoryAdjustmentInput): Promise<InventoryAdjustment> {
  return apiRequest<InventoryAdjustment>('/inventory/adjustments', { method: 'POST', token, body: input })
}

export async function listInventoryCounts(token: string): Promise<InventoryCount[]> {
  return apiRequest<InventoryCount[]>('/inventory/counts', { token })
}

export async function createInventoryCount(token: string, input: CreateInventoryCountInput): Promise<InventoryCount> {
  return apiRequest<InventoryCount>('/inventory/counts', { method: 'POST', token, body: input })
}

export async function listPurchaseRequisitions(token: string): Promise<PurchaseRequisition[]> {
  return apiRequest<PurchaseRequisition[]>('/procurement/requisitions', { token })
}

export async function createPurchaseRequisition(token: string, input: CreatePurchaseRequisitionInput): Promise<PurchaseRequisition> {
  return apiRequest<PurchaseRequisition>('/procurement/requisitions', { method: 'POST', token, body: input })
}

export async function submitPurchaseRequisition(token: string, id: string): Promise<PurchaseRequisition> {
  return apiRequest<PurchaseRequisition>(`/procurement/requisitions/${id}/submit`, { method: 'POST', token })
}

export async function approvePurchaseRequisition(token: string, id: string): Promise<PurchaseRequisition> {
  return apiRequest<PurchaseRequisition>(`/procurement/requisitions/${id}/approve`, { method: 'POST', token })
}

export async function listPurchaseOrders(token: string): Promise<PurchaseOrder[]> {
  return apiRequest<PurchaseOrder[]>('/procurement/orders', { token })
}

export async function createPurchaseOrder(token: string, input: CreatePurchaseOrderInput): Promise<PurchaseOrder> {
  return apiRequest<PurchaseOrder>('/procurement/orders', { method: 'POST', token, body: input })
}

export async function submitPurchaseOrder(token: string, id: string): Promise<PurchaseOrder> {
  return apiRequest<PurchaseOrder>(`/procurement/orders/${id}/submit`, { method: 'POST', token })
}

export async function approvePurchaseOrder(token: string, id: string): Promise<PurchaseOrder> {
  return apiRequest<PurchaseOrder>(`/procurement/orders/${id}/approve`, { method: 'POST', token })
}

export async function listPurchaseReceipts(token: string): Promise<PurchaseReceipt[]> {
  return apiRequest<PurchaseReceipt[]>('/procurement/receipts', { token })
}

export async function createPurchaseReceipt(token: string, input: CreatePurchaseReceiptInput): Promise<PurchaseReceipt> {
  return apiRequest<PurchaseReceipt>('/procurement/receipts', { method: 'POST', token, body: input })
}

export async function postPurchaseReceipt(token: string, id: string): Promise<PurchaseReceipt> {
  return apiRequest<PurchaseReceipt>(`/procurement/receipts/${id}/post`, { method: 'POST', token })
}

export async function listPurchaseReturns(token: string): Promise<PurchaseReturn[]> {
  return apiRequest<PurchaseReturn[]>('/procurement/returns', { token })
}

export async function createPurchaseReturn(token: string, input: CreatePurchaseReturnInput): Promise<PurchaseReturn> {
  return apiRequest<PurchaseReturn>('/procurement/returns', { method: 'POST', token, body: input })
}

export async function listSalesQuotations(token: string): Promise<SalesQuotation[]> {
  return apiRequest<SalesQuotation[]>('/sales/quotations', { token })
}

export async function createSalesQuotation(token: string, input: CreateSalesQuotationInput): Promise<SalesQuotation> {
  return apiRequest<SalesQuotation>('/sales/quotations', { method: 'POST', token, body: input })
}

export async function listSalesOrders(token: string): Promise<SalesOrder[]> {
  return apiRequest<SalesOrder[]>('/sales/orders', { token })
}

export async function createSalesOrder(token: string, input: CreateSalesOrderInput): Promise<SalesOrder> {
  return apiRequest<SalesOrder>('/sales/orders', { method: 'POST', token, body: input })
}

export async function confirmSalesOrder(token: string, id: string): Promise<SalesOrder> {
  return apiRequest<SalesOrder>(`/sales/orders/${id}/confirm`, { method: 'POST', token })
}

export async function approveSalesOrder(token: string, id: string): Promise<SalesOrder> {
  return apiRequest<SalesOrder>(`/sales/orders/${id}/approve`, { method: 'POST', token })
}

export async function listSalesShipments(token: string): Promise<SalesShipment[]> {
  return apiRequest<SalesShipment[]>('/sales/shipments', { token })
}

export async function createSalesShipment(token: string, input: CreateSalesShipmentInput): Promise<SalesShipment> {
  return apiRequest<SalesShipment>('/sales/shipments', { method: 'POST', token, body: input })
}

export async function postSalesShipment(token: string, id: string): Promise<SalesShipment> {
  return apiRequest<SalesShipment>(`/sales/shipments/${id}/post`, { method: 'POST', token })
}

export async function listSalesReturns(token: string): Promise<SalesReturn[]> {
  return apiRequest<SalesReturn[]>('/sales/returns', { token })
}

export async function createSalesReturn(token: string, input: CreateSalesReturnInput): Promise<SalesReturn> {
  return apiRequest<SalesReturn>('/sales/returns', { method: 'POST', token, body: input })
}

export async function listCurrencies(token: string): Promise<Currency[]> {
  return apiRequest<Currency[]>('/costing/currencies', { token })
}

export async function upsertCurrency(token: string, input: CreateCurrencyInput): Promise<Currency> {
  return apiRequest<Currency>('/costing/currencies', { method: 'POST', token, body: input })
}

export async function voidCurrency(token: string, code: string): Promise<Currency> {
  return apiRequest<Currency>(`/costing/currencies/${encodeURIComponent(code)}/void`, { method: 'POST', token })
}

export async function listExchangeRates(token: string): Promise<ExchangeRateVersion[]> {
  return apiRequest<ExchangeRateVersion[]>('/costing/exchange-rates', { token })
}

export async function createExchangeRate(token: string, input: CreateExchangeRateInput): Promise<ExchangeRateVersion> {
  return apiRequest<ExchangeRateVersion>('/costing/exchange-rates', { method: 'POST', token, body: input })
}

export async function updateExchangeRate(token: string, id: string, input: CreateExchangeRateInput): Promise<ExchangeRateVersion> {
  return apiRequest<ExchangeRateVersion>(`/costing/exchange-rates/${id}`, { method: 'PATCH', token, body: input })
}

export async function voidExchangeRate(token: string, id: string): Promise<ExchangeRateVersion> {
  return apiRequest<ExchangeRateVersion>(`/costing/exchange-rates/${id}/void`, { method: 'POST', token })
}

export async function convertCostAmount(token: string, input: ConvertCostInput): Promise<ConversionResult> {
  return apiRequest<ConversionResult>('/costing/convert', { method: 'POST', token, body: input })
}

export async function listCostRateCards(token: string): Promise<CostRateCard[]> {
  return apiRequest<CostRateCard[]>('/costing/rate-cards', { token })
}

export async function createCostRateCard(token: string, input: CreateCostRateCardInput): Promise<CostRateCard> {
  return apiRequest<CostRateCard>('/costing/rate-cards', { method: 'POST', token, body: input })
}

export async function updateCostRateCard(token: string, id: string, input: CreateCostRateCardInput): Promise<CostRateCard> {
  return apiRequest<CostRateCard>(`/costing/rate-cards/${id}`, { method: 'PATCH', token, body: input })
}

export async function voidCostRateCard(token: string, id: string): Promise<CostRateCard> {
  return apiRequest<CostRateCard>(`/costing/rate-cards/${id}/void`, { method: 'POST', token })
}

export async function listCostBudgets(token: string): Promise<CostBudget[]> {
  return apiRequest<CostBudget[]>('/costing/budgets', { token })
}

export async function createCostBudget(token: string, input: CreateCostBudgetInput): Promise<CostBudget> {
  return apiRequest<CostBudget>('/costing/budgets', { method: 'POST', token, body: input })
}

export async function updateCostBudget(token: string, id: string, input: CreateCostBudgetInput): Promise<CostBudget> {
  return apiRequest<CostBudget>(`/costing/budgets/${id}`, { method: 'PATCH', token, body: input })
}

export async function voidCostBudget(token: string, id: string): Promise<CostBudget> {
  return apiRequest<CostBudget>(`/costing/budgets/${id}/void`, { method: 'POST', token })
}

export async function listCostLedgerEntries(token: string): Promise<CostLedgerEntry[]> {
  return apiRequest<CostLedgerEntry[]>('/costing/ledger-entries', { token })
}

export async function createCostLedgerEntry(token: string, input: CreateCostLedgerEntryInput): Promise<CostLedgerEntry> {
  return apiRequest<CostLedgerEntry>('/costing/ledger-entries', { method: 'POST', token, body: input })
}

export async function updateCostLedgerEntry(token: string, id: string, input: CreateCostLedgerEntryInput): Promise<CostLedgerEntry> {
  return apiRequest<CostLedgerEntry>(`/costing/ledger-entries/${id}`, { method: 'PATCH', token, body: input })
}

export async function voidCostLedgerEntry(token: string, id: string): Promise<CostLedgerEntry> {
  return apiRequest<CostLedgerEntry>(`/costing/ledger-entries/${id}/void`, { method: 'POST', token })
}

export async function getCostSummary(token: string): Promise<CostSummary> {
  return apiRequest<CostSummary>('/costing/summary', { token })
}

export interface AuthResponse {
  token: string
  user_id: string
  user_type: 'human' | 'ai'
  expires_at: number
  onboarding_required?: boolean
  default_organization_id?: string
  platform_role?: string
  organizations?: SessionOrganization[]
  enabled_modules?: Record<string, boolean>
}

export interface SessionOrganization {
  id: string
  name: string
  description?: string
  membership_id?: string
  authority_tier?: string
  is_owner?: boolean
  status?: 'active' | 'closed' | string
  closed_at?: string
  closed_by?: string
  closed_reason?: string
}

export interface PlatformPermissionProfile {
  role: string
  permissions: Record<string, boolean>
  menu_items: string[]
}

export interface UserProfile {
  id: string
  name: string
  email: string
  account_status: string
  onboarding_status: string
  onboarding_required: boolean
  default_organization_id?: string
  platform_role?: string
  organizations: SessionOrganization[]
  enabled_modules?: Record<string, boolean>
}

export interface SaaSModule {
  module_key: string
  display_name: string
  category: string
  enabled_default: boolean
  license_scope: 'mit' | 'commercial'
  metadata: Record<string, unknown>
}

export interface OrganizationSubscription {
  id: string
  organization_id: string
  plan_id?: string
  plan_code?: string
  plan_name?: string
  status: string
  trial_ends_at?: string
  current_period_start?: string
  current_period_end?: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface OrganizationInvitation {
  id: string
  organization_id: string
  email: string
  name?: string
  role_id?: string
  authority_tier: string
  status: string
  invited_by?: string
  accepted_by?: string
  expires_at: string
  metadata: Record<string, unknown>
  accepted_at?: string
  created_at: string
  updated_at: string
  token?: string
}

export interface CreateOrganizationInvitationInput {
  email: string
  name?: string
  role_id?: string
  authority_tier?: string
  expires_in_days?: number
  metadata?: Record<string, unknown>
}

export interface OnboardingOrganizationInput {
  organization_name: string
  description?: string
  enabled_modules?: string[]
}

export interface OnboardingOrganizationResponse {
  profile: UserProfile
  organization: SessionOrganization
}

export interface UserResponse {
  id: string
  name: string
  email: string
  avatar_url?: string
  created_at: string
  updated_at: string
}

export interface RegisterUserInput {
  name: string
  email: string
  password: string
}

export interface Role {
  id: string
  name: string
  role_type: 'planner' | 'executor' | 'reviewer'
  description?: string
  permissions: string[]
}

export interface PlatformMaster {
  master_key: string
  module_key: string
  entity_type: string
  source_table: string
  source_pk: string
  title: string
  status: string
  organization_id?: string
  payload: Record<string, unknown>
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface PlatformDetail {
  detail_key: string
  master_key: string
  detail_type: string
  field_key: string
  line_no: number
  payload: Record<string, unknown>
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface OrganizationSchemaTarget {
  organization_id: string
  schema_name: string
  template_version: string
  status: string
  last_change_request_id?: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface SchemaPackage {
  format_version: string
  module_key: string
  tables: SchemaTableDefinition[]
  metadata?: Record<string, unknown>
}

export interface SchemaTableDefinition {
  name: string
  previous_name?: string
  fields: SchemaFieldDefinition[]
  indexes?: SchemaIndexDefinition[]
  constraints?: string[]
  seeds?: Array<Record<string, unknown>>
  metadata?: Record<string, unknown>
}

export interface SchemaFieldDefinition {
  name: string
  previous_name?: string
  data_type: string
  nullable: boolean
  primary_key?: boolean
  default?: string
}

export interface SchemaIndexDefinition {
  name: string
  fields: string[]
  unique?: boolean
  where?: string
  comment?: string
}

export interface SchemaChangeRequest {
  id: string
  organization_id: string
  schema_name: string
  request_type: string
  status: string
  risk_level?: string
  reason: string
  schema_package: SchemaPackage
  diff?: SchemaDiff
  statements: string[]
  requested_by?: string
  reviewed_by?: string
  applied_by?: string
  review_reason?: string
  created_at: string
  reviewed_at?: string
  applied_at?: string
  updated_at: string
}

export interface SchemaApplyJob {
  id: string
  change_request_id: string
  organization_id: string
  schema_name: string
  status: string
  statements: string[]
  error_message?: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateSchemaChangeRequestInput {
  organization_id?: string
  request_type?: string
  reason?: string
  current_schema_package?: SchemaPackage
  schema_package: SchemaPackage
}

export interface SchemaDiff {
  summary?: string[]
  tables_added?: string[]
  tables_removed?: string[]
  tables_renamed?: SchemaRenameDiff[]
  fields_added?: SchemaFieldDiff[]
  fields_removed?: SchemaFieldDiff[]
  fields_renamed?: SchemaFieldRenameDiff[]
  fields_changed?: SchemaFieldChangeDiff[]
  indexes_added?: SchemaIndexDiff[]
  destructive?: boolean
}

export interface SchemaRenameDiff {
  from: string
  to: string
}

export interface SchemaFieldDiff {
  table: string
  field: string
}

export interface SchemaFieldRenameDiff {
  table: string
  from: string
  to: string
}

export interface SchemaFieldChangeDiff {
  table: string
  field: string
  changes: string[]
}

export interface SchemaIndexDiff {
  table: string
  index: string
}

export interface DataTable {
  table_name: string
  master_table_name: string
  detail_table_name: string
  key_prefix: string
  display_name: string
  category: string
  is_base_data: boolean
  is_business_scenario: boolean
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface DataField {
  table_name: string
  field_name: string
  data_type: string
  display_name: string
  is_master_key: boolean
  is_sub_key: boolean
  is_visible_default: boolean
  permission_level: string
  display_order: number
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface UserFieldPreference {
  actor_id: string
  table_name: string
  visible_fields: string[]
  field_order: string[]
  field_widths: Record<string, number>
  created_at?: string
  updated_at?: string
}

export interface SaveUserFieldPreferenceInput {
  visible_fields: string[]
  field_order: string[]
  field_widths: Record<string, number>
}

export interface UserPreference {
  actor_id: string
  preference_key: string
  value: Record<string, unknown>
  created_at?: string
  updated_at?: string
}

export interface FieldPermissionRule {
  id: string
  table_name: string
  field_name: string
  actor_type: string
  actor_id?: string
  role_id?: string
  action: 'read' | 'write' | 'delete' | 'admin'
  behavior: 'allow' | 'notify' | 'approve' | 'deny'
  required_level: string
  reason: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateFieldPermissionRuleInput {
  table_name: string
  field_name?: string
  actor_type?: string
  actor_id?: string
  role_id?: string
  action: 'read' | 'write' | 'delete' | 'admin'
  behavior?: 'allow' | 'notify' | 'approve' | 'deny'
  required_level?: string
  reason?: string
  metadata?: Record<string, unknown>
}

export interface FieldAccessCheckInput {
  actor_id?: string
  actor_type?: string
  table_name: string
  field_name?: string
  action: 'read' | 'write' | 'delete' | 'admin'
}

export interface FieldAccessCheckResult {
  allowed: boolean
  behavior: string
  required_level: string
  reason: string
}

export interface AIAgent {
  id: string
  name: string
  model_type: string
  capabilities: string[]
  permission_level: string
  agent_origin: 'internal' | 'external'
  provider?: string
  service_class: string
  vendor?: string
  contract_ref?: string
  risk_level: 'low' | 'medium' | 'high' | 'critical'
  metadata: Record<string, unknown>
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface DashboardOverview {
  generated_at: string
  identity: {
    users: number
    active_agents: number
    total_agents: number
    roles: number
  }
  organization: {
    organizations: number
    mvrus: number
    mvrus_by_status: Record<string, number>
    members: number
    relationships: number
  }
  workflow: {
    templates: number
    active_templates: number
    instances: number
    instances_by_status: Record<string, number>
    tasks_by_status: Record<string, number>
    decisions_7d: number
  }
  capability: {
    capabilities: number
    active_capabilities: number
    bindings: number
    invocations_24h: number
    failed_invocations_24h: number
    average_duration_ms: number
    cost_24h: number
  }
  observability: {
    active_traces: number
    completed_traces: number
    failed_traces: number
    spans_24h: number
    metrics_24h: number
  }
  verification: {
    reports: number
    average_score: number
    pending_reviews: number
  }
  governance: {
    permissions: number
    active_principles: number
    control_rules: number
    active_control_rules: number
  }
  evolution: {
    weighted_actors: number
    experiments_by_status: Record<string, number>
    knowledge_entries: number
    unacknowledged_signals: number
    high_priority_signals: number
  }
  recent_events: RecentEvent[]
}

export interface MetaOrgOverview {
  generated_at: string
  health: {
    open_requirements: number
    active_projects: number
    active_agents: number
    pending_approvals: number
    unexported_cost: number
    currency: string
  }
  projects: {
    by_status: Record<string, number>
    over_budget: number
  }
  agents: {
    total: number
    active: number
    by_risk_level: Record<string, number>
  }
  cost: {
    today: number
    month_to_date: number
    unexported: number
    currency: string
    by_provider: Record<string, number>
  }
  risks: Array<{ id: string; title: string; severity: string; source: string }>
  activity: RecentEvent[]
}

export interface InboxItem {
  id: string
  type: string
  title: string
  status: string
  priority: string
  source?: string
  created_at: string
}

export interface MetaResource {
  id: string
  resource_type: string
  source_type?: string
  source_id?: string
  name: string
  status: string
  organization_id?: string
  department_id?: string
  owner_actor_id?: string
  owner_actor_type?: string
  capability_profile: Record<string, unknown>
  cost_profile: Record<string, unknown>
  capacity_profile: Record<string, unknown>
  risk_profile: Record<string, unknown>
  performance_profile: Record<string, unknown>
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateMetaResourceInput {
  resource_type: string
  source_type?: string
  source_id?: string
  name: string
  status?: string
  organization_id?: string
  department_id?: string
  owner_actor_id?: string
  owner_actor_type?: string
  capability_profile?: Record<string, unknown>
  cost_profile?: Record<string, unknown>
  capacity_profile?: Record<string, unknown>
  risk_profile?: Record<string, unknown>
  performance_profile?: Record<string, unknown>
  metadata?: Record<string, unknown>
}

export interface MetaResourceSummary {
  total: number
  active: number
  by_type: Record<string, number>
  by_status: Record<string, number>
  average_cost_score: number
  average_fit_score: number
  recent: MetaResource[]
  metadata?: Record<string, unknown>
}

export interface DemandProfile {
  id: string
  requirement_id?: string
  project_id?: string
  title: string
  goal: string
  status: string
  acceptance_criteria: unknown[]
  required_capabilities: unknown[]
  budget_constraints: Record<string, unknown>
  time_constraints: Record<string, unknown>
  risk_constraints: Record<string, unknown>
  resource_fit_candidates: unknown[]
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateDemandProfileInput {
  requirement_id?: string
  project_id?: string
  title: string
  goal?: string
  status?: string
  acceptance_criteria?: unknown[]
  required_capabilities?: unknown[]
  budget_constraints?: Record<string, unknown>
  time_constraints?: Record<string, unknown>
  risk_constraints?: Record<string, unknown>
  resource_fit_candidates?: unknown[]
  metadata?: Record<string, unknown>
}

export interface PDCACycle {
  id: string
  demand_profile_id?: string
  requirement_id?: string
  project_id?: string
  status: string
  current_stage: string
  outcome_score: number
  summary: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
  completed_at?: string
}

export interface CreatePDCACycleInput {
  demand_profile_id?: string
  requirement_id?: string
  project_id?: string
  status?: string
  current_stage?: string
  summary?: string
  metadata?: Record<string, unknown>
}

export interface PDCAEvent {
  id: string
  cycle_id: string
  stage: string
  event_type: string
  source_type?: string
  source_id?: string
  actor_id?: string
  actor_type?: string
  resource_refs: unknown[]
  cost_refs: unknown[]
  evidence: Record<string, unknown>
  decision: string
  next_action: string
  metadata: Record<string, unknown>
  created_at: string
}

export interface CreatePDCAEventInput {
  cycle_id: string
  stage: string
  event_type?: string
  source_type?: string
  source_id?: string
  actor_id?: string
  actor_type?: string
  resource_refs?: unknown[]
  cost_refs?: unknown[]
  evidence?: Record<string, unknown>
  decision?: string
  next_action?: string
  metadata?: Record<string, unknown>
}

export interface RecentEvent {
  id: string
  type: string
  title: string
  status?: string
  created_at: string
}

export interface ModelProvider {
  id: string
  name: string
  provider_type: 'openai' | 'anthropic' | 'gemini'
  base_url: string
  masked_api_key: string
  status: string
  timeout_ms: number
  retry_count: number
  risk_level: string
  tags: string[]
  metadata: Record<string, unknown>
  last_test_status: string
  last_test_error?: string
  last_tested_at?: string
  created_at: string
  updated_at: string
}

export interface CreateModelProviderInput {
  name: string
  provider_type: 'openai' | 'anthropic' | 'gemini'
  base_url?: string
  api_key: string
  risk_level?: string
  timeout_ms?: number
  retry_count?: number
  tags?: string[]
  metadata?: Record<string, unknown>
}

export interface ProviderChannel {
  id: string
  provider_id: string
  name: string
  base_url: string
  masked_api_key: string
  owner_type?: string
  user_id?: string
  agent_id?: string
  status: string
  priority: number
  concurrency_limit: number
  inflight_requests: number
  load_factor: number
  rate_multiplier: number
  quota_amount: number
  quota_used: number
  quota_currency: string
  supported_model_patterns: string[]
  model_mapping: Record<string, string>
  health_status: string
  last_error?: string
  last_tested_at?: string
  last_used_at?: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateProviderChannelInput {
  provider_id?: string
  name: string
  base_url?: string
  api_key: string
  owner_type?: string
  user_id?: string
  agent_id?: string
  status?: string
  priority?: number
  concurrency_limit?: number
  load_factor?: number
  rate_multiplier?: number
  quota_amount?: number
  quota_currency?: string
  supported_model_patterns?: string[]
  model_mapping?: Record<string, string>
  metadata?: Record<string, unknown>
}

export type UpdateProviderChannelInput = Partial<Omit<CreateProviderChannelInput, 'api_key' | 'provider_id'>>

export interface ModelCatalogItem {
  id: string
  provider_id: string
  model_key: string
  display_name: string
  context_window: number
  max_output_tokens: number
  capabilities: string[]
  status: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateModelInput {
  provider_id: string
  model_key: string
  display_name?: string
  context_window?: number
  max_output_tokens?: number
  capabilities?: string[]
  status?: string
  input_price_per_1k?: number
  output_price_per_1k?: number
  cache_creation_price_per_1k?: number
  cache_read_price_per_1k?: number
  cache_creation_5m_price_per_1k?: number
  cache_creation_1h_price_per_1k?: number
  image_output_price_per_1k?: number
  priority_input_price_per_1k?: number
  priority_output_price_per_1k?: number
  priority_cache_read_price_per_1k?: number
  long_context_threshold?: number
  long_context_input_multiplier?: number
  long_context_output_multiplier?: number
  billing_mode?: string
  pricing_source?: string
  currency?: string
  metadata?: Record<string, unknown>
}

export interface ToolDefinition {
  id: string
  name: string
  description: string
  source_type: string
  default_policy: string
  risk_level: string
  required_level: string
  tool_category: string
  approval_tier_required: string
  status: string
  input_schema: Record<string, unknown>
  output_schema: Record<string, unknown>
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface InterfaceFile {
  id: string
  name: string
  file_type: 'json' | 'yaml' | 'markdown'
  content: string
  metadata: Record<string, unknown>
  created_by?: string
  created_at: string
  updated_at: string
}

export interface CreateInterfaceFileInput {
  name: string
  file_type: 'json' | 'yaml' | 'markdown'
  content: string
  metadata?: Record<string, unknown>
}

export interface UpdateInterfaceFileInput {
  name?: string
  file_type?: 'json' | 'yaml' | 'markdown'
  content?: string
  metadata?: Record<string, unknown>
}

export interface ToolExecution {
  id: string
  tool_id: string
  tool_name?: string
  actor_id: string
  actor_type: string
  requested_by_human_id?: string
  policy: string
  status: string
  result_summary?: string
  error_message?: string
  created_at: string
  completed_at?: string
}

export interface ToolApproval {
  id: string
  execution_id: string
  status: string
  requested_by?: string
  reviewed_by?: string
  approved_by_human_id?: string
  reason?: string
  expires_at?: string
  created_at: string
  reviewed_at?: string
}

export interface ToolApprovalReviewResult {
  approval: ToolApproval
  execution: ToolExecution
}

export interface AIInvocation {
  id: string
  provider_id: string
  model_id: string
  channel_id?: string
  mode: string
  status: string
  attribution?: Record<string, unknown>
  requested_model?: string
  upstream_model?: string
  model_mapping_chain?: string
  service_tier?: string
  reasoning_effort?: string
  request_hash?: string
  provider_request_id?: string
  cost_amount: number
  cost_breakdown?: CostBreakdown
  currency: string
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  cache_creation_5m_tokens: number
  cache_creation_1h_tokens: number
  image_output_tokens: number
  estimated_input_tokens: number
  estimated_output_tokens: number
  duration_ms: number
  error_message?: string
  metadata?: Record<string, unknown>
  created_at: string
  completed_at?: string
}

export interface AssistantSession {
  id: string
  title: string
  mode: 'business_process' | 'self_evolution'
  module_key: string
  status: string
  agent_id?: string
  provider_type?: string
  model?: string
  service_tier?: string
  reasoning_effort?: string
  organization_id?: string
  department_id?: string
  position_id?: string
  position_assignment_id?: string
  project_id?: string
  workflow_id?: string
  task_id?: string
  target_type?: string
  target_id?: string
  working_memory: Record<string, unknown>
  metadata: Record<string, unknown>
  last_error?: string
  created_at: string
  updated_at: string
}

export interface CreateAssistantSessionInput {
  title?: string
  mode?: 'business_process' | 'self_evolution'
  module_key: string
  agent_id?: string
  provider_id?: string
  preferred_channel_id?: string
  provider_type?: 'openai' | 'anthropic' | 'gemini'
  model?: string
  service_tier?: string
  reasoning_effort?: string
  organization_id?: string
  department_id?: string
  position_id?: string
  position_assignment_id?: string
  project_id?: string
  workflow_id?: string
  task_id?: string
  target_type?: string
  target_id?: string
  auto_model?: boolean
  metadata?: Record<string, unknown>
}

export interface AssistantStep {
  id: string
  session_id: string
  module_key: string
  organization_id?: string
  department_id?: string
  position_id?: string
  position_assignment_id?: string
  invocation_id?: string
  tool_execution_id?: string
  tool_approval_id?: string
  step_type: string
  status: string
  summary: string
  data: Record<string, unknown>
  turn: number
  created_at: string
}

export interface AssistantContextTarget {
  id: string
  type: string
  title: string
  status: string
  created_at: string
}

export interface AssistantProposal {
  id: string
  session_id: string
  module_key: string
  target_type: string
  target_id?: string
  proposal_type: string
  title: string
  summary: string
  payload: Record<string, unknown>
  status: string
  reviewer_id?: string
  review_reason?: string
  apply_result: Record<string, unknown>
  error_message?: string
  source_step_id?: string
  applied_at?: string
  created_at: string
  updated_at: string
}

export interface AssistantBusinessSkill {
  id: string
  skill_key: string
  scope_level: string
  deployment_mode: string
  organization_id?: string
  owner_user_id?: string
  module_key: string
  target_type: string
  business_function_key: string
  name: string
  description: string
  trigger_intent: string
  prompt_template: string
  tool_allowlist: string[]
  input_schema: Record<string, unknown>
  output_schema: Record<string, unknown>
  skill_components: AssistantSkillComponent[]
  permission_policy: Record<string, unknown>
  context_policy: Record<string, unknown>
  pricing_policy: Record<string, unknown>
  activation_policy: Record<string, unknown>
  version: number
  status: string
  created_by?: string
  created_by_type?: string
  reviewed_by?: string
  source_session_id?: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface AssistantSkillComponent {
  key: string
  label?: {
    zh?: string
    en?: string
  }
  weight: number
  instruction: string
  required_context?: string[]
  permission_tags?: string[]
}

export interface CreateAssistantBusinessSkillInput {
  skill_key?: string
  scope_level?: string
  deployment_mode?: string
  organization_id?: string
  owner_user_id?: string
  module_key: string
  target_type?: string
  business_function_key?: string
  name: string
  description?: string
  trigger_intent?: string
  prompt_template: string
  tool_allowlist?: string[]
  input_schema?: Record<string, unknown>
  output_schema?: Record<string, unknown>
  skill_components: AssistantSkillComponent[]
  permission_policy?: Record<string, unknown>
  context_policy?: Record<string, unknown>
  pricing_policy?: Record<string, unknown>
  activation_policy?: Record<string, unknown>
  source_session_id?: string
  metadata?: Record<string, unknown>
}

export interface AssistantSkillRun {
  id: string
  skill_id: string
  session_id?: string
  module_key: string
  target_type: string
  target_id?: string
  input: Record<string, unknown>
  output: Record<string, unknown>
  status: string
  error_message?: string
  created_by?: string
  created_by_type?: string
  created_at: string
  completed_at?: string
}

export interface AICostSummary {
  total: number
  unexported: number
  currency: string
  by_provider: Record<string, number>
  by_channel?: Record<string, number>
}

export interface TokenUsage {
  input_tokens: number
  output_tokens: number
  cache_creation_tokens?: number
  cache_read_tokens?: number
  cache_creation_5m_tokens?: number
  cache_creation_1h_tokens?: number
  image_output_tokens?: number
}

export interface CostBreakdown {
  input_cost: number
  output_cost: number
  cache_creation_cost: number
  cache_read_cost: number
  image_output_cost: number
  total_cost: number
  actual_cost: number
  rate_multiplier: number
  billing_mode: string
  service_tier?: string
}

export interface AIRoutingRule {
  id: string
  name: string
  provider_id?: string
  channel_id?: string
  match_scope: string
  match_value: string
  model_pattern: string
  priority: number
  status: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateAIRoutingRuleInput {
  name: string
  provider_id?: string
  channel_id?: string
  match_scope?: string
  match_value?: string
  model_pattern?: string
  priority?: number
  status?: string
  metadata?: Record<string, unknown>
}

export interface AIUsageAnalysis {
  currency: string
  total_cost: number
  invocation_count: number
  by_provider: Record<string, number>
  by_channel: Record<string, number>
  by_model: Record<string, number>
  by_actor: Record<string, number>
  recent: AIInvocation[]
}

export interface EstimateAICostInput {
  model: string
  provider_id?: string
  provider_type?: string
  usage: TokenUsage
  service_tier?: string
  rate_multiplier?: number
}

export interface EstimateAICostOutput {
  model: string
  cost_breakdown: CostBreakdown
  currency: string
}

export interface FinanceAdapter {
  id: string
  name: string
  endpoint_url: string
  auth_type: 'hmac' | 'bearer'
  adapter_type: string
  direction: string
  masked_secret: string
  status: string
  timeout_ms: number
  retry_count: number
  field_mapping: Record<string, unknown>
  pull_config: Record<string, unknown>
  last_sync_at?: string
  last_sync_status: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateFinanceAdapterInput {
  name: string
  endpoint_url: string
  auth_type: 'hmac' | 'bearer'
  adapter_type?: string
  direction?: string
  secret: string
  timeout_ms?: number
  retry_count?: number
  field_mapping?: Record<string, unknown>
  pull_config?: Record<string, unknown>
  metadata?: Record<string, unknown>
}

export interface FinanceExportLine {
  id: string
  batch_id: string
  usage_ledger_id?: string
  cost_ledger_entry_id?: string
  project_cost_entry_id?: string
  project_id?: string
  provider_id?: string
  model_id?: string
  amount: number
  currency: string
  external_line_id: string
  status: string
  metadata: Record<string, unknown>
  created_at: string
}

export interface FinanceExportBatch {
  id: string
  adapter_id: string
  period_start: string
  period_end: string
  status: string
  currency: string
  total_amount: number
  external_batch_id: string
  error_message: string
  idempotency_key: string
  metadata: Record<string, unknown>
  lines?: FinanceExportLine[]
  created_at: string
  submitted_at?: string
  updated_at: string
}

export interface CreateFinanceExportBatchInput {
  adapter_id: string
  period_start: string
  period_end: string
  currency?: string
  metadata?: Record<string, unknown>
}

export interface FinanceReconciliationItem {
  batch_id: string
  adapter_id: string
  status: string
  currency: string
  total_amount: number
  external_amount: number
  difference_amount: number
  external_batch_id: string
  error_message: string
  submitted_at?: string
  updated_at: string
}

export interface ImportFinanceExpensesInput {
  adapter_id: string
  source_type?: string
  file_name?: string
  records: Array<Record<string, unknown>>
  metadata?: Record<string, unknown>
}

export interface FinanceImportBatch {
  id: string
  adapter_id?: string
  source_type: string
  file_name: string
  status: string
  total_records: number
  processed_records: number
  failed_records: number
  metadata: Record<string, unknown>
  created_at: string
  completed_at?: string
}

export interface FinanceImportRecord {
  id: string
  batch_id: string
  adapter_id?: string
  external_record_id: string
  expense_type: string
  raw_payload: Record<string, unknown>
  normalized_payload: Record<string, unknown>
  cost_ledger_entry_id?: string
  payable_id?: string
  status: string
  error_message: string
  metadata: Record<string, unknown>
  created_at: string
}

export interface FinanceImportResult {
  batch: FinanceImportBatch
  records: FinanceImportRecord[]
}

export interface FinanceSettlementLine {
  id: string
  settlement_order_id: string
  line_type: string
  source_type: string
  source_id?: string
  deliverable_id?: string
  description: string
  quantity: number
  unit_price: number
  amount: number
  tax_amount: number
  total_amount: number
  metadata: Record<string, unknown>
  created_at: string
}

export interface FinanceSettlementOrder {
  id: string
  settlement_number: string
  project_id?: string
  requirement_id?: string
  deliverable_id?: string
  customer_id: string
  customer_name: string
  title: string
  description: string
  subtotal: number
  tax_amount: number
  total_amount: number
  currency: string
  settlement_date?: string
  due_date?: string
  status: string
  receivable_id?: string
  metadata: Record<string, unknown>
  lines?: FinanceSettlementLine[]
  created_at: string
  updated_at: string
}

export interface CreateFinanceSettlementOrderInput {
  settlement_number?: string
  project_id?: string
  requirement_id?: string
  deliverable_id?: string
  customer_id?: string
  customer_name?: string
  title?: string
  description?: string
  currency?: string
  settlement_date?: string
  due_date?: string
  status?: string
  metadata?: Record<string, unknown>
  lines?: Array<{
    line_type?: string
    source_type?: string
    source_id?: string
    deliverable_id?: string
    description?: string
    quantity?: number
    unit_price?: number
    amount: number
    tax_amount?: number
    metadata?: Record<string, unknown>
  }>
}

export interface FinanceReceivable {
  id: string
  receivable_type: string
  settlement_order_id?: string
  source_type: string
  source_id?: string
  external_receivable_id: string
  invoice_number: string
  customer_id: string
  customer_name: string
  project_id?: string
  requirement_id?: string
  organization_id?: string
  department_id?: string
  account_code: string
  account_name: string
  amount: number
  tax_amount: number
  currency: string
  period_start?: string
  period_end?: string
  invoice_date?: string
  due_date?: string
  status: string
  received_amount: number
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateFinanceReceivableInput {
  receivable_type?: string
  settlement_order_id?: string
  source_type?: string
  source_id?: string
  external_receivable_id?: string
  invoice_number?: string
  customer_id?: string
  customer_name?: string
  project_id?: string
  requirement_id?: string
  amount: number
  tax_amount?: number
  currency?: string
  invoice_date?: string
  due_date?: string
  status?: string
  metadata?: Record<string, unknown>
}

export interface FinanceReceipt {
  id: string
  receipt_number: string
  external_receipt_id: string
  payment_method: string
  payer_account: string
  receiver_account: string
  customer_id: string
  customer_name: string
  amount: number
  currency: string
  received_at?: string
  status: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateFinanceReceiptInput {
  receipt_number?: string
  external_receipt_id?: string
  payment_method?: string
  payer_account?: string
  receiver_account?: string
  customer_id?: string
  customer_name?: string
  amount: number
  currency?: string
  received_at?: string
  status?: string
  metadata?: Record<string, unknown>
}

export interface AllocateFinanceReceiptInput {
  receivable_id: string
  amount: number
  currency?: string
  metadata?: Record<string, unknown>
}

export interface FinanceReceiptAllocation {
  id: string
  receipt_id: string
  receivable_id: string
  amount: number
  currency: string
  metadata: Record<string, unknown>
  created_at: string
}

export interface FinancePayable {
  id: string
  payable_type: string
  source_type: string
  external_payable_id: string
  invoice_number: string
  vendor_id: string
  vendor_name: string
  employee_id: string
  employee_name: string
  project_id?: string
  account_code: string
  account_name: string
  cost_center_code: string
  cost_center_name: string
  amount: number
  tax_amount: number
  currency: string
  period_start?: string
  period_end?: string
  invoice_date?: string
  due_date?: string
  status: string
  paid_amount: number
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateFinancePayableInput {
  payable_type?: string
  external_payable_id?: string
  invoice_number?: string
  vendor_id?: string
  vendor_name?: string
  employee_id?: string
  employee_name?: string
  project_id?: string
  account_code?: string
  account_name?: string
  cost_center_code?: string
  cost_center_name?: string
  amount: number
  tax_amount?: number
  currency?: string
  invoice_date?: string
  due_date?: string
  status?: string
  metadata?: Record<string, unknown>
}

export interface FinancePayment {
  id: string
  payment_number: string
  external_payment_id: string
  payment_method: string
  payer_account: string
  payee_account: string
  vendor_id: string
  vendor_name: string
  employee_id: string
  employee_name: string
  amount: number
  currency: string
  paid_at?: string
  status: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateFinancePaymentInput {
  payment_number?: string
  external_payment_id?: string
  payment_method?: string
  payer_account?: string
  payee_account?: string
  vendor_id?: string
  vendor_name?: string
  employee_id?: string
  employee_name?: string
  amount: number
  currency?: string
  paid_at?: string
  status?: string
  metadata?: Record<string, unknown>
}

export interface AllocateFinancePaymentInput {
  payable_id: string
  amount: number
  currency?: string
  metadata?: Record<string, unknown>
}

export interface FinancePaymentAllocation {
  id: string
  payment_id: string
  payable_id: string
  amount: number
  currency: string
  metadata: Record<string, unknown>
  created_at: string
}

export interface BusinessPartner {
  id: string
  master_key?: string
  partner_code: string
  partner_type: string
  name: string
  email: string
  phone: string
  status: string
  organization_id?: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateBusinessPartnerInput {
  partner_code?: string
  partner_type: string
  name: string
  email?: string
  phone?: string
  status?: string
  organization_id?: string
  metadata?: Record<string, unknown>
}

export interface InventoryItem {
  id: string
  master_key?: string
  item_code: string
  name: string
  item_type: string
  base_uom: string
  status: string
  organization_id?: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateInventoryItemInput {
  item_code?: string
  name: string
  item_type?: string
  base_uom?: string
  status?: string
  organization_id?: string
  metadata?: Record<string, unknown>
}

export interface Warehouse {
  id: string
  master_key?: string
  warehouse_code: string
  name: string
  status: string
  organization_id?: string
  department_id?: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateWarehouseInput {
  warehouse_code?: string
  name: string
  status?: string
  organization_id?: string
  department_id?: string
  metadata?: Record<string, unknown>
}

export interface InventoryBalance {
  id: string
  master_key?: string
  item_id: string
  warehouse_id: string
  location_id?: string
  quantity: number
  reserved_qty: number
  average_cost: number
  value_amount: number
  currency: string
  organization_id?: string
  metadata: Record<string, unknown>
  updated_at: string
}

export interface InventoryMovement {
  id: string
  master_key?: string
  movement_type: string
  source_type: string
  source_id?: string
  item_id: string
  warehouse_id: string
  location_id?: string
  quantity: number
  unit_cost: number
  amount: number
  currency: string
  balance_after: number
  organization_id?: string
  department_id?: string
  occurred_at: string
  metadata: Record<string, unknown>
  created_at: string
}

export interface CreateInventoryMovementInput {
  movement_type: string
  source_type?: string
  source_id?: string
  item_id: string
  warehouse_id: string
  location_id?: string
  quantity: number
  unit_cost?: number
  currency?: string
  organization_id?: string
  department_id?: string
  occurred_at?: string
  metadata?: Record<string, unknown>
}

export interface InventoryTransfer {
  id: string
  master_key?: string
  transfer_number: string
  from_warehouse_id: string
  to_warehouse_id: string
  status: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  metadata: Record<string, unknown>
  lines?: InventoryTransferLine[]
  created_at: string
  updated_at: string
}

export interface InventoryTransferLine {
  id: string
  transfer_id: string
  item_id: string
  quantity: number
  unit_cost: number
}

export interface CreateInventoryTransferInput {
  transfer_number?: string
  from_warehouse_id: string
  to_warehouse_id: string
  status?: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  metadata?: Record<string, unknown>
  lines?: Array<{
    item_id: string
    quantity: number
    unit_cost?: number
  }>
}

export interface InventoryAdjustment {
  id: string
  master_key?: string
  adjustment_number: string
  warehouse_id: string
  reason: string
  status: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  metadata: Record<string, unknown>
  lines?: InventoryAdjustmentLine[]
  created_at: string
  updated_at: string
}

export interface InventoryAdjustmentLine {
  id: string
  adjustment_id: string
  item_id: string
  quantity_delta: number
  unit_cost: number
}

export interface CreateInventoryAdjustmentInput {
  adjustment_number?: string
  warehouse_id: string
  reason?: string
  status?: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  metadata?: Record<string, unknown>
  lines?: Array<{
    item_id: string
    quantity_delta: number
    unit_cost?: number
  }>
}

export interface InventoryCount {
  id: string
  master_key?: string
  count_number: string
  warehouse_id: string
  status: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  metadata: Record<string, unknown>
  lines?: InventoryCountLine[]
  created_at: string
  updated_at: string
}

export interface InventoryCountLine {
  id: string
  count_id: string
  item_id: string
  book_qty: number
  counted_qty: number
  variance_qty: number
}

export interface CreateInventoryCountInput {
  count_number?: string
  warehouse_id: string
  status?: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  metadata?: Record<string, unknown>
  lines?: Array<{
    item_id: string
    book_qty?: number
    counted_qty: number
  }>
}

export interface PurchaseRequisition {
  id: string
  master_key?: string
  title: string
  supplier_id: string
  supplier_name: string
  status: string
  approval_status: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  currency: string
  total_amount: number
  metadata: Record<string, unknown>
  lines?: PurchaseRequisitionLine[]
  created_at: string
  updated_at: string
}

export interface PurchaseRequisitionLine {
  id: string
  requisition_id: string
  item_id: string
  warehouse_id: string
  quantity: number
  unit_cost: number
  amount: number
  metadata: Record<string, unknown>
}

export interface CreatePurchaseRequisitionInput {
  title: string
  supplier_id?: string
  supplier_name?: string
  status?: string
  approval_status?: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  currency?: string
  metadata?: Record<string, unknown>
  lines?: Array<{
    item_id: string
    warehouse_id: string
    quantity: number
    unit_cost: number
    metadata?: Record<string, unknown>
  }>
}

export interface PurchaseOrder {
  id: string
  master_key?: string
  order_number: string
  requisition_id?: string
  supplier_id: string
  supplier_name: string
  status: string
  approval_status: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  currency: string
  subtotal: number
  tax_amount: number
  total_amount: number
  metadata: Record<string, unknown>
  lines?: PurchaseOrderLine[]
  created_at: string
  updated_at: string
}

export interface PurchaseOrderLine {
  id: string
  order_id: string
  item_id: string
  warehouse_id: string
  quantity: number
  unit_cost: number
  tax_rate: number
  amount: number
  tax_amount: number
  total_amount: number
  metadata: Record<string, unknown>
}

export interface CreatePurchaseOrderInput {
  order_number?: string
  requisition_id?: string
  supplier_id?: string
  supplier_name?: string
  status?: string
  approval_status?: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  currency?: string
  metadata?: Record<string, unknown>
  lines?: Array<{
    item_id: string
    warehouse_id: string
    quantity: number
    unit_cost: number
    tax_rate?: number
    metadata?: Record<string, unknown>
  }>
}

export interface PurchaseReceipt {
  id: string
  master_key?: string
  receipt_number: string
  order_id?: string
  supplier_id: string
  supplier_name: string
  status: string
  approval_status: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  warehouse_id?: string
  payable_id?: string
  currency: string
  subtotal: number
  tax_amount: number
  total_amount: number
  metadata: Record<string, unknown>
  lines?: PurchaseReceiptLine[]
  created_at: string
  updated_at: string
}

export interface PurchaseReceiptLine {
  id: string
  receipt_id: string
  order_line_id?: string
  item_id: string
  warehouse_id: string
  location_id?: string
  quantity: number
  unit_cost: number
  tax_rate: number
  amount: number
  tax_amount: number
  total_amount: number
  metadata: Record<string, unknown>
}

export interface CreatePurchaseReceiptInput {
  receipt_number?: string
  order_id?: string
  supplier_id?: string
  supplier_name?: string
  status?: string
  approval_status?: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  warehouse_id?: string
  currency?: string
  metadata?: Record<string, unknown>
  lines?: Array<{
    order_line_id?: string
    item_id: string
    warehouse_id: string
    location_id?: string
    quantity: number
    unit_cost: number
    tax_rate?: number
    metadata?: Record<string, unknown>
  }>
}

export interface PurchaseReturn {
  id: string
  master_key?: string
  return_number: string
  receipt_id?: string
  supplier_id: string
  supplier_name: string
  status: string
  approval_status: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  currency: string
  subtotal: number
  tax_amount: number
  total_amount: number
  metadata: Record<string, unknown>
  lines?: PurchaseReturnLine[]
  created_at: string
  updated_at: string
}

export interface PurchaseReturnLine {
  id: string
  return_id: string
  item_id: string
  warehouse_id: string
  location_id?: string
  quantity: number
  unit_cost: number
  tax_amount: number
  amount: number
  total_amount: number
  metadata: Record<string, unknown>
}

export interface CreatePurchaseReturnInput {
  return_number?: string
  receipt_id?: string
  supplier_id?: string
  supplier_name?: string
  status?: string
  approval_status?: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  currency?: string
  metadata?: Record<string, unknown>
  lines?: Array<{
    item_id: string
    warehouse_id: string
    location_id?: string
    quantity: number
    unit_cost: number
    tax_rate?: number
    metadata?: Record<string, unknown>
  }>
}

export interface SalesQuotation {
  id: string
  master_key?: string
  quotation_number: string
  customer_id: string
  customer_name: string
  status: string
  approval_status: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  currency: string
  subtotal: number
  tax_amount: number
  total_amount: number
  metadata: Record<string, unknown>
  lines?: SalesQuotationLine[]
  created_at: string
  updated_at: string
}

export interface SalesQuotationLine {
  id: string
  quotation_id: string
  item_id: string
  warehouse_id: string
  quantity: number
  unit_price: number
  tax_rate: number
  amount: number
  tax_amount: number
  total_amount: number
  metadata: Record<string, unknown>
}

export interface CreateSalesQuotationInput {
  quotation_number?: string
  customer_id?: string
  customer_name?: string
  status?: string
  approval_status?: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  currency?: string
  metadata?: Record<string, unknown>
  lines?: Array<{
    item_id: string
    warehouse_id: string
    quantity: number
    unit_price: number
    tax_rate?: number
    metadata?: Record<string, unknown>
  }>
}

export interface SalesOrder {
  id: string
  master_key?: string
  order_number: string
  quotation_id?: string
  customer_id: string
  customer_name: string
  status: string
  approval_status: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  currency: string
  subtotal: number
  tax_amount: number
  total_amount: number
  metadata: Record<string, unknown>
  lines?: SalesOrderLine[]
  created_at: string
  updated_at: string
}

export interface SalesOrderLine {
  id: string
  order_id: string
  item_id: string
  warehouse_id: string
  quantity: number
  unit_price: number
  tax_rate: number
  amount: number
  tax_amount: number
  total_amount: number
  metadata: Record<string, unknown>
}

export interface CreateSalesOrderInput {
  order_number?: string
  quotation_id?: string
  customer_id?: string
  customer_name?: string
  status?: string
  approval_status?: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  currency?: string
  metadata?: Record<string, unknown>
  lines?: Array<{
    item_id: string
    warehouse_id: string
    quantity: number
    unit_price: number
    tax_rate?: number
    metadata?: Record<string, unknown>
  }>
}

export interface SalesShipment {
  id: string
  master_key?: string
  shipment_number: string
  order_id?: string
  customer_id: string
  customer_name: string
  status: string
  approval_status: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  receivable_id?: string
  currency: string
  subtotal: number
  tax_amount: number
  total_amount: number
  metadata: Record<string, unknown>
  lines?: SalesShipmentLine[]
  created_at: string
  updated_at: string
}

export interface SalesShipmentLine {
  id: string
  shipment_id: string
  order_line_id?: string
  item_id: string
  warehouse_id: string
  location_id?: string
  quantity: number
  unit_price: number
  tax_rate: number
  amount: number
  tax_amount: number
  total_amount: number
  metadata: Record<string, unknown>
}

export interface CreateSalesShipmentInput {
  shipment_number?: string
  order_id?: string
  customer_id?: string
  customer_name?: string
  status?: string
  approval_status?: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  currency?: string
  metadata?: Record<string, unknown>
  lines?: Array<{
    order_line_id?: string
    item_id: string
    warehouse_id: string
    location_id?: string
    quantity: number
    unit_price: number
    tax_rate?: number
    metadata?: Record<string, unknown>
  }>
}

export interface SalesReturn {
  id: string
  master_key?: string
  return_number: string
  shipment_id?: string
  customer_id: string
  customer_name: string
  status: string
  approval_status: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  currency: string
  subtotal: number
  tax_amount: number
  total_amount: number
  metadata: Record<string, unknown>
  lines?: SalesReturnLine[]
  created_at: string
  updated_at: string
}

export interface SalesReturnLine {
  id: string
  return_id: string
  item_id: string
  warehouse_id: string
  location_id?: string
  quantity: number
  unit_price: number
  tax_amount: number
  amount: number
  total_amount: number
  metadata: Record<string, unknown>
}

export interface CreateSalesReturnInput {
  return_number?: string
  shipment_id?: string
  customer_id?: string
  customer_name?: string
  status?: string
  approval_status?: string
  organization_id?: string
  department_id?: string
  workflow_instance_id?: string
  currency?: string
  metadata?: Record<string, unknown>
  lines?: Array<{
    item_id: string
    warehouse_id: string
    location_id?: string
    quantity: number
    unit_price: number
    tax_rate?: number
    metadata?: Record<string, unknown>
  }>
}

export interface Currency {
  code: string
  name: string
  currency_type: string
  symbol: string
  precision_digits: number
  chain_id?: string
  contract_address?: string
  external_source?: string
  is_active: boolean
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateCurrencyInput {
  code: string
  name?: string
  currency_type?: string
  symbol?: string
  precision_digits?: number
  chain_id?: string
  contract_address?: string
  external_source?: string
  is_active?: boolean
  metadata?: Record<string, unknown>
}

export interface ExchangeRateVersion {
  id: string
  from_currency: string
  to_currency: string
  rate: number
  source: string
  provider?: string
  external_rate_id?: string
  effective_from: string
  effective_to?: string
  metadata: Record<string, unknown>
  created_at: string
}

export interface CreateExchangeRateInput {
  from_currency: string
  to_currency: string
  rate: number
  source?: string
  provider?: string
  external_rate_id?: string
  effective_from?: string
  effective_to?: string
  metadata?: Record<string, unknown>
}

export interface ConvertCostInput {
  amount: number
  from_currency: string
  to_currency: string
  at?: string
}

export interface ConversionResult {
  amount: number
  from_currency: string
  to_currency: string
  converted_amount: number
  rate: number
  exchange_rate_version_id?: string
}

export interface CostRateCard {
  id: string
  subject_type: string
  subject_id?: string
  scope_type?: string
  scope_id?: string
  rate_type: string
  amount: number
  currency: string
  base_amount: number
  base_currency: string
  exchange_rate_version_id?: string
  effective_from: string
  effective_to?: string
  status: string
  metadata: Record<string, unknown>
  created_at: string
}

export interface CreateCostRateCardInput {
  subject_type: string
  subject_id?: string
  scope_type?: string
  scope_id?: string
  rate_type?: string
  amount: number
  currency?: string
  effective_from?: string
  effective_to?: string
  status?: string
  metadata?: Record<string, unknown>
}

export interface CostBudget {
  id: string
  scope_type: string
  scope_id?: string
  amount: number
  currency: string
  base_amount: number
  base_currency: string
  exchange_rate_version_id?: string
  period_start?: string
  period_end?: string
  status: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateCostBudgetInput {
  scope_type: string
  scope_id?: string
  amount: number
  currency?: string
  period_start?: string
  period_end?: string
  status?: string
  metadata?: Record<string, unknown>
}

export interface CostLedgerEntry {
  id: string
  ledger_type: string
  cost_category: string
  source_type: string
  source_id?: string
  organization_id?: string
  department_id?: string
  requirement_id?: string
  project_id?: string
  workflow_id?: string
  task_id?: string
  capability_id?: string
  actor_id?: string
  actor_type?: string
  resource_type?: string
  amount: number
  currency: string
  base_amount: number
  base_currency: string
  exchange_rate_version_id?: string
  occurred_at: string
  status: string
  finance_export_line_id?: string
  description: string
  metadata: Record<string, unknown>
  created_at: string
}

export interface CreateCostLedgerEntryInput {
  ledger_type?: string
  cost_category?: string
  source_type?: string
  source_id?: string
  organization_id?: string
  department_id?: string
  requirement_id?: string
  project_id?: string
  workflow_id?: string
  task_id?: string
  capability_id?: string
  actor_id?: string
  actor_type?: string
  resource_type?: string
  amount: number
  currency?: string
  occurred_at?: string
  status?: string
  description?: string
  metadata?: Record<string, unknown>
}

export interface CostSummary {
  scope_type?: string
  scope_id?: string
  currency: string
  total_amount: number
  budget_amount: number
  budget_variance: number
  entry_count: number
  by_category: Record<string, number>
  by_source: Record<string, number>
  by_currency: Record<string, number>
  recent_entries: CostLedgerEntry[]
  metadata?: Record<string, unknown>
}
