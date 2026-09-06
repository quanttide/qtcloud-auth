# provider 协议合规升级规划

目标：`src/provider` 严格遵循 OAuth 2.0（RFC 6749 / 6750）并成为完整的 OpenID Connect 提供方（OIDC Core 1.0）。当前实现是一个带私有扩展的 OAuth 2.0 子集，存在若干与规范的偏差；本规划逐项消除偏差，再补齐 OIDC 能力。

## 偏差清单

对照代码（`internal/api/oauth.go`、`response.go`、`middleware.go`）与规范，当前偏差如下：

- 客户端认证被架空：`ClientInfoHandler` 硬编码返回固定客户端，token 请求不校验 `client_id`（RFC 6749 §2.3、§4.3.2 要求公共客户端也须提供并校验）；
- 错误状态码不符：密码错误、验证码错误返回 401，规范规定 token 端点错误一律 400，仅 `invalid_client` 可用 401（RFC 6749 §5.2）；
- 受保护资源缺少 `WWW-Authenticate: Bearer error="invalid_token"` 响应头（RFC 6750 §3）；
- userinfo claims 命名偏离 OIDC：`phone` / `phone_verified` 应为 `phone_number` / `phone_number_verified`，`updated_at` 应为 NumericDate 而非 RFC3339 字符串（OIDC Core §5.1）；
- 缺少 OIDC 核心要素：无 `id_token` 签发、无 scope 支持、无 authorization code 流程、无发现与密钥分发端点。

## Phase 1 — 错误处理对齐 RFC 6749 / 6750

1. token 端点全部错误改为 400，`invalid_client` 场景保留 401；
2. `WriteOAuthError` 增加可选 `WWW-Authenticate` 头参数，受保护资源（userinfo、password/change 等）返回 401 时携带 `WWW-Authenticate: Bearer error="invalid_token"`；
3. 自定义扩展 `sms_code` 在文档中标注为私有授权类型，错误码沿用标准集合（`invalid_request` / `invalid_grant` / `unsupported_grant_type`）。

验证：集成测试断言各错误场景的状态码与响应头。

## Phase 2 — 客户端认证与客户端模型

1. token 请求必须携带 `client_id`，未知客户端返回 `invalid_client`；
2. 保留「第一方公共客户端」模型：无 secret，但 `client_id` 必须匹配已注册值；`qtcloud-auth` 作为首个注册客户端；
3. 为后续第三方接入预留机密客户端支持（`client_secret` 走 Basic 或 body 认证，按 §2.3.1 禁止同时出现在两处）。

验证：无 `client_id` 与错误 `client_id` 的请求被拒绝。

## Phase 3 — OIDC claims 对齐（破坏性变更）

1. userinfo 响应字段重命名：`phone` → `phone_number`，`phone_verified` → `phone_number_verified`；
2. `updated_at` 改为 NumericDate（Unix 秒数）；
3. 补充 `email_verified` 字段；
4. 同步更新 API 文档与前端/客户端调用方，此阶段为破坏性变更，需协调发版。

验证：claims 与 OIDC Core §5.1 标准定义逐项比对。

## Phase 4 — scope 与 id_token 签发

1. 引入 scope：token 请求支持 `scope` 参数，缺省 `openid`；userinfo 按授权 scope 过滤返回 claims（`profile` / `email` / `phone` 分别对应三组标准 claims）；
2. 授权类型含 `openid` scope 时，token 响应追加 `id_token`（RS256 签名，claims 含 `iss` / `sub` / `aud` / `exp` / `iat` / `nonce`）；
3. `iss` 取服务对外根 URL，经环境变量注入。

验证：解析 id_token 校验签名与标准 claims。

## Phase 5 — authorization code 流程与发现端点

1. 实现 authorization code 流程（§4.1），支持 PKCE（RFC 7636），公共客户端强制 `S256`；
2. 新增 `GET /.well-known/openid-configuration`（发现文档）与 `GET /keys`（JWKS，RFC 7517）；
3. 现有 `/oauth/register`、`/oauth/sms/send`、`/oauth/password/change` 保持私有扩展端点不变，不在发现文档中声明。

验证：用标准 OIDC 客户端库（如 `oidc-client-ts`）完成登录闭环。

## Phase 6 — 令牌生命周期增强（P2）

1. refresh token 轮换：每次刷新签发新 refresh_token 并作废旧值；
2. 令牌撤销端点 `POST /oauth/revoke`（RFC 7009）；
3. 内省端点 `POST /oauth/introspect`（RFC 7662），供资源服务器使用。

## 实施顺序与约束

Phase 1 → 2 可直接实施，无兼容性影响；Phase 3 是破坏性变更，实施前需盘点调用方；Phase 4 → 5 依赖密钥管理与 `iss` 配置就绪；Phase 6 优先级最低。

统一约束：

- 签名算法固定 RS256，密钥经环境变量注入并支持 JWKS 轮换；
- 每阶段合并后更新 `docs/api-references/provider.md`，Phase 3 完成前文档不得再声称「遵循 OIDC 标准」；
- 每阶段以集成测试作为验收事实源，`pytest -m integration` 全绿方可合并。
