package netsys

import "golang.zx2c4.com/wireguard/tun"

type manualUpTUN struct {
	tun.Device
	events chan tun.Event
}

func HoldUp(d tun.Device) tun.Device {
	w := &manualUpTUN{Device: d, events: make(chan tun.Event, 10)}
	go func() {
		defer close(w.events)
		for e := range d.Events() {
			if e&tun.EventUp != 0 {
				e &^= tun.EventUp
				if e == 0 {
					continue
				}
			}
			w.events <- e
		}
	}()
	return w
}

func (w *manualUpTUN) Events() <-chan tun.Event { return w.events }
