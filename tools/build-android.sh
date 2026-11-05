#!/usr/bin/env bash
# Сборка Android-клиента Veil: Go-ядро (gomobile bind -> AAR) и APK (Gradle).
# Переменные окружения (значения по умолчанию — каталог ../android-tools рядом с репозиторием):
#   TOOLS         каталог с инструментами (jdk, sdk, gradle, gobin)
#   JAVA_HOME     JDK 17
#   ANDROID_HOME  Android SDK (platforms;android-34, build-tools;34.0.0, ndk;26.1.10909125)
#   GRADLE        gradle (или gradlew)
# Использование: tools/build-android.sh [aar|apk|all]   (по умолчанию all)
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
# gomobile для android/arm64 требует cgo с NDK-компилятором (его выбирает сам gomobile)

what="${1:-all}"
cd "$ROOT"

if [ "$what" = aar ] || [ "$what" = all ]; then
  command -v gomobile >/dev/null || go install golang.org/x/mobile/cmd/gomobile@latest golang.org/x/mobile/cmd/gobind@latest
  mkdir -p android/app/libs
  gomobile bind -v -target=android/arm64,android/amd64 -androidapi 26 -trimpath \
    -o android/app/libs/veilcore.aar ./mobile
fi

if [ "$what" = apk ] || [ "$what" = all ]; then
  ( cd android && "$GRADLE" --no-daemon assembleDebug )
  APK=android/app/build/outputs/apk/debug/app-debug.apk
  mkdir -p dist/android && cp "$APK" dist/android/veil-debug.apk
  echo "APK: dist/android/veil-debug.apk"
fi
