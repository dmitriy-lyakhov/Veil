package server

import (
	"errors"
	"net"
	"net/netip"
	"strings"

	"veil/internal/config"
	"veil/internal/transport/front"
)

// FrontKey — общий ключ метки; отдаётся только владельцу через управляющий сокет.
func (s *Server) FrontKey() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg.Front == nil {
		return ""
	}
	return s.cfg.Front.Key
}

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
// уходила бы на чужой сайт. Решает адрес источника, а не назначения: адрес из подсети туннеля может
// быть только у аутентифицированного пира WireGuard (ответ на SYN всё равно уйдёт ему), а пакет
// снаружи с адресом назначения 10.66.0.1 дошёл бы до туннельного TLS и выдал наш сертификат.
func (s *Server) FrontBypass(c net.Conn) bool {
	ap, err := netip.ParseAddrPort(c.RemoteAddr().String())
	if err != nil {
		return false
	}
	a := ap.Addr().Unmap()
	if a.IsLoopback() {
		return true
	}
	for _, p := range s.tunnelNets {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// SetFront включает маскировку под sni (dest — куда проксировать посторонних, по умолчанию sni:443)
// или выключает при пустом sni. Ключ создаётся при первом включении и сохраняется.
// Конфиги клиентов, выданные до включения, нужно выпустить заново: в них нет ключа.
func (s *Server) SetFront(sni, dest string) error {
	sni, err := front.NormalizeSNI(sni)
	if err != nil {
		return err
	}
	dest = strings.TrimSpace(dest)
	if sni != "" && dest != "" {
		if _, _, err := net.SplitHostPort(dest); err != nil {
			return errors.New("адрес проксирования должен быть вида хост:порт")
		}
	}
	if err := s.setFrontConfig(sni, dest); err != nil {
		return err
	}
	s.mu.Lock()
	hook := s.onFront
	s.mu.Unlock()
	if hook != nil {
		hook()
	}
	return nil
}

func (s *Server) setFrontConfig(sni, dest string) error {
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
		k, err := front.NewKey()
		if err != nil {
			return err
		}
		key = k
	}
	s.cfg.Front = &config.Front{SNI: sni, Dest: dest, Key: key}
	return s.saveLocked()
}

// OnFrontChange регистрирует вызов после каждого включения или выключения маскировки.
func (s *Server) OnFrontChange(f func()) {
	s.mu.Lock()
	s.onFront = f
	s.mu.Unlock()
}

type FrontInfo struct {
	Enabled bool   `json:"enabled"`
	SNI     string `json:"sni,omitempty"`
	Dest    string `json:"dest,omitempty"`
}

func (s *Server) FrontInfo() FrontInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.frontInfoLocked()
}
