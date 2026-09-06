package inventory

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// newBoundaryProject legt ein Projekt und ein Verzeichnis daneben an. Das
// Nachbarverzeichnis ist das „draußen", gegen das die Grenze geprüft wird.
func newBoundaryProject(t *testing.T) (projectRoot string, outside string) {
	t.Helper()

	base := t.TempDir()
	// Der Temp-Pfad ist auf macOS selbst ein Symlink; ohne Auflösen verglichen
	// der Test den falschen Pfad.
	if resolved, err := filepath.EvalSymlinks(base); err == nil {
		base = resolved
	}
	projectRoot = filepath.Join(base, "projekt")
	outside = filepath.Join(base, "draussen")
	for _, dir := range []string{projectRoot, outside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(outside, "geheim.txt"), []byte("nicht für uns\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	return projectRoot, outside
}

func TestCheckLehntAusbruchUeberPunktPunktAb(t *testing.T) {
	projectRoot, _ := newBoundaryProject(t)
	boundary, err := NewBoundary(projectRoot, nil)
	if err != nil {
		t.Fatalf("NewBoundary: %v", err)
	}

	_, err = boundary.Check("../draussen/geheim.txt")
	if err == nil {
		t.Fatal("ein Ausbruch über .. muss abgelehnt werden")
	}
	pathError, ok := err.(*PathError)
	if !ok {
		t.Fatalf("Fehlertyp = %T", err)
	}
	if pathError.Resolved == "" || !strings.Contains(pathError.Reason, "außerhalb") {
		t.Errorf("die Ablehnung muss aufgelösten Pfad und Grund nennen: %+v", pathError)
	}
	if !strings.Contains(pathError.Resolved, "draussen") {
		t.Errorf("gemeldet werden muss das aufgelöste Ziel: %q", pathError.Resolved)
	}
}

// Ein Symlink innerhalb des Projekts, der nach außen zeigt, ist ein Ausbruch:
// geprüft wird das aufgelöste Ergebnis, nicht der angefragte Pfad.
func TestCheckLehntSymlinkAusDemProjektHerausAb(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Symlinks brauchen unter Windows besondere Rechte")
	}
	projectRoot, outside := newBoundaryProject(t)
	link := filepath.Join(projectRoot, "extern.txt")
	if err := os.Symlink(filepath.Join(outside, "geheim.txt"), link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	boundary, err := NewBoundary(projectRoot, nil)
	if err != nil {
		t.Fatalf("NewBoundary: %v", err)
	}

	_, _, _, err = boundary.ReadFile("extern.txt")
	if err == nil {
		t.Fatal("ein Symlink nach draußen muss abgelehnt werden")
	}
	pathError := err.(*PathError)
	if !strings.Contains(pathError.Resolved, "geheim.txt") {
		t.Errorf("gemeldet werden muss das aufgelöste Ziel: %+v", pathError)
	}
}

// Auch ein Elternsegment kann der Symlink sein. Geprüft wird der vollständig
// aufgelöste Pfad, nicht nur sein letztes Segment.
func TestCheckLoestAuchElternsegmenteAuf(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Symlinks brauchen unter Windows besondere Rechte")
	}
	projectRoot, outside := newBoundaryProject(t)
	if err := os.Symlink(outside, filepath.Join(projectRoot, "extern")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	boundary, _ := NewBoundary(projectRoot, nil)

	if _, err := boundary.Check("extern/geheim.txt"); err == nil {
		t.Fatal("ein verlinktes Elternverzeichnis muss abgelehnt werden")
	}
}

func TestExpandLehntGlobAusbruchAb(t *testing.T) {
	projectRoot, outside := newBoundaryProject(t)
	if err := os.WriteFile(filepath.Join(outside, "values-prod.yaml"), []byte("image: a\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	boundary, _ := NewBoundary(projectRoot, nil)

	paths, rejections := boundary.Expand("../draussen/*.yaml")
	if len(paths) != 0 {
		t.Errorf("ein Glob darf kein Weg an der Prüfung vorbei sein: %v", paths)
	}
	if len(rejections) != 1 {
		t.Fatalf("Ablehnungen = %d, erwartet 1: %+v", len(rejections), rejections)
	}
	if !strings.Contains(rejections[0].Reason, "außerhalb") {
		t.Errorf("Grund = %q", rejections[0].Reason)
	}
}

// Eine ausdrücklich freigegebene Wurzel macht denselben Pfad lesbar — und nur
// sie: ein absoluter Pfad in sources: gibt seine Wurzel nicht selbst frei.
func TestFreigegebeneWurzelErlaubtLesenSegmentweise(t *testing.T) {
	projectRoot, outside := newBoundaryProject(t)
	boundary, err := NewBoundary(projectRoot, []string{outside})
	if err != nil {
		t.Fatalf("NewBoundary: %v", err)
	}

	data, _, exists, err := boundary.ReadFile(filepath.Join(outside, "geheim.txt"))
	if err != nil || !exists || !strings.Contains(string(data), "nicht für uns") {
		t.Fatalf("freigegebene Wurzel = %v / %v / %q", err, exists, data)
	}

	// /srv/deploy erlaubt /srv/deploy/x, aber nicht /srv/deploy-alt/x.
	sibling := outside + "-alt"
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatalf("anlegen: %v", err)
	}
	if _, err := boundary.Check(filepath.Join(sibling, "x.txt")); err == nil {
		t.Fatal("der Wurzelvergleich muss segmentweise laufen, nicht als Zeichenketten-Präfix")
	}
}

// `~` und `$VAR` werden nicht expandiert: ein Pfad, dessen Bedeutung von der
// Umgebung des Aufrufers abhängt, bedeutet im Webserver-Prozess etwas anderes
// als auf dem CLI-Weg.
func TestCheckExpandiertNichts(t *testing.T) {
	projectRoot, _ := newBoundaryProject(t)
	boundary, _ := NewBoundary(projectRoot, nil)

	resolved, err := boundary.Check("~/.ssh/id_rsa")
	if err != nil {
		t.Fatalf("der Pfad bleibt ein gewöhnlicher relativer Pfad: %v", err)
	}
	if !strings.HasPrefix(resolved, projectRoot) {
		t.Errorf("aufgelöst = %q, erwartet unterhalb von %q", resolved, projectRoot)
	}
	if _, err := boundary.Check("$HOME/geheim"); err != nil {
		t.Errorf("$VAR bleibt Bestandteil des Pfads: %v", err)
	}
}

func TestReadFileLiestNurRegulaereDateien(t *testing.T) {
	projectRoot, _ := newBoundaryProject(t)
	if err := os.MkdirAll(filepath.Join(projectRoot, "unterverzeichnis"), 0o755); err != nil {
		t.Fatalf("anlegen: %v", err)
	}
	boundary, _ := NewBoundary(projectRoot, nil)

	if _, _, _, err := boundary.ReadFile("unterverzeichnis"); err == nil {
		t.Fatal("ein Verzeichnis ist keine Quelle")
	}
	_, _, exists, err := boundary.ReadFile("gibtesnicht.txt")
	if err != nil {
		t.Fatalf("eine fehlende Datei ist keine Ablehnung: %v", err)
	}
	if exists {
		t.Error("exists = true für eine fehlende Datei")
	}
}

func TestNewBoundaryLehntRelativeWurzelAb(t *testing.T) {
	projectRoot, _ := newBoundaryProject(t)
	if _, err := NewBoundary(projectRoot, []string{"../draussen"}); err == nil {
		t.Fatal("eine relative Wurzel hinge vom Arbeitsverzeichnis ab und muss abgelehnt werden")
	}
}
