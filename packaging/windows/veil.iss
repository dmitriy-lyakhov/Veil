; Установщик Veil для Windows (Inno Setup 6). Сборка:
;   ISCC.exe /DAppVersion=1.1.0 /DSrcDir=<папка с veil.exe, veil-cli.exe и server\> /DOutDir=<куда> packaging\windows\veil.iss
; Ставит без прав администратора в %LOCALAPPDATA%\Programs\Veil (папку можно выбрать), делает ярлыки и деинсталлятор.
; Настройки и профили (%APPDATA%\Veil) при удалении по умолчанию сохраняются.

#ifndef AppVersion
  #define AppVersion "1.1.0"
#endif
#ifndef SrcDir
  #define SrcDir "..\..\dist\windows"
#endif
#ifndef OutDir
  #define OutDir "..\..\dist"
#endif

[Setup]
AppId={{6C1E0B7A-3D52-4F0B-9B6E-7E6F2B7C4A11}
AppName=Veil
AppVersion={#AppVersion}
AppVerName=Veil {#AppVersion}
AppPublisher=Veil
AppPublisherURL=https://github.com/dmitriy-lyakhov/Veil
AppSupportURL=https://github.com/dmitriy-lyakhov/Veil/issues
VersionInfoVersion={#AppVersion}
DefaultDirName={autopf}\Veil
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
SetupIconFile=veil.ico
UninstallDisplayIcon={app}\veil.exe
UninstallDisplayName=Veil
OutputDir={#OutDir}
OutputBaseFilename=Veil-{#AppVersion}-windows-setup
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
CloseApplications=yes
RestartApplications=no
UsePreviousAppDir=yes

[Languages]
Name: "russian"; MessagesFile: "compiler:Languages\Russian.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#SrcDir}\veil.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SrcDir}\veil-cli.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SrcDir}\server\*"; DestDir: "{app}\server"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{autoprograms}\Veil"; Filename: "{app}\veil.exe"; WorkingDir: "{app}"
Name: "{autodesktop}\Veil"; Filename: "{app}\veil.exe"; WorkingDir: "{app}"; Tasks: desktopicon

[Registry]
; запись старого установщика (IExpress, версии 0.6–0.7 до мастера) заменяется записью Inno Setup
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Uninstall\Veil"; Flags: deletekey

[InstallDelete]
Type: files; Name: "{app}\uninstall.cmd"

[Run]
; обновление из самого приложения (/SILENT /RELAUNCH=1): после установки запускаем новую версию
Filename: "{app}\veil.exe"; Flags: nowait; Check: RelaunchRequested
Filename: "{app}\veil.exe"; Description: "{cm:LaunchProgram,Veil}"; Flags: nowait postinstall skipifsilent

[Code]
function RelaunchRequested: Boolean;
begin
  Result := ExpandConstant('{param:RELAUNCH|0}') = '1';
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Dir: String;
  Ask: String;
begin
  if CurUninstallStep = usPostUninstall then
  begin
    Dir := ExpandConstant('{userappdata}\Veil');
    if DirExists(Dir) and not UninstallSilent then
    begin
      if ActiveLanguage = 'russian' then
        Ask := 'Удалить настройки и сохранённые серверы Veil (' + Dir + ')?' + #13#10 + 'Если вы планируете установить Veil снова, нажмите «Нет».'
      else
        Ask := 'Delete Veil settings and saved servers (' + Dir + ')?' + #13#10 + 'If you plan to install Veil again, choose No.';
      if MsgBox(Ask, mbConfirmation, MB_YESNO or MB_DEFBUTTON2) = IDYES then
        DelTree(Dir, True, True, True);
    end;
  end;
end;
