# provider · 认证提供者

认证提供者模块，封装 OAuth 2.0 身份认证核心逻辑，支持密码登录、手机验证码登录、令牌刷新与用户信息查询。

## 包结构

| 包 | 说明 |
|---|------|
| [`auth`](./auth/) | JWT 签名与验证（HS256） |
| [`model`](./model/) | 数据模型：`User`、`Role`、`VerificationCode` |
| [`api`](./api/)  | OAuth 2.0 端点：统一认证、短信验证码、用户信息、中间件 |

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

## api — OAuth 2.0 端点

### 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/oauth/token` | 统一认证入口（密码/短信/刷新令牌） |
| POST | `/oauth/sms/send` | 发送手机验证码 |
| GET  | `/userinfo` | 获取当前用户信息（OIDC 标准） |

### `/oauth/token` — 统一认证

根据 `grant_type` 分发：

| grant_type | 参数 | 说明 |
|------------|------|------|
| `password` | `username`, `password` | 用户名密码登录 |
| `sms_code` | `phone`, `code` | 手机验证码登录/自动注册 |
| `refresh_token` | `refresh_token` | 刷新访问令牌 |

成功响应（OAuth 2.0 标准格式）：

```json
{
  "access_token": "eyJ...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "eyJ..."
}
```

### `/oauth/sms/send` — 发送验证码

```json
// 请求
{ "phone": "13800138000" }
// 响应
{ "message": "code sent" }
```

内置频率限制（默认 60 秒重发间隔）和验证码有效期（默认 5 分钟）。

### `/userinfo` — 用户信息

需携带 `Authorization: Bearer <access_token>`。

```json
{
  "sub": "u1",
  "phone": "13800138000",
  "phone_verified": true,
  "nickname": "138****8000",
  "picture": null,
  "updated_at": "2026-07-11T..."
}
```

不暴露 `password_hash` 等敏感字段。

### 中间件

`AuthMiddleware(secret)` — 从请求头解析 Bearer Token，验证后将 claims 注入请求上下文。

### SMS 短信

`SMSSender` 接口抽象短信发送通道：

- `ConsoleSender` — 开发调试用，验证码直接输出到日志
- 可对接阿里云/腾讯云等短信 SDK（实现 `SMSSender` 接口即可）

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

## 模块路径

```
github.com/quanttide/qtcloud-auth
```

## 许可

Apache 2.0 — 见项目根目录 [LICENSE](../../LICENSE)。
