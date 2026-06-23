'use client'

import {
  Activity,
  BrainCircuit,
  CheckCircle2,
  GitBranch,
  Loader2,
  Plus,
  RefreshCw,
  Save,
  Scale,
  ShieldCheck,
  Sparkles,
  type LucideIcon,
} from 'lucide-react'
import { FormEvent, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import {
  addEdge,
  Background,
  Controls,
  MiniMap,
  ReactFlow,
  useEdgesState,
  useNodesState,
} from '@xyflow/react'
import type { Connection, Edge, Node } from '@xyflow/react'
import { apiRequest } from '@/lib/api'
import { useI18n } from '@/lib/i18n'

interface WorkspaceProps {
  token: string
  currentUserId?: string | null
}

interface Permission {
  id: string
  level: number
  name: string
  description?: string
  behavior: string
}

interface AccessDecision {
  id: string
  master_key: string
  actor_id: string
  actor_type: string
  action: string
  resource: string
  required_level: string
  risk_level: string
  decision: string
  allowed: boolean
  reason: string
  weight_snapshot?: number
  created_at: string
}

interface ContextWeight {
  id: string
  actor_id: string
  actor_type: string
  scope_hash: string
  overall_score: number
  expertise_score: number
  track_record_score: number
  reliability_score: number
  context_fit_score: number
  risk_level: string
  decision_count: number
  last_updated: string
}

interface Capability {
  id: string
  name: string
  version: string
  description?: string
  permission_level: string
  is_active: boolean
}

interface CapabilityEvaluation {
  id: string
  capability_id?: string
  actor_id?: string
  actor_type?: string
  evaluator_type: string
  quality_score: number
  reliability_score: number
  overall_score: number
  conclusion?: string
  created_at: string
}

interface Organization {
  id: string
  name: string
}

interface Department {
  id: string
  name: string
  children?: Department[]
  positions?: Position[]
}

interface Position {
  id: string
  department_id: string
  name: string
  code?: string
  permission_level: string
  required_capabilities?: string[]
}

interface WorkflowTemplate {
  id: string
  name: string
  description?: string
  stages: WorkflowStage[]
  visual_graph?: { nodes?: WorkflowNode[]; edges?: Edge[] }
  organization_id?: string
  department_id?: string
}

interface WorkflowStage {
  id?: string
  type: 'plan' | 'execute' | 'review'
  name: string
  assignee_type: string
  position_id?: string
  required_capabilities?: string[]
  required_permission_level: string
  risk_level: string
  preferred_actor_types?: string[]
  matching_policy?: Record<string, unknown>
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

type WorkflowNodeData = Record<string, unknown> & {
  label: string
  stage_type: 'plan' | 'execute' | 'review'
  name: string
  assignee_type: string
  position_id: string
  required_capabilities: string
  required_permission_level: string
  risk_level: string
}

type WorkflowNode = Node<WorkflowNodeData>

interface CapabilityBridge {
  task_description: string
  required_capabilities?: string[]
  required_level?: string
  risk_level?: string
  capability_match_path: string
  context_weight_path: string
  access_decision_path: string
  workflow_start_path: string
  context: Record<string, unknown>
}

const emptyPermission = {
  level: '2',
  name: 'workflow.execute',
  description: 'Execute workflow task',
  behavior: 'notify',
}

const defaultDecisionForm = {
  actor_type: 'internal_human',
  action: 'workflow.assign',
  resource: 'workflow_task',
  required_level: 'L2',
  risk_level: 'medium',
  weight_snapshot: '0.65',
}

const defaultWeightForm = {
  actor_type: 'internal_human',
  task_type: 'review launch readiness',
  workflow_stage: 'review',
  capability_id: '',
  risk_level: 'medium',
  outcome_score: '0.82',
}

const defaultEvaluationForm = {
  capability_id: '',
  actor_type: 'internal_agent',
  quality_score: '0.8',
  reliability_score: '0.75',
  cost_score: '0.7',
  latency_score: '0.7',
  risk_score: '0.65',
  compliance_score: '0.8',
  conclusion: 'human-reviewed capability evaluation',
}

const defaultMatchForm = {
  organization_id: '',
  department_id: '',
  task_description: 'review launch readiness',
  required_capabilities: 'review, compliance',
  required_level: 'L2',
  risk_level: 'medium',
}

const initialWorkflowNodes: WorkflowNode[] = [
  {
    id: 'plan-1',
    type: 'default',
    position: { x: 40, y: 70 },
    data: {
      label: '需求拆解',
      stage_type: 'plan',
      name: '需求拆解',
      assignee_type: 'internal',
      position_id: '',
      required_capabilities: 'planning',
      required_permission_level: 'L1',
      risk_level: 'low',
    },
  },
  {
    id: 'execute-1',
    type: 'default',
    position: { x: 300, y: 70 },
    data: {
      label: '能力执行',
      stage_type: 'execute',
      name: '能力执行',
      assignee_type: 'either',
      position_id: '',
      required_capabilities: 'delivery, execution',
      required_permission_level: 'L2',
      risk_level: 'medium',
    },
  },
  {
    id: 'review-1',
    type: 'default',
    position: { x: 560, y: 70 },
    data: {
      label: '人工复核',
      stage_type: 'review',
      name: '人工复核',
      assignee_type: 'internal',
      position_id: '',
      required_capabilities: 'review',
      required_permission_level: 'L2',
      risk_level: 'medium',
    },
  },
]

const initialWorkflowEdges: Edge[] = [
  { id: 'plan-1-execute-1', source: 'plan-1', target: 'execute-1' },
  { id: 'execute-1-review-1', source: 'execute-1', target: 'review-1' },
]

function percent(value: number | undefined): string {
  return `${Math.round((value ?? 0) * 100)}%`
}

function splitCsv(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function asArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : []
}

function flattenDepartmentPositions(nodes: Department[]): Position[] {
  return nodes.flatMap((node) => [...(node.positions ?? []), ...flattenDepartmentPositions(node.children ?? [])])
}

export function GovernanceWorkspace({ token, currentUserId }: WorkspaceProps) {
  const [permissions, setPermissions] = useState<Permission[]>([])
  const [decisions, setDecisions] = useState<AccessDecision[]>([])
  const [permissionForm, setPermissionForm] = useState(emptyPermission)
  const [decisionForm, setDecisionForm] = useState(defaultDecisionForm)
  const [decisionActorId, setDecisionActorId] = useState(currentUserId ?? '')
  const [latestDecision, setLatestDecision] = useState<AccessDecision | null>(null)
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    Promise.all([
      apiRequest<Permission[]>('/governance/permissions', { token }),
      apiRequest<AccessDecision[]>('/governance/access/decisions?limit=20', { token }),
    ])
      .then(([permissionData, decisionData]) => {
        if (cancelled) return
        setPermissions(asArray(permissionData))
        setDecisions(asArray(decisionData))
      })
      .catch(() => {
        if (!cancelled) {
          setPermissions([])
          setDecisions([])
        }
      })

    return () => {
      cancelled = true
    }
  }, [token])

  async function loadGovernance() {
    const [permissionData, decisionData] = await Promise.all([
      apiRequest<Permission[]>('/governance/permissions', { token }),
      apiRequest<AccessDecision[]>('/governance/access/decisions?limit=20', { token }),
    ])
    setPermissions(asArray(permissionData))
    setDecisions(asArray(decisionData))
  }

  async function run(action: () => Promise<void>, success: string) {
    setLoading(true)
    setError(null)
    setMessage(null)
    try {
      await action()
      setMessage(success)
    } catch (err) {
      setError(err instanceof Error ? err.message : '操作失败')
    } finally {
      setLoading(false)
    }
  }

  async function createPermission(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(async () => {
      await apiRequest<Permission>('/governance/permissions', {
        method: 'POST',
        token,
        body: {
          level: Number(permissionForm.level),
          name: permissionForm.name,
          description: permissionForm.description,
          behavior: permissionForm.behavior,
        },
      })
      await loadGovernance()
    }, '权限已创建')
  }

  async function decideAccess(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(async () => {
      const decision = await apiRequest<AccessDecision>('/governance/access/decide', {
        method: 'POST',
        token,
        body: {
          actor_id: decisionActorId,
          actor_type: decisionForm.actor_type,
          action: decisionForm.action,
          resource: decisionForm.resource,
          required_level: decisionForm.required_level,
          risk_level: decisionForm.risk_level,
          weight_snapshot: Number(decisionForm.weight_snapshot),
          context: { source: 'frontend_governance_workspace' },
        },
      })
      setLatestDecision(decision)
      await loadGovernance()
    }, '准入决策已生成')
  }

  return (
    <WorkspaceShell
      title="权限策略"
      icon={ShieldCheck}
      message={message}
      error={error}
      onRefresh={() => run(loadGovernance, '权限数据已刷新')}
    >
      <div className="grid gap-5 xl:grid-cols-[0.9fr_1.1fr]">
        <Panel icon={Plus} title="创建权限级别行为">
          <form className="space-y-3" onSubmit={createPermission}>
            <div className="grid gap-3 sm:grid-cols-2">
              <TextInput label="Level" value={permissionForm.level} onChange={(value) => setPermissionForm({ ...permissionForm, level: value })} />
              <SelectInput
                label="行为"
                value={permissionForm.behavior}
                onChange={(value) => setPermissionForm({ ...permissionForm, behavior: value })}
                options={['auto', 'notify', 'approve', 'deny']}
              />
            </div>
            <TextInput label="权限名" value={permissionForm.name} onChange={(value) => setPermissionForm({ ...permissionForm, name: value })} />
            <TextArea
              label="描述"
              value={permissionForm.description}
              onChange={(value) => setPermissionForm({ ...permissionForm, description: value })}
            />
            <SubmitButton loading={loading} label="创建权限" />
          </form>
        </Panel>

        <Panel icon={Scale} title="准入决策">
          <form className="space-y-3" onSubmit={decideAccess}>
            <TextInput label="Actor ID" value={decisionActorId} onChange={setDecisionActorId} />
            <div className="grid gap-3 sm:grid-cols-2">
              <SelectInput
                label="Actor 类型"
                value={decisionForm.actor_type}
                onChange={(value) => setDecisionForm({ ...decisionForm, actor_type: value })}
                options={['internal_human', 'external_human', 'internal_agent', 'external_agent']}
              />
              <SelectInput
                label="风险"
                value={decisionForm.risk_level}
                onChange={(value) => setDecisionForm({ ...decisionForm, risk_level: value })}
                options={['low', 'medium', 'high', 'critical']}
              />
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <TextInput label="动作" value={decisionForm.action} onChange={(value) => setDecisionForm({ ...decisionForm, action: value })} />
              <TextInput label="资源" value={decisionForm.resource} onChange={(value) => setDecisionForm({ ...decisionForm, resource: value })} />
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <SelectInput
                label="权限级别"
                value={decisionForm.required_level}
                onChange={(value) => setDecisionForm({ ...decisionForm, required_level: value })}
                options={['L1', 'L2', 'L3', 'L4']}
              />
              <TextInput
                label="权重快照"
                value={decisionForm.weight_snapshot}
                onChange={(value) => setDecisionForm({ ...decisionForm, weight_snapshot: value })}
              />
            </div>
            <SubmitButton loading={loading || !decisionActorId} label="生成决策" />
          </form>
          {latestDecision && (
            <div className="mt-4 rounded-lg border border-slate-200 bg-slate-50 p-3">
              <div className="flex items-center justify-between gap-3">
                <StatusBadge label={latestDecision.decision} tone={latestDecision.allowed ? 'green' : 'amber'} />
                <span className="text-sm font-semibold text-slate-700">{latestDecision.master_key || latestDecision.required_level}</span>
              </div>
              <p className="mt-2 text-sm text-slate-600">{latestDecision.reason}</p>
            </div>
          )}
        </Panel>
      </div>

      <div className="grid gap-5 xl:grid-cols-2">
        <Panel icon={ShieldCheck} title="权限目录">
          <List>
            {permissions.map((permission) => (
              <ListRow key={permission.id} title={permission.name} detail={`L${permission.level} · ${permission.behavior}`} />
            ))}
          </List>
        </Panel>
        <Panel icon={Activity} title="最近准入审计">
          <List>
            {decisions.map((decision) => (
              <ListRow
                key={decision.id}
                title={`${decision.master_key || decision.id} · ${decision.action} / ${decision.resource}`}
                detail={`${decision.actor_type} · ${decision.decision} · ${decision.reason}`}
                badge={decision.allowed ? 'allowed' : decision.decision}
              />
            ))}
          </List>
        </Panel>
      </div>
    </WorkspaceShell>
  )
}

export function WeightWorkspace({ token, currentUserId }: WorkspaceProps) {
  const { t } = useI18n()
  const [weights, setWeights] = useState<ContextWeight[]>([])
  const [form, setForm] = useState(defaultWeightForm)
  const [actorId, setActorId] = useState(currentUserId ?? '')
  const [latestWeight, setLatestWeight] = useState<ContextWeight | null>(null)
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    apiRequest<ContextWeight[]>('/evolution/context-weights?limit=30', { token })
      .then((data) => {
        if (!cancelled) setWeights(asArray(data))
      })
      .catch(() => {
        if (!cancelled) setWeights([])
      })

    return () => {
      cancelled = true
    }
  }, [token])

  async function loadWeights() {
    const data = await apiRequest<ContextWeight[]>('/evolution/context-weights?limit=30', { token })
    setWeights(asArray(data))
  }

  async function run(action: () => Promise<void>, success: string) {
    setLoading(true)
    setError(null)
    setMessage(null)
    try {
      await action()
      setMessage(success)
    } catch (err) {
      setError(err instanceof Error ? err.message : '操作失败')
    } finally {
      setLoading(false)
    }
  }

  async function computeWeight(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(async () => {
      const weight = await apiRequest<ContextWeight>('/evolution/context-weights/compute', {
        method: 'POST',
        token,
        body: {
          actor_id: actorId,
          actor_type: form.actor_type,
          scope: {
            workflow_stage: form.workflow_stage,
            task_type: form.task_type,
            capability_id: form.capability_id || null,
            risk_level: form.risk_level,
            context: { source: 'frontend_weight_workspace' },
          },
        },
      })
      setLatestWeight(weight)
      await loadWeights()
    }, '上下文权重已计算')
  }

  async function recordOutcome() {
    await run(async () => {
      const weight = await apiRequest<ContextWeight>('/evolution/context-weights/outcome', {
        method: 'POST',
        token,
        body: {
          actor_id: actorId,
          actor_type: form.actor_type,
          outcome_score: Number(form.outcome_score),
          scope: {
            workflow_stage: form.workflow_stage,
            task_type: form.task_type,
            capability_id: form.capability_id || null,
            risk_level: form.risk_level,
            context: { source: 'frontend_weight_workspace' },
          },
        },
      })
      setLatestWeight(weight)
      await loadWeights()
    }, '结果已写入权重')
  }

  return (
    <WorkspaceShell title="权重中心" icon={BrainCircuit} message={message} error={error} onRefresh={() => run(loadWeights, '权重已刷新')}>
      <div className="grid gap-5 xl:grid-cols-[0.9fr_1.1fr]">
        <Panel icon={Scale} title="计算上下文权重">
          <form className="space-y-3" onSubmit={computeWeight}>
            <TextInput label="Actor ID" value={actorId} onChange={setActorId} />
            <div className="grid gap-3 sm:grid-cols-2">
              <SelectInput
                label="Actor 类型"
                value={form.actor_type}
                onChange={(value) => setForm({ ...form, actor_type: value })}
                options={['internal_human', 'external_human', 'internal_agent', 'external_agent']}
              />
              <SelectInput
                label="风险"
                value={form.risk_level}
                onChange={(value) => setForm({ ...form, risk_level: value })}
                options={['low', 'medium', 'high', 'critical']}
              />
            </div>
            <TextInput label="任务类型" value={form.task_type} onChange={(value) => setForm({ ...form, task_type: value })} />
            <div className="grid gap-3 sm:grid-cols-2">
              <TextInput label="工作流节点" value={form.workflow_stage} onChange={(value) => setForm({ ...form, workflow_stage: value })} />
              <TextInput label="能力 ID" value={form.capability_id} onChange={(value) => setForm({ ...form, capability_id: value })} />
            </div>
            <div className="flex flex-wrap gap-2">
              <SubmitButton loading={loading || !actorId} label="计算权重" />
              <button
                type="button"
                onClick={recordOutcome}
                disabled={loading || !actorId}
                className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-300 px-3 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <CheckCircle2 className="h-4 w-4" />}
                {t('记录结果')}
              </button>
            </div>
          </form>
        </Panel>

        <Panel icon={Activity} title="当前权重画像">
          {latestWeight ? (
            <div className="grid gap-3 sm:grid-cols-2">
              <ScoreTile label="总分" value={latestWeight.overall_score} />
              <ScoreTile label="专业" value={latestWeight.expertise_score} />
              <ScoreTile label="历史" value={latestWeight.track_record_score} />
              <ScoreTile label="可靠性" value={latestWeight.reliability_score} />
              <ScoreTile label="上下文匹配" value={latestWeight.context_fit_score} />
              <ScoreTile label="决策次数" value={latestWeight.decision_count / 10} display={String(latestWeight.decision_count)} />
            </div>
          ) : (
            <EmptyText>尚未计算权重</EmptyText>
          )}
        </Panel>
      </div>

      <Panel icon={BrainCircuit} title="上下文权重排行">
        <List>
          {weights.map((weight) => (
            <ListRow
              key={weight.id}
              title={`${weight.actor_type} · ${weight.actor_id}`}
              detail={`${weight.risk_level} · ${weight.scope_hash.slice(0, 12)} · ${weight.decision_count} decisions`}
              badge={percent(weight.overall_score)}
            />
          ))}
        </List>
      </Panel>
    </WorkspaceShell>
  )
}

export function CapabilityEvaluationWorkspace({ token, currentUserId }: WorkspaceProps) {
  const { t } = useI18n()
  const [capabilities, setCapabilities] = useState<Capability[]>([])
  const [evaluations, setEvaluations] = useState<CapabilityEvaluation[]>([])
  const [form, setForm] = useState(defaultEvaluationForm)
  const [actorId, setActorId] = useState(currentUserId ?? '')
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    Promise.all([
      apiRequest<Capability[]>('/capabilities', { token }),
      apiRequest<CapabilityEvaluation[]>('/capabilities/evaluations?limit=30', { token }),
    ])
      .then(([capabilityData, evaluationData]) => {
        if (cancelled) return
        const nextCapabilities = asArray(capabilityData)
        setCapabilities(nextCapabilities)
        setEvaluations(asArray(evaluationData))
        if (nextCapabilities.length > 0) {
          setForm((current) => (current.capability_id ? current : { ...current, capability_id: nextCapabilities[0].id }))
        }
      })
      .catch(() => {
        if (cancelled) return
        setCapabilities([])
        setEvaluations([])
      })

    return () => {
      cancelled = true
    }
  }, [token])

  async function loadEvaluationData() {
    const [capabilityData, evaluationData] = await Promise.all([
      apiRequest<Capability[]>('/capabilities', { token }),
      apiRequest<CapabilityEvaluation[]>('/capabilities/evaluations?limit=30', { token }),
    ])
    const nextCapabilities = asArray(capabilityData)
    setCapabilities(nextCapabilities)
    setEvaluations(asArray(evaluationData))
    if (!form.capability_id && nextCapabilities.length > 0) {
      setForm((current) => ({ ...current, capability_id: nextCapabilities[0].id }))
    }
  }

  async function run(action: () => Promise<void>, success: string) {
    setLoading(true)
    setError(null)
    setMessage(null)
    try {
      await action()
      setMessage(success)
    } catch (err) {
      setError(err instanceof Error ? err.message : '操作失败')
    } finally {
      setLoading(false)
    }
  }

  async function createEvaluation(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await run(async () => {
      await apiRequest<CapabilityEvaluation>('/capabilities/evaluations', {
        method: 'POST',
        token,
        body: {
          capability_id: form.capability_id || null,
          actor_id: actorId || null,
          actor_type: form.actor_type,
          evaluator_id: currentUserId || null,
          evaluator_type: 'human',
          quality_score: Number(form.quality_score),
          reliability_score: Number(form.reliability_score),
          cost_score: Number(form.cost_score),
          latency_score: Number(form.latency_score),
          risk_score: Number(form.risk_score),
          compliance_score: Number(form.compliance_score),
          evidence: { source: 'frontend_capability_evaluation', risk_level: 'medium' },
          conclusion: form.conclusion,
        },
      })
      await loadEvaluationData()
    }, '能力评估已提交')
  }

  const selectedCapability = useMemo(
    () => capabilities.find((capability) => capability.id === form.capability_id),
    [capabilities, form.capability_id],
  )

  return (
    <WorkspaceShell
      title="能力评估"
      icon={CheckCircle2}
      message={message}
      error={error}
      onRefresh={() => run(loadEvaluationData, '评估数据已刷新')}
    >
      <div className="grid gap-5 xl:grid-cols-[0.9fr_1.1fr]">
        <Panel icon={CheckCircle2} title="人类评估">
          <form className="space-y-3" onSubmit={createEvaluation}>
            <label className="block">
              <span className="text-sm font-medium text-slate-700">{t('能力')}</span>
              <select
                value={form.capability_id}
                onChange={(event) => setForm({ ...form, capability_id: event.target.value })}
                className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
              >
                <option value="">{t('仅评估 Actor')}</option>
                {capabilities.map((capability) => (
                  <option key={capability.id} value={capability.id}>
                    {capability.name} {capability.version}
                  </option>
                ))}
              </select>
            </label>
            <div className="grid gap-3 sm:grid-cols-2">
              <TextInput label="Actor ID" value={actorId} onChange={setActorId} />
              <SelectInput
                label="Actor 类型"
                value={form.actor_type}
                onChange={(value) => setForm({ ...form, actor_type: value })}
                options={['internal_human', 'external_human', 'internal_agent', 'external_agent']}
              />
            </div>
            <div className="grid gap-3 sm:grid-cols-3">
              <TextInput label="质量" value={form.quality_score} onChange={(value) => setForm({ ...form, quality_score: value })} />
              <TextInput label="可靠性" value={form.reliability_score} onChange={(value) => setForm({ ...form, reliability_score: value })} />
              <TextInput label="合规" value={form.compliance_score} onChange={(value) => setForm({ ...form, compliance_score: value })} />
              <TextInput label="成本" value={form.cost_score} onChange={(value) => setForm({ ...form, cost_score: value })} />
              <TextInput label="时延" value={form.latency_score} onChange={(value) => setForm({ ...form, latency_score: value })} />
              <TextInput label="风险控制" value={form.risk_score} onChange={(value) => setForm({ ...form, risk_score: value })} />
            </div>
            <TextArea label="结论" value={form.conclusion} onChange={(value) => setForm({ ...form, conclusion: value })} />
            <SubmitButton loading={loading || (!form.capability_id && !actorId)} label="提交评估" />
          </form>
        </Panel>

        <Panel icon={Sparkles} title="当前能力">
          {selectedCapability ? (
            <div className="space-y-3">
              <ListRow
                title={selectedCapability.name}
                detail={`${selectedCapability.version} · ${selectedCapability.permission_level}`}
                badge={selectedCapability.is_active ? 'active' : 'inactive'}
              />
              <p className="text-sm text-slate-600">{selectedCapability.description || t('无描述')}</p>
            </div>
          ) : (
            <EmptyText>选择一个能力或仅评估 Actor</EmptyText>
          )}
        </Panel>
      </div>

      <Panel icon={Activity} title="最近评估">
        <List>
          {evaluations.map((evaluation) => (
            <ListRow
              key={evaluation.id}
              title={evaluation.capability_id || evaluation.actor_id || evaluation.id}
              detail={`${evaluation.evaluator_type} · quality ${percent(evaluation.quality_score)} · reliability ${percent(evaluation.reliability_score)}`}
              badge={percent(evaluation.overall_score)}
            />
          ))}
        </List>
      </Panel>
    </WorkspaceShell>
  )
}

export function WorkflowDesignerWorkspace({ token }: WorkspaceProps) {
  const { t } = useI18n()
  const [organizationId, setOrganizationId] = useState('')
  const [departments, setDepartments] = useState<Department[]>([])
  const [departmentId, setDepartmentId] = useState('')
  const [positions, setPositions] = useState<Position[]>([])
  const [templates, setTemplates] = useState<WorkflowTemplate[]>([])
  const [templateName, setTemplateName] = useState('岗位协同工作流')
  const [description, setDescription] = useState('基于组织岗位、能力和权限级别匹配执行节点')
  const [nodes, setNodes, onNodesChange] = useNodesState<WorkflowNode>(initialWorkflowNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialWorkflowEdges)
  const [selectedNodeId, setSelectedNodeId] = useState(initialWorkflowNodes[0].id)
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const selectedNode = nodes.find((node) => node.id === selectedNodeId) ?? nodes[0]

  useEffect(() => {
    apiRequest<{ organization: { id: string }; departments: Department[] }>('/organization/current', { token })
      .then((data) => {
        setOrganizationId(data.organization.id)
        const tree = Array.isArray(data.departments) ? data.departments : []
        const flatDepartments = flattenDepartments(tree)
        setDepartments(flatDepartments)
        setDepartmentId((current) => current || flatDepartments[0]?.id || '')
        setPositions(flattenDepartmentPositions(tree))
      })
      .catch((err) => setError(err instanceof Error ? err.message : t('加载组织失败')))

    apiRequest<WorkflowTemplate[]>('/workflows/templates', { token })
      .then((data) => setTemplates(Array.isArray(data) ? data : []))
      .catch(() => setTemplates([]))
  }, [t, token])

  function handleConnect(connection: Connection) {
    setEdges((current) => addEdge(connection, current))
  }

  function addStage(stageType: WorkflowNodeData['stage_type']) {
    const id = `${stageType}-${Date.now()}`
    const label = stageType === 'plan' ? '计划节点' : stageType === 'execute' ? '执行节点' : '复核节点'
    setNodes((current) => [
      ...current,
      {
        id,
        type: 'default',
        position: { x: 100 + current.length * 80, y: 160 + current.length * 20 },
        data: {
          label,
          stage_type: stageType,
          name: label,
          assignee_type: stageType === 'review' ? 'internal' : 'either',
          position_id: '',
          required_capabilities: '',
          required_permission_level: stageType === 'plan' ? 'L1' : 'L2',
          risk_level: stageType === 'plan' ? 'low' : 'medium',
        },
      },
    ])
    setSelectedNodeId(id)
  }

  function updateSelectedNode(patch: Partial<WorkflowNodeData>) {
    if (!selectedNode) return
    setNodes((current) =>
      current.map((node) => {
        if (node.id !== selectedNode.id) return node
        const data = { ...node.data, ...patch }
        const position = positions.find((item) => item.id === data.position_id)
        return {
          ...node,
          data: {
            ...data,
            label: `${data.name}${position ? ` · ${position.name}` : ''}`,
          },
        }
      }),
    )
  }

  async function saveWorkflowTemplate() {
    if (nodes.length === 0) return
    setLoading(true)
    setError(null)
    setMessage(null)
    try {
      const stages = nodes
        .slice()
        .sort((a, b) => a.position.x - b.position.x || a.position.y - b.position.y)
        .map<WorkflowStage>((node) => ({
          id: node.id,
          type: node.data.stage_type,
          name: node.data.name,
          assignee_type: node.data.assignee_type,
          position_id: node.data.position_id || undefined,
          required_capabilities: splitCsv(node.data.required_capabilities),
          required_permission_level: node.data.required_permission_level,
          risk_level: node.data.risk_level,
          preferred_actor_types:
            node.data.assignee_type === 'internal'
              ? ['internal_human', 'internal_agent']
              : ['internal_human', 'external_human', 'internal_agent', 'external_agent'],
          matching_policy: {
            mode: 'position_capability_permission',
            source: 'workflow_designer',
          },
        }))

      await apiRequest<WorkflowTemplate>('/workflows/templates', {
        method: 'POST',
        token,
          body: {
            name: templateName,
            description,
            organization_id: organizationId || null,
            department_id: departmentId || null,
            stages,
            assignee_type: 'either',
            required_weight: 0.6,
            routing_rules: { organization_id: organizationId || null, department_id: departmentId || null, source: 'workflow_designer' },
            visual_graph: { nodes, edges },
          },
      })
      const data = await apiRequest<WorkflowTemplate[]>('/workflows/templates', { token })
      setTemplates(Array.isArray(data) ? data : [])
      setMessage(t('工作流模板已创建'))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('操作失败'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <WorkspaceShell title="工作流设计" icon={GitBranch} message={message} error={error} onRefresh={() => setMessage(t('可重新执行匹配'))}>
      <div className="grid gap-5 xl:grid-cols-[1.3fr_0.7fr]">
        <Panel icon={GitBranch} title="拖拽流程画布">
          <div className="mb-3 flex flex-wrap gap-2">
            <ActionButtonLike loading={loading} icon={Plus} label="计划节点" onClick={() => addStage('plan')} />
            <ActionButtonLike loading={loading} icon={Sparkles} label="执行节点" onClick={() => addStage('execute')} />
            <ActionButtonLike loading={loading} icon={CheckCircle2} label="复核节点" onClick={() => addStage('review')} />
          </div>
          <div className="h-[460px] overflow-hidden rounded-lg border border-slate-200 bg-slate-50">
            <ReactFlow
              nodes={nodes}
              edges={edges}
              onNodesChange={onNodesChange}
              onEdgesChange={onEdgesChange}
              onConnect={handleConnect}
              onNodeClick={(_, node) => setSelectedNodeId(node.id)}
              fitView
            >
              <MiniMap />
              <Controls />
              <Background />
            </ReactFlow>
          </div>
        </Panel>

        <Panel icon={Scale} title="节点与模板属性">
          <div className="space-y-3">
            <TextInput label="名称" value={templateName} onChange={setTemplateName} />
            <TextArea label="描述" value={description} onChange={setDescription} />
            <label className="block">
              <span className="text-sm font-medium text-slate-700">{t('部门')}</span>
              <select
                value={departmentId}
                onChange={(event) => setDepartmentId(event.target.value)}
                className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
              >
                <option value="">{t('未选择')}</option>
                {departments.map((department) => (
                  <option key={department.id} value={department.id}>
                    {department.name}
                  </option>
                ))}
              </select>
            </label>
            {selectedNode && (
              <div className="rounded-lg border border-slate-200 p-3">
                <p className="mb-3 text-sm font-semibold text-slate-950">{t('当前节点')}</p>
                <TextInput label="节点名称" value={selectedNode.data.name} onChange={(value) => updateSelectedNode({ name: value })} />
                <div className="mt-3 grid gap-3 sm:grid-cols-2">
                  <SelectInput label="节点类型" value={selectedNode.data.stage_type} options={['plan', 'execute', 'review']} onChange={(value) => updateSelectedNode({ stage_type: value as WorkflowNodeData['stage_type'] })} />
                  <SelectInput label="权限级别" value={selectedNode.data.required_permission_level} options={['L1', 'L2', 'L3', 'L4']} onChange={(value) => updateSelectedNode({ required_permission_level: value })} />
                </div>
                <div className="mt-3 grid gap-3 sm:grid-cols-2">
                  <SelectInput label="风险" value={selectedNode.data.risk_level} options={['low', 'medium', 'high', 'critical']} onChange={(value) => updateSelectedNode({ risk_level: value })} />
                  <SelectInput label="Actor 类型" value={selectedNode.data.assignee_type} options={['internal', 'external', 'either']} onChange={(value) => updateSelectedNode({ assignee_type: value })} />
                </div>
                <label className="mt-3 block">
                  <span className="text-sm font-medium text-slate-700">{t('岗位')}</span>
                  <select
                    value={selectedNode.data.position_id}
                    onChange={(event) => {
                      const position = positions.find((item) => item.id === event.target.value)
                      updateSelectedNode({
                        position_id: event.target.value,
                        required_permission_level: position?.permission_level ?? selectedNode.data.required_permission_level,
                        required_capabilities: position?.required_capabilities?.join(', ') ?? selectedNode.data.required_capabilities,
                      })
                    }}
                    className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                  >
                    <option value="">{t('未选择')}</option>
                    {positions.map((position) => (
                      <option key={position.id} value={position.id}>
                        {position.name} · {position.permission_level}
                      </option>
                    ))}
                  </select>
                </label>
                <div className="mt-3">
                  <TextInput label="所需能力" value={selectedNode.data.required_capabilities} onChange={(value) => updateSelectedNode({ required_capabilities: value })} />
                </div>
              </div>
            )}
            <button
              type="button"
              onClick={saveWorkflowTemplate}
              disabled={loading || !templateName}
              className="inline-flex h-10 items-center gap-2 rounded-lg bg-slate-950 px-3 text-sm font-semibold text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
              {t('保存流程模板')}
            </button>
          </div>
        </Panel>
      </div>

      <Panel icon={Activity} title="模板列表">
        <List>
          {templates.map((template) => (
            <ListRow
              key={template.id}
              title={template.name}
              detail={`${template.stages?.length ?? 0} stages · ${template.description ?? ''}`}
              badge={template.organization_id ? 'active' : 'draft'}
            />
          ))}
        </List>
      </Panel>
    </WorkspaceShell>
  )
}

export function WorkflowMatchingWorkspace({ token }: WorkspaceProps) {
  const { t } = useI18n()
  const [organizations, setOrganizations] = useState<Organization[]>([])
  const [departments, setDepartments] = useState<Department[]>([])
  const [form, setForm] = useState(defaultMatchForm)
  const [candidates, setCandidates] = useState<MatchCandidate[]>([])
  const [bridge, setBridge] = useState<CapabilityBridge | null>(null)
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    apiRequest<{ organization: Organization }>('/organization/current', { token })
      .then((data) => {
        const nextOrganizations = data.organization ? [data.organization] : []
        setOrganizations(nextOrganizations)
        if (nextOrganizations.length > 0) {
          setForm((current) => (current.organization_id ? current : { ...current, organization_id: nextOrganizations[0].id }))
        }
      })
      .catch(() => setOrganizations([]))
  }, [token])

  useEffect(() => {
    if (!form.organization_id) return
    apiRequest<Department[]>(`/organizations/${form.organization_id}/departments/tree`, { token })
      .then((data) => {
        const flat = flattenDepartments(asArray(data))
        setDepartments(flat)
        if (flat.length > 0) {
          setForm((current) => (current.department_id ? current : { ...current, department_id: flat[0].id }))
        }
      })
      .catch(() => setDepartments([]))
  }, [form.organization_id, token])

  async function run(action: () => Promise<void>, success: string) {
    setLoading(true)
    setError(null)
    setMessage(null)
    try {
      await action()
      setMessage(success)
    } catch (err) {
      setError(err instanceof Error ? err.message : '操作失败')
    } finally {
      setLoading(false)
    }
  }

  async function matchMembers() {
    await run(async () => {
      const data = await apiRequest<MatchCandidate[]>('/organization/match-members', {
        method: 'POST',
        token,
        body: {
          organization_id: form.organization_id,
          department_id: form.department_id || null,
          task_description: form.task_description,
          required_capabilities: splitCsv(form.required_capabilities),
          required_level: form.required_level,
          risk_level: form.risk_level,
          member_types: ['internal', 'external', 'agent'],
        },
      })
      setCandidates(asArray(data))
    }, '成员匹配已完成')
  }

  async function matchCapabilities() {
    await run(async () => {
      const data = await apiRequest<CapabilityBridge>('/organization/match-capabilities', {
        method: 'POST',
        token,
        body: {
          department_id: form.department_id || null,
          task_description: form.task_description,
          required_capabilities: splitCsv(form.required_capabilities),
          required_level: form.required_level,
          risk_level: form.risk_level,
          context: { organization_id: form.organization_id },
        },
      })
      setBridge(data)
    }, '能力桥接已生成')
  }

  async function createWorkflowTemplate() {
    await run(async () => {
      await apiRequest('/workflows/templates', {
        method: 'POST',
        token,
        body: {
          name: `需求流程 ${new Date().toLocaleDateString('zh-CN')}`,
          description: form.task_description,
          assignee_type: 'either',
          required_weight: 0.6,
          routing_rules: { organization_id: form.organization_id, department_id: form.department_id },
          stages: [
            {
              type: 'plan',
              name: '需求拆解',
              assignee_type: 'internal',
              required_capabilities: ['planning'],
              required_permission_level: 'L1',
              risk_level: 'low',
              preferred_actor_types: ['internal_human'],
            },
            {
              type: 'execute',
              name: '能力执行',
              assignee_type: 'either',
              required_capabilities: splitCsv(form.required_capabilities),
              required_permission_level: form.required_level,
              risk_level: form.risk_level,
              preferred_actor_types: ['internal_human', 'internal_agent', 'external_agent'],
            },
            {
              type: 'review',
              name: '人工复核',
              assignee_type: 'internal',
              required_capabilities: ['review'],
              required_permission_level: 'L2',
              risk_level: form.risk_level,
              preferred_actor_types: ['internal_human'],
              evaluation_policy: { primary_reviewer: 'human', update_weight: true },
            },
          ],
        },
      })
    }, '工作流模板已创建')
  }

  return (
    <WorkspaceShell
      title="工作流匹配"
      icon={GitBranch}
      message={message}
      error={error}
      onRefresh={() => setMessage(t('可重新执行匹配'))}
    >
      <Panel icon={Sparkles} title="业务需求匹配">
        <div className="grid gap-3 xl:grid-cols-2">
          <label className="block">
            <span className="text-sm font-medium text-slate-700">{t('组织')}</span>
            <select
              value={form.organization_id}
              onChange={(event) => setForm({ ...form, organization_id: event.target.value, department_id: '' })}
              className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
            >
              {organizations.map((organization) => (
                <option key={organization.id} value={organization.id}>
                  {organization.name}
                </option>
              ))}
            </select>
          </label>
          <label className="block">
            <span className="text-sm font-medium text-slate-700">{t('部门')}</span>
            <select
              value={form.department_id}
              onChange={(event) => setForm({ ...form, department_id: event.target.value })}
              className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
            >
              <option value="">{t('全组织')}</option>
              {departments.map((department) => (
                <option key={department.id} value={department.id}>
                  {department.name}
                </option>
              ))}
            </select>
          </label>
          <div className="xl:col-span-2">
            <TextArea label="业务需求" value={form.task_description} onChange={(value) => setForm({ ...form, task_description: value })} />
          </div>
          <TextInput
            label="所需能力"
            value={form.required_capabilities}
            onChange={(value) => setForm({ ...form, required_capabilities: value })}
          />
          <div className="grid gap-3 sm:grid-cols-2">
            <SelectInput
              label="权限级别"
              value={form.required_level}
              onChange={(value) => setForm({ ...form, required_level: value })}
              options={['L1', 'L2', 'L3', 'L4']}
            />
            <SelectInput
              label="风险"
              value={form.risk_level}
              onChange={(value) => setForm({ ...form, risk_level: value })}
              options={['low', 'medium', 'high', 'critical']}
            />
          </div>
        </div>
        <div className="mt-4 flex flex-wrap gap-2">
          <button
            type="button"
            onClick={matchMembers}
            disabled={loading || !form.organization_id}
            className="inline-flex h-10 items-center gap-2 rounded-lg bg-slate-950 px-3 text-sm font-semibold text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Sparkles className="h-4 w-4" />}
            {t('匹配成员')}
          </button>
          <button
            type="button"
            onClick={matchCapabilities}
            disabled={loading}
            className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-300 px-3 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
          >
            <BrainCircuit className="h-4 w-4" />
            {t('生成能力桥接')}
          </button>
          <button
            type="button"
            onClick={createWorkflowTemplate}
            disabled={loading}
            className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-300 px-3 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
          >
            <Plus className="h-4 w-4" />
            {t('创建流程模板')}
          </button>
        </div>
      </Panel>

      <div className="grid gap-5 xl:grid-cols-2">
        <Panel icon={Activity} title="候选成员">
          <List>
            {candidates.map((candidate) => (
              <ListRow
                key={candidate.membership_id}
                title={candidate.member_name}
                detail={`${candidate.member_type} · ${candidate.reason}`}
                badge={`${percent(candidate.score)} · ${candidate.requires_approval ? 'approval' : candidate.access_decision}`}
              />
            ))}
          </List>
        </Panel>
        <Panel icon={BrainCircuit} title="能力桥接">
          {bridge ? <JsonBlock value={bridge} /> : <EmptyText>尚未生成能力桥接</EmptyText>}
        </Panel>
      </div>
    </WorkspaceShell>
  )
}

function WorkspaceShell({
  title,
  icon: Icon,
  message,
  error,
  onRefresh,
  children,
}: {
  title: string
  icon: typeof ShieldCheck
  message?: string | null
  error?: string | null
  onRefresh?: () => void
  children: ReactNode
}) {
  const { t } = useI18n()
  return (
    <div className="space-y-5">
      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-2">
            <Icon className="h-5 w-5 text-slate-500" />
            <h2 className="text-base font-semibold text-slate-950">{t(title)}</h2>
          </div>
          {onRefresh && (
            <button
              type="button"
              onClick={onRefresh}
              className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-300 px-3 text-sm font-semibold text-slate-700 hover:bg-slate-100"
            >
              <RefreshCw className="h-4 w-4" />
              {t('刷新')}
            </button>
          )}
        </div>
        {(message || error) && (
          <div
            className={`mt-4 rounded-lg border px-4 py-3 text-sm ${
              error ? 'border-red-200 bg-red-50 text-red-700' : 'border-emerald-200 bg-emerald-50 text-emerald-700'
            }`}
          >
            {t(error || message || '')}
          </div>
        )}
      </section>
      {children}
    </div>
  )
}

function ActionButtonLike({
  icon: Icon,
  loading,
  onClick,
  label,
}: {
  icon: LucideIcon
  loading: boolean
  onClick: () => void
  label: string
}) {
  const { t } = useI18n()
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={loading}
      className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-300 px-3 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
    >
      {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Icon className="h-4 w-4" />}
      {t(label)}
    </button>
  )
}

function Panel({
  icon: Icon,
  title,
  children,
}: {
  icon: LucideIcon
  title: string
  children: ReactNode
}) {
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

function TextInput({
  label,
  value,
  onChange,
}: {
  label: string
  value: string
  onChange: (value: string) => void
}) {
  const { t } = useI18n()
  return (
    <label className="block">
      <span className="text-sm font-medium text-slate-700">{t(label)}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
      />
    </label>
  )
}

function TextArea({
  label,
  value,
  onChange,
}: {
  label: string
  value: string
  onChange: (value: string) => void
}) {
  const { t } = useI18n()
  return (
    <label className="block">
      <span className="text-sm font-medium text-slate-700">{t(label)}</span>
      <textarea
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-24 w-full resize-y rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
      />
    </label>
  )
}

function SelectInput({
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
        className="mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
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

function List({ children }: { children: ReactNode }) {
  return <div className="divide-y divide-slate-100">{children}</div>
}

function ListRow({ title, detail, badge }: { title: string; detail: string; badge?: string }) {
  return (
    <div className="grid gap-2 py-3 first:pt-0 last:pb-0 sm:grid-cols-[1fr_auto]">
      <div className="min-w-0">
        <p className="truncate text-sm font-semibold text-slate-950">{title}</p>
        <p className="mt-1 line-clamp-2 text-sm text-slate-500">{detail}</p>
      </div>
      {badge && <StatusBadge label={badge} tone={badge.includes('deny') ? 'red' : badge.includes('approval') ? 'amber' : 'green'} />}
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

  return <span className={`inline-flex h-7 items-center rounded-md border px-2.5 text-xs font-semibold ${toneClass}`}>{t(label)}</span>
}

function ScoreTile({ label, value, display }: { label: string; value: number; display?: string }) {
  const { t } = useI18n()
  const width = `${Math.max(Math.min(value * 100, 100), 2)}%`
  return (
    <div className="rounded-lg border border-slate-200 p-3">
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm font-medium text-slate-600">{t(label)}</span>
        <span className="text-sm font-semibold text-slate-950">{display ?? percent(value)}</span>
      </div>
      <div className="mt-3 h-2 rounded-full bg-slate-100">
        <div className="h-2 rounded-full bg-slate-700" style={{ width }} />
      </div>
    </div>
  )
}

function JsonBlock({ value }: { value: unknown }) {
  return (
    <pre className="max-h-[360px] overflow-auto rounded-lg border border-slate-200 bg-slate-50 p-3 text-sm text-slate-800">
      {JSON.stringify(value, null, 2)}
    </pre>
  )
}

function EmptyText({ children }: { children: ReactNode }) {
  const { t } = useI18n()
  return <p className="text-sm text-slate-500">{typeof children === 'string' ? t(children) : children}</p>
}

function flattenDepartments(nodes: Department[]): Department[] {
  return nodes.flatMap((node) => [node, ...flattenDepartments(node.children ?? [])])
}
