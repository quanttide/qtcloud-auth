# 基础设施指南 · qtcloud-auth（IaC）

> CI 流水线与踩坑见 [ci.md](./ci.md)；本地开发见 [index.md](./index.md)。
> 本文覆盖部署选型、Terraform 结构、ACR / RDS / FC 的运维经验。

## 部署选型

| 维度 | 选型 | 说明 |
|------|------|------|
| 数据库 | 阿里云 RDS Serverless（PostgreSQL 18） | 系统级**共享实例**（quanttide-platform 管理）；各应用独立库/账号 |
| 服务计算 | FC 3.0 custom-container | Dockerfile 构建镜像，双通道发布（Docker Hub 对外 + ACR 部署） |
| 镜像源 | ACR 个人版（PUBLIC 仓库） | FC 免凭证直拉；实例专属域名走 secret |
| 网络 | VPC 内网 | RDS ↔ FC 内网互通 |
| API 网关 | **预留（系统层面统一规划）** | 统一 `api.quanttide.com`，不在本应用 IaC 范围 |

## Terraform 结构（manifests/terraform）

| 文件 | 内容 |
|------|------|
| `platform.tf` | 引用 quanttide-platform 远程状态（VPC / 交换机 / 安全组 / 共享 RDS 实例） |
| `rds.tf` | 应用级数据库 + 账号 + DBOwner 授权（`qtcloud_auth`） |
| `fc.tf` | FC 3.0 函数（custom-container + VPC 配置 + 环境变量注入）+ RAM 角色 + HTTP 触发器 |
| `variables.tf` | `image` **无默认值**（实例地址敏感，由 CI 注入 `TF_VAR_image`）；`db_password` / `jwt_secret` / `admin_password` 为敏感变量 |
| `providers.tf` | OSS 远程状态（`qtcloud-auth/terraform.tfstate`） |

资源命名：应用级 `<app>-<env>`（如 `qtcloud-auth-prod`）；系统级由 quanttide-platform 管理。

## ACR 镜像仓库

- **仓库由主账号一次性创建**（`quanttide/qtcloud-auth-provider`，可见性 **PUBLIC**——FC 免凭证直拉的前提）
- 实例地址（`crpi-*.personal.cr.aliyuncs.com`）属敏感信息：只放 secret `ALIYUN_ACR_REGISTRY`，不写代码/文档
- **镜像构建必须 `provenance: false`**（buildx attestation 的 unknown/unknown 平台会被 FC 拒绝，见 ci.md 坑 #1）
- 验证仓库状态：`docker manifest inspect <registry>/<ns>/<repo>:<tag>`（`no such manifest` = 存在仅缺 tag；`denied` = 不存在或私有）

## RDS 共享实例

- **Serverless + AutoPause（保留，省钱）**：空闲自动暂停；暂停期间 RDS 资源操作报 `IncorrectDBInstanceState`，FC 首次访问有唤醒延迟（~1 分钟）
- **CI 适配**：deploy job「Ensure RDS Instance Running」——terraform 前幂等启动并轮询（最多 3 分钟），实例 ID 从 platform 远程状态提取
- **本地运维**：先查实例状态再操作（`aliyun rds DescribeDBInstances`）；STOPPED 先 `StartDBInstance`

### RDS 运维通道：临时公网连接

需要连库执行 SQL（如删除 admin 重建密码）时，连接串是内网地址，本机不可达。可行路径：**临时公网通道，用完即回收**（完整命令见 data/context/ci-troubleshooting.md 第五章）：

```sh
# 申请公网地址 → 白名单 Append 本机 IP → pgx/psql 连库执行 → 白名单 Cover 恢复 → 释放公网
aliyun rds AllocateInstancePublicConnection --DBInstanceId $IID --ConnectionStringPrefix <前缀> --Port 5432
aliyun rds ModifySecurityIps --DBInstanceId $IID --SecurityIps <本机公网IP> --ModifyMode Append
# ... 执行 SQL ...
aliyun rds ModifySecurityIps --DBInstanceId $IID --SecurityIps "<原列表>" --DBInstanceIPArrayName default --ModifyMode Cover
aliyun rds ReleaseInstancePublicConnection --DBInstanceId $IID --CurrentConnectionString <公网地址>
```

要点：操作前先启动实例；先读原白名单再改；窗口越短越好；完成后复查恢复。

## 密钥与配置管理（现状与方向）

| 项 | 现状 | 方向 |
|----|------|------|
| `JWT_SECRET` / `ADMIN_PASSWORD` / `DB_PASSWORD` | FC 环境变量注入（明文落 tfstate 已知风险） | FC 配置中心 / 密钥管理（KMS）注入 |
| `DATABASE_URL`（含明文密码） | FC env 可读（`aliyun fc GetFunction` 能提取） | 同上 |
| CI 凭证 | RAM 用户 AccessKey（org secrets） | OIDC 联邦升级（CI 不落长期凭证） |
| admin 密码轮换 | 删库重建（`DELETE FROM users WHERE username='admin'` + 冷启动重新 seed） | 提供密码变更 API |

## 已知问题 / 待办

- 镜像体积 34.3MB（CGO+musl sqlite 所致）；<20MB 需纯 Go sqlite 驱动或生产仅 postgres 镜像（P2）
- 过期验证码未清理（P2）
- 短信仍为 `console` 驱动（Phase 2：接入阿里云 SMS）
- API 网关统一接入（系统层面预留）
- FC 函数未配 logConfig（拉镜像失败等错误无日志可查，建议配 SLS）
