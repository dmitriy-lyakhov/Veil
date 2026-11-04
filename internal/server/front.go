package server

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"net/netip"
	"strings"

	"veil/internal/config"
	"veil/internal/front"
)

// FrontConfig — текущие настройки маскировки для слушателя (nil — выключена).
func (s *Server) FrontConfig() *front.Config {
	s.mu.Lock()
	f := s.cfg.Front
	s.mu.Unlock()
	if f == nil || f.SNI == "" {
		return nil
	}
	key, err := front.DecodeKey(f.Key)
	if err != nil {
		return nil
	}
	return &front.Config{SNI: f.SNI, Dest: f.Dest, Key: key}
}

// FrontBypass — соединения изнутри туннеля и локальные не маскируются: иначе панель по адресу VPN
// уходила бы на чужой сайт.
func (s *Server) FrontBypass(c net.Conn) bool {
	ap, err := netip.ParseAddrPort(c.LocalAddr().String())
	if err != nil {
		return false
	}
	a := ap.Addr().Unmap()
	if a.IsLoopback() {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range []string{s.cfg.Address4, s.cfg.Address6} {
		if pf, err := netip.ParsePrefix(p); err == nil && pf.Contains(a) {
			return true
		}
	}
	return false
}

// SetFront включает маскировку под sni (dest — куда проксировать посторонних, по умолчанию sni:443)
// или выключает при пустом sni. Ключ создаётся при первом включении и сохраняется.
// Конфиги клиентов, выданные до включения, нужно выпустить заново: в них нет ключа.
func (s *Server) SetFront(sni, dest string) error {
	sni = strings.ToLower(strings.TrimSpace(sni))
	dest = strings.TrimSpace(dest)
	if sni != "" {
		if strings.ContainsAny(sni, " /:") || !strings.Contains(sni, ".") {
			return errors.New("укажите доменное имя сайта, например ya.ru")
		}
		if dest != "" {
			if _, _, err := net.SplitHostPort(dest); err != nil {
				return errors.New("адрес проксирования должен быть вида хост:порт")
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if sni == "" {
		s.cfg.Front = nil
		return s.saveLocked()
	}
	key := ""
	if s.cfg.Front != nil {
		key = s.cfg.Front.Key
	}
	if key == "" {
		b := make([]byte, front.KeyLen)
		if _, err := rand.Read(b); err != nil {
			return err
		}
		key = base64.StdEncoding.EncodeToString(b)
	}
	s.cfg.Front = &config.Front{SNI: sni, Dest: dest, Key: key}
	return s.saveLocked()
}
