'use client'

import {
  Activity,
  BarChart3,
  Bot,
  CheckCircle2,
  ClipboardCheck,
  ClipboardList,
  Coins,
  FileCheck2,
  Download,
  GitBranch,
  Loader2,
  Plus,
  Play,
  RefreshCw,
  Save,
  Search,
  Send,
  Star,
  Upload,
  Users,
} from 'lucide-react'
import { createContext, useContext, useEffect, useMemo, useState } from 'react'
import type { FormEvent, ReactNode } from 'react'
import { API_BASE, apiRequest, getUserFieldPreference, saveUserFieldPreference } from '@/lib/api'
import { apiErrorFromResponse, createRequestId } from '@/lib/api-error'
import { getCurrentOrganizationId, normalizeOrganizationId } from '@/lib/auth'
import { useI18n } from '@/lib/i18n'

type LifecycleMode = 'Requirement' | 'Project' | 'Delivery' | 'Cost' | 'Feedback'

const FieldPreferenceTokenContext = createContext('')

interface WorkspaceProps {
  token: string
  currentUserId?: string | null
  mode: LifecycleMode
  externalSelection?: {
    targetType: string
    targetID?: string
    record?: Record<string, unknown>
  } | null
  onOperationContextChange?: (context: Record<string, string>) => void
}

interface Organization {
  id: string
  name: string
}

interface Department {
  id: string
  organization_id: string
  parent_id?: string
  name: string
  children?: Department[] | null
}

interface Requirement {
  id: string
  master_key?: string
  title: string
  description: string
  source: string
  status: string
  priority: string
  risk_level: string
  required_level: string
  organization_id?: string
  department_id?: string
  analysis: Record<string, unknown>
  metadata: Record<string, unknown>
  created_at: string
}

interface RequirementDocument {
  id: string
  master_key?: string
  requirement_id: string
  file_name: string
  content_type: string
  size_bytes: number
  uploaded_by_type?: string
  created_at: string
}

interface RequirementAnalysisWorkflow {
  id: string
  master_key?: string
  requirement_id: string
  workflow_id: string
  workflow_template_id: string
  status: string
  analysis_result: Record<string, unknown>
  created_at: string
  updated_at: string
}

interface WorkflowTemplate {
  id: string
  name: string
  description?: string
  is_active: boolean
}

interface Project {
  id: string
  master_key?: string
  requirement_id?: string
  organization_id?: string
  department_id?: string
  name: string
  description: string
  status: string
  priority: string
  risk_level: string
  required_level: string
  budget_amount: number
  metadata: Record<string, unknown>
  created_at: string
}

interface ProjectMember {
  id: string
  master_key?: string
  project_id: string
  actor_id: string
  actor_type: string
  role: string
  title: string
  allocation_percent: number
  cost_rate: number
  permission_level: string
  capabilities: string[]
  status: string
}

interface ProjectWorkflow {
  id: string
  master_key?: string
  project_id: string
  workflow_id: string
  workflow_template_id?: string
  purpose: string
  status: string
}

interface Deliverable {
  id: string
  master_key?: string
  project_id: string
  name: string
  deliverable_type: string
  uri: string
  version: string
  status: string
  submitted_by_type?: string
  accepted_by_type?: string
  evidence: Record<string, unknown>
  created_at: string
}

interface CostEntry {
  id: string
  master_key?: string
  project_id: string
  source_type: string
  actor_id?: string
  actor_type?: string
  amount: number
  currency: string
  occurred_at: string
  description: string
}

interface CostSummary {
  project_id: string
  currency: string
  entry_count: number
  total_amount: number
  budget_amount: number
  budget_variance: number
  by_source: Array<{ source_type: string; amount: number; count: number }>
}

interface ProjectEvaluation {
  id: string
  master_key?: string
  project_id: string
  actor_id?: string
  actor_type?: string
  capability_id?: string
  evaluator_type: string
  quality_score: number
  delivery_score: number
  cost_score: number
  collaboration_score: number
  overall_score: number
  conclusion: string
  created_at: string
}

interface ProjectOverview {
  project: Project
  requirement?: Requirement
  members: ProjectMember[] | null
  workflows: ProjectWorkflow[] | null
  deliverables: Deliverable[] | null
  cost_summary: CostSummary
  evaluations: ProjectEvaluation[] | null
  lifecycle?: ProjectLifecycle
}

interface ProjectLifecycle {
  stage: string
  pdca_stage: string
  requirement_id?: string
  demand_profile_id?: string
  pdca_cycle_id?: string
  allowed_actions: string[]
  blockers: string[]
  next_action: string
}

interface MatchCandidate {
  membership_id: string
  member_type: string
  member_id: string
  member_name: string
  title?: string
  score: number
  weight_snapshot: number
  access_decision: string
  access_allowed: boolean
  requires_approval: boolean
  reason: string
}

const modeMeta: Record<LifecycleMode, { title: string; icon: typeof ClipboardList }> = {
  Requirement: { title: '需求', icon: ClipboardList },
  Project: { title: '项目', icon: GitBranch },
  Delivery: { title: '交付', icon: FileCheck2 },
  Cost: { title: '成本', icon: Coins },
  Feedback: { title: '反馈评估', icon: Star },
}

const requirementDefaults = {
  title: '业务流程自动化需求',
  description: '从业务需求拆解到流程、成员、Agent 与能力匹配的闭环交付',
  source: 'manual',
  priority: 'medium',
  risk_level: 'medium',
  required_level: 'L2',
  organization_id: '',
  department_id: '',
  analysis_notes: '',
  convert_budget: '5000',
}

const projectDefaults = {
  requirement_id: '',
  name: '需求驱动交付项目',
  description: '围绕需求创建项目并进行过程管控、交付、成本核算和反馈评估',
  status: 'planning',
  priority: 'medium',
  risk_level: 'medium',
  required_level: 'L2',
  budget_amount: '5000',
  organization_id: '',
  department_id: '',
}

function requirementToForm(requirement: Requirement, current: typeof requirementDefaults = requirementDefaults) {
  return {
    ...current,
    title: requirement.title,
    description: requirement.description,
    source: requirement.source,
    priority: requirement.priority,
    risk_level: requirement.risk_level,
    required_level: requirement.required_level,
    organization_id: requirement.organization_id ?? '',
    department_id: requirement.department_id ?? '',
  }
}

function recordKey(record: { id: string; master_key?: string }) {
  return record.master_key || record.id
}

function projectToForm(project: Project, current: typeof projectDefaults = projectDefaults) {
  return {
    ...current,
    requirement_id: project.requirement_id ?? '',
    name: project.name,
    description: project.description,
    status: project.status,
    priority: project.priority,
    risk_level: project.risk_level,
    required_level: project.required_level,
    budget_amount: String(project.budget_amount ?? 0),
    organization_id: project.organization_id ?? '',
    department_id: project.department_id ?? '',
  }
}

const memberDefaults = {
  member_actor_id: '',
  member_actor_type: 'internal_human',
  role: 'owner',
  title: '项目负责人',
  allocation_percent: '100',
  cost_rate: '800',
  permission_level: 'L2',
  capabilities: 'planning, delivery, review',
}

const workflowDefaults = {
  workflow_id: '',
  workflow_template_id: '',
  purpose: 'delivery',
}

const matchDefaults = {
  task_description: '需求拆解、执行交付与人工验收',
  required_capabilities: 'planning, delivery, review',
  required_level: 'L2',
  risk_level: 'medium',
}

const deliverableDefaults = {
  name: '交付物 v1',
  deliverable_type: 'document',
  uri: '',
  version: '1.0',
  status: 'draft',
}

const costDefaults = {
  source_type: 'manual',
  entry_actor_id: '',
  entry_actor_type: 'internal_human',
  amount: '1000',
  currency: 'CNY',
  description: '人工录入成本',
}

const evaluationDefaults = {
  evaluated_actor_id: '',
  evaluated_actor_type: 'internal_human',
  capability_id: '',
  quality_score: '0.8',
  delivery_score: '0.8',
  cost_score: '0.75',
  collaboration_score: '0.8',
  conclusion: '人工复核通过',
  outcome_score: '0.8',
}

const formatter = new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 })

function asArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : []
}

function deferStateUpdate(callback: () => void): () => void {
  if (typeof window === 'undefined') {
    callback()
    return () => undefined
  }
  const timeout = window.setTimeout(callback, 0)
  return () => window.clearTimeout(timeout)
}

function percent(value: number | undefined): string {
  return `${Math.round((value ?? 0) * 100)}%`
}

function money(value: number | undefined, currency = 'CNY'): string {
  return `${currency} ${formatter.format(value ?? 0)}`
}

function formatDate(value?: string): string {
  if (!value) return ''
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

function splitCsv(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function flattenDepartments(nodes: Department[] | null | undefined): Department[] {
  return asArray(nodes).flatMap((node) => [node, ...flattenDepartments(node.children)])
}

function actionForStatus(status: string): string {
  const actions: Record<string, string> = {
    active: 'activate_project',
    delivering: 'start_delivery',
    completed: 'complete_project',
    closed: 'close_feedback',
  }
  return actions[status] ?? status
}

function canRunRequirementAction(status: string, action: 'analyze' | 'approve' | 'convert' | 'sync_analysis'): boolean {
  switch (action) {
    case 'analyze':
    case 'sync_analysis':
      return status === 'draft' || status === 'analyzed' || status === 'converted'
    case 'approve':
      return status === 'analyzed'
    case 'convert':
      return true
  }
}

export function ProjectLifecycleWorkspace({ token, currentUserId, mode, externalSelection, onOperationContextChange }: WorkspaceProps) {
  const { t } = useI18n()
  const [requirements, setRequirements] = useState<Requirement[]>([])
  const [requirementDocuments, setRequirementDocuments] = useState<RequirementDocument[]>([])
  const [analysisWorkflows, setAnalysisWorkflows] = useState<RequirementAnalysisWorkflow[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [organizations, setOrganizations] = useState<Organization[]>([])
  const [departments, setDepartments] = useState<Department[]>([])
  const [workflowTemplates, setWorkflowTemplates] = useState<WorkflowTemplate[]>([])
  const [selectedRequirementId, setSelectedRequirementId] = useState('')
  const [selectedProjectId, setSelectedProjectId] = useState('')
  const [overview, setOverview] = useState<ProjectOverview | null>(null)
  const [costEntries, setCostEntries] = useState<CostEntry[]>([])
  const [candidates, setCandidates] = useState<MatchCandidate[]>([])
  const [requirementForm, setRequirementForm] = useState(requirementDefaults)
  const [selectedDocument, setSelectedDocument] = useState<File | null>(null)
  const [analysisWorkflowForm, setAnalysisWorkflowForm] = useState({ workflow_template_id: '', purpose: 'requirement_analysis' })
  const [projectForm, setProjectForm] = useState(projectDefaults)
  const [memberForm, setMemberForm] = useState(memberDefaults)
  const [workflowForm, setWorkflowForm] = useState(workflowDefaults)
  const [matchForm, setMatchForm] = useState(matchDefaults)
  const [deliverableForm, setDeliverableForm] = useState(deliverableDefaults)
  const [costForm, setCostForm] = useState(costDefaults)
  const [evaluationForm, setEvaluationForm] = useState(evaluationDefaults)
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const selectedProject = useMemo(
    () => projects.find((project) => recordKey(project) === selectedProjectId || project.id === selectedProjectId) ?? overview?.project ?? null,
    [overview, projects, selectedProjectId],
  )

  const selectedRequirement = useMemo(
    () => requirements.find((requirement) => recordKey(requirement) === selectedRequirementId || requirement.id === selectedRequirementId) ?? overview?.requirement ?? null,
    [overview, requirements, selectedRequirementId],
  )

  useEffect(() => {
    onOperationContextChange?.({
      requirement_id: selectedRequirementId,
      project_id: selectedProjectId,
    })
  }, [onOperationContextChange, selectedProjectId, selectedRequirementId])

  useEffect(() => {
    if (!externalSelection?.targetID) return
    const targetID = externalSelection.targetID
    const targetType = externalSelection.targetType
    const record = externalSelection.record
    return deferStateUpdate(() => {
      if (targetType === 'requirement') {
        setSelectedRequirementId(targetID)
      }
      if (targetType === 'project') {
        setSelectedProjectId(targetID)
        const requirementID = typeof record?.requirement_id === 'string' ? record.requirement_id : ''
        if (requirementID) setSelectedRequirementId(requirementID)
      }
    })
  }, [externalSelection?.targetID, externalSelection?.targetType, externalSelection?.record])

  useEffect(() => {
    let cancelled = false

    Promise.all([
      apiRequest<Requirement[]>('/requirements?limit=100', { token }),
      apiRequest<Project[]>('/projects?limit=100', { token }),
      apiRequest<{ organization: Organization }>('/organization/current', { token }),
      apiRequest<WorkflowTemplate[]>('/workflows/templates', { token }),
    ])
      .then(([requirementData, projectData, organizationData, workflowTemplateData]) => {
        if (cancelled) return
        const safeRequirements = asArray(requirementData)
        const safeProjects = asArray(projectData)
        const safeOrganizations = organizationData.organization ? [organizationData.organization] : []
        const safeWorkflowTemplates = asArray(workflowTemplateData)
        setRequirements(safeRequirements)
        setProjects(safeProjects)
        setOrganizations(safeOrganizations)
        setWorkflowTemplates(safeWorkflowTemplates)
        setSelectedRequirementId((current) => current || (safeRequirements[0] ? recordKey(safeRequirements[0]) : ''))
        setSelectedProjectId((current) => current || (safeProjects[0] ? recordKey(safeProjects[0]) : ''))
        setRequirementForm((current) => ({
          ...current,
          organization_id: current.organization_id || safeOrganizations[0]?.id || '',
        }))
        setProjectForm((current) => ({
          ...current,
          organization_id: current.organization_id || safeOrganizations[0]?.id || '',
        }))
        setAnalysisWorkflowForm((current) => ({
          ...current,
          workflow_template_id: current.workflow_template_id || safeWorkflowTemplates[0]?.id || '',
        }))
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : t('加载生命周期数据失败'))
      })

    return () => {
      cancelled = true
    }
  }, [t, token])

  useEffect(() => {
    const organizationID = requirementForm.organization_id || projectForm.organization_id
    if (!organizationID) {
      Promise.resolve().then(() => setDepartments([]))
      return
    }
    let cancelled = false
    apiRequest<Department[]>(`/organizations/${organizationID}/departments/tree`, { token })
      .then((data) => {
        if (!cancelled) setDepartments(flattenDepartments(data))
      })
      .catch(() => {
        if (!cancelled) setDepartments([])
      })
    return () => {
      cancelled = true
    }
  }, [projectForm.organization_id, requirementForm.organization_id, token])

  useEffect(() => {
    if (!selectedRequirementId) {
      Promise.resolve().then(() => {
        setRequirementDocuments([])
        setAnalysisWorkflows([])
      })
      return
    }
    let cancelled = false

    Promise.all([
      apiRequest<RequirementDocument[]>(`/requirements/${selectedRequirementId}/documents`, { token }),
      apiRequest<RequirementAnalysisWorkflow[]>(`/requirements/${selectedRequirementId}/analysis-workflows`, { token }),
    ])
      .then(([documentData, workflowData]) => {
        if (cancelled) return
        setRequirementDocuments(asArray(documentData))
        setAnalysisWorkflows(asArray(workflowData))
      })
      .catch(() => {
        if (cancelled) return
        setRequirementDocuments([])
        setAnalysisWorkflows([])
      })

    return () => {
      cancelled = true
    }
  }, [selectedRequirementId, token])

  useEffect(() => {
    if (!selectedProjectId) {
      Promise.resolve().then(() => {
        setOverview(null)
        setCostEntries([])
      })
      return
    }
    let cancelled = false

    Promise.all([
      apiRequest<ProjectOverview>(`/projects/${selectedProjectId}/overview`, { token }),
      apiRequest<CostEntry[]>(`/projects/${selectedProjectId}/cost-entries`, { token }),
    ])
      .then(([overviewData, costData]) => {
        if (cancelled) return
        setOverview({
          ...overviewData,
          members: asArray(overviewData.members),
          workflows: asArray(overviewData.workflows),
          deliverables: asArray(overviewData.deliverables),
          evaluations: asArray(overviewData.evaluations),
        })
        setCostEntries(asArray(costData))
      })
      .catch((err) => {
        if (!cancelled) {
          setOverview(null)
          setCostEntries([])
          setError(err instanceof Error ? err.message : t('加载项目详情失败'))
        }
      })

    return () => {
      cancelled = true
    }
  }, [selectedProjectId, t, token])

  async function loadLifecycle() {
    const [requirementData, projectData, organizationData, workflowTemplateData] = await Promise.all([
      apiRequest<Requirement[]>('/requirements?limit=100', { token }),
      apiRequest<Project[]>('/projects?limit=100', { token }),
      apiRequest<{ organization: Organization }>('/organization/current', { token }),
      apiRequest<WorkflowTemplate[]>('/workflows/templates', { token }),
    ])
    const safeRequirements = asArray(requirementData)
    const safeProjects = asArray(projectData)
    setRequirements(safeRequirements)
    setProjects(safeProjects)
    const safeOrganizations = organizationData.organization ? [organizationData.organization] : []
    setOrganizations(safeOrganizations)
    setRequirementForm((current) => ({
      ...current,
      organization_id: current.organization_id || safeOrganizations[0]?.id || '',
    }))
    setProjectForm((current) => ({
      ...current,
      organization_id: current.organization_id || safeOrganizations[0]?.id || '',
    }))
    setWorkflowTemplates(asArray(workflowTemplateData))
    setSelectedRequirementId((current) => current || (safeRequirements[0] ? recordKey(safeRequirements[0]) : ''))
    setSelectedProjectId((current) => current || (safeProjects[0] ? recordKey(safeProjects[0]) : ''))
    if (selectedRequirementId) {
      await loadRequirementDetail(selectedRequirementId)
    }
    if (selectedProjectId) {
      await loadProjectDetail(selectedProjectId)
    }
  }

  async function loadRequirementDetail(requirementID = selectedRequirementId) {
    if (!requirementID) return
    const [documentData, workflowData] = await Promise.all([
      apiRequest<RequirementDocument[]>(`/requirements/${requirementID}/documents`, { token }),
      apiRequest<RequirementAnalysisWorkflow[]>(`/requirements/${requirementID}/analysis-workflows`, { token }),
    ])
    setRequirementDocuments(asArray(documentData))
    setAnalysisWorkflows(asArray(workflowData))
  }

  async function loadProjectDetail(projectID = selectedProjectId) {
    if (!projectID) return
    const [overviewData, costData] = await Promise.all([
      apiRequest<ProjectOverview>(`/projects/${projectID}/overview`, { token }),
      apiRequest<CostEntry[]>(`/projects/${projectID}/cost-entries`, { token }),
    ])
    setOverview({
      ...overviewData,
      members: asArray(overviewData.members),
      workflows: asArray(overviewData.workflows),
      deliverables: asArray(overviewData.deliverables),
      evaluations: asArray(overviewData.evaluations),
    })
    setCostEntries(asArray(costData))
  }

  async function run(action: () => Promise<void>, success: string) {
    setLoading(true)
    setError(null)
    setMessage(null)
    try {
      await action()
      setMessage(t(success))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('操作失败'))
    } finally {
      setLoading(false)
    }
  }

  function selectRequirement(requirementID: string) {
    setSelectedRequirementId(requirementID)
    const requirement = requirements.find((item) => recordKey(item) === requirementID || item.id === requirementID)
    if (requirement) {
      setRequirementForm((current) => requirementToForm(requirement, current))
    }
  }

  function selectProject(projectID: string) {
    setSelectedProjectId(projectID)
    const project = projects.find((item) => recordKey(item) === projectID || item.id === projectID) ?? (overview?.project && (recordKey(overview.project) === projectID || overview.project.id === projectID) ? overview.project : null)
    if (project) {
      setProjectForm((current) => projectToForm(project, current))
    }
  }

  async function createRequirement(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(async () => {
      const req = await apiRequest<Requirement>('/requirements', {
        method: 'POST',
        token,
        body: {
          title: requirementForm.title,
          description: requirementForm.description,
          source: requirementForm.source,
          priority: requirementForm.priority,
          risk_level: requirementForm.risk_level,
          required_level: requirementForm.required_level,
          organization_id: requirementForm.organization_id || null,
          department_id: requirementForm.department_id || null,
          created_by_id: currentUserId || null,
          created_by_type: 'internal_human',
          metadata: { source_ui: 'project_lifecycle_workspace' },
        },
      })
      setSelectedRequirementId(recordKey(req))
      await loadLifecycle()
    }, '需求已创建')
  }

  async function updateRequirement(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedRequirementId) return
    await run(async () => {
      const req = await apiRequest<Requirement>(`/requirements/${selectedRequirementId}`, {
        method: 'PATCH',
        token,
        body: {
          title: requirementForm.title,
          description: requirementForm.description,
          source: requirementForm.source,
          priority: requirementForm.priority,
          risk_level: requirementForm.risk_level,
          required_level: requirementForm.required_level,
          metadata: { source_ui: 'project_lifecycle_workspace', direct_edit: true },
        },
      })
      setSelectedRequirementId(recordKey(req))
      setRequirementForm((current) => requirementToForm(req, current))
      await loadLifecycle()
      await loadRequirementDetail(recordKey(req))
    }, '需求已更新')
  }

  async function uploadRequirementDocument(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedRequirementId || !selectedDocument) return
    await run(async () => {
      const formData = new FormData()
      formData.append('file', selectedDocument)
      formData.append('metadata', JSON.stringify({ source_ui: 'requirement_workspace' }))
      await apiRequest<RequirementDocument>(`/requirements/${selectedRequirementId}/documents`, {
        method: 'POST',
        token,
        body: formData,
      })
      setSelectedDocument(null)
      await loadRequirementDetail(selectedRequirementId)
    }, '文档已上传')
  }

  async function startRequirementAnalysisWorkflow(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedRequirementId || !analysisWorkflowForm.workflow_template_id) return
    await run(async () => {
      await apiRequest<RequirementAnalysisWorkflow>(`/requirements/${selectedRequirementId}/analysis-workflows`, {
        method: 'POST',
        token,
        body: {
          workflow_template_id: analysisWorkflowForm.workflow_template_id,
          purpose: analysisWorkflowForm.purpose,
          context: {
            source_ui: 'requirement_workspace',
          },
          metadata: {},
        },
      })
      await loadRequirementDetail(selectedRequirementId)
    }, '需求分析流程已启动')
  }

  async function syncRequirementAnalysisWorkflow(workflowID: string) {
    if (!selectedRequirementId) return
    await run(async () => {
      await apiRequest(`/requirements/${selectedRequirementId}/analysis-workflows/${workflowID}/sync`, {
        method: 'POST',
        token,
        body: {},
      })
      await loadLifecycle()
      await loadRequirementDetail(selectedRequirementId)
    }, '流程结果已同步到需求')
  }

  async function downloadRequirementDocument(documentID: string, fileName: string) {
    await run(async () => {
      const headers: Record<string, string> = {
        Authorization: `Bearer ${token}`,
      }
      const requestId = createRequestId()
      if (requestId) headers['X-Request-ID'] = requestId
      const organizationId = normalizeOrganizationId(getCurrentOrganizationId('tenant'))
      if (organizationId) {
        headers['X-Organization-ID'] = organizationId
      }
      const response = await fetch(`${API_BASE}/requirement-documents/${documentID}/download`, {
        headers,
      })
      if (!response.ok) {
        throw await apiErrorFromResponse(response)
      }
      const blob = await response.blob()
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = fileName
      document.body.appendChild(link)
      link.click()
      link.remove()
      URL.revokeObjectURL(url)
    }, '文档已下载')
  }

  async function analyzeRequirement(requirementID = selectedRequirementId) {
    if (!requirementID) return
    await run(async () => {
      const req = await apiRequest<Requirement>(`/requirements/${requirementID}/analyze`, {
        method: 'POST',
        token,
        body: { notes: requirementForm.analysis_notes },
      })
      setSelectedRequirementId(recordKey(req))
      await loadLifecycle()
    }, '需求分析已完成')
  }

  async function approveRequirement(requirementID = selectedRequirementId) {
    if (!requirementID) return
    await run(async () => {
      await apiRequest<Requirement>(`/requirements/${requirementID}/approve`, {
        method: 'POST',
        token,
        body: {},
      })
      await loadLifecycle()
    }, '需求已审批')
  }

  async function convertRequirement(requirementID = selectedRequirementId) {
    if (!requirementID) return
    await run(async () => {
      const proj = await apiRequest<Project>(`/requirements/${requirementID}/convert-to-project`, {
        method: 'POST',
        token,
        body: {
          budget_amount: Number(requirementForm.convert_budget),
          metadata: { source_ui: 'requirement_convert' },
        },
      })
      setSelectedProjectId(recordKey(proj))
      await loadLifecycle()
      await loadProjectDetail(recordKey(proj))
    }, '需求已转为项目')
  }

  async function createProject(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(async () => {
      const proj = await apiRequest<Project>('/projects', {
        method: 'POST',
        token,
        body: {
          requirement_id: projectForm.requirement_id || null,
          organization_id: projectForm.organization_id || null,
          department_id: projectForm.department_id || null,
          name: projectForm.name,
          description: projectForm.description,
          status: projectForm.status,
          priority: projectForm.priority,
          risk_level: projectForm.risk_level,
          required_level: projectForm.required_level,
          budget_amount: Number(projectForm.budget_amount),
          metadata: { source_ui: 'project_workspace' },
        },
      })
      setSelectedProjectId(recordKey(proj))
      await loadLifecycle()
      await loadProjectDetail(recordKey(proj))
    }, '项目已创建')
  }

  async function updateProject(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedProjectId) return
    await run(async () => {
      const proj = await apiRequest<Project>(`/projects/${selectedProjectId}`, {
        method: 'PATCH',
        token,
        body: {
          name: projectForm.name,
          description: projectForm.description,
          status: projectForm.status,
          priority: projectForm.priority,
          risk_level: projectForm.risk_level,
          required_level: projectForm.required_level,
          budget_amount: Number(projectForm.budget_amount),
          metadata: { source_ui: 'project_workspace', direct_edit: true },
        },
      })
      setSelectedProjectId(recordKey(proj))
      setProjectForm((current) => projectToForm(proj, current))
      await loadLifecycle()
      await loadProjectDetail(recordKey(proj))
    }, '项目已更新')
  }

  async function updateProjectStatus(status: string) {
    if (!selectedProjectId) return
    await run(async () => {
      await apiRequest<Project>(`/projects/${selectedProjectId}/status`, {
        method: 'POST',
        token,
        body: { status },
      })
      await loadLifecycle()
      await loadProjectDetail(selectedProjectId)
    }, '项目状态已更新')
  }

  async function addProjectMember(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedProjectId) return
    await run(async () => {
      await apiRequest<ProjectMember>(`/projects/${selectedProjectId}/members`, {
        method: 'POST',
        token,
        body: {
          member_actor_id: memberForm.member_actor_id,
          member_actor_type: memberForm.member_actor_type,
          role: memberForm.role,
          title: memberForm.title,
          allocation_percent: Number(memberForm.allocation_percent),
          cost_rate: Number(memberForm.cost_rate),
          permission_level: memberForm.permission_level,
          capabilities: splitCsv(memberForm.capabilities),
          metadata: { source_ui: 'project_member_form' },
        },
      })
      await loadProjectDetail(selectedProjectId)
    }, '成员已加入项目')
  }

  async function bindWorkflow(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedProjectId) return
    await run(async () => {
      await apiRequest<ProjectWorkflow>(`/projects/${selectedProjectId}/workflows`, {
        method: 'POST',
        token,
        body: {
          workflow_id: workflowForm.workflow_id || null,
          workflow_template_id: workflowForm.workflow_template_id || null,
          purpose: workflowForm.purpose,
          metadata: { source_ui: 'project_workflow_form' },
        },
      })
      await loadProjectDetail(selectedProjectId)
    }, '流程已绑定项目')
  }

  async function matchActors() {
    if (!selectedProjectId) return
    await run(async () => {
      const data = await apiRequest<MatchCandidate[]>(`/projects/${selectedProjectId}/match-actors`, {
        method: 'POST',
        token,
        body: {
          task_description: matchForm.task_description,
          required_capabilities: splitCsv(matchForm.required_capabilities),
          required_level: matchForm.required_level,
          risk_level: matchForm.risk_level,
          member_types: ['internal', 'external', 'agent'],
        },
      })
      setCandidates(asArray(data))
    }, '候选成员已匹配')
  }

  async function createDeliverable(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedProjectId) return
    await run(async () => {
      await apiRequest<Deliverable>(`/projects/${selectedProjectId}/deliverables`, {
        method: 'POST',
        token,
        body: {
          name: deliverableForm.name,
          deliverable_type: deliverableForm.deliverable_type,
          uri: deliverableForm.uri,
          version: deliverableForm.version,
          status: deliverableForm.status,
          metadata: { source_ui: 'delivery_workspace' },
        },
      })
      await loadProjectDetail(selectedProjectId)
    }, '交付物已创建')
  }

  async function changeDeliverable(id: string, action: 'submit' | 'accept' | 'reject') {
    await run(async () => {
      await apiRequest<Deliverable>(`/deliverables/${id}/${action}`, {
        method: 'POST',
        token,
        body: { reason: action === 'reject' ? '未通过验收' : '' },
      })
      await loadProjectDetail(selectedProjectId)
    }, action === 'submit' ? '交付物已提交' : action === 'accept' ? '交付物已验收' : '交付物已退回')
  }

  async function createCostEntry(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedProjectId) return
    await run(async () => {
      await apiRequest<CostEntry>(`/projects/${selectedProjectId}/cost-entries`, {
        method: 'POST',
        token,
        body: {
          source_type: costForm.source_type,
          entry_actor_id: costForm.entry_actor_id || null,
          entry_actor_type: costForm.entry_actor_type,
          amount: Number(costForm.amount),
          currency: costForm.currency,
          description: costForm.description,
          metadata: { source_ui: 'cost_workspace' },
        },
      })
      await loadProjectDetail(selectedProjectId)
    }, '成本已入账')
  }

  async function refreshCost() {
    if (!selectedProjectId) return
    await run(async () => {
      await apiRequest<CostEntry[]>(`/projects/${selectedProjectId}/cost-refresh`, {
        method: 'POST',
        token,
        body: {},
      })
      await loadProjectDetail(selectedProjectId)
    }, '成员成本已刷新')
  }

  async function createEvaluation(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedProjectId) return
    await run(async () => {
      await apiRequest<ProjectEvaluation>(`/projects/${selectedProjectId}/evaluations`, {
        method: 'POST',
        token,
        body: {
          evaluated_actor_id: evaluationForm.evaluated_actor_id || null,
          evaluated_actor_type: evaluationForm.evaluated_actor_type,
          capability_id: evaluationForm.capability_id || null,
          quality_score: Number(evaluationForm.quality_score),
          delivery_score: Number(evaluationForm.delivery_score),
          cost_score: Number(evaluationForm.cost_score),
          collaboration_score: Number(evaluationForm.collaboration_score),
          conclusion: evaluationForm.conclusion,
          evidence: { source_ui: 'feedback_workspace' },
        },
      })
      await loadProjectDetail(selectedProjectId)
    }, '评估已记录')
  }

  async function closeFeedback() {
    if (!selectedProjectId) return
    await run(async () => {
      await apiRequest(`/projects/${selectedProjectId}/close-feedback`, {
        method: 'POST',
        token,
        body: {
          outcome_score: Number(evaluationForm.outcome_score),
          conclusion: evaluationForm.conclusion,
          metadata: { source_ui: 'feedback_workspace' },
        },
      })
      await loadLifecycle()
      await loadProjectDetail(selectedProjectId)
    }, '项目反馈已闭环')
  }

  const meta = modeMeta[mode]
  const Icon = meta.icon

  return (
    <FieldPreferenceTokenContext.Provider value={token}>
      <div className="space-y-5">
      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-2">
            <Icon className="h-5 w-5 text-slate-500" />
            <h2 className="text-base font-semibold text-slate-950">{t(meta.title)}</h2>
          </div>
          <button
            type="button"
            onClick={() => run(loadLifecycle, '生命周期数据已刷新')}
            className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-300 px-3 text-sm font-semibold text-slate-700 hover:bg-slate-100"
          >
            <RefreshCw className="h-4 w-4" />
            {t('刷新')}
          </button>
        </div>
        {(message || error) && (
          <div
            className={`mt-4 rounded-lg border px-4 py-3 text-sm ${
              error ? 'border-red-200 bg-red-50 text-red-700' : 'border-emerald-200 bg-emerald-50 text-emerald-700'
            }`}
          >
            {error || message}
          </div>
        )}
      </section>

      {mode === 'Requirement' && (
        <RequirementView
          requirements={requirements}
          documents={requirementDocuments}
          analysisWorkflows={analysisWorkflows}
          workflowTemplates={workflowTemplates}
          organizations={organizations}
          departments={departments}
          selectedRequirementId={selectedRequirementId}
          selectedRequirement={selectedRequirement}
          form={requirementForm}
          selectedDocument={selectedDocument}
          analysisWorkflowForm={analysisWorkflowForm}
          loading={loading}
          onSelectRequirement={selectRequirement}
          onFormChange={setRequirementForm}
          onDocumentChange={setSelectedDocument}
          onAnalysisWorkflowFormChange={setAnalysisWorkflowForm}
          onCreate={createRequirement}
          onUpdate={updateRequirement}
          onUploadDocument={uploadRequirementDocument}
          onDownloadDocument={downloadRequirementDocument}
          onStartAnalysisWorkflow={startRequirementAnalysisWorkflow}
          onSyncAnalysisWorkflow={syncRequirementAnalysisWorkflow}
          onAnalyze={analyzeRequirement}
          onApprove={approveRequirement}
          onConvert={convertRequirement}
        />
      )}

      {mode === 'Project' && (
        <ProjectView
          requirements={requirements}
          projects={projects}
          organizations={organizations}
          departments={departments}
          selectedProjectId={selectedProjectId}
          selectedProject={selectedProject}
          overview={overview}
          candidates={candidates}
          projectForm={projectForm}
          memberForm={memberForm}
          workflowForm={workflowForm}
          matchForm={matchForm}
          loading={loading}
          onSelectProject={selectProject}
          onProjectFormChange={setProjectForm}
          onMemberFormChange={setMemberForm}
          onWorkflowFormChange={setWorkflowForm}
          onMatchFormChange={setMatchForm}
          onCreateProject={createProject}
          onUpdateProject={updateProject}
          onStatus={updateProjectStatus}
          onAddMember={addProjectMember}
          onBindWorkflow={bindWorkflow}
          onMatchActors={matchActors}
        />
      )}

      {mode === 'Delivery' && (
        <DeliveryView
          projects={projects}
          selectedProjectId={selectedProjectId}
          deliverables={asArray(overview?.deliverables)}
          form={deliverableForm}
          loading={loading}
          onSelectProject={selectProject}
          onFormChange={setDeliverableForm}
          onCreate={createDeliverable}
          onAction={changeDeliverable}
        />
      )}

      {mode === 'Cost' && (
        <CostView
          projects={projects}
          selectedProjectId={selectedProjectId}
          summary={overview?.cost_summary}
          entries={costEntries}
          form={costForm}
          loading={loading}
          onSelectProject={selectProject}
          onFormChange={setCostForm}
          onCreate={createCostEntry}
          onRefreshCost={refreshCost}
        />
      )}

      {mode === 'Feedback' && (
        <FeedbackView
          projects={projects}
          selectedProjectId={selectedProjectId}
          members={asArray(overview?.members)}
          evaluations={asArray(overview?.evaluations)}
          form={evaluationForm}
          loading={loading}
          onSelectProject={selectProject}
          onFormChange={setEvaluationForm}
          onCreate={createEvaluation}
          onClose={closeFeedback}
        />
      )}
      </div>
    </FieldPreferenceTokenContext.Provider>
  )
}

function RequirementView({
  requirements,
  documents,
  analysisWorkflows,
  workflowTemplates,
  organizations,
  departments,
  selectedRequirementId,
  selectedRequirement,
  form,
  selectedDocument,
  analysisWorkflowForm,
  loading,
  onSelectRequirement,
  onFormChange,
  onDocumentChange,
  onAnalysisWorkflowFormChange,
  onCreate,
  onUpdate,
  onUploadDocument,
  onDownloadDocument,
  onStartAnalysisWorkflow,
  onSyncAnalysisWorkflow,
  onAnalyze,
  onApprove,
  onConvert,
}: {
  requirements: Requirement[]
  documents: RequirementDocument[]
  analysisWorkflows: RequirementAnalysisWorkflow[]
  workflowTemplates: WorkflowTemplate[]
  organizations: Organization[]
  departments: Department[]
  selectedRequirementId: string
  selectedRequirement: Requirement | null
  form: typeof requirementDefaults
  selectedDocument: File | null
  analysisWorkflowForm: { workflow_template_id: string; purpose: string }
  loading: boolean
  onSelectRequirement: (id: string) => void
  onFormChange: (value: typeof requirementDefaults) => void
  onDocumentChange: (file: File | null) => void
  onAnalysisWorkflowFormChange: (value: { workflow_template_id: string; purpose: string }) => void
  onCreate: (event: FormEvent<HTMLFormElement>) => void
  onUpdate: (event: FormEvent<HTMLFormElement>) => void
  onUploadDocument: (event: FormEvent<HTMLFormElement>) => void
  onDownloadDocument: (documentID: string, fileName: string) => void
  onStartAnalysisWorkflow: (event: FormEvent<HTMLFormElement>) => void
  onSyncAnalysisWorkflow: (workflowID: string) => void
  onAnalyze: (id?: string) => void
  onApprove: (id?: string) => void
  onConvert: (id?: string) => void
}) {
  const { t } = useI18n()
  const selectedCanAnalyze = selectedRequirement ? canRunRequirementAction(selectedRequirement.status, 'analyze') : false
  const selectedCanApprove = selectedRequirement ? canRunRequirementAction(selectedRequirement.status, 'approve') : false
  const selectedCanConvert = selectedRequirement ? canRunRequirementAction(selectedRequirement.status, 'convert') : false
  return (
    <div className="space-y-5">
      <Panel icon={ClipboardList} title="需求主表">
        <ConfigurableRecordTable
          tableName="requirements"
          rows={requirements}
          selectedId={selectedRequirementId}
          onSelect={onSelectRequirement}
          columns={[
            { key: 'title', label: '标题', render: (requirement) => <span className="font-semibold text-slate-900">{requirement.title}</span> },
            { key: 'status', label: '状态', render: (requirement) => <StatusBadge label={requirement.status} tone="green" /> },
            { key: 'priority', label: '优先级', render: (requirement) => requirement.priority },
            { key: 'risk_level', label: '风险', render: (requirement) => requirement.risk_level },
            { key: 'required_level', label: '权限级别', render: (requirement) => requirement.required_level },
            { key: 'created_at', label: '创建时间', render: (requirement) => formatDate(requirement.created_at) },
          ]}
          actions={(requirement) => {
            const canAnalyze = canRunRequirementAction(requirement.status, 'analyze')
            const canApprove = canRunRequirementAction(requirement.status, 'approve')
            const canConvert = canRunRequirementAction(requirement.status, 'convert')
            return (
              <div className="flex flex-wrap justify-end gap-2">
                <ActionButton icon={Search} loading={loading} disabled={!canAnalyze} onClick={() => onAnalyze(recordKey(requirement))} label="分析需求" variant="secondary" />
                <ActionButton icon={CheckCircle2} loading={loading} disabled={!canApprove} onClick={() => onApprove(recordKey(requirement))} label="审批需求" variant="secondary" />
                <ActionButton icon={GitBranch} loading={loading} disabled={!canConvert} onClick={() => onConvert(recordKey(requirement))} label="转为项目" variant="secondary" />
              </div>
            )
          }}
        />
      </Panel>

      <div className="grid gap-5 xl:grid-cols-[0.9fr_1.1fr]">
        <Panel icon={Plus} title={selectedRequirementId ? '需求详情编辑' : '创建需求'}>
          <form className="space-y-3" onSubmit={selectedRequirementId ? onUpdate : onCreate}>
            <TextInput label="标题" value={form.title} onChange={(value) => onFormChange({ ...form, title: value })} />
            <TextArea label="描述" value={form.description} onChange={(value) => onFormChange({ ...form, description: value })} />
            <div className="grid gap-3 sm:grid-cols-3">
              <SelectInput label="优先级" value={form.priority} options={['low', 'medium', 'high', 'critical']} onChange={(value) => onFormChange({ ...form, priority: value })} />
              <SelectInput label="风险" value={form.risk_level} options={['low', 'medium', 'high', 'critical']} onChange={(value) => onFormChange({ ...form, risk_level: value })} />
              <SelectInput label="权限级别" value={form.required_level} options={['L1', 'L2', 'L3', 'L4']} onChange={(value) => onFormChange({ ...form, required_level: value })} />
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <SelectInput
                label="组织"
                value={form.organization_id}
                options={['', ...organizations.map((item) => item.id)]}
                labels={{ '': '未选择', ...Object.fromEntries(organizations.map((item) => [item.id, item.name])) }}
                onChange={(value) => onFormChange({ ...form, organization_id: value, department_id: '' })}
              />
              <SelectInput
                label="部门"
                value={form.department_id}
                options={['', ...departments.map((item) => item.id)]}
                labels={{ '': '未选择', ...Object.fromEntries(departments.map((item) => [item.id, item.name])) }}
                onChange={(value) => onFormChange({ ...form, department_id: value })}
              />
            </div>
            <div className="flex flex-wrap gap-2">
              <SubmitButton loading={loading} label={selectedRequirementId ? '保存需求' : '创建需求'} />
            </div>
          </form>
        </Panel>

        <Panel icon={Bot} title="需求操作">
          <div className="space-y-3">
            {selectedRequirement ? (
              <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
                <ListRow title={selectedRequirement.title} detail={`${selectedRequirement.status} · ${selectedRequirement.priority} · ${selectedRequirement.required_level}`} badge={selectedRequirement.risk_level} />
              </div>
            ) : (
              <EmptyText>请选择需求</EmptyText>
            )}
          <TextArea label="分析备注" value={form.analysis_notes} onChange={(value) => onFormChange({ ...form, analysis_notes: value })} />
          <TextInput label="转项目预算" value={form.convert_budget} onChange={(value) => onFormChange({ ...form, convert_budget: value })} />
          <div className="flex flex-wrap gap-2">
            <ActionButton icon={Search} loading={loading} disabled={!selectedRequirementId || !selectedCanAnalyze} onClick={() => onAnalyze()} label="分析需求" />
            <ActionButton icon={CheckCircle2} loading={loading} disabled={!selectedRequirementId || !selectedCanApprove} onClick={() => onApprove()} label="审批需求" variant="secondary" />
            <ActionButton icon={GitBranch} loading={loading} disabled={!selectedRequirementId || !selectedCanConvert} onClick={() => onConvert()} label="转为项目" variant="secondary" />
          </div>
          {selectedRequirement && <JsonBlock value={selectedRequirement.analysis} />}
          </div>
        </Panel>
      </div>

      <div className="grid gap-5 xl:grid-cols-2">
        <Panel icon={Upload} title="需求文档明细">
          <form className="space-y-3" onSubmit={onUploadDocument}>
          <label className="block">
            <span className="text-sm font-medium text-slate-700">{t('上传文档')}</span>
            <input
              type="file"
              onChange={(event) => onDocumentChange(event.target.files?.[0] ?? null)}
              className="mt-1 block w-full rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 file:mr-3 file:rounded-md file:border-0 file:bg-slate-100 file:px-3 file:py-1.5 file:text-sm file:font-semibold file:text-slate-700 hover:file:bg-slate-200"
            />
          </label>
          <SubmitButton loading={loading || !selectedRequirementId || !selectedDocument} label="上传文档" />
        </form>
        <div className="mt-4">
          <ConfigurableRecordTable
            tableName="requirement_documents"
            rows={documents}
            columns={[
              { key: 'file_name', label: '文件名', render: (document) => <span className="font-semibold text-slate-900">{document.file_name}</span> },
              { key: 'content_type', label: '类型', render: (document) => document.content_type },
              { key: 'size_bytes', label: '大小', render: (document) => `${formatter.format(document.size_bytes / 1024)} KB` },
              { key: 'created_at', label: '创建时间', render: (document) => formatDate(document.created_at) },
              { key: 'uploaded_by_type', label: '上传方', render: (document) => document.uploaded_by_type || 'human' },
            ]}
            actions={(document) => (
              <ActionButton icon={Download} loading={false} onClick={() => onDownloadDocument(recordKey(document), document.file_name)} label="下载" variant="secondary" />
            )}
          />
          </div>
        </Panel>

        <Panel icon={Play} title="需求分析流程明细">
          <form className="space-y-3" onSubmit={onStartAnalysisWorkflow}>
          <SelectInput
            label="分析流程模板"
            value={analysisWorkflowForm.workflow_template_id}
            options={workflowTemplates.map((item) => item.id)}
            labels={Object.fromEntries(workflowTemplates.map((item) => [item.id, item.name]))}
            onChange={(value) => onAnalysisWorkflowFormChange({ ...analysisWorkflowForm, workflow_template_id: value })}
          />
          <TextInput
            label="用途"
            value={analysisWorkflowForm.purpose}
            onChange={(value) => onAnalysisWorkflowFormChange({ ...analysisWorkflowForm, purpose: value })}
          />
          <SubmitButton loading={loading} disabled={!selectedRequirementId || !analysisWorkflowForm.workflow_template_id || !selectedCanAnalyze} label="启动分析流程" />
        </form>
        <div className="mt-4">
          <ConfigurableRecordTable
            tableName="requirement_analysis_workflows"
            rows={analysisWorkflows}
            columns={[
              { key: 'workflow_id', label: '工作流', render: (item) => <span className="break-all font-semibold text-slate-900">{item.workflow_id}</span> },
              { key: 'workflow_template_id', label: '模板', render: (item) => <span className="break-all">{item.workflow_template_id}</span> },
              { key: 'status', label: '状态', render: (item) => <StatusBadge label={item.status} tone="green" /> },
              { key: 'updated_at', label: '更新时间', render: (item) => formatDate(item.updated_at) },
            ]}
            actions={(item) => (
              <ActionButton icon={RefreshCw} loading={loading} disabled={!selectedCanAnalyze} onClick={() => onSyncAnalysisWorkflow(item.workflow_id)} label="同步结果" variant="secondary" />
            )}
          />
          </div>
        </Panel>
      </div>
    </div>
  )
}

function ProjectView({
  requirements,
  projects,
  organizations,
  departments,
  selectedProjectId,
  selectedProject,
  overview,
  candidates,
  projectForm,
  memberForm,
  workflowForm,
  matchForm,
  loading,
  onSelectProject,
  onProjectFormChange,
  onMemberFormChange,
  onWorkflowFormChange,
  onMatchFormChange,
  onCreateProject,
  onUpdateProject,
  onStatus,
  onAddMember,
  onBindWorkflow,
  onMatchActors,
}: {
  requirements: Requirement[]
  projects: Project[]
  organizations: Organization[]
  departments: Department[]
  selectedProjectId: string
  selectedProject: Project | null
  overview: ProjectOverview | null
  candidates: MatchCandidate[]
  projectForm: typeof projectDefaults
  memberForm: typeof memberDefaults
  workflowForm: typeof workflowDefaults
  matchForm: typeof matchDefaults
  loading: boolean
  onSelectProject: (id: string) => void
  onProjectFormChange: (value: typeof projectDefaults) => void
  onMemberFormChange: (value: typeof memberDefaults) => void
  onWorkflowFormChange: (value: typeof workflowDefaults) => void
  onMatchFormChange: (value: typeof matchDefaults) => void
  onCreateProject: (event: FormEvent<HTMLFormElement>) => void
  onUpdateProject: (event: FormEvent<HTMLFormElement>) => void
  onStatus: (status: string) => void
  onAddMember: (event: FormEvent<HTMLFormElement>) => void
  onBindWorkflow: (event: FormEvent<HTMLFormElement>) => void
  onMatchActors: () => void
}) {
  const lifecycle = overview?.lifecycle
  const allowedActions = new Set(lifecycle?.allowed_actions ?? [])
  return (
    <div className="space-y-5">
      <Panel icon={GitBranch} title="项目主表">
        <ConfigurableRecordTable
          tableName="projects"
          rows={projects}
          selectedId={selectedProjectId}
          onSelect={onSelectProject}
          columns={[
            { key: 'name', label: '项目名', render: (project) => <span className="font-semibold text-slate-900">{project.name}</span> },
            { key: 'status', label: '状态', render: (project) => <StatusBadge label={project.status} tone="green" /> },
            { key: 'risk_level', label: '风险', render: (project) => project.risk_level },
            { key: 'budget_amount', label: '预算', render: (project) => money(project.budget_amount) },
            { key: 'created_at', label: '创建时间', render: (project) => formatDate(project.created_at) },
          ]}
          actions={(project) => (
            <div className="flex flex-wrap justify-end gap-2">
              {['active', 'delivering', 'completed', 'closed'].map((status) => (
                <ActionButton key={status} icon={CheckCircle2} loading={loading} disabled={selectedProjectId !== recordKey(project) || !allowedActions.has(actionForStatus(status))} onClick={() => onStatus(status)} label={`lifecycle.status.${status}`} variant="secondary" />
              ))}
            </div>
          )}
        />
      </Panel>

      <div className="grid gap-5 xl:grid-cols-[0.9fr_1.1fr]">
        <Panel icon={Plus} title={selectedProjectId ? '项目详情编辑' : '创建项目'}>
          <form className="space-y-3" onSubmit={selectedProjectId ? onUpdateProject : onCreateProject}>
            <TextInput label="项目名" value={projectForm.name} onChange={(value) => onProjectFormChange({ ...projectForm, name: value })} />
            <TextArea label="描述" value={projectForm.description} onChange={(value) => onProjectFormChange({ ...projectForm, description: value })} />
            <SelectInput
              label="关联需求"
              value={projectForm.requirement_id}
              options={['', ...requirements.map((item) => item.id)]}
              labels={{ '': '不关联', ...Object.fromEntries(requirements.map((item) => [item.id, item.title])) }}
              onChange={(value) => onProjectFormChange({ ...projectForm, requirement_id: value })}
            />
            <div className="grid gap-3 sm:grid-cols-3">
              <SelectInput label="状态" value={projectForm.status} options={['planning', 'active', 'paused', 'delivering', 'completed']} onChange={(value) => onProjectFormChange({ ...projectForm, status: value })} />
              <SelectInput label="风险" value={projectForm.risk_level} options={['low', 'medium', 'high', 'critical']} onChange={(value) => onProjectFormChange({ ...projectForm, risk_level: value })} />
              <TextInput label="预算" value={projectForm.budget_amount} onChange={(value) => onProjectFormChange({ ...projectForm, budget_amount: value })} />
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <SelectInput
                label="组织"
                value={projectForm.organization_id}
                options={['', ...organizations.map((item) => item.id)]}
                labels={{ '': '未选择', ...Object.fromEntries(organizations.map((item) => [item.id, item.name])) }}
                onChange={(value) => onProjectFormChange({ ...projectForm, organization_id: value, department_id: '' })}
              />
              <SelectInput
                label="部门"
                value={projectForm.department_id}
                options={['', ...departments.map((item) => item.id)]}
                labels={{ '': '未选择', ...Object.fromEntries(departments.map((item) => [item.id, item.name])) }}
                onChange={(value) => onProjectFormChange({ ...projectForm, department_id: value })}
              />
            </div>
            <SubmitButton loading={loading} label={selectedProjectId ? '保存项目' : '创建项目'} />
          </form>
        </Panel>

        <Panel icon={Activity} title="项目详情与操作">
          {selectedProject ? (
            <div className="space-y-4">
              <ListRow title={selectedProject.name} detail={`${selectedProject.priority} · ${selectedProject.required_level} · ${money(selectedProject.budget_amount)}`} badge={selectedProject.status} />
              <div className="grid gap-3 sm:grid-cols-4">
                <Metric label="成员" value={String(asArray(overview?.members).length)} />
                <Metric label="流程" value={String(asArray(overview?.workflows).length)} />
                <Metric label="交付" value={String(asArray(overview?.deliverables).length)} />
                <Metric label="评估" value={String(asArray(overview?.evaluations).length)} />
              </div>
              {lifecycle && <LifecycleSummary lifecycle={lifecycle} />}
              <div className="flex flex-wrap gap-2">
                {['active', 'delivering', 'completed', 'closed'].map((status) => (
                  <ActionButton key={status} icon={CheckCircle2} loading={loading} disabled={!allowedActions.has(actionForStatus(status))} onClick={() => onStatus(status)} label={`lifecycle.status.${status}`} variant="secondary" />
                ))}
              </div>
            </div>
          ) : (
            <EmptyText>暂无项目</EmptyText>
          )}
        </Panel>
      </div>

      <div className="grid gap-5 xl:grid-cols-[0.85fr_1.15fr]">
        <Panel icon={Users} title="成员与流程">
          <div className="grid gap-5 xl:grid-cols-2">
            <form className="space-y-3" onSubmit={onAddMember}>
              <TextInput label="成员 Actor ID" value={memberForm.member_actor_id} onChange={(value) => onMemberFormChange({ ...memberForm, member_actor_id: value })} />
              <SelectInput label="Actor 类型" value={memberForm.member_actor_type} options={['internal_human', 'external_human', 'internal_agent', 'external_agent']} onChange={(value) => onMemberFormChange({ ...memberForm, member_actor_type: value })} />
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput label="角色" value={memberForm.role} onChange={(value) => onMemberFormChange({ ...memberForm, role: value })} />
                <TextInput label="职位" value={memberForm.title} onChange={(value) => onMemberFormChange({ ...memberForm, title: value })} />
              </div>
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput label="投入比例" value={memberForm.allocation_percent} onChange={(value) => onMemberFormChange({ ...memberForm, allocation_percent: value })} />
                <TextInput label="成本费率" value={memberForm.cost_rate} onChange={(value) => onMemberFormChange({ ...memberForm, cost_rate: value })} />
              </div>
              <TextInput label="能力" value={memberForm.capabilities} onChange={(value) => onMemberFormChange({ ...memberForm, capabilities: value })} />
              <SubmitButton loading={loading || !selectedProjectId || !memberForm.member_actor_id} label="加入项目" />
            </form>

            <form className="space-y-3" onSubmit={onBindWorkflow}>
              <TextInput label="工作流实例 ID" value={workflowForm.workflow_id} onChange={(value) => onWorkflowFormChange({ ...workflowForm, workflow_id: value })} />
              <TextInput label="工作流模板 ID" value={workflowForm.workflow_template_id} onChange={(value) => onWorkflowFormChange({ ...workflowForm, workflow_template_id: value })} />
              <TextInput label="用途" value={workflowForm.purpose} onChange={(value) => onWorkflowFormChange({ ...workflowForm, purpose: value })} />
              <SubmitButton loading={loading || !selectedProjectId || (!workflowForm.workflow_id && !workflowForm.workflow_template_id)} label="绑定流程" />
            </form>
          </div>
          <div className="mt-5 grid gap-5 xl:grid-cols-2">
            <ConfigurableRecordTable
              tableName="project_members"
              rows={asArray(overview?.members)}
              columns={[
                { key: 'title', label: '职位', render: (member) => <span className="font-semibold text-slate-900">{member.title || member.role}</span> },
                { key: 'actor_type', label: 'Actor 类型', render: (member) => member.actor_type },
                { key: 'allocation_percent', label: '投入比例', render: (member) => `${member.allocation_percent}%` },
                { key: 'permission_level', label: '权限级别', render: (member) => member.permission_level },
                { key: 'status', label: '状态', render: (member) => <StatusBadge label={member.status} tone="green" /> },
              ]}
            />
            <ConfigurableRecordTable
              tableName="project_workflows"
              rows={asArray(overview?.workflows)}
              columns={[
                { key: 'workflow_id', label: '工作流', render: (item) => <span className="break-all font-semibold text-slate-900">{item.workflow_id}</span> },
                { key: 'workflow_template_id', label: '模板', render: (item) => item.workflow_template_id || '-' },
                { key: 'purpose', label: '用途', render: (item) => item.purpose },
                { key: 'status', label: '状态', render: (item) => <StatusBadge label={item.status} tone="green" /> },
              ]}
            />
          </div>
        </Panel>
      </div>

      <div className="grid gap-5 xl:grid-cols-2">
        <Panel icon={Search} title="候选匹配">
          <div className="space-y-3">
            <TextArea label="任务描述" value={matchForm.task_description} onChange={(value) => onMatchFormChange({ ...matchForm, task_description: value })} />
            <TextInput label="所需能力" value={matchForm.required_capabilities} onChange={(value) => onMatchFormChange({ ...matchForm, required_capabilities: value })} />
            <div className="grid gap-3 sm:grid-cols-2">
              <SelectInput label="权限级别" value={matchForm.required_level} options={['L1', 'L2', 'L3', 'L4']} onChange={(value) => onMatchFormChange({ ...matchForm, required_level: value })} />
              <SelectInput label="风险" value={matchForm.risk_level} options={['low', 'medium', 'high', 'critical']} onChange={(value) => onMatchFormChange({ ...matchForm, risk_level: value })} />
            </div>
            <ActionButton icon={Search} loading={loading} disabled={!selectedProjectId} onClick={onMatchActors} label="匹配候选" />
          </div>
        </Panel>
        <Panel icon={Users} title="匹配结果">
          <List>
            {candidates.map((candidate) => (
              <ListRow key={candidate.membership_id} title={candidate.member_name} detail={`${candidate.member_type} · ${candidate.reason}`} badge={`${percent(candidate.score)} · ${candidate.requires_approval ? 'approval' : candidate.access_decision}`} />
            ))}
          </List>
        </Panel>
      </div>
    </div>
  )
}

function DeliveryView({
  projects,
  selectedProjectId,
  deliverables,
  form,
  loading,
  onSelectProject,
  onFormChange,
  onCreate,
  onAction,
}: {
  projects: Project[]
  selectedProjectId: string
  deliverables: Deliverable[]
  form: typeof deliverableDefaults
  loading: boolean
  onSelectProject: (id: string) => void
  onFormChange: (value: typeof deliverableDefaults) => void
  onCreate: (event: FormEvent<HTMLFormElement>) => void
  onAction: (id: string, action: 'submit' | 'accept' | 'reject') => void
}) {
  return (
    <div className="space-y-5">
      <Panel icon={FileCheck2} title="交付主表">
        <SelectInput label="项目" value={selectedProjectId} options={projects.map(recordKey)} labels={Object.fromEntries(projects.map((item) => [recordKey(item), item.name]))} onChange={onSelectProject} />
        <div className="mt-4">
          <ConfigurableRecordTable
            tableName="deliverables"
            rows={deliverables}
            columns={[
              { key: 'name', label: '名称', render: (deliverable) => <span className="font-semibold text-slate-900">{deliverable.name}</span> },
              { key: 'deliverable_type', label: '类型', render: (deliverable) => deliverable.deliverable_type },
              { key: 'version', label: '版本', render: (deliverable) => deliverable.version },
              { key: 'status', label: '状态', render: (deliverable) => <StatusBadge label={deliverable.status} tone="green" /> },
              { key: 'created_at', label: '创建时间', render: (deliverable) => formatDate(deliverable.created_at) },
            ]}
            actions={(deliverable) => (
              <div className="flex flex-wrap justify-end gap-2">
                <ActionButton icon={Send} loading={loading} disabled={deliverable.status !== 'draft' && deliverable.status !== 'rejected'} onClick={() => onAction(recordKey(deliverable), 'submit')} label="提交" variant="secondary" />
                <ActionButton icon={CheckCircle2} loading={loading} disabled={deliverable.status !== 'submitted'} onClick={() => onAction(recordKey(deliverable), 'accept')} label="验收" variant="secondary" />
                <ActionButton icon={ClipboardCheck} loading={loading} disabled={deliverable.status !== 'submitted'} onClick={() => onAction(recordKey(deliverable), 'reject')} label="退回" variant="secondary" />
              </div>
            )}
          />
        </div>
      </Panel>
      <Panel icon={Plus} title="交付详情">
        <form className="space-y-3" onSubmit={onCreate}>
          <TextInput label="名称" value={form.name} onChange={(value) => onFormChange({ ...form, name: value })} />
          <div className="grid gap-3 sm:grid-cols-2">
            <TextInput label="类型" value={form.deliverable_type} onChange={(value) => onFormChange({ ...form, deliverable_type: value })} />
            <TextInput label="版本" value={form.version} onChange={(value) => onFormChange({ ...form, version: value })} />
          </div>
          <TextInput label="URI" value={form.uri} onChange={(value) => onFormChange({ ...form, uri: value })} />
          <SelectInput label="状态" value={form.status} options={['draft', 'submitted']} onChange={(value) => onFormChange({ ...form, status: value })} />
          <SubmitButton loading={loading || !selectedProjectId} label="创建交付物" />
        </form>
      </Panel>
    </div>
  )
}

function CostView({
  projects,
  selectedProjectId,
  summary,
  entries,
  form,
  loading,
  onSelectProject,
  onFormChange,
  onCreate,
  onRefreshCost,
}: {
  projects: Project[]
  selectedProjectId: string
  summary?: CostSummary
  entries: CostEntry[]
  form: typeof costDefaults
  loading: boolean
  onSelectProject: (id: string) => void
  onFormChange: (value: typeof costDefaults) => void
  onCreate: (event: FormEvent<HTMLFormElement>) => void
  onRefreshCost: () => void
}) {
  return (
    <div className="space-y-5">
      <Panel icon={Activity} title="成本主表">
        <SelectInput label="项目" value={selectedProjectId} options={projects.map(recordKey)} labels={Object.fromEntries(projects.map((item) => [recordKey(item), item.name]))} onChange={onSelectProject} />
        <div className="mt-4">
          <ConfigurableRecordTable
            tableName="project_cost_entries"
            rows={entries}
            columns={[
              { key: 'amount', label: '金额', render: (entry) => <span className="font-semibold text-slate-900">{money(entry.amount, entry.currency)}</span> },
              { key: 'source_type', label: '来源类型', render: (entry) => entry.source_type },
              { key: 'actor_type', label: 'Actor 类型', render: (entry) => entry.actor_type || 'manual' },
              { key: 'description', label: '说明', render: (entry) => entry.description },
              { key: 'occurred_at', label: '发生时间', render: (entry) => formatDate(entry.occurred_at) },
            ]}
          />
        </div>
      </Panel>
      <div className="grid gap-5 xl:grid-cols-[0.85fr_1.15fr]">
        <Panel icon={Coins} title="成本入账">
          <form className="space-y-3" onSubmit={onCreate}>
            <div className="grid gap-3 sm:grid-cols-2">
              <TextInput label="来源类型" value={form.source_type} onChange={(value) => onFormChange({ ...form, source_type: value })} />
              <TextInput label="金额" value={form.amount} onChange={(value) => onFormChange({ ...form, amount: value })} />
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <TextInput label="Actor ID" value={form.entry_actor_id} onChange={(value) => onFormChange({ ...form, entry_actor_id: value })} />
              <SelectInput label="Actor 类型" value={form.entry_actor_type} options={['internal_human', 'external_human', 'internal_agent', 'external_agent']} onChange={(value) => onFormChange({ ...form, entry_actor_type: value })} />
            </div>
            <TextInput label="说明" value={form.description} onChange={(value) => onFormChange({ ...form, description: value })} />
            <div className="flex flex-wrap gap-2">
              <SubmitButton loading={loading || !selectedProjectId} label="写入成本" />
              <ActionButton icon={RefreshCw} loading={loading} disabled={!selectedProjectId} onClick={onRefreshCost} label="刷新成员成本" variant="secondary" />
            </div>
          </form>
        </Panel>
        <Panel icon={BarChart3} title="成本汇总">
          {summary ? (
            <div className="space-y-4">
              <div className="grid gap-3 sm:grid-cols-3">
                <Metric label="总成本" value={money(summary.total_amount, summary.currency)} />
                <Metric label="预算" value={money(summary.budget_amount, summary.currency)} />
                <Metric label="差额" value={money(summary.budget_variance, summary.currency)} />
              </div>
              <List>
                {asArray(summary.by_source).map((item) => (
                  <ListRow key={item.source_type} title={item.source_type} detail={`${item.count} entries`} badge={money(item.amount, summary.currency)} />
                ))}
              </List>
            </div>
          ) : (
            <EmptyText>暂无成本汇总</EmptyText>
          )}
        </Panel>
      </div>
    </div>
  )
}

function FeedbackView({
  projects,
  selectedProjectId,
  members,
  evaluations,
  form,
  loading,
  onSelectProject,
  onFormChange,
  onCreate,
  onClose,
}: {
  projects: Project[]
  selectedProjectId: string
  members: ProjectMember[]
  evaluations: ProjectEvaluation[]
  form: typeof evaluationDefaults
  loading: boolean
  onSelectProject: (id: string) => void
  onFormChange: (value: typeof evaluationDefaults) => void
  onCreate: (event: FormEvent<HTMLFormElement>) => void
  onClose: () => void
}) {
  return (
    <div className="space-y-5">
      <Panel icon={Activity} title="反馈主表">
        <SelectInput label="项目" value={selectedProjectId} options={projects.map(recordKey)} labels={Object.fromEntries(projects.map((item) => [recordKey(item), item.name]))} onChange={onSelectProject} />
        <div className="mt-4">
          <ConfigurableRecordTable
            tableName="project_evaluations"
            rows={evaluations}
            columns={[
              { key: 'actor_id', label: 'Actor ID', render: (evaluation) => evaluation.actor_id || evaluation.capability_id || 'project' },
              { key: 'evaluator_type', label: '评估方', render: (evaluation) => evaluation.evaluator_type },
              { key: 'quality_score', label: '质量', render: (evaluation) => percent(evaluation.quality_score) },
              { key: 'delivery_score', label: '交付', render: (evaluation) => percent(evaluation.delivery_score) },
              { key: 'overall_score', label: '综合', render: (evaluation) => <span className="font-semibold text-slate-900">{percent(evaluation.overall_score)}</span> },
              { key: 'created_at', label: '创建时间', render: (evaluation) => formatDate(evaluation.created_at) },
            ]}
          />
        </div>
      </Panel>
      <Panel icon={Star} title="反馈详情">
        <form className="space-y-3" onSubmit={onCreate}>
          <SelectInput
            label="被评估成员"
            value={form.evaluated_actor_id}
            options={['', ...members.map((item) => item.actor_id)]}
            labels={{ '': '项目整体', ...Object.fromEntries(members.map((item) => [item.actor_id, `${item.title || item.role} · ${item.actor_type}`])) }}
            onChange={(value) => {
              const member = members.find((item) => item.actor_id === value)
              onFormChange({ ...form, evaluated_actor_id: value, evaluated_actor_type: member?.actor_type ?? form.evaluated_actor_type })
            }}
          />
          <div className="grid gap-3 sm:grid-cols-2">
            <SelectInput label="Actor 类型" value={form.evaluated_actor_type} options={['internal_human', 'external_human', 'internal_agent', 'external_agent']} onChange={(value) => onFormChange({ ...form, evaluated_actor_type: value })} />
            <TextInput label="能力 ID" value={form.capability_id} onChange={(value) => onFormChange({ ...form, capability_id: value })} />
          </div>
          <div className="grid gap-3 sm:grid-cols-4">
            <TextInput label="质量" value={form.quality_score} onChange={(value) => onFormChange({ ...form, quality_score: value })} />
            <TextInput label="交付" value={form.delivery_score} onChange={(value) => onFormChange({ ...form, delivery_score: value })} />
            <TextInput label="成本" value={form.cost_score} onChange={(value) => onFormChange({ ...form, cost_score: value })} />
            <TextInput label="协作" value={form.collaboration_score} onChange={(value) => onFormChange({ ...form, collaboration_score: value })} />
          </div>
          <TextArea label="结论" value={form.conclusion} onChange={(value) => onFormChange({ ...form, conclusion: value })} />
          <div className="flex flex-wrap gap-2">
            <SubmitButton loading={loading || !selectedProjectId} label="提交评估" />
            <ActionButton icon={CheckCircle2} loading={loading} disabled={!selectedProjectId} onClick={onClose} label="闭环反馈" variant="secondary" />
          </div>
        </form>
      </Panel>
    </div>
  )
}

function Panel({ icon: Icon, title, children }: { icon: typeof ClipboardList; title: string; children: ReactNode }) {
  const { t } = useI18n()
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="mb-4 flex items-center gap-2">
        <Icon className="h-5 w-5 text-slate-500" />
        <h3 className="text-base font-semibold text-slate-950">{t(title)}</h3>
      </div>
      {children}
    </section>
  )
}

type RecordColumn<T> = {
  key: string
  label: string
  render: (row: T) => ReactNode
}

function ConfigurableRecordTable<T extends { id: string; master_key?: string }>({
  tableName,
  rows,
  columns,
  selectedId,
  onSelect,
  actions,
}: {
  tableName: string
  rows: T[]
  columns: Array<RecordColumn<T>>
  selectedId?: string
  onSelect?: (id: string) => void
  actions?: (row: T) => ReactNode
}) {
  const { t } = useI18n()
  const token = useContext(FieldPreferenceTokenContext)
  const storageKey = `meta_org.field_preferences.${tableName}.v1`
  const columnKeys = columns.map((column) => column.key).join('|')
  const orderedFields = useMemo(() => columnKeys.split('|').filter(Boolean), [columnKeys])
  const knownFields = useMemo(() => new Set(orderedFields), [orderedFields])
  const defaultVisible = orderedFields
  const [visibleFields, setVisibleFields] = useState<string[]>(() => {
    if (typeof window === 'undefined') return defaultVisible
    const raw = window.localStorage.getItem(storageKey)
    if (!raw) return defaultVisible
    try {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed)) {
        const known = new Set(columns.map((column) => column.key))
        const next = parsed.filter((item) => typeof item === 'string' && known.has(item))
        return next.length > 0 ? next : defaultVisible
      }
    } catch {
      return defaultVisible
    }
    return defaultVisible
  })

  useEffect(() => {
    if (!token) return
    let cancelled = false
    getUserFieldPreference(token, tableName)
      .then((preference) => {
        if (cancelled) return
        const next = preference.visible_fields.filter((field) => knownFields.has(field))
        if (next.length > 0) {
          setVisibleFields(next)
          if (typeof window !== 'undefined') {
            window.localStorage.setItem(storageKey, JSON.stringify(next))
          }
        }
      })
      .catch(() => undefined)
    return () => {
      cancelled = true
    }
  }, [columnKeys, knownFields, storageKey, tableName, token])

  const visibleColumns = columns.filter((column) => visibleFields.includes(column.key))

  function toggleField(field: string) {
    const next = visibleFields.includes(field)
      ? visibleFields.filter((item) => item !== field)
      : [...visibleFields, field]
    const safeNext = next.length > 0 ? next : [field]
    setVisibleFields(safeNext)
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(storageKey, JSON.stringify(safeNext))
    }
    if (token) {
      saveUserFieldPreference(token, tableName, {
        visible_fields: safeNext,
        field_order: orderedFields,
        field_widths: {},
      }).catch(() => undefined)
    }
  }

  return (
    <div className="space-y-3">
      <details className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2">
        <summary className="cursor-pointer text-sm font-semibold text-slate-700">{t('table.customizeFields')}</summary>
        <div className="mt-3 flex flex-wrap gap-2">
          {columns.map((column) => (
            <label key={column.key} className="inline-flex items-center gap-2 rounded-md border border-slate-200 bg-white px-2 py-1 text-xs font-medium text-slate-600">
              <input type="checkbox" checked={visibleFields.includes(column.key)} onChange={() => toggleField(column.key)} />
              {t(column.label)}
            </label>
          ))}
        </div>
      </details>
      <div className="overflow-x-auto rounded-lg border border-slate-200">
        <table className="min-w-full divide-y divide-slate-200 text-sm">
          <thead className="bg-slate-50">
            <tr>
              {visibleColumns.map((column) => (
                <th key={column.key} scope="col" className="px-3 py-2 text-left text-xs font-semibold uppercase tracking-normal text-slate-500">
                  {t(column.label)}
                </th>
              ))}
              {actions && <th scope="col" className="px-3 py-2 text-right text-xs font-semibold uppercase tracking-normal text-slate-500">{t('table.actions')}</th>}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100 bg-white">
            {rows.map((row) => {
              const rowKey = recordKey(row)
              const active = selectedId === rowKey || selectedId === row.id
              return (
                <tr key={rowKey} className={`${active ? 'bg-orange-50' : 'bg-white'} ${onSelect ? 'cursor-pointer hover:bg-slate-50' : ''}`} onClick={() => onSelect?.(rowKey)}>
                  {visibleColumns.map((column) => (
                    <td key={column.key} className="max-w-[18rem] px-3 py-2 align-top text-slate-700">
                      {column.render(row)}
                    </td>
                  ))}
                  {actions && (
                    <td className="px-3 py-2 text-right align-top" onClick={(event) => event.stopPropagation()}>
                      {actions(row)}
                    </td>
                  )}
                </tr>
              )
            })}
            {rows.length === 0 && (
              <tr>
                <td className="px-3 py-6 text-center text-sm text-slate-500" colSpan={visibleColumns.length + (actions ? 1 : 0)}>
                  {t('table.empty')}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function LifecycleSummary({ lifecycle }: { lifecycle: ProjectLifecycle }) {
  const { t } = useI18n()
  return (
    <div className="grid gap-3 rounded-lg border border-slate-200 bg-slate-50 p-3 text-sm">
      <div className="flex flex-wrap items-center gap-2">
        <StatusBadge label={t(`metaresource.stage.${lifecycle.pdca_stage}`)} tone="green" />
        <span className="font-semibold text-slate-900">{t('lifecycle.nextAction')}</span>
        <span className="text-slate-600">{t(`lifecycle.action.${lifecycle.next_action}`)}</span>
      </div>
      {lifecycle.pdca_cycle_id && (
        <p className="break-all text-xs text-slate-500">
          {t('lifecycle.pdcaCycle')}: {lifecycle.pdca_cycle_id}
        </p>
      )}
      {lifecycle.blockers.length > 0 && (
        <div className="space-y-1">
          <p className="font-semibold text-amber-700">{t('lifecycle.blockers')}</p>
          <ul className="grid gap-1 text-slate-600">
            {lifecycle.blockers.map((blocker) => (
              <li key={blocker}>- {t(`lifecycle.blocker.${blocker}`)}</li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}

function TextInput({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  const { t } = useI18n()
  return (
    <label className="block">
      <span className="text-sm font-medium text-slate-700">{t(label)}</span>
      <input value={value} onChange={(event) => onChange(event.target.value)} className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200" />
    </label>
  )
}

function TextArea({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  const { t } = useI18n()
  return (
    <label className="block">
      <span className="text-sm font-medium text-slate-700">{t(label)}</span>
      <textarea value={value} onChange={(event) => onChange(event.target.value)} className="mt-1 h-24 w-full resize-y rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200" />
    </label>
  )
}

function SelectInput({
  label,
  value,
  onChange,
  options,
  labels,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  options: string[]
  labels?: Record<string, string>
}) {
  const { t } = useI18n()
  return (
    <label className="block">
      <span className="text-sm font-medium text-slate-700">{t(label)}</span>
      <select value={value} onChange={(event) => onChange(event.target.value)} className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200">
        {options.map((option) => (
          <option key={option || 'empty'} value={option}>
            {t(labels?.[option] ?? option)}
          </option>
        ))}
      </select>
    </label>
  )
}

function SubmitButton({ loading, disabled, label }: { loading: boolean; disabled?: boolean; label: string }) {
  const { t } = useI18n()
  return (
    <button type="submit" disabled={loading || disabled} className="inline-flex h-10 items-center gap-2 rounded-lg bg-slate-950 px-3 text-sm font-semibold text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60">
      {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
      {t(label)}
    </button>
  )
}

function ActionButton({
  icon: Icon,
  loading,
  disabled,
  onClick,
  label,
  variant = 'primary',
}: {
  icon: typeof ClipboardList
  loading: boolean
  disabled?: boolean
  onClick: () => void
  label: string
  variant?: 'primary' | 'secondary'
}) {
  const { t } = useI18n()
  const className =
    variant === 'primary'
      ? 'bg-slate-950 text-white hover:bg-slate-800'
      : 'border border-slate-300 text-slate-700 hover:bg-slate-100'
  return (
    <button type="button" disabled={loading || disabled} onClick={onClick} className={`inline-flex h-10 items-center gap-2 rounded-lg px-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-60 ${className}`}>
      {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Icon className="h-4 w-4" />}
      {t(label)}
    </button>
  )
}

function List({ children }: { children: ReactNode }) {
  return <div className="divide-y divide-slate-100">{children}</div>
}

function ListRow({
  title,
  detail,
  badge,
  active,
  onClick,
  onAction,
  actionLabel,
}: {
  title: string
  detail: string
  badge?: string
  active?: boolean
  onClick?: () => void
  onAction?: () => void
  actionLabel?: string
}) {
  const { t } = useI18n()
  const content = (
    <>
      <div className="min-w-0">
        <p className="truncate text-sm font-semibold text-slate-950">{title}</p>
        <p className="mt-1 line-clamp-2 text-sm text-slate-500">{detail}</p>
      </div>
      <div className="flex items-center gap-2 sm:justify-end">
        {badge && <StatusBadge label={badge} tone={badge.includes('deny') || badge.includes('rejected') ? 'red' : badge.includes('approval') || badge.includes('draft') ? 'amber' : 'green'} />}
        {onAction && (
          <button
            type="button"
            onClick={(event) => {
              event.stopPropagation()
              onAction()
            }}
            className="inline-flex h-7 items-center gap-1.5 rounded-md border border-slate-300 px-2.5 text-xs font-semibold text-slate-700 hover:bg-slate-100"
          >
            {actionLabel === '下载' ? <Download className="h-3.5 w-3.5" /> : <RefreshCw className="h-3.5 w-3.5" />}
            {t(actionLabel || '操作')}
          </button>
        )}
      </div>
    </>
  )

  if (onClick) {
    return (
      <button type="button" onClick={onClick} className={`grid w-full gap-2 py-3 text-left first:pt-0 last:pb-0 sm:grid-cols-[1fr_auto] ${active ? 'text-slate-950' : ''}`}>
        {content}
      </button>
    )
  }

  return (
    <div className={`grid w-full gap-2 py-3 text-left first:pt-0 last:pb-0 sm:grid-cols-[1fr_auto] ${active ? 'text-slate-950' : ''}`}>
      {content}
    </div>
  )
}

function StatusBadge({ label, tone }: { label: string; tone: 'green' | 'amber' | 'red' }) {
  const { t } = useI18n()
  const toneClass = {
    green: 'border-emerald-200 bg-emerald-50 text-emerald-700',
    amber: 'border-amber-200 bg-amber-50 text-amber-700',
    red: 'border-red-200 bg-red-50 text-red-700',
  }[tone]

  return <span className={`inline-flex h-7 max-w-[180px] items-center truncate rounded-md border px-2.5 text-xs font-semibold ${toneClass}`}>{t(label)}</span>
}

function Metric({ label, value }: { label: string; value: string }) {
  const { t } = useI18n()
  return (
    <div className="rounded-lg border border-slate-200 p-3">
      <p className="text-xs font-medium text-slate-500">{t(label)}</p>
      <p className="mt-2 truncate text-lg font-semibold text-slate-950">{value}</p>
    </div>
  )
}

function JsonBlock({ value }: { value: unknown }) {
  return (
    <pre className="max-h-[280px] overflow-auto rounded-lg border border-slate-200 bg-slate-50 p-3 text-sm text-slate-800">
      {JSON.stringify(value, null, 2)}
    </pre>
  )
}

function EmptyText({ children }: { children: ReactNode }) {
  const { t } = useI18n()
  return <p className="text-sm text-slate-500">{typeof children === 'string' ? t(children) : children}</p>
}
