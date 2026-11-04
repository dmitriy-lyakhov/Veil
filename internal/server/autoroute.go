package server

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"slices"
	"strings"
	"sync"
	"time"

	"veil/internal/config"
)

const (
	maxSolverNodes   = 20
	rerouteGain      = 0.2
	rerouteMinGainMs = 5
	AllHops          = -1
)

type graph struct {
	ids       []string
	idx       map[string]int
	cost      [][]float64
	names     map[[2]int]string
	companies map[string]string
}

func newGraph() *graph {
	return &graph{idx: map[string]int{}, names: map[[2]int]string{}, companies: map[string]string{}}
}

func (g *graph) node(id string) int {
	if i, ok := g.idx[id]; ok {
		return i
	}
	i := len(g.ids)
	g.ids = append(g.ids, id)
	g.idx[id] = i
	for r := range g.cost {
		g.cost[r] = append(g.cost[r], math.Inf(1))
	}
	row := make([]float64, i+1)
	for c := range row {
		row[c] = math.Inf(1)
	}
	g.cost = append(g.cost, row)
	return i
}

func (g *graph) edge(from, to string, ms float64, name string) {
	a, b := g.node(from), g.node(to)
	if a == b {
		return
	}
	g.cost[a][b] = ms
	g.names[[2]int{a, b}] = name
}

func (g *graph) pathCost(path []int) float64 {
	total := 0.0
	for i := 1; i < len(path); i++ {
		total += g.cost[path[i-1]][path[i]]
	}
	return total
}

func (g *graph) indices(ids []string) ([]int, bool) {
	out := make([]int, len(ids))
	for i, id := range ids {
		n, ok := g.idx[id]
		if !ok {
			return nil, false
		}
		out[i] = n
	}
	return out, true
}

func (g *graph) routeNames(path []int) ([]string, error) {
	names := make([]string, 0, len(path)-1)
	for i := 1; i < len(path); i++ {
		n, ok := g.names[[2]int{path[i-1], path[i]}]
		if !ok {
			return nil, errors.New("нет связи между узлами маршрута")
		}
		names = append(names, n)
	}
	return names, nil
}

func solve(g *graph, src, dst, maxInter int) ([]int, float64, error) {
	var cand []int
	for i := range g.ids {
		if i != src && i != dst {
			cand = append(cand, i)
		}
	}
	if len(cand) > maxSolverNodes {
		return nil, 0, fmt.Errorf("слишком много узлов для автомаршрута (%d, максимум %d)", len(cand), maxSolverNodes)
	}
	k := len(cand)
	if maxInter >= 0 && maxInter < k {
		k = maxInter
	}
	m := len(cand)
	full := 1 << m
	dp := make([][]float64, full)
	parent := make([][]int, full)
	for mask := range dp {
		dp[mask] = make([]float64, m)
		parent[mask] = make([]int, m)
		for j := range dp[mask] {
			dp[mask][j] = math.Inf(1)
			parent[mask][j] = -1
		}
	}
	for j := 0; j < m; j++ {
		dp[1<<j][j] = g.cost[src][cand[j]]
	}
	for mask := 1; mask < full; mask++ {
		if bits.OnesCount(uint(mask)) >= k {
			continue
		}
		for j := 0; j < m; j++ {
			cur := dp[mask][j]
			if mask&(1<<j) == 0 || math.IsInf(cur, 1) {
				continue
			}
			for n := 0; n < m; n++ {
				if mask&(1<<n) != 0 {
					continue
				}
				next := mask | 1<<n
				if c := cur + g.cost[cand[j]][cand[n]]; c < dp[next][n] {
					dp[next][n] = c
					parent[next][n] = j
				}
			}
		}
	}
	for want := k; want >= 0; want-- {
		if want == 0 {
			if c := g.cost[src][dst]; !math.IsInf(c, 1) {
				return []int{src, dst}, c, nil
			}
			break
		}
		bestCost, bestMask, bestJ := math.Inf(1), -1, -1
		for mask := 1; mask < full; mask++ {
			if bits.OnesCount(uint(mask)) != want {
				continue
			}
			for j := 0; j < m; j++ {
				if c := dp[mask][j] + g.cost[cand[j]][dst]; c < bestCost {
					bestCost, bestMask, bestJ = c, mask, j
				}
			}
		}
		if bestMask < 0 {
			continue
		}
		var rev []int
		for mask, j := bestMask, bestJ; j >= 0; {
			rev = append(rev, cand[j])
			pj := parent[mask][j]
			mask &^= 1 << j
			j = pj
		}
		path := []int{src}
		for i := len(rev) - 1; i >= 0; i-- {
			path = append(path, rev[i])
		}
		return append(path, dst), bestCost, nil
	}
	return nil, 0, errors.New("нет ни одного рабочего пути до выходного узла — проверьте связи каскада")
}

func (s *Server) gatherGraph(ctx context.Context) (*graph, string, error) {
	g := newGraph()
	s.mu.Lock()
	self := s.ownPublicKey()
	g.companies[self] = s.cfg.Site.Company
	s.mu.Unlock()
	g.node(self)
	for _, l := range s.Latencies() {
		g.companies[l.ID] = l.Company
		if l.OK {
			g.edge(self, l.ID, l.RTTms, l.Name)
		}
	}
	ups := s.upstreamSnapshot()
	reports := make([]relayResponse, len(ups))
	var wg sync.WaitGroup
	for i, up := range ups {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, err := s.upstreamDialer(up.Name, "")
			if err != nil {
				return
			}
			cctx, cancel := context.WithTimeout(ctx, probeTimeout)
			defer cancel()
			_ = d.Call(cctx, relayAPISuffix, relayRequest{Op: relayOpLatency}, &reports[i])
		}()
	}
	wg.Wait()
	reported := map[string]bool{self: true}
	add := func(r relayResponse) {
		if r.Self == "" || reported[r.Self] {
			return
		}
		reported[r.Self] = true
		g.companies[r.Self] = r.Company
		g.node(r.Self)
		for _, l := range r.Links {
			g.companies[l.ID] = l.Company
			if l.OK {
				g.edge(r.Self, l.ID, l.RTTms, l.Name)
			}
		}
	}
	for _, r := range reports {
		add(r)
	}
	for i, up := range ups {
		r := reports[i]
		if r.Self == "" {
			continue
		}
		for _, l := range r.Links {
			if !l.OK || reported[l.ID] {
				continue
			}
			d, err := s.upstreamDialer(up.Name, "")
			if err != nil {
				continue
			}
			cctx, cancel := context.WithTimeout(ctx, probeTimeout)
			var via relayResponse
			if d.Call(cctx, relayAPISuffix, relayRequest{Op: relayOpLatency, Route: []string{l.Name}}, &via) == nil {
				add(via)
			}
			cancel()
		}
	}
	return g, self, nil
}

type plan struct {
	ids   []string
	names []string
	hops  []string
	cost  float64
}

func (s *Server) planRoute(g *graph, self, exit string, maxHops int) (plan, error) {
	dst, ok := g.idx[exit]
	if !ok {
		return plan{}, errors.New("выходной узел недоступен")
	}
	path, cost, err := solve(g, g.idx[self], dst, maxHops)
	if err != nil {
		return plan{}, err
	}
	names, err := g.routeNames(path)
	if err != nil {
		return plan{}, err
	}
	p := plan{names: names, cost: cost}
	for _, n := range path[1:] {
		p.ids = append(p.ids, g.ids[n])
	}
	for _, n := range path {
		p.hops = append(p.hops, g.companies[g.ids[n]])
	}
	return p, nil
}

func (s *Server) exitByUpstream(name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	up := s.upstreamLocked(name)
	if up == nil {
		return "", fmt.Errorf("на этом сервере нет узла «%s»", name)
	}
	return up.Link.ServerPublicKey, nil
}

func autoUpName(name string, gen int) string { return fmt.Sprintf("%s#%d", name, gen) }

func (s *Server) AddAutoClient(ctx context.Context, name, note, exitUpstream string, maxHops int) (*config.Profile, error) {
	return s.AddAutoClientWithKey(ctx, name, note, exitUpstream, maxHops, "")
}

func (s *Server) AddAutoClientWithKey(ctx context.Context, name, note, exitUpstream string, maxHops int, publicKey string) (*config.Profile, error) {
	name = strings.TrimSpace(name)
	if err := validClientName(name); err != nil {
		return nil, err
	}
	if strings.ContainsAny(name, "/#") {
		return nil, errors.New("имя не должно содержать «/» и «#»")
	}
	exit, err := s.exitByUpstream(exitUpstream)
	if err != nil {
		return nil, err
	}
	if len(s.Latencies()) == 0 {
		s.ProbeNow(ctx)
	}
	g, self, err := s.gatherGraph(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.planRoute(g, self, exit, maxHops)
	if err != nil {
		return nil, err
	}
	priv, pub, err := keyPair(publicKey)
	if err != nil {
		return nil, err
	}
	info, err := s.createPeer(ctx, name, strings.TrimSpace(note), "", pub, p.names, autoUpName(name, 1))
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if i := s.peerIndexLocked(name); i >= 0 {
		s.cfg.Peers[i].Auto = &config.AutoRoute{Exit: exit, ExitName: exitUpstream, MaxHops: maxHops, Path: p.ids, Gen: 1, CostMs: p.cost, Rerouted: time.Now().UTC().Truncate(time.Second)}
		if err := s.saveLocked(); err != nil {
			s.log.Error("save config", "err", err)
		}
	}
	s.mu.Unlock()
	s.setAutoCost(name, p.cost)
	info.Hops = p.hops
	prof := s.clientProfile(name, priv, info)
	return prof, nil
}

func (s *Server) setAutoCost(name string, ms float64) {
	s.autoMu.Lock()
	s.autoCost[name] = ms
	s.autoMu.Unlock()
}

func (s *Server) autoCostOf(name string) float64 {
	s.autoMu.Lock()
	defer s.autoMu.Unlock()
	return s.autoCost[name]
}

func (s *Server) hasAutoClients() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.ContainsFunc(s.cfg.Peers, func(p config.Peer) bool { return p.Auto != nil && !p.Disabled })
}

type RerouteEvent struct {
	Client string  `json:"client"`
	From   string  `json:"from"`
	To     string  `json:"to"`
	OldMs  float64 `json:"old_ms"`
	NewMs  float64 `json:"new_ms"`
	Error  string  `json:"error,omitempty"`
}

func (s *Server) OptimizeNow(ctx context.Context) []RerouteEvent {
	s.mu.Lock()
	var autos []config.Peer
	for _, p := range s.cfg.Peers {
		if p.Auto != nil && !p.Disabled && p.RelayOf == "" {
			autos = append(autos, p)
		}
	}
	s.mu.Unlock()
	if len(autos) == 0 {
		return nil
	}
	g, self, err := s.gatherGraph(ctx)
	if err != nil {
		return []RerouteEvent{{Error: err.Error()}}
	}
	var events []RerouteEvent
	for _, peer := range autos {
		p, err := s.planRoute(g, self, peer.Auto.Exit, peer.Auto.MaxHops)
		if err != nil {
			continue
		}
		curPath, known := g.indices(append([]string{self}, peer.Auto.Path...))
		cur := math.Inf(1)
		if known {
			cur = g.pathCost(curPath)
		}
		s.setAutoCost(peer.Name, cur)
		key := strings.Join(p.ids, ",")
		if key == strings.Join(peer.Auto.Path, ",") {
			s.clearCandidate(peer.Name)
			continue
		}
		broken := math.IsInf(cur, 1)
		better := p.cost <= cur*(1-rerouteGain) && cur-p.cost >= rerouteMinGainMs
		if !broken && !better {
			s.clearCandidate(peer.Name)
			continue
		}
		if !broken && !s.confirmCandidate(peer.Name, key) {
			continue
		}
		ev := RerouteEvent{Client: peer.Name, From: strings.Join(peer.Route, " → "), To: strings.Join(p.names, " → "), OldMs: cur, NewMs: p.cost}
		if math.IsInf(cur, 1) {
			ev.OldMs = -1
		}
		if err := s.reroute(ctx, peer, p); err != nil {
			ev.Error = err.Error()
			s.log.Warn("auto reroute failed", "client", peer.Name, "err", err)
		} else {
			s.log.Info("auto reroute", "client", peer.Name, "from", ev.From, "to", ev.To, "old_ms", ev.OldMs, "new_ms", ev.NewMs)
		}
		s.clearCandidate(peer.Name)
		events = append(events, ev)
	}
	return events
}

func (s *Server) confirmCandidate(name, key string) bool {
	s.autoMu.Lock()
	defer s.autoMu.Unlock()
	if s.candidates[name] == key {
		return true
	}
	s.candidates[name] = key
	return false
}

func (s *Server) clearCandidate(name string) {
	s.autoMu.Lock()
	delete(s.candidates, name)
	s.autoMu.Unlock()
}

func (s *Server) reroute(ctx context.Context, peer config.Peer, p plan) error {
	gen := peer.Auto.Gen + 1
	upName := autoUpName(peer.Name, gen)
	d, err := s.upstreamDialer(p.names[0], "")
	if err != nil {
		return err
	}
	var resp relayResponse
	if err := d.Call(ctx, relayAPISuffix, relayRequest{Op: relayOpRegister, Name: upName, PublicKey: peer.PublicKey, Route: p.names[1:]}, &resp); err != nil {
		return fmt.Errorf("узел %s: %w", p.names[0], err)
	}
	if resp.Address4 != peer.Address4 || resp.ServerPublicKey == "" {
		_ = d.Call(ctx, relayAPISuffix, relayRequest{Op: relayOpRemove, Name: upName}, nil)
		return errors.New("выходной узел выдал другие адреса — маршрут не изменён")
	}
	s.mu.Lock()
	i := s.peerIndexLocked(peer.Name)
	if i < 0 {
		s.mu.Unlock()
		return errors.New("клиент удалён во время перестройки")
	}
	cur := &s.cfg.Peers[i]
	oldRoute, oldUp := cur.Route, cur.UpName
	cur.Route, cur.UpAuth, cur.UpName = append([]string(nil), p.names...), resp.AuthKey, upName
	cur.Auto.Path, cur.Auto.Gen, cur.Auto.CostMs, cur.Auto.Rerouted = p.ids, gen, p.cost, time.Now().UTC().Truncate(time.Second)
	if err := s.saveLocked(); err != nil {
		s.log.Error("save config", "err", err)
	}
	s.mu.Unlock()
	s.setAutoCost(peer.Name, p.cost)
	s.killSessions(func(n string) bool { return n == peer.Name })
	if len(oldRoute) > 0 {
		if od, err := s.upstreamDialer(oldRoute[0], ""); err == nil {
			if err := od.Call(ctx, relayAPISuffix, relayRequest{Op: relayOpRemove, Name: oldUp}, nil); err != nil {
				s.log.Info("old route cleanup", "client", peer.Name, "err", err)
			}
		}
	}
	return nil
}
