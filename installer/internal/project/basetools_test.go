package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const baseMatrixHeader = "name\tgroup\trole\tguarded\tmethods\tapt_package\tgithub_repo\tasset_ref\tasset_pattern\n"

// newInstallationWithBaseMatrix legt ein Projekt an, dessen Installation eine
// Matrix der Basis-Werkzeuge enthält.
func newInstallationWithBaseMatrix(t *testing.T, body string) string {
	t.Helper()

	root := t.TempDir()
	scriptDir := filepath.Join(PlaybookDir(root), "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	matrix := filepath.Join(scriptDir, "base-tools.tsv")
	if err := os.WriteFile(matrix, []byte(baseMatrixHeader+body), 0o644); err != nil {
		t.Fatalf("Matrix anlegen: %v", err)
	}
	return root
}

// fakeBinDir baut ein PATH-Verzeichnis mit den genannten Programmen. Jedes
// hinterlässt beim Aufruf eine Spur — so lässt sich belegen, dass der Befund
// keines von ihnen startet.
func fakeBinDir(t *testing.T, marker string, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		body := "#!/bin/sh\necho gestartet >> " + marker + "\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatalf("Programm %s anlegen: %v", name, err)
		}
	}
	return dir
}

func TestDetectBaseToolsMeldetFehlende(t *testing.T) {
	root := newInstallationWithBaseMatrix(t,
		"k-da\t-\tIst da\tja\tapt\tpaket-da\t-\t-\t-\n"+
			"k-weg\t-\tFehlt\tnein\tapt,github\tpaket-weg\tbeispiel/weg\tweg\t^{tool}$\n")

	marker := filepath.Join(t.TempDir(), "aufrufe")
	t.Setenv("PATH", fakeBinDir(t, marker, "k-da"))

	state := DetectBaseTools(root)
	if !state.Present {
		t.Fatalf("Matrix wurde nicht gelesen: %s", state.Error)
	}
	if state.OK {
		t.Error("OK ist true, obwohl ein Werkzeug fehlt")
	}
	if len(state.Missing) != 1 || state.Missing[0].Name != "k-weg" {
		t.Fatalf("Missing = %+v, erwartet genau k-weg", state.Missing)
	}
	if state.Missing[0].Role != "Fehlt" {
		t.Errorf("Role = %q, erwartet die Rolle aus der Matrix", state.Missing[0].Role)
	}
	// Die Methodenspalte wird unverändert durchgereicht. Die Rangfolge, die das
	// Skript daraus ableitet, wird in Go nicht nachgebaut.
	if state.Missing[0].Methods != "apt,github" {
		t.Errorf("Methods = %q, erwartet apt,github", state.Missing[0].Methods)
	}
	if !strings.HasSuffix(state.InstallCommand, "install-base-tools.sh\" --install") {
		t.Errorf("InstallCommand = %q, erwartet einen einzelnen Skriptaufruf", state.InstallCommand)
	}
}

// TestDetectBaseToolsStartetKeinenUnterprozess ist die Kostenzusage des
// Befunds: gemessen wird Anwesenheit im PATH, nicht eine Version.
//
// Belegt wird das am Verhalten und nicht an einer Laufzeit: jedes Programm im
// PATH schreibt beim Aufruf in eine Datei. Ein `--version` je Werkzeug — der
// Weg des Security-Preflights — hinterließe dort Spuren; dieser Befund
// hinterlässt keine.
func TestDetectBaseToolsStartetKeinenUnterprozess(t *testing.T) {
	root := newInstallationWithBaseMatrix(t,
		"k-eins\t-\tErstes\tja\tapt\tpaket-eins\t-\t-\t-\n"+
			"k-zwei\t-\tZweites\tja\tapt\tpaket-zwei\t-\t-\t-\n")

	marker := filepath.Join(t.TempDir(), "aufrufe")
	t.Setenv("PATH", fakeBinDir(t, marker, "k-eins", "k-zwei"))

	state := DetectBaseTools(root)
	if !state.OK {
		t.Fatalf("beide Werkzeuge liegen im PATH, gemeldet fehlt: %+v", state.Missing)
	}
	if _, err := os.Stat(marker); err == nil {
		data, _ := os.ReadFile(marker)
		t.Fatalf("der Befund hat Werkzeuge gestartet:\n%s", data)
	} else if !os.IsNotExist(err) {
		t.Fatalf("Marker prüfen: %v", err)
	}
}

// TestDetectBaseToolsGruppeGenuegtEines: von curl und wget genügt eines. Ohne
// diese Regel widersprächen sich Skript und Kontextbefund auf demselben Host.
func TestDetectBaseToolsGruppeGenuegtEines(t *testing.T) {
	root := newInstallationWithBaseMatrix(t,
		"k-curl\tdownload\tDownload\tja\tapt\tcurl\t-\t-\t-\n"+
			"k-wget\tdownload\tRückfall\tja\tapt\twget\t-\t-\t-\n")

	marker := filepath.Join(t.TempDir(), "aufrufe")

	t.Run("eines vorhanden genügt", func(t *testing.T) {
		t.Setenv("PATH", fakeBinDir(t, marker, "k-curl"))
		state := DetectBaseTools(root)
		if !state.OK {
			t.Errorf("die Gruppe gilt als unvollständig, obwohl ein Mitglied da ist: %+v", state.Missing)
		}
	})

	t.Run("keines vorhanden meldet die Gruppe einmal", func(t *testing.T) {
		t.Setenv("PATH", fakeBinDir(t, marker))
		state := DetectBaseTools(root)
		if len(state.Missing) != 1 {
			t.Errorf("Missing = %+v, erwartet genau einen Eintrag für die Gruppe", state.Missing)
		}
	})
}

// TestDetectBaseToolsOhneMatrix: eine fehlende Matrix ist kein Abbruch. Der
// Kontext steht am Anfang jedes Commands und bleibt nutzbar.
func TestDetectBaseToolsOhneMatrix(t *testing.T) {
	root := t.TempDir()

	state := DetectBaseTools(root)
	if state.Present {
		t.Error("Present ist true, obwohl die Matrix fehlt")
	}
	if state.Error == "" {
		t.Error("der Zustand wird verschwiegen statt gemeldet")
	}
	if len(state.Missing) != 0 || state.OK {
		t.Errorf("ohne Matrix wird geraten: Missing=%+v OK=%v", state.Missing, state.OK)
	}
	if state.Matrix == "" {
		t.Error("der erwartete Ort der Matrix fehlt in der Antwort")
	}
}

// TestBuildContextTraegtBaseTools stellt sicher, dass der Befund neben gh in
// der Kontextausgabe landet.
func TestBuildContextTraegtBaseTools(t *testing.T) {
	root := newContextProject(t)

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if context.BaseTools.Matrix == "" {
		t.Error("baseTools fehlt in der Kontextausgabe")
	}
	if context.BaseTools.InstallCommand == "" {
		t.Error("baseTools nennt keinen Installationsbefehl")
	}
}
