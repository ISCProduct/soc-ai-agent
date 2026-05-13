# OCI Terraform 環境

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

## CI/CD (GitHub Actions)

`.github/workflows/terraform-oci.yml` により自動実行されます。

| イベント | 動作 |
|---|---|
| PR作成・更新 (`infra/terraform/**` 変更時) | `terraform plan` を実行してプランを確認 |
| `main` へのマージ | `terraform apply` を実行してインフラを更新 |

### 必要な GitHub Secrets

| Secret 名 | 内容 |
|---|---|
| `OCI_TENANCY_OCID` | テナンシーの OCID |
| `OCI_USER_OCID` | ユーザーの OCID |
| `OCI_FINGERPRINT` | API キーのフィンガープリント |
| `OCI_PRIVATE_KEY` | API 秘密鍵の内容 (PEM全体) |
| `OCI_REGION` | リージョン (例: `ap-tokyo-1`) |
| `OCI_COMPARTMENT_ID` | コンパートメントの OCID (空でもOK) |
| `OCI_AVAILABILITY_DOMAIN` | 可用性ドメイン名 |
| `OCI_SSH_PUBLIC_KEY` | Compute Instance の SSH 公開鍵 |
| `OCI_IMAGE_ID` | イメージの OCID |
| `OCI_DB_ADMIN_PASSWORD` | MySQL 管理者パスワード |
| `OCI_STORAGE_NAMESPACE` | Object Storage ネームスペース |

## セキュリティ注意事項

- `terraform.tfvars` は `.gitignore` で除外済み。**絶対にコミットしないこと**
- `*.tfstate` も除外済み。ステートファイルはローカルまたは Object Storage バックエンドで管理すること
- MySQL はプライベートサブネットに配置し、VCN 内部からのみアクセス可能
