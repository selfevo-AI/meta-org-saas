import { AIAccessToken, AIAdapterDescriptor, AIBalanceTransaction, AIGatewayBalance, AIModelChannelAbility, AIModelGroup, AdjustAIGatewayBalanceInput, CreateAIAccessTokenInput, CreateAIModelChannelAbilityInput, CreateAIModelGroupInput, CreateModelInput, CreateModelProviderInput, CreateProviderChannelInput, ModelCatalogItem, ModelProvider, ProviderChannel, UpdateProviderChannelInput } from './types'
import { aiGatewayPath, apiRequest, platformAdminPath } from './core'
export async function createModelProvider(token: string, input: CreateModelProviderInput, scope: 'tenant' | 'platform' = 'tenant'): Promise<ModelProvider> {
  return apiRequest<ModelProvider>(platformAdminPath('/model-providers', scope), { method: 'POST', token, body: input })
}

// Removes a provider together with its channel pool; the backend rejects the
// call with a conflict while catalog models still reference the provider.
export async function deleteModelProvider(token: string, id: string, scope: 'tenant' | 'platform' = 'tenant'): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(platformAdminPath(`/model-providers/${id}`, scope), { method: 'DELETE', token })
}

export async function deleteProviderChannel(token: string, id: string, scope: 'tenant' | 'platform' = 'tenant'): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(platformAdminPath(`/model-provider-channels/${id}`, scope), { method: 'DELETE', token })
}

export async function rotateModelProviderKey(token: string, id: string, apiKey: string, scope: 'tenant' | 'platform' = 'tenant'): Promise<ModelProvider> {
  return apiRequest<ModelProvider>(platformAdminPath(`/model-providers/${id}/rotate-key`, scope), {
    method: 'POST',
    token,
    body: { api_key: apiKey },
  })
}

export async function testModelProvider(token: string, id: string, model?: string, scope: 'tenant' | 'platform' = 'tenant'): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(platformAdminPath(`/model-providers/${id}/test`, scope), {
    method: 'POST',
    token,
    body: { model },
  })
}

export async function listProviderChannels(token: string, providerID?: string, scope: 'tenant' | 'platform' = 'tenant'): Promise<ProviderChannel[]> {
  const query = providerID ? `?provider_id=${encodeURIComponent(providerID)}` : ''
  return apiRequest<ProviderChannel[]>(platformAdminPath(`/model-provider-channels${query}`, scope), { token })
}

export async function createProviderChannel(token: string, providerID: string, input: CreateProviderChannelInput, scope: 'tenant' | 'platform' = 'tenant'): Promise<ProviderChannel> {
  return apiRequest<ProviderChannel>(platformAdminPath(`/model-providers/${providerID}/channels`, scope), { method: 'POST', token, body: input })
}

export async function updateProviderChannel(token: string, id: string, input: UpdateProviderChannelInput, scope: 'tenant' | 'platform' = 'tenant'): Promise<ProviderChannel> {
  return apiRequest<ProviderChannel>(platformAdminPath(`/model-provider-channels/${id}`, scope), { method: 'PATCH', token, body: input })
}

export async function rotateProviderChannelKey(token: string, id: string, apiKey: string, scope: 'tenant' | 'platform' = 'tenant'): Promise<ProviderChannel> {
  return apiRequest<ProviderChannel>(platformAdminPath(`/model-provider-channels/${id}/rotate-key`, scope), {
    method: 'POST',
    token,
    body: { api_key: apiKey },
  })
}

export async function testProviderChannel(token: string, id: string, model?: string, scope: 'tenant' | 'platform' = 'tenant'): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(platformAdminPath(`/model-provider-channels/${id}/test`, scope), {
    method: 'POST',
    token,
    body: { model },
  })
}

export async function createModel(token: string, input: CreateModelInput, scope: 'tenant' | 'platform' = 'tenant'): Promise<ModelCatalogItem> {
  return apiRequest<ModelCatalogItem>(platformAdminPath('/models', scope), { method: 'POST', token, body: input })
}

export async function listAIModelGroups(token: string, organizationID?: string, scope: 'tenant' | 'platform' = 'tenant'): Promise<AIModelGroup[]> {
  const query = organizationID ? `?organization_id=${encodeURIComponent(organizationID)}` : ''
  return apiRequest<AIModelGroup[]>(aiGatewayPath(`/ai-gateway/model-groups${query}`, scope), { token })
}

export async function createAIModelGroup(token: string, input: CreateAIModelGroupInput, scope: 'tenant' | 'platform' = 'tenant'): Promise<AIModelGroup> {
  return apiRequest<AIModelGroup>(aiGatewayPath('/ai-gateway/model-groups', scope), { method: 'POST', token, body: input })
}

export async function listAIModelChannelAbilities(token: string, modelGroupID?: string, scope: 'tenant' | 'platform' = 'tenant'): Promise<AIModelChannelAbility[]> {
  const query = modelGroupID ? `?model_group_id=${encodeURIComponent(modelGroupID)}` : ''
  return apiRequest<AIModelChannelAbility[]>(aiGatewayPath(`/ai-gateway/model-channel-abilities${query}`, scope), { token })
}

export async function createAIModelChannelAbility(token: string, input: CreateAIModelChannelAbilityInput, scope: 'tenant' | 'platform' = 'tenant'): Promise<AIModelChannelAbility> {
  return apiRequest<AIModelChannelAbility>(aiGatewayPath('/ai-gateway/model-channel-abilities', scope), { method: 'POST', token, body: input })
}

export async function listAIAccessTokens(token: string, organizationID?: string, scope: 'tenant' | 'platform' = 'tenant'): Promise<AIAccessToken[]> {
  const query = organizationID ? `?organization_id=${encodeURIComponent(organizationID)}` : ''
  return apiRequest<AIAccessToken[]>(aiGatewayPath(`/ai-gateway/access-tokens${query}`, scope), { token })
}

export async function createAIAccessToken(token: string, input: CreateAIAccessTokenInput, scope: 'tenant' | 'platform' = 'tenant'): Promise<AIAccessToken> {
  return apiRequest<AIAccessToken>(aiGatewayPath('/ai-gateway/access-tokens', scope), { method: 'POST', token, body: input })
}

export async function getAIGatewayBalance(token: string, organizationID: string, currency = 'CNY', scope: 'tenant' | 'platform' = 'tenant'): Promise<AIGatewayBalance> {
  return apiRequest<AIGatewayBalance>(aiGatewayPath(`/ai-gateway/balance?organization_id=${encodeURIComponent(organizationID)}&currency=${encodeURIComponent(currency)}`, scope), { token })
}

export async function adjustAIGatewayBalance(token: string, input: AdjustAIGatewayBalanceInput, scope: 'tenant' | 'platform' = 'tenant'): Promise<AIGatewayBalance> {
  return apiRequest<AIGatewayBalance>(aiGatewayPath('/ai-gateway/balance-adjustments', scope), { method: 'POST', token, body: input })
}

export async function listAIBalanceTransactions(token: string, organizationID: string, scope: 'tenant' | 'platform' = 'tenant'): Promise<AIBalanceTransaction[]> {
  return apiRequest<AIBalanceTransaction[]>(aiGatewayPath(`/ai-gateway/balance-transactions?organization_id=${encodeURIComponent(organizationID)}`, scope), { token })
}

export async function listAIAdapters(token: string, scope: 'tenant' | 'platform' = 'tenant'): Promise<AIAdapterDescriptor[]> {
  return apiRequest<AIAdapterDescriptor[]>(aiGatewayPath('/ai-gateway/adapters', scope), { token })
}
