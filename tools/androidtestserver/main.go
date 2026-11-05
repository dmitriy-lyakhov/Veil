// androidtestserver — локальный тестовый сервер Veil для проверки Android-клиента (и вообще клиентов) без
// настоящего сервера. Сервер слушает 127.0.0.1:PORT, внутри туннеля (netstack) отвечает HTTP на порту 80
// по адресу туннеля; ссылку veil://… пишет в файл. Телефон: adb reverse tcp:PORT tcp:PORT.
//
//	go run ./tools/androidtestserver -port 18443 -link link.txt
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"

	"golang.zx2c4.com/wireguard/tun/netstack"

	"veil/internal/config"
	"veil/internal/provision"
	"veil/internal/server"
)

func main() {
	port := flag.Int("port", 18443, "TCP-порт на 127.0.0.1")
	linkFile := flag.String("link", "link.txt", "куда записать ссылку veil://")
	admin := flag.Bool("admin", false, "выдать клиенту роль администратора (вкладка «Доступ» на телефоне)")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := provision.NewServer("ru")
	check(err)
	prof, err := provision.AddPeer(cfg, "android")
	check(err)
	if *admin {
		// клиент-администратор: сервер узнаёт его по адресу в туннеле, админ-API доступно без пароля
		cfg.Peers[len(cfg.Peers)-1].Role = "admin"
		prof.Role = "admin"
		prof.AdminURL = provision.AdminURL(cfg)
	}
	s4 := netip.MustParsePrefix(cfg.Address4).Addr()
	s6 := netip.MustParsePrefix(cfg.Address6).Addr()
	tunDev, tnet, err := netstack.CreateNetTUN([]netip.Addr{s4, s6}, nil, cfg.MTU)
	check(err)
	srv, err := server.New(cfg, "", tunDev, log)
	check(err)
	defer srv.Close()

	for _, a := range []netip.Addr{s4, s6} {
		ln, err := tnet.ListenTCPAddrPort(netip.AddrPortFrom(a, 80))
		check(err)
		go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Info("inner request", "from", r.RemoteAddr, "path", r.URL.Path)
			if r.URL.Path == "/big" {
				w.Write(make([]byte, 8<<20))
				return
			}
			fmt.Fprintf(w, "hello from inside the Veil tunnel (%s)\n", r.RemoteAddr)
		}))
	}

	hs, err := srv.HTTPServer()
	check(err)
	ln, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", *port))
	check(err)
	go hs.ServeTLS(ln, "", "")
	for _, a := range []netip.Addr{s4, s6} {
		tl, err := tnet.ListenTCPAddrPort(netip.AddrPortFrom(a, 443))
		check(err)
		go hs.ServeTLS(tl, "", "")
	}

	cfg.Listen = fmt.Sprintf(":%d", *port)
	cfg.PublicIPv4 = "127.0.0.1"
	prof.Port = *port
	prof.IPv4 = "127.0.0.1"
	prof.IPv6 = ""
	prof.Name = "Local test"
	link, err := config.EncodeLink(prof)
	check(err)
	check(os.WriteFile(*linkFile, []byte(link), 0o600))
	log.Info("test server ready", "port", *port, "tunnel_addr4", s4.String(), "link_file", *linkFile)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
