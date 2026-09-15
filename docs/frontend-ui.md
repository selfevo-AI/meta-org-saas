# Frontend UI / 前端界面与交互

## 使用方式 / Everyday workflow

- 工作台以待办、关键指标、常用入口和最近动态为主。左侧可筛选导航，顶部 `Ctrl/Cmd + K` 可搜索模块和单据入口。
- 单据默认以查看模式展示。点击“编辑单据”后修改；保存时先核对字段的修改前后值，再确认提交。明细也使用同样的核对流程。
- 切换单据、页面内导航、取消编辑或刷新前，会提示处理未保存内容。刷新和关闭浏览器页面使用浏览器原生离开提示。
- 业务操作显示当前单据、状态和影响。删除收纳在“更多操作”，确认框默认聚焦取消；已审批、已过账及其他受保护单据继续受业务规则约束。
- 供应商、客户、物料、仓库和科目等关联字段提供编号及名称建议，查看时显示可用的业务名称。
- AI 建议在提交、批准或驳回前展示操作内容。修改分析要求后，需要重新分析再提交新建议；审批意见传递给现有审批接口。
- 平台用户列表支持搜索，创建用户使用独立表单。重置密码、禁用用户、关闭组织、删除或应用行业方案需要核对目标后确认。

The home page brings together pending work, metrics, shortcuts, and recent activity. Use the sidebar filter or `Ctrl/Cmd + K` to find a module or document entry.

Documents open in view mode. Choose **Edit record**, make changes, then review the before/after comparison before saving. Line items use the same review flow. Navigation within the application, record changes, cancellation, and refresh are guarded against discarding unsaved changes; page reload and close use the browser's native warning.

Workflow actions identify the record and its current state. Deletion is in **More actions** and requires confirmation. Existing business locks still apply. Related fields offer business name/code suggestions where the current account can read the relevant catalog.

AI proposals require a contextual review. Adjusting the request requires another analysis before submitting the updated proposal. Approval notes are sent to the existing API. Platform account and organization maintenance actions identify their target before confirmation.

## 界面约定 / UI conventions

- 统一使用浅色画布、内容卡片、主次按钮、状态和反馈样式；支持深色主题及持久化主题选择。
- 桌面使用导航、单据列表和详情布局；平板及手机根据工作区实际宽度重排。宽表格在区域内滚动，操作列和菜单保持可用。
- 对话框使用原生 `dialog`，支持键盘操作、焦点恢复、嵌套弹窗及滚动锁定。上下文菜单使用原生 popover，避免被表格容器裁剪。
- 所有新增界面文案通过 `useI18n` 提供中文和英文。新文案使用稳定的 `ui.*` 键。
- 表单错误保留在当前操作中；单据更新仅提交已改变且允许写入的字段。

Use the shared light/dark palette, cards, buttons, status badges, and feedback. Container queries adapt the workspace to the available width. Dialogs support keyboard control, focus restoration, and nested scroll locking; contextual menus remain visible outside scrolling table bounds. New copy must provide Chinese and English translations through `useI18n`. Failed document saves preserve the input and review, and PATCH requests contain only changed writable fields.

## 代码入口 / Implementation

| 文件 / File | 职责 / Responsibility |
| --- | --- |
| `frontend/src/app/workspace-ui.tsx` | 对话框、菜单、搜索、未保存提示 / Dialogs, menus, search, unsaved-change guards |
| `frontend/src/app/globals.css` | 设计变量、主题、布局和响应式样式 / Theme tokens, layouts, responsive styles |
| `frontend/src/app/page.tsx` | 登录、应用导航、首页和待办核对 / Login, shell, home and inbox review |
| `frontend/src/app/document-workbench.tsx` | 单据查看、编辑、差异核对和操作确认 / Document editing, comparisons and action review |
| `frontend/src/app/erp-business-module-workspace.tsx` | 现有业务 API、字段查询建议和状态约束 / Existing business APIs, lookup suggestions and state constraints |
| `frontend/src/app/business-ai-workbench.tsx`, `ai-assistant.tsx` | AI 建议与审批核对 / AI proposals and approval review |
| `frontend/src/app/system-admin-workspace.tsx` | 平台用户列表及管理操作核对 / Platform user management and contextual confirmations |

## 验证 / Verification

```powershell
cd frontend
npm run lint
npx tsc --noEmit
npm run build
npm run test:ui
npm run test:erp-business
npm run test:unified-workbench
npm run test:system-admin
npm run test:routing
npm run test:bundle
npm run test:i18n-bundle
```

`npm run test:ui` 针对已运行的前端进行验证，默认地址为 `http://127.0.0.1:3000`，可通过 `PLAYWRIGHT_BASE_URL` 修改。此套件拦截 API，不运行全局数据库初始化，不修改真实业务数据。覆盖桌面、Pixel 手机和平板三种视口，以及中英文、深浅主题、保存核对、失败重试、取消及确认删除、未保存保护、键盘导航、AI 调整与审批、平台用户操作和弹窗焦点。

`npm run test:ui` targets a running frontend at `http://127.0.0.1:3000` by default; override `PLAYWRIGHT_BASE_URL` as needed. It intercepts APIs and skips database setup, so it does not modify real business records. It covers desktop, phone and tablet layouts, language/theme switching, reviewed mutations, failures/retries, unsaved changes, keyboard navigation, AI review, platform user actions, and dialog focus.

真实 API 回归使用 `session-scopes.spec.ts` 与 `ontology-commerce.spec.ts` 的采购、销售和归档用例。应指定独立测试租户；设置 `PLAYWRIGHT_SKIP_GLOBAL_SETUP=1` 可避免默认夹具初始化。这些业务回归会在指定测试租户中创建并处理测试单据。AI 确认界面的回归使用模拟服务，不代表已验证某个外部模型供应商。

Real API regressions use `session-scopes.spec.ts` and the commerce/archive cases in `ontology-commerce.spec.ts`. Supply a dedicated test tenant and set `PLAYWRIGHT_SKIP_GLOBAL_SETUP=1` to skip default fixture provisioning. These commerce tests create and process records in that tenant. AI confirmation regressions use intercepted APIs; they do not verify a particular external model provider.

本次界面更新不改变数据库结构、迁移文件或业务接口契约。

This UI update introduces no database schema, migration, or business API contract changes.
