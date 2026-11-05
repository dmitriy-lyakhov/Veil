package site

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type ent struct {
	name string
	body string
	typ  byte // 0 = файл
	link string
}

func makeTarGz(t *testing.T, ents []ent) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for _, e := range ents {
		h := &tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.body)), Typeflag: tar.TypeReg}
		switch e.typ {
		case tar.TypeSymlink, tar.TypeLink:
			h.Typeflag, h.Linkname, h.Size = e.typ, e.link, 0
		case tar.TypeDir:
			h.Typeflag, h.Size = tar.TypeDir, 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Size > 0 {
			tw.Write([]byte(e.body))
		}
	}
	tw.Close()
	zw.Close()
	return buf.Bytes()
}

func makeZip(t *testing.T, names map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for n, b := range names {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(b))
	}
	zw.Close()
	return buf.Bytes()
}

// fetchFrom поднимает https-сервер с архивом и забирает его через FetchRepo.
func fetchFrom(t *testing.T, archive []byte, sp RepoSpec, lim Limits) (*Snapshot, error) {
	t.Helper()
	var gotAuth string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write(archive)
	}))
	t.Cleanup(srv.Close)
	if sp.URL == "" {
		sp.URL = "https://example.com/o/r"
	}
	opt := FetchOptions{
		Limits: lim, NoGit: true, HTTPClient: srv.Client(),
		ArchiveURL: func(RepoSpec) (string, http.Header, error) {
			h := http.Header{}
			if sp.Token != "" {
				h.Set("Authorization", "Bearer "+sp.Token)
			}
			return srv.URL + "/a.tar.gz", h, nil
		},
	}
	snap, err := FetchRepo(context.Background(), sp, t.TempDir(), opt)
	if sp.Token != "" && err == nil && gotAuth != "Bearer "+sp.Token {
		t.Fatalf("токен не передан заголовком: %q", gotAuth)
	}
	return snap, err
}

func lim() Limits { l := DefaultLimits(); l.Timeout = 20 * time.Second; return l }

func exists(dir, rel string) bool {
	_, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(rel)))
	return err == nil
}

func TestFetchArchiveFilters(t *testing.T) {
	ar := makeTarGz(t, []ent{
		{name: "pax", typ: tar.TypeXGlobalHeader},
		{name: "o-r-0123abc/", typ: tar.TypeDir},
		{name: "o-r-0123abc/index.html", body: "<h1>hi</h1>"},
		{name: "o-r-0123abc/css/a.css", body: "body{}"},
		{name: "o-r-0123abc/img/p.PNG", body: "png"},
		{name: "o-r-0123abc/shell.php", body: "<?php"},
		{name: "o-r-0123abc/run.sh", body: "x"},
		{name: "o-r-0123abc/.env", body: "SECRET=1"},
		{name: "o-r-0123abc/.git/config", body: "x"},
		{name: "o-r-0123abc/package.json", body: "{}"},
		{name: "o-r-0123abc/node_modules/x/i.js", body: "x"},
		{name: "o-r-0123abc/README.md", body: "x"},
	})
	snap, err := fetchFrom(t, ar, RepoSpec{Token: "tok123"}, lim())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"index.html", "css/a.css", "img/p.PNG"} {
		if !exists(snap.Dir, p) {
			t.Errorf("нет %s", p)
		}
	}
	for _, p := range []string{"shell.php", "run.sh", ".env", ".git/config", "package.json", "node_modules/x/i.js", "README.md"} {
		if exists(snap.Dir, p) {
			t.Errorf("лишний файл %s", p)
		}
	}
	if snap.Files != 3 || snap.Commit != "0123abc" {
		t.Errorf("files=%d commit=%q", snap.Files, snap.Commit)
	}
}

func TestFetchArchiveSubdir(t *testing.T) {
	ar := makeTarGz(t, []ent{
		{name: "r/index.html", body: "root"},
		{name: "r/public/index.html", body: "pub"},
		{name: "r/public/a.css", body: "x"},
		{name: "r/other/b.css", body: "x"},
	})
	snap, err := fetchFrom(t, ar, RepoSpec{Dir: "/public/"}, lim())
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(snap.Dir, "index.html"))
	if string(b) != "pub" || exists(snap.Dir, "b.css") || !exists(snap.Dir, "a.css") {
		t.Fatalf("подкаталог обработан неверно: %q", b)
	}
}

func TestFetchNoIndex(t *testing.T) {
	ar := makeTarGz(t, []ent{{name: "r/a.css", body: "x"}})
	if _, err := fetchFrom(t, ar, RepoSpec{}, lim()); err == nil || !strings.Contains(err.Error(), "index.html") {
		t.Fatalf("ожидалась ошибка про index.html: %v", err)
	}
}

func TestTarSlip(t *testing.T) {
	for _, name := range []string{"r/../../evil.html", "../evil.html", "/abs/evil.html", "r/a/../../../x.css", "r\\..\\evil.html"} {
		ar := makeTarGz(t, []ent{{name: "r/index.html", body: "x"}, {name: name, body: "evil"}})
		if _, err := fetchFrom(t, ar, RepoSpec{}, lim()); err == nil {
			t.Errorf("%q: ожидалась ошибка", name)
		}
	}
}

func TestZipSlipAndZipOK(t *testing.T) {
	bad := makeZip(t, map[string]string{"r/index.html": "x", "r/../../evil.html": "e"})
	if _, err := fetchFrom(t, bad, RepoSpec{}, lim()); err == nil {
		t.Fatal("zip slip не отвергнут")
	}
	good := makeZip(t, map[string]string{"r/index.html": "x", "r/s.css": "y", "r/x.php": "z"})
	snap, err := fetchFrom(t, good, RepoSpec{}, lim())
	if err != nil || snap.Files != 2 {
		t.Fatalf("zip: %v %+v", err, snap)
	}
}

func TestSymlinksIgnored(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "secret.html")
	os.WriteFile(secret, []byte("TOP SECRET"), 0o600)
	ar := makeTarGz(t, []ent{
		{name: "r/index.html", body: "x"},
		{name: "r/leak.html", typ: tar.TypeSymlink, link: secret},
		{name: "r/leak2.html", typ: tar.TypeSymlink, link: "../../../etc/passwd"},
		{name: "r/hard.html", typ: tar.TypeLink, link: "r/index.html"},
	})
	snap, err := fetchFrom(t, ar, RepoSpec{}, lim())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"leak.html", "leak2.html", "hard.html"} {
		if exists(snap.Dir, p) {
			t.Errorf("ссылка %s попала в снимок", p)
		}
	}
	if snap.Skipped != 3 {
		t.Errorf("skipped=%d", snap.Skipped)
	}
}

func TestLimits(t *testing.T) {
	big := strings.Repeat("a", 2000)
	ents := []ent{{name: "r/index.html", body: "x"}, {name: "r/big.css", body: big}}
	l := lim()
	l.MaxFile = 1000
	if _, err := fetchFrom(t, makeTarGz(t, ents), RepoSpec{}, l); err == nil || !strings.Contains(err.Error(), "лимита") {
		t.Errorf("лимит файла: %v", err)
	}
	l = lim()
	l.MaxFiles = 1
	if _, err := fetchFrom(t, makeTarGz(t, ents), RepoSpec{}, l); err == nil {
		t.Error("лимит числа файлов не сработал")
	}
	l = lim()
	l.MaxTotal = 1500
	if _, err := fetchFrom(t, makeTarGz(t, ents), RepoSpec{}, l); err == nil {
		t.Error("лимит общего размера не сработал")
	}
	// файл-бомба: заявленный размер небольшой, но поток не ограничен лимитом файла
	l = lim()
	l.MaxFile = 1 << 10
	l.MaxTotal = 1 << 20
	if _, err := fetchFrom(t, makeTarGz(t, []ent{{name: "r/index.html", body: "x"}, {name: "r/z.txt", body: strings.Repeat("0", 1<<16)}}), RepoSpec{}, l); err == nil {
		t.Error("большой файл принят")
	}
}

func TestDownloadTimeout(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer srv.Close()
	l := DefaultLimits()
	l.Timeout = 300 * time.Millisecond
	start := time.Now()
	_, err := FetchRepo(context.Background(), RepoSpec{URL: "https://example.com/o/r"}, t.TempDir(), FetchOptions{
		Limits: l, NoGit: true, HTTPClient: srv.Client(),
		ArchiveURL: func(RepoSpec) (string, http.Header, error) { return srv.URL, nil, nil },
	})
	if err == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("таймаут не сработал: %v %v", err, time.Since(start))
	}
}

func TestHTTPErrors(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }))
	defer srv.Close()
	_, err := FetchRepo(context.Background(), RepoSpec{URL: "https://example.com/o/r", Token: "SECRETTOKEN"}, t.TempDir(), FetchOptions{
		NoGit: true, HTTPClient: srv.Client(),
		ArchiveURL: func(RepoSpec) (string, http.Header, error) { return srv.URL, nil, nil },
	})
	if err == nil || strings.Contains(err.Error(), "SECRETTOKEN") {
		t.Fatalf("ошибка: %v", err)
	}
}

func TestValidateSpec(t *testing.T) {
	bad := []RepoSpec{
		{URL: "http://github.com/o/r"},
		{URL: "https://user:pw@github.com/o/r"},
		{URL: "https://x:tok@github.com/o/r"},
		{URL: "https://github.com/o/r?token=1"},
		{URL: "https://github.com/"},
		{URL: "file:///etc"},
		{URL: "ssh://git@github.com/o/r"},
		{URL: "https://github.com/o/r", Ref: "--upload-pack=x"},
		{URL: "https://github.com/o/r", Ref: "a..b"},
		{URL: "https://github.com/o/r", Dir: "../x"},
		{URL: "https://github.com/o/r", Dir: "a/../../x"},
		{URL: "https://github.com/o/r", Dir: ".git"},
		{URL: "https://github.com/o/r", Token: "a\nb"},
	}
	for _, sp := range bad {
		if _, err := ValidateSpec(sp); err == nil {
			t.Errorf("принято: %+v", sp)
		}
	}
	sp, err := ValidateSpec(RepoSpec{URL: " https://github.com/o/r ", Ref: "feature/x", Dir: "/site/dist/"})
	if err != nil || sp.Dir != "site/dist" || sp.Ref != "feature/x" {
		t.Fatalf("%+v %v", sp, err)
	}
}

func TestArchiveRequest(t *testing.T) {
	cases := []struct {
		sp      RepoSpec
		url     string
		hdrKey  string
		wantErr bool
	}{
		{RepoSpec{URL: "https://github.com/o/r.git", Ref: "main", Token: "t"}, "https://api.github.com/repos/o/r/tarball/main", "Authorization", false},
		{RepoSpec{URL: "https://github.com/o/r"}, "https://api.github.com/repos/o/r/tarball", "", false},
		{RepoSpec{URL: "https://gitlab.com/g/sub/p", Ref: "v1", Token: "t"}, "https://gitlab.com/api/v4/projects/g%2Fsub%2Fp/repository/archive.tar.gz?sha=v1", "Private-Token", false},
		{RepoSpec{URL: "https://codeberg.org/o/r", Ref: "main"}, "https://codeberg.org/api/v1/repos/o/r/archive/main.tar.gz", "", false},
		{RepoSpec{URL: "https://codeberg.org/o/r"}, "", "", true},
		{RepoSpec{URL: "https://bitbucket.org/o/r"}, "", "", true},
		{RepoSpec{URL: "https://git.example.org/o/r"}, "", "", true},
	}
	for _, c := range cases {
		u, h, err := archiveRequest(c.sp)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err=%v", c.sp.URL, err)
			continue
		}
		if err == nil && u != c.url {
			t.Errorf("%s: url %q, want %q", c.sp.URL, u, c.url)
		}
		if c.hdrKey != "" && h.Get(c.hdrKey) == "" {
			t.Errorf("%s: нет заголовка %s", c.sp.URL, c.hdrKey)
		}
		if strings.Contains(u, "t=") && strings.Contains(u, "token") {
			t.Errorf("токен в адресе: %s", u)
		}
	}
	if _, _, err := archiveRequest(RepoSpec{URL: "https://bitbucket.org/o/r"}); err == nil || !strings.Contains(err.Error(), "git") {
		t.Errorf("сообщение об ошибке должно советовать git: %v", err)
	}
}

func TestRedirectStripsToken(t *testing.T) {
	first, _ := http.NewRequest("GET", "https://gitlab.com/a", nil)
	next, _ := http.NewRequest("GET", "https://cdn.example.net/b", nil)
	next.Header.Set("PRIVATE-TOKEN", "x")
	next.Header.Set("Authorization", "Bearer x")
	if err := checkRedirect(next, []*http.Request{first}); err != nil {
		t.Fatal(err)
	}
	if next.Header.Get("Private-Token") != "" || next.Header.Get("Authorization") != "" {
		t.Error("токен ушёл на другой хост")
	}
	plain, _ := http.NewRequest("GET", "http://evil.example/", nil)
	if checkRedirect(plain, []*http.Request{first}) == nil {
		t.Error("перенаправление на http разрешено")
	}
}

func TestBlockedIP(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.1.2.3", "192.168.0.1", "169.254.169.254", "::1", "fd00::1", "0.0.0.0"} {
		if !blockedIP(netParse(s)) {
			t.Errorf("%s не заблокирован", s)
		}
	}
	if blockedIP(netParse("93.184.216.34")) {
		t.Error("публичный адрес заблокирован")
	}
}

func TestGitLocalClone(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git не установлен")
	}
	repo := t.TempDir()
	run := func(args ...string) {
		c := exec.Command(git, append([]string{"-c", "user.email=a@b", "-c", "user.name=t"}, args...)...)
		c.Dir = repo
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	os.MkdirAll(filepath.Join(repo, "www"), 0o755)
	os.WriteFile(filepath.Join(repo, "www", "index.html"), []byte("<h1>git</h1>"), 0o644)
	os.WriteFile(filepath.Join(repo, "www", "app.php"), []byte("<?php"), 0o644)
	os.WriteFile(filepath.Join(repo, "www", ".env"), []byte("S=1"), 0o644)
	os.WriteFile(filepath.Join(repo, "index.html"), []byte("root"), 0o644)
	run("add", "-A")
	run("commit", "-q", "-m", "x")
	snap, err := FetchRepo(context.Background(), RepoSpec{URL: repo, Ref: "main", Dir: "www"}, t.TempDir(), FetchOptions{AllowFile: true, AllowPrivate: true})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(snap.Dir, "index.html"))
	if string(b) != "<h1>git</h1>" || exists(snap.Dir, "app.php") || exists(snap.Dir, ".env") || snap.Method != "git" || len(snap.Commit) != 40 {
		t.Fatalf("снимок git: %q %+v", b, snap)
	}
	if _, err := FetchRepo(context.Background(), RepoSpec{URL: repo, Ref: "nope"}, t.TempDir(), FetchOptions{AllowFile: true, AllowPrivate: true}); err == nil {
		t.Error("несуществующая ветка принята")
	}
}

func TestTreeServing(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	write("index.html", "ROOT")
	write("about/index.html", "ABOUT")
	write("a.css", "css")
	write("admin/x.html", "evil")
	write("admin.html", "evil")
	write("tun/api.json", "{}")
	write("s.php", "php")
	write(".secret.txt", "s")
	tree, err := LoadTree(dir, func(p string) bool { return strings.HasPrefix(p, "/admin") || strings.HasPrefix(p, "/tun") })
	if err != nil {
		t.Fatal(err)
	}
	get := func(p string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		if !tree.Serve(rec, httptest.NewRequest("GET", p, nil)) {
			rec.Code = 404
		}
		return rec
	}
	for p, want := range map[string]string{"/": "ROOT", "/index.html": "ROOT", "/about/": "ABOUT", "/a.css": "css"} {
		if r := get(p); r.Code != 200 || r.Body.String() != want {
			t.Errorf("%s: %d %q", p, r.Code, r.Body.String())
		}
	}
	for _, p := range []string{"/admin/x.html", "/admin.html", "/tun/api.json", "/s.php", "/.secret.txt", "/about", "/../../etc/passwd", "/%2e%2e/x", "/a.css/../index.html", "//a.css", "/nope"} {
		if r := get(p); r.Code != 404 {
			t.Errorf("%s: должен быть 404, получили %d", p, r.Code)
		}
	}
	if !tree.IsDir("/about") || tree.IsDir("/nope") {
		t.Error("IsDir")
	}
	r := get("/a.css")
	if r.Header().Get("Content-Type") != "text/css; charset=utf-8" || r.Header().Get("Cache-Control") != "max-age=86400" || r.Header().Get("Content-Length") != "3" {
		t.Errorf("заголовки: %v", r.Header())
	}
	if r := get("/"); r.Header().Get("Cache-Control") != "max-age=600" || len(r.Header()) != 3 {
		t.Errorf("заголовки html: %v", r.Header())
	}
	rec := httptest.NewRecorder()
	if tree.Serve(rec, httptest.NewRequest("POST", "/", nil)) {
		t.Error("POST обслужен")
	}
	if _, err := LoadTree(t.TempDir(), nil); err == nil {
		t.Error("пустой каталог принят")
	}
}

func TestTreeSkipsSymlinks(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("x"), 0o644)
	secret := filepath.Join(t.TempDir(), "s.html")
	os.WriteFile(secret, []byte("secret"), 0o644)
	if err := os.Symlink(secret, filepath.Join(dir, "link.html")); err != nil {
		t.Skip("symlink недоступен: ", err)
	}
	tree, err := LoadTree(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Has("/link.html") {
		t.Error("симлинк раздаётся")
	}
}

func netParse(s string) net.IP { return net.ParseIP(s) }
