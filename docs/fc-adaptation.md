# provider 适配 FC 规划

目标：让 `src/provider`（Go 认证服务）能在阿里云 FC 3.0 custom-container 上稳定运行，基础设施已由 [manifests/terraform](../manifests/terraform/README.md) 就绪。本规划对齐 qtcloud-pay 已落地的模式（GORM 方言切换 + `DB_DRIVER`/`DATABASE_URL` 运行时约定）。

## 现状盘点：哪些已适配、哪些阻塞

| 维度 | 现状 | 结论 |
|------|------|------|
| 存储 | BuntDB（`DB_PATH`，默认 `:memory:`） | **阻塞**：FC 容器磁盘是临时存储（实例回收/滚动发布即丢数据），多实例各写各的、数据不互通 |
| 短信 | 仅 `console` 驱动（验证码打日志） | **阻塞**：生产需真实下发，参考 [aliyun-sms-integration](https://github.com/quanttide/quanttide-library-of-authorization/blob/main/aliyun-sms-integration.md) |
| 密钥 | `JWT_SECRET` / `ADMIN_PASSWORD` 走环境变量 | ✓ 已适配，FC 环境变量直接注入（IaC 已接） |
| 监听端口 | `LISTEN_ADDR` 默认 `:8080` | ✓ 与 IaC `custom_container_config.port = 8080` 对齐 |
| 会话/令牌 | JWT HS256 无状态校验 | ✓ 天然适配无状态 FaaS，多实例无需会话粘滞 |
| 数据库 | `alicloud_db_database`/`account` 已在共享 RDS 建好，`DATABASE_URL` 已注入 FC 环境变量 | ✓ 基础设施就绪，等 provider 接 PostgreSQL |

## 目标形态

```
FC custom-container ──VPC内网──▶ 共享 RDS PostgreSQL（quanttide-platform 管理）
   env: DB_DRIVER=postgres / DATABASE_URL=...
   镜像: ACR 同地域直拉（Docker Hub 仅作分发）
```

本地开发保持 SQLite（`DB_SQLITE_DSN`），生产 `DB_DRIVER=postgres`，与 pay 的 `internal/app/app.go` 完全同构。

## 改造步骤

### Phase 1 — 存储后端 PostgreSQL（✅ 已完成）

1. 引入依赖：`gorm.io/gorm`、`gorm.io/driver/postgres`、`gorm.io/driver/sqlite`（版本对齐 qtcloud-pay：gorm v1.31.2 / postgres v1.6.2 / sqlite v1.6.0）
2. 新增 `internal/app`（`Open(driver, dsn)` + `OpenDB()`）：
   - `DB_DRIVER=postgres` → `DATABASE_URL` 作 DSN
   - 默认 → `DB_SQLITE_DSN`（缺省 `qtcloud-auth.db`），`SetMaxOpenConns(1)` 规避 SQLite 单写者锁
   - 启动时 `AutoMigrate` 全部模型（User / Role / VerificationCode）
3. 新增 `internal/store`：GORM 实现 `Storer` 接口（UUID 字符串主键，每个集合一张表），`main.go` 移除 BuntDB；验证码与发送频率限制也改走 Storer（跨实例共享）
4. 新增 `internal/store/token_store.go`：GORM 实现 `oauth2.TokenStore`（access/refresh 双键 + 过期摘除 + Create 幂等 upsert），`SetupOAuth` 注入，refresh_token 跨实例可用
5. 注意：`gorm.io/driver/sqlite` 依赖 CGO（mattn/go-sqlite3）。生产镜像只走 postgres，用 `CGO_ENABLED=0` 静态编译；本地开发正常 CGO 编译即可（见 Phase 5）

**验证**：`go test ./...` 全绿（含 `internal/store` 新增用例）；`tests/` 集成测试 11 个全过（SQLite 后端）。

**剩余缺口（FC 多实例）**：
- 过期验证码未清理（每次发送一条记录，P2 增加清理任务）

### Phase 2 — 阿里云短信

1. 实现 `AliyunSMSSender`（实现现有 `SMSSender` 接口），参考 [aliyun-sms-integration](https://github.com/quanttide/quanttide-library-of-authorization/blob/main/aliyun-sms-integration.md)
2. `SMS_DRIVER=aliyun` 时启用；密钥经环境变量注入，或后续走 FC 服务角色（`fc.tf` 的 RAM role 已存在，附短信权限策略即可）
3. 验证码短信模板需在阿里云短信控制台申请
4. IaC 联动：`sms_driver` 变量切 `aliyun`，新增短信密钥变量

### Phase 3 — 密钥与配置管理（与 TODO.md「CI 与安全」同步）

- 过渡：先用 FC 环境变量（tfvar 注入 `JWT_SECRET`/`ADMIN_PASSWORD`/`db_password`，注意明文落 tfstate 的已知问题）
- 排期：FC 配置中心 / 密钥管理（KMS）注入，去除 tfstate 明文
- 长期：CI 升级 OIDC + RAM 角色，不落任何长期凭证

### Phase 4 — 可观测与生命周期（P2，ROADMAP 已有条目）

- `/healthz` 健康检查端点：FC 探活、后续 API 网关接入都依赖它
- 日志结构化（JSON），便于 FC 日志服务检索
- 优雅退出：捕获 SIGTERM，关闭 DB 连接池

### Phase 5 — 镜像与发布（阻塞项）

1. Dockerfile 多阶段构建（ROADMAP P3.5，目标 < 20MB）：静态编译 + 精简基础镜像
2. CI workflow（deploy-provider，对齐 pay）：镜像构建 → 双通道发布（Docker Hub + ACR `registry.cn-hangzhou.aliyuncs.com/quanttide/qtcloud-auth-provider`）→ `terraform init/plan/apply`
3. ACR 命名空间/仓库由 CI 幂等创建

## 验收标准

1. FC 部署后 `DB_DRIVER=postgres` + `DATABASE_URL` 生效，`/oauth/token` 全链路可用
2. FC 多实例/滚动发布下用户、角色数据一致（写入即共享可见）
3. `SMS_DRIVER=aliyun` 验证码真实下发
4. `terraform output fc_http_url` 完成「短信登录 → 取 token → /userinfo」闭环
5. 镜像可复现、CI 一键发布，本地开发不依赖生产环境

## 实施顺序

Phase 1（存储 ✅ 已落地，含 refresh_token 共享存储）→ Phase 5（镜像/CI，打通端到端）→ Phase 2（短信）→ Phase 4（可观测）→ Phase 3（密钥收敛，可并行）。
