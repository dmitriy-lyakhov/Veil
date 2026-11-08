# Сервер Veil (HTTP/3 + HTTP/2 на 443, редирект с 80). Контекст сборки — корень репозитория.
#   BIN_FROM=source   (по умолчанию) — veil-server собирается из исходников репозитория;
#   BIN_FROM=prebuilt — берётся готовый bin/veil-server-linux-<arch> (так собирает образ приложение на сервере).
ARG GO_VERSION=1.27
ARG ALPINE_VERSION=3.22
ARG BIN_FROM=source

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS bin-source
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags '-s -w' -o /out/veil-server ./cmd/veil-server

FROM scratch AS bin-prebuilt
ARG TARGETARCH
COPY bin/veil-server-linux-${TARGETARCH} /out/veil-server

FROM bin-${BIN_FROM} AS bins

FROM alpine:${ALPINE_VERSION}
RUN apk add --no-cache iproute2 iptables ip6tables nftables curl ca-certificates tini
COPY --from=bins --chmod=755 /out/veil-server /usr/local/bin/veil-server
COPY docker/common/net.sh /usr/local/lib/veil/net.sh
COPY --chmod=755 docker/veil/entrypoint.sh /usr/local/bin/veil-entrypoint
# «client add NAME» — одинаковая команда во всех контейнерах
RUN printf '#!/bin/sh\nexec veil-server client "$@"\n' > /usr/local/bin/client && chmod 755 /usr/local/bin/client
VOLUME /etc/veil
EXPOSE 80/tcp 443/tcp 443/udp
STOPSIGNAL SIGTERM
ENTRYPOINT ["/sbin/tini", "--", "veil-entrypoint"]
