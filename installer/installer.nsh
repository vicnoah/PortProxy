; Installer hooks for PortProxy.
;
; mygo's NSIS installer already installs a newer version into the folder of
; the installed one (InstallDirRegKey) and closes the running app before it
; replaces the files. PortProxy runs elevated, though, so a non-elevated
; installer cannot send WM_CLOSE to it (UIPI). The app polls this signal
; file and quits itself, which lets the in-place overwrite succeed.
;
; customInit runs in .onInit, before the installer's CloseApp step. Write
; the signal, give the app a moment to exit (it deletes the file as it
; quits), then remove any leftover so a later launch never sees a stale one.

!macro customInit
  FileOpen $0 "$TEMP\PortProxy-update.quit" w
  FileClose $0
  Sleep 1200
  Delete "$TEMP\PortProxy-update.quit"
!macroend
