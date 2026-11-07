package gui

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"veil/internal/config"
	"veil/internal/server"
)

func entriesNamed(names ...string) []*config.Profile {
	var out []*config.Profile
	for _, n := range names {
		out = append(out, &config.Profile{Name: n})
	}
	return out
}

func TestRevokeAcrossAllOK(t *testing.T) {
	var calls []string
	call := func(e *config.Profile, name string, cb func(error)) {
		calls = append(calls, e.Name+"/"+name)
		cb(nil)
	}
	var done, failed []string
	finished := false
	revokeAcross(entriesNamed("A", "B", "C"), "Team", call, shortErr, func(d, f []string) { done, failed, finished = d, f, true })
	if !finished {
		t.Fatal("finish not called")
	}
	if !reflect.DeepEqual(calls, []string{"A/Team", "B/Team", "C/Team"}) {
		t.Fatalf("calls %v", calls)
	}
	if !reflect.DeepEqual(done, []string{"A", "B", "C"}) || len(failed) != 0 {
		t.Fatalf("done %v failed %v", done, failed)
	}
}

func TestRevokeAcrossPartialFailure(t *testing.T) {
	call := func(e *config.Profile, name string, cb func(error)) {
		if e.Name == "B" {
			cb(errors.New("ssh: timeout"))
			return
		}
		cb(nil)
	}
	var done, failed []string
	revokeAcross(entriesNamed("A", "B", "C"), "Team", call, func(err error) string { return err.Error() }, func(d, f []string) { done, failed = d, f })
	if !reflect.DeepEqual(done, []string{"A", "C"}) {
		t.Fatalf("done %v: a failure on B must not stop C", done)
	}
	if !reflect.DeepEqual(failed, []string{"B: ssh: timeout"}) {
		t.Fatalf("failed %v", failed)
	}
}

func TestRevokeAcrossAllFail(t *testing.T) {
	call := func(e *config.Profile, name string, cb func(error)) { cb(errors.New("no route")) }
	var done, failed []string
	revokeAcross(entriesNamed("A", "B"), "X", call, func(err error) string { return err.Error() }, func(d, f []string) { done, failed = d, f })
	if len(done) != 0 || len(failed) != 2 {
		t.Fatalf("done %v failed %v", done, failed)
	}
}

func TestRevokeAcrossAsyncAndOrder(t *testing.T) {
	// ответы приходят позже (как после запроса пароля SSH): следующий вход не начинается раньше
	var pending []func()
	var order []string
	call := func(e *config.Profile, name string, cb func(error)) {
		order = append(order, "start "+e.Name)
		pending = append(pending, func() { order = append(order, "end "+e.Name); cb(nil) })
	}
	finished := false
	revokeAcross(entriesNamed("A", "B"), "X", call, shortErr, func(d, f []string) { finished = true })
	if finished || len(pending) != 1 {
		t.Fatalf("second entry started early: %v", order)
	}
	pending[0]()
	if len(pending) != 2 || finished {
		t.Fatalf("order %v", order)
	}
	pending[1]()
	if !finished || !reflect.DeepEqual(order, []string{"start A", "end A", "start B", "end B"}) {
		t.Fatalf("order %v finished %v", order, finished)
	}
}

func TestRevokeAcrossNoEntries(t *testing.T) {
	called := false
	revokeAcross(nil, "X", nil, shortErr, func(d, f []string) { called = len(d) == 0 && len(f) == 0 })
	if !called {
		t.Fatal("finish was not called for an empty list")
	}
}

// ---------- опрос сайта-прикрытия ----------

type fakeClock struct{ now time.Duration }

func (c *fakeClock) sleep(d time.Duration) { c.now += d }

func newTestPoller(c *fakeClock, states []server.SiteRepoInfo, errAt int) (siteRepoPoller, *[]server.SiteRepoInfo, *int) {
	var seen []server.SiteRepoInfo
	n := 0
	return siteRepoPoller{
		fetch: func() (server.SiteRepoInfo, error) {
			i := n
			n++
			if errAt >= 0 && i == errAt {
				return server.SiteRepoInfo{}, errors.New("connection lost")
			}
			if i >= len(states) {
				i = len(states) - 1
			}
			return states[i], nil
		},
		sleep:    c.sleep,
		interval: 2 * time.Second,
		limit:    2 * time.Minute,
		onState:  func(s server.SiteRepoInfo) { seen = append(seen, s) },
	}, &seen, &n
}

func TestSitePollSyncingThenDone(t *testing.T) {
	c := &fakeClock{}
	p, seen, n := newTestPoller(c, []server.SiteRepoInfo{{Syncing: true}, {Syncing: true}, {Enabled: true, Files: 42}}, -1)
	if end := p.run(); end != pollDone {
		t.Fatalf("end %v", end)
	}
	if *n != 3 || len(*seen) != 3 || (*seen)[2].Syncing || (*seen)[2].Files != 42 {
		t.Fatalf("polls %d seen %+v", *n, *seen)
	}
	if c.now != 6*time.Second {
		t.Fatalf("waited %v, want 6s (interval 2s)", c.now)
	}
}

func TestSitePollTimeoutAfterTwoMinutes(t *testing.T) {
	c := &fakeClock{}
	p, seen, n := newTestPoller(c, []server.SiteRepoInfo{{Syncing: true}}, -1)
	if end := p.run(); end != pollTimeout {
		t.Fatalf("end %v", end)
	}
	if c.now != 2*time.Minute || *n != 60 || len(*seen) != 60 {
		t.Fatalf("waited %v, polls %d", c.now, *n)
	}
}

func TestSitePollErrorStops(t *testing.T) {
	c := &fakeClock{}
	p, seen, n := newTestPoller(c, []server.SiteRepoInfo{{Syncing: true}}, 2)
	if end := p.run(); end != pollError {
		t.Fatalf("end %v", end)
	}
	if *n != 3 || len(*seen) != 2 {
		t.Fatalf("polls %d seen %d", *n, len(*seen))
	}
}

func TestSitePollAlreadyDone(t *testing.T) {
	c := &fakeClock{}
	p, _, n := newTestPoller(c, []server.SiteRepoInfo{{Enabled: true}}, -1)
	if p.run() != pollDone || *n != 1 {
		t.Fatalf("polls %d", *n)
	}
}
