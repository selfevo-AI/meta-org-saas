import type { ApiOperation } from '../types'

export const erpOperations: ApiOperation[] = [
  {
    id: 'erp-catalog',
    domain: 'ERP',
    title: 'operation.erp.catalog',
    method: 'GET',
    path: '/erp/catalog',
    resultView: 'list',
  },
  {
    id: 'erp-record-list',
    domain: 'ERP',
    title: 'operation.erp.recordList',
    method: 'GET',
    path: '/erp/{tableCode}',
    pathParams: [{ name: 'tableCode', label: 'operation.erp.tableCode', placeholder: 'MCRD' }],
    queryParams: [{ name: 'limit', label: 'operation.param.limit', placeholder: '100' }],
    resultView: 'list',
  },
  {
    id: 'erp-record-create',
    domain: 'ERP',
    title: 'operation.erp.recordCreate',
    method: 'POST',
    path: '/erp/{tableCode}',
    pathParams: [{ name: 'tableCode', label: 'operation.erp.tableCode', placeholder: 'MCRD' }],
    bodyTemplate: {
      key: 'C0001',
      data: {
        CardCode: 'C0001',
        CardName: 'Acme Customer',
        CardType: 'C',
      },
    },
    resultView: 'detail',
  },
  {
    id: 'erp-record-get',
    domain: 'ERP',
    title: 'operation.erp.recordGet',
    method: 'GET',
    path: '/erp/{tableCode}/{key}',
    pathParams: [
      { name: 'tableCode', label: 'operation.erp.tableCode', placeholder: 'MCRD' },
      { name: 'key', label: 'operation.erp.recordKey', placeholder: 'C0001' },
    ],
    resultView: 'detail',
  },
  {
    id: 'erp-record-update',
    domain: 'ERP',
    title: 'operation.erp.recordUpdate',
    method: 'PATCH',
    path: '/erp/{tableCode}/{key}',
    pathParams: [
      { name: 'tableCode', label: 'operation.erp.tableCode', placeholder: 'MCRD' },
      { name: 'key', label: 'operation.erp.recordKey', placeholder: 'C0001' },
    ],
    bodyTemplate: {
      data: {
        CardName: 'Updated Customer',
      },
    },
    resultView: 'detail',
  },
  {
    id: 'erp-child-list',
    domain: 'ERP',
    title: 'operation.erp.childList',
    method: 'GET',
    path: '/erp/{tableCode}/{key}/{childCode}',
    pathParams: [
      { name: 'tableCode', label: 'operation.erp.tableCode', placeholder: 'MINV' },
      { name: 'key', label: 'operation.erp.recordKey', placeholder: '1001' },
      { name: 'childCode', label: 'operation.erp.childTableCode', placeholder: 'INV1' },
    ],
    queryParams: [{ name: 'limit', label: 'operation.param.limit', placeholder: '100' }],
    resultView: 'list',
  },
  {
    id: 'erp-child-create',
    domain: 'ERP',
    title: 'operation.erp.childCreate',
    method: 'POST',
    path: '/erp/{tableCode}/{key}/{childCode}',
    pathParams: [
      { name: 'tableCode', label: 'operation.erp.tableCode', placeholder: 'MINV' },
      { name: 'key', label: 'operation.erp.recordKey', placeholder: '1001' },
      { name: 'childCode', label: 'operation.erp.childTableCode', placeholder: 'INV1' },
    ],
    bodyTemplate: {
      key: '1',
      data: {
        LineNum: 1,
        LineStatus: 'O',
        Payload: {
          ItemCode: 'A-100',
          Quantity: 1,
        },
      },
    },
    resultView: 'detail',
  },
  {
    id: 'erp-action-list',
    domain: 'ERP',
    title: 'operation.erp.actions',
    method: 'GET',
    path: '/erp/actions',
    resultView: 'list',
    assistantEligible: true,
  },
]
