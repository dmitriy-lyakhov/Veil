package gui

// Демо-данные новых экранов (установка, «Сервер уже настроен», контейнеры) для проверки вёрстки
// в тестах и для снимков экрана (сборка с -tags uidemo). На сервер ничего не отправляют.

import (
	"net/netip"

	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/vpnconf/wgconf"
)

func demoInstallJob(proto, name string) installJob {
	return installJob{req: deploy.Request{Name: name, IPv4: "203.0.113.10", User: "root", SSHPort: 22, AdminPanel: proto == "veil"}, proto: proto, admin: proto == "veil"}
}

// demoInstallSteps — шаги установки Veil в Docker; до шага upto (не включая) — выполнены, upto — идёт.
func demoInstallSteps(upto int) []deploy.Step {
	all := []deploy.Step{
		{ID: "ssh", Title: "Подключение по SSH", Detail: "root@203.0.113.10, ключ хоста SHA256:ZZYW8f1kQp0", State: "done"},
		{ID: "system", Title: "Проверка системы", Detail: "Ubuntu 24.04 LTS, linux/amd64", State: "done"},
		{ID: "docker", Title: "Установка Docker", Detail: "get.docker.com → Docker 29.9.3, compose, buildx", State: "done"},
		{ID: "files", Title: "Файлы контейнеров", Detail: "/opt/veil: docker/, bin/veil-server-linux-amd64 (8317 КБ)", State: "done"},
		{ID: "build", Title: "Сборка образа", Detail: "veil/veil-server:latest", State: "done"},
		{ID: "start", Title: "Запуск контейнеров", Detail: "veil — порты 443/tcp, 443/udp, 80; veil-admin — без портов", State: "done"},
		{ID: "profile", Title: "Профиль сохранён", Detail: "Подключение по HTTP/3, IPv6", State: "done"},
	}
	var out []deploy.Step
	for i, s := range all {
		if i > upto {
			break
		}
		if i == upto {
			s.State = "active"
		}
		out = append(out, s)
	}
	return out
}

func demoWG() *wgconf.Config {
	return &wgconf.Config{
		PrivateKey: "yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=",
		Address:    []netip.Prefix{netip.MustParsePrefix("10.8.1.5/32"), netip.MustParsePrefix("fd85:8399:407a:1::5/128")},
		DNS:        []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("2606:4700:4700::1111")},
		MTU:        1380,
		PublicKey:  "6c7QxAoE1Xnq8b9mY2tq0p3jR6k5wZ4hN8sV1uL2dE0=",
		Endpoint:   "203.0.113.10:51820",
		AllowedIPs: []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0"), netip.MustParsePrefix("::/0")},
		Keepalive:  25,
	}
}

// demoExisting — профиль и итог DeployProtocol для экрана «Сервер уже настроен».
func demoExisting(name string) (*config.Profile, *deploy.ProtocolResult) {
	res := &deploy.ProtocolResult{SSH: &config.SSHInfo{Port: 22, User: "root"}, WG: demoWG(), Existing: true, Container: "veil-wireguard",
		Others: []deploy.ContainerInfo{
			{Name: "veil", Protocol: "veil", Image: "veil/veil-server:latest", State: "running", Status: "Up 2 days", Ports: "0.0.0.0:443->443/tcp, 0.0.0.0:443->443/udp, 0.0.0.0:80->80/tcp"},
			{Name: "veil-admin", Protocol: "admin", Image: "veil/veil-server:latest", State: "running", Status: "Up 2 days"},
			{Name: "veil-openvpn", Protocol: "openvpn", Image: "veil/openvpn:latest", State: "exited", Status: "Exited (0) 5 hours ago", Ports: "0.0.0.0:1194->1194/udp"},
		}}
	job := demoInstallJob("wireguard", name)
	return protocolProfile(job.req, "wireguard", res), res
}

// demoDocker — контейнеры на сервере для вкладки «Управление».
func demoDocker() *dockerInfo {
	return &dockerInfo{version: "29.9.3", adminOn: true, adminAvail: true, list: []deploy.ContainerInfo{
		{Name: "veil", Protocol: "veil", Image: "veil/veil-server:latest", State: "running", Status: "Up 2 days", Ports: "[::]:443->443/tcp, [::]:443->443/udp, 0.0.0.0:80->80/tcp"},
		{Name: "veil-admin", Protocol: "admin", Image: "veil/veil-server:latest", State: "running", Status: "Up 2 days"},
		{Name: "veil-wireguard", Protocol: "wireguard", Image: "veil/wireguard:latest", State: "running", Status: "Up 2 days", Ports: "0.0.0.0:51820->51820/udp"},
		{Name: "veil-openvpn", Protocol: "openvpn", Image: "veil/openvpn:latest", State: "restarting", Status: "Restarting (1) 5 seconds ago", Ports: "0.0.0.0:1194->1194/udp"},
	}}
}

const demoContainerLog = `level=INFO msg="quic listener started" udp=[::]:443
level=INFO msg="http redirect started" addr=[::]:80
level=INFO msg="veil server started" tcp=[::]:443 peers=6 iface=veil0
level=INFO msg="links changed" client_to_client=true client_to_site=true site_to_site=false
level=INFO msg="client connected" name=ноутбук-бухгалтерии transport=h3 remote=[2a03:6f02:0000:0000:0000:0000:0001:039b]:54493`

// demoInstallView — экран установки в состоянии state: "progress", "done" или "fail".
func (u *ui) demoInstallView(state string) *installView {
	v := u.newInstallView(nil, demoInstallJob("veil", "Амстердам"))
	upto := 3
	if state == "done" {
		upto = 6
	}
	for _, s := range demoInstallSteps(upto) {
		v.apply(s)
	}
	switch state {
	case "done":
		v.finishOK(func() {})
	case "fail":
		v.finishErr(errDemoInstall)
	default:
		v.toggleLog()
	}
	v.w.Show()
	return v
}

type demoErr string

func (e demoErr) Error() string { return string(e) }

const errDemoInstall = demoErr("сборка образа: docker build завершился с ошибкой: failed to solve: process \"/bin/sh -c apk add --no-cache ca-certificates\" did not complete successfully")
