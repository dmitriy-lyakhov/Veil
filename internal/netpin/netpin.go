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

var name atomic.Value // string: имя выбранного интерфейса (Linux, macOS)

// SetName задаёт имя интерфейса, на котором должно работать приложение (пусто — автоматически).
func SetName(n string) { name.Store(n) }

// Name возвращает имя выбранного интерфейса.
func Name() string {
	s, _ := name.Load().(string)
	return s
}

var protect atomic.Pointer[func(fd int) bool]

// SetProtect задаёт хук защиты сокета от попадания в собственный туннель (Android: VpnService.protect).
// Хук вызывается из Control (Linux/Android) для каждого нового сокета до остальной привязки; nil — отключить.
func SetProtect(f func(fd int) bool) {
	if f == nil {
		protect.Store(nil)
		return
	}
	protect.Store(&f)
}

// protectFD вызывает хук (если задан); false — хук отказал.
func protectFD(fd int) bool {
	if p := protect.Load(); p != nil {
		return (*p)(fd)
	}
	return true
}
