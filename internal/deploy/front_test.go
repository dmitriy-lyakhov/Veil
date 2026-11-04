package deploy

import (
	"testing"

	"veil/internal/config"
)

func TestApplyFrontKeepsKeyAcrossReinstalls(t *testing.T) {
	cfg := &config.Server{}
	if err := applyFront(cfg, ""); err != nil || cfg.Front != nil {
		t.Fatalf("empty choice must leave masking off: %v %+v", err, cfg.Front)
	}
	if err := applyFront(cfg, " HTTPS://YA.RU/ "); err != nil || cfg.Front == nil || cfg.Front.SNI != "ya.ru" || cfg.Front.Key == "" {
		t.Fatalf("enable: %v %+v", err, cfg.Front)
	}
	key := cfg.Front.Key
	if err := applyFront(cfg, "ya.ru"); err != nil || cfg.Front.Key != key {
		t.Fatalf("same site again changed the key: %v", err)
	}
	if err := applyFront(cfg, "vk.com"); err != nil || cfg.Front.SNI != "vk.com" || cfg.Front.Key != key {
		t.Fatalf("switching the site must keep the key so issued configs still work: %v %+v", err, cfg.Front)
	}
	if err := applyFront(cfg, ""); err != nil || cfg.Front == nil || cfg.Front.SNI != "vk.com" {
		t.Fatalf("an installer run without a choice must not silently turn masking off: %v %+v", err, cfg.Front)
	}
	if err := applyFront(cfg, "not a domain"); err == nil {
		t.Fatal("garbage accepted")
	}
}
