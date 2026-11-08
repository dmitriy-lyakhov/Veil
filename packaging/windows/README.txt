Установщик Windows — мастер на Inno Setup 6 (veil.iss): выбор языка и папки, ярлыки, значок, деинсталлятор в «Программах и компонентах».
Сборка (нужен установленный Inno Setup 6, значок вшивается в veil.exe файлом cmd/veil/veil_windows_amd64.syso):
  ISCC.exe /DAppVersion=1.1.0 /DSrcDir=<папка с veil.exe, veil-cli.exe и server\veil-server-linux-*> /DOutDir=<куда> packaging\windows\veil.iss
(в Git Bash перед этим: export MSYS_NO_PATHCONV=1). Установка идёт в %LOCALAPPDATA%\Programs\Veil без прав администратора.
Значок: packaging\windows\veil.ico делается из packaging\macos\icon.png (go run ./tools/mkico), ресурс — windres -O coff -i cmd/veil/veil_windows.rc -o cmd/veil/veil_windows_amd64.syso.
Старые скрипты install.cmd / uninstall.cmd (установщик на IExpress, версии до мастера) оставлены для справки.
Портативная версия: папка с veil.exe, veil-cli.exe и server\ в архиве.
