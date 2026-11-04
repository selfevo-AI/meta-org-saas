export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

export interface OperationField {
  name: string
  label: string
  placeholder?: string
}

export interface ApiOperation {
  id: string
  domain: string
  title: string
  method: HttpMethod
  path: string
  auth?: boolean
  pathParams?: OperationField[]
  queryParams?: OperationField[]
  bodyTemplate?: unknown
  operationKind?: OperationKind
  dangerLevel?: OperationDangerLevel
  resultView?: OperationResultView
  assistantEligible?: boolean
  requiresEntityContext?: boolean
  metadata?: Record<string, unknown>
}

export type OperationKind = 'direct' | 'contextual' | 'agent_assisted' | 'admin'
export type OperationDangerLevel = 'low' | 'medium' | 'high'
export type OperationResultView = 'summary' | 'list' | 'detail' | 'audit'
