import type { ApiOperation, OperationDangerLevel, OperationKind, OperationResultView } from './types'

export interface OperationProfile {
  kind: OperationKind
  dangerLevel: OperationDangerLevel
  resultView: OperationResultView
  assistantEligible: boolean
  requiresEntityContext: boolean
  humanReady: boolean
}

const agentSignals = [
  'ai',
  'agent',
  'assistant',
  'analyze',
  'analysis',
  'match',
  'estimate',
  'classify',
  'compute',
  'sync',
  'recommend',
]

const adminSignals = [
  'identity',
  'provider',
  'channel',
  'routing-rule',
  'governance',
  'observability',
  'verification',
  'tool',
  'trace',
  'metric',
  'layer',
]

function includesAny(value: string, signals: string[]): boolean {
  return signals.some((signal) => value.includes(signal))
}

export function getOperationProfile(operation: ApiOperation): OperationProfile {
  const searchable = `${operation.id} ${operation.domain} ${operation.path} ${operation.title}`.toLowerCase()
  const requiresEntityContext = operation.requiresEntityContext ?? ((operation.pathParams?.length ?? 0) > 0)
  const assistantEligible = operation.assistantEligible ?? includesAny(searchable, agentSignals)
  const isAdmin = includesAny(searchable, adminSignals)
  const isDelete = operation.method === 'DELETE'
  const isMutation = operation.method !== 'GET'
  const dangerLevel = operation.dangerLevel ?? (isDelete ? 'high' : isMutation || requiresEntityContext ? 'medium' : 'low')
  const kind =
    operation.operationKind ??
    (assistantEligible ? 'agent_assisted' : isAdmin ? 'admin' : requiresEntityContext ? 'contextual' : 'direct')
  const resultView =
    operation.resultView ??
    (operation.method === 'GET'
      ? operation.id.includes('list') || operation.path.endsWith('s')
        ? 'list'
        : 'detail'
      : isAdmin || searchable.includes('log') || searchable.includes('audit')
        ? 'audit'
        : 'summary')

  return {
    kind,
    dangerLevel,
    resultView,
    assistantEligible,
    requiresEntityContext,
    humanReady: !requiresEntityContext || kind === 'direct' || operation.method === 'GET',
  }
}
