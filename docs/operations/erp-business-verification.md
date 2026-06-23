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
