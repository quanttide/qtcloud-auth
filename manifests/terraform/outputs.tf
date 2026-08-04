output "rds_connection_string" {
  description = "系统级共享 RDS 内网连接地址（由 quanttide-platform 管理，已注入 FC 环境变量）"
  value       = data.terraform_remote_state.platform.outputs.rds_connection_string
}

output "fc_function_name" {
  description = "函数计算函数名"
  value       = alicloud_fcv3_function.this.function_name
}

output "fc_http_url" {
  description = "FC HTTP 触发器公网地址（系统级 API 网关接入前的直连入口）"
  value       = try(alicloud_fcv3_trigger.http.http_trigger[0].url_internet, "尚未创建")
}
