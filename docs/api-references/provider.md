# API 参考

认证服务提供以下端点：

- `POST /oauth/register`：用户名密码注册；
- `POST /oauth/token`：统一认证入口；
- `POST /oauth/sms/send`：发送短信验证码；
- `GET /userinfo`：获取当前用户信息；
- `POST /userinfo`：更新个人资料；
- `POST /oauth/password/change`：修改密码。

除注册与认证入口外，其余端点均须携带 `Authorization` 头（`Bearer <access_token>`）。

## POST /oauth/register

用户名密码注册，可选补充邮箱、手机号与昵称。请求体为 JSON：

- `username`：必填，3-64 个字符；
- `password`：必填，至少 6 个字符；
- `email`：可选，须为合法邮箱格式；
- `phone`：可选，须为合法手机号格式；
- `nickname`：可选，缺省使用用户名。

用户名、邮箱、手机号三者均须唯一，任一冲突返回 409。注册成功即签发令牌，响应与 `/oauth/token` 相同（见下文）。

请求体非法、字段缺失或格式无效返回 400。

## POST /oauth/token

统一认证入口。请求以表单编码（`application/x-www-form-urlencoded`）提交，所有方式均须携带 `grant_type` 参数，服务端据此分发认证策略。

密码登录时携带 `grant_type=password`，并附 `username` 与 `password`。`username` 支持用户名、邮箱或手机号任一登录标识。

短信验证码登录时携带 `grant_type=sms_code`，并附 `phone` 与 `code`。首次使用该手机号时自动注册账号，并将手机号标记为已验证。

刷新令牌时携带 `grant_type=refresh_token`，并附 `refresh_token` 参数。

成功响应返回 200，JSON 字段如下：

```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 604800,
  "refresh_token": "eyJhbGciOiJSUzI1NiJ9..."
}
```

- `access_token`：JWT 访问令牌，RS256 签名，有效期 7 天；
- `token_type`：固定为 `Bearer`；
- `expires_in`：过期时间，单位秒；
- `refresh_token`：刷新令牌，有效期 30 天。

错误响应中，`grant_type` 不支持返回 400；密码错误或验证码无效、已过期返回 401。

## POST /oauth/sms/send

发送短信验证码。请求体为 JSON，仅需提供 `phone` 字段（手机号）。

成功响应返回 200：

```json
{
  "message": "code sent"
}
```

缺少 `phone` 或手机号格式无效时返回 400。

## GET /userinfo

获取当前用户信息，字段遵循 OIDC 标准 claims。

成功响应返回 200：

```json
{
  "sub": "u1",
  "username": "alice",
  "email": "alice@example.com",
  "phone": "13800138000",
  "phone_verified": true,
  "nickname": "Alice",
  "picture": null,
  "updated_at": "2026-07-11T12:00:00Z"
}
```

- `sub`：用户唯一标识；
- `username`：用户名；
- `email`：邮箱，可为空；
- `phone`：手机号，可为空；
- `phone_verified`：手机号是否已验证；
- `nickname`：昵称；
- `picture`：头像 URL，可为空；
- `updated_at`：更新时间，ISO 8601 格式。

响应不包含 `password_hash` 等敏感字段。

未携带 `Authorization` 头、或 `access_token` 无效或已过期时，返回 401；令牌对应的用户不存在时返回 404。

## POST /userinfo

更新个人资料。当前仅支持更新昵称，用户名、邮箱、手机号作为登录凭证不允许通过此端点修改。路由使用 `POST` 而非 `PATCH`，原因是传统版 API 网关不支持 `PATCH` 方法。

请求体为 JSON，提供 `nickname` 字段，必填且不超过 64 个字符。

成功响应返回 200：

```json
{
  "sub": "u1",
  "nickname": "Alice"
}
```

缺少 `Authorization` 头或令牌无效返回 401；`nickname` 缺失、为空或超过 64 个字符返回 400。

## POST /oauth/password/change

修改当前登录用户密码。请求体为 JSON：

- `old_password`：当前密码；
- `new_password`：新密码，至少 6 个字符。

服务端验证旧密码正确后，以 Argon2id 哈希存储新密码。成功响应返回 200：

```json
{
  "success": true
}
```

未携带 `Authorization` 头或令牌无效返回 401；旧密码错误返回 401；新密码少于 6 个字符或请求体非法返回 400。
