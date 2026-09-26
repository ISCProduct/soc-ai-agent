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
  description = "各リポジトリで保持する最新の**タグ付き**イメージ数（それ以外は自動失効）"
  default     = 20
}

variable "lifecycle_untagged_keep_count" {
  type        = number
  description = <<-EOT
    保持するタグ無しイメージ数。タグ無しには「タグ付き image index の子manifest」も
    含まれるため、少なすぎるとタグは残っているのに pull できないイメージができる。
    1ビルドで増えるタグ無しは（--provenance=false 後は）1〜2件なので、直近数十ビルド分を残す。
  EOT
  default     = 60
}

variable "tags" {
  type    = map(string)
  default = {}
}
