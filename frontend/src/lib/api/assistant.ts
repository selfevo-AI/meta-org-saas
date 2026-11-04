import { AssistantBusinessSkill, AssistantContextPackageDiagnostic, AssistantContextTarget, AssistantProposal, AssistantSession, AssistantSkillRun, AssistantStep, CreateAssistantBusinessSkillInput, CreateAssistantSessionInput } from './types'
import { apiRequest } from './core'
export async function createAssistantSession(token: string, input: CreateAssistantSessionInput): Promise<AssistantSession> {
  return apiRequest<AssistantSession>('/assistant/sessions', { method: 'POST', token, body: input })
}

export async function listAssistantSessions(token: string, moduleKey?: string): Promise<AssistantSession[]> {
  const query = moduleKey ? `?module_key=${encodeURIComponent(moduleKey)}` : ''
  return apiRequest<AssistantSession[]>(`/assistant/sessions${query}`, { token })
}

export async function listAssistantSteps(token: string, sessionID: string): Promise<AssistantStep[]> {
  return apiRequest<AssistantStep[]>(`/assistant/sessions/${sessionID}/steps`, { token })
}

export async function listAssistantContextTargets(token: string, moduleKey: string, targetType?: string): Promise<AssistantContextTarget[]> {
  const params = new URLSearchParams()
  if (moduleKey) params.set('module_key', moduleKey)
  if (targetType) params.set('target_type', targetType)
  const query = params.toString() ? `?${params.toString()}` : ''
  return apiRequest<AssistantContextTarget[]>(`/assistant/context-targets${query}`, { token })
}

export async function getAssistantContextPackageDiagnostic(token: string, packageID: string): Promise<AssistantContextPackageDiagnostic> {
  return apiRequest<AssistantContextPackageDiagnostic>(`/assistant/context-packages/${packageID}`, { token })
}

export async function listAssistantProposals(token: string, sessionID: string): Promise<AssistantProposal[]> {
  return apiRequest<AssistantProposal[]>(`/assistant/sessions/${sessionID}/proposals`, { token })
}

export async function confirmAssistantProposal(token: string, proposalID: string): Promise<AssistantProposal> {
  return apiRequest<AssistantProposal>(`/assistant/proposals/${proposalID}/confirm`, { method: 'POST', token })
}

export async function rejectAssistantProposal(token: string, proposalID: string, reason = ''): Promise<AssistantProposal> {
  return apiRequest<AssistantProposal>(`/assistant/proposals/${proposalID}/reject`, {
    method: 'POST',
    token,
    body: { reason },
  })
}

export async function listAssistantSkills(token: string, moduleKey?: string, targetType?: string): Promise<AssistantBusinessSkill[]> {
  const params = new URLSearchParams()
  if (moduleKey) params.set('module_key', moduleKey)
  if (targetType) params.set('target_type', targetType)
  const query = params.toString() ? `?${params.toString()}` : ''
  return apiRequest<AssistantBusinessSkill[]>(`/assistant/skills${query}`, { token })
}

export async function createAssistantSkill(
  token: string,
  input: CreateAssistantBusinessSkillInput,
): Promise<AssistantBusinessSkill> {
  return apiRequest<AssistantBusinessSkill>('/assistant/skills', { method: 'POST', token, body: input })
}

export async function activateAssistantSkill(token: string, skillID: string): Promise<AssistantBusinessSkill> {
  return apiRequest<AssistantBusinessSkill>(`/assistant/skills/${skillID}/activate`, { method: 'POST', token })
}

export async function runAssistantSkill(
  token: string,
  skillID: string,
  input: Record<string, unknown>,
): Promise<AssistantSkillRun> {
  return apiRequest<AssistantSkillRun>(`/assistant/skills/${skillID}/run`, { method: 'POST', token, body: input })
}
