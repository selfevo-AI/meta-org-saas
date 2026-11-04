import { ToolApprovalReviewResult } from './types'
import { apiRequest } from './core'
export async function approveToolApproval(token: string, id: string, reason = 'approved from human review console'): Promise<ToolApprovalReviewResult> {
  return apiRequest<ToolApprovalReviewResult>(`/tool-approvals/${id}/approve`, { method: 'POST', token, body: { reason } })
}

export async function rejectToolApproval(token: string, id: string, reason = 'rejected from human review console'): Promise<ToolApprovalReviewResult> {
  return apiRequest<ToolApprovalReviewResult>(`/tool-approvals/${id}/reject`, { method: 'POST', token, body: { reason } })
}
