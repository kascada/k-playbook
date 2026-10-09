//go:build unix

package vscodeext

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// LockName ist der Name der Sperrdatei im Laufzeitverzeichnis.
const LockName = "vscodeext.lock"

// lock nimmt die Dateisperre und gibt die Freigabe zurück.
//
// Gesperrt wird mit flock und blockierend: mehrere Projekt-Dienste können
// gleichzeitig starten, und zwei gleichzeitige `--install-extension … --force`
// auf dasselbe Erweiterungsverzeichnis sind nicht abgesprochen. Wer wartet,
// prüft danach neu und findet in der Regel nichts mehr zu tun. flock gibt die
// Sperre mit dem Prozess frei, auch nach einem kill — eine Sperrdatei mit PID
// müsste Verwaistes selbst erkennen.
//
// Der Ort ist das Laufzeitverzeichnis des Dienstes ($XDG_RUNTIME_DIR, sonst
// $XDG_STATE_HOME, sonst ~/.local/state): dieselbe Grenze, die schon Host und
// Dev Container trennt. Ein Container hat seine eigene VS-Code-Umgebung und
// braucht die Sperre des Hosts nicht.
func lock(dir string) (release func(), err error) {
	if dir == "" {
		return nil, fmt.Errorf("kein Verzeichnis für die Sperre")
	}
	path := filepath.Join(dir, LockName)
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
