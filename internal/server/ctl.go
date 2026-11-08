package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultControlSocket = "/run/veil/ctl.sock"
	ctlTimeout           = 2 * time.Minute
)

func (s *Server) ServeControl(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	hs := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := strings.TrimPrefix(r.URL.Path, "/")
			if strings.HasPrefix(p, adminFrontCtl) {
				// процесс админ-панели (контейнер veil-admin): права как у веб-панели, не как у SSH
				s.handleAdminFront(w, r, strings.TrimPrefix(p, adminFrontCtl))
				return
			}
			s.handleAPI(w, r, p, true)
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go hs.Serve(ln)
	return func() { _ = hs.Close(); _ = os.Remove(path) }, nil
}

func Control(socket, method, path string, body []byte) ([]byte, error) {
	hc := &http.Client{
		Timeout: ctlTimeout,
		Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		}},
	}
	req, err := http.NewRequest(method, "http://veil/"+strings.TrimPrefix(path, "/"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("служба veil не отвечает: %w", err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return out, errors.New(strings.TrimSpace(string(out)))
	}
	return out, nil
}
