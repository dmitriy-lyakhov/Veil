package main

// Команды для запуска без установщика по SSH (Docker, ручная установка):
//
//	veil-server init   — создать /etc/veil/server.json, если его нет, и выдать первого клиента;
//	veil-server client — добавить, перевыпустить или удалить клиента у запущенного сервера.

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"

	"veil/internal/config"
	"veil/internal/provision"
	"veil/internal/server"
	"veil/internal/transport/front"
)

const adminUser = "admin"

func initCmd(args []string) int {
	fset := flag.NewFlagSet("init", flag.ExitOnError)
	cfgPath := fset.String("config", "/etc/veil/server.json", "path to server config")
	ipv4 := fset.String("ipv4", "", "public IPv4 address of the server (goes into client configs)")
	ipv6 := fset.String("ipv6", "", "public IPv6 address of the server (goes into client configs)")
	lang := fset.String("lang", "ru", "cover site language: ru or en")
	frontSNI := fset.String("front", "", "mask the entry as this site (SNI), e.g. ya.ru")
	domain := fset.String("domain", "", "domain for the cover site and Let's Encrypt certificate")
	listen := fset.String("listen", "", "listen address, default :443")
	client := fset.String("client", "client", "name of the first client; empty — do not create")
	clientsDir := fset.String("clients-dir", "", "where to save client .veil files (default: <config dir>/clients)")
	admin := fset.String("admin", "", "web admin panel: container (separate veil-admin process), off, builtin; empty — keep")
	_ = fset.Parse(args)
	if *clientsDir == "" {
		*clientsDir = filepath.Join(filepath.Dir(*cfgPath), "clients")
	}
	if err := runInit(*cfgPath, *ipv4, *ipv6, *lang, *frontSNI, *domain, *listen, *client, *clientsDir); err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return 1
	}
	if err := setAdminMode(*cfgPath, *admin); err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return 1
	}
	return 0
}

// setAdminMode записывает режим веб-панели в конфиг (при каждом запуске контейнера — из VEIL_ADMIN).
func setAdminMode(cfgPath, mode string) error {
	switch mode {
	case "":
		return nil
	case "builtin":
		mode = config.AdminBuiltin
	case config.AdminContainer, config.AdminOff:
	default:
		return fmt.Errorf("-admin: container, off или builtin, а не %q", mode)
	}
	cfg, err := config.Load[config.Server](cfgPath)
	if err != nil {
		return err
	}
	if cfg.Admin.Mode == mode {
		return nil
	}
	cfg.Admin.Mode = mode
	fmt.Printf("veil: admin panel mode: %q\n", map[string]string{"": "builtin"}[mode]+mode)
	return config.Save(cfgPath, cfg)
}

func runInit(cfgPath, ipv4, ipv6, lang, frontSNI, domain, listen, client, clientsDir string) error {
	v4, v6, err := publicAddrs(ipv4, ipv6)
	if err != nil {
		return err
	}
	cfg, err := config.Load[config.Server](cfgPath)
	if err == nil {
		// конфиг уже есть (том с данными): обновляем только адреса, ключи и клиенты не трогаем
		changed := false
		if v4 != "" && v4 != cfg.PublicIPv4 {
			cfg.PublicIPv4, changed = v4, true
		}
		if v6 != "" && v6 != cfg.PublicIPv6 {
			cfg.PublicIPv6, changed = v6, true
		}
		if changed {
			fmt.Printf("veil: public addresses updated: ipv4=%q ipv6=%q\n", cfg.PublicIPv4, cfg.PublicIPv6)
			return config.Save(cfgPath, cfg)
		}
		fmt.Println("veil: existing config found, keeping it")
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if v4 == "" && v6 == "" {
		return errors.New("укажите публичный адрес сервера: -ipv4 и/или -ipv6 (VEIL_PUBLIC_IPV4 / VEIL_PUBLIC_IPV6)")
	}

	cfg, err = provision.NewServer(lang)
	if err != nil {
		return err
	}
	cfg.PublicIPv4, cfg.PublicIPv6 = v4, v6
	if listen != "" {
		cfg.Listen = listen
	}
	if sni, err := front.NormalizeSNI(frontSNI); err != nil {
		return err
	} else if sni != "" {
		key, err := front.NewKey()
		if err != nil {
			return err
		}
		cfg.Front = &config.Front{SNI: sni, Key: key}
	}
	if d, err := server.NormalizeDomain(domain); err != nil {
		return err
	} else {
		cfg.Domain = d
	}
	password := provision.NewAdminPassword()
	hash, err := provision.HashAdminPassword(password)
	if err != nil {
		return err
	}
	cfg.Admin = config.Admin{User: adminUser, PasswordHash: hash}

	var prof *config.Profile
	if client != "" {
		if prof, err = provision.AddPeer(cfg, client); err != nil {
			return err
		}
		prof.IPv4, prof.IPv6 = v4, v6
		prof.Port = cfg.Port()
		if cfg.Front != nil {
			prof.FrontSNI, prof.FrontKey = cfg.Front.SNI, cfg.Front.Key
		}
		prof.AdminURL = provision.AdminURL(cfg)
		prof.AdminUser, prof.AdminPassword = adminUser, password
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(cfgPath), "admin-password"), []byte(password+"\n"), 0o600); err != nil {
		return err
	}
	fmt.Printf("veil: new server config created: %s\n", cfgPath)
	fmt.Printf("veil: cover site «%s» — %s\n", cfg.Site.Company, cfg.Site.Tagline)
	fmt.Printf("veil: admin panel (only via VPN): %s, login %s, password %s\n", provision.AdminURL(cfg), adminUser, password)
	if prof != nil {
		link, err := config.EncodeLink(prof)
		if err != nil {
			return err
		}
		return printClient(clientsDir, server.LinkResponse{Name: prof.Name, Link: link, File: config.SafeFileName(prof.Name) + config.FileExt})
	}
	return nil
}

// adminCmd — админ-панель отдельным процессом (контейнер veil-admin): сессии и вход здесь,
// всё о сервере — через сокет управления veil-server (общий каталог /run/veil).
func adminCmd(args []string) int {
	fset := flag.NewFlagSet("admin", flag.ExitOnError)
	listen := fset.String("listen", ":8080", "listen address (only inside the Docker network)")
	socket := fset.String("control", server.DefaultControlSocket, "control socket of veil-server")
	_ = fset.Parse(args)
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	hs := &http.Server{Addr: *listen, Handler: server.NewAdminFront(*socket, log), ReadHeaderTimeout: 10 * time.Second}
	log.Info("admin panel started", "addr", *listen, "control", *socket)
	if err := hs.ListenAndServe(); err != nil {
		log.Error("admin panel", "err", err)
		return 1
	}
	return 0
}

// publicAddrs проверяет адреса; IPv6 приводится к каноничной записи без скобок.
func publicAddrs(v4, v6 string) (string, string, error) {
	v4, v6 = strings.TrimSpace(v4), strings.Trim(strings.TrimSpace(v6), "[]")
	if v4 != "" {
		a, err := netip.ParseAddr(v4)
		if err != nil || !a.Is4() {
			return "", "", fmt.Errorf("некорректный IPv4: %q", v4)
		}
	}
	if v6 != "" {
		a, err := netip.ParseAddr(v6)
		if err != nil || !a.Is6() || a.Is4In6() {
			return "", "", fmt.Errorf("некорректный IPv6: %q", v6)
		}
		v6 = a.String()
	}
	return v4, v6, nil
}

func clientCmd(args []string, socket string) int {
	usage := func() int {
		fmt.Fprintln(os.Stderr, `usage:
  veil-server client add NAME [-role user|admin] [-note TEXT]
  veil-server client reissue NAME
  veil-server client delete NAME
  veil-server client list`)
		return 2
	}
	if len(args) < 1 {
		return usage()
	}
	clientsDir := filepath.Join(filepath.Dir("/etc/veil/server.json"), "clients")
	if d := os.Getenv("VEIL_CLIENTS_DIR"); d != "" {
		clientsDir = d
	}
	call := func(method, path string, body any) ([]byte, error) {
		var b []byte
		if body != nil {
			b, _ = json.Marshal(body)
		}
		out, err := server.Control(socket, method, path, b)
		if err != nil {
			var e struct {
				Error string `json:"error"`
			}
			if json.Unmarshal(out, &e) == nil && e.Error != "" {
				return nil, errors.New(e.Error)
			}
		}
		return out, err
	}
	withLink := func(out []byte, err error) int {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		var lr server.LinkResponse
		if err := json.Unmarshal(out, &lr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := printClient(clientsDir, lr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	switch args[0] {
	case "add":
		fset := flag.NewFlagSet("client add", flag.ExitOnError)
		role := fset.String("role", "user", "user or admin")
		note := fset.String("note", "", "note")
		if len(args) < 2 {
			return usage()
		}
		_ = fset.Parse(args[2:])
		return withLink(call("POST", "clients", map[string]string{"name": args[1], "role": *role, "note": *note}))
	case "reissue":
		if len(args) < 2 {
			return usage()
		}
		return withLink(call("POST", "clients/reissue", map[string]string{"name": args[1]}))
	case "delete", "del", "rm":
		if len(args) < 2 {
			return usage()
		}
		if _, err := call("POST", "clients/delete", map[string]string{"name": args[1]}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		_ = os.Remove(filepath.Join(clientsDir, config.SafeFileName(args[1])+config.FileExt))
		fmt.Println("deleted", args[1])
		return 0
	case "list", "ls":
		out, err := call("GET", "state", nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		var st struct {
			Clients []struct {
				Name     string `json:"name"`
				Role     string `json:"role"`
				Disabled bool   `json:"disabled"`
				Online   bool   `json:"online"`
			} `json:"clients"`
		}
		if err := json.Unmarshal(out, &st); err != nil {
			os.Stdout.Write(out)
			return 0
		}
		for _, c := range st.Clients {
			state := "offline"
			if c.Online {
				state = "online"
			}
			if c.Disabled {
				state = "disabled"
			}
			fmt.Printf("%-32s %-6s %s\n", c.Name, c.Role, state)
		}
		return 0
	}
	return usage()
}

// printClient выводит ссылку veil:// с QR-кодом для терминала и сохраняет файл .veil.
func printClient(dir string, lr server.LinkResponse) error {
	fmt.Printf("\nclient %q:\n%s\n\n", lr.Name, lr.Link)
	if qr, err := qrcode.New(lr.Link, qrcode.Low); err == nil {
		fmt.Println(qr.ToSmallString(false))
	}
	if lr.File == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	p := filepath.Join(dir, lr.File)
	if err := os.WriteFile(p, []byte(lr.Link+"\n"), 0o600); err != nil {
		return err
	}
	fmt.Println("saved:", p)
	return nil
}
