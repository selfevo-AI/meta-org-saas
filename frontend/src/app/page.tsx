'use client'

import {
  Activity,
  ArrowDown,
  ArrowUp,
  Bot,
  Boxes,
  BrainCircuit,
  BriefcaseBusiness,
  ChevronDown,
  ChevronRight,
  CheckCircle2,
  CircleDollarSign,
  Code2,
  FolderKanban,
  Gauge,
  GitBranch,
  Github,
  Home as HomeIcon,
  KeyRound,
  LogOut,
  Menu,
  Moon,
  MoreHorizontal,
  PackageCheck,
  RefreshCw,
  Send,
  ShieldCheck,
  SlidersHorizontal,
  Sparkles,
  ShoppingCart,
  Sun,
  Users,
  WalletCards,
  Workflow,
  X,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { CSSProperties, DragEvent, FormEvent, PointerEvent as ReactPointerEvent } from 'react'
import {
  activateAssistantSkill,
  approveToolApproval,
  apiRequest,
  completeOnboarding,
  confirmAssistantProposal,
  createAssistantSkill,
  getUserPreference,
  getMe,
  getMetaOrgInbox,
  getMetaOrgOverview,
  listAssistantContextTargets,
  listAssistantProposals,
  listAssistantSkills,
  listModels,
  listRoles,
  listSaaSModules,
  login,
  registerUser,
  rejectAssistantProposal,
  rejectToolApproval,
  runAssistantSkill,
  saveUserPreference,
} from '@/lib/api'
import type {
  AssistantBusinessSkill,
  AssistantContextTarget,
  AssistantProposal,
  AssistantSkillComponent,
  InboxItem,
  MetaOrgOverview,
  ModelCatalogItem,
  Role,
  SaaSModule,
  SessionOrganization,
} from '@/lib/api'
import { clearSession, getCurrentOrganizationId, getSessionUser, getToken, setCurrentOrganizationId, setSession } from '@/lib/auth'
import { useI18n } from '@/lib/i18n'
import { apiOperations, getOperationProfile, operationDomains } from '@/lib/operations'
import type { ApiOperation } from '@/lib/operations'
import { AIAssistant } from './ai-assistant'
import {
  CapabilityEvaluationWorkspace,
  GovernanceWorkspace,
  WeightWorkspace,
  WorkflowDesignerWorkspace,
  WorkflowMatchingWorkspace,
} from './control-workspaces'
import { CostingWorkspace } from './costing-workspace'
import { DeveloperToolsWorkspace } from './developer-tools-workspace'
import { FinanceWorkspace } from './finance-workspace'
import { InventoryWorkspace } from './inventory-workspace'
import { MetaResourceWorkspace } from './meta-resource-workspace'
import { OrganizationWorkspace } from './organization-workspace'
import { ProcurementWorkspace } from './procurement-workspace'
import { ProjectLifecycleWorkspace } from './project-lifecycle-workspace'
import { SalesWorkspace } from './sales-workspace'
import { SystemAdminWorkspace } from './system-admin-workspace'

type AuthMode = 'login' | 'register'
type LoginSurface = 'tenant' | 'platform'
type WorkspaceView = 'overview' | `domain:${string}`
type ThemeMode = 'dark' | 'light'

const domainLabels: Record<string, string> = {
  MetaOrg: 'Meta-Org',
  Dashboard: '系统',
  Identity: '身份',
  Organization: '组织',
  Layer: '分层',
  Capability: '能力',
  Workflow: '工作流',
  Observability: '可观测',
  Verification: '验证',
  Governance: '治理',
  Evolution: '自进化',
  Requirement: '需求',
  Project: '项目',
  Delivery: '交付',
  Cost: '成本',
  Feedback: '反馈评估',
  DeveloperTools: '模型设置',
  SystemAdmin: 'SystemAdmin',
  Finance: '财务核算',
  Costing: '成本核算',
  FinanceAccounting: '财务核算',
  FinanceReceivables: '应收',
  FinancePayables: '应付',
  FinanceCostAccounting: '成本核算',
  MetaResource: 'Meta 资源',
  Inventory: '库存',
  Procurement: '采购',
  Sales: '销售',
}

const domainIcons: Record<string, typeof Gauge> = {
  MetaOrg: Sparkles,
  Dashboard: Gauge,
  Identity: KeyRound,
  Organization: Users,
  Layer: Boxes,
  Capability: BrainCircuit,
  Workflow,
  Observability: Activity,
  Verification: CheckCircle2,
  Governance: ShieldCheck,
  Evolution: GitBranch,
  Requirement: BriefcaseBusiness,
  Project: FolderKanban,
  Delivery: ArrowUp,
  Cost: CircleDollarSign,
  Feedback: Activity,
  DeveloperTools: Code2,
  SystemAdmin: SlidersHorizontal,
  Finance: WalletCards,
  Costing: CircleDollarSign,
  FinanceAccounting: WalletCards,
  FinanceReceivables: ArrowDown,
  FinancePayables: ArrowUp,
  FinanceCostAccounting: CircleDollarSign,
  MetaResource: Boxes,
  Inventory: Boxes,
  Procurement: PackageCheck,
  Sales: ShoppingCart,
}

function defaultSkillComponents(prompt: string): AssistantSkillComponent[] {
  return [
    {
      key: 'intent',
      label: { zh: '意图', en: 'Intent' },
      weight: 0.3,
      instruction: 'Identify the business intent and expected outcome.',
      permission_tags: ['skill:read'],
    },
    {
      key: 'context',
      label: { zh: '上下文', en: 'Context' },
      weight: 0.4,
      instruction: 'Use governed context for the selected module, function, and target.',
      required_context: ['module', 'target'],
      permission_tags: ['context:read'],
    },
    {
      key: 'action',
      label: { zh: '动作', en: 'Action' },
      weight: 0.3,
      instruction: prompt.trim() || 'Execute the skill prompt and produce the next business action.',
      permission_tags: ['skill:run'],
    },
  ]
}

type MenuGroup = {
  id: string
  label: string
  domains: string[]
}

type BusinessTargetType =
  | 'requirement'
  | 'project'
  | 'organization'
  | 'department'
  | 'position'
  | 'meta_resource'
  | 'finance_settlement'
  | 'finance_receivable'
  | 'finance_payable'
  | 'business_partner'
  | 'inventory_item'
  | 'warehouse'
  | 'inventory_balance'
  | 'inventory_movement'
  | 'purchase_requisition'
  | 'purchase_order'
  | 'purchase_receipt'
  | 'purchase_return'
  | 'sales_quotation'
  | 'sales_order'
  | 'sales_shipment'
  | 'sales_return'
  | 'cost_rate_card'
  | 'cost_budget'
  | 'cost_ledger_entry'
  | 'developer_record'
  | 'api_operation'

type BusinessRecord = Record<string, unknown>

type BusinessTreeNode = {
  id: string
  domain: string
  targetType: BusinessTargetType
  targetID?: string
  label: string
  description?: string
  status?: string
  children?: BusinessTreeNode[]
  record?: BusinessRecord
}

type BusinessSelection = BusinessTreeNode

type OverviewBusinessFunction = {
  id: string
  domain: string
  moduleKey: string
  targetType: string
  label: string
  intentKey: string
  icon: typeof Gauge
}

type OrganizationTreeRecord = {
  id: string
  name: string
  description?: string
  status?: string
  created_at?: string
  updated_at?: string
}

type DepartmentTreeRecord = {
  id: string
  organization_id: string
  parent_id?: string
  name: string
  code?: string
  description?: string
  status?: string
  children?: DepartmentTreeRecord[] | null
  positions?: PositionTreeRecord[] | null
}

type PositionTreeRecord = {
  id: string
  organization_id: string
  department_id: string
  name: string
  code?: string
  description?: string
  status?: string
  permission_level?: string
  required_capabilities?: string[]
  assignments?: BusinessRecord[] | null
}

type WorkspaceLayoutWidths = {
  menu: number
  business: number
  status: number
}

type WorkspaceLayoutPane = keyof WorkspaceLayoutWidths

const lifecycleDomains = ['Requirement', 'Project', 'Delivery', 'Cost', 'Feedback']
const virtualDomains = ['Costing', 'MetaResource', 'SystemAdmin']
const dedicatedDomains = new Set([
  'MetaResource',
  'SystemAdmin',
  'Organization',
  'Governance',
  'Evolution',
  'Capability',
  'Workflow',
  'DeveloperTools',
  'Finance',
  'Costing',
  'FinanceAccounting',
  'FinanceReceivables',
  'FinancePayables',
  'FinanceCostAccounting',
  'Inventory',
  'Procurement',
  'Sales',
  ...lifecycleDomains,
])
const menuStorageKey = 'meta_org.menu.groups.v2'
const expandedMenuStorageKey = 'meta_org.menu.expanded.v2'
const themeStorageKey = 'meta_org.theme.v1'
const workspaceLayoutPreferenceKey = 'workspace.layout.widths.v1'
const modelPreferenceKey = 'meta_org.assistant.model_by_module.v1'
const legacyMenuStorageKey = 'harness.menu.groups.v1'
const legacyExpandedMenuStorageKey = 'harness.menu.expanded.v1'
const projectGithubURL = 'https://github.com/selfevo-AI/meta-org'

const defaultWorkspaceLayoutWidths: WorkspaceLayoutWidths = {
  menu: 248,
  business: 300,
  status: 340,
}

const workspaceLayoutLimits: Record<WorkspaceLayoutPane, { min: number; max: number }> = {
  menu: { min: 220, max: 320 },
  business: { min: 240, max: 420 },
  status: { min: 280, max: 480 },
}

const defaultMenuGroups: MenuGroup[] = [
  {
    id: 'business',
    label: '业务闭环',
    domains: ['Requirement', 'Project', 'Delivery', 'Cost', 'Feedback'],
  },
  {
    id: 'baseData',
    label: '基础数据',
    domains: [
      'MetaResource',
      'Organization',
      'Workflow',
      'Capability',
      'Governance',
      'Evolution',
      'Verification',
      'MetaOrg',
      'Dashboard',
      'SystemAdmin',
      'DeveloperTools',
      'Identity',
      'Layer',
      'Observability',
    ],
  },
  {
    id: 'supplyChain',
    label: 'nav.group.supplyChain',
    domains: ['Procurement', 'Sales', 'Inventory'],
  },
  {
    id: 'finance',
    label: '财务',
    domains: ['FinanceAccounting', 'FinanceReceivables', 'FinancePayables', 'FinanceCostAccounting'],
  },
]

const numberFormatter = new Intl.NumberFormat('zh-CN')
const compactFormatter = new Intl.NumberFormat('zh-CN', { notation: 'compact' })
const percentFormatter = new Intl.NumberFormat('zh-CN', {
  maximumFractionDigits: 1,
  minimumFractionDigits: 0,
})

function formatNumber(value: number): string {
  return numberFormatter.format(value)
}

function formatCompact(value: number): string {
  return compactFormatter.format(value)
}

function formatPercent(value: number): string {
  return `${percentFormatter.format(value * 100)}%`
}

function formatMoney(value: number, currency = 'CNY'): string {
  return `${currency} ${Number(value || 0).toFixed(2)}`
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

function clampLayoutWidth(pane: WorkspaceLayoutPane, value: number): number {
  const limit = workspaceLayoutLimits[pane]
  return Math.min(Math.max(Math.round(value), limit.min), limit.max)
}

function normalizeWorkspaceLayoutWidths(value?: Record<string, unknown>): WorkspaceLayoutWidths {
  return (Object.keys(defaultWorkspaceLayoutWidths) as WorkspaceLayoutPane[]).reduce(
    (result, pane) => {
      const next = value?.[pane]
      result[pane] = clampLayoutWidth(pane, typeof next === 'number' && Number.isFinite(next) ? next : defaultWorkspaceLayoutWidths[pane])
      return result
    },
    { ...defaultWorkspaceLayoutWidths },
  )
}

function workspaceLayoutStyle(widths: WorkspaceLayoutWidths): CSSProperties {
  return {
    '--workspace-grid-lg': `${widths.menu}px 8px ${widths.business}px 8px minmax(520px, 1fr)`,
    '--workspace-grid-xl': `${widths.menu}px 8px ${widths.business}px 8px minmax(520px, 1fr) 8px ${widths.status}px`,
    '--workspace-overview-grid-lg': `${widths.menu}px 8px minmax(520px, 1fr)`,
    '--workspace-overview-grid-xl': `${widths.menu}px 8px minmax(520px, 1fr) 8px ${widths.status}px`,
  } as CSSProperties
}

function loadModelPreferences(): Record<string, string> {
  if (typeof window === 'undefined') return {}
  try {
    return JSON.parse(window.localStorage.getItem(modelPreferenceKey) || '{}')
  } catch {
    return {}
  }
}

function saveModelPreference(moduleKey: string, modelID: string) {
  if (typeof window === 'undefined' || !moduleKey || !modelID) return
  const preferences = loadModelPreferences()
  preferences[moduleKey] = modelID
  window.localStorage.setItem(modelPreferenceKey, JSON.stringify(preferences))
}

function deferStateUpdate(callback: () => void): () => void {
  if (typeof window === 'undefined') {
    callback()
    return () => undefined
  }
  const timeout = window.setTimeout(callback, 0)
  return () => window.clearTimeout(timeout)
}

function asRecord(value: unknown): BusinessRecord {
  return value && typeof value === 'object' ? (value as BusinessRecord) : {}
}

function asRecords(value: unknown): BusinessRecord[] {
  return Array.isArray(value) ? value.map(asRecord) : []
}

function textValue(record: BusinessRecord, keys: string[], fallback = ''): string {
  for (const key of keys) {
    const value = record[key]
    if (typeof value === 'string' && value.trim()) return value
    if (typeof value === 'number' && Number.isFinite(value)) return String(value)
  }
  return fallback
}

function numberValue(record: BusinessRecord, keys: string[]): number | null {
  for (const key of keys) {
    const value = record[key]
    if (typeof value === 'number' && Number.isFinite(value)) return value
  }
  return null
}

function arrayCount(record: BusinessRecord, key: string): number {
  const value = record[key]
  return Array.isArray(value) ? value.length : 0
}

function recordBusinessKey(record: BusinessRecord): string {
  return textValue(record, ['master_key', 'id'])
}

function buildOperationNodes(domain: string): BusinessTreeNode[] {
  return apiOperations
    .filter((operation) => operation.domain === domain)
    .map((operation) => ({
      id: `operation:${operation.id}`,
      domain,
      targetType: 'api_operation' as const,
      targetID: operation.id,
      label: operation.title,
      description: `${operation.method} ${operation.path}`,
      status: getOperationProfile(operation).requiresEntityContext ? 'operation.contextual' : 'operation.ready',
      record: { ...operation },
    }))
}

function buildRecordNodes(
  domain: string,
  records: BusinessRecord[],
  targetType: BusinessTargetType,
  options: {
    labelKeys: string[]
    descriptionKeys?: string[]
    statusKeys?: string[]
    idPrefix?: string
  },
): BusinessTreeNode[] {
  return records.map((record) => {
    const targetID = recordBusinessKey(record)
    return {
      id: `${options.idPrefix ?? targetType}:${targetID}`,
      domain,
      targetType,
      targetID,
      label: textValue(record, options.labelKeys, targetID),
      description: textValue(record, options.descriptionKeys ?? ['description', 'code', 'source_type', 'source_id'], targetID),
      status: textValue(record, options.statusKeys ?? ['status']),
      record,
    }
  })
}

function buildDepartmentNodes(domain: string, departments: DepartmentTreeRecord[]): BusinessTreeNode[] {
  return departments.map((department) => {
    const record = asRecord(department)
    const positionNodes = (department.positions ?? []).map((position) => ({
      id: `position:${position.id}`,
      domain,
      targetType: 'position' as const,
      targetID: position.id,
      label: position.name,
      description: position.code || position.permission_level || position.id,
      status: position.status,
      record: asRecord(position),
    }))
    return {
      id: `department:${department.id}`,
      domain,
      targetType: 'department' as const,
      targetID: department.id,
      label: department.name,
      description: department.code || department.description || department.id,
      status: department.status,
      record,
      children: [...buildDepartmentNodes(domain, department.children ?? []), ...positionNodes],
    }
  })
}

function buildOrganizationNode(organization: OrganizationTreeRecord, departments: DepartmentTreeRecord[]): BusinessTreeNode {
  return {
    id: `organization:${organization.id}`,
    domain: 'Organization',
    targetType: 'organization',
    targetID: organization.id,
    label: organization.name,
    description: organization.description || organization.id,
    status: organization.status,
    record: asRecord(organization),
    children: buildDepartmentNodes('Organization', departments),
  }
}

async function loadBusinessTreeNodes(token: string, domain: string): Promise<BusinessTreeNode[]> {
  if (domain === 'Requirement') {
    const data = await apiRequest<unknown>('/requirements?limit=100', { token })
    return buildRecordNodes(domain, asRecords(data), 'requirement', {
      labelKeys: ['title', 'name'],
      descriptionKeys: ['description', 'source', 'id'],
      statusKeys: ['status', 'priority'],
    })
  }

  if (['Project', 'Delivery', 'Cost', 'Feedback'].includes(domain)) {
    const data = await apiRequest<unknown>('/projects?limit=100', { token })
    return buildRecordNodes(domain, asRecords(data), 'project', {
      labelKeys: ['name', 'title'],
      descriptionKeys: ['description', 'requirement_id', 'id'],
      statusKeys: ['status', 'risk_level'],
    })
  }

  if (domain === 'Organization') {
    const organizations = (await apiRequest<OrganizationTreeRecord[]>('/organizations?limit=100', { token })) ?? []
    const departmentTrees = await Promise.all(
      organizations.map((organization) =>
        apiRequest<DepartmentTreeRecord[]>(`/organizations/${organization.id}/departments/tree`, { token }).catch(() => []),
      ),
    )
    return organizations.map((organization, index) => buildOrganizationNode(organization, departmentTrees[index] ?? []))
  }

  if (domain === 'MetaResource') {
    const data = await apiRequest<unknown>('/meta-resources?limit=100', { token })
    return buildRecordNodes(domain, asRecords(data), 'meta_resource', {
      labelKeys: ['name'],
      descriptionKeys: ['resource_type', 'source_type', 'id'],
    })
  }

  if (domain === 'Inventory') {
    const [partners, items, warehouses, balances, movements] = await Promise.all([
      apiRequest<unknown>('/inventory/partners?limit=100', { token }).catch(() => []),
      apiRequest<unknown>('/inventory/items?limit=100', { token }).catch(() => []),
      apiRequest<unknown>('/inventory/warehouses?limit=100', { token }).catch(() => []),
      apiRequest<unknown>('/inventory/balances?limit=100', { token }).catch(() => []),
      apiRequest<unknown>('/inventory/movements?limit=100', { token }).catch(() => []),
    ])
    return [
      {
        id: 'inventory:partners',
        domain,
        targetType: 'business_partner',
        label: 'inventory.partners',
        children: buildRecordNodes(domain, asRecords(partners), 'business_partner', {
          labelKeys: ['name', 'partner_code'],
          descriptionKeys: ['partner_type', 'email', 'id'],
        }),
      },
      {
        id: 'inventory:items',
        domain,
        targetType: 'inventory_item',
        label: 'inventory.items',
        children: buildRecordNodes(domain, asRecords(items), 'inventory_item', {
          labelKeys: ['name', 'item_code'],
          descriptionKeys: ['item_type', 'base_uom', 'id'],
        }),
      },
      {
        id: 'inventory:warehouses',
        domain,
        targetType: 'warehouse',
        label: 'inventory.warehouses',
        children: buildRecordNodes(domain, asRecords(warehouses), 'warehouse', {
          labelKeys: ['name', 'warehouse_code'],
          descriptionKeys: ['status', 'id'],
        }),
      },
      {
        id: 'inventory:balances',
        domain,
        targetType: 'inventory_balance',
        label: 'inventory.balances',
        children: buildRecordNodes(domain, asRecords(balances), 'inventory_balance', {
          labelKeys: ['master_key', 'item_id', 'id'],
          descriptionKeys: ['warehouse_id', 'currency'],
        }),
      },
      {
        id: 'inventory:movements',
        domain,
        targetType: 'inventory_movement',
        label: 'inventory.movements',
        children: buildRecordNodes(domain, asRecords(movements), 'inventory_movement', {
          labelKeys: ['movement_type', 'master_key', 'id'],
          descriptionKeys: ['source_type', 'item_id', 'warehouse_id'],
        }),
      },
    ]
  }

  if (domain === 'Procurement') {
    const [requisitions, orders, receipts, returns] = await Promise.all([
      apiRequest<unknown>('/procurement/requisitions?limit=100', { token }).catch(() => []),
      apiRequest<unknown>('/procurement/orders?limit=100', { token }).catch(() => []),
      apiRequest<unknown>('/procurement/receipts?limit=100', { token }).catch(() => []),
      apiRequest<unknown>('/procurement/returns?limit=100', { token }).catch(() => []),
    ])
    return [
      {
        id: 'procurement:requisitions',
        domain,
        targetType: 'purchase_requisition',
        label: 'procurement.requisitions',
        children: buildRecordNodes(domain, asRecords(requisitions), 'purchase_requisition', {
          labelKeys: ['title', 'master_key'],
          descriptionKeys: ['supplier_name', 'currency', 'id'],
        }),
      },
      {
        id: 'procurement:orders',
        domain,
        targetType: 'purchase_order',
        label: 'procurement.orders',
        children: buildRecordNodes(domain, asRecords(orders), 'purchase_order', {
          labelKeys: ['order_number', 'master_key'],
          descriptionKeys: ['supplier_name', 'currency', 'id'],
        }),
      },
      {
        id: 'procurement:receipts',
        domain,
        targetType: 'purchase_receipt',
        label: 'procurement.receipts',
        children: buildRecordNodes(domain, asRecords(receipts), 'purchase_receipt', {
          labelKeys: ['receipt_number', 'master_key'],
          descriptionKeys: ['supplier_name', 'payable_id', 'id'],
        }),
      },
      {
        id: 'procurement:returns',
        domain,
        targetType: 'purchase_return',
        label: 'procurement.returns',
        children: buildRecordNodes(domain, asRecords(returns), 'purchase_return', {
          labelKeys: ['return_number', 'master_key'],
          descriptionKeys: ['supplier_name', 'receipt_id', 'id'],
        }),
      },
    ]
  }

  if (domain === 'Sales') {
    const [quotations, orders, shipments, returns] = await Promise.all([
      apiRequest<unknown>('/sales/quotations?limit=100', { token }).catch(() => []),
      apiRequest<unknown>('/sales/orders?limit=100', { token }).catch(() => []),
      apiRequest<unknown>('/sales/shipments?limit=100', { token }).catch(() => []),
      apiRequest<unknown>('/sales/returns?limit=100', { token }).catch(() => []),
    ])
    return [
      {
        id: 'sales:quotations',
        domain,
        targetType: 'sales_quotation',
        label: 'sales.quotations',
        children: buildRecordNodes(domain, asRecords(quotations), 'sales_quotation', {
          labelKeys: ['quotation_number', 'master_key'],
          descriptionKeys: ['customer_name', 'currency', 'id'],
        }),
      },
      {
        id: 'sales:orders',
        domain,
        targetType: 'sales_order',
        label: 'sales.orders',
        children: buildRecordNodes(domain, asRecords(orders), 'sales_order', {
          labelKeys: ['order_number', 'master_key'],
          descriptionKeys: ['customer_name', 'currency', 'id'],
        }),
      },
      {
        id: 'sales:shipments',
        domain,
        targetType: 'sales_shipment',
        label: 'sales.shipments',
        children: buildRecordNodes(domain, asRecords(shipments), 'sales_shipment', {
          labelKeys: ['shipment_number', 'master_key'],
          descriptionKeys: ['customer_name', 'receivable_id', 'id'],
        }),
      },
      {
        id: 'sales:returns',
        domain,
        targetType: 'sales_return',
        label: 'sales.returns',
        children: buildRecordNodes(domain, asRecords(returns), 'sales_return', {
          labelKeys: ['return_number', 'master_key'],
          descriptionKeys: ['customer_name', 'shipment_id', 'id'],
        }),
      },
    ]
  }

  if (domain === 'Finance' || domain === 'FinanceAccounting') {
    const data = await apiRequest<unknown>('/finance/settlement-orders', { token })
    return buildRecordNodes(domain, asRecords(data), 'finance_settlement', {
      labelKeys: ['title', 'settlement_number'],
      descriptionKeys: ['customer_name', 'description', 'id'],
    })
  }

  if (domain === 'FinanceReceivables') {
    const data = await apiRequest<unknown>('/finance/receivables', { token })
    return buildRecordNodes(domain, asRecords(data), 'finance_receivable', {
      labelKeys: ['invoice_number', 'external_receivable_id', 'customer_name'],
      descriptionKeys: ['customer_name', 'account_name', 'source_type'],
    })
  }

  if (domain === 'FinancePayables') {
    const data = await apiRequest<unknown>('/finance/payables', { token })
    return buildRecordNodes(domain, asRecords(data), 'finance_payable', {
      labelKeys: ['invoice_number', 'external_payable_id', 'vendor_name'],
      descriptionKeys: ['vendor_name', 'employee_name', 'account_name'],
    })
  }

  if (domain === 'Costing' || domain === 'FinanceCostAccounting') {
    const [rateCards, budgets, ledgerEntries] = await Promise.all([
      apiRequest<unknown>('/costing/rate-cards', { token }).catch(() => []),
      apiRequest<unknown>('/costing/budgets', { token }).catch(() => []),
      apiRequest<unknown>('/costing/ledger-entries', { token }).catch(() => []),
    ])
    return [
      {
        id: `${domain}:rate-cards`,
        domain,
        targetType: 'cost_rate_card',
        label: 'businessTree.costRateCards',
        children: buildRecordNodes(domain, asRecords(rateCards), 'cost_rate_card', {
          labelKeys: ['subject_id', 'subject_type', 'id'],
          descriptionKeys: ['rate_type', 'scope_type'],
        }),
      },
      {
        id: `${domain}:budgets`,
        domain,
        targetType: 'cost_budget',
        label: 'businessTree.costBudgets',
        children: buildRecordNodes(domain, asRecords(budgets), 'cost_budget', {
          labelKeys: ['scope_id', 'scope_type', 'id'],
          descriptionKeys: ['currency', 'period_start'],
        }),
      },
      {
        id: `${domain}:ledger`,
        domain,
        targetType: 'cost_ledger_entry',
        label: 'businessTree.costLedgerEntries',
        children: buildRecordNodes(domain, asRecords(ledgerEntries), 'cost_ledger_entry', {
          labelKeys: ['cost_category', 'source_type', 'id'],
          descriptionKeys: ['source_id', 'ledger_type'],
        }),
      },
    ]
  }

  if (domain === 'DeveloperTools') {
    const [providers, models] = await Promise.all([
      apiRequest<unknown>('/model-providers', { token }).catch(() => []),
      apiRequest<unknown>('/models', { token }).catch(() => []),
    ])
    const providerNodes = buildRecordNodes(domain, asRecords(providers), 'developer_record', {
      labelKeys: ['name', 'provider'],
      descriptionKeys: ['provider_type', 'base_url', 'id'],
      idPrefix: 'model-provider',
    })
    const modelNodes = buildRecordNodes(domain, asRecords(models), 'developer_record', {
      labelKeys: ['model', 'name', 'model_name'],
      descriptionKeys: ['provider', 'model_type', 'id'],
      idPrefix: 'model',
    })
    return providerNodes.length + modelNodes.length > 0 ? [...providerNodes, ...modelNodes] : buildOperationNodes(domain)
  }

  return buildOperationNodes(domain)
}

function normalizeMenuGroups(input?: MenuGroup[]): MenuGroup[] {
  const allMenuDomains = [...operationDomains, ...virtualDomains]
  const knownDomains = new Set(allMenuDomains)
  const defaultByID = new Map(defaultMenuGroups.map((group) => [group.id, group]))
  const defaultTargetByDomain = new Map(
    defaultMenuGroups.flatMap((group) => group.domains.map((domain) => [domain, group.id] as const)),
  )
  const sourceGroups = Array.isArray(input) && input.length > 0 ? input : defaultMenuGroups
  const nextGroups = defaultMenuGroups.map((group) => ({
    ...group,
    label: defaultByID.get(group.id)?.label ?? group.label,
    domains: [] as string[],
  }))
  const groupByID = new Map(nextGroups.map((group) => [group.id, group]))
  const assigned = new Set<string>()

  sourceGroups.forEach((sourceGroup) => {
    const target = groupByID.get(sourceGroup.id)
    if (!target) return
    sourceGroup.domains.forEach((domain) => {
      if (!knownDomains.has(domain) || assigned.has(domain)) return
      target.domains.push(domain)
      assigned.add(domain)
    })
  })

  allMenuDomains.forEach((domain) => {
    if (assigned.has(domain)) return
    const targetID = defaultTargetByDomain.get(domain) ?? 'system'
    const target = groupByID.get(targetID) ?? nextGroups[nextGroups.length - 1]
    target.domains.push(domain)
    assigned.add(domain)
  })

  return nextGroups
}

function defaultExpandedGroups(): Record<string, boolean> {
  return Object.fromEntries(defaultMenuGroups.map((group) => [group.id, true]))
}

function loadMenuGroups(): MenuGroup[] {
  if (typeof window === 'undefined') return normalizeMenuGroups()
  try {
    const legacyRaw = window.localStorage.getItem(legacyMenuStorageKey)
    const raw = window.localStorage.getItem(menuStorageKey) || legacyRaw
    if (legacyRaw && raw) {
      window.localStorage.setItem(menuStorageKey, raw)
      window.localStorage.removeItem(legacyMenuStorageKey)
    }
    if (!raw) return normalizeMenuGroups()
    return normalizeMenuGroups(JSON.parse(raw))
  } catch {
    return normalizeMenuGroups()
  }
}

function loadExpandedGroups(): Record<string, boolean> {
  if (typeof window === 'undefined') return defaultExpandedGroups()
  try {
    const legacyRaw = window.localStorage.getItem(legacyExpandedMenuStorageKey)
    const raw = window.localStorage.getItem(expandedMenuStorageKey) || legacyRaw
    if (legacyRaw && raw) {
      window.localStorage.setItem(expandedMenuStorageKey, raw)
      window.localStorage.removeItem(legacyExpandedMenuStorageKey)
    }
    if (!raw) return defaultExpandedGroups()
    return { ...defaultExpandedGroups(), ...JSON.parse(raw) }
  } catch {
    return defaultExpandedGroups()
  }
}

function loadThemeMode(): ThemeMode {
  if (typeof window === 'undefined') return 'dark'
  return window.localStorage.getItem(themeStorageKey) === 'light' ? 'light' : 'dark'
}

function assistantModuleForDomain(domain: string): string {
  const modules: Record<string, string> = {
    Overview: 'meta_org',
    MetaOrg: 'meta_org',
    Dashboard: 'meta_org',
    MetaResource: 'meta_resource',
    Requirement: 'requirement',
    Project: 'project',
    Delivery: 'delivery',
    Cost: 'project_cost',
    Feedback: 'feedback',
    Organization: 'organization',
    Workflow: 'workflow',
    Capability: 'capability',
    Governance: 'governance',
    Evolution: 'self_evolution',
    Verification: 'verification',
    DeveloperTools: 'model_settings',
    Costing: 'costing',
    Finance: 'finance',
    FinanceAccounting: 'finance',
    FinanceReceivables: 'finance',
    FinancePayables: 'finance',
    FinanceCostAccounting: 'costing',
    Inventory: 'inventory',
    Procurement: 'procurement',
    Sales: 'sales',
  }
  return modules[domain] ?? domain.toLowerCase()
}

const globalAssistantModules = [
  { id: 'meta_org', key: 'meta_org', targetType: '', label: 'MetaOrg' },
  { id: 'requirement', key: 'requirement', targetType: 'requirement', label: 'Requirement' },
  { id: 'project', key: 'project', targetType: 'project', label: 'Project' },
  { id: 'delivery', key: 'delivery', targetType: 'deliverable', label: 'Delivery' },
  { id: 'project_cost', key: 'project_cost', targetType: 'project_cost', label: 'Cost' },
  { id: 'feedback', key: 'feedback', targetType: 'project_evaluation', label: 'Feedback' },
  { id: 'workflow', key: 'workflow', targetType: 'workflow_instance', label: 'Workflow' },
  { id: 'finance_accounting', key: 'finance', targetType: 'finance_settlement', label: 'FinanceAccounting' },
  { id: 'finance_receivables', key: 'finance', targetType: 'finance_receivable', label: 'FinanceReceivables' },
  { id: 'finance_payables', key: 'finance', targetType: 'finance_payable', label: 'FinancePayables' },
  { id: 'finance_costing', key: 'costing', targetType: 'cost_ledger_entry', label: 'FinanceCostAccounting' },
  { id: 'inventory', key: 'inventory', targetType: 'inventory_item', label: 'Inventory' },
  { id: 'procurement', key: 'procurement', targetType: 'purchase_order', label: 'Procurement' },
  { id: 'sales', key: 'sales', targetType: 'sales_order', label: 'Sales' },
]

const overviewBusinessFunctions: OverviewBusinessFunction[] = [
  {
    id: 'meta_org',
    domain: 'MetaOrg',
    moduleKey: 'meta_org',
    targetType: '',
    label: 'MetaOrg',
    intentKey: 'overview.business.metaOrgIntent',
    icon: Sparkles,
  },
  {
    id: 'requirement',
    domain: 'Requirement',
    moduleKey: 'requirement',
    targetType: 'requirement',
    label: 'Requirement',
    intentKey: 'overview.business.requirementIntent',
    icon: BriefcaseBusiness,
  },
  {
    id: 'project',
    domain: 'Project',
    moduleKey: 'project',
    targetType: 'project',
    label: 'Project',
    intentKey: 'overview.business.projectIntent',
    icon: FolderKanban,
  },
  {
    id: 'delivery',
    domain: 'Delivery',
    moduleKey: 'delivery',
    targetType: 'deliverable',
    label: 'Delivery',
    intentKey: 'overview.business.deliveryIntent',
    icon: ArrowUp,
  },
  {
    id: 'cost',
    domain: 'Cost',
    moduleKey: 'project_cost',
    targetType: 'project_cost',
    label: 'Cost',
    intentKey: 'overview.business.costIntent',
    icon: CircleDollarSign,
  },
  {
    id: 'feedback',
    domain: 'Feedback',
    moduleKey: 'feedback',
    targetType: 'project_evaluation',
    label: 'Feedback',
    intentKey: 'overview.business.feedbackIntent',
    icon: Activity,
  },
  {
    id: 'organization',
    domain: 'Organization',
    moduleKey: 'organization',
    targetType: 'organization',
    label: 'Organization',
    intentKey: 'overview.business.organizationIntent',
    icon: Users,
  },
  {
    id: 'workflow',
    domain: 'Workflow',
    moduleKey: 'workflow',
    targetType: 'workflow_instance',
    label: 'Workflow',
    intentKey: 'overview.business.workflowIntent',
    icon: Workflow,
  },
  {
    id: 'capability',
    domain: 'Capability',
    moduleKey: 'capability',
    targetType: 'capability',
    label: 'Capability',
    intentKey: 'overview.business.capabilityIntent',
    icon: BrainCircuit,
  },
  {
    id: 'governance',
    domain: 'Governance',
    moduleKey: 'governance',
    targetType: 'governance_policy',
    label: 'Governance',
    intentKey: 'overview.business.governanceIntent',
    icon: ShieldCheck,
  },
  {
    id: 'evolution',
    domain: 'Evolution',
    moduleKey: 'self_evolution',
    targetType: 'evolution_task',
    label: 'Evolution',
    intentKey: 'overview.business.evolutionIntent',
    icon: GitBranch,
  },
  {
    id: 'meta_resource',
    domain: 'MetaResource',
    moduleKey: 'meta_resource',
    targetType: 'meta_resource',
    label: 'MetaResource',
    intentKey: 'overview.business.metaResourceIntent',
    icon: Boxes,
  },
  {
    id: 'finance_accounting',
    domain: 'FinanceAccounting',
    moduleKey: 'finance',
    targetType: 'finance_settlement',
    label: 'FinanceAccounting',
    intentKey: 'overview.business.financeIntent',
    icon: WalletCards,
  },
  {
    id: 'finance_receivables',
    domain: 'FinanceReceivables',
    moduleKey: 'finance',
    targetType: 'finance_receivable',
    label: 'FinanceReceivables',
    intentKey: 'overview.business.receivablesIntent',
    icon: ArrowDown,
  },
  {
    id: 'finance_payables',
    domain: 'FinancePayables',
    moduleKey: 'finance',
    targetType: 'finance_payable',
    label: 'FinancePayables',
    intentKey: 'overview.business.payablesIntent',
    icon: ArrowUp,
  },
  {
    id: 'finance_costing',
    domain: 'FinanceCostAccounting',
    moduleKey: 'costing',
    targetType: 'cost_ledger_entry',
    label: 'FinanceCostAccounting',
    intentKey: 'overview.business.financeCostIntent',
    icon: CircleDollarSign,
  },
]

function agentIntentForOperation(operation: ApiOperation, context: Record<string, string>): string {
  const contextLines = Object.entries(context)
    .filter(([, value]) => value)
    .map(([key, value]) => `${key}: ${value}`)
    .join('\n')
  const profile = getOperationProfile(operation)

  return [
    `请作为当前功能工作台的执行 Agent 处理这个操作：${operation.title}`,
    `操作 ID: ${operation.id}`,
    `业务域: ${operation.domain}`,
    `目标接口语义: ${operation.method} ${operation.path}`,
    `风险级别: ${profile.dangerLevel}`,
    '执行要求: 这是人类在当前工作台主动调用 Agent 的操作。请先读取当前工作记录和历史数据，必要时调用工具执行；不要要求人类手动调用 API。普通写入操作直接执行并记录结果；仅破坏性删除、密钥/模型供应商配置、资金实付或明确高风险动作需要进入审批。',
    contextLines ? `当前已选业务上下文:\n${contextLines}` : '当前没有选中记录，请先根据数据库工作记录识别可操作对象，无法确定时向人类提出审核问题。',
  ].join('\n')
}

export default function Home() {
  const { locale, setLocale, t } = useI18n()
  const [ready, setReady] = useState(false)
  const [mode, setMode] = useState<AuthMode>('login')
  const [loginSurface, setLoginSurface] = useState<LoginSurface>('tenant')
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [token, setToken] = useState<string | null>(null)
  const [userId, setUserId] = useState<string | null>(null)
  const [userType, setUserType] = useState<string | null>(null)
  const [platformRole, setPlatformRole] = useState<string | null>(null)
  const [onboardingRequired, setOnboardingRequired] = useState(false)
  const [organizations, setOrganizations] = useState<SessionOrganization[]>([])
  const [currentOrganizationID, setCurrentOrganizationID] = useState<string | null>(null)
  const [saasModules, setSaaSModules] = useState<SaaSModule[]>([])
  const [onboardingOrganizationName, setOnboardingOrganizationName] = useState('')
  const [onboardingDescription, setOnboardingDescription] = useState('')
  const [onboardingModules, setOnboardingModules] = useState<string[]>([])
  const [overview, setOverview] = useState<MetaOrgOverview | null>(null)
  const [inbox, setInbox] = useState<InboxItem[]>([])
  const [roles, setRoles] = useState<Role[]>([])
  const [loading, setLoading] = useState(false)
  const [overviewLoading, setOverviewLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [workspaceView, setWorkspaceView] = useState<WorkspaceView>('overview')
  const [menuGroups, setMenuGroups] = useState<MenuGroup[]>(() => normalizeMenuGroups())
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>(() => defaultExpandedGroups())
  const [themeMode, setThemeMode] = useState<ThemeMode>('dark')
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [menuReady, setMenuReady] = useState(false)
  const [draggedDomain, setDraggedDomain] = useState<string | null>(null)
  const [assistantOpen, setAssistantOpen] = useState(false)
  const [assistantIntent, setAssistantIntent] = useState('')
  const [assistantIntentKey, setAssistantIntentKey] = useState('')
  const [overviewPrompt, setOverviewPrompt] = useState('')
  const [overviewFunctionID, setOverviewFunctionID] = useState('meta_org')
  const [overviewModels, setOverviewModels] = useState<ModelCatalogItem[]>([])
  const [overviewModelID, setOverviewModelID] = useState('')
  const [overviewSkills, setOverviewSkills] = useState<AssistantBusinessSkill[]>([])
  const [overviewSkillID, setOverviewSkillID] = useState('')
  const [skillImportOpen, setSkillImportOpen] = useState(false)
  const [skillLibrary, setSkillLibrary] = useState<AssistantBusinessSkill[]>([])
  const [skillImportID, setSkillImportID] = useState('')
  const [overviewControlLoading, setOverviewControlLoading] = useState(false)
  const [overviewControlNotice, setOverviewControlNotice] = useState('')
  const [overviewControlError, setOverviewControlError] = useState('')
  const [operationContext, setOperationContext] = useState<Record<string, string>>({})
  const [businessNodesByDomain, setBusinessNodesByDomain] = useState<Record<string, BusinessTreeNode[]>>({})
  const [businessSelection, setBusinessSelection] = useState<BusinessSelection | null>(null)
  const [businessTreeLoading, setBusinessTreeLoading] = useState(false)
  const [businessTreeError, setBusinessTreeError] = useState<string | null>(null)
  const [mobileBusinessOpen, setMobileBusinessOpen] = useState(false)
  const [workspaceLayoutWidths, setWorkspaceLayoutWidths] = useState<WorkspaceLayoutWidths>(defaultWorkspaceLayoutWidths)

  const orderedOverviewFunctions = useMemo(() => {
    const domainOrder = menuGroups.flatMap((group) => group.domains)
    return [...overviewBusinessFunctions].sort((left, right) => {
      if (left.id === 'meta_org') return -1
      if (right.id === 'meta_org') return 1
      const leftIndex = domainOrder.indexOf(left.domain)
      const rightIndex = domainOrder.indexOf(right.domain)
      const normalizedLeft = leftIndex === -1 ? Number.MAX_SAFE_INTEGER : leftIndex
      const normalizedRight = rightIndex === -1 ? Number.MAX_SAFE_INTEGER : rightIndex
      return normalizedLeft - normalizedRight
    })
  }, [menuGroups])
  const selectedOverviewFunction =
    orderedOverviewFunctions.find((item) => item.id === overviewFunctionID) ?? orderedOverviewFunctions[0] ?? overviewBusinessFunctions[0]
  const selectedOverviewSkill = overviewSkills.find((skill) => skill.id === overviewSkillID)
  const selectedOverviewModel = overviewModels.find((model) => model.id === overviewModelID)
  const isPlatformAdminSession = !!platformRole

  useEffect(() => {
    let cancelled = false

    Promise.resolve().then(() => {
      if (cancelled) return
      const existingToken = getToken()
      const sessionUser = getSessionUser()
      const sessionPlatformRole = sessionUser?.platform_role || null
      setToken(existingToken)
      setUserId(sessionUser?.id ?? null)
      setUserType(sessionUser?.type ?? null)
      setPlatformRole(sessionPlatformRole)
      setOnboardingRequired(sessionPlatformRole ? false : !!sessionUser?.onboarding_required)
      setOrganizations(sessionUser?.organizations ?? [])
      if (sessionPlatformRole) {
        setCurrentOrganizationId(null)
        setCurrentOrganizationID(null)
        setWorkspaceView('domain:SystemAdmin')
      } else {
        setCurrentOrganizationID(getCurrentOrganizationId())
      }
      setMenuGroups(loadMenuGroups())
      setExpandedGroups(loadExpandedGroups())
      setThemeMode(loadThemeMode())
      setMenuReady(true)
      setReady(true)
    })

    listRoles()
      .then((data) => {
        if (!cancelled) setRoles(data)
      })
      .catch(() => {
        if (!cancelled) setRoles([])
      })

    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (!menuReady || typeof window === 'undefined') return
    window.localStorage.setItem(menuStorageKey, JSON.stringify(menuGroups))
  }, [menuGroups, menuReady])

  useEffect(() => {
    if (!token) {
      Promise.resolve().then(() => {
        setOnboardingRequired(false)
        setOrganizations([])
        setCurrentOrganizationID(null)
        setPlatformRole(null)
        setSaaSModules([])
      })
      return
    }
    let cancelled = false
    Promise.all([getMe(token).catch(() => null), listSaaSModules(token).catch(() => [])]).then(([profile, modules]) => {
      if (cancelled) return
      setSaaSModules(modules)
      if (modules.length > 0 && onboardingModules.length === 0) {
        setOnboardingModules(modules.filter((item) => item.enabled_default).map((item) => item.module_key))
      }
      if (!profile) return
      const nextPlatformRole = profile.platform_role || null
      setPlatformRole(nextPlatformRole)
      setOnboardingRequired(nextPlatformRole ? false : profile.onboarding_required)
      setOrganizations(profile.organizations ?? [])
      if (nextPlatformRole) {
        setCurrentOrganizationId(null)
        setCurrentOrganizationID(null)
        setWorkspaceView((current) => (current === 'overview' ? 'domain:SystemAdmin' : current))
        return
      }
      const storedOrgID = getCurrentOrganizationId()
      const storedOrgIsValid = !!storedOrgID && profile.organizations?.some((organization) => organization.id === storedOrgID)
      const nextOrgID = storedOrgIsValid ? storedOrgID : profile.default_organization_id || profile.organizations?.[0]?.id || null
      if (nextOrgID) {
        setCurrentOrganizationId(nextOrgID)
        setCurrentOrganizationID(nextOrgID)
      } else {
        setCurrentOrganizationId(null)
        setCurrentOrganizationID(null)
      }
    })
    return () => {
      cancelled = true
    }
  }, [onboardingModules.length, token])

  useEffect(() => {
    if (!orderedOverviewFunctions.some((item) => item.id === overviewFunctionID)) {
      return deferStateUpdate(() => setOverviewFunctionID(orderedOverviewFunctions[0]?.id || 'meta_org'))
    }
  }, [orderedOverviewFunctions, overviewFunctionID])

  useEffect(() => {
    if (!token) {
      return deferStateUpdate(() => {
        setOverviewModels([])
        setOverviewModelID('')
      })
    }
    let cancelled = false
    listModels(token)
      .then((items) => {
        if (cancelled) return
        setOverviewModels(items.filter((model) => model.status === 'active'))
      })
      .catch(() => {
        if (!cancelled) {
          setOverviewModels([])
          setOverviewModelID('')
        }
      })
    return () => {
      cancelled = true
    }
  }, [token])

  useEffect(() => {
    return deferStateUpdate(() => {
      if (overviewModels.length === 0) {
        setOverviewModelID('')
        return
      }
      const preferences = loadModelPreferences()
      const preferred = preferences[selectedOverviewFunction.moduleKey] || overviewModels[0]?.id || ''
      setOverviewModelID(overviewModels.some((model) => model.id === preferred) ? preferred : overviewModels[0]?.id || '')
    })
  }, [overviewModels, selectedOverviewFunction.moduleKey])

  useEffect(() => {
    if (!token) {
      return deferStateUpdate(() => {
        setOverviewSkills([])
        setOverviewSkillID('')
      })
    }
    let cancelled = false
    const cancelDeferred = deferStateUpdate(() => {
      if (cancelled) return
      setOverviewControlLoading(true)
      setOverviewControlError('')
      listAssistantSkills(token, selectedOverviewFunction.moduleKey, selectedOverviewFunction.targetType)
        .then((items) => {
          if (cancelled) return
          setOverviewSkills(items)
          setOverviewSkillID((current) => (items.some((skill) => skill.id === current) ? current : items[0]?.id || ''))
        })
        .catch((err) => {
          if (!cancelled) {
            setOverviewSkills([])
            setOverviewSkillID('')
            setOverviewControlError(err instanceof Error ? err.message : t('assistant.global.loadFailed'))
          }
        })
        .finally(() => {
          if (!cancelled) setOverviewControlLoading(false)
        })
    })
    return () => {
      cancelled = true
      cancelDeferred()
    }
  }, [selectedOverviewFunction.moduleKey, selectedOverviewFunction.targetType, t, token])

  useEffect(() => {
    if (!menuReady || typeof window === 'undefined') return
    window.localStorage.setItem(expandedMenuStorageKey, JSON.stringify(expandedGroups))
  }, [expandedGroups, menuReady])

  useEffect(() => {
    if (!menuReady || typeof window === 'undefined') return
    window.localStorage.setItem(themeStorageKey, themeMode)
  }, [menuReady, themeMode])

  useEffect(() => {
    if (!token) {
      return deferStateUpdate(() => setWorkspaceLayoutWidths(defaultWorkspaceLayoutWidths))
    }
    let cancelled = false

    getUserPreference(token, workspaceLayoutPreferenceKey)
      .then((preference) => {
        if (!cancelled) setWorkspaceLayoutWidths(normalizeWorkspaceLayoutWidths(preference.value))
      })
      .catch(() => {
        if (!cancelled) setWorkspaceLayoutWidths(defaultWorkspaceLayoutWidths)
      })

    return () => {
      cancelled = true
    }
  }, [token])

  useEffect(() => {
    if (!token || onboardingRequired || isPlatformAdminSession) return
    let cancelled = false

    Promise.all([getMetaOrgOverview(token), getMetaOrgInbox(token)])
      .then(([overviewData, inboxData]) => {
        if (!cancelled) {
          setOverview(overviewData)
          setInbox(inboxData)
        }
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : t('加载概览失败'))
      })

    return () => {
      cancelled = true
    }
  }, [isPlatformAdminSession, onboardingRequired, t, token])

  const activeDomain = workspaceView === 'overview' ? 'MetaOrg' : workspaceView.replace('domain:', '')

  useEffect(() => {
    if (!token || workspaceView === 'overview') return
    if (businessNodesByDomain[activeDomain]) return
    let cancelled = false

    const cancelDeferred = deferStateUpdate(() => {
      if (cancelled) return
      setBusinessTreeLoading(true)
      setBusinessTreeError(null)
      loadBusinessTreeNodes(token, activeDomain)
        .then((nodes) => {
          if (!cancelled) {
            setBusinessNodesByDomain((current) => ({ ...current, [activeDomain]: nodes }))
          }
        })
        .catch((err) => {
          if (!cancelled) {
            setBusinessTreeError(err instanceof Error ? err.message : t('businessTree.loadFailed'))
            setBusinessNodesByDomain((current) => ({ ...current, [activeDomain]: buildOperationNodes(activeDomain) }))
          }
        })
        .finally(() => {
          if (!cancelled) setBusinessTreeLoading(false)
        })
    })

    return () => {
      cancelled = true
      cancelDeferred()
    }
  }, [activeDomain, businessNodesByDomain, t, token, workspaceView])

  const healthRatio = useMemo(() => {
    if (!overview) return 0
    const active = overview.health.active_projects
    const total = Math.max(overview.health.active_projects + overview.health.open_requirements, 1)
    return active / total
  }, [overview])

  const handleOperationContextChange = useCallback((context: Record<string, string>) => {
    setOperationContext((current) => ({ ...current, ...context }))
  }, [])

  if (!ready) {
    return (
      <main className="app-dark">
        <div className="flex min-h-screen items-center justify-center">
          <RefreshCw className="h-8 w-8 animate-spin text-[#DF6A24]" />
        </div>
      </main>
    )
  }

  async function loadOverview(activeToken = token) {
    if (!activeToken || isPlatformAdminSession) return
    setOverviewLoading(true)
    setError(null)
    try {
      const [overviewData, inboxData] = await Promise.all([getMetaOrgOverview(activeToken), getMetaOrgInbox(activeToken)])
      setOverview(overviewData)
      setInbox(inboxData)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('加载概览失败'))
    } finally {
      setOverviewLoading(false)
    }
  }

  function handleLoginSurfaceChange(surface: LoginSurface) {
    setLoginSurface(surface)
    setError(null)
    setNotice(null)
    if (surface === 'platform') {
      setMode('login')
    }
  }

  async function handleAuth(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setLoading(true)
    setError(null)
    setNotice(null)

    try {
      const response =
        mode === 'register' ? await registerUser({ name, email, password }) : await login(email, password)
      const nextPlatformRole = response.platform_role || null
      if (loginSurface === 'platform' && !nextPlatformRole) {
        throw new Error(t('auth.platformRoleRequired'))
      }
      if (loginSurface === 'tenant' && nextPlatformRole) {
        throw new Error(t('auth.usePlatformLogin'))
      }
      setSession(response.token, response.user_id, response.user_type, {
        onboarding_required: response.onboarding_required,
        default_organization_id: response.default_organization_id,
        platform_role: response.platform_role,
        organizations: response.organizations,
        enabled_modules: response.enabled_modules,
      })
      setOverview(null)
      setToken(response.token)
      setUserId(response.user_id)
      setUserType(response.user_type)
      setPlatformRole(nextPlatformRole)
      setOnboardingRequired(nextPlatformRole ? false : !!response.onboarding_required)
      setOrganizations(response.organizations ?? [])
      const nextOrgID = nextPlatformRole ? null : response.default_organization_id || response.organizations?.[0]?.id || null
      if (nextPlatformRole) {
        setCurrentOrganizationId(null)
        setCurrentOrganizationID(null)
        setBusinessNodesByDomain({})
        setBusinessSelection(null)
        setWorkspaceView('domain:SystemAdmin')
      } else {
        setCurrentOrganizationID(nextOrgID)
      }
      if (mode === 'register') {
        setNotice(t('auth.accountCreated'))
      }
      setPassword('')
    } catch (err) {
      setError(err instanceof Error ? err.message : t('auth.failed'))
    } finally {
      setLoading(false)
    }
  }

  function toggleOnboardingModule(moduleKey: string) {
    setOnboardingModules((current) =>
      current.includes(moduleKey) ? current.filter((item) => item !== moduleKey) : [...current, moduleKey],
    )
  }

  function saasModuleLabel(item: SaaSModule) {
    const key = `saas.module.${item.module_key}`
    const label = t(key)
    return label === key ? item.display_name : label
  }

  async function handleOnboarding(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!token) return
    setLoading(true)
    setError(null)
    setNotice(null)
    try {
      const result = await completeOnboarding(token, {
        organization_name: onboardingOrganizationName,
        description: onboardingDescription,
        enabled_modules: onboardingModules,
      })
      setSession(token, result.profile.id, 'human', {
        onboarding_required: result.profile.onboarding_required,
        default_organization_id: result.profile.default_organization_id,
        platform_role: result.profile.platform_role,
        organizations: result.profile.organizations,
        enabled_modules: result.profile.enabled_modules,
      })
      const nextOrgID = result.profile.default_organization_id || result.organization.id
      setCurrentOrganizationId(nextOrgID)
      setCurrentOrganizationID(nextOrgID)
      setPlatformRole(result.profile.platform_role || null)
      setOrganizations(result.profile.organizations ?? [])
      setOnboardingRequired(false)
      setBusinessNodesByDomain({})
      setOverview(null)
      setNotice(t('onboarding.created'))
      await loadOverview(token)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('onboarding.failed'))
    } finally {
      setLoading(false)
    }
  }

  function handleSignOut() {
    clearSession()
    setToken(null)
    setUserId(null)
    setUserType(null)
    setPlatformRole(null)
    setOnboardingRequired(false)
    setOrganizations([])
    setCurrentOrganizationID(null)
    setOverview(null)
    setInbox([])
    setBusinessNodesByDomain({})
    setBusinessSelection(null)
    setError(null)
    setWorkspaceView('overview')
  }

  function handleOrganizationChange(organizationID: string) {
    if (isPlatformAdminSession) {
      setCurrentOrganizationId(null)
      setCurrentOrganizationID(null)
      return
    }
    setCurrentOrganizationId(organizationID)
    setCurrentOrganizationID(organizationID)
    setBusinessNodesByDomain({})
    setBusinessSelection(null)
    setOverview(null)
    loadOverview(token)
  }

  function toggleMenuGroup(groupID: string) {
    setExpandedGroups((current) => ({
      ...current,
      [groupID]: !current[groupID],
    }))
  }

  function handleDomainDragStart(event: DragEvent<HTMLButtonElement>, domain: string) {
    setDraggedDomain(domain)
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', domain)
  }

  function handleDomainDrop(event: DragEvent<HTMLElement>, groupID: string) {
    event.preventDefault()
    const domain = event.dataTransfer.getData('text/plain') || draggedDomain
    if (!domain || ![...operationDomains, ...virtualDomains].includes(domain)) return
    setMenuGroups((current) =>
      current.map((group) => {
        const domains = group.domains.filter((item) => item !== domain)
        if (group.id !== groupID) return { ...group, domains }
        return { ...group, domains: [...domains, domain] }
      }),
    )
    setExpandedGroups((current) => ({ ...current, [groupID]: true }))
    setDraggedDomain(null)
  }

  function resetMenuLayout() {
    setMenuGroups(normalizeMenuGroups())
    setExpandedGroups(defaultExpandedGroups())
  }

  function persistWorkspaceLayout(widths: WorkspaceLayoutWidths) {
    if (!token) return
    saveUserPreference(token, workspaceLayoutPreferenceKey, widths).catch(() => undefined)
  }

  function handleResetWorkspaceLayout() {
    setWorkspaceLayoutWidths(defaultWorkspaceLayoutWidths)
    persistWorkspaceLayout(defaultWorkspaceLayoutWidths)
  }

  function handleLayoutResizeStart(event: ReactPointerEvent<HTMLButtonElement>, pane: WorkspaceLayoutPane) {
    event.preventDefault()
    const startX = event.clientX
    const startWidths = workspaceLayoutWidths
    let latestWidths = startWidths
    const previousCursor = document.body.style.cursor
    const previousUserSelect = document.body.style.userSelect

    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'

    const handlePointerMove = (moveEvent: PointerEvent) => {
      const delta = moveEvent.clientX - startX
      const next = { ...startWidths }
      if (pane === 'menu') {
        next.menu = clampLayoutWidth('menu', startWidths.menu + delta)
      } else if (pane === 'business') {
        next.business = clampLayoutWidth('business', startWidths.business + delta)
      } else {
        next.status = clampLayoutWidth('status', startWidths.status - delta)
      }
      latestWidths = next
      setWorkspaceLayoutWidths(next)
    }

    const handlePointerUp = () => {
      document.removeEventListener('pointermove', handlePointerMove)
      document.removeEventListener('pointerup', handlePointerUp)
      document.body.style.cursor = previousCursor
      document.body.style.userSelect = previousUserSelect
      persistWorkspaceLayout(latestWidths)
    }

    document.addEventListener('pointermove', handlePointerMove)
    document.addEventListener('pointerup', handlePointerUp)
  }

  function overviewAssistantIntent(intent: string): string {
    const trimmed = intent.trim()
    if (!trimmed) return ''
    const functionLabel = t(selectedOverviewFunction.label)
    if (!selectedOverviewSkill) {
      return [
        trimmed,
        '',
        `${t('overview.business.currentContext')}: ${functionLabel}`,
        `${t('assistant.global.module')}: ${selectedOverviewFunction.moduleKey}`,
      ].join('\n')
    }
    return [
      selectedOverviewSkill.prompt_template,
      '',
      `${t('overview.business.userRequest')}: ${trimmed}`,
      `${t('overview.business.currentContext')}: ${functionLabel}`,
      `${t('assistant.global.module')}: ${selectedOverviewFunction.moduleKey}`,
      `${t('overview.technique')}: ${selectedOverviewSkill.name}`,
    ].join('\n')
  }

  function openAssistantWithIntent(intent: string) {
    const nextIntent = overviewAssistantIntent(intent)
    if (!nextIntent) return
    setAssistantIntent(nextIntent)
    setAssistantIntentKey(`overview:${selectedOverviewFunction.id}:${Date.now()}`)
    setAssistantOpen(true)
  }

  function handleOverviewFunctionSelect(functionID: string) {
    const nextFunction = orderedOverviewFunctions.find((item) => item.id === functionID)
    if (!nextFunction) return
    setOverviewFunctionID(nextFunction.id)
    setOverviewControlNotice('')
    setOverviewControlError('')
    setAssistantIntent('')
    setAssistantIntentKey('')
  }

  function handleOverviewModelChange(modelID: string) {
    setOverviewModelID(modelID)
    saveModelPreference(selectedOverviewFunction.moduleKey, modelID)
  }

  function openSkillImport() {
    if (!token) return
    setSkillImportOpen(true)
    setOverviewControlError('')
    setOverviewControlNotice('')
    setOverviewControlLoading(true)
    listAssistantSkills(token)
      .then((items) => {
        setSkillLibrary(items)
        setSkillImportID(items[0]?.id || '')
      })
      .catch((err) => {
        setSkillLibrary([])
        setSkillImportID('')
        setOverviewControlError(err instanceof Error ? err.message : t('assistant.global.loadFailed'))
      })
      .finally(() => setOverviewControlLoading(false))
  }

  async function handleImportSkill() {
    if (!token || !skillImportID) return
    const source = skillLibrary.find((skill) => skill.id === skillImportID)
    if (!source) return
    setOverviewControlLoading(true)
    setOverviewControlError('')
    setOverviewControlNotice('')
    try {
      const imported = await createAssistantSkill(token, {
        module_key: selectedOverviewFunction.moduleKey,
        target_type: selectedOverviewFunction.targetType,
        business_function_key: selectedOverviewFunction.id,
        name: source.name,
        description: source.description,
        trigger_intent: source.trigger_intent,
        prompt_template: source.prompt_template,
        tool_allowlist: source.tool_allowlist,
        input_schema: source.input_schema,
        output_schema: source.output_schema,
        skill_components: source.skill_components?.length ? source.skill_components : defaultSkillComponents(source.prompt_template),
        permission_policy: source.permission_policy,
        context_policy: source.context_policy,
        pricing_policy: source.pricing_policy,
        activation_policy: source.activation_policy,
        source_session_id: source.source_session_id,
        metadata: {
          ...(source.metadata || {}),
          imported_from_skill_id: source.id,
          imported_from_module_key: source.module_key,
          imported_from_target_type: source.target_type,
          imported_to_overview_function: selectedOverviewFunction.id,
        },
      })
      setOverviewSkills((current) => [imported, ...current])
      setOverviewSkillID(imported.id)
      setSkillImportOpen(false)
      setOverviewControlNotice(t('overview.skillImported'))
    } catch (err) {
      setOverviewControlError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setOverviewControlLoading(false)
    }
  }

  function openAssistantWithoutIntent() {
    setAssistantIntent('')
    setAssistantIntentKey('')
    setAssistantOpen(true)
  }

  function handleViewChange(view: WorkspaceView) {
    setWorkspaceView(view)
    setBusinessSelection(null)
    setMobileMenuOpen(false)
    setMobileBusinessOpen(false)
  }

  function delegateOperationToAgent(operation: ApiOperation) {
    setAssistantIntent(agentIntentForOperation(operation, operationContext))
    setAssistantIntentKey(`${operation.id}-${Date.now()}`)
    setAssistantOpen(true)
  }

  function handleBusinessSelect(node: BusinessTreeNode) {
    const view = node.domain === 'MetaOrg' ? 'overview' : (`domain:${node.domain}` as WorkspaceView)
    setWorkspaceView(view)
    setBusinessSelection(node)
    setOperationContext((current) => ({
      ...current,
      domain: node.domain,
      target_type: node.targetType,
      target_id: node.targetID ?? '',
      [`${node.targetType}_id`]: node.targetID ?? '',
      operation_id: node.targetType === 'api_operation' ? node.targetID ?? '' : current.operation_id ?? '',
    }))
    setMobileBusinessOpen(false)
  }

  async function handleToolApproval(id: string, decision: 'approve' | 'reject') {
    if (!token) return
    setOverviewLoading(true)
    setError(null)
    try {
      if (decision === 'approve') {
        await approveToolApproval(token, id)
        setNotice(t('agent.approvalApproved'))
      } else {
        await rejectToolApproval(token, id)
        setNotice(t('agent.approvalRejected'))
      }
      await loadOverview(token)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setOverviewLoading(false)
    }
  }

  const activeGroup = menuGroups.find((group) => group.domains.includes(activeDomain))
  const isOverview = workspaceView === 'overview'
  const activeOperationCount =
    isOverview
      ? apiOperations.filter((operation) => operation.domain === 'MetaOrg').length
      : apiOperations.filter((operation) => operation.domain === activeDomain).length
  const activeOperations = apiOperations.filter((operation) => operation.domain === (isOverview ? 'MetaOrg' : activeDomain))
  const activeBusinessSelection = businessSelection?.domain === activeDomain ? businessSelection : null
  const assistantModule = isOverview ? selectedOverviewFunction.moduleKey : assistantModuleForDomain(activeDomain)
  const assistantTargetType = isOverview ? selectedOverviewFunction.targetType : activeBusinessSelection?.targetType
  const assistantTargetID = isOverview ? undefined : activeBusinessSelection?.targetID
  const activeBusinessNodes = isOverview ? buildOperationNodes('MetaOrg') : businessNodesByDomain[activeDomain] ?? []

  return (
    <main className={`app-dark ${themeMode === 'light' ? 'theme-light' : ''}`}>
      {!token ? (
        <div className="mx-auto grid min-h-screen max-w-6xl gap-5 px-4 py-8 sm:px-6 lg:grid-cols-[360px_1fr] lg:items-center lg:px-8">
          <section className="studio-panel rounded-lg p-5">
            <div className="mb-8 flex items-center gap-3">
              <div className="flex h-12 w-12 items-center justify-center rounded-lg border border-[#DF6A24]/25 bg-[#DF6A24]/10">
                <Sparkles className="h-6 w-6 text-[#F6A66A]" />
              </div>
              <div>
                <p className="text-xs font-bold uppercase text-[#F6A66A]">{t('shell.breadcrumbRoot')}</p>
                <h1 className="text-2xl font-semibold text-white">{t('app.title')}</h1>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-2">
              <button
                type="button"
                onClick={() => handleLoginSurfaceChange('tenant')}
                title={t('auth.organizationLogin')}
                className={`inline-flex h-11 items-center justify-center gap-2 rounded-lg border text-sm font-semibold transition ${
                  loginSurface === 'tenant'
                    ? 'border-[#DF6A24] bg-[#DF6A24]/15 text-[#fffaf5]'
                    : 'border-slate-700 bg-slate-950/40 text-slate-300 hover:border-slate-500'
                }`}
              >
                <Users className="h-4 w-4" />
                {t('auth.tenantConsole')}
              </button>
              <button
                type="button"
                onClick={() => handleLoginSurfaceChange('platform')}
                title={t('auth.platformLogin')}
                className={`inline-flex h-11 items-center justify-center gap-2 rounded-lg border text-sm font-semibold transition ${
                  loginSurface === 'platform'
                    ? 'border-[#DF6A24] bg-[#DF6A24]/15 text-[#fffaf5]'
                    : 'border-slate-700 bg-slate-950/40 text-slate-300 hover:border-slate-500'
                }`}
              >
                <ShieldCheck className="h-4 w-4" />
                {t('auth.platformAdmin')}
              </button>
            </div>
            <div className="mt-3 flex rounded-lg bg-slate-100 p-1">
              <button
                type="button"
                onClick={() => setMode('login')}
                className={`h-9 flex-1 rounded-md text-sm font-medium transition ${
                  mode === 'login' ? 'bg-white text-slate-950 shadow-sm' : 'text-slate-500 hover:text-slate-800'
                }`}
              >
                {t('auth.login')}
              </button>
              {loginSurface === 'tenant' && (
                <button
                  type="button"
                  onClick={() => setMode('register')}
                  className={`h-9 flex-1 rounded-md text-sm font-medium transition ${
                    mode === 'register' ? 'bg-white text-slate-950 shadow-sm' : 'text-slate-500 hover:text-slate-800'
                  }`}
                >
                  {t('auth.register')}
                </button>
              )}
            </div>

            <form className="mt-5 space-y-4" onSubmit={handleAuth}>
              {loginSurface === 'tenant' && mode === 'register' && (
                <label className="block">
                  <span className="text-sm font-medium text-slate-700">{t('auth.name')}</span>
                  <input
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    className="mt-1 h-11 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                    autoComplete="name"
                    required
                  />
                </label>
              )}
              <label className="block">
                <span className="text-sm font-medium text-slate-700">{t('auth.email')}</span>
                <input
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  className="mt-1 h-11 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                  autoComplete="email"
                  type="email"
                  required
                />
              </label>
              <label className="block">
                <span className="text-sm font-medium text-slate-700">{t('auth.password')}</span>
                <input
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  className="mt-1 h-11 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                  autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
                  type="password"
                  required
                />
              </label>

              {error && (
                <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                  {error}
                </div>
              )}
              {notice && (
                <div className="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                  {notice}
                </div>
              )}

              <button
                type="submit"
                disabled={loading}
                className="inline-flex h-11 w-full items-center justify-center gap-2 rounded-lg bg-[#AD4714] px-4 text-sm font-semibold text-[#fffaf5] transition hover:bg-[#B84F18] disabled:cursor-not-allowed disabled:opacity-60"
              >
                <KeyRound className="h-4 w-4" />
                {loading ? t('auth.processing') : mode === 'login' ? t('auth.signIn') : t('auth.createAccount')}
              </button>
            </form>
          </section>

          <RoleDirectory roles={roles} />
        </div>
      ) : onboardingRequired ? (
        <div className="mx-auto grid min-h-screen max-w-5xl gap-5 px-4 py-8 sm:px-6 lg:grid-cols-[380px_1fr] lg:items-center lg:px-8">
          <section className="studio-panel rounded-lg p-5">
            <div className="mb-6 flex items-center gap-3">
              <div className="flex h-11 w-11 items-center justify-center rounded-lg border border-[#DF6A24]/25 bg-[#DF6A24]/10">
                <Users className="h-5 w-5 text-[#F6A66A]" />
              </div>
              <div>
                <p className="text-xs font-bold uppercase text-[#F6A66A]">{t('onboarding.kicker')}</p>
                <h1 className="text-2xl font-semibold text-white">{t('onboarding.title')}</h1>
              </div>
            </div>
            <form className="space-y-4" onSubmit={handleOnboarding}>
              <label className="block">
                <span className="text-sm font-medium text-slate-700">{t('onboarding.organizationName')}</span>
                <input
                  value={onboardingOrganizationName}
                  onChange={(event) => setOnboardingOrganizationName(event.target.value)}
                  className="mt-1 h-11 w-full rounded-lg border border-slate-300 px-3 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                  required
                />
              </label>
              <label className="block">
                <span className="text-sm font-medium text-slate-700">{t('onboarding.description')}</span>
                <textarea
                  value={onboardingDescription}
                  onChange={(event) => setOnboardingDescription(event.target.value)}
                  className="mt-1 min-h-24 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                />
              </label>

              {error && (
                <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                  {error}
                </div>
              )}
              {notice && (
                <div className="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                  {notice}
                </div>
              )}

              <div className="flex gap-2">
                <button
                  type="submit"
                  disabled={loading}
                  className="inline-flex h-11 flex-1 items-center justify-center gap-2 rounded-lg bg-[#AD4714] px-4 text-sm font-semibold text-[#fffaf5] transition hover:bg-[#B84F18] disabled:cursor-not-allowed disabled:opacity-60"
                >
                  <CheckCircle2 className="h-4 w-4" />
                  {loading ? t('auth.processing') : t('onboarding.create')}
                </button>
                <button
                  type="button"
                  onClick={handleSignOut}
                  className="inline-flex h-11 items-center justify-center gap-2 rounded-lg border border-slate-300 bg-white px-4 text-sm font-semibold text-slate-700 transition hover:bg-slate-50"
                >
                  <LogOut className="h-4 w-4" />
                  {t('common.signOut')}
                </button>
              </div>
            </form>
          </section>

          <section className="studio-panel rounded-lg p-5">
            <div className="mb-4 flex items-center justify-between gap-3">
              <div>
                <h2 className="text-lg font-semibold text-white">{t('onboarding.modules')}</h2>
                <p className="text-sm text-slate-400">{t('onboarding.modulesSubtitle')}</p>
              </div>
              <Boxes className="h-5 w-5 text-[#F6A66A]" />
            </div>
            <div className="grid gap-2 sm:grid-cols-2">
              {saasModules.map((item) => (
                <label
                  key={item.module_key}
                  className="flex min-h-14 cursor-pointer items-center gap-3 rounded-lg border border-slate-700 bg-slate-900/60 px-3 py-2 text-sm text-slate-100 transition hover:border-[#DF6A24]/60"
                >
                  <input
                    type="checkbox"
                    checked={onboardingModules.includes(item.module_key)}
                    onChange={() => toggleOnboardingModule(item.module_key)}
                    className="h-4 w-4 rounded border-slate-500 text-[#AD4714] focus:ring-[#DF6A24]"
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate font-medium">{saasModuleLabel(item)}</span>
                    <span className="block text-xs text-slate-400">
                      {item.license_scope === 'commercial' ? t('onboarding.commercial') : t('onboarding.mit')}
                    </span>
                  </span>
                </label>
              ))}
            </div>
          </section>
        </div>
      ) : (
        <div className={`workspace-shell grid min-h-screen ${isOverview ? 'workspace-shell-overview' : ''}`} style={workspaceLayoutStyle(workspaceLayoutWidths)}>
          <div
            className={`workspace-sidebar-pane fixed inset-y-0 left-0 z-40 w-[248px] transform transition lg:static lg:w-auto lg:translate-x-0 ${
              mobileMenuOpen ? 'translate-x-0' : '-translate-x-full'
            }`}
          >
            <NavigationSidebar
              workspaceView={workspaceView}
              groups={menuGroups}
              expandedGroups={expandedGroups}
              onViewChange={handleViewChange}
              onToggleGroup={toggleMenuGroup}
              onDragStart={handleDomainDragStart}
              onDropDomain={handleDomainDrop}
              onReset={resetMenuLayout}
            />
          </div>
          {mobileMenuOpen && (
            <button
              type="button"
              aria-label="Close menu"
              className="fixed inset-0 z-30 bg-black/60 lg:hidden"
              onClick={() => setMobileMenuOpen(false)}
            />
          )}

          <WorkspaceLayoutResizer pane="menu" label={t('layout.resizeMenu')} onResizeStart={handleLayoutResizeStart} className="workspace-menu-resizer lg:flex" />

          <div className="workspace-topbar min-w-0">
            <Topbar
              activeTitle={workspaceView === 'overview' ? t('nav.overview') : t(domainLabels[activeDomain] ?? activeDomain)}
              activeDomain={workspaceView === 'overview' ? 'SuperClaw' : activeDomain}
              locale={locale}
              setLocale={setLocale}
              themeMode={themeMode}
              setThemeMode={setThemeMode}
              userType={userType}
              platformRole={platformRole}
              organizations={organizations}
              currentOrganizationID={currentOrganizationID}
              onOrganizationChange={handleOrganizationChange}
              overview={overview}
              overviewLoading={overviewLoading}
              onRefresh={() => loadOverview()}
              onSignOut={handleSignOut}
              onOpenMenu={() => setMobileMenuOpen(true)}
              showBusinessControl={!isOverview}
              onOpenBusiness={() => setMobileBusinessOpen(true)}
              onResetLayout={handleResetWorkspaceLayout}
            />
          </div>

          {!isOverview && (
            <div
              className={`workspace-business-pane fixed inset-y-0 left-0 z-30 w-[300px] transform transition lg:static lg:w-auto lg:translate-x-0 ${
                mobileBusinessOpen ? 'translate-x-0' : '-translate-x-full'
              }`}
            >
              <BusinessTreePanel
                domain={activeDomain}
                nodes={activeBusinessNodes}
                selectedID={activeBusinessSelection?.id}
                loading={businessTreeLoading}
                error={businessTreeError}
                onSelect={handleBusinessSelect}
              />
            </div>
          )}
          {!isOverview && mobileBusinessOpen && (
            <button
              type="button"
              aria-label={t('businessTree.close')}
              className="fixed inset-0 z-20 bg-black/60 lg:hidden"
              onClick={() => setMobileBusinessOpen(false)}
            />
          )}

          {!isOverview && (
            <WorkspaceLayoutResizer pane="business" label={t('layout.resizeBusiness')} onResizeStart={handleLayoutResizeStart} className="workspace-business-resizer lg:flex" />
          )}

          <section className="workspace-main-pane min-w-0">
            <div className="mx-auto max-w-7xl space-y-5 px-4 py-6 sm:px-6 lg:px-8">
              {workspaceView !== 'overview' && (
                <WorkspaceHeader
                  title={activeBusinessSelection?.label ?? (domainLabels[activeDomain] ?? activeDomain)}
                  domain={activeBusinessSelection ? t(`businessTree.type.${activeBusinessSelection.targetType}`) : activeDomain}
                  groupLabel={activeGroup?.label ?? '功能台'}
                  selection={activeBusinessSelection}
                  operationCount={activeOperationCount}
                  operations={activeOperations}
                  dedicated={dedicatedDomains.has(activeDomain)}
                  onOperationSelect={delegateOperationToAgent}
                  onAssistantOpen={() => {
                    setAssistantIntent('')
                    setAssistantIntentKey('')
                    setAssistantOpen(true)
                  }}
                />
              )}
              {error && (
                <div className="mb-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
                  {error}
                </div>
              )}
              {workspaceView === 'overview' ? (
                overview ? (
                  <OverviewAssistantHome
                    overview={overview}
                    inbox={inbox}
                    healthRatio={healthRatio}
                    businessFunctions={orderedOverviewFunctions}
                    selectedFunctionID={selectedOverviewFunction.id}
                    onSelectFunction={handleOverviewFunctionSelect}
                    models={overviewModels}
                    selectedModelID={overviewModelID}
                    selectedModel={selectedOverviewModel}
                    onModelChange={handleOverviewModelChange}
                    skills={overviewSkills}
                    selectedSkillID={overviewSkillID}
                    selectedSkill={selectedOverviewSkill}
                    onSkillChange={setOverviewSkillID}
                    onImportSkill={openSkillImport}
                    controlsLoading={overviewControlLoading}
                    controlNotice={overviewControlNotice}
                    controlError={overviewControlError}
                    prompt={overviewPrompt}
                    onPromptChange={setOverviewPrompt}
                    onSubmitPrompt={() => openAssistantWithIntent(overviewPrompt)}
                    onQuickPrompt={(intent) => {
                      setOverviewPrompt(intent)
                      openAssistantWithIntent(intent)
                    }}
                    onOpenAssistant={openAssistantWithoutIntent}
                    onReviewApproval={(id, decision) => void handleToolApproval(id, decision)}
                  />
                ) : (
                  <div className="flex min-h-[420px] items-center justify-center rounded-lg border border-slate-200 bg-white">
                    <RefreshCw className="h-5 w-5 animate-spin text-slate-500" />
                  </div>
                )
              ) : workspaceView === 'domain:Organization' ? (
                <OrganizationWorkspace token={token} currentUserId={userId} externalSelection={activeBusinessSelection} />
              ) : workspaceView === 'domain:MetaResource' ? (
                <MetaResourceWorkspace token={token} />
              ) : workspaceView === 'domain:Governance' ? (
                <GovernanceWorkspace token={token} currentUserId={userId} />
              ) : workspaceView === 'domain:Evolution' ? (
                <WeightWorkspace token={token} currentUserId={userId} />
              ) : workspaceView === 'domain:Capability' ? (
                <CapabilityEvaluationWorkspace token={token} currentUserId={userId} />
              ) : workspaceView === 'domain:Workflow' ? (
                <div className="space-y-5">
                  <WorkflowDesignerWorkspace token={token} currentUserId={userId} />
                  <WorkflowMatchingWorkspace token={token} currentUserId={userId} />
                </div>
              ) : ['domain:Requirement', 'domain:Project', 'domain:Delivery', 'domain:Cost', 'domain:Feedback'].includes(
                  workspaceView,
                ) ? (
                <ProjectLifecycleWorkspace
                  token={token}
                  currentUserId={userId}
                  mode={workspaceView.replace('domain:', '') as 'Requirement' | 'Project' | 'Delivery' | 'Cost' | 'Feedback'}
                  externalSelection={activeBusinessSelection}
                  onOperationContextChange={handleOperationContextChange}
                />
              ) : workspaceView === 'domain:DeveloperTools' ? (
                <DeveloperToolsWorkspace token={token} />
              ) : workspaceView === 'domain:SystemAdmin' ? (
                <SystemAdminWorkspace token={token} organizations={organizations} currentOrganizationID={currentOrganizationID} />
              ) : workspaceView === 'domain:Costing' || workspaceView === 'domain:FinanceCostAccounting' ? (
                <CostingWorkspace token={token} />
              ) : workspaceView === 'domain:Inventory' ? (
                <InventoryWorkspace token={token} />
              ) : workspaceView === 'domain:Procurement' ? (
                <ProcurementWorkspace token={token} />
              ) : workspaceView === 'domain:Sales' ? (
                <SalesWorkspace token={token} />
              ) : workspaceView === 'domain:Finance' || workspaceView === 'domain:FinanceAccounting' ? (
                <FinanceWorkspace token={token} mode="accounting" />
              ) : workspaceView === 'domain:FinanceReceivables' ? (
                <FinanceWorkspace token={token} mode="receivables" />
              ) : workspaceView === 'domain:FinancePayables' ? (
                <FinanceWorkspace token={token} mode="payables" />
              ) : (
                <AgentOnlyWorkspace domain={workspaceView.replace('domain:', '')} onAssistantOpen={() => setAssistantOpen(true)} />
              )}
              <div className="xl:hidden">
                <BusinessStatusPanel token={token} selection={activeBusinessSelection} operations={activeOperations} />
              </div>
            </div>
          </section>
          <WorkspaceLayoutResizer pane="status" label={t('layout.resizeStatus')} onResizeStart={handleLayoutResizeStart} className="workspace-status-resizer xl:flex" />
          <aside className="workspace-status-pane hidden min-w-0 border-l border-slate-800 bg-[#121317] xl:block">
            <BusinessStatusPanel token={token} selection={activeBusinessSelection} operations={activeOperations} />
          </aside>
          {skillImportOpen && (
            <div className="fixed inset-0 z-50 flex items-center justify-center px-4">
              <button
                type="button"
                className="absolute inset-0 bg-black/60"
                aria-label={t('common.close')}
                onClick={() => setSkillImportOpen(false)}
              />
              <section className="relative w-full max-w-lg rounded-lg border border-slate-700 bg-[#17181d] p-5 shadow-2xl">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <h2 className="text-base font-semibold text-white">{t('overview.importSkill')}</h2>
                    <p className="mt-1 text-sm text-slate-400">
                      {t('overview.importSkillHint')}: {t(selectedOverviewFunction.label)}
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => setSkillImportOpen(false)}
                    className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-slate-700 text-slate-300 transition hover:bg-slate-800 hover:text-white"
                    aria-label={t('common.close')}
                  >
                    <X className="h-4 w-4" />
                  </button>
                </div>
                <div className="mt-4 space-y-3">
                  <select
                    value={skillImportID}
                    onChange={(event) => setSkillImportID(event.target.value)}
                    className="h-11 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 text-sm text-slate-100 outline-none focus:border-[#DF6A24] focus:ring-2 focus:ring-[#DF6A24]/20"
                  >
                    {skillLibrary.length === 0 ? (
                      <option value="">{overviewControlLoading ? t('common.loading') : t('overview.noImportableSkills')}</option>
                    ) : (
                      skillLibrary.map((skill) => (
                        <option key={skill.id} value={skill.id}>
                          {skill.name} · {skill.module_key}
                        </option>
                      ))
                    )}
                  </select>
                  {overviewControlError && <p className="rounded-md bg-red-950/40 px-3 py-2 text-sm text-red-200">{overviewControlError}</p>}
                  <div className="flex justify-end gap-2">
                    <button
                      type="button"
                      onClick={() => setSkillImportOpen(false)}
                      className="inline-flex h-10 items-center rounded-lg border border-slate-700 px-4 text-sm font-semibold text-slate-200 transition hover:bg-slate-800"
                    >
                      {t('common.cancel')}
                    </button>
                    <button
                      type="button"
                      onClick={() => void handleImportSkill()}
                      disabled={!skillImportID || overviewControlLoading}
                      className="inline-flex h-10 items-center rounded-lg bg-[#AD4714] px-4 text-sm font-semibold text-[#fffaf5] transition hover:bg-[#B84F18] disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {overviewControlLoading ? t('auth.processing') : t('overview.importSkill')}
                    </button>
                  </div>
                </div>
              </section>
            </div>
          )}
          {assistantOpen && (
            <div className="fixed inset-0 z-50">
              <button
                type="button"
                className="absolute inset-0 bg-black/55"
                aria-label={t('common.close')}
                onClick={() => {
                  setAssistantOpen(false)
                  setAssistantIntent('')
                  setAssistantIntentKey('')
                }}
              />
              <aside className="absolute right-0 top-0 h-full w-full max-w-xl shadow-2xl">
                <button
                  type="button"
                  onClick={() => {
                    setAssistantOpen(false)
                    setAssistantIntent('')
                    setAssistantIntentKey('')
                  }}
                  className="absolute right-3 top-3 z-10 inline-flex h-9 w-9 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-500 transition hover:bg-slate-100 hover:text-slate-900"
                  aria-label={t('common.close')}
                >
                  <X className="h-4 w-4" />
                </button>
                <AIAssistant
                  token={token}
                  contextType={assistantModule}
                  targetType={assistantTargetType}
                  targetID={assistantTargetID}
                  initialIntent={assistantIntent}
                  initialIntentKey={assistantIntentKey}
                  autoRunInitialIntent={Boolean(assistantIntent)}
                />
              </aside>
            </div>
          )}
        </div>
      )}
    </main>
  )
}

function WorkspaceLayoutResizer({
  pane,
  label,
  onResizeStart,
  className = 'lg:flex',
}: {
  pane: WorkspaceLayoutPane
  label: string
  onResizeStart: (event: ReactPointerEvent<HTMLButtonElement>, pane: WorkspaceLayoutPane) => void
  className?: string
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      onPointerDown={(event) => onResizeStart(event, pane)}
      className={`workspace-layout-resizer hidden h-full items-stretch justify-center ${className}`}
    >
      <span className="my-3 w-px rounded-full bg-slate-700/70 transition" />
    </button>
  )
}

function Topbar({
  activeTitle,
  activeDomain,
  locale,
  setLocale,
  themeMode,
  setThemeMode,
  userType,
  platformRole,
  organizations,
  currentOrganizationID,
  onOrganizationChange,
  overview,
  overviewLoading,
  onRefresh,
  onSignOut,
  onOpenMenu,
  showBusinessControl,
  onOpenBusiness,
  onResetLayout,
}: {
  activeTitle: string
  activeDomain: string
  locale: 'zh' | 'en'
  setLocale: (locale: 'zh' | 'en') => void
  themeMode: ThemeMode
  setThemeMode: (mode: ThemeMode) => void
  userType: string | null
  platformRole: string | null
  organizations: SessionOrganization[]
  currentOrganizationID: string | null
  onOrganizationChange: (organizationID: string) => void
  overview: MetaOrgOverview | null
  overviewLoading: boolean
  onRefresh: () => void
  onSignOut: () => void
  onOpenMenu: () => void
  showBusinessControl: boolean
  onOpenBusiness: () => void
  onResetLayout: () => void
}) {
  const { t } = useI18n()
  const activeWork = overview?.health.active_projects ?? 0
  const unexportedCost = overview ? formatMoney(overview.health.unexported_cost, overview.health.currency) : 'CNY 0.00'

  return (
    <header className="sticky top-0 z-20 border-b border-slate-800/80 bg-[#121317]/88 backdrop-blur-xl">
      <div className="flex h-[60px] items-center justify-between gap-3 px-4 sm:px-6 lg:px-8">
        <div className="flex min-w-0 items-center gap-3">
          <button
            type="button"
            onClick={onOpenMenu}
            className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-slate-700 text-slate-300 lg:hidden"
          >
            <Menu className="h-4 w-4" />
          </button>
          {showBusinessControl && (
            <button
              type="button"
              onClick={onOpenBusiness}
              className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-slate-700 text-slate-300 lg:hidden"
              aria-label={t('businessTree.open')}
            >
              <GitBranch className="h-4 w-4" />
            </button>
          )}
          <div className="min-w-0 text-xs font-semibold">
            <span className="text-[#F6A66A]">{t('shell.breadcrumbRoot')}</span>
            <span className="px-2 text-slate-500">/</span>
            <span className="truncate text-slate-400">{activeTitle}</span>
          </div>
        </div>

        <div className="flex min-w-0 items-center justify-end gap-2">
          <div className="hidden items-center gap-2 text-xs font-semibold text-slate-400 xl:flex">
            <span>{t('shell.liveBounties', { count: activeWork })}</span>
            <span className="text-slate-600">·</span>
            <span>{t('shell.escrow', { amount: unexportedCost })}</span>
            <span className="text-slate-600">·</span>
            <span>{activeDomain}</span>
          </div>
          {!platformRole && organizations.length > 0 && (
            <label className="hidden items-center gap-2 rounded-lg border border-slate-700 bg-slate-950/40 px-2 text-xs font-semibold text-slate-400 md:flex">
              <span>{t('organization.current')}</span>
              <select
                value={currentOrganizationID ?? organizations[0]?.id ?? ''}
                onChange={(event) => onOrganizationChange(event.target.value)}
                className="h-7 max-w-44 rounded-md border border-slate-700 bg-slate-950 px-2 text-xs font-semibold text-slate-100 outline-none"
              >
                {organizations.map((organization) => (
                  <option key={organization.id} value={organization.id}>
                    {organization.name}
                  </option>
                ))}
              </select>
            </label>
          )}
          {platformRole && <StatusPill label={platformRole} tone="green" />}
          {userType && <StatusPill label={userType === 'ai' ? 'AI Agent' : 'Human'} tone="blue" />}
          <button
            type="button"
            onClick={onResetLayout}
            className="hidden h-9 items-center gap-2 rounded-lg border border-slate-700 px-3 text-xs font-bold text-slate-300 transition hover:border-blue-400/60 hover:text-blue-200 lg:inline-flex"
            aria-label={t('layout.resetWidths')}
            title={t('layout.resetWidths')}
          >
            <SlidersHorizontal className="h-3.5 w-3.5" />
            <span className="hidden 2xl:inline">{t('layout.resetWidths')}</span>
          </button>
          {overview && (
            <button
              type="button"
              onClick={onRefresh}
              disabled={overviewLoading}
              className="hidden h-9 items-center gap-2 rounded-lg border border-slate-700 px-3 text-xs font-semibold text-slate-300 transition hover:border-blue-400/60 hover:text-blue-200 disabled:opacity-60 sm:inline-flex"
            >
              <RefreshCw className={`h-3.5 w-3.5 ${overviewLoading ? 'animate-spin' : ''}`} />
              {formatDate(overview.generated_at)}
            </button>
          )}
          <a
            href={projectGithubURL}
            target="_blank"
            rel="noreferrer"
            aria-label={t('shell.githubLink')}
            className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-700 px-3 text-xs font-bold text-slate-300 transition hover:border-blue-400/60 hover:text-blue-200"
          >
            <Github className="h-3.5 w-3.5" />
            <span className="hidden sm:inline">GitHub</span>
          </a>
          <div className="inline-flex h-9 items-center rounded-lg border border-slate-700 bg-slate-950/40 p-1">
            <button
              type="button"
              onClick={() => setLocale('zh')}
              className={`h-7 rounded-md px-2 text-xs font-bold transition ${
                locale === 'zh' ? 'bg-slate-100 text-slate-950' : 'text-slate-400 hover:text-white'
              }`}
            >
              中文
            </button>
            <button
              type="button"
              onClick={() => setLocale('en')}
              className={`h-7 rounded-md px-2 text-xs font-bold transition ${
                locale === 'en' ? 'bg-slate-100 text-slate-950' : 'text-slate-400 hover:text-white'
              }`}
            >
              EN
            </button>
          </div>
          <button
            type="button"
            onClick={() => setThemeMode(themeMode === 'dark' ? 'light' : 'dark')}
            className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-700 px-3 text-xs font-bold uppercase text-slate-300 transition hover:border-blue-400/60 hover:text-blue-200"
          >
            {themeMode === 'dark' ? <Moon className="h-3.5 w-3.5" /> : <Sun className="h-3.5 w-3.5" />}
            <span className="hidden sm:inline">{themeMode === 'dark' ? t('shell.theme.dark') : t('shell.theme.light')}</span>
          </button>
          <button
            type="button"
            onClick={onSignOut}
            className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-slate-700 text-slate-300 transition hover:border-blue-400/60 hover:text-blue-200"
          >
            <LogOut className="h-4 w-4" />
          </button>
        </div>
      </div>
    </header>
  )
}

function NavigationSidebar({
  workspaceView,
  groups,
  expandedGroups,
  onViewChange,
  onToggleGroup,
  onDragStart,
  onDropDomain,
  onReset,
}: {
  workspaceView: WorkspaceView
  groups: MenuGroup[]
  expandedGroups: Record<string, boolean>
  onViewChange: (view: WorkspaceView) => void
  onToggleGroup: (groupID: string) => void
  onDragStart: (event: DragEvent<HTMLButtonElement>, domain: string) => void
  onDropDomain: (event: DragEvent<HTMLElement>, groupID: string) => void
  onReset: () => void
}) {
  const { t } = useI18n()
  return (
    <aside className="studio-sidebar flex h-full min-h-screen flex-col px-3 py-4">
      <div className="flex items-center gap-3 px-2 pb-6">
        <div className="flex h-11 w-11 items-center justify-center rounded-lg border border-[#DF6A24]/25 bg-[#DF6A24]/10">
          <Sparkles className="h-6 w-6 text-[#F6A66A]" />
        </div>
        <div className="min-w-0">
          <p className="text-xl font-extrabold leading-none tracking-normal text-white">META-ORG</p>
          <p className="mt-1 text-[9px] font-bold uppercase tracking-[0.18em] text-[#F6A66A]">AI Operating System</p>
        </div>
      </div>

      <div className="space-y-1">
        <SidebarButton
          active={workspaceView === 'overview'}
          icon={HomeIcon}
          label={t('nav.item.home')}
          onClick={() => onViewChange('overview')}
        />
      </div>

      <div className="mt-6 flex-1 space-y-5 overflow-y-auto pr-1">
        {groups.map((group) => {
          const expanded = expandedGroups[group.id] ?? true
          const groupOperations = group.domains.reduce(
            (sum, domain) => sum + apiOperations.filter((operation) => operation.domain === domain).length,
            0,
          )

          return (
            <div
              key={group.id}
              className="space-y-1"
              onDragOver={(event) => event.preventDefault()}
              onDrop={(event) => onDropDomain(event, group.id)}
            >
              <button
                type="button"
                onClick={() => onToggleGroup(group.id)}
                className="flex h-9 w-full items-center justify-between px-2 text-left text-base font-semibold tracking-normal text-slate-300"
              >
                <span className="inline-flex min-w-0 items-center gap-2">
                  {expanded ? <ChevronDown className="h-4 w-4 shrink-0" /> : <ChevronRight className="h-4 w-4 shrink-0" />}
                  <span className="truncate">{t(`nav.group.${group.id}`)}</span>
                </span>
                <span className="text-xs font-bold text-slate-500">{groupOperations}</span>
              </button>

              {expanded && (
                <div className="space-y-1">
                  {group.domains.map((domain) => {
                    const menuKey = `domain:${domain}` as const
                    const count = apiOperations.filter((operation) => operation.domain === domain).length
                    const Icon = domainIcons[domain] ?? FolderKanban

                    return (
                      <SidebarButton
                        key={domain}
                        active={workspaceView === menuKey}
                        icon={Icon}
                        label={t(domainLabels[domain] ?? domain)}
                        count={count}
                        onClick={() => onViewChange(menuKey)}
                        draggable
                        onDragStart={(event) => onDragStart(event, domain)}
                      />
                    )
                  })}
                  {group.domains.length === 0 && <p className="px-3 py-2 text-sm text-slate-500">{t('nav.empty')}</p>}
                </div>
              )}
            </div>
          )
        })}
      </div>

      <div className="mt-5 border-t border-slate-800 pt-3">
        <button
          type="button"
          onClick={onReset}
          className="inline-flex h-9 w-full items-center justify-center gap-2 rounded-lg border border-slate-800 text-xs font-bold text-slate-400 transition hover:border-blue-400/50 hover:text-blue-200"
        >
          <SlidersHorizontal className="h-3.5 w-3.5" />
          {t('nav.reset')}
        </button>
      </div>
    </aside>
  )
}

function SidebarButton({
  active,
  icon: Icon,
  label,
  badge,
  count,
  onClick,
  draggable,
  onDragStart,
}: {
  active: boolean
  icon: typeof Gauge
  label: string
  badge?: string
  count?: number
  onClick: () => void
  draggable?: boolean
  onDragStart?: (event: DragEvent<HTMLButtonElement>) => void
}) {
  return (
    <button
      type="button"
      draggable={draggable}
      onDragStart={onDragStart}
      onClick={onClick}
      className={`studio-nav-item flex h-10 w-full items-center justify-between gap-2 rounded-lg border border-transparent px-3 text-left text-sm font-semibold transition ${
        active ? 'studio-nav-item-active' : ''
      }`}
    >
      <span className="inline-flex min-w-0 items-center gap-3">
        <Icon className={`h-4 w-4 shrink-0 ${active ? 'text-[#F6A66A]' : 'text-slate-500'}`} />
        <span className="truncate">{label}</span>
      </span>
      {badge ? (
        <span className="rounded-full border border-emerald-400/40 bg-emerald-500/15 px-2 py-0.5 text-[10px] font-bold text-emerald-300">
          {badge}
        </span>
      ) : count !== undefined ? (
        <span className="text-[11px] text-slate-500">{count}</span>
      ) : null}
    </button>
  )
}

function BusinessFunctionTreePanel({
  groups,
  expandedGroups,
  activeDomain,
  onToggleGroup,
  onViewChange,
}: {
  groups: MenuGroup[]
  expandedGroups: Record<string, boolean>
  activeDomain: string
  onToggleGroup: (groupID: string) => void
  onViewChange: (view: WorkspaceView) => void
}) {
  const { t } = useI18n()

  return (
    <aside className="flex h-full min-h-screen flex-col border-r border-slate-800 bg-[#17181d] px-3 py-4 lg:min-h-0">
      <div className="px-2 pb-4">
        <p className="text-xs font-bold uppercase tracking-[0.16em] text-[#F6A66A]">{t('businessTree.functions')}</p>
        <h2 className="mt-1 flex min-w-0 items-center gap-2 text-base font-semibold text-white">
          <GitBranch className="h-4 w-4 shrink-0 text-slate-400" />
          <span className="truncate">{t('businessTree.functionNavigator')}</span>
        </h2>
      </div>
      <div className="flex-1 overflow-y-auto pr-1">
        <div className="space-y-2">
          {groups.map((group) => {
            const expanded = expandedGroups[group.id] ?? true
            return (
              <div key={group.id} className="rounded-lg border border-slate-800 bg-slate-950/25 p-1.5">
                <button
                  type="button"
                  onClick={() => onToggleGroup(group.id)}
                  className="flex h-9 w-full items-center justify-between rounded-md px-2 text-left text-xs font-bold uppercase tracking-[0.08em] text-slate-500 transition hover:bg-slate-900/60 hover:text-slate-200"
                >
                  <span className="inline-flex min-w-0 items-center gap-2">
                    {expanded ? <ChevronDown className="h-3.5 w-3.5 shrink-0" /> : <ChevronRight className="h-3.5 w-3.5 shrink-0" />}
                    <span className="truncate">{t(`nav.group.${group.id}`)}</span>
                  </span>
                  <span>{group.domains.length}</span>
                </button>
                {expanded && (
                  <div className="mt-1 space-y-1">
                    {group.domains.map((domain) => {
                      const Icon = domainIcons[domain] ?? Gauge
                      const active = activeDomain === domain
                      const count = apiOperations.filter((operation) => operation.domain === domain).length
                      return (
                        <button
                          type="button"
                          key={`${group.id}-${domain}`}
                          onClick={() => onViewChange(domain === 'MetaOrg' ? 'overview' : (`domain:${domain}` as WorkspaceView))}
                          className={`flex h-10 w-full items-center justify-between gap-2 rounded-md px-2 text-left text-sm font-semibold transition ${
                            active
                              ? 'border border-[#DF6A24]/35 bg-[#DF6A24]/10 text-white'
                              : 'border border-transparent text-slate-300 hover:border-slate-700 hover:bg-slate-950/40'
                          }`}
                        >
                          <span className="inline-flex min-w-0 items-center gap-2">
                            <Icon className={`h-4 w-4 shrink-0 ${active ? 'text-[#F6A66A]' : 'text-slate-500'}`} />
                            <span className="truncate">{t(domainLabels[domain] ?? domain)}</span>
                          </span>
                          <span className="shrink-0 rounded-md border border-slate-700 px-1.5 py-0.5 text-[10px] font-bold text-slate-500">
                            {count}
                          </span>
                        </button>
                      )
                    })}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      </div>
    </aside>
  )
}

function BusinessTreePanel({
  domain,
  nodes,
  selectedID,
  loading,
  error,
  onSelect,
}: {
  domain: string
  nodes: BusinessTreeNode[]
  selectedID?: string
  loading: boolean
  error: string | null
  onSelect: (node: BusinessTreeNode) => void
}) {
  const { t } = useI18n()
  const Icon = domainIcons[domain] ?? GitBranch

  return (
    <aside className="flex h-full min-h-screen flex-col border-r border-slate-800 bg-[#17181d] px-3 py-4 lg:min-h-0">
      <div className="flex items-center justify-between gap-2 px-2 pb-4">
        <div className="min-w-0">
          <p className="text-xs font-bold uppercase tracking-[0.16em] text-[#F6A66A]">{t('businessTree.title')}</p>
          <h2 className="mt-1 flex min-w-0 items-center gap-2 text-base font-semibold text-white">
            <Icon className="h-4 w-4 shrink-0 text-slate-400" />
            <span className="truncate">{t(domainLabels[domain] ?? domain)}</span>
          </h2>
        </div>
        {loading && <RefreshCw className="h-4 w-4 animate-spin text-slate-500" />}
      </div>
      {error && <div className="mx-2 mb-3 rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-200">{error}</div>}
      <div className="flex-1 overflow-y-auto pr-1">
        {nodes.length > 0 ? (
          <div className="space-y-1">
            {nodes.map((node) => (
              <BusinessTreeNodeButton key={node.id} node={node} selectedID={selectedID} depth={0} onSelect={onSelect} />
            ))}
          </div>
        ) : (
          <div className="mx-2 rounded-lg border border-slate-800 bg-slate-950/30 px-3 py-4 text-sm text-slate-400">
            {loading ? t('businessTree.loading') : t('businessTree.empty')}
          </div>
        )}
      </div>
    </aside>
  )
}

function BusinessTreeNodeButton({
  node,
  selectedID,
  depth,
  onSelect,
}: {
  node: BusinessTreeNode
  selectedID?: string
  depth: number
  onSelect: (node: BusinessTreeNode) => void
}) {
  const { t } = useI18n()
  const [expanded, setExpanded] = useState(depth < 1)
  const children = node.children ?? []
  const active = selectedID === node.id

  return (
    <div>
      <div className="flex items-start gap-1" style={{ paddingLeft: `${Math.min(depth * 14, 42)}px` }}>
        {children.length > 0 ? (
          <button
            type="button"
            onClick={() => setExpanded((current) => !current)}
            className="mt-1 inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-slate-500 hover:bg-slate-800 hover:text-slate-200"
          >
            {expanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
          </button>
        ) : (
          <span className="h-6 w-6 shrink-0" />
        )}
        <button
          type="button"
          onClick={() => onSelect(node)}
          className={`min-w-0 flex-1 rounded-lg border px-2.5 py-2 text-left transition ${
            active
              ? 'border-[#DF6A24]/50 bg-[#DF6A24]/10 text-white'
              : 'border-transparent text-slate-300 hover:border-slate-700 hover:bg-slate-950/35'
          }`}
        >
          <div className="flex min-w-0 items-center justify-between gap-2">
            <span className="truncate text-sm font-semibold">{t(node.label)}</span>
            {node.status && (
              <span className="shrink-0 rounded-md border border-slate-700 px-1.5 py-0.5 text-[10px] font-semibold text-slate-400">
                {t(node.status)}
              </span>
            )}
          </div>
          {node.description && <p className="mt-1 truncate text-xs text-slate-500">{t(node.description)}</p>}
        </button>
      </div>
      {expanded && children.length > 0 && (
        <div className="mt-1 space-y-1">
          {children.map((child) => (
            <BusinessTreeNodeButton key={child.id} node={child} selectedID={selectedID} depth={depth + 1} onSelect={onSelect} />
          ))}
        </div>
      )}
    </div>
  )
}

function WorkspaceHeader({
  title,
  domain,
  groupLabel,
  selection,
  operationCount,
  operations,
  dedicated,
  onOperationSelect,
  onAssistantOpen,
}: {
  title: string
  domain: string
  groupLabel: string
  selection?: BusinessSelection | null
  operationCount: number
  operations: ApiOperation[]
  dedicated: boolean
  onOperationSelect: (operation: ApiOperation) => void
  onAssistantOpen: () => void
}) {
  const { t } = useI18n()
  const [moreOpen, setMoreOpen] = useState(false)
  const prioritizedOperations = [...operations].sort((left, right) => {
    const order = { direct: 0, contextual: 1, agent_assisted: 2, admin: 3 }
    return order[getOperationProfile(left).kind] - order[getOperationProfile(right).kind]
  })
  const primaryOperations = prioritizedOperations.slice(0, 4)
  const overflowOperations = prioritizedOperations.slice(4)
  return (
    <section className="studio-panel rounded-lg p-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <p className="text-xs font-bold uppercase tracking-[0.16em] text-slate-500">{t(groupLabel)}</p>
          <h2 className="mt-1 truncate text-xl font-semibold text-white">{t(title)}</h2>
          {selection?.description && <p className="mt-1 truncate text-sm text-slate-400">{t(selection.description)}</p>}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <span className="studio-badge inline-flex h-8 items-center rounded-md px-2.5 text-xs font-semibold">
            {domain}
          </span>
          {operationCount > 0 && (
            <span className="inline-flex h-8 items-center rounded-md border border-slate-700 bg-slate-950/30 px-2.5 text-xs font-semibold text-slate-300">
              {operationCount} {t('agent.actions')}
            </span>
          )}
          <span
            className={`inline-flex h-8 items-center rounded-md border px-2.5 text-xs font-semibold ${
              dedicated
                ? 'border-emerald-400/40 bg-emerald-500/15 text-emerald-300'
                : 'border-[#DF6A24]/25 bg-[#DF6A24]/10 text-[#F6A66A]'
            }`}
          >
            {dedicated ? t('workspace.workspace') : t('workspace.api')}
          </span>
          {primaryOperations.map((operation) => (
            <OperationButton key={operation.id} operation={operation} onClick={() => onOperationSelect(operation)} />
          ))}
          {overflowOperations.length > 0 && (
            <div className="relative">
              <button
                type="button"
                onClick={() => setMoreOpen((current) => !current)}
                className="inline-flex h-8 items-center gap-1.5 rounded-md border border-slate-700 bg-slate-950/30 px-2.5 text-xs font-semibold text-slate-200 transition hover:border-[#DF6A24]/50 hover:text-[#F6A66A]"
              >
                <MoreHorizontal className="h-3.5 w-3.5" />
                {t('operation.more')}
              </button>
              {moreOpen && (
                <div className="absolute right-0 z-20 mt-2 w-64 rounded-lg border border-slate-700 bg-slate-950 p-2 shadow-xl">
                  {overflowOperations.map((operation) => (
                    <button
                      key={operation.id}
                      type="button"
                      onClick={() => {
                        setMoreOpen(false)
                        onOperationSelect(operation)
                      }}
                      className="flex h-10 w-full items-center justify-between gap-2 rounded-md px-2 text-left text-xs font-semibold text-slate-300 transition hover:bg-[#DF6A24]/10 hover:text-[#F6A66A]"
                    >
                      <span className="min-w-0">
                        <span className="block truncate">{t(operation.title)}</span>
                        <span className="block truncate text-[10px] font-medium text-slate-500">{t(`operation.kind.${getOperationProfile(operation).kind}`)}</span>
                      </span>
                      <span className="text-[10px] text-slate-500">{t('agent.delegate')}</span>
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}
          <button
            type="button"
            onClick={onAssistantOpen}
            className="inline-flex h-8 items-center gap-1.5 rounded-md bg-[#AD4714] px-2.5 text-xs font-bold text-[#fffaf5] transition hover:bg-[#B84F18]"
          >
            <Bot className="h-3.5 w-3.5" />
            {t('assistant.title')}
          </button>
        </div>
      </div>
    </section>
  )
}

function OperationButton({ operation, onClick }: { operation: ApiOperation; onClick: () => void }) {
  const { t } = useI18n()
  const profile = getOperationProfile(operation)
  const toneClass = {
    direct: 'border-blue-400/35 bg-blue-500/10 text-blue-100 hover:border-blue-300/60',
    contextual: 'border-amber-400/35 bg-amber-500/10 text-amber-100 hover:border-amber-300/60',
    agent_assisted: 'border-violet-400/35 bg-violet-500/10 text-violet-100 hover:border-violet-300/60',
    admin: 'border-slate-700 bg-slate-950/30 text-slate-200 hover:border-[#DF6A24]/50 hover:text-[#F6A66A]',
  }[profile.kind]

  return (
    <button
      type="button"
      onClick={onClick}
      className={`inline-flex h-8 max-w-[190px] items-center gap-1.5 rounded-md border px-2.5 text-xs font-semibold transition ${toneClass}`}
      title={`${t('agent.delegate')} · ${t(operation.title)} · ${t(`operation.kind.${profile.kind}`)}`}
    >
      <Bot className="h-3.5 w-3.5 shrink-0" />
      <span className="truncate">{t(operation.title)}</span>
      {profile.requiresEntityContext && <span className="text-[10px] opacity-70">*</span>}
    </button>
  )
}

function BusinessStatusPanel({
  token,
  selection,
  operations,
}: {
  token: string
  selection?: BusinessSelection | null
  operations: ApiOperation[]
}) {
  const { t } = useI18n()
  const [projectOverview, setProjectOverview] = useState<BusinessRecord | null>(null)
  const operation =
    selection?.targetType === 'api_operation' ? apiOperations.find((item) => item.id === selection.targetID) ?? null : null
  const fields = selection ? getBusinessStatusFields(selection, operation, projectOverview) : []
  const relatedOperations = selection
    ? operations.filter((item) => item.domain === selection.domain).slice(0, 5)
    : operations.slice(0, 5)

  useEffect(() => {
    if (selection?.targetType !== 'project' || !selection.targetID) {
      return deferStateUpdate(() => setProjectOverview(null))
    }
    let cancelled = false
    apiRequest<BusinessRecord>(`/projects/${selection.targetID}/overview`, { token })
      .then((data) => {
        if (!cancelled) setProjectOverview(data)
      })
      .catch(() => {
        if (!cancelled) setProjectOverview(null)
      })
    return () => {
      cancelled = true
    }
  }, [selection?.targetID, selection?.targetType, token])

  return (
    <section className="sticky top-[60px] max-h-[calc(100vh-60px)] overflow-y-auto p-4">
      <div className="rounded-lg border border-slate-800 bg-slate-950/35 p-4">
        <p className="text-xs font-bold uppercase tracking-[0.16em] text-[#F6A66A]">{t('businessStatus.title')}</p>
        {selection ? (
          <div className="mt-4 space-y-4">
            <div>
              <h2 className="truncate text-lg font-semibold text-white">{t(selection.label)}</h2>
              <p className="mt-1 text-sm text-slate-400">{t(`businessTree.type.${selection.targetType}`)}</p>
            </div>
            <div className="grid gap-2">
              {fields.map((field) => (
                <div key={field.label} className="rounded-lg border border-slate-800 bg-[#17181d] px-3 py-2">
                  <p className="text-[11px] font-bold uppercase tracking-[0.12em] text-slate-500">{t(field.label)}</p>
                  <p className="mt-1 break-words text-sm font-semibold text-slate-100">{field.value ? t(field.value) : t('common.none')}</p>
                </div>
              ))}
            </div>
          </div>
        ) : (
          <div className="mt-4 rounded-lg border border-slate-800 bg-[#17181d] px-3 py-4 text-sm text-slate-400">
            {t('businessStatus.noSelection')}
          </div>
        )}
      </div>

      <div className="mt-4 rounded-lg border border-slate-800 bg-slate-950/35 p-4">
        <p className="text-xs font-bold uppercase tracking-[0.16em] text-slate-500">{t('businessStatus.actions')}</p>
        <div className="mt-3 space-y-2">
          {relatedOperations.map((item) => {
            const profile = getOperationProfile(item)
            return (
              <div key={item.id} className="rounded-lg border border-slate-800 bg-[#17181d] px-3 py-2">
                <div className="flex items-center justify-between gap-2">
                  <p className="truncate text-sm font-semibold text-slate-100">{t(item.title)}</p>
                  <span className="rounded-md border border-slate-700 px-1.5 py-0.5 text-[10px] font-bold text-slate-400">{item.method}</span>
                </div>
                <p className="mt-1 truncate text-xs text-slate-500">{item.path}</p>
                <p className="mt-1 text-[11px] text-slate-500">
                  {profile.requiresEntityContext ? t('businessStatus.needsContext') : t('businessStatus.ready')}
                </p>
              </div>
            )
          })}
          {relatedOperations.length === 0 && <p className="text-sm text-slate-500">{t('businessStatus.noActions')}</p>}
        </div>
      </div>
    </section>
  )
}

function getBusinessStatusFields(
  selection: BusinessSelection,
  operation: ApiOperation | null,
  projectOverview: BusinessRecord | null,
): Array<{ label: string; value: string }> {
  const record = selection.record ?? {}
  if (operation) {
    return [
      { label: 'businessStatus.method', value: operation.method },
      { label: 'businessStatus.path', value: operation.path },
      { label: 'businessStatus.context', value: getOperationProfile(operation).requiresEntityContext ? 'businessStatus.needsContext' : 'businessStatus.ready' },
      { label: 'businessStatus.operationKind', value: `operation.kind.${getOperationProfile(operation).kind}` },
    ]
  }

  if (selection.targetType === 'requirement') {
    return [
      { label: 'businessStatus.status', value: textValue(record, ['status']) },
      { label: 'businessStatus.priority', value: textValue(record, ['priority']) },
      { label: 'businessStatus.risk', value: textValue(record, ['risk_level']) },
      { label: 'businessStatus.documents', value: String(arrayCount(record, 'documents')) },
      { label: 'businessStatus.workflows', value: String(arrayCount(record, 'analysis_workflows')) },
      { label: 'businessStatus.updated', value: textValue(record, ['updated_at', 'created_at']) },
    ]
  }

  if (selection.targetType === 'project') {
    const budget = numberValue(record, ['budget_amount'])
    const costSummary = asRecord(projectOverview?.cost_summary)
    const actualCost = numberValue(costSummary, ['total_amount', 'base_total_amount', 'actual_amount'])
    return [
      { label: 'businessStatus.lifecycle', value: textValue(record, ['status']) },
      { label: 'businessStatus.priority', value: textValue(record, ['priority']) },
      { label: 'businessStatus.risk', value: textValue(record, ['risk_level']) },
      { label: 'businessStatus.budget', value: budget === null ? '' : formatMoney(budget, textValue(record, ['currency'], 'CNY')) },
      { label: 'businessStatus.members', value: String(arrayCount(projectOverview ?? record, 'members')) },
      { label: 'businessStatus.workflows', value: String(arrayCount(projectOverview ?? record, 'workflows')) },
      { label: 'businessStatus.deliverables', value: String(arrayCount(projectOverview ?? record, 'deliverables')) },
      { label: 'businessStatus.evaluations', value: String(arrayCount(projectOverview ?? record, 'evaluations')) },
      { label: 'businessStatus.actualCost', value: actualCost === null ? '' : formatMoney(actualCost, textValue(costSummary, ['currency', 'base_currency'], 'CNY')) },
      { label: 'businessStatus.updated', value: textValue(record, ['updated_at', 'created_at']) },
    ]
  }

  if (['organization', 'department', 'position'].includes(selection.targetType)) {
    return [
      { label: 'businessStatus.status', value: textValue(record, ['status']) },
      { label: 'businessStatus.code', value: textValue(record, ['code', 'id']) },
      { label: 'businessStatus.members', value: String(arrayCount(record, 'members')) },
      { label: 'businessStatus.positions', value: String(arrayCount(record, 'positions')) },
      { label: 'businessStatus.resourceFit', value: String(arrayCount(record, 'assignments')) },
      { label: 'businessStatus.updated', value: textValue(record, ['updated_at', 'created_at']) },
    ]
  }

  const amount = numberValue(record, ['total_amount', 'amount', 'base_amount'])
  return [
    { label: 'businessStatus.status', value: textValue(record, ['status']) },
    { label: 'businessStatus.type', value: textValue(record, ['resource_type', 'receivable_type', 'payable_type', 'ledger_type', 'subject_type', 'scope_type']) },
    { label: 'businessStatus.owner', value: textValue(record, ['customer_name', 'vendor_name', 'owner_actor_id', 'organization_id', 'project_id']) },
    { label: 'businessStatus.amount', value: amount === null ? '' : formatMoney(amount, textValue(record, ['currency', 'base_currency'], 'CNY')) },
    { label: 'businessStatus.updated', value: textValue(record, ['updated_at', 'created_at', 'invoice_date']) },
  ]
}

function AgentOnlyWorkspace({ domain, onAssistantOpen }: { domain: string; onAssistantOpen: () => void }) {
  const { t } = useI18n()
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p className="text-sm font-semibold text-slate-500">{domain}</p>
          <h2 className="mt-1 text-base font-semibold text-slate-950">{t('agent.consoleTitle')}</h2>
          <p className="mt-2 max-w-2xl text-sm leading-6 text-slate-600">{t('agent.consoleHint')}</p>
        </div>
        <button
          type="button"
          onClick={onAssistantOpen}
          className="inline-flex h-10 items-center justify-center gap-2 rounded-lg bg-[#AD4714] px-3 text-sm font-semibold text-[#fffaf5] transition hover:bg-[#B84F18]"
        >
          <Bot className="h-4 w-4" />
          {t('assistant.title')}
        </button>
      </div>
    </section>
  )
}

function OverviewAssistantHome({
  overview,
  inbox,
  healthRatio,
  businessFunctions,
  selectedFunctionID,
  onSelectFunction,
  models,
  selectedModelID,
  selectedModel,
  onModelChange,
  skills,
  selectedSkillID,
  selectedSkill,
  onSkillChange,
  onImportSkill,
  controlsLoading,
  controlNotice,
  controlError,
  prompt,
  onPromptChange,
  onSubmitPrompt,
  onQuickPrompt,
  onOpenAssistant,
  onReviewApproval,
}: {
  overview: MetaOrgOverview
  inbox: InboxItem[]
  healthRatio: number
  businessFunctions: OverviewBusinessFunction[]
  selectedFunctionID: string
  onSelectFunction: (functionID: string) => void
  models: ModelCatalogItem[]
  selectedModelID: string
  selectedModel?: ModelCatalogItem
  onModelChange: (modelID: string) => void
  skills: AssistantBusinessSkill[]
  selectedSkillID: string
  selectedSkill?: AssistantBusinessSkill
  onSkillChange: (skillID: string) => void
  onImportSkill: () => void
  controlsLoading: boolean
  controlNotice: string
  controlError: string
  prompt: string
  onPromptChange: (value: string) => void
  onSubmitPrompt: () => void
  onQuickPrompt: (intent: string) => void
  onOpenAssistant: () => void
  onReviewApproval: (id: string, decision: 'approve' | 'reject') => void
}) {
  const { t } = useI18n()
  const modes = [
    ['overview.mode.code', 'overview.mode.codeIntent', Code2],
    ['overview.mode.office', 'overview.mode.officeIntent', BriefcaseBusiness],
    ['overview.mode.design', 'overview.mode.designIntent', Sparkles],
  ] as const
  const pendingInbox = inbox.slice(0, 3)
  const selectedBusinessFunction = businessFunctions.find((item) => item.id === selectedFunctionID) ?? businessFunctions[0]

  return (
    <div className="overview-assistant-home flex min-h-[calc(100vh-132px)] flex-col">
      <section className="flex flex-1 flex-col items-center justify-center px-2 py-8 text-center sm:py-12">
        <div className="relative">
          <div className="flex h-28 w-28 items-center justify-center rounded-full border border-slate-800 bg-slate-950/45 shadow-[0_20px_80px_rgba(0,0,0,0.24)]">
            <Bot className="h-14 w-14 text-slate-300" />
          </div>
          <button
            type="button"
            onClick={onOpenAssistant}
            aria-label={t('assistant.title')}
            className="absolute -right-3 top-9 inline-flex h-8 w-8 items-center justify-center rounded-full border border-slate-800 bg-slate-900 text-slate-300 transition hover:border-[#DF6A24]/50 hover:text-[#F6A66A]"
          >
            <Sparkles className="h-3.5 w-3.5" />
          </button>
        </div>

        <h1 className="mt-6 text-balance text-3xl font-extrabold tracking-normal text-white sm:text-4xl">{t('overview.heroTitle')}</h1>
        <p className="mt-3 max-w-2xl text-sm font-medium leading-6 text-slate-400 sm:text-base">{t('overview.heroSubtitle')}</p>

        <div className="mt-6 inline-flex max-w-full flex-wrap items-center justify-center gap-1 rounded-full bg-slate-200/10 p-1">
          {modes.map(([label, intent, Icon], index) => (
            <button
              key={label}
              type="button"
              onClick={() => onQuickPrompt(t(intent))}
              className={`inline-flex h-9 items-center gap-2 rounded-full px-4 text-sm font-bold transition ${
                index === 1 ? 'bg-white text-slate-950 shadow-sm' : 'text-slate-300 hover:bg-slate-900/70 hover:text-white'
              }`}
            >
              <Icon className="h-4 w-4" />
              {t(label)}
            </button>
          ))}
        </div>

        <div className="mt-24 w-full max-w-4xl">
          <div className="mb-3 flex flex-wrap justify-center gap-2">
            {businessFunctions.map((item) => {
              const Icon = item.icon
              const selected = item.id === selectedFunctionID
              return (
              <button
                key={item.id}
                type="button"
                onClick={() => onSelectFunction(item.id)}
                title={t(item.intentKey)}
                className={`inline-flex h-9 items-center gap-2 rounded-full px-3 text-sm font-semibold shadow-sm transition ${
                  selected ? 'bg-white text-slate-950 ring-2 ring-[#DF6A24]/50' : 'bg-slate-100 text-slate-800 hover:bg-white'
                }`}
              >
                <Icon className="h-4 w-4" />
                {t(item.label)}
              </button>
              )
            })}
          </div>
          <form
            onSubmit={(event) => {
              event.preventDefault()
              onSubmitPrompt()
            }}
            className="rounded-[22px] border border-slate-800 bg-[#17181d] p-3 text-left shadow-[0_24px_90px_rgba(0,0,0,0.26)]"
          >
            <textarea
              value={prompt}
              onChange={(event) => onPromptChange(event.target.value)}
              placeholder={t('overview.promptPlaceholder')}
              className="min-h-[86px] w-full resize-none border-0 bg-transparent px-2 py-2 text-sm text-slate-100 outline-none placeholder:text-slate-500"
            />
            <div className="flex flex-wrap items-center justify-between gap-2 border-t border-slate-800 px-1 pt-3">
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <label className="inline-flex h-9 items-center gap-2 rounded-full border border-slate-700 bg-slate-950/45 px-3 text-xs font-semibold text-slate-300">
                  <span>{t('overview.model')}</span>
                  <select
                    value={selectedModelID}
                    onChange={(event) => onModelChange(event.target.value)}
                    className="max-w-[190px] bg-transparent text-sm font-semibold text-slate-100 outline-none"
                    aria-label={t('overview.model')}
                  >
                    {models.length === 0 ? (
                      <option value="">{t('overview.noModels')}</option>
                    ) : (
                      models.map((model) => (
                        <option key={model.id} value={model.id}>
                          {model.display_name || model.model_key}
                        </option>
                      ))
                    )}
                  </select>
                </label>
                <label className="inline-flex h-9 items-center gap-2 rounded-full border border-slate-700 bg-slate-950/45 px-3 text-xs font-semibold text-slate-300">
                  <span>{t('overview.technique')}</span>
                  <select
                    value={selectedSkillID}
                    onChange={(event) => {
                      if (event.target.value === '__import__') {
                        onImportSkill()
                        return
                      }
                      onSkillChange(event.target.value)
                    }}
                    className="max-w-[190px] bg-transparent text-sm font-semibold text-slate-100 outline-none"
                    aria-label={t('overview.technique')}
                  >
                    <option value="">{controlsLoading ? t('common.loading') : t('overview.noSkills')}</option>
                    {skills.map((skill) => (
                      <option key={skill.id} value={skill.id}>
                        {skill.name} · {t(skill.status)}
                      </option>
                    ))}
                    <option value="__import__">{t('overview.importSkill')}</option>
                  </select>
                </label>
                <StatusPill label={formatMoney(overview.health.unexported_cost, overview.health.currency)} tone="amber" />
              </div>
              <button
                type="submit"
                disabled={!prompt.trim()}
                className="inline-flex h-10 w-10 items-center justify-center rounded-full bg-[#AD4714] text-[#fffaf5] transition hover:bg-[#B84F18] disabled:cursor-not-allowed disabled:opacity-45"
                aria-label={t('overview.send')}
              >
                <Send className="h-4 w-4" />
              </button>
            </div>
          </form>
          <div className="mt-2 flex flex-wrap justify-center gap-2 text-xs">
            <StatusPill label={`${t('overview.business.currentContext')}: ${selectedBusinessFunction ? t(selectedBusinessFunction.label) : t('common.none')}`} tone="blue" />
            {selectedModel && <StatusPill label={selectedModel.display_name || selectedModel.model_key} tone="green" />}
            {selectedSkill && <StatusPill label={selectedSkill.name} tone="amber" />}
            {(controlNotice || controlError) && (
              <span className={`inline-flex min-h-7 items-center rounded-full px-3 font-semibold ${controlError ? 'bg-red-100 text-red-700' : 'bg-emerald-100 text-emerald-700'}`}>
                {controlError || controlNotice}
              </span>
            )}
          </div>
        </div>
      </section>

      {pendingInbox.length > 0 && (
        <section className="mx-auto mb-6 w-full max-w-4xl rounded-lg border border-slate-800 bg-slate-950/25 p-3">
          <div className="flex items-center justify-between gap-3 px-1">
            <p className="text-xs font-bold uppercase tracking-[0.14em] text-slate-500">{t('overview.pending')}</p>
            <StatusPill label={formatNumber(inbox.length)} tone="blue" />
          </div>
          <div className="mt-2 divide-y divide-slate-800">
            {pendingInbox.map((item) => (
              <div key={`${item.type}-${item.id}`} className="grid gap-2 py-3 sm:grid-cols-[1fr_auto]">
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold text-slate-100">{item.title}</p>
                  <p className="mt-1 text-xs text-slate-500">
                    {item.type} · {item.source || t('common.none')} · {formatDate(item.created_at)}
                  </p>
                </div>
                <div className="flex flex-wrap justify-end gap-2">
                  <StatusPill label={item.priority} tone={item.priority === 'high' || item.priority === 'critical' ? 'amber' : 'blue'} />
                  <StatusPill label={item.status} tone="green" />
                  {item.type === 'tool_approval' && item.status === 'pending' && (
                    <>
                      <button
                        type="button"
                        onClick={() => onReviewApproval(item.id, 'approve')}
                        className="inline-flex h-7 items-center rounded-md bg-emerald-600 px-2.5 text-xs font-semibold text-white transition hover:bg-emerald-700"
                      >
                        {t('agent.approve')}
                      </button>
                      <button
                        type="button"
                        onClick={() => onReviewApproval(item.id, 'reject')}
                        className="inline-flex h-7 items-center rounded-md border border-red-500/30 bg-red-500/10 px-2.5 text-xs font-semibold text-red-200 transition hover:bg-red-500/20"
                      >
                        {t('agent.reject')}
                      </button>
                    </>
                  )}
                </div>
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  )
}

function Dashboard({
  token,
  overview,
  inbox,
  healthRatio,
  onReviewApproval,
}: {
  token: string
  overview: MetaOrgOverview
  inbox: InboxItem[]
  healthRatio: number
  onReviewApproval: (id: string, decision: 'approve' | 'reject') => void
}) {
  const { t } = useI18n()
  const agentCoverage = overview.agents.total > 0 ? overview.agents.active / overview.agents.total : 0
  const filters = [
    ['shell.filter.all', Sparkles],
    ['shell.filter.business', BriefcaseBusiness],
    ['shell.filter.organization', Users],
    ['shell.filter.governance', ShieldCheck],
    ['shell.filter.aiOps', Bot],
    ['shell.filter.finance', WalletCards],
  ] as const

  return (
    <div className="space-y-5">
        <section className="px-2 py-6 text-center sm:py-10">
          <div className="mx-auto inline-flex items-center gap-2 text-xs font-bold uppercase tracking-[0.16em] text-slate-500">
            <span className="h-2 w-2 rounded-full bg-emerald-400 shadow-[0_0_14px_rgba(52,211,153,0.75)]" />
            {t('nav.badge.live')} - Meta-Org
          </div>
          <h1 className="mx-auto mt-5 max-w-4xl text-balance text-4xl font-extrabold tracking-normal text-white sm:text-5xl">
            {t('shell.commandTitle').replace('?', '')}
            <span className="text-[#DF6A24]">?</span>
          </h1>
          <p className="mx-auto mt-4 max-w-2xl text-base font-medium text-slate-400">{t('shell.commandSubtitle')}</p>

          <div className="mt-8 flex flex-wrap justify-center gap-2">
            {filters.map(([label, Icon], index) => (
              <button
                key={label}
                type="button"
                className={`inline-flex h-9 items-center gap-2 rounded-full border px-4 text-sm font-bold transition ${
                  index === 0
                    ? 'border-[#DF6A24] bg-[#DF6A24]/10 text-[#F6A66A]'
                    : 'border-slate-700 bg-slate-950/30 text-slate-300 hover:border-[#DF6A24]/35'
                }`}
              >
                <Icon className="h-4 w-4" />
                {t(label)}
              </button>
            ))}
          </div>
        </section>

        <GlobalBusinessInteraction token={token} />

        <div className="flex items-center justify-between gap-3 px-1">
          <h2 className="text-sm font-bold uppercase tracking-normal text-slate-400">{t('shell.openWork')}</h2>
          <span className="text-xs font-bold text-slate-500">
            {t('shell.results', { count: overview.activity.length + inbox.length })}
          </span>
        </div>

        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <MetricCard
            icon={Users}
            label={t('meta.openRequirements')}
            value={formatNumber(overview.health.open_requirements)}
            detail={t('meta.pendingWork')}
            tone="blue"
          />
          <MetricCard
            icon={GitBranch}
            label={t('meta.activeProjects')}
            value={formatNumber(overview.health.active_projects)}
            detail={`${formatPercent(healthRatio)} ${t('active')}`}
            tone="emerald"
          />
          <MetricCard
            icon={Bot}
            label={t('meta.activeAgents')}
            value={formatNumber(overview.health.active_agents)}
            detail={`${formatNumber(overview.agents.total)} ${t('meta.totalAgents')}`}
            tone="amber"
          />
          <MetricCard
            icon={ShieldCheck}
            label={t('meta.pendingApprovals')}
            value={formatNumber(overview.health.pending_approvals)}
            detail={formatMoney(overview.health.unexported_cost, overview.health.currency)}
            tone="violet"
          />
        </div>

        <div className="grid gap-4 xl:grid-cols-3">
          <StatusBars
            title={t('meta.projectStatus')}
            icon={Workflow}
            data={overview.projects.by_status}
            labels={{
              planning: 'planning',
              active: 'active',
              paused: 'paused',
              delivering: 'delivering',
              completed: 'completed',
            }}
          />
          <StatusBars
            title={t('meta.agentRisk')}
            icon={BrainCircuit}
            data={overview.agents.by_risk_level}
            labels={{
              low: 'low',
              medium: 'medium',
              high: 'high',
              critical: 'critical',
            }}
          />
          <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
            <div className="flex items-center gap-2">
              <Bot className="h-5 w-5 text-slate-500" />
              <h2 className="text-base font-semibold text-slate-950">{t('meta.agentCoverage')}</h2>
            </div>
            <div className="mt-5">
              <div className="flex items-end justify-between">
                <span className="text-3xl font-semibold text-slate-950">{formatPercent(agentCoverage)}</span>
                <span className="text-sm text-slate-500">
                  {formatNumber(overview.agents.active)} / {formatNumber(overview.agents.total)}
                </span>
              </div>
              <div className="mt-3 h-2 rounded-full bg-slate-100">
                <div
                  className="h-2 rounded-full bg-emerald-500"
                  style={{ width: `${Math.min(agentCoverage * 100, 100)}%` }}
                />
              </div>
            </div>
          </section>
        </div>

        <div className="grid gap-4 xl:grid-cols-[1fr_0.8fr]">
          <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <Gauge className="h-5 w-5 text-slate-500" />
                <h2 className="text-base font-semibold text-slate-950">{t('meta.costPanel')}</h2>
              </div>
              <StatusPill label={overview.cost.currency || 'CNY'} tone="blue" />
            </div>
            <div className="mt-5 grid gap-3 sm:grid-cols-3">
              <SignalStat label={t('meta.todayCost')} value={formatMoney(overview.cost.today, overview.cost.currency)} />
              <SignalStat
                label={t('meta.monthCost')}
                value={formatMoney(overview.cost.month_to_date, overview.cost.currency)}
              />
              <SignalStat
                label={t('meta.unexportedCost')}
                value={formatMoney(overview.cost.unexported, overview.cost.currency)}
              />
            </div>
            <div className="mt-5 grid gap-3 sm:grid-cols-2">
              {Object.entries(overview.cost.by_provider).map(([provider, amount]) => (
                <SignalStat key={provider} label={t(`provider.${provider}`)} value={formatMoney(amount, overview.cost.currency)} />
              ))}
            </div>
          </section>

          <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
            <div className="flex items-center gap-2">
              <ShieldCheck className="h-5 w-5 text-slate-500" />
              <h2 className="text-base font-semibold text-slate-950">{t('meta.riskPanel')}</h2>
            </div>
            <div className="mt-4 divide-y divide-slate-100">
              {overview.risks.length > 0 ? (
                overview.risks.map((risk) => (
                  <div key={`${risk.source}-${risk.id}`} className="grid gap-2 py-3">
                    <div className="flex items-start justify-between gap-3">
                      <p className="text-sm font-semibold text-slate-900">{risk.title}</p>
                      <StatusPill label={risk.severity} tone={risk.severity === 'critical' || risk.severity === 'high' ? 'amber' : 'blue'} />
                    </div>
                    <p className="text-xs text-slate-500">{risk.source}</p>
                  </div>
                ))
              ) : (
                <p className="py-3 text-sm text-slate-500">{t('meta.noRisks')}</p>
              )}
            </div>
          </section>
        </div>

        <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <div className="flex items-center gap-2">
            <Activity className="h-5 w-5 text-slate-500" />
            <h2 className="text-base font-semibold text-slate-950">{t('meta.inbox')}</h2>
          </div>
          <div className="mt-5 divide-y divide-slate-100">
            {inbox.length > 0 ? (
              inbox.map((item) => (
                <div key={`${item.type}-${item.id}`} className="grid gap-2 py-3 sm:grid-cols-[1fr_auto]">
                  <div className="min-w-0">
                    <p className="truncate text-sm font-semibold text-slate-950">{item.title}</p>
                    <p className="mt-1 text-xs text-slate-500">
                      {item.type} · {item.source || t('common.none')} · {formatDate(item.created_at)}
                    </p>
                  </div>
                  <div className="flex flex-wrap justify-end gap-2">
                    <StatusPill label={item.priority} tone={item.priority === 'high' || item.priority === 'critical' ? 'amber' : 'blue'} />
                    <StatusPill label={item.status} tone="green" />
                    {item.type === 'tool_approval' && item.status === 'pending' && (
                      <>
                        <button
                          type="button"
                          onClick={() => onReviewApproval(item.id, 'approve')}
                          className="inline-flex h-7 items-center rounded-md bg-emerald-600 px-2.5 text-xs font-semibold text-white transition hover:bg-emerald-700"
                        >
                          {t('agent.approve')}
                        </button>
                        <button
                          type="button"
                          onClick={() => onReviewApproval(item.id, 'reject')}
                          className="inline-flex h-7 items-center rounded-md border border-red-200 bg-red-50 px-2.5 text-xs font-semibold text-red-700 transition hover:bg-red-100"
                        >
                          {t('agent.reject')}
                        </button>
                      </>
                    )}
                  </div>
                </div>
              ))
            ) : (
              <p className="py-3 text-sm text-slate-500">{t('meta.noInbox')}</p>
            )}
          </div>
        </section>

        <RecentEvents events={overview.activity} />
    </div>
  )
}

function GlobalBusinessInteraction({ token }: { token: string }) {
  const { t } = useI18n()
  const [moduleID, setModuleID] = useState('meta_org')
  const [targets, setTargets] = useState<AssistantContextTarget[]>([])
  const [targetValue, setTargetValue] = useState('')
  const [skills, setSkills] = useState<AssistantBusinessSkill[]>([])
  const [selectedSkillID, setSelectedSkillID] = useState('')
  const [sessionID, setSessionID] = useState('')
  const [proposals, setProposals] = useState<AssistantProposal[]>([])
  const [skillName, setSkillName] = useState('')
  const [skillPrompt, setSkillPrompt] = useState('')
  const [skillIntent, setSkillIntent] = useState('')
  const [skillIntentKey, setSkillIntentKey] = useState('')
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const pendingSkillRunRef = useRef<{
    skillID: string
    moduleKey: string
    targetType: string
    targetID: string
  } | null>(null)
  const activeModule = globalAssistantModules.find((item) => item.id === moduleID) ?? globalAssistantModules[0]
  const moduleKey = activeModule.key
  const moduleTargetType = activeModule.targetType
  const selectedSkill = skills.find((skill) => skill.id === selectedSkillID)

  const selectedTarget = useMemo(() => {
    if (!targetValue) return undefined
    return targets.find((target) => `${target.type}:${target.id}` === targetValue)
  }, [targetValue, targets])
  const activeTargetType = selectedTarget?.type || moduleTargetType
  const assistantContextKey = `${moduleKey}:${activeTargetType}:${selectedTarget?.id || ''}`

  function resetAssistantContext() {
    setSessionID('')
    setProposals([])
    setSkillIntent('')
    setSkillIntentKey('')
    pendingSkillRunRef.current = null
  }

  function handleModuleChange(nextModuleID: string) {
    setModuleID(nextModuleID)
    setTargetValue('')
    resetAssistantContext()
  }

  function handleTargetChange(nextTargetValue: string) {
    setTargetValue(nextTargetValue)
    resetAssistantContext()
  }

  function handleSessionCreated(nextSessionID: string) {
    setSessionID(nextSessionID)
    setProposals([])
    const pending = pendingSkillRunRef.current
    if (!pending) return
    pendingSkillRunRef.current = null
    setBusy(true)
    runAssistantSkill(token, pending.skillID, {
      module_key: pending.moduleKey,
      target_type: pending.targetType,
      target_id: pending.targetID,
      session_id: nextSessionID,
    })
      .then(() => setNotice(t('assistant.global.skillRunCreated')))
      .catch((err) => setError(err instanceof Error ? err.message : t('common.operationFailed')))
      .finally(() => setBusy(false))
  }

  useEffect(() => {
    let cancelled = false
    Promise.all([listAssistantContextTargets(token, moduleKey, moduleTargetType), listAssistantSkills(token, moduleKey, moduleTargetType)])
      .then(([targetItems, skillItems]) => {
        if (cancelled) return
        setTargets(targetItems)
        setSkills(skillItems)
        setSelectedSkillID((current) => (skillItems.some((skill) => skill.id === current) ? current : skillItems[0]?.id || ''))
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : t('assistant.global.loadFailed'))
      })
    return () => {
      cancelled = true
    }
  }, [moduleKey, moduleTargetType, t, token])

  useEffect(() => {
    if (!sessionID) return
    let cancelled = false
    const timers = [0, 2200, 5200, 8200].map((delay) =>
      window.setTimeout(() => {
        listAssistantProposals(token, sessionID)
          .then((items) => {
            if (!cancelled) setProposals(items)
          })
          .catch(() => {
            if (!cancelled) setProposals([])
          })
      }, delay),
    )
    return () => {
      cancelled = true
      timers.forEach((timer) => window.clearTimeout(timer))
    }
  }, [sessionID, token])

  async function refreshProposals() {
    if (!sessionID) return
    setBusy(true)
    setError('')
    try {
      setProposals(await listAssistantProposals(token, sessionID))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('assistant.global.loadFailed'))
    } finally {
      setBusy(false)
    }
  }

  async function handleProposal(id: string, decision: 'confirm' | 'reject') {
    setBusy(true)
    setError('')
    setNotice('')
    try {
      if (decision === 'confirm') {
        await confirmAssistantProposal(token, id)
        setNotice(t('assistant.global.proposalConfirmed'))
      } else {
        await rejectAssistantProposal(token, id, 'rejected from global business interaction')
        setNotice(t('assistant.global.proposalRejected'))
      }
      await refreshProposals()
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setBusy(false)
    }
  }

  async function handleCreateSkill() {
    if (!skillName.trim() || !skillPrompt.trim()) return
    setBusy(true)
    setError('')
    setNotice('')
    try {
      const skill = await createAssistantSkill(token, {
        module_key: moduleKey,
        target_type: activeTargetType,
        business_function_key: activeModule.id,
        name: skillName.trim(),
        prompt_template: skillPrompt.trim(),
        skill_components: defaultSkillComponents(skillPrompt),
        source_session_id: sessionID || undefined,
        metadata: {
          source: 'global_business_interaction',
          target_id: selectedTarget?.id || '',
        },
      })
      setSkills((current) => [skill, ...current])
      setSelectedSkillID(skill.id)
      setSkillName('')
      setSkillPrompt('')
      setNotice(t('assistant.global.skillSaved'))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setBusy(false)
    }
  }

  async function handleActivateSkill(id: string) {
    setBusy(true)
    setError('')
    try {
      const updated = await activateAssistantSkill(token, id)
      setSkills((current) => current.map((skill) => (skill.id === id ? updated : skill)))
      setNotice(t('assistant.global.skillActivated'))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.operationFailed'))
    } finally {
      setBusy(false)
    }
  }

  async function handleRunSkill() {
    if (!selectedSkillID) return
    const skill = selectedSkill
    if (!skill || skill.status !== 'active') return
    setBusy(true)
    setError('')
    setNotice('')
    pendingSkillRunRef.current = {
      skillID: selectedSkillID,
      moduleKey,
      targetType: activeTargetType,
      targetID: selectedTarget?.id || '',
    }
    setSkillIntent(
      [
        skill.prompt_template,
        '',
        `${t('assistant.global.module')}: ${t(activeModule.label)}`,
        selectedTarget
          ? `${t('assistant.global.target')}: ${selectedTarget.type} ${selectedTarget.id} ${selectedTarget.title}`
          : `${t('assistant.global.target')}: ${t('assistant.global.noTarget')}`,
      ].join('\n'),
    )
    setSkillIntentKey(`${skill.id}-${Date.now()}`)
    setBusy(false)
  }

  return (
    <section className="rounded-lg border border-slate-200 bg-white shadow-sm">
      <div className="grid gap-0 xl:grid-cols-[1fr_360px]">
        <div className="min-h-[560px]">
          <AIAssistant
            key={assistantContextKey}
            token={token}
            contextType={moduleKey}
            targetType={activeTargetType}
            targetID={selectedTarget?.id}
            autoModel
            hideModelSelector
            initialIntent={skillIntent}
            initialIntentKey={skillIntentKey}
            autoRunInitialIntent
            className="min-h-[560px] rounded-l-lg"
            onSessionCreated={handleSessionCreated}
          />
        </div>
        <aside className="border-t border-slate-200 p-4 xl:border-l xl:border-t-0">
          <div className="space-y-3">
            <div>
              <label className="text-xs font-semibold text-slate-500">{t('assistant.global.module')}</label>
              <select
                value={moduleID}
                onChange={(event) => handleModuleChange(event.target.value)}
                className="mt-1 h-10 w-full rounded-lg border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20"
              >
                {globalAssistantModules.map((item) => (
                  <option key={item.id} value={item.id}>
                    {t(item.label)}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="text-xs font-semibold text-slate-500">{t('assistant.global.target')}</label>
              <select
                value={targetValue}
                onChange={(event) => handleTargetChange(event.target.value)}
                className="mt-1 h-10 w-full rounded-lg border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20"
              >
                <option value="">{t('assistant.global.noTarget')}</option>
                {targets.map((target) => (
                  <option key={`${target.type}:${target.id}`} value={`${target.type}:${target.id}`}>
                    {target.type} · {target.title || target.id}
                  </option>
                ))}
              </select>
            </div>

            <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
              <div className="flex items-center justify-between gap-2">
                <h3 className="text-sm font-semibold text-slate-950">{t('assistant.global.proposals')}</h3>
                <button
                  type="button"
                  onClick={() => void refreshProposals()}
                  disabled={!sessionID || busy}
                  className="inline-flex h-8 items-center rounded-md border border-slate-300 px-2 text-xs font-semibold text-slate-700 transition hover:bg-white disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {t('common.refresh')}
                </button>
              </div>
              <div className="mt-3 space-y-2">
                {proposals.length > 0 ? (
                  proposals.map((proposal) => (
                    <div key={proposal.id} className="rounded-md border border-slate-200 bg-white p-3">
                      <div className="flex items-start justify-between gap-2">
                        <p className="line-clamp-2 text-sm font-semibold text-slate-900">{proposal.title || proposal.summary}</p>
                        <StatusPill label={proposal.status} tone={proposal.status === 'applied' ? 'green' : 'blue'} />
                      </div>
                      <p className="mt-2 line-clamp-3 text-xs text-slate-500">{proposal.summary}</p>
                      {proposal.status === 'pending' && (
                        <div className="mt-3 flex flex-wrap gap-2">
                          <button
                            type="button"
                            onClick={() => void handleProposal(proposal.id, 'confirm')}
                            disabled={busy}
                            className="inline-flex h-8 items-center rounded-md bg-emerald-600 px-2.5 text-xs font-semibold text-white transition hover:bg-emerald-700 disabled:opacity-50"
                          >
                            {t('assistant.global.confirmWriteback')}
                          </button>
                          <button
                            type="button"
                            onClick={() => void handleProposal(proposal.id, 'reject')}
                            disabled={busy}
                            className="inline-flex h-8 items-center rounded-md border border-red-200 bg-red-50 px-2.5 text-xs font-semibold text-red-700 transition hover:bg-red-100 disabled:opacity-50"
                          >
                            {t('assistant.global.rejectProposal')}
                          </button>
                        </div>
                      )}
                    </div>
                  ))
                ) : (
                  <p className="text-sm text-slate-500">{t('assistant.global.noProposals')}</p>
                )}
              </div>
            </div>

            <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
              <h3 className="text-sm font-semibold text-slate-950">{t('assistant.global.skills')}</h3>
              <div className="mt-3 space-y-2">
                <select
                  value={selectedSkillID}
                  onChange={(event) => setSelectedSkillID(event.target.value)}
                  className="h-10 w-full rounded-lg border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20"
                >
                  <option value="">{t('assistant.global.noSkill')}</option>
                  {skills.map((skill) => (
                    <option key={skill.id} value={skill.id}>
                      {skill.name} · {t(skill.status)}
                    </option>
                  ))}
                </select>
                <div className="flex flex-wrap gap-2">
                  <button
                    type="button"
                    onClick={() => void handleRunSkill()}
                    disabled={!selectedSkillID || selectedSkill?.status !== 'active' || busy}
                    className="inline-flex h-8 items-center rounded-md bg-[#AD4714] px-2.5 text-xs font-semibold text-[#fffaf5] transition hover:bg-[#B84F18] disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {t('assistant.global.runSkill')}
                  </button>
                  {selectedSkillID && selectedSkill?.status !== 'active' && (
                    <button
                      type="button"
                      onClick={() => void handleActivateSkill(selectedSkillID)}
                      disabled={busy}
                      className="inline-flex h-8 items-center rounded-md border border-slate-300 px-2.5 text-xs font-semibold text-slate-700 transition hover:bg-white disabled:opacity-50"
                    >
                      {t('assistant.global.activateSkill')}
                    </button>
                  )}
                </div>
              </div>

              <div className="mt-4 space-y-2">
                <input
                  value={skillName}
                  onChange={(event) => setSkillName(event.target.value)}
                  placeholder={t('assistant.global.skillName')}
                  className="h-10 w-full rounded-lg border border-slate-300 px-3 text-sm text-slate-900 outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20"
                />
                <textarea
                  value={skillPrompt}
                  onChange={(event) => setSkillPrompt(event.target.value)}
                  placeholder={t('assistant.global.skillPrompt')}
                  className="min-h-[84px] w-full resize-none rounded-lg border border-slate-300 p-3 text-sm text-slate-900 outline-none focus:border-[#AD4714] focus:ring-2 focus:ring-[#DF6A24]/20"
                />
                <button
                  type="button"
                  onClick={() => void handleCreateSkill()}
                  disabled={!skillName.trim() || !skillPrompt.trim() || busy}
                  className="inline-flex h-9 w-full items-center justify-center rounded-md border border-slate-300 bg-white px-3 text-sm font-semibold text-slate-800 transition hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {t('assistant.global.saveSkill')}
                </button>
              </div>
            </div>

            {(notice || error) && (
              <p className={`rounded-md px-3 py-2 text-sm ${error ? 'bg-red-50 text-red-700' : 'bg-emerald-50 text-emerald-700'}`}>
                {error || notice}
              </p>
            )}
          </div>
        </aside>
      </div>
    </section>
  )
}

function RoleDirectory({ roles }: { roles: Role[] }) {
  const { t } = useI18n()
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="flex items-center gap-2">
        <CheckCircle2 className="h-5 w-5 text-slate-500" />
        <h2 className="text-base font-semibold text-slate-950">{t('角色目录')}</h2>
      </div>
      <div className="mt-5 space-y-3">
        {roles.length > 0 ? (
          roles.map((role) => (
            <div key={role.id} className="border-t border-slate-100 py-3 first:border-t-0 first:pt-0">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="text-sm font-semibold text-slate-900">{role.name}</p>
                  {role.description && <p className="mt-1 text-sm text-slate-500">{role.description}</p>}
                </div>
                <StatusPill label={role.role_type} tone="blue" />
              </div>
            </div>
          ))
        ) : (
          <p className="text-sm text-slate-500">{t('角色目录暂不可用')}</p>
        )}
      </div>
    </section>
  )
}

function MetricCard({
  icon: Icon,
  label,
  value,
  detail,
  tone,
}: {
  icon: typeof Users
  label: string
  value: string
  detail: string
  tone: 'blue' | 'emerald' | 'amber' | 'violet'
}) {
  const toneClass = {
    blue: 'bg-blue-50 text-blue-700',
    emerald: 'bg-emerald-50 text-emerald-700',
    amber: 'bg-amber-50 text-amber-700',
    violet: 'bg-violet-50 text-violet-700',
  }[tone]

  return (
    <article className="min-h-[142px] rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className={`flex h-10 w-10 items-center justify-center rounded-lg ${toneClass}`}>
        <Icon className="h-5 w-5" />
      </div>
      <p className="mt-4 text-sm font-medium text-slate-500">{label}</p>
      <div className="mt-1 flex items-end justify-between gap-3">
        <p className="text-3xl font-semibold text-slate-950">{value}</p>
        <p className="pb-1 text-right text-sm text-slate-500">{detail}</p>
      </div>
    </article>
  )
}

function StatusBars({
  title,
  icon: Icon,
  data,
  labels,
}: {
  title: string
  icon: typeof Users
  data: Record<string, number>
  labels: Record<string, string>
}) {
  const { t } = useI18n()
  const entries = Object.entries(labels)
  const total = Math.max(
    entries.reduce((sum, [key]) => sum + (data[key] ?? 0), 0),
    1,
  )

  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="flex items-center gap-2">
        <Icon className="h-5 w-5 text-slate-500" />
        <h2 className="text-base font-semibold text-slate-950">{title}</h2>
      </div>
      <div className="mt-5 space-y-4">
        {entries.map(([key, label]) => {
          const value = data[key] ?? 0
          const width = `${Math.max((value / total) * 100, value > 0 ? 3 : 0)}%`

          return (
            <div key={key}>
              <div className="mb-1 flex items-center justify-between text-sm">
                <span className="font-medium text-slate-700">{t(label)}</span>
                <span className="text-slate-500">{formatNumber(value)}</span>
              </div>
              <div className="h-2 rounded-full bg-slate-100">
                <div className="h-2 rounded-full bg-slate-700" style={{ width }} />
              </div>
            </div>
          )
        })}
      </div>
    </section>
  )
}

function SignalStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="border-l-2 border-slate-200 py-1 pl-3">
      <p className="text-xs font-medium text-slate-500">{label}</p>
      <p className="mt-2 text-xl font-semibold text-slate-950">{value}</p>
    </div>
  )
}

function RecentEvents({ events }: { events: MetaOrgOverview['activity'] }) {
  const { t } = useI18n()
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="flex items-center gap-2">
        <Activity className="h-5 w-5 text-slate-500" />
        <h2 className="text-base font-semibold text-slate-950">{t('近期事件')}</h2>
      </div>
      <div className="mt-5 divide-y divide-slate-100">
        {events.length > 0 ? (
          events.map((event) => (
            <div key={`${event.type}-${event.id}`} className="grid gap-2 py-3 sm:grid-cols-[140px_1fr_auto]">
              <span className="text-sm text-slate-500">{formatDate(event.created_at)}</span>
              <span className="min-w-0 truncate text-sm font-medium text-slate-900">{event.title}</span>
              <StatusPill label={event.status || event.type} tone="blue" />
            </div>
          ))
        ) : (
          <p className="py-3 text-sm text-slate-500">{t('暂无事件')}</p>
        )}
      </div>
    </section>
  )
}

function StatusPill({ label, tone }: { label: string; tone: 'blue' | 'green' | 'amber' }) {
  const { t } = useI18n()
  const toneClass = {
    blue: 'border-blue-200 bg-blue-50 text-blue-700',
    green: 'border-emerald-200 bg-emerald-50 text-emerald-700',
    amber: 'border-amber-200 bg-amber-50 text-amber-700',
  }[tone]

  return (
    <span
      className={`inline-flex h-7 max-w-[180px] items-center truncate rounded-full border px-2.5 text-xs font-semibold ${toneClass}`}
    >
      {t(label)}
    </span>
  )
}
