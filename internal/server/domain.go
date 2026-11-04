package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/idna"
)

type certStatus struct {
	State   string    `json:"state"`
	Message string    `json:"message,omitempty"`
	Expires time.Time `json:"expires,omitzero"`
}

type certManager struct {
	m      *autocert.Manager
	log    *slog.Logger
	mu     sync.Mutex
	status certStatus
}

func newCertManager(cacheDir string, domain func() string, log *slog.Logger) *certManager {
	m := &autocert.Manager{
		Prompt: autocert.AcceptTOS,
		HostPolicy: func(_ context.Context, host string) error {
			if d := domain(); d != "" && strings.EqualFold(host, d) {
				return nil
			}
			return errors.New("host not allowed")
		},
	}
	if cacheDir != "" {
		m.Cache = autocert.DirCache(cacheDir)
	}
	return &certManager{m: m, log: log, status: certStatus{State: "none"}}
}

func (c *certManager) get(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cert, err := c.m.GetCertificate(hello)
	if err == nil && cert.Leaf != nil {
		c.set(certStatus{State: "ready", Expires: cert.Leaf.NotAfter})
	}
	return cert, err
}

func (c *certManager) set(st certStatus) {
	c.mu.Lock()
	c.status = st
	c.mu.Unlock()
}

func (c *certManager) Status() certStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

func (c *certManager) Ready() bool { return c.Status().State == "ready" }

func (c *certManager) warm(domain string) {
	c.set(certStatus{State: "pending", Message: "запрашиваем сертификат Let's Encrypt"})
	cert, err := c.m.GetCertificate(&tls.ClientHelloInfo{ServerName: domain, SupportedProtos: []string{"h2"}})
	if err != nil {
		c.log.Warn("acme", "domain", domain, "err", err)
		c.set(certStatus{State: "error", Message: "не удалось получить сертификат: " + err.Error() + ". Проверьте, что A/AAAA-записи домена указывают на этот сервер и порт 443 открыт."})
		return
	}
	leaf := cert.Leaf
	if leaf == nil && len(cert.Certificate) > 0 {
		leaf, _ = x509.ParseCertificate(cert.Certificate[0])
	}
	st := certStatus{State: "ready"}
	if leaf != nil {
		st.Expires = leaf.NotAfter
	}
	c.set(st)
	c.log.Info("acme certificate ready", "domain", domain)
}

func normalizeDomain(d string) (string, error) {
	d = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(d)), ".")
	d = strings.TrimPrefix(strings.TrimPrefix(d, "https://"), "http://")
	d = strings.TrimSuffix(d, "/")
	if d == "" {
		return "", nil
	}
	ascii, err := idna.Lookup.ToASCII(d)
	if err != nil || !strings.Contains(ascii, ".") || strings.ContainsAny(ascii, ":/ ") {
		return "", fmt.Errorf("«%s» не похоже на доменное имя", d)
	}
	return ascii, nil
}

func (s *Server) SetDomain(domain string) error {
	d, err := normalizeDomain(domain)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.cfg.Domain = d
	err = s.saveLocked()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if d == "" {
		s.certs.set(certStatus{State: "none"})
		return nil
	}
	go s.certs.warm(d)
	return nil
}

func (s *Server) SetPublicAddresses(v4, v6 string) error {
	v4, v6 = strings.TrimSpace(v4), strings.Trim(strings.TrimSpace(v6), "[]")
	if v4 == "" && v6 == "" {
		return errors.New("укажите хотя бы один публичный адрес")
	}
	if v4 != "" {
		if a, err := netip.ParseAddr(v4); err != nil || !a.Is4() {
			return fmt.Errorf("«%s» — не IPv4-адрес", v4)
		}
	}
	if v6 != "" {
		if a, err := netip.ParseAddr(v6); err != nil || !a.Is6() || a.Is4In6() {
			return fmt.Errorf("«%s» — не IPv6-адрес", v6)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.PublicIPv4, s.cfg.PublicIPv6 = v4, v6
	return s.saveLocked()
}
