#!/usr/bin/env bash
# SunnySMS 原生一键部署脚本（不使用 Docker）
# 适配 Ubuntu 24.04 云服务器；检查 Go / Node.js / PostgreSQL 环境，
# 缺失时中断并给出对应安装命令；就绪后自动建库、生成密钥、构建并注册 systemd 服务。
set -Eeuo pipefail

APP_NAME="SunnySMS"
SERVICE_NAME="sunnysms"
REQUIRED_GO_VERSION="1.25.0"
REQUIRED_NODE_VERSION="20.19.0"
DEFAULT_HTTP_ADDR=":8080"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NATIVE_DIR="$SCRIPT_DIR"
BIN_DIR="$NATIVE_DIR/bin"
BIN_PATH="$BIN_DIR/sunnysms-api"
ENV_FILE="$NATIVE_DIR/.env"
STORAGE_DIR="$NATIVE_DIR/storage/card_exports"
STATIC_DIR="$PROJECT_ROOT/frontend/dist"

DB_NAME="${SUNNYSMS_DB_NAME:-sunnysms}"
DB_USER="${SUNNYSMS_DB_USER:-sunnysms}"

log()  { printf '\033[1;32m[%s]\033[0m %s\n' "$APP_NAME" "$*"; }
warn() { printf '\033[1;33m[%s]\033[0m %s\n' "$APP_NAME" "$*"; }
fail() { printf '\033[1;31m[%s]\033[0m %s\n' "$APP_NAME" "$*" >&2; exit 1; }

hint() { printf '\n    %s\n' "$@" >&2; printf '\n' >&2; }

version_ge() {
  # version_ge A B => A >= B ?
  [ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]
}

random_hex() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
  else
    head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'
  fi
}

# ---------------------------------------------------------------- 环境检查

check_linux() {
  [ "$(uname -s)" = "Linux" ] || fail "该脚本仅支持 Linux。Windows 请使用 deploy/native/install-native-windows.ps1"
  if [ -r /etc/os-release ]; then
    . /etc/os-release
    log "检测到系统: ${PRETTY_NAME:-unknown}"
    case "${ID:-}" in
      ubuntu|debian) ;;
      *) warn "非 Ubuntu/Debian 系统，下述安装提示命令请按你的发行版调整。" ;;
    esac
  fi
}

check_basic_tools() {
  local missing=()
  for tool in curl tar openssl; do
    command -v "$tool" >/dev/null 2>&1 || missing+=("$tool")
  done
  if [ "${#missing[@]}" -gt 0 ]; then
    warn "缺少基础工具: ${missing[*]}"
    hint "sudo apt-get update && sudo apt-get install -y ${missing[*]}"
    fail "请先安装以上基础工具后重新运行本脚本。"
  fi
}

check_go() {
  if ! command -v go >/dev/null 2>&1; then
    warn "未检测到 Go。Ubuntu 24.04 安装 Go ${REQUIRED_GO_VERSION}+ 的方式（推荐官方 tarball）:"
    hint \
      "curl -LO https://go.dev/dl/go${REQUIRED_GO_VERSION}.linux-amd64.tar.gz" \
      "sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go${REQUIRED_GO_VERSION}.linux-amd64.tar.gz" \
      "echo 'export PATH=\$PATH:/usr/local/go/bin' >> ~/.profile && . ~/.profile" \
      "# 或使用 snap: sudo snap install go --classic"
    fail "Go 未安装。请安装后重新运行本脚本。"
  fi
  local go_version
  go_version="$(go version | sed -E 's/.*go([0-9]+\.[0-9]+(\.[0-9]+)?).*/\1/')"
  if ! version_ge "$go_version" "${REQUIRED_GO_VERSION%.*}"; then
    warn "Go 版本过低（当前 ${go_version}，需要 >= ${REQUIRED_GO_VERSION%.*}）。请升级后重试:"
    hint "sudo rm -rf /usr/local/go && curl -L https://go.dev/dl/go${REQUIRED_GO_VERSION}.linux-amd64.tar.gz | sudo tar -C /usr/local -xz"
    fail "Go 版本不满足要求。"
  fi
  log "Go ${go_version} ✓"
}

check_node() {
  if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then
    warn "未检测到 Node.js/npm（前端构建需要 Node >= ${REQUIRED_NODE_VERSION}）。Ubuntu 24.04 推荐 NodeSource 安装 Node 22 LTS:"
    hint \
      "curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -" \
      "sudo apt-get install -y nodejs"
    fail "Node.js/npm 未安装。请安装后重新运行本脚本。"
  fi
  local node_version
  node_version="$(node --version | tr -d 'v')"
  if ! version_ge "$node_version" "$REQUIRED_NODE_VERSION"; then
    warn "Node.js 版本过低（当前 ${node_version}，Vite 7 需要 >= ${REQUIRED_NODE_VERSION}）。升级方式:"
    hint \
      "curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -" \
      "sudo apt-get install -y nodejs"
    fail "Node.js 版本不满足要求。"
  fi
  log "Node.js ${node_version} / npm $(npm --version) ✓"
}

check_postgres() {
  # 已有 .env 且包含 DATABASE_DSN 时只做连通性检查
  if [ -f "$ENV_FILE" ] && grep -q '^DATABASE_DSN=' "$ENV_FILE"; then
    log "检测到已有 $ENV_FILE，将复用其中的 DATABASE_DSN。"
    return
  fi
  if ! command -v psql >/dev/null 2>&1; then
    warn "未检测到 PostgreSQL 客户端/服务端。Ubuntu 24.04 安装方式:"
    hint \
      "sudo apt-get update" \
      "sudo apt-get install -y postgresql postgresql-contrib" \
      "sudo systemctl enable --now postgresql"
    fail "PostgreSQL 未安装。请安装后重新运行本脚本。"
  fi
  if command -v pg_isready >/dev/null 2>&1 && ! pg_isready -q 2>/dev/null && ! pg_isready -q -h 127.0.0.1 2>/dev/null; then
    warn "PostgreSQL 已安装但服务未运行。启动方式:"
    hint "sudo systemctl enable --now postgresql"
    fail "PostgreSQL 服务未运行。请启动后重新运行本脚本。"
  fi
  log "PostgreSQL $(psql --version | awk '{print $3}') ✓"
}

check_port() {
  local addr port
  addr="$DEFAULT_HTTP_ADDR"
  [ -f "$ENV_FILE" ] && addr="$(grep -E '^HTTP_ADDR=' "$ENV_FILE" | tail -n1 | cut -d= -f2- || true)"
  addr="${addr:-$DEFAULT_HTTP_ADDR}"
  port="${addr##*:}"
  if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
    return  # 本服务占用属正常，稍后会重启
  fi
  if command -v ss >/dev/null 2>&1 && ss -ltn | awk '{print $4}' | grep -Eq "(:|\])${port}$"; then
    fail "端口 ${port} 已被占用。请修改 ${ENV_FILE} 中的 HTTP_ADDR 后重试。"
  fi
}

# ---------------------------------------------------------------- 数据库与配置

provision_database() {
  [ -f "$ENV_FILE" ] && grep -q '^DATABASE_DSN=' "$ENV_FILE" && return

  DB_PASSWORD="$(random_hex | cut -c1-32)"
  if id postgres >/dev/null 2>&1 && { sudo -n true 2>/dev/null || sudo -v 2>/dev/null; }; then
    log "使用本机 PostgreSQL 创建数据库 ${DB_NAME} 与用户 ${DB_USER}（幂等）..."
    if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='${DB_USER}'" | grep -q 1; then
      sudo -u postgres psql -c "CREATE ROLE ${DB_USER} LOGIN PASSWORD '${DB_PASSWORD}'"
    else
      sudo -u postgres psql -c "ALTER ROLE ${DB_USER} WITH LOGIN PASSWORD '${DB_PASSWORD}'"
      warn "数据库用户 ${DB_USER} 已存在，密码已重置为新生成值。"
    fi
    if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'" | grep -q 1; then
      sudo -u postgres createdb -O "${DB_USER}" "${DB_NAME}"
    fi
  else
    warn "无法通过 sudo 访问本机 postgres 用户。请手动创建数据库后，在 ${ENV_FILE} 中配置 DATABASE_DSN 并重新运行。"
    fail "数据库未就绪。示例 DSN: host=127.0.0.1 user=${DB_USER} password=xxx dbname=${DB_NAME} port=5432 sslmode=disable TimeZone=Asia/Shanghai"
  fi
  DATABASE_DSN="host=127.0.0.1 user=${DB_USER} password=${DB_PASSWORD} dbname=${DB_NAME} port=5432 sslmode=disable TimeZone=Asia/Shanghai"
}

write_env_file() {
  if [ -f "$ENV_FILE" ]; then
    warn "$ENV_FILE 已存在，保留现有配置。"
    return
  fi
  local admin_password
  admin_password="$(random_hex | cut -c1-18)"
  cat > "$ENV_FILE" <<EOF
# SunnySMS 原生部署配置（由 install-native-linux.sh 生成）
APP_ENV=production
HTTP_ADDR=${DEFAULT_HTTP_ADDR}
DATABASE_DSN=${DATABASE_DSN}
JWT_SECRET=$(random_hex)
DATA_ENCRYPTION_KEY=$(random_hex)
JWT_EXPIRE_HOURS=24
ADMIN_DEFAULT_USERNAME=admin
ADMIN_DEFAULT_PASSWORD=${admin_password}
STATIC_DIR=${STATIC_DIR}
CARD_EXPORT_DIR=${STORAGE_DIR}
ORDER_POLL_INTERVAL_SECONDS=8
ORDER_TIMEOUT_SECONDS=1200
# 供应商 API Key 可留空，启动后在管理后台“供应商”页面配置
EOF
  chmod 600 "$ENV_FILE" || true
  log "已生成 $ENV_FILE（含随机数据库密码、JWT 密钥、加密密钥与管理员密码）。"
}

# ---------------------------------------------------------------- 构建

build_frontend() {
  log "构建前端（npm ci && npm run build）..."
  (
    cd "$PROJECT_ROOT/frontend"
    npm ci --no-audit --no-fund 2>/dev/null || npm install --no-audit --no-fund
    npm run build
  ) || fail "前端构建失败。若为网络问题可尝试: npm config set registry https://registry.npmmirror.com"
  [ -f "$STATIC_DIR/index.html" ] || fail "前端产物缺失: $STATIC_DIR/index.html"
  log "前端构建完成 ✓"
}

build_backend() {
  log "构建后端二进制..."
  mkdir -p "$BIN_DIR"
  (
    cd "$PROJECT_ROOT/backend"
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$BIN_PATH" ./cmd/api
  ) || fail "后端构建失败。若为模块下载超时可尝试: go env -w GOPROXY=https://goproxy.cn,direct"
  log "后端构建完成 ✓ ($BIN_PATH)"
}

# ---------------------------------------------------------------- 启动

install_systemd_service() {
  if ! command -v systemctl >/dev/null 2>&1; then
    warn "系统无 systemd，改为前台/后台手动运行:"
    hint "cd $NATIVE_DIR && ./bin/sunnysms-api"
    return 1
  fi
  local run_user unit_file
  run_user="$(id -un)"
  unit_file="/etc/systemd/system/${SERVICE_NAME}.service"
  log "注册 systemd 服务 ${SERVICE_NAME}（需要 sudo）..."
  sudo tee "$unit_file" >/dev/null <<EOF
[Unit]
Description=SunnySMS API (serves frontend and API)
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
User=${run_user}
WorkingDirectory=${NATIVE_DIR}
EnvironmentFile=${ENV_FILE}
ExecStart=${BIN_PATH}
Restart=always
RestartSec=3
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
EOF
  sudo systemctl daemon-reload
  sudo systemctl enable --now "$SERVICE_NAME"
  sudo systemctl restart "$SERVICE_NAME"
}

wait_health() {
  local addr port attempts=30
  addr="$(grep -E '^HTTP_ADDR=' "$ENV_FILE" | tail -n1 | cut -d= -f2-)"
  port="${addr##*:}"
  log "等待服务健康检查通过..."
  while [ "$attempts" -gt 0 ]; do
    if curl -fs "http://127.0.0.1:${port}/health" >/dev/null 2>&1; then
      log "服务已就绪 ✓"
      return 0
    fi
    attempts=$((attempts - 1))
    sleep 2
  done
  warn "健康检查超时。查看日志: sudo journalctl -u ${SERVICE_NAME} -n 100 --no-pager"
  return 1
}

print_summary() {
  local addr port server_ip
  addr="$(grep -E '^HTTP_ADDR=' "$ENV_FILE" | tail -n1 | cut -d= -f2-)"
  port="${addr##*:}"
  server_ip="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
  server_ip="${server_ip:-SERVER_IP}"
  log "================= 部署完成 ================="
  log "访问地址   : http://${server_ip}:${port}"
  log "管理后台   : http://${server_ip}:${port}/admin"
  log "管理员账号 : $(grep -E '^ADMIN_DEFAULT_USERNAME=' "$ENV_FILE" | cut -d= -f2-)"
  log "管理员密码 : 见 ${ENV_FILE} 中 ADMIN_DEFAULT_PASSWORD"
  log "服务管理   : sudo systemctl {status|restart|stop} ${SERVICE_NAME}"
  log "查看日志   : sudo journalctl -u ${SERVICE_NAME} -f"
  if command -v ufw >/dev/null 2>&1 && sudo ufw status 2>/dev/null | grep -q "Status: active"; then
    warn "检测到 UFW 防火墙已启用，如需公网访问请放行端口: sudo ufw allow ${port}/tcp"
  fi
}

main() {
  log "开始 ${APP_NAME} 原生部署（Ubuntu 24.04 适配）"
  check_linux
  check_basic_tools
  check_go
  check_node
  check_postgres
  mkdir -p "$STORAGE_DIR"
  provision_database
  write_env_file
  check_port
  build_frontend
  build_backend
  if install_systemd_service; then
    wait_health || true
    print_summary
  fi
}

main "$@"
