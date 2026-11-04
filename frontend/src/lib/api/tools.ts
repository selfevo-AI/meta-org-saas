import { ToolDefinition } from './types'
import { apiRequest } from './core'
export async function listTools(token: string): Promise<ToolDefinition[]> {
  return apiRequest<ToolDefinition[]>('/tools', { token })
}
