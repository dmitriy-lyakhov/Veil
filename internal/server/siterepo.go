package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"veil/internal/config"
	"veil/internal/site"
)

const siteRepoInterval = 6 * time.Hour

// SiteRepoInfo — состояние сайта-прикрытия из репозитория (токен не отдаётся).
type SiteRepoInfo struct {
	Enabled  bool       `json:"enabled"`
	URL      string     `json:"url,omitempty"`
	Ref      string     `json:"ref,omitempty"`
	Dir      string     `json:"dir,omitempty"`
	HasToken bool       `json:"has_token,omitempty"`
	Active   bool       `json:"active"` // раздаётся сайт из репозитория (иначе — сгенерированный)
	Syncing  bool       `json:"syncing,omitempty"`
	Commit   string     `json:"commit,omitempty"`
	Updated  *time.Time `json:"updated,omitempty"` // когда получена последняя удачная версия
	Checked  *time.Time `json:"checked,omitempty"` // последняя попытка обновления
	Error    string     `json:"error,omitempty"`
	Files    int        `json:"files,omitempty"`
	Bytes    int64      `json:"bytes,omitempty"`
}

type siteRepoMeta struct {
	Dir         string    `json:"dir"`
	Fingerprint string    `json:"fingerprint"`
	Commit      string    `json:"commit"`
	Updated     time.Time `json:"updated"`
	Files       int       `json:"files"`
	Bytes       int64     `json:"bytes"`
}

type siteRepoManager struct {
	s       *Server
	log     *slog.Logger
	dir     string
	ownsDir bool
	opt     site.FetchOptions

	mu      sync.Mutex
	spec    site.RepoSpec
	gen     int
	cancel  context.CancelFunc // отмена текущей загрузки
	syncing bool
	info    SiteRepoInfo
	curDir  string
	trigger chan struct{}
}

func fingerprint(sp site.RepoSpec) string { return sp.URL + "\x00" + sp.Ref + "\x00" + sp.Dir }

// newSiteRepoManager создаёт менеджер и запускает фоновое обновление до отмены ctx.
func newSiteRepoManager(s *Server, cfgPath string, log *slog.Logger) *siteRepoManager {
	m := &siteRepoManager{s: s, log: log, trigger: make(chan struct{}, 1)}
	if cfgPath != "" {
		m.dir = filepath.Join(filepath.Dir(cfgPath), "siterepo")
	} else if d, err := os.MkdirTemp("", "veil-siterepo-"); err == nil {
		m.dir, m.ownsDir = d, true
	}
	if s.cfg.SiteRepo != nil && s.cfg.SiteRepo.URL != "" {
		sp := site.RepoSpec{URL: s.cfg.SiteRepo.URL, Ref: s.cfg.SiteRepo.Ref, Dir: s.cfg.SiteRepo.Dir, Token: s.cfg.SiteRepo.Token}
		if v, err := site.ValidateSpec(sp); err == nil {
			m.spec = v
		} else {
			log.Warn("site repo config", "err", err)
		}
	}
	s.web.reserved = m.reserved
	m.setInfoLocked()
	m.loadCached()
	return m
}

// start запускает фоновое обновление до отмены ctx.
func (m *siteRepoManager) start(ctx context.Context) { go m.loop(ctx) }

// reserved — пути, которые сайт из репозитория занять не может: админка и туннель.
func (m *siteRepoManager) reserved(p string) bool {
	if strings.HasPrefix(p, adminPrefix) {
		return true
	}
	m.s.mu.Lock()
	tp := m.s.cfg.Path
	m.s.mu.Unlock()
	return tp != "" && strings.HasPrefix(p, tp)
}

// setInfoLocked пересобирает описание настройки (вызывать под m.mu или до запуска).
func (m *siteRepoManager) setInfoLocked() {
	keep := m.info
	m.info = SiteRepoInfo{
		Enabled: m.spec.URL != "", URL: m.spec.URL, Ref: m.spec.Ref, Dir: m.spec.Dir, HasToken: m.spec.Token != "",
		Active: keep.Active, Syncing: m.syncing, Commit: keep.Commit, Updated: keep.Updated, Checked: keep.Checked,
		Error: keep.Error, Files: keep.Files, Bytes: keep.Bytes,
	}
}

func (m *siteRepoManager) get() SiteRepoInfo {
	if m == nil {
		return SiteRepoInfo{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.info
}

func (m *siteRepoManager) metaPath() string { return filepath.Join(m.dir, "current.json") }

// loadCached поднимает последнюю удачную версию с диска (если она от этой же настройки).
func (m *siteRepoManager) loadCached() {
	if m.dir == "" || m.spec.URL == "" {
		return
	}
	b, err := os.ReadFile(m.metaPath())
	if err != nil {
		return
	}
	var meta siteRepoMeta
	if json.Unmarshal(b, &meta) != nil || meta.Fingerprint != fingerprint(m.spec) || meta.Dir == "" || meta.Dir != filepath.Base(meta.Dir) {
		return
	}
	root := filepath.Join(m.dir, meta.Dir)
	tree, err := site.LoadTree(root, m.reserved)
	if err != nil {
		return
	}
	m.s.web.cover.Store(tree)
	m.curDir = meta.Dir
	t := meta.Updated
	m.info.Active, m.info.Commit, m.info.Updated, m.info.Files, m.info.Bytes = true, meta.Commit, &t, tree.Files, tree.Bytes
	m.prune()
}

// prune удаляет всё в каталоге кэша, кроме текущего снимка и его метаданных.
func (m *siteRepoManager) prune() {
	ents, err := os.ReadDir(m.dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		if e.Name() == "current.json" || e.Name() == m.curDir {
			continue
		}
		os.RemoveAll(filepath.Join(m.dir, e.Name()))
	}
}

func (m *siteRepoManager) loop(ctx context.Context) {
	defer func() {
		if m.ownsDir {
			os.RemoveAll(m.dir)
		}
	}()
	t := time.NewTicker(siteRepoInterval)
	defer t.Stop()
	m.sync(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-m.trigger:
		}
		m.sync(ctx)
	}
}

// configure применяет новую настройку и запускает немедленное обновление.
func (m *siteRepoManager) configure(sp site.RepoSpec) {
	m.mu.Lock()
	same := fingerprint(sp) == fingerprint(m.spec)
	m.spec = sp
	m.gen++
	if m.cancel != nil {
		m.cancel()
	}
	if !same {
		m.s.web.cover.Store(nil)
		m.curDir = ""
		m.info = SiteRepoInfo{}
		os.Remove(m.metaPath())
	}
	m.info.Error = ""
	m.setInfoLocked()
	m.mu.Unlock()
	m.poke()
}

// clear возвращает сгенерированный сайт и удаляет кэш.
func (m *siteRepoManager) clear() {
	m.mu.Lock()
	m.spec = site.RepoSpec{}
	m.gen++
	if m.cancel != nil {
		m.cancel()
	}
	m.s.web.cover.Store(nil)
	m.curDir = ""
	m.info = SiteRepoInfo{}
	m.mu.Unlock()
	if m.dir != "" {
		os.Remove(m.metaPath())
		m.prune()
	}
}

func (m *siteRepoManager) poke() {
	select {
	case m.trigger <- struct{}{}:
	default:
	}
}

func (m *siteRepoManager) sync(parent context.Context) {
	m.mu.Lock()
	if m.spec.URL == "" || m.dir == "" {
		m.mu.Unlock()
		return
	}
	sp, gen := m.spec, m.gen
	ctx, cancel := context.WithCancel(parent)
	m.cancel, m.syncing = cancel, true
	m.setInfoLocked()
	m.mu.Unlock()
	defer cancel()

	snap, err := site.FetchRepo(ctx, sp, m.dir, m.opt)
	var tree *site.Tree
	if err == nil {
		tree, err = site.LoadTree(snap.Dir, m.reserved)
		if err != nil {
			os.RemoveAll(snap.Dir)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncing = false
	if gen != m.gen { // настройку успели поменять — результат устарел
		if snap != nil && err == nil {
			os.RemoveAll(snap.Dir)
		}
		m.setInfoLocked()
		return
	}
	now := time.Now()
	m.info.Checked = &now
	if err != nil {
		m.info.Error = err.Error()
		m.log.Warn("site repo update failed", "url", sp.URL, "err", err)
		m.setInfoLocked()
		return
	}
	prev := m.curDir
	m.curDir = filepath.Base(snap.Dir)
	meta := siteRepoMeta{Dir: m.curDir, Fingerprint: fingerprint(sp), Commit: snap.Commit, Updated: now, Files: tree.Files, Bytes: tree.Bytes}
	if b, e := json.Marshal(meta); e == nil {
		_ = os.WriteFile(m.metaPath(), b, 0o600)
	}
	m.s.web.cover.Store(tree)
	m.info.Active, m.info.Error, m.info.Commit, m.info.Updated = true, "", snap.Commit, &now
	m.info.Files, m.info.Bytes = tree.Files, tree.Bytes
	m.setInfoLocked()
	m.log.Info("site repo updated", "url", sp.URL, "commit", snap.Commit, "files", tree.Files, "method", snap.Method)
	if prev != "" && prev != m.curDir { // запросы, которые ещё читают старый снимок, успеют закончиться
		old := filepath.Join(m.dir, prev)
		time.AfterFunc(2*time.Minute, func() { os.RemoveAll(old) })
	}
}

// ---------- методы сервера ----------

// SiteRepoInfo — состояние для панели и State().
func (s *Server) SiteRepoInfo() SiteRepoInfo { return s.siteRepo.get() }

// SetSiteRepo сохраняет настройку и запускает загрузку в фоне. Пустой url — вернуться к сгенерированному сайту.
// Пустой токен при том же адресе сохраняет прежний; clearToken его удаляет.
func (s *Server) SetSiteRepo(url, ref, dir, token string, clearToken bool) error {
	if s.siteRepo == nil {
		return errors.New("недоступно")
	}
	if strings.TrimSpace(url) == "" {
		s.mu.Lock()
		s.cfg.SiteRepo = nil
		err := s.saveLocked()
		s.mu.Unlock()
		if err != nil {
			return err
		}
		s.siteRepo.clear()
		return nil
	}
	sp, err := site.ValidateSpec(site.RepoSpec{URL: url, Ref: ref, Dir: dir, Token: token})
	if err != nil {
		return err
	}
	s.mu.Lock()
	if sp.Token == "" && !clearToken && s.cfg.SiteRepo != nil && s.cfg.SiteRepo.URL == sp.URL {
		sp.Token = s.cfg.SiteRepo.Token
	}
	if clearToken && token == "" {
		sp.Token = ""
	}
	s.cfg.SiteRepo = &config.SiteRepo{URL: sp.URL, Ref: sp.Ref, Dir: sp.Dir, Token: sp.Token}
	err = s.saveLocked()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	s.siteRepo.configure(sp)
	return nil
}

// RefreshSiteRepo просит обновить сайт немедленно (в фоне).
func (s *Server) RefreshSiteRepo() error {
	if s.siteRepo == nil || !s.siteRepo.get().Enabled {
		return errors.New("репозиторий сайта не задан")
	}
	s.siteRepo.poke()
	return nil
}
