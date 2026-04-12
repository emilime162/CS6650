#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# User data script for album-store EC2 instance
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

exec > >(tee /var/log/user-data.log)
exec 2>&1

echo "=== Album Store EC2 Setup Started: $(date) ==="

# ── Install Go ────────────────────────────────────────────────────────────────
echo "[1/5] Installing Go ${go_version}"
cd /tmp
curl -sL -o go${go_version}.linux-amd64.tar.gz https://go.dev/dl/go${go_version}.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go${go_version}.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> /home/ec2-user/.bashrc
echo 'export PATH=$PATH:/home/ec2-user/go/bin' >> /home/ec2-user/.bashrc
export PATH=$PATH:/usr/local/go/bin
go version

# ── Install git (if needed for cloning) ──────────────────────────────────────
echo "[2/5] Installing git"
sudo yum install -y git

%{ if git_repo_url != "" ~}
# ── Clone repository ──────────────────────────────────────────────────────────
echo "[3/5] Cloning repository from ${git_repo_url}"
cd /home/ec2-user
sudo -u ec2-user git clone --depth 1 --branch ${git_branch} ${git_repo_url} album-store-src
cd album-store-src

# ── Build application ─────────────────────────────────────────────────────────
echo "[4/5] Building album-store application"
sudo -u ec2-user /usr/local/go/bin/go mod download
sudo -u ec2-user /usr/local/go/bin/go build -ldflags="-s -w" -o /home/ec2-user/album-store .
%{ else ~}
# ── Manual deployment mode ────────────────────────────────────────────────────
echo "[3/5] Skipping git clone (no repo URL provided)"
echo "[4/5] Manual deployment: use 'make deploy' to upload binary"
echo "      The binary should be placed at /home/ec2-user/album-store"
%{ endif ~}

# ── Create systemd service ────────────────────────────────────────────────────
echo "[5/5] Creating systemd service"
sudo tee /etc/systemd/system/album-store.service > /dev/null <<'EOF'
[Unit]
Description=Album Store — CS 6650 ChaosArena service
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
User=ec2-user
WorkingDirectory=/home/ec2-user

ExecStart=/home/ec2-user/album-store

Restart=always
RestartSec=1

# Graceful shutdown
KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=10

# Environment
Environment=PORT=8080
Environment=AWS_REGION=${aws_region}
Environment=ALBUMS_TABLE=${albums_table}
Environment=PHOTOS_TABLE=${photos_table}
Environment=S3_BUCKET=${s3_bucket}
Environment=WORKER_COUNT=384

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable album-store

%{ if git_repo_url != "" ~}
# Start the service (only if binary was built)
sudo systemctl start album-store
echo "Service started. Check status with: sudo systemctl status album-store"
%{ else ~}
# Don't start yet - waiting for binary upload
echo "Service enabled but not started (waiting for binary deployment)"
echo "After uploading binary, run: sudo systemctl start album-store"
%{ endif ~}

echo "=== Album Store EC2 Setup Complete: $(date) ==="
echo ""
echo "AWS Region:       ${aws_region}"
echo "Albums Table:     ${albums_table}"
echo "Photos Table:     ${photos_table}"
echo "S3 Bucket:        ${s3_bucket}"
echo ""
echo "Instance is ready!"
