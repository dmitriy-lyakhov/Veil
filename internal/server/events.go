package server

// Журнал событий сервера (что, кто и откуда менял) и почасовая статистика трафика для админ-панели.
// Журнал — последние maxEvents записей в памяти и в файле events.jsonl рядом с конфигом.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxEvents       = 1000
	actorHeader     = "X-Veil-Actor"
	actorIPHeader   = "X-Veil-Actor-IP"
	actorApp        = "приложение (SSH)"
	trafficInterval = time.Minute
)

// Event — запись журнала. Kind: login, clients, links, cascade, settings, system.
type Event struct {
	Time  time.Time `json:"time"`
	Kind  string    `json:"kind"`
	Actor string    `json:"actor"`
	IP    string    `json:"ip,omitempty"`
	Text  string    `json:"text"`
}

type eventLog struct {
	mu   sync.Mutex
	list []Event // от старых к новым
	path string
}

func newEventLog(cfgPath string) *eventLog {
	l := &eventLog{}
	if cfgPath == "" {
		return l
	}
	l.path = filepath.Join(filepath.Dir(cfgPath), "events.jsonl")
	if f, err := os.Open(l.path); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var e Event
			if json.Unmarshal(sc.Bytes(), &e) == nil {
				l.list = append(l.list, e)
			}
		}
		f.Close()
		if len(l.list) > maxEvents {
			l.list = l.list[len(l.list)-maxEvents:]
			l.rewrite()
		}
	}
	return l
}

func (l *eventLog) rewrite() {
	var b bytes.Buffer
	for _, e := range l.list {
		j, _ := json.Marshal(e)
		b.Write(append(j, '\n'))
	}
	_ = os.WriteFile(l.path, b.Bytes(), 0o600)
}

func (l *eventLog) add(e Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list = append(l.list, e)
	if len(l.list) > maxEvents {
		l.list = l.list[len(l.list)-maxEvents:]
		if l.path != "" {
			l.rewrite()
		}
		return
	}
	if l.path == "" {
		return
	}
	if f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		j, _ := json.Marshal(e)
		_, _ = f.Write(append(j, '\n'))
		f.Close()
	}
}

// recent — последние n записей, новые первыми.
func (l *eventLog) recent(n int) []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n <= 0 || n > len(l.list) {
		n = len(l.list)
	}
	out := make([]Event, 0, n)
	for i := len(l.list) - 1; i >= len(l.list)-n; i-- {
		out = append(out, l.list[i])
	}
	return out
}

// Event записывает событие в журнал.
func (s *Server) Event(kind, actor, ip, text string) {
	if actor == "" {
		actor = "veil"
	}
	s.events.add(Event{Time: time.Now().UTC().Truncate(time.Second), Kind: kind, Actor: actor, IP: ip, Text: text})
}

// apiActor — кто выполняет запрос API: заголовки ставит сам сервер (веб-панель) или процесс панели;
// запрос через сокет управления без них — приложение по SSH.
func apiActor(r *http.Request) (actor, ip string) {
	actor, ip = r.Header.Get(actorHeader), r.Header.Get(actorIPHeader)
	if actor == "" {
		actor = actorApp
	}
	return actor, ip
}

// logAPI пишет в журнал успешное изменение через API: что сделано — по пути и телу запроса.
func (s *Server) logAPI(r *http.Request, path string, body []byte) {
	var m map[string]any
	_ = json.Unmarshal(body, &m)
	str := func(k string) string {
		if v, ok := m[k].(string); ok {
			return v
		}
		return ""
	}
	onoff := func(k string) string {
		if v, _ := m[k].(bool); v {
			return "вкл"
		}
		return "выкл"
	}
	name := str("name")
	kind, text := "", ""
	switch path {
	case "clients":
		kind, text = "clients", fmt.Sprintf("Клиент «%s» создан", name)
	case "clients/update":
		kind = "clients"
		var parts []string
		if nn := str("new_name"); nn != "" && nn != name {
			parts = append(parts, "переименован в «"+nn+"»")
		}
		if d, ok := m["disabled"].(bool); ok {
			parts = append(parts, map[bool]string{true: "заблокирован", false: "разблокирован"}[d])
		}
		if role := str("role"); role != "" {
			parts = append(parts, "роль: "+map[string]string{RoleAdmin: "администратор", RoleUser: "пользователь"}[role])
		}
		if _, ok := m["note"]; ok {
			parts = append(parts, "заметка изменена")
		}
		if len(parts) == 0 {
			return
		}
		text = fmt.Sprintf("Клиент «%s»: %s", name, strings.Join(parts, ", "))
	case "clients/subnets":
		var nets []string
		if l, ok := m["subnets"].([]any); ok {
			for _, x := range l {
				if v, ok := x.(string); ok && strings.TrimSpace(v) != "" {
					nets = append(nets, strings.TrimSpace(v))
				}
			}
		}
		list := "нет"
		if len(nets) > 0 {
			list = strings.Join(nets, ", ")
		}
		kind, text = "clients", fmt.Sprintf("Сети за клиентом «%s»: %s", name, list)
	case "clients/reissue":
		kind, text = "clients", fmt.Sprintf("Новый конфиг для «%s»", name)
	case "clients/delete":
		kind, text = "clients", fmt.Sprintf("Клиент «%s» удалён", name)
	case "sessions/kill":
		kind, text = "clients", fmt.Sprintf("Разорваны сессии «%s»", name)
	case "settings/links":
		kind, text = "links", fmt.Sprintf("Связи: клиент↔клиент %s, клиент↔сайт %s, сайт↔сайт %s", onoff("client_to_client"), onoff("client_to_site"), onoff("site_to_site"))
	case "relays":
		kind, text = "cascade", fmt.Sprintf("Выдан ключ каскада «%s»", name)
		if a, _ := m["admin"].(bool); a {
			text += " (с правом администрирования)"
		}
	case "relays/delete":
		kind, text = "cascade", fmt.Sprintf("Ключ каскада «%s» отозван", name)
	case "upstreams":
		kind, text = "cascade", fmt.Sprintf("Подключён следующий узел «%s»", name)
	case "upstreams/delete":
		kind, text = "cascade", fmt.Sprintf("Отключён следующий узел «%s»", name)
	case "cascade/optimize":
		kind, text = "cascade", "Перестройка маршрутов по задержкам"
	case "settings/front":
		kind, text = "settings", "Маскировка выключена"
		if sni := str("sni"); sni != "" {
			text = "Маскировка под " + sni
		}
	case "settings/siterepo":
		kind, text = "settings", "Сайт-прикрытие: настройки репозитория изменены"
	case "settings/siterepo/refresh":
		kind, text = "settings", "Сайт-прикрытие обновлён из репозитория"
	case "settings/domain":
		kind, text = "settings", "Домен: "+str("domain")
		if str("domain") == "" {
			text = "Домен убран"
		}
	case "settings/addresses":
		kind, text = "settings", fmt.Sprintf("Публичные адреса: %s %s", str("ipv4"), str("ipv6"))
	case "settings/password":
		kind, text = "login", "Пароль панели изменён"
	case "settings/admin":
		kind, text = "settings", "Режим админ-панели: "+str("mode")
	default:
		return
	}
	actor, ip := apiActor(r)
	s.Event(kind, actor, ip, text)
}

// ---------- трафик по часам ----------

type trafficStats struct {
	mu      sync.Mutex
	lastRx  uint64
	lastTx  uint64
	started bool
	hours   [24]trafficHour
}

type trafficHour struct {
	T  time.Time `json:"t"`
	Rx uint64    `json:"rx"`
	Tx uint64    `json:"tx"`
}

func (t *trafficStats) sample(now time.Time, rx, tx uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.started {
		t.lastRx, t.lastTx, t.started = rx, tx, true
		return
	}
	drx, dtx := rx-t.lastRx, tx-t.lastTx
	if rx < t.lastRx || tx < t.lastTx { // клиенты удалены или переподключились: счётчики сброшены
		drx, dtx = 0, 0
	}
	t.lastRx, t.lastTx = rx, tx
	h := now.UTC().Truncate(time.Hour)
	b := &t.hours[h.Hour()]
	if !b.T.Equal(h) {
		*b = trafficHour{T: h}
	}
	b.Rx += drx
	b.Tx += dtx
}

// last24 — 24 часовых интервала по порядку, последний — текущий час.
func (t *trafficStats) last24(now time.Time) []trafficHour {
	t.mu.Lock()
	defer t.mu.Unlock()
	cur := now.UTC().Truncate(time.Hour)
	out := make([]trafficHour, 24)
	for i := 0; i < 24; i++ {
		h := cur.Add(time.Duration(i-23) * time.Hour)
		b := t.hours[h.Hour()]
		if !b.T.Equal(h) {
			b = trafficHour{T: h}
		}
		out[i] = b
	}
	return out
}

func (s *Server) trafficLoop(stop <-chan struct{}) {
	t := time.NewTicker(trafficInterval)
	defer t.Stop()
	collect := func() {
		var rx, tx uint64
		for _, st := range s.wgStats() {
			rx += st.rx
			tx += st.tx
		}
		s.traffic.sample(time.Now(), rx, tx)
	}
	collect()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			collect()
		}
	}
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
