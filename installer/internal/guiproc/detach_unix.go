//go:build unix

package guiproc

import "syscall"

// detachAttr löst den Server vom Terminal des Aufrufs: eine eigene Sitzung,
// damit ihn weder ein Ctrl+C noch das Schließen des Terminals mitnimmt. Gilt
// für Linux und macOS; eine Windows-Fassung bräuchte eine eigene Datei.
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
