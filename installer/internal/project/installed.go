package project

import (
	"errors"
	"os"
	"path/filepath"
)

// InstalledCommandName ist der Name, unter dem k-playbook einmal je Host oder
// DevContainer installiert und danach aufgerufen wird.
const InstalledCommandName = "k-playbook"

// BootstrapCommand ist die kanonische Form des Bootstraps: der Aufruf, mit dem
// ein Zielprojekt das zu seinem Clone passende Binary installiert.
// BootstrapCommandNoMake ist derselbe Weg ohne make.
//
// Beide stehen hier und nicht als Literal in den Meldungen, damit Oberfläche
// und Dokumentation nicht auseinanderlaufen. Wichtig ist die Form: ein
// Zielprojekt hat **kein eigenes** install-Target — sein Makefile ist das des
// Clones, und der Aufruf geht deshalb immer über den Clone. Ein bloßes
// `make install` liefe dort ins Leere.
const (
	BootstrapCommand       = "make -C " + PlaybookDirName + " install"
	BootstrapCommandNoMake = PlaybookDirName + "/bin/install"
)

// BootstrapHint nennt beide Formen in einem Satzteil — die Fassung, die in
// Meldungen eingesetzt wird.
const BootstrapHint = BootstrapCommand + " (ohne make: " + BootstrapCommandNoMake + ")"

// installedBinDirName und installedBinSubDir bilden das Verzeichnis, in das
// bin/install schreibt: ~/.local/bin. Die Aufteilung steht hier und nicht als
// zusammengesetzter Pfad, damit filepath.Join den Trenner der Plattform setzt.
const (
	installedBinDirName = ".local"
	installedBinSubDir  = "bin"
)

// ErrNoInstalledCommand meldet, dass sich kein installiertes k-playbook
// auflösen ließ. Dann wird keine Registrierung geschrieben: ein Eintrag, der
// auf nichts zeigt, ist schlechter als keiner.
var ErrNoInstalledCommand = errors.New("kein installiertes " + InstalledCommandName + " gefunden")

// InstalledCommandPath löst den absoluten Pfad des installierten k-playbook
// auf.
//
// Genau dieser Wert wird bei den Assistenten registriert. Ein bloßer
// Kommandoname wäre von der geerbten Shell-PATH abhängig, und aus Dock oder
// Finder gestartete Clients erben sie nicht: dort fehlt ~/.local/bin
// typischerweise, und der Eintrag wäre in genau den Umgebungen tot, in denen
// er gebraucht wird.
//
// Gesucht wird in zwei Schritten:
//
//  1. ~/.local/bin/k-playbook — das Ziel von bin/install und von
//     `make dev-install`. Es ist der kanonische Ort und wird deshalb zuerst
//     genommen, unabhängig davon, welches Binary gerade läuft.
//  2. der laufende Prozess selbst, wenn er k-playbook heißt. Das ist der Fall
//     eines Binaries, das jemand woandershin gelegt hat; sein Pfad ist
//     wenigstens ehrlich.
//
// Bewusst **nicht** gesucht wird über die PATH: ein PATH-Treffer wäre wieder
// von der Umgebung des Aufrufers abhängig, und in einem Entwicklungsrepo kann
// dort noch ein abgelöster Wrapper vor ~/.local/bin stehen.
func InstalledCommandPath() (string, error) {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidate := filepath.Join(home, installedBinDirName, installedBinSubDir, InstalledCommandName)
		if isExecutableFile(candidate) {
			return candidate, nil
		}
	}

	if self, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(self); err == nil {
			self = resolved
		}
		if filepath.Base(self) == InstalledCommandName && isExecutableFile(self) {
			return self, nil
		}
	}

	return "", ErrNoInstalledCommand
}

// isExecutableFile meldet, ob an path eine ausführbare Datei liegt. Symlinks
// werden dabei verfolgt: ein Link auf ein echtes Binary ist eine gültige
// Installation, und der Linkname bleibt der stabilere Eintrag.
func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}
