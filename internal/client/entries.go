package client

import (
	"veil/internal/netpin"
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"
	"time"

	"veil/internal/config"
)

const (
	entryProbeTimeout   = 3 * time.Second
	handshakeRoundTrips = 2
)

type EntryRTT struct {
	Name string        `json:"name"`
	Via  string        `json:"via,omitempty"`
	RTT  time.Duration `json:"rtt"`
	Err  string        `json:"error,omitempty"`
}

func (e EntryRTT) OK() bool { return e.Err == "" }

func ProbeEntries(ctx context.Context, p *config.Profile, dial DialFunc) []EntryRTT {
	if dial == nil {
		dial = (&net.Dialer{Control: netpin.Control}).DialContext
	}
	out := make([]EntryRTT, len(p.Entries))
	var wg sync.WaitGroup
	for i, e := range p.Entries {
		out[i] = EntryRTT{Name: e.Name, Via: e.Via}
		wg.Add(1)
		go func() {
			defer wg.Done()
			ep := p.ForEntry(e)
			targets, err := ep.Targets()
			if err != nil {
				out[i].Err = err.Error()
				return
			}
			ctx, cancel := context.WithTimeout(ctx, entryProbeTimeout)
			defer cancel()
			best := time.Duration(-1)
			var mu sync.Mutex
			var twg sync.WaitGroup
			for _, t := range targets {
				twg.Add(1)
				go func() {
					defer twg.Done()
					start := time.Now()
					c, err := dial(ctx, "tcp", t.Addr.String())
					if err != nil {
						return
					}
					stop := context.AfterFunc(ctx, func() { c.Close() })
					hs := &Dialer{Profile: ep}
					uc, err := hs.tlsOnly(c)
					stop()
					if err != nil {
						c.Close()
						return
					}
					rtt := time.Since(start) / handshakeRoundTrips
					uc.Close()
					mu.Lock()
					if best < 0 || rtt < best {
						best = rtt
					}
					mu.Unlock()
				}()
			}
			twg.Wait()
			if best < 0 {
				out[i].Err = "недоступен"
				return
			}
			out[i].RTT = best
		}()
	}
	wg.Wait()
	rank(out, p.PreferredEntry)
	return out
}

func rank(v []EntryRTT, preferred string) {
	slices.SortStableFunc(v, func(a, b EntryRTT) int {
		switch {
		case a.OK() != b.OK():
			if a.OK() {
				return -1
			}
			return 1
		case !a.OK():
			if a.Name == preferred {
				return -1
			}
			if b.Name == preferred {
				return 1
			}
			return 0
		case a.RTT < b.RTT:
			return -1
		case a.RTT > b.RTT:
			return 1
		}
		return 0
	})
}

func (d *Dialer) connectEntries(ctx context.Context) (Transport, error) {
	ranked := ProbeEntries(ctx, d.Profile, d.Dial)
	var errs []error
	for _, r := range ranked {
		idx := slices.IndexFunc(d.Profile.Entries, func(e config.Entry) bool { return e.Name == r.Name })
		if idx < 0 {
			continue
		}
		sub := &Dialer{Profile: d.Profile.ForEntry(d.Profile.Entries[idx]), Dial: d.Dial, Prefer: d.Prefer}
		t, err := sub.Connect(ctx)
		if err == nil {
			d.LastEntry, d.LastEntryRTT = r.Name, r.RTT
			return t, nil
		}
		errs = append(errs, fmt.Errorf("вход %s: %w", r.Name, err))
		if ctx.Err() != nil {
			break
		}
	}
	if len(errs) == 0 {
		return nil, errors.New("в профиле нет входов")
	}
	return nil, errors.Join(errs...)
}
