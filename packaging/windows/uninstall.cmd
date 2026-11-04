@echo off
taskkill /IM veil.exe /F >nul 2>&1
del "%APPDATA%\Microsoft\Windows\Start Menu\Programs\Veil.lnk" >nul 2>&1
reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\Veil" /f >nul 2>&1
rem Настройки и профили (%APPDATA%\Veil) не удаляются.
start "" /min cmd /c "ping -n 3 127.0.0.1 >nul & rmdir /s /q "%LOCALAPPDATA%\Programs\Veil""
exit /b 0
