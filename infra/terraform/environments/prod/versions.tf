terraform {
  # backend "s3" の use_lockfile（S3ネイティブロック、DynamoDB不要）に 1.10+ が必要
  required_version = ">= 1.10.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }

  # bootstrap apply 後に有効化:
  #   cp backend.hcl.example backend.hcl  # bucket を記入
  #   terraform init -backend-config=backend.hcl
  #
  # 初回検証でローカル state のまま使う場合は、下の backend "s3" ブロックをコメントアウトしたまま init。
  # backend "s3" {
  #   # configured via -backend-config=backend.hcl
  # }
}
