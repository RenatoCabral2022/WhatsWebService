#!/usr/bin/env bash
# Whats-Service VM startup script (idempotent)
# Runs on first boot: install deps, clone, build, start
# Runs on subsequent boots: just start services
set -euo pipefail

MARKER="/var/lib/whats-initialized"
LOG="/var/log/whats-startup.log"
REPO_DIR="/opt/whats-service"

exec > >(tee -a "$LOG") 2>&1
echo "=== startup.sh $(date -u) ==="

# ---- Subsequent boots: just start services ----
if [ -f "$MARKER" ]; then
  echo "Already initialized, starting services..."
  cd "$REPO_DIR"
  docker compose -f docker-compose.yml -f infra/gce/docker-compose.prod.yml up -d
  echo "Services started."
  exit 0
fi

# ---- First boot: full provisioning ----
echo "First boot — full provisioning..."

# 1. Install Docker CE + compose plugin + git
apt-get update
apt-get install -y ca-certificates curl gnupg git make

install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg

echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  tee /etc/apt/sources.list.d/docker.list > /dev/null

apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

systemctl enable docker
systemctl start docker

echo "Docker installed: $(docker --version)"
echo "Compose installed: $(docker compose version)"

# 2. Clone repository
CLONE_URL="${repo_url}"
%{ if git_token != "" ~}
# Inject token for private repo
CLONE_URL=$(echo "$CLONE_URL" | sed "s|https://|https://${git_token}@|")
%{ endif ~}

echo "Cloning ${repo_url} (branch: ${repo_branch})..."
git clone --branch "${repo_branch}" --single-branch "$CLONE_URL" "$REPO_DIR"
cd "$REPO_DIR"

echo "Repo cloned. Contents:"
ls -la

# 3. Generate self-signed TLS certificate (for HTTPS / getUserMedia)
CERT_DIR="/opt/whats-certs"
if [ ! -f "$CERT_DIR/cert.pem" ]; then
  echo "Generating self-signed TLS certificate..."
  mkdir -p "$CERT_DIR"
  openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
    -keyout "$CERT_DIR/key.pem" \
    -out "$CERT_DIR/cert.pem" \
    -subj "/CN=${public_ip}" \
    -addext "subjectAltName=IP:${public_ip}"
  echo "TLS certificate generated."
fi

# 4. Generate protobuf stubs (gen/ is gitignored)
echo "Running make proto (Docker-based, may pull images)..."
make proto
echo "Proto generation complete. gen/ contents:"
ls -la gen/

# 4. Build all Docker images
echo "Building Docker images (this will take a while on first run)..."
docker compose -f docker-compose.yml -f infra/gce/docker-compose.prod.yml build

# 5. Start all services
echo "Starting services..."
docker compose -f docker-compose.yml -f infra/gce/docker-compose.prod.yml up -d

echo "Waiting 10s for services to initialize..."
sleep 10

echo "Service status:"
docker compose -f docker-compose.yml -f infra/gce/docker-compose.prod.yml ps

# 6. Mark as initialized
touch "$MARKER"
echo "=== Provisioning complete $(date -u) ==="
echo "Web client: http://${public_ip}"
echo "Control plane: http://${public_ip}:8080"
echo "Metrics: http://${public_ip}:9092/metrics"
