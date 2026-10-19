# SMT MES Industry Solution / SMT 车间执行行业解决方案

Meta-Org 把 SMT 电子制造的车间执行（上料防错、料盘级库存、SN 过站防错、首件/巡检、维修、Andon、停机、拼板、产线看板与追溯）实现为平台的一个行业解决方案 `smt_mes_v1`。它完全运行在平台内（Go 领域模块 + Next.js 工作区 + 每租户独立数据库），不再依赖外部 Python 版 SMT-MES；同时保留与真实 SMT-MES／设备边缘系统的对接通道（实例注册、主数据与工单下发、HMAC 事件回传、轮询兜底），用于存量部署的渐进迁移与设备接入。

Meta-Org ships SMT electronics shop-floor execution (feeder verification, reel-level inventory, SN pass-station foolproofing, FAI/IPQC, repair, andon, downtime, panels, line board, and traceability) as the industry solution `smt_mes_v1`. Everything runs inside the platform (Go domain module + Next.js workspace + per-tenant database) and no longer requires the external Python SMT-MES; an edge bridge (instance registry, master-data/work-order push, HMAC-signed webhooks, polling fallback) remains for migrating existing deployments and connecting equipment.

## 一、方案标识 / Solution identity

| 项目 / Item | 值 / Value |
| --- | --- |
| 行业分类 / Industry | `smt_manufacturing`（SMT Electronics Manufacturing） |
| 解决方案 / Solution | `smt_mes_v1`（version 1） |
| 模块 / Module | `smt_mes`（category `industry`，license `commercial`） |
| 方案启用模块 / Enabled modules | `erp`, `inventory`, `manufacturing`, `smt_mes` |
| 解决方案流程 / Solution flow | `POST /platform/admin/organizations/{id}/industry-solution-flows/smt-mes`（RequestType `smt_mes_solution_flow`，verify → approve → apply） |
| 前端工作区 / Workspace | 租户域 `SMT`（路由 slug `smt`），模块工作台 `smt_mes` + 车间工作区十个页签 |
| 能力包 / Capability package | `platform.capability_packages('smt_mes_v1','industry_solution','v1')` + marketplace listing |
| 前提 / Prerequisite | 租户必须使用 `dedicated_database` 部署模式（ERP 拒绝 `shared_schema`） |

## 二、码表设计 / Code tables

全部为平台码表（`MREG` 注册 + `create_erp_master/child` 建表，`migrations/036_smt_mes_solution.sql`，租户库通过 `migrations/tenant/011` 的 `tenantdb:include` 获得同一套表）。主表为 `PK TEXT + Payload JSONB`，子表为 `(ParentKey, LineNum)`。

| 码表 / Code | 名称 | 主键 | 子表 | 说明 |
| --- | --- | --- | --- | --- |
| `MLIN` | SMT 产线 | LineCode | `LIN1` | LIN1 行＝机台清单 |
| `MMAC` | SMT 机台 | MachineCode | `MAC1` | MAC1 行＝站位（SlotNo/Barcode/Locked）；payload 存当前工单与核对/签核状态 |
| `MFSB` | 料站表 | SetupCode | `FSB1` | 产品×机台×程序；FSB1 行＝站位→料号/替代料 |
| `MOPR` | 工序库 | OpCode | — | OpType：SMT/REFLOW/INSPECT/TEST/ASSEMBLY/PACK/DEPANEL/OTHER |
| `MROU` | 工艺路线 | RouteCode | `ROU1` | ROU1 行＝序号→工序 |
| `MPRD` | 生产产品 | ProductCode | `PRD1` | PRD1 行＝BOM（料号/用量/位号/替代料） |
| `MCPM` | 客供料号对照 | MapCode | — | 客户料号↔厂内料号 |
| `MREL` | 料盘库存 | ReelCode | `REL1` | payload：料号/批次/数量/状态/归属/发料工单/库位；REL1 行＝收/发/退/调/废流水 |
| `MSWO` | SMT 工单 | WorkOrderCode | `SWO1` | payload：产品/产线/计划/完成/客户/路线；SWO1 行＝用料需求与缺口 |
| `MRQN` | 领料申请 | ReqNo | `RQN1` | RQN1 行＝料号/需求/已发 |
| `MLOD` | 上料记录 | LoadCode | `LOD1` | payload：工单/机台/站位/料盘/料号/主接料/状态；LOD1 行＝接料与卸料明细 |
| `MVER` | 防错扫描记录 | ScanCode | — | 站位/料盘/需求与实扫料号/结果/错误类型/提示（含语音） |
| `MLWA` | 缺料预警 | AlertCode | — | 站位/料号/余量估算/可生产板数/状态 |
| `MUNT` | SN 在制品 | SN | `UNT1` | payload：工单/产品/状态/当前工序/不良次数/拼板；UNT1 行＝过站履历 |
| `MPNL` | 拼板 | PanelCode | `PNL1` | PNL1 行＝板位→SN |
| `MDFC` | 不良代码 | DefectCode | — | 名称/分类 |
| `MUDF` | 不良维修记录 | DefectID | — | SN/工序/不良码/位号/料号/状态/维修措施/原因 |
| `MINS` | 首件与巡检 | InspNo | `INS1` | type=FAI/IPQC，机台核对快照；INS1 行＝检查项 |
| `MAND` | Andon 呼叫 | CallNo | `AND1` | 产线/机台/工单/类型/状态/来源/原因/措施；AND1 行＝处理时间线 |
| `MDWN` | 停机记录 | DowntimeCode | — | 机台/产线/工单/停机大类/计划/起止/来源 |
| `MSET` | 车间参数 | SettingKey | — | 覆盖服务端默认参数 |

另有租户库本地的 `smt_settings`（KV 覆盖）、`smt_edge_events`（入站事件收件箱，`UNIQUE(instance_key,event_id)`）与 `smt_edge_sync_log`（出站同步日志）。

复用（不新建）的既有码表：`MITM` 物料、`MCRD` 业务伙伴、`MWHS` 仓库、`MIGE`/`MIGN` 库存收发货、`MJDT` 会计凭证。物料收发既可用 SMT 料盘动作，也可继续走 ERP 库存单据。

## 三、业务动作 / Governed actions

全部通过 `POST /erp/{table}/{key}/actions/{action}` 执行，在租户事务 + ERP 动作账本（advisory lock、幂等键、执行历史）中完成。

| 动作 / Action | 语义 |
| --- | --- |
| `MREL.receive` | 收料入库：批量生成料盘（前缀＋日期＋随机序号）、REL1 流水、客供料标记 |
| `MREL.issue/return/empty/scrap/lost/return-to-customer/adjust` | 料盘状态机迁移 + 流水（活动上料中的料盘禁止转换） |
| `MLOD.load` | 上料防错（22 类拦截，固定顺序：工单→站位→锁定→料站表→料盘解析→料盘状态→客供料策略→发料归属→料号/替代料→站位占用 CONFIRM/接料/换料） |
| `MLOD.unload` | 卸料：料盘处置（用完/退库/留机台）、接料盘提升为主料、清除核对、清缺料预警 |
| `MMAC.bind` | 绑定/切换工单（换线）：记录换线清单快照、清核对与签核、联动换线停机 |
| `MMAC.verify/sign-off` | 整机核对 / 第二人签核（参数控制是否必须签核） |
| `MMAC.lock-slot/unlock-slot` | 站位锁定/解锁（错料自动锁定参数） |
| `MSWO.start/progress/finish/close` | 工单开工/报工（触发缺料估算与 MLWA/Andon）/完工/关闭 |
| `MRQN.issue/close/cancel` | 领料发料（扫料盘，写 RQN1 与工单用料缺口）/完成/取消 |
| `MUNT.pass/fail/repair/scrap` | 过站（21 类拦截）/判不良（不良码+位号，连续不良触发 Andon）/维修（回原工序复测）/报废 |
| `MPNL.bind/split` | 拼板绑定/分板（整板过站全有或全无；DEPANEL 工序通过时自动分板） |
| `MINS.record` | 首件/巡检登记（校验机台核对前提，存机台快照） |
| `MAND.raise/ack/close` | Andon 人工/自动呼叫（AUTO 按产线×类型×ref 去重）、响应、关闭（联动结束自动停机） |
| `MDWN.start/end` | 停机登记 |
| 配置表 | `MLIN/MMAC/MFSB/MOPR/MROU/MPRD/MCPM/MDFC/MSET` 允许通用 CRUD；`MREL/MLOD/MVER/MLWA/MUNT/MPNL/MUDF/MINS/MAND/MDWN` 禁止通用 CRUD，只能经动作产生（`erp/permissions.go` 的 `smtActionOnlyTables`） |

执行权限：`organization_admin`/`reviewer` 可执行全部动作；`executor` 额外放行车间高频动作（上料/卸料/过站/报工/Andon/停机/领料发料等，见 `smtExecutorActions`），治理动作（签核、解锁、报废、盘亏、调整、工单关闭、领料取消、客供料退回）保持 reviewer 级。

参数（MSET/smt_settings 覆盖）：`sn_regex`、`fai_required`、`fai_require_verified`、`signoff_required`、`signoff_role`、`pass_require_machine_ready`、`pass_limit_plan_qty`、`pass_auto_progress`、`andon_fail_threshold`、`andon_auto_material`、`downtime_from_andon`、`downtime_from_changeover`、`lock_on_mismatch`、`require_issued_to_wo`、`customer_material_policy`、`smt_low_material_threshold`、`low_qty_threshold`、`low_pcs_threshold`、`shift_definitions`、`reel_uid_prefix` 等。

## 四、专用服务域与前端 / Service domain and frontend

专用域 `backend/internal/domain/smtmes/`（model/repository/service/handler），全部路由在 `/api/v1/smt` 下并由模块 `smt_mes` 门控（`middleware/tenant.go` 的 `moduleForPath`）：

- `GET/PATCH /smt/settings`
- `GET /smt/machines/{code}/context`（站位状态唯一事实来源：OK/ALT/MISMATCH/EXTRA/EMPTY/NA + 主/接料盘 + 余量估算 + 就绪/待签核）
- `GET /smt/board`、`GET /smt/work-orders/{key}/progress`
- `GET /smt/trace/sn/{sn}`、`GET /smt/trace/reels/{uid}/units`
- `GET /smt/reports/{production|andon|downtime|materials}`（工序 FPY/RTY、Andon 响应与处理时长、停机可用率与 MTTR、物料收发）
- `GET/POST/PATCH/DELETE /smt/edges*`（实例管理、`push`、`pull`、事件日志、同步日志）

前端 `frontend/src/app/smt-workspace.tsx` 提供十个页签：上料工作站、过站采集、质量与维修（含 FAI/IPQC）、安灯、产线看板（15s 自动刷新）、追溯查询、生产报表、参数设置、边缘对接、基础数据（复用 ERP 模块工作台渲染 `smt_mes` 码表，配置表可编辑、事务表只读＋动作）。扫描反馈使用 WebAudio 蜂鸣与 `speechSynthesis` 语音（`frontend/src/lib/smt-feedback.ts`），操作员可用键盘楔扫码枪直接在输入框回车提交。

## 五、边缘对接契约 / Edge bridge contract

### 实例注册（平台库）

`platform.smt_edge_instances`：`instance_key`(UUID)、`organization_id`、`name`、`base_url`（SMT-MES 根地址，如 `http://127.0.0.1:8000`）、`api_key_cipher`（`MODEL_SECRET_KEY` 派生的 secretbox 加密）、`webhook_secret_cipher`、`subscribed_events`、`status`、`auto_push`、`last_poll_id`、`last_sync_at`、`last_event_at`。

### 出站（平台 → 边缘）

`POST /smt/edges/{key}/push`，scope：`materials`（MITM）、`customers`（MCRD + MCPM 客供料号）、`products`（MPRD + PRD1 BOM）、`station-tables`（MFSB + FSB1）、`work-orders`（MSWO），分别 POST 到 SMT-MES 的 `/api/v1/integration/{scope}`，携带 `X-API-Key`。工单动作：`POST /smt/edges/{key}/work-orders/{wo}/actions/{action}`（start/finish/close/progress）。每次调用记录 `smt_edge_sync_log`。工单放行时若实例 `auto_push` 开启则自动下发（失败不阻断业务）。

### 入站（边缘 → 平台）

SMT-MES 侧把 Webhook 指向 `POST /api/v1/smt/hooks/{instance}`（公共路由，自有鉴权）：

- 头部 `X-SMT-Signature: sha256=<HMAC-SHA256(webhook_secret, raw body)>`，平台用实例密钥验签（`hmac.Equal`）。
- 体为事件信封 `{"id","type","time","ref","data"}`；写入租户库 `smt_edge_events`（`UNIQUE(instance_key,event_id)` 去重，失败可重认领），随后在租户事务中经 `erp.ApplyEdgeEvent` 映射落账，镜像键为 `EDGE-{instance8}-{eventID}`（幂等）。
- 处理失败时返回 5xx，SMT-MES 按其退避策略重试。

轮询兜底：`POST /smt/edges/{key}/pull` 调 `GET {base}/api/v1/integration/events?since_id=&limit=`（返回 `{"items":[…],"last_id":N}`），逐条走同一映射并推进 `last_poll_id`。

事件映射覆盖 SMT-MES 的全部 39 种事件：`work_order.*`（进度/状态同步）、`machine.bound/verified/verify_failed/signed_off`、`loading.ok/error/unloaded`（含 MVER 扫描留痕）、`material.low`（MLWA/自动 Andon）、`reel.*`、`requisition.created/closed`、`unit.passed/failed/rejected/finished/repaired/scrapped`、`panel.bound/split`、`downtime.started/ended`、`inspection.recorded`、`andon.raised/acked/closed`、`ping`。

## 六、与原 SMT-MES 的行为差异 / Differences from the reference SMT-MES

- **交互载体**：原系统为 Vue3 免构建页面 + 桌面程序（货架灯控、设备文件采集后台线程）；平台版为 Next.js 工作区，扫码用键盘楔输入框、反馈用蜂鸣/语音，货架灯控、设备结果文件采集、Windows 服务式部署属于后续深化项。
- **权限**：原系统为 7 个内置角色；平台版复用 Meta-Org 的租户权限体系（organization_admin / reviewer / executor + 模块门控）。
- **库存口径**：原系统料盘库存独立自洽；平台版料盘是车间库存事实来源，同时可与 ERP 库存单据（MIGE/MIGN/MITW）并行，避免两套口径互相覆盖。
- **事件归属**：平台版通过 `source` 字段区分 `MANUAL` 与 `EQUIPMENT`/边缘来源，报表可按来源区分人工与设备数据。
- **数据模型**：原系统 45+ 关系表；平台版为 21 主表 + 13 子表 + 2 集成表，payload 保留域字段以对齐原系统语义（字段名沿用 snake_case 域键，如 `part_no`/`slot_no`/`work_order`）。

## 七、部署与迁移 / Deployment and migration

1. 平台库与每个租户库执行迁移 `036`（含 `tenant/011`）与 `037`（仅平台）；新迁移文件为纯新增，不改动基线，无需 checksum drift 接受。
2. 组织必须为 `dedicated_database` 模式（ERP 强制）；SMT 解决方案启用模块 `erp, inventory, manufacturing, smt_mes`。
3. 应用解决方案：平台管理 → 组织 → 行业方案流程（`smt-mes`），走 verify → approve → apply；或由系统管理员直接启用模块。
4. 存量迁移：先在「边缘对接」注册既有 SMT-MES 实例，下发主数据/工单；SMT-MES 继续承担现场执行并通过 Webhook 回传；随后把现场操作逐步切换到平台工作区，事件收件箱保证双向数据不丢。
5. 现场验证（本地）：`docker compose up --build` → 平台管理登录（`platform-admin@local.test`）→ 新建 dedicated 组织 → 应用方案 → 建产线/机台站位/工序/路线/料站表/产品 BOM/工单 → 收料 → 领料 → 上料（含接料/换料/整机核对/签核）→ 首件 → 过站/判不良 → 维修 → 报工 → 看板/追溯/报表。

## 八、代码入口 / Implementation map

| 文件 / File | 职责 / Responsibility |
| --- | --- |
| `migrations/036_smt_mes_solution.sql` | 码表 MREG 注册与建表、`smt_settings`、边缘收件箱/同步日志、模块与计划映射 |
| `migrations/037_smt_edge_platform.sql` | 边缘实例表、行业目录/资产、能力包与市场条目、`platform.runtime_operations` 种子 |
| `migrations/tenant/011_smt_mes_solution.sql` | 租户库 include 036 |
| `backend/internal/domain/erp/catalog.go` | 21 张 SMT 码表与 `smt_mes` 模块定义 |
| `backend/internal/domain/erp/actions.go` | 34 个车间动作注册 |
| `backend/internal/domain/erp/smt_actions.go` | 参数、料盘、工单、领料、机台核对、Andon、停机、拼板等动作实现与调度 |
| `backend/internal/domain/erp/smt_loading.go` | 上料防错链、料盘解析、卸料与缺料预警 |
| `backend/internal/domain/erp/smt_production.go` | 过站防错链、判不良/维修/报废、拼板过站 |
| `backend/internal/domain/erp/smt_api.go` | 站位状态、看板、追溯、生产/Andon/停机/物料报表 |
| `backend/internal/domain/erp/smt_edge.go` | 39 类边缘事件的映射与幂等镜像 |
| `backend/internal/domain/erp/permissions.go` | action-only 表集合、executor 动作白名单、模块字段保护 |
| `backend/internal/domain/ontology/catalog.go` | SMT 对象类型（属性/链接/动作/历史）供 agent 面使用 |
| `backend/internal/domain/systemadmin/service.go` | `BuildSMTMESSolutionFlow` / `BuildSMTMESSolutionManifest`（业务功能、工艺闭环、验证场景、上下文规则、技能与工具） |
| `backend/internal/domain/smtmes/` | 专用服务域（设置、读接口、边缘实例、Webhook 验签与入库、推送、轮询） |
| `frontend/src/app/smt-workspace.tsx` | 车间工作区（十个页签） |
| `frontend/src/app/erp-business-module-workspace.tsx` | `smt_mes` 模块的单据/字段/只读与动作声明 |
| `frontend/src/lib/api.ts` | SMT API 客户端 |
| `frontend/src/lib/smt-feedback.ts` | 蜂鸣与语音反馈 |
| `frontend/src/lib/i18n.tsx` / `i18n.en.ts` | 全量中英文文案（含动作名与参数名） |
