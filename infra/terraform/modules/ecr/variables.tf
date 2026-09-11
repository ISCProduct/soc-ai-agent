variable "repository_names" {
  type        = list(string)
  description = "作成するECRリポジトリ名の一覧"
}

variable "force_delete" {
  type        = bool
  description = "destroy時にイメージが残っていても削除するか（stagingはtrue推奨）"
  default     = false
}

variable "lifecycle_keep_count" {
  type        = number
  description = "各リポジトリで保持する最新イメージ数（それ以外は自動失効）"
  default     = 20
}

variable "tags" {
  type    = map(string)
  default = {}
}
