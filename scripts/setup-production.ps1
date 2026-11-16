param(
    [Parameter(Mandatory)][string]$Domain,
    [Parameter(Mandatory)][string]$AllowedTargets,
    [string]$User = 'owner',
    [string]$OutputDirectory = ''
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
if ($Domain -notmatch '^[a-zA-Z0-9][a-zA-Z0-9.-]*\.[a-zA-Z]{2,}$') { throw 'Domain 只填写域名，不含 https:// 或路径' }
if ($User -notmatch '^[a-zA-Z0-9_-]{1,64}$') { throw '用户名使用字母、数字、下划线或短横线' }
foreach ($target in $AllowedTargets.Split(',')) {
    $uri = $null
    if (-not [Uri]::TryCreate($target.Trim(), [UriKind]::Absolute, [ref]$uri) -or $uri.Scheme -notin @('http','https') -or $uri.UserInfo -or $uri.AbsolutePath -ne '/' -or $uri.Query -or $uri.Fragment) { throw 'AllowedTargets 只填 http/https 来源地址，不含路径或密钥' }
}
if (-not $OutputDirectory) { $OutputDirectory = Join-Path $root '.runtime/production' }
$output = [IO.Path]::GetFullPath($OutputDirectory)
$secretDirectory = Join-Path $output 'secrets'
$dataDirectory = Join-Path $output 'data'
New-Item -ItemType Directory -Force -Path $secretDirectory,$dataDirectory | Out-Null
function Set-TaskPrivateDirectory([string]$Path) {
    if ($env:OS -eq 'Windows_NT') {
        $acl = Get-Acl -LiteralPath $Path
        $acl.SetAccessRuleProtection($true,$false)
        $sid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User
        $rule = [System.Security.AccessControl.FileSystemAccessRule]::new($sid,'FullControl','ContainerInherit,ObjectInherit','None','Allow')
        $acl.SetAccessRule($rule)
        Set-Acl -LiteralPath $Path -AclObject $acl
    } else { & chmod 700 $Path; if ($LASTEXITCODE -ne 0) { throw '设置目录权限失败' } }
}
Set-TaskPrivateDirectory $output
foreach ($name in @('database-password.txt','platform-password.txt')) {
    $path = Join-Path $secretDirectory $name
    if (-not (Test-Path -LiteralPath $path)) {
        $bytes = New-Object byte[] 32
        $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
        try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
        [IO.File]::WriteAllText($path,[Convert]::ToBase64String($bytes),[Text.UTF8Encoding]::new($false))
        if ($env:OS -ne 'Windows_NT') { & chmod 600 $path }
    }
}
$lines = @("PLATFORM_DOMAIN=$Domain","PLATFORM_AUTH_USER=$User","PLATFORM_ALLOWED_TARGETS=$AllowedTargets","PLATFORM_SECRETS_DIR=$($secretDirectory.Replace('\','/'))","PLATFORM_DATA_DIR=$($dataDirectory.Replace('\','/'))")
[IO.File]::WriteAllLines((Join-Path $output 'production.env'),$lines,[Text.UTF8Encoding]::new($false))
Write-Host "生产配置已保存到 $output。重复运行会保留已有密码。"
Write-Host '平台登录密码在 secrets/platform-password.txt，请自行查看并保存；未在终端输出密码。'
Write-Host '迁移既有数据时，必须恢复数据库与匹配的 project-test.key 后再启动后端。'
