# 开发运维指南 · qtcloud-auth

## 本地开发

### 环境要求

- Go >= 1.23
- 可选：Python >= 3.12（运行集成测试）

### 启动服务

```bash
# 进入 provider 模块
cd src/provider

# 运行
go run .
```

服务默认监听 `:8080`。

### 运行测试

#### Go 单元测试

```bash
cd src/provider
go test ./...
```

#### Python 集成测试（需服务已启动）

```bash
# 安装依赖
pip install -e ".[dev]"

# 运行（跳过 integration 标记的测试只跑单元级）
pytest -v

# 运行全部（含需要服务在线的集成测试）
pytest -v -m integration
```

### 项目结构

```
qtcloud-auth/
├── src/provider/          ← Go 认证提供者
│   ├── api/               ← HTTP 端点（handler + middleware）
│   ├── model/             ← 数据模型（User, Role, VerificationCode）
│   └── go.mod
├── tests/                 ← Python 集成测试
│   ├── conftest.py
│   └── test_oauth.py
├── docs/                  ← 文档
├── pyproject.toml
└── README.md
```

## 配置

当前无外部配置文件，配置通过代码常量控制：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| 监听端口 | `:8080` | HTTP 服务端口 |
| JWT 签发密钥 | `quanttide-auth-secret` | HS256 签名密钥 |
| 令牌有效期 | 3600s | access_token 有效期 |
| 刷新令牌有效期 | 604800s（7天） | refresh_token 有效期 |
| 验证码有效期 | 300s（5分钟） | 短信验证码有效期 |
| 验证码重发间隔 | 60s | 同一手机号最短重发间隔 |
| SMS 发送器 | `ConsoleSender` | 开发环境验证码输出到日志 |

后续可提取到环境变量或配置文件。

## SMS 短信提供商对接

`SMSSender` 是一个 Go 接口，按需实现即可：

```go
type SMSSender interface {
    Send(phone, code string) error
}
```

参考实现：

- `ConsoleSender` — 开发调试用，直接输出到日志
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

当前使用内存存储（BuntDB），可按需对接 MySQL / PostgreSQL / Redis 等。

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
