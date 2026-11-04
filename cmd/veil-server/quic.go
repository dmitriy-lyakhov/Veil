package main

import (
	"errors"
	"log/slog"
	"net"
	"sync"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"

	"veil/internal/server"
)

// quicService держит слушатель HTTP/3 на UDP только пока маскировка выключена: клиент с маскировкой
// ходит по TCP, а QUIC нельзя прозрачно проксировать на чужой сайт — он ответил бы нашим сертификатом.
type quicService struct {
	srv  *server.Server
	addr string
	log  *slog.Logger

	mu sync.Mutex
	h3 *http3.Server
	pc net.PacketConn
}

func (q *quicService) Sync() {
	wantRunning := q.srv.FrontConfig() == nil
	q.mu.Lock()
	defer q.mu.Unlock()
	switch {
	case wantRunning && q.h3 == nil:
		q.start()
	case !wantRunning && q.h3 != nil:
		q.stopLocked()
		q.log.Info("quic listener stopped: masking is on")
	}
}

func (q *quicService) start() {
	h3, err := q.srv.HTTP3Server()
	if err != nil {
		q.log.Error("quic", "err", err)
		return
	}
	pc, err := net.ListenPacket("udp", q.addr)
	if err != nil {
		q.log.Error("quic listen", "addr", q.addr, "err", err)
		return
	}
	q.h3, q.pc = h3, pc
	go func() {
		if err := h3.Serve(pc); err != nil && !errors.Is(err, quic.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
			q.log.Warn("quic serve", "err", err)
		}
	}()
	q.log.Info("quic listener started", "udp", pc.LocalAddr().String())
}

func (q *quicService) stopLocked() {
	_ = q.h3.Close()
	_ = q.pc.Close()
	q.h3, q.pc = nil, nil
}

func (q *quicService) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.h3 != nil {
		q.stopLocked()
	}
}
