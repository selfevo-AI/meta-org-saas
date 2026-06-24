# Phase 2: 平台行业方案工厂设计

## Summary

Phase 2 的目标是把 SystemAdmin 从“配置后台”推进到“行业解决方案工厂”。平台管理员应能为组织生成 ERP 标准行业方案包，查看方案资产和目标组织当前状态的差异，执行 schema change verify，通过审批后应用，并在同一个控制面看到应用状态、失败原因和发布门禁。

本阶段不重复建设 Phase 1 已完成的 ERP action 执行内核，也不提前实现 Phase 3 的租户业务文档工作台、Phase 4 的完整 Verified Context 迁移、Phase 5 的夜间 Monitoring Agent。Phase 2 的核心交付是平台侧的 package -> diff -> verify -> approve/apply -> publish gate 闭环。

## Current Context

当前代码已经具备较强基础：

- `POST /platform/admin/organizations/{id}/industry-solution-flows/erp-standard` 已能生成 ERP standard schema change request。
- ERP standard package metadata 已覆盖 `database_assets`、`business_functions`、`process_loops`、`permissions`、`api_operations`、`ui_workspaces`、`assistant_targets`、`context_rules`、`tool_definitions`、`assistant_skills`、`quality_gates`、`verification_scenarios`。
- `POST /platform/admin/schema-change-requests/{id}/verify` 已存在，并能做 schema package、DDL plan、risk、lifecycle status 和行业包 coverage 检查。
- 前端 SystemAdmin 已有 schema diff、verification report、apply job、ERP solution asset 文案和基础 UI。
- `platform.runtime_operations`、Tool Runtime `tool_definitions`、Assistant context `context_rules`、`custom_package_publication_requests` 等底座已经存在。

因此 Phase 2 应优先补强“结构化资产、真实差异、真实门禁、应用同步、发布前检查”，而不是继续堆入口。

## Scope

### In Scope

- 标准化行业方案 manifest，形成稳定的资产结构和风险/依赖元数据。
- 扩展 ERP standard solution package，让 runtime operations、tool policies、assistant/context/verification 资产都可被结构化读取和验证。
- 扩展 schema change verify，输出可执行性、权限影响、Runtime Operation 覆盖、Tool Policy 覆盖、Assistant Context 覆盖、Quality Gate 覆盖、Verification Scenario 覆盖和 rollback risk。
- 建立 package diff V2，按资产类型展示 current vs desired 的新增、变更、缺失、风险和依赖。
- 应用 schema change 时同步平台控制面资产，包括 runtime operation metadata、tool definition metadata、context rule drafts、assistant skill metadata、quality gate metadata、verification scenario metadata。
- SystemAdmin UI 展示 package assets、package diff、verification report、apply job timeline、失败原因和阻塞项。
- `custom_package_publication_requests` 发布前增加匿名化、知识来源权限、验证场景通过门禁。
- 同步 staged baseline SQL 和 `migrations/BASELINE_RESTRUCTURE.md`。

### Out Of Scope

- 不做租户 ERP 业务文档工作台；该能力属于 Phase 3。
- 不把 AI 生成的 context rule 自动激活；Phase 2 只允许进入 draft/proposal/verify/apply。
- 不允许 AI 自动执行 DDL 或绕过 schema approval。
- 不建设夜间 Monitoring Agent；该能力属于 Phase 5。
- 不恢复 `/projects`、`/sales`、`/procurement` 等语义业务路径作为租户主接口。

## Design

### 1. Industry Package Manifest

新增或收敛一个平台内部 manifest 结构，用于描述行业方案包内所有资产。Manifest 不替代现有 `SchemaPackage.Metadata`，而是作为 metadata 的稳定子结构，便于后端验证、前端展示和后续发布复用。

建议结构：

- `manifest_version`
- `industry_key`
- `package_key`
- `package_version`
- `assets`
- `dependencies`
- `quality_gates`
- `verification_scenarios`

每个 asset 至少包含：

- `asset_key`
- `asset_type`
- `version`
- `source`
- `owner`
- `risk_level`
- `depends_on`
- `payload`

资产类型包括：

- `database_asset`
- `business_function`
- `process_loop`
- `runtime_operation`
- `ui_workspace`
- `permission`
- `tool_policy`
- `tool_definition`
- `assistant_target`
- `context_rule`
- `assistant_skill`
- `quality_gate`
- `verification_scenario`

### 2. Package Diff V2

现有 schema diff 偏 DDL 层。Phase 2 需要 package-level diff，用于回答“这个行业包会给组织带来什么变化”。

Diff 结构建议：

- `asset_type`
- `asset_key`
- `action`: `create`、`update`、`remove`、`unchanged`
- `risk_level`
- `current_version`
- `desired_version`
- `summary`
- `blocking_reason`
- `depends_on`

后端生成 diff 时应基于 manifest 和当前平台资产状态进行对比。第一版可以只对 schema package 中的 desired manifest 和当前 schema package/current template 对比；后续再扩展到数据库实际状态和平台 runtime asset 状态。

### 3. Schema Verify V2

`VerifySchemaChange` 继续是 schema change 生命周期的质量门禁入口。Phase 2 扩展其 checks，使其从 coverage check 变成可审批前的实际 gate。

建议 checks：

- `schema_package`: schema package 是否合法。
- `ddl_plan`: 是否存在可执行 DDL，是否包含不允许的 destructive statement。
- `permissions_impact`: 权限和角色影响是否声明，是否包含高风险权限。
- `runtime_operations`: runtime operations 是否覆盖 API operations，是否有重复 operation key。
- `tool_policy`: tool definitions 是否声明 policy、risk、required permission、approval tier。
- `assistant_context`: assistant targets 和 context rules 是否声明，是否处于 draft/proposal 而非直接 active。
- `assistant_skills`: assistant skills 是否声明 targets、context rules、allowed tools。
- `quality_gates`: 每个高风险流程是否存在 gate。
- `verification_scenarios`: 每条关键业务流是否存在 scenario。
- `rollback_risk`: destructive 变更、runtime asset 覆盖、tool policy 改动是否有回滚说明。

规则：

- `failed` 增加 `blocking_issues`，阻止 apply。
- `warning` 不阻止 apply，但必须在 UI 显示原因。
- `CanApply` 仍要求 request 已 approved 且无 blocking issues。

### 4. Apply Orchestration

应用 schema change 不应只执行 DDL。对于行业方案工厂 request，apply 还需要同步平台控制面资产。

第一版建议同步到以下目标：

- `platform.runtime_operations`: 根据 manifest 的 runtime operation asset upsert。
- Tool Runtime definitions: 根据 manifest 的 tool definition/tool policy upsert metadata。
- Assistant/context: context rules 仅写入 draft/proposal 状态，不能自动 active。
- Assistant skills: 写入 skill metadata 或 solution asset table，供后续 Assistant Runtime 读取。
- Quality gates 和 verification scenarios: 写入 solution asset table 或平台 metadata table，供 verify/report/UI 读取。

事务策略：

- DDL apply 和 metadata apply 应共享 apply job 记录。
- metadata apply 失败时，apply job 标记 failed，并记录失败 asset key。
- 不要求第一版实现跨 DDL 和 metadata 的完整数据库事务回滚，但必须有失败状态、错误信息和可重试路径。

### 5. SystemAdmin UI

前端 SystemAdmin 保持控制面风格，不做租户业务工作台。

页面能力：

- 选择组织和模块，生成 ERP standard solution flow。
- 展示 package assets，按 asset type 分组显示数量和风险。
- 展示 package diff V2，按 create/update/remove/blocked 分组。
- 展示 verification report，突出 blocking issues、warning、CanApply。
- Apply 按钮只在 approved 且 verify 允许时可用。
- 展示 apply job status、statement count、metadata asset apply status、error message。
- 对 custom package publication request 展示匿名化、知识权限、验证场景门禁结果。

所有新增 UI 文案必须走 `frontend/src/lib/i18n.tsx`，中英文同时补齐。

### 6. Custom Package Publication Gate

沿用 `platform.custom_package_publication_requests`。发布前新增三类 gate：

- `anonymization_check`: 包内 payload 不包含客户、用户、订单、付款等可识别明文数据。
- `knowledge_source_permission_check`: knowledge/context 来源必须有合法 organization/package 权限。
- `verification_scenario_check`: manifest 中要求的 scenarios 必须通过或明确标记为 warning。

Gate 结果写入 publication request metadata。未通过时 request 不得 approve。

## Data And Migration

数据库变更应优先复用现有平台表；只有当现有 JSON metadata 不足以支持状态追踪和查询时才新增表。

可能需要新增或扩展：

- `platform.industry_solution_manifests`
- `platform.industry_solution_asset_diffs`
- `platform.schema_change_verification_reports`
- `platform.schema_apply_asset_results`
- `platform.custom_package_publication_requests.metadata`

如果新增表，归属 `migrations/000_saas_platform_management_baseline.sql`，因为这是平台管理控制面能力。同步更新 `migrations/BASELINE_RESTRUCTURE.md`。

## API Surface

优先复用现有入口：

- `POST /platform/admin/organizations/{id}/industry-solution-flows/erp-standard`
- `POST /platform/admin/schema-change-requests/{id}/verify`
- `POST /platform/admin/schema-change-requests/{id}/apply`
- `GET /platform/admin/runtime/operations`
- custom package publication request review endpoints

如需新增，建议限制为查询型或 review 型：

- `GET /platform/admin/schema-change-requests/{id}/package-diff`
- `GET /platform/admin/schema-change-requests/{id}/asset-results`
- `POST /platform/admin/industry-publication-requests/{id}/verify`

## Testing Strategy

Backend:

- `systemadmin` service tests for manifest generation, package diff, verify V2 blocking/warning logic, apply asset result persistence.
- `industry` service tests for publication gate reject/approve behavior.
- `runtime` or `toolruntime` tests for generated/upserted runtime operation and tool definition metadata.
- `go test ./...` must pass.

Migration:

- Fresh DB apply order: `000 -> 001 -> 002 -> 004`.
- Verify `SELECT COUNT(*) FROM pg_constraint WHERE NOT convalidated` returns `0`.
- If new platform tables are added, verify indexes and ownership are documented in `BASELINE_RESTRUCTURE.md`.

Frontend:

- Existing lint/build commands for frontend.
- If component tests are added, focus on SystemAdmin package diff, verify report, and apply gating states.

E2E smoke:

- Create tenant organization.
- Generate ERP standard industry solution flow.
- Verify schema change.
- Approve and apply.
- Confirm package diff and verification report are visible.
- Confirm runtime operation/tool/context/verification assets are visible or persisted.

## Success Criteria

- Platform admin can generate ERP standard industry package for an organization.
- Package assets are visible by type and include risk/dependency metadata.
- Package diff shows current vs desired changes beyond raw DDL diff.
- Verify report blocks apply on failed gates and permits apply on warnings.
- Apply job shows DDL and metadata asset status.
- Publication request cannot be approved when anonymization, knowledge permission, or verification scenario checks fail.
- Backend and migration verification pass.

## Risks And Decisions

- Existing `SchemaPackage.Metadata` is flexible but too loose for long-term governance. Phase 2 should introduce a manifest structure while preserving backward compatibility.
- Some platform assets may initially remain stored as JSON metadata rather than normalized tables. This is acceptable for Phase 2 if verify/apply/report behavior is deterministic and tested.
- Context rules must not become active automatically. This preserves the product rule that AI and generated packages propose rules, humans activate them.
- Apply orchestration should prefer explicit failed state and retryability over pretending DDL and metadata sync can always be atomically rolled back.
