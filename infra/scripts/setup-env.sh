#!/bin/bash
# EC2 上で Secrets Manager からシークレットを取得し .env を生成するスクリプト
# IAM ロールに secretsmanager:GetSecretValue 権限が必要
#
# 必須環境変数（GitHub Actions Secrets で管理）:
#   DB_SECRET_ARN     - RDS マネージドシークレットの ARN
#   OPENAI_SECRET_ARN - OpenAI API キーシークレットの ARN
set -euo pipefail

REGION="ap-northeast-1"
APP_DIR="/home/ubuntu/soc-app"
ENV_FILE="${APP_DIR}/.env"

# ARN は環境変数から受け取る（ハードコード禁止）
: "${DB_SECRET_ARN:?DB_SECRET_ARN が設定されていません}"
: "${OPENAI_SECRET_ARN:?OPENAI_SECRET_ARN が設定されていません}"

echo "Fetching secrets from Secrets Manager..."

DB_SECRET=$(aws secretsmanager get-secret-value \
  --secret-id "${DB_SECRET_ARN}" \
  --query SecretString \
  --output text \
  --region "${REGION}")

OPENAI_API_KEY=$(aws secretsmanager get-secret-value \
  --secret-id "${OPENAI_SECRET_ARN}" \
  --query SecretString \
  --output text \
  --region "${REGION}")

mkdir -p "${APP_DIR}"

cat > "${ENV_FILE}" <<EOF
DB_SECRET=${DB_SECRET}
OPENAI_API_KEY=${OPENAI_API_KEY}
EOF

chmod 600 "${ENV_FILE}"
echo ".env generated at ${ENV_FILE}"
