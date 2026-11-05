package gui

import (
	"time"

	"veil/internal/config"
	"veil/internal/server"
)

// revokeCall удаляет клиента name на одном сервере-входе и сообщает результат через cb.
// Вызов асинхронный (может ждать пароль SSH), поэтому логика отзыва принимает его как параметр: в тестах он подменяется.
type revokeCall func(e *config.Profile, name string, cb func(error))

// revokeAcross отзывает клиента на каждом входе по очереди (один сбой не останавливает остальных)
// и в конце вызывает finish со списком успехов («имя») и сбоев («имя: причина»).
func revokeAcross(entries []*config.Profile, name string, call revokeCall, errText func(error) string, finish func(done, failed []string)) {
	var done, failed []string
	var step func(i int)
	step = func(i int) {
		if i == len(entries) {
			finish(done, failed)
			return
		}
		e := entries[i]
		call(e, name, func(err error) {
			if err != nil {
				failed = append(failed, e.Name+": "+errText(err))
			} else {
				done = append(done, e.Name)
			}
			step(i + 1)
		})
	}
	step(0)
}

// siteRepoPoller опрашивает состояние сайта-прикрытия, пока сервер загружает репозиторий.
// Источник состояния и ожидание подставляются (в тестах — подменённые), чтобы не ждать реальные минуты.
type siteRepoPoller struct {
	fetch    func() (server.SiteRepoInfo, error)
	sleep    func(time.Duration)
	interval time.Duration
	limit    time.Duration
	onState  func(server.SiteRepoInfo)
}

type pollEnd int

const (
	pollDone    pollEnd = iota // сервер закончил загрузку
	pollTimeout                // загрузка не завершилась за limit
	pollError                  // не удалось спросить состояние
)

func (p siteRepoPoller) run() pollEnd {
	var waited time.Duration
	for waited < p.limit {
		p.sleep(p.interval)
		waited += p.interval
		st, err := p.fetch()
		if err != nil {
			return pollError
		}
		p.onState(st)
		if !st.Syncing {
			return pollDone
		}
	}
	return pollTimeout
}
