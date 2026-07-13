# Meta-Org - AI 原生组织操作平台

[English](README_EN.md) | 简体中文

Meta-Org 是一个面向混合人力组织的 AI 原生组织操作平台。它把人类员工、AI Agent、外部协作者、组织结构、项目交付、治理规则和持续学习机制放进同一套运行系统中，用于支持从需求进入、项目组建、工作流执行、交付验收、成本归集到反馈沉淀的完整业务闭环。

项目基于 **ETCLOVG** 框架构建：Execution、Tooling、Context、Lifecycle、Observability、Verification、Governance。当前仓库已经包含 Go 后端、Next.js 前端、PostgreSQL 迁移、Docker Compose 编排、JWT 鉴权、Meta-Org 首页、Meta Resource / PDCA 工作台、组织/项目工作台、模型设置、AI Gateway、工具运行闭环、成本核算、通用财务、SaaS 模块门控、安全内核接入，以及库存、采购、销售供应链闭环。

## 项目目标

Meta-Org 要解决的问题不是单点任务管理，而是“组织如何在 AI Agent 参与后持续可靠地运转”：

- **人类与 AI Agent 同域管理**：用户、Agent、外部成员都作为组织参与者接入身份、角色、岗位、权限和项目分工。
- **组织结构可执行化**：部门、岗位、岗位任命、MVRU、工作流模板和项目成员不是静态目录，而是调度、授权和评估的依据。
- **需求到反馈闭环**：需求可以上传材料、进入分析工作流、审批、转项目、绑定成员和工作流、管理交付物、记录成本并关闭反馈。
- **治理内嵌到流程**：权限原则、控制规则、访问决策、风险等级和决策权重在项目关键动作中参与判断。
- **自进化能力沉淀**：通过权重计算、执行结果、实验、知识库和信号机制，让组织运行经验能够被记录和再利用。

## 核心概念

| 概念 | 说明 |
|---|---|
| ETCLOVG | Execution、Tooling、Context、Lifecycle、Observability、Verification、Governance 七类组织运行能力。 |
| AI Agent 一等公民 | Agent 有独立身份、权限等级、能力、来源、服务商、风险等级和元数据，可以参与项目与工作流。 |
| MVRU | Minimal Viable Reconfigurable Unit，最小可重组组织单元，用于承载可调整的组织结构、成员和关系。当前 API 路径沿用 `/muvrs`。 |
| P-E-R 工作流 | Planner、Executor、Reviewer 三类阶段组成的工作流模板与实例，支持任务、决策和上下文记录。 |
| Meta Resource | 从 meta 角度统一索引人类、外部人类、Agent、模型通道、工具、物料、时间、能力和预算等资源，记录能力、成本、容量和风险画像。 |
| Demand Profile | 需求的目标、验收标准、能力要求、预算/时间/风险约束和候选资源适配结果。 |
| PDCA 循环 | 围绕需求画像按 Plan、Do、Change、Accept 记录计划、行动、调整和接受/验收事件。 |
| 决策权重 | 结合能力、历史结果、风险、组织上下文等因素，为人类或 Agent 计算可信度和决策权重。 |
| 治理访问决策 | 基于权限、治理原则、控制规则、风险等级、所需权限级别和权重快照生成访问判断。 |
| 能力匹配 | 按能力、风险、上下文和候选对象，匹配合适的人类成员、Agent 或能力资源。 |
| 自进化闭环 | 感知信号、实验、验证、知识沉淀和权重更新共同形成持续优化机制。 |

## 当前能力总览

### 业务闭环

系统当前支持一条从需求到反馈的项目生命周期：

1. **需求进入**：创建需求，记录组织、部门、提交人、优先级、风险等级、预算和元数据。
2. **材料与分析**：上传需求文档，启动需求分析工作流，同步工作流输出，沉淀分析结果。
3. **需求审批**：由人类或 Agent 作为 actor 执行审批，审批动作可以触发治理校验和结果记录。
4. **项目转化**：将已审批需求转为项目，保留组织、部门、预算、风险和上下文。
5. **项目组建**：添加项目成员，关联组织岗位或岗位任命，按能力和风险匹配参与者。
6. **流程绑定**：为项目绑定工作流模板，创建工作流实例，跟踪任务、决策和上下文。
7. **交付管理**：创建、更新、提交、验收或拒绝交付物。
8. **成本管理**：记录成本条目，刷新成本汇总，按来源类型聚合预算消耗。
9. **反馈评估**：创建项目评价，关闭反馈，向演化域记录执行结果和可学习信号。

### 组织能力

- 多组织模型和当前组织查询。
- 部门树、部门状态、排序和元数据。
- 岗位、岗位权限级别、岗位所需能力和岗位任命。
- 人类用户、AI Agent、外部成员三类组织成员。
- 部门与 MVRU 关联。
- 组织成员、项目成员和岗位任命之间的连接。
- 面向任务和项目的成员匹配、能力匹配。

### 治理与演化能力

- 权限、原则、控制规则和访问决策记录。
- AI Agent 来源、服务类别、供应商、合同引用、风险等级等治理字段。
- 员工画像、上下文权重、能力评估和访问决策数据结构。
- 权重计算、上下文权重计算、结果回写、实验、知识条目和信号确认。

### AI 运行、工具与财务

- Meta-Org 首页聚合组织健康、项目状态、Agent 状态、AI 成本、风险、近期事件和待办收件箱。
- AI Gateway 支持 OpenAI、Anthropic、Gemini 三类模型供应商配置、加密密钥、模型目录、流式调用、调用日志和成本汇总。
- Tool Runtime 支持工具注册、治理决策、审批策略、执行审计和内部工具调用。
- Meta Resource 支持同步现有人类、外部成员、Agent、模型通道、工具和能力资源，统一沉淀能力、成本、容量和风险画像。
- Demand Profile 和 PDCA Cycle 支持把需求约束、资源适配、计划、执行、改变和接受事件显式记录为可查询对象。
- 模型设置提供模型供应商、模型目录、工具注册表、接口文件、调用日志和成本汇总视图。
- 财务导出支持通用财务适配器、HMAC/Bearer 鉴权、导出批次、Webhook 回调和对账差异。

### 前端工作台

前端是一个面向实际操作的单页工作台，而不是营销页：

- 登录、注册、会话保存和退出。
- 中英文语言切换，使用 `LanguageProvider` 和 `useI18n`。
- 系统总览 Dashboard，展示身份、组织、工作流、能力、观测、验证、治理、演化统计和近期事件。
- Meta-Org Home，展示组织健康、AI 成本、风险、收件箱和上下文 AI 助手。
- Meta Resource Workspace：资源总览、现有资源同步、需求画像、PDCA 循环和事件记录。
- 可拖拽的侧边菜单分组：业务闭环、组织能力、治理演进、系统工具。
- 组织工作台：组织、部门、岗位、成员、外部成员、岗位任命、MVRU 关联和匹配。
- 控制工作台：治理、权重、能力评估、工作流设计、工作流匹配。
- 项目生命周期工作台：需求、项目、交付、成本和反馈。
- 模型设置：模型供应商、模型目录、工具注册表、接口文件、调用日志和成本汇总。
- Finance Exports：财务适配器、导出批次、对账和失败回调。
- 上下文 AI 助手：支持 Meta-Org、组织、项目、治理和模型设置场景的流式调用与成本展示。
- API Workbench：按域浏览和调用后端 API，支持路径参数、查询参数、请求体模板和认证 Token。

## 技术架构

| 层 | 当前实现 |
|---|---|
| 前端 | Next.js 16 App Router、React 19、TypeScript、Tailwind CSS、lucide-react、@xyflow/react |
| 后端 | Go 1.22、Chi Router v5、领域模块化单体、pgx PostgreSQL 驱动 |
| 数据库 | PostgreSQL 16，根目录 SQL migrations，后端启动时自动执行 |
| 鉴权 | JWT Bearer Token；公开路由与受保护业务路由分组注册 |
| 部署 | Docker Compose 启动 PostgreSQL、backend、frontend |

### 后端结构

后端入口是 `backend/cmd/server/main.go`。启动流程：

1. 读取环境配置。
2. 连接 PostgreSQL。
3. 执行 `migrations/` 下的 SQL 迁移。
4. 初始化各领域 repository、service、handler。
5. 在 `backend/internal/gateway/router.go` 注册 `/api/v1` 路由。
6. 启动 HTTP 服务并支持优雅关闭。

后端领域按 `backend/internal/domain/<domain>/` 组织，通常包含：

- `model.go`：API 和数据库模型。
- `repository.go`：PostgreSQL 持久化。
- `service.go`：业务规则和跨域编排。
- `handler.go`：HTTP 参数解析和响应。

共享包位于 `backend/internal/pkg/`，包含配置、数据库、迁移、中间件和服务器封装。

### 后端领域

| 领域 | 主要职责 |
|---|---|
| `identity` | 用户、AI Agent、角色、登录、注册、Agent 鉴权。 |
| `organization` | 组织、部门树、岗位、岗位任命、外部成员、组织成员、MVRU、关系和匹配。 |
| `layer` | 战略、战术、执行层分类和 MVRU 分层配置。 |
| `capability` | 能力目录、能力绑定、能力匹配、能力评估。 |
| `dashboard` | 聚合各域统计和近期事件，提供系统总览。 |
| `metaorg` | 聚合 Meta-Org 首页、组织健康、风险、活动和收件箱。 |
| `metaresource` | 统一资源画像、需求画像、PDCA 循环和事件记录。 |
| `aigateway` | 模型供应商、模型目录、流式调用、调用日志和 AI 使用成本。 |
| `toolruntime` | 工具注册、治理策略、审批、执行审计和内部工具适配。 |
| `finance` | 通用财务适配器、导出批次、Webhook 回调和对账。 |
| `workflow` | 工作流模板、实例、任务、决策和上下文。 |
| `project` | 需求、文档、需求分析工作流、项目、成员、项目工作流、交付、成本、反馈。 |
| `governance` | 权限、治理原则、控制规则、权限检查和访问决策。 |
| `evolution` | 决策权重、上下文权重、实验、知识库、信号和结果回写。 |
| `observability` | Trace、Span、Metric 和执行遥测。 |
| `verification` | 验证报告、评审分配、评审完成和评分。 |

### 前端结构

| 路径 | 说明 |
|---|---|
| `frontend/src/app/page.tsx` | 应用主入口、登录注册、布局、总览、菜单和工作区切换。 |
| `frontend/src/app/organization-workspace.tsx` | 组织、部门、岗位、成员、外部成员和 MVRU 相关操作。 |
| `frontend/src/app/control-workspaces.tsx` | 治理、权重、能力评估、工作流设计和工作流匹配工作区。 |
| `frontend/src/app/project-lifecycle-workspace.tsx` | 需求、项目、交付、成本和反馈工作区。 |
| `frontend/src/app/meta-resource-workspace.tsx` | Meta Resource、Demand Profile 和 PDCA 循环工作区。 |
| `frontend/src/app/api-workbench.tsx` | 通用 API 调用面板。 |
| `frontend/src/app/ai-assistant.tsx` | 上下文 AI 助手和 SSE 流式响应面板。 |
| `frontend/src/app/developer-tools-workspace.tsx` | 模型、工具、接口文件、调用日志和成本视图。 |
| `frontend/src/app/finance-workspace.tsx` | 财务适配器、导出批次、对账和失败回调视图。 |
| `frontend/src/lib/api.ts` | API 请求封装、基础类型和 Dashboard 数据结构。 |
| `frontend/src/lib/operations.ts` | API Workbench 的域、路径、参数和请求体模板。 |
| `frontend/src/lib/i18n.tsx` | 中英文语言包和 i18n Provider。 |
| `frontend/src/lib/auth.ts` | Token 与会话存储。 |

## 数据库迁移

后端启动时会按文件名顺序执行根目录 `migrations/` 中的 SQL 文件。当前版本只保留重整后的阶段基线：

| 迁移 | 主题 |
|---|---|
| `000_saas_platform_management_baseline.sql` | SaaS 管理平台、平台账号、权限治理、订阅、模块、租户和平台主从数据基础。 |
| `001_erp_code_baseline.sql` | ERP/行业业务基线，包含组织、项目、工作流、财务、成本，以及以 ERP code-table 为主模型的供应链和行业解决方案表。 |
| `002_erp_platform_integration_baseline.sql` | ERP 与平台管理的运行期投影、模块集成和平台主数据同步。 |
| `004_ai_capability_baseline.sql` | 模型、provider/channel、agent、工具运行时、AI 助手、业务五阶段 AI 审计、上下文、skill、AI 用量，以及跨阶段外键重建。 |
| `005_industry_solution_consolidation.sql` | 行业方案存储和历史方案数据归并。 |
| `006_saas_manufacturing_module_seed.sql` | 制造模块及 ERPNext 风格行业方案种子。 |
| `007_saas_runtime_organization_target_repair.sql` | SaaS 运行组织和租户数据库目标修复。 |
| `008_ai_gateway_model_group_repair.sql` | 兼容性修复：为已应用旧版 `004` 的本地/开发库补齐 AI Gateway 模型组、访问令牌和余额表。 |
| `009_platform_tenant_data_permissions.sql` | 平台跨租户读取和管理权限修复，确保审计员只读。 |
| `010_tenant_database_provisioning_jobs.sql` | 持久化租户数据库开通作业、租约领取、失败重试和存量目标修复。 |
| `011_ai_module_master_detail_runtime_repair.sql` | 补齐 AI Gateway、Tool Runtime、Assistant 跨阶段主从表、源键与投影触发器。 |
| `012_tenant_database_target_state_repair.sql` | 防止已开通租户目标被重复 onboarding 降级，并修复“作业成功但目标仍 provisioning”的存量状态。 |
| `013_tenant_event_projection_infrastructure.sql` | 平台事件 inbox、租户运营/工作流/活动投影，以及 Dashboard/Meta-Org 跨库读模型。 |
| `014_platform_migration_checksum_governance.sql` | 平台迁移 checksum、受控漂移修复和不可变审计历史。 |
| `015_authentication_rate_limit_buckets.sql` | 多实例共享的认证限流、失败计数和封禁 bucket。 |
| `016_business_stage_ai_runs.sql` | Plan/Do/Change/Accept/Learn 项目 AI 分析调用、结构化结果、成本和提案审计。 |
| `017_business_ai_tool_proposal_execution.sql` | AI 提案到 Tool Runtime 强制审批、幂等执行及结果回写闭环。 |

租户库按 `migrations/tenant/` 独立迁移：`001_tenant_business_baseline.sql` 建立业务基线，`002_tenant_projection_outbox.sql` 建立带租约、重试和发布状态的事务 outbox。

阶段原则：先有 SaaS 管理平台，再由平台创建或调整行业解决方案，最后落到 ERP 基线和 AI 能力基线。未来任何数据库结构、表关系、外键、索引、种子数据或 schema 生成逻辑调整，都必须同步更新对应阶段 SQL 和 `migrations/BASELINE_RESTRUCTURE.md`。

`000` baseline 会在 SaaS 管理平台目录中写入 `erpnext_manufacturing_demo` 行业解决方案样例，包含样例租户库模板、ERPNext 风格 BOM/工单工作台和制造业闭环操作元数据。物理样例库仍由租户数据库 provisioning 或后续维护任务创建，不在 SQL baseline 事务中直接创建。

## SaaS 分库命名规则

- SaaS 管理平台库固定为 `meta_org_saas`。
- 租户物理业务库固定为 `meta_org_xxxx`，其中 `xxxx` 是租户组织 UUID 去掉连字符后的前 4 位小写 hex。
- `TENANT_DATABASE_NAME_PREFIX` 默认值为 `meta_org_`。
- `TENANT_DATABASE_ADMIN_URL` 应指向同一 PostgreSQL 实例上的管理库，例如 `postgres`，用于创建和迁移租户库。
- 租户库必须通过后端 tenant database provisioner 或等价的 tenant migrator 创建；不要直接用 `psql -f migrations/tenant/001_tenant_business_baseline.sql` 创建新租户库，因为该文件包含 `tenantdb:include`，直接执行不会展开 ERP/Finance 基线。
- 旧单库 `meta_org` 不再作为活动运行库；如需迁移，只能作为明确命名的备份或迁移源，再同步到新建并迁移完成的 `meta_org_saas`。
- SaaS 管理台维护租户组织、组织所有者、组织账号状态、组织账号权限层级和密码重置。普通账号只能通过自助改密接口修改自己的密码。

## ERP code-table 供应链规范

- 租户业务库中的采购、销售、库存、仓库和零售行业方案统一以 ERP code-table 工作台为主模型。
- 采购订单使用 `MPOR/POR1`，销售订单使用 `MRDR/RDR1`，仓库库存余额使用 `MITW/ITW1`，库存收发使用 `MIGN/IGN1` 和 `MIGE/IGE1`。
- 零售配送闭环使用 `MRPS/RPS1`、`MDRQ/DRQ1`、`MDSP/DSP1`、`MDRC/DRC1`、`MDIF/DIF1`、`MSTP/STP1`、`MCNT/CNT1` 和 `MSPR/SPR1`。
- fresh tenant baseline 不再创建旧语义供应链表，例如 `inventory_counts`、`inventory_transfers`、`purchase_orders`、`sales_shipments`、`inventory_balances`、`sales_orders`。
- 旧库存/采购/销售语义 API 若继续保留，只能作为兼容层或迁移入口；新增行业方案、UI 工作台、Agent API 和 demo 验证必须优先走 ERP code-table。

## API 概览

所有 API 默认挂载在 `/api/v1` 下。

公开接口：

- `GET /health`
- `POST /auth/login`
- `POST /auth/register`
- `POST /agents/auth`
- `GET /roles`

其余业务接口通过 JWT Bearer Token 保护。

`/api/v1` 错误响应统一保留兼容字段 `error`，并返回稳定的 `code` 和 `request_id`；相同请求 ID 同时写入 `X-Request-ID`，错误码写入 `X-Error-Code`。客户端应按 `code` 分支处理，使用 `request_id` 关联服务日志，不要解析自由文本错误。OpenAI 兼容 `/v1` 接口继续遵循其独立协议；`POST /v1/chat/completions` 支持 `stream: true` 的标准 SSE chunk 与 `[DONE]` 终止标记，并进入访问令牌授权、余额预留、实际用量结算和调用审计链路。

| 域 | 主要接口 |
|---|---|
| Dashboard | `GET /dashboard/overview` |
| Meta-Org | `GET /meta-org/overview`, `GET /meta-org/inbox` |
| Meta Resource | `GET/POST /meta-resources`, `POST /meta-resources/sync-existing`, `GET /meta-resources/summary`, `GET/POST /demand-profiles`, `GET/POST /pdca-cycles`, `GET/POST /pdca-events` |
| Identity | `POST /agents/register`, `GET /agents` |
| AI Gateway | 模型供应商、通道/key 池、模型目录、多维计费、路由规则、`POST /ai-gateway/invoke`、`GET/POST /ai-gateway/stream`、调用日志、用量分析和成本汇总接口 |
| Tool Runtime | 工具定义、工具测试、工具执行日志和工具审批接口 |
| Finance | 财务适配器、导出批次、提交导出、Webhook 回调和对账接口 |
| Organization | `GET/POST/PATCH /organizations`, `GET /organization/current`, 部门、部门树、岗位、岗位任命、组织成员、外部成员、MVRU、关系、成员匹配和能力匹配接口 |
| Layer | `POST /layers/classify`, `GET/PUT /layers/config/{mvruId}`, `GET /layers/rules` |
| Capability | `GET/POST /capabilities`, `GET /capabilities/{id}`, `POST /capabilities/match`, 能力评估、绑定和解绑接口 |
| Workflow | 工作流模板、实例、状态、任务完成、决策记录和上下文读写接口 |
| Project Lifecycle | 需求、需求文档、需求分析工作流、审批、转项目、项目成员、项目工作流、项目总览、交付物、成本、反馈接口 |
| Governance | 权限、原则、控制规则、权限检查、访问决策和访问决策列表接口 |
| Evolution | 权重计算、结果回写、上下文权重、alpha 配置、实验、知识、信号和信号确认接口 |
| Observability | Trace、Span、Trace 完成、Metric 写入和查询接口 |
| Verification | 验证报告、报告查询、评审分配和评审完成接口 |

前端的 API Workbench 元数据位于 `frontend/src/lib/operations.ts`，它按 MetaOrg、MetaResource、DeveloperTools（模型设置）、Finance、Dashboard、Identity、Organization、Layer、Capability、Workflow、Observability、Verification、Governance、Evolution、Requirement、Project、Delivery、Cost、Feedback 等操作域组织。

## 快速开始

使用 Docker Compose 启动完整环境：

```bash
docker compose up --build
```

服务地址：

- PostgreSQL：`localhost:5432`
- Go API：`http://localhost:8080`
- API health：`http://localhost:8080/api/v1/health`
- Next.js 前端：`http://localhost:3000`

默认 Docker 环境变量见 `docker-compose.yml`：

- 数据库：`postgres://postgres:postgres@postgres:5432/meta_org_saas?sslmode=disable`
- 后端端口：`8080`
- 模型与财务密钥加密：`MODEL_SECRET_KEY=0123456789abcdef0123456789abcdef`
- 前端 API 地址：`http://localhost:8080/api/v1`

## 本地开发

后端：

```bash
cd backend
go run ./cmd/server
go test ./...
go build ./cmd/server
```

如果不通过 Docker 运行后端，需要准备 PostgreSQL，并设置：

```bash
set MIGRATIONS_PATH=../migrations
```

PowerShell 可使用：

```powershell
$env:MIGRATIONS_PATH = '../migrations'
go run ./cmd/server
```

前端：

```bash
cd frontend
npm install
npm run dev
npm run lint
npm run build
npm run test:e2e
```

`npm run test:e2e` 使用 Playwright 验证桌面与移动端的平台/租户登录、会话作用域和横向溢出；执行前需启动 `8080` 后端和 `3000` 前端。CI 会自动安装 Chromium 并启动完整测试环境。

前端默认读取：

```bash
NEXT_PUBLIC_API_URL=http://127.0.0.1:8080/api/v1
```

### Windows 本机重启注意事项

如果 `docker compose up --build` 提示 `docker` 命令不可用，可以使用本机 PostgreSQL、Go 和 Node 启动开发服务。先确认 PostgreSQL 可连接，再分别启动后端和前端。

本项目在 Windows PowerShell 中通过 `Start-Process -ArgumentList` 后台启动时，不要在嵌套命令里写 `$env:NAME="value"`。外层 PowerShell 可能提前解析 `$env:`，导致子进程实际收到 `=value` 或未加引号的 URL/路径，常见报错包括：

- `migrations failed: read migrations dir: open migrations: The system cannot find the file specified.`
- `../migrations` 或 `http://localhost:8080/api/v1` 被当作命令执行。

推荐使用 `Set-Item Env:` 设置环境变量：

```powershell
Start-Process -FilePath "powershell" -ArgumentList @(
  '-NoProfile',
  '-Command',
  'Set-Item Env:MIGRATIONS_PATH ../migrations; Set-Item Env:SERVER_PORT 8080; Set-Item Env:DATABASE_URL postgres://postgres:postgres@localhost:5432/meta_org_saas?sslmode=disable; Set-Item Env:PLATFORM_DATABASE_URL postgres://postgres:postgres@localhost:5432/meta_org_saas?sslmode=disable; Set-Item Env:TENANT_DATABASE_ADMIN_URL postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable; Set-Item Env:TENANT_DATABASE_NAME_PREFIX meta_org_; go run ./cmd/server'
 ) -WorkingDirectory "D:\project\meta-org-saas\backend" -WindowStyle Hidden -RedirectStandardOutput "D:\project\meta-org-saas\backend-dev.log" -RedirectStandardError "D:\project\meta-org-saas\backend-dev-err.log"

Start-Process -FilePath "powershell" -ArgumentList @(
  '-NoProfile',
  '-Command',
  'Set-Item Env:NEXT_PUBLIC_API_URL http://localhost:8080/api/v1; npm run dev'
 ) -WorkingDirectory "D:\project\meta-org-saas\frontend" -WindowStyle Hidden -RedirectStandardOutput "D:\project\meta-org-saas\frontend-dev.log" -RedirectStandardError "D:\project\meta-org-saas\frontend-dev-err.log"
```

验证方式：

```powershell
Get-NetTCPConnection -LocalPort 3000,8080 -ErrorAction SilentlyContinue |
  Select-Object LocalAddress,LocalPort,State,OwningProcess

Invoke-WebRequest -Uri http://127.0.0.1:3000 -UseBasicParsing -TimeoutSec 8 |
  Select-Object StatusCode

Invoke-WebRequest -Uri http://127.0.0.1:8080/api/v1/health -UseBasicParsing -TimeoutSec 8 |
  Select-Object StatusCode,Content
```

成功状态应为前端 `3000` 和后端 `8080` 都处于 `Listen`，前端返回 HTTP `200`，后端 health 的 `status`、`platform_database.status` 和 `security_kernel.status` 均为 `ok`，并在 `tenant_database_pools` 中返回无租户标识的连接池聚合指标。如果需要停止旧进程，先用上面的端口查询确认 `OwningProcess`，再对单个 PID 执行 `Stop-Process -Id <PID> -Force`。

AI Gateway、Meta Resource、SaaS、安全内核和 ERP code-table 工作台启动时必须确认阶段基线与兼容性修复 `000/001/002/004/008` 都已执行。若后端启动、模型设置、Meta Resource、SaaS 模块或 ERP 工作台出现 `column ... does not exist`、`relation model_provider_channels does not exist`、`relation ai_routing_rules does not exist`、`relation ai_model_groups does not exist`、`relation meta_resources does not exist`、`relation tenant_modules does not exist`、`relation security_policies does not exist`，或缺少 `MITW`、`MPOR`、`MRDR`、`MRPS`、`MDRQ` 等 ERP code-table 关系，通常是 `MIGRATIONS_PATH` 指向错误、连接到了旧数据库，或迁移尚未执行。若出现 `relation platform.database_maintenance_jobs does not exist` 或 `relation platform.tenant_database_targets does not exist`，通常是后端仍连接旧 `meta_org` 或不完整的平台库。处理顺序：

1. 确认 `DATABASE_URL` 和 `PLATFORM_DATABASE_URL` 指向当前 `meta_org_saas` 平台管理库。
2. 确认从 `backend/` 本地运行时使用 `MIGRATIONS_PATH=../migrations`。
3. 重启后端，让迁移器执行 `000_saas_platform_management_baseline.sql`、`001_erp_code_baseline.sql`、`002_erp_platform_integration_baseline.sql`、`004_ai_capability_baseline.sql` 和 `008_ai_gateway_model_group_repair.sql`。
4. 再打开模型设置，检查 Channels / Keys、Routing、Usage Analysis 页面是否能加载。

如果租户侧 Finance / ERP 接口出现 `relation gl_journal_entries does not exist`，检查当前组织对应的 `meta_org_xxxx` 租户库是否由 tenant migrator 创建。手工 `psql -f migrations/tenant/001_tenant_business_baseline.sql` 不会展开 `tenantdb:include`，会导致 ERP/Finance 表缺失。

如果 SaaS 管理台在为 `general` 行业组织启用 ERP 或行业方案时出现 `industry module policy denied update: module erp is not allowed by industry general`，说明平台行业包或代码层默认模块白名单未包含 ERP 入口模块。当前规范要求 `general` 可启用基础组织治理模块以及 `erp`、`finance`、`costing`、`inventory`、`procurement`、`sales`、`retail`、`manufacturing` 等 ERP/行业闭环模块；新增行业包时必须同步维护 baseline seed、后端策略校验和 SystemAdmin 前端入口。

如果浏览器显示 `Failed to fetch`，先验证 API 健康接口，再检查后端响应是否包含 `Access-Control-Allow-Origin`。Windows PowerShell 启动后端时，逗号分隔的 `CORS_ORIGINS` 必须作为一个字符串传入。
5. 打开 Meta Resource 工作区，先执行一次“同步现有资源”，确认 human、agent、external_human、model_channel、tool、capability 资源能进入统一资源视图。

## 配置

后端配置在 `backend/internal/pkg/config/config.go` 中读取：

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `SERVER_PORT` | `8080` | 后端监听端口。 |
| `DATABASE_URL` | `postgres://postgres:postgres@127.0.0.1:5432/meta_org_saas?sslmode=disable` | 兼容旧入口的 PostgreSQL 连接串；未设置 `PLATFORM_DATABASE_URL` 时也作为平台控制库连接串。 |
| `PLATFORM_DATABASE_URL` | 跟随 `DATABASE_URL` | SaaS 平台控制库连接串，保存平台管理、租户组织、能力包、市场和租户数据库目录。 |
| `TENANT_DATABASE_ADMIN_URL` | `DATABASE_URL` 同实例的 `postgres` 库 | 租户物理业务库创建/维护的管理连接串；SaaS onboarding 在 `dedicated_database` 模式下会用它尝试创建租户库。 |
| `TENANT_DATABASE_NAME_PREFIX` | `meta_org_` | 按租户组织生成物理业务库名的前缀；物理租户库名为 `meta_org_` 加租户组织 UUID 前 4 位小写 hex。 |
| `TENANT_DATABASE_MODE` | `dedicated_database` | 租户数据库目标模式；`dedicated_database` 为每租户物理库，`shared_schema` 为兼容的单库多 schema。 |
| `TENANT_DATABASE_DEFAULT_CLUSTER` | `local-primary` | 平台目录中默认租户数据库集群 key。 |
| `TENANT_DATABASE_DEFAULT_REGION` | `local` | 平台目录中默认租户数据库区域。 |
| `TENANT_DATABASE_POOL_MAX_ENTRIES` | `16` | 单个后端实例最多缓存的租户数据库连接池数；达到上限时回收无活跃租约的最久未使用池。 |
| `TENANT_DATABASE_POOL_MAX_CONNECTIONS` | `4` | 每个租户连接池的最大 PostgreSQL 连接数；应与实例数及数据库总连接预算一起规划。 |
| `TENANT_DATABASE_POOL_MIN_CONNECTIONS` | `0` | 每个租户连接池保持的最小连接数；默认不为非活跃租户预占连接。 |
| `TENANT_DATABASE_POOL_IDLE_SECONDS` | `900` | 无活跃查询或事务的租户池在回收前可空闲的秒数。 |
| `TENANT_DATABASE_POOL_SWEEP_SECONDS` | `60` | 后台扫描空闲租户池的周期秒数。 |
| `TENANT_DATABASE_CONNECTION_IDLE_SECONDS` | `300` | pgx 在单个租户池内回收空闲连接前的秒数。 |
| `TENANT_DATABASE_CONNECTION_LIFETIME_SECONDS` | `1800` | 单个租户数据库连接的最长生命周期秒数。 |
| `TENANT_PROJECTION_WORKER_ENABLED` | `true` | 启用租户 outbox 到平台投影的后台 Worker。 |
| `TENANT_PROJECTION_POLL_SECONDS` | `2` | 无事件时的轮询间隔。 |
| `TENANT_PROJECTION_LEASE_SECONDS` | `60` | 单批 outbox 事件租约时长。 |
| `TENANT_PROJECTION_RETRY_SECONDS` | `5` | 投影失败后的基础重试间隔。 |
| `TENANT_PROJECTION_BATCH_SIZE` | `100` | 每租户单次领取的最大事件数。 |
| `TENANT_PROJECTION_TARGET_LIMIT` | `100` | 每轮扫描的最大已开通租户数。 |
| `TENANT_PROJECTION_ACTIVITY_LIMIT` | `50` | 每租户保留的近期活动投影数。 |
| `TENANT_PROJECTION_MAX_ATTEMPTS` | `20` | outbox 事件最大处理尝试次数。 |
| `JWT_SECRET` | `dev-secret-change-in-production` | JWT 签名密钥，生产环境必须替换。 |
| `MODEL_SECRET_KEY` | `0123456789abcdef0123456789abcdef` | 32 字符密钥，用于模型供应商和财务适配器密钥加密，生产环境必须替换。 |
| `CORS_ORIGINS` | `http://localhost:3000,http://127.0.0.1:3000` | 允许访问 API 的前端来源。 |
| `TRUSTED_PROXY_CIDRS` | 空 | 可信反向代理 CIDR；仅来自这些地址的请求可使用 `X-Forwarded-For` / `X-Real-IP`。 |
| `AUTH_RATE_LIMIT_WINDOW_SECONDS` | `60` | 登录、Agent 认证和注册尝试统计窗口。 |
| `AUTH_RATE_LIMIT_MAX_ATTEMPTS` | `10` | 每个登录客户端或主体在窗口内的最大尝试数。 |
| `AUTH_RATE_LIMIT_FAILURE_THRESHOLD` | `5` | 触发账号或客户端临时封禁的认证失败数。 |
| `AUTH_RATE_LIMIT_BLOCK_SECONDS` | `300` | 限流或失败阈值触发后的封禁时长。 |
| `AUTH_REGISTRATION_MAX_ATTEMPTS` | `5` | 每客户端在窗口内允许的最大注册请求数。 |
| `AI_GATEWAY_INVOKE_TIMEOUT_SECONDS` | `60` | 非流式模型调用的部署级最大执行时间；实际时限取该值与 provider `timeout_ms` 中较小的正值。 |
| `AI_GATEWAY_STREAM_TIMEOUT_SECONDS` | `600` | AI Gateway 和 Assistant SSE 流的部署级最大生命周期；Gateway 流同时受 provider `timeout_ms` 约束，且不受全局 15 秒写超时截断。 |
| `AI_GATEWAY_RESERVATION_RECOVERY_ENABLED` | `true` | 启用 AI Gateway 未完成余额预留恢复 worker。 |
| `AI_GATEWAY_RESERVATION_STALE_SECONDS` | `1800` | 预留进入恢复队列前的最短年龄；必须大于流式调用上限。 |
| `AI_GATEWAY_RESERVATION_POLL_SECONDS` | `300` | 恢复 worker 扫描间隔。 |
| `AI_GATEWAY_RESERVATION_LEASE_SECONDS` | `60` | 多副本恢复任务租约时长。 |
| `AI_GATEWAY_RESERVATION_BATCH_SIZE` | `100` | 单次最多恢复的预留数量，上限 `1000`。 |
| `BUSINESS_AI_PROVIDER_TYPE` | 空 | 业务五阶段 AI 默认 provider 类型，例如 `openai`；也可在单次请求中指定。 |
| `BUSINESS_AI_MODEL` | 空 | 业务五阶段 AI 默认模型键；必须对应 AI Gateway 中已启用的模型。 |
| `BUSINESS_AI_MAX_TOKENS` | `1800` | 单次业务阶段结构化分析允许的最大输出 token。 |
| `MIGRATIONS_PATH` | `migrations` | SQL 迁移目录；本地从 `backend/` 运行时通常设为 `../migrations`。 |
| `META_ORG_ENVIRONMENT` | `development` | 部署环境：`development`、`test` 或 `production`。生产模式会在连接数据库前拒绝默认开发密钥和无效运行参数。 |
| `META_ORG_MODE` | `single_org` | 运行模式；可设为 `saas` 启用多租户/SaaS 语义。 |
| `META_ORG_DISTRIBUTION_MODE` | 跟随 `META_ORG_MODE` | 分发模式：`saas`、`saas_org_private`、`single_org_commercial` 或 `private_deployment`。 |
| `META_ORG_LICENSE_MODE` | `commercial` | 授权模式：`community`、`commercial`、`enterprise` 或 `private_contract`。 |
| `SECURITY_KERNEL_URL` | 空 | 外部安全内核地址；SaaS 模式必填，缺失时 `blocking` 模式拒绝受保护操作。 |
| `SECURITY_KERNEL_SHARED_SECRET` | 空 | 请求 HMAC 共享密钥；配置 URL 后必填，安全内核自身也拒绝空密钥启动。 |
| `SECURITY_KERNEL_MAX_CLOCK_SKEW_SECONDS` | `60` | 签名时间戳允许的最大时钟偏差，也是 nonce 防重放保留窗口。 |
| `SECURITY_KERNEL_ENFORCEMENT_MODE` | `blocking` | 安全内核执行模式；支持 `blocking` 和 `audit`。 |

SaaS 模式下可通过 `META_ORG_PLATFORM_ADMIN_EMAIL` 和 `META_ORG_PLATFORM_ADMIN_PASSWORD_HASH` 初始化平台管理员。密码哈希可在 `backend/` 下生成：

```powershell
$env:BCRYPT_PASSWORD = '<your-admin-password>'
go run ./cmd/bcrypt-hash
```

也可以使用管道输入：`'<your-admin-password>' | go run ./cmd/bcrypt-hash -stdin`。不要提交明文密码或生产环境 hash。

前端配置：

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `NEXT_PUBLIC_API_URL` | `http://127.0.0.1:8080/api/v1` | 浏览器端调用的 API 基础地址。 |

## 项目结构

```text
backend/
  cmd/server/                 后端入口
  internal/domain/            领域模块
  internal/gateway/           路由注册
  internal/pkg/               配置、数据库、迁移、中间件、server
frontend/
  src/app/                    Next.js App Router 页面和工作台
  src/lib/                    API、认证、i18n、API Workbench 元数据
migrations/                   PostgreSQL SQL 阶段基线 000、001、002、004
docs/operations/              生产运维、财务适配器协议和排障文档
.github/workflows/            GitHub Actions CI
docker-compose.yml            本地完整环境编排
```

## 当前状态与边界

当前代码已经具备单企业 Meta-Org 入口、Meta Resource / PDCA 资源框架、组织管理、项目生命周期、AI Gateway、工具运行闭环、成本核算、通用财务、SaaS 模块门控、安全内核、库存/采购/销售供应链、治理、演化、观测和验证骨架，适合作为 10-50 人团队与 50-250+ Agent 的生产 v1 基础。

从旧 `harness_org` 或 `meta_org` 数据库升级到 `meta_org_saas` 时，必须先显式备份并迁移数据；系统不会自动删除或覆盖旧库。

需要继续增强的方向：

- 扩展更多模型能力、Agent 执行器和外部工具运行时。
- 将 MVRU 沙箱执行从数据模型扩展为可隔离执行环境。
- 为关键前端状态和端到端业务场景补充自动化测试。
- 完善生产级密钥管理、审计报表、告警和权限策略可视化。
- 扩展多组织租户边界、审批流模板和更细粒度的操作审计。
