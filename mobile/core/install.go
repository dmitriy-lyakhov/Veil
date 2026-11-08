package veilcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"

	"veil/internal/config"
	"veil/internal/deploy"
)

// releaseRepo — репозиторий GitHub, из релизов которого сервер скачивает свои файлы при установке с телефона.
const releaseRepo = "dmitriy-lyakhov/Veil"

// InstallListener получает ход установки сервера (вызывается из рабочего потока Go).
type InstallListener interface {
	// OnStep — шаг установки: id (ssh, system, docker, download, files, build, start, client, profile), заголовок,
	// подробности и состояние: active, done, fail, skip.
	OnStep(id, title, detail, state string)
	// OnLog — строка журнала.
	OnLog(line string)
}

type installRequest struct {
	Name       string `json:"name"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	Password   string `json:"password"`
	Key        string `json:"key"`        // закрытый ключ OpenSSH (PEM), если вход по ключу
	Passphrase string `json:"passphrase"` // парольная фраза ключа
	Protocol   string `json:"protocol"`   // veil, wireguard, amneziawg
	Admin      bool   `json:"admin"`      // Veil: поднять админ-панель в контейнере veil-admin
	Client     string `json:"client"`     // имя клиента этого устройства на сервере
	Version    string `json:"version"`    // версия приложения: из её релиза сервер скачивает файлы
	HostKey    string `json:"hostkey"`    // ключ хоста, запомненный при прошлой установке на этот адрес (пусто — первое подключение)
}

type installContainer struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	State    string `json:"state"`
	Ports    string `json:"ports"`
}

type installResult struct {
	Protocol   string             `json:"protocol"`
	Existing   bool               `json:"existing"`   // протокол уже стоял: выдан только новый клиент
	Config     string             `json:"config"`     // ссылка veil:// или текст .conf — импортируется как обычно
	Client     string             `json:"client"`     // имя выданного клиента
	HostKey    string             `json:"hostkey"`    // ключ хоста SSH: телефон запоминает его для следующих установок
	Containers []installContainer `json:"containers"` // контейнеры Veil на сервере
}

var (
	installMu     sync.Mutex
	installCancel context.CancelFunc
)

// Install ставит протокол на сервер по SSH (или, если он уже стоит, только выдаёт клиента) и возвращает JSON
// installResult. Блокирует до конца установки: вызывать из фонового потока. Пароль и ключ нигде не сохраняются.
// Файлы сервера (veil-server, wireguard-go, amneziawg-go) сервер скачивает сам из GitHub-релиза версии приложения
// и проверяет по SHA256SUMS.txt. Отмена — CancelInstall.
func Install(reqJSON string, l InstallListener) (string, error) {
	var in installRequest
	if err := json.Unmarshal([]byte(reqJSON), &in); err != nil {
		return "", errors.New(ErrFormat)
	}
	host := strings.Trim(strings.TrimSpace(in.Host), "[]")
	if host == "" {
		return "", errors.New("укажите адрес сервера")
	}
	if in.Password == "" && strings.TrimSpace(in.Key) == "" {
		return "", errors.New("укажите пароль SSH или ключ")
	}
	proto := strings.ToLower(strings.TrimSpace(in.Protocol))
	switch proto {
	case "veil", "wireguard", "amneziawg":
	default:
		return "", fmt.Errorf("протокол %q не поддерживается на Android", in.Protocol)
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = host
	}
	req := deploy.Request{
		Name: name, SSHPort: in.Port, User: strings.TrimSpace(in.User),
		Password: in.Password, KeyPassphrase: in.Passphrase,
		AdminPanel: in.Admin, ClientName: strings.TrimSpace(in.Client),
		Release: deploy.GitHubRelease(releaseRepo, in.Version), AddClientIfRunning: true,
		KnownHostKey: strings.TrimSpace(in.HostKey),
	}
	if k := strings.TrimSpace(in.Key); k != "" {
		req.KeyPEM = []byte(k + "\n")
	}
	if a, err := netip.ParseAddr(host); err == nil && a.Is6() {
		req.IPv6 = a.String()
	} else {
		req.IPv4 = host // IPv4 или доменное имя
	}
	if l != nil {
		req.Progress = func(s deploy.Step) { l.OnStep(s.ID, s.Title, s.Detail, s.State) }
	}
	logf := func(format string, args ...any) {
		if l != nil {
			l.OnLog(fmt.Sprintf(format, args...))
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	installMu.Lock()
	if installCancel != nil {
		installMu.Unlock()
		cancel()
		return "", errors.New("установка уже идёт")
	}
	installCancel = cancel
	installMu.Unlock()
	defer func() {
		installMu.Lock()
		installCancel = nil
		installMu.Unlock()
		cancel()
	}()

	out := installResult{Protocol: proto, Containers: []installContainer{}}
	var containers []deploy.ContainerInfo
	if proto == "veil" {
		res, err := deploy.DeployVeil(ctx, req, logf)
		if err != nil {
			return "", cancelled(ctx, err)
		}
		link, err := config.EncodeLink(res.Profile)
		if err != nil {
			return "", err
		}
		out.Existing, out.Config, out.Client, containers = res.Existing, link, res.Client, res.Containers
		if res.Profile.SSH != nil {
			out.HostKey = res.Profile.SSH.HostKey
		}
	} else {
		res, err := deploy.DeployProtocol(ctx, req, proto, 0, logf)
		if err != nil {
			return "", cancelled(ctx, err)
		}
		if res.WG == nil {
			return "", errors.New("сервер не выдал конфигурацию клиента")
		}
		out.Existing, out.Config, out.Client = res.Existing, res.WG.String(), req.ClientName
		if res.SSH != nil {
			out.HostKey = res.SSH.HostKey
		}
		if res.Container != "" {
			containers = append(containers, deploy.ContainerInfo{Name: res.Container, Protocol: proto, State: "running"})
		}
		containers = append(containers, res.Others...)
	}
	for _, c := range containers {
		out.Containers = append(out.Containers, installContainer{Name: c.Name, Protocol: c.Protocol, State: c.State, Ports: c.Ports})
	}
	b, err := json.Marshal(out)
	return string(b), err
}

// CancelInstall прерывает идущую установку (текущая команда на сервере доработает, следующие не начнутся).
func CancelInstall() {
	installMu.Lock()
	defer installMu.Unlock()
	if installCancel != nil {
		installCancel()
	}
}

// ErrSSHAuth — сервер не принял пароль или ключ SSH.
const ErrSSHAuth = "E_SSH_AUTH"

// ErrSSHHostKey — ключ хоста сервера не совпал с сохранённым при первом подключении.
const ErrSSHHostKey = "E_SSH_HOSTKEY"

func cancelled(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return errors.New("установка отменена")
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "unable to authenticate"), strings.Contains(msg, "no supported methods remain"):
		return errors.New(ErrSSHAuth)
	case strings.Contains(msg, "ключ хоста изменился"):
		return errors.New(ErrSSHHostKey)
	}
	return err
}
