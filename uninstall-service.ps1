# 停止并删除 sql-relay Windows 服务。不删除 exe、config.yaml 和日志。
# 请用管理员身份运行 PowerShell：
#   .\uninstall-service.ps1
#Requires -Version 5.1
$ErrorActionPreference = "Stop"

$ServiceName = "sql-relay"

function Assert-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw "请用管理员身份运行此脚本。"
    }
}

Assert-Administrator

$existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if (-not $existing) {
    Write-Host "服务 $ServiceName 不存在，无需卸载。"
    exit 0
}

if ($existing.Status -ne "Stopped") {
    Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
    $deadline = (Get-Date).AddSeconds(20)
    while ((Get-Service -Name $ServiceName).Status -ne "Stopped" -and (Get-Date) -lt $deadline) {
        Start-Sleep -Seconds 1
    }
}

if ((Get-Service -Name $ServiceName).Status -ne "Stopped") {
    $procId = (Get-CimInstance Win32_Service -Filter "Name='$ServiceName'").ProcessId
    if ($procId) {
        Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 1
    }
}

& sc.exe delete $ServiceName | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw "删除服务失败，退出码 $LASTEXITCODE"
}

$deadline = (Get-Date).AddSeconds(20)
while ((Get-Service -Name $ServiceName -ErrorAction SilentlyContinue) -and (Get-Date) -lt $deadline) {
    Start-Sleep -Seconds 1
}

if (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue) {
    throw "服务已标记删除，但仍被占用。请关掉占用它的进程后重新运行本脚本。"
}

Write-Host "已卸载服务 $ServiceName。程序文件仍留在本目录。"
