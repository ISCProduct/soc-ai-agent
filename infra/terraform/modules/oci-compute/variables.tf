variable "project_name" {
  type = string
}

variable "compartment_id" {
  type        = string
  description = "OCI コンパートメントの OCID"
}

variable "availability_domain" {
  type        = string
  description = "可用性ドメイン名 (例: Uocm:AP-TOKYO-1-AD-1)"
}

variable "subnet_id" {
  type        = string
  description = "パブリックサブネットの OCID"
}

variable "shape" {
  type        = string
  default     = "VM.Standard.A1.Flex"
  description = "Always Free対応シェイプ"
}

variable "ocpus" {
  type    = number
  default = 1
}

variable "memory_in_gbs" {
  type    = number
  default = 6
}

variable "fault_domain" {
  type        = string
  default     = "FAULT-DOMAIN-1"
  description = "フォルト・ドメイン (容量不足時は FAULT-DOMAIN-2 / FAULT-DOMAIN-3 に変更)"
}

variable "ssh_authorized_keys" {
  type        = string
  description = "SSH 公開鍵"
}

variable "image_id" {
  type        = string
  description = "OCIコンピュートイメージのOCID (Oracle Linux 8 or Ubuntu 22.04)"
}
