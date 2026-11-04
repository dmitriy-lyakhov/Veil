package deploy

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"veil/internal/config"
)

const controlSocket = "/run/veil/ctl.sock"

var ErrOutdated = errors.New("на сервере старая версия Veil без управления — нажмите «Обновить сервер»")

type Session struct {
	Profile *config.Profile
	r       *remote
	arch    string
}

func NeedsPassword(p *config.Profile) bool {
	return p.SSH != nil && p.SSH.KeyPath == ""
}

func Open(ctx context.Context, p *config.Profile, password, passphrase string) (*Session, error) {
	if p.SSH == nil {
		return nil, errors.New("этот сервер добавлен по файлу конфигурации — управлять им можно только из приложения, которое его установило")
	}
	req := Request{
		IPv4: p.IPv4, IPv6: p.IPv6, SSHPort: p.SSH.Port, User: p.SSH.User,
		Password: password, KeyPath: p.SSH.KeyPath, KeyPassphrase: passphrase, KnownHostKey: p.SSH.HostKey,
	}
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	client, _, err := connect(ctx, req, func(string, ...any) {})
	if err != nil {
		return nil, err
	}
	r := &remote{c: client, password: password}
	uid, err := r.exec("id -u", nil, false)
	if err != nil {
		client.Close()
		return nil, err
	}
	r.root = uid == "0"
	if !r.root {
		if _, err := r.exec("true", nil, true); err != nil {
			client.Close()
			return nil, fmt.Errorf("пользователь %s не может выполнить sudo: %w", req.User, err)
		}
	}
	return &Session{Profile: p, r: r}, nil
}

func (s *Session) Close() { _ = s.r.c.Close() }

func (s *Session) Call(method, path string, in, out any) error {
	if _, err := s.r.exec("test -S "+controlSocket, nil, true); err != nil {
		return ErrOutdated
	}
	cmd := "veil-server ctl " + method + " " + shellQuote(path)
	if in != nil {
		body, err := json.Marshal(in)
		if err != nil {
			return err
		}
		cmd += " " + base64.StdEncoding.EncodeToString(body)
	}
	stdout, err := s.r.exec(cmd, nil, true)
	if err != nil {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal([]byte(stdout), &e) == nil && e.Error != "" {
			return errors.New(e.Error)
		}
		if strings.Contains(err.Error(), "usage") || strings.Contains(err.Error(), "flag provided but not defined") {
			return ErrOutdated
		}
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal([]byte(stdout), out)
}

func (s *Session) Update(ctx context.Context, binDir string, logf Logf) error {
	machine, err := s.r.exec("uname -sm", nil, false)
	if err != nil {
		return err
	}
	arch, err := goArch(machine)
	if err != nil {
		return err
	}
	bin, err := ServerBinary(binDir, arch)
	if err != nil {
		return err
	}
	logf("Загрузка серверного бинарника (%d КБ)…", len(bin)/1024)
	if err := s.r.upload(bin, remoteBinary, "0755"); err != nil {
		return err
	}
	s.r.openFirewall(logf)
	logf("Перезапуск службы veil…")
	if _, err := s.r.exec("systemctl restart veil", nil, true); err != nil {
		return err
	}
	if err := s.r.waitActive(); err != nil {
		journal, _ := s.r.exec("journalctl -u veil -n 30 --no-pager", nil, true)
		return fmt.Errorf("служба не запустилась: %w\n%s", err, journal)
	}
	logf("Сервер обновлён")
	return ctx.Err()
}
