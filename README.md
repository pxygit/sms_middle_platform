# sms_purchase

接码中台，MVP 包含：

- Go + Gin + GORM + PostgreSQL 后端
- React + TypeScript + Vite 前端
- smspool 接码适配器
- 多接码供应商适配器接口预留
- 管理后台卡密生成、TXT 导出、服务配置、订单与审计
- 用户端卡密取号、等待验证码、取消号码、历史查询

## Backend

```powershell
cd backend
Copy-Item .env.example .env
go mod tidy
go run ./cmd/api
```

默认管理员来自 `.env`：

- `ADMIN_DEFAULT_USERNAME=admin`
- `ADMIN_DEFAULT_PASSWORD=admin123456`

## Frontend

```powershell
cd frontend
npm install
npm run dev
```

PowerShell 如果禁止执行 `npm.ps1`，可使用：

```powershell
npm.cmd install
npm.cmd run dev
```

## Notes

- 数据库表统一以 `sys_` 开头。
- 卡密不明文入库，只保存 hash 和 mask。
- 卡密明文仅在批次生成后用于 TXT 导出。
- 当前只实现 smspool，后续供应商通过 `internal/adapter/sms` 扩展。
