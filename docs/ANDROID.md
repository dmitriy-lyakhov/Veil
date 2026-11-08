# Android-клиент Veil

Клиент для Android 8.0+ (minSdk 26, targetSdk 34, `net.veil.android`, версия 1.1.0). Протоколы: **Veil** (WireGuard внутри HTTPS),
**WireGuard** и **AmneziaWG**. Интерфейс по дизайну Veil: кольцо подключения, страницы вместо диалогов, тёмная и светлая тема, ru/en.

## Возможности

- Подключение через `VpnService` (foreground-сервис с уведомлением и кнопкой «Отключить»); статус: подключение / подключено после рукопожатия /
  переподключение / ошибка; транспорт и вход каскада, счётчики трафика (сеанс, день, месяц — только на устройстве), график последних 60 с.
- Добавление сервера: ссылка `veil://` (в том числе из других приложений), вставка из буфера, текст `.conf` WireGuard/AmneziaWG,
  JSON-профиль, QR (камера, разрешение запрашивается при первом запуске), файл `.conf`/`.veil` через системный выбор файла.
- Несколько профилей: список, смена активного из шторки на главной, переименование, удаление с подтверждением.
- Хранение: профили шифруются AES-256-GCM, ключ в Android Keystore (`files/profiles.bin`, без открытого текста); старое хранилище
  `shared_prefs/veil.xml` при первом запуске переносится и очищается. Резервная копия — «Настройки → Серверы → Экспорт/Импорт»
  (**файл содержит ссылки и ключи открытым текстом**). Ключ Keystore не переживает удаление приложения.
- Раздельное туннелирование («Раздельный доступ»): по приложениям (`addAllowedApplication` / `addDisallowedApplication`; режимы «через VPN» и
  «напрямую», свой пакет всегда вне туннеля) и по сайтам/адресам (домены разрешаются до подъёма туннеля; «через VPN» — только выбранные
  маршруты и DNS; «напрямую» — на Android 13+ `excludeRoute`, ниже — дополнение маршрутов, считаемое в Go).
- Доступ (вкладка «Доступ», только профиль с ролью «Администратор» и поднятый туннель): список клиентов сервера с ролями, трафиком и состоянием,
  создание клиента (имя, заметка, роль «Пользователь» / «Администратор») и выдача QR, ссылки или файла `.veil`, смена роли, блокировка и
  разблокировка, новый конфиг, имя и заметка, удаление. Работает через админ-API сервера (`/admin/api/…`) по туннелю: Go-ядро (`Veilcore.admin`)
  проверяет отпечаток сертификата профиля и шлёт запрос обычным сокетом, сервер узнаёт администратора по адресу в туннеле (пароль не нужен).
  Для профилей Veil приложение не исключается из VPN (сокеты ядра защищены `protect`); профилям без роли администратора вкладка показывает пояснение.
  «Трафик» вынесен в «Настройки → Трафик» и открывается по нажатию на строку трафика на главной.
- Автозапуск: «Автоподключение при запуске приложения», «Подключаться при включении устройства» (`BOOT_COMPLETED`), восстановление VPN после
  обновления пакета (`MY_PACKAGE_REPLACED`), перезапуск сервиса системой (`START_STICKY`) и запуск системным Always-on VPN.
- Настройки: DNS, KillSwitch (подсказка по системному Always-on VPN и «Блокировать соединения без VPN»), тема, язык, «Уменьшить движение»
  (также учитывается системное «убрать анимацию»), журналы, «О Veil». Работает только в портретной ориентации (экран не поворачивается).

## Структура

| Путь | Что это |
|---|---|
| `mobile/core/` | Go-ядро (`package veilcore`) для `gomobile bind`: `Import`, `Rename`, `Entries`, `Parse`, `Plan`, `SetDNS`, `Start`, `Stop`, `Running`, `Status`, `Reselect`, `Admin` (админ-API, `admin.go`) |
| `internal/client/engine.go` | `StartEngine`: запуск движка по протоколу профиля (Veil, WireGuard, AmneziaWG) на готовом TUN |
| `internal/transport/netpin` | хук `SetProtect(func(fd int) bool)`: сокеты клиента защищаются от петли через `VpnService.protect` |
| `mobile/android/` | Gradle-проект (Kotlin, без Compose): `MainActivity`, `Pages.kt` (экраны), `Access.kt` (вкладка «Доступ»), `Drawer.kt` (выдвижная панель серверов), `RingView.kt` (кольцо), `Ui.kt`, `Data.kt` (профили, настройки, статистика), `VeilVpnService`, `BootReceiver` |
| `scripts/build-android.sh`, `.ps1` | сборка AAR, debug- и release-APK |
| `tools/androidtestserver/` | локальный тестовый сервер Veil (ссылка пишется в файл) |

Как это работает: `VeilVpnService` вызывает `Veilcore.parse()` (адреса туннеля, DNS, MTU, маршруты), настраивает `VpnService.Builder`
(приложения, маршруты), получает fd через `establish()` → `detachFd()` и отдаёт его Go. Go создаёт TUN без netlink (он закрыт на Android 11+)
и запускает движок. Адреса входов в профиле — IP, поэтому DNS-запросы самим клиентом не выполняются.

## Сборка

Нужны: JDK 17, Android SDK (`platforms;android-34`, `build-tools;34.0.0`, `ndk;26.1.10909125`), Gradle 8.7, Go (версия из `go.mod`),
`gomobile`/`gobind`. Скрипты по умолчанию ищут их в `..\android-tools` рядом с репозиторием (`TOOLS`, `JAVA_HOME`, `ANDROID_HOME`, `GRADLE` можно переопределить).

```
scripts/build-android.sh aar        # Go-ядро -> mobile/android/app/libs/veilcore.aar
scripts/build-android.sh apk        # debug-APK -> dist/android/veil-debug.apk
scripts/build-android.sh release    # R8 + подпись -> dist/android/veil-release.apk, проверка подписи через apksigner.jar
scripts/build-android.sh all        # aar + debug-APK
```

`build-android.ps1 -What all` — то же для PowerShell (без release).

**Release** подписывается ключом из `mobile/android/signing/veil-release.jks` (в репозиторий не коммитится). Параметры — `VEIL_KEYSTORE`,
`VEIL_STORE_PASSWORD`, `VEIL_KEY_ALIAS`, `VEIL_KEY_PASSWORD` либо файл `$TOOLS/signing-secrets.txt` (`KEY=VALUE`, вне репозитория).
Подпись v2 + v3; APK не отладочный, ~54 МБ (arm64-v8a и x86_64, Go-ядро). Правила R8 — `mobile/android/app/proguard-rules.pro` (gomobile/JNI и zxing).

Установка: `adb install -r dist/android/veil-release.apk`. **Debug и release подписаны разными ключами**: чтобы поставить release поверх debug,
сначала сделайте резервную копию серверов, затем `adb uninstall net.veil.android` (профили и ключ Keystore при удалении теряются) и импортируйте копию.
При первом подключении Android покажет разовый запрос VPN — нужно нажать «ОК».

Замечания для Windows:
- если Gradle падает с `Unable to establish loopback connection`, укажите короткий `TEMP`/`TMP` (скрипты делают это сами, каталог `$TOOLS/tmp`);
- `apksigner.bat` может блокироваться групповой политикой — скрипт запускает `apksigner.jar` напрямую.

## Проверка без настоящего сервера

```
go run ./tools/androidtestserver -port 18443 -link link.txt   # флаг -admin: клиент получает роль администратора (вкладка «Доступ»)
adb reverse tcp:18443 tcp:18443
adb shell am start -a android.intent.action.VIEW -d "'$(cat link.txt)'" net.veil.android
```

Внутри туннеля тестовый сервер отвечает ICMP и HTTP на `10.66.0.1:80` (`/big` — 8 МБ): `adb shell ping 10.66.0.1`. `adb reverse` пробрасывает только TCP,
поэтому проверить рукопожатие WireGuard/AmneziaWG (UDP) так нельзя — нужен сервер, доступный из сети телефона.

Полезные проверки на устройстве: маршруты — `adb shell ip route show table all | grep tun0`, правила по приложениям — `adb shell ip rule`.
В debug-сборке настройки можно задать через `run-as net.veil.android` (`shared_prefs/settings.xml`). `uiautomator dump` в приложении
не работает (`could not get idle state`), поэтому интерфейс проверяют по координатам и снимкам (`adb exec-out screencap -p`).

## Ограничения

- OpenVPN и IPsec не поддерживаются: им нужны внешний процесс `openvpn` и модуль ядра/служба strongSwan, которых нет в приложении; при импорте
  показывается сообщение (ru/en);
- kill switch — только системный «Always-on VPN» + «Блокировать соединения без VPN» в настройках Android (приложение подсказывает, как включить);
- не проверено на устройстве: рукопожатие WireGuard/AmneziaWG, `BOOT_COMPLETED` (нужна перезагрузка), системный Always-on VPN, перезапуск сервиса
  после гибели процесса системой, смена Wi-Fi → мобильная сеть, Android 13/14, распознавание реального QR камерой;
- при каскадных входах и в профилях с `FrontSNI`/`FrontKey` поведение как в десктопном клиенте (общий код `internal/client`), но на телефоне
  с реальными каскадами не проверялось;
- Android 14: сервис объявлен как `specialUse`; Android 13+: запрашивается `POST_NOTIFICATIONS`.
