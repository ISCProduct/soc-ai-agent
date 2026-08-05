# NAT Gateway なしの public VPC（staging コスト優先）

resource "aws_vpc" "this" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = merge(var.tags, {
    Name = "${var.project_name}-vpc"
  })
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id

  tags = merge(var.tags, {
    Name = "${var.project_name}-igw"
  })
}

resource "aws_subnet" "public" {
  count = length(var.public_subnet_cidrs)

  vpc_id                  = aws_vpc.this.id
  cidr_block              = var.public_subnet_cidrs[count.index]
  availability_zone       = var.azs[count.index]
  map_public_ip_on_launch = true

  tags = merge(var.tags, {
    Name = "${var.project_name}-public-${var.azs[count.index]}"
    Tier = "public"
  })
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.this.id
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-public-rt"
  })
}

resource "aws_route_table_association" "public" {
  count = length(aws_subnet.public)

  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

resource "aws_security_group" "ecs" {
  name        = "${var.project_name}-ecs-sg"
  description = "ECS EC2 hosts"
  vpc_id      = aws_vpc.this.id

  # ALB を使う場合はALB経由のみとし、直接ポート公開はしない（二重露出防止）
  dynamic "ingress" {
    for_each = var.enable_alb ? [] : [80]
    content {
      description = "HTTP"
      from_port   = ingress.value
      to_port     = ingress.value
      protocol    = "tcp"
      cidr_blocks = var.allowed_http_cidrs
    }
  }

  dynamic "ingress" {
    for_each = var.enable_alb ? [] : [443]
    content {
      description = "HTTPS"
      from_port   = ingress.value
      to_port     = ingress.value
      protocol    = "tcp"
      cidr_blocks = var.allowed_http_cidrs
    }
  }

  dynamic "ingress" {
    for_each = var.enable_alb ? [] : [3000]
    content {
      description = "Frontend"
      from_port   = ingress.value
      to_port     = ingress.value
      protocol    = "tcp"
      cidr_blocks = var.allowed_http_cidrs
    }
  }

  dynamic "ingress" {
    for_each = var.enable_alb ? [] : [8080]
    content {
      description = "Backend"
      from_port   = ingress.value
      to_port     = ingress.value
      protocol    = "tcp"
      cidr_blocks = var.allowed_http_cidrs
    }
  }

  dynamic "ingress" {
    for_each = var.enable_ssh ? [1] : []
    content {
      description = "SSH"
      from_port   = 22
      to_port     = 22
      protocol    = "tcp"
      cidr_blocks = var.allowed_ssh_cidrs
    }
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-ecs-sg"
  })
}

resource "aws_security_group" "rds" {
  name        = "${var.project_name}-rds-sg"
  description = "RDS MySQL - ECS hosts only"
  vpc_id      = aws_vpc.this.id

  ingress {
    description     = "MySQL from ECS"
    from_port       = 3306
    to_port         = 3306
    protocol        = "tcp"
    security_groups = [aws_security_group.ecs.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-rds-sg"
  })
}

# --- ALB + Fargate 向け（enable_alb=true のときだけ作成。EC2/ALBなしのstagingには影響しない） ---

resource "aws_security_group" "alb" {
  count = var.enable_alb ? 1 : 0

  name        = "${var.project_name}-alb-sg"
  description = "ALB (public 80/443)"
  vpc_id      = aws_vpc.this.id

  ingress {
    description = "HTTP"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = var.alb_ingress_cidrs
  }

  ingress {
    description = "HTTPS"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = var.alb_ingress_cidrs
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-alb-sg"
  })
}

resource "aws_security_group" "fargate" {
  count = var.enable_alb ? 1 : 0

  name        = "${var.project_name}-fargate-sg"
  description = "Fargate tasks - ALB only"
  vpc_id      = aws_vpc.this.id

  dynamic "ingress" {
    for_each = var.fargate_container_ports
    content {
      description     = "From ALB"
      from_port       = ingress.value
      to_port         = ingress.value
      protocol        = "tcp"
      security_groups = [aws_security_group.alb[0].id]
    }
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-fargate-sg"
  })
}

resource "aws_security_group_rule" "ecs_dynamic_ports_from_alb" {
  count = var.enable_alb ? 1 : 0

  type                     = "ingress"
  description              = "ALB to ECS on EC2 dynamic host port mapping"
  from_port                = 32768
  to_port                  = 65535
  protocol                 = "tcp"
  security_group_id        = aws_security_group.ecs.id
  source_security_group_id = aws_security_group.alb[0].id
}

resource "aws_security_group_rule" "rds_from_fargate" {
  count = var.enable_alb ? 1 : 0

  type                     = "ingress"
  description              = "MySQL from Fargate tasks"
  from_port                = 3306
  to_port                  = 3306
  protocol                 = "tcp"
  security_group_id        = aws_security_group.rds.id
  source_security_group_id = aws_security_group.fargate[0].id
}
