package veilcore

import (
	"context"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

var resolvers atomic.Value // []string "ip:53"

func init() {
	resolvers.Store([]string{"1.1.1.1:53", "8.8.8.8:53"})
	// На Android нет /etc/resolv.conf: встроенный резолвер Go по умолчанию ходит на 127.0.0.1:53 и не работает.
	// Подставляем свой Dial на заданные DNS-серверы (SetDNS).
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var last error
			d := net.Dialer{Timeout: 3 * time.Second}
			for _, s := range resolvers.Load().([]string) {
				c, err := d.DialContext(ctx, network, s)
				if err == nil {
					return c, nil
				}
				last = err
			}
			return nil, last
		},
	}
}

// SetDNS задаёт DNS-серверы (через запятую, IP) для разрешения имён самим ядром (адрес сервера, домены списков).
// Вызывать с серверами активной сети до подъёма VPN; в конец добавляются 1.1.1.1 и 8.8.8.8.
func SetDNS(csv string) {
	var out []string
	for _, s := range strings.Split(csv, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if ip := net.ParseIP(strings.Trim(s, "[]")); ip != nil {
			out = append(out, net.JoinHostPort(ip.String(), "53"))
		}
	}
	out = append(out, "1.1.1.1:53", "8.8.8.8:53")
	resolvers.Store(out)
}
