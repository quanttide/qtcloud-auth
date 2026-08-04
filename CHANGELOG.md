# Changelog

## 0.1.0-alpha.3 (2026-08-04)

- CI 修复：ACR 地址改走 secret（`ALIYUN_ACR_REGISTRY`），实例专属域名不再写入代码；移除 CI 内 ACR 建仓步骤（RAM 用户无 ACR 权限，aliyun CLI cr API 漂移），仓库由主账号一次性创建（PUBLIC）
- 镜像地址敏感化：`image` 变量不再有默认值，由 CI 注入或 tfvars 提供

## 0.1.0-alpha.2 (2026-08-04)

- 项目结构对齐 Go 社区惯例：入口迁至 `cmd/server`，`api` / `model` 迁入 `internal/`
- Dockerfile：多阶段构建（alpine + musl/CGO），非 root 运行
- CI：新增 deploy-provider workflow（`provider/*` tag 触发；Docker Hub + ACR 双通道发布镜像 + Terraform 部署到 FC）

## 0.1.0-alpha.1 (2026-08-04)

- 存储后端迁移：BuntDB → GORM（开发 SQLite / 生产 PostgreSQL），`DB_DRIVER` / `DATABASE_URL` / `DB_SQLITE_DSN` 环境变量替代 `DB_PATH`
- 密码哈希升级：SHA256 → bcrypt（DefaultCost）
- 短信发送器可配置：新增 `SMS_DRIVER` 环境变量
- 验证码与发送频率限制迁移到共享存储（FC 多实例可用）
- OAuth 令牌存储：GORM 实现 `oauth2.TokenStore`，refresh_token 跨实例可用、刷新轮换作废旧令牌
- 部署：新增 Terraform IaC（FC 3.0 + 共享 RDS，manifests/terraform）与 FC 适配规划（docs/fc-adaptation.md）

## 0.0.1 (2026-07-11)

- 初始化认证服务：OAuth 2.0 令牌端点、短信验证码、用户信息端点
- 集成 pytest 测试套件（11 项全部通过）
- 文档：用户指南、开发运维指南、API 参考
