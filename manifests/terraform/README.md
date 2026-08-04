# qtcloud-auth 部署选型（IaC）

依据 qtcloud-auth 开发运维指南（[docs/dev-guide.md](../../docs/dev-guide.md)）与路线图（[ROADMAP.md](../../ROADMAP.md)），部署方案对齐 qtcloud-pay 已落地选型（阿里云 RDS Serverless + FC 容器镜像 + 统一 API 网关预留），作为 Terraform 基础设施代码的设计依据。

## 部署选型

| 维度 | 选型 | 说明 |
|------|------|------|
| 数据库 | 阿里云 RDS Serverless（PostgreSQL） | 系统级共享实例（由 quanttide-platform 管理）；provider 已支持 GORM 方言切换（开发 SQLite / 生产 PostgreSQL，见 docs/fc-adaptation.md Phase 1） |
| 服务计算 | FaaS（函数计算）+ 容器镜像 | Dockerfile 构建镜像，双通道发布（Docker Hub 对外分发 + 阿里云 ACR 同地域直拉，FC 中国区拉不到 Docker Hub）；服务无需常驻、按调用计费 |
| 存储与网络 | 随计算/数据库一并解决 | VPC 内网互通（RDS ↔ FC） |
| API 网关 | **预留（系统层面统一规划）** | 统一 `api.quanttide.com`，路径按应用名（如 `/qtcloud-auth`）；不在本应用 IaC 范围内，由系统级网关统一接入 |

## 本 IaC 范围

- **系统级共享**（quanttide 体系统一管理，`quanttide-<env>` 命名）：VPC / 交换机 / 安全组、RDS 实例
- **应用级**（`qtcloud-auth-<env>` 命名）：数据库与账号（`qtcloud_auth`）、FC 函数与默认角色
- **不含** API 网关、域名、DNS（系统层面预留）

## 运行时配置（注入 FC 环境变量）

对齐 provider 运行时约定（[src/provider/main.go](../../src/provider/main.go)）：

| 环境变量 | 来源 | 说明 |
|----------|------|------|
| `JWT_SECRET` | `var.jwt_secret` | HS256 签名密钥，生产必须显式配置 |
| `ADMIN_PASSWORD` | `var.admin_password` | 种子管理员密码，生产必须显式配置 |
| `SMS_DRIVER` | `var.sms_driver` | 当前仅 `console`；阿里云短信接入见 dev-guide 与 [aliyun-sms-integration](../../../../data/library/aliyun-sms-integration.md) |
| `DB_DRIVER` / `DATABASE_URL` | RDS 资源自动生成 | 对齐共享 RDS 方案，provider 已支持（GORM，见 docs/fc-adaptation.md） |

## 使用

```sh
terraform init \
  -backend-config="bucket=quanttide-terraform-state" \
  -backend-config="key=qtcloud-auth/terraform.tfstate" \
  -backend-config="region=cn-hangzhou"
terraform plan -var-file=terraform.tfvars
terraform apply -var-file=terraform.tfvars
```
