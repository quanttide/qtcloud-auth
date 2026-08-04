# 应用数据库：创建在系统级共享 RDS 实例上（实例本身由 quanttide-platform 管理）
resource "alicloud_db_database" "this" {
  instance_id    = data.terraform_remote_state.platform.outputs.rds_instance_id
  data_base_name = var.db_name
  character_set  = "UTF8"
}

resource "alicloud_db_account" "this" {
  db_instance_id   = data.terraform_remote_state.platform.outputs.rds_instance_id
  account_name     = var.db_username
  account_password = var.db_password
  account_type     = "Normal"
}

resource "alicloud_db_account_privilege" "this" {
  instance_id  = data.terraform_remote_state.platform.outputs.rds_instance_id
  account_name = alicloud_db_account.this.account_name
  # RDS PostgreSQL 仅支持 DBOwner（数据库所有者），ReadWrite 为 MySQL 专有
  privilege = "DBOwner"
  db_names  = [alicloud_db_database.this.data_base_name]
}
