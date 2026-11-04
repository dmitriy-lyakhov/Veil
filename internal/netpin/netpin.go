// Package netpin привязывает собственные соединения Veil к физическому
// сетевому интерфейсу (Windows), чтобы они не попадали в собственный туннель.
package netpin

import "sync/atomic"

var v4, v6 atomic.Uint32

// Set задаёт индексы физических интерфейсов для IPv4 и IPv6 (0 — не привязывать).
func Set(a, b uint32) {
	v4.Store(a)
	v6.Store(b)
}

func Get() (uint32, uint32) { return v4.Load(), v6.Load() }
