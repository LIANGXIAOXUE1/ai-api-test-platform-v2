param(
    [string]$ComposeFile = '',
    [string]$EnvFile = '',
    [string]$OutputDirectory = '',
    [string]$KeyDirectory = '',
    [switch]$ContainerKey
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
if (-not $ComposeFile) { $ComposeFile = Join-Path $root 'deploy/docker-compose.yml' }
if (-not $OutputDirectory) { $OutputDirectory = Join-Path $root ('.runtime/backups/' + (Get-Date -Format 'yyyyMMdd-HHmmss')) }
if (-not $KeyDirectory) { $KeyDirectory = Join-Path $root 'backend/data' }
$output = [IO.Path]::GetFullPath($OutputDirectory)
if (Test-Path -LiteralPath $output) { throw '备份目标目录已存在，请换一个新目录' }
New-Item -ItemType Directory -Path $output -Force | Out-Null
if ($env:OS -eq 'Windows_NT') {
    $acl=Get-Acl -LiteralPath $output; $acl.SetAccessRuleProtection($true,$false)
    $acl.SetAccessRule([Security.AccessControl.FileSystemAccessRule]::new([Security.Principal.WindowsIdentity]::GetCurrent().User,'FullControl','ContainerInherit,ObjectInherit','None','Allow'))
    Set-Acl -LiteralPath $output -AclObject $acl
} else { & chmod 700 $output }
$dockerCommand = Get-Command docker -ErrorAction SilentlyContinue
$docker = if ($dockerCommand) { $dockerCommand.Source } else { 'D:/app/Docker/resources/bin/docker.exe' }
$compose = @('compose','-f',$ComposeFile)
if ($EnvFile) { $compose += @('--env-file',$EnvFile) }
$temporary = '/tmp/pulse-' + [Guid]::NewGuid().ToString('N') + '.dump'
& $docker @compose exec -T db pg_dump -U api_user -d api_test -Fc -f $temporary
if ($LASTEXITCODE -ne 0) { throw '数据库备份失败' }
& $docker @compose cp "db:$temporary" (Join-Path $output 'database.dump')
if ($LASTEXITCODE -ne 0) { throw '读取数据库备份失败' }
& $docker @compose exec -T db rm -f $temporary
if ($ContainerKey) {
    & $docker @compose cp 'backend:/app/data/project-test.key' (Join-Path $output 'project-test.key')
    if ($LASTEXITCODE -ne 0) { throw '没有拿到加密密钥，不能把本次备份标为完整' }
} else {
    $key = Join-Path $KeyDirectory 'project-test.key'
    if (-not (Test-Path -LiteralPath $key)) { throw '原加密密钥不存在，请先检查；不能用新密钥替代' }
    Copy-Item -LiteralPath $key -Destination (Join-Path $output 'project-test.key')
}
$hashes = @{}
foreach ($name in @('database.dump','project-test.key')) { $hashes[$name]=(Get-FileHash -LiteralPath (Join-Path $output $name) -Algorithm SHA256).Hash }
$manifest = @{createdAt=(Get-Date).ToUniversalTime().ToString('o'); database='api_test'; hashes=$hashes; note='数据库和密钥为一组。恢复脚本仅创建新的验证数据库。'}
[IO.File]::WriteAllText((Join-Path $output 'manifest.json'),($manifest|ConvertTo-Json -Depth 4),[Text.UTF8Encoding]::new($false))
Write-Host "备份完成：$output。备份包含敏感数据，请妥善保管。"
