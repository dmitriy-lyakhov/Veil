package server

import (
	"context"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"time"

	"veil/internal/client/dialer"
	"veil/internal/config"
)

const (
	defaultProbeInterval    = 30 * time.Second
	defaultOptimizeInterval = 60 * time.Second
	probeTimeout            = 10 * time.Second
	ewmaAlpha               = 0.3
	staleAfter              = 3
)

type LinkLatency struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Company string    `json:"company,omitempty"`
	RTTms   float64   `json:"rtt_ms"`
	OK      bool      `json:"ok"`
	Updated time.Time `json:"updated,omitzero"`
	Error   string    `json:"error,omitempty"`
	fails   int
}

type latencyBook struct {
	mu    sync.Mutex
	links map[string]*LinkLatency
}

func (s *Server) SetIntervals(probe, optimize time.Duration) {
	s.tuneMu.Lock()
	s.probeEvery, s.optimizeEvery = probe, optimize
	s.tuneMu.Unlock()
}

func (s *Server) intervals() (time.Duration, time.Duration) {
	s.tuneMu.Lock()
	defer s.tuneMu.Unlock()
	return s.probeEvery, s.optimizeEvery
}

func jitter(d time.Duration) time.Duration {
	return d - d/4 + rand.N(d/2+1)
}

func (s *Server) backgroundLoops(ctx context.Context) {
	go func() {
		for {
			probe, _ := s.intervals()
			select {
			case <-ctx.Done():
				return
			case <-time.After(jitter(probe)):
			}
			s.ProbeNow(ctx)
		}
	}()
	go func() {
		for {
			_, optimize := s.intervals()
			select {
			case <-ctx.Done():
				return
			case <-time.After(jitter(optimize)):
			}
			if s.hasAutoClients() {
				s.OptimizeNow(ctx)
			}
		}
	}()
}

func (s *Server) upstreamSnapshot() []config.Upstream {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.cfg.Upstreams)
}

func (s *Server) ProbeNow(ctx context.Context) {
	ups := s.upstreamSnapshot()
	var wg sync.WaitGroup
	for _, up := range ups {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pctx, cancel := context.WithTimeout(ctx, probeTimeout)
			defer cancel()
			d := &dialer.Dialer{Profile: up.Link.DialProfile()}
			rtt, err := d.Probe(pctx, relayAPISuffix, relayRequest{Op: relayOpPing})
			s.recordProbe(up, rtt, err)
		}()
	}
	wg.Wait()
	s.lat.mu.Lock()
	for name := range s.lat.links {
		if !slices.ContainsFunc(ups, func(u config.Upstream) bool { return u.Name == name }) {
			delete(s.lat.links, name)
		}
	}
	s.lat.mu.Unlock()
}

func (s *Server) recordProbe(up config.Upstream, rtt time.Duration, err error) {
	s.lat.mu.Lock()
	defer s.lat.mu.Unlock()
	l := s.lat.links[up.Name]
	if l == nil || l.ID != up.Link.ServerPublicKey {
		l = &LinkLatency{ID: up.Link.ServerPublicKey, Name: up.Name}
		s.lat.links[up.Name] = l
	}
	l.Company = up.Link.Company
	if err != nil {
		l.fails++
		l.Error = err.Error()
		if l.fails >= staleAfter {
			l.OK = false
		}
		return
	}
	ms := float64(rtt.Microseconds()) / 1000
	if !l.OK || l.RTTms == 0 {
		l.RTTms = ms
	} else {
		l.RTTms = ewmaAlpha*ms + (1-ewmaAlpha)*l.RTTms
	}
	l.OK, l.fails, l.Error, l.Updated = true, 0, "", time.Now()
}

func (s *Server) Latencies() []LinkLatency {
	s.lat.mu.Lock()
	defer s.lat.mu.Unlock()
	out := make([]LinkLatency, 0, len(s.lat.links))
	for _, l := range s.lat.links {
		out = append(out, *l)
	}
	slices.SortFunc(out, func(a, b LinkLatency) int { return strings.Compare(a.Name, b.Name) })
	return out
}

func (s *Server) latencyReport() relayResponse {
	s.mu.Lock()
	company := s.cfg.Site.Company
	self := s.ownPublicKey()
	s.mu.Unlock()
	return relayResponse{Self: self, Company: company, Links: s.Latencies()}
}
