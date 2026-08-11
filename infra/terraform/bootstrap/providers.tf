provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project   = "soc-ai-agent"
      ManagedBy = "terraform"
      Component = "tfstate-bootstrap"
    }
  }
}
