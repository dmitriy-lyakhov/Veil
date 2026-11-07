#!/usr/bin/env bash
# Сборка Android-клиента Veil: Go-ядро (gomobile bind -> AAR) и APK (Gradle).
# Использование: scripts/build-android.sh [aar|apk|release|all]   (по умолчанию all = aar + debug apk)
#   release — app-release.apk с R8 и подписью (нужны параметры подписи, см. ниже)
# Переменные окружения (значения по умолчанию — каталог ../android-tools рядом с репозиторием):
#   TOOLS         каталог с инструментами (jdk, sdk, gradle, gobin)
#   JAVA_HOME, ANDROID_HOME, GRADLE
#   Подпись release: VEIL_KEYSTORE, VEIL_STORE_PASSWORD, VEIL_KEY_ALIAS, VEIL_KEY_PASSWORD — либо файл
#   $TOOLS/signing-secrets.txt (строки KEY=VALUE, вне репозитория); keystore лежит в mobile/android/signing/ (не коммитится)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOLS="${TOOLS:-$ROOT/../android-tools}"
export JAVA_HOME="${JAVA_HOME:-$(ls -d "$TOOLS"/jdk/jdk-17* | head -1)}"
export ANDROID_HOME="${ANDROID_HOME:-$TOOLS/sdk}"
export ANDROID_SDK_ROOT="$ANDROID_HOME"
export JAVA_TOOL_OPTIONS="-Dfile.encoding=UTF-8"  # javac: кириллица в комментариях Go-API
# Windows: у JDK/Gradle бывает "Unable to establish loopback connection" (AF_UNIX в %TEMP% с короткими именами 8.3) —
# обходится коротким каталогом TEMP внутри $TOOLS.
if command -v cygpath >/dev/null; then
  mkdir -p "$TOOLS/tmp"; export TEMP="$(cygpath -w "$TOOLS/tmp")" TMP="$(cygpath -w "$TOOLS/tmp")"
fi
export ANDROID_NDK_HOME="${ANDROID_NDK_HOME:-$ANDROID_HOME/ndk/26.1.10909125}"
GRADLE="${GRADLE:-$(ls -d "$TOOLS"/gradle/gradle-8.7/bin/gradle* | grep -v '\.bat$' | head -1)}"
export PATH="$TOOLS/gobin:$JAVA_HOME/bin:$PATH"
[ -d "/c/Program Files/Go/bin" ] && export PATH="/c/Program Files/Go/bin:$PATH"

what="${1:-all}"
cd "$ROOT"

if [ "$what" = aar ] || [ "$what" = all ]; then
  command -v gomobile >/dev/null || go install golang.org/x/mobile/cmd/gomobile@latest golang.org/x/mobile/cmd/gobind@latest
  mkdir -p mobile/android/app/libs
  gomobile bind -v -target=android/arm64,android/amd64 -androidapi 26 -trimpath \
    -o mobile/android/app/libs/veilcore.aar ./mobile/core
fi

if [ "$what" = apk ] || [ "$what" = all ]; then
  ( cd mobile/android && "$GRADLE" --no-daemon assembleDebug )
  mkdir -p dist/android && cp mobile/android/app/build/outputs/apk/debug/app-debug.apk dist/android/veil-debug.apk
  echo "APK: dist/android/veil-debug.apk"
fi

if [ "$what" = release ]; then
  S="${VEIL_SECRETS:-$TOOLS/signing-secrets.txt}"
  if [ -z "${VEIL_KEYSTORE:-}" ] && [ -f "$S" ]; then
    while IFS='=' read -r k v; do [ -n "$k" ] && export "$k=$v"; done < "$S"
  fi
  [ -n "${VEIL_KEYSTORE:-}" ] || { echo "нет параметров подписи (VEIL_KEYSTORE...)"; exit 1; }
  ( cd mobile/android && "$GRADLE" --no-daemon assembleRelease )
  mkdir -p dist/android && cp mobile/android/app/build/outputs/apk/release/app-release.apk dist/android/veil-release.apk
  # через jar, а не apksigner.bat: .bat на Windows бывает заблокирован групповой политикой
  "$JAVA_HOME/bin/java" -jar "$ANDROID_HOME/build-tools/34.0.0/lib/apksigner.jar" verify --verbose dist/android/veil-release.apk 2>&1 | head -8 || true
  echo "APK: dist/android/veil-release.apk"
fi
