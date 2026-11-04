package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/quic-go/quic-go"
	"golang.zx2c4.com/wireguard/tun"

	"veil/internal/config"
	"veil/internal/server"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "ctl" {
		os.Exit(ctl(os.Args[2:]))
	}
	cfgPath := flag.String("config", "/etc/veil/server.json", "path to server config")
	socket := flag.String("control", server.DefaultControlSocket, "control socket path")
	flag.Parse()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(*cfgPath, *socket, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func ctl(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: veil-server ctl METHOD PATH [base64-json]")
		return 2
	}
	var body []byte
	if len(args) > 2 {
		b, err := base64.StdEncoding.DecodeString(args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, "bad body:", err)
			return 2
		}
		body = b
	}
	out, err := server.Control(server.DefaultControlSocket, args[0], args[1], body)
	os.Stdout.Write(out)
	if err != nil {
		if len(out) == 0 {
			fmt.Fprintln(os.Stderr, err)
		}
		return 1
	}
	return 0
}

func run(cfgPath, socket string, log *slog.Logger) error {
	cfg, err := config.Load[config.Server](cfgPath)
	if err != nil {
		return err
	}
	ifname := cfg.Interface
	if ifname == "" {
		ifname = config.DefaultInterface
	}
	mtu := cfg.MTU
	if mtu == 0 {
		mtu = config.DefaultMTU
	}
	tunDev, err := tun.CreateTUN(ifname, mtu)
	if err != nil {
		return err
	}
	if name, err := tunDev.Name(); err == nil {
		ifname = name
	}
	srv, err := server.New(cfg, cfgPath, tunDev, log)
	if err != nil {
		return err
	}
	defer srv.Close()
	ns, err := server.SetupNetwork(cfg, ifname, log)
	if err != nil {
		return err
	}
	defer ns.Teardown()
	stopCtl, err := srv.ServeControl(socket)
	if err != nil {
		return err
	}
	defer stopCtl()

	hs, err := srv.HTTPServer()
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return err
	}
	h3, err := srv.HTTP3Server()
	if err != nil {
		return err
	}
	pc, err := net.ListenPacket("udp", cfg.Listen)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 2)
	go func() { errc <- hs.ServeTLS(ln, "", "") }()
	go func() { errc <- h3.Serve(pc) }()
	defer h3.Close()
	if port80 := redirectAddr(cfg.Listen); port80 != "" {
		if rl, err := net.Listen("tcp", port80); err != nil {
			log.Warn("http redirect disabled", "addr", port80, "err", err)
		} else {
			rs := server.HTTPRedirectServer()
			defer rs.Close()
			go func() { _ = rs.Serve(rl) }()
			log.Info("http redirect started", "addr", rl.Addr().String())
		}
	}
	log.Info("veil server started", "tcp", ln.Addr().String(), "udp", pc.LocalAddr().String(), "peers", len(cfg.Peers), "iface", ifname)
	select {
	case <-ctx.Done():
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, quic.ErrServerClosed) {
			return err
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = hs.Shutdown(shutdown)
	return nil
}

// redirectAddr — адрес для редиректа HTTP→HTTPS: тот же хост, порт 80; только если сервис слушает 443.
func redirectAddr(listen string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil || port != "443" {
		return ""
	}
	return net.JoinHostPort(host, "80")
}
