# OCI Terraform 環境

> **方針 (2026-08-05):** アプリケーション **staging の正は AWS**（`../staging`）です。  
> 本 OCI 環境は **現行では使いません**（アーカイブ候補）。  
> 詳細: [`docs/architecture/infra-decision-oci-stg-aws-prod.md`](../../../docs/architecture/infra-decision-oci-stg-aws-prod.md)

Oracle Cloud Infrastructure (OCI) へのインフラデプロイ用 Terraform 設定。

## 構成リソース

| リソース | 内容 |
|---|---|
| VCN | `10.0.0.0/16` |
| パブリックサブネット | `10.0.1.0/24` (Compute Instance) |
| プライベートサブネット | `10.0.2.0/24` (MySQL) |
| Compute Instance | VM.Standard.A1.Flex (Always Free: 1 OCPU / 6GB) |
| MySQL Database Service | MySQL.Free (Always Free) |
| Object Storage | `soc-app-storage`, `soc-app-uploads` |

## 初回セットアップ

### 1. OCI API キーの設定

```bash
mkdir -p ~/.oci
# OCI コンソール > プロファイル > APIキー > APIキーの追加 でキーペアを生成
# 秘密鍵を ~/.oci/oci_api_key.pem に配置し、パーミッションを設定
chmod 600 ~/.oci/oci_api_key.pem
```

### 2. terraform.tfvars の作成

```bash
cp terraform.tfvars.example terraform.tfvars
# terraform.tfvars を編集して実際の値を入力
```

必要な情報の取得場所:

- `tenancy_ocid`: OCI コンソール右上 > プロファイル > テナンシー
- `user_ocid`: OCI コンソール右上 > プロファイル > ユーザー設定
- `fingerprint`: OCI コンソール > APIキー一覧で確認
- `availability_domain`: コンソール > コンピュート > インスタンスの作成 > 可用性ドメイン
- `image_id`: コンソール > コンピュート > イメージ > プラットフォーム・イメージ (Ubuntu 22.04 ARM64)
- `storage_namespace`: コンソール > Object Storage > バケット > ネームスペース

### 3. Terraform の実行

```bash
terraform init
terraform plan
terraform apply
```

## CI/CD

現行運用では OCI を使わないため、GitHub Actions（旧 `terraform-oci.yml`）は削除済み。
必要になった場合はローカルで `terraform plan` / `apply` する。

## セキュリティ注意事項

- `terraform.tfvars` は `.gitignore` で除外済み。**絶対にコミットしないこと**
- `*.tfstate` も除外済み。ステートファイルはローカルまたは Object Storage バックエンドで管理すること
- MySQL はプライベートサブネットに配置し、VCN 内部からのみアクセス可能
