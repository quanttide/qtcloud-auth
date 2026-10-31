# qtcloud-auth lite · 量潮账号中心（lite）

用户端自服务客户端（Flutter）——**用户管理自己的账号信息**：登录、查看/编辑昵称、绑定信息展示、修改密码。

## 定位

- 面向普通用户：管自己
- 企业管理（用户/角色/权限的管理操作）走同仓 `src/studio`（管理后台），两者同一后端 `src/provider`
- 钱包/支付属支付域，走 `qtcloud-pay` 的 `src/lite`（量潮钱包）
- 前身是 `domains/quanttide-auth/apps/qtaccount`（账号+钱包双 tab 一体），按域拆分后账号侧落此

## 开发

```bash
flutter pub get
flutter run -d linux --dart-define=QTCLOUD_AUTH_BASE_URL=http://localhost:8080
flutter test
```
