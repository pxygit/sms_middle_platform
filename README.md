# SunnySMS

<p align="center">
  <strong>开箱即用的短信验证码接收中台</strong>
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

SunnySMS 是一个多供应商聚合的短信验证码接收中台。系统通过统一的适配器层对接多家上游接码供应商，向终端用户提供免登录的「卡密取号 → 接收验证码」流程，并为运营方提供完整的管理后台：供应商管理、服务配置、卡密批次、订单、公告、余额监控与审计日志。

> **合规声明**：本项目仅提供技术实现。使用者应自行确保业务符合所在地法律法规、目标平台服务条款及短信供应商使用规范。

## 核心特性

- **免登录用户端**：输入卡密即可校验、取号、轮询验证码、查看历史记录，支持多订单并行。
- **多供应商聚合**：适配器架构解耦业务与上游 API，已集成 `smspool`、`firefox`、`herosms`、`smsbower`、`5sim`、`lubansms`、`68sms`、`62-us`，可平滑扩展新供应商。
- **短效 / 长效双模式**：同时支持即时接码供应商与长效号码供应商（手动查收、有效期档位、库存查询）。
- **卡密安全设计**：卡密不明文入库，仅存哈希与掩码；后台按需授权查看明文，操作留痕。
- **凭证加密存储**：供应商 API Key 与登录凭证使用 `DATA_ENCRYPTION_KEY` 加密后入库。
- **运营能力**：供应商余额检测、国家/服务元数据同步、订单成本记录、访问统计、排行看板。
- **公告系统**：支持静默/弹窗两种触达方式、生效时间窗与已读统计。
- **审计日志**：管理端关键操作全量记录（动作、资源、操作者、IP、UA）。
- **现代前端体验**：Light/Dark 主题、中英文切换、路由懒加载、暗色防闪烁、流畅过渡动画。
- **三种部署方式**：Docker Compose 一键部署、Linux 原生一键部署（systemd）、Windows 原生一键部署，均内置环境检查。

## 系统架构

```text
┌─────────────┐     ┌──────────────────────────────┐     ┌─────────────────┐
│ React SPA   │ ──> │ Go API (Gin)                 │ ──> │ PostgreSQL      │
│ 用户端/后台  │     │ ├─ 业务服务层                 │     └─────────────────┘
└─────────────┘     │ ├─ 订单轮询 / 元数据同步任务    │
                    │ └─ 供应商适配器层              │ ──> smspool / 5sim / 68sms / ...
                    └──────────────────────────────┘
```

- Docker 部署：Nginx 托管前端静态资源并反向代理 API。
- 原生部署：Go 进程通过 `STATIC_DIR` 直接托管前端产物，单进程单端口，无需 Nginx。

## 技术栈

| 层级 | 技术 |
| --- | --- |
| 后端 | Go 1.25、Gin、GORM |
| 数据库 | PostgreSQL 16 |
| 前端 | React 19、TypeScript、Vite 7、TanStack Query |
| 界面组件 | Ant Design 6、Lucide Icons |
| 部署 | Docker Compose / systemd 原生部署，Nginx（可选） |

## 项目结构

```text
SunnySMS/
├── backend/                      # Go 后端 API 服务
│   ├── cmd/api/                  # 启动入口
│   ├── internal/                 # 配置、路由、服务、处理器、供应商适配器、模型
│   ├── Dockerfile
│   └── .env.example
├── frontend/                     # React 前端（用户端 + 管理后台）
│   ├── src/
│   ├── Dockerfile
│   └── package.json
├── deploy/                       # 部署文件
│   ├── docker-compose.yml        # PostgreSQL + API + Nginx
│   ├── .env.example
│   ├── nginx/default.conf
│   ├── native/                   # 原生部署（不使用 Docker）
│   │   ├── install-native-linux.sh
│   │   └── install-native-windows.ps1
│   └── README.md                 # 部署详细文档
├── docs/                         # API 文档
├── install.sh                    # Docker 一键部署脚本
├── LICENSE
└── README.md
```

## 快速开始：三种部署方式

| 方式 | 依赖 | 适用场景 |
| --- | --- | --- |
| ① Docker 一键部署 | Docker 24+ / Compose v2 | Linux 云服务器生产环境（推荐，Ubuntu 24.04 已验证） |
| ② Linux 原生一键部署 | Go 1.25+ / Node 20.19+ / PostgreSQL | 不便使用 Docker 的 Linux 服务器，systemd 托管 |
| ③ Windows 原生一键部署 | Go 1.25+ / Node 20.19+ / PostgreSQL | Windows 环境试用与开发验证 |

三个脚本均会先检查环境：依赖齐备才开始部署，缺失时中断并输出对应系统可直接复制的安装命令。

### 方式一：Docker 一键部署（推荐）

```bash
git clone <your-repository-url> SunnySMS
cd SunnySMS
chmod +x install.sh
./install.sh
```

脚本自动完成：检查 Docker/Compose 版本与守护进程 → 检查端口占用 → 生成 `deploy/.env`（随机数据库密码、JWT 密钥、数据加密密钥、管理员密码）→ 创建数据目录 → 构建并启动 PostgreSQL、API、Nginx 三个容器 → 等待健康检查通过。

部署完成后访问：

```text
http://SERVER_IP:8088          # 用户端
http://SERVER_IP:8088/admin    # 管理后台（账号密码见 deploy/.env）
```

常用运维命令：

```bash
# 查看状态 / 日志
docker compose --env-file deploy/.env -f deploy/docker-compose.yml ps
docker compose --env-file deploy/.env -f deploy/docker-compose.yml logs -f

# 更新代码后重建
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d --build

# 停止服务
docker compose --env-file deploy/.env -f deploy/docker-compose.yml down
```

### 方式二：Linux 原生一键部署（systemd）

适用于不使用 Docker 的场景，已针对 Ubuntu 24.04 适配：

```bash
chmod +x deploy/native/install-native-linux.sh
./deploy/native/install-native-linux.sh
```

脚本自动完成：

1. 检查 Go ≥ 1.25、Node.js ≥ 20.19、npm、PostgreSQL 与基础工具，缺失时中断并给出 Ubuntu 24.04 安装命令。
2. 幂等创建 `sunnysms` 数据库与用户（本机 PostgreSQL），或复用已有 `deploy/native/.env` 中的 `DATABASE_DSN`。
3. 生成 `deploy/native/.env`（随机数据库密码、JWT 密钥、加密密钥、管理员密码）。
4. 构建前端产物与后端二进制，Go 进程直接托管前端页面（无需 Nginx）。
5. 注册并启动 `sunnysms` systemd 服务（开机自启、崩溃自动重启），等待健康检查。
6. 检测到 UFW 防火墙启用时提示放行端口。

服务管理：

```bash
sudo systemctl status sunnysms     # 状态
sudo systemctl restart sunnysms    # 重启
sudo journalctl -u sunnysms -f     # 日志
```

重复执行脚本即可增量更新：保留 `.env`，重新构建并重启服务。

### 方式三：Windows 原生一键部署

```powershell
powershell -ExecutionPolicy Bypass -File deploy\native\install-native-windows.ps1
```

脚本检查 Go / Node.js / PostgreSQL（缺失时给出 `winget` 安装命令），提示输入本机 `postgres` 超级用户密码以创建应用数据库，生成 `deploy/native/.env`，构建前后端并启动 `sunnysms-api.exe`，等待健康检查通过。

```powershell
Stop-Process -Name sunnysms-api    # 停止服务；重新部署直接再次运行脚本
```

## 数据持久化与备份

Docker 部署的数据落盘位置：

```text
deploy/data/postgres     # PostgreSQL 数据
deploy/data/storage      # 运行时存储（卡密 TXT 导出等）
deploy/.env              # 密钥与配置（迁移必备）
```

数据库备份示例：

```bash
set -a; . deploy/.env; set +a
mkdir -p deploy/backups
docker compose --env-file deploy/.env -f deploy/docker-compose.yml exec -T postgres \
  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" > deploy/backups/sunnysms_$(date +%F_%H%M%S).sql
```

原生部署对应备份 PostgreSQL 数据库与 `deploy/native/.env`。

## 本地开发

```bash
# 后端（默认 http://127.0.0.1:8080）
cd backend
cp .env.example .env      # 按需修改 DATABASE_DSN
go run ./cmd/api

# 前端（默认 http://127.0.0.1:5173，已配置 API 代理）
cd frontend
npm install
npm run dev
```

## 重要配置

| 变量 | 说明 |
| --- | --- |
| `DATABASE_DSN` | PostgreSQL 连接字符串 |
| `JWT_SECRET` | 管理员登录令牌签名密钥 |
| `DATA_ENCRYPTION_KEY` | 供应商凭证与敏感数据加密密钥 |
| `ADMIN_DEFAULT_USERNAME` / `ADMIN_DEFAULT_PASSWORD` | 初始管理员账号密码 |
| `STATIC_DIR` | 前端产物目录，设置后由 Go 进程直接托管页面（原生部署使用） |
| `CARD_EXPORT_DIR` | 卡密 TXT 导出目录 |
| `ORDER_POLL_INTERVAL_SECONDS` | 验证码轮询间隔 |
| `ORDER_TIMEOUT_SECONDS` | 短效订单超时时间 |

完整变量清单见 `backend/.env.example` 与 `deploy/.env.example`；供应商 API Key 也可在启动后于管理后台「供应商」页面配置（加密入库）。

## 安全建议

- 生产环境务必修改默认管理员密码，使用一键脚本时密码已随机生成。
- `JWT_SECRET` 与 `DATA_ENCRYPTION_KEY` 必须足够长且随机；更换 `DATA_ENCRYPTION_KEY` 会导致已加密凭证无法解密，请提前规划。
- `deploy/.env`、`deploy/native/.env`、`backend/.env` 与数据目录均已被 `.gitignore` 排除，切勿提交到仓库。
- 建议将服务部署在 HTTPS 反向代理（如 Caddy / Nginx + certbot）之后。
- 供应商 API Key 建议通过管理后台或部署环境变量管理，并在供应商侧限制出口 IP。

## 贡献

欢迎提交 Issue 与 Pull Request。提交前请确保：

```bash
cd backend && go build ./... && go test ./...
cd ../frontend && npm run build
```

## 开源协议

本项目基于 [MIT License](LICENSE) 开源。
