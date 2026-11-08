# OpenVPN для Veil (PKI easy-rsa, tls-crypt, dual-stack). Контекст сборки — корень репозитория.
ARG ALPINE_VERSION=3.22
FROM alpine:${ALPINE_VERSION}
RUN apk add --no-cache bash openvpn easy-rsa openssl iproute2 iptables ip6tables curl ca-certificates tini \
 && ln -sf /usr/share/easy-rsa/easyrsa /usr/local/bin/easyrsa
COPY docker/common/net.sh /usr/local/lib/veil/net.sh
COPY --chmod=755 docker/openvpn/ovpn.sh /usr/local/bin/ovpn.sh
RUN ln -s ovpn.sh /usr/local/bin/client
VOLUME /etc/openvpn/veil
EXPOSE 1194/udp
ENTRYPOINT ["/sbin/tini", "--", "ovpn.sh"]
CMD ["start"]
