# 用户指南 · 接入 qtcloud-auth

## 概述

qtcloud-auth 提供 OAuth 2.0 标准的认证授权服务，支持用户名密码登录、手机验证码登录（含自动注册）、令牌刷新三种认证方式。

## 前置条件

- 获取服务地址（如 `https://auth.quanttide.com`）
- 获取客户端凭证（如有 client_id / client_secret 要求）

## 认证接入

### 1. 用户名密码登录

向令牌端点发送 `grant_type=password` 请求：

```http
POST /oauth/token
Content-Type: application/x-www-form-urlencoded

grant_type=password&username=admin&password=123456
```

成功响应：

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "eyJhbGciOiJIUzI1NiJ9..."
}
```

### 2. 手机验证码登录（含自动注册）

#### 先获取验证码

```http
POST /oauth/sms/send
Content-Type: application/json

{"phone": "13800138000"}
```

成功响应：

```json
{
  "message": "code sent"
}
```

#### 再用验证码登录

```http
POST /oauth/token
Content-Type: application/x-www-form-urlencoded

grant_type=sms_code&phone=13800138000&code=123456
```

首次使用该手机号时自动创建账号。成功响应格式同密码登录。

### 3. 刷新令牌

```http
POST /oauth/token
Content-Type: application/x-www-form-urlencoded

grant_type=refresh_token&refresh_token=<refresh_token>
```

## 获取用户信息

携带 `access_token` 请求用户信息端点：

```http
GET /userinfo
Authorization: Bearer <access_token>
```

成功响应：

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

## 错误处理

| 状态码 | 说明 | 常见原因 |
|--------|------|----------|
| 400 | 请求参数错误 | grant_type 不支持、缺少必填参数、手机号格式无效 |
| 401 | 认证失败 | 密码错误、验证码错误、令牌过期或无效 |

## 快速集成示例

### Go

```go
type AuthClient struct {
    baseURL    string
    httpClient *http.Client
}

func (c *AuthClient) LoginPassword(username, password string) (*Token, error) {
    resp, err := c.httpClient.PostForm(c.baseURL+"/oauth/token", url.Values{
        "grant_type": {"password"},
        "username":   {username},
        "password":   {password},
    })
    // ...
}

func (c *AuthClient) GetUserInfo(accessToken string) (*UserInfo, error) {
    req, _ := http.NewRequest("GET", c.baseURL+"/userinfo", nil)
    req.Header.Set("Authorization", "Bearer "+accessToken)
    resp, err := c.httpClient.Do(req)
    // ...
}
```

### Python

```python
class AuthClient:
    def __init__(self, base_url: str):
        self.base_url = base_url

    def login_password(self, username: str, password: str) -> dict:
        resp = requests.post(f"{self.base_url}/oauth/token", data={
            "grant_type": "password",
            "username": username,
            "password": password,
        })
        resp.raise_for_status()
        return resp.json()

    def get_userinfo(self, access_token: str) -> dict:
        resp = requests.get(f"{self.base_url}/userinfo",
            headers={"Authorization": f"Bearer {access_token}"})
        resp.raise_for_status()
        return resp.json()
```

### 开发调试

开发环境默认的验证码使用 `ConsoleSender`，验证码直接输出到服务端日志，方便调试。
