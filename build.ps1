$ErrorActionPreference = "Stop"

$binDir = Join-Path $PSScriptRoot "bin"
if (-not (Test-Path $binDir)) {
    New-Item -ItemType Directory -Path $binDir | Out-Null
}

$binary = Join-Path $binDir "novelgen.exe"
go build -o $binary
Write-Host "Built $binary"
