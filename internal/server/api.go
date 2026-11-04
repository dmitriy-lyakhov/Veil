package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"veil/internal/config"
)

const (
	maxAPIBody = 64 << 10
	apiTimeout = 90 * time.Second
	qrSize     = 360
)

type nameRequest struct {
	Name string `json:"name"`
}

type clientRequest struct {
	Name      string   `json:"name"`
	Note      string   `json:"note"`
	Route     []string `json:"route"`
	Auto      bool     `json:"auto"`
	Exit      string   `json:"exit"`
	MaxHops   *int     `json:"max_hops"`
	PublicKey string   `json:"public_key"`
}

type updateRequest struct {
	Name string `json:"name"`
	ClientUpdate
}

type upstreamRequest struct {
	Name string `json:"name"`
	Link string `json:"link"`
}

type routeRequest struct {
	Route []string `json:"route"`
}

type domainRequest struct {
	Domain string `json:"domain"`
}

type passwordRequest struct {
	Current string `json:"current"`
	New     string `json:"new"`
}

type LinkResponse struct {
	Name string `json:"name"`
	Link string `json:"link"`
	File string `json:"file,omitempty"`
	QR   string `json:"qr,omitempty"`
	Via  string `json:"via,omitempty"`
}

func QRDataURI(text string) string {
	png, err := qrcode.Encode(text, qrcode.Medium, qrSize)
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

func profileLink(p *config.Profile) (LinkResponse, error) {
	link, err := config.EncodeLink(p)
	if err != nil {
		return LinkResponse{}, err
	}
	return LinkResponse{Name: p.Name, Link: link, File: config.SafeFileName(p.Name) + config.FileExt, QR: QRDataURI(link), Via: p.Via}, nil
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request, path string, trusted bool) {
	ctx, cancel := context.WithTimeout(r.Context(), apiTimeout)
	defer cancel()
	if r.Method == http.MethodGet && path == "state" {
		writeJSON(w, http.StatusOK, s.State())
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	decode := func(v any) bool {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAPIBody)).Decode(v); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "некорректный запрос"})
			return false
		}
		return true
	}
	reply := func(v any, err error) {
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if v == nil {
			v = map[string]string{"ok": "1"}
		}
		writeJSON(w, http.StatusOK, v)
	}
	withLink := func(p *config.Profile, err error) {
		if err != nil {
			reply(nil, err)
			return
		}
		resp, err := profileLink(p)
		reply(resp, err)
	}
	switch path {
	case "clients":
		var req clientRequest
		if decode(&req) {
			var prof *config.Profile
			var err error
			if req.Auto {
				hops := AllHops
				if req.MaxHops != nil {
					hops = *req.MaxHops
				}
				prof, err = s.AddAutoClientWithKey(ctx, req.Name, req.Note, req.Exit, hops, req.PublicKey)
			} else {
				prof, err = s.AddClientWithKey(ctx, req.Name, req.Note, req.Route, req.PublicKey)
			}
			if req.PublicKey != "" {
				reply(map[string]any{"profile": prof}, err)
			} else {
				withLink(prof, err)
			}
		}
	case "clients/update":
		var req updateRequest
		if decode(&req) {
			reply(nil, s.UpdateClient(req.Name, req.ClientUpdate))
		}
	case "clients/reissue":
		var req nameRequest
		if decode(&req) {
			withLink(s.ReissueClient(ctx, req.Name))
		}
	case "clients/delete":
		var req nameRequest
		if decode(&req) {
			reply(nil, s.RemoveClient(ctx, req.Name))
		}
	case "relays":
		var req nameRequest
		if decode(&req) {
			link, err := s.IssueRelay(req.Name)
			reply(LinkResponse{Name: req.Name, Link: link, QR: QRDataURI(link)}, err)
		}
	case "relays/delete":
		var req nameRequest
		if decode(&req) {
			reply(nil, s.RemoveRelay(ctx, req.Name))
		}
	case "upstreams":
		var req upstreamRequest
		if decode(&req) {
			reply(nil, s.AddUpstream(ctx, req.Name, req.Link))
		}
	case "upstreams/delete":
		var req nameRequest
		if decode(&req) {
			reply(nil, s.RemoveUpstream(req.Name))
		}
	case "explore":
		var req routeRequest
		if decode(&req) {
			names, err := s.Explore(ctx, req.Route)
			reply(map[string][]string{"upstreams": names}, err)
		}
	case "cascade/probe":
		s.ProbeNow(ctx)
		reply(map[string]any{"latency": s.Latencies()}, nil)
	case "cascade/optimize":
		s.ProbeNow(ctx)
		events := s.OptimizeNow(ctx)
		if events == nil {
			events = []RerouteEvent{}
		}
		reply(map[string]any{"events": events}, nil)
	case "settings/front":
		var req struct {
			SNI  string `json:"sni"`
			Dest string `json:"dest"`
		}
		if decode(&req) {
			err := s.SetFront(req.SNI, req.Dest)
			resp := map[string]string{"ok": "1"}
			if err == nil && trusted {
				resp["sni"], resp["key"] = s.FrontInfo().SNI, s.FrontKey()
			}
			reply(resp, err)
		}
	case "settings/domain":
		var req domainRequest
		if decode(&req) {
			reply(nil, s.SetDomain(req.Domain))
		}
	case "settings/addresses":
		var req struct {
			IPv4 string `json:"ipv4"`
			IPv6 string `json:"ipv6"`
		}
		if decode(&req) {
			reply(nil, s.SetPublicAddresses(req.IPv4, req.IPv6))
		}
	case "settings/password":
		var req passwordRequest
		if decode(&req) {
			var current *string
			if !trusted {
				current = &req.Current
			}
			reply(nil, s.SetAdminPassword(current, req.New))
		}
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}
