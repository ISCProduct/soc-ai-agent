resource "aws_cloudwatch_log_group" "this" {
  name              = "/ecs/${var.project_name}/${var.service_name}"
  retention_in_days = var.log_retention_days

  # ponytail: タグ付けにはlogs:TagResourceが別途必要になり、IAM権限が限られた
  # 環境ではCreateLogGroup自体が失敗する。タグ無しでの作成に留める。
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
  count = var.create_secrets_policy ? 1 : 0

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
  count = var.create_s3_policy ? 1 : 0

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
  requires_compatibilities = ["EC2"]
  network_mode             = "bridge"
  cpu                      = var.cpu
  memory                   = var.memory
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  container_definitions = jsonencode([
    {
      name      = var.container_name
      image     = var.container_image
      essential = true
      cpu       = var.cpu
      memory    = var.memory
      portMappings = [
        {
          containerPort = var.container_port
          hostPort      = var.target_group_arn != "" ? 0 : var.host_port
          protocol      = "tcp"
        }
      ]
      environment = local.env_list
      secrets     = var.secrets
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.this.name
          "awslogs-region"        = var.region
          "awslogs-stream-prefix" = "ecs"
        }
      }
    }
  ])

  tags = var.tags
}

resource "aws_ecs_service" "this" {
  name            = var.service_name
  cluster         = var.cluster_id
  task_definition = aws_ecs_task_definition.this.arn
  desired_count   = var.desired_count
  launch_type     = null

  capacity_provider_strategy {
    capacity_provider = var.capacity_provider_name
    weight            = 1
    base              = 1
  }

  deployment_minimum_healthy_percent = 0
  deployment_maximum_percent         = 100

  health_check_grace_period_seconds = var.target_group_arn != "" ? 60 : null

  dynamic "load_balancer" {
    for_each = var.target_group_arn != "" ? [1] : []
    content {
      target_group_arn = var.target_group_arn
      container_name   = var.container_name
      container_port   = var.container_port
    }
  }

  # hostPort 固定のため同一ホストで複数起動しない（ALB利用時は動的ポートのためbinpackは主にメモリ効率目的）
  ordered_placement_strategy {
    type  = "binpack"
    field = "memory"
  }

  lifecycle {
    ignore_changes = [desired_count]
  }

  tags = var.tags

  depends_on = [aws_iam_role_policy_attachment.execution]
}
