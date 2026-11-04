import type { ApiOperation } from '../types'

export const identityOperations: ApiOperation[] = [
  {
    id: 'identity-list-roles',
    domain: 'Identity',
    title: '角色列表',
    method: 'GET',
    path: '/roles',
    auth: false,
  },
  {
    id: 'identity-register-agent',
    domain: 'Identity',
    title: '注册 AI Agent',
    method: 'POST',
    path: '/agents/register',
    bodyTemplate: {
      name: 'Finance Analyst Agent',
      model_type: 'gpt-4.1',
      capabilities: ['analysis', 'reporting'],
      permission_level: 'L2',
      agent_origin: 'internal',
      provider: 'OpenAI',
      service_class: 'model',
      vendor: '',
      contract_ref: '',
      risk_level: 'medium',
      metadata: {},
    },
  },
  {
    id: 'identity-list-agents',
    domain: 'Identity',
    title: 'Agent 列表',
    method: 'GET',
    path: '/agents',
    queryParams: [{ name: 'limit', label: '数量', placeholder: '50' }],
  },
  {
    id: 'identity-agent-auth',
    domain: 'Identity',
    title: 'Agent 鉴权',
    method: 'POST',
    path: '/agents/auth',
    auth: false,
    bodyTemplate: {
      agent_id: '',
      api_key: '',
    },
  },
]
