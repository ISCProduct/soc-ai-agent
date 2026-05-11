#!/bin/bash
set -euo pipefail

# Docker インストール
curl -fsSL https://get.docker.com | sh
systemctl enable --now docker
usermod -aG docker ubuntu || usermod -aG docker opc

# Docker Compose インストール
COMPOSE_VERSION="v2.27.0"
curl -SL "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-linux-aarch64" \
  -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose

# アプリディレクトリ作成
mkdir -p /opt/${project_name}
chown -R ubuntu:ubuntu /opt/${project_name} 2>/dev/null || chown -R opc:opc /opt/${project_name}
