# 开发运维指南 · qtcloud-auth

## 本地开发

### 环境要求

- Go >= 1.23
- 可选：Python >= 3.12（运行集成测试）

### 启动服务

```bash
cd src/provider

# 普通启动
go run .

# 启动并固定验证码（用于集成测试）
SMS_TEST_CODE=123456 go run .
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
SMS_TEST_CODE=123456 LISTEN_ADDR=:8081 go run . &
TEST_BASE_URL=http://localhost:8081 pytest -v
```

可通过 `TEST_BASE_URL` 环境变量指定服务地址（默认 `http://localhost:8080`）。

### 项目结构

```
qtcloud-auth/
├── src/provider/          ← Go 认证提供者
│   ├── api/               ← HTTP 端点（handler + middleware）
│   ├── model/             ← 数据模型（User, Role, VerificationCode）
│   ├── main.go            ← 服务入口
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
| `DB_PATH` | `:memory:` | BuntDB 数据库路径。`data.db` 持久化到文件，重启不丢数据 |

| 硬编码常量 | 值 | 说明 |
|-----------|-----|------|
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

`Storer` 接口抽象数据持久化：

```go
type Storer interface {
    List(collection string) ([]byte, error)
    Create(collection string, data []byte) (string, error)
    Get(collection string, id string) ([]byte, error)
    Update(collection string, id string, data []byte) error
}
```

当前使用 **BuntDB**（[`DB_PATH` 环境变量](#) 控制）：

- **`:memory:`**（默认）— 纯内存，进程重启数据丢失，适合开发调试
- **文件路径**（如 `data.db`）— 持久化到磁盘，重启不丢

可按需对接 SQLite / MySQL / PostgreSQL 等。

## 部署

### 构建

```bash
cd src/provider
go build -o qtcloud-auth .
```

### 启动生产实例

```bash
./qtcloud-auth
```

### 负载与扩容

- 无状态服务，水平扩容（多实例 + 负载均衡）
- 如果使用内存存储，各实例数据不共享，需更换为共享存储后端
- 令牌验证基于 JWT 签名，无状态，无需集中存储
