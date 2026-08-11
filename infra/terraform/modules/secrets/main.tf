resource "aws_secretsmanager_secret" "db" {
  name = "${var.project_name}/db"

  tags = merge(var.tags, {
    Name = "${var.project_name}-db-secret"
  })
}

resource "aws_secretsmanager_secret_version" "db" {
  secret_id = aws_secretsmanager_secret.db.id
  secret_string = jsonencode({
    host     = var.db_host
    port     = var.db_port
    dbname   = var.db_name
    username = var.db_username
    password = var.db_password
    engine   = "mysql"
  })
}
