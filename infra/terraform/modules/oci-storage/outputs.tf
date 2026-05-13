output "app_bucket_name" {
  value = oci_objectstorage_bucket.app.name
}

output "uploads_bucket_name" {
  value = oci_objectstorage_bucket.uploads.name
}
