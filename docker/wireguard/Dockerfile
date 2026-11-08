# WireGuard для Veil: модуль ядра хоста, если он есть, иначе wireguard-go (той же версии, что в go.mod).
# Контекст сборки — корень репозитория. BIN_FROM=prebuilt — готовый bin/wireguard-go-linux-<arch>.
ARG GO_VERSION=1.27
ARG ALPINE_VERSION=3.22
ARG BIN_FROM=source

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS bin-source
WORKDIR /src
COPY go.mod go.sum ./
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags '-s -w' -o /out/wireguard-go golang.zx2c4.com/wireguard

FROM scratch AS bin-prebuilt
ARG TARGETARCH
COPY bin/wireguard-go-linux-${TARGETARCH} /out/wireguard-go

FROM bin-${BIN_FROM} AS bins

FROM alpine:${ALPINE_VERSION}
RUN apk add --no-cache bash wireguard-tools-wg wireguard-tools-wg-quick iproute2 iptables ip6tables \
        curl ca-certificates libqrencode-tools tini
COPY --from=bins --chmod=755 /out/wireguard-go /usr/local/bin/wireguard-go
COPY docker/common/net.sh /usr/local/lib/veil/net.sh
COPY --chmod=755 docker/wireguard/wg.sh /usr/local/bin/wg.sh
RUN ln -s wg.sh /usr/local/bin/client
ENV WG_FLAVOR=wg \
    WG_QUICK_USERSPACE_IMPLEMENTATION=wireguard-go \
    WG_I_PREFER_BUGGY_USERSPACE_TO_POLISHED_KMOD=1
VOLUME /etc/wireguard
EXPOSE 51820/udp
ENTRYPOINT ["/sbin/tini", "--", "wg.sh"]
CMD ["start"]
