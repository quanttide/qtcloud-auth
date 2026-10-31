// 账号信息页：查看用户名/昵称/手机号/邮箱 + 编辑昵称 + 修改密码入口。

import 'package:flutter/material.dart';

import '../services/account_api.dart';
import '../services/auth_service.dart';
import 'change_password_screen.dart';

class AccountScreen extends StatefulWidget {
  const AccountScreen({super.key, required this.auth, this.api});

  final AuthService auth;

  /// 测试注入替身；生产按当前 token 创建。
  final AccountApi? api;

  @override
  State<AccountScreen> createState() => _AccountScreenState();
}

class _AccountScreenState extends State<AccountScreen> {
  late AccountApi _api;
  UserInfo? _info;
  String? _error;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _init();
  }

  Future<void> _init() async {
    if (widget.api != null) {
      _api = widget.api!;
    } else {
      final token = await widget.auth.storedToken();
      if (token == null) {
        setState(() => _error = '未登录');
        return;
      }
      _api = AccountApi(baseUrl: widget.auth.baseUrl, token: token);
    }
    await _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final info = await _api.fetchUserInfo();
      if (mounted) {
        setState(() {
          _info = info;
          _loading = false;
        });
      }
    } catch (_) {
      if (mounted) {
        setState(() {
          _error = '加载账号信息失败，请稍后重试';
          _loading = false;
        });
      }
    }
  }

  Future<void> _editNickname() async {
    final controller = TextEditingController(text: _info?.nickname ?? '');
    final nickname = await showDialog<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('编辑昵称'),
        content: TextField(
          controller: controller,
          autofocus: true,
          maxLength: 64,
          decoration: const InputDecoration(labelText: '昵称'),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, controller.text.trim()),
            child: const Text('保存'),
          ),
        ],
      ),
    );
    if (nickname == null || nickname.isEmpty || nickname == _info?.nickname) {
      return;
    }
    try {
      await _api.updateNickname(nickname);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('昵称已更新')));
      }
      await _load();
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('昵称更新失败，请稍后重试')));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_error != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(_error!),
            const SizedBox(height: 12),
            FilledButton(onPressed: _load, child: const Text('重试')),
          ],
        ),
      );
    }
    final info = _info!;
    return ListView(
      children: [
        ListTile(
          leading: const Icon(Icons.badge_outlined),
          title: const Text('昵称'),
          subtitle: Text(info.nickname.isEmpty ? '未设置' : info.nickname),
          trailing: const Icon(Icons.edit_outlined),
          onTap: _editNickname,
        ),
        const Divider(height: 1),
        ListTile(
          leading: const Icon(Icons.person_outline),
          title: const Text('用户名'),
          subtitle: Text(info.username.isEmpty ? '-' : info.username),
        ),
        ListTile(
          leading: const Icon(Icons.phone_outlined),
          title: const Text('手机号'),
          subtitle: Text(info.phone.isEmpty ? '未绑定' : info.phone),
        ),
        ListTile(
          leading: const Icon(Icons.email_outlined),
          title: const Text('邮箱'),
          subtitle: Text(info.email.isEmpty ? '未绑定' : info.email),
        ),
        ListTile(
          leading: const Icon(Icons.shield_outlined),
          title: const Text('角色'),
          subtitle: Text(info.role.isEmpty ? '普通用户' : info.role),
        ),
        const Divider(height: 1),
        ListTile(
          leading: const Icon(Icons.lock_outline),
          title: const Text('修改密码'),
          trailing: const Icon(Icons.chevron_right),
          onTap: () => Navigator.push(
            context,
            MaterialPageRoute(
              builder: (_) => ChangePasswordScreen(auth: widget.auth, api: _api),
            ),
          ),
        ),
      ],
    );
  }
}
