FROM ubuntu:24.04
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends systemd systemd-sysv openssh-server iproute2 iptables sudo curl ca-certificates && rm -rf /var/lib/apt/lists/*
RUN useradd -m -s /bin/bash deployer && echo 'deployer:deployer-pass' | chpasswd && usermod -aG sudo deployer \
 && echo 'root:root-pass' | chpasswd \
 && sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin yes/; s/^#\?PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config \
 && systemctl enable ssh
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
