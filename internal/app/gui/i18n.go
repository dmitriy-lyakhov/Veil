package gui

import "veil/internal/app/gui/i18n"

// L — короткое имя для i18n.L: строки интерфейса пишутся парами L("Connect", "Подключить").
func L(en, ru string) string { return i18n.L(en, ru) }
