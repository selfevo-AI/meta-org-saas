const baseURL = process.env.API_BASE_URL || 'http://127.0.0.1:8080/api/v1'
const email = process.env.DEMO_EMAIL || 'demo@local.com'
const password = process.env.DEMO_PASSWORD || 'MetaOrgSampleTenant!2026'
const suffix = process.env.FLOW_SUFFIX || String(Date.now()).slice(-8)

async function request(path, options = {}) {
  const response = await fetch(`${baseURL}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    },
  })
  const text = await response.text()
  const body = text ? JSON.parse(text) : null
  if (!response.ok) {
    throw new Error(`${options.method || 'GET'} ${path} failed ${response.status}: ${text}`)
  }
  return body
}

function authHeaders(token, organizationID) {
  return {
    Authorization: `Bearer ${token}`,
    'X-Organization-ID': organizationID,
  }
}

async function createRecord(headers, table, key, data) {
  return request(`/erp/${table}`, {
    method: 'POST',
    headers,
    body: JSON.stringify({ key, data }),
  })
}

async function createChild(headers, table, parentKey, childTable, lineKey, data) {
  return request(`/erp/${table}/${encodeURIComponent(parentKey)}/${childTable}`, {
    method: 'POST',
    headers,
    body: JSON.stringify({ key: lineKey, data }),
  })
}

async function action(headers, table, key, name, data = {}) {
  return request(`/erp/${table}/${encodeURIComponent(key)}/actions/${name}`, {
    method: 'POST',
    headers,
    body: JSON.stringify({ idempotency_key: `${suffix}-${table}-${key}-${name}`, data }),
  })
}

async function getRecord(headers, table, key) {
  return request(`/erp/${table}/${encodeURIComponent(key)}`, { headers })
}

async function listChildren(headers, table, parentKey, childTable) {
  const result = await request(`/erp/${table}/${encodeURIComponent(parentKey)}/${childTable}`, { headers })
  return result.records || []
}

function assertEqual(actual, expected, label) {
  if (actual !== expected) {
    throw new Error(`${label}: got ${actual}, want ${expected}`)
  }
}

function assertBalanced(record) {
  const debit = Number(record.data?.TotalDebit || 0)
  const credit = Number(record.data?.TotalCredit || 0)
  if (debit <= 0 || debit !== credit) {
    throw new Error(`trial balance not balanced: debit=${debit} credit=${credit}`)
  }
}

const login = await request('/auth/login', {
  method: 'POST',
  body: JSON.stringify({ email, password }),
})
const organizationID = login.default_organization_id || login.organizations?.[0]?.id
if (!organizationID) {
  throw new Error('demo user has no organization')
}
const headers = authHeaders(login.token, organizationID)

const reqKey = `REQ-FLOW-${suffix}`
const prjKey = `PRJ-FLOW-${suffix}`
const poKey = `PO-FLOW-${suffix}`
const grpoKey = `GRPO-FLOW-${suffix}`
const rawItem = `RAW-FLOW-${suffix}`
const fgItem = `FG-FLOW-${suffix}`
const bomKey = `BOM-FLOW-${suffix}`
const woKey = `WO-FLOW-${suffix}`
const deliveryKey = `DLV-FLOW-${suffix}`
const invoiceKey = `INV-${deliveryKey}`
const paymentKey = `PAY-FLOW-${suffix}`
const trialBalanceKey = `TB-FLOW-${suffix}`

await createRecord(headers, 'MREQ', reqKey, { ReqCode: reqKey, Name: 'Full-flow manufacturing requirement', Status: 'draft' })
await action(headers, 'MREQ', reqKey, 'analyze')
await action(headers, 'MREQ', reqKey, 'approve')
await action(headers, 'MREQ', reqKey, 'convert-to-project', { PrjCode: prjKey })
await action(headers, 'MPRJ', prjKey, 'refresh-cost')
await action(headers, 'MPRJ', prjKey, 'close-feedback', { result: 'accepted' })

await createRecord(headers, 'MPOR', poKey, { DocEntry: poKey, DocStatus: 'O', WddStatus: 'W', CardCode: 'SUP-FLOW' })
await action(headers, 'MPOR', poKey, 'submit')
await action(headers, 'MPOR', poKey, 'approve')
await createRecord(headers, 'MPDN', grpoKey, { DocEntry: grpoKey, DocStatus: 'O', WddStatus: 'A', CardCode: 'SUP-FLOW' })
await createChild(headers, 'MPDN', grpoKey, 'PDN1', '1', { LineNum: '1', Payload: { ItemCode: rawItem, WhsCode: 'RM', Quantity: 10, Price: 5 } })
await action(headers, 'MPDN', grpoKey, 'post')
assertEqual(Number((await getRecord(headers, 'MITW', `${rawItem}|RM`)).data.OnHand), 10, 'raw material after procurement')

await createRecord(headers, 'MBOM', bomKey, {
  BOMCode: bomKey,
  Status: 'draft',
  ItemCode: fgItem,
  Quantity: 1,
  SourceWhsCode: 'RM',
  FinishedWhsCode: 'FG',
  WipWhsCode: 'WIP',
})
await createChild(headers, 'MBOM', bomKey, 'BOM1', '1', { LineNum: '1', Payload: { ItemCode: rawItem, WhsCode: 'RM', Quantity: 2, Price: 5 } })
await action(headers, 'MBOM', bomKey, 'approve')
await action(headers, 'MBOM', bomKey, 'make-work-order', { WorkOrderCode: woKey, Quantity: 3 })
await action(headers, 'MWOR', woKey, 'release')
await action(headers, 'MWOR', woKey, 'issue-material')
assertEqual(Number((await getRecord(headers, 'MITW', `${rawItem}|RM`)).data.OnHand), 4, 'raw material after work order issue')
await action(headers, 'MWOR', woKey, 'complete')
assertEqual(Number((await getRecord(headers, 'MITW', `${fgItem}|FG`)).data.OnHand), 3, 'finished goods after completion')
assertEqual((await listChildren(headers, 'MJDT', `JE-${woKey}`, 'JDT1')).length, 2, 'production journal rows')

await createRecord(headers, 'MDLN', deliveryKey, { DocEntry: deliveryKey, DocStatus: 'O', WddStatus: 'A', CardCode: 'CUS-FLOW' })
await createChild(headers, 'MDLN', deliveryKey, 'DLN1', '1', { LineNum: '1', Payload: { ItemCode: fgItem, WhsCode: 'FG', Quantity: 2, Price: 20 } })
await action(headers, 'MDLN', deliveryKey, 'post')
assertEqual(Number((await getRecord(headers, 'MITW', `${fgItem}|FG`)).data.OnHand), 1, 'finished goods after delivery')
await action(headers, 'MINV', invoiceKey, 'post')
await createRecord(headers, 'MRCT', paymentKey, { DocEntry: paymentKey, DocTotal: 40, OpenBal: 40 })
await action(headers, 'MRCT', paymentKey, 'allocate', { TargetTable: 'MINV', TargetKey: invoiceKey, Amount: 40 })
assertEqual((await getRecord(headers, 'MINV', invoiceKey)).data.DocStatus, 'C', 'invoice status after payment')

const trialBalance = await action(headers, 'MGLR', trialBalanceKey, 'run', { ReportCode: trialBalanceKey, Currency: 'CNY' })
assertBalanced(trialBalance.record)

console.log(JSON.stringify({
  organization_id: organizationID,
  tenant_email: email,
  suffix,
  project: prjKey,
  work_order: woKey,
  invoice: invoiceKey,
  trial_balance: {
    key: trialBalanceKey,
    total_debit: trialBalance.record.data.TotalDebit,
    total_credit: trialBalance.record.data.TotalCredit,
  },
}, null, 2))
