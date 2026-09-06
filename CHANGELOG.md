# Changelog

provider 模块的变更记录见 `src/provider/CHANGELOG.md`，此处仅记录仓库级变更。

## Unreleased

- 文档：API 参考迁至 `docs/api-references/provider.md` 并按《量潮科技文档格式章程》重写（表格改段落与列表，标题收敛至三级）
- 文档：新增端点 `/oauth/register`、`POST /userinfo`、`/oauth/password/change`，修正 userinfo 响应字段与令牌有效期
- 规划：重写 `src/provider/ROADMAP.md` 为协议合规升级规划（严格对齐 RFC 6749 / 6750 与 OIDC Core）

## 0.0.1 (2026-07-11)

- 初始化认证服务仓库：集成 pytest 测试套件（11 项全部通过）
- 文档：用户指南、开发运维指南、API 参考
