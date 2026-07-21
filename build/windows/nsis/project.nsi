; Self-contained NSIS installer for geckty — no Wails or other framework
; macros (unlike termizard's wails_tools.nsh-based equivalent, which this
; is structurally modeled on but with that dependency stripped, since
; geckty is a plain `go build` binary, not a Wails app). Per-arch installer
; (pass /DARCH=amd64 or /DARCH=arm64 on the makensis command line — see
; build/windows/Taskfile.yml's create:installer task) rather than a single
; universal installer bundling both, for simplicity: this project cannot
; test NSIS output at all in its development environment (no makensis
; available — see the project plan's M10 write-up), so the simpler,
; easier-to-reason-about shape was chosen deliberately.
Unicode true

!include "tools.nsh"
!include "MUI2.nsh"

!ifndef ARCH
  !define ARCH "amd64"
!endif

VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"
VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

ManifestDPIAware true

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_FINISHPAGE_NOAUTOCLOSE
!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXECUTABLE}"
!define MUI_FINISHPAGE_RUN_TEXT "Launch geckty"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\..\bin\${INFO_PRODUCTNAME}-${ARCH}-installer.exe"

; Per-user install directory: no admin elevation required, matching how
; most modern portable-style Windows dev tools install (avoids UAC
; prompts and per-machine registry writes for a single-user terminal app).
InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
RequestExecutionLevel user
ShowInstDetails show

Section "Install"
    SetOutPath $INSTDIR
    File "..\..\..\bin\${PRODUCT_EXECUTABLE}"
    File "..\icon.ico"

    CreateDirectory "$SMPROGRAMS\${INFO_PRODUCTNAME}"
    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}" "" "$INSTDIR\icon.ico" 0
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}" "" "$INSTDIR\icon.ico" 0

    WriteUninstaller "$INSTDIR\uninstall.exe"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${INFO_PRODUCTNAME}" "DisplayName" "${INFO_PRODUCTNAME}"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${INFO_PRODUCTNAME}" "UninstallString" "$INSTDIR\uninstall.exe"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${INFO_PRODUCTNAME}" "DisplayVersion" "${INFO_PRODUCTVERSION}"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${INFO_PRODUCTNAME}" "Publisher" "${INFO_COMPANYNAME}"
    WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${INFO_PRODUCTNAME}" "NoModify" 1
    WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${INFO_PRODUCTNAME}" "NoRepair" 1
SectionEnd

Section "Uninstall"
    Delete "$INSTDIR\${PRODUCT_EXECUTABLE}"
    Delete "$INSTDIR\icon.ico"
    Delete "$INSTDIR\uninstall.exe"
    RMDir "$INSTDIR"

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}\${INFO_PRODUCTNAME}.lnk"
    RMDir "$SMPROGRAMS\${INFO_PRODUCTNAME}"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${INFO_PRODUCTNAME}"
SectionEnd
