# CI 维护指南 · qtcloud-auth（deploy-provider）

> 基础设施与部署选型见 [iac.md](./iac.md)；本地开发见 [index.md](./index.md)。
> 本文沉淀自 2026-08-04 实战：alpha.2 → alpha.8 七轮发布踩过的全部坑。

## 概述

[.github/workflows/deploy-provider.yml](../../.github/workflows/deploy-provider.yml) 是唯一部署入口：

- **触发**：推送 `provider/*` tag（如 `provider/v0.1.0-alpha.8`）；**无手动触发**（生产保护）
- **job 1 `build-image`**：Dockerfile 多阶段构建 → 双通道发布（Docker Hub + ACR `quanttide/qtcloud-auth-provider`）→ 各带版本 tag 与 `latest`
- **job 2 `deploy`**：Ensure RDS Running → `terraform init/validate/plan/apply`（OSS 远程状态 `qtcloud-auth/terraform.tfstate`），挂 GitHub `production` 环境（可配 required reviewers）
- **并发控制**：`concurrency: deploy-provider`，避免同时 apply 竞争状态

## 凭证与环境变量（org secrets）

| secret | 用途 | 注入方式 |
|--------|------|---------|
| `ALIYUN_ACCESS_KEY_ID` / `ALIYUN_ACCESS_KEY_SECRET` | RAM 用户 AccessKey（terraform + aliyun CLI） | terraform：`ALICLOUD_ACCESS_KEY` / `ALICLOUD_SECRET_KEY`；aliyun CLI：`ALICLOUD_ACCESS_KEY_ID` / `ALICLOUD_ACCESS_KEY_SECRET` |
| `ALIYUN_ACR_USERNAME` / `ALIYUN_ACR_PASSWORD` / `ALIYUN_ACR_REGISTRY` | ACR 固定凭证（docker login）；`REGISTRY` 为实例公网地址（`crpi-*.personal.cr.aliyuncs.com`，控制台可见） | `docker/login-action`；镜像地址拼接待 secret |
| `DB_PASSWORD` / `JWT_SECRET` / `ADMIN_PASSWORD` | RDS 密码 / JWT 密钥 / 管理员密码 | `TF_VAR_db_password` / `TF_VAR_jwt_secret` / `TF_VAR_admin_password` |
| `DOCKERHUB_USERNAME` / `DOCKERHUB_PASSWORD` | Docker Hub 凭证（双通道之一） | `docker/login-action` |

> ⚠️ secrets 为空时 terraform 接受空字符串，provider 会用代码默认值（如 `ADMIN_PASSWORD` 默认 `123456`）——**上线前必须确认 secrets 已配置**（部署后查 FC env 是否有值）。

## 关键坑（实战沉淀，按杀伤力排序）

### 1. buildx attestation 导致 FC 拉不到镜像（最贵的一课）

buildx 默认生成 provenance/sbom attestation，其 manifest 平台为 `unknown/unknown`，**FC 3.0 镜像优化器直接拒绝**：

```
lastUpdateStatusReason = invalid image, platform of image is unknown/unknown
lastUpdateStatusReasonCode = ImageOptimizingFailed
```

函数表现为 `state=Pending` + `imagePrewarmStatus=InProgress` 卡 15 分钟以上，无明确错误（配了 logConfig 才能看到 reason）。**修复**：build-push-action 显式 `provenance: false` + `sbom: false`。排查手段：`docker manifest inspect` 看 index 平台列表；对照实验（换阿里云官方镜像 90s Active 证明问题在镜像不在链路）。

### 2. GitHub Actions rerun 使用原 run 的 workflow 快照

修改 workflow 后 rerun 旧 run，**不会**使用新文件（deploy job 步骤列表可证实）。**必须发新 tag**（workflow 取自 tag 指向的 commit）才能让改动生效。

### 3. bash 脚本最后一条命令的退出码

`[ x ] && { echo err; exit 1; }` 作为脚本**最后一条命令**时：条件为假 → 该行返回 1 → **整个脚本以 1 退出**（与 `set -e` 无关，纯退出码传递）。表现：日志显示成功路径（轮询 1: Running）后立刻 exit 1。**修复**：用 `if [ x ]; then ...; exit 1; fi`（if 条件为假时返回 0）。

### 4. aliyun CLI `version: latest` 漂移

`latest` 装到新版本后 `cr ListInstance --InstanceType` 等参数失效（客户端校验即挂）。**修复**：`aliyun/setup-aliyun-cli-action@v1` 锁版本 `3.4.11`（本机验证 rds API 可用）。同理：其他第三方工具一律锁版本。

### 5. ACR 个人版实例专属域名（Forbidden Host）

老地址 `registry.cn-hangzhou.aliyuncs.com` 会被新实例拒绝（`Forbidden Host`），必须用控制台 `crpi-*.personal.cr.aliyuncs.com`。**实例地址属敏感信息**，只放 secret `ALIYUN_ACR_REGISTRY`，不写进代码/文档。

### 6. 不要在 CI 里建 ACR 仓库

CI 的 RAM 用户**无 ACR 管理权限**；且 aliyun CLI cr API 不稳定。仓库由**主账号一次性创建（PUBLIC）**，CI 只负责 docker login（ACR 固定凭证）推送。教训：管理操作交给有权限的账号，CI 只做有凭证的操作。

### 7. ALICLOUD_* 新旧变量名不互通

terraform 读 `ALICLOUD_ACCESS_KEY` / `ALICLOUD_SECRET_KEY`；aliyun CLI 3.x 读 `ALICLOUD_ACCESS_KEY_ID` / `ALICLOUD_ACCESS_KEY_SECRET` / `ALICLOUD_REGION_ID`。写错不报错，只会在远端 API 失败。

### 8. 含 secret 的值只写 `$GITHUB_ENV`

写 `$GITHUB_OUTPUT` 会被脱敏置空（如镜像名拼接 ACR 地址）。

### 9. 空仓库的 `tags/list` 返回 `NAME_UNKNOWN`

属注册表正常行为，不代表仓库不存在。验证存在性用 `docker manifest inspect`（`no such manifest` = 存在、仅缺 tag）。

### 10. `terraform state pull` 的 JSON 结构

data source 的 type 是 `terraform_remote_state`（**无 `data.` 前缀**），outputs 嵌套在 `.instances[0].attributes.outputs.value` 下：

```sh
terraform state pull | jq -r '.resources[] | select(.type=="terraform_remote_state" and .name=="platform") | .instances[0].attributes.outputs.value.rds_instance_id'
```

### 11. RDS Serverless AutoPause（持续状态）

共享 RDS 实例空闲自动暂停（省钱），暂停时 RDS 资源操作报 `OperationDenied.DBInstanceStatus` / `IncorrectDBInstanceState`（曾致账号创建卡 13 分钟）。**适配**：deploy job 的「Ensure RDS Instance Running」步骤（幂等：Running 跳过，STOPPED 启动并轮询最长 3 分钟）。详见 [iac.md](./iac.md)。

## 发版流程

```sh
# 1. 补 CHANGELOG → 提交（先推修复再发 tag——tag 指向的 commit 决定 CI 用的代码）
# 2. 打 tag 并推送（触发 workflow）
git tag -a provider/v0.1.0-alpha.N -m "provider v0.1.0-alpha.N"
git push origin provider/v0.1.0-alpha.N
# 3. 观察 Actions：build-image → deploy
```

- 失败版本**直接发 alpha.N+1**，不重发旧 tag（语义更干净）
- 发布成功判定：ACR tags/list 匿名可列 + `terraform output fc_http_url` 登录闭环可用

## 调试方法

```sh
gh run view <run-id> --repo quanttide/qtcloud-auth --log-failed   # 失败步骤日志
gh run view <run-id> --repo quanttide/qtcloud-auth --log          # 全部日志
gh run rerun <run-id> --repo quanttide/qtcloud-auth               # 重跑（注意：用旧 workflow 快照）
```

原则：**改 workflow 前本地预演**（本机有 aliyun CLI + terraform + 真实 state/凭证，deploy 每步都能先跑通再发 tag），不要"发 tag → 等 5 分钟 → 失败 → 看日志"循环。
