import { CreatePurchaseOrderInput, CreatePurchaseReceiptInput, CreatePurchaseRequisitionInput, CreatePurchaseReturnInput, PurchaseOrder, PurchaseReceipt, PurchaseRequisition, PurchaseReturn } from './types'
import { apiRequest } from './core'
export async function listPurchaseRequisitions(token: string): Promise<PurchaseRequisition[]> {
  return apiRequest<PurchaseRequisition[]>('/procurement/requisitions', { token })
}

export async function createPurchaseRequisition(token: string, input: CreatePurchaseRequisitionInput): Promise<PurchaseRequisition> {
  return apiRequest<PurchaseRequisition>('/procurement/requisitions', { method: 'POST', token, body: input })
}

export async function submitPurchaseRequisition(token: string, id: string): Promise<PurchaseRequisition> {
  return apiRequest<PurchaseRequisition>(`/procurement/requisitions/${id}/submit`, { method: 'POST', token })
}

export async function approvePurchaseRequisition(token: string, id: string): Promise<PurchaseRequisition> {
  return apiRequest<PurchaseRequisition>(`/procurement/requisitions/${id}/approve`, { method: 'POST', token })
}

export async function listPurchaseOrders(token: string): Promise<PurchaseOrder[]> {
  return apiRequest<PurchaseOrder[]>('/procurement/orders', { token })
}

export async function createPurchaseOrder(token: string, input: CreatePurchaseOrderInput): Promise<PurchaseOrder> {
  return apiRequest<PurchaseOrder>('/procurement/orders', { method: 'POST', token, body: input })
}

export async function submitPurchaseOrder(token: string, id: string): Promise<PurchaseOrder> {
  return apiRequest<PurchaseOrder>(`/procurement/orders/${id}/submit`, { method: 'POST', token })
}

export async function approvePurchaseOrder(token: string, id: string): Promise<PurchaseOrder> {
  return apiRequest<PurchaseOrder>(`/procurement/orders/${id}/approve`, { method: 'POST', token })
}

export async function listPurchaseReceipts(token: string): Promise<PurchaseReceipt[]> {
  return apiRequest<PurchaseReceipt[]>('/procurement/receipts', { token })
}

export async function createPurchaseReceipt(token: string, input: CreatePurchaseReceiptInput): Promise<PurchaseReceipt> {
  return apiRequest<PurchaseReceipt>('/procurement/receipts', { method: 'POST', token, body: input })
}

export async function postPurchaseReceipt(token: string, id: string): Promise<PurchaseReceipt> {
  return apiRequest<PurchaseReceipt>(`/procurement/receipts/${id}/post`, { method: 'POST', token })
}

export async function listPurchaseReturns(token: string): Promise<PurchaseReturn[]> {
  return apiRequest<PurchaseReturn[]>('/procurement/returns', { token })
}

export async function createPurchaseReturn(token: string, input: CreatePurchaseReturnInput): Promise<PurchaseReturn> {
  return apiRequest<PurchaseReturn>('/procurement/returns', { method: 'POST', token, body: input })
}
