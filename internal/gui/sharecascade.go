package gui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"

	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/server"
)

// showSharedCascade — окно результата «Поделиться каскадом»: получатель увидит одно подключение;
// отозвать его можно сразу на всех входах.
func (u *ui) showSharedCascade(parent fyne.Window, r server.LinkResponse, job multiJob) {
	var names []string
	for _, e := range job.entries {
		names = append(names, e.Name)
	}
	before := []fyne.CanvasObject{
		infoRow(L("Entries", "Входы"), strings.Join(names, ", "), nil),
		infoRow(L("Exit", "Выход"), job.exit.Name, nil),
	}
	revoke := dangerButton(L("Revoke on all entries", "Отозвать у всех входов"), func() {
		showConfirm(L("Revoke the cascade", "Отозвать каскад"), fmt.Sprintf(L("The client “%s” will be deleted on every entry: the connection stops for everyone who got this link. Continue?", "Клиент «%s» будет удалён на каждом входе: подключение прекратится у всех, кто получил эту ссылку. Продолжить?"), r.Name), func(ok bool) {
			if ok {
				u.revokeEverywhere(parent, r.Name, job.entries)
			}
		}, parent)
	})
	after := []fyne.CanvasObject{
		revoke,
		noteIcon("lock", L("Revoking stops the connection for everyone who got this link, on every entry at once.", "Отзыв прекращает подключение у всех, кто получил эту ссылку, сразу на всех входах."), colMuted),
	}
	u.showShareX(parent, L("Shared cascade", "Общий каскад"), r, L("The recipient will see one connection. Scan the QR or send the link. Shown once: the config contains a private key.", "Получатель увидит одно подключение. Отсканируйте QR или отправьте ссылку. Показывается один раз: в конфиге приватный ключ."), true, before, after)
}

// revokeEverywhere удаляет клиента с этим именем (clients/delete) на каждом из серверов-входов по очереди
// и сообщает итог: на каких серверах отозвано, на каких не удалось.
func (u *ui) revokeEverywhere(parent fyne.Window, name string, entries []*config.Profile) {
	var done, failed []string
	var step func(i int)
	step = func(i int) {
		if i == len(entries) {
			msg := fmt.Sprintf(L("Revoked on all entries (%d).", "Каскад отозван у всех входов (%d)."), len(done))
			if len(failed) > 0 {
				msg = fmt.Sprintf(L("Revoked: %s.\nFailed:\n%s", "Отозван: %s.\nНе удалось:\n%s"), strings.Join(done, ", "), strings.Join(failed, "\n"))
			}
			showInfo(L("Revoke the cascade", "Отозвать каскад"), msg, parent)
			return
		}
		e := entries[i]
		u.withSession(parent, e, func(s *deploy.Session, err error) {
			if err != nil {
				failed = append(failed, e.Name+": "+shortErr(err))
				step(i + 1)
				return
			}
			go func() {
				err := s.Call("POST", "/clients/delete", map[string]string{"name": name}, nil)
				fyne.Do(func() {
					if err != nil {
						failed = append(failed, e.Name+": "+shortErr(err))
					} else {
						done = append(done, e.Name)
					}
					step(i + 1)
				})
			}()
		})
	}
	step(0)
}
