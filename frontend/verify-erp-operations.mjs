import { readFileSync } from 'node:fs'

const source = readFileSync('src/lib/operations.ts', 'utf8')

const required = [
  '/erp/actions',
  '/erp/MREQ/{ReqCode}/actions/approve',
  '/erp/MREQ/{ReqCode}/actions/convert-to-project',
  '/erp/MPRJ/{PrjCode}/actions/refresh-cost',
  '/erp/MPRJ/{PrjCode}/actions/close-feedback',
  '/erp/MPOR/{DocEntry}/actions/submit',
  '/erp/MPOR/{DocEntry}/actions/approve',
  '/erp/MPDN/{DocEntry}/actions/post',
  '/erp/MRDR/{DocEntry}/actions/confirm',
  '/erp/MRDR/{DocEntry}/actions/approve',
  '/erp/MDLN/{DocEntry}/actions/post',
  '/erp/MINV/{DocEntry}/actions/post',
  '/erp/MRCT/{DocEntry}/actions/allocate',
  '/erp/MIGN/{DocEntry}/actions/post',
  '/erp/MIGE/{DocEntry}/actions/post',
  '/erp/MJDT/{TransId}/actions/post',
]

for (const path of required) {
  if (!source.includes(path)) {
    console.error(`missing ERP operation ${path}`)
    process.exit(1)
  }
}

const actionPaths = source.match(/\/erp\/[^'"]+\/actions\/[^'"]+/g) ?? []
if (actionPaths.length < 12) {
  console.error(`expected at least 12 ERP action operations, found ${actionPaths.length}`)
  process.exit(1)
}

console.log(`Verified ${required.length} required ERP operations`)
