// 认证服务：qtcloud-auth 账号登录（账号系统统一入口）。
// 登录成功后 JWT 存本地（shared_preferences，Web=localStorage）。

import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';

/// 登录异常：携带状态码与服务端错误信息。
class AuthException implements Exception {
  const AuthException(this.message);

  final String message;

  @override
  String toString() => 'AuthException: $message';
}

class AuthService {
  AuthService({http.Client? client, String? baseUrl})
    : _client = client ?? http.Client(),
      baseUrl = baseUrl ?? defaultBaseUrl();

  final http.Client _client;

  /// 认证服务地址（qtcloud-auth，经系统级 API 网关统一入口）。
  final String baseUrl;

  static const String _tokenKey = 'qtcloud_account_jwt';

  /// 默认认证服务地址：
  /// - `--dart-define=QTCLOUD_AUTH_BASE_URL=...` 显式指定（部署注入生产网关）
  /// - 其余默认 localhost（本地联调）
  static String defaultBaseUrl() {
    const String fromEnv = String.fromEnvironment('QTCLOUD_AUTH_BASE_URL');
    if (fromEnv.isNotEmpty) {
      return fromEnv;
    }
    return 'http://localhost:8080';
  }

  /// 已登录 token（本地存储）。
  Future<String?> storedToken() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_tokenKey);
  }

  /// 账号密码登录（qtcloud-auth /oauth/token，grant_type=password）。
  Future<String> login({required String username, required String password}) async {
    final http.Response response = await _client
        .post(
          Uri.parse('$baseUrl/oauth/token'),
          headers: const {'Content-Type': 'application/x-www-form-urlencoded'},
          body: {
            'grant_type': 'password',
            'username': username,
            'password': password,
          },
        )
        .timeout(const Duration(seconds: 15));
    if (response.statusCode != 200) {
      throw AuthException(_errorMessage(response));
    }
    final Map<String, dynamic> body = jsonDecode(utf8.decode(response.bodyBytes)) as Map<String, dynamic>;
    final String token = body['access_token'] as String;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_tokenKey, token);
    return token;
  }

  /// 登出（清除本地 token）。
  Future<void> logout() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_tokenKey);
  }

  String _errorMessage(http.Response response) {
    try {
      final Map<String, dynamic> body = jsonDecode(utf8.decode(response.bodyBytes)) as Map<String, dynamic>;
      return body['error_description'] as String? ?? body['error'] as String? ?? 'HTTP ${response.statusCode}';
    } on FormatException {
      return 'HTTP ${response.statusCode}';
    }
  }
}
