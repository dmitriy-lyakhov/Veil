package gui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"

	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/server"
)

// showSharedCascade — страница «Общий каскад» (дизайн «cascadeShared»): получатель увидит одно подключение;
// отозвать его можно сразу на всех входах. Кнопка «Отозвать» закреплена внизу страницы.
func (u *ui) showSharedCascade(parent fyne.Window, r server.LinkResponse, job multiJob) {
	var names []string
	for _, e := range job.entries {
		names = append(names, e.Name)
	}
	extra := []fyne.CanvasObject{
		infoRow(L("Entries", "Входы"), strings.Join(names, ", "), nil),
		infoRow(L("Exit", "Выход"), job.exit.Name, nil),
	}
	var page *subPage
	revoke := pageButtonDanger(L("Revoke on all entries", "Отозвать у всех входов"), "trash", func() {
		showConfirm(L("Revoke the cascade", "Отозвать каскад"), fmt.Sprintf(L("The client “%s” will be deleted on every entry: the connection stops for everyone who got this link. Continue?", "Клиент «%s» будет удалён на каждом входе: подключение прекратится у всех, кто получил эту ссылку. Продолжить?"), r.Name), func(ok bool) {
			if ok {
				u.revokeEverywhere(page, r.Name, job.entries)
			}
		}, page)
	})
	page = u.showShareX(parent, L("Shared cascade", "Общий каскад"), r, L("The recipient will see one connection. Scan the QR or send the link.", "Получатель увидит одно подключение. Отсканируйте QR или отправьте ссылку."), true, extra, []fyne.CanvasObject{revoke})
}

// revokeText — итог отзыва для окна сообщения.
func revokeText(done, failed []string) string {
	if len(failed) == 0 {
		return fmt.Sprintf(L("Revoked on all entries (%d).", "Каскад отозван у всех входов (%d)."), len(done))
	}
	return fmt.Sprintf(L("Revoked: %s.\nFailed:\n%s", "Отозван: %s.\nНе удалось:\n%s"), strings.Join(done, ", "), strings.Join(failed, "\n"))
}

// revokeEverywhere удаляет клиента с этим именем (clients/delete) на каждом из серверов-входов по очереди
// и сообщает итог: на каких серверах отозвано, на каких не удалось.
func (u *ui) revokeEverywhere(parent *subPage, name string, entries []*config.Profile) {
	call := func(e *config.Profile, name string, cb func(error)) {
		u.withSession(parent, e, func(s *deploy.Session, err error) {
			if err != nil {
				cb(err)
				return
			}
			go func() {
				err := s.Call("POST", "/clients/delete", map[string]string{"name": name}, nil)
				fyne.Do(func() { cb(err) })
			}()
		})
	}
	revokeAcross(entries, name, call, shortErr, func(done, failed []string) {
		showInfo(L("Revoke the cascade", "Отозвать каскад"), revokeText(done, failed), parent)
		if len(failed) == 0 {
			parent.Close()
		}
	})
}
