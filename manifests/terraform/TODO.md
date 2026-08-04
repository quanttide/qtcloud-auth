# Terraform 部署 TODO

## 上线前(阻塞项)

- [ ] provider 存储后端对接 PostgreSQL（当前为 BuntDB，FC 容器磁盘为临时存储且多实例不共享；对齐 qtcloud-pay 的 `DB_DRIVER` / `DATABASE_URL` 运行时约定），实现后 `DB_DRIVER` / `DATABASE_URL` 环境变量才会被读取
- [ ] Dockerfile 多阶段构建（见 ROADMAP P3.5，生产镜像 < 20MB），镜像由 deploy-provider workflow 双通道发布（Docker Hub + ACR `registry.cn-hangzhou.aliyuncs.com/quanttide/qtcloud-auth-provider`，`provider/*` tag 触发）
- [ ] GitHub Actions workflow：terraform init / plan / apply，引用 org secrets（`ALIYUN_ACCESS_KEY_ID` / `ALIYUN_ACCESS_KEY_SECRET`）；ACR 命名空间/仓库幂等创建
- [ ] 复制 `terraform.tfvars.example` → `terraform.tfvars` 填写真实值（含 `jwt_secret` / `admin_password`），执行 `terraform apply`
- [ ] 上线验证：通过 `terraform output fc_http_url` 调用 `/oauth/token`、`/userinfo` 确认全链路可用

## CI 与安全(P1)

- [ ] 短信发送器接入阿里云 SMS（当前仅 `console`，验证码输出到日志；参考 data/library/aliyun-sms-integration.md），并配置 `SMS_DRIVER=aliyun` 及对应密钥
- [ ] 密码/密钥不再明文落入 tfstate（改 FC 配置中心 / 密钥管理注入）
- [ ] 本地/开发凭证收敛为只读（第 2 层生产保护），apply 仅存于 CI

## 后续(P2)

- [ ] **OIDC 联邦升级**：RAM 角色信任 GitHub OIDC，workflow 不再需要 Secret Key，CI 不落任何长期凭证
- [ ] 权限收敛为最小策略集
- [ ] API 网关统一接入 `api.quanttide.com/qtcloud-auth`（系统层面预留，另行规划）
- [ ] VPC/RDS 抽离到系统级 IaC（quanttide 体系统一管理，如 quanttide-infra），qtcloud-auth 仅保留 FC 与数据库
