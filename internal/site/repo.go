package site

// Сайт-прикрытие из репозитория: загрузка (git или архив по HTTPS), строгая фильтрация
// (только статика, без исполнения чего-либо из репозитория) и раздача из готового снимка.

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// RepoSpec — что забирать.
type RepoSpec struct {
	URL   string
	Ref   string
	Dir   string
	Token string
}

// Limits — ограничения на загружаемый сайт.
type Limits struct {
	MaxTotal int64         // суммарный размер оставленных файлов
	MaxFiles int           // число оставленных файлов
	MaxFile  int64         // размер одного файла
	Timeout  time.Duration // полное время загрузки
}

func DefaultLimits() Limits {
	return Limits{MaxTotal: 50 << 20, MaxFiles: 2000, MaxFile: 10 << 20, Timeout: 2 * time.Minute}
}

// FetchOptions — необязательные настройки (в основном для тестов).
type FetchOptions struct {
	Limits       Limits
	HTTPClient   *http.Client // по умолчанию — свой клиент с запретом внутренних адресов
	AllowPrivate bool         // разрешить адреса из частных сетей и loopback
	NoGit        bool         // не использовать git, даже если он есть
	GitBin       string       // путь к git (по умолчанию ищется в PATH)

	AllowFile  bool                                        // тест: разрешить локальный путь вместо https
	ArchiveURL func(RepoSpec) (string, http.Header, error) // тест: подмена адреса архива
}

// Snapshot — готовый каталог со статикой.
type Snapshot struct {
	Dir     string
	Commit  string
	Files   int
	Bytes   int64
	Skipped int // файлы, которые не раздаются (не статика, скрытые, ссылки)
	Method  string
}

var refRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/@+-]{0,199}$`)

// ValidateSpec проверяет и нормализует настройки репозитория.
func ValidateSpec(sp RepoSpec) (RepoSpec, error) {
	return validateSpec(sp, false)
}

func validateSpec(sp RepoSpec, allowFile bool) (RepoSpec, error) {
	sp.URL = strings.TrimSpace(sp.URL)
	sp.Ref = strings.TrimSpace(sp.Ref)
	sp.Dir = strings.TrimSpace(sp.Dir)
	sp.Token = strings.TrimSpace(sp.Token)
	if !allowFile {
		u, err := url.Parse(sp.URL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" {
			return sp, errors.New("адрес репозитория должен быть вида https://хост/путь")
		}
		if u.User != nil {
			return sp, errors.New("не указывайте логин и токен в адресе: токен задаётся отдельным полем")
		}
		if u.RawQuery != "" || u.Fragment != "" {
			return sp, errors.New("адрес репозитория не должен содержать параметров и якоря")
		}
		if strings.Trim(u.Path, "/") == "" {
			return sp, errors.New("в адресе репозитория нет пути")
		}
	}
	if sp.Ref != "" && (!refRe.MatchString(sp.Ref) || strings.Contains(sp.Ref, "..")) {
		return sp, errors.New("недопустимое имя ветки или тега")
	}
	if strings.ContainsAny(sp.Token, "\r\n\x00 ") {
		return sp, errors.New("недопустимые символы в токене")
	}
	d, err := cleanDir(sp.Dir)
	if err != nil {
		return sp, err
	}
	sp.Dir = d
	return sp, nil
}

func cleanDir(d string) (string, error) {
	d = strings.ReplaceAll(strings.TrimSpace(d), "\\", "/")
	d = strings.Trim(d, "/")
	if d == "" || d == "." {
		return "", nil
	}
	for _, c := range strings.Split(d, "/") {
		if c == ".." || c == "" || c == "." || strings.HasPrefix(c, ".") || strings.ContainsAny(c, ":\x00") {
			return "", errors.New("недопустимый подкаталог сайта")
		}
	}
	return d, nil
}

// ---------- фильтр и приёмник файлов ----------

// Расширения -> тип содержимого. Только статика; ничего исполняемого на сервере.
var contentTypes = map[string]string{
	".html": "text/html; charset=utf-8", ".htm": "text/html; charset=utf-8",
	".css": "text/css; charset=utf-8", ".js": "application/javascript; charset=utf-8",
	".mjs":  "application/javascript; charset=utf-8",
	".json": "application/json", ".webmanifest": "application/manifest+json",
	".txt": "text/plain; charset=utf-8", ".xml": "application/xml", ".csv": "text/csv; charset=utf-8",
	".svg": "image/svg+xml", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".gif": "image/gif", ".webp": "image/webp", ".avif": "image/avif", ".ico": "image/x-icon",
	".bmp":  "image/bmp",
	".woff": "font/woff", ".woff2": "font/woff2", ".ttf": "font/ttf", ".otf": "font/otf",
	".pdf": "application/pdf",
}

// Имена, которые не раздаются, даже если расширение допустимо.
var blockedNames = map[string]bool{
	"package.json": true, "package-lock.json": true, "composer.json": true, "composer.lock": true,
	"tsconfig.json": true, "yarn.lock": true,
}

func contentTypeFor(name string) (string, bool) {
	ct, ok := contentTypes[strings.ToLower(path.Ext(name))]
	return ct, ok
}

// SupportedExtensions — перечень раздаваемых расширений (для документации и сообщений).
func SupportedExtensions() []string {
	out := make([]string, 0, len(contentTypes))
	for e := range contentTypes {
		out = append(out, e)
	}
	sort.Strings(out)
	return out
}

func servable(rel string) bool {
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, ".") || p == "node_modules" {
			return false
		}
		if i == len(parts)-1 && blockedNames[strings.ToLower(p)] {
			return false
		}
	}
	_, ok := contentTypeFor(rel)
	return ok
}

var errSlip = errors.New("архив содержит недопустимый путь (выход за пределы каталога)")

// splitName проверяет сырое имя из архива/каталога и возвращает компоненты без «.».
func splitName(name string) ([]string, error) {
	if strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") {
		return nil, errSlip
	}
	var out []string
	for _, c := range strings.Split(name, "/") {
		switch c {
		case "", ".":
		case "..":
			return nil, errSlip
		default:
			out = append(out, c)
		}
	}
	return out, nil
}

type sink struct {
	root    string
	lim     Limits
	dir     string // подкаталог-префикс (уже очищен)
	files   int
	total   int64
	skipped int
}

// add принимает файл с относительным (от корня репозитория) путём parts.
func (k *sink) add(parts []string, size int64, r io.Reader) error {
	if k.dir != "" {
		pre := strings.Split(k.dir, "/")
		if len(parts) <= len(pre) {
			return nil
		}
		for i, p := range pre {
			if parts[i] != p {
				return nil
			}
		}
		parts = parts[len(pre):]
	}
	rel := strings.Join(parts, "/")
	if rel == "" || !servable(rel) {
		k.skipped++
		return nil
	}
	if size > k.lim.MaxFile {
		return fmt.Errorf("файл %s больше лимита %d МБ", rel, k.lim.MaxFile>>20)
	}
	if k.files+1 > k.lim.MaxFiles {
		return fmt.Errorf("в репозитории больше %d файлов сайта", k.lim.MaxFiles)
	}
	dst := filepath.Join(k.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(r, k.lim.MaxFile+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if n > k.lim.MaxFile {
		return fmt.Errorf("файл %s больше лимита %d МБ", rel, k.lim.MaxFile>>20)
	}
	k.total += n
	if k.total > k.lim.MaxTotal {
		return fmt.Errorf("сайт больше лимита %d МБ", k.lim.MaxTotal>>20)
	}
	k.files++
	return nil
}

// ---------- извлечение архивов ----------

// extractTar читает tar.gz; первый компонент пути (каталог, который создают GitHub/GitLab/Gitea)
// отбрасывается. Возвращает имя этого каталога.
func (k *sink) extractTar(r io.Reader) (string, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return "", fmt.Errorf("архив не tar.gz: %w", err)
	}
	defer zr.Close()
	tr := tar.NewReader(&capReader{r: zr, left: k.lim.MaxTotal*4 + 1<<20})
	top := ""
	for n := 0; ; n++ {
		if n > 50000 {
			return top, errors.New("в архиве слишком много записей")
		}
		h, err := tr.Next()
		if err == io.EOF {
			return top, nil
		}
		if err != nil {
			return top, fmt.Errorf("повреждённый архив: %w", err)
		}
		switch h.Typeflag {
		case tar.TypeXGlobalHeader, tar.TypeXHeader, tar.TypeDir:
			continue
		case tar.TypeReg, tar.TypeRegA:
		default: // ссылки, устройства и прочее — не берём
			if _, err := splitName(h.Name); err != nil {
				return top, err
			}
			k.skipped++
			continue
		}
		parts, err := splitName(h.Name)
		if err != nil {
			return top, err
		}
		if len(parts) < 2 {
			k.skipped++
			continue
		}
		if top == "" {
			top = parts[0]
		}
		if err := k.add(parts[1:], h.Size, tr); err != nil {
			return top, err
		}
	}
}

func (k *sink) extractZip(zr *zip.Reader) (string, error) {
	if len(zr.File) > 50000 {
		return "", errors.New("в архиве слишком много записей")
	}
	top := ""
	for _, f := range zr.File {
		parts, err := splitName(f.Name)
		if err != nil {
			return top, err
		}
		mode := f.FileInfo().Mode()
		if f.FileInfo().IsDir() || strings.HasSuffix(f.Name, "/") {
			continue
		}
		if !mode.IsRegular() {
			k.skipped++
			continue
		}
		if len(parts) < 2 {
			k.skipped++
			continue
		}
		if top == "" {
			top = parts[0]
		}
		if int64(f.UncompressedSize64) > k.lim.MaxFile && servable(strings.Join(parts[1:], "/")) {
			return top, fmt.Errorf("файл %s больше лимита %d МБ", strings.Join(parts[1:], "/"), k.lim.MaxFile>>20)
		}
		rc, err := f.Open()
		if err != nil {
			return top, err
		}
		err = k.add(parts[1:], int64(f.UncompressedSize64), rc)
		rc.Close()
		if err != nil {
			return top, err
		}
	}
	return top, nil
}

type capReader struct {
	r    io.Reader
	left int64
}

func (c *capReader) Read(p []byte) (int, error) {
	if c.left <= 0 {
		return 0, errors.New("архив слишком велик после распаковки")
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	return n, err
}

// ---------- загрузка ----------

var hexRe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// FetchRepo забирает репозиторий и кладёт отфильтрованную статику в новый каталог внутри workDir.
// При любой ошибке всё временное удаляется.
func FetchRepo(ctx context.Context, sp RepoSpec, workDir string, opt FetchOptions) (*Snapshot, error) {
	sp, err := validateSpec(sp, opt.AllowFile)
	if err != nil {
		return nil, err
	}
	if opt.Limits.MaxFiles == 0 {
		opt.Limits = DefaultLimits()
	}
	ctx, cancel := context.WithTimeout(ctx, opt.Limits.Timeout)
	defer cancel()
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, err
	}
	snapDir, err := os.MkdirTemp(workDir, "snap-")
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(snapDir)
		}
	}()
	k := &sink{root: snapDir, lim: opt.Limits, dir: sp.Dir}
	snap := &Snapshot{Dir: snapDir}

	gitBin := opt.GitBin
	if gitBin == "" && !opt.NoGit {
		gitBin, _ = exec.LookPath("git")
	}
	if opt.NoGit {
		gitBin = ""
	}
	if gitBin != "" {
		snap.Method = "git"
		snap.Commit, err = fetchGit(ctx, gitBin, sp, workDir, k, opt)
	} else {
		snap.Method = "archive"
		snap.Commit, err = fetchArchive(ctx, sp, workDir, k, opt)
	}
	if err != nil {
		return nil, scrub(err, sp.Token)
	}
	if st, err := os.Lstat(filepath.Join(snapDir, "index.html")); err != nil || !st.Mode().IsRegular() {
		d := "корне репозитория"
		if sp.Dir != "" {
			d = "подкаталоге " + sp.Dir
		}
		return nil, fmt.Errorf("в %s нет файла index.html", d)
	}
	snap.Files, snap.Bytes, snap.Skipped = k.files, k.total, k.skipped
	ok = true
	return snap, nil
}

func scrub(err error, token string) error {
	if token == "" {
		return err
	}
	return errors.New(strings.ReplaceAll(err.Error(), token, "***"))
}

func blockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast()
}

func (o FetchOptions) client() *http.Client {
	if o.HTTPClient != nil {
		return o.HTTPClient
	}
	d := &net.Dialer{Timeout: 15 * time.Second}
	if !o.AllowPrivate {
		d.Control = func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if ip := net.ParseIP(host); ip == nil || blockedIP(ip) {
				return errors.New("адрес из внутренней сети запрещён")
			}
			return nil
		}
	}
	tr := &http.Transport{DialContext: d.DialContext, TLSHandshakeTimeout: 15 * time.Second, ResponseHeaderTimeout: 30 * time.Second, Proxy: http.ProxyFromEnvironment}
	return &http.Client{Transport: tr, CheckRedirect: checkRedirect}
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("слишком много перенаправлений")
	}
	if req.URL.Scheme != "https" {
		return errors.New("перенаправление на небезопасный адрес")
	}
	if req.URL.Host != via[0].URL.Host { // токен не уходит на другой хост
		for _, h := range []string{"Authorization", "Private-Token"} {
			req.Header.Del(h)
		}
	}
	return nil
}

// archiveRequest строит адрес архива и заголовки для известных хостингов.
func archiveRequest(sp RepoSpec) (string, http.Header, error) {
	u, err := url.Parse(sp.URL)
	if err != nil {
		return "", nil, err
	}
	host := strings.ToLower(u.Hostname())
	p := strings.Trim(strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git"), "/")
	segs := strings.Split(p, "/")
	hdr := http.Header{}
	switch {
	case host == "github.com" || host == "www.github.com":
		if len(segs) != 2 {
			return "", nil, errors.New("адрес GitHub должен быть вида https://github.com/владелец/репозиторий")
		}
		a := "https://api.github.com/repos/" + url.PathEscape(segs[0]) + "/" + url.PathEscape(segs[1]) + "/tarball"
		if sp.Ref != "" {
			a += "/" + sp.Ref
		}
		hdr.Set("Accept", "application/vnd.github+json")
		if sp.Token != "" {
			hdr.Set("Authorization", "Bearer "+sp.Token)
		}
		return a, hdr, nil
	case host == "gitlab.com" || strings.HasPrefix(host, "gitlab."):
		if len(segs) < 2 {
			return "", nil, errors.New("адрес GitLab должен быть вида https://хост/группа/проект")
		}
		a := "https://" + u.Host + "/api/v4/projects/" + url.PathEscape(p) + "/repository/archive.tar.gz"
		if sp.Ref != "" {
			a += "?sha=" + url.QueryEscape(sp.Ref)
		}
		if sp.Token != "" {
			hdr.Set("PRIVATE-TOKEN", sp.Token)
		}
		return a, hdr, nil
	case host == "codeberg.org" || strings.Contains(host, "gitea") || strings.Contains(host, "forgejo"):
		if len(segs) != 2 {
			return "", nil, errors.New("адрес Gitea должен быть вида https://хост/владелец/репозиторий")
		}
		if sp.Ref == "" {
			return "", nil, errors.New("для Gitea/Forgejo укажите ветку или тег")
		}
		a := "https://" + u.Host + "/api/v1/repos/" + url.PathEscape(segs[0]) + "/" + url.PathEscape(segs[1]) + "/archive/" + url.PathEscape(sp.Ref) + ".tar.gz"
		if sp.Token != "" {
			hdr.Set("Authorization", "token "+sp.Token)
		}
		return a, hdr, nil
	}
	return "", nil, fmt.Errorf("хостинг %s не поддерживается без git: поддерживаются GitHub, GitLab и Gitea/Forgejo; для остальных установите git на сервере", host)
}

func fetchArchive(ctx context.Context, sp RepoSpec, workDir string, k *sink, opt FetchOptions) (string, error) {
	build := archiveRequest
	if opt.ArchiveURL != nil {
		build = opt.ArchiveURL
	}
	au, hdr, err := build(sp)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, au, nil)
	if err != nil {
		return "", err
	}
	for h, v := range hdr {
		req.Header[h] = v
	}
	req.Header.Set("User-Agent", "veil-site-fetch")
	resp, err := opt.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("загрузка архива: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404:
		return "", fmt.Errorf("репозиторий не найден или нет доступа (HTTP %d): проверьте адрес, ветку и токен", resp.StatusCode)
	case resp.StatusCode != 200:
		return "", fmt.Errorf("сервер репозитория ответил HTTP %d", resp.StatusCode)
	}
	tmp, err := os.CreateTemp(workDir, "dl-")
	if err != nil {
		return "", err
	}
	defer func() { tmp.Close(); os.Remove(tmp.Name()) }()
	h := sha256.New()
	max := k.lim.MaxTotal*2 + 1<<20
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(resp.Body, max+1))
	if err != nil {
		return "", fmt.Errorf("загрузка архива: %w", err)
	}
	if n > max {
		return "", errors.New("архив больше допустимого размера")
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	magic := make([]byte, 2)
	if _, err := io.ReadFull(tmp, magic); err != nil {
		return "", errors.New("пустой или повреждённый архив")
	}
	tmp.Seek(0, io.SeekStart)
	var top string
	if magic[0] == 'P' && magic[1] == 'K' {
		zr, err := zip.NewReader(tmp, n)
		if err != nil {
			return "", fmt.Errorf("повреждённый zip: %w", err)
		}
		top, err = k.extractZip(zr)
		if err != nil {
			return "", err
		}
	} else {
		top, err = k.extractTar(tmp)
		if err != nil {
			return "", err
		}
	}
	if i := strings.LastIndexByte(top, '-'); i >= 0 && hexRe.MatchString(top[i+1:]) {
		return top[i+1:], nil
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))[:12], nil
}

func fetchGit(ctx context.Context, gitBin string, sp RepoSpec, workDir string, k *sink, opt FetchOptions) (string, error) {
	if !opt.AllowFile {
		u, _ := url.Parse(sp.URL)
		if !opt.AllowPrivate {
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", u.Hostname())
			if err != nil {
				return "", fmt.Errorf("не удалось определить адрес %s", u.Hostname())
			}
			for _, ip := range ips {
				if blockedIP(ip) {
					return "", errors.New("адрес из внутренней сети запрещён")
				}
			}
		}
	}
	cloneParent, err := os.MkdirTemp(workDir, "clone-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(cloneParent)
	dest := filepath.Join(cloneParent, "repo")
	home := filepath.Join(cloneParent, "home")
	os.MkdirAll(home, 0o700)
	os.MkdirAll(filepath.Join(cloneParent, "nohooks"), 0o700)
	_ = os.WriteFile(filepath.Join(cloneParent, "gitconfig"), nil, 0o600)

	proto := "https"
	if opt.AllowFile {
		proto = "file"
	}
	args := []string{
		"-c", "core.hooksPath=" + filepath.Join(cloneParent, "nohooks"),
		"-c", "protocol.allow=never", "-c", "protocol." + proto + ".allow=always",
		"-c", "core.symlinks=false", "-c", "core.fsmonitor=false",
		"-c", "filter.lfs.smudge=", "-c", "filter.lfs.process=", "-c", "filter.lfs.required=false",
		"-c", "http.followRedirects=initial",
		"-c", "http.lowSpeedLimit=1000", "-c", "http.lowSpeedTime=30",
		"clone", "--depth", "1", "--single-branch", "--no-tags", "--no-recurse-submodules",
	}
	if sp.Ref != "" {
		args = append(args, "--branch", sp.Ref)
	}
	args = append(args, "--", sp.URL, dest)
	cmd := exec.CommandContext(ctx, gitBin, args...)
	env := []string{
		"GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + filepath.Join(cloneParent, "gitconfig"),
		"HOME=" + home, "USERPROFILE=" + home, "XDG_CONFIG_HOME=" + home,
		"GIT_ASKPASS=", "SSH_ASKPASS=", "GIT_LFS_SKIP_SMUDGE=1",
		"PATH=" + os.Getenv("PATH"), "SystemRoot=" + os.Getenv("SystemRoot"),
	}
	for _, e := range []string{"HTTPS_PROXY", "https_proxy", "NO_PROXY", "no_proxy", "SSL_CERT_FILE", "SSL_CERT_DIR", "TEMP", "TMP"} {
		if v := os.Getenv(e); v != "" {
			env = append(env, e+"="+v)
		}
	}
	if sp.Token != "" { // токен — в переменных окружения процесса, не в аргументах и не в адресе
		auth := base64.StdEncoding.EncodeToString([]byte("oauth2:" + sp.Token))
		env = append(env, "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.extraHeader", "GIT_CONFIG_VALUE_0=Authorization: Basic "+auth)
	}
	cmd.Env = env
	var out limitedBuf
	cmd.Stdout, cmd.Stderr = &out, &out

	// страховка от огромного репозитория: следим за размером каталога клона
	wctx, wcancel := context.WithCancel(ctx)
	defer wcancel()
	tooBig := make(chan struct{})
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-wctx.Done():
				return
			case <-t.C:
				if dirSize(dest) > k.lim.MaxTotal*4+(64<<20) {
					close(tooBig)
					if cmd.Process != nil {
						cmd.Process.Kill()
					}
					return
				}
			}
		}
	}()
	err = cmd.Run()
	wcancel()
	select {
	case <-tooBig:
		return "", errors.New("репозиторий слишком велик")
	default:
	}
	if err != nil {
		if ctx.Err() != nil {
			return "", errors.New("превышено время загрузки репозитория")
		}
		msg := strings.TrimSpace(out.String())
		if i := strings.LastIndexByte(msg, '\n'); i >= 0 {
			msg = msg[i+1:]
		}
		return "", fmt.Errorf("git clone: %s", msg)
	}
	commit := ""
	rp := exec.CommandContext(ctx, gitBin, "-C", dest, "rev-parse", "HEAD")
	rp.Env = env
	if b, err := rp.Output(); err == nil {
		commit = strings.TrimSpace(string(b))
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return "", err
	}
	defer root.Close()
	err = fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != "." && (strings.HasPrefix(path.Base(p), ".") || path.Base(p) == "node_modules") {
				return fs.SkipDir
			}
			return nil
		}
		parts, err := splitName(p)
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() { // симлинки и прочее не берём
			k.skipped++
			return nil
		}
		f, err := root.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil {
			return err
		}
		return k.add(parts, st.Size(), f)
	})
	if err != nil {
		return "", err
	}
	return commit, nil
}

type limitedBuf struct{ b []byte }

func (l *limitedBuf) Write(p []byte) (int, error) {
	if len(l.b) < 8192 {
		l.b = append(l.b, p...)
	}
	return len(p), nil
}
func (l *limitedBuf) String() string { return string(l.b) }

func dirSize(dir string) int64 {
	var n int64
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if fi, e := d.Info(); e == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return n
}

// ---------- раздача ----------

type treeFile struct {
	abs  string
	ct   string
	size int64
}

// Tree — неизменяемый индекс файлов снимка. Раздаются только файлы из индекса,
// так что обращение к файловой системе по пути из запроса невозможно.
type Tree struct {
	files map[string]treeFile
	dirs  map[string]bool
	Files int
	Bytes int64
}

// LoadTree строит индекс по каталогу снимка. reserved — пути, которые сайт не может занять
// (админка, туннель); такие файлы в индекс не попадают.
func LoadTree(root string, reserved func(urlPath string) bool) (*Tree, error) {
	t := &Tree{files: map[string]treeFile{}, dirs: map[string]bool{}}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil // ссылки и прочее не раздаём
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		ct, ok := contentTypeFor(rel)
		if !ok || !servable(rel) {
			return nil
		}
		up := "/" + rel
		if reserved != nil && reserved(up) {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		t.files[up] = treeFile{abs: p, ct: ct, size: fi.Size()}
		t.Files++
		t.Bytes += fi.Size()
		for dir := path.Dir(up); dir != "/" && dir != "."; dir = path.Dir(dir) {
			t.dirs[dir] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if _, ok := t.files["/index.html"]; !ok {
		return nil, errors.New("в снимке нет index.html")
	}
	return t, nil
}

// IsDir сообщает, что urlPath (без завершающего «/») — каталог с index.html.
func (t *Tree) IsDir(urlPath string) bool {
	if !t.dirs[urlPath] {
		return false
	}
	_, ok := t.files[urlPath+"/index.html"]
	return ok
}

// Has сообщает, есть ли такой файл (или index.html каталога).
func (t *Tree) Has(urlPath string) bool {
	_, ok := t.lookup(urlPath)
	return ok
}

func (t *Tree) lookup(urlPath string) (treeFile, bool) {
	if strings.HasSuffix(urlPath, "/") {
		urlPath += "index.html"
	}
	f, ok := t.files[urlPath]
	return f, ok
}

// Serve отдаёт файл тем же набором заголовков, что и сгенерированный сайт.
// Возвращает false, если такого файла нет (ответ формирует вызывающий).
func (t *Tree) Serve(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	f, ok := t.lookup(r.URL.Path)
	if !ok {
		return false
	}
	file, err := os.Open(f.abs)
	if err != nil {
		return false
	}
	defer file.Close()
	h := w.Header()
	h.Set("Content-Type", f.ct)
	if strings.HasPrefix(f.ct, "text/html") {
		h.Set("Cache-Control", "max-age=600")
	} else {
		h.Set("Cache-Control", "max-age=86400")
	}
	h.Set("Content-Length", strconv.FormatInt(f.size, 10))
	if r.Method == http.MethodGet {
		_, _ = io.Copy(w, io.LimitReader(file, f.size))
	}
	return true
}
