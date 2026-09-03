//go:build unix

package guiproc

import (
	"errors"
	"syscall"
)

// processAlive prüft per Signal 0, ob die PID vergeben ist. EPERM heißt: der
// Prozess existiert, gehört aber jemand anderem — er lebt.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Terminate schickt SIGTERM. Der Server behandelt es wie Ctrl+C: Laufzeitdatei
// entfernen, beenden. Vorher muss IdentityMatches bestätigt haben, dass die
// PID noch der eigene Prozess ist.
func Terminate(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}
