package deploy

import (
	"strings"
	"testing"

	"veil/internal/vpnconf/wgconf"
)

func TestValidateAWGInstallDefaults(t *testing.T) {
	o, srv, err := ValidateAWGInstall(AWGInstall{Port: 39999})
	if err != nil || o.Subnet != "10.9.1.0/24" || srv != "10.9.1.1/24" {
		t.Fatalf("%+v %s %v", o, srv, err)
	}
	a := o.AWG
	if a.Jc != 4 || a.Jmin != 10 || a.Jmax != 50 || a.S1 == 0 || a.S2 == 0 || a.S1+56 == a.S2 || a.H1 == "" || a.Validate() != nil {
		t.Fatalf("параметры по умолчанию: %+v", a)
	}
	// явные значения не перезаписываются
	o, _, err = ValidateAWGInstall(AWGInstall{Port: 1, AWG: wgconf.AWG{Jc: 7, Jmin: 20, Jmax: 90, S1: 30, S2: 40, H1: "11111", H2: "22222", H3: "33333", H4: "44444"}})
	if err != nil || o.AWG.Jc != 7 || o.AWG.S2 != 40 || o.AWG.H4 != "44444" {
		t.Fatalf("%+v %v", o.AWG, err)
	}
}

func TestValidateAWGInstallErrors(t *testing.T) {
	bad := []AWGInstall{
		{Port: 0}, {Port: 1, Subnet: "10.0.0.0/8"}, {Port: 1, Subnet: "10.9.1.0/24; reboot"},
		{Port: 1, AWG: wgconf.AWG{Jc: 3, Jmin: 100, Jmax: 50}},
		{Port: 1, AWG: wgconf.AWG{S1: 40, S2: 96}},
		{Port: 1, AWG: wgconf.AWG{H1: "100", H2: "200"}},
		{Port: 1, AWG: wgconf.AWG{H1: "100", H2: "100", H3: "300", H4: "400"}},
		{Port: 1, AWG: wgconf.AWG{I: [5]string{"<b 0x00>"}}},
	}
	for _, b := range bad {
		if _, _, err := ValidateAWGInstall(b); err == nil {
			t.Errorf("%+v: ожидалась ошибка", b)
		}
	}
}

func TestAWGScript(t *testing.T) {
	o, srv, _ := ValidateAWGInstall(AWGInstall{Port: 49126, AWG: wgconf.AWG{Jc: 5, Jmin: 8, Jmax: 80, S1: 31, S2: 42, H1: "1000", H2: "2000", H3: "3000", H4: "4000"}})
	s := awgScript(o, srv)
	for _, want := range []string{
		"ppa:amnezia/ppa", "apt-get install -y -qq amneziawg", "modprobe amneziawg", "IFACE=veilawg",
		"/etc/amnezia/amneziawg/veilawg.conf", "ListenPort = 49126", "Address = 10.9.1.1/24", "-s 10.9.1.0/24",
		"Jc = 5\nJmin = 8\nJmax = 80\nS1 = 31\nS2 = 42\nH1 = 1000\nH2 = 2000\nH3 = 3000\nH4 = 4000\n",
		"awg-quick@$IFACE", "awg genkey", "awg show $IFACE public-key",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("в скрипте нет %q", want)
		}
	}
	for _, left := range []string{"@PORT@", "@OBFS@", "@SERVER@", "@SUBNET@", "@IFACE@", "@CONF@"} {
		if strings.Contains(s, left) {
			t.Errorf("осталась метка %s", left)
		}
	}
	if strings.Contains(s, "veilwg") {
		t.Error("скрипт AmneziaWG затрагивает veilwg")
	}
}

func TestDefaultAWGValid(t *testing.T) {
	for i := 0; i < 200; i++ {
		a := DefaultAWG()
		if err := a.Validate(); err != nil {
			t.Fatalf("%+v: %v", a, err)
		}
	}
}
