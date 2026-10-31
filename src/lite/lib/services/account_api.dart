// 账号中心 API：账号信息 + 修改密码（复用 qtcloud-auth）。

import 'dart:convert';

import 'package:http/http.dart' as http;

class AccountApiException implements Exception {
  const AccountApiException(this.message);
  final String message;
  @override
  String toString() => message;
}

class UserInfo {
  const UserInfo({
    required this.sub,
    required this.username,
    required this.email,
    required this.phone,
    required this.nickname,
    this.role = '',
  });

  final String sub;
  final String username;
  final String email;
  final String phone;
  final String nickname;

  /// 角色 ID（空=普通用户；admin=管理员）。
  final String role;

  factory UserInfo.fromJson(Map<String, dynamic> json) => UserInfo(
    sub: json['sub'] as String? ?? '',
    username: json['username'] as String? ?? '',
    email: json['email'] as String? ?? '',
    phone: json['phone'] as String? ?? '',
    nickname: json['nickname'] as String? ?? '',
    role: json['role'] as String? ?? '',
  );
}

class AccountApi {
  AccountApi({http.Client? client, required this.baseUrl, required this.token})
    : _client = client ?? http.Client();

  /// 默认账号服务地址（与 AuthService 同网关路径）。
  static String defaultBaseUrl() {
    const String fromEnv = String.fromEnvironment('QTCLOUD_AUTH_BASE_URL');
    if (fromEnv.isNotEmpty) {
      return fromEnv;
    }
    return 'http://localhost:8080';
  }

  final http.Client _client;
  final String baseUrl;
  final String token;

  Map<String, String> get _headers => {
    'Authorization': 'Bearer $token',
    'Content-Type': 'application/json',
  };

  Future<UserInfo> fetchUserInfo() async {
    final resp = await _client
        .get(Uri.parse('$baseUrl/userinfo'), headers: _headers)
        .timeout(const Duration(seconds: 15));
    if (resp.statusCode != 200) {
      throw AccountApiException('获取账号信息失败（HTTP ${resp.statusCode}）');
    }
    return UserInfo.fromJson(jsonDecode(utf8.decode(resp.bodyBytes)) as Map<String, dynamic>);
  }

  Future<void> updateNickname(String nickname) async {
    // 用 POST 而非 PATCH：传统版 API 网关不支持 PATCH 方法（网关实测报错）
    final resp = await _client
        .post(
          Uri.parse('$baseUrl/userinfo'),
          headers: _headers,
          body: jsonEncode({'nickname': nickname}),
        )
        .timeout(const Duration(seconds: 15));
    if (resp.statusCode != 200) {
      throw AccountApiException(_error(resp));
    }
  }

  Future<void> changePassword({required String oldPassword, required String newPassword}) async {
    final resp = await _client
        .post(
          Uri.parse('$baseUrl/oauth/password/change'),
          headers: _headers,
          body: jsonEncode({
            'old_password': oldPassword,
            'new_password': newPassword,
          }),
        )
        .timeout(const Duration(seconds: 15));
    if (resp.statusCode != 200) {
      throw AccountApiException(_error(resp));
    }
  }

  String _error(http.Response resp) {
    try {
      final body = jsonDecode(utf8.decode(resp.bodyBytes)) as Map<String, dynamic>;
      return body['error_description'] as String? ?? body['error'] as String? ?? 'HTTP ${resp.statusCode}';
    } on FormatException {
      return 'HTTP ${resp.statusCode}';
    }
  }
}
