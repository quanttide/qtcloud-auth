// 冒烟测试：AuthGate 依据本地 token 在登录页/主界面间切换。
// 通过注入内存替身 AuthService（不发网络请求）测试主流程。

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:qtcloud_auth_lite/main.dart';
import 'package:qtcloud_auth_lite/screens/login_screen.dart';
import 'package:qtcloud_auth_lite/services/auth_service.dart';

/// 内存替身：token 存于字段，不发网络请求。
class _FakeAuth extends AuthService {
  _FakeAuth({this.token});

  String? token;
  bool logoutCalled = false;

  @override
  Future<String?> storedToken() async => token;

  @override
  Future<String> login({required String username, required String password}) async {
    token = 'fake-token';
    return token!;
  }

  @override
  Future<void> logout() async {
    token = null;
    logoutCalled = true;
  }
}

void main() {
  testWidgets('无 token 时显示登录页', (WidgetTester tester) async {
    await tester.pumpWidget(MainApp(auth: _FakeAuth()));
    await tester.pump();

    expect(find.text('量潮账号中心'), findsOneWidget);
    expect(find.byType(LoginScreen), findsOneWidget);
    expect(find.widgetWithText(FilledButton, '登录'), findsOneWidget);
  });

  testWidgets('有 token 时直接显示账号信息页', (WidgetTester tester) async {
    await tester.pumpWidget(MainApp(auth: _FakeAuth(token: 'saved-token')));
    await tester.pumpAndSettle();

    expect(find.byType(LoginScreen), findsNothing);
    expect(find.text('账号信息'), findsOneWidget);
  });

  testWidgets('登录成功后切换到主界面', (WidgetTester tester) async {
    final auth = _FakeAuth();
    await tester.pumpWidget(MainApp(auth: auth));
    await tester.pump();

    await tester.enterText(find.byType(TextField).first, 'alice');
    await tester.enterText(find.byType(TextField).last, 'secret');
    await tester.tap(find.widgetWithText(FilledButton, '登录'));
    await tester.pumpAndSettle();

    expect(auth.token, 'fake-token');
    expect(find.byType(LoginScreen), findsNothing);
    expect(find.text('账号信息'), findsOneWidget);
  });

  testWidgets('登出后返回登录页', (WidgetTester tester) async {
    final auth = _FakeAuth(token: 'saved-token');
    await tester.pumpWidget(MainApp(auth: auth));
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('退出登录'));
    await tester.pumpAndSettle();

    expect(auth.logoutCalled, isTrue);
    expect(find.byType(LoginScreen), findsOneWidget);
  });
}
