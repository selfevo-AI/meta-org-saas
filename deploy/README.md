# 部署

[English](README_EN.md)

浏览器始终访问当前站点的 `/api/v1`。Caddy 将 API 请求转发到
`backend:8080`，其余请求转发到 `frontend:3000`。同一个前端镜像可以用于
不同域名，无需在构建时设置 API 地址。

## 本地运行

在仓库根目录运行：

```sh
docker compose up -d --build
```

打开 `http://127.0.0.1:3000`，同域健康检查地址为
`http://127.0.0.1:3000/api/v1/health`。开发编排同时公开 PostgreSQL、Go API
和安全内核的端口，便于本地调试。已有环境应继续使用原来的 Compose 项目名
（例如 `-p meta-org-ontology-final`），以复用原来的数据库卷。

本机开发前端使用 `npm --prefix frontend run dev`。Next.js 开发代理默认转发到
`http://127.0.0.1:8080`，可通过 `API_PROXY_TARGET` 指定其他 HTTP(S) origin。
该值不能包含认证信息、路径、查询参数或片段。浏览器不再读取
`NEXT_PUBLIC_API_URL`。生产版 `npm start` / standalone 服务需要 Caddy 提供 API 代理。

## 公网 HTTPS 部署

准备 Docker Compose、一个指向服务器的域名，以及可从公网访问的 TCP 80/443
端口。Caddy 自动申请和续期证书；UDP 443 用于 HTTP/3。生产编排只公开网关端口。

复制配置模板：

```sh
cp deploy/.env.production.example deploy/.env.production
```

PowerShell 使用 `Copy-Item`。真实配置文件已被 `.gitignore` 和 Docker 构建上下文
排除规则覆盖。填写以下字段；模板没有可直接使用的生产密钥：

| 字段 | 设置方式 |
| --- | --- |
| `APP_DOMAIN` | 例如 `app.company.com`；只填主机名，不带协议、路径或端口。 |
| `ACME_EMAIL` | 接收证书相关通知的邮箱。 |
| `POSTGRES_PASSWORD` | PostgreSQL 的独立随机密码。 |
| `PLATFORM_DATABASE_URL` | `postgres://postgres:<URL 编码后的密码>@postgres:5432/meta_org_saas?sslmode=disable`。 |
| `TENANT_DATABASE_ADMIN_URL` | 同一实例的管理库：`postgres://postgres:<URL 编码后的密码>@postgres:5432/postgres?sslmode=disable`。 |
| `JWT_SECRET` | 至少 32 字符的独立随机密钥。 |
| `MODEL_SECRET_KEY` | 恰好 32 字节的独立密钥；用于解密已有供应商等配置，需随数据库一起备份。 |
| `SECURITY_KERNEL_SHARED_SECRET` | 至少 32 字符的独立随机密钥。 |
| `META_ORG_PLATFORM_ADMIN_EMAIL` | 初始平台管理员邮箱。 |
| `META_ORG_PLATFORM_ADMIN_PASSWORD_HASH` | 通过 `backend/cmd/bcrypt-hash` 生成的 bcrypt hash；在 `.env` 中用单引号包裹，以保留 `$`。 |

可分别运行 `openssl rand -hex 32` 生成 JWT 和安全内核密钥，运行
`openssl rand -hex 16` 生成 32 个 ASCII 字符的模型加密密钥。管理员密码 hash
生成方法见[根目录配置说明](../README.md#配置)。配置中的两个数据库 URL
使用同一个 PostgreSQL 密码；密码中的 URL 保留字符必须编码。

默认应用子网为 `172.30.40.0/24`，网关地址为 `172.30.40.2`。其他容器从
`172.30.40.128/25` 动态分配地址，避免占用网关的固定 IP。如果与服务器现有
网络冲突，同时调整 `APP_NETWORK_SUBNET`、`APP_NETWORK_DYNAMIC_RANGE` 和
`PROXY_IP_ADDRESS`：动态范围和网关都要在子网内，网关必须在动态范围外。
后端只信任此网关 IP 提供的客户端地址。当前配置面向 Caddy 直接接收
公网流量；在前面增加 CDN 或其他代理时，需要相应设计真实客户端 IP 的信任链。

从仓库根目录检查并启动：

```sh
docker compose --env-file deploy/.env.production -f docker-compose.production.yml config --quiet
docker compose --env-file deploy/.env.production -f docker-compose.production.yml up -d --build --wait
```

该编排使用独立项目名 `meta-org-production` 和数据卷。PostgreSQL 先创建平台库
`meta_org_saas`，后端随后执行平台迁移。租户库由 provisioner 创建为
`meta_org_xxxx` 并展开 `tenantdb:include`，其中 `xxxx` 是组织 UUID 去掉连字符后的
前四位小写十六进制字符。租户管理连接指向 `postgres`，不指向平台业务库。

安全内核的健康检查依赖后端迁移创建的 nonce 表，所以后端等待内核进程启动后
即可执行迁移。对外网关仍等待后端健康检查通过，后者包含安全内核就绪状态。

验证健康检查并打开登录页：

```sh
curl --fail https://app.company.com/api/v1/health
docker compose --env-file deploy/.env.production -f docker-compose.production.yml ps
```

健康响应应报告平台数据库和安全内核为 `ok`。使用所配置的管理员账号，在登录页
选择 **SaaS 管理**。证书申请依赖真实 DNS、公网端口以及证书颁发机构可达性。

## 更新和数据保留

更新代码后使用相同的 env 文件、Compose 文件和项目名重新执行启动命令。
后端启动时自动执行受校验的迁移。升级前备份平台库、每个租户库以及加密密钥；
数据库内容存放于 `pgdata`，证书和 Caddy 状态存放于 `caddy_data` / `caddy_config`。
普通容器更新会复用这些数据卷。

## 网关验证

以下检查使用独立的 HTTP 测试服务，不连接应用数据库：

```sh
docker compose -f deploy/tests/docker-compose.yml up --abort-on-container-exit --exit-code-from checks
docker compose -f deploy/tests/docker-compose.yml down
```

检查覆盖页面/API 路由、查询参数、认证和组织请求头、伪造客户端 IP 请求头、
上传下载、错误状态、SSE 首包以及客户端断开时的上游取消。CI 同时验证生产
Compose 和 Caddy 配置，并让浏览器测试通过开发服务器的同域 API 代理访问后端。

安装 Node.js 20.9+ 后，还可以运行完整的新环境验证：

```sh
node deploy/tests/production-smoke.mjs
```

该命令构建当前代码，以独立项目和临时数据库启动生产编排，在本机 18080/18443
端口验证 HTTPS、平台登录与租户创建。它使用 `localhost` 的临时测试证书，
不申请公网证书，并在结束时清理自身的容器、数据卷和临时构建镜像。
如需复用已构建镜像，同时设置 `DEPLOY_TEST_FRONTEND_IMAGE`、
`DEPLOY_TEST_BACKEND_IMAGE`、`DEPLOY_TEST_KERNEL_IMAGE`；复用的镜像会保留。

若 `/api/v1/health` 返回 502，先查看后端健康状态与网关日志；若返回前端 404，
确认访问的是 Caddy 或 Next.js 开发服务器，而不是未经代理的生产前端端口。

## 验证记录（2026-09-14）

| 检查 | 结果 |
| --- | --- |
| Next.js 16.3.5 容器生产构建、TypeScript、前端 lint | 通过 |
| npm 依赖审计 | 0 个已知漏洞 |
| 网关转发、文件传输、请求头与流式响应 | 5 项通过 |
| 开发代理下的桌面/手机端登录与会话作用域 | 6 项通过 |
| Docker 网关下的完整业务和登录回归 | 14 项通过 |
| 生产编排全新启动、HTTPS/HSTS、平台和租户登录、租户数据库创建 | 通过 |

全新启动验证使用独立数据卷和 `localhost` 测试证书，确认平台库
`meta_org_saas` 与 provisioner 创建的 `meta_org_xxxx` 租户库均可用。
公网 DNS 和 ACME 证书申请需要在实际域名环境验证。
