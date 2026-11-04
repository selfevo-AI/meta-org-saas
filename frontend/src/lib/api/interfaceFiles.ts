import { CreateInterfaceFileInput, InterfaceFile, UpdateInterfaceFileInput } from './types'
import { apiRequest } from './core'
export async function listInterfaceFiles(token: string): Promise<InterfaceFile[]> {
  return apiRequest<InterfaceFile[]>('/interface-files', { token })
}

export async function getInterfaceFile(token: string, id: string): Promise<InterfaceFile> {
  return apiRequest<InterfaceFile>(`/interface-files/${id}`, { token })
}

export async function createInterfaceFile(token: string, input: CreateInterfaceFileInput): Promise<InterfaceFile> {
  return apiRequest<InterfaceFile>('/interface-files', { method: 'POST', token, body: input })
}

export async function updateInterfaceFile(token: string, id: string, input: UpdateInterfaceFileInput): Promise<InterfaceFile> {
  return apiRequest<InterfaceFile>(`/interface-files/${id}`, { method: 'PATCH', token, body: input })
}
