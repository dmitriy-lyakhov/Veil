package config

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strings"
)

const (
	LinkScheme      = "veil://"
	RelayLinkScheme = "veil-relay://"
	FileExt         = ".veil"
	maxLinkBytes    = 64 << 10
	keyLen          = 32
	byteOrderMark   = "\uFEFF"
)

func EncodeLink(p *Profile) (string, error) {
	shared := *p
	shared.SSH = nil
	shared.AdminUser, shared.AdminPassword = "", ""
	// адрес админки в туннеле остаётся только в профиле администратора: по нему телефон обращается к админ-API (это не секрет)
	if shared.Role != "admin" {
		shared.AdminURL = ""
	}
	return packLink(LinkScheme, shared)
}

func EncodeRelayLink(l *RelayLink) (string, error) {
	return packLink(RelayLinkScheme, l)
}

func DecodeRelayLink(text string) (*RelayLink, error) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, RelayLinkScheme) {
		return nil, errors.New("это не ключ каскада: ожидается ссылка veil-relay://…")
	}
	raw, err := unpackLink(text[len(RelayLinkScheme):])
	if err != nil {
		return nil, err
	}
	var l RelayLink
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, fmt.Errorf("ключ каскада повреждён: %w", err)
	}
	p := l.DialProfile()
	if _, err := p.Targets(); err != nil {
		return nil, err
	}
	if l.CertPin == "" && l.ServerName == "" {
		return nil, errors.New("в ключе каскада нет ни отпечатка сертификата, ни домена")
	}
	keys := map[string]string{"auth_key": l.AuthKey, "server_public_key": l.ServerPublicKey}
	if l.CertPin != "" {
		keys["cert_pin"] = l.CertPin
	}
	for name, v := range keys {
		if b, err := base64.StdEncoding.DecodeString(v); err != nil || len(b) != keyLen {
			return nil, fmt.Errorf("в ключе каскада некорректное поле %s", name)
		}
	}
	if !strings.HasPrefix(l.Path, "/") {
		return nil, errors.New("в ключе каскада нет пути")
	}
	return &l, nil
}

func packLink(scheme string, v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	zw, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		return "", err
	}
	if _, err := zw.Write(raw); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	return scheme + base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}

func unpackLink(body string) ([]byte, error) {
	packed, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(body, "="))
	if err != nil {
		return nil, errors.New("ссылка повреждена: неверная кодировка")
	}
	raw, err := io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(packed)), maxLinkBytes))
	if err != nil {
		return nil, errors.New("ссылка повреждена: не удалось распаковать")
	}
	return raw, nil
}

func DecodeLink(text string) (*Profile, error) {
	text = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), byteOrderMark))
	var raw []byte
	switch {
	case strings.HasPrefix(text, "{"):
		raw = []byte(text)
	case strings.HasPrefix(text, LinkScheme):
		var err error
		if raw, err = unpackLink(text[len(LinkScheme):]); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("это не конфигурация Veil: ожидается ссылка veil://… или файл .veil")
	}
	var p Profile
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("конфигурация повреждена: %w", err)
	}
	p.SSH = nil
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *Profile) Validate() error {
	if len(p.Entries) > 0 {
		for _, e := range p.Entries {
			ep := p.ForEntry(e)
			if err := ep.validateTransport(); err != nil {
				return fmt.Errorf("вход %s: %w", e.Name, err)
			}
		}
		return p.validateTunnel()
	}
	if err := p.validateTransport(); err != nil {
		return err
	}
	return p.validateTunnel()
}

func (p *Profile) validateTransport() error {
	if _, err := p.Targets(); err != nil {
		return err
	}
	if !strings.HasPrefix(p.Path, "/") {
		return errors.New("в конфигурации нет пути туннеля")
	}
	if p.CertPin == "" && p.ServerName == "" {
		return errors.New("в конфигурации нет ни отпечатка сертификата, ни домена")
	}
	keys := map[string]string{"auth_key": p.AuthKey}
	if p.CertPin != "" {
		keys["cert_pin"] = p.CertPin
	}
	for name, v := range keys {
		if b, err := base64.StdEncoding.DecodeString(v); err != nil || len(b) != keyLen {
			return fmt.Errorf("в конфигурации некорректное поле %s", name)
		}
	}
	return nil
}

func (p *Profile) validateTunnel() error {
	for name, v := range map[string]string{"server_public_key": p.ServerPublicKey, "private_key": p.PrivateKey} {
		if b, err := base64.StdEncoding.DecodeString(v); err != nil || len(b) != keyLen {
			return fmt.Errorf("в конфигурации некорректное поле %s", name)
		}
	}
	for _, a := range []string{p.Address4, p.Address6} {
		if _, err := netip.ParsePrefix(a); err != nil {
			return fmt.Errorf("в конфигурации некорректный адрес туннеля %q", a)
		}
	}
	if p.Name == "" {
		p.Name = firstNonEmpty(p.IPv6, p.IPv4)
	}
	return nil
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
