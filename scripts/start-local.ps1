param(
    [string]$CloudDiskRoot,
    [int]$FrontendPort = 5177,
    [int]$CloudDiskPort = 8082,
    [switch]$RunSmoke
)
$ErrorActionPreference = 'Stop'
$platformRoot = Split-Path -Parent $PSScriptRoot
if (-not $CloudDiskRoot) { $CloudDiskRoot = Join-Path (Split-Path -Parent $platformRoot) 'cloud-disk' }
if (-not (Test-Path -LiteralPath (Join-Path $CloudDiskRoot 'sql/init.sql'))) {
    throw '请先把 https://github.com/LIANGXIAOXUE1/cloud-disk 克隆到平台同级目录，或用 -CloudDiskRoot 指定源码目录。'
}
$dockerCommand = Get-Command docker.exe -ErrorAction Stop
$env:CLOUD_DISK_SOURCE = (Resolve-Path -LiteralPath $CloudDiskRoot).Path
$env:PLATFORM_WEB_PORT = "$FrontendPort"
$env:CLOUD_DISK_PORT = "$CloudDiskPort"
Push-Location $platformRoot
try {
    if (-not (Test-Path -LiteralPath '.env.acceptance')) { Copy-Item -LiteralPath '.env.acceptance.example' -Destination '.env.acceptance' }
    & $dockerCommand.Source info --format '{{.ServerVersion}}'
    if ($LASTEXITCODE -ne 0) { throw '请先启动 Docker 引擎。' }
    & $dockerCommand.Source compose --env-file .env.acceptance -f deploy/docker-compose.acceptance.yml up -d --build
    if ($LASTEXITCODE -ne 0) { throw '启动未完成；请查看构建日志、端口占用和Docker镜像网络。未清理任何数据库。' }
    & $dockerCommand.Source compose --env-file .env.acceptance -f deploy/docker-compose.acceptance.yml ps
    if ($RunSmoke) {
        & $dockerCommand.Source compose --env-file .env.acceptance -f deploy/docker-compose.acceptance.yml --profile checks run --rm smoke
        if ($LASTEXITCODE -ne 0) { throw '冒烟存在失败/阻塞；请查看 .runtime/peer-acceptance 实测报告。' }
    }
    Write-Host "平台：http://127.0.0.1:$FrontendPort/；云盘OpenAPI：http://127.0.0.1:$CloudDiskPort/v3/api-docs"
    Write-Host '平台登录凭据在本地 .env.acceptance；任务后台目标使用 http://cloud-disk:8082。'
} finally { Pop-Location }
