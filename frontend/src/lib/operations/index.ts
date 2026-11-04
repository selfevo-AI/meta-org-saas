import type { ApiOperation } from './types'
import { erpOperations } from './catalog/erp'
import { projectOperations } from './catalog/project'
import { procurementOperations } from './catalog/procurement'
import { salesOperations } from './catalog/sales'
import { financeReceivablesOperations } from './catalog/financeReceivables'
import { inventoryOperations } from './catalog/inventory'
import { financeAccountingOperations } from './catalog/financeAccounting'
import { dashboardOperations } from './catalog/dashboard'
import { identityOperations } from './catalog/identity'
import { systemAdminOperations } from './catalog/systemAdmin'
import { organizationOperations } from './catalog/organization'
import { layerOperations } from './catalog/layer'
import { capabilityOperations } from './catalog/capability'
import { workflowOperations } from './catalog/workflow'
import { observabilityOperations } from './catalog/observability'
import { verificationOperations } from './catalog/verification'
import { governanceOperations } from './catalog/governance'
import { evolutionOperations } from './catalog/evolution'
import { requirementOperations } from './catalog/requirement'
import { deliveryOperations } from './catalog/delivery'
import { costOperations } from './catalog/cost'
import { feedbackOperations } from './catalog/feedback'
import { metaOrgOperations } from './catalog/metaOrg'
import { metaResourceOperations } from './catalog/metaResource'
import { developerToolsOperations } from './catalog/developerTools'
import { financePayablesOperations } from './catalog/financePayables'

export * from './types'
export * from './profile'

const allApiOperations: ApiOperation[] = [
  ...erpOperations,
  ...projectOperations,
  ...procurementOperations,
  ...salesOperations,
  ...financeReceivablesOperations,
  ...inventoryOperations,
  ...financeAccountingOperations,
  ...dashboardOperations,
  ...identityOperations,
  ...systemAdminOperations,
  ...organizationOperations,
  ...layerOperations,
  ...capabilityOperations,
  ...workflowOperations,
  ...observabilityOperations,
  ...verificationOperations,
  ...governanceOperations,
  ...evolutionOperations,
  ...requirementOperations,
  ...deliveryOperations,
  ...costOperations,
  ...feedbackOperations,
  ...metaOrgOperations,
  ...metaResourceOperations,
  ...developerToolsOperations,
  ...financePayablesOperations,
]

const legacyBusinessPathPrefixes = [
  '/finance',
  '/inventory',
  '/procurement',
  '/sales',
  '/requirements',
  '/projects',
]

function isLegacyBusinessOperation(operation: ApiOperation): boolean {
  if (operation.path.startsWith('/finance/gl/')) return false
  return legacyBusinessPathPrefixes.some((prefix) => operation.path === prefix || operation.path.startsWith(`${prefix}/`))
}

export const apiOperations: ApiOperation[] = allApiOperations.filter((operation) => !isLegacyBusinessOperation(operation))

export const operationDomains = Array.from(new Set(apiOperations.map((operation) => operation.domain)))
