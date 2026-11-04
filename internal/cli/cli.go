package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"veil/internal/client"
	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/helper"
	"veil/internal/routing"
	"veil/internal/settings"
)

const usage = `Использование:
  veil deploy [флаги]          установить сервер по SSH и сохранить профиль
  veil import <файл|ссылка> [название]  добавить сервер из файла .veil или ссылки veil://
  veil connect <профиль|файл>  поднять туннель в терминале (нужны права root)
  veil list                    показать сохранённые профили
`

func Run(args []string) {
	if len(args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	os.Args = args
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	var err error
	switch os.Args[1] {
	case "deploy":
		err = cmdDeploy(os.Args[2:])
	case "import":
		err = cmdImport(os.Args[2:])
	case "connect":
		err = cmdConnect(os.Args[2:], log)
	case "list":
		err = cmdList()
	case "tun":
		err = cmdTun(os.Args[2:], log)
	case "unblock":
		err = client.Unblock()
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ошибка:", err)
		os.Exit(1)
	}
}

func cmdDeploy(args []string) error {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	var req deploy.Request
	fs.StringVar(&req.Name, "name", "", "название профиля")
	fs.StringVar(&req.IPv4, "ipv4", "", "IPv4-адрес сервера")
	fs.StringVar(&req.IPv6, "ipv6", "", "IPv6-адрес сервера")
	fs.IntVar(&req.SSHPort, "ssh-port", 22, "SSH-порт")
	fs.StringVar(&req.User, "user", "root", "пользователь SSH")
	fs.StringVar(&req.KeyPath, "key", "", "путь к приватному SSH-ключу")
	fs.StringVar(&req.ServerBinDir, "server-dir", "", "каталог с veil-server-linux-*")
	fs.StringVar(&req.SiteLang, "site-lang", "ru", "язык сайта-прикрытия: ru или en")
	usePassword := fs.Bool("password", false, "спросить пароль SSH (или задать VEIL_SSH_PASSWORD)")
	_ = fs.Parse(args)
	if *usePassword || req.KeyPath == "" {
		req.Password = os.Getenv("VEIL_SSH_PASSWORD")
		if req.Password == "" {
			p, err := readSecret("Пароль SSH: ")
			if err != nil {
				return err
			}
			req.Password = p
		}
	}
	if req.KeyPath != "" {
		req.KeyPassphrase = os.Getenv("VEIL_SSH_KEY_PASSPHRASE")
	}
	prof, err := deploy.Deploy(context.Background(), req, func(f string, a ...any) {
		fmt.Printf(f+"\n", a...)
	})
	if err != nil {
		return err
	}
	path, err := config.ProfilePath(prof.Name)
	if err != nil {
		return err
	}
	if err := config.Save(path, prof); err != nil {
		return err
	}
	fmt.Println("Профиль сохранён:", path)
	return nil
}

func resolveProfile(arg string) (string, error) {
	if _, err := os.Stat(arg); err == nil {
		return arg, nil
	}
	path, err := config.ProfilePath(arg)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("профиль %q не найден", arg)
	}
	return path, nil
}

func cmdConnect(args []string, log *slog.Logger) error {
	if len(args) != 1 {
		return fmt.Errorf("укажите профиль")
	}
	path, err := resolveProfile(args[0])
	if err != nil {
		return err
	}
	prof, err := config.Load[config.Profile](path)
	if err != nil {
		return err
	}
	rc := loadRouting()
	opt := client.Options{}
	if sp, err := settings.Path(); err == nil {
		if st, err := settings.Load(sp); err == nil {
			opt.KillSwitch = st.KillSwitch
			if d := st.DNS; len(d) > 0 {
				prof.DNS = d
			}
		}
	}
	tn, err := client.StartSystem(prof, rc, opt, log)
	if err != nil {
		return err
	}
	defer tn.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nОтключение…")
			return nil
		case <-t.C:
			s := tn.Status()
			fmt.Printf("%s via %s %s %s  rx=%d tx=%d\n", s.State, s.Transport, s.Family, s.Remote, s.RxBytes, s.TxBytes)
		}
	}
}

func cmdImport(args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("укажите файл .veil или ссылку veil://")
	}
	src := args[0]
	if !strings.HasPrefix(src, config.LinkScheme) {
		b, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		src = string(b)
	}
	prof, err := config.DecodeLink(src)
	if err != nil {
		return err
	}
	if len(args) == 2 {
		prof.Name = args[1]
	}
	path, err := config.ProfilePath(prof.Name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("профиль %q уже существует — укажите другое название вторым аргументом", prof.Name)
	}
	if err := config.Save(path, prof); err != nil {
		return err
	}
	fmt.Println("Профиль сохранён:", path)
	return nil
}

func cmdList() error {
	ps, err := config.ListProfiles()
	if err != nil {
		return err
	}
	for _, p := range ps {
		fmt.Printf("%-24s v6=%-28s v4=%s\n", p.Name, p.IPv6, p.IPv4)
	}
	return nil
}

func cmdTun(args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("tun", flag.ExitOnError)
	profile := fs.String("profile", "", "")
	control := fs.String("control", "", "")
	tokenFile := fs.String("token-file", "", "")
	routes := fs.String("routes", "", "")
	settingsPath := fs.String("settings", "", "")
	_ = fs.Parse(args)
	return helper.Serve(*profile, *control, *tokenFile, *routes, *settingsPath, log)
}

func readSecret(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		return string(b), err
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}

// loadRouting читает сохранённые списки «через VPN»/«мимо VPN» (если есть).
func loadRouting() *routing.Config {
	path, err := routing.Path()
	if err != nil {
		return nil
	}
	rc, err := routing.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "списки адресов:", err)
		return nil
	}
	return rc
}
