resource "oci_objectstorage_bucket" "app" {
  compartment_id = var.compartment_id
  namespace      = var.namespace
  name           = "${var.project_name}-storage"
  access_type    = "NoPublicAccess"

  freeform_tags = {
    Project = var.project_name
  }
}

# 履歴書・添付ファイル用バケット
resource "oci_objectstorage_bucket" "uploads" {
  compartment_id = var.compartment_id
  namespace      = var.namespace
  name           = "${var.project_name}-uploads"
  access_type    = "NoPublicAccess"

  versioning = "Enabled"

  freeform_tags = {
    Project = var.project_name
  }
}
