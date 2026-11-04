import { OnboardingOrganizationInput, OnboardingOrganizationResponse } from './types'
import { apiRequest } from './core'
export async function completeOnboarding(token: string, input: OnboardingOrganizationInput): Promise<OnboardingOrganizationResponse> {
  return apiRequest<OnboardingOrganizationResponse>('/onboarding/organization', {
    method: 'POST',
    token,
    body: input,
  })
}
