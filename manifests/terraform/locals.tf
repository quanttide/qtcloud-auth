locals {
  # 应用级资源命名：<app>-<env>（系统级资源由 quanttide-platform 管理）
  app_name_prefix = "${var.project}-${var.environment}"
}
