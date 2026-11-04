// Package ovpnconf разбирает и проверяет клиентские файлы OpenVPN (.ovpn).
//
// Клиент запускает openvpn.exe с правами администратора, поэтому файл от постороннего человека
// не должен уметь выполнять команды или читать и писать произвольные файлы: такие директивы
// отклоняются при разборе. Сертификаты и ключи должны быть вложены в сам файл.
package ovpnconf

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Config — клиентская конфигурация OpenVPN в виде текста .ovpn с вложенными сертификатами.
type Config struct {
	Text string `json:"text"`
}

// Директивы, которые запускают команды, обращаются к файлам и службам системы
// или требуют ввода (логин и пароль).
var forbidden = map[string]bool{
	"script-security": true, "up": true, "down": true, "route-up": true, "route-pre-down": true,
	"tls-verify": true, "ipchange": true, "client-connect": true, "client-disconnect": true,
	"learn-address": true, "auth-user-pass-verify": true, "plugin": true, "config": true,
	"log": true, "log-append": true, "writepid": true, "cd": true, "chroot": true, "setenv": true,
	"setenv-safe": true, "askpass": true, "daemon": true, "service": true, "status": true,
	"engine": true, "providers": true, "tmp-dir": true, "down-pre": true,
	"management": true, "management-client": true, "management-query-passwords": true,
	"management-hold": true, "management-signal": true, "management-forget-disconnect": true,
	"management-log-cache": true, "management-up-down": true, "management-external-key": true,
	"management-external-cert": true, "management-client-user": true, "management-client-group": true,
	"echo": true, "errors-to-stderr": true, "syslog": true, "msg-channel": true,
	"auth-user-pass": true, "http-proxy-user-pass": true, "socks-proxy-auth": true,
}

// Директивы с ключами и сертификатами: допустима только вложенная форма <тег>…</тег>.
var inlineOnly = map[string]bool{
	"ca": true, "cert": true, "key": true, "dh": true, "pkcs12": true, "tls-auth": true,
	"tls-crypt": true, "tls-crypt-v2": true, "extra-certs": true, "secret": true, "crl-verify": true,
}

// Parse проверяет текст .ovpn и возвращает нормализованную конфигурацию.
func Parse(text string) (*Config, error) {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	text = strings.TrimPrefix(text, string(rune(0xFEFF)))
	var out []string
	var open string // имя открытого вложенного блока
	hasRemote, hasClient := false, false
	blocks := map[string]bool{}
	for i, raw := range strings.Split(text, "\n") {
		ln := strings.TrimSpace(raw)
		if open != "" {
			out = append(out, ln)
			if ln == "</"+open+">" {
				open = ""
			}
			continue
		}
		if ln == "" || ln[0] == '#' || ln[0] == ';' {
			continue
		}
		if ln[0] == '<' && strings.HasSuffix(ln, ">") && !strings.HasPrefix(ln, "</") {
			name := strings.ToLower(ln[1 : len(ln)-1])
			if forbidden[name] || !(inlineOnly[name] || name == "connection") {
				return nil, fmt.Errorf("строка %d: блок <%s> не поддерживается", i+1, name)
			}
			open = name
			blocks[name] = true
			out = append(out, "<"+name+">")
			continue
		}
		f := strings.Fields(ln)
		key := strings.TrimPrefix(strings.ToLower(f[0]), "--")
		if forbidden[key] {
			return nil, fmt.Errorf("строка %d: директива %q не поддерживается (она может запускать команды или менять файлы); уберите её из файла", i+1, key)
		}
		if inlineOnly[key] && len(f) > 1 {
			return nil, fmt.Errorf("строка %d: %q указывает на отдельный файл; вложите его в .ovpn в виде <%s>…</%s>", i+1, key, key, key)
		}
		switch key {
		case "remote":
			hasRemote = true
		case "client", "pull":
			hasClient = true
		}
		out = append(out, strings.Join(append([]string{key}, f[1:]...), " "))
	}
	if open != "" {
		return nil, fmt.Errorf("блок <%s> не закрыт", open)
	}
	if !hasRemote {
		return nil, errors.New("в файле нет строки remote: не указан адрес сервера")
	}
	if !hasClient {
		out = append([]string{"client"}, out...)
	}
	if !blocks["ca"] {
		return nil, errors.New("в файле нет сертификата <ca>: вложите его в .ovpn")
	}
	c := &Config{Text: strings.Join(out, "\n") + "\n"}
	if _, _, _, err := c.Remote(); err != nil {
		return nil, err
	}
	return c, nil
}

// Looks — похоже ли содержимое на файл OpenVPN.
func Looks(text string) bool {
	for _, ln := range strings.Split(text, "\n") {
		f := strings.Fields(strings.TrimSpace(ln))
		if len(f) == 0 {
			continue
		}
		if f[0] == "client" || f[0] == "remote" || f[0] == "<ca>" {
			return true
		}
	}
	return false
}

// Remote возвращает адрес, порт и транспорт ("udp" или "tcp") первого сервера.
func (c *Config) Remote() (host string, port int, proto string, err error) {
	proto, port = "udp", 1194
	var remote []string
	inBlock := false
	for _, ln := range strings.Split(c.Text, "\n") {
		if strings.HasPrefix(ln, "<") {
			inBlock = !strings.HasPrefix(ln, "</")
			continue
		}
		f := strings.Fields(ln)
		if inBlock || len(f) == 0 {
			continue
		}
		switch f[0] {
		case "proto":
			if len(f) > 1 {
				proto = f[1]
			}
		case "port":
			if len(f) > 1 {
				if n, e := strconv.Atoi(f[1]); e == nil {
					port = n
				}
			}
		case "remote":
			if remote == nil {
				remote = f
			}
		}
	}
	if len(remote) < 2 {
		return "", 0, "", errors.New("в файле нет строки remote")
	}
	host = remote[1]
	if len(remote) > 2 {
		n, e := strconv.Atoi(remote[2])
		if e != nil || n < 1 || n > 65535 {
			return "", 0, "", fmt.Errorf("неверный порт в строке remote: %q", remote[2])
		}
		port = n
	}
	if len(remote) > 3 {
		proto = remote[3]
	}
	proto = strings.ToLower(proto)
	switch {
	case strings.HasPrefix(proto, "tcp"):
		proto = "tcp"
	case strings.HasPrefix(proto, "udp"):
		proto = "udp"
	default:
		return "", 0, "", fmt.Errorf("неизвестный транспорт %q", proto)
	}
	return host, port, proto, nil
}

func (c *Config) String() string { return c.Text }

// WithDNS возвращает текст с добавленными DNS-серверами (из настроек приложения).
func (c *Config) WithDNS(dns []string) string {
	if len(dns) == 0 {
		return c.Text
	}
	var b strings.Builder
	b.WriteString(c.Text)
	for _, d := range dns {
		fmt.Fprintf(&b, "dhcp-option DNS %s\n", d)
	}
	return b.String()
}
