provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project   = "soc-ai-agent"
      Env       = "production"
      ManagedBy = "terraform"
    }
  }
}
