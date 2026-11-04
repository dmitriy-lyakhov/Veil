package settings

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	s, err := Load(p)
	if err != nil || s.KillSwitch {
		t.Fatalf("%v %+v", err, s)
	}
	s.DNS = []string{"1.1.1.1", "bad", "2606:4700:4700::1111"}
	s.KillSwitch = true
	if err := s.Save(p); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil || !got.KillSwitch || len(got.DNSAddrs()) != 2 {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestLogExpired(t *testing.T) {
	now := time.Now()
	s := &Settings{LogEnabled: true, LogSince: now.Add(-15 * 24 * time.Hour)}
	if !s.LogExpired(now) {
		t.Fatal("должно быть просрочено")
	}
	s.LogSince = now.Add(-time.Hour)
	if s.LogExpired(now) {
		t.Fatal("рано")
	}
	if (&Settings{}).LogExpired(now) {
		t.Fatal("выключено")
	}
}
