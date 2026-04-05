#!/bin/bash
set -ex

# Update and Install Docker
apt-get update
apt-get install -y docker.io docker-compose-v2 git nginx certbot python3-certbot-nginx

# Enable Docker
systemctl enable docker
systemctl start docker

# Add ubuntu user to docker group
usermod -aG docker ubuntu

# Enable Nginx
systemctl enable nginx
systemctl start nginx

# Prepare Application Directory
mkdir -p /home/ubuntu/soc-app
chown -R ubuntu:ubuntu /home/ubuntu/soc-app
