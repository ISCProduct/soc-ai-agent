#!/bin/bash
# OCI Compute Instance へのデプロイスクリプト
# 使用方法: ./infra/scripts/deploy-oci.sh <SERVER_IP>
set -euo pipefail

SERVER_IP="${1:-}"
SSH_KEY="${2:-~/.ssh/id_ed25519}"
REMOTE_USER="ubuntu"
APP_DIR="/opt/soc-app"
DOMAIN="soc-ai-agent.kazuyukitech.com"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

if [ -z "$SERVER_IP" ]; then
  echo "使用方法: $0 <SERVER_IP> [SSH_KEY_PATH]"
  echo "例: $0 1.2.3.4"
  exit 1
fi

echo "==> $SERVER_IP へデプロイ開始"

if [ ! -f "$REPO_ROOT/.env" ]; then
  echo "エラー: .env ファイルが見つかりません ($REPO_ROOT/.env)"
  echo "  .env.oci.example をコピーして .env を作成してください"
  exit 1
fi

SSH_OPTS="-i $SSH_KEY -o StrictHostKeyChecking=no"

# Docker 起動確認（cloud-init 完了待ち）
echo "==> Docker 起動確認中..."
for i in $(seq 1 18); do
  if ssh $SSH_OPTS "$REMOTE_USER@$SERVER_IP" "docker info > /dev/null 2>&1"; then
    echo "==> Docker 起動済み"
    break
  fi
  echo "    待機中... ($i/18)"
  sleep 10
done

# AWS ECR ログイン
echo "==> ECR ログイン..."
AWS_TOKEN=$(aws ecr get-login-password --region ap-northeast-1 2>/dev/null || echo "")
if [ -n "$AWS_TOKEN" ]; then
  ssh $SSH_OPTS "$REMOTE_USER@$SERVER_IP" \
    "echo '$AWS_TOKEN' | docker login --username AWS --password-stdin 970835573274.dkr.ecr.ap-northeast-1.amazonaws.com"
fi

# ファイル転送
echo "==> ファイル転送中..."
ssh $SSH_OPTS "$REMOTE_USER@$SERVER_IP" "mkdir -p $APP_DIR/infra/nginx"
scp $SSH_OPTS "$REPO_ROOT/compose.oci.yml"        "$REMOTE_USER@$SERVER_IP:$APP_DIR/docker-compose.yml"
scp $SSH_OPTS "$REPO_ROOT/.env"                   "$REMOTE_USER@$SERVER_IP:$APP_DIR/.env"
scp $SSH_OPTS "$REPO_ROOT/infra/nginx/nginx.conf" "$REMOTE_USER@$SERVER_IP:$APP_DIR/infra/nginx/nginx.conf"

# Step1: HTTP のみで起動（certbot 証明書取得のため）
echo "==> Step1: HTTP で初回起動..."
ssh $SSH_OPTS "$REMOTE_USER@$SERVER_IP" "cd $APP_DIR && docker compose up -d mysql backend frontend rag-review company-graph"

# Let's Encrypt 証明書取得
echo "==> SSL証明書取得中 ($DOMAIN)..."
ssh $SSH_OPTS "$REMOTE_USER@$SERVER_IP" "
  docker run --rm \
    -v $APP_DIR/infra/certbot/www:/var/www/certbot \
    -v $APP_DIR/infra/certbot/conf:/etc/letsencrypt \
    -p 80:80 \
    certbot/certbot certonly \
      --standalone \
      --non-interactive \
      --agree-tos \
      --email oohashi.0428kazuyuki@gmail.com \
      -d $DOMAIN \
      -d api.$DOMAIN \
    2>&1 || echo '証明書取得済みまたはスキップ'
"

# nginx設定のボリュームパスを更新してフル起動
echo "==> Step2: Nginx + SSL でフル起動..."
ssh $SSH_OPTS "$REMOTE_USER@$SERVER_IP" "cd $APP_DIR && docker compose up -d"

echo ""
echo "==> デプロイ完了"
echo "  フロントエンド: https://$DOMAIN"
echo "  バックエンドAPI: https://api.$DOMAIN"
echo ""
echo "  ※ CloudflareのDNSに以下を設定してください:"
echo "    A  soc-ai-agent.kazuyukitech.com     -> $SERVER_IP"
echo "    A  api.soc-ai-agent.kazuyukitech.com -> $SERVER_IP"
