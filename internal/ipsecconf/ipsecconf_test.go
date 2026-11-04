package ipsecconf

import (
	"strings"
	"testing"
)

func TestValidateAndInstructions(t *testing.T) {
	c := &Config{Server: "203.0.113.5", CAPEM: "-----BEGIN CERTIFICATE-----\nAA\n-----END CERTIFICATE-----", Auth: AuthEAP, Username: "bob", Password: "pw"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	txt := c.Instructions()
	for _, want := range []string{"203.0.113.5", "Логин: bob", "Пароль: pw", "Windows", "Android", "iOS", "IKEv2"} {
		if !strings.Contains(txt, want) {
			t.Errorf("нет %q", want)
		}
	}
	bad := *c
	bad.Password = ""
	if bad.Validate() == nil {
		t.Error("пустой пароль должен быть ошибкой")
	}
	bad = *c
	bad.CAPEM = ""
	if bad.Validate() == nil {
		t.Error("без сертификата центра должна быть ошибка")
	}
	cert := &Config{Server: "x", CAPEM: c.CAPEM, Auth: AuthCert, P12: "AA", P12Pass: "pp"}
	if err := cert.Validate(); err != nil || !strings.Contains(cert.Instructions(), ".p12") {
		t.Fatal(err)
	}
}
