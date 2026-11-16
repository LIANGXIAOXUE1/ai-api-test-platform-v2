param([string]$OutputDirectory = '')
$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
if(-not $OutputDirectory){$OutputDirectory=Join-Path $root ('.runtime/releases/'+(Get-Date -Format 'yyyyMMdd-HHmmss'))}
$output=[IO.Path]::GetFullPath($OutputDirectory)
if(Test-Path -LiteralPath $output){throw '发布快照目标已经存在，请选择新目录'}
New-Item -ItemType Directory -Path $output -Force|Out-Null
# Explicit source directories; never enumerate reports, keys, data or all docs.
$directories=@('backend/cmd','backend/internal','backend/vendor','frontend/src','frontend/tests','deploy','scripts')
$files=@('backend/go.mod','backend/go.sum','frontend/package.json','frontend/package-lock.json','frontend/index.html','frontend/vite.config.ts','README.md','.gitignore','.dockerignore','docs/五项修复与生产上线说明-2026-11-12.md')
foreach($directory in $directories){
    $source=Join-Path $root $directory
    foreach($file in Get-ChildItem -LiteralPath $source -File -Recurse){
        if($file.Extension -in @('.key','.dump','.log','.exe','.pem') -or $file.Name -like '.env*'){continue}
        $files+=([IO.Path]::GetRelativePath($root,$file.FullName)).Replace('\','/')
    }
}
$manifest=@()
foreach($relative in ($files|Sort-Object -Unique)){
    $source=Join-Path $root $relative;$destination=Join-Path $output $relative
    New-Item -ItemType Directory -Path (Split-Path -Parent $destination) -Force|Out-Null
    Copy-Item -LiteralPath $source -Destination $destination
    $manifest+=@{path=$relative;sha256=(Get-FileHash -LiteralPath $destination -Algorithm SHA256).Hash}
}
[IO.File]::WriteAllText((Join-Path $output 'release-manifest.json'),($manifest|ConvertTo-Json -Depth 3),[Text.UTF8Encoding]::new($false))
Write-Host "源码快照已保存：$output；文件数：$($manifest.Count)。无需提交整个工作目录。"
