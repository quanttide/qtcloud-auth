# provider · 认证提供者

认证提供者模块，封装身份认证核心逻辑，提供 JWT 签发/验证、用户与角色模型、HTTP 接口层。

## 包结构

| 包 | 说明 |
|---|------|
| [`auth`](./auth/) | JWT 签名与验证（HS256） |
| [`model`](./model/) | 数据模型：`User`、`Role` |
| [`api`](./api/)  | HTTP 处理层：登录、令牌刷新、当前用户查询、认证中间件 |

## auth — JWT

基于 HMAC-SHA256 的 JWT 签发与验证。

- `Sign(payload, secret)` — 签发 HS256 JWT
- `Verify(token, secret)` — 验证签名、检查 exp 过期时间

## model — 数据模型

| 类型 | 字段 |
|------|------|
| `User` | `id`, `username`, `password_hash`, `role_id`, `created_at` |
| `Role` | `id`, `name`, `permissions` |

## api — HTTP 接口

基于 `net/http` 的 RESTful 接口。

### 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/login` | 用户名密码登录，返回 JWT + 用户信息 |
| POST | `/refresh` | 用现有有效 JWT 换取新令牌 |
| GET  | `/me`  | 获取当前登录用户信息（需 Bearer Token） |

### 中间件

`AuthMiddleware(secret)` — 从 `Authorization: Bearer <token>` 头中解析 JWT，验证后将 claims 注入请求上下文。

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

`api.AuthHandler` 依赖 `Storer` 接口实现持久化，可对接任意存储后端（文件、内存、数据库等）。

## 模块路径

```
github.com/quanttide/qtcloud-auth
```

## 许可

Apache 2.0 — 见项目根目录 [LICENSE](../../LICENSE)。
