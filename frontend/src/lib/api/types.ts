import type { SessionScope } from '@/lib/auth'
export interface RequestOptions {
  method?: string
  body?: unknown
  token?: string
  organizationId?: string | null
  scope?: SessionScope
  responseType?: 'json' | 'blob'
}

export type ERPRecord<T = Record<string, unknown>> = {
  table_code?: string
  parent_table_code?: string
  parent_key?: string
  key: string
  data: T
  created_at?: string
  updated_at?: string
}

export type ERPActionResult<T = Record<string, unknown>> = {
  table_code: string
  key: string
  action: string
  status: string
  record?: ERPRecord<T>
  generated_records?: ERPRecord[]
  effects?: Record<string, unknown>
  execution_id?: string
  idempotency_key?: string
  preconditions_checked?: Array<{ key: string; status: string; message?: string }>
  provenance?: Record<string, unknown>
  failure_reason?: { code: string; message: string }
}

export type ERPActionGeneratedRecord = {
  action_id: string
  line_num: number
  generated_table_code: string
  generated_key: string
  relation_type: string
  payload?: Record<string, unknown>
}

export type ERPActionExecution = {
  id: string
  table_code: string
  record_key: string
  action: string
  status: string
  idempotency_key: string
  actor_id?: string
  actor_type?: string
  tool_execution_id?: string
  assistant_session_id?: string
  source?: string
  failure_code?: string
  failure_message?: string
  payload: Record<string, unknown>
  generated_records?: ERPActionGeneratedRecord[]
  started_at?: string
  completed_at?: string
}

// ---------------------------------------------------------------------------
// SMT MES shop floor (workstation, pass station, boards, trace, reports, edges).
// ---------------------------------------------------------------------------
export interface SMTMachineContext {
  machine_code: string
  line_code: string
  work_order: string
  product_code: string
  verified_at: string
  verified_by: string
  signed_off_at: string
  signed_off_by: string
  signoff_required: boolean
  ready: boolean
  awaiting_signoff: boolean
  slots: Array<Record<string, unknown>>
  summary: Record<string, number>
}

export interface SMTEdgeInstance {
  instance_key: string
  organization_id: string
  name: string
  base_url: string
  api_key_masked?: string
  subscribed_events: string
  status: string
  auto_push: boolean
  last_poll_id: number
  last_sync_at?: string
  last_event_at?: string
  webhook_path?: string
}

export interface SMTEdgeEvent {
  id: number
  instance_key: string
  event_id: number
  event_type: string
  ref: string
  status: string
  error?: string
  received_at: string
  processed_at?: string
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

export interface SampleTenantResponse {
  organization: SessionOrganization
  owner_user_id: string
  owner_email: string
  enabled_modules: string[]
  tenant_database_status: string
  tenant_database_name?: string
  industry_solution_package: string
}

export interface PlatformPermissionProfile {
  role: string
  permissions: Record<string, boolean>
  menu_items: string[]
}

export interface PlatformFeature {
  feature_key: string
  parent_key?: string
  module_key: string
  category: string
  title: string
  description?: string
  status: string
  sort_order: number
  permission_keys: string[]
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreatePlatformFeatureInput {
  feature_key: string
  parent_key?: string
  module_key: string
  category?: string
  title: string
  description?: string
  sort_order?: number
  permission_keys?: string[]
  metadata?: Record<string, unknown>
}

export interface PlatformPermission {
  permission_key: string
  name: string
  description?: string
  category: string
  status: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface PlatformRole {
  role_key: string
  name: string
  description?: string
  status: string
  is_system: boolean
  permissions?: string[]
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface PlatformUser {
  user_id: string
  name: string
  email: string
  account_status: string
  roles: string[]
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreatePlatformUserInput {
  name: string
  email: string
  roles?: string[]
  metadata?: Record<string, unknown>
}

export interface CreatePlatformUserResponse {
  user: PlatformUser
  temporary_password: string
}

export interface ResetPlatformUserPasswordResponse {
  user_id: string
  temporary_password: string
}

export interface OrganizationUserAccount {
  user_id: string
  organization_id: string
  membership_id?: string
  name: string
  email: string
  account_status: string
  authority_tier: string
  is_owner: boolean
  member_status: string
  updated_at?: string
}

export interface UpdateOrganizationAccountInput {
  name?: string
  email?: string
  account_status?: string
  authority_tier?: string
}

export interface ResetOrganizationAccountPasswordResponse {
  user_id: string
  temporary_password: string
}

export interface DatabaseMaintenanceJob {
  id: string
  job_type: 'backup' | 'restore' | string
  scope: string
  status: string
  reason?: string
  backup_ref?: string
  requested_by?: string
  reviewed_by?: string
  review_reason?: string
  result?: Record<string, unknown>
  metadata: Record<string, unknown>
  created_at: string
  reviewed_at?: string
  completed_at?: string
  updated_at: string
}

export interface CreateDatabaseMaintenanceJobInput {
  job_type: 'backup' | 'restore'
  scope?: string
  reason?: string
  backup_ref?: string
  metadata?: Record<string, unknown>
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

export interface Industry {
  industry_key: string
  name: string
  description?: string
  status: string
  metadata: Record<string, unknown>
  created_by?: string
  created_at: string
  updated_at: string
}

export interface IndustryPackageAsset {
  id?: string
  package_id?: string
  asset_key: string
  asset_type: string
  payload: Record<string, unknown>
  metadata?: Record<string, unknown>
}

export interface IndustryPackage {
  id: string
  industry_key: string
  package_key: string
  version: number
  name: string
  description?: string
  status: string
  assets: IndustryPackageAsset[]
  metadata: Record<string, unknown>
  created_by?: string
  created_at: string
  updated_at: string
}

export interface UpdateIndustryPackageInput {
  name?: string
  description?: string
  status?: string
  assets?: IndustryPackageAsset[]
  metadata?: Record<string, unknown>
}

export interface OrganizationIndustryAdoption {
  organization_id: string
  industry_key: string
  package_id: string
  primary: boolean
  enabled_modules: string[]
  status: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface IndustryExtension {
  id: string
  organization_id: string
  industry_key: string
  package_id?: string
  extension_key: string
  name: string
  description?: string
  status: string
  assets: IndustryPackageAsset[]
  metadata: Record<string, unknown>
  created_by?: string
  created_at: string
  updated_at: string
}

export interface CreateIndustryExtensionInput {
  organization_id: string
  industry_key: string
  package_id?: string
  extension_key: string
  name: string
  description?: string
  assets: IndustryPackageAsset[]
  metadata?: Record<string, unknown>
}

export interface IndustryPublicationRequest {
  id: string
  extension_id: string
  source_organization_id: string
  industry_key: string
  status: string
  reason?: string
  review_reason?: string
  requested_by?: string
  reviewed_by?: string
  metadata: {
    publication_gates?: PublicationGateResult[]
    [key: string]: unknown
  }
  created_at: string
  updated_at: string
  reviewed_at?: string
}

export interface PublicationGateResult {
  key: string
  status: string
  message: string
  metadata?: Record<string, unknown>
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

export interface OrganizationIndustrySolutionTarget {
  organization_id: string
  target_schema_name: string
  template_version: string
  status: string
  last_change_request_id?: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface IndustrySolutionManifest {
  format_version: string
  module_key: string
  tables: IndustrySolutionTableDefinition[]
  metadata?: {
    industry_manifest?: IndustrySolutionAssetManifest
    package_diff?: IndustrySolutionAssetDiff[]
    [key: string]: unknown
  }
}

export interface IndustrySolutionAssetManifest {
  manifest_version: string
  industry_key: string
  package_key: string
  package_version: string
  assets: IndustrySolutionAsset[]
  dependencies?: string[]
  quality_gates?: string[]
  verification_scenarios?: string[]
}

export interface IndustrySolutionAsset {
  asset_key: string
  asset_type: string
  version: string
  source: string
  owner: string
  risk_level: string
  depends_on?: string[]
  payload: Record<string, unknown>
}

export interface IndustrySolutionAssetDiff {
  asset_type: string
  asset_key: string
  action: string
  risk_level: string
  current_version?: string
  desired_version?: string
  summary: string
  blocking_reason?: string
  depends_on?: string[]
}

export interface IndustrySolutionTableDefinition {
  name: string
  previous_name?: string
  fields: IndustrySolutionFieldDefinition[]
  indexes?: IndustrySolutionIndexDefinition[]
  constraints?: string[]
  seeds?: Array<Record<string, unknown>>
  metadata?: Record<string, unknown>
}

export interface IndustrySolutionFieldDefinition {
  name: string
  previous_name?: string
  data_type: string
  nullable: boolean
  primary_key?: boolean
  default?: string
}

export interface IndustrySolutionIndexDefinition {
  name: string
  fields: string[]
  unique?: boolean
  where?: string
  comment?: string
}

export interface IndustrySolutionFieldInput {
  name: string
  previous_name?: string
  data_type: string
  nullable: boolean
  default?: string
}

export interface IndustrySolutionTableInput {
  name: string
  previous_name?: string
  display_name?: string
  fields: IndustrySolutionFieldInput[]
  metadata?: Record<string, unknown>
}

export interface CreateIndustrySolutionTableFieldChangeInput {
  organization_id?: string
  industry_key: string
  package_key: string
  table: IndustrySolutionTableInput
  current_solution_manifest?: IndustrySolutionManifest
  reason?: string
}

export interface IndustrySolutionChangeRequest {
  id: string
  organization_id: string
  target_schema_name: string
  request_type: string
  status: string
  risk_level?: string
  reason: string
  solution_manifest: IndustrySolutionManifest
  diff?: IndustrySolutionDiff[]
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

export interface IndustrySolutionVerificationReport {
  change_request_id: string
  organization_id: string
  target_schema_name: string
  request_status: string
  status: string
  risk_level: string
  statement_count: number
  blocking_issues: number
  can_apply: boolean
  checks: IndustrySolutionVerificationCheck[]
}

export interface IndustrySolutionVerificationCheck {
  key: string
  status: string
  message: string
  metadata?: Record<string, unknown>
}

export interface IndustrySolutionApplyJob {
  id: string
  change_request_id: string
  organization_id: string
  target_schema_name: string
  status: string
  statements: string[]
  error_message?: string
  metadata: {
    asset_results?: IndustrySolutionApplyAssetResult[]
    [key: string]: unknown
  }
  created_at: string
  updated_at: string
}

export interface IndustrySolutionApplyAssetResult {
  asset_key: string
  asset_type: string
  status: string
  target: string
  error_message?: string
  metadata?: Record<string, unknown>
}

export interface CreateIndustrySolutionChangeRequestInput {
  organization_id?: string
  request_type?: string
  reason?: string
  current_solution_manifest?: IndustrySolutionManifest
  solution_manifest: IndustrySolutionManifest
}

export interface IndustrySolutionDiff {
  action: string
  table?: string
  field?: string
  from?: string
  to?: string
  risk: string
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
  last_response_ms: number
  success_count: number
  failure_count: number
  consecutive_failure_count: number
  circuit_open_until?: string
  auto_disabled_at?: string
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

export interface AssistantContextItem {
  entity_key: string
  field_key: string
  record_id: string
  value: unknown
  weight: number
  estimated_tokens: number
  validation_state: string
  source: string
  metadata?: Record<string, unknown>
}

export interface AssistantContextOmission {
  entity_key: string
  field_key: string
  reason: string
}

export interface AssistantContextPackageSummary {
  attention_core_count: number
  supporting_context_count: number
  risk_signal_count: number
  omission_count: number
  token_budget: number
  estimated_tokens: number
  source: string
}

export interface AssistantContextPackageDiagnostic {
  id: string
  session_id: string
  dictionary_version_id?: string
  summary: AssistantContextPackageSummary
  attention_core: AssistantContextItem[]
  supporting_context: AssistantContextItem[]
  risk_and_signals: AssistantContextItem[]
  omissions: AssistantContextOmission[]
  weights: Record<string, number>
  validations: Record<string, unknown>
  provenance: Record<string, unknown>
}

export interface AssistantContextHealthSummary {
  organization_id?: string
  active_rule_count: number
  strict_modules: string[]
  strict_module_coverage: Record<string, number>
  missing_strict_modules: string[]
  recent_package_count: number
  fallback_package_count: number
  context_build_failure_count: number
  pending_proposal_count: number
  approved_proposal_count: number
  applied_proposal_count: number
  tool_approval_backlog: number
}

export interface MonitoringAgentRun {
  id: string
  trigger_type: string
  organization_id?: string
  status: string
  lookback_started_at: string
  lookback_ended_at: string
  signals_created: number
  duplicates_suppressed: number
  summary: Record<string, unknown>
  error_message?: string
  started_at: string
  completed_at?: string
}

export interface MonitoringAgentStatus {
  scheduler_enabled: boolean
  daily_time: string
  lookback_hours: number
  max_signals_per_run: number
  latest_run?: MonitoringAgentRun
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

export interface AIModelGroup {
  id: string
  organization_id?: string
  department_id?: string
  project_id?: string
  agent_id?: string
  name: string
  group_key: string
  status: string
  rate_multiplier: number
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateAIModelGroupInput {
  organization_id?: string
  department_id?: string
  project_id?: string
  agent_id?: string
  name: string
  group_key?: string
  status?: string
  rate_multiplier?: number
  metadata?: Record<string, unknown>
}

export interface AIModelChannelAbility {
  id: string
  model_group_id?: string
  channel_id: string
  requested_model: string
  model_pattern: string
  upstream_model: string
  priority: number
  enabled: boolean
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateAIModelChannelAbilityInput {
  model_group_id?: string
  channel_id: string
  requested_model?: string
  model_pattern?: string
  upstream_model?: string
  priority?: number
  enabled?: boolean
  metadata?: Record<string, unknown>
}

export interface AIAccessToken {
  id: string
  organization_id: string
  model_group_id?: string
  name: string
  masked_token: string
  plain_token?: string
  status: string
  allowed_models: string[]
  allowed_model_patterns: string[]
  allow_channel_override: boolean
  quota_amount: number
  quota_used: number
  quota_currency: string
  expires_at?: string
  last_used_at?: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateAIAccessTokenInput {
  organization_id: string
  model_group_id?: string
  name: string
  allowed_models?: string[]
  allowed_model_patterns?: string[]
  allow_channel_override?: boolean
  quota_amount?: number
  quota_currency?: string
  expires_at?: string
  metadata?: Record<string, unknown>
}

export interface AIGatewayBalance {
  id?: string
  organization_id: string
  balance_amount: number
  reserved_amount: number
  currency: string
  metadata: Record<string, unknown>
  created_at?: string
  updated_at?: string
}

export interface AdjustAIGatewayBalanceInput {
  organization_id: string
  amount: number
  currency?: string
  reason: string
  metadata?: Record<string, unknown>
}

export interface AIBalanceTransaction {
  id: string
  organization_id: string
  access_token_id?: string
  model_group_id?: string
  invocation_id?: string
  reservation_id?: string
  transaction_type: string
  amount: number
  currency: string
  reason: string
  metadata: Record<string, unknown>
  created_at: string
}

export interface AIAdapterDescriptor {
  adapter_key: string
  display_name: string
  provider_type: string
  adapter_mode: string
  default_base_url: string
  supported_models: string[]
  supported_modes: string[]
  requires_native_io: boolean
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

export interface FinanceGLAccount {
  id: string
  master_key?: string
  account_code: string
  name: string
  account_type: string
  currency: string
  parent_account_code: string
  postable: boolean
  active: boolean
  organization_id?: string
  department_id?: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateFinanceGLAccountInput {
  account_code: string
  name: string
  account_type?: string
  currency?: string
  parent_account_code?: string
  postable?: boolean
  active?: boolean
  organization_id?: string
  department_id?: string
  metadata?: Record<string, unknown>
}

export interface FinanceGLCostCenter {
  id: string
  master_key?: string
  cost_center_code: string
  name: string
  active: boolean
  organization_id?: string
  department_id?: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateFinanceGLCostCenterInput {
  cost_center_code: string
  name: string
  active?: boolean
  organization_id?: string
  department_id?: string
  metadata?: Record<string, unknown>
}

export interface FinanceGLJournalEntryLine {
  id: string
  entry_id: string
  line_num: number
  account_code: string
  account_name: string
  cost_center_code: string
  debit: number
  credit: number
  description: string
  metadata: Record<string, unknown>
  created_at: string
}

export interface FinanceGLJournalEntry {
  id: string
  master_key?: string
  entry_number: string
  reference_date: string
  memo: string
  status: string
  currency: string
  source_type: string
  source_id?: string
  organization_id?: string
  department_id?: string
  metadata: Record<string, unknown>
  lines?: FinanceGLJournalEntryLine[]
  posted_at?: string
  created_at: string
  updated_at: string
}

export interface CreateFinanceGLJournalEntryInput {
  entry_number?: string
  reference_date: string
  memo?: string
  currency?: string
  source_type?: string
  source_id?: string
  organization_id?: string
  department_id?: string
  metadata?: Record<string, unknown>
  lines: Array<{
    account_code: string
    account_name?: string
    cost_center_code?: string
    debit?: number
    credit?: number
    description?: string
    metadata?: Record<string, unknown>
  }>
}

export interface FinanceGLTrialBalanceInput {
  organization_id?: string
  period_start?: string
  period_end?: string
  currency?: string
}

export interface FinanceGLTrialBalanceRow {
  account_code: string
  account_name: string
  debit: number
  credit: number
  net_amount: number
}

export interface FinanceGLTrialBalance {
  rows: FinanceGLTrialBalanceRow[]
  total_debit: number
  total_credit: number
  currency: string
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
