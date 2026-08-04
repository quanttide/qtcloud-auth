# Terraform 部署 TODO

## 上线前(阻塞项)

- [x] provider 存储后端对接 PostgreSQL（GORM，`DB_DRIVER` / `DATABASE_URL` 运行时约定已生效，见 docs/fc-adaptation.md Phase 1；本地开发默认 SQLite）
- [x] go-oauth2 refresh_token 存储迁移到共享存储（GORM TokenStore，见 internal/store/token_store.go，FC 多实例刷新可用）
- [x] Dockerfile 多阶段构建（alpine + musl/CGO，非 root），镜像由 deploy-provider workflow 双通道发布（Docker Hub + ACR 个人版实例，`provider/*` tag 触发）
- [x] GitHub Actions workflow：terraform init / plan / apply，引用 org secrets（`ALIYUN_ACCESS_KEY_ID` / `ALIYUN_ACCESS_KEY_SECRET`，另需 `DB_PASSWORD` / `JWT_SECRET` / `ADMIN_PASSWORD` / ACR / Docker Hub 凭证）；ACR 仓库由主账号创建（PUBLIC），CI 不建仓
- [x] 上线验证：`terraform apply` 后通过 `terraform output fc_http_url` 调用 `/oauth/token`、`/userinfo` 确认全链路可用（2026-08-04 alpha.4 验证通过）
- [x] 共享 RDS 实例 AutoPause **保留**（Serverless 省钱）；部署流程已适配：deploy job 在 terraform 前 Ensure RDS Running（幂等启动 + 轮询）；代价：FC 首次访问有 RDS 唤醒延迟（~1 分钟）

## CI 与安全(P1)

- [ ] 镜像体积优化到 <20MB（CGO+musl sqlite 致 34.3MB；需纯 Go sqlite 驱动或生产仅 postgres 镜像）
- [ ] 短信发送器接入阿里云 SMS（当前仅 `console`，验证码输出到日志；参考 data/library/aliyun-sms-integration.md），并配置 `SMS_DRIVER=aliyun` 及对应密钥
- [ ] 密码/密钥不再明文落入 tfstate（改 FC 配置中心 / 密钥管理注入）
- [ ] 本地/开发凭证收敛为只读（第 2 层生产保护），apply 仅存于 CI

## 后续(P2)

- [ ] **OIDC 联邦升级**：RAM 角色信任 GitHub OIDC，workflow 不再需要 Secret Key，CI 不落任何长期凭证
- [ ] 权限收敛为最小策略集
- [ ] API 网关统一接入 `api.quanttide.com/qtcloud-auth`（系统层面预留，另行规划）
- [ ] VPC/RDS 抽离到系统级 IaC（quanttide 体系统一管理，如 quanttide-infra），qtcloud-auth 仅保留 FC 与数据库
