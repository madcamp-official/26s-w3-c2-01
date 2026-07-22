; Inno Setup script for Libra CLI.
; Compile with ISCC (Inno Setup 6+): iscc scripts\windows\libra.iss /DMyAppVersion=1.0.0
; Requires libra.exe to already be built at the repo root (go build -o libra.exe .).

#define MyAppName "Libra"
#define MyAppExeName "libra.exe"
#ifndef MyAppVersion
  #define MyAppVersion "0.0.0-dev"
#endif
#define MyAppPublisher "madcamp-official"
#define MyAppURL "https://github.com/madcamp-official/26s-w3-c2-01"

[Setup]
AppId={{7C4A9F1E-2B6D-4E8A-9C3F-1D5B8A6E2F40}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
DefaultDirName={localappdata}\Programs\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
; Per-user install: no admin/UAC prompt needed, and HKCU PATH edits below
; match this scope.
PrivilegesRequired=lowest
OutputDir=..\..\dist
OutputBaseFilename=libra-setup-{#MyAppVersion}
Compression=lzma2
SolidCompression=yes
UninstallDisplayIcon={app}\{#MyAppExeName}
WizardStyle=modern

[Languages]
Name: "korean"; MessagesFile: "compiler:Languages\Korean.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[CustomMessages]
korean.AddToPath=PATH 환경 변수에 추가 (터미널에서 바로 libra 명령 사용)
english.AddToPath=Add to PATH (lets you run "libra" from any terminal)

[Tasks]
Name: "addtopath"; Description: "{cm:AddToPath}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: checkedonce

[Files]
Source: "..\..\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion

[Code]
const
  EnvironmentKey = 'Environment';
  WM_SETTINGCHANGE = $1A;
  SMTO_ABORTIFHUNG = $0002;

function SendMessageTimeoutA(H: HWnd; Msg: UINT; WParam: Longint; LParam: PAnsiChar;
  Flags, Timeout: Integer; var Res: Integer): Integer;
  external 'SendMessageTimeoutA@user32.dll stdcall';

procedure RefreshEnvironment;
var
  Res: Integer;
begin
  SendMessageTimeoutA(HWND_BROADCAST, WM_SETTINGCHANGE, 0, 'Environment', SMTO_ABORTIFHUNG, 5000, Res);
end;

procedure EnvAddPath(Path: string);
var
  Paths: string;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, EnvironmentKey, 'Path', Paths) then
    Paths := '';

  { Already present? }
  if Pos(';' + Uppercase(Path) + ';', ';' + Uppercase(Paths) + ';') > 0 then
    exit;

  if (Length(Paths) > 0) and (Paths[Length(Paths)] <> ';') then
    Paths := Paths + ';';
  Paths := Paths + Path;

  if RegWriteStringValue(HKEY_CURRENT_USER, EnvironmentKey, 'Path', Paths) then
    RefreshEnvironment;
end;

procedure EnvRemovePath(Path: string);
var
  Paths: string;
  P: Integer;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, EnvironmentKey, 'Path', Paths) then
    exit;

  P := Pos(';' + Uppercase(Path) + ';', ';' + Uppercase(Paths) + ';');
  if P = 0 then
  begin
    { Path might be at the very end without a trailing semicolon. }
    if Pos(Uppercase(Path), Uppercase(Paths)) = Length(Paths) - Length(Path) + 1 then
      P := Length(Paths) - Length(Path) + 2
    else
      exit;
  end;

  Delete(Paths, P - 1, Length(Path) + 1);

  if RegWriteStringValue(HKEY_CURRENT_USER, EnvironmentKey, 'Path', Paths) then
    RefreshEnvironment;
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if (CurStep = ssPostInstall) and WizardIsTaskSelected('addtopath') then
    EnvAddPath(ExpandConstant('{app}'));
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usPostUninstall then
    EnvRemovePath(ExpandConstant('{app}'));
end;
