# OCI 認証情報 (terraform.tfvars で指定 / GitHub Secrets で管理)
variable "tenancy_ocid" {
  type        = string
  description = "テナンシーの OCID"
}

variable "user_ocid" {
  type        = string
  description = "API キーを持つ IAM ユーザーの OCID"
}

variable "fingerprint" {
  type        = string
  description = "API キーのフィンガープリント"
}

variable "private_key_path" {
  type        = string
  description = "API 秘密鍵のパス"
  default     = "~/.oci/oci_api_key.pem"
}

variable "private_key_content" {
  type        = string
  sensitive   = true
  description = "API 秘密鍵の内容 (CI/CD環境用, private_key_path の代替)"
  default     = ""
}

variable "region" {
  type        = string
  default     = "ap-tokyo-1"
  description = "OCI リージョン"
}

variable "compartment_id" {
  type        = string
  description = "リソースを配置するコンパートメントの OCID (未指定時はルートコンパートメント=tenancy_ocid)"
  default     = ""
}

# アプリケーション設定
variable "project_name" {
  type    = string
  default = "soc-app"
}

variable "availability_domain" {
  type        = string
  description = "可用性ドメイン名 (例: Uocm:AP-TOKYO-1-AD-1)"
}

variable "fault_domain" {
  type    = string
  default = "FAULT-DOMAIN-1"
}

variable "ssh_authorized_keys" {
  type        = string
  description = "Compute Instance に登録する SSH 公開鍵"
}

variable "image_id" {
  type        = string
  description = "Compute Instance のイメージ OCID (Oracle Linux 8 または Ubuntu 22.04 ARM)"
}

# ネットワーク設定
variable "vcn_cidr" {
  type    = string
  default = "10.0.0.0/16"
}

variable "public_subnet_cidr" {
  type    = string
  default = "10.0.1.0/24"
}

variable "private_subnet_cidr" {
  type    = string
  default = "10.0.2.0/24"
}
