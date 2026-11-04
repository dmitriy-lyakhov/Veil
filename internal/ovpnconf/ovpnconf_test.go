package ovpnconf

import (
	"strings"
	"testing"
)

const good = `# comment
client
dev tun
proto tcp
remote 203.0.113.5 443
<ca>
-----BEGIN CERTIFICATE-----
AAA
-----END CERTIFICATE-----
</ca>
<tls-crypt>
x
</tls-crypt>
`

func TestParse(t *testing.T) {
	c, err := Parse(good)
	if err != nil {
		t.Fatal(err)
	}
	h, p, pr, err := c.Remote()
	if err != nil || h != "203.0.113.5" || p != 443 || pr != "tcp" {
		t.Fatal(h, p, pr, err)
	}
	if strings.Contains(c.Text, "# comment") {
		t.Error("комментарий должен быть убран")
	}
}

func TestRemoteForms(t *testing.T) {
	c, err := Parse("remote example.org 1194 udp4\n<ca>\nx\n</ca>\n")
	if err != nil {
		t.Fatal(err)
	}
	h, p, pr, _ := c.Remote()
	if h != "example.org" || p != 1194 || pr != "udp" || !strings.HasPrefix(c.Text, "client\n") {
		t.Fatal(h, p, pr, c.Text)
	}
}

func TestReject(t *testing.T) {
	for _, bad := range []string{
		good + "up C:\\evil.bat\n",
		good + "script-security 2\n",
		good + "plugin foo.dll\n",
		good + "management 127.0.0.1 7505\n",
		good + "auth-user-pass\n",
		good + "tls-auth ta.key 1\n",
		good + "ca ca.crt\n",
		good + "<up>\nx\n</up>\n",
		strings.Replace(good, "remote 203.0.113.5 443", "", 1),
		"remote a 99999\n<ca>\nx\n</ca>\n",
		strings.ReplaceAll(good, "</ca>", ""),
	} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("ожидалась ошибка для:\n%s", bad)
		}
	}
}

func TestInlineBlockNotChecked(t *testing.T) {
	// содержимое блока не разбирается как директивы
	if _, err := Parse(good + "<cert>\nup\n</cert>\n"); err != nil {
		t.Fatal(err)
	}
}

func TestDNS(t *testing.T) {
	c, _ := Parse(good)
	if !strings.Contains(c.WithDNS([]string{"1.1.1.1"}), "dhcp-option DNS 1.1.1.1\n") {
		t.Fatal("нет DNS")
	}
}
