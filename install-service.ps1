# 把本目录的 sql-relay.exe 安装为 Windows 服务并启动。
# 请用管理员身份运行 PowerShell：
#   .\install-service.ps1
#Requires -Version 5.1
$ErrorActionPreference = "Stop"

$ServiceName = "sql-relay"
$DisplayName = "sql-relay"

function Assert-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw "请用管理员身份运行此脚本。"
    }
}

Assert-Administrator

$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$Exe = Join-Path $Root "sql-relay.exe"
$Config = Join-Path $Root "config.yaml"
if (-not (Test-Path -LiteralPath $Exe)) {
    throw "找不到 $Exe"
}
if (-not (Test-Path -LiteralPath $Config)) {
    throw "找不到 $Config"
}

$existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($existing) {
    if ($existing.Status -ne "Stopped") {
        Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
        $deadline = (Get-Date).AddSeconds(20)
        while ((Get-Service -Name $ServiceName).Status -ne "Stopped" -and (Get-Date) -lt $deadline) {
            Start-Sleep -Seconds 1
        }
    }
    & sc.exe delete $ServiceName | Out-Null
    $deadline = (Get-Date).AddSeconds(20)
    while ((Get-Service -Name $ServiceName -ErrorAction SilentlyContinue) -and (Get-Date) -lt $deadline) {
        Start-Sleep -Seconds 1
    }
}

# CreateService 需要的二进制路径。路径含空格时必须给 exe 加引号。
$binaryPath = '"{0}" -config "{1}"' -f $Exe, $Config
New-Service -Name $ServiceName `
    -BinaryPathName $binaryPath `
    -DisplayName $DisplayName `
    -Description "Yearning 只读 SQL 中转代理。日志写入程序目录下的 sql-relay.log。" `
    -StartupType Automatic | Out-Null

# 进程异常退出后自动拉起。
& sc.exe failure $ServiceName reset= 86400 actions= restart/5000/restart/5000/restart/5000 | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw "设置服务失败恢复策略失败，退出码 $LASTEXITCODE"
}

Start-Service -Name $ServiceName
Get-Service -Name $ServiceName | Format-List Name, Status, StartType, DisplayName
Write-Host "已安装并启动。工作目录和日志都在 $Root"
Write-Host "日志文件: $(Join-Path $Root 'sql-relay.log')"
