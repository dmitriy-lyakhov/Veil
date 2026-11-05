package client

import (
	"testing"
	"time"
)

func TestDeprioritizeFailed(t *testing.T) {
	d := &Dialer{}
	ranked := []EntryRTT{{Name: "A", RTT: 5 * time.Millisecond}, {Name: "B", RTT: 20 * time.Millisecond}, {Name: "C", RTT: 40 * time.Millisecond}}
	if got := d.deprioritizeFailed(append([]EntryRTT(nil), ranked...)); got[0].Name != "A" {
		t.Fatalf("no failures: order must not change, got %v", got)
	}
	d.MarkFailed("A")
	got := d.deprioritizeFailed(append([]EntryRTT(nil), ranked...))
	if got[0].Name != "B" || got[1].Name != "C" || got[2].Name != "A" {
		t.Fatalf("recently failed entry must go last, order of others kept: %v", got)
	}
	d.MarkFailed("B")
	got = d.deprioritizeFailed(append([]EntryRTT(nil), ranked...))
	if got[0].Name != "C" || got[1].Name != "A" || got[2].Name != "B" {
		t.Fatalf("failed entries keep their relative RTT order: %v", got)
	}
	d.failMu.Lock()
	d.failed["A"] = time.Now().Add(-2 * failedCooldown)
	d.failMu.Unlock()
	if d.recentlyFailed("A") {
		t.Fatal("cooldown must expire")
	}
}
