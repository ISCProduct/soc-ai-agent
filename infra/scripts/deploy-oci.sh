#!/bin/bash
# OCI Compute Instance へのデプロイスクリプト
# 使用方法: ./infra/scripts/deploy-oci.sh <SERVER_IP>
set -euo pipefail

SERVER_IP="${1:-}"
SSH_KEY="${2:-~/.ssh/id_ed25519}"
REMOTE_USER="ubuntu"
APP_DIR="/opt/soc-app"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

if [ -z "$SERVER_IP" ]; then
  echo "使用方法: $0 <SERVER_IP> [SSH_KEY_PATH]"
  echo "例: $0 1.2.3.4"
  exit 1
fi

echo "==> $SERVER_IP へデプロイ開始"

# .env ファイルの存在確認
if [ ! -f "$REPO_ROOT/.env" ]; then
  echo "エラー: .env ファイルが見つかりません ($REPO_ROOT/.env)"
  exit 1
fi

SSH_OPTS="-i $SSH_KEY -o StrictHostKeyChecking=no"

# Docker が起動済みか確認（cloud-init完了待ち）
echo "==> Docker 起動確認中..."
for i in $(seq 1 12); do
  if ssh $SSH_OPTS "$REMOTE_USER@$SERVER_IP" "docker info > /dev/null 2>&1"; then
    echo "==> Docker 起動済み"
    break
  fi
  echo "    待機中... ($i/12)"
  sleep 10
done

# AWS ECR ログイン (イメージをECRから取得するため)
echo "==> ECR ログイン情報をサーバーへ転送..."
AWS_TOKEN=$(aws ecr get-login-password --region ap-northeast-1 2>/dev/null || echo "")
if [ -n "$AWS_TOKEN" ]; then
  ssh $SSH_OPTS "$REMOTE_USER@$SERVER_IP" \
    "echo '$AWS_TOKEN' | docker login --username AWS --password-stdin 970835573274.dkr.ecr.ap-northeast-1.amazonaws.com"
fi

# ファイル転送
echo "==> ファイル転送中..."
ssh $SSH_OPTS "$REMOTE_USER@$SERVER_IP" "mkdir -p $APP_DIR"
scp $SSH_OPTS "$REPO_ROOT/compose.oci.yml" "$REMOTE_USER@$SERVER_IP:$APP_DIR/docker-compose.yml"
scp $SSH_OPTS "$REPO_ROOT/.env" "$REMOTE_USER@$SERVER_IP:$APP_DIR/.env"

# 起動
echo "==> docker compose up..."
ssh $SSH_OPTS "$REMOTE_USER@$SERVER_IP" "cd $APP_DIR && docker compose pull && docker compose up -d"

echo ""
echo "==> デプロイ完了"
echo "  フロントエンド: http://$SERVER_IP:3000"
echo "  バックエンド:   http://$SERVER_IP:8080"
