// Package assets — общие встроенные ресурсы: шрифт Montserrat (SIL Open Font License, см. fonts/OFL.txt)
// для графического приложения и админ-панели сервера.
package assets

import "embed"

// Fonts — каталог fonts/ целиком; файлы читаются как Fonts.ReadFile("fonts/<имя>").
//
//go:embed fonts/*.ttf fonts/OFL.txt
var Fonts embed.FS

var (
	//go:embed fonts/Montserrat-Medium.ttf
	FontMedium []byte
	//go:embed fonts/Montserrat-SemiBold.ttf
	FontSemiBold []byte
	//go:embed fonts/Montserrat-Bold.ttf
	FontBold []byte
)
