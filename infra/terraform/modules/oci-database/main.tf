resource "oci_mysql_mysql_db_system" "main" {
  compartment_id      = var.compartment_id
  availability_domain = var.availability_domain
  display_name        = "${var.project_name}-mysql"
  shape_name          = var.shape_name

  subnet_id = var.subnet_id

  admin_username = "admin"
  admin_password = var.db_admin_password

  data_storage_size_in_gb = 50

  # Always Free DBシステムはbackup_policy・mysql_version指定不可
  freeform_tags = {
    Project = var.project_name
  }
}
