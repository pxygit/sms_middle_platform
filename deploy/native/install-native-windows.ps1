# SunnySMS 原生一键部署脚本（Windows，不使用 Docker）
# 检查 Go / Node.js / PostgreSQL 环境，缺失时中断并给出安装命令；
# 就绪后自动建库、生成密钥、构建前后端并启动服务（后端同时托管前端页面）。
#
# 运行方式（以管理员或普通用户均可，PowerShell 5.1+）:
#   powershell -ExecutionPolicy Bypass -File deploy\native\install-native-windows.ps1

$ErrorActionPreference = 'Stop'

$RequiredGoVersion   = [version]'1.25.0'
$RequiredNodeVersion = [version]'20.19.0'
$DefaultHttpAddr     = ':8080'
$DbName = 'sunnysms'
$DbUser = 'sunnysms'

$ScriptDir   = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = (Resolve-Path (Join-Path $ScriptDir '..\..')).Path
$NativeDir   = $ScriptDir
$BinDir      = Join-Path $NativeDir 'bin'
$BinPath     = Join-Path $BinDir 'sunnysms-api.exe'
$EnvFile     = Join-Path $NativeDir '.env'
$StorageDir  = Join-Path $NativeDir 'storage\card_exports'
$StaticDir   = Join-Path $ProjectRoot 'frontend\dist'

function Write-Log([string]$Message)  { Write-Host "[SunnySMS] $Message" -ForegroundColor Green }
function Write-Warn([string]$Message) { Write-Host "[SunnySMS] $Message" -ForegroundColor Yellow }
function Fail([string]$Message) {
    Write-Host "[SunnySMS] $Message" -ForegroundColor Red
    exit 1
}

function Get-RandomHex([int]$Bytes = 32) {
    $buffer = New-Object byte[] $Bytes
    [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($buffer)
    return -join ($buffer | ForEach-Object { $_.ToString('x2') })
}

# ---------------------------------------------------------------- 环境检查

function Test-Go {
    $go = Get-Command go -ErrorAction SilentlyContinue
    if (-not $go) {
        Write-Warn '未检测到 Go。安装方式:'
        Write-Host ''
        Write-Host '    winget install GoLang.Go' -ForegroundColor Cyan
        Write-Host '    # 或访问 https://go.dev/dl/ 下载 Windows 安装包' -ForegroundColor Cyan
        Write-Host ''
        Fail '请安装 Go 后重新打开 PowerShell 并再次运行本脚本。'
    }
    $raw = (& go version)
    if ($raw -match 'go(\d+\.\d+(\.\d+)?)') {
        $current = [version]($Matches[1] + $(if ($Matches[2]) { '' } else { '.0' }))
        if ($current -lt $RequiredGoVersion) {
            Fail "Go 版本过低（当前 $current，需要 >= $RequiredGoVersion）。请执行 winget upgrade GoLang.Go 或从 https://go.dev/dl/ 升级。"
        }
        Write-Log "Go $current √"
    } else {
        Write-Warn "无法解析 Go 版本号: $raw（继续执行）"
    }
}

function Test-Node {
    $node = Get-Command node -ErrorAction SilentlyContinue
    $npm  = Get-Command npm  -ErrorAction SilentlyContinue
    if (-not $node -or -not $npm) {
        Write-Warn "未检测到 Node.js/npm（前端构建需要 Node >= $RequiredNodeVersion）。安装方式:"
        Write-Host ''
        Write-Host '    winget install OpenJS.NodeJS.LTS' -ForegroundColor Cyan
        Write-Host '    # 或访问 https://nodejs.org 下载 LTS 安装包' -ForegroundColor Cyan
        Write-Host ''
        Fail '请安装 Node.js 后重新打开 PowerShell 并再次运行本脚本。'
    }
    $current = [version]((& node --version).TrimStart('v'))
    if ($current -lt $RequiredNodeVersion) {
        Fail "Node.js 版本过低（当前 $current，Vite 7 需要 >= $RequiredNodeVersion）。请执行 winget upgrade OpenJS.NodeJS.LTS 升级。"
    }
    Write-Log "Node.js $current / npm $(& npm --version) √"
}

function Test-Postgres {
    if ((Test-Path $EnvFile) -and (Select-String -Path $EnvFile -Pattern '^DATABASE_DSN=' -Quiet)) {
        Write-Log "检测到已有 $EnvFile，将复用其中的 DATABASE_DSN。"
        $script:SkipProvision = $true
        return
    }
    $psql = Get-Command psql -ErrorAction SilentlyContinue
    if (-not $psql) {
        # 尝试常见安装目录
        $found = Get-ChildItem 'C:\Program Files\PostgreSQL\*\bin\psql.exe' -ErrorAction SilentlyContinue |
            Sort-Object FullName -Descending | Select-Object -First 1
        if ($found) {
            $env:Path = "$($found.DirectoryName);$env:Path"
            Write-Log "已临时将 $($found.DirectoryName) 加入 PATH。"
        } else {
            Write-Warn '未检测到 PostgreSQL。安装方式:'
            Write-Host ''
            Write-Host '    winget install PostgreSQL.PostgreSQL.16' -ForegroundColor Cyan
            Write-Host '    # 安装后将 C:\Program Files\PostgreSQL\16\bin 加入 PATH，并记住 postgres 超级用户密码' -ForegroundColor Cyan
            Write-Host ''
            Fail '请安装 PostgreSQL 后重新运行本脚本。'
        }
    }
    $service = Get-Service -Name 'postgresql*' -ErrorAction SilentlyContinue | Where-Object Status -eq 'Running'
    if (-not $service) {
        $anyService = Get-Service -Name 'postgresql*' -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($anyService) {
            Write-Warn "PostgreSQL 服务未运行。启动方式: Start-Service $($anyService.Name) （或在 services.msc 中启动）"
            Fail 'PostgreSQL 服务未运行。请启动后重新运行本脚本。'
        }
        Write-Warn '未找到 PostgreSQL Windows 服务，若数据库在远程主机运行请忽略。'
    }
    Write-Log 'PostgreSQL √'
}

# ---------------------------------------------------------------- 数据库与配置

function Initialize-Database {
    if ($script:SkipProvision) { return }
    $script:DbPassword = (Get-RandomHex 16)
    $superPassword = Read-Host '请输入本机 PostgreSQL 超级用户 postgres 的密码（用于创建应用数据库）' -AsSecureString
    $plain = [Runtime.InteropServices.Marshal]::PtrToStringAuto(
        [Runtime.InteropServices.Marshal]::SecureStringToBSTR($superPassword))
    $env:PGPASSWORD = $plain
    try {
        & psql -U postgres -h 127.0.0.1 -tAc 'SELECT 1' *> $null
        if ($LASTEXITCODE -ne 0) { Fail '无法使用 postgres 超级用户连接本机数据库，请确认密码与服务状态后重试。' }

        $roleExists = (& psql -U postgres -h 127.0.0.1 -tAc "SELECT 1 FROM pg_roles WHERE rolname='$DbUser'").Trim()
        if ($roleExists -ne '1') {
            & psql -U postgres -h 127.0.0.1 -c "CREATE ROLE $DbUser LOGIN PASSWORD '$($script:DbPassword)'" | Out-Null
        } else {
            & psql -U postgres -h 127.0.0.1 -c "ALTER ROLE $DbUser WITH LOGIN PASSWORD '$($script:DbPassword)'" | Out-Null
            Write-Warn "数据库用户 $DbUser 已存在，密码已重置为新生成值。"
        }
        $dbExists = (& psql -U postgres -h 127.0.0.1 -tAc "SELECT 1 FROM pg_database WHERE datname='$DbName'").Trim()
        if ($dbExists -ne '1') {
            & psql -U postgres -h 127.0.0.1 -c "CREATE DATABASE $DbName OWNER $DbUser" | Out-Null
        }
        Write-Log "数据库 $DbName / 用户 $DbUser 就绪 √"
    } finally {
        Remove-Item Env:\PGPASSWORD -ErrorAction SilentlyContinue
    }
    $script:DatabaseDsn = "host=127.0.0.1 user=$DbUser password=$($script:DbPassword) dbname=$DbName port=5432 sslmode=disable TimeZone=Asia/Shanghai"
}

function Write-EnvFile {
    if (Test-Path $EnvFile) {
        Write-Warn "$EnvFile 已存在，保留现有配置。"
        return
    }
    $adminPassword = (Get-RandomHex 16).Substring(0, 18)
    @"
# SunnySMS 原生部署配置（由 install-native-windows.ps1 生成）
APP_ENV=production
HTTP_ADDR=$DefaultHttpAddr
DATABASE_DSN=$($script:DatabaseDsn)
JWT_SECRET=$(Get-RandomHex 32)
DATA_ENCRYPTION_KEY=$(Get-RandomHex 32)
JWT_EXPIRE_HOURS=24
ADMIN_DEFAULT_USERNAME=admin
ADMIN_DEFAULT_PASSWORD=$adminPassword
STATIC_DIR=$StaticDir
CARD_EXPORT_DIR=$StorageDir
ORDER_POLL_INTERVAL_SECONDS=8
ORDER_TIMEOUT_SECONDS=1200
# 供应商 API Key 可留空，启动后在管理后台"供应商"页面配置
"@ | Set-Content -Path $EnvFile -Encoding UTF8
    Write-Log "已生成 $EnvFile（含随机数据库密码、JWT 密钥、加密密钥与管理员密码）。"
}

# ---------------------------------------------------------------- 构建与启动

function Build-Frontend {
    Write-Log '构建前端（npm ci && npm run build）...'
    Push-Location (Join-Path $ProjectRoot 'frontend')
    try {
        & npm ci --no-audit --no-fund
        if ($LASTEXITCODE -ne 0) {
            & npm install --no-audit --no-fund
            if ($LASTEXITCODE -ne 0) { Fail '前端依赖安装失败。网络问题可尝试: npm config set registry https://registry.npmmirror.com' }
        }
        & npm run build
        if ($LASTEXITCODE -ne 0) { Fail '前端构建失败，请查看上方错误输出。' }
    } finally {
        Pop-Location
    }
    if (-not (Test-Path (Join-Path $StaticDir 'index.html'))) { Fail "前端产物缺失: $StaticDir\index.html" }
    Write-Log '前端构建完成 √'
}

function Build-Backend {
    Write-Log '构建后端二进制...'
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    Push-Location (Join-Path $ProjectRoot 'backend')
    try {
        $env:CGO_ENABLED = '0'
        & go build -trimpath -ldflags '-s -w' -o $BinPath ./cmd/api
        if ($LASTEXITCODE -ne 0) { Fail '后端构建失败。模块下载超时可尝试: go env -w GOPROXY=https://goproxy.cn,direct' }
    } finally {
        Pop-Location
    }
    Write-Log "后端构建完成 √ ($BinPath)"
}

function Import-EnvFile {
    Get-Content $EnvFile | ForEach-Object {
        if ($_ -match '^\s*#') { return }
        if ($_ -match '^\s*([A-Z0-9_]+)=(.*)$') {
            [Environment]::SetEnvironmentVariable($Matches[1], $Matches[2], 'Process')
        }
    }
}

function Start-Service-App {
    $existing = Get-Process -Name 'sunnysms-api' -ErrorAction SilentlyContinue
    if ($existing) {
        Write-Warn "检测到已在运行的 sunnysms-api（PID $($existing.Id)），先停止旧进程..."
        $existing | Stop-Process -Force
        Start-Sleep -Seconds 1
    }
    New-Item -ItemType Directory -Force -Path $StorageDir | Out-Null
    Import-EnvFile
    Write-Log '启动 SunnySMS 服务...'
    $process = Start-Process -FilePath $BinPath -WorkingDirectory $NativeDir -PassThru -WindowStyle Minimized
    $process.Id | Set-Content (Join-Path $NativeDir 'sunnysms.pid')

    $addr = ([Environment]::GetEnvironmentVariable('HTTP_ADDR', 'Process'))
    if (-not $addr) { $addr = $DefaultHttpAddr }
    $port = $addr.Split(':')[-1]
    $healthy = $false
    foreach ($i in 1..30) {
        Start-Sleep -Seconds 2
        try {
            Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$port/health" -TimeoutSec 3 | Out-Null
            $healthy = $true
            break
        } catch { }
    }
    if (-not $healthy) {
        Fail "健康检查超时。请查看最小化窗口中的服务日志（进程 PID $($process.Id)）。"
    }

    Write-Log '================= 部署完成 ================='
    Write-Log "访问地址   : http://localhost:$port"
    Write-Log "管理后台   : http://localhost:$port/admin"
    Write-Log "管理员账号 : admin（密码见 $EnvFile 中 ADMIN_DEFAULT_PASSWORD）"
    Write-Log "停止服务   : Stop-Process -Name sunnysms-api"
    Write-Log "重新部署   : 再次运行本脚本即可（会自动重启）"
}

# ---------------------------------------------------------------- 主流程

Write-Log '开始 SunnySMS 原生部署（Windows）'
$script:SkipProvision = $false
Test-Go
Test-Node
Test-Postgres
Initialize-Database
Write-EnvFile
Build-Frontend
Build-Backend
Start-Service-App
