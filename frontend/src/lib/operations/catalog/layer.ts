import type { ApiOperation } from '../types'

export const layerOperations: ApiOperation[] = [
  {
    id: 'layer-classify',
    domain: 'Layer',
    title: '任务分层',
    method: 'POST',
    path: '/layers/classify',
    bodyTemplate: {
      task_description: 'Review a strategic compliance initiative',
      complexity_score: 0.5,
      risk_score: 0.3,
      strategic_score: 0.7,
      metadata: {},
    },
  },
  {
    id: 'layer-get-config',
    domain: 'Layer',
    title: '读取层配置',
    method: 'GET',
    path: '/layers/config/{mvruId}',
    pathParams: [{ name: 'mvruId', label: 'MVRU ID' }],
    queryParams: [{ name: 'layer', label: '层级', placeholder: 'strategic | tactical | operational' }],
  },
  {
    id: 'layer-set-config',
    domain: 'Layer',
    title: '保存层配置',
    method: 'PUT',
    path: '/layers/config/{mvruId}',
    pathParams: [{ name: 'mvruId', label: 'MVRU ID' }],
    bodyTemplate: {
      layer: 'tactical',
      config: {
        routing: 'balanced',
      },
    },
  },
  {
    id: 'layer-list-rules',
    domain: 'Layer',
    title: '路由规则',
    method: 'GET',
    path: '/layers/rules',
  },
]
