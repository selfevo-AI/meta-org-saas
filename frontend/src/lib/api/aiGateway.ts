import { AICostSummary, AIInvocation, AIRoutingRule, AIUsageAnalysis, CreateAIRoutingRuleInput, EstimateAICostInput, EstimateAICostOutput } from './types'
import { apiRequest } from './core'
export async function listInvocations(token: string): Promise<AIInvocation[]> {
  return apiRequest<AIInvocation[]>('/ai-gateway/invocations', { token })
}

export async function getAIInvocation(token: string, id: string): Promise<AIInvocation> {
  return apiRequest<AIInvocation>(`/ai-gateway/invocations/${id}`, { token })
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
