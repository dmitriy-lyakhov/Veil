package e2e

// Живая проверка установщика в Docker-режиме против настоящего сервера по SSH.
// Сервер — чистый Linux без Docker, например test/env (Ubuntu + systemd + SSH):
//
//	docker build -t veil-testenv test/env
//	docker network create --opt com.docker.network.driver.mtu=1400 veiltest   # MTU ниже, чем у вложенных сетей
//	docker run -d --name veil-testenv --network veiltest --privileged --cgroupns=host \
//	    -v /sys/fs/cgroup:/sys/fs/cgroup:rw -v veil-testenv-docker:/var/lib/docker \
//	    -v veil-testenv-containerd:/var/lib/containerd -p 2222:22 veil-testenv
//	make server   # бинарники для сервера в dist/server
//	VEIL_DOCKER_E2E=127.0.0.1:2222 VEIL_DOCKER_E2E_PASSWORD=root-pass VEIL_SERVER_DIR=dist/server \
//	    go test ./test/e2e -run TestDockerDeploy -v -timeout 40m
//
// Проверяется: установка Docker, контейнеры veil и veil-<протокол>, повторная установка
// только добавляет клиента, управление через docker exec, удаление контейнеров.

import (
	"context"
	"net"
	"os"
	"strconv"
	"testing"

	"veil/internal/deploy"
)

func TestDockerDeploy(t *testing.T) {
	target := os.Getenv("VEIL_DOCKER_E2E")
	if target == "" {
		t.Skip("VEIL_DOCKER_E2E не задан — живая проверка установки в Docker пропущена")
	}
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portStr)
	user := os.Getenv("VEIL_DOCKER_E2E_USER")
	if user == "" {
		user = "root"
	}
	req := deploy.Request{Name: "e2e", IPv4: host, SSHPort: port, User: user,
		Password: os.Getenv("VEIL_DOCKER_E2E_PASSWORD"), ServerBinDir: os.Getenv("VEIL_SERVER_DIR"), SiteLang: "ru", AdminPanel: true}
	logf := func(f string, a ...any) { t.Logf(f, a...) }
	ctx := context.Background()
	steps := map[string]string{}
	req.Progress = func(s deploy.Step) { steps[s.ID] = s.State; t.Logf("шаг %s: %s — %s (%s)", s.ID, s.Title, s.Detail, s.State) }

	prof, err := deploy.Deploy(ctx, req, logf)
	if err != nil {
		t.Fatalf("Deploy veil: %v", err)
	}
	for _, id := range []string{"ssh", "system", "docker", "files", "build", "start", "profile"} {
		if steps[id] != "done" {
			t.Errorf("шаг %s: %q, ожидался done (%v)", id, steps[id], steps)
		}
	}
	if prof.AdminPassword == "" || prof.AuthKey == "" {
		t.Fatalf("неполный профиль veil: %+v", prof)
	}
	// повторная установка на тот же сервер — новое устройство, контейнер не пересоздаётся
	prof2, err := deploy.Deploy(ctx, req, logf)
	if err != nil {
		t.Fatalf("повторный Deploy veil: %v", err)
	}
	if prof2.AuthKey == prof.AuthKey {
		t.Fatal("повторный Deploy не выдал нового клиента")
	}

	for _, proto := range []string{"wireguard", "amneziawg", "openvpn", "ipsec"} {
		res, err := deploy.DeployProtocol(ctx, req, proto, 0, logf)
		if err != nil {
			t.Fatalf("DeployProtocol %s: %v", proto, err)
		}
		again, err := deploy.DeployProtocol(ctx, req, proto, 0, logf)
		if err != nil {
			t.Fatalf("повторный DeployProtocol %s: %v", proto, err)
		}
		switch proto {
		case "wireguard", "amneziawg":
			if res.WG == nil || again.WG == nil || res.WG.PublicKey != again.WG.PublicKey || res.WG.Address[0] == again.WG.Address[0] {
				t.Fatalf("%s: ожидались два клиента одного сервера: %+v / %+v", proto, res.WG, again.WG)
			}
			if len(res.WG.Address) < 2 {
				t.Errorf("%s: у клиента нет IPv6 в туннеле: %v", proto, res.WG.Address)
			}
		case "openvpn":
			if res.OVPN == nil || again.OVPN == nil {
				t.Fatalf("openvpn: нет конфигурации")
			}
		case "ipsec":
			if res.IPsec == nil || again.IPsec == nil || res.IPsec.Username == again.IPsec.Username {
				t.Fatalf("ipsec: ожидались два разных логина")
			}
		}
	}

	prof.SSH.KeyPath = ""
	s, err := deploy.Open(ctx, prof, req.Password, "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var st struct {
		Clients []struct{ Name string } `json:"clients"`
	}
	if err := s.Call("GET", "state", nil, &st); err != nil {
		t.Fatalf("управление veil через docker exec: %v", err)
	}
	if len(st.Clients) != 2 {
		t.Fatalf("клиентов veil: %d, ожидалось 2", len(st.Clients))
	}
	// связывание: сети за первым клиентом (шлюз сайта) и политика
	if err := s.Call("POST", "clients/subnets", map[string]any{"name": st.Clients[0].Name, "subnets": []string{"192.168.77.0/24"}}, nil); err != nil {
		t.Fatalf("clients/subnets: %v", err)
	}
	if err := s.Call("POST", "settings/links", map[string]bool{"client_to_client": false, "client_to_site": true, "site_to_site": true}, nil); err != nil {
		t.Fatalf("settings/links: %v", err)
	}
	var st2 struct {
		Server struct {
			Links map[string]bool `json:"links"`
		} `json:"server"`
		Clients []struct {
			Subnets []string `json:"subnets"`
		} `json:"clients"`
	}
	if err := s.Call("GET", "state", nil, &st2); err != nil {
		t.Fatal(err)
	}
	if st2.Server.Links["client_to_client"] || !st2.Server.Links["site_to_site"] || len(st2.Clients[0].Subnets) != 1 {
		t.Fatalf("связывание не сохранилось: %+v", st2)
	}
	// админ-панель в отдельном контейнере: включена при установке, выключается и включается из приложения
	if on, avail := s.AdminPanel(); !on || !avail {
		t.Fatalf("AdminPanel после установки: on=%v available=%v", on, avail)
	}
	if err := s.SetAdminPanel(false, logf); err != nil {
		t.Fatalf("выключить панель: %v", err)
	}
	if on, _ := s.AdminPanel(); on {
		t.Fatal("панель не выключилась")
	}
	if err := s.SetAdminPanel(true, logf); err != nil {
		t.Fatalf("включить панель: %v", err)
	}
	ctrs, err := s.Containers()
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, c := range ctrs {
		names[c.Name] = c.Running()
	}
	for _, n := range []string{"veil", "veil-admin", "veil-wireguard", "veil-amneziawg", "veil-openvpn", "veil-ipsec"} {
		if !names[n] {
			t.Errorf("контейнер %s не работает: %v", n, names)
		}
	}
	if logs, err := s.ContainerLogs("veil-admin", 20); err != nil || logs == "" {
		t.Errorf("журнал veil-admin: %q %v", logs, err)
	}
	if users, err := s.IPsecUsers(); err != nil || len(users) != 2 {
		t.Fatalf("IPsecUsers: %v %v", users, err)
	}
	if names, err := s.OpenVPNClients(); err != nil || len(names) != 2 {
		t.Fatalf("OpenVPNClients: %v %v", names, err)
	}
	if os.Getenv("VEIL_DOCKER_E2E_KEEP") == "" {
		for _, rm := range []func() error{s.RemoveWireGuard, s.RemoveAmneziaWG, s.RemoveOpenVPN, s.RemoveIPsec, s.Uninstall} {
			if err := rm(); err != nil {
				t.Errorf("удаление: %v", err)
			}
		}
	}
}
