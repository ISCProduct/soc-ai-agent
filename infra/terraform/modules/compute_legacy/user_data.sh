#!/bin/bash
set -ex

# Update and Install Docker + AWS CLI + Nginx
apt-get update
apt-get install -y docker.io docker-compose-v2 git nginx certbot python3-certbot-nginx awscli

# Enable Docker
systemctl enable docker
systemctl start docker

# Add ubuntu user to docker group
usermod -aG docker ubuntu

# Nginx reverse proxy config
cat > /etc/nginx/sites-available/soc-app <<'NGINX'
server {
    listen 80;
    server_name it-industryanalysis.jp www.it-industryanalysis.jp;

    location / {
        proxy_pass http://localhost:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}

server {
    listen 80;
    server_name api.it-industryanalysis.jp;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
NGINX

ln -sf /etc/nginx/sites-available/soc-app /etc/nginx/sites-enabled/soc-app
rm -f /etc/nginx/sites-enabled/default

# Enable Nginx
systemctl enable nginx
systemctl start nginx

# Prepare Application Directory
mkdir -p /home/ubuntu/soc-app
chown -R ubuntu:ubuntu /home/ubuntu/soc-app
