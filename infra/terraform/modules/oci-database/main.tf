resource "oci_mysql_mysql_db_system" "main" {
  compartment_id      = var.compartment_id
  availability_domain = var.availability_domain
  display_name        = "${var.project_name}-mysql"
  shape_name          = var.shape_name

  subnet_id = var.subnet_id

  admin_username = "admin"
  admin_password = var.db_admin_password

  mysql_version = var.mysql_version

  data_storage_size_in_gb = 50

  backup_policy {
    is_enabled        = true
    retention_in_days = 7
    window_start_time = "02:00"
  }

  freeform_tags = {
    Project = var.project_name
  }
}
