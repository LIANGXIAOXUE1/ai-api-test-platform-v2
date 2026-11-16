param(
    [Parameter(Mandatory)][string]$BackupDirectory,
    [string]$DatabaseName = '',
    [string]$ComposeFile = '',
    [string]$EnvFile = ''
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
if (-not $ComposeFile) { $ComposeFile = Join-Path $root 'deploy/docker-compose.yml' }
if (-not $DatabaseName) { $DatabaseName = 'pulse_restore_' + (Get-Date -Format 'yyyyMMddHHmmss') }
if ($DatabaseName -notmatch '^pulse_restore_[a-z0-9_]{1,40}$') { throw '恢复只允许新建 pulse_restore_ 开头的验证数据库，不覆盖当前平台' }
$backup = [IO.Path]::GetFullPath($BackupDirectory)
$manifest = Get-Content -LiteralPath (Join-Path $backup 'manifest.json') -Raw | ConvertFrom-Json
foreach ($name in @('database.dump','project-test.key')) {
    $actual = (Get-FileHash -LiteralPath (Join-Path $backup $name) -Algorithm SHA256).Hash
    if ($actual -ne $manifest.hashes.$name) { throw "备份校验失败：$name" }
}
$dockerCommand=Get-Command docker -ErrorAction SilentlyContinue
$docker=if($dockerCommand){$dockerCommand.Source}else{'D:/app/Docker/resources/bin/docker.exe'}
$compose=@('compose','-f',$ComposeFile); if($EnvFile){$compose+=@('--env-file',$EnvFile)}
& $docker @compose exec -T db createdb -U api_user $DatabaseName
if ($LASTEXITCODE -ne 0) { throw '验证数据库创建失败，已有数据库不会被覆盖' }
$temporary='/tmp/pulse-restore-'+[Guid]::NewGuid().ToString('N')+'.dump'
& $docker @compose cp (Join-Path $backup 'database.dump') "db:$temporary"
if ($LASTEXITCODE -ne 0) { throw '读取备份失败' }
& $docker @compose exec -T db pg_restore -U api_user --no-owner --exit-on-error -d $DatabaseName $temporary
if ($LASTEXITCODE -ne 0) { throw '恢复失败，请检查新的验证数据库；原平台保持原样' }
& $docker @compose exec -T db rm -f $temporary
$keyOutput=Join-Path $root ".runtime/restored/$DatabaseName"
New-Item -ItemType Directory -Path $keyOutput -Force | Out-Null
if ($env:OS -eq 'Windows_NT') {
    $acl=Get-Acl -LiteralPath $keyOutput;$acl.SetAccessRuleProtection($true,$false)
    $acl.SetAccessRule([Security.AccessControl.FileSystemAccessRule]::new([Security.Principal.WindowsIdentity]::GetCurrent().User,'FullControl','ContainerInherit,ObjectInherit','None','Allow'))
    Set-Acl -LiteralPath $keyOutput -AclObject $acl
} else { & chmod 700 $keyOutput }
Copy-Item -LiteralPath (Join-Path $backup 'project-test.key') -Destination (Join-Path $keyOutput 'project-test.key')
Write-Host "已恢复到独立验证数据库 $DatabaseName；密钥目录：$keyOutput。原数据库未替换。"
Write-Host '先用独立后端端口验证任务/模型/报告，再在维护窗口决定切换。'
