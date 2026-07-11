# provider · 认证提供者

认证提供者模块，封装身份认证核心逻辑，提供 JWT 签发/验证、用户与角色模型、HTTP 接口层。

## 包结构

| 包 | 说明 |
|---|------|
| [`auth`](./auth/) | JWT 签名与验证（HS256） |
| [`model`](./model/) | 数据模型：`User`、`Role`、`VerificationCode` |
| [`api`](./api/)  | HTTP 处理层：登录（密码/验证码）、令牌刷新、当前用户查询、认证中间件、短信发送 |

## auth — JWT

基于 HMAC-SHA256 的 JWT 签发与验证。

- `Sign(payload, secret)` — 签发 HS256 JWT
- `Verify(token, secret)` — 验证签名、检查 exp 过期时间

## model — 数据模型

| 类型 | 字段 |
|------|------|
| `User` | `id`, `username`, `password_hash`, `phone`, `phone_verified`, `nickname`, `avatar`, `role_id`, `created_at` |
| `Role` | `id`, `name`, `permissions` |
| `VerificationCode` | `phone`, `code`, `expires_at`, `used`, `created_at` |

## api — HTTP 接口

基于 `net/http` 的 RESTful 接口。

### 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/login` | 用户名密码登录，返回 JWT + 用户信息 |
| POST | `/api/v1/sms/send` | 发送手机验证码 |
| POST | `/api/v1/login/phone` | 手机验证码登录/自动注册 |
| POST | `/refresh` | 用现有有效 JWT 换取新令牌 |
| GET  | `/me`  | 获取当前登录用户信息（需 Bearer Token） |

### 中间件

`AuthMiddleware(secret)` — 从 `Authorization: Bearer <token>` 头中解析 JWT，验证后将 claims 注入请求上下文。

### SMS 短信

`SMSSender` 接口抽象短信发送通道：

- `ConsoleSender` — 开发调试用，验证码直接输出到日志
- 可对接阿里云/腾讯云等短信 SDK（实现 `SMSSender` 接口即可）

内置频率限制（默认 60 秒重发间隔）和验证码有效期（默认 5 分钟）。

### 辅助函数

- `WriteJSON(w, v, status)` — 写入 JSON 响应
- `WriteError(w, code, message, status)` — 写入标准错误 JSON

## Storer 接口

```go
type Storer interface {
    List(collection string) ([]byte, error)
    Create(collection string, data []byte) (string, error)
    Get(collection string, id string) ([]byte, error)
    Update(collection string, id string, data []byte) error
}
```

`AuthHandler` 依赖 `Storer` 接口实现持久化，可对接任意存储后端（文件、内存、数据库等）。

## 手机验证码自动注册流程

1. `POST /api/v1/sms/send` → 校验手机号 → 生成 6 位验证码 → 存储（内存）→ 调用 `SMSSender` 发送
2. `POST /api/v1/login/phone` → 校验验证码 → 标记已使用 → 按手机号查找用户 → 不存在则自动创建 → 签发 JWT

## 模块路径

```
github.com/quanttide/qtcloud-auth
```

## 许可

Apache 2.0 — 见项目根目录 [LICENSE](../../LICENSE)。
