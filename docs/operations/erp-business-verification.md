# ERP Business Verification

This checklist proves the ERP rebuild is not only table CRUD.

1. Create `MREQ`, run `analyze`, `approve`, and `convert-to-project`, then verify `MPRJ` exists.
2. Create `MPOR` and `POR1`, run `submit` and `approve`.
3. Create `MPDN` and `PDN1`, run `post`, then verify generated `MIGN`, `IGN1`, `MPCH`, `PCH1`, and updated `MITW`.
4. Create `MRDR` and `RDR1`, run `confirm` and `approve`.
5. Create `MDLN` and `DLN1`, run `post`, then verify generated `MIGE`, `IGE1`, `MINV`, `INV1`, and reduced `MITW`.
6. Run `MINV.post`, then verify generated `MJDT` uses `TransId`.
7. Create `MRCT`, run `allocate`, then verify invoice `PaidToDate`, invoice `DocStatus`, and payment `OpenBal`.
8. Run `MPRJ.refresh-cost`, then verify `MCST`.
9. Run `MPRJ.close-feedback`, then verify `MFDB`.
10. Open API workbench and verify ERP action APIs are assistant eligible.
11. Open project, procurement, sales, inventory, and finance workspaces and verify they show ERP module, submodule, business document, child row, and action controls.
12. Query assistant context targets with `module_key=erp` and verify ERP business objects are available.
13. Confirm an assistant proposal with ERP payload `table_code`, `key`, and `action`, then verify writeback under `"Payload".assistant_confirmed_proposals`.
14. Create the SaaS platform ERP standard industry solution flow and verify the change package includes database assets, business functions, process loops, permissions, API operations, UI workspaces, and assistant targets.
15. Create the SaaS platform retail distribution solution flow and verify the generated package is code-table-only.
16. In the demo tenant, open the Retail menu and verify the following documents are available under the main menu without using the removed middle business tree: branches, terminals, members, promotions, publishing, POS sale, distribution request, distribution shipment, distribution receipt, distribution difference, stock policy, store count, and special purchase request.
17. Run the replenishment loop: create `MSTP/STP1`, run `replenish`, verify generated `MDRQ/DRQ1`, run `submit`, `approve`, and `auto-allocate`, then verify generated `MDSP/DSP1`.
18. Run the distribution loop: seed `MITW` for HQ inventory, run `MDSP.ship`, verify `MIGE/IGE1` and `MDRC/DRC1`, then run `MDRC.receive` and verify `MIGN/IGN1` and receiving warehouse `MITW`.
19. Run the POS loop: create `MRPS/RPS1`, run `close`, then verify generated `MIGE/IGE1`, `MINV/INV1`, and `MRCT/RCT1`.
20. Run the count loop: create `MCNT/CNT1`, run `submit`, `approve`, and `post-adjustment`, then verify `MITW` adjustment plus `MIGN/IGN1` or `MIGE/IGE1` according to the count variance.
21. Run the special procurement loop: create `MSPR/SPR1`, run `submit`, `approve`, and `convert-to-purchase-order`, then verify generated `MPOR/POR1`.

Fresh tenant verification must check ERP code-table tables such as `MPOR`, `MRDR`, `MITW`, `MRPS`, `MDRQ`, and `MCNT`. It must not rely on old semantic supply-chain tables such as `inventory_counts`, `inventory_transfers`, `purchase_orders`, `sales_shipments`, `inventory_balances`, or `sales_orders`.
