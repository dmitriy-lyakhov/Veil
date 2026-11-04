@echo off
setlocal
set "DST=%LOCALAPPDATA%\Programs\Veil"
taskkill /IM veil.exe /F >nul 2>&1
mkdir "%DST%\server" 2>nul
copy /Y "%~dp0veil.exe" "%DST%\" >nul
copy /Y "%~dp0veil-cli.exe" "%DST%\" >nul
copy /Y "%~dp0veil-server-linux-amd64" "%DST%\server\" >nul
copy /Y "%~dp0veil-server-linux-arm64" "%DST%\server\" >nul
copy /Y "%~dp0uninstall.cmd" "%DST%\" >nul
set "LNK=%APPDATA%\Microsoft\Windows\Start Menu\Programs\Veil.lnk"
powershell -NoProfile -ExecutionPolicy Bypass -Command "$s=(New-Object -ComObject WScript.Shell).CreateShortcut($env:LNK);$s.TargetPath=$env:DST+'\veil.exe';$s.WorkingDirectory=$env:DST;$s.Save()"
set "K=HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\Veil"
reg add "%K%" /v DisplayName /d "Veil" /f >nul
reg add "%K%" /v DisplayVersion /d "0.7.0" /f >nul
reg add "%K%" /v Publisher /d "Veil" /f >nul
reg add "%K%" /v DisplayIcon /d "%DST%\veil.exe" /f >nul
reg add "%K%" /v UninstallString /d "\"%DST%\uninstall.cmd\"" /f >nul
start "" "%DST%\veil.exe"
exit /b 0
