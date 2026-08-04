variable "region" {
  description = "阿里云地域"
  type        = string
  default     = "cn-hangzhou"
}

variable "project" {
  description = "项目名（资源命名前缀）"
  type        = string
  default     = "qtcloud-auth"
}

variable "environment" {
  description = "环境：dev / prod"
  type        = string
  default     = "prod"
}

variable "db_name" {
  description = "RDS 数据库名（创建在系统级共享实例上，实例由 quanttide-platform 管理）"
  type        = string
  default     = "qtcloud_auth"
}

variable "db_username" {
  description = "RDS 数据库账号名"
  type        = string
  default     = "qtcloud_auth"
}

variable "db_password" {
  description = "RDS 数据库账号密码（8-32 位，含大小写字母与数字）"
  type        = string
  sensitive   = true
}

variable "image" {
  description = "FC 容器镜像。由 CI 注入（TF_VAR_image 拼接 secret ALIYUN_ACR_REGISTRY 的实例地址）或 terraform.tfvars 提供；实例地址属敏感信息不写默认值"
  type        = string
}

variable "jwt_secret" {
  description = "JWT HS256 签名密钥（provider 环境变量 JWT_SECRET，生产必须显式配置，勿用默认值）"
  type        = string
  sensitive   = true
}

variable "admin_password" {
  description = "种子管理员密码（provider 环境变量 ADMIN_PASSWORD，生产必须显式配置，勿用默认值）"
  type        = string
  sensitive   = true
}

variable "sms_driver" {
  description = "短信发送器（provider 环境变量 SMS_DRIVER；当前实现仅 console，阿里云短信见 docs/dev-guide/index.md 与 data/library/aliyun-sms-integration.md）"
  type        = string
  default     = "console"
}

variable "fc_memory" {
  description = "FC 函数内存（MB）"
  type        = number
  default     = 512
}

variable "fc_timeout" {
  description = "FC 函数超时（秒）"
  type        = number
  default     = 60
}
