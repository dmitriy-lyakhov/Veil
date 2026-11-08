package gui

import (
	"strings"
	"testing"

	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/server"
)

// screenPages — экраны установки, связей и контейнеров для матрицы вёрстки.
func screenPages() []uiPage {
	mgr := func(u *ui) *manager {
		for _, p := range u.profiles {
			if managed(p) {
				m := u.buildManager(p)
				m.state, m.loaded = demoState(p.Name), true
				m.callHook = fakeCall
				return m
			}
		}
		return nil
	}
	return []uiPage{
		{"deploy-form-wg", func(u *ui) { u.deployForm(1) }},
		{"deploy-form-ipsec", func(u *ui) { u.deployForm(4) }},
		{"install-progress", func(u *ui) { u.demoInstallView("progress") }},
		{"install-done", func(u *ui) { u.demoInstallView("done") }},
		{"install-fail", func(u *ui) { u.demoInstallView("fail") }},
		{"existing", func(u *ui) {
			name := "Амстердам"
			if p := u.current(); p != nil {
				name = p.Name
			}
			prof, res := demoExisting(name)
			job := demoInstallJob("wireguard", name)
			u.openExisting(nil, job, prof, res)
		}},
		{"client-subnets", func(u *ui) {
			if m := demoManager(u); m != nil {
				m.openClientSubnets(m.state.Clients[1], nil)
			}
		}},
		{"client-subnets-empty", func(u *ui) {
			if m := demoManager(u); m != nil {
				m.openClientSubnets(m.state.Clients[0], nil)
			}
		}},
		{"containers", func(u *ui) {
			if m := mgr(u); m != nil {
				m.docker = demoDocker()
				m.dockerBusy = map[string]bool{"veil-openvpn": true}
				m.show(tabManagement)
			}
		}},
		{"containers-systemd", func(u *ui) {
			if m := mgr(u); m != nil {
				m.docker = &dockerInfo{}
				m.show(tabManagement)
			}
		}},
		{"links-empty", func(u *ui) {
			if m := mgr(u); m != nil {
				m.state.Server.Links = config.Links{}
				m.state.Clients = []server.ClientInfo{{Name: "Ноутбук", Address4: "10.66.0.2/32"}}
				m.show(tabLinks)
			}
		}},
		{"container-log", func(u *ui) { u.showLogSheet(u.win, "veil", demoContainerLog) }},
	}
}

func TestCheckSubnet(t *testing.T) {
	m := &manager{state: demoState("x")}
	other := m.state.Clients[1].Name // у «Телефона» 192.168.1.0/24 и 10.20.0.0/16
	cases := []struct {
		in, want, err string
	}{
		{"192.168.50.0/24", "192.168.50.0/24", ""},
		{"192.168.50.7/24", "192.168.50.0/24", ""},
		{"172.16.5.1", "172.16.5.1/32", ""},
		{"2001:db8:5::/48", "2001:db8:5::/48", ""},
		{"абв", "", "подсеть"},
		{"10.0.0.0/40", "", "/32"},
		{"10.0.0.0/4", "", "широкая"},
		{"10.66.0.0/16", "", "туннеля"},
		{"127.0.0.0/8", "", "Служебные"},
		{"fe80::/64", "", "Служебные"},
		{"192.168.1.0/24", "", "клиента"},
		{"192.168.77.0/24", "", "Уже есть"},
	}
	for _, c := range cases {
		got, msg := m.checkSubnet(c.in, []string{"192.168.77.0/24"}, "Ноутбук")
		if c.err == "" {
			if msg != "" || got != c.want {
				t.Errorf("%s: got %q, %q; want %q", c.in, got, msg, c.want)
			}
			continue
		}
		if !strings.Contains(msg, c.err) {
			t.Errorf("%s: error %q, want containing %q", c.in, msg, c.err)
		}
	}
	// своя же подсеть клиента «Телефон» при его редактировании — не конфликт с другими клиентами
	if _, msg := m.checkSubnet("192.168.1.0/24", nil, other); msg != "" {
		t.Errorf("own subnet: %q", msg)
	}
}

func TestInstallStepsOrder(t *testing.T) {
	u := headlessUI(t, "ru", true, testProfiles("Амстердам"))
	v := u.newInstallView(nil, demoInstallJob("veil", "Амстердам"))
	// Veil службой systemd: сборки образа нет, Docker пропущен
	for _, s := range []deploy.Step{
		{ID: "ssh", State: "done"}, {ID: "system", State: "done"}, {ID: "docker", State: "skip"},
		{ID: "files", State: "active"}, {ID: "files", State: "done"}, {ID: "start", State: "active"},
	} {
		v.apply(s)
	}
	var ids []string
	for _, r := range v.rows {
		ids = append(ids, r.id+":"+r.state)
	}
	want := "ssh:done system:done docker:skip files:done start:active profile:wait"
	if got := strings.Join(ids, " "); got != want {
		t.Fatalf("steps = %s, want %s", got, want)
	}
	v.apply(deploy.Step{ID: "extra", State: "active"})
	v.finishOK(func() {})
	ids = nil
	for _, r := range v.rows {
		ids = append(ids, r.id+":"+r.state)
	}
	want = "ssh:done system:done docker:skip files:done start:done extra:done"
	if got := strings.Join(ids, " "); got != want {
		t.Fatalf("after finish = %s, want %s", got, want)
	}
	v.w.Close()
}
