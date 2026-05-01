; Inno Setup script for Stand Reminder
; Uses Inno Setup 6.x

#define MyAppName "Stand Reminder"
#define MyAppVersion "0.6.3"
#define MyAppPublisher "sheetung"
#define MyAppURL "https://github.com/sheetung/stand-Reminder"
#define MyAppExeName "stand-reminder.exe"

[Setup]
AppId={{B8F4A3D2-1C5E-4F7A-9B6D-3E2F1C8A0D5B}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
AllowNoIcons=yes
OutputDir=dist
OutputBaseFilename=StandReminder-{#MyAppVersion}-Setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
UninstallDisplayIcon={app}\{#MyAppExeName}
ArchitecturesInstallIn64BitMode=x64compatible

[Tasks]
Name: "desktopicon"; Description: "Create a &desktop shortcut"; GroupDescription: "Additional shortcuts:"
Name: "startup"; Description: "&Launch at Windows startup"; GroupDescription: "Additional options:"

[Files]
Source: "stand-reminder.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "assets\stand-reminder.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; AppUserModelID: "StandReminder.App"; IconFilename: "{app}\stand-reminder.ico"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"; IconFilename: "{app}\stand-reminder.ico"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon; AppUserModelID: "StandReminder.App"; IconFilename: "{app}\stand-reminder.ico"

[Registry]
; Add to Windows startup
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "Stand Reminder"; ValueData: """{app}\{#MyAppExeName}"""; Tasks: startup; Flags: uninsdeletevalue

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#MyAppName}}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "taskkill"; Parameters: "/f /im stand-reminder.exe"; Flags: runhidden
