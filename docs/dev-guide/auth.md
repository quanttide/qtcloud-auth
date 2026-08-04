# 鉴权架构与安全现状 · qtcloud-auth

> 本文记录当前鉴权实现、暴露面与分层防护规划。CI 见 [ci.md](./ci.md)，基础设施见 [iac.md](./iac.md)。

## 一、鉴权分层总览

```
公网 → FC HTTP 触发器（authType=anonymous，无网关层鉴权）── 当前
公网 → 系统级 API 网关（统一鉴权/限流/WAF）────────────── 规划
              │
              ▼
      应用层（Go 服务，JWT 鉴权在这里）
        ├─ /oauth/token        公开（签发令牌；OAuth 设计使然）
        ├─ /oauth/sms/send     公开（应用内 60s 频率限制）
        └─ /userinfo           需 Bearer JWT（AuthMiddleware 验签）
```

**核心结论：JWT 鉴权只在应用层；FC 触发器为 `anonymous`（任何人都可调用函数，但用户数据端点有 JWT 保护）。**

## 二、当前实现（代码事实）

### 令牌签发（`internal/api/oauth.go`）

- **授权方式**：`password`（用户名+密码）、`sms_code`（手机验证码）、`refresh_token`
- **access_token**：JWT（HS256，`generates.NewJWTAccessGenerate`），claims 为 `sub`（用户 UUID）、`aud`（客户端）、`exp`；**无状态验签**（/userinfo 只验签不查库）
- **refresh_token**：不透明随机串，存 `oauth_tokens` 表（GORM `oauth2.TokenStore`，见 `internal/store/token_store.go`）；**刷新即轮换**，旧 token 作废
- **密码校验**：bcrypt（`PasswordAuthorizationHandler`）

### 令牌校验（`internal/api/handler.go`）

- `parseToken`：HS256 验签（拒绝非 HMAC 算法，防算法混淆）
- `AuthMiddleware`：解析 Bearer JWT → claims 注入上下文；缺失/无效 → 401

### 验证码（`internal/api/sms.go`）

- 5 分钟 TTL、60s 重发限制、一次性使用（Used 标记）——全部走共享存储（FC 多实例可用）
- 发送器当前为 `console`（日志输出）；`SMS_TEST_CODE` 固定验证码仅用于集成测试

### 管理员（`cmd/server/main.go`）

- `EnsureAdmin`：启动时若无 `admin` 则用 `ADMIN_PASSWORD` 创建（bcrypt 哈希）
- **已知**：admin 已存在时不会更新密码；轮换需删库重建（见 iac.md）

### 密钥（环境变量）

| 变量 | 现状 | 说明 |
|------|------|------|
| `JWT_SECRET` | 已注入强随机值（64 字符） | 之前为空时用默认 `quanttide-auth-secret`（已轮换，旧 token 全部失效） |
| `ADMIN_PASSWORD` | 已注入强随机值（20 字符） | 之前为空时用默认 `123456`（已轮换） |
| `DATABASE_URL` | FC env 明文含数据库密码 | 可被 `aliyun fc GetFunction` 提取，已知风险（方向：配置中心/KMS） |

## 三、当前暴露面与风险

| 风险 | 等级 | 现状 | 缓解/规划 |
|------|------|------|-----------|
| FC 入口无网关层鉴权（anonymous） | 中 | 任何人可调用函数端点 | 系统级 API 网关接入（鉴权/限流/WAF） |
| `/oauth/token` 无限流防刷 | 中 | 可暴力尝试（bcrypt 校验有成本，但无 IP 级限流） | 网关限流 / ROADMAP P3.1 频率限制 |
| JWT 为 HS256 单密钥 | 低 | 密钥已强化；无算法协商风险 | ROADMAP P3.2 RS256 非对称 |
| refresh_token 存储于 DB | 低 | 轮换 + 过期摘除 | — |
| 验证码仅 console 发送器 | 低 | 生产不真实下发 | Phase 2：阿里云 SMS |
| 无 MFA / 会话管理 | 规划 | — | ROADMAP P1/P3 |

## 四、分层防护路线（按优先级）

1. **系统级 API 网关**（`api.quanttide.com/qtcloud-auth`）：网关层统一鉴权、限流、WAF；接入后 FC 触发器可改 `disableURLInternet=true` 关闭公网直连
2. **FC 触发器 `authType=function`**：仅适合服务间调用（签名头不适合浏览器/移动端 OAuth 客户端），不用于面向用户入口
3. **密钥管理**：`JWT_SECRET` / `ADMIN_PASSWORD` / `DATABASE_URL` 从环境变量迁移到 FC 配置中心 / KMS
4. **ROADMAP P3**：IP/手机号级频率限制、RS256、MFA、健康检查与指标

## 五、鉴权决策记录

- **为什么 token 端点公开**：OAuth 2.0 的令牌端点必须匿名可达（客户端无 token 时获取 token）
- **为什么 access_token 无状态**：JWT 验签零存储，FC 多实例水平扩容无需共享会话
- **为什么 refresh_token 有状态**：不透明随机串必须查表验证；存共享存储使其跨实例可用（曾因内存存储导致跨实例刷新失败，已修复）
- **为什么验证码走共享存储**：FC 多实例下验证码必须全局可见（曾因内存 map 导致跨实例无法校验）
- **为什么 FC 层不做 JWT**：FC 网关层鉴权（function 签名）面向服务间调用，面向用户的 JWT 校验在应用层更灵活（可随时改算法/吊销策略）；网关接入后由网关统一处理
