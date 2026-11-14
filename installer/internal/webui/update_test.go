package webui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/sharedir/sharetest"
)

// newProject baut ein Zielprojekt mit Installation darin auf.
func newProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	// Der Clone steht noch, solange er Regeln und Skripte trägt; die
	// Commands und Skills kommen schon aus dem mitgelieferten Verzeichnis.
	writeFile(t, filepath.Join(root, project.PlaybookDirName, "VERSION"), "v0.0.0\n")
	writeFile(t, filepath.Join(sharetest.Dir(t), "commands", "k-test.md"), "test\n")
	writeFile(t, filepath.Join(sharetest.Dir(t), "skills", "beispiel", "SKILL.md"), "# Beispiel\n")
	return root
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("%s schreiben: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s lesen: %v", path, err)
	}
	return string(data)
}

// Scheitert die Programmaktualisierung, nennt die Meldung den Weg im
// Terminal: `k-playbook upgrade`. Geprüft wird der ganze Satz, nicht nur ein
// Wortbestandteil: die Meldung ist die Stelle, an der ein Nutzer erfährt, wie
// er das Programm von Hand nachholt.
//
// Ausdrücklich mitgeprüft wird, was **nicht** mehr dastehen darf: der
// Bootstrap des Clones. Das Programm kommt aus dem Release, nicht aus dem
// Clone.
func TestInstallationsfehlerNenntUpgrade(t *testing.T) {
	message := installFailureMessage(errors.New("kein Netz"))

	for _, want := range []string{
		"kein Netz",
		"läuft mit dem bisherigen Programm weiter",
		"Nachholen im Terminal: " + project.UpgradeCommand + ", danach k-playbook aufrufen.",
	} {
		if !strings.Contains(message, want) {
			t.Errorf("in der Meldung fehlt %q: %s", want, message)
		}
	}
	for _, gone := range []string{"make", project.PlaybookDirName + "/bin/install"} {
		if strings.Contains(message, gone) {
			t.Errorf("die Meldung nennt noch %q: %s", gone, message)
		}
	}
}

// Die Oberfläche zeigt denselben Weg wie die Antwort des Servers. Beide
// Texte stehen an verschiedenen Stellen — Go-Meldung und service.js, wo der
// Update-Knopf der Statusseite und von /setup bedient wird —, und genau
// deshalb wird der Gleichlauf geprüft statt vorausgesetzt. Dazu liest die
// Seite das Feld latest der Antwort, nicht mehr clone.
func TestOberflaecheNenntDenselbenWeg(t *testing.T) {
	source, err := staticFiles.ReadFile("static/service.js")
	if err != nil {
		t.Fatalf("service.js lesen: %v", err)
	}
	script := string(source)

	for _, want := range []string{
		"Nachholen im Terminal: " + project.UpgradeCommand + ".",
		project.UpgradeCommand + " und danach k-playbook aufrufen.",
		"program.latest",
		"älter als das neueste Release",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("in service.js fehlt %q", want)
		}
	}
	for _, gone := range []string{
		"make -C " + project.PlaybookDirName + " install",
		project.PlaybookDirName + "/bin/install",
		"program.clone",
		"älter als die Installation",
		"zum Clone passende Programm",
		// Task 156: kein Pull mehr, kein Modus dafür und kein Stand des Clones.
		`"pull"`,
		"Update verfügbar",
		"data.available",
		"data.branch",
		"Clone",
	} {
		if strings.Contains(script, gone) {
			t.Errorf("service.js nennt noch %q", gone)
		}
	}
}
