#!/bin/bash
# EC2 上で Secrets Manager からシークレットを取得し .env を生成するスクリプト
# IAM ロールに secretsmanager:GetSecretValue 権限が必要
set -euo pipefail

REGION="ap-northeast-1"
APP_DIR="/home/ubuntu/soc-app"
ENV_FILE="${APP_DIR}/.env"

DB_SECRET_ARN="arn:aws:secretsmanager:${REGION}:970835573274:secret:rds!db-3abb75b9-c579-47de-a978-d4f6e56dd4ba-kXjKwE"
OPENAI_SECRET_ARN="arn:aws:secretsmanager:${REGION}:970835573274:secret:prod/openai/api-key-ctv4bB"

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
