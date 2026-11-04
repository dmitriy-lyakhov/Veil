Windows-установщик собирается встроенным в Windows IExpress: положите рядом install.cmd, uninstall.cmd,
veil.exe, veil-cli.exe и server\veil-server-linux-* (плоско), составьте SED (InstallApp, AppLaunched=cmd /c install.cmd)
и выполните: iexpress /N /Q veil.sed. Установка идёт в %LOCALAPPDATA%\Programs\Veil без прав администратора.
Портативная версия: папка с veil.exe, veil-cli.exe и server\ в архиве.
