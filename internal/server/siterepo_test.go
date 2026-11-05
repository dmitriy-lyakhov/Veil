package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"veil/internal/config"
	"veil/internal/site"
)

func tgz(files map[string]string) []byte {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for n, b := range files {
		tw.WriteHeader(&tar.Header{Name: "r-abc1234/" + n, Mode: 0o644, Size: int64(len(b)), Typeflag: tar.TypeReg})
		tw.Write([]byte(b))
	}
	tw.Close()
	zw.Close()
	return buf.Bytes()
}

type repoEnv struct {
	s      *Server
	m      *siteRepoManager
	fail   atomic.Bool
	body   atomic.Value // []byte
	gen    string
	ctx    context.Context
	cancel context.CancelFunc
}

func newRepoEnv(t *testing.T) *repoEnv {
	t.Helper()
	g := site.Generate("en")
	s := &Server{cfg: &config.Server{Path: "/tunnel"}, web: newWebSite(&g)}
	e := &repoEnv{s: s, gen: string(g.Index())}
	e.ctx, e.cancel = context.WithCancel(context.Background())
	t.Cleanup(e.cancel)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if e.fail.Load() {
			w.WriteHeader(500)
			return
		}
		w.Write(e.body.Load().([]byte))
	}))
	t.Cleanup(srv.Close)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	e.m = newSiteRepoManager(s, filepath.Join(t.TempDir(), "config.json"), log)
	e.m.opt = site.FetchOptions{
		NoGit: true, HTTPClient: srv.Client(),
		ArchiveURL: func(site.RepoSpec) (string, http.Header, error) { return srv.URL, nil, nil },
	}
	s.siteRepo = e.m
	e.m.start(e.ctx)
	return e
}

func (e *repoEnv) get(method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	e.s.web.ServeHTTP(rec, req)
	return rec
}

func (e *repoEnv) waitFor(t *testing.T, cond func(SiteRepoInfo) bool) SiteRepoInfo {
	t.Helper()
	for i := 0; i < 200; i++ {
		if in := e.m.get(); cond(in) && !in.Syncing {
			return in
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("не дождались: %+v", e.m.get())
	return SiteRepoInfo{}
}

func TestSiteRepoServeFallbackAndRollback(t *testing.T) {
	e := newRepoEnv(t)
	e.body.Store(tgz(map[string]string{
		"index.html":       "<h1>REPO</h1>",
		"a/index.html":     "<p>A</p>",
		"style.css":        "b{}",
		"admin/panel.html": "evil",
		"tunnel":           "evil", // без расширения — не раздаётся
		"tunnel/x.json":    "{}",
		"shell.php":        "<?php",
	}))

	// до настройки — сгенерированный сайт
	if got := e.get("GET", "/").Body.String(); got != e.gen {
		t.Fatal("ожидался сгенерированный сайт")
	}

	if err := e.s.SetSiteRepo("https://example.com/o/r", "", "", "tok", false); err != nil {
		t.Fatal(err)
	}
	in := e.waitFor(t, func(i SiteRepoInfo) bool { return i.Active })
	if in.Commit != "abc1234" || in.Files != 3 || !in.HasToken || in.Updated == nil {
		t.Fatalf("статус: %+v", in)
	}
	if r := e.get("GET", "/"); r.Body.String() != "<h1>REPO</h1>" || r.Header().Get("Cache-Control") != "max-age=600" {
		t.Fatalf("корень: %q %v", r.Body.String(), r.Header())
	}
	if r := e.get("HEAD", "/style.css"); r.Code != 200 || r.Body.Len() != 0 || r.Header().Get("Content-Length") != "3" {
		t.Fatalf("HEAD: %d %v", r.Code, r.Header())
	}
	if r := e.get("GET", "/a"); r.Code != 301 || r.Header().Get("Location") != "https://example.com/a/" {
		t.Fatalf("каталог без слэша: %d %v", r.Code, r.Header())
	}
	// 404 как у обычного сайта
	for _, p := range []string{"/nope", "/admin/panel.html", "/admin", "/tunnel/x.json", "/shell.php", "/assets/site.css"} {
		r := e.get("GET", p)
		if r.Code != 404 || r.Body.String() != notFoundHTML {
			t.Errorf("%s: %d %q", p, r.Code, r.Body.String())
		}
	}
	if r := e.get("POST", "/contact"); r.Code != 404 {
		t.Errorf("POST /contact: %d", r.Code)
	}
	if r := e.get("GET", "/robots.txt"); r.Body.String() != robotsTXT {
		t.Errorf("robots.txt: %q", r.Body.String())
	}
	// репозиторий в состоянии без токена
	st := State{SiteRepo: e.s.SiteRepoInfo()}
	if strings.Contains(st.SiteRepo.URL, "tok") {
		t.Error("токен в состоянии")
	}

	// сбой обновления: остаётся последняя удачная версия
	e.fail.Store(true)
	if err := e.s.RefreshSiteRepo(); err != nil {
		t.Fatal(err)
	}
	in = e.waitFor(t, func(i SiteRepoInfo) bool { return i.Error != "" })
	if !in.Active || in.Commit != "abc1234" || in.Checked == nil {
		t.Fatalf("после сбоя: %+v", in)
	}
	if e.get("GET", "/").Body.String() != "<h1>REPO</h1>" {
		t.Fatal("после сбоя перестал отдаваться репозиторий")
	}

	// новая версия применяется
	e.fail.Store(false)
	e.body.Store(tgz(map[string]string{"index.html": "<h1>V2</h1>"}))
	e.s.RefreshSiteRepo()
	e.waitFor(t, func(i SiteRepoInfo) bool { return i.Error == "" && i.Files == 1 })
	if e.get("GET", "/").Body.String() != "<h1>V2</h1>" || e.get("GET", "/style.css").Code != 404 {
		t.Fatal("новая версия не применена")
	}

	// пустой url — обратно к сгенерированному
	if err := e.s.SetSiteRepo("", "", "", "", false); err != nil {
		t.Fatal(err)
	}
	if got := e.get("GET", "/").Body.String(); got != e.gen {
		t.Fatal("после сброса ожидался сгенерированный сайт")
	}
	if in := e.m.get(); in.Enabled || in.Active {
		t.Fatalf("статус после сброса: %+v", in)
	}
	if err := e.s.RefreshSiteRepo(); err == nil {
		t.Error("refresh без настройки должен вернуть ошибку")
	}
}

func TestSiteRepoBadRepoKeepsGenerated(t *testing.T) {
	e := newRepoEnv(t)
	e.body.Store(tgz(map[string]string{"style.css": "b{}"})) // нет index.html
	if err := e.s.SetSiteRepo("https://example.com/o/r", "", "", "", false); err != nil {
		t.Fatal(err)
	}
	in := e.waitFor(t, func(i SiteRepoInfo) bool { return i.Error != "" })
	if in.Active || !strings.Contains(in.Error, "index.html") {
		t.Fatalf("%+v", in)
	}
	if e.get("GET", "/").Body.String() != e.gen {
		t.Fatal("должен отдаваться сгенерированный сайт")
	}
}

func TestSiteRepoRejectsBadSettings(t *testing.T) {
	e := newRepoEnv(t)
	for _, u := range []string{"http://example.com/o/r", "https://u:p@example.com/o/r", "ssh://example.com/o/r"} {
		if err := e.s.SetSiteRepo(u, "", "", "", false); err == nil {
			t.Errorf("принят %s", u)
		}
	}
	if err := e.s.SetSiteRepo("https://example.com/o/r", "", "../x", "", false); err == nil {
		t.Error("принят подкаталог с ..")
	}
	if e.s.cfg.SiteRepo != nil {
		t.Error("плохая настройка сохранена")
	}
}

func TestSiteRepoTokenKeptForSameURL(t *testing.T) {
	e := newRepoEnv(t)
	e.body.Store(tgz(map[string]string{"index.html": "x"}))
	e.s.SetSiteRepo("https://example.com/o/r", "", "", "secret", false)
	e.s.SetSiteRepo("https://example.com/o/r", "main", "", "", false)
	if e.s.cfg.SiteRepo.Token != "secret" || e.s.cfg.SiteRepo.Ref != "main" {
		t.Fatalf("%+v", e.s.cfg.SiteRepo)
	}
	e.s.SetSiteRepo("https://example.com/o/r", "main", "", "", true)
	if e.s.cfg.SiteRepo.Token != "" {
		t.Fatal("токен не удалён")
	}
	e.s.SetSiteRepo("https://example.com/o/r", "main", "", "new", false)
	e.s.SetSiteRepo("https://example.com/other", "", "", "", false)
	if e.s.cfg.SiteRepo.Token != "" {
		t.Fatal("токен перенесён на другой адрес")
	}
}

func TestSiteRepoCachedAcrossRestart(t *testing.T) {
	e := newRepoEnv(t)
	e.body.Store(tgz(map[string]string{"index.html": "<h1>CACHED</h1>"}))
	e.s.SetSiteRepo("https://example.com/o/r", "", "", "", false)
	e.waitFor(t, func(i SiteRepoInfo) bool { return i.Active })
	cfgPath := filepath.Join(filepath.Dir(e.m.dir), "config.json")

	// «перезапуск»: новый сервер с той же настройкой и недоступным репозиторием
	g := site.Generate("en")
	s2 := &Server{cfg: &config.Server{Path: "/tunnel", SiteRepo: e.s.cfg.SiteRepo}, web: newWebSite(&g)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m2 := newSiteRepoManager(s2, cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m2.opt = site.FetchOptions{NoGit: true, ArchiveURL: func(site.RepoSpec) (string, http.Header, error) { return "https://127.0.0.1:1/", nil, nil }}
	s2.siteRepo = m2
	m2.start(ctx)
	rec := httptest.NewRecorder()
	s2.web.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Body.String() != "<h1>CACHED</h1>" {
		t.Fatalf("кэш не поднят: %q", rec.Body.String())
	}
}
