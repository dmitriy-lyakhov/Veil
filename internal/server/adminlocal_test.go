package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLatinName(t *testing.T) {
	for in, want := range map[string]string{
		"Ноутбук Анны":  "Noutbuk-Anny",
		"laptop-01":     "laptop-01",
		"Щука ёж":       "Shchuka-ezh",
		"  @@@ ":        "",
		"Сергей — ПК 2": "Sergey-PK-2",
	} {
		if got := latinName(in); got != want {
			t.Errorf("latinName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAdminLocalSessionsAndBlocks(t *testing.T) {
	st := newAdminState()
	now := time.Now()
	tok1, tok2 := st.newSession(now), st.newSession(now)
	_ = tok2
	var cur [32]byte
	for k := range st.sessions {
		cur = k
		break
	}
	for k := range st.sessions {
		st.touch(k, "10.66.0.2", "test")
	}
	for i := 0; i < loginMaxFails; i++ {
		st.recordFailure("10.66.0.9", now)
	}
	var events []string
	l := adminLocal{st: st, cur: cur, user: "admin", ip: "10.66.0.2", mode: "builtin", event: func(_, text string) { events = append(events, text) }}
	call := func(method, path, body string) (int, map[string]any) {
		r := httptest.NewRequest(method, "/admin/api/"+path, strings.NewReader(body))
		w := httptest.NewRecorder()
		if !serveAdminLocal(w, r, path, l) {
			t.Fatalf("%s не обработан как локальный", path)
		}
		var m map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &m)
		return w.Code, m
	}
	if _, m := call(http.MethodGet, "admin/sessions", ""); len(m["sessions"].([]any)) != 2 {
		t.Fatalf("сессий: %v", m)
	}
	if code, _ := call(http.MethodPost, "admin/sessions/kill", `{"all_others":true}`); code != 200 {
		t.Fatal("kill others")
	}
	if len(st.sessions) != 1 {
		t.Fatalf("осталось сессий %d, ожидалась 1 (текущая)", len(st.sessions))
	}
	if _, m := call(http.MethodGet, "admin/blocked", ""); len(m["blocked"].([]any)) != 1 {
		t.Fatalf("blocked: %v", m)
	}
	if code, _ := call(http.MethodPost, "admin/unblock", `{"ip":"10.66.0.9"}`); code != 200 || st.blocked("10.66.0.9", now) {
		t.Fatal("unblock")
	}
	if _, m := call(http.MethodGet, "admin/protocols", ""); m["docker"] != false {
		t.Fatalf("protocols без Docker: %v", m)
	}
	if len(events) != 2 {
		t.Fatalf("событий %d: %v", len(events), events)
	}
	_ = tok1
	if serveAdminLocal(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/admin/api/state", nil), "state", l) {
		t.Fatal("state не должен быть локальным")
	}
}

func TestTrafficStats(t *testing.T) {
	var ts trafficStats
	base := time.Date(2026, 11, 8, 10, 5, 0, 0, time.UTC)
	ts.sample(base, 100, 50)
	ts.sample(base.Add(time.Minute), 1100, 250)
	ts.sample(base.Add(time.Hour), 2100, 450)
	ts.sample(base.Add(time.Hour+time.Minute), 10, 10) // счётчики сброшены
	h := ts.last24(base.Add(time.Hour + 2*time.Minute))
	if len(h) != 24 || h[22].Rx != 1000 || h[23].Rx != 1000 || h[23].Tx != 200 {
		t.Fatalf("часы: %+v %+v", h[22], h[23])
	}
}
