// Package i18n — язык интерфейса. Строки пишутся парами, как в макете: L("Connect", "Подключить").
package i18n

// Lang — язык интерфейса ("ru" или "en").
var Lang = "ru"

// L возвращает английский или русский вариант строки в зависимости от Lang.
func L(en, ru string) string {
	if Lang == "en" {
		return en
	}
	return ru
}
