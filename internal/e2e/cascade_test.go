package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"veil/internal/client"
	"veil/internal/server"
)

func buildChain(t *testing.T, n int) ([]*env, []string) {
	t.Helper()
	nodes := make([]*env, n)
	names := make([]string, n)
	for i := range nodes {
		nodes[i] = startServer(t, "tcp4")
		names[i] = fmt.Sprintf("node%d", i)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for i := 0; i+1 < n; i++ {
		link, err := nodes[i+1].srv.IssueRelay(names[i])
		if err != nil {
			t.Fatal(err)
		}
		if err := nodes[i].srv.AddUpstream(ctx, names[i+1], link); err != nil {
			t.Fatalf("link %d→%d: %v", i, i+1, err)
		}
	}
	return nodes, names
}

func TestCascade(t *testing.T) {
	for _, hops := range []int{2, 5, 15} {
		t.Run(fmt.Sprintf("%d_nodes", hops), func(t *testing.T) { testCascade(t, hops) })
	}
}

func testCascade(t *testing.T, hops int) {
	nodes, names := buildChain(t, hops)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if hops > 3 {
		options, err := nodes[0].srv.Explore(ctx, names[1:3])
		if err != nil || len(options) != 1 || options[0] != names[3] {
			t.Fatalf("explore from node0 via node1,node2: %v %v", options, err)
		}
	}

	prof, err := nodes[0].srv.AddClient(ctx, "laptop-cascade", "через всю цепочку", names[1:])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(prof.Via, "→") != hops-1 {
		t.Fatalf("via = %q", prof.Via)
	}
	prof.IPv6 = ""
	start := time.Now()
	tn, tnet := startClient(t, prof)
	exit := nodes[hops-1]
	got := fetchInside(t, tnet, mustAddr(exit.srvCfg.Address4))
	if !strings.Contains(got, exit.srvCfg.Site.Company) {
		t.Fatalf("traffic did not leave via the last node: %q (want %s)", got, exit.srvCfg.Site.Company)
	}
	fetchBig(t, tnet, mustAddr(exit.srvCfg.Address6))
	t.Logf("%d-node cascade up in %v, transport %s", hops, time.Since(start).Round(time.Millisecond), tn.Status().Transport)

	for i, n := range nodes {
		st := n.srv.State()
		found := false
		for _, c := range st.Clients {
			if strings.HasSuffix(c.Name, "laptop-cascade") {
				found = true
				if i == hops-1 && (c.RxBytes == 0 || len(c.Route) != 0) {
					t.Fatalf("exit node stats: %+v", c)
				}
				if i < hops-1 && c.RxBytes == 0 {
					t.Fatalf("node%d relayed nothing: %+v", i, c)
				}
			}
		}
		if !found {
			t.Fatalf("node%d has no record of the client", i)
		}
	}

	if err := nodes[0].srv.RemoveClient(ctx, "laptop-cascade"); err != nil {
		t.Fatal(err)
	}
	for i, n := range nodes {
		for _, c := range n.srv.State().Clients {
			if strings.HasSuffix(c.Name, "laptop-cascade") {
				t.Fatalf("node%d still has %s after removal", i, c.Name)
			}
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && tn.Status().State == client.StateConnected {
		time.Sleep(50 * time.Millisecond)
	}
	if tn.Status().State == client.StateConnected {
		t.Fatal("removed cascade client still connected")
	}
}

func TestBlockUnblockAndRename(t *testing.T) {
	e := startServer(t, "tcp4")
	ctx := context.Background()
	prof, err := e.srv.AddClient(ctx, "tablet", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	prof.Port = e.port
	tn, tnet := startClient(t, prof)
	fetchInside(t, tnet, mustAddr(e.srvCfg.Address4))

	yes, no := true, false
	if err := e.srv.UpdateClient("tablet", serverUpdate("", nil, &yes)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && tn.Status().State == client.StateConnected {
		time.Sleep(50 * time.Millisecond)
	}
	if tn.Status().State == client.StateConnected {
		t.Fatal("blocked client still connected")
	}
	d := &client.Dialer{Profile: prof}
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := d.Connect(cctx); err == nil {
		t.Fatal("blocked client authenticated")
	}

	note := "планшет в офисе"
	if err := e.srv.UpdateClient("tablet", serverUpdate("office-tablet", &note, &no)); err != nil {
		t.Fatal(err)
	}
	tn2, tnet2 := startClient(t, prof)
	if !strings.HasPrefix(fetchInside(t, tnet2, mustAddr(e.srvCfg.Address4)), insideBody) {
		t.Fatal("unblocked client cannot pass traffic")
	}
	var found bool
	for _, c := range e.srv.State().Clients {
		if c.Name == "office-tablet" && c.Note == note && !c.Disabled && c.Online {
			found = true
		}
	}
	if !found {
		t.Fatalf("renamed client not in state: %+v", e.srv.State().Clients)
	}
	_ = tn2
}

func TestAdminPasswordChange(t *testing.T) {
	e := startServer(t, "tcp4")
	if err := e.srv.SetAdminPassword(nil, "first-password"); err != nil {
		t.Fatal(err)
	}
	wrong := "nope"
	if err := e.srv.SetAdminPassword(&wrong, "second-password"); err == nil {
		t.Fatal("password changed without the correct current password")
	}
	cur := "first-password"
	if err := e.srv.SetAdminPassword(&cur, "short"); err == nil {
		t.Fatal("short password accepted")
	}
	if err := e.srv.SetAdminPassword(&cur, "second-password"); err != nil {
		t.Fatal(err)
	}
}

func serverUpdate(newName string, note *string, disabled *bool) server.ClientUpdate {
	return server.ClientUpdate{NewName: newName, Note: note, Disabled: disabled}
}
