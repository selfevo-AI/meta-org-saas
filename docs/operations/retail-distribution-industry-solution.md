# Retail Distribution Industry Solution

This document records the current retail distribution solution baseline after
the split-database and ERP code-table unification work.

## Scope

The retail solution supports a self-evolving organization where human operators
use a complete UI for query, approval, creation, update, and deletion, while
authorized internal or external agents use the same business capability through
API operations and ERP action execution.

The tenant workbench uses ERP code-table master/detail documents as the primary
runtime model. The old semantic supply-chain physical tables are not part of
fresh tenant databases.

## Code-Table Documents

- Base retail data: `MBRN/BRN1`, `MTER/TER1`, `MMBR/MBR1`, `MPRM/PRM1`,
  `MPUB/PUB1`
- POS sale: `MRPS/RPS1`
- Distribution request: `MDRQ/DRQ1`
- Distribution shipment: `MDSP/DSP1`
- Distribution receipt: `MDRC/DRC1`
- Distribution difference: `MDIF/DIF1`
- Store stock policy: `MSTP/STP1`
- Store inventory count: `MCNT/CNT1`
- Special purchase request: `MSPR/SPR1`
- Shared inventory and finance side effects: `MITW/ITW1`, `MIGE/IGE1`,
  `MIGN/IGN1`, `MINV/INV1`, `MRCT/RCT1`, `MPOR/POR1`

## Closed Loops

1. Replenishment to distribution:
   `MSTP.replenish` creates `MDRQ/DRQ1`. `MDRQ.submit`, `MDRQ.approve`, and
   `MDRQ.auto-allocate` create `MDSP/DSP1`.
2. Distribution execution:
   `MDSP.ship` reduces source `MITW`, creates `MIGE/IGE1`, and creates
   `MDRC/DRC1`. `MDRC.receive` increases target `MITW` and creates
   `MIGN/IGN1`. Quantity variance creates or links `MDIF/DIF1`.
3. POS to cash:
   `MRPS.close` reduces store `MITW`, creates `MIGE/IGE1`, creates
   `MINV/INV1`, and creates `MRCT/RCT1`.
4. Store count to adjustment:
   `MCNT.submit` and `MCNT.approve` prepare the count. `MCNT.post-adjustment`
   updates `MITW` and creates `MIGN/IGN1` or `MIGE/IGE1` for positive or
   negative variance.
5. Special procurement:
   `MSPR.submit` and `MSPR.approve` prepare a store-originated purchase need.
   `MSPR.convert-to-purchase-order` creates `MPOR/POR1`.

## SaaS Management

The SaaS management console exposes industry package operations for human
operators:

- List packages by industry.
- Update package name, description, status, assets, and metadata.
- Delete draft packages without adoptions or archive packages that have runtime
  adoption/history.
- Create the retail distribution solution flow from
  `/platform/admin/organizations/{id}/industry-solution-flows/retail-distribution`.
- Apply a selected package to a tenant organization.

The same functions are available to authorized agents through the platform API.

## Demo Tenant Verification

Use the `demo` tenant after it has been provisioned through the tenant database
provisioner and migrated with `tenantdb:include` expansion.

1. Confirm the physical tenant database is named with the `meta_org_xxxx` rule.
2. Confirm the tenant database has `MREG`, `MITW`, `MPOR`, `MRDR`, `MRPS`,
   `MDRQ`, `MDSP`, `MDRC`, `MCNT`, and `MSPR`.
3. Apply or create the retail distribution package from SaaS management.
4. Open the tenant Retail menu. All retail documents must be under the main
   menu; the middle business tree remains disabled.
5. Run the five closed loops above through the UI or equivalent ERP action API.
6. Verify generated records and `MITW` balances after each action.

Do not validate the retail solution by restoring old semantic tables such as
`inventory_counts`, `inventory_transfers`, `purchase_orders`,
`sales_shipments`, `inventory_balances`, or `sales_orders` as active runtime
tables.
