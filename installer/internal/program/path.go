package program

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// PathCheck ist das Ergebnis der Frage, ob `k-playbook` im PATH auf das
// Installationsziel zeigt.
type PathCheck struct {
	// Target ist das Installationsziel, ~/.local/bin/k-playbook.
	Target string `json:"target"`
	// Found ist, was der PATH unter `k-playbook` findet. Leer, wenn er nichts
	// findet.
	Found string `json:"found"`
	// OK: Found ist dieselbe Datei wie Target.
	OK bool `json:"ok"`
}

// CheckPath sieht nach, welche Datei ein Aufruf von `k-playbook` träfe, und
// vergleicht sie mit dem Installationsziel.
//
// Warum das vor jedem Angebot geprüft wird: installiert wird ausschließlich
// nach ~/.local/bin/k-playbook. Findet der PATH vorher eine andere Datei,
// startet der nächste `k-playbook`-Aufruf wieder das alte Programm, dessen
// reuseOrStart den eben gestarteten neuen Dienst als „anderen Stand" beendet —
// und der Fehler, den die Aktualisierung beheben sollte, ist zurück. In dem
// Fall gibt es deshalb keinen Knopf, sondern einen Hinweis mit beiden Pfaden.
// Eine fremde Datei wird weder entfernt noch überschrieben.
//
// Gelesen wird der PATH dieses Prozesses. Beim Dienst ist das der PATH der
// Shell, aus der `k-playbook` ihn gestartet hat — dieselbe, aus der auch der
// nächste Aufruf kommt.
//
// Verglichen wird über os.SameFile, nicht über den Pfadtext: ein Symlink im
// PATH auf das Ziel und ein Ziel, das selbst ein Symlink ist, zeigen auf
// dieselbe Datei und sind in Ordnung.
func CheckPath() PathCheck {
	var check PathCheck
	if target, err := project.InstallTargetPath(); err == nil {
		check.Target = target
	}
	found, err := exec.LookPath(project.InstalledCommandName)
	if err != nil && !errors.Is(err, exec.ErrDot) {
		return check
	}
	check.Found = found
	check.OK = check.Target != "" && sameFile(found, check.Target)
	return check
}

func sameFile(a string, b string) bool {
	infoA, err := os.Stat(a)
	if err != nil {
		return false
	}
	infoB, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(infoA, infoB)
}

// Hint ist der Hinweis, der statt des Knopfes erscheint. Er nennt beide Pfade
// und den Weg im Terminal. Leer, wenn der PATH stimmt.
func (c PathCheck) Hint() string {
	if c.OK {
		return ""
	}
	target := c.Target
	if target == "" {
		target = "~/.local/bin/" + project.InstalledCommandName
	}
	if c.Found == "" {
		return fmt.Sprintf("Programm nicht aktualisierbar: %s ist im PATH nicht zu finden, installiert wird aber nach %s. "+
			"Ein neues Programm dort startete der nächste Aufruf nicht. %s in den PATH aufnehmen, dann im Terminal: %s.",
			project.InstalledCommandName, target, filepath.Dir(target), project.BootstrapHint)
	}
	return fmt.Sprintf("Programm nicht aktualisierbar: %s im PATH ist %s, installiert wird aber nach %s. "+
		"Ein neues Programm dort startete der nächste Aufruf nicht. Die andere Datei entfernen oder %s im PATH nach vorn stellen, dann im Terminal: %s.",
		project.InstalledCommandName, c.Found, target, filepath.Dir(target), project.BootstrapHint)
}
