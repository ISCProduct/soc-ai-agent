variable "project_name" {
  type = string
}

variable "compartment_id" {
  type        = string
  description = "OCI コンパートメントの OCID"
}

variable "namespace" {
  type        = string
  description = "Object Storage ネームスペース (テナンシー名)"
}
