package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"veil/internal/client"
	"veil/internal/config"
	"veil/internal/wgkey"
)

func TestNearestEntryAndReselect(t *testing.T) {
	m := buildMesh(t, []string{"E1", "E2", "X"}, "X")
	exit := m.nodes["X"]
	priv, pub, err := wgkey.Generate()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	base := &config.Profile{Name: "multi", PrivateKey: priv, ExitName: "X"}
	entryProxy := map[string]*delayProxy{}
	for _, n := range []string{"E1", "E2"} {
		prof, err := m.nodes[n].srv.AddClientWithKey(ctx, "phone", "", []string{"X"}, pub)
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		if prof.PrivateKey != "" {
			t.Fatal("server returned a private key for a client-supplied public key")
		}
		if base.Address4 != "" && base.Address4 != prof.Address4 {
			t.Fatalf("entries disagree on tunnel address: %s vs %s", base.Address4, prof.Address4)
		}
		base.ServerPublicKey, base.Address4, base.Address6, base.MTU = prof.ServerPublicKey, prof.Address4, prof.Address6, prof.MTU
		px := newDelayProxy(t, fmt.Sprintf("127.0.0.1:%d", m.nodes[n].port))
		entryProxy[n] = px
		base.Entries = append(base.Entries, config.Entry{Name: n, IPv4: "127.0.0.1", Port: px.port(), Path: prof.Path, CertPin: prof.CertPin, AuthKey: prof.AuthKey, Transports: []string{"h2"}})
	}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	link, err := config.EncodeLink(base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.DecodeLink(link); err != nil {
		t.Fatalf("multi-entry link round trip: %v", err)
	}
	exitShares := 0
	for _, c := range exit.srv.State().Clients {
		if strings.HasSuffix(c.Name, "phone") {
			exitShares++
		}
	}
	if exitShares != 2 {
		t.Fatalf("exit should hold two access paths for one WireGuard peer, has %d", exitShares)
	}

	entryProxy["E1"].set(40)
	entryProxy["E2"].set(1)
	tn, tnet := startClient(t, base)
	if st := tn.Status(); st.Entry != "E2" || st.Exit != "X" {
		t.Fatalf("want nearest entry E2 and exit X, got %+v", st)
	}
	if got := fetchInside(t, tnet, mustAddr(exit.srvCfg.Address4)); !strings.Contains(got, exit.srvCfg.Site.Company) {
		t.Fatalf("not exiting via X: %q", got)
	}

	entryProxy["E1"].set(1)
	entryProxy["E2"].set(40)
	tn.Reselect()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && tn.Status().Entry != "E1" {
		time.Sleep(50 * time.Millisecond)
	}
	if st := tn.Status(); st.Entry != "E1" {
		t.Fatalf("reselect did not move to the nearest entry: %+v", st)
	}
	if got := fetchInside(t, tnet, mustAddr(exit.srvCfg.Address4)); !strings.Contains(got, exit.srvCfg.Site.Company) {
		t.Fatalf("after reselect: %q", got)
	}
	t.Logf("entry switched to %s (%.1f ms), exit still %s", tn.Status().Entry, tn.Status().EntryRTT, tn.Status().Exit)

	entryProxy["E1"].dead.Store(true)
	entryProxy["E1"].cut()
	deadline = time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && !(tn.Status().Entry == "E2" && tn.Status().State == client.StateConnected) {
		time.Sleep(100 * time.Millisecond)
	}
	if st := tn.Status(); st.Entry != "E2" || st.State != client.StateConnected {
		t.Fatalf("did not fail over to E2: %+v", st)
	}
	if got := fetchInside(t, tnet, mustAddr(exit.srvCfg.Address4)); !strings.Contains(got, exit.srvCfg.Site.Company) {
		t.Fatalf("after failover: %q", got)
	}
}
