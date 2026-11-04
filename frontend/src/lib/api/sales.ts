import { CreateSalesOrderInput, CreateSalesQuotationInput, CreateSalesReturnInput, CreateSalesShipmentInput, SalesOrder, SalesQuotation, SalesReturn, SalesShipment } from './types'
import { apiRequest } from './core'
export async function listSalesQuotations(token: string): Promise<SalesQuotation[]> {
  return apiRequest<SalesQuotation[]>('/sales/quotations', { token })
}

export async function createSalesQuotation(token: string, input: CreateSalesQuotationInput): Promise<SalesQuotation> {
  return apiRequest<SalesQuotation>('/sales/quotations', { method: 'POST', token, body: input })
}

export async function listSalesOrders(token: string): Promise<SalesOrder[]> {
  return apiRequest<SalesOrder[]>('/sales/orders', { token })
}

export async function createSalesOrder(token: string, input: CreateSalesOrderInput): Promise<SalesOrder> {
  return apiRequest<SalesOrder>('/sales/orders', { method: 'POST', token, body: input })
}

export async function confirmSalesOrder(token: string, id: string): Promise<SalesOrder> {
  return apiRequest<SalesOrder>(`/sales/orders/${id}/confirm`, { method: 'POST', token })
}

export async function approveSalesOrder(token: string, id: string): Promise<SalesOrder> {
  return apiRequest<SalesOrder>(`/sales/orders/${id}/approve`, { method: 'POST', token })
}

export async function listSalesShipments(token: string): Promise<SalesShipment[]> {
  return apiRequest<SalesShipment[]>('/sales/shipments', { token })
}

export async function createSalesShipment(token: string, input: CreateSalesShipmentInput): Promise<SalesShipment> {
  return apiRequest<SalesShipment>('/sales/shipments', { method: 'POST', token, body: input })
}

export async function postSalesShipment(token: string, id: string): Promise<SalesShipment> {
  return apiRequest<SalesShipment>(`/sales/shipments/${id}/post`, { method: 'POST', token })
}

export async function listSalesReturns(token: string): Promise<SalesReturn[]> {
  return apiRequest<SalesReturn[]>('/sales/returns', { token })
}

export async function createSalesReturn(token: string, input: CreateSalesReturnInput): Promise<SalesReturn> {
  return apiRequest<SalesReturn>('/sales/returns', { method: 'POST', token, body: input })
}
