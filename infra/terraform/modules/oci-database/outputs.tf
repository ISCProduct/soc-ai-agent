output "db_system_id" {
  value = oci_mysql_mysql_db_system.main.id
}

output "db_endpoint" {
  value = oci_mysql_mysql_db_system.main.endpoints[0].hostname
}
