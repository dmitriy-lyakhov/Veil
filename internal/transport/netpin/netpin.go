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

// Reachable сообщает, можно ли открыть сокет к адресу из этого семейства, не попав в туннель: пока VPN подключён (задана привязка
// хотя бы для одного семейства), адрес семейства без привязки ушёл бы в туннель (например, IPv6 на компьютере без IPv6-доступа).
// Без активного VPN привязки нет и проверять нечего.
func Reachable(v6Family bool) bool {
	a, b := Get()
	if a == 0 && b == 0 {
		return true
	}
	if v6Family {
		return b != 0
	}
	return a != 0
}
