 数据库表结构参考文档

模块	包含的主表 → 子表
💰 财务模块	MACT→AACT、MJDT→JDT1、MBTF→BTF1、MBTD、MPRC→APRC
🤝 业务伙伴	MCRD→CRD1
📦 产品/物料	MITM→ITM1、MITW→ITW1、MPRJ→APRJ
🛒 销售模块	MINV→INV1、MRIN→RIN1、MDLN→DLN1、MRDN→RDN1、MRDR→RDR1、MQUT→QUT1、MRCT→RCT1、MDPS→DPS1
📋 采购模块	MPCH→PCH1、MRPC→RPC1、MPDN→PDN1、MRPD→RPD1、MPOR→POR1
🏭 库存/仓库	MWHS→AWHS、MIGN→IGN1、MIGE→IGE1
👤 用户权限	MUSR→AUSR
每个表都列出了完整的字段列表（字段名、数据类型、大小、主键标识、描述）以及索引信息，主表和其关联的子表放在一起展示，方便对照查阅。
## 📑 目录

- [💰 财务模块](#fiscal)
- [🤝 业务伙伴模块](#partner)
- [📦 产品/物料模块](#product)
- [🛒 销售模块](#sale)
- [📋 采购模块](#purchase)
- [🏭 库存/仓库模块](#warehouse)
- [👤 用户与权限模块](#user)


## 💰 财务模块

### MACT — G/L Accounts

- **字段总数**: 44
- **关联子表**: AACT (G/L Account - History, 43 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| Finanse | VarChar | 1 |  | Cash Account |
| Budget | VarChar | 1 |  | Budget |
| Frozen | VarChar | 1 |  | Account on Hold [Y/N] |
| Postable | VarChar | 1 |  | Account [Active/Title] |
| CashBox | VarChar | 1 |  | Capital Account [Y/N] |
| RateTrans | VarChar | 1 |  | For Conversion Differences |
| TaxIncome | VarChar | 1 |  | Tax Definition |
| ExmIncome | VarChar | 1 |  | Tax Definition |
| ActType | VarChar | 1 |  | Account Type |
| Transfered | VarChar | 1 |  | Year Transfer [Y/N] |
| BlncTrnsfr | VarChar | 1 |  | Balances Transferred [Y/N] |
| OverType | VarChar | 1 |  | Loading Type |
| PrevYear | VarChar | 1 |  | There are accounts from the previous year |
| Protected | VarChar | 1 |  | Confidential Account |
| RealAcct | VarChar | 1 |  | Indexed Account |
| Advance | VarChar | 1 |  | Advance Payments |
| RevalMatch | VarChar | 1 |  | Revaluation Coordinated |
| DataSource | VarChar | 1 |  | Data Source |
| LocMth | VarChar | 1 |  | LC Reconciliation |
| LocManTran | VarChar | 1 |  | Control Account |
| ValidFor | VarChar | 1 |  | Active |
| FrozenFor | VarChar | 1 |  | Inactive |
| CfwRlvnt | VarChar | 1 |  | Cash Flow Relevant [Y/N] |
| ExchRate | VarChar | 1 |  | Exchange Rate Differences |
| VatChange | VarChar | 1 |  | Allow Change VAT Group |
| TaxPostAcc | VarChar | 1 |  | Default Tax Posting Account |
| BalDirect | nVarChar | 4 |  | Direction of Balance |
| MultiLink | VarChar | 1 |  | Allow Multiple Linking |
| PrjRelvnt | VarChar | 1 |  | Project Relevant |
| Dim1Relvnt | VarChar | 1 |  | Dimension 1 Relevant |
| Dim2Relvnt | VarChar | 1 |  | Dimension 2 Relevant |
| Dim3Relvnt | VarChar | 1 |  | Dimension 3 Relevant |
| Dim4Relvnt | VarChar | 1 |  | Dimension 4 Relevant |
| Dim5Relvnt | VarChar | 1 |  | Dimension 5 Relevant |
| AccrualTyp | VarChar | 1 |  | Accrual Type |
| DatevAutoA | VarChar | 1 |  | DATEV Automatic Account |
| DatevFirst | VarChar | 1 |  | First Data Entry |
| PCN874Rpt | VarChar | 1 |  | PCN 874 Report Relevant |
| SCAdjust | VarChar | 1 |  | SC Adjustment |
| BlocManPos | VarChar | 1 |  | Block Manual Posting |
| CstAccOnly | VarChar | 1 |  | Cost Account Only |
| BalanceA | VarChar | 1 |  | Account Balance Allowed |
| CemRelvnt | VarChar | 1 |  | Cost Element Relevant |
| EUBPRelvnt | VarChar | 1 |  | euBP-Relevant |

**索引：**
- PRIMARY (AcctCode) 🔑 UNIQUE
- INTER_KEY (FatherNum)
- CURRENCY (ActCurr)
- FORMAT (FormatCode)
- COUNTER (Counter)
- IDENTIFIER (ActId) 🔑 UNIQUE

### MBTD — Journal Vouchers List

- **字段总数**: 1

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| Status | VarChar | 1 |  | Open/Closed Document |

**索引：**
- PRIMARY (BatchNum) 🔑 UNIQUE
- STATUS (Status)

### MBTF — Journal Voucher Entry

- **字段总数**: 25
- **关联子表**: BTF1 (Journal Voucher - Rows, 12 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| BtfStatus | VarChar | 1 |  | Status |
| TransType | nVarChar | 20 |  | Origin |
| PCAddition | VarChar | 1 |  | PC Addition |
| DataSource | VarChar | 1 |  | Data Source |
| RefndRprt | VarChar | 1 |  | Repayment Report |
| AdjTran | VarChar | 1 |  | Adjusting Transaction |
| RevSource | VarChar | 1 |  | Revaluation Source |
| AutoStorno | VarChar | 1 |  | Use Auto-Reverse |
| Corisptivi | VarChar | 1 |  | Transaction Values |
| StampTax | VarChar | 1 |  | Stamp Tax |
| AutoVAT | VarChar | 1 |  | Automatic Tax |
| BlockDunn | VarChar | 1 |  | Block Dunning Letter |
| ReportEU | VarChar | 1 |  | Include in EU Report |
| Report347 | VarChar | 1 |  | Include in 347 Report |
| Printed | VarChar | 1 |  | Printed |
| GenRegNo | VarChar | 1 |  | Generate Reg. No. or Not |
| AutoWT | VarChar | 1 |  | Automatic WTax |
| ResidenNum | VarChar | 1 |  | Residence Number |
| DeferedTax | VarChar | 1 |  | Deferred Tax |
| ECDPosTyp | VarChar | 1 |  | ECD Posting Type |
| PrlLinked | VarChar | 1 |  | Is JE linked by MX Payroll |
| ExclTaxRep | VarChar | 1 |  | Exclude from Control Statement |
| IsCoEntry | VarChar | 1 |  | Cost Center Transfer form |
| EBookable | VarChar | 1 |  | E-Books Enabled |
| SAFTType | VarChar | 1 |  | SAF-T Transaction Type |

**索引：**
- PRIMARY (BatchNum) 🔑 UNIQUE
- Â (TransId)
- TRANS_TYPE (TransType)
- Â (CreatedBy)
- JDT_NUM (TransId)
- BTF_STATUS (BtfStatus)

### MJDT — Journal Entry

- **字段总数**: 25
- **关联子表**: JDT1 (Journal Entry - Rows, 13 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| BtfStatus | VarChar | 1 |  | Status |
| TransType | nVarChar | 20 |  | Origin |
| PCAddition | VarChar | 1 |  | PC Addition |
| DataSource | VarChar | 1 |  | Data Source |
| RefndRprt | VarChar | 1 |  | Repayment Report |
| AdjTran | VarChar | 1 |  | Adjusting Transaction |
| RevSource | VarChar | 1 |  | Revaluation Source |
| AutoStorno | VarChar | 1 |  | Use Auto-Reverse |
| Corisptivi | VarChar | 1 |  | Transaction Values |
| StampTax | VarChar | 1 |  | Stamp Tax |
| AutoVAT | VarChar | 1 |  | Automatic Tax |
| BlockDunn | VarChar | 1 |  | Block Dunning Letter |
| ReportEU | VarChar | 1 |  | Include in EU Report |
| Report347 | VarChar | 1 |  | Include in 347 Report |
| Printed | VarChar | 1 |  | Printed |
| GenRegNo | VarChar | 1 |  | Generate Reg. No. or Not |
| AutoWT | VarChar | 1 |  | Automatic WTax |
| ResidenNum | VarChar | 1 |  | Residence Number |
| DeferedTax | VarChar | 1 |  | Deferred Tax |
| ECDPosTyp | VarChar | 1 |  | ECD Posting Type |
| PrlLinked | VarChar | 1 |  | Is JE linked by MX Payroll |
| ExclTaxRep | VarChar | 1 |  | Exclude from Control Statement |
| IsCoEntry | VarChar | 1 |  | Cost Center Transfer form |
| EBookable | VarChar | 1 |  | E-Books Enabled |
| SAFTType | VarChar | 1 |  | SAF-T Transaction Type |

**索引：**
- PRIMARY (TransId) 🔑 UNIQUE
- TRANS_TYPE (TransType)
- Â (CreatedBy)
- REFDATE (RefDate)
- STORNO_TRA (StornoToTr)
- SERIES (Series) 🔑 UNIQUE
- Â (Number)
- STORNO (StornoDate)
- Â (AutoStorno)
- PROJECT (Project)

### MPRC — Cost Center

- **字段总数**: 3
- **关联子表**: APRC (Cost Center, 3 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| Locked | VarChar | 1 |  | Locked |
| DataSource | VarChar | 1 |  | Data Source |
| Active | VarChar | 1 |  | Active |

**索引：**
- PRIMARY (PrcCode) 🔑 UNIQUE

---

<a name="partner"></a>
## 🤝 业务伙伴模块

### MCRD — Business Partners

- **字段总数**: 132
- **关联子表**: CRD1 (Business Partners - Addresses, 4 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| CardType | VarChar | 1 |  | BP Type |
| CmpPrivate | VarChar | 1 |  | Type of Business |
| VatStatus | VarChar | 1 |  | Tax Definition |
| DdctStatus | VarChar | 1 |  | Liable for Ded. at Source |
| Transfered | VarChar | 1 |  | Year Transfer |
| BalTrnsfrd | VarChar | 1 |  | Balances transferred |
| PrevYearAc | VarChar | 1 |  | Previous Year Balance |
| Protected | VarChar | 1 |  | Protected BP |
| FatherType | VarChar | 1 |  | Parent Summary Type |
| QryGroup1 | VarChar | 1 |  | Property 1 |
| QryGroup2 | VarChar | 1 |  | Property 2 |
| QryGroup3 | VarChar | 1 |  | Property 3 |
| QryGroup4 | VarChar | 1 |  | Property 4 |
| QryGroup5 | VarChar | 1 |  | Property 5 |
| QryGroup6 | VarChar | 1 |  | Property 6 |
| QryGroup7 | VarChar | 1 |  | Property 7 |
| QryGroup8 | VarChar | 1 |  | Property 8 |
| QryGroup9 | VarChar | 1 |  | Property 9 |
| QryGroup10 | VarChar | 1 |  | Property 10 |
| QryGroup11 | VarChar | 1 |  | Property 11 |
| QryGroup12 | VarChar | 1 |  | Property 12 |
| QryGroup13 | VarChar | 1 |  | Property 13 |
| QryGroup14 | VarChar | 1 |  | Property 14 |
| QryGroup15 | VarChar | 1 |  | Property 15 |
| QryGroup16 | VarChar | 1 |  | Property 16 |
| QryGroup17 | VarChar | 1 |  | Property 17 |
| QryGroup18 | VarChar | 1 |  | Property 18 |
| QryGroup19 | VarChar | 1 |  | Property 19 |
| QryGroup20 | VarChar | 1 |  | Property 20 |
| QryGroup21 | VarChar | 1 |  | Property 21 |
| QryGroup22 | VarChar | 1 |  | Property 22 |
| QryGroup23 | VarChar | 1 |  | Property 23 |
| QryGroup24 | VarChar | 1 |  | Property 24 |
| QryGroup25 | VarChar | 1 |  | Property 25 |
| QryGroup26 | VarChar | 1 |  | Property 26 |
| QryGroup27 | VarChar | 1 |  | Property 27 |
| QryGroup28 | VarChar | 1 |  | Property 28 |
| QryGroup29 | VarChar | 1 |  | Property 29 |
| QryGroup30 | VarChar | 1 |  | Property 30 |
| QryGroup31 | VarChar | 1 |  | Property 31 |
| QryGroup32 | VarChar | 1 |  | Property 32 |
| QryGroup33 | VarChar | 1 |  | Property 33 |
| QryGroup34 | VarChar | 1 |  | Property 34 |
| QryGroup35 | VarChar | 1 |  | Property 35 |
| QryGroup36 | VarChar | 1 |  | Property 36 |
| QryGroup37 | VarChar | 1 |  | Property 37 |
| QryGroup38 | VarChar | 1 |  | Property 38 |
| QryGroup39 | VarChar | 1 |  | Property 39 |
| QryGroup40 | VarChar | 1 |  | Property 40 |
| QryGroup41 | VarChar | 1 |  | Property 41 |
| QryGroup42 | VarChar | 1 |  | Property 42 |
| QryGroup43 | VarChar | 1 |  | Property 43 |
| QryGroup44 | VarChar | 1 |  | Property 44 |
| QryGroup45 | VarChar | 1 |  | Property 45 |
| QryGroup46 | VarChar | 1 |  | Property 46 |
| QryGroup47 | VarChar | 1 |  | Property 47 |
| QryGroup48 | VarChar | 1 |  | Property 48 |
| QryGroup49 | VarChar | 1 |  | Property 49 |
| QryGroup50 | VarChar | 1 |  | Property 50 |
| QryGroup51 | VarChar | 1 |  | Property 51 |
| QryGroup52 | VarChar | 1 |  | Property 52 |
| QryGroup53 | VarChar | 1 |  | Property 53 |
| QryGroup54 | VarChar | 1 |  | Property 54 |
| QryGroup55 | VarChar | 1 |  | Property 55 |
| QryGroup56 | VarChar | 1 |  | Property 56 |
| QryGroup57 | VarChar | 1 |  | Property 57 |
| QryGroup58 | VarChar | 1 |  | Property 58 |
| QryGroup59 | VarChar | 1 |  | Property 59 |
| QryGroup60 | VarChar | 1 |  | Property 60 |
| QryGroup61 | VarChar | 1 |  | Property 61 |
| QryGroup62 | VarChar | 1 |  | Property 62 |
| QryGroup63 | VarChar | 1 |  | Property 63 |
| QryGroup64 | VarChar | 1 |  | Property 64 |
| DscntObjct | Int | 6 |  | Object for Discounts |
| DscntRel | VarChar | 1 |  | Discounts Ratio |
| DataSource | VarChar | 1 |  | Data Source |
| LocMth | VarChar | 1 |  | LC Reconciliation |
| validFor | VarChar | 1 |  | Active |
| frozenFor | VarChar | 1 |  | Inactive |
| sEmployed | VarChar | 1 |  | Self-Employed |
| chainStore | VarChar | 1 |  | Belongs to Retail Store |
| DiscInRet | VarChar | 1 |  | Allow Doc Discount in Returns |
| Deleted | VarChar | 1 |  | Deleted |
| BackOrder | VarChar | 1 |  | Backorder |
| PartDelivr | VarChar | 1 |  | Partial Delivery |
| BlockDunn | VarChar | 1 |  | Block Dunning |
| CollecAuth | VarChar | 1 |  | Collection Authorization |
| SinglePaym | VarChar | 1 |  | Single Payment |
| PaymBlock | VarChar | 1 |  | Payment Block |
| AccCritria | VarChar | 1 |  | Accrual |
| Equ | VarChar | 1 |  | Equalization Tax |
| TypWTReprt | VarChar | 1 |  | BP Type for WTax Report |
| IsDomestic | VarChar | 1 |  | Is Domestic |
| IsResident | VarChar | 1 |  | Is Resident |
| AutoCalBCG | VarChar | 1 |  | Auto Calculated Bank Charges |
| UseShpdGd | VarChar | 1 |  | Use Shipped Goods Account |
| InsurOp347 | VarChar | 1 |  | 347 Insurance Operation |
| TaxRndRule | VarChar | 1 |  | Tax Rounding Rule |
| ThreshOver | VarChar | 1 |  | Threshold Overlook |
| SurOver | VarChar | 1 |  | Surcharge Overlook |
| ResidenNum | VarChar | 1 |  | Residence Number |
| Affiliate | VarChar | 1 |  | Affiliate |
| MivzExpSts | VarChar | 1 |  | Mivzak Export Status |
| HierchDdct | VarChar | 1 |  | Hierarchical Deduction |
| CertWHT | VarChar | 1 |  | Withholding Tax Certified |
| CertBKeep | VarChar | 1 |  | Bookkeeping Certified |
| WHShaamGrp | VarChar | 1 |  | Withholding Shaam Group |
| DatevFirst | VarChar | 1 |  | First Data Entry |
| AutoPost | VarChar | 1 |  | Automatic Posting |
| TaxIdIdent | VarChar | 1 |  | Tax ID Category |
| DiscRel | VarChar | 1 |  | Disc. Relations |
| NoDiscount | VarChar | 1 |  | No Discounts |
| SCAdjust | VarChar | 1 |  | SC Adjustment |
| SefazCheck | VarChar | 1 |  | Check BP Status on SEFAZ |
| TypeOfOp | VarChar | 1 |  | Type of Operation |
| BlockComm | VarChar | 1 |  | Block Sending Marketing |
| EdrsFromBP | VarChar | 1 |  | Endorsable Checks from This BP |
| EdrsToBP | VarChar | 1 |  | This BP Accepts Endorsed Checks |
| EffecPrice | VarChar | 1 |  | Effective Price |
| TxExMxVdTp | VarChar | 1 |  | Exemption Max validate type |
| UseBilAddr | VarChar | 1 |  | Determine GST by Using Bill to |
| NaturalPer | VarChar | 1 |  | Natural Person |
| DPPStatus | VarChar | 1 |  | Data Protection Status |
| EnERD4In | VarChar | 1 |  | Enable ERD for Incoming Payments |
| EnERD4Out | VarChar | 1 |  | Enable ERD for Outgoing Payments |
| DflCustomr | VarChar | 1 |  | Default Customer |
| FCERelevnt | VarChar | 1 |  | FCE Relevant |
| FCEVldte | VarChar | 1 |  | FCE Validate Base Delivery |
| AggregDoc | VarChar | 1 |  | Aggregate Document |
| EffcAllSrc | VarChar | 1 |  | Considers All Price Sources |
| FCEPmnMean | VarChar | 1 |  | Use FCEs as Payment Means |
| NotRel4MI | VarChar | 1 |  | Not Relevant for Monthly Invoice |

**索引：**
- PRIMARY (CardCode) 🔑 UNIQUE
- CARD_NAME (CardName)
- CARD_TYPE (CardType)
- FATHER (FatherCard)
- TERMS (GroupNum)
- CURRENCY (Currency)
- COM_GROUP (CommGrCode)
- PRICE_LIST (ListNum)
- PAY_ACCT (DebPayAcct)
- ABS_ENTRY (DocEntry) 🔑 UNIQUE
- OWNER_CODE (OwnerCode)

---

<a name="product"></a>
## 📦 产品/物料模块

### MITM — Items

- **字段总数**: 117
- **关联子表**: ITM1 (Items - Prices, 4 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| VATLiable | VarChar | 1 |  | Tax Liable |
| PrchseItem | VarChar | 1 |  | Purchasing Item |
| SellItem | VarChar | 1 |  | Sales Item |
| InvntItem | VarChar | 1 |  | Inventory Item |
| Canceled | VarChar | 1 |  | Canceled Item [Yes/No] |
| TrackSales | VarChar | 1 |  | Follow-Up [Yes/No] |
| FREE | VarChar | 1 |  | Free [Yes/No] |
| Transfered | VarChar | 1 |  | Year Transfer [Y/N] |
| BlncTrnsfr | VarChar | 1 |  | Balances transferred [Yes/No] |
| TreeType | VarChar | 1 |  | BOM Type |
| AssetItem | VarChar | 1 |  | Fixed Asset Indicator |
| WasCounted | VarChar | 1 |  | Counted |
| ManSerNum | VarChar | 1 |  | Serial No. Management |

| ManBtchNum | VarChar | 1 |  | Manage Batch No. [Yes/No] |
| ManOutOnly | VarChar | 1 |  | Manage SN Only on Exit |
| DataSource | VarChar | 1 |  | Data Source |
| validFor | VarChar | 1 |  | Active |
| frozenFor | VarChar | 1 |  | Inactive |
| BlockOut | VarChar | 1 |  | Force selection of serial no. |
| Deleted | VarChar | 1 |  | Deleted |
| GLMethod | VarChar | 1 |  | Set G/L Accounts By |
| TaxType | VarChar | 1 |  | Tax Type |
| WTLiable | VarChar | 1 |  | WTax Liable |
| ItemType | VarChar | 1 |  | Item Type |
| Phantom | VarChar | 1 |  | Phantom Item |
| MngMethod | VarChar | 1 |  | Management Method |
| PlaningSys | VarChar | 1 |  | Planning Method |
| PrcrmntMtd | VarChar | 1 |  | Procurement Method |
| IndirctTax | VarChar | 1 |  | Indirect Tax |
| ItemClass | VarChar | 1 |  | Service or Material |
| Excisable | VarChar | 1 |  | Excisable [Yes/No] |
| StatAsset | VarChar | 1 |  | Owned by Company |
| Cession | VarChar | 1 |  | Cession |
| DeacAftUL | VarChar | 1 |  | Deactivate After Useful Life |
| AsstStatus | VarChar | 1 |  | Asset Status |
| GLPickMeth | VarChar | 1 |  | G/L Account Pick Method |
| NoDiscount | VarChar | 1 |  | No Discounts |
| MgrByQty | VarChar | 1 |  | Manage Asset by Quantity |
| OneBOneRec | VarChar | 1 |  | One Batch One Receipt |
| CompoWH | VarChar | 1 |  | Component Warehouse |
| VirtAstItm | VarChar | 1 |  | Virtual Asset Item |
| InCostRoll | VarChar | 1 |  | Include in Prod. Cost Rollup |
| EnAstSeri | VarChar | 1 |  | Enforce Asset Serial Numbers |
| GSTRelevnt | VarChar | 1 |  | GST Relevant |
| GstTaxCtg | VarChar | 1 |  | GST Tax Category |
| SOIExc | VarChar | 1 |  | SOI Excisable |
| Imported | VarChar | 1 |  | Imported Item |
| AutoBatch | VarChar | 1 |  | Automatic Batches [Yes/No] |
| CstmActing | VarChar | 1 |  | Customer Accounting |
| Traceable | VarChar | 1 |  | Traceable |
| IfrsPsRev | VarChar | 1 |  | IFRS Posting for Revaluation |
| ProdctType | VarChar | 1 |  | SAF-T Product Type |
| NoApDisc | VarChar | 1 |  | Do Not Apply Discount |

**索引：**
- PRIMARY (ItemCode) 🔑 UNIQUE
- ITEM_NAME (ItemName)
- TREE_TYPE (TreeType)
- COM_GROUP (CommisGrp)
- SALE (SellItem)
- PURCHASE (PrchseItem)
- INVENTORY (InvntItem)

### MITW — Items - Warehouse

- **字段总数**: 5
- **关联子表**: ITW1 (Item Count Alert, 2 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| WasCounted | VarChar | 1 |  | Counted Yes/No |
| Locked | VarChar | 1 |  | Locked |
| DftBinEnfd | VarChar | 1 |  | Default Bin Enforced [Y/N] |
| Freezed | VarChar | 1 |  | Item Frozen in Warehouse |
| IndEscala | VarChar | 1 |  | Indicator for Relevant Scale |

**索引：**
- PRIMARY (ItemCode) 🔑 UNIQUE
- Â (WhsCode)
- WHS (WhsCode)
- DFT_BIN (DftBinAbs)

### MPRJ — Project Codes

- **字段总数**: 3
- **关联子表**: APRJ (Project Codes, 3 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| Locked | VarChar | 1 |  | Locked |
| DataSource | VarChar | 1 |  | Data Source |
| Active | VarChar | 1 |  | Active |

**索引：**
- PRIMARY (PrjCode) 🔑 UNIQUE

---

<a name="sale"></a>
## 🛒 销售模块

### MDLN — Delivery

- **字段总数**: 101
- **关联子表**: DLN1 (Delivery - Rows, 42 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| DocType | VarChar | 1 |  | Document Type |
| CANCELED | VarChar | 1 |  | Canceled |
| Handwrtten | VarChar | 1 |  | Manual Numbering |
| Printed | VarChar | 1 |  | Printed |
| DocStatus | VarChar | 1 |  | Document Status |
| InvntSttus | VarChar | 1 |  | Warehouse Status |
| Transfered | VarChar | 1 |  | Year Transfer |
| ObjType | nVarChar | 20 |  | Object Type |
| PartSupply | VarChar | 1 |  | Partial Delivery |
| Confirmed | VarChar | 1 |  | Confirmed |
| CreateTran | VarChar | 1 |  | Create Journal Entry |
| SummryType | VarChar | 1 |  | Summary Method |
| UpdInvnt | VarChar | 1 |  | Warehouse Update |
| UpdCardBal | VarChar | 1 |  | Update Balances |
| InvntDirec | VarChar | 1 |  | Warehouse Direction |
| ShowSCN | VarChar | 1 |  | Display BP Catalog Number |
| CurSource | VarChar | 1 |  | Base Currency |
| FatherType | VarChar | 1 |  | Parent Summary Type |
| IsICT | VarChar | 1 |  | A/R Invoice + Payment |
| DataSource | VarChar | 1 |  | Data Source |
| isCrin | VarChar | 1 |  | Corrected Invoice |
| selfInv | VarChar | 1 |  | Autom. Invoice |
| WddStatus | VarChar | 1 |  | Authorization Status |
| Exported | VarChar | 1 |  | Exported |
| NetProc | VarChar | 1 |  | Net Procedure |
| submitted | VarChar | 1 |  | Submitted |
| PoPrss | VarChar | 1 |  | PO Process |
| Rounding | VarChar | 1 |  | Rounding |
| RevisionPo | VarChar | 1 |  | Split PO |
| PickStatus | VarChar | 1 |  | Pick Status |
| Pick | VarChar | 1 |  | Pick |
| BlockDunn | VarChar | 1 |  | Block Dunning |
| PayBlock | VarChar | 1 |  | Payment Block |
| MaxDscn | VarChar | 1 |  | Maximum Discount |
| Reserve | VarChar | 1 |  | Reserve |
| BoeReserev | VarChar | 1 |  | Bill of Exchange Reserved |
| CEECFlag | VarChar | 1 |  | Block Creation of Target Corr. Inv. |
| UseShpdGd | VarChar | 1 |  | Use Shipped Goods Account |
| DocSubType | nVarChar | 2 |  | VAT Code for Tax Invoice Rpt |
| DpmStatus | VarChar | 1 |  | Summary VAT Abstract ID |
| DpmDrawn | VarChar | 1 |  | Drawn to Down Payment |
| Posted | VarChar | 1 |  | Down Payment Was Posted |
| isIns | VarChar | 1 |  | Reserve Invoice |
| BPNameOW | VarChar | 1 |  | BP_NAME_OVERWRITTEN |
| BillToOW | VarChar | 1 |  | BILL_TO_OVERWRITTEN |
| ShipToOW | VarChar | 1 |  | SHIP_TO_OVERWRITTEN |
| RetInvoice | VarChar | 1 |  | Credit Memo |
| UseCorrVat | VarChar | 1 |  | Use Correction VAT Group |
| BlkCredMmo | VarChar | 1 |  | Block Creating Credit Memo Tgt |
| OpenForLaC | VarChar | 1 |  | Open For Landed Costs |
| Excised | VarChar | 1 |  | Excised |
| DutyStatus | VarChar | 1 |  | Duty Status |
| AutoCrtFlw | VarChar | 1 |  | Auto Create Follow-up Document |
| InsurOp347 | VarChar | 1 |  | 347 Insurance Operation |
| IgnRelDoc | VarChar | 1 |  | Ignore Relevant Doc on Archive |
| ResidenNum | VarChar | 1 |  | Residence Number |
| PQTGrpHW | VarChar | 1 |  | Pur Quotation Group Manual |
| DocManClsd | VarChar | 1 |  | Document Was Closed Manually |
| Ordered | VarChar | 1 |  | Payment Ordered |
| NTSApprov | VarChar | 1 |  | NTS Approved |
| EDocGenTyp | VarChar | 1 |  | Electr. Doc. Generation Type |
| OnlineQuo | VarChar | 1 |  | Create Online Quotation |
| EDocStatus | VarChar | 1 |  | Electronic Document Status |
| EDocProces | VarChar | 1 |  | Electronic Document Process |
| EDocCancel | VarChar | 1 |  | Electronic Document - Canceled |
| EDocTest | VarChar | 1 |  | Electronic Document - Testing |
| DpmAsDscnt | VarChar | 1 |  | Discount Document with Dpm |
| GTSRlvnt | VarChar | 1 |  | Relevant To GTS |
| SrvTaxRule | VarChar | 1 |  | Apply Service Tax Rule |
| ReqType | Int | 11 |  | Requester Type User/Employee |
| OriginType | VarChar | 1 |  | Document Origin |
| IsReuseNum | VarChar | 1 |  | Is Reusing Document Number |
| IsReuseNFN | VarChar | 1 |  | Is Reusing Nota Fiscal Number |
| IsAlt | VarChar | 1 |  | Is Alteration |
| AltBaseTyp | Int | 11 |  | Alteration Base Type |
| PrintSEPA | VarChar | 1 |  | Print SEPA Direct Debit Prenotification |
| RelatedTyp | Int | 11 |  | Related Type |
| NfePrntFo | Int | 11 |  | NF-e Printing Format |
| InterimTyp | Int | 6 |  | Interim Type |
| PoDropPrss | VarChar | 1 |  | PO Drop-Ship Process |
| ExclTaxRep | VarChar | 1 |  | Exclude from Control Statement |
| Revision | VarChar | 1 |  | Revision |
| BaseType | Int | 11 |  | Base Document Type |
| ComTrade | VarChar | 1 |  | Commission Trade |
| IssReason | Int | 6 |  | Reason for issuing note |
| ComTradeRt | VarChar | 1 |  | Commission Trade Return |
| SplitPmnt | VarChar | 1 |  | A/P Split Payment |
| SelfPosted | VarChar | 1 |  | Self Invoice Created [Yes/No] |
| DPPStatus | VarChar | 1 |  | Data Protection Status |
| EDocType | VarChar | 1 |  | eDoc Type |
| AggregDoc | VarChar | 1 |  | Aggregate Document |
| IndFinal | VarChar | 1 |  | Indicator for Final Consumer |
| PostPmntWT | VarChar | 1 |  | Post Payment Withholding Tax |
| FCEPmnMean | VarChar | 1 |  | Use FCEs as Payment Means |
| NotRel4MI | VarChar | 1 |  | Not Relevant for Monthly Invoice |
| Rel4PPTax | VarChar | 1 |  | Relevant for Plastic Packaging Tax |
| BookeTdsBP | VarChar | 1 |  | Book the eTDS to the Business Partner |
| DigPayment | VarChar | 1 |  | Digital Payments |
| RShipToOW | VarChar | 1 |  | Ship-to Address Overwritten for Return |
| AplTaxOnFr | VarChar | 1 |  | Apply Only Taxes in First Installment |
| CpyDtyStts | VarChar | 1 |  | Copy Duty Status from Base |

**索引：**
- PRIMARY (DocEntry) 🔑 UNIQUE
- AT_CARD (NumAtCard)

- CUSTOMER (CardCode)
- NUM (DocNum)

- DOC_STATUS (DocStatus)

- FTHR_CARD (FatherCard)

- SERIES (Series)
- OWNER_CODE (OwnerCode)
- DATE_PIND (DocDate)

- ESERIES (ESeries)

- FOL_SERIES (FolSeries)
- PROJECT (Project)

### MDPS — Deposit

- **字段总数**: 9
- **关联子表**: DPS1 (Deposit - Rows, 1 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| DeposType | VarChar | 1 |  | Payment Type |
| ChkType | VarChar | 1 |  | Check Deposit Type |
| Printed | VarChar | 1 |  | Original/Copy |
| IsCard | VarChar | 1 |  | Business Partner or Account |
| Transfered | VarChar | 1 |  | Year Transfer |
| Splited | VarChar | 1 |  | Split |
| DataSource | VarChar | 1 |  | Data Source |
| PostType | VarChar | 1 |  | Transaction Type |
| Canceled | VarChar | 1 |  | Canceled |

**索引：**
- PRIMARY (DeposId) 🔑 UNIQUE
- VIS_NUM (DeposNum) 🔑 UNIQUE

- SERIES (Series)

### MINV — A/R Invoice

- **字段总数**: 100
- **关联子表**: INV1 (A/R Invoice - Rows, 42 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| DocType | VarChar | 1 |  | Document Type |
| CANCELED | VarChar | 1 |  | Canceled |
| Handwrtten | VarChar | 1 |  | Manual Numbering |
| Printed | VarChar | 1 |  | Printed |
| DocStatus | VarChar | 1 |  | Document Status |
| InvntSttus | VarChar | 1 |  | Warehouse Status |
| Transfered | VarChar | 1 |  | Year Transfer |
| ObjType | nVarChar | 20 |  | Object Type |
| PartSupply | VarChar | 1 |  | Partial Delivery |
| Confirmed | VarChar | 1 |  | Confirmed |
| CreateTran | VarChar | 1 |  | Create Journal Entry |
| SummryType | VarChar | 1 |  | Summary Method |
| UpdInvnt | VarChar | 1 |  | Whse Update |
| UpdCardBal | VarChar | 1 |  | Update Balances |
| InvntDirec | VarChar | 1 |  | Warehouse Direction |
| ShowSCN | VarChar | 1 |  | Display BP Catalog Number |
| CurSource | VarChar | 1 |  | Base Currency |
| FatherType | VarChar | 1 |  | Parent Summary Type |
| IsICT | VarChar | 1 |  | A/R Invoice + Payment |
| DataSource | VarChar | 1 |  | Data Source |
| isCrin | VarChar | 1 |  | Correction Invoice |
| selfInv | VarChar | 1 |  | Autom. Invoice |
| WddStatus | VarChar | 1 |  | Authorization Status |
| Exported | VarChar | 1 |  | Exported |
| NetProc | VarChar | 1 |  | Net Procedure |
| submitted | VarChar | 1 |  | Submitted |
| PoPrss | VarChar | 1 |  | PO Process |
| Rounding | VarChar | 1 |  | Rounding |
| RevisionPo | VarChar | 1 |  | Split Purchase Order |
| PickStatus | VarChar | 1 |  | Pick Status |
| Pick | VarChar | 1 |  | Pick |
| BlockDunn | VarChar | 1 |  | Block Dunning |
| PayBlock | VarChar | 1 |  | Payment Block |
| MaxDscn | VarChar | 1 |  | Maximum Discount |
| Reserve | VarChar | 1 |  | Reserve |
| BoeReserev | VarChar | 1 |  | Bill of Exchange Reserved |
| CEECFlag | VarChar | 1 |  | Block Creation of Tgt Corr Doc |
| UseShpdGd | VarChar | 1 |  | Use Shipped Goods Account |
| DocSubType | nVarChar | 2 |  | Document Sub-Type |
| DpmStatus | VarChar | 1 |  | Summary VAT Abstract ID |
| DpmDrawn | VarChar | 1 |  | Drawn to Down Payment |
| Posted | VarChar | 1 |  | Down Payment Was Posted |
| isIns | VarChar | 1 |  | Reserve Invoice |
| BPNameOW | VarChar | 1 |  | BP Name Overwritten |
| BillToOW | VarChar | 1 |  | Bill-To Overwritten |
| ShipToOW | VarChar | 1 |  | Ship-to Overwritten |
| RetInvoice | VarChar | 1 |  | Credit Memo |
| UseCorrVat | VarChar | 1 |  | Use Correction VAT Group |
| BlkCredMmo | VarChar | 1 |  | Block Creation of Target Credit Memo |
| OpenForLaC | VarChar | 1 |  | Open For Landed Costs |
| Excised | VarChar | 1 |  | Excised |
| DutyStatus | VarChar | 1 |  | Duty Status |
| AutoCrtFlw | VarChar | 1 |  | Auto Create Follow-up Document |
| InsurOp347 | VarChar | 1 |  | 347 Insurance Operation |
| IgnRelDoc | VarChar | 1 |  | Ignore Relevant Doc on Archive |
| ResidenNum | VarChar | 1 |  | Residence Number |
| DocManClsd | VarChar | 1 |  | Document Was Closed Manually |
| Ordered | VarChar | 1 |  | Payment Ordered |
| NTSApprov | VarChar | 1 |  | NTS Approved |
| EDocGenTyp | VarChar | 1 |  | Electr. Doc. Generation Type |
| OnlineQuo | VarChar | 1 |  | Create Online Quotation |
| EDocStatus | VarChar | 1 |  | Electronic Document Status |
| EDocProces | VarChar | 1 |  | Electronic Document Process |
| EDocCancel | VarChar | 1 |  | Electronic Document - Canceled |
| EDocTest | VarChar | 1 |  | Electronic Document - Testing |
| DpmAsDscnt | VarChar | 1 |  | Discount Document with Dpm |
| GTSRlvnt | VarChar | 1 |  | Relevant To GTS |
| SrvTaxRule | VarChar | 1 |  | Apply Service Tax Rule |
| ReqType | Int | 11 |  | Requester Type User/Employee |
| OriginType | VarChar | 1 |  | Document Origin |
| IsReuseNum | VarChar | 1 |  | Is Reusing Document Number |
| IsReuseNFN | VarChar | 1 |  | Is Reusing Nota Fiscal Number |
| IsAlt | VarChar | 1 |  | Is Alteration |
| AltBaseTyp | Int | 11 |  | Alteration Base Type |
| PrintSEPA | VarChar | 1 |  | Print SEPA Direct Debit Prenotification |
| RelatedTyp | Int | 11 |  | Related Type |
| NfePrntFo | Int | 11 |  | NF-e Printing Format |
| InterimTyp | Int | 6 |  | Interim Type |
| PoDropPrss | VarChar | 1 |  | PO Drop-Ship Process |
| ExclTaxRep | VarChar | 1 |  | Exclude from Control Statement |
| Revision | VarChar | 1 |  | Revision |
| BaseType | Int | 11 |  | Base Document Type |
| ComTrade | VarChar | 1 |  | Commission Trade |
| IssReason | Int | 6 |  | Reason for issuing note |
| ComTradeRt | VarChar | 1 |  | Commission Trade Return |
| SplitPmnt | VarChar | 1 |  | A/P Split Payment |
| SelfPosted | VarChar | 1 |  | Self Invoice Created [Yes/No] |
| DPPStatus | VarChar | 1 |  | Data Protection Status |
| EDocType | VarChar | 1 |  | eDoc Type |
| AggregDoc | VarChar | 1 |  | Aggregate Document |
| IndFinal | VarChar | 1 |  | Indicator for Final Consumer |
| PostPmntWT | VarChar | 1 |  | Post Payment Withholding Tax |
| FCEPmnMean | VarChar | 1 |  | Use FCEs as Payment Means |
| NotRel4MI | VarChar | 1 |  | Not Relevant for Monthly Invoice |
| Rel4PPTax | VarChar | 1 |  | Relevant for Plastic Packaging Tax |
| BookeTdsBP | VarChar | 1 |  | Book the eTDS to the Business Partner |
| DigPayment | VarChar | 1 |  | Digital Payments |
| RShipToOW | VarChar | 1 |  | Ship-to Address Overwritten for Return |
| AplTaxOnFr | VarChar | 1 |  | Apply Only Taxes in First Installment |
| CpyDtyStts | VarChar | 1 |  | Copy Duty Status from Base |

**索引：**
- PRIMARY (DocEntry) 🔑 UNIQUE
- AT_CARD (NumAtCard)

- CUSTOMER (CardCode)
- NUM (DocNum)
-
- DOC_STATUS (DocStatus)

- FTHR_CARD (FatherCard)

- SERIES (Series)
- OWNER_CODE (OwnerCode)
- DATE_PIND (DocDate)
-
- ESERIES (ESeries)

- STS_CNCL (InvntSttus)

- FOL_SERIES (FolSeries)
- PROJECT (Project)

### MQUT — Sales Quotation

- **字段总数**: 101
- **关联子表**: QUT1 (Sales Quotation - Rows, 42 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| DocType | VarChar | 1 |  | Document Type |
| CANCELED | VarChar | 1 |  | Canceled |
| Handwrtten | VarChar | 1 |  | Manual Numbering |
| Printed | VarChar | 1 |  | Printed |
| DocStatus | VarChar | 1 |  | Document Status |
| InvntSttus | VarChar | 1 |  | Warehouse Status |
| Transfered | VarChar | 1 |  | Year Transfer |
| ObjType | nVarChar | 20 |  | Object Type |
| PartSupply | VarChar | 1 |  | Partial Delivery |
| Confirmed | VarChar | 1 |  | Confirmed |
| CreateTran | VarChar | 1 |  | Create Journal Entry |
| SummryType | VarChar | 1 |  | Summary Method |
| UpdInvnt | VarChar | 1 |  | Whse Update |
| UpdCardBal | VarChar | 1 |  | Update Balances |
| InvntDirec | VarChar | 1 |  | Warehouse Direction |
| ShowSCN | VarChar | 1 |  | Display BP Catalog Number |
| CurSource | VarChar | 1 |  | Base Currency |
| FatherType | VarChar | 1 |  | Parent Summary Type |
| IsICT | VarChar | 1 |  | A/R Invoice + Payment |
| DataSource | VarChar | 1 |  | Data Source |
| isCrin | VarChar | 1 |  | Corrected Invoice |
| selfInv | VarChar | 1 |  | Autom. Invoice |
| WddStatus | VarChar | 1 |  | Authorization Status |
| Exported | VarChar | 1 |  | Exported |
| NetProc | VarChar | 1 |  | Net Procedure |
| submitted | VarChar | 1 |  | Submitted |
| PoPrss | VarChar | 1 |  | PO Process |
| Rounding | VarChar | 1 |  | Rounding |
| RevisionPo | VarChar | 1 |  | Split Purchase Order |
| PickStatus | VarChar | 1 |  | Pick Status |
| Pick | VarChar | 1 |  | Pick |
| BlockDunn | VarChar | 1 |  | Block Dunning |
| PayBlock | VarChar | 1 |  | Payment Block |
| MaxDscn | VarChar | 1 |  | Maximum Discount |
| Reserve | VarChar | 1 |  | Reserve |
| BoeReserev | VarChar | 1 |  | Bill of Exchange Reserved |
| CEECFlag | VarChar | 1 |  | A/P Correction Invoice |
| UseShpdGd | VarChar | 1 |  | Use Shipped Goods Account |
| DocSubType | nVarChar | 2 |  | VAT Code for Tax Invoice Rpt |
| DpmStatus | VarChar | 1 |  | Summary VAT Abstract ID |
| DpmDrawn | VarChar | 1 |  | Drawn to Down Payment |
| Posted | VarChar | 1 |  | Down Payment Was Posted |
| isIns | VarChar | 1 |  | Reserve Invoice |
| BPNameOW | VarChar | 1 |  | BP Name Overwritten |
| BillToOW | VarChar | 1 |  | Bill-To Overwritten |
| ShipToOW | VarChar | 1 |  | Ship-to Overwritten |
| RetInvoice | VarChar | 1 |  | Returning Invoice |
| UseCorrVat | VarChar | 1 |  | Use Correction VAT Group |
| BlkCredMmo | VarChar | 1 |  | Block Creation of Target Credit Memo |
| OpenForLaC | VarChar | 1 |  | Open For Landed Costs |
| Excised | VarChar | 1 |  | Excised |
| DutyStatus | VarChar | 1 |  | Duty Status |
| AutoCrtFlw | VarChar | 1 |  | Auto Create Follow-up Document |
| InsurOp347 | VarChar | 1 |  | 347 Insurance Operation |
| IgnRelDoc | VarChar | 1 |  | Ignore Relevant Doc on Archive |
| ResidenNum | VarChar | 1 |  | Residence Number |
| PQTGrpHW | VarChar | 1 |  | Pur Quotation Group Manual |
| DocManClsd | VarChar | 1 |  | Document Was Closed Manually |
| Ordered | VarChar | 1 |  | Payment Ordered |
| NTSApprov | VarChar | 1 |  | NTS Approved |
| EDocGenTyp | VarChar | 1 |  | Electr. Doc. Generation Type |
| OnlineQuo | VarChar | 1 |  | Create Online Quotation |
| EDocStatus | VarChar | 1 |  | Electronic Document Status |
| EDocProces | VarChar | 1 |  | Electronic Document Process |
| EDocCancel | VarChar | 1 |  | Electronic Document - Canceled |
| EDocTest | VarChar | 1 |  | Electronic Document - Testing |
| DpmAsDscnt | VarChar | 1 |  | Discount Document with Dpm |
| GTSRlvnt | VarChar | 1 |  | Relevant To GTS |
| SrvTaxRule | VarChar | 1 |  | Apply Service Tax Rule |
| ReqType | Int | 11 |  | Requester Type User/Employee |
| OriginType | VarChar | 1 |  | Document Origin |
| IsReuseNum | VarChar | 1 |  | Is Reusing Document Number |
| IsReuseNFN | VarChar | 1 |  | Is Reusing Nota Fiscal Number |
| IsAlt | VarChar | 1 |  | Is Alteration |
| AltBaseTyp | Int | 11 |  | Alteration Base Type |
| PrintSEPA | VarChar | 1 |  | Print SEPA Direct Debit Prenotification |
| RelatedTyp | Int | 11 |  | Related Type |
| NfePrntFo | Int | 11 |  | NF-e Printing Format |
| InterimTyp | Int | 6 |  | Interim Type |
| PoDropPrss | VarChar | 1 |  | PO Drop-Ship Process |
| ExclTaxRep | VarChar | 1 |  | Exclude from Control Statement |
| Revision | VarChar | 1 |  | Revision |
| BaseType | Int | 11 |  | Base Document Type |
| ComTrade | VarChar | 1 |  | Commission Trade |
| IssReason | Int | 6 |  | Reason for issuing note |
| ComTradeRt | VarChar | 1 |  | Commission Trade Return |
| SplitPmnt | VarChar | 1 |  | A/P Split Payment |
| SelfPosted | VarChar | 1 |  | Self Invoice Created [Yes/No] |
| DPPStatus | VarChar | 1 |  | Data Protection Status |
| EDocType | VarChar | 1 |  | eDoc Type |
| AggregDoc | VarChar | 1 |  | Aggregate Document |
| IndFinal | VarChar | 1 |  | Indicator for Final Consumer |
| PostPmntWT | VarChar | 1 |  | Post Payment Withholding Tax |
| FCEPmnMean | VarChar | 1 |  | Use FCEs as Payment Means |
| NotRel4MI | VarChar | 1 |  | Not Relevant for Monthly Invoice |
| Rel4PPTax | VarChar | 1 |  | Relevant for Plastic Packaging Tax |
| BookeTdsBP | VarChar | 1 |  | Book the eTDS to the Business Partner |
| DigPayment | VarChar | 1 |  | Digital Payments |
| RShipToOW | VarChar | 1 |  | Ship-to Address Overwritten for Return |
| AplTaxOnFr | VarChar | 1 |  | Apply Only Taxes in First Installment |
| CpyDtyStts | VarChar | 1 |  | Copy Duty Status from Base |

**索引：**
- PRIMARY (DocEntry) 🔑 UNIQUE
- AT_CARD (NumAtCard)

- CUSTOMER (CardCode)
- NUM (DocNum) 🔑 UNIQUE
-
- DOC_STATUS (DocStatus)

- FTHR_CARD (FatherCard)

- SERIES (Series)
- OWNER_CODE (OwnerCode)
- DATE_PIND (DocDate)
- 
- ESERIES (ESeries)

- PROJECT (Project)

### MRCT — Incoming Payments

- **字段总数**: 31
- **关联子表**: RCT1 (Incoming Payment - Checks, 4 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| DocType | VarChar | 1 |  | Document Type |
| Canceled | VarChar | 1 |  | Canceled |
| Handwrtten | VarChar | 1 |  | Manual Numbering |
| Printed | VarChar | 1 |  | Printed |
| PayNoDoc | VarChar | 1 |  | Non-Calculated Payment |
| DiffCurr | VarChar | 1 |  | Enter in local currency |
| ShowAtCard | VarChar | 1 |  | Display Customer Ref. No. |
| SpiltTrans | VarChar | 1 |  | Split Transaction Journal |
| CreateTran | VarChar | 1 |  | Create Journal Entry |
| ObjType | nVarChar | 20 |  | Object Type |
| ApplyVAT | VarChar | 1 |  | Tax Definition |
| confirmed | VarChar | 1 |  | Approved |
| ShowJDT | VarChar | 1 |  | Display Journal Entries |
| DataSource | VarChar | 1 |  | Data Source |
| SpltCredLn | VarChar | 1 |  | Split Vendor Credit Row |
| Submitted | VarChar | 1 |  | Submitted |
| Status | VarChar | 1 |  | Created by Payment Run |
| Proforma | VarChar | 1 |  | Proforma |
| PaPriority | VarChar | 1 |  | Payment Priority |
| IsPaytoBnk | VarChar | 1 |  | Is Pay to Bank |
| WizDunBlck | VarChar | 1 |  | Wizard Dunning Block |
| PaymType | VarChar | 1 |  | Payment Type |
| WddStatus | VarChar | 1 |  | Authorization Status |
| ResidenNum | VarChar | 1 |  | Residence Number |
| ShowDocNo | VarChar | 1 |  | Display Document No. |
| BPLCentPmt | VarChar | 1 |  | Centralized Payment |
| PmntWTCert | VarChar | 1 |  | Payment by WT Certificate Only |
| DPPStatus | VarChar | 1 |  | Data Protection Status |
| EnblDpmTax | VarChar | 1 |  | Enable Tax Calculation in Down Payment Invoices |
| DigPayment | VarChar | 1 |  | Digital Payments |
| BaseType | Int | 11 |  | Base Document Type |

**索引：**
- PRIMARY (DocEntry) 🔑 UNIQUE
- NUM (DocNum) 🔑 UNIQUE

- CARD (CardCode)
- HANDWRITEN (Handwrtten)
- CANCELED (Canceled)
- SERIES (Series)

### MRDN — Returns

- **字段总数**: 101
- **关联子表**: RDN1 (Returns - Rows, 42 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| DocType | VarChar | 1 |  | Document Type |
| CANCELED | VarChar | 1 |  | Canceled |
| Handwrtten | VarChar | 1 |  | Manual Numbering |
| Printed | VarChar | 1 |  | Printed |
| DocStatus | VarChar | 1 |  | Document Status |
| InvntSttus | VarChar | 1 |  | Warehouse Status |
| Transfered | VarChar | 1 |  | Year Transfer |
| ObjType | nVarChar | 20 |  | Object Type |
| PartSupply | VarChar | 1 |  | Partial Delivery |
| Confirmed | VarChar | 1 |  | Confirmed |
| CreateTran | VarChar | 1 |  | Create Journal Entry |
| SummryType | VarChar | 1 |  | Summary Method |
| UpdInvnt | VarChar | 1 |  | Warehouse Update |
| UpdCardBal | VarChar | 1 |  | Update Balances |
| InvntDirec | VarChar | 1 |  | Warehouse Direction |
| ShowSCN | VarChar | 1 |  | Display BP Catalog Number |
| CurSource | VarChar | 1 |  | Base Currency |
| FatherType | VarChar | 1 |  | Parent Summary Type |
| IsICT | VarChar | 1 |  | A/R Invoice + Payment |
| DataSource | VarChar | 1 |  | Data Source |
| isCrin | VarChar | 1 |  | Corrected Invoice |
| selfInv | VarChar | 1 |  | Autom. Invoice |
| WddStatus | VarChar | 1 |  | Authorization Status |
| Exported | VarChar | 1 |  | Exported |
| NetProc | VarChar | 1 |  | Net Procedure |
| submitted | VarChar | 1 |  | Submitted |
| PoPrss | VarChar | 1 |  | PO Process |
| Rounding | VarChar | 1 |  | Rounding |
| RevisionPo | VarChar | 1 |  | Split PO |
| PickStatus | VarChar | 1 |  | Pick Status |
| Pick | VarChar | 1 |  | Pick |
| BlockDunn | VarChar | 1 |  | Block Dunning |
| PayBlock | VarChar | 1 |  | Payment Block |
| MaxDscn | VarChar | 1 |  | Maximum Discount |
| Reserve | VarChar | 1 |  | Reserve |
| BoeReserev | VarChar | 1 |  | Bill of Exchange Reserved |
| CEECFlag | VarChar | 1 |  | Block Creation Target Corr Inv |
| UseShpdGd | VarChar | 1 |  | Use Shipped Goods Account |
| DocSubType | nVarChar | 2 |  | VAT Code for Tax Invoice Rpt |
| DpmStatus | VarChar | 1 |  | Summary VAT Abstract ID |
| DpmDrawn | VarChar | 1 |  | Drawn to Down Payment |
| Posted | VarChar | 1 |  | Down Payment Was Posted |
| isIns | VarChar | 1 |  | Reserve Invoice |
| BPNameOW | VarChar | 1 |  | BP Name Overwritten |
| BillToOW | VarChar | 1 |  | Bill-To Overwritten |
| ShipToOW | VarChar | 1 |  | Ship-to Overwritten |
| RetInvoice | VarChar | 1 |  | Returning Invoice |
| UseCorrVat | VarChar | 1 |  | Use Correction VAT Group |
| BlkCredMmo | VarChar | 1 |  | Block Creation of Target Credit Memo |
| OpenForLaC | VarChar | 1 |  | Open For Landed Costs |
| Excised | VarChar | 1 |  | Excised |
| DutyStatus | VarChar | 1 |  | Duty Status |
| AutoCrtFlw | VarChar | 1 |  | Auto Create Follow-up Document |
| InsurOp347 | VarChar | 1 |  | 347 Insurance Operation |
| IgnRelDoc | VarChar | 1 |  | Ignore Relevant Doc on Archive |
| ResidenNum | VarChar | 1 |  | Residence Number |
| PQTGrpHW | VarChar | 1 |  | Pur Quotation Group Manual |
| DocManClsd | VarChar | 1 |  | Document Was Closed Manually |
| Ordered | VarChar | 1 |  | Payment Ordered |
| NTSApprov | VarChar | 1 |  | NTS Approved |
| EDocGenTyp | VarChar | 1 |  | Electr. Doc. Generation Type |
| OnlineQuo | VarChar | 1 |  | Create Online Quotation |
| EDocStatus | VarChar | 1 |  | Electronic Document Status |
| EDocProces | VarChar | 1 |  | Electronic Document Process |
| EDocCancel | VarChar | 1 |  | Electronic Document - Canceled |
| EDocTest | VarChar | 1 |  | Electronic Document - Testing |
| DpmAsDscnt | VarChar | 1 |  | Discount Document with Dpm |
| GTSRlvnt | VarChar | 1 |  | Relevant To GTS |
| SrvTaxRule | VarChar | 1 |  | Apply Service Tax Rule |
| ReqType | Int | 11 |  | Requester Type User/Employee |
| OriginType | VarChar | 1 |  | Document Origin |
| IsReuseNum | VarChar | 1 |  | Is Reusing Document Number |
| IsReuseNFN | VarChar | 1 |  | Is Reusing Nota Fiscal Number |
| IsAlt | VarChar | 1 |  | Is Alteration |
| AltBaseTyp | Int | 11 |  | Alteration Base Type |
| PrintSEPA | VarChar | 1 |  | Print SEPA Direct Debit Prenotification |
| RelatedTyp | Int | 11 |  | Related Type |
| NfePrntFo | Int | 11 |  | NF-e Printing Format |
| InterimTyp | Int | 6 |  | Interim Type |
| PoDropPrss | VarChar | 1 |  | PO Drop-Ship Process |
| ExclTaxRep | VarChar | 1 |  | Exclude from Control Statement |
| Revision | VarChar | 1 |  | Revision |
| BaseType | Int | 11 |  | Base Document Type |
| ComTrade | VarChar | 1 |  | Commission Trade |
| IssReason | Int | 6 |  | Reason for issuing note |
| ComTradeRt | VarChar | 1 |  | Commission Trade Return |
| SplitPmnt | VarChar | 1 |  | A/P Split Payment |
| SelfPosted | VarChar | 1 |  | Self Invoice Created [Yes/No] |
| DPPStatus | VarChar | 1 |  | Data Protection Status |
| EDocType | VarChar | 1 |  | eDoc Type |
| AggregDoc | VarChar | 1 |  | Aggregate Document |
| IndFinal | VarChar | 1 |  | Indicator for Final Consumer |
| PostPmntWT | VarChar | 1 |  | Post Payment Withholding Tax |
| FCEPmnMean | VarChar | 1 |  | Use FCEs as Payment Means |
| NotRel4MI | VarChar | 1 |  | Not Relevant for Monthly Invoice |
| Rel4PPTax | VarChar | 1 |  | Relevant for Plastic Packaging Tax |
| BookeTdsBP | VarChar | 1 |  | Book the eTDS to the Business Partner |
| DigPayment | VarChar | 1 |  | Digital Payments |
| RShipToOW | VarChar | 1 |  | Ship-to Address Overwritten for Return |
| AplTaxOnFr | VarChar | 1 |  | Apply Only Taxes in First Installment |
| CpyDtyStts | VarChar | 1 |  | Copy Duty Status from Base |

**索引：**
- PRIMARY (DocEntry) 🔑 UNIQUE
- AT_CARD (NumAtCard)

- CUSTOMER (CardCode)
- NUM (DocNum)
-
- DOC_STATUS (DocStatus)

- FTHR_CARD (FatherCard)

- SERIES (Series)
- OWNER_CODE (OwnerCode)
- DATE_PIND (DocDate)

- ESERIES (ESeries)

- FOL_SERIES (FolSeries)
- PROJECT (Project)

### MRDR — Sales Order

- **字段总数**: 101
- **关联子表**: RDR1 (Sales Order - Rows, 42 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| DocType | VarChar | 1 |  | Document Type |
| CANCELED | VarChar | 1 |  | Canceled |
| Handwrtten | VarChar | 1 |  | Manual Numbering |
| Printed | VarChar | 1 |  | Printed |
| DocStatus | VarChar | 1 |  | Document Status |
| InvntSttus | VarChar | 1 |  | Warehouse Status |
| Transfered | VarChar | 1 |  | Year Transfer |
| ObjType | nVarChar | 20 |  | Object Type |
| PartSupply | VarChar | 1 |  | Partial Delivery |
| Confirmed | VarChar | 1 |  | Confirmed |
| CreateTran | VarChar | 1 |  | Create Journal Entry |
| SummryType | VarChar | 1 |  | Summary Method |
| UpdInvnt | VarChar | 1 |  | Whse Update |
| UpdCardBal | VarChar | 1 |  | Update Balances |
| InvntDirec | VarChar | 1 |  | Warehouse Direction |
| ShowSCN | VarChar | 1 |  | Display BP Catalog Number |
| CurSource | VarChar | 1 |  | Base Currency |
| FatherType | VarChar | 1 |  | Parent Summary Type |
| IsICT | VarChar | 1 |  | A/R Invoice + Payment |
| DataSource | VarChar | 1 |  | Data Source |
| isCrin | VarChar | 1 |  | Corrected Invoice |
| selfInv | VarChar | 1 |  | Autom. Invoice |
| WddStatus | VarChar | 1 |  | Authorization Status |
| Exported | VarChar | 1 |  | Exported |
| NetProc | VarChar | 1 |  | Net Procedure |
| submitted | VarChar | 1 |  | Submitted |
| PoPrss | VarChar | 1 |  | PO Process |
| Rounding | VarChar | 1 |  | Rounding |
| RevisionPo | VarChar | 1 |  | Split PO |
| PickStatus | VarChar | 1 |  | Pick Status |
| Pick | VarChar | 1 |  | Pick |
| BlockDunn | VarChar | 1 |  | Block Dunning |
| PayBlock | VarChar | 1 |  | Payment Block |
| MaxDscn | VarChar | 1 |  | Maximum Discount |
| Reserve | VarChar | 1 |  | Reserve |
| BoeReserev | VarChar | 1 |  | Bill of Exchange Reserved |
| CEECFlag | VarChar | 1 |  | Block Creation Target Corr Inv |
| UseShpdGd | VarChar | 1 |  | Use Shipped Goods Account |
| DocSubType | nVarChar | 2 |  | VAT Code for Tax Invoice Rpt |
| DpmStatus | VarChar | 1 |  | Summary VAT Abstract ID |
| DpmDrawn | VarChar | 1 |  | Drawn to Down Payment |
| Posted | VarChar | 1 |  | Down Payment Was Posted |
| isIns | VarChar | 1 |  | Reserve Invoice |
| BPNameOW | VarChar | 1 |  | BP Name Overwritten |
| BillToOW | VarChar | 1 |  | Bill-To Overwritten |
| ShipToOW | VarChar | 1 |  | Ship-to Overwritten |
| RetInvoice | VarChar | 1 |  | Returning Invoice |
| UseCorrVat | VarChar | 1 |  | Use Correction VAT Group |
| BlkCredMmo | VarChar | 1 |  | Block Creation of Target Credit Memo |
| OpenForLaC | VarChar | 1 |  | Open For Landed Costs |
| Excised | VarChar | 1 |  | Excised |
| DutyStatus | VarChar | 1 |  | Duty Status |
| AutoCrtFlw | VarChar | 1 |  | Auto Create Follow-up Document |
| InsurOp347 | VarChar | 1 |  | 347 Insurance Operation |
| IgnRelDoc | VarChar | 1 |  | Ignore Relevant Doc on Archive |
| ResidenNum | VarChar | 1 |  | Residence Number |
| PQTGrpHW | VarChar | 1 |  | Pur Quotation Group Manual |
| DocManClsd | VarChar | 1 |  | Document Was Closed Manually |
| Ordered | VarChar | 1 |  | Payment Ordered |
| NTSApprov | VarChar | 1 |  | NTS Approved |
| EDocGenTyp | VarChar | 1 |  | Electr. Doc. Generation Type |
| OnlineQuo | VarChar | 1 |  | Create Online Quotation |
| EDocStatus | VarChar | 1 |  | Electronic Document Status |
| EDocProces | VarChar | 1 |  | Electronic Document Process |
| EDocCancel | VarChar | 1 |  | Electronic Document - Canceled |
| EDocTest | VarChar | 1 |  | Electronic Document - Testing |
| DpmAsDscnt | VarChar | 1 |  | Discount Document with Dpm |
| GTSRlvnt | VarChar | 1 |  | Relevant To GTS |
| SrvTaxRule | VarChar | 1 |  | Apply Service Tax Rule |
| ReqType | Int | 11 |  | Requester Type User/Employee |
| OriginType | VarChar | 1 |  | Document Origin |
| IsReuseNum | VarChar | 1 |  | Is Reusing Document Number |
| IsReuseNFN | VarChar | 1 |  | Is Reusing Nota Fiscal Number |
| IsAlt | VarChar | 1 |  | Is Alteration |
| AltBaseTyp | Int | 11 |  | Alteration Base Type |
| PrintSEPA | VarChar | 1 |  | Print SEPA Direct Debit Prenotification |
| RelatedTyp | Int | 11 |  | Related Type |
| NfePrntFo | Int | 11 |  | NF-e Printing Format |
| InterimTyp | Int | 6 |  | Interim Type |
| PoDropPrss | VarChar | 1 |  | PO Drop-Ship Process |
| ExclTaxRep | VarChar | 1 |  | Exclude from Control Statement |
| Revision | VarChar | 1 |  | Revision |
| BaseType | Int | 11 |  | Base Document Type |
| ComTrade | VarChar | 1 |  | Commission Trade |
| IssReason | Int | 6 |  | Reason for issuing note |
| ComTradeRt | VarChar | 1 |  | Commission Trade Return |
| SplitPmnt | VarChar | 1 |  | A/P Split Payment |
| SelfPosted | VarChar | 1 |  | Self Invoice Created [Yes/No] |
| DPPStatus | VarChar | 1 |  | Data Protection Status |
| EDocType | VarChar | 1 |  | eDoc Type |
| AggregDoc | VarChar | 1 |  | Aggregate Document |
| IndFinal | VarChar | 1 |  | Indicator for Final Consumer |
| PostPmntWT | VarChar | 1 |  | Post Payment Withholding Tax |
| FCEPmnMean | VarChar | 1 |  | Use FCEs as Payment Means |
| NotRel4MI | VarChar | 1 |  | Not Relevant for Monthly Invoice |
| Rel4PPTax | VarChar | 1 |  | Relevant for Plastic Packaging Tax |
| BookeTdsBP | VarChar | 1 |  | Book the eTDS to the Business Partner |
| DigPayment | VarChar | 1 |  | Digital Payments |
| RShipToOW | VarChar | 1 |  | Ship-to Address Overwritten for Return |
| AplTaxOnFr | VarChar | 1 |  | Apply Only Taxes in First Installment |
| CpyDtyStts | VarChar | 1 |  | Copy Duty Status from Base |

**索引：**
- PRIMARY (DocEntry) 🔑 UNIQUE
- AT_CARD (NumAtCard)

- CUSTOMER (CardCode)
- NUM (DocNum) 🔑 UNIQUE
-
- DOC_STATUS (DocStatus)

- FTHR_CARD (FatherCard)

- SERIES (Series)
- OWNER_CODE (OwnerCode)
- DATE_PIND (DocDate)

- ESERIES (ESeries)

- PROJECT (Project)

### MRIN — A/R Credit Memo

- **字段总数**: 101
- **关联子表**: RIN1 (A/R Credit Memo - Rows, 42 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| DocType | VarChar | 1 |  | Document Type |
| CANCELED | VarChar | 1 |  | Canceled |
| Handwrtten | VarChar | 1 |  | Manual Numbering |
| Printed | VarChar | 1 |  | Printed |
| DocStatus | VarChar | 1 |  | Document Status |
| InvntSttus | VarChar | 1 |  | Warehouse Status |
| Transfered | VarChar | 1 |  | Year Transfer |
| ObjType | nVarChar | 20 |  | Object Type |
| PartSupply | VarChar | 1 |  | Partial Delivery |
| Confirmed | VarChar | 1 |  | Confirmed |
| CreateTran | VarChar | 1 |  | Create Journal Entry |
| SummryType | VarChar | 1 |  | Summary Method |
| UpdInvnt | VarChar | 1 |  | Warehouse Update |
| UpdCardBal | VarChar | 1 |  | Update Balances |
| InvntDirec | VarChar | 1 |  | Warehouse Direction |
| ShowSCN | VarChar | 1 |  | Display BP Catalog Number |
| CurSource | VarChar | 1 |  | Base Currency |
| FatherType | VarChar | 1 |  | Parent Summary Type |
| IsICT | VarChar | 1 |  | A/R Invoice + Payment |
| DataSource | VarChar | 1 |  | Data Source |
| isCrin | VarChar | 1 |  | Corrected Invoice |
| selfInv | VarChar | 1 |  | Autom. Invoice |
| WddStatus | VarChar | 1 |  | Authorization Status |
| Exported | VarChar | 1 |  | Exported |
| NetProc | VarChar | 1 |  | Net Procedure |
| submitted | VarChar | 1 |  | Submitted |
| PoPrss | VarChar | 1 |  | PO Process |
| Rounding | VarChar | 1 |  | Rounding |
| RevisionPo | VarChar | 1 |  | Split PO |
| PickStatus | VarChar | 1 |  | Pick Status |
| Pick | VarChar | 1 |  | Pick |
| BlockDunn | VarChar | 1 |  | Block Dunning |
| PayBlock | VarChar | 1 |  | Payment Block |
| MaxDscn | VarChar | 1 |  | Maximum Discount |
| Reserve | VarChar | 1 |  | Reserve |
| BoeReserev | VarChar | 1 |  | Bill of Exchange Reserved |
| CEECFlag | VarChar | 1 |  | Block Creation Target Corr Inv |
| UseShpdGd | VarChar | 1 |  | Use Shipped Goods Account |
| DocSubType | nVarChar | 2 |  | VAT Code for Tax Invoice Rpt |
| DpmStatus | VarChar | 1 |  | Summary VAT Abstract ID |
| DpmDrawn | VarChar | 1 |  | Drawn to Down Payment |
| Posted | VarChar | 1 |  | Down Payment Was Posted |
| isIns | VarChar | 1 |  | Reserve Invoice |
| BPNameOW | VarChar | 1 |  | BP_NAME_OVERWRITTEN |
| BillToOW | VarChar | 1 |  | BILL_TO_OVERWRITTEN |
| ShipToOW | VarChar | 1 |  | SHIP_TO_OVERWRITTEN |
| RetInvoice | VarChar | 1 |  | Credit Memo |
| UseCorrVat | VarChar | 1 |  | Use Correction VAT Group |
| BlkCredMmo | VarChar | 1 |  | Block Creating Credit Memo Tgt |
| OpenForLaC | VarChar | 1 |  | Open For Landed Costs |
| Excised | VarChar | 1 |  | Excised |
| DutyStatus | VarChar | 1 |  | Duty Status |
| AutoCrtFlw | VarChar | 1 |  | Auto Create Follow-up Document |
| InsurOp347 | VarChar | 1 |  | 347 Insurance Operation |
| IgnRelDoc | VarChar | 1 |  | Ignore Relevant Doc on Archive |
| ResidenNum | VarChar | 1 |  | Residence Number |
| PQTGrpHW | VarChar | 1 |  | Pur Quotation Group Manual |
| DocManClsd | VarChar | 1 |  | Document Was Closed Manually |
| Ordered | VarChar | 1 |  | Payment Ordered |
| NTSApprov | VarChar | 1 |  | NTS Approved |
| EDocGenTyp | VarChar | 1 |  | Electr. Doc. Generation Type |
| OnlineQuo | VarChar | 1 |  | Create Online Quotation |
| EDocStatus | VarChar | 1 |  | Electronic Document Status |
| EDocProces | VarChar | 1 |  | Electronic Document Process |
| EDocCancel | VarChar | 1 |  | Electronic Document - Canceled |
| EDocTest | VarChar | 1 |  | Electronic Document - Testing |
| DpmAsDscnt | VarChar | 1 |  | Discount Document with Dpm |
| GTSRlvnt | VarChar | 1 |  | Relevant To GTS |
| SrvTaxRule | VarChar | 1 |  | Apply Service Tax Rule |
| ReqType | Int | 11 |  | Requester Type User/Employee |
| OriginType | VarChar | 1 |  | Document Origin |
| IsReuseNum | VarChar | 1 |  | Is Reusing Document Number |
| IsReuseNFN | VarChar | 1 |  | Is Reusing Nota Fiscal Number |
| IsAlt | VarChar | 1 |  | Is Alteration |
| AltBaseTyp | Int | 11 |  | Alteration Base Type |
| PrintSEPA | VarChar | 1 |  | Print SEPA Direct Debit Prenotification |
| RelatedTyp | Int | 11 |  | Related Type |
| NfePrntFo | Int | 11 |  | NF-e Printing Format |
| InterimTyp | Int | 6 |  | Interim Type |
| PoDropPrss | VarChar | 1 |  | PO Drop-Ship Process |
| ExclTaxRep | VarChar | 1 |  | Exclude from Control Statement |
| Revision | VarChar | 1 |  | Revision |
| BaseType | Int | 11 |  | Base Document Type |
| ComTrade | VarChar | 1 |  | Commission Trade |
| IssReason | Int | 6 |  | Reason for issuing note |
| ComTradeRt | VarChar | 1 |  | Commission Trade Return |
| SplitPmnt | VarChar | 1 |  | A/P Split Payment |
| SelfPosted | VarChar | 1 |  | Self Invoice Created [Yes/No] |
| DPPStatus | VarChar | 1 |  | Data Protection Status |
| EDocType | VarChar | 1 |  | eDoc Type |
| AggregDoc | VarChar | 1 |  | Aggregate Document |
| IndFinal | VarChar | 1 |  | Indicator for Final Consumer |
| PostPmntWT | VarChar | 1 |  | Post Payment Withholding Tax |
| FCEPmnMean | VarChar | 1 |  | Use FCEs as Payment Means |
| NotRel4MI | VarChar | 1 |  | Not Relevant for Monthly Invoice |
| Rel4PPTax | VarChar | 1 |  | Relevant for Plastic Packaging Tax |
| BookeTdsBP | VarChar | 1 |  | Book the eTDS to the Business Partner |
| DigPayment | VarChar | 1 |  | Digital Payments |
| RShipToOW | VarChar | 1 |  | Ship-to Address Overwritten for Return |
| AplTaxOnFr | VarChar | 1 |  | Apply Only Taxes in First Installment |
| CpyDtyStts | VarChar | 1 |  | Copy Duty Status from Base |

**索引：**
- PRIMARY (DocEntry) 🔑 UNIQUE
- AT_CARD (NumAtCard)

- CUSTOMER (CardCode)
- NUM (DocNum)
-
- DOC_STATUS (DocStatus)

- FTHR_CARD (FatherCard)

- SERIES (Series)
- OWNER_CODE (OwnerCode)
- DATE_PIND (DocDate)
- 
- ESERIES (ESeries)

- FOL_SERIES (FolSeries)
- PROJECT (Project)

---

<a name="purchase"></a>
## 📋 采购模块

### MPCH — A/P Invoice

- **字段总数**: 101
- **关联子表**: PCH1 (A/P Invoice - Rows, 42 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| DocType | VarChar | 1 |  | Document Type |
| CANCELED | VarChar | 1 |  | Canceled |
| Handwrtten | VarChar | 1 |  | Manual Numbering |
| Printed | VarChar | 1 |  | Printed |
| DocStatus | VarChar | 1 |  | Document Status |
| InvntSttus | VarChar | 1 |  | Warehouse Status |
| Transfered | VarChar | 1 |  | Transfer Year |
| ObjType | nVarChar | 20 |  | Object Type |
| PartSupply | VarChar | 1 |  | Partial Delivery |
| Confirmed | VarChar | 1 |  | Confirmed |
| CreateTran | VarChar | 1 |  | Create Journal Entry |
| SummryType | VarChar | 1 |  | Summary Method |
| UpdInvnt | VarChar | 1 |  | Whse Update |
| UpdCardBal | VarChar | 1 |  | Update Balances |
| InvntDirec | VarChar | 1 |  | Warehouse Direction |
| ShowSCN | VarChar | 1 |  | Display BP Catalog Number |
| CurSource | VarChar | 1 |  | Base Currency |
| FatherType | VarChar | 1 |  | Parent Summary Type |
| IsICT | VarChar | 1 |  | A/R Invoice + Payment |
| DataSource | VarChar | 1 |  | Data Source |
| isCrin | VarChar | 1 |  | Corrected Invoice |
| selfInv | VarChar | 1 |  | Autom. Invoice |
| WddStatus | VarChar | 1 |  | Authorization Status |
| Exported | VarChar | 1 |  | Exported |
| NetProc | VarChar | 1 |  | Net Procedure |
| submitted | VarChar | 1 |  | Submitted |
| PoPrss | VarChar | 1 |  | PO Process |
| Rounding | VarChar | 1 |  | Rounding |
| RevisionPo | VarChar | 1 |  | Split Purchase Order |
| PickStatus | VarChar | 1 |  | Pick Status |
| Pick | VarChar | 1 |  | Pick |
| BlockDunn | VarChar | 1 |  | Block Dunning |
| PayBlock | VarChar | 1 |  | Payment Block |
| MaxDscn | VarChar | 1 |  | Maximum Discount |
| Reserve | VarChar | 1 |  | Reserve |
| BoeReserev | VarChar | 1 |  | Bill of Exchange Reserved |
| CEECFlag | VarChar | 1 |  | A/P Correction Invoice |
| UseShpdGd | VarChar | 1 |  | Use Shipped Goods Account |
| DocSubType | nVarChar | 2 |  | VAT Code for Tax Invoice Rpt |
| DpmStatus | VarChar | 1 |  | Summary VAT Abstract ID |
| DpmDrawn | VarChar | 1 |  | Drawn to Down Payment |
| Posted | VarChar | 1 |  | Down Payment Was Posted |
| isIns | VarChar | 1 |  | Reserve Invoice |
| BPNameOW | VarChar | 1 |  | BP_NAME_OVERWRITTEN |
| BillToOW | VarChar | 1 |  | BILL_TO_OVERWRITTEN |
| ShipToOW | VarChar | 1 |  | SHIP_TO_OVERWRITTEN |
| RetInvoice | VarChar | 1 |  | Returning Invoice |
| UseCorrVat | VarChar | 1 |  | Use Correction VAT Group |
| BlkCredMmo | VarChar | 1 |  | Block Creating Credit Memo Tgt |
| OpenForLaC | VarChar | 1 |  | Open For Landed Costs |
| Excised | VarChar | 1 |  | Excised |
| DutyStatus | VarChar | 1 |  | Duty Status |
| AutoCrtFlw | VarChar | 1 |  | Auto Create Follow-up Document |
| InsurOp347 | VarChar | 1 |  | 347 Insurance Operation |
| IgnRelDoc | VarChar | 1 |  | Ignore Relevant Doc on Archive |
| ResidenNum | VarChar | 1 |  | Residence Number |
| PQTGrpHW | VarChar | 1 |  | Pur Quotation Group Manual |
| DocManClsd | VarChar | 1 |  | Document Was Closed Manually |
| Ordered | VarChar | 1 |  | Payment Ordered |
| NTSApprov | VarChar | 1 |  | NTS Approved |
| EDocGenTyp | VarChar | 1 |  | Electr. Doc. Generation Type |
| OnlineQuo | VarChar | 1 |  | Create Online Quotation |
| EDocStatus | VarChar | 1 |  | Electronic Document Status |
| EDocProces | VarChar | 1 |  | Electronic Document Process |
| EDocCancel | VarChar | 1 |  | Electronic Document - Canceled |
| EDocTest | VarChar | 1 |  | Electronic Document - Testing |
| DpmAsDscnt | VarChar | 1 |  | Discount Document with Dpm |
| GTSRlvnt | VarChar | 1 |  | Relevant To GTS |
| SrvTaxRule | VarChar | 1 |  | Apply Service Tax Rule |
| ReqType | Int | 11 |  | Requester Type User/Employee |
| OriginType | VarChar | 1 |  | Document Origin |
| IsReuseNum | VarChar | 1 |  | Is Reusing Document Number |
| IsReuseNFN | VarChar | 1 |  | Is Reusing Nota Fiscal Number |
| IsAlt | VarChar | 1 |  | Is Alteration |
| AltBaseTyp | Int | 11 |  | Alteration Base Type |
| PrintSEPA | VarChar | 1 |  | Print SEPA Direct Debit Prenotification |
| RelatedTyp | Int | 11 |  | Related Type |
| NfePrntFo | Int | 11 |  | NF-e Printing Format |
| InterimTyp | Int | 6 |  | Interim Type |
| PoDropPrss | VarChar | 1 |  | PO Drop-Ship Process |
| ExclTaxRep | VarChar | 1 |  | Exclude from Control Statement |
| Revision | VarChar | 1 |  | Revision |
| BaseType | Int | 11 |  | Base Document Type |
| ComTrade | VarChar | 1 |  | Commission Trade |
| IssReason | Int | 6 |  | Reason for issuing note |
| ComTradeRt | VarChar | 1 |  | Commission Trade Return |
| SplitPmnt | VarChar | 1 |  | A/P Split Payment |
| SelfPosted | VarChar | 1 |  | Self Invoice Created [Yes/No] |
| DPPStatus | VarChar | 1 |  | Data Protection Status |
| EDocType | VarChar | 1 |  | eDoc Type |
| AggregDoc | VarChar | 1 |  | Aggregate Document |
| IndFinal | VarChar | 1 |  | Indicator for Final Consumer |
| PostPmntWT | VarChar | 1 |  | Post Payment Withholding Tax |
| FCEPmnMean | VarChar | 1 |  | Use FCEs as Payment Means |
| NotRel4MI | VarChar | 1 |  | Not Relevant for Monthly Invoice |
| Rel4PPTax | VarChar | 1 |  | Relevant for Plastic Packaging Tax |
| BookeTdsBP | VarChar | 1 |  | Book the eTDS to the Business Partner |
| DigPayment | VarChar | 1 |  | Digital Payments |
| RShipToOW | VarChar | 1 |  | Ship-to Address Overwritten for Return |
| AplTaxOnFr | VarChar | 1 |  | Apply Only Taxes in First Installment |
| CpyDtyStts | VarChar | 1 |  | Copy Duty Status from Base |

**索引：**
- PRIMARY (DocEntry) 🔑 UNIQUE
- AT_CARD (NumAtCard)

- CUSTOMER (CardCode)
- NUM (DocNum)
-
- DOC_STATUS (DocStatus)

- FTHR_CARD (FatherCard)

- SERIES (Series)
- OWNER_CODE (OwnerCode)
- DATE_PIND (DocDate)

- ESERIES (ESeries)

- PROJECT (Project)

### MPDN — Goods Receipt PO

- **字段总数**: 101
- **关联子表**: PDN1 (Goods Receipt PO - Rows, 42 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| DocType | VarChar | 1 |  | Document Type |
| CANCELED | VarChar | 1 |  | Canceled |
| Handwrtten | VarChar | 1 |  | Manual Numbering |
| Printed | VarChar | 1 |  | Printed |
| DocStatus | VarChar | 1 |  | Document Status |
| InvntSttus | VarChar | 1 |  | Warehouse Status |
| Transfered | VarChar | 1 |  | Year Transfer |
| ObjType | nVarChar | 20 |  | Object Type |
| PartSupply | VarChar | 1 |  | Partial Delivery |
| Confirmed | VarChar | 1 |  | Confirmed |
| CreateTran | VarChar | 1 |  | Create Journal Entry |
| SummryType | VarChar | 1 |  | Summary Method |
| UpdInvnt | VarChar | 1 |  | Whse Update |
| UpdCardBal | VarChar | 1 |  | Update Balances |
| InvntDirec | VarChar | 1 |  | Warehouse Direction |
| ShowSCN | VarChar | 1 |  | Display BP Catalog Number |
| CurSource | VarChar | 1 |  | Base Currency |
| FatherType | VarChar | 1 |  | Parent Summary Type |
| IsICT | VarChar | 1 |  | A/R Invoice + Payment |
| DataSource | VarChar | 1 |  | Data Source |
| isCrin | VarChar | 1 |  | Corrected Invoice |
| selfInv | VarChar | 1 |  | Autom. Invoice |
| WddStatus | VarChar | 1 |  | Authorization Status |
| Exported | VarChar | 1 |  | Exported |
| NetProc | VarChar | 1 |  | Net Procedure |
| submitted | VarChar | 1 |  | Submitted |
| PoPrss | VarChar | 1 |  | PO Process |
| Rounding | VarChar | 1 |  | Rounding |
| RevisionPo | VarChar | 1 |  | Split Purchase Order |
| PickStatus | VarChar | 1 |  | Pick Status |
| Pick | VarChar | 1 |  | Pick |
| BlockDunn | VarChar | 1 |  | Block Dunning |
| PayBlock | VarChar | 1 |  | Payment Block |
| MaxDscn | VarChar | 1 |  | Maximum Discount |
| Reserve | VarChar | 1 |  | Reserve |
| BoeReserev | VarChar | 1 |  | Bill of Exchange Reserved |
| CEECFlag | VarChar | 1 |  | Block Creation of Target Corr. Inv. |
| UseShpdGd | VarChar | 1 |  | Use Shipped Goods Account |
| DocSubType | nVarChar | 2 |  | VAT Code for Tax Invoice Rpt |
| DpmStatus | VarChar | 1 |  | Summary VAT Abstract ID |
| DpmDrawn | VarChar | 1 |  | Drawn to Down Payment |
| Posted | VarChar | 1 |  | Down Payment Was Posted |
| isIns | VarChar | 1 |  | Reserve Invoice |
| BPNameOW | VarChar | 1 |  | BP Name Overwritten |
| BillToOW | VarChar | 1 |  | Bill-To Overwritten |
| ShipToOW | VarChar | 1 |  | Ship-to Overwritten |
| RetInvoice | VarChar | 1 |  | Credit Memo |
| UseCorrVat | VarChar | 1 |  | Use Correction VAT Group |
| BlkCredMmo | VarChar | 1 |  | Block Creation of Target Credit Memo |
| OpenForLaC | VarChar | 1 |  | Open For Landed Costs |
| Excised | VarChar | 1 |  | Excised |
| DutyStatus | VarChar | 1 |  | Duty Status |
| AutoCrtFlw | VarChar | 1 |  | Auto Create Follow-up Document |
| InsurOp347 | VarChar | 1 |  | 347 Insurance Operation |
| IgnRelDoc | VarChar | 1 |  | Ignore Relevant Doc on Archive |
| ResidenNum | VarChar | 1 |  | Residence Number |
| PQTGrpHW | VarChar | 1 |  | Pur Quotation Group Manual |
| DocManClsd | VarChar | 1 |  | Document Was Closed Manually |
| Ordered | VarChar | 1 |  | Payment Ordered |
| NTSApprov | VarChar | 1 |  | NTS Approved |
| EDocGenTyp | VarChar | 1 |  | Electr. Doc. Generation Type |
| OnlineQuo | VarChar | 1 |  | Create Online Quotation |
| EDocStatus | VarChar | 1 |  | Electronic Document Status |
| EDocProces | VarChar | 1 |  | Electronic Document Process |
| EDocCancel | VarChar | 1 |  | Electronic Document - Canceled |
| EDocTest | VarChar | 1 |  | Electronic Document - Testing |
| DpmAsDscnt | VarChar | 1 |  | Discount Document with Dpm |
| GTSRlvnt | VarChar | 1 |  | Relevant To GTS |
| SrvTaxRule | VarChar | 1 |  | Apply Service Tax Rule |
| ReqType | Int | 11 |  | Requester Type User/Employee |
| OriginType | VarChar | 1 |  | Document Origin |
| IsReuseNum | VarChar | 1 |  | Is Reusing Document Number |
| IsReuseNFN | VarChar | 1 |  | Is Reusing Nota Fiscal Number |
| IsAlt | VarChar | 1 |  | Is Alteration |
| AltBaseTyp | Int | 11 |  | Alteration Base Type |
| PrintSEPA | VarChar | 1 |  | Print SEPA Direct Debit Prenotification |
| RelatedTyp | Int | 11 |  | Related Type |
| NfePrntFo | Int | 11 |  | NF-e Printing Format |
| InterimTyp | Int | 6 |  | Interim Type |
| PoDropPrss | VarChar | 1 |  | PO Drop-Ship Process |
| ExclTaxRep | VarChar | 1 |  | Exclude from Control Statement |
| Revision | VarChar | 1 |  | Revision |
| BaseType | Int | 11 |  | Base Document Type |
| ComTrade | VarChar | 1 |  | Commission Trade |
| IssReason | Int | 6 |  | Reason for issuing note |
| ComTradeRt | VarChar | 1 |  | Commission Trade Return |
| SplitPmnt | VarChar | 1 |  | A/P Split Payment |
| SelfPosted | VarChar | 1 |  | Self Invoice Created [Yes/No] |
| DPPStatus | VarChar | 1 |  | Data Protection Status |
| EDocType | VarChar | 1 |  | eDoc Type |
| AggregDoc | VarChar | 1 |  | Aggregate Document |
| IndFinal | VarChar | 1 |  | Indicator for Final Consumer |
| PostPmntWT | VarChar | 1 |  | Post Payment Withholding Tax |
| FCEPmnMean | VarChar | 1 |  | Use FCEs as Payment Means |
| NotRel4MI | VarChar | 1 |  | Not Relevant for Monthly Invoice |
| Rel4PPTax | VarChar | 1 |  | Relevant for Plastic Packaging Tax |
| BookeTdsBP | VarChar | 1 |  | Book the eTDS to the Business Partner |
| DigPayment | VarChar | 1 |  | Digital Payments |
| RShipToOW | VarChar | 1 |  | Ship-to Address Overwritten for Return |
| AplTaxOnFr | VarChar | 1 |  | Apply Only Taxes in First Installment |
| CpyDtyStts | VarChar | 1 |  | Copy Duty Status from Base |

**索引：**
- PRIMARY (DocEntry) 🔑 UNIQUE
- AT_CARD (NumAtCard)

- CUSTOMER (CardCode)
- NUM (DocNum)

- DOC_STATUS (DocStatus)

- FTHR_CARD (FatherCard)

- SERIES (Series)
- OWNER_CODE (OwnerCode)
- DATE_PIND (DocDate)

- ESERIES (ESeries)

- PROJECT (Project)

### MPOR — Purchase Order

- **字段总数**: 101
- **关联子表**: POR1 (Purchase Order - Rows, 42 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| DocType | VarChar | 1 |  | Document Type |
| CANCELED | VarChar | 1 |  | Canceled |
| Handwrtten | VarChar | 1 |  | Manual Numbering |
| Printed | VarChar | 1 |  | Printed |
| DocStatus | VarChar | 1 |  | Document Status |
| InvntSttus | VarChar | 1 |  | Warehouse Status |
| Transfered | VarChar | 1 |  | Year Transfer |
| ObjType | nVarChar | 20 |  | Object Type |
| PartSupply | VarChar | 1 |  | Partial Delivery |
| Confirmed | VarChar | 1 |  | Confirmed |
| CreateTran | VarChar | 1 |  | Create Journal Entry |
| SummryType | VarChar | 1 |  | Summary Method |
| UpdInvnt | VarChar | 1 |  | Warehouse Update |
| UpdCardBal | VarChar | 1 |  | Update Balances |
| InvntDirec | VarChar | 1 |  | Warehouse Direction |
| ShowSCN | VarChar | 1 |  | Display BP Catalog Number |
| CurSource | VarChar | 1 |  | Base Currency |
| FatherType | VarChar | 1 |  | Parent Summary Type |
| IsICT | VarChar | 1 |  | A/R Invoice + Payment |
| DataSource | VarChar | 1 |  | Data Source |
| isCrin | VarChar | 1 |  | Corrected Invoice |
| selfInv | VarChar | 1 |  | Autom. Invoice |
| WddStatus | VarChar | 1 |  | Authorization Status |
| Exported | VarChar | 1 |  | Exported |
| NetProc | VarChar | 1 |  | Net Procedure |
| submitted | VarChar | 1 |  | Submitted |
| PoPrss | VarChar | 1 |  | PO Process |
| Rounding | VarChar | 1 |  | Rounding |
| RevisionPo | VarChar | 1 |  | Split PO |
| PickStatus | VarChar | 1 |  | Pick Status |
| Pick | VarChar | 1 |  | Pick |
| BlockDunn | VarChar | 1 |  | Block Dunning |
| PayBlock | VarChar | 1 |  | Payment Block |
| MaxDscn | VarChar | 1 |  | Maximum Discount |
| Reserve | VarChar | 1 |  | Reserve |
| BoeReserev | VarChar | 1 |  | Bill of Exchange Reserved |
| CEECFlag | VarChar | 1 |  | Block Creation Target Corr Inv |
| UseShpdGd | VarChar | 1 |  | Use Shipped Goods Account |
| DocSubType | nVarChar | 2 |  | VAT Code for Tax Invoice Rpt |
| DpmStatus | VarChar | 1 |  | Summary VAT Abstract ID |
| DpmDrawn | VarChar | 1 |  | Drawn to Down Payment |
| Posted | VarChar | 1 |  | Down Payment Was Posted |
| isIns | VarChar | 1 |  | Reserve Invoice |
| BPNameOW | VarChar | 1 |  | BP_NAME_OVERWRITTEN |
| BillToOW | VarChar | 1 |  | BILL_TO_OVERWRITTEN |
| ShipToOW | VarChar | 1 |  | SHIP_TO_OVERWRITTEN |
| RetInvoice | VarChar | 1 |  | Credit Memo |
| UseCorrVat | VarChar | 1 |  | Use Correction VAT Group |
| BlkCredMmo | VarChar | 1 |  | Block Creating Credit Memo Tgt |
| OpenForLaC | VarChar | 1 |  | Open For Landed Costs |
| Excised | VarChar | 1 |  | Excised |
| DutyStatus | VarChar | 1 |  | Duty Status |
| AutoCrtFlw | VarChar | 1 |  | Auto Create Follow-up Document |
| InsurOp347 | VarChar | 1 |  | 347 Insurance Operation |
| IgnRelDoc | VarChar | 1 |  | Ignore Relevant Doc on Archive |
| ResidenNum | VarChar | 1 |  | Residence Number |
| PQTGrpHW | VarChar | 1 |  | Pur Quotation Group Manual |
| DocManClsd | VarChar | 1 |  | Document Was Closed Manually |
| Ordered | VarChar | 1 |  | Payment Ordered |
| NTSApprov | VarChar | 1 |  | NTS Approved |
| EDocGenTyp | VarChar | 1 |  | Electr. Doc. Generation Type |
| OnlineQuo | VarChar | 1 |  | Create Online Quotation |
| EDocStatus | VarChar | 1 |  | Electronic Document Status |
| EDocProces | VarChar | 1 |  | Electronic Document Process |
| EDocCancel | VarChar | 1 |  | Electronic Document - Canceled |
| EDocTest | VarChar | 1 |  | Electronic Document - Testing |
| DpmAsDscnt | VarChar | 1 |  | Discount Document with Dpm |
| GTSRlvnt | VarChar | 1 |  | Relevant To GTS |
| SrvTaxRule | VarChar | 1 |  | Apply Service Tax Rule |
| ReqType | Int | 11 |  | Requester Type User/Employee |
| OriginType | VarChar | 1 |  | Document Origin |
| IsReuseNum | VarChar | 1 |  | Is Reusing Document Number |
| IsReuseNFN | VarChar | 1 |  | Is Reusing Nota Fiscal Number |
| IsAlt | VarChar | 1 |  | Is Alteration |
| AltBaseTyp | Int | 11 |  | Alteration Base Type |
| PrintSEPA | VarChar | 1 |  | Print SEPA Direct Debit Prenotification |
| RelatedTyp | Int | 11 |  | Related Type |
| NfePrntFo | Int | 11 |  | NF-e Printing Format |
| InterimTyp | Int | 6 |  | Interim Type |
| PoDropPrss | VarChar | 1 |  | PO Drop-Ship Process |
| ExclTaxRep | VarChar | 1 |  | Exclude from Control Statement |
| Revision | VarChar | 1 |  | Revision |
| BaseType | Int | 11 |  | Base Document Type |
| ComTrade | VarChar | 1 |  | Commission Trade |
| IssReason | Int | 6 |  | Reason for issuing note |
| ComTradeRt | VarChar | 1 |  | Commission Trade Return |
| SplitPmnt | VarChar | 1 |  | A/P Split Payment |
| SelfPosted | VarChar | 1 |  | Self Invoice Created [Yes/No] |
| DPPStatus | VarChar | 1 |  | Data Protection Status |
| EDocType | VarChar | 1 |  | eDoc Type |
| AggregDoc | VarChar | 1 |  | Aggregate Document |
| IndFinal | VarChar | 1 |  | Indicator for Final Consumer |
| PostPmntWT | VarChar | 1 |  | Post Payment Withholding Tax |
| FCEPmnMean | VarChar | 1 |  | Use FCEs as Payment Means |
| NotRel4MI | VarChar | 1 |  | Not Relevant for Monthly Invoice |
| Rel4PPTax | VarChar | 1 |  | Relevant for Plastic Packaging Tax |
| BookeTdsBP | VarChar | 1 |  | Book the eTDS to the Business Partner |
| DigPayment | VarChar | 1 |  | Digital Payments |
| RShipToOW | VarChar | 1 |  | Ship-to Address Overwritten for Return |
| AplTaxOnFr | VarChar | 1 |  | Apply Only Taxes in First Installment |
| CpyDtyStts | VarChar | 1 |  | Copy Duty Status from Base |

**索引：**
- PRIMARY (DocEntry) 🔑 UNIQUE
- AT_CARD (NumAtCard)

- CUSTOMER (CardCode)
- NUM (DocNum) 🔑 UNIQUE
-
- DOC_STATUS (DocStatus)

- FTHR_CARD (FatherCard)

- SERIES (Series)
- OWNER_CODE (OwnerCode)
- DATE_PIND (DocDate)

- ESERIES (ESeries)

- PROJECT (Project)

### MRPC — A/P Credit Memo

- **字段总数**: 101
- **关联子表**: RPC1 (A/P Credit Memo - Rows, 42 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| DocType | VarChar | 1 |  | Document Type |
| CANCELED | VarChar | 1 |  | Canceled |
| Handwrtten | VarChar | 1 |  | Manual Numbering |
| Printed | VarChar | 1 |  | Printed |
| DocStatus | VarChar | 1 |  | Document Status |
| InvntSttus | VarChar | 1 |  | Warehouse Status |
| Transfered | VarChar | 1 |  | Year Transfer |
| ObjType | nVarChar | 20 |  | Object Type |
| PartSupply | VarChar | 1 |  | Partial Delivery |
| Confirmed | VarChar | 1 |  | Confirmed |
| CreateTran | VarChar | 1 |  | Create Journal Entry |
| SummryType | VarChar | 1 |  | Summary Method |
| UpdInvnt | VarChar | 1 |  | Warehouse Update |
| UpdCardBal | VarChar | 1 |  | Update Balances |
| InvntDirec | VarChar | 1 |  | Warehouse Direction |
| ShowSCN | VarChar | 1 |  | Display BP Catalog Number |
| CurSource | VarChar | 1 |  | Base Currency |
| FatherType | VarChar | 1 |  | Parent Summary Type |
| IsICT | VarChar | 1 |  | A/R Invoice + Payment |
| DataSource | VarChar | 1 |  | Data Source |
| isCrin | VarChar | 1 |  | Corrected Invoice |
| selfInv | VarChar | 1 |  | Autom. Invoice |
| WddStatus | VarChar | 1 |  | Authorization Status |
| Exported | VarChar | 1 |  | Exported |
| NetProc | VarChar | 1 |  | Net Procedure |
| submitted | VarChar | 1 |  | Submitted |
| PoPrss | VarChar | 1 |  | PO Process |
| Rounding | VarChar | 1 |  | Rounding |
| RevisionPo | VarChar | 1 |  | Split Purchase Order |
| PickStatus | VarChar | 1 |  | Pick Status |
| Pick | VarChar | 1 |  | Pick |
| BlockDunn | VarChar | 1 |  | Block Dunning |
| PayBlock | VarChar | 1 |  | Payment Block |
| MaxDscn | VarChar | 1 |  | Maximum Discount |
| Reserve | VarChar | 1 |  | Reserve |
| BoeReserev | VarChar | 1 |  | Bill of Exchange Reserved |
| CEECFlag | VarChar | 1 |  | A/P Correction Invoice |
| UseShpdGd | VarChar | 1 |  | Use Shipped Goods Account |
| DocSubType | nVarChar | 2 |  | VAT Code for Tax Invoice Rpt |
| DpmStatus | VarChar | 1 |  | Summary VAT Abstract ID |
| DpmDrawn | VarChar | 1 |  | Drawn to Down Payment |
| Posted | VarChar | 1 |  | Down Payment Was Posted |
| isIns | VarChar | 1 |  | Reserve Invoice |
| BPNameOW | VarChar | 1 |  | BP_NAME_OVERWRITTEN |
| BillToOW | VarChar | 1 |  | Bill-To Overwritten |
| ShipToOW | VarChar | 1 |  | Ship-to Overwritten |
| RetInvoice | VarChar | 1 |  | Returning Invoice |
| UseCorrVat | VarChar | 1 |  | Use Correction VAT Group |
| BlkCredMmo | VarChar | 1 |  | Block Creation of Target Credit Memo |
| OpenForLaC | VarChar | 1 |  | Open For Landed Costs |
| Excised | VarChar | 1 |  | Excised |
| DutyStatus | VarChar | 1 |  | Duty Status |
| AutoCrtFlw | VarChar | 1 |  | Auto Create Follow-up Document |
| InsurOp347 | VarChar | 1 |  | 347 Insurance Operation |
| IgnRelDoc | VarChar | 1 |  | Ignore Relevant Doc on Archive |
| ResidenNum | VarChar | 1 |  | Residence Number |
| PQTGrpHW | VarChar | 1 |  | Pur Quotation Group Manual |
| DocManClsd | VarChar | 1 |  | Document Was Closed Manually |
| Ordered | VarChar | 1 |  | Payment Ordered |
| NTSApprov | VarChar | 1 |  | NTS Approved |
| EDocGenTyp | VarChar | 1 |  | Electr. Doc. Generation Type |
| OnlineQuo | VarChar | 1 |  | Create Online Quotation |
| EDocStatus | VarChar | 1 |  | Electronic Document Status |
| EDocProces | VarChar | 1 |  | Electronic Document Process |
| EDocCancel | VarChar | 1 |  | Electronic Document - Canceled |
| EDocTest | VarChar | 1 |  | Electronic Document - Testing |
| DpmAsDscnt | VarChar | 1 |  | Discount Document with Dpm |
| GTSRlvnt | VarChar | 1 |  | Relevant To GTS |
| SrvTaxRule | VarChar | 1 |  | Apply Service Tax Rule |
| ReqType | Int | 11 |  | Requester Type User/Employee |
| OriginType | VarChar | 1 |  | Document Origin |
| IsReuseNum | VarChar | 1 |  | Is Reusing Document Number |
| IsReuseNFN | VarChar | 1 |  | Is Reusing Nota Fiscal Number |
| IsAlt | VarChar | 1 |  | Is Alteration |
| AltBaseTyp | Int | 11 |  | Alteration Base Type |
| PrintSEPA | VarChar | 1 |  | Print SEPA Direct Debit Prenotification |
| RelatedTyp | Int | 11 |  | Related Type |
| NfePrntFo | Int | 11 |  | NF-e Printing Format |
| InterimTyp | Int | 6 |  | Interim Type |
| PoDropPrss | VarChar | 1 |  | PO Drop-Ship Process |
| ExclTaxRep | VarChar | 1 |  | Exclude from Control Statement |
| Revision | VarChar | 1 |  | Revision |
| BaseType | Int | 11 |  | Base Document Type |
| ComTrade | VarChar | 1 |  | Commission Trade |
| IssReason | Int | 6 |  | Reason for issuing note |
| ComTradeRt | VarChar | 1 |  | Commission Trade Return |
| SplitPmnt | VarChar | 1 |  | A/P Split Payment |
| SelfPosted | VarChar | 1 |  | Self Invoice Created [Yes/No] |
| DPPStatus | VarChar | 1 |  | Data Protection Status |
| EDocType | VarChar | 1 |  | eDoc Type |
| AggregDoc | VarChar | 1 |  | Aggregate Document |
| IndFinal | VarChar | 1 |  | Indicator for Final Consumer |
| PostPmntWT | VarChar | 1 |  | Post Payment Withholding Tax |
| FCEPmnMean | VarChar | 1 |  | Use FCEs as Payment Means |
| NotRel4MI | VarChar | 1 |  | Not Relevant for Monthly Invoice |
| Rel4PPTax | VarChar | 1 |  | Relevant for Plastic Packaging Tax |
| BookeTdsBP | VarChar | 1 |  | Book the eTDS to the Business Partner |
| DigPayment | VarChar | 1 |  | Digital Payments |
| RShipToOW | VarChar | 1 |  | Ship-to Address Overwritten for Return |
| AplTaxOnFr | VarChar | 1 |  | Apply Only Taxes in First Installment |
| CpyDtyStts | VarChar | 1 |  | Copy Duty Status from Base |

**索引：**
- PRIMARY (DocEntry) 🔑 UNIQUE
- AT_CARD (NumAtCard)

- CUSTOMER (CardCode)
- NUM (DocNum)
-
- DOC_STATUS (DocStatus)

- FTHR_CARD (FatherCard)

- SERIES (Series)
- OWNER_CODE (OwnerCode)
- DATE_PIND (DocDate)

- ESERIES (ESeries)

- PROJECT (Project)

### MRPD — Goods Return

- **字段总数**: 101
- **关联子表**: RPD1 (Goods Return - Rows, 42 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| DocType | VarChar | 1 |  | Document Type |
| CANCELED | VarChar | 1 |  | Canceled |
| Handwrtten | VarChar | 1 |  | Manual Numbering |
| Printed | VarChar | 1 |  | Printed |
| DocStatus | VarChar | 1 |  | Document Status |
| InvntSttus | VarChar | 1 |  | Warehouse Status |
| Transfered | VarChar | 1 |  | Year Transfer |
| ObjType | nVarChar | 20 |  | Object Type |
| PartSupply | VarChar | 1 |  | Partial Delivery |
| Confirmed | VarChar | 1 |  | Confirmed |
| CreateTran | VarChar | 1 |  | Create Journal Entry |
| SummryType | VarChar | 1 |  | Summary Method |
| UpdInvnt | VarChar | 1 |  | Warehouse Update |
| UpdCardBal | VarChar | 1 |  | Update Balances |
| InvntDirec | VarChar | 1 |  | Warehouse Direction |
| ShowSCN | VarChar | 1 |  | Display BP Catalog Number |
| CurSource | VarChar | 1 |  | Base Currency |
| FatherType | VarChar | 1 |  | Parent Summary Type |
| IsICT | VarChar | 1 |  | A/R Invoice + Payment |
| DataSource | VarChar | 1 |  | Data Source |
| isCrin | VarChar | 1 |  | Corrected Invoice |
| selfInv | VarChar | 1 |  | Autom. Invoice |
| WddStatus | VarChar | 1 |  | Authorization Status |
| Exported | VarChar | 1 |  | Exported |
| NetProc | VarChar | 1 |  | Net Procedure |
| submitted | VarChar | 1 |  | Submitted |
| PoPrss | VarChar | 1 |  | PO Process |
| Rounding | VarChar | 1 |  | Rounding |
| RevisionPo | VarChar | 1 |  | Split PO |
| PickStatus | VarChar | 1 |  | Pick Status |
| Pick | VarChar | 1 |  | Pick |
| BlockDunn | VarChar | 1 |  | Block Dunning |
| PayBlock | VarChar | 1 |  | Payment Block |
| MaxDscn | VarChar | 1 |  | Maximum Discount |
| Reserve | VarChar | 1 |  | Reserve |
| BoeReserev | VarChar | 1 |  | Bill of Exchange Reserved |
| CEECFlag | VarChar | 1 |  | Block Creation Target Corr Inv |
| UseShpdGd | VarChar | 1 |  | Use Shipped Goods Account |
| DocSubType | nVarChar | 2 |  | VAT Code for Tax Invoice Rpt |
| DpmStatus | VarChar | 1 |  | Summary VAT Abstract ID |
| DpmDrawn | VarChar | 1 |  | Drawn to Down Payment |
| Posted | VarChar | 1 |  | Down Payment Was Posted |
| isIns | VarChar | 1 |  | Reserve Invoice |
| BPNameOW | VarChar | 1 |  | BP Name Overwritten |
| BillToOW | VarChar | 1 |  | Bill-To Overwritten |
| ShipToOW | VarChar | 1 |  | Ship-to Overwritten |
| RetInvoice | VarChar | 1 |  | Returning Invoice |
| UseCorrVat | VarChar | 1 |  | Use Correction VAT Group |
| BlkCredMmo | VarChar | 1 |  | Block Creation of Target Credit Memo |
| OpenForLaC | VarChar | 1 |  | Open For Landed Costs |
| Excised | VarChar | 1 |  | Excised |
| DutyStatus | VarChar | 1 |  | Duty Status |
| AutoCrtFlw | VarChar | 1 |  | Auto Create Follow-up Document |
| InsurOp347 | VarChar | 1 |  | 347 Insurance Operation |
| IgnRelDoc | VarChar | 1 |  | Ignore Relevant Doc on Archive |
| ResidenNum | VarChar | 1 |  | Residence Number |
| PQTGrpHW | VarChar | 1 |  | Pur Quotation Group Manual |
| DocManClsd | VarChar | 1 |  | Document Was Closed Manually |
| Ordered | VarChar | 1 |  | Payment Ordered |
| NTSApprov | VarChar | 1 |  | NTS Approved |
| EDocGenTyp | VarChar | 1 |  | Electr. Doc. Generation Type |
| OnlineQuo | VarChar | 1 |  | Create Online Quotation |
| EDocStatus | VarChar | 1 |  | Electronic Document Status |
| EDocProces | VarChar | 1 |  | Electronic Document Process |
| EDocCancel | VarChar | 1 |  | Electronic Document - Canceled |
| EDocTest | VarChar | 1 |  | Electronic Document - Testing |
| DpmAsDscnt | VarChar | 1 |  | Discount Document with Dpm |
| GTSRlvnt | VarChar | 1 |  | Relevant To GTS |
| SrvTaxRule | VarChar | 1 |  | Apply Service Tax Rule |
| ReqType | Int | 11 |  | Requester Type User/Employee |
| OriginType | VarChar | 1 |  | Document Origin |
| IsReuseNum | VarChar | 1 |  | Is Reusing Document Number |
| IsReuseNFN | VarChar | 1 |  | Is Reusing Nota Fiscal Number |
| IsAlt | VarChar | 1 |  | Is Alteration |
| AltBaseTyp | Int | 11 |  | Alteration Base Type |
| PrintSEPA | VarChar | 1 |  | Print SEPA Direct Debit Prenotification |
| RelatedTyp | Int | 11 |  | Related Type |
| NfePrntFo | Int | 11 |  | NF-e Printing Format |
| InterimTyp | Int | 6 |  | Interim Type |
| PoDropPrss | VarChar | 1 |  | PO Drop-Ship Process |
| ExclTaxRep | VarChar | 1 |  | Exclude from Control Statement |
| Revision | VarChar | 1 |  | Revision |
| BaseType | Int | 11 |  | Base Document Type |
| ComTrade | VarChar | 1 |  | Commission Trade |
| IssReason | Int | 6 |  | Reason for issuing note |
| ComTradeRt | VarChar | 1 |  | Commission Trade Return |
| SplitPmnt | VarChar | 1 |  | A/P Split Payment |
| SelfPosted | VarChar | 1 |  | Self Invoice Created [Yes/No] |
| DPPStatus | VarChar | 1 |  | Data Protection Status |
| EDocType | VarChar | 1 |  | eDoc Type |
| AggregDoc | VarChar | 1 |  | Aggregate Document |
| IndFinal | VarChar | 1 |  | Indicator for Final Consumer |
| PostPmntWT | VarChar | 1 |  | Post Payment Withholding Tax |
| FCEPmnMean | VarChar | 1 |  | Use FCEs as Payment Means |
| NotRel4MI | VarChar | 1 |  | Not Relevant for Monthly Invoice |
| Rel4PPTax | VarChar | 1 |  | Relevant for Plastic Packaging Tax |
| BookeTdsBP | VarChar | 1 |  | Book the eTDS to the Business Partner |
| DigPayment | VarChar | 1 |  | Digital Payments |
| RShipToOW | VarChar | 1 |  | Ship-to Address Overwritten for Return |
| AplTaxOnFr | VarChar | 1 |  | Apply Only Taxes in First Installment |
| CpyDtyStts | VarChar | 1 |  | Copy Duty Status from Base |

**索引：**
- PRIMARY (DocEntry) 🔑 UNIQUE
- AT_CARD (NumAtCard)

- CUSTOMER (CardCode)
- NUM (DocNum)
-
- DOC_STATUS (DocStatus)

- FTHR_CARD (FatherCard)

- SERIES (Series)
- OWNER_CODE (OwnerCode)
- DATE_PIND (DocDate)

- ESERIES (ESeries)

- PROJECT (Project)

---

<a name="warehouse"></a>
## 🏭 库存/仓库模块

### MIGE — Goods Issue

- **字段总数**: 102
- **关联子表**: IGE1 (Goods Issue - Rows, 42 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| DocType | VarChar | 1 |  | Document Type |
| CANCELED | VarChar | 1 |  | Canceled |
| Handwrtten | VarChar | 1 |  | Manual Numbering |
| Printed | VarChar | 1 |  | Printed |
| DocStatus | VarChar | 1 |  | Document Status |
| InvntSttus | VarChar | 1 |  | Warehouse Status |
| Transfered | VarChar | 1 |  | Year Transfer |
| ObjType | nVarChar | 20 |  | Object Type |
| PartSupply | VarChar | 1 |  | Partial Delivery |
| Confirmed | VarChar | 1 |  | Confirmed |
| CreateTran | VarChar | 1 |  | Create Journal Entry |
| SummryType | VarChar | 1 |  | Summary Method |
| UpdInvnt | VarChar | 1 |  | Warehouse Update |
| UpdCardBal | VarChar | 1 |  | Update Balances |
| InvntDirec | VarChar | 1 |  | Warehouse Direction |
| ShowSCN | VarChar | 1 |  | Display BP Catalog Number |
| CurSource | VarChar | 1 |  | Base Currency |
| FatherType | VarChar | 1 |  | Parent Summary Type |
| IsICT | VarChar | 1 |  | A/R Invoice + Payment |
| DataSource | VarChar | 1 |  | Data Source |
| isCrin | VarChar | 1 |  | Corrected Invoice |
| selfInv | VarChar | 1 |  | Autom. Invoice |
| WddStatus | VarChar | 1 |  | Authorization Status |
| Exported | VarChar | 1 |  | Exported |
| NetProc | VarChar | 1 |  | Net Procedure |
| submitted | VarChar | 1 |  | Submitted |
| PoPrss | VarChar | 1 |  | PO Process |
| Rounding | VarChar | 1 |  | Rounding |
| RevisionPo | VarChar | 1 |  | Split PO |
| PickStatus | VarChar | 1 |  | Pick Status |
| Pick | VarChar | 1 |  | Pick |
| BlockDunn | VarChar | 1 |  | Block Dunning |
| PayBlock | VarChar | 1 |  | Payment Block |
| MaxDscn | VarChar | 1 |  | Maximum Discount |
| Reserve | VarChar | 1 |  | Reserve |
| DeferrTax | VarChar | 1 |  | Deferred Tax |
| BoeReserev | VarChar | 1 |  | Bill of Exchange Reserved |
| CEECFlag | VarChar | 1 |  | Block Creation Target Corr Inv |
| UseShpdGd | VarChar | 1 |  | Use Shipped Goods Account |
| DocSubType | nVarChar | 2 |  | VAT Code for Tax Invoice Rpt |
| DpmStatus | VarChar | 1 |  | Summary VAT Abstract ID |
| DpmDrawn | VarChar | 1 |  | Drawn to Down Payment |
| Posted | VarChar | 1 |  | Down Payment Was Posted |
| isIns | VarChar | 1 |  | Reserve Invoice |
| BPNameOW | VarChar | 1 |  | BP_NAME_OVERWRITTEN |
| BillToOW | VarChar | 1 |  | BILL_TO_OVERWRITTEN |
| ShipToOW | VarChar | 1 |  | SHIP_TO_OVERWRITTEN |
| RetInvoice | VarChar | 1 |  | Credit Memo |
| UseCorrVat | VarChar | 1 |  | Use Correction VAT Group |
| BlkCredMmo | VarChar | 1 |  | Block Creating Credit Memo Tgt |
| OpenForLaC | VarChar | 1 |  | Open For Landed Costs |
| Excised | VarChar | 1 |  | Excised |
| DutyStatus | VarChar | 1 |  | Duty Status |
| AutoCrtFlw | VarChar | 1 |  | Auto Create Follow-up Document |
| InsurOp347 | VarChar | 1 |  | 347 Insurance Operation |
| IgnRelDoc | VarChar | 1 |  | Ignore Relevant Doc on Archive |
| ResidenNum | VarChar | 1 |  | Residence Number |
| PQTGrpHW | VarChar | 1 |  | Pur Quotation Group Manual |
| DocManClsd | VarChar | 1 |  | Document Was Closed Manually |
| Ordered | VarChar | 1 |  | Payment Ordered |
| NTSApprov | VarChar | 1 |  | NTS Approved |
| EDocGenTyp | VarChar | 1 |  | Electr. Doc. Generation Type |
| OnlineQuo | VarChar | 1 |  | Create Online Quotation |
| EDocStatus | VarChar | 1 |  | Electronic Document Status |
| EDocProces | VarChar | 1 |  | Electronic Document Process |
| EDocCancel | VarChar | 1 |  | Electronic Document - Canceled |
| EDocTest | VarChar | 1 |  | Electronic Document - Testing |
| DpmAsDscnt | VarChar | 1 |  | Discount Document with Dpm |
| GTSRlvnt | VarChar | 1 |  | Relevant To GTS |
| SrvTaxRule | VarChar | 1 |  | Apply Service Tax Rule |
| ReqType | Int | 11 |  | Requester Type User/Employee |
| OriginType | VarChar | 1 |  | Document Origin |
| IsReuseNum | VarChar | 1 |  | Is Reusing Document Number |
| IsReuseNFN | VarChar | 1 |  | Is Reusing Nota Fiscal Number |
| IsAlt | VarChar | 1 |  | Is Alteration |
| AltBaseTyp | Int | 11 |  | Alteration Base Type |
| PrintSEPA | VarChar | 1 |  | Print SEPA Direct Debit Prenotification |
| RelatedTyp | Int | 11 |  | Related Type |
| NfePrntFo | Int | 11 |  | NF-e Printing Format |
| InterimTyp | Int | 6 |  | Interim Type |
| PoDropPrss | VarChar | 1 |  | PO Drop-Ship Process |
| ExclTaxRep | VarChar | 1 |  | Exclude from Control Statement |
| Revision | VarChar | 1 |  | Revision |
| BaseType | Int | 11 |  | Base Document Type |
| ComTrade | VarChar | 1 |  | Commission Trade |
| IssReason | Int | 6 |  | Reason for issuing note |
| ComTradeRt | VarChar | 1 |  | Commission Trade Return |
| SplitPmnt | VarChar | 1 |  | A/P Split Payment |
| SelfPosted | VarChar | 1 |  | Self Invoice Created [Yes/No] |
| DPPStatus | VarChar | 1 |  | Data Protection Status |
| EDocType | VarChar | 1 |  | eDoc Type |
| AggregDoc | VarChar | 1 |  | Aggregate Document |
| IndFinal | VarChar | 1 |  | Indicator for Final Consumer |
| PostPmntWT | VarChar | 1 |  | Post Payment Withholding Tax |
| FCEPmnMean | VarChar | 1 |  | Use FCEs as Payment Means |
| NotRel4MI | VarChar | 1 |  | Not Relevant for Monthly Invoice |
| Rel4PPTax | VarChar | 1 |  | Relevant for Plastic Packaging Tax |
| BookeTdsBP | VarChar | 1 |  | Book the eTDS to the Business Partner |
| DigPayment | VarChar | 1 |  | Digital Payments |
| RShipToOW | VarChar | 1 |  | Ship-to Address Overwritten for Return |
| AplTaxOnFr | VarChar | 1 |  | Apply Only Taxes in First Installment |
| CpyDtyStts | VarChar | 1 |  | Copy Duty Status from Base |

**索引：**
- PRIMARY (DocEntry) 🔑 UNIQUE
- AT_CARD (NumAtCard)

- CUSTOMER (CardCode)
- NUM (DocNum) 🔑 UNIQUE
-
- DOC_STATUS (DocStatus)

- FTHR_CARD (FatherCard)

- SERIES (Series)
- OWNER_CODE (OwnerCode)
- DATE_PIND (DocDate)

- ESERIES (ESeries)

- PROJECT (Project)

### MIGN — Goods Receipt

- **字段总数**: 102
- **关联子表**: IGN1 (Goods Receipt - Rows, 42 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| DocType | VarChar | 1 |  | Document Type |
| CANCELED | VarChar | 1 |  | Canceled |
| Handwrtten | VarChar | 1 |  | Manual Numbering |
| Printed | VarChar | 1 |  | Printed |
| DocStatus | VarChar | 1 |  | Document Status |
| InvntSttus | VarChar | 1 |  | Warehouse Status |
| Transfered | VarChar | 1 |  | Year Transfer |
| ObjType | nVarChar | 20 |  | Object Type |
| PartSupply | VarChar | 1 |  | Partial Delivery |
| Confirmed | VarChar | 1 |  | Confirmed |
| CreateTran | VarChar | 1 |  | Create Journal Entry |
| SummryType | VarChar | 1 |  | Summary Method |
| UpdInvnt | VarChar | 1 |  | Warehouse Update |
| UpdCardBal | VarChar | 1 |  | Update Balances |
| InvntDirec | VarChar | 1 |  | Warehouse Direction |
| ShowSCN | VarChar | 1 |  | Display BP Catalog Number |
| CurSource | VarChar | 1 |  | Base Currency |
| FatherType | VarChar | 1 |  | Parent Summary Type |
| IsICT | VarChar | 1 |  | A/R Invoice + Payment |
| DataSource | VarChar | 1 |  | Data Source |
| isCrin | VarChar | 1 |  | Corrected Invoice |
| selfInv | VarChar | 1 |  | Autom. Invoice |
| WddStatus | VarChar | 1 |  | Authorization Status |
| Exported | VarChar | 1 |  | Exported |
| NetProc | VarChar | 1 |  | Net Procedure |
| submitted | VarChar | 1 |  | Submitted |
| PoPrss | VarChar | 1 |  | PO Process |
| Rounding | VarChar | 1 |  | Rounding |
| RevisionPo | VarChar | 1 |  | Split PO |
| PickStatus | VarChar | 1 |  | Pick Status |
| Pick | VarChar | 1 |  | Pick |
| BlockDunn | VarChar | 1 |  | Block Dunning |
| PayBlock | VarChar | 1 |  | Payment Block |
| MaxDscn | VarChar | 1 |  | Maximum Discount |
| Reserve | VarChar | 1 |  | Reserve |
| DeferrTax | VarChar | 1 |  | Deferred Tax |
| BoeReserev | VarChar | 1 |  | Bill of Exchange Reserved |
| CEECFlag | VarChar | 1 |  | Block Creation Target Corr Inv |
| UseShpdGd | VarChar | 1 |  | Use Shipped Goods Account |
| DocSubType | nVarChar | 2 |  | VAT Code for Tax Invoice Rpt |
| DpmStatus | VarChar | 1 |  | Summary VAT Abstract ID |
| DpmDrawn | VarChar | 1 |  | Drawn to Down Payment |
| Posted | VarChar | 1 |  | Down Payment Was Posted |
| isIns | VarChar | 1 |  | Reserve Invoice |
| BPNameOW | VarChar | 1 |  | BP_NAME_OVERWRITTEN |
| BillToOW | VarChar | 1 |  | BILL_TO_OVERWRITTEN |
| ShipToOW | VarChar | 1 |  | SHIP_TO_OVERWRITTEN |
| RetInvoice | VarChar | 1 |  | Credit Memo |
| UseCorrVat | VarChar | 1 |  | Use Correction VAT Group |
| BlkCredMmo | VarChar | 1 |  | Block Creating Credit Memo Tgt |
| OpenForLaC | VarChar | 1 |  | Open For Landed Costs |
| Excised | VarChar | 1 |  | Excised |
| DutyStatus | VarChar | 1 |  | Duty Status |
| AutoCrtFlw | VarChar | 1 |  | Auto Create Follow-up Document |
| InsurOp347 | VarChar | 1 |  | 347 Insurance Operation |
| IgnRelDoc | VarChar | 1 |  | Ignore Relevant Doc on Archive |
| ResidenNum | VarChar | 1 |  | Residence Number |
| PQTGrpHW | VarChar | 1 |  | Pur Quotation Group Manual |
| DocManClsd | VarChar | 1 |  | Document Was Closed Manually |
| Ordered | VarChar | 1 |  | Payment Ordered |
| NTSApprov | VarChar | 1 |  | NTS Approved |
| EDocGenTyp | VarChar | 1 |  | Electr. Doc. Generation Type |
| OnlineQuo | VarChar | 1 |  | Create Online Quotation |
| EDocStatus | VarChar | 1 |  | Electronic Document Status |
| EDocProces | VarChar | 1 |  | Electronic Document Process |
| EDocCancel | VarChar | 1 |  | Electronic Document - Canceled |
| EDocTest | VarChar | 1 |  | Electronic Document - Testing |
| DpmAsDscnt | VarChar | 1 |  | Discount Document with Dpm |
| GTSRlvnt | VarChar | 1 |  | Relevant To GTS |
| SrvTaxRule | VarChar | 1 |  | Apply Service Tax Rule |
| ReqType | Int | 11 |  | Requester Type User/Employee |
| OriginType | VarChar | 1 |  | Document Origin |
| IsReuseNum | VarChar | 1 |  | Is Reusing Document Number |
| IsReuseNFN | VarChar | 1 |  | Is Reusing Nota Fiscal Number |
| IsAlt | VarChar | 1 |  | Is Alteration |
| AltBaseTyp | Int | 11 |  | Alteration Base Type |
| PrintSEPA | VarChar | 1 |  | Print SEPA Direct Debit Prenotification |
| RelatedTyp | Int | 11 |  | Related Type |
| NfePrntFo | Int | 11 |  | NF-e Printing Format |
| InterimTyp | Int | 6 |  | Interim Type |
| PoDropPrss | VarChar | 1 |  | PO Drop-Ship Process |
| ExclTaxRep | VarChar | 1 |  | Exclude from Control Statement |
| Revision | VarChar | 1 |  | Revision |
| BaseType | Int | 11 |  | Base Document Type |
| ComTrade | VarChar | 1 |  | Commission Trade |
| IssReason | Int | 6 |  | Reason for issuing note |
| ComTradeRt | VarChar | 1 |  | Commission Trade Return |
| SplitPmnt | VarChar | 1 |  | A/P Split Payment |
| SelfPosted | VarChar | 1 |  | Self Invoice Created [Yes/No] |
| DPPStatus | VarChar | 1 |  | Data Protection Status |
| EDocType | VarChar | 1 |  | eDoc Type |
| AggregDoc | VarChar | 1 |  | Aggregate Document |
| IndFinal | VarChar | 1 |  | Indicator for Final Consumer |
| PostPmntWT | VarChar | 1 |  | Post Payment Withholding Tax |
| FCEPmnMean | VarChar | 1 |  | Use FCEs as Payment Means |
| NotRel4MI | VarChar | 1 |  | Not Relevant for Monthly Invoice |
| Rel4PPTax | VarChar | 1 |  | Relevant for Plastic Packaging Tax |
| BookeTdsBP | VarChar | 1 |  | Book the eTDS to the Business Partner |
| DigPayment | VarChar | 1 |  | Digital Payments |
| RShipToOW | VarChar | 1 |  | Ship-to Address Overwritten for Return |
| AplTaxOnFr | VarChar | 1 |  | Apply Only Taxes in First Installment |
| CpyDtyStts | VarChar | 1 |  | Copy Duty Status from Base |

**索引：**
- PRIMARY (DocEntry) 🔑 UNIQUE
- AT_CARD (NumAtCard)

- CUSTOMER (CardCode)
- NUM (DocNum) 🔑 UNIQUE
-
- DOC_STATUS (DocStatus)

- FTHR_CARD (FatherCard)

- SERIES (Series)
- OWNER_CODE (OwnerCode)
- DATE_PIND (DocDate)

- ESERIES (ESeries)

- PROJECT (Project)

### MWHS — Warehouses

- **字段总数**: 20
- **关联子表**: AWHS (Warehouses - History, 20 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| Locked | VarChar | 1 |  | Locked |
| DataSource | VarChar | 1 |  | Data Source |
| DropShip | VarChar | 1 |  | Drop-Ship |
| UseTax | VarChar | 1 |  | Allow Use Tax |
| Nettable | VarChar | 1 |  | Nettable |
| OwnerCode | VarChar | 1 |  | Owner Code |
| Excisable | VarChar | 1 |  | Excisable [Yes/No] |
| BinActivat | VarChar | 1 |  | Bin Activated [Y/N] |
| DftBinEnfd | VarChar | 1 |  | Default Bin Enforced [Y/N] |
| AutoIssMtd | Int | 6 |  | Auto. Issue Method |
| ManageSnB | VarChar | 1 |  | Drop-Ship Manage SnB |
| RecItemsBy | Int | 6 |  | Receiving Bin Locations Method |
| RecBinEnab | VarChar | 1 |  | Enable Receiving Bin Locations |
| RecvEmpBin | VarChar | 1 |  | Restrict Receipts to Empty Bin |
| Inactive | VarChar | 1 |  | Inactive |
| RecvMaxQty | VarChar | 1 |  | Recv. up to Max. Qty |
| AutoRecvMd | Int | 6 |  | Auto. Receipt Method |
| RecvMaxWT | VarChar | 1 |  | Recv. up to Max. Weight |
| RecvUpTo | nVarChar | 6 |  | Receive up to |
| External | VarChar | 1 |  | External |

**索引：**
- PRIMARY (WhsCode) 🔑 UNIQUE
- DFT_BIN (DftBinAbs)

---

<a name="user"></a>
## 👤 用户与权限模块

### MUSR — Users

- **字段总数**: 50
- **关联子表**: AUSR (Users - History, 51 字段)

| 字段名 | 数据类型 | 大小 | PK | 描述 |
| :--- | :--- | :--- | :---: | :--- |
| GROUPS | Int | 6 |  | User Group |
| SUPERUSER | VarChar | 1 |  | Superuser |
| dType | VarChar | 1 |  | dType |
| OutOfOffic | VarChar | 1 |  | Out of Office |
| SendEMail | VarChar | 1 |  | Send E-Mail |
| SendSMS | VarChar | 1 |  | Send SMS |
| CashLimit | VarChar | 1 |  | Cash Amount Limit |
| SendFax | VarChar | 1 |  | Send Fax |
| Locked | VarChar | 1 |  | User Locked |
| OpenCdt | VarChar | 1 |  | Open Window for Credit Reference |
| DsplyRates | VarChar | 1 |  | Display Rate Table on start up |
| AuImpRates | VarChar | 1 |  | Import Currency Rates Automatically |
| OpenDps | VarChar | 1 |  | Open Postdated Checks Window |
| RcrFlag | VarChar | 1 |  | Display Transactions Scheduled for Today |
| CheckFiles | VarChar | 1 |  | File Check |
| ContactLog | VarChar | 1 |  | Today's Activity Alert |
| ShowNewMsg | VarChar | 1 |  | Open Message on Arrival |
| GENDER | VarChar | 1 |  | Gender |
| EnbMenuFlt | VarChar | 1 |  | Enable Forbidden Menu Items |
| OneLogPwd | VarChar | 1 |  | At first logon change password |
| PwdNeverEx | VarChar | 1 |  | Password Never Expires |
| RclFlag | VarChar | 1 |  | Display Recurring Transactions |
| MobileUser | VarChar | 1 |  | Mobile User |
| PrsWkCntEb | VarChar | 1 |  | Personal Work Center Enable |
| SupportUsr | VarChar | 1 |  | Support User |
| ShowNewTsk | VarChar | 1 |  | Open Worklist on Task Arrival |
| IntgrtEb | VarChar | 1 |  | Enable Setting Integration |
| AllBrnchF | VarChar | 1 |  | Allow Viewing of All (Including Unassigned To) Branches in Financial Reports |
| IgnDtOwn | VarChar | 1 |  | Ignore Data Ownership for this user |
| EnterAsTab | VarChar | 1 |  | Use Numeric Keypad Enter Key as Tab Key |
| DotAsSep | VarChar | 1 |  | Use Del Key As Separator |
| MouseOnly | VarChar | 1 |  | Document Operation by Mouse Only |
| NaturalPer | VarChar | 1 |  | Natural Person |
| DPPStatus | VarChar | 1 |  | Data Protection Status |
| AutoAsnBPL | VarChar | 1 |  | Auto. Assign Branches |
| HandleEDoc | VarChar | 1 |  | Can Process Electronic Documents |
| ShowLicBal | VarChar | 1 |  | Show License Balloon |
| ShowFstMsg | VarChar | 1 |  | Show Only Part of Message |
| CleanChb | VarChar | 1 |  | Checkbox of Message Cleanup |
| CleanIbx | VarChar | 1 |  | Checkbox of Cleanup Inbox |
| CleanObx | VarChar | 1 |  | Checkbox of Cleanup Outbox |
| CleanSnt | VarChar | 1 |  | Checkbox of Cleanup Sent |
| validFor | VarChar | 1 |  | Active |
| frozenFor | VarChar | 1 |  | Inactive |
| AprvDsnc | VarChar | 1 |  | Approval Decision Notification |
| ReqAprvl | VarChar | 1 |  | Requests for Approval Notification |
| RnwBlktA | VarChar | 1 |  | Renew Blanket Agreement Reminders Notification |
| SvcClRem | VarChar | 1 |  | Service Call Reminders Notification |
| SvcClAsn | VarChar | 1 |  | Service Call Assignments Notification |
| WzdTsk | VarChar | 1 |  | Wizard Tasks Notification |

**索引：**
- PRIMARY (USERID) 🔑 UNIQUE
- PASSWORD (PASSWORD)
- USER_CODE (USER_CODE) 🔑 UNIQUE
- INTERNAL (INTERNAL_K) 🔑 UNIQUE

---
