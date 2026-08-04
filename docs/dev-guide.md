# 开发运维指南 · qtcloud-auth

## 本地开发

### 环境要求

- Go >= 1.23
- 可选：Python >= 3.12（运行集成测试）

### 启动服务

```bash
cd src/provider

# 普通启动
go run ./cmd/server

# 启动并固定验证码（用于集成测试）
SMS_TEST_CODE=123456 go run ./cmd/server
```

服务默认监听 `:8080`，通过 `LISTEN_ADDR` 环境变量修改。

### 运行测试

#### Go 单元测试

```bash
cd src/provider
go test ./...
```

#### Python 集成测试（需服务已启动）

```bash
# 安装依赖
uv sync --dev
# 或直接用 pip
pip install pytest pytest-asyncio httpx

# 运行全部集成测试
SMS_TEST_CODE=123456 LISTEN_ADDR=:8081 go run ./cmd/server &
TEST_BASE_URL=http://localhost:8081 pytest -v
```

可通过 `TEST_BASE_URL` 环境变量指定服务地址（默认 `http://localhost:8080`）。

### 项目结构

```
qtcloud-auth/
├── src/provider/          ← Go 认证提供者
│   ├── cmd/server/        ← 服务入口（main.go，对齐 qtcloud-pay 的 cmd/server 布局）
│   ├── internal/api/      ← HTTP 端点（handler + middleware）
│   ├── internal/app/      ← 数据库打开与迁移（GORM，对齐 qtcloud-pay 的 DB_DRIVER 约定）
│   ├── internal/model/    ← 数据模型（User, Role, VerificationCode）
│   ├── internal/store/    ← Storer 的 GORM 实现（users / roles / verification_codes）
│   └── go.mod
├── tests/                 ← Python 集成测试
│   ├── conftest.py
│   └── test_oauth.py
├── docs/                  ← 文档
├── pyproject.toml
├── .gitignore
└── README.md
```

## 配置

通过环境变量配置：

| 环境变量 | 默认值 | 说明 |
|----------|--------|------|
| `LISTEN_ADDR` | `:8080` | HTTP 监听地址 |
| `JWT_SECRET` | `quanttide-auth-secret` | HS256 签名密钥 |
| `ADMIN_PASSWORD` | `123456` | 管理员密码 |
| `SMS_DRIVER` | `console` | SMS 发送器。`console` 输出到日志，其他值待扩展（如 `aliyun`） |
| `SMS_TEST_CODE` | （不设置） | 设为 `123456` 则验证码固定，用于集成测试 |
| `DB_DRIVER` | （不设置） | 数据库驱动。`postgres` 时使用 `DATABASE_URL`；默认 SQLite |
| `DATABASE_URL` | （不设置） | PostgreSQL DSN（`DB_DRIVER=postgres` 时生效），如 `postgres://user:pass@host:5432/db?sslmode=disable` |
| `DB_SQLITE_DSN` | `qtcloud-auth.db` | SQLite 路径（开发默认；`:memory:` 为纯内存，重启不丢文件则用文件路径） |

| 硬编码常量 | 值 | 说明 |
|-----------|-----|------|
| 密码哈希 | bcrypt (DefaultCost) | 用户密码哈希 |
| 令牌有效期 | 3600s | access_token 有效期 |
| 刷新令牌有效期 | 604800s（7天） | refresh_token 有效期 |
| 验证码有效期 | 300s（5分钟） | 短信验证码有效期 |
| 验证码重发间隔 | 60s | 同一手机号最短重发间隔 |
| SMS 发送器 | `ConsoleSender` | 开发环境验证码输出到日志 |

## SMS 短信提供商对接

`SMSSender` 是一个 Go 接口，按需实现即可：

```go
type SMSSender interface {
    Send(ctx context.Context, phone, code string) error
}
```

参考实现：

- `ConsoleSender` — 开发调试用，直接输出到日志
- `fixedCodeSender` — `main.go` 中内置的固定验证码实现（用于集成测试）
- 阿里云 SMS — 参考 [library 指南](https://github.com/quanttide/quanttide-library-of-authorization/blob/main/aliyun-sms-integration.md)

在 `api/sms.go` 中将 `SMSSender` 实例注入 `AuthHandler`。

## 存储后端

`Storer` 接口抽象数据持久化（集合 + JSON 记录，见 `api/handler.go`）：

```go
type Storer interface {
    List(collection string) ([]byte, error)
    Create(collection string, data []byte) (string, error)
    Get(collection string, id string) ([]byte, error)
    Update(collection string, id string, data []byte) error
}
```

当前使用 **GORM**（`internal/app` 打开数据库，`internal/store` 实现 Storer）：

- **开发默认 SQLite**（`DB_SQLITE_DSN`，缺省 `qtcloud-auth.db`）— 零依赖，单文件，重启不丢
- **生产 PostgreSQL**（`DB_DRIVER=postgres` + `DATABASE_URL`）— 对接系统级共享 RDS，FC 多实例共享同一份数据

模型（User / Role / VerificationCode）在启动时由 `AutoMigrate` 建表；验证码与频率限制同样走 Storer（FC 多实例可用），不保留实例本地状态。

OAuth 令牌（access/refresh）存于 `oauth_tokens` 表（GORM 实现 `oauth2.TokenStore`，见 `internal/store/token_store.go`）：refresh_token 跨实例可用，刷新轮换后旧 token 自动作废。

## 部署

### 构建

```bash
cd src/provider
go build -o qtcloud-auth ./cmd/server
```

### 启动生产实例

```bash
./qtcloud-auth
```

### 负载与扩容

- 无状态服务，水平扩容（多实例 + 负载均衡）
- 如果使用内存存储，各实例数据不共享，需更换为共享存储后端
- 令牌验证基于 JWT 签名，无状态，无需集中存储

## CI 部署（deploy-provider）

[.github/workflows/deploy-provider.yml](../.github/workflows/deploy-provider.yml) 是唯一部署入口（对齐 qtcloud-pay）：

- **触发**：推送 `provider/*` tag（如 `provider/v0.1.0-alpha.3`）；**无手动触发**（生产保护）
- **job 1 `build-image`**：Dockerfile 多阶段构建 → 双通道发布（Docker Hub + ACR `quanttide/qtcloud-auth-provider`，实例地址来自 secret `ALIYUN_ACR_REGISTRY`）→ 各带版本 tag 与 `latest`；ACR 仓库由主账号预先创建（PUBLIC，FC 免凭证直拉），CI 不建仓
- **job 2 `deploy`**：`terraform init/validate/plan/apply`（OSS 远程状态 `qtcloud-auth/terraform.tfstate`），挂 GitHub `production` 环境（可配 required reviewers）

### 所需 GitHub secrets（org 级）

| secret | 用途 | 注入方式 |
|--------|------|---------|
| `ALIYUN_ACCESS_KEY_ID` / `ALIYUN_ACCESS_KEY_SECRET` | RAM 用户 AccessKey（仅 terraform） | `ALICLOUD_ACCESS_KEY` / `ALICLOUD_SECRET_KEY` |
| `ALIYUN_ACR_USERNAME` / `ALIYUN_ACR_PASSWORD` / `ALIYUN_ACR_REGISTRY` | ACR 固定凭证（docker login）；`REGISTRY` 为实例公网地址（`crpi-*.personal.cr.aliyuncs.com`，控制台可见） | `docker/login-action`；镜像地址拼接待 secret |
| `DB_PASSWORD` / `JWT_SECRET` / `ADMIN_PASSWORD` | RDS 密码 / JWT 密钥 / 管理员密码 | `TF_VAR_db_password` / `TF_VAR_jwt_secret` / `TF_VAR_admin_password` |
| `DOCKERHUB_USERNAME` / `DOCKERHUB_PASSWORD` | Docker Hub 凭证（双通道之一） | `docker/login-action` |

### 关键坑（实战沉淀）

1. **ACR 个人版实例是专属域名**：老地址 `registry.cn-hangzhou.aliyuncs.com` 会被新实例拒绝（`Forbidden Host`），必须用控制台的 `crpi-*.personal.cr.aliyuncs.com`；该地址属敏感信息，只放 secret，**不写进代码/文档**
2. **不要在 CI 里建 ACR 仓库**：CI 的 RAM 用户无 ACR 管理权限；且 aliyun CLI 的 cr API 不稳定（`version: latest` 下 `ListInstance --InstanceType` 已失效）。仓库由主账号一次性创建（PUBLIC）
3. **ALICLOUD_* 新旧变量名不互通**：terraform 读 `ALICLOUD_ACCESS_KEY` / `ALICLOUD_SECRET_KEY`。写错不报错，只会在远端 API 失败
4. **含 secret 的值只写 `$GITHUB_ENV`**，写 `$GITHUB_OUTPUT` 会被脱敏置空（如镜像名拼接 ACR 地址）
5. **空仓库的 `tags/list` 返回 `NAME_UNKNOWN`**：属注册表正常行为，不代表仓库不存在；验证存在性用 `docker manifest inspect`（`no such manifest` = 存在、仅缺 tag）

### 发版流程

1. 补 CHANGELOG → 提交
2. 打 tag：`provider/v0.1.0-alpha.N`，推送（tag 指向的 commit 决定 CI 用的代码，**先推修复再发 tag**）
3. 观察 Actions：build-image → deploy；失败版本建议直接发 alpha.N+1，语义更干净
