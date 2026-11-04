import { ToolExecution } from './types'
import { apiRequest } from './core'
export async function listToolExecutions(token: string): Promise<ToolExecution[]> {
  return apiRequest<ToolExecution[]>('/tool-executions', { token })
}
