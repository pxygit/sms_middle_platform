# SunnySMS

<p align="center">
  <strong>企业级短信号码与验证码接收平台</strong>
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

## 中文

SunnySMS 是一个面向企业级使用场景的短信接码中台系统，用于管理接码服务配置、卡密兑换码、号码申请、验证码查询、订单状态、供应商余额与审计日志。系统提供用户端免登录卡密取号流程，以及管理员后台用于供应商、服务配置、卡密批次、订单、公告和审计管理。

> 本项目仅提供技术实现。使用者应确保自身业务符合所在地法律法规、目标平台服务条款和短信供应商使用规范。

### 核心特性

- 用户端通过卡密获取手机号、接收验证码、查询历史记录。
- 管理后台支持服务配置、卡密批次生成、TXT 导出、卡密管理和订单管理。
- 支持短效即时接码供应商和长效号码接码供应商两类模式。
- 已实现多供应商适配器结构，业务层不直接耦合具体供应商 API。
- 已集成供应商包括 `smspool`、`firefox`、`herosms`、`smsbower`、`lubansms`、`68sms`、`62-us`。
- 卡密不明文入库，数据库仅保存哈希与掩码；后台按需授权查看明文。
- 支持供应商余额检测、供应商国家/服务元数据同步、订单成本记录。
- 支持 Light/Dark 主题和中文/英文界面切换。
- 管理后台提供公告管理、审计日志、密码修改和凭证过期跳转登录。
- 支持 Docker Compose 部署和 `install.sh` 一键安装。

### 技术栈

| 层级 | 技术 |
| --- | --- |
| 后端 | Go、Gin、GORM |
| 数据库 | PostgreSQL |
| 前端 | React、TypeScript、Vite |
| 界面组件 | Ant Design、Lucide Icons |
| 部署 | Docker、Docker Compose、Nginx |

### 项目结构

```text
SunnySMS/
├── backend/              # Go 后端 API 服务
│   ├── cmd/api/          # API 启动入口
│   ├── internal/         # 业务服务、处理器、适配器、模型
│   ├── Dockerfile
│   └── .env.example
├── frontend/             # React 前端应用
│   ├── src/
│   ├── Dockerfile
│   └── package.json
├── deploy/               # 生产部署文件
│   ├── docker-compose.yml
│   ├── .env.example
│   ├── nginx/default.conf
│   └── README.md
├── docs/                 # API 与供应商文档
├── install.sh            # Docker 一键部署脚本
├── LICENSE
└── README.md
```

### 快速部署：Docker 一键安装

推荐在 Linux 服务器中使用 Docker 方式部署。

#### 环境要求

- Linux 服务器
- Docker Engine 24+
- Docker Compose v2+
- 可用公网或内网端口，默认 `8088`

#### 安装命令

```bash
git clone <your-repository-url> SunnySMS
cd SunnySMS
chmod +x install.sh
./install.sh
```

安装脚本会自动完成：

- 检查 Docker 和 Compose 版本。
- 检查 Docker daemon 是否可访问。
- 检查部署端口是否被占用。
- 创建 `deploy/.env`。
- 生成数据库密码、JWT 密钥、数据加密密钥和默认管理员密码。
- 创建宿主机数据目录。
- 构建并启动 PostgreSQL、Go API 和 Nginx Web 服务。

默认访问地址：

```text
http://SERVER_IP:8088
```

管理员账号信息在 `deploy/.env` 中：

```env
ADMIN_DEFAULT_USERNAME=admin
ADMIN_DEFAULT_PASSWORD=<generated-password>
```

### Docker Compose 手动部署

如果需要手动配置部署参数：

```bash
cp deploy/.env.example deploy/.env
vim deploy/.env

docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d --build
```

常用命令：

```bash
# 查看服务状态
docker compose --env-file deploy/.env -f deploy/docker-compose.yml ps

# 查看日志
docker compose --env-file deploy/.env -f deploy/docker-compose.yml logs -f

# 停止服务
docker compose --env-file deploy/.env -f deploy/docker-compose.yml down

# 更新代码后重新构建
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d --build
```

### 数据持久化与迁移

Docker 部署时，PostgreSQL 数据会映射到宿主机目录：

```text
deploy/data/postgres
```

运行时存储，例如卡密 TXT 导出文件，会映射到：

```text
deploy/data/storage
```

迁移服务器前，应备份以下目录：

```text
deploy/data/postgres
deploy/data/storage
deploy/.env
```

数据库备份示例：

```bash
set -a
. deploy/.env
set +a

mkdir -p deploy/backups
docker compose --env-file deploy/.env -f deploy/docker-compose.yml exec -T postgres \
  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" > deploy/backups/sunnysms_$(date +%F_%H%M%S).sql
```

### 本地开发

#### 后端

```bash
cd backend
cp .env.example .env
go mod tidy
go run ./cmd/api
```

默认 API 地址：

```text
http://127.0.0.1:8080
```

#### 前端

```bash
cd frontend
npm install
npm run dev
```

默认前端地址：

```text
http://127.0.0.1:5173
```

### 重要配置

主要配置文件：

- `backend/.env.example`：本地后端开发环境模板。
- `deploy/.env.example`：生产 Docker 部署环境模板。

关键环境变量：

| 变量 | 说明 |
| --- | --- |
| `DATABASE_DSN` | API 服务使用的 PostgreSQL 连接字符串 |
| `JWT_SECRET` | 管理员登录令牌签名密钥 |
| `DATA_ENCRYPTION_KEY` | 供应商凭证与敏感数据加密密钥 |
| `ADMIN_DEFAULT_USERNAME` | 初始管理员账号 |
| `ADMIN_DEFAULT_PASSWORD` | 初始管理员密码 |
| `CARD_EXPORT_DIR` | 卡密 TXT 导出目录 |
| `ORDER_POLL_INTERVAL_SECONDS` | 短信轮询间隔 |
| `ORDER_TIMEOUT_SECONDS` | 短效订单超时时间 |

### 安全建议

- 生产环境必须修改默认管理员密码。
- 生产环境必须使用足够长且随机的 `JWT_SECRET` 和 `DATA_ENCRYPTION_KEY`。
- 不要提交 `deploy/.env`、`backend/.env` 或 `deploy/data/`。
- 建议将服务部署在 HTTPS 反向代理之后。
- 供应商 API Key 应通过后台配置或部署环境变量管理，并限制服务器出口 IP。

### 贡献

欢迎提交 Issue 和 Pull Request。提交代码前建议执行：

```bash
cd backend && go test ./...
cd ../frontend && npm run build
```

### 开源协议

本项目基于 [MIT License](LICENSE) 开源。