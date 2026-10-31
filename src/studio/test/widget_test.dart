// studio 冒烟：应用可启动且展示后台占位页。

import 'package:flutter_test/flutter_test.dart';

import 'package:qtcloud_auth_studio/main.dart';

void main() {
  testWidgets('启动展示管理后台占位页', (WidgetTester tester) async {
    await tester.pumpWidget(const MainApp());

    expect(find.text('量潮身份云管理后台 · 建设中'), findsOneWidget);
  });
}
