// 量潮身份云管理后台入口：企业管理账号信息的后台系统。
// API 基址经 --dart-define 注入（QTCLOUD_AUTH_BASE_URL，本地联调 = localhost）。

import 'package:flutter/material.dart';

void main() {
  runApp(const MainApp());
}

class MainApp extends StatelessWidget {
  const MainApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: '量潮身份云管理后台',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: Colors.deepPurple),
      ),
      home: const Scaffold(
        body: Center(child: Text('量潮身份云管理后台 · 建设中')),
      ),
    );
  }
}
