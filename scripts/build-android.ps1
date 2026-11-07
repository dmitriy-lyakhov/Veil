# Сборка Android-клиента Veil: Go-ядро (gomobile bind -> AAR) и APK (Gradle).
# Использование: scripts\build-android.ps1 [-What aar|apk|all]
# Переменные окружения не меняются глобально — всё задаётся только для процесса.
param(
    [ValidateSet('aar', 'apk', 'all')][string]$What = 'all',
    [string]$Tools = (Join-Path (Split-Path $PSScriptRoot -Parent) '..\android-tools')
)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$Tools = (Resolve-Path $Tools).Path

$env:JAVA_HOME        = (Get-ChildItem (Join-Path $Tools 'jdk') -Directory | Where-Object Name -like 'jdk-17*' | Select-Object -First 1).FullName
$env:ANDROID_HOME     = Join-Path $Tools 'sdk'
$env:ANDROID_SDK_ROOT = $env:ANDROID_HOME
$env:JAVA_TOOL_OPTIONS = '-Dfile.encoding=UTF-8'
New-Item -ItemType Directory -Force (Join-Path $Tools 'tmp') | Out-Null
$env:TEMP = Join-Path $Tools 'tmp'; $env:TMP = $env:TEMP  # обход «Unable to establish loopback connection» в JDK/Gradle
$env:ANDROID_NDK_HOME = Join-Path $env:ANDROID_HOME 'ndk\26.1.10909125'
$gradle               = Join-Path $Tools 'gradle\gradle-8.7\bin\gradle.bat'
$env:PATH = "$(Join-Path $Tools 'gobin');$(Join-Path $env:JAVA_HOME 'bin');C:\Program Files\Go\bin;$env:PATH"

Set-Location $root
if ($What -in 'aar', 'all') {
    if (-not (Get-Command gomobile -ErrorAction SilentlyContinue)) {
        $env:GOBIN = Join-Path $Tools 'gobin'
        go install golang.org/x/mobile/cmd/gomobile@latest golang.org/x/mobile/cmd/gobind@latest
    }
    New-Item -ItemType Directory -Force mobile\android\app\libs | Out-Null
    gomobile bind -v -target=android/arm64,android/amd64 -androidapi 26 -trimpath -o mobile\android\app\libs\veilcore.aar ./mobile/core
    if ($LASTEXITCODE -ne 0) { throw 'gomobile bind failed' }
}
if ($What -in 'apk', 'all') {
    Push-Location mobile\android
    try {
        & $gradle --no-daemon assembleDebug
        if ($LASTEXITCODE -ne 0) { throw 'gradle failed' }
    } finally { Pop-Location }
    New-Item -ItemType Directory -Force dist\android | Out-Null
    Copy-Item mobile\android\app\build\outputs\apk\debug\app-debug.apk dist\android\veil-debug.apk -Force
    Write-Host 'APK: dist\android\veil-debug.apk'
}
