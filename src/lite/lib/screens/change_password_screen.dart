// 修改密码页：旧密码验证 + 新密码（qtcloud-auth /oauth/password/change）。

import 'package:flutter/material.dart';

import '../services/account_api.dart';
import '../services/auth_service.dart';

class ChangePasswordScreen extends StatefulWidget {
  const ChangePasswordScreen({super.key, required this.auth, required this.api});

  final AuthService auth;
  final AccountApi api;

  @override
  State<ChangePasswordScreen> createState() => _ChangePasswordScreenState();
}

class _ChangePasswordScreenState extends State<ChangePasswordScreen> {
  final _oldPassword = TextEditingController();
  final _newPassword = TextEditingController();
  final _confirmPassword = TextEditingController();
  bool _loading = false;
  String? _error;

  Future<void> _submit() async {
    if (_newPassword.text.length < 6) {
      setState(() => _error = '新密码至少 6 位');
      return;
    }
    if (_newPassword.text != _confirmPassword.text) {
      setState(() => _error = '两次输入的新密码不一致');
      return;
    }
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      await widget.api.changePassword(
        oldPassword: _oldPassword.text,
        newPassword: _newPassword.text,
      );
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('密码修改成功')));
        Navigator.pop(context);
      }
    } catch (_) {
      if (mounted) {
        setState(() => _error = '修改失败，请检查原密码是否正确');
      }
    } finally {
      if (mounted) {
        setState(() => _loading = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('修改密码')),
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 360),
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TextField(
                  controller: _oldPassword,
                  obscureText: true,
                  decoration: const InputDecoration(
                    labelText: '原密码',
                    border: OutlineInputBorder(),
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _newPassword,
                  obscureText: true,
                  decoration: const InputDecoration(
                    labelText: '新密码（至少 6 位）',
                    border: OutlineInputBorder(),
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _confirmPassword,
                  obscureText: true,
                  decoration: const InputDecoration(
                    labelText: '确认新密码',
                    border: OutlineInputBorder(),
                  ),
                ),
                if (_error != null) ...[
                  const SizedBox(height: 12),
                  Text(_error!, style: const TextStyle(color: Colors.red)),
                ],
                const SizedBox(height: 20),
                FilledButton(
                  onPressed: _loading ? null : _submit,
                  child: _loading
                      ? const SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Text('确认修改'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
