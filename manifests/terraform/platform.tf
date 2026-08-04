# 系统级资源引用：由 quanttide-platform 仓库管理（VPC / 交换机 / 安全组 / RDS 实例）
data "terraform_remote_state" "platform" {
  backend = "oss"
  config = {
    bucket = "quanttide-terraform-state"
    key    = "quanttide-platform/terraform.tfstate"
    region = "cn-hangzhou"
  }
}
