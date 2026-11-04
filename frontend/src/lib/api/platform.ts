import { AIInvocation, AssistantBusinessSkill, AssistantContextHealthSummary, AssistantContextPackageDiagnostic, AssistantContextTarget, AssistantProposal, AssistantSession, AssistantSkillRun, CreateAssistantBusinessSkillInput, CreateAssistantSessionInput, CreateDatabaseMaintenanceJobInput, CreateIndustrySolutionChangeRequestInput, CreateIndustrySolutionTableFieldChangeInput, CreatePlatformFeatureInput, CreatePlatformUserInput, CreatePlatformUserResponse, DatabaseMaintenanceJob, InboxItem, IndustryPackage, IndustryPublicationRequest, IndustrySolutionApplyJob, IndustrySolutionAssetDiff, IndustrySolutionChangeRequest, IndustrySolutionManifest, IndustrySolutionVerificationReport, MetaOrgOverview, ModelCatalogItem, ModelProvider, MonitoringAgentRun, MonitoringAgentStatus, OrganizationIndustryAdoption, OrganizationIndustrySolutionTarget, OrganizationUserAccount, PlatformDetail, PlatformFeature, PlatformMaster, PlatformPermission, PlatformPermissionProfile, PlatformRole, PlatformUser, ResetOrganizationAccountPasswordResponse, ResetPlatformUserPasswordResponse, SampleTenantResponse, SessionOrganization, ToolApprovalReviewResult, UpdateIndustryPackageInput, UpdateOrganizationAccountInput } from './types'
import { apiRequest } from './core'
import type { ApiOperation } from '@/lib/operations'
export async function listIndustryPackages(token: string, industryKey: string, limit = 100): Promise<IndustryPackage[]> {
  const query = limit > 0 ? `?limit=${encodeURIComponent(String(limit))}` : ''
  return apiRequest<IndustryPackage[]>(`/platform/admin/industries/${encodeURIComponent(industryKey)}/packages${query}`, { token })
}

export async function updateIndustryPackage(
  token: string,
  packageID: string,
  input: UpdateIndustryPackageInput,
): Promise<IndustryPackage> {
  return apiRequest<IndustryPackage>(`/platform/admin/industry-packages/${encodeURIComponent(packageID)}`, {
    method: 'PATCH',
    token,
    body: input,
  })
}

export async function deleteIndustryPackage(token: string, packageID: string): Promise<IndustryPackage> {
  return apiRequest<IndustryPackage>(`/platform/admin/industry-packages/${encodeURIComponent(packageID)}`, {
    method: 'DELETE',
    token,
  })
}

export async function applyIndustryPackageToOrganization(
  token: string,
  packageID: string,
  organizationID: string,
  moduleKeys: string[],
): Promise<OrganizationIndustryAdoption> {
  return apiRequest<OrganizationIndustryAdoption>(
    `/platform/admin/industry-packages/${encodeURIComponent(packageID)}/apply-to-organization/${encodeURIComponent(organizationID)}`,
    {
      method: 'POST',
      token,
      body: { module_keys: moduleKeys },
    },
  )
}

export async function listIndustryPublicationRequests(token: string, limit = 100): Promise<IndustryPublicationRequest[]> {
  const query = limit > 0 ? `?limit=${encodeURIComponent(String(limit))}` : ''
  return apiRequest<IndustryPublicationRequest[]>(`/platform/admin/industry-publication-requests${query}`, { token })
}

export async function reviewIndustryPublicationRequest(
  token: string,
  requestID: string,
  action: 'approve' | 'reject',
  reason = '',
): Promise<IndustryPublicationRequest> {
  return apiRequest<IndustryPublicationRequest>(
    `/platform/admin/industry-publication-requests/${encodeURIComponent(requestID)}/${action}`,
    {
      method: 'POST',
      token,
      body: { reason },
    },
  )
}

export async function listPlatformRuntimeOperations(token: string): Promise<ApiOperation[]> {
  return apiRequest<ApiOperation[]>('/platform/admin/runtime/operations', { token })
}

export async function listPlatformOrganizations(token: string, limit = 100): Promise<SessionOrganization[]> {
  const query = limit > 0 ? `?limit=${encodeURIComponent(String(limit))}` : ''
  return apiRequest<SessionOrganization[]>(`/platform/organizations${query}`, { token })
}

export async function createBusinessClosureSampleTenant(token: string): Promise<SampleTenantResponse> {
  return apiRequest<SampleTenantResponse>('/platform/admin/sample-tenants/business-closure', {
    method: 'POST',
    token,
  })
}

export async function getPlatformPermissionProfile(token: string): Promise<PlatformPermissionProfile> {
  return apiRequest<PlatformPermissionProfile>('/platform/admin/me/permissions', { token })
}

export async function listPlatformFeatures(token: string, status = '', limit = 200): Promise<PlatformFeature[]> {
  const params = new URLSearchParams()
  if (status) params.set('status', status)
  if (limit > 0) params.set('limit', String(limit))
  const query = params.toString() ? `?${params.toString()}` : ''
  return apiRequest<PlatformFeature[]>(`/platform/admin/features${query}`, { token })
}

export async function createPlatformFeature(token: string, input: CreatePlatformFeatureInput): Promise<PlatformFeature> {
  return apiRequest<PlatformFeature>('/platform/admin/features', { method: 'POST', token, body: input })
}

export async function publishPlatformFeature(token: string, featureKey: string): Promise<PlatformFeature> {
  return apiRequest<PlatformFeature>(`/platform/admin/features/${encodeURIComponent(featureKey)}/publish`, { method: 'POST', token })
}

export async function listPlatformPermissions(token: string): Promise<PlatformPermission[]> {
  return apiRequest<PlatformPermission[]>('/platform/admin/permissions', { token })
}

export async function listPlatformRoles(token: string): Promise<PlatformRole[]> {
  return apiRequest<PlatformRole[]>('/platform/admin/roles', { token })
}

export async function setPlatformRolePermissions(token: string, roleKey: string, permissionKeys: string[]): Promise<PlatformRole> {
  return apiRequest<PlatformRole>(`/platform/admin/roles/${encodeURIComponent(roleKey)}/permissions`, {
    method: 'PUT',
    token,
    body: { permission_keys: permissionKeys },
  })
}

export async function listPlatformUsers(token: string, limit = 100): Promise<PlatformUser[]> {
  const query = limit > 0 ? `?limit=${encodeURIComponent(String(limit))}` : ''
  return apiRequest<PlatformUser[]>(`/platform/admin/users${query}`, { token })
}

export async function createPlatformUser(token: string, input: CreatePlatformUserInput): Promise<CreatePlatformUserResponse> {
  return apiRequest<CreatePlatformUserResponse>('/platform/admin/users', { method: 'POST', token, body: input })
}

export async function setPlatformUserRoles(token: string, userID: string, roles: string[]): Promise<PlatformUser> {
  return apiRequest<PlatformUser>(`/platform/admin/users/${encodeURIComponent(userID)}/roles`, {
    method: 'PUT',
    token,
    body: { roles },
  })
}

export async function resetPlatformUserPassword(token: string, userID: string): Promise<ResetPlatformUserPasswordResponse> {
  return apiRequest<ResetPlatformUserPasswordResponse>(`/platform/admin/users/${encodeURIComponent(userID)}/reset-password`, {
    method: 'POST',
    token,
  })
}

export async function disablePlatformUser(token: string, userID: string): Promise<PlatformUser> {
  return apiRequest<PlatformUser>(`/platform/admin/users/${encodeURIComponent(userID)}/disable`, { method: 'POST', token })
}

export async function listDatabaseMaintenanceJobs(token: string, limit = 100): Promise<DatabaseMaintenanceJob[]> {
  const query = limit > 0 ? `?limit=${encodeURIComponent(String(limit))}` : ''
  return apiRequest<DatabaseMaintenanceJob[]>(`/platform/admin/database-maintenance/jobs${query}`, { token })
}

export async function createDatabaseMaintenanceJob(token: string, input: CreateDatabaseMaintenanceJobInput): Promise<DatabaseMaintenanceJob> {
  return apiRequest<DatabaseMaintenanceJob>('/platform/admin/database-maintenance/jobs', { method: 'POST', token, body: input })
}

export async function createPrivateDeploymentExport(token: string, organizationID: string): Promise<DatabaseMaintenanceJob> {
  return apiRequest<DatabaseMaintenanceJob>(
    `/platform/admin/organizations/${encodeURIComponent(organizationID)}/private-deployment-exports`,
    { method: 'POST', token },
  )
}

export async function reviewDatabaseMaintenanceJob(
  token: string,
  jobID: string,
  decision: 'approve' | 'reject',
  reason = '',
): Promise<DatabaseMaintenanceJob> {
  return apiRequest<DatabaseMaintenanceJob>(`/platform/admin/database-maintenance/jobs/${encodeURIComponent(jobID)}/${decision}`, {
    method: 'POST',
    token,
    body: { reason },
  })
}

export async function closePlatformOrganization(token: string, organizationID: string, reason = ''): Promise<SessionOrganization> {
  return apiRequest<SessionOrganization>(`/platform/admin/organizations/${encodeURIComponent(organizationID)}/close`, {
    method: 'POST',
    token,
    body: { reason },
  })
}

export async function updatePlatformOrganizationProfile(
  token: string,
  organizationID: string,
  input: { name: string; description?: string },
): Promise<SessionOrganization> {
  return apiRequest<SessionOrganization>(`/platform/admin/organizations/${encodeURIComponent(organizationID)}`, {
    method: 'PATCH',
    token,
    body: input,
  })
}

export async function listOrganizationAccounts(token: string, organizationID: string, limit = 100): Promise<OrganizationUserAccount[]> {
  const query = limit > 0 ? `?limit=${encodeURIComponent(String(limit))}` : ''
  return apiRequest<OrganizationUserAccount[]>(`/platform/admin/organizations/${encodeURIComponent(organizationID)}/accounts${query}`, { token })
}

export async function updateOrganizationAccount(
  token: string,
  organizationID: string,
  userID: string,
  input: UpdateOrganizationAccountInput,
): Promise<OrganizationUserAccount> {
  return apiRequest<OrganizationUserAccount>(
    `/platform/admin/organizations/${encodeURIComponent(organizationID)}/accounts/${encodeURIComponent(userID)}`,
    { method: 'PATCH', token, body: input },
  )
}

export async function resetOrganizationAccountPassword(
  token: string,
  organizationID: string,
  userID: string,
  password?: string,
): Promise<ResetOrganizationAccountPasswordResponse> {
  return apiRequest<ResetOrganizationAccountPasswordResponse>(
    `/platform/admin/organizations/${encodeURIComponent(organizationID)}/accounts/${encodeURIComponent(userID)}/reset-password`,
    { method: 'POST', token, body: password ? { password } : {} },
  )
}

export async function listPlatformMasters(token: string, moduleKey: string, limit = 100): Promise<PlatformMaster[]> {
  const query = limit > 0 ? `?limit=${encodeURIComponent(String(limit))}` : ''
  return apiRequest<PlatformMaster[]>(`/platform/admin/modules/${encodeURIComponent(moduleKey)}/masters${query}`, { token })
}

export async function listPlatformDetails(token: string, masterKey: string): Promise<PlatformDetail[]> {
  return apiRequest<PlatformDetail[]>(`/platform/admin/masters/${encodeURIComponent(masterKey)}/details`, { token })
}

export async function listOrganizationIndustrySolutionTargets(token: string, limit = 100): Promise<OrganizationIndustrySolutionTarget[]> {
  const query = limit > 0 ? `?limit=${encodeURIComponent(String(limit))}` : ''
  return apiRequest<OrganizationIndustrySolutionTarget[]>(`/platform/admin/industry-solution-targets${query}`, { token })
}

export async function exportOrganizationIndustrySolutionManifest(token: string, organizationID: string): Promise<IndustrySolutionManifest> {
  return apiRequest<IndustrySolutionManifest>(`/platform/admin/organizations/${encodeURIComponent(organizationID)}/industry-solution-manifest/export`, { token })
}

export async function createIndustrySolutionChangeRequest(
  token: string,
  organizationID: string,
  input: CreateIndustrySolutionChangeRequestInput,
): Promise<IndustrySolutionChangeRequest> {
  return apiRequest<IndustrySolutionChangeRequest>(`/platform/admin/organizations/${encodeURIComponent(organizationID)}/industry-solution-change-requests`, {
    method: 'POST',
    token,
    body: input,
  })
}

export async function createERPStandardSolutionFlow(
  token: string,
  organizationID: string,
  input: {
    industry_key: string
    package_key: string
    name: string
    enabled_modules: string[]
  },
): Promise<IndustrySolutionChangeRequest> {
  return apiRequest<IndustrySolutionChangeRequest>(
    `/platform/admin/organizations/${encodeURIComponent(organizationID)}/industry-solution-flows/erp-standard`,
    {
      method: 'POST',
      token,
      body: input,
    },
  )
}

export async function createRetailDistributionSolutionFlow(token: string, organizationID: string): Promise<IndustrySolutionChangeRequest> {
  return apiRequest<IndustrySolutionChangeRequest>(
    `/platform/admin/organizations/${encodeURIComponent(organizationID)}/industry-solution-flows/retail-distribution`,
    {
      method: 'POST',
      token,
    },
  )
}

export async function createIndustrySolutionTableFieldChange(
  token: string,
  organizationID: string,
  input: CreateIndustrySolutionTableFieldChangeInput,
): Promise<IndustrySolutionChangeRequest> {
  return apiRequest<IndustrySolutionChangeRequest>(
    `/platform/admin/organizations/${encodeURIComponent(organizationID)}/industry-solution-table-fields/change-requests`,
    {
      method: 'POST',
      token,
      body: input,
    },
  )
}

export async function approveIndustrySolutionChange(token: string, requestID: string, reason = ''): Promise<IndustrySolutionChangeRequest> {
  return apiRequest<IndustrySolutionChangeRequest>(`/platform/admin/industry-solution-change-requests/${encodeURIComponent(requestID)}/approve`, {
    method: 'POST',
    token,
    body: { reason },
  })
}

export async function verifyIndustrySolutionChange(token: string, requestID: string): Promise<IndustrySolutionVerificationReport> {
  return apiRequest<IndustrySolutionVerificationReport>(`/platform/admin/industry-solution-change-requests/${encodeURIComponent(requestID)}/verify`, {
    method: 'POST',
    token,
  })
}

export async function getIndustrySolutionChangeAssetDiff(token: string, requestID: string): Promise<IndustrySolutionAssetDiff[]> {
  const result = await apiRequest<{ diff: IndustrySolutionAssetDiff[] }>(
    `/platform/admin/industry-solution-change-requests/${encodeURIComponent(requestID)}/asset-diff`,
    { token },
  )
  return result.diff
}

export async function applyIndustrySolutionChange(token: string, requestID: string): Promise<IndustrySolutionApplyJob> {
  return apiRequest<IndustrySolutionApplyJob>(`/platform/admin/industry-solution-change-requests/${encodeURIComponent(requestID)}/apply`, {
    method: 'POST',
    token,
  })
}

export async function getPlatformMetaOrgOverview(token: string): Promise<MetaOrgOverview> {
  return apiRequest<MetaOrgOverview>('/platform/admin/meta-org/overview', { token })
}

export async function getPlatformMetaOrgInbox(token: string): Promise<InboxItem[]> {
  return apiRequest<InboxItem[]>('/platform/admin/meta-org/inbox', { token })
}

export async function listPlatformModelProviders(token: string): Promise<ModelProvider[]> {
  return apiRequest<ModelProvider[]>('/platform/admin/model-providers', { token })
}

export async function listPlatformModels(token: string): Promise<ModelCatalogItem[]> {
  return apiRequest<ModelCatalogItem[]>('/platform/admin/models', { token })
}

export async function approvePlatformToolApproval(token: string, id: string, reason = 'approved from platform assistant'): Promise<ToolApprovalReviewResult> {
  return apiRequest<ToolApprovalReviewResult>(`/platform/admin/tool-approvals/${id}/approve`, { method: 'POST', token, body: { reason } })
}

export async function rejectPlatformToolApproval(token: string, id: string, reason = 'rejected from platform assistant'): Promise<ToolApprovalReviewResult> {
  return apiRequest<ToolApprovalReviewResult>(`/platform/admin/tool-approvals/${id}/reject`, { method: 'POST', token, body: { reason } })
}

export async function getPlatformAIInvocation(token: string, id: string): Promise<AIInvocation> {
  return apiRequest<AIInvocation>(`/platform/admin/ai-gateway/invocations/${id}`, { token })
}

export async function createPlatformAssistantSession(token: string, input: CreateAssistantSessionInput): Promise<AssistantSession> {
  return apiRequest<AssistantSession>('/platform/admin/assistant/sessions', { method: 'POST', token, body: input })
}

export async function listPlatformAssistantContextTargets(token: string, moduleKey: string, targetType?: string): Promise<AssistantContextTarget[]> {
  const params = new URLSearchParams()
  if (moduleKey) params.set('module_key', moduleKey)
  if (targetType) params.set('target_type', targetType)
  const query = params.toString() ? `?${params.toString()}` : ''
  return apiRequest<AssistantContextTarget[]>(`/platform/admin/assistant/context-targets${query}`, { token })
}

export async function getPlatformAssistantContextPackageDiagnostic(token: string, packageID: string): Promise<AssistantContextPackageDiagnostic> {
  return apiRequest<AssistantContextPackageDiagnostic>(`/platform/admin/assistant/context-packages/${packageID}`, { token })
}

export async function getPlatformAssistantContextHealth(token: string, organizationID?: string): Promise<AssistantContextHealthSummary> {
  const query = organizationID ? `?organization_id=${encodeURIComponent(organizationID)}` : ''
  return apiRequest<AssistantContextHealthSummary>(`/platform/admin/assistant/context-health${query}`, { token })
}

export async function getMonitoringAgentStatus(token: string, organizationID?: string): Promise<MonitoringAgentStatus> {
  const query = organizationID ? `?organization_id=${encodeURIComponent(organizationID)}` : ''
  return apiRequest<MonitoringAgentStatus>(`/platform/admin/monitoring-agent/status${query}`, { token })
}

export async function runMonitoringAgent(
  token: string,
  input: { organization_id?: string; lookback_hours?: number } = {},
): Promise<MonitoringAgentRun> {
  return apiRequest<MonitoringAgentRun>('/platform/admin/monitoring-agent/runs', {
    method: 'POST',
    token,
    body: input,
  })
}

export async function listMonitoringAgentRuns(token: string, organizationID?: string, limit = 20): Promise<MonitoringAgentRun[]> {
  const params = new URLSearchParams({ limit: String(limit) })
  if (organizationID) params.set('organization_id', organizationID)
  const result = await apiRequest<{ runs: MonitoringAgentRun[] }>(`/platform/admin/monitoring-agent/runs?${params.toString()}`, { token })
  return result.runs
}

export async function listPlatformAssistantProposals(token: string, sessionID: string): Promise<AssistantProposal[]> {
  return apiRequest<AssistantProposal[]>(`/platform/admin/assistant/sessions/${sessionID}/proposals`, { token })
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

export async function listPlatformAssistantSkills(token: string, moduleKey?: string, targetType?: string): Promise<AssistantBusinessSkill[]> {
  const params = new URLSearchParams()
  if (moduleKey) params.set('module_key', moduleKey)
  if (targetType) params.set('target_type', targetType)
  const query = params.toString() ? `?${params.toString()}` : ''
  return apiRequest<AssistantBusinessSkill[]>(`/platform/admin/assistant/skills${query}`, { token })
}

export async function createPlatformAssistantSkill(
  token: string,
  input: CreateAssistantBusinessSkillInput,
): Promise<AssistantBusinessSkill> {
  return apiRequest<AssistantBusinessSkill>('/platform/admin/assistant/skills', { method: 'POST', token, body: input })
}

export async function activatePlatformAssistantSkill(token: string, skillID: string): Promise<AssistantBusinessSkill> {
  return apiRequest<AssistantBusinessSkill>(`/platform/admin/assistant/skills/${skillID}/activate`, { method: 'POST', token })
}

export async function runPlatformAssistantSkill(
  token: string,
  skillID: string,
  input: Record<string, unknown>,
): Promise<AssistantSkillRun> {
  return apiRequest<AssistantSkillRun>(`/platform/admin/assistant/skills/${skillID}/run`, { method: 'POST', token, body: input })
}
