# API 参考

> 本文档以集成测试为事实源自动对齐。运行 `pytest --collect-only -m integration` 可查看最新端点清单。

---

## POST /oauth/token

统一认证入口。根据 `grant_type` 分发认证策略。

### 请求

**Content-Type:** `application/x-www-form-urlencoded`

#### 密码登录

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `grant_type` | string | 是 | `password` |
| `username` | string | 是 | 用户名 |
| `password` | string | 是 | 密码 |

#### 短信验证码登录

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `grant_type` | string | 是 | `sms_code` |
| `phone` | string | 是 | 手机号 |
| `code` | string | 是 | 短信验证码 |

首次使用该手机号时自动注册账号。

#### 刷新令牌

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `grant_type` | string | 是 | `refresh_token` |
| `refresh_token` | string | 是 | 刷新令牌 |

### 成功响应（200）

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "eyJhbGciOiJIUzI1NiJ9..."
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `access_token` | string | JWT 访问令牌 |
| `token_type` | string | 固定 `Bearer` |
| `expires_in` | int | 过期时间（秒） |
| `refresh_token` | string | 刷新令牌 |

### 错误响应

| 状态码 | 场景 |
|--------|------|
| 400 | `grant_type` 不支持（如 `client_credentials`） |
| 401 | 密码错误、验证码错误 |

---

## POST /oauth/sms/send

发送短信验证码。

### 请求

**Content-Type:** `application/json`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `phone` | string | 是 | 手机号 |

### 成功响应（200）

```json
{
  "message": "code sent"
}
```

### 错误响应

| 状态码 | 场景 |
|--------|------|
| 400 | 缺少 `phone`、手机号格式无效 |

---

## GET /userinfo

获取当前用户信息（OIDC 标准）。

### 请求

**Headers:**

| 头 | 必填 | 说明 |
|----|------|------|
| `Authorization` | 是 | `Bearer <access_token>` |

### 成功响应（200）

```json
{
  "sub": "u1",
  "phone": "13800138000",
  "phone_verified": true,
  "nickname": "138****8000",
  "picture": null,
  "updated_at": "2026-07-11T12:00:00Z"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `sub` | string | 用户唯一标识 |
| `phone` | string | 手机号 |
| `phone_verified` | boolean | 手机号是否已验证 |
| `nickname` | string | 昵称（默认脱敏手机号） |
| `picture` | string? | 头像 URL |
| `updated_at` | string | ISO 8601 更新时间 |

不暴露 `password_hash` 等敏感字段。

### 错误响应

| 状态码 | 场景 |
|--------|------|
| 401 | 未携带 `Authorization` 头 |
| 401 | `access_token` 无效或已过期 |
