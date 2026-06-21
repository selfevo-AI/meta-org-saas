'use client'

import {
  Bot,
  Building2,
  GitBranch,
  Loader2,
  Plus,
  RefreshCw,
  Save,
  ShieldCheck,
  Sparkles,
  UserPlus,
  Users,
} from 'lucide-react'
import { FormEvent, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import { apiRequest, type AIAgent, type MetaResource } from '@/lib/api'
import { useI18n } from '@/lib/i18n'

interface OrganizationWorkspaceProps {
  token: string
  currentUserId?: string | null
  externalSelection?: {
    targetType: string
    targetID?: string
    record?: Record<string, unknown>
  } | null
}

interface Organization {
  id: string
  name: string
  description?: string
  created_at: string
  updated_at: string
}

interface Department {
  id: string
  organization_id: string
  parent_id?: string
  name: string
  code?: string
  description?: string
  status: string
  sort_order: number
  metadata: Record<string, unknown>
  children?: Department[]
  positions?: Position[]
}

interface Position {
  id: string
  organization_id: string
  department_id: string
  name: string
  code?: string
  description?: string
  status: string
  sort_order: number
  permission_level: string
  required_capabilities: string[]
  assignments?: PositionAssignment[]
}

interface PositionAssignment {
  id: string
  position_id: string
  meta_resource_id?: string
  meta_resource_name?: string
  meta_resource_type?: string
  actor_id: string
  actor_type: 'internal_human' | 'external_human' | 'internal_agent' | 'external_agent'
  actor_name?: string
  assignment_type: string
  allocation_percent: number
  status: string
}

interface TaskMatrixAssignment {
  id: string
  task_id: string
  position_id: string
  position_name?: string
  position_assignment_id?: string
  meta_resource_id: string
  meta_resource_name?: string
  meta_resource_type?: string
  actor_id: string
  actor_type: string
  role_in_task: string
  allocation_percent: number
  status: string
}

interface ExternalMember {
  id: string
  name: string
  email?: string
  vendor?: string
  contract_type?: string
  status: string
}

interface OrganizationMembership {
  id: string
  department_id: string
  member_type: 'internal' | 'external' | 'agent'
  user_id?: string
  external_member_id?: string
  agent_id?: string
  member_name?: string
  member_email?: string
  title?: string
  authority_tier?: string
  status: string
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

interface PermissionChangeRequest {
  id: string
  organization_id: string
  membership_id: string
  requested_by?: string
  requested_by_type: string
  requested_change: Partial<OrganizationMembership>
  reason?: string
  status: string
  review_reason?: string
  created_at: string
  updated_at: string
}

interface OrganizationAccessRule {
  id: string
  organization_id: string
  scope_type: string
  scope_id?: string
  resource_type: string
  resource_key?: string
  action: string
  actor_type: string
  actor_id?: string
  authority_tier?: string
  behavior: string
  required_level: string
  priority: number
  status: string
  reason?: string
}

interface AgentPlan {
  organizationName: string
  description: string
  departments: Array<{
    name: string
    code: string
    description: string
  }>
}

const emptyOrgForm = { name: '', description: '' }
const emptyDepartmentForm = { name: '', code: '', description: '', status: 'active', sort_order: '10' }
const emptyPositionForm = {
  name: '',
  code: '',
  description: '',
  permission_level: 'L1',
  required_capabilities: '',
  status: 'active',
  sort_order: '10',
}
const emptyExternalForm = { name: '', email: '', vendor: '', contract_type: '', status: 'active' }
const emptyAgentForm = {
  name: '',
  model_type: 'gpt-4.1',
  capabilities: 'analysis, review',
  permission_level: 'L2',
  agent_origin: 'internal',
  provider: 'OpenAI',
  service_class: 'model',
  vendor: '',
  contract_ref: '',
  risk_level: 'medium',
}

function deferStateUpdate(callback: () => void): () => void {
  if (typeof window === 'undefined') {
    callback()
    return () => undefined
  }
  const timeout = window.setTimeout(callback, 0)
  return () => window.clearTimeout(timeout)
}

export function OrganizationWorkspace({ token, currentUserId, externalSelection }: OrganizationWorkspaceProps) {
  const { t } = useI18n()
  const [organizations, setOrganizations] = useState<Organization[]>([])
  const [selectedOrgId, setSelectedOrgId] = useState<string>('')
  const [departmentTree, setDepartmentTree] = useState<Department[]>([])
  const [selectedDepartmentId, setSelectedDepartmentId] = useState<string>('')
  const [selectedPositionId, setSelectedPositionId] = useState<string>('')
  const [members, setMembers] = useState<OrganizationMembership[]>([])
  const [permissionRequests, setPermissionRequests] = useState<PermissionChangeRequest[]>([])
  const [accessRules, setAccessRules] = useState<OrganizationAccessRule[]>([])
  const [externalMembers, setExternalMembers] = useState<ExternalMember[]>([])
  const [agents, setAgents] = useState<AIAgent[]>([])
  const [metaResources, setMetaResources] = useState<MetaResource[]>([])
  const [orgForm, setOrgForm] = useState(emptyOrgForm)
  const [departmentForm, setDepartmentForm] = useState(emptyDepartmentForm)
  const [positionForm, setPositionForm] = useState(emptyPositionForm)
  const [positionAssignments, setPositionAssignments] = useState<PositionAssignment[]>([])
  const [externalForm, setExternalForm] = useState(emptyExternalForm)
  const [agentForm, setAgentForm] = useState(emptyAgentForm)
  const [createdAgentKey, setCreatedAgentKey] = useState<string | null>(null)
  const [memberForm, setMemberForm] = useState({
    member_type: 'internal',
    user_id: currentUserId ?? '',
    external_member_id: '',
    agent_id: '',
    title: '',
  })
  const [permissionRequestForm, setPermissionRequestForm] = useState({
    membership_id: '',
    authority_tier: 'organization_admin',
    status: 'active',
    reason: '',
  })
  const [accessRuleForm, setAccessRuleForm] = useState({
    scope_type: 'project',
    scope_id: '',
    resource_type: 'project',
    resource_key: '',
    action: 'read',
    actor_type: '*',
    actor_id: '',
    authority_tier: '',
    behavior: 'allow',
    required_level: 'L1',
    priority: '0',
    reason: '',
  })
  const [positionAssignmentForm, setPositionAssignmentForm] = useState<{
    actor_type: PositionAssignment['actor_type']
    actor_id: string
    meta_resource_id: string
    assignment_type: string
    allocation_percent: string
  }>({
    actor_type: 'internal_human',
    actor_id: currentUserId ?? '',
    meta_resource_id: '',
    assignment_type: 'candidate',
    allocation_percent: '100',
  })
  const [taskMatrixForm, setTaskMatrixForm] = useState({
    task_id: '',
    position_assignment_id: '',
    role_in_task: 'owner',
    allocation_percent: '100',
  })
  const [taskMatrixAssignments, setTaskMatrixAssignments] = useState<TaskMatrixAssignment[]>([])
  const [agentPrompt, setAgentPrompt] = useState('')
  const [agentPlan, setAgentPlan] = useState<AgentPlan | null>(null)
  const [matchTask, setMatchTask] = useState('review launch readiness')
  const [matchCapabilities, setMatchCapabilities] = useState('review')
  const [matchLevel, setMatchLevel] = useState('L2')
  const [matchRisk, setMatchRisk] = useState('medium')
  const [candidates, setCandidates] = useState<MatchCandidate[]>([])
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const selectedOrganization = asArray<Organization>(organizations).find((org) => org.id === selectedOrgId)
  const selectedDepartment = useMemo(
    () => findDepartment(asArray<Department>(departmentTree), selectedDepartmentId),
    [departmentTree, selectedDepartmentId],
  )
  const selectedPosition = useMemo(
    () => asArray<Position>(selectedDepartment?.positions).find((position) => position.id === selectedPositionId) ?? null,
    [selectedDepartment, selectedPositionId],
  )
  const selectedAgent = asArray<AIAgent>(agents).find((agent) => agent.id === memberForm.agent_id)
  const assignmentActorOptions = useMemo(() => {
    if (positionAssignmentForm.actor_type === 'external_human') {
      return asArray<ExternalMember>(externalMembers).map((member) => ({
        id: member.id,
        label: `${member.name}${member.vendor ? ` · ${member.vendor}` : ''}`,
      }))
    }
    if (positionAssignmentForm.actor_type === 'internal_agent') {
      return asArray<AIAgent>(agents)
        .filter((agent) => agent.agent_origin === 'internal')
        .map((agent) => ({ id: agent.id, label: `${agent.name} · ${agent.permission_level}` }))
    }
    if (positionAssignmentForm.actor_type === 'external_agent') {
      return asArray<AIAgent>(agents)
        .filter((agent) => agent.agent_origin === 'external')
        .map((agent) => ({ id: agent.id, label: `${agent.name} · ${agent.provider ?? agent.vendor ?? agent.permission_level}` }))
    }
    return currentUserId ? [{ id: currentUserId, label: t('当前登录用户') }] : []
  }, [agents, currentUserId, externalMembers, positionAssignmentForm.actor_type, t])
  const assignmentMetaOptions = useMemo(
    () =>
      asArray<MetaResource>(metaResources).filter(
        (resource) =>
          metaResourceMatchesActor(resource, positionAssignmentForm.actor_type) &&
          (!positionAssignmentForm.actor_id ||
            resource.owner_actor_id === positionAssignmentForm.actor_id ||
            resource.source_id === positionAssignmentForm.actor_id ||
            (!resource.owner_actor_id && !resource.source_id)),
      ),
    [metaResources, positionAssignmentForm.actor_id, positionAssignmentForm.actor_type],
  )
  const matrixAssignmentOptions = useMemo(
    () => asArray<PositionAssignment>(positionAssignments).filter((assignment) => assignment.meta_resource_id && assignment.status !== 'archived'),
    [positionAssignments],
  )

  useEffect(() => {
    let cancelled = false

    apiRequest<Organization[]>('/organizations?limit=100', { token })
      .then((data) => {
        if (cancelled) return
        const organizations = asArray<Organization>(data)
        setOrganizations(organizations)
        if (organizations.length > 0) {
          setSelectedOrgId(organizations[0].id)
          setOrgForm({
            name: organizations[0].name,
            description: organizations[0].description ?? '',
          })
        }
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : t('加载组织失败'))
      })

    apiRequest<ExternalMember[]>('/external-members?limit=100', { token })
      .then((data) => {
        if (!cancelled) setExternalMembers(asArray<ExternalMember>(data))
      })
      .catch(() => {
        if (!cancelled) setExternalMembers([])
      })

    apiRequest<AIAgent[]>('/agents?limit=100', { token })
      .then((data) => {
        if (cancelled) return
        const agents = asArray<AIAgent>(data)
        setAgents(agents)
        if (agents.length > 0) {
          setMemberForm((current) => ({ ...current, agent_id: current.agent_id || agents[0].id }))
        }
      })
      .catch(() => {
        if (!cancelled) setAgents([])
      })

    apiRequest<MetaResource[]>('/meta-resources?limit=200&status=active', { token })
      .then((data) => {
        if (!cancelled) setMetaResources(asArray<MetaResource>(data))
      })
      .catch(() => {
        if (!cancelled) setMetaResources([])
      })

    return () => {
      cancelled = true
    }
  }, [t, token])

  useEffect(() => {
    if (!selectedOrgId) return
    let cancelled = false

    apiRequest<Department[]>(`/organizations/${selectedOrgId}/departments/tree`, { token })
      .then((data) => {
        if (cancelled) return
        const tree = asArray<Department>(data)
        setDepartmentTree(tree)
        if (tree.length > 0) {
          setDepartmentSelection(tree[0])
        } else {
          setSelectedDepartmentId('')
          setSelectedPositionId('')
          setPositionAssignments([])
          setDepartmentForm(emptyDepartmentForm)
          setPositionForm(emptyPositionForm)
        }
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : t('加载部门树失败'))
      })

    apiRequest<OrganizationMembership[]>(`/organizations/${selectedOrgId}/members`, { token })
      .then((data) => {
        if (!cancelled) setMembers(asArray<OrganizationMembership>(data))
      })

    apiRequest<PermissionChangeRequest[]>(`/organizations/${selectedOrgId}/permission-change-requests`, { token })
      .then((data) => {
        if (!cancelled) setPermissionRequests(asArray<PermissionChangeRequest>(data))
      })
      .catch(() => {
        if (!cancelled) setPermissionRequests([])
      })

    apiRequest<OrganizationAccessRule[]>(`/organizations/${selectedOrgId}/permission-rules`, { token })
      .then((data) => {
        if (!cancelled) setAccessRules(asArray<OrganizationAccessRule>(data))
      })
      .catch(() => {
        if (!cancelled) setAccessRules([])
      })
      .catch(() => {
        if (!cancelled) setMembers([])
      })

    return () => {
      cancelled = true
    }
  }, [selectedOrgId, t, token])

  useEffect(() => {
    if (!selectedPositionId) {
      return
    }
    let cancelled = false

    apiRequest<PositionAssignment[]>(`/positions/${selectedPositionId}/assignments`, { token })
      .then((data) => {
        if (!cancelled) {
          const assignments = asArray<PositionAssignment>(data)
          setPositionAssignments(assignments)
          const first = assignments.find((assignment) => assignment.meta_resource_id && assignment.status !== 'archived')
          if (first) setTaskMatrixForm((current) => ({ ...current, position_assignment_id: current.position_assignment_id || first.id }))
        }
      })
      .catch(() => {
        if (!cancelled) setPositionAssignments([])
      })

    return () => {
      cancelled = true
    }
  }, [selectedPositionId, token])

  useEffect(() => {
    if (!externalSelection?.targetID) return
    const targetID = externalSelection.targetID
    const targetType = externalSelection.targetType
    const record = externalSelection.record
    return deferStateUpdate(() => {
      if (targetType === 'organization') {
        const org = asArray<Organization>(organizations).find((item) => item.id === targetID)
        if (org) {
          setSelectedOrgId(org.id)
          setSelectedDepartmentId('')
          setSelectedPositionId('')
          setPositionAssignments([])
          setOrgForm({
            name: org.name,
            description: org.description ?? '',
          })
          setDepartmentForm(emptyDepartmentForm)
          setPositionForm(emptyPositionForm)
        } else {
          setSelectedOrgId(targetID)
        }
        return
      }

      if (targetType === 'department') {
        const organizationID = typeof record?.organization_id === 'string' ? record.organization_id : ''
        if (organizationID && organizationID !== selectedOrgId) setSelectedOrgId(organizationID)
        const department = findDepartment(asArray<Department>(departmentTree), targetID)
        if (department) {
          setSelectedDepartmentId(department.id)
          setSelectedPositionId('')
          setPositionAssignments([])
          setDepartmentForm({
            name: department.name,
            code: department.code ?? '',
            description: department.description ?? '',
            status: department.status,
            sort_order: String(department.sort_order),
          })
          setPositionForm(emptyPositionForm)
        } else {
          setSelectedDepartmentId(targetID)
        }
        return
      }

      if (targetType === 'position') {
        const organizationID = typeof record?.organization_id === 'string' ? record.organization_id : ''
        const departmentID = typeof record?.department_id === 'string' ? record.department_id : ''
        if (organizationID && organizationID !== selectedOrgId) setSelectedOrgId(organizationID)
        if (departmentID) setSelectedDepartmentId(departmentID)
        setSelectedPositionId(targetID)
      }
    })
  }, [departmentTree, externalSelection?.record, externalSelection?.targetID, externalSelection?.targetType, organizations, selectedOrgId])

  async function runAction(action: () => Promise<void>, success: string) {
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

  function selectOrganization(org: Organization) {
    setSelectedOrgId(org.id)
    setSelectedDepartmentId('')
    setSelectedPositionId('')
    setPositionAssignments([])
    setOrgForm({
      name: org.name,
      description: org.description ?? '',
    })
    setDepartmentForm(emptyDepartmentForm)
    setPositionForm(emptyPositionForm)
  }

  function setDepartmentSelection(department: Department) {
    setSelectedDepartmentId(department.id)
    setSelectedPositionId('')
    setPositionAssignments([])
    setDepartmentForm({
      name: department.name,
      code: department.code ?? '',
      description: department.description ?? '',
      status: department.status,
      sort_order: String(department.sort_order),
    })
    setPositionForm(emptyPositionForm)
  }

  function setPositionSelection(department: Department, position: Position) {
    setSelectedDepartmentId(department.id)
    setSelectedPositionId(position.id)
    setDepartmentForm({
      name: department.name,
      code: department.code ?? '',
      description: department.description ?? '',
      status: department.status,
      sort_order: String(department.sort_order),
    })
    setPositionForm(positionToForm(position))
  }

  function createNewPositionDraft() {
    setSelectedPositionId('')
    setPositionAssignments([])
    setPositionForm(emptyPositionForm)
  }

  function setPositionActorType(actorType: PositionAssignment['actor_type']) {
    const nextActorId =
      actorType === 'internal_human'
        ? currentUserId ?? ''
        : actorType === 'external_human'
          ? asArray<ExternalMember>(externalMembers)[0]?.id ?? ''
          : asArray<AIAgent>(agents).find((agent) => agent.agent_origin === (actorType === 'internal_agent' ? 'internal' : 'external'))?.id ?? ''
    setPositionAssignmentForm((current) => ({ ...current, actor_type: actorType, actor_id: nextActorId, meta_resource_id: '' }))
  }

  async function loadOrganizations() {
    const data = await apiRequest<Organization[]>('/organizations?limit=100', { token })
    const organizations = asArray<Organization>(data)
    setOrganizations(organizations)
    if (!selectedOrgId && organizations.length > 0) {
      selectOrganization(organizations[0])
    }
  }

  async function loadDepartmentTree(orgId = selectedOrgId) {
    if (!orgId) return
    const data = await apiRequest<Department[]>(`/organizations/${orgId}/departments/tree`, { token })
    const tree = asArray<Department>(data)
    setDepartmentTree(tree)
    if (!selectedDepartmentId && tree.length > 0) {
      setDepartmentSelection(tree[0])
    }
  }

  async function loadMembers(orgId = selectedOrgId) {
    if (!orgId) return
    const data = await apiRequest<OrganizationMembership[]>(`/organizations/${orgId}/members`, { token })
    setMembers(asArray<OrganizationMembership>(data))
  }

  async function loadPermissionGovernance(orgId = selectedOrgId) {
    if (!orgId) return
    const [requests, rules] = await Promise.all([
      apiRequest<PermissionChangeRequest[]>(`/organizations/${orgId}/permission-change-requests`, { token }),
      apiRequest<OrganizationAccessRule[]>(`/organizations/${orgId}/permission-rules`, { token }),
    ])
    setPermissionRequests(asArray<PermissionChangeRequest>(requests))
    setAccessRules(asArray<OrganizationAccessRule>(rules))
  }

  async function loadExternalMembers() {
    const data = await apiRequest<ExternalMember[]>('/external-members?limit=100', { token })
    setExternalMembers(asArray<ExternalMember>(data))
  }

  async function loadAgents() {
    const data = await apiRequest<AIAgent[]>('/agents?limit=100', { token })
    const agents = asArray<AIAgent>(data)
    setAgents(agents)
    if (!memberForm.agent_id && agents.length > 0) {
      setMemberForm((current) => ({ ...current, agent_id: agents[0].id }))
    }
  }

  async function loadPositionAssignments(positionId = selectedPositionId) {
    if (!positionId) {
      setPositionAssignments([])
      return
    }
    const data = await apiRequest<PositionAssignment[]>(`/positions/${positionId}/assignments`, { token })
    const assignments = asArray<PositionAssignment>(data)
    setPositionAssignments(assignments)
    const first = assignments.find((assignment) => assignment.meta_resource_id && assignment.status !== 'archived')
    if (first) setTaskMatrixForm((current) => ({ ...current, position_assignment_id: current.position_assignment_id || first.id }))
  }

  async function createOrganization(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await runAction(async () => {
      const org = await apiRequest<Organization>('/organizations', {
        method: 'POST',
        token,
        body: orgForm,
      })
      await loadOrganizations()
      setSelectedOrgId(org.id)
      setOrgForm(emptyOrgForm)
    }, '组织已创建')
  }

  async function updateOrganization(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedOrgId) return
    await runAction(async () => {
      const org = await apiRequest<Organization>(`/organizations/${selectedOrgId}`, {
        method: 'PATCH',
        token,
        body: orgForm,
      })
      setOrganizations((current) => current.map((item) => (item.id === org.id ? org : item)))
    }, '组织已更新')
  }

  async function createDepartment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedOrgId) return
    await runAction(async () => {
      await apiRequest<Department>(`/organizations/${selectedOrgId}/departments`, {
        method: 'POST',
        token,
        body: {
          name: departmentForm.name,
          code: departmentForm.code,
          description: departmentForm.description,
          status: departmentForm.status,
          sort_order: Number(departmentForm.sort_order) || 0,
          parent_id: selectedDepartmentId || null,
          metadata: {},
        },
      })
      await loadDepartmentTree()
      setDepartmentForm(emptyDepartmentForm)
    }, '部门已创建')
  }

  async function updateDepartment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedDepartmentId) return
    await runAction(async () => {
      await apiRequest<Department>(`/departments/${selectedDepartmentId}`, {
        method: 'PATCH',
        token,
        body: {
          name: departmentForm.name,
          code: departmentForm.code,
          description: departmentForm.description,
          status: departmentForm.status,
          sort_order: Number(departmentForm.sort_order) || 0,
          metadata: {},
        },
      })
      await loadDepartmentTree()
    }, '部门已更新')
  }

  async function createPosition(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedDepartmentId) return
    await runAction(async () => {
      const position = await apiRequest<Position>(`/departments/${selectedDepartmentId}/positions`, {
        method: 'POST',
        token,
        body: {
          name: positionForm.name,
          code: positionForm.code,
          description: positionForm.description,
          permission_level: positionForm.permission_level,
          required_capabilities: splitCsv(positionForm.required_capabilities),
          status: positionForm.status,
          sort_order: Number(positionForm.sort_order) || 0,
          metadata: {},
        },
      })
      setSelectedPositionId(position.id)
      await loadDepartmentTree()
    }, '岗位已创建')
  }

  async function updatePosition(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedPositionId) return
    await runAction(async () => {
      const position = await apiRequest<Position>(`/positions/${selectedPositionId}`, {
        method: 'PATCH',
        token,
        body: {
          name: positionForm.name,
          code: positionForm.code,
          description: positionForm.description,
          permission_level: positionForm.permission_level,
          required_capabilities: splitCsv(positionForm.required_capabilities),
          status: positionForm.status,
          sort_order: Number(positionForm.sort_order) || 0,
          metadata: {},
        },
      })
      setPositionForm(positionToForm(position))
      await loadDepartmentTree()
    }, '岗位已更新')
  }

  async function assignPosition(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedPositionId || !positionAssignmentForm.actor_id || !positionAssignmentForm.meta_resource_id) return
    await runAction(async () => {
      await apiRequest<PositionAssignment>(`/positions/${selectedPositionId}/assignments`, {
        method: 'POST',
        token,
        body: {
          meta_resource_id: positionAssignmentForm.meta_resource_id,
          actor_id: positionAssignmentForm.actor_id,
          actor_type: positionAssignmentForm.actor_type,
          assignment_type: positionAssignmentForm.assignment_type,
          allocation_percent: Number(positionAssignmentForm.allocation_percent) || 100,
          status: 'active',
          metadata: {},
        },
      })
      const position = await apiRequest<Position>(`/positions/${selectedPositionId}`, { token })
      setPositionForm(positionToForm(position))
      setPositionAssignments(asArray<PositionAssignment>(position.assignments))
      await loadDepartmentTree()
    }, '岗位适配已保存')
  }

  async function removePositionAssignment(id: string) {
    await runAction(async () => {
      await apiRequest(`/position-assignments/${id}`, { method: 'DELETE', token })
      await loadPositionAssignments()
    }, '岗位适配已移除')
  }

  async function loadTaskMatrixAssignments(taskId = taskMatrixForm.task_id) {
    if (!taskId.trim()) {
      setTaskMatrixAssignments([])
      return
    }
    const data = await apiRequest<TaskMatrixAssignment[]>(`/tasks/${taskId.trim()}/matrix-assignments`, { token })
    setTaskMatrixAssignments(asArray<TaskMatrixAssignment>(data))
  }

  async function createTaskMatrixAssignment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const selectedAssignment = matrixAssignmentOptions.find((assignment) => assignment.id === taskMatrixForm.position_assignment_id)
    if (!taskMatrixForm.task_id.trim() || !selectedPositionId || !selectedAssignment?.meta_resource_id) return
    await runAction(async () => {
      await apiRequest<TaskMatrixAssignment>(`/tasks/${taskMatrixForm.task_id.trim()}/matrix-assignments`, {
        method: 'POST',
        token,
        body: {
          position_id: selectedPositionId,
          position_assignment_id: selectedAssignment.id,
          meta_resource_id: selectedAssignment.meta_resource_id,
          role_in_task: taskMatrixForm.role_in_task,
          allocation_percent: Number(taskMatrixForm.allocation_percent) || 100,
          status: 'active',
          metadata: {},
        },
      })
      await loadTaskMatrixAssignments(taskMatrixForm.task_id)
    }, '任务矩阵已更新')
  }

  async function removeTaskMatrixAssignment(id: string) {
    await runAction(async () => {
      await apiRequest(`/task-matrix-assignments/${id}`, { method: 'DELETE', token })
      await loadTaskMatrixAssignments(taskMatrixForm.task_id)
    }, '任务矩阵已移除')
  }

  async function createExternalMember(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await runAction(async () => {
      await apiRequest<ExternalMember>('/external-members', {
        method: 'POST',
        token,
        body: { ...externalForm, metadata: {} },
      })
      setExternalForm(emptyExternalForm)
      await loadExternalMembers()
    }, '外部成员已创建')
  }

  async function createAgent(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await runAction(async () => {
      const response = await apiRequest<{ agent: AIAgent; api_key: string }>('/agents/register', {
        method: 'POST',
        token,
        body: {
          name: agentForm.name,
          model_type: agentForm.model_type,
          capabilities: splitCsv(agentForm.capabilities),
          permission_level: agentForm.permission_level,
          agent_origin: agentForm.agent_origin,
          provider: agentForm.provider,
          service_class: agentForm.service_class,
          vendor: agentForm.vendor,
          contract_ref: agentForm.contract_ref,
          risk_level: agentForm.risk_level,
          metadata: {},
        },
      })
      setCreatedAgentKey(response.api_key)
      setMemberForm((current) => ({ ...current, member_type: 'agent', agent_id: response.agent.id }))
      setAgentForm(emptyAgentForm)
      await loadAgents()
    }, 'Agent 已创建')
  }

  async function addDepartmentMember(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedDepartmentId) return
    await runAction(async () => {
      await apiRequest<OrganizationMembership>(`/departments/${selectedDepartmentId}/members`, {
        method: 'POST',
        token,
        body: {
          member_type: memberForm.member_type,
          user_id: memberForm.member_type === 'internal' ? memberForm.user_id : null,
          external_member_id: memberForm.member_type === 'external' ? memberForm.external_member_id : null,
          agent_id: memberForm.member_type === 'agent' ? memberForm.agent_id : null,
          title: memberForm.title,
          status: 'active',
          metadata: memberForm.member_type === 'agent' ? { agent_origin: selectedAgent?.agent_origin ?? 'internal' } : {},
        },
      })
      await loadMembers()
    }, '成员已添加')
  }

  async function removeMembership(id: string) {
    await runAction(async () => {
      await apiRequest(`/memberships/${id}`, { method: 'DELETE', token })
      await loadMembers()
    }, '成员关系已移除')
  }

  async function createPermissionChangeRequest(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedOrgId || !permissionRequestForm.membership_id) return
    await runAction(async () => {
      await apiRequest<PermissionChangeRequest>(`/organizations/${selectedOrgId}/permission-change-requests`, {
        method: 'POST',
        token,
        body: {
          membership_id: permissionRequestForm.membership_id,
          requested_change: {
            authority_tier: permissionRequestForm.authority_tier,
            status: permissionRequestForm.status,
          },
          reason: permissionRequestForm.reason,
        },
      })
      await loadPermissionGovernance()
    }, '权限变更请求已提交')
  }

  async function reviewPermissionChangeRequest(id: string, decision: 'approve' | 'reject') {
    await runAction(async () => {
      await apiRequest<PermissionChangeRequest>(`/permission-change-requests/${id}/${decision}`, {
        method: 'POST',
        token,
        body: { reason: decision === 'approve' ? 'reviewed from organization workspace' : 'rejected from organization workspace' },
      })
      await Promise.all([loadPermissionGovernance(), loadMembers()])
    }, decision === 'approve' ? '权限变更已批准' : '权限变更已拒绝')
  }

  async function createAccessRule(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedOrgId) return
    await runAction(async () => {
      await apiRequest<OrganizationAccessRule>(`/organizations/${selectedOrgId}/permission-rules`, {
        method: 'POST',
        token,
        body: {
          ...accessRuleForm,
          priority: Number(accessRuleForm.priority || 0),
          metadata: {},
        },
      })
      await loadPermissionGovernance()
    }, '权限规则已创建')
  }

  function analyzeWithAgent() {
    const text = agentPrompt.trim()
    const seed = text || 'Create a product and engineering organization'
    const departments = inferDepartments(seed)
    setAgentPlan({
      organizationName: selectedOrganization?.name || inferOrganizationName(seed),
      description: `Agent analysis plan from: ${seed.slice(0, 120)}`,
      departments,
    })
  }

  async function applyAgentPlan() {
    if (!agentPlan) return
    await runAction(async () => {
      let orgId = selectedOrgId
      if (!orgId) {
        const org = await apiRequest<Organization>('/organizations', {
          method: 'POST',
          token,
          body: {
            name: agentPlan.organizationName,
            description: agentPlan.description,
          },
        })
        orgId = org.id
        setSelectedOrgId(org.id)
      }
      for (const department of agentPlan.departments) {
        await apiRequest<Department>(`/organizations/${orgId}/departments`, {
          method: 'POST',
          token,
          body: {
            ...department,
            status: 'active',
            sort_order: 10,
            metadata: { source: 'agent_plan' },
          },
        })
      }
      await loadOrganizations()
      await loadDepartmentTree(orgId)
    }, 'Agent 方案已创建')
  }

  async function matchMembers() {
    if (!selectedOrgId) return
    await runAction(async () => {
      const data = await apiRequest<MatchCandidate[]>('/organization/match-members', {
        method: 'POST',
        token,
        body: {
          organization_id: selectedOrgId,
          department_id: selectedDepartmentId || null,
          position_id: selectedPositionId || null,
          task_description: matchTask,
          required_capabilities: splitCsv(matchCapabilities),
          required_level: matchLevel,
          risk_level: matchRisk,
          member_types: ['internal', 'external', 'agent'],
        },
      })
      setCandidates(asArray<MatchCandidate>(data))
    }, '匹配完成')
  }

  return (
    <div className="space-y-5">
      <aside className="space-y-5">
        <section className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-2">
              <Building2 className="h-5 w-5 text-slate-500" />
              <h2 className="text-base font-semibold text-slate-950">{t('组织')}</h2>
            </div>
            <button
              type="button"
              onClick={() => runAction(loadOrganizations, '组织已刷新')}
              className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-slate-300 text-slate-600 hover:bg-slate-100"
            >
              <RefreshCw className="h-4 w-4" />
            </button>
          </div>
          <div className="mt-4 space-y-2">
            {asArray<Organization>(organizations).map((org) => (
              <button
                key={org.id}
                type="button"
                onClick={() => selectOrganization(org)}
                className={`w-full rounded-lg border px-3 py-2 text-left transition ${
                  selectedOrgId === org.id
                    ? 'border-slate-950 bg-slate-50'
                    : 'border-slate-200 hover:border-slate-300 hover:bg-slate-50'
                }`}
              >
                <p className="truncate text-sm font-semibold text-slate-950">{org.name}</p>
                <p className="mt-1 truncate text-xs text-slate-500">{org.description || org.id}</p>
              </button>
            ))}
            {asArray<Organization>(organizations).length === 0 && <p className="text-sm text-slate-500">{t('暂无组织')}</p>}
          </div>
        </section>

        <section className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
          <div className="flex items-center gap-2">
            <GitBranch className="h-5 w-5 text-slate-500" />
            <h2 className="text-base font-semibold text-slate-950">{t('部门树')}</h2>
          </div>
          <div className="mt-4 space-y-1">
            {asArray<Department>(departmentTree).map((department) => (
              <DepartmentNode
                key={department.id}
                department={department}
                selectedDepartmentId={selectedDepartmentId}
                selectedPositionId={selectedPositionId}
                onSelectDepartment={(id) => {
                  const department = findDepartment(departmentTree, id)
                  if (department) setDepartmentSelection(department)
                }}
                onSelectPosition={(department, position) => setPositionSelection(department, position)}
              />
            ))}
            {selectedOrgId && asArray<Department>(departmentTree).length === 0 && (
              <p className="text-sm text-slate-500">{t('当前组织暂无部门')}</p>
            )}
          </div>
        </section>
      </aside>

      <section className="min-w-0 space-y-5">
        {(message || error) && (
          <div
            className={`rounded-lg border px-4 py-3 text-sm ${
              error ? 'border-red-200 bg-red-50 text-red-700' : 'border-emerald-200 bg-emerald-50 text-emerald-700'
            }`}
          >
            {error || message}
          </div>
        )}

        <div className="space-y-5">
          <Panel icon={Plus} title="创建组织">
            <form className="space-y-3" onSubmit={createOrganization}>
              <TextInput label="组织名称" value={orgForm.name} onChange={(value) => setOrgForm({ ...orgForm, name: value })} />
              <TextArea
                label="组织描述"
                value={orgForm.description}
                onChange={(value) => setOrgForm({ ...orgForm, description: value })}
              />
              <SubmitButton loading={loading} label="创建组织" />
            </form>
          </Panel>

          <Panel icon={Save} title="修改当前组织">
            <form className="space-y-3" onSubmit={updateOrganization}>
              <TextInput label="组织名称" value={orgForm.name} onChange={(value) => setOrgForm({ ...orgForm, name: value })} />
              <TextArea
                label="组织描述"
                value={orgForm.description}
                onChange={(value) => setOrgForm({ ...orgForm, description: value })}
              />
              <SubmitButton loading={loading || !selectedOrgId} label="保存组织" />
            </form>
          </Panel>
        </div>

        <div className="space-y-5">
          <Panel icon={GitBranch} title={selectedDepartment ? '修改部门' : '创建部门'}>
            <form className="space-y-3" onSubmit={selectedDepartment ? updateDepartment : createDepartment}>
              <TextInput
                label="部门名称"
                value={departmentForm.name}
                onChange={(value) => setDepartmentForm({ ...departmentForm, name: value })}
              />
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput
                  label="部门编码"
                  value={departmentForm.code}
                  onChange={(value) => setDepartmentForm({ ...departmentForm, code: value })}
                />
                <TextInput
                  label="排序"
                  value={departmentForm.sort_order}
                  onChange={(value) => setDepartmentForm({ ...departmentForm, sort_order: value })}
                />
              </div>
              <TextArea
                label="部门描述"
                value={departmentForm.description}
                onChange={(value) => setDepartmentForm({ ...departmentForm, description: value })}
              />
              <SubmitButton loading={loading || !selectedOrgId} label={selectedDepartment ? '保存部门' : '创建为选中部门的子部门'} />
            </form>
          </Panel>

          <Panel icon={Users} title="岗位">
            <div className="space-y-4">
              <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-slate-200 bg-slate-50 p-3">
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold text-slate-950">{selectedDepartment?.name ?? t('未选择部门')}</p>
                  <p className="mt-1 text-xs text-slate-500">
                    {selectedPosition ? `${t('当前岗位')}：${selectedPosition.name}` : t('可在当前部门下创建岗位')}
                  </p>
                </div>
                <button
                  type="button"
                  onClick={createNewPositionDraft}
                  disabled={!selectedDepartmentId}
                  className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-300 px-3 text-sm font-semibold text-slate-700 hover:bg-white disabled:cursor-not-allowed disabled:opacity-60"
                >
                  <Plus className="h-4 w-4" />
                  {t('新建岗位')}
                </button>
              </div>

              <div className="grid gap-2 sm:grid-cols-2">
                {asArray<Position>(selectedDepartment?.positions).map((position) => (
                  <button
                    key={position.id}
                    type="button"
                    onClick={() => selectedDepartment && setPositionSelection(selectedDepartment, position)}
                    className={`rounded-lg border px-3 py-2 text-left transition ${
                      selectedPositionId === position.id
                        ? 'border-slate-950 bg-slate-50'
                        : 'border-slate-200 hover:border-slate-300 hover:bg-slate-50'
                    }`}
                  >
                    <p className="truncate text-sm font-semibold text-slate-950">{position.name}</p>
                    <p className="mt-1 truncate text-xs text-slate-500">
                      {position.code || t('未设置编码')} · {position.permission_level} · {position.status}
                    </p>
                  </button>
                ))}
                {selectedDepartment && asArray<Position>(selectedDepartment.positions).length === 0 && (
                  <p className="text-sm text-slate-500">{t('当前部门暂无岗位')}</p>
                )}
              </div>

              <form className="space-y-3" onSubmit={selectedPositionId ? updatePosition : createPosition}>
                <TextInput
                  label="岗位名称"
                  value={positionForm.name}
                  onChange={(value) => setPositionForm({ ...positionForm, name: value })}
                />
                <div className="grid gap-3 sm:grid-cols-2">
                  <TextInput
                    label="岗位编码"
                    value={positionForm.code}
                    onChange={(value) => setPositionForm({ ...positionForm, code: value })}
                  />
                  <TextInput
                    label="排序"
                    value={positionForm.sort_order}
                    onChange={(value) => setPositionForm({ ...positionForm, sort_order: value })}
                  />
                </div>
                <div className="grid gap-3 sm:grid-cols-2">
                  <label className="block">
                    <span className="text-sm font-medium text-slate-700">{t('权限级别')}</span>
                    <select
                      value={positionForm.permission_level}
                      onChange={(event) => setPositionForm({ ...positionForm, permission_level: event.target.value })}
                      className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                    >
                      <option value="L1">L1</option>
                      <option value="L2">L2</option>
                      <option value="L3">L3</option>
                      <option value="L4">L4</option>
                    </select>
                  </label>
                  <label className="block">
                    <span className="text-sm font-medium text-slate-700">{t('状态')}</span>
                    <select
                      value={positionForm.status}
                      onChange={(event) => setPositionForm({ ...positionForm, status: event.target.value })}
                      className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                    >
                      <option value="active">{t('active')}</option>
                      <option value="inactive">{t('inactive')}</option>
                      <option value="archived">{t('archived')}</option>
                    </select>
                  </label>
                </div>
                <TextInput
                  label="所需能力"
                  value={positionForm.required_capabilities}
                  onChange={(value) => setPositionForm({ ...positionForm, required_capabilities: value })}
                  placeholder="planning, review"
                />
                <TextArea
                  label="岗位描述"
                  value={positionForm.description}
                  onChange={(value) => setPositionForm({ ...positionForm, description: value })}
                />
                <SubmitButton loading={loading || !selectedDepartmentId} label={selectedPositionId ? '保存岗位' : '创建岗位'} />
              </form>

              <form className="space-y-3 border-t border-slate-200 pt-4" onSubmit={assignPosition}>
                <p className="text-sm font-semibold text-slate-950">{t('适配身份')}</p>
                <div className="grid gap-3 sm:grid-cols-2">
                  <label className="block">
                    <span className="text-sm font-medium text-slate-700">{t('身份类型')}</span>
                    <select
                      value={positionAssignmentForm.actor_type}
                      onChange={(event) => setPositionActorType(event.target.value as PositionAssignment['actor_type'])}
                      className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                    >
                      <option value="internal_human">{t('内部员工')}</option>
                      <option value="external_human">{t('外部员工')}</option>
                      <option value="internal_agent">{t('内部 Agent')}</option>
                      <option value="external_agent">{t('外部 Agent')}</option>
                    </select>
                  </label>
                  <label className="block">
                    <span className="text-sm font-medium text-slate-700">{t('适配类型')}</span>
                    <select
                      value={positionAssignmentForm.assignment_type}
                      onChange={(event) => setPositionAssignmentForm({ ...positionAssignmentForm, assignment_type: event.target.value })}
                      className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                    >
                      <option value="primary">{t('primary')}</option>
                      <option value="backup">{t('backup')}</option>
                      <option value="candidate">{t('candidate')}</option>
                    </select>
                  </label>
                </div>
                {assignmentActorOptions.length > 0 ? (
                  <label className="block">
                    <span className="text-sm font-medium text-slate-700">{t('适配身份')}</span>
                    <select
                      value={positionAssignmentForm.actor_id}
                      onChange={(event) => setPositionAssignmentForm({ ...positionAssignmentForm, actor_id: event.target.value })}
                      className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                    >
                      {assignmentActorOptions.map((option) => (
                        <option key={option.id} value={option.id}>
                          {option.label}
                        </option>
                      ))}
                    </select>
                  </label>
                ) : (
                  <TextInput
                    label="适配身份 ID"
                    value={positionAssignmentForm.actor_id}
                    onChange={(value) => setPositionAssignmentForm({ ...positionAssignmentForm, actor_id: value })}
                  />
                )}
                <label className="block">
                  <span className="text-sm font-medium text-slate-700">{t('承接 Meta')}</span>
                  <select
                    value={positionAssignmentForm.meta_resource_id}
                    onChange={(event) => setPositionAssignmentForm({ ...positionAssignmentForm, meta_resource_id: event.target.value })}
                    className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                  >
                    <option value="">{t('请选择 Meta 资源')}</option>
                    {assignmentMetaOptions.map((resource) => (
                      <option key={resource.id} value={resource.id}>
                        {resource.name} · {t(resource.resource_type)}
                      </option>
                    ))}
                  </select>
                </label>
                <TextInput
                  label="投入比例"
                  value={positionAssignmentForm.allocation_percent}
                  onChange={(value) => setPositionAssignmentForm({ ...positionAssignmentForm, allocation_percent: value })}
                />
                <SubmitButton loading={loading || !selectedPositionId || !positionAssignmentForm.meta_resource_id} label="保存岗位适配" />
              </form>

              <div className="space-y-2 border-t border-slate-200 pt-4">
                <p className="text-sm font-semibold text-slate-950">{t('当前岗位适配')}</p>
                {asArray<PositionAssignment>(positionAssignments).map((assignment) => (
                  <div key={assignment.id} className="grid gap-2 rounded-lg border border-slate-200 p-3 sm:grid-cols-[1fr_auto]">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-semibold text-slate-950">{assignment.actor_name || assignment.actor_id}</p>
                      <p className="mt-1 text-xs text-slate-500">
                        {t(assignment.actor_type)} · {t(assignment.assignment_type)} · {assignment.allocation_percent}%
                      </p>
                      <p className="mt-1 text-xs text-slate-500">
                        {t('承接 Meta')}：{assignment.meta_resource_name || assignment.meta_resource_id || t('未绑定')}
                        {assignment.meta_resource_type ? ` · ${t(assignment.meta_resource_type)}` : ''}
                      </p>
                    </div>
                    <button
                      type="button"
                      onClick={() => removePositionAssignment(assignment.id)}
                      className="h-9 rounded-lg border border-red-200 px-3 text-sm font-medium text-red-700 hover:bg-red-50"
                    >
                      {t('移除')}
                    </button>
                  </div>
                ))}
                {asArray<PositionAssignment>(positionAssignments).length === 0 && (
                  <p className="text-sm text-slate-500">{t('暂无岗位适配')}</p>
                )}
              </div>
            </div>
          </Panel>

          <Panel icon={GitBranch} title="任务矩阵">
            <div className="space-y-4">
              <form className="space-y-3" onSubmit={createTaskMatrixAssignment}>
                <TextInput
                  label="任务 ID"
                  value={taskMatrixForm.task_id}
                  onChange={(value) => setTaskMatrixForm({ ...taskMatrixForm, task_id: value })}
                />
                <div className="flex flex-wrap gap-2">
                  <button
                    type="button"
                    onClick={() => void loadTaskMatrixAssignments()}
                    disabled={!taskMatrixForm.task_id.trim() || loading}
                    className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-300 px-3 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    <RefreshCw className="h-4 w-4" />
                    {t('加载任务矩阵')}
                  </button>
                </div>
                <label className="block">
                  <span className="text-sm font-medium text-slate-700">{t('岗位 Meta 任命')}</span>
                  <select
                    value={taskMatrixForm.position_assignment_id}
                    onChange={(event) => setTaskMatrixForm({ ...taskMatrixForm, position_assignment_id: event.target.value })}
                    className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                  >
                    <option value="">{t('请选择岗位 Meta 任命')}</option>
                    {matrixAssignmentOptions.map((assignment) => (
                      <option key={assignment.id} value={assignment.id}>
                        {assignment.meta_resource_name || assignment.actor_name || assignment.actor_id} · {t(assignment.assignment_type)}
                      </option>
                    ))}
                  </select>
                </label>
                <div className="grid gap-3 sm:grid-cols-2">
                  <label className="block">
                    <span className="text-sm font-medium text-slate-700">{t('任务角色')}</span>
                    <select
                      value={taskMatrixForm.role_in_task}
                      onChange={(event) => setTaskMatrixForm({ ...taskMatrixForm, role_in_task: event.target.value })}
                      className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                    >
                      <option value="owner">{t('owner')}</option>
                      <option value="reviewer">{t('reviewer')}</option>
                      <option value="support">{t('support')}</option>
                      <option value="observer">{t('observer')}</option>
                    </select>
                  </label>
                  <TextInput
                    label="投入比例"
                    value={taskMatrixForm.allocation_percent}
                    onChange={(value) => setTaskMatrixForm({ ...taskMatrixForm, allocation_percent: value })}
                  />
                </div>
                <SubmitButton
                  loading={loading || !selectedPositionId || !taskMatrixForm.task_id.trim() || !taskMatrixForm.position_assignment_id}
                  label="写入任务矩阵"
                />
              </form>

              <div className="space-y-2 border-t border-slate-200 pt-4">
                <p className="text-sm font-semibold text-slate-950">{t('当前任务矩阵')}</p>
                {asArray<TaskMatrixAssignment>(taskMatrixAssignments).map((assignment) => (
                  <div key={assignment.id} className="grid gap-2 rounded-lg border border-slate-200 p-3 sm:grid-cols-[1fr_auto]">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-semibold text-slate-950">
                        {assignment.meta_resource_name || assignment.meta_resource_id}
                      </p>
                      <p className="mt-1 text-xs text-slate-500">
                        {t(assignment.role_in_task)} · {assignment.position_name || assignment.position_id} · {assignment.allocation_percent}%
                      </p>
                    </div>
                    <button
                      type="button"
                      onClick={() => removeTaskMatrixAssignment(assignment.id)}
                      className="h-9 rounded-lg border border-red-200 px-3 text-sm font-medium text-red-700 hover:bg-red-50"
                    >
                      {t('移除')}
                    </button>
                  </div>
                ))}
                {asArray<TaskMatrixAssignment>(taskMatrixAssignments).length === 0 && (
                  <p className="text-sm text-slate-500">{t('暂无任务矩阵')}</p>
                )}
              </div>
            </div>
          </Panel>

          <Panel icon={UserPlus} title="添加成员">
            <form className="space-y-3" onSubmit={addDepartmentMember}>
              <label className="block">
                <span className="text-sm font-medium text-slate-700">{t('成员类型')}</span>
                <select
                  value={memberForm.member_type}
                  onChange={(event) => setMemberForm({ ...memberForm, member_type: event.target.value })}
                  className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                >
                  <option value="internal">{t('内部员工')}</option>
                  <option value="external">{t('外部员工')}</option>
                  <option value="agent">Agent</option>
                </select>
              </label>
              {memberForm.member_type === 'internal' && (
                <TextInput
                  label="内部员工 User ID"
                  value={memberForm.user_id}
                  onChange={(value) => setMemberForm({ ...memberForm, user_id: value })}
                />
              )}
              {memberForm.member_type === 'external' && (
                <label className="block">
                  <span className="text-sm font-medium text-slate-700">{t('外部成员')}</span>
                  <select
                    value={memberForm.external_member_id}
                    onChange={(event) => setMemberForm({ ...memberForm, external_member_id: event.target.value })}
                    className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                  >
                    <option value="">{t('选择外部成员')}</option>
                    {asArray<ExternalMember>(externalMembers).map((member) => (
                      <option key={member.id} value={member.id}>
                        {member.name}
                      </option>
                    ))}
                  </select>
                </label>
              )}
              {memberForm.member_type === 'agent' && (
                <label className="block">
                  <span className="text-sm font-medium text-slate-700">Agent</span>
                  <select
                    value={memberForm.agent_id}
                    onChange={(event) => setMemberForm({ ...memberForm, agent_id: event.target.value })}
                    className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                  >
                    <option value="">{t('选择 Agent')}</option>
                    {asArray<AIAgent>(agents).map((agent) => (
                      <option key={agent.id} value={agent.id}>
                        {agent.name} · {agent.agent_origin} · {agent.risk_level}
                      </option>
                    ))}
                  </select>
                </label>
              )}
              <TextInput
                label="职位/职责"
                value={memberForm.title}
                onChange={(value) => setMemberForm({ ...memberForm, title: value })}
              />
              <SubmitButton loading={loading || !selectedDepartmentId} label="添加到当前部门" />
            </form>
          </Panel>
        </div>

        <div className="space-y-5">
          <Panel icon={Users} title="外部成员">
            <form className="space-y-3" onSubmit={createExternalMember}>
              <TextInput
                label="姓名"
                value={externalForm.name}
                onChange={(value) => setExternalForm({ ...externalForm, name: value })}
              />
              <TextInput
                label="邮箱"
                value={externalForm.email}
                onChange={(value) => setExternalForm({ ...externalForm, email: value })}
              />
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput
                  label="供应商"
                  value={externalForm.vendor}
                  onChange={(value) => setExternalForm({ ...externalForm, vendor: value })}
                />
                <TextInput
                  label="合同类型"
                  value={externalForm.contract_type}
                  onChange={(value) => setExternalForm({ ...externalForm, contract_type: value })}
                />
              </div>
              <SubmitButton loading={loading} label="创建外部成员" />
            </form>
          </Panel>

          <Panel icon={Bot} title="Agent 服务">
            <form className="space-y-3" onSubmit={createAgent}>
              <TextInput
                label="名称"
                value={agentForm.name}
                onChange={(value) => setAgentForm({ ...agentForm, name: value })}
              />
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput
                  label="模型/服务类型"
                  value={agentForm.model_type}
                  onChange={(value) => setAgentForm({ ...agentForm, model_type: value })}
                />
                <TextInput
                  label="能力标签"
                  value={agentForm.capabilities}
                  onChange={(value) => setAgentForm({ ...agentForm, capabilities: value })}
                />
              </div>
              <div className="grid gap-3 sm:grid-cols-3">
                <label className="block">
                  <span className="text-sm font-medium text-slate-700">{t('来源')}</span>
                  <select
                    value={agentForm.agent_origin}
                    onChange={(event) => setAgentForm({ ...agentForm, agent_origin: event.target.value })}
                    className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                  >
                    <option value="internal">{t('内部自建')}</option>
                    <option value="external">{t('外部购买')}</option>
                  </select>
                </label>
                <label className="block">
                  <span className="text-sm font-medium text-slate-700">{t('权限')}</span>
                  <select
                    value={agentForm.permission_level}
                    onChange={(event) => setAgentForm({ ...agentForm, permission_level: event.target.value })}
                    className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                  >
                    <option value="L1">L1</option>
                    <option value="L2">L2</option>
                    <option value="L3">L3</option>
                    <option value="L4">L4</option>
                  </select>
                </label>
                <label className="block">
                  <span className="text-sm font-medium text-slate-700">{t('风险')}</span>
                  <select
                    value={agentForm.risk_level}
                    onChange={(event) => setAgentForm({ ...agentForm, risk_level: event.target.value })}
                    className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                  >
                    <option value="low">low</option>
                    <option value="medium">medium</option>
                    <option value="high">high</option>
                    <option value="critical">critical</option>
                  </select>
                </label>
              </div>
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput
                  label="Provider"
                  value={agentForm.provider}
                  onChange={(value) => setAgentForm({ ...agentForm, provider: value })}
                />
                <TextInput
                  label="Vendor/Contract"
                  value={agentForm.vendor}
                  onChange={(value) => setAgentForm({ ...agentForm, vendor: value })}
                />
              </div>
              <SubmitButton loading={loading} label="创建 Agent" />
              {createdAgentKey && (
                <div className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
                  <p className="font-semibold">API Key</p>
                  <p className="mt-1 break-all font-mono">{createdAgentKey}</p>
                </div>
              )}
            </form>
          </Panel>

          <Panel icon={Bot} title="Agent 分析创建">
            <div className="space-y-3">
              <TextArea
                label="组织需求"
                value={agentPrompt}
                onChange={setAgentPrompt}
                placeholder="organization.agentPromptPlaceholder"
              />
              <div className="flex flex-wrap gap-2">
                <button
                  type="button"
                  onClick={analyzeWithAgent}
                  className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-300 px-3 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                >
                  <Sparkles className="h-4 w-4" />
                  {t('分析')}
                </button>
                <button
                  type="button"
                  onClick={applyAgentPlan}
                  disabled={!agentPlan || loading}
                  className="inline-flex h-10 items-center gap-2 rounded-lg bg-slate-950 px-3 text-sm font-semibold text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
                  {t('应用方案')}
                </button>
              </div>
              {agentPlan && (
                <div className="rounded-lg border border-slate-200 bg-slate-50 p-3 text-sm">
                  <p className="font-semibold text-slate-950">{agentPlan.organizationName}</p>
                  <p className="mt-1 text-slate-500">{agentPlan.description}</p>
                  <div className="mt-3 flex flex-wrap gap-2">
                    {agentPlan.departments.map((department) => (
                      <span key={department.code} className="rounded-md border border-slate-200 bg-white px-2 py-1 text-xs text-slate-600">
                        {department.name}
                      </span>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </Panel>
        </div>

        <Panel icon={Users} title="当前组织成员">
          <div className="space-y-2">
            {asArray<OrganizationMembership>(members).map((member) => (
              <div key={member.id} className="grid gap-2 border-t border-slate-100 py-3 first:border-t-0 first:pt-0 sm:grid-cols-[1fr_auto]">
                <div>
                  <p className="text-sm font-semibold text-slate-950">{member.member_name || member.member_type}</p>
                  <p className="text-sm text-slate-500">
                    {member.title || t('未设置职责')} · {t(member.authority_tier || 'executor')} · {member.member_type} · {member.status}
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => removeMembership(member.id)}
                  className="h-9 rounded-lg border border-red-200 px-3 text-sm font-medium text-red-700 hover:bg-red-50"
                >
                  {t('移除')}
                </button>
              </div>
            ))}
            {asArray<OrganizationMembership>(members).length === 0 && <p className="text-sm text-slate-500">{t('暂无成员')}</p>}
          </div>
        </Panel>

        <Panel icon={ShieldCheck} title="权限治理">
          <div className="grid gap-5 xl:grid-cols-2">
            <div className="space-y-4">
              <form className="space-y-3 rounded-lg border border-slate-200 bg-slate-50 p-4" onSubmit={createPermissionChangeRequest}>
                <p className="text-sm font-semibold text-slate-950">{t('提交权限变更复核')}</p>
                <label className="block">
                  <span className="text-sm font-medium text-slate-700">{t('成员')}</span>
                  <select
                    value={permissionRequestForm.membership_id}
                    onChange={(event) => setPermissionRequestForm({ ...permissionRequestForm, membership_id: event.target.value })}
                    className="mt-1 h-10 w-full rounded-lg border border-slate-300 bg-white px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                  >
                    <option value="">{t('选择成员')}</option>
                    {asArray<OrganizationMembership>(members).map((member) => (
                      <option key={member.id} value={member.id}>
                        {member.member_name || member.member_type}
                      </option>
                    ))}
                  </select>
                </label>
                <div className="grid gap-3 sm:grid-cols-2">
                  <SelectField
                    label="组织权限"
                    value={permissionRequestForm.authority_tier}
                    onChange={(value) => setPermissionRequestForm({ ...permissionRequestForm, authority_tier: value })}
                    options={['organization_admin', 'reviewer', 'executor']}
                  />
                  <SelectField
                    label="状态"
                    value={permissionRequestForm.status}
                    onChange={(value) => setPermissionRequestForm({ ...permissionRequestForm, status: value })}
                    options={['active', 'inactive', 'archived']}
                  />
                </div>
                <TextArea
                  label="原因"
                  value={permissionRequestForm.reason}
                  onChange={(value) => setPermissionRequestForm({ ...permissionRequestForm, reason: value })}
                />
                <SubmitButton loading={loading || !selectedOrgId} label="提交复核" />
              </form>

              <div className="space-y-2">
                <p className="text-sm font-semibold text-slate-950">{t('待复核请求')}</p>
                {asArray<PermissionChangeRequest>(permissionRequests).map((request) => (
                  <div key={request.id} className="rounded-lg border border-slate-200 p-3">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="truncate text-sm font-semibold text-slate-950">{request.membership_id}</p>
                        <p className="mt-1 text-xs text-slate-500">
                          {t('状态')} {t(request.status)} · {request.reason || t('未设置原因')}
                        </p>
                      </div>
                      {request.status === 'pending' && (
                        <div className="flex shrink-0 gap-2">
                          <button
                            type="button"
                            onClick={() => reviewPermissionChangeRequest(request.id, 'approve')}
                            className="h-8 rounded-md border border-emerald-200 px-2 text-xs font-semibold text-emerald-700 hover:bg-emerald-50"
                          >
                            {t('批准')}
                          </button>
                          <button
                            type="button"
                            onClick={() => reviewPermissionChangeRequest(request.id, 'reject')}
                            className="h-8 rounded-md border border-red-200 px-2 text-xs font-semibold text-red-700 hover:bg-red-50"
                          >
                            {t('拒绝')}
                          </button>
                        </div>
                      )}
                    </div>
                  </div>
                ))}
                {permissionRequests.length === 0 && <p className="text-sm text-slate-500">{t('暂无权限变更请求')}</p>}
              </div>
            </div>

            <div className="space-y-4">
              <form className="space-y-3 rounded-lg border border-slate-200 bg-slate-50 p-4" onSubmit={createAccessRule}>
                <p className="text-sm font-semibold text-slate-950">{t('创建权限规则')}</p>
                <div className="grid gap-3 sm:grid-cols-2">
                  <SelectField
                    label="范围"
                    value={accessRuleForm.scope_type}
                    onChange={(value) => setAccessRuleForm({ ...accessRuleForm, scope_type: value })}
                    options={['organization', 'department', 'project', 'function', 'form', 'field']}
                  />
                  <TextInput label="范围 ID" value={accessRuleForm.scope_id} onChange={(value) => setAccessRuleForm({ ...accessRuleForm, scope_id: value })} />
                  <TextInput
                    label="资源类型"
                    value={accessRuleForm.resource_type}
                    onChange={(value) => setAccessRuleForm({ ...accessRuleForm, resource_type: value })}
                  />
                  <SelectField
                    label="操作"
                    value={accessRuleForm.action}
                    onChange={(value) => setAccessRuleForm({ ...accessRuleForm, action: value })}
                    options={['read', 'write', 'delete', 'admin', 'execute']}
                  />
                  <SelectField
                    label="行为"
                    value={accessRuleForm.behavior}
                    onChange={(value) => setAccessRuleForm({ ...accessRuleForm, behavior: value })}
                    options={['allow', 'notify', 'approve', 'deny']}
                  />
                  <SelectField
                    label="所需级别"
                    value={accessRuleForm.required_level}
                    onChange={(value) => setAccessRuleForm({ ...accessRuleForm, required_level: value })}
                    options={['L1', 'L2', 'L3', 'L4']}
                  />
                  <TextInput label="Actor ID" value={accessRuleForm.actor_id} onChange={(value) => setAccessRuleForm({ ...accessRuleForm, actor_id: value })} />
                  <TextInput label="优先级" value={accessRuleForm.priority} onChange={(value) => setAccessRuleForm({ ...accessRuleForm, priority: value })} />
                </div>
                <TextArea label="原因" value={accessRuleForm.reason} onChange={(value) => setAccessRuleForm({ ...accessRuleForm, reason: value })} />
                <SubmitButton loading={loading || !selectedOrgId} label="创建权限规则" />
              </form>

              <div className="space-y-2">
                <p className="text-sm font-semibold text-slate-950">{t('组织权限规则')}</p>
                {asArray<OrganizationAccessRule>(accessRules).map((rule) => (
                  <div key={rule.id} className="rounded-lg border border-slate-200 p-3">
                    <p className="text-sm font-semibold text-slate-950">
                      {rule.scope_type}:{rule.scope_id || '*'} · {rule.resource_type}.{rule.action}
                    </p>
                    <p className="mt-1 text-xs text-slate-500">
                      {t(rule.behavior)} · {rule.required_level} · {t('优先级')} {rule.priority}
                    </p>
                  </div>
                ))}
                {accessRules.length === 0 && <p className="text-sm text-slate-500">{t('暂无权限规则')}</p>}
              </div>
            </div>
          </div>
        </Panel>

        <Panel icon={Sparkles} title="成员匹配">
          <div className="space-y-3">
            <TextInput label="任务描述" value={matchTask} onChange={setMatchTask} />
            <div className="grid gap-3 sm:grid-cols-3">
              <TextInput label="所需能力" value={matchCapabilities} onChange={setMatchCapabilities} />
              <label className="block">
                <span className="text-sm font-medium text-slate-700">{t('权限')}</span>
                <select
                  value={matchLevel}
                  onChange={(event) => setMatchLevel(event.target.value)}
                  className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                >
                  <option value="L1">L1</option>
                  <option value="L2">L2</option>
                  <option value="L3">L3</option>
                  <option value="L4">L4</option>
                </select>
              </label>
              <label className="block">
                <span className="text-sm font-medium text-slate-700">{t('风险')}</span>
                <select
                  value={matchRisk}
                  onChange={(event) => setMatchRisk(event.target.value)}
                  className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                >
                  <option value="low">low</option>
                  <option value="medium">medium</option>
                  <option value="high">high</option>
                  <option value="critical">critical</option>
                </select>
              </label>
            </div>
            <button
              type="button"
              onClick={matchMembers}
              disabled={!selectedOrgId || loading}
              className="inline-flex h-10 items-center gap-2 rounded-lg bg-slate-950 px-3 text-sm font-semibold text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Sparkles className="h-4 w-4" />}
              {t('匹配成员')}
            </button>
            <div className="grid gap-2 md:grid-cols-2">
              {asArray<MatchCandidate>(candidates).map((candidate) => (
                <div key={candidate.membership_id} className="rounded-lg border border-slate-200 p-3">
                  <div className="flex items-center justify-between gap-3">
                    <p className="truncate text-sm font-semibold text-slate-950">{candidate.member_name}</p>
                    <span className="text-sm font-semibold text-emerald-700">{Math.round(candidate.score * 100)}%</span>
                  </div>
                  <p className="mt-1 text-sm text-slate-500">{candidate.reason}</p>
                  <div className="mt-3 flex flex-wrap gap-2 text-xs font-semibold">
                    <span className="rounded-md border border-slate-200 bg-slate-50 px-2 py-1 text-slate-600">
                      {t('权重')} {Math.round(candidate.weight_snapshot * 100)}%
                    </span>
                    <span
                      className={`rounded-md border px-2 py-1 ${
                        candidate.requires_approval
                          ? 'border-amber-200 bg-amber-50 text-amber-700'
                          : candidate.access_allowed
                            ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
                            : 'border-red-200 bg-red-50 text-red-700'
                      }`}
                    >
                      {candidate.requires_approval ? t('需审批') : candidate.access_decision}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </Panel>
      </section>
    </div>
  )
}

function DepartmentNode({
  department,
  selectedDepartmentId,
  selectedPositionId,
  onSelectDepartment,
  onSelectPosition,
  depth = 0,
}: {
  department: Department
  selectedDepartmentId: string
  selectedPositionId: string
  onSelectDepartment: (id: string) => void
  onSelectPosition: (department: Department, position: Position) => void
  depth?: number
}) {
  return (
    <div>
      <button
        type="button"
        onClick={() => onSelectDepartment(department.id)}
        className={`flex h-9 w-full items-center justify-between rounded-lg px-3 text-left text-sm transition ${
          selectedDepartmentId === department.id && !selectedPositionId
            ? 'bg-slate-950 text-white'
            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-950'
        }`}
        style={{ paddingLeft: `${12 + depth * 14}px` }}
      >
        <span className="min-w-0 truncate">{department.name}</span>
        <span className="text-xs opacity-70">
          {asArray<Department>(department.children).length + asArray<Position>(department.positions).length}
        </span>
      </button>
      {asArray<Department>(department.children).map((child) => (
        <DepartmentNode
          key={child.id}
          department={child}
          selectedDepartmentId={selectedDepartmentId}
          selectedPositionId={selectedPositionId}
          onSelectDepartment={onSelectDepartment}
          onSelectPosition={onSelectPosition}
          depth={depth + 1}
        />
      ))}
      {asArray<Position>(department.positions).map((position) => (
        <button
          key={position.id}
          type="button"
          onClick={() => onSelectPosition(department, position)}
          className={`mt-1 flex h-8 w-full items-center gap-2 rounded-lg px-3 text-left text-xs transition ${
            selectedPositionId === position.id
              ? 'bg-slate-950 text-white'
              : 'text-slate-500 hover:bg-slate-100 hover:text-slate-950'
          }`}
          style={{ paddingLeft: `${26 + depth * 14}px` }}
        >
          <Users className="h-3.5 w-3.5 shrink-0" />
          <span className="min-w-0 flex-1 truncate">{position.name}</span>
          <span className="shrink-0 opacity-70">{position.permission_level}</span>
        </button>
      ))}
    </div>
  )
}

function Panel({
  icon: Icon,
  title,
  children,
}: {
  icon: typeof Building2
  title: string
  children: ReactNode
}) {
  const { t } = useI18n()
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="mb-4 flex items-center gap-2">
        <Icon className="h-5 w-5 text-slate-500" />
        <h2 className="text-base font-semibold text-slate-950">{t(title)}</h2>
      </div>
      {children}
    </section>
  )
}

function TextInput({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  placeholder?: string
}) {
  const { t } = useI18n()
  return (
    <label className="block">
      <span className="text-sm font-medium text-slate-700">{t(label)}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder ? t(placeholder) : undefined}
        className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
      />
    </label>
  )
}

function SelectField({
  label,
  value,
  onChange,
  options,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  options: string[]
}) {
  const { t } = useI18n()
  return (
    <label className="block">
      <span className="text-sm font-medium text-slate-700">{t(label)}</span>
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-10 w-full rounded-lg border border-slate-300 bg-white px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
      >
        {options.map((option) => (
          <option key={option} value={option}>
            {t(option)}
          </option>
        ))}
      </select>
    </label>
  )
}

function TextArea({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  placeholder?: string
}) {
  const { t } = useI18n()
  return (
    <label className="block">
      <span className="text-sm font-medium text-slate-700">{t(label)}</span>
      <textarea
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder ? t(placeholder) : undefined}
        className="mt-1 h-24 w-full resize-y rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
      />
    </label>
  )
}

function SubmitButton({ loading, label }: { loading: boolean; label: string }) {
  const { t } = useI18n()
  return (
    <button
      type="submit"
      disabled={loading}
      className="inline-flex h-10 items-center gap-2 rounded-lg bg-slate-950 px-3 text-sm font-semibold text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
    >
      {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
      {t(label)}
    </button>
  )
}

function findDepartment(nodes: Department[] | null | undefined, id: string): Department | undefined {
  if (!Array.isArray(nodes) || !id) return undefined
  for (const node of nodes) {
    if (node.id === id) return node
    const child = findDepartment(asArray<Department>(node.children), id)
    if (child) return child
  }
  return undefined
}

function positionToForm(position: Position) {
  return {
    name: position.name ?? '',
    code: position.code ?? '',
    description: position.description ?? '',
    permission_level: position.permission_level || 'L1',
    required_capabilities: asArray<string>(position.required_capabilities).join(', '),
    status: position.status || 'active',
    sort_order: String(position.sort_order ?? 10),
  }
}

function inferOrganizationName(prompt: string): string {
  if (prompt.includes('销售')) return 'Revenue Organization'
  if (prompt.includes('产品')) return 'Product Organization'
  if (prompt.includes('交付')) return 'Delivery Organization'
  return 'Agent Designed Organization'
}

function inferDepartments(prompt: string): AgentPlan['departments'] {
  const normalized = prompt.toLowerCase()
  const departments = [
    { name: 'Strategy Office', code: 'STR', description: 'Strategy and planning' },
    { name: 'Operations', code: 'OPS', description: 'Execution and coordination' },
  ]

  if (normalized.includes('product') || prompt.includes('产品')) {
    departments.push({ name: 'Product', code: 'PRD', description: 'Product management and discovery' })
  }
  if (normalized.includes('engineer') || prompt.includes('工程') || prompt.includes('研发')) {
    departments.push({ name: 'Engineering', code: 'ENG', description: 'Engineering delivery and platform' })
  }
  if (normalized.includes('sales') || prompt.includes('销售')) {
    departments.push({ name: 'Sales', code: 'SAL', description: 'Revenue and customer acquisition' })
  }
  if (normalized.includes('governance') || prompt.includes('治理') || prompt.includes('合规')) {
    departments.push({ name: 'Governance', code: 'GOV', description: 'Governance, risk and compliance' })
  }
  if (normalized.includes('agent') || prompt.includes('AI') || prompt.includes('智能体')) {
    departments.push({ name: 'AI Agent Ops', code: 'AIO', description: 'Agent operations and capability routing' })
  }

  return departments.slice(0, 6)
}

function splitCsv(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function metaResourceMatchesActor(resource: MetaResource, actorType: PositionAssignment['actor_type']): boolean {
  if (actorType === 'internal_human') return resource.resource_type === 'internal_human' || resource.resource_type === 'human'
  if (actorType === 'external_human') return resource.resource_type === 'external_human'
  if (actorType === 'internal_agent') return resource.resource_type === 'internal_agent' || resource.resource_type === 'agent'
  if (actorType === 'external_agent') return resource.resource_type === 'external_agent'
  return false
}

function asArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : []
}
