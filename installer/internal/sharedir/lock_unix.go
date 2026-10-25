//go:build unix

package sharedir

import (
	"fmt"
	"os"
	"syscall"
)

// lock nimmt die Dateisperre und gibt die Freigabe zurück.
//
// Blockierend mit flock wie in vscodeext: nach einem Programmwechsel starten
// oft mehrere Aufrufe zugleich (MCP-Server mehrerer Sitzungen), und zwei
// gleichzeitige Tauschvorgänge auf dasselbe Verzeichnis sind nicht
// abgesprochen. Die Sperrdatei liegt im Zielverzeichnis selbst: Teilen sich
// Host und Container das Verzeichnis, teilen sie damit auch die Sperre.
func lock(path string) (release func(), err error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("Sperre %s öffnen: %w", path, err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("Sperre %s nehmen: %w", path, err)
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}
