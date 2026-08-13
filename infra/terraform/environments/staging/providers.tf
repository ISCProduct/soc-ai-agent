provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project   = "soc-ai-agent"
      Env       = "staging"
      ManagedBy = "terraform"
    }
  }
}

# CloudFront 用 ACM（us-east-1 必須）
provider "aws" {
  alias  = "us_east_1"
  region = "us-east-1"

  default_tags {
    tags = {
      Project   = "soc-ai-agent"
      Env       = "staging"
      ManagedBy = "terraform"
    }
  }
}
