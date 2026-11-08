package server

// Минимальный клиент Docker Engine API для страницы «Протоколы» отдельной админ-панели.
// Работает, только если в контейнер veil-admin смонтирован /var/run/docker.sock (это выбор
// владельца сервера: доступ к Docker равен правам root на хосте). Трогает только контейнеры
// с меткой net.veil.protocol.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const dockerSocket = "/var/run/docker.sock"

type dockerClient struct{ hc *http.Client }

// newDockerClient — клиент, если сокет Docker доступен; иначе nil.
func newDockerClient() *dockerClient {
	if fi, err := os.Stat(dockerSocket); err != nil || fi.Mode()&os.ModeSocket == 0 {
		return nil
	}
	return &dockerClient{hc: &http.Client{Timeout: 2 * time.Minute, Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "unix", dockerSocket)
	}}}}
}

func (d *dockerClient) do(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://docker"+path, rd)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := d.hc.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("docker: %w", err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return out, resp.StatusCode, err
}

type dockerContainer struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Image    string `json:"image"`
	State    string `json:"state"`
	Status   string `json:"status"`
	Ports    string `json:"ports"`
}

func (d *dockerClient) containers(ctx context.Context) ([]dockerContainer, error) {
	filters := url.QueryEscape(`{"label":["net.veil.protocol"]}`)
	raw, code, err := d.do(ctx, http.MethodGet, "/containers/json?all=1&filters="+filters, nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("docker: %d %s", code, strings.TrimSpace(string(raw)))
	}
	var list []struct {
		Names  []string
		Image  string
		State  string
		Status string
		Labels map[string]string
		Ports  []struct {
			PublicPort int
			Type       string
			IP         string
		}
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	out := []dockerContainer{}
	for _, c := range list {
		seen := map[string]bool{}
		var ports []string
		for _, p := range c.Ports {
			if p.PublicPort == 0 {
				continue
			}
			s := fmt.Sprintf("%d/%s", p.PublicPort, p.Type)
			if !seen[s] {
				seen[s] = true
				ports = append(ports, s)
			}
		}
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		out = append(out, dockerContainer{Name: name, Protocol: c.Labels["net.veil.protocol"], Image: c.Image, State: c.State, Status: c.Status, Ports: strings.Join(ports, " ")})
	}
	return out, nil
}

var veilContainerRe = regexp.MustCompile(`^veil(-[a-z0-9]+)?$`)

// own проверяет, что контейнер — наш (метка net.veil.protocol), и возвращает его протокол.
func (d *dockerClient) own(ctx context.Context, name string) (string, error) {
	if !veilContainerRe.MatchString(name) {
		return "", errors.New("недопустимое имя контейнера")
	}
	list, err := d.containers(ctx)
	if err != nil {
		return "", err
	}
	for _, c := range list {
		if c.Name == name {
			return c.Protocol, nil
		}
	}
	return "", fmt.Errorf("контейнер %s не найден", name)
}

func (d *dockerClient) restart(ctx context.Context, name string) error {
	if _, err := d.own(ctx, name); err != nil {
		return err
	}
	raw, code, err := d.do(ctx, http.MethodPost, "/containers/"+name+"/restart?t=5", nil)
	if err != nil {
		return err
	}
	if code != http.StatusNoContent && code != http.StatusOK {
		return fmt.Errorf("docker: %d %s", code, strings.TrimSpace(string(raw)))
	}
	return nil
}

var clientNameRe = regexp.MustCompile(`^[\p{L}\p{N} ._@-]{1,48}$`)

// addClient выполняет «client add NAME» в контейнере протокола и возвращает конфиг клиента.
func (d *dockerClient) addClient(ctx context.Context, name, client string) (text, file string, err error) {
	proto, err := d.own(ctx, name)
	if err != nil {
		return "", "", err
	}
	if proto == "veil" || proto == "admin" {
		return "", "", errors.New("клиентов Veil выдают на странице «Клиенты»")
	}
	client = strings.TrimSpace(client)
	if !clientNameRe.MatchString(client) {
		return "", "", errors.New("имя клиента: буквы, цифры, пробел, «.», «_», «@», «-», до 48 символов")
	}
	// скрипты контейнеров принимают только латиницу: «Ноутбук Анны» → «Noutbuk-Anny»
	client = latinName(client)
	if client == "" {
		return "", "", errors.New("имя клиента должно содержать буквы или цифры")
	}
	// имя в файлах контейнера — латиница; кириллицу и пробелы скрипт заменяет сам
	// в ответ — только файл конфигурации (без журнала и QR для терминала); у IPsec — текст с логином и паролем
	files := map[string]string{"wireguard": "/etc/wireguard/clients/%s.conf", "amneziawg": "/etc/amnezia/amneziawg/clients/%s.conf", "openvpn": "/etc/openvpn/veil/clients/%s.ovpn"}
	script := "client add " + client + " 2>&1"
	if f, ok := files[proto]; ok {
		script = "client add " + client + " >/tmp/veil-add.log 2>&1 || { cat /tmp/veil-add.log; exit 1; }; cat " + fmt.Sprintf(f, client)
	}
	raw, code, err := d.do(ctx, http.MethodPost, "/containers/"+name+"/exec", map[string]any{
		"Cmd": []string{"sh", "-c", script}, "AttachStdout": true, "AttachStderr": true, "Tty": true,
	})
	if err != nil {
		return "", "", err
	}
	if code != http.StatusCreated {
		return "", "", fmt.Errorf("docker exec: %d %s", code, strings.TrimSpace(string(raw)))
	}
	var created struct{ Id string }
	if err := json.Unmarshal(raw, &created); err != nil || created.Id == "" {
		return "", "", errors.New("docker exec: нет id")
	}
	out, code, err := d.do(ctx, http.MethodPost, "/exec/"+created.Id+"/start", map[string]any{"Detach": false, "Tty": true})
	if err != nil {
		return "", "", err
	}
	if code != http.StatusOK {
		return "", "", fmt.Errorf("docker exec: %d", code)
	}
	insp, _, err := d.do(ctx, http.MethodGet, "/exec/"+created.Id+"/json", nil)
	var st struct{ ExitCode int }
	if err == nil && json.Unmarshal(insp, &st) == nil && st.ExitCode != 0 {
		return "", "", errors.New(strings.TrimSpace(stripANSI(string(out))))
	}
	ext := map[string]string{"wireguard": ".conf", "amneziawg": ".conf", "openvpn": ".ovpn", "ipsec": ".txt"}[proto]
	return stripANSI(strings.ReplaceAll(string(out), "\r\n", "\n")), client + ext, nil
}

var translit = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "zh", 'з': "z", 'и': "i", 'й': "y",
	'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f",
	'х': "kh", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "shch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
}

// latinName — имя для файлов и конфигов контейнера: латиница, цифры, «.», «_», «-».
func latinName(s string) string {
	var b strings.Builder
	for _, r := range s {
		lower := []rune(strings.ToLower(string(r)))[0]
		switch {
		case r < 128 && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-'):
			b.WriteRune(r)
		case translit[lower] != "" || (translit[lower] == "" && lower >= 'а' && lower <= 'я'):
			t := translit[lower]
			if r != lower && t != "" {
				t = strings.ToUpper(t[:1]) + t[1:]
			}
			b.WriteString(t)
		default:
			b.WriteByte('-')
		}
	}
	out := regexp.MustCompile(`-+`).ReplaceAllString(b.String(), "-")
	out = strings.Trim(out, "-._")
	if len(out) > 48 {
		out = out[:48]
	}
	return out
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }
