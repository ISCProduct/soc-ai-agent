output "state_bucket_name" {
  description = "S3 bucket for Terraform state"
  value       = aws_s3_bucket.tfstate.id
}

output "backend_hcl_example" {
  description = "Paste into environments/staging/backend.hcl (or versions.tf backend block)"
  value       = <<-EOT
    bucket       = "${aws_s3_bucket.tfstate.id}"
    key          = "staging/terraform.tfstate"
    region       = "${var.region}"
    use_lockfile = true
    encrypt      = true
  EOT
}

output "bootstrap_apply_hint" {
  value = "Next: cd ../environments/staging && cp backend.hcl.example backend.hcl && edit bucket, then terraform init -backend-config=backend.hcl"
}
