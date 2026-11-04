package gui

// curLang — язык интерфейса ("ru" или "en"). Строки пишутся парами, как в макете: L("Connect", "Подключить").
var curLang = "ru"

func L(en, ru string) string {
	if curLang == "en" {
		return en
	}
	return ru
}
