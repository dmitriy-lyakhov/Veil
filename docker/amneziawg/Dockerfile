# AmneziaWG для Veil: amneziawg-go (версия из go.mod) и amneziawg-tools (awg, awg-quick).
# Модуль ядра amneziawg не нужен; если он есть на хосте, awg-quick возьмёт его.
# Контекст сборки — корень репозитория. BIN_FROM=prebuilt — готовый bin/amneziawg-go-linux-<arch>.
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
    go build -trimpath -ldflags '-s -w' -o /out/amneziawg-go github.com/amnezia-vpn/amneziawg-go

FROM scratch AS bin-prebuilt
ARG TARGETARCH
COPY bin/amneziawg-go-linux-${TARGETARCH} /out/amneziawg-go

FROM bin-${BIN_FROM} AS bins

FROM alpine:${ALPINE_VERSION} AS tools
ARG AWG_TOOLS_REF=v1.0.20260618-2
RUN apk add --no-cache build-base git linux-headers bash
RUN git clone --depth 1 --branch "$AWG_TOOLS_REF" https://github.com/amnezia-vpn/amneziawg-tools /awg \
 && make -C /awg/src -j"$(nproc)" \
 && make -C /awg/src install DESTDIR=/out WITH_WGQUICK=yes WITH_BASHCOMPLETION=no WITH_SYSTEMDUNITS=no

FROM alpine:${ALPINE_VERSION}
RUN apk add --no-cache bash iproute2 iptables ip6tables curl ca-certificates libqrencode-tools tini
COPY --from=bins --chmod=755 /out/amneziawg-go /usr/local/bin/amneziawg-go
COPY --from=tools /out/usr/bin/awg /out/usr/bin/awg-quick /usr/local/bin/
COPY docker/common/net.sh /usr/local/lib/veil/net.sh
COPY --chmod=755 docker/wireguard/wg.sh /usr/local/bin/wg.sh
RUN ln -s wg.sh /usr/local/bin/client
ENV WG_FLAVOR=awg \
    WG_QUICK_USERSPACE_IMPLEMENTATION=amneziawg-go \
    WG_I_PREFER_BUGGY_USERSPACE_TO_POLISHED_KMOD=1
VOLUME /etc/amnezia/amneziawg
EXPOSE 51821/udp
ENTRYPOINT ["/sbin/tini", "--", "wg.sh"]
CMD ["start"]
