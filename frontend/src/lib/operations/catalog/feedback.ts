import type { ApiOperation } from '../types'

export const feedbackOperations: ApiOperation[] = [
  {
    id: 'feedback-create',
    domain: 'Feedback',
    title: '创建项目评估',
    method: 'POST',
    path: '/projects/{id}/evaluations',
    pathParams: [{ name: 'id', label: '项目 ID' }],
    bodyTemplate: {
      evaluated_actor_id: null,
      evaluated_actor_type: 'internal_human',
      capability_id: null,
      quality_score: 0.8,
      delivery_score: 0.8,
      cost_score: 0.75,
      collaboration_score: 0.8,
      conclusion: 'human-reviewed project delivery',
      evidence: {},
    },
  },
  {
    id: 'feedback-list',
    domain: 'Feedback',
    title: '项目评估列表',
    method: 'GET',
    path: '/projects/{id}/evaluations',
    pathParams: [{ name: 'id', label: '项目 ID' }],
  },
  {
    id: 'feedback-close',
    domain: 'Feedback',
    title: '闭环反馈',
    method: 'POST',
    path: '/projects/{id}/close-feedback',
    pathParams: [{ name: 'id', label: '项目 ID' }],
    bodyTemplate: {
      outcome_score: 0.8,
      conclusion: 'project closed with feedback',
      metadata: {},
    },
  },
]
