# Сборка Veil для Windows без make.
# Нужны: Go 1.27+ и MinGW-w64 (gcc) в PATH. Результат: dist\windows\ — veil.exe (GUI),
# veil-cli.exe и server\ с серверными бинарниками для Linux (из них приложение ставит сервер).
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
$out = Join-Path $PSScriptRoot 'dist\windows'
New-Item -ItemType Directory -Force (Join-Path $out 'server') | Out-Null
$flags = '-s -w'

function Build($goos, $goarch, $cgo, $ldflags, $target, $pkg) {
    $env:GOOS = $goos; $env:GOARCH = $goarch; $env:CGO_ENABLED = $cgo
    & go build -trimpath -ldflags $ldflags -o $target $pkg
    if ($LASTEXITCODE -ne 0) { throw "go build $pkg не удался" }
}

Build 'windows' 'amd64' '1' "$flags -H=windowsgui" (Join-Path $out 'veil.exe') './cmd/veil'
Build 'windows' 'amd64' '0' $flags (Join-Path $out 'veil-cli.exe') './cmd/veil-cli'
foreach ($arch in 'amd64', 'arm64') {
    Build 'linux' $arch '0' $flags (Join-Path $out "server\veil-server-linux-$arch") './cmd/veil-server'
}
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
Write-Host "Готово: $out"
