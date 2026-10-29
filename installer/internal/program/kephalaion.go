package program

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// KephalaionResult ist der Ausgang von InstallKephalaion.
type KephalaionResult struct {
	// Path ist der Ort von kephalaion im PATH: vorhanden oder gerade
	// installiert.
	Path string `json:"path,omitempty"`
	// Installed: bin/install-kephalaion ist gelaufen.
	Installed bool `json:"installed,omitempty"`
	// Output ist die Ausgabe von bin/install-kephalaion, stdout und stderr
	// zusammen.
	Output string `json:"output,omitempty"`
}

// InstallKephalaion sorgt dafür, dass kephalaion im PATH liegt. Ist es da,
// geschieht nichts — auch keine Aktualisierung, die macht kephalaion upgrade.
// Sonst läuft bin/install-kephalaion des Clones aus dem Hauptverzeichnis, mit
// demselben Zeitlimit wie der Bootstrap.
//
// Erfolg heißt nicht Exit 0, sondern: danach liegt kephalaion im PATH. Das
// install.sh von Kephalaion installiert nach ~/.local/bin; fehlt das im PATH,
// ist das Binary zwar da, aber für k-playbook so gut wie nicht.
func InstallKephalaion(projectDir string) (KephalaionResult, error) {
	result := KephalaionResult{}
	if path, ok := kephalaion.FindBinary(); ok {
		result.Path = path
		return result, nil
	}

	script := filepath.Join(project.PlaybookDir(projectDir), "bin", "install-kephalaion")
	if _, err := os.Stat(script); err != nil {
		return result, fmt.Errorf("die Installation kennt %s noch nicht", project.KephalaionInstallCommand)
	}

	result.Installed = true
	output, _, err := runScript(projectDir, script, project.KephalaionInstallCommand)
	result.Output = output
	if err != nil {
		return result, err
	}
	path, ok := kephalaion.FindBinary()
	if !ok {
		return result, errors.New("kephalaion liegt nach der Installation nicht im PATH")
	}
	result.Path = path
	return result, nil
}
