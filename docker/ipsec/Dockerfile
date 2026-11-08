# IPsec/IKEv2 для Veil: strongSwan (charon + swanctl) на Debian — те же пакеты и плагины EAP,
# что ставит установщик по SSH. Контекст сборки — корень репозитория.
ARG DEBIAN_VERSION=trixie
FROM debian:${DEBIAN_VERSION}-slim
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      strongswan-charon strongswan-swanctl strongswan-pki \
      libcharon-extra-plugins libcharon-extauth-plugins libstrongswan-extra-plugins \
      iproute2 iptables openssl curl ca-certificates tini \
 && rm -rf /var/lib/apt/lists/*
# журнал charon — в stdout контейнера, а не в syslog
RUN printf 'charon {\n  filelog {\n    stdout {\n      default = 1\n      ike = 1\n      time_format = %%H:%%M:%%S\n    }\n  }\n  syslog {\n    daemon {\n      default = -1\n    }\n  }\n}\n' \
      > /etc/strongswan.d/zz-veil-docker.conf
COPY docker/common/net.sh /usr/local/lib/veil/net.sh
COPY --chmod=755 docker/ipsec/ipsec.sh /usr/local/bin/ipsec.sh
RUN ln -s ipsec.sh /usr/local/bin/client
VOLUME /etc/swanctl
EXPOSE 500/udp 4500/udp
ENTRYPOINT ["/usr/bin/tini", "--", "ipsec.sh"]
CMD ["start"]
