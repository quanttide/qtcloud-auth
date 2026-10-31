# qtcloud-auth studio · 量潮身份云管理后台

企业管理账号信息的后台系统（Flutter 客户端，配合 `src/provider` 认证服务使用）。

## 定位

- 面向企业管理员，管理账号体系：用户、角色、权限等
- 后端为同仓 `src/provider`（qtcloud-auth 认证服务），本客户端经 API 调用
- 普通用户的账号/钱包前台是 `domains/quanttide-auth/apps/qtaccount`（账号中心），与本后台分工：后台管别人，前台管自己

## 开发

```bash
flutter pub get
flutter run -d linux --dart-define=QTCLOUD_AUTH_BASE_URL=http://localhost:8080
flutter test
```
