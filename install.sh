#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="SunnySMS"
REQUIRED_DOCKER_MAJOR=24
REQUIRED_COMPOSE_MAJOR=2
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$SCRIPT_DIR/deploy"
ENV_FILE="$DEPLOY_DIR/.env"
ENV_EXAMPLE="$DEPLOY_DIR/.env.example"
COMPOSE_FILE="$DEPLOY_DIR/docker-compose.yml"
DATA_DIR="$DEPLOY_DIR/data"

log() { printf '\033[1;32m[SunnySMS]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[SunnySMS]\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31m[SunnySMS]\033[0m %s\n' "$*" >&2; exit 1; }

version_major() {
  printf '%s' "${1:-0}" | sed -E 's/^[^0-9]*([0-9]+).*/\1/'
}

random_hex() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
  else
    head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'
  fi
}

replace_env_value() {
  local key="$1"
  local value="$2"
  if grep -q "^${key}=" "$ENV_FILE"; then
    sed -i "s|^${key}=.*|${key}=${value}|" "$ENV_FILE"
  else
    printf '%s=%s\n' "$key" "$value" >> "$ENV_FILE"
  fi
}

get_env_value() {
  local key="$1"
  awk -v key="$key" '
    index($0, key "=") == 1 { value = substr($0, length(key) + 2) }
    END { print value }
  ' "$ENV_FILE"
}

check_linux() {
  case "$(uname -s)" in
    Linux) ;;
    *) fail "install.sh currently supports Linux servers only. Use docker compose manually on this system." ;;
  esac
}

check_command() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required but not installed. Install it first and rerun this script."
}

docker_install_hint() {
  cat >&2 <<'EOF'

[SunnySMS] Docker 未安装。Ubuntu 24.04 可执行以下命令安装（官方脚本，含 Compose v2 插件）:

    curl -fsSL https://get.docker.com | sudo sh
    sudo systemctl enable --now docker
    sudo usermod -aG docker "$USER" && newgrp docker

安装完成后重新运行 ./install.sh
EOF
}

check_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    docker_install_hint
    fail "Docker is required but not installed."
  fi
  if ! docker info >/dev/null 2>&1; then
    warn "无法访问 Docker 守护进程。请尝试: sudo systemctl enable --now docker"
    warn "若为权限问题: sudo usermod -aG docker \"\$USER\" && newgrp docker"
    fail "Docker is installed but not running, or current user cannot access Docker."
  fi
  local docker_version docker_major compose_version compose_major
  docker_version="$(docker version --format '{{.Server.Version}}' 2>/dev/null || true)"
  docker_major="$(version_major "$docker_version")"
  if [ "${docker_major:-0}" -lt "$REQUIRED_DOCKER_MAJOR" ]; then
    fail "Docker ${REQUIRED_DOCKER_MAJOR}.x or newer is required. Current: ${docker_version:-unknown}."
  fi
  if ! docker compose version >/dev/null 2>&1; then
    warn "缺少 Docker Compose v2 插件。Ubuntu 24.04 可执行: sudo apt-get update && sudo apt-get install -y docker-compose-plugin"
    fail "Docker Compose v2 plugin is required. Install docker compose and rerun this script."
  fi
  compose_version="$(docker compose version --short 2>/dev/null || docker compose version | awk '{print $NF}')"
  compose_major="$(version_major "$compose_version")"
  if [ "${compose_major:-0}" -lt "$REQUIRED_COMPOSE_MAJOR" ]; then
    fail "Docker Compose v${REQUIRED_COMPOSE_MAJOR}.x or newer is required. Current: ${compose_version:-unknown}."
  fi
  log "Docker ${docker_version}, Compose ${compose_version} detected."
}

check_port() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -ltn | awk '{print $4}' | grep -Eq "(:|\])${port}$" && fail "Port ${port} is already in use. Edit deploy/.env SUNNYSMS_HTTP_PORT and rerun."
  elif command -v netstat >/dev/null 2>&1; then
    netstat -ltn | awk '{print $4}' | grep -Eq "(:|\])${port}$" && fail "Port ${port} is already in use. Edit deploy/.env SUNNYSMS_HTTP_PORT and rerun."
  else
    warn "Cannot check port usage because ss/netstat is unavailable."
  fi
}

prepare_env() {
  [ -f "$ENV_EXAMPLE" ] || fail "Missing $ENV_EXAMPLE"
  if [ ! -f "$ENV_FILE" ]; then
    cp "$ENV_EXAMPLE" "$ENV_FILE"
    chmod 600 "$ENV_FILE" || true
    replace_env_value POSTGRES_PASSWORD "$(random_hex)"
    replace_env_value JWT_SECRET "$(random_hex)"
    replace_env_value DATA_ENCRYPTION_KEY "$(random_hex)"
    replace_env_value ADMIN_DEFAULT_PASSWORD "$(random_hex | cut -c1-18)"
    replace_env_value PUID "$(id -u)"
    replace_env_value PGID "$(id -g)"
    log "Created deploy/.env with generated database, JWT, encryption, and admin passwords."
  else
    warn "deploy/.env already exists. Existing values will be kept."
  fi
}

prepare_dirs() {
  mkdir -p "$DATA_DIR/postgres" "$DATA_DIR/storage/card_exports"
  chmod -R 700 "$DATA_DIR" || true
  log "Data directories prepared under $DATA_DIR."
}

compose() {
  docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"
}

main() {
  check_linux
  check_docker
  prepare_env
  prepare_dirs

  local port
  port="$(get_env_value SUNNYSMS_HTTP_PORT)"
  port="${port:-8088}"
  if [ -z "$(compose ps -q web 2>/dev/null || true)" ]; then
    check_port "$port"
  else
    warn "Existing SunnySMS web container detected. Skipping host port pre-check."
  fi

  log "Building and starting services..."
  compose up -d --build

  log "Waiting for services to become healthy..."
  local attempts=60 web_container web_health
  web_container="$(compose ps -q web)"
  while :; do
    web_health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$web_container" 2>/dev/null || true)"
    [ "$web_health" = "healthy" ] && break
    attempts=$((attempts - 1))
    [ "$attempts" -le 0 ] && { compose ps; fail "Services did not become healthy in time. Run: docker compose --env-file deploy/.env -f deploy/docker-compose.yml logs -f"; }
    sleep 3
  done

  local server_ip
  server_ip="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
  server_ip="${server_ip:-SERVER_IP}"

  log "Deployment completed."
  log "URL: http://${server_ip}:${port}"
  log "Admin user: $(get_env_value ADMIN_DEFAULT_USERNAME)"
  log "Admin password is stored in deploy/.env as ADMIN_DEFAULT_PASSWORD."
  log "PostgreSQL data: $DATA_DIR/postgres"
}

main "$@"