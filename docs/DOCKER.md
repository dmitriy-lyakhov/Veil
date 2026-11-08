# Veil в Docker

Сервер Veil и обычные протоколы (WireGuard, AmneziaWG, OpenVPN, IPsec/IKEv2) запускаются в отдельных
контейнерах. Имена проектов compose и контейнеров начинаются с `veil`:

| Контейнер | Протокол | Порты | Данные (том) |
|---|---|---|---|
| `veil` | Veil: HTTP/3 + HTTP/2, сайт-прикрытие, редирект с 80 | 443/tcp, 443/udp, 80/tcp | `/etc/veil` |
| `veil-wireguard` | WireGuard (`veilwg`) | 51820/udp | `/etc/wireguard` |
| `veil-amneziawg` | AmneziaWG (`veilawg`, amneziawg-go) | 51821/udp | `/etc/amnezia/amneziawg` |
| `veil-openvpn` | OpenVPN (easy-rsa, tls-crypt) | 1194/udp | `/etc/openvpn/veil` |
| `veil-ipsec` | IPsec/IKEv2 (strongSwan, EAP или сертификаты) | 500/udp, 4500/udp | `/etc/swanctl` |

Есть два способа: приложение ставит всё само по SSH, или вы поднимаете контейнеры из репозитория.

## 1. Из приложения (рекомендуется)

«+» → «Свой сервер по SSH» → выберите **протокол** → адрес и вход SSH → «Установить и подключить».
Приложение по SSH:

1. проверяет, что сервер — Linux, и есть ли `docker`, `docker compose` и `docker buildx`;
2. если Docker нет, а дистрибутив поддерживается (Debian/Ubuntu и производные, Fedora, RHEL/CentOS/Rocky/Alma),
   ставит его официальным скриптом `get.docker.com`, при неудаче — пакетами дистрибутива; для Docker старше 27
   включает `ip6tables` в `/etc/docker/daemon.json` (если файла ещё нет) — нужен для IPv6;
3. если контейнер протокола уже есть — **только выдаёт нового клиента** и подключается;
4. иначе выкладывает Docker-файлы (встроены в приложение) и готовые бинарники в `/opt/veil`, собирает образ
   на сервере и запускает контейнер. Сеть контейнера получает MTU внешнего интерфейса сервера.

Где Docker поставить нельзя (другой дистрибутив, нет прав), протокол ставится как раньше — системной службой.
Уже установленные без Docker серверы обновляются как были: приложение не переносит их в контейнеры само.

Управление из приложения (клиенты, домен, маскировка, каскад, обновление, удаление) работает одинаково
в обоих режимах: для контейнеров команды выполняются через `docker exec`.

Раскладка на сервере:

```
/opt/veil/docker/<протокол>/compose.yml, Dockerfile, .env   файлы контейнера (.env пишет приложение)
/opt/veil/bin/                                                veil-server, wireguard-go, amneziawg-go
/etc/veil/                                                    конфиг Veil (каталог хоста, смонтирован в контейнер)
```

Вручную на сервере:

```bash
cd /opt/veil/docker/wireguard && docker compose logs -f
docker exec veil-wireguard client list
```

## 2. Из репозитория

Нужны Docker 27+ (IPv6 и NAT66 для сетей compose включены по умолчанию) и доступ к `/dev/net/tun` на хосте.

```bash
cp .env.example .env            # необязательно: адреса, порты, подсети
docker compose up -d --build    # все пять контейнеров
docker compose up -d --build veil openvpn   # только нужные
```

Каждый протокол — самостоятельный compose-файл, его можно поднять отдельно:

```bash
docker compose -f docker/wireguard/compose.yml up -d --build
```

Go-бинарники собираются из исходников внутри образа (`BIN_FROM=source`). Приложение на сервере собирает те же
образы из готовых бинарников (`BIN_FROM=prebuilt`, каталог `bin/`).

### Клиенты

В каждом контейнере есть команда `client`:

```bash
docker compose exec veil client add phone            # ссылка veil:// + QR в терминале, файл /etc/veil/clients/phone.veil
docker compose exec veil client add boss -role admin
docker compose exec veil client list
docker compose exec veil client delete phone

docker compose exec wireguard client add laptop      # .conf + QR; так же amneziawg
docker compose exec wireguard client show laptop
docker compose exec wireguard client remove laptop

docker compose exec openvpn client add laptop        # .ovpn
docker compose exec openvpn client remove laptop     # отзыв сертификата (CRL)

docker compose exec ipsec client add laptop          # логин и пароль EAP + команды для Windows
docker compose exec ipsec client add-cert laptop     # сертификат .p12
```

Скопировать файл: `docker compose cp wireguard:/etc/wireguard/clients/laptop.conf .`

При первом запуске каждый контейнер создаёт клиента `client` (имя меняется переменной `*_FIRST_CLIENT`,
пустое значение — не создавать). Пароль админ-панели Veil — в журнале первого запуска
(`docker compose logs veil`) и в файле `/etc/veil/admin-password` тома.

### Настройки (.env)

Все переменные с описанием — в [`.env.example`](../.env.example). Главные:

- `VEIL_PUBLIC_IPV4`, `VEIL_PUBLIC_IPV6` — адреса сервера для конфигов клиентов Veil. Пусто — определяются при
  первом запуске через api.ipify.org / api6.ipify.org (`VEIL_DETECT_IP=0` отключает обращение к ним).
  Изменённые адреса применяются при перезапуске и к уже существующему конфигу.
- `VEIL_HOST` — адрес (IP или домен) в конфигах WireGuard, AmneziaWG, OpenVPN и IPsec.
- `VEIL_FRONT`, `VEIL_DOMAIN`, `VEIL_SITE_LANG` — маскировка, домен, язык сайта (первый запуск; потом — из панели).
- порты и подсети протоколов: `WG_PORT`, `AWG_PORT`, `OVPN_PORT`/`OVPN_PROTO`, `*_SUBNET`, `IPSEC_POOL`.

## Админ-панель в отдельном контейнере

Веб-панель (`https://10.66.0.1/admin`, только через VPN) работает в своём контейнере `veil-admin` — тот же образ,
команда `veil-server admin`. Портов наружу у него нет:

- `veil` проверяет, что запрос пришёл из туннеля, и проксирует `/admin` в `veil-admin`, добавляя адрес клиента
  в туннеле и общий ключ (`/run/veil/admin.key`, общий том `veil-run`); без ключа `veil-admin` отвечает 404;
- `veil-admin` хранит сессии и блокировки входа, а всё, что касается сервера, спрашивает у `veil` через сокет
  управления `/run/veil/ctl.sock` — с правами веб-панели (менять режим панели из неё нельзя);
- панель выключена — `/admin` отвечает 404, как несуществующая страница.

Выбор при установке:

- **из приложения** — переключатель «Админ-панель» в форме «Свой сервер по SSH» (только для Veil); позже —
  страница сервера → «Управление» → «Админ-панель (veil-admin)». Приложение пишет в `/opt/veil/docker/veil/.env`
  `COMPOSE_PROFILES=admin` / `VEIL_ADMIN=container` (или пусто / `off`) и поднимает или удаляет контейнер;
- **из репозитория** — в `.env`: `COMPOSE_PROFILES=admin` и `VEIL_ADMIN=container` (по умолчанию в `.env.example`),
  без панели — `COMPOSE_PROFILES=` и `VEIL_ADMIN=off`.

Страница панели «Протоколы» управляет контейнерами `veil-*` (перезапуск, выдача клиентов WireGuard, AmneziaWG,
OpenVPN, IPsec) через Docker. Доступ к Docker равен правам root на хосте, поэтому по умолчанию его нет;
включается так: `VEIL_ADMIN_DOCKER_SOCK=/var/run/docker.sock`. Без него страница показывает состояние только
из приложения.

Журнал событий панели (входы, изменения клиентов, связей, каскада, настроек — кто, что и откуда) хранится в
`/etc/veil/events.jsonl` (последние 1000 записей).

## IPv6

- Каждая сеть compose создаётся с `enable_ipv6` и своей ULA-подсетью (`fd76:6569:6c00:N::/64`). Docker публикует
  порты и на `0.0.0.0`, и на `[::]`, поэтому клиенты подключаются к серверу и по IPv4, и по IPv6.
- Внутри туннелей — dual-stack: у Veil, как и без Docker, ULA `/64` с NAT66; WireGuard, AmneziaWG и OpenVPN
  получают в контейнере ULA `/64`, IPsec — пул `/112`; клиенты получают IPv6-адрес и маршрут `::/0`.
- Выход в интернет по IPv6 идёт через NAT66 контейнера и NAT66 Docker (`ip6tables`), поэтому у хоста должен быть
  глобальный IPv6. Docker 27+ включает `ip6tables` сам; для более старого добавьте в `/etc/docker/daemon.json`
  `{"ip6tables": true, "experimental": true}` (приложение делает это само, если файла нет).
- В контейнерах заданы `net.ipv6.conf.all.disable_ipv6=0` и `forwarding=1`; если на хосте IPv6 выключен
  полностью, протоколы работают только по IPv4.

## Требования к хосту и ограничения

- Linux с `/dev/net/tun`; контейнерам нужна возможность `NET_ADMIN`. OpenVZ/LXC-VPS без TUN не подойдут.
- WireGuard использует модуль ядра, если он есть (ядро 5.6+), иначе встроенный `wireguard-go`; AmneziaWG —
  `amneziawg-go`, модуль ядра не нужен.
- IPsec управляет политиками ядра (XFRM) из контейнера; через NAT Docker клиенты подключаются по NAT-T (UDP 4500).
- `net.core.rmem_max` не меняется изнутри контейнера: для максимальной скорости QUIC задайте на хосте
  `sysctl -w net.core.rmem_max=7500000 net.core.wmem_max=7500000`.
- Подсети OpenVPN (`10.9.0.0/24`) и AmneziaWG (`10.9.1.0/24`) не пересекаются; IPsec — `10.10.0.0/24`.

## Проверка установщика

Живая проверка всего сценария (чистый Ubuntu без Docker → установка Docker → контейнеры → повторная установка
добавляет клиентов → связывание сетей → удаление) — `test/e2e/docker_deploy_test.go`, инструкция в его заголовке.
