# ERP Code-Table Business Migration and SaaS Industry Solution Standard

## Purpose

This design corrects the ERP restructuring direction for `meta-org-saas`.
The target is not an empty ERP table CRUD system. The target is a complete
business platform where existing project, procurement, sales, inventory,
finance, and costing capabilities are migrated into the ERP code-table naming
system, and missing business steps are rebuilt following standard SAP-style ERP
business flows.

AI assistant capabilities and SaaS platform administration remain in place.
They must connect to the new ERP business objects and industry solution process
without losing their existing behavior.

## Core Requirements

1. Existing useful business functions must be migrated, not removed.
2. Business routes must use ERP table-code APIs instead of old semantic paths.
3. Missing functionality must be rebuilt using standard ERP/SAP business
   process patterns.
4. Project management must become part of the end-to-end ERP business loop.
5. Useful reconstruction practices must become reusable SaaS platform
   administration standards for creating and adjusting industry solutions.
6. AI assistant and SaaS platform management modules must remain compatible and
   continue to operate.

## Database Model

The physical ERP business schema uses quoted SAP-style table codes and fields.
The naming convention for future business tables and fields follows
`tables_structure.md`.

ERP capabilities must be organized as a first-class hierarchy, not as a flat
table list:

1. Module: high-level business area such as Project, Sales, Purchasing,
   Inventory, Finance, Master Data, or Platform.
2. Submodule: a functional area inside a module, such as Sales Orders,
   Deliveries, A/R Invoices, Purchase Orders, Goods Receipts, Items,
   Warehouses, Requirements, or Project Cost.
3. Business document or master-data type: the human business object operated
   by users and agents.
4. Master table and child tables: the physical ERP code tables backing that
   business document.
5. Actions: submit, approve, post, allocate, refresh, close, analyze, convert,
   and other business transitions available for that document.

The ERP catalog API must expose this hierarchy so the frontend, API workbench,
AI assistant, and SaaS industry solution builder can render and validate the
same module/submodule/document structure.

Standard table families:

- Finance: `MACT/AACT`, `MJDT/JDT1`, `MBTF/BTF1`, `MBTD`, `MPRC/APRC`
- Partners: `MCRD/CRD1`
- Products and project dimensions: `MITM/ITM1`, `MITW/ITW1`, `MPRJ/APRJ`
- Sales: `MQUT/QUT1`, `MRDR/RDR1`, `MDLN/DLN1`, `MINV/INV1`, `MRIN/RIN1`,
  `MRDN/RDN1`, `MRCT/RCT1`, `MDPS/DPS1`
- Purchasing: `MPOR/POR1`, `MPDN/PDN1`, `MPCH/PCH1`, `MRPC/RPC1`,
  `MRPD/RPD1`
- Warehouse: `MWHS/AWHS`, `MIGN/IGN1`, `MIGE/IGE1`
- Users and permissions: `MUSR/AUSR`

Project management extensions use the same table-code convention:

- `MREQ/REQ1`: requirements and requirement rows
- `MPRJ/APRJ`: project master data and project history
- `MDLN/DLN1`: project/customer delivery documents
- `MCST/CST1`: project cost records and cost rows
- `MFDB/FDB1`: feedback and evaluation records

Required module/submodule/document mapping:

- Project
  - Requirements: Requirement `MREQ/REQ1`
  - Projects: Project `MPRJ/APRJ`
  - Deliverables: Delivery `MDLN/DLN1`
  - Cost: Cost Record `MCST/CST1`
  - Feedback: Feedback `MFDB/FDB1`
- Master Data
  - Business Partners: Business Partner `MCRD/CRD1`
  - Items: Item `MITM/ITM1`
  - Warehouses: Warehouse `MWHS/AWHS`
- Purchasing
  - Purchase Orders: Purchase Order `MPOR/POR1`
  - Goods Receipt PO: Goods Receipt PO `MPDN/PDN1`
  - A/P Invoices: A/P Invoice `MPCH/PCH1`
  - Goods Returns: Goods Return `MRPD/RPD1`
  - A/P Credit Memos: A/P Credit Memo `MRPC/RPC1`
- Sales
  - Quotations: Sales Quotation `MQUT/QUT1`
  - Sales Orders: Sales Order `MRDR/RDR1`
  - Deliveries: Delivery `MDLN/DLN1`
  - A/R Invoices: A/R Invoice `MINV/INV1`
  - Returns: Return `MRDN/RDN1`
  - Incoming Payments: Incoming Payment `MRCT/RCT1`
- Inventory
  - Warehouse Balances: Item Warehouse `MITW/ITW1`
  - Goods Receipts: Goods Receipt `MIGN/IGN1`
  - Goods Issues: Goods Issue `MIGE/IGE1`
  - Inventory Adjustments: Goods Receipt/Goods Issue actions over `MIGN/MIGE`
- Finance
  - Chart of Accounts: G/L Account `MACT/AACT`
  - Cost Centers: Cost Center `MPRC/APRC`
  - Journal Entries: Journal Entry `MJDT/JDT1`
  - Journal Vouchers: Journal Voucher `MBTF/BTF1`
- Platform
  - Users and Permissions: User `MUSR/AUSR`
  - SaaS and Industry Solution Management: platform extension code tables

If `tables_structure.md` does not define fields needed by a business process,
the implementation may store extension fields in `Payload JSONB`, but the API
must expose typed business fields and actions. Users should not need to operate
raw JSON for normal workflows.

## Business Process Coverage

### Lead-To-Cash / Project-To-Cash

1. Requirement is created in `MREQ`.
2. Requirement is analyzed and approved.
3. Approved requirement is converted into project `MPRJ`.
4. Project can create quotation `MQUT`.
5. Accepted quotation creates or links sales order `MRDR`.
6. Sales order confirmation and approval update `MRDR.DocStatus` and
   authorization status fields.
7. Delivery or shipment posts `MDLN` and inventory issue `MIGE`.
8. Invoice posts `MINV`.
9. Incoming payment posts `MRCT` and allocates to invoice.
10. Revenue, cost, and margin are visible on the project record.
11. Feedback closes the loop in `MFDB`.

### Source-To-Pay

1. Project, stock demand, or manual request creates purchase order `MPOR`.
2. Purchase order submission and approval update document status fields.
3. Goods receipt posts `MPDN` and inventory receipt `MIGN`.
4. Supplier invoice posts `MPCH`.
5. Outgoing payment allocates to payable.
6. Cost is attributed to item, warehouse, cost center, and project where
   applicable.

### Inventory

1. Business partners use `MCRD`.
2. Items use `MITM`.
3. Warehouses use `MWHS`.
4. Item warehouse balance and policy use `MITW`.
5. Goods receipt uses `MIGN`.
6. Goods issue uses `MIGE`.
7. Transfers, adjustments, and counts are represented as warehouse documents
   with clear action semantics and balance effects.

### Finance

1. Chart of accounts uses `MACT`.
2. Cost centers use `MPRC`.
3. Journal entries use `MJDT/JDT1`.
4. A/R invoices use `MINV/INV1`.
5. A/P invoices use `MPCH/PCH1`.
6. Incoming payments use `MRCT/RCT1`.
7. Posting actions create journal entries or journal voucher records where the
   implementation needs auditable finance effects.

## ERP API Model

Generic records remain available:

- `GET /api/v1/erp/catalog`
- `GET /api/v1/erp/{TableCode}`
- `POST /api/v1/erp/{TableCode}`
- `GET /api/v1/erp/{TableCode}/{Key}`
- `PATCH /api/v1/erp/{TableCode}/{Key}`
- `DELETE /api/v1/erp/{TableCode}/{Key}`
- `GET /api/v1/erp/{TableCode}/{Key}/{ChildCode}`
- `POST /api/v1/erp/{TableCode}/{Key}/{ChildCode}`

Business actions are required for normal workflows:

- `POST /api/v1/erp/MREQ/{ReqCode}/actions/analyze`
- `POST /api/v1/erp/MREQ/{ReqCode}/actions/approve`
- `POST /api/v1/erp/MREQ/{ReqCode}/actions/convert-to-project`
- `POST /api/v1/erp/MPRJ/{PrjCode}/actions/add-member`
- `POST /api/v1/erp/MPRJ/{PrjCode}/actions/add-deliverable`
- `POST /api/v1/erp/MPRJ/{PrjCode}/actions/refresh-cost`
- `POST /api/v1/erp/MPRJ/{PrjCode}/actions/close-feedback`
- `POST /api/v1/erp/MPOR/{DocEntry}/actions/submit`
- `POST /api/v1/erp/MPOR/{DocEntry}/actions/approve`
- `POST /api/v1/erp/MPDN/{DocEntry}/actions/post`
- `POST /api/v1/erp/MRDR/{DocEntry}/actions/confirm`
- `POST /api/v1/erp/MRDR/{DocEntry}/actions/approve`
- `POST /api/v1/erp/MDLN/{DocEntry}/actions/post`
- `POST /api/v1/erp/MINV/{DocEntry}/actions/post`
- `POST /api/v1/erp/MRCT/{DocEntry}/actions/allocate`
- `POST /api/v1/erp/MIGE/{DocEntry}/actions/post`
- `POST /api/v1/erp/MIGN/{DocEntry}/actions/post`
- `POST /api/v1/erp/MJDT/{TransId}/actions/post`

Old semantic tenant routes such as `/projects`, `/requirements`,
`/procurement/orders`, `/sales/orders`, `/inventory/items`, and
`/finance/receivables` should not be reintroduced as public tenant APIs.

## Backend Architecture

The ERP domain must have three layers:

1. Catalog and record repository:
   table-code validation, quoted identifiers, master/child CRUD.
2. Business action service:
   workflow-specific state transitions and cross-table effects.
3. HTTP handler:
   generic record endpoints plus action endpoints.

Existing service logic in old domains is migration reference material. The
business rules should be ported into ERP actions so the system keeps behavior
without keeping old route names as the primary API.

Examples:

- Procurement `SubmitRequisition`, `ApproveOrder`, and `PostReceipt` become
  `MPOR` and `MPDN` actions.
- Sales `ConfirmOrder`, `ApproveOrder`, and `PostShipment` become `MRDR` and
  `MDLN` actions.
- Inventory `PostMovement` becomes `MIGN/MIGE` posting actions and `MITW`
  balance updates.
- Finance receivable/payable/payment/receipt allocation becomes `MINV`,
  `MPCH`, `MRCT`, and journal actions.
- Project requirement conversion, cost refresh, deliverables, and feedback
  become `MREQ`, `MPRJ`, `MDLN`, `MCST`, and `MFDB` actions.

## Frontend Architecture

The business UI must not use the generic JSON table editor as the main user
experience.

Required workspaces:

- Project and requirement workspace
- Procurement workspace
- Sales workspace
- Inventory workspace
- Finance workspace
- Cost and feedback workspace

These workspaces use the new ERP endpoints, show typed fields, and expose
business actions such as submit, approve, post, allocate, refresh cost, and
close feedback. The generic ERP table workspace remains available for platform
administrators or developers.

All visible UI strings must continue to use the existing bilingual i18n system.

## SaaS Industry Solution Standard Flow

The reconstruction process becomes a reusable SaaS platform administration
workflow for industry solution creation and adjustment.

The standard flow is:

1. Select or create an industry package.
2. Define enabled business modules.
3. Define standard database assets:
   master table code, child table code, primary key, required fields, indexes,
   status fields, and JSONB extension policy.
4. Define business functions:
   list, create, update, submit, approve, post, void, allocate, close, refresh.
5. Define business process loops:
   source table, target table, preconditions, postconditions, generated records,
   inventory effect, finance effect, and project effect.
6. Generate or update ERP catalog entries.
7. Generate or update API operation metadata.
8. Generate or update frontend workspace configuration.
9. Generate or update permissions.
10. Generate or update AI assistant context targets and skill templates.
11. Create a schema or solution change request.
12. Platform admin approves the request.
13. Apply the package to an organization.
14. Run smoke and automated verification for the generated flow.

This flow should live in SaaS platform management under the existing industry
solution and schema-change area. It should reuse the current industry package,
schema target, schema export, schema change request, approval, and apply
concepts instead of creating a separate administration subsystem.

## AI Assistant Integration

AI assistant functionality stays in place. It must be retargeted to ERP objects:

- Context targets should include ERP table codes and business object types.
- Assistant proposals should write to ERP actions or ERP records.
- Skill templates should use the industry package process definitions.
- Permission checks and approval queues remain unchanged.

## Error Handling and Validation

The action service must reject invalid transitions, such as:

- approving an already closed document
- posting an unapproved receipt or shipment
- posting inventory without item, warehouse, and quantity
- allocating payment beyond open amount
- converting a requirement that is not approved
- closing project feedback before deliverables or cost records are complete

Errors should return structured API errors with validation messages and no
partial cross-table writes.

## Testing and Verification

Required backend tests:

- Catalog includes all standard and project extension table codes.
- Generic ERP CRUD still works.
- Each business action validates state transitions.
- Procurement receipt post updates inventory and payable records.
- Sales delivery post updates inventory and receivable records.
- Requirement conversion creates a project record.
- Project cost refresh reads procurement, inventory, finance, and cost rows.
- Industry solution flow can generate a package/change request structure.

Required end-to-end verification:

- A fresh database can run the ERP baseline migration and expose the full ERP
  catalog.
- A smoke run can create a requirement, approve it, convert it into a project,
  execute purchasing, receive inventory, execute sales delivery, post invoices,
  allocate payment, refresh project cost, and close feedback.
- Every business action used by the smoke run must write at least one auditable
  ERP master or child record with a changed status or generated follow-on
  document.
- The API workbench must list generic ERP record APIs and ERP action APIs.
- The main frontend workspaces must operate with typed business forms and
  action buttons, not raw JSON-only table editing.
- The AI assistant context target API must list ERP business objects, and at
  least one assistant-created proposal must target an ERP action or ERP record.
- SaaS platform management must expose the industry solution creation or
  adjustment standard flow and must be able to create a change request package
  containing database assets, business functions, process steps, permissions,
  API operations, UI workspace metadata, and assistant context metadata.

Required frontend verification:

- Main workspaces call ERP endpoints, not old semantic tenant APIs.
- Business action buttons appear only in valid states.
- API workbench exposes ERP action operations.
- Bilingual strings exist for new modules and actions.

Required commands:

- `cd backend && go test ./...`
- `cd backend && go build ./cmd/server`
- `cd frontend && npm run lint`
- `cd frontend && npm run build`
- `git diff --check`

## Non-Goals

- Do not restore old semantic tenant API paths as the primary API surface.
- Do not change AI assistant or SaaS platform management behavior unrelated to
  ERP object integration.
- Do not migrate old production data; fresh database reset is acceptable for
  this reconstruction.
- Do not require normal business users to edit raw JSON payloads.

## Implementation Decomposition

This scope should be implemented in phases:

1. ERP action API foundation and action registry.
2. Project and requirement migration into `MREQ/MPRJ/MDLN/MCST/MFDB`.
3. Procurement and inventory posting flow.
4. Sales, delivery, invoicing, and payment flow.
5. Finance posting and allocation flow.
6. Frontend business workspaces restored on ERP endpoints.
7. SaaS industry solution standard flow integrated into platform management.
8. Assistant context and skill retargeting to ERP objects.
9. Full smoke and regression verification.
