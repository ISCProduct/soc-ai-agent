resource "aws_cloudwatch_log_group" "this" {
  name              = "/ecs/${var.project_name}/${var.service_name}"
  retention_in_days = var.log_retention_days

  tags = merge(var.tags, {
    Name = "${var.project_name}-${var.service_name}-logs"
  })
}

resource "aws_iam_role" "execution" {
  name = "${var.project_name}-${var.service_name}-exec"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = var.tags
}

resource "aws_iam_role_policy_attachment" "execution" {
  role       = aws_iam_role.execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role_policy" "execution_secrets" {
  count = length(var.secret_arns) > 0 ? 1 : 0

  name = "${var.project_name}-${var.service_name}-secrets"
  role = aws_iam_role.execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["secretsmanager:GetSecretValue"]
      Resource = var.secret_arns
    }]
  })
}

resource "aws_iam_role" "task" {
  name = "${var.project_name}-${var.service_name}-task"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = var.tags
}

resource "aws_iam_role_policy" "task_s3" {
  count = var.s3_bucket_arn != "" ? 1 : 0

  name = "${var.project_name}-${var.service_name}-s3"
  role = aws_iam_role.task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject",
        "s3:ListBucket"
      ]
      Resource = [
        var.s3_bucket_arn,
        "${var.s3_bucket_arn}/*"
      ]
    }]
  })
}

resource "aws_iam_role_policy" "task_execute_command" {
  count = var.enable_execute_command ? 1 : 0

  name = "${var.project_name}-${var.service_name}-exec-command"
  role = aws_iam_role.task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "ssmmessages:CreateControlChannel",
        "ssmmessages:CreateDataChannel",
        "ssmmessages:OpenControlChannel",
        "ssmmessages:OpenDataChannel",
      ]
      Resource = "*"
    }]
  })
}

resource "aws_iam_role_policy" "task_efs" {
  count = length(var.efs_volumes) > 0 ? 1 : 0

  name = "${var.project_name}-${var.service_name}-efs"
  role = aws_iam_role.task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "elasticfilesystem:ClientMount",
        "elasticfilesystem:ClientWrite",
      ]
      Resource = [for v in var.efs_volumes : v.file_system_arn]
    }]
  })
}

locals {
  env_list = [
    for k, v in var.environment : {
      name  = k
      value = v
    }
  ]
}

resource "aws_ecs_task_definition" "this" {
  family                   = "${var.project_name}-${var.service_name}"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.cpu
  memory                   = var.memory
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  container_definitions = jsonencode(concat([
    merge(
      {
        name      = var.container_name
        image     = var.container_image
        essential = true
        portMappings = [
          {
            containerPort = var.container_port
            protocol      = "tcp"
          }
        ]
        environment = local.env_list
        secrets     = var.secrets
        mountPoints = var.container_mount_points
        logConfiguration = {
          logDriver = "awslogs"
          options = {
            "awslogs-group"         = aws_cloudwatch_log_group.this.name
            "awslogs-region"        = var.region
            "awslogs-stream-prefix" = "ecs"
          }
        }
      },
      var.container_health_check != null ? {
        healthCheck = {
          command     = var.container_health_check.command
          interval    = var.container_health_check.interval
          timeout     = var.container_health_check.timeout
          retries     = var.container_health_check.retries
          startPeriod = var.container_health_check.start_period
        }
      } : {}
    )
  ], var.extra_container_definitions))

  dynamic "volume" {
    for_each = var.efs_volumes
    content {
      name = volume.value.name
      efs_volume_configuration {
        file_system_id     = volume.value.file_system_id
        root_directory     = "/"
        transit_encryption = "ENABLED"
        authorization_config {
          access_point_id = volume.value.access_point_id
          iam             = "ENABLED"
        }
      }
    }
  }

  tags = var.tags
}

resource "aws_ecs_service" "this" {
  name                   = var.service_name
  cluster                = var.cluster_id
  task_definition        = aws_ecs_task_definition.this.arn
  desired_count          = var.desired_count
  launch_type            = "FARGATE"
  enable_execute_command = var.enable_execute_command

  network_configuration {
    subnets          = var.subnet_ids
    security_groups  = [var.security_group_id]
    assign_public_ip = var.assign_public_ip
  }

  dynamic "load_balancer" {
    for_each = var.target_group_arn != "" ? [1] : []
    content {
      target_group_arn = var.target_group_arn
      container_name   = var.container_name
      container_port   = var.container_port
    }
  }

  dynamic "service_registries" {
    for_each = var.service_discovery_registry_arn != "" ? [1] : []
    content {
      registry_arn = var.service_discovery_registry_arn
    }
  }

  # ALBのヘルスチェック猶予・デプロイサーキットブレーカーはALB配下のサービスにのみ設定する
  # 起動から healthz 応答まで実測で数秒（マイグレーションとシード込み）。120秒は過大で、
  # そのままデプロイの待ち時間に乗るため 60秒へ短縮する。
  health_check_grace_period_seconds = var.target_group_arn != "" ? 60 : null
  # 0/100 は「旧タスクを止めてから新タスクを起動する」直列切替で、切替のたびに
  # 停止→起動→ヘルスチェックが積み上がり、かつ切替中は無応答になる。
  # 100/200 にして新タスクが healthy になってから旧タスクを落とす（無停止・短時間）。
  # デプロイ中のみタスクが一時的に2倍になるが、0.25vCPU×数分のため実費はごく僅か。
  deployment_minimum_healthy_percent = 100
  deployment_maximum_percent         = 200

  deployment_circuit_breaker {
    enable   = var.target_group_arn != ""
    rollback = var.target_group_arn != ""
  }

  lifecycle {
    # desired_count: uptimeスケジューラ(prod-uptime-scheduler.yml)が稼働日に応じて更新する。
    # task_definition: デプロイ(deployment.yml)がregister-task-definitionで新リビジョンを
    # 登録しサービスへ適用する。ここでignoreしないと、applyの度に稼働中のリビジョンが
    # tfvarsのimage定義まで巻き戻り、デプロイ済みの変更が本番から消える。
    ignore_changes = [desired_count, task_definition]
  }

  tags = var.tags

  depends_on = [aws_iam_role_policy_attachment.execution]
}
