# Changelog

## 0.1.0-alpha.17 (2026-08-15)

- 新增 `POST /userinfo`：更新当前用户昵称（账号中心编辑昵称所需；仅允许昵称字段，用户名/邮箱/手机号为登录凭证本期锁定；路由用 POST 而非 PATCH，传统版 API 网关不支持 PATCH 方法）

## 0.1.0-alpha.8 (2026-08-04)

- 安全加固：注入真实 `JWT_SECRET` / `ADMIN_PASSWORD`（替换默认值 quanttide-auth-secret / 123456）；部署后需删除旧 admin 记录并由实例冷启动重建（见 docs）

## 0.1.0-alpha.7 (2026-08-04)

- 修复 Ensure RDS 步骤成功路径误报失败：`[ x ] && { exit 1; }` 作为脚本最后一条命令时，条件为假返回 1 即脚本以 1 退出（与 set -e 无关）——改为 if 语句

## 0.1.0-alpha.6 (2026-08-04)

- 修复 Ensure RDS 步骤：`terraform state pull` 中 data source 类型为 `terraform_remote_state`（无 `data.` 前缀），outputs 嵌套在 `.outputs.value` 下——实例 ID 提取修正

## 0.1.0-alpha.5 (2026-08-04)

- CI：保留共享 RDS Serverless AutoPause（省钱）；deploy job 新增「Ensure RDS Instance Running」步骤（terraform 前幂等启动实例并轮询，最多 3 分钟）
- 注：GitHub Actions rerun 使用原 run 的 workflow 快照，workflow 变更后需发新 tag 生效

## 0.1.0-alpha.4 (2026-08-04)

- 修复 FC 无法拉取镜像：关闭 buildx provenance/sbom attestation（其 manifest 平台为 unknown/unknown，FC 镜像优化器拒绝，报 `ImageOptimizingFailed`）

## 0.1.0-alpha.3 (2026-08-04)

- CI 修复：ACR 地址改走 secret（`ALIYUN_ACR_REGISTRY`），实例专属域名不再写入代码；移除 CI 内 ACR 建仓步骤（RAM 用户无 ACR 权限，aliyun CLI cr API 漂移），仓库由主账号一次性创建（PUBLIC）
- 镜像地址敏感化：`image` 变量不再有默认值，由 CI 注入或 tfvars 提供

## 0.1.0-alpha.2 (2026-08-04)

- 项目结构对齐 Go 社区惯例：入口迁至 `cmd/server`，`api` / `model` 迁入 `internal/`
- Dockerfile：多阶段构建（alpine + musl/CGO），非 root 运行

## 0.1.0-alpha.1 (2026-08-04)

- 存储后端迁移：BuntDB → GORM（开发 SQLite / 生产 PostgreSQL），`DB_DRIVER` / `DATABASE_URL` / `DB_SQLITE_DSN` 环境变量替代 `DB_PATH`
- 密码哈希升级：SHA256 → bcrypt（DefaultCost）
- 短信发送器可配置：新增 `SMS_DRIVER` 环境变量
- 验证码与发送频率限制迁移到共享存储（FC 多实例可用）
- OAuth 令牌存储：GORM 实现 `oauth2.TokenStore`，refresh_token 跨实例可用、刷新轮换作废旧令牌

## 0.0.1 (2026-07-11)

- 初始化 provider 模块：OAuth 2.0 认证提供者
- 支持密码登录、短信验证码登录（含自动注册）、令牌刷新
- 支持 JWT (HS256) 签发与验证
- 支持 AuthMiddleware 中间件
