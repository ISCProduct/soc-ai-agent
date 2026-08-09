# Terraform bootstrap（AWS remote state）

staging / 将来の prod 共通の **S3（stateバケット、ロックはS3ネイティブロック）** を作成する。

```bash
terraform init
terraform apply
terraform output
```

出力の `backend_hcl_example` を `environments/staging/backend.hcl` に反映する。

このディレクトリ自体はローカル state のままでよい（state 用バケットの state を S3 に置く循環を避けるため）。
