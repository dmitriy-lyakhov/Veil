# Android-клиент Veil

Минимальный клиент протокола **Veil** (WireGuard внутри HTTPS) для Android 8.0+ (minSdk 26, targetSdk 34, `net.veil.android`).

## Возможности

- вставка ссылки `veil://…` (поле, кнопка «Вставить из буфера», открытие ссылки `veil://` из других приложений);
- подключение/отключение через `VpnService` (foreground-сервис с уведомлением и кнопкой «Отключить»);
- статус (подключение / подключено после рукопожатия WireGuard / переподключение), имя профиля, вход и транспорт, счётчики трафика;
- каскад входов, `FrontSNI`/`FrontKey` и смена входа — как в десктопном клиенте (используется тот же код `internal/client`);
- последний профиль хранится в SharedPreferences (без шифрования; ссылка и ключи в журнал не пишутся).

## Структура

| Путь | Что это |
|---|---|
| `mobile/` | Go-ядро (`package veilcore`) для `gomobile bind`: `Parse(link)`, `Start(link, tunFd, protector)`, `Stop()`, `Status()` |
| `internal/netpin` | хук `SetProtect(func(fd int) bool)`; в `Control` (Linux/Android) вызывается первым и затем не мешает `SO_BINDTODEVICE` |
| `android/` | Gradle-проект (Kotlin, AGP 8.3, без AndroidX): `MainActivity`, `VeilVpnService`, `Store` |
| `tools/build-android.sh`, `tools/build-android.ps1` | сборка AAR + APK |
| `tools/androidtestserver/` | локальный тестовый сервер Veil для проверки (ссылка пишется в файл) |

Как это работает: `VeilVpnService` вызывает `Veilcore.parse()` (адреса туннеля, DNS, MTU), настраивает `VpnService.Builder` (маршруты `0.0.0.0/0` и `::/0`), получает fd через `establish()` → `detachFd()` и передаёт его Go. Go создаёт TUN через `tun.CreateUnmonitoredTUNFromFD` (без netlink — он закрыт на Android 11+) и запускает `client.Start`. Сокеты клиента (TCP/TLS к входу, UDP для HTTP/3) защищаются от петли через `VpnService.protect(fd)` (интерфейс `Protector`). Адреса входов в профиле — IP, поэтому DNS-запросы самим клиентом не выполняются.

## Сборка

Нужны: JDK 17, Android SDK (`platforms;android-34`, `build-tools;34.0.0`, `ndk;26.1.10909125`), Gradle 8.7, Go (версия из `go.mod`), `gomobile`/`gobind` (`go install golang.org/x/mobile/cmd/gomobile@latest golang.org/x/mobile/cmd/gobind@latest`). Скрипты по умолчанию ищут их в `..\android-tools` рядом с репозиторием (`TOOLS`, `JAVA_HOME`, `ANDROID_HOME`, `GRADLE` можно переопределить).

```
tools/build-android.sh [aar|apk|all]        # Git Bash / Linux / macOS
tools\build-android.ps1 -What all           # PowerShell
```

Эквивалент вручную:

```
gomobile bind -target=android/arm64,android/amd64 -androidapi 26 -trimpath -o android/app/libs/veilcore.aar ./mobile
cd android && gradle assembleDebug          # android/app/build/outputs/apk/debug/app-debug.apk
```

Скрипт копирует APK в `dist/android/veil-debug.apk`. Подпись — отладочный keystore; release-сборка не настроена. `*.aar` и каталоги сборки в `.gitignore`.

Для `gomobile bind` в `go.mod` добавлена зависимость `golang.org/x/mobile` (версия, уже входившая в граф модулей).

Установка: `adb install -r dist/android/veil-debug.apk`. При первом подключении Android покажет разовый диалог «Запрос на подключение» — нужно нажать «ОК».

Замечание для Windows: если Gradle падает с `Unable to establish loopback connection`, укажите короткий `TEMP`/`TMP` (скрипты делают это сами, каталог `$TOOLS/tmp`).

## Проверка без настоящего сервера

```
go run ./tools/androidtestserver -port 18443 -link link.txt
adb reverse tcp:18443 tcp:18443
adb shell am start -a android.intent.action.VIEW -d "$(cat link.txt)" net.veil.android
```

Внутри туннеля тестовый сервер отвечает ICMP и HTTP на `10.66.0.1:80` (`/big` — 8 МБ): `adb shell ping 10.66.0.1`.

## Ограничения

- только протокол Veil (профили WireGuard/OpenVPN/IPsec отклоняются);
- нет QR-сканера (ссылка — вставкой или через intent `veil://`);
- нет раздельного туннелирования: весь трафик (IPv4 и IPv6) идёт в туннель;
- kill switch — только системный «Always-on VPN» + «Блокировать соединения без VPN» в настройках Android;
- нет автозапуска при загрузке, нескольких профилей, настройки DNS и выбора входа в интерфейсе;
- хранение ссылки в SharedPreferences без шифрования;
- ABI: arm64-v8a и x86_64; APK ~50 МБ (отладочный, ядро Go);
- Android 13+: для уведомления запрашивается POST_NOTIFICATIONS; на Android 14 сервис объявлен как `specialUse`.
