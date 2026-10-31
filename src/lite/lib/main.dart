// 量潮账号中心（lite）入口：AuthGate 依据本地 JWT 决定登录页/主界面。
// 用户端自服务客户端——查看/编辑自己的账号信息、修改密码（企业管理走 qtcloud-auth studio）。
// API 基址经 --dart-define 注入（QTCLOUD_AUTH_BASE_URL，本地联调 = localhost）。

import 'package:flutter/material.dart';

import 'screens/account_screen.dart';
import 'screens/login_screen.dart';
import 'services/auth_service.dart';

void main() {
  runApp(MainApp(auth: AuthService()));
}

class MainApp extends StatelessWidget {
  const MainApp({super.key, required this.auth});

  final AuthService auth;

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: '量潮账号中心',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: Colors.indigo),
      ),
      home: AuthGate(auth: auth),
    );
  }
}

/// 登录闸门：本地有 token 进主界面，否则进登录页；登出回登录页。
class AuthGate extends StatefulWidget {
  const AuthGate({super.key, required this.auth});

  final AuthService auth;

  @override
  State<AuthGate> createState() => _AuthGateState();
}

class _AuthGateState extends State<AuthGate> {
  bool? _hasToken;

  @override
  void initState() {
    super.initState();
    _init();
  }

  Future<void> _init() async {
    final token = await widget.auth.storedToken();
    if (mounted) {
      setState(() => _hasToken = token != null && token.isNotEmpty);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_hasToken == null) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    if (_hasToken == true) {
      return AccountShell(auth: widget.auth, onLogout: _logout);
    }
    return LoginScreen(auth: widget.auth, onSuccess: _login);
  }

  void _login() => setState(() => _hasToken = true);

  void _logout() => setState(() => _hasToken = false);
}

/// 主界面：账号信息 + 登出（钱包属支付域，走 qtcloud-pay lite）。
class AccountShell extends StatelessWidget {
  const AccountShell({super.key, required this.auth, required this.onLogout});

  final AuthService auth;
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('账号信息'),
        actions: [
          IconButton(
            tooltip: '退出登录',
            icon: const Icon(Icons.logout),
            onPressed: () async {
              await auth.logout();
              onLogout();
            },
          ),
        ],
      ),
      body: AccountScreen(auth: auth),
    );
  }
}
