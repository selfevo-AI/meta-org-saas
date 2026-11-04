import type { ApiOperation } from '../types'

export const verificationOperations: ApiOperation[] = [
  {
    id: 'verification-create-report',
    domain: 'Verification',
    title: '创建验证报告',
    method: 'POST',
    path: '/verification/reports',
    bodyTemplate: {
      workflow_id: null,
      task_id: null,
      result_score: 0.9,
      path_score: 0.8,
      environment_score: 0.85,
      conclusion: 'passed',
      suggestions: [],
    },
  },
  {
    id: 'verification-list-reports',
    domain: 'Verification',
    title: '报告列表',
    method: 'GET',
    path: '/verification/reports',
    queryParams: [
      { name: 'workflow_id', label: '工作流 ID' },
      { name: 'limit', label: '数量', placeholder: '50' },
    ],
  },
  {
    id: 'verification-get-report',
    domain: 'Verification',
    title: '报告详情',
    method: 'GET',
    path: '/verification/reports/{id}',
    pathParams: [{ name: 'id', label: '报告 ID' }],
  },
  {
    id: 'verification-assign-review',
    domain: 'Verification',
    title: '分配复核',
    method: 'POST',
    path: '/verification/reports/{id}/reviews',
    pathParams: [{ name: 'id', label: '报告 ID' }],
    bodyTemplate: {
      level: 'L1',
      reviewer_id: null,
      reviewer_type: 'machine',
    },
  },
  {
    id: 'verification-complete-review',
    domain: 'Verification',
    title: '完成复核',
    method: 'PATCH',
    path: '/verification/reviews/{id}',
    pathParams: [{ name: 'id', label: '复核 ID' }],
    bodyTemplate: {
      result: {
        status: 'approved',
      },
    },
  },
]
