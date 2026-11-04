import { BusinessPartner, CreateBusinessPartnerInput, CreateInventoryAdjustmentInput, CreateInventoryCountInput, CreateInventoryItemInput, CreateInventoryMovementInput, CreateInventoryTransferInput, CreateWarehouseInput, InventoryAdjustment, InventoryBalance, InventoryCount, InventoryItem, InventoryMovement, InventoryTransfer, Warehouse } from './types'
import { apiRequest } from './core'
export async function listBusinessPartners(token: string): Promise<BusinessPartner[]> {
  return apiRequest<BusinessPartner[]>('/inventory/partners', { token })
}

export async function createBusinessPartner(token: string, input: CreateBusinessPartnerInput): Promise<BusinessPartner> {
  return apiRequest<BusinessPartner>('/inventory/partners', { method: 'POST', token, body: input })
}

export async function listInventoryItems(token: string): Promise<InventoryItem[]> {
  return apiRequest<InventoryItem[]>('/inventory/items', { token })
}

export async function createInventoryItem(token: string, input: CreateInventoryItemInput): Promise<InventoryItem> {
  return apiRequest<InventoryItem>('/inventory/items', { method: 'POST', token, body: input })
}

export async function listWarehouses(token: string): Promise<Warehouse[]> {
  return apiRequest<Warehouse[]>('/inventory/warehouses', { token })
}

export async function createWarehouse(token: string, input: CreateWarehouseInput): Promise<Warehouse> {
  return apiRequest<Warehouse>('/inventory/warehouses', { method: 'POST', token, body: input })
}

export async function listInventoryBalances(token: string): Promise<InventoryBalance[]> {
  return apiRequest<InventoryBalance[]>('/inventory/balances', { token })
}

export async function listInventoryMovements(token: string): Promise<InventoryMovement[]> {
  return apiRequest<InventoryMovement[]>('/inventory/movements', { token })
}

export async function createInventoryMovement(token: string, input: CreateInventoryMovementInput): Promise<InventoryMovement> {
  return apiRequest<InventoryMovement>('/inventory/movements', { method: 'POST', token, body: input })
}

export async function listInventoryTransfers(token: string): Promise<InventoryTransfer[]> {
  return apiRequest<InventoryTransfer[]>('/inventory/transfers', { token })
}

export async function createInventoryTransfer(token: string, input: CreateInventoryTransferInput): Promise<InventoryTransfer> {
  return apiRequest<InventoryTransfer>('/inventory/transfers', { method: 'POST', token, body: input })
}

export async function listInventoryAdjustments(token: string): Promise<InventoryAdjustment[]> {
  return apiRequest<InventoryAdjustment[]>('/inventory/adjustments', { token })
}

export async function createInventoryAdjustment(token: string, input: CreateInventoryAdjustmentInput): Promise<InventoryAdjustment> {
  return apiRequest<InventoryAdjustment>('/inventory/adjustments', { method: 'POST', token, body: input })
}

export async function listInventoryCounts(token: string): Promise<InventoryCount[]> {
  return apiRequest<InventoryCount[]>('/inventory/counts', { token })
}

export async function createInventoryCount(token: string, input: CreateInventoryCountInput): Promise<InventoryCount> {
  return apiRequest<InventoryCount>('/inventory/counts', { method: 'POST', token, body: input })
}
