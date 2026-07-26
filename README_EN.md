# SunnySMS

<p align="center">
  <strong>Enterprise SMS Number and Verification Code Receiving Platform</strong>
</p>

<p align="center">
  <a href="./README.md">中文</a> | <a href="./README_EN.md">English</a>
</p>

<p align="center">
  <a href="https://go.dev/" target="_blank" rel="noreferrer"><img alt="Go" src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go&logoColor=white"></a>
  <a href="https://react.dev/" target="_blank" rel="noreferrer"><img alt="React" src="https://img.shields.io/badge/React-19+-61DAFB?style=flat-square&logo=react&logoColor=111111"></a>
  <a href="https://www.postgresql.org/" target="_blank" rel="noreferrer"><img alt="PostgreSQL" src="https://img.shields.io/badge/PostgreSQL-16+-4169E1?style=flat-square&logo=postgresql&logoColor=white"></a>
  <a href="https://www.docker.com/" target="_blank" rel="noreferrer"><img alt="Docker" src="https://img.shields.io/badge/Docker-Ready-2496ED?style=flat-square&logo=docker&logoColor=white"></a>
  <a href="./LICENSE"><img alt="License" src="https://img.shields.io/badge/License-MIT-green?style=flat-square"></a>
</p>

---
SunnySMS is an enterprise-oriented SMS number and verification-code receiving platform. It manages service configurations, redeemable card codes, number orders, SMS retrieval, supplier balances, announcements, and audit logs. The public user flow is card-code based and does not require user registration, while the admin console provides operational management for providers, services, cards, orders, announcements, and auditing.

> This project provides technical implementation only. Operators are responsible for ensuring that their usage complies with applicable laws, target-platform terms, and SMS supplier policies.

### Features

- Public card-code based number ordering and SMS verification-code retrieval.
- Admin console for service configuration, card batch generation, TXT export, card management, and order management.
- Supports both short-lived instant SMS providers and long-lived number providers.
- Provider adapter architecture keeps business logic independent from supplier-specific APIs.
- Integrated providers include `smspool`, `firefox`, `herosms`, `smsbower`, `5sim`, `lubansms`, `68sms`, and `62-us`.
- Card codes are not stored in plaintext. Only hashes and masked values are stored in the database.
- Provider balance checks, country/service metadata synchronization, and order cost tracking.
- Light/Dark theme and Simplified Chinese/English language switching.
- Admin announcements, audit logs, password change, and token-expiration login redirect.
- Three deployment options with built-in environment checks: one-command Docker Compose install, native Linux install (systemd), and native Windows install.

### Technology Stack

| Layer | Technology |
| --- | --- |
| Backend | Go, Gin, GORM |
| Database | PostgreSQL |
| Frontend | React, TypeScript, Vite |
| UI | Ant Design, Lucide Icons |
| Deployment | Docker, Docker Compose, Nginx |

### Deployment Options

| Option | Requirements | Best for |
| --- | --- | --- |
| ① One-command Docker install | Docker 24+ / Compose v2 | Production Linux servers (recommended, verified on Ubuntu 24.04) |
| ② Native Linux install | Go 1.25+ / Node 20.19+ / PostgreSQL | Linux servers without Docker, managed by systemd |
| ③ Native Windows install | Go 1.25+ / Node 20.19+ / PostgreSQL | Windows evaluation and development |

All three installers verify the environment first and abort with copy-paste
install commands when a dependency is missing.

### Option 1: Quick Deployment with Docker

A Linux server with Docker is the recommended production deployment method.

#### Requirements

- Linux server
- Docker Engine 24+
- Docker Compose v2+
- An available HTTP port, default `8088`

#### Installation

```bash
git clone <your-repository-url> SunnySMS
cd SunnySMS
chmod +x install.sh
./install.sh
```

The installer will:

- Check Docker and Compose versions.
- Check Docker daemon access.
- Check whether the configured HTTP port is available.
- Create `deploy/.env`.
- Generate strong random database, JWT, encryption, and default admin credentials.
- Create host data directories.
- Build and start PostgreSQL, the Go API, and the Nginx web service.

Default URL:

```text
http://SERVER_IP:8088
```

Admin credentials are stored in `deploy/.env`:

```env
ADMIN_DEFAULT_USERNAME=admin
ADMIN_DEFAULT_PASSWORD=<generated-password>
```

### Manual Docker Compose Deployment

```bash
cp deploy/.env.example deploy/.env
vim deploy/.env

docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d --build
```

Common commands:

```bash
# Service status
docker compose --env-file deploy/.env -f deploy/docker-compose.yml ps

# Logs
docker compose --env-file deploy/.env -f deploy/docker-compose.yml logs -f

# Stop services
docker compose --env-file deploy/.env -f deploy/docker-compose.yml down

# Rebuild after code changes
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d --build
```

### Option 2: Native Linux Deployment (systemd)

For servers without Docker, adapted for Ubuntu 24.04:

```bash
chmod +x deploy/native/install-native-linux.sh
./deploy/native/install-native-linux.sh
```

The script checks Go >= 1.25, Node.js >= 20.19, npm, and PostgreSQL (aborting
with Ubuntu 24.04 install commands when missing), provisions the `sunnysms`
database idempotently, generates `deploy/native/.env` with random secrets,
builds the frontend and backend, and installs a `sunnysms` systemd service.
The Go process serves both the API and the frontend (`STATIC_DIR`), so no
Nginx is required.

```bash
sudo systemctl status sunnysms
sudo systemctl restart sunnysms
sudo journalctl -u sunnysms -f
```

Re-running the script rebuilds and restarts the service while keeping the
existing `.env`.

### Option 3: Native Windows Deployment

```powershell
powershell -ExecutionPolicy Bypass -File deploy\native\install-native-windows.ps1
```

The script checks Go / Node.js / PostgreSQL (with `winget` install hints),
prompts once for the local `postgres` superuser password to create the
application database, generates `deploy/native/.env`, builds both ends, and
starts `sunnysms-api.exe` with a health check. Stop it with
`Stop-Process -Name sunnysms-api`.

### Data Persistence and Migration

PostgreSQL data is stored on the host at:

```text
deploy/data/postgres
```

Runtime storage, including card-code TXT exports, is stored at:

```text
deploy/data/storage
```

Back up these paths before server migration:

```text
deploy/data/postgres
deploy/data/storage
deploy/.env
```

Example database backup:

```bash
set -a
. deploy/.env
set +a

mkdir -p deploy/backups
docker compose --env-file deploy/.env -f deploy/docker-compose.yml exec -T postgres \
  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" > deploy/backups/sunnysms_$(date +%F_%H%M%S).sql
```

### Local Development

Backend:

```bash
cd backend
cp .env.example .env
go mod tidy
go run ./cmd/api
```

Frontend:

```bash
cd frontend
npm install
npm run dev
```

### Configuration

Main configuration templates:

- `backend/.env.example` for local backend development.
- `deploy/.env.example` for production Docker deployment.

Important variables:

| Variable | Description |
| --- | --- |
| `DATABASE_DSN` | PostgreSQL connection string used by the API service |
| `JWT_SECRET` | Admin authentication token signing secret |
| `DATA_ENCRYPTION_KEY` | Provider credentials and sensitive data encryption key |
| `ADMIN_DEFAULT_USERNAME` | Initial admin username |
| `ADMIN_DEFAULT_PASSWORD` | Initial admin password |
| `STATIC_DIR` | Frontend build directory served by the Go process (native deployments) |
| `CARD_EXPORT_DIR` | Card-code TXT export directory |
| `ORDER_POLL_INTERVAL_SECONDS` | SMS polling interval |
| `ORDER_TIMEOUT_SECONDS` | Short-lived order timeout |

### Security Recommendations

- Change the default admin password in production.
- Use strong random values for `JWT_SECRET` and `DATA_ENCRYPTION_KEY`.
- Never commit `deploy/.env`, `deploy/native/.env`, `backend/.env`, or `deploy/data/`.
- Deploy behind an HTTPS reverse proxy when exposed publicly.
- Manage supplier API keys carefully and restrict outbound server IPs when possible.

### Contributing

Issues and pull requests are welcome. Before submitting changes, run:

```bash
cd backend && go test ./...
cd ../frontend && npm run build
```

### License

SunnySMS is open-sourced under the [MIT License](LICENSE).
