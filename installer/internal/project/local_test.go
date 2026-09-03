package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckLocalMeldetFehlendeStruktur(t *testing.T) {
	root := t.TempDir()

	statuses := CheckLocal(root)
	if LocalOK(statuses) {
		t.Fatal("leeres Projekt gilt als vollständig")
	}
	for _, status := range statuses {
		if status.Present {
			t.Errorf("%s als vorhanden gemeldet", status.Path)
		}
	}
}

func TestCreateLocalLegtStrukturAn(t *testing.T) {
	root := t.TempDir()

	statuses, err := CreateLocal(root)
	if err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}
	if !LocalOK(statuses) {
		t.Fatalf("nach CreateLocal unvollständig: %+v", statuses)
	}

	local := LocalDir(root)
	for _, name := range []string{"rules", "reviews", "checks", "results", "guidelines", "tasks", "priv", "material"} {
		if !isDir(filepath.Join(local, name)) {
			t.Errorf("%s fehlt", name)
		}
	}
	if !isDir(filepath.Join(local, "tasks", "done")) {
		t.Error("tasks/done fehlt")
	}
	if !isDir(filepath.Join(local, "docs", "manual")) {
		t.Error("docs/manual fehlt")
	}
	if !fileExists(filepath.Join(local, "TODO.md")) {
		t.Error("TODO.md fehlt")
	}
}

// docs/code/, docs/libs/ und docs/extracted/ gehören je einem Erzeuger und
// entstehen beim ersten Lauf des jeweiligen Commands, nicht beim Einrichten.
func TestCreateLocalLegtErzeugteDocsVerzeichnisseNichtAn(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	for _, name := range []string{"code", "libs", "extracted"} {
		if pathExists(filepath.Join(LocalDir(root), "docs", name)) {
			t.Errorf("docs/%s wurde beim Einrichten angelegt, gehört aber seinem Erzeuger", name)
		}
	}
}

// Git speichert keine leeren Verzeichnisse; ohne README wären sie nach einem
// Clone des Projekts verschwunden.
func TestCreateLocalLegtReadmesAn(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	readme := filepath.Join(LocalDir(root), "rules", "README.md")
	content, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("README fehlt: %v", err)
	}
	if !strings.Contains(string(content), "Enforcement-Regeln") {
		t.Errorf("README ohne Zweckbeschreibung:\n%s", content)
	}
}

func TestCreateLocalLegtDocsIndexAn(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	path := filepath.Join(LocalDir(root), "docs", "README.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Docs-Index lesen: %v", err)
	}
	for _, want := range []string{"Projektwissen", "../../AGENTS.md"} {
		if !strings.Contains(string(content), want) {
			t.Errorf("Docs-Index enthält %q nicht:\n%s", want, content)
		}
	}
}

func TestCreateLocalUeberschreibtNichts(t *testing.T) {
	root := t.TempDir()
	local := LocalDir(root)

	// Ein Projekt, das schon Inhalte hat.
	if err := os.MkdirAll(filepath.Join(local, "rules"), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	eigen := filepath.Join(local, "rules", "README.md")
	if err := os.WriteFile(eigen, []byte("# eigene Beschreibung\n"), 0o644); err != nil {
		t.Fatalf("README anlegen: %v", err)
	}
	todo := filepath.Join(local, "TODO.md")
	if err := os.WriteFile(todo, []byte("- offener Punkt\n"), 0o644); err != nil {
		t.Fatalf("TODO.md anlegen: %v", err)
	}

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	for path, want := range map[string]string{
		eigen: "# eigene Beschreibung\n",
		todo:  "- offener Punkt\n",
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s lesen: %v", path, err)
		}
		if string(content) != want {
			t.Errorf("%s wurde verändert: %q", path, content)
		}
	}
}

func TestCreateLocalIstIdempotent(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	statuses, err := CreateLocal(root)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if !LocalOK(statuses) {
		t.Errorf("zweiter Lauf unvollständig: %+v", statuses)
	}
}

// Die drei Overlay-Sorten müssen ein lokales Gegenstück haben, sonst greift
// die Overlay-Auflösung ins Leere.
func TestLocalStructureDecktOverlaySortenAb(t *testing.T) {
	vorhanden := map[string]bool{}
	for _, entry := range LocalStructure() {
		vorhanden[entry.Path] = true
	}

	for _, kind := range []string{"rules", "reviews", "checks"} {
		if !vorhanden[kind] {
			t.Errorf("Overlay-Sorte %s fehlt in der lokalen Struktur", kind)
		}
	}
}

// priv/ und material/ werden angelegt wie jedes andere Verzeichnis. Dass ihr
// Inhalt oft privat bleiben soll, steht in ihrer README — entschieden wird es
// vom Projekt, nicht von k-playbook.
func TestCreateLocalLegtPrivateVerzeichnisseAn(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	for _, name := range []string{"priv", "material"} {
		dir := filepath.Join(LocalDir(root), name)
		if !isDir(dir) {
			t.Errorf("%s/ fehlt", name)
			continue
		}
		readme := filepath.Join(dir, "README.md")
		if !fileExists(readme) {
			t.Errorf("%s/README.md fehlt", name)
			continue
		}
		content, err := os.ReadFile(readme)
		if err != nil {
			t.Fatalf("%s/README.md lesen: %v", name, err)
		}
		if !strings.Contains(string(content), ".gitignore") {
			t.Errorf("%s/README.md erklärt den .gitignore-Weg nicht:\n%s", name, content)
		}
	}
}

// CreateLocal schreibt eine .gitignore nur für Einträge mit PrivateByDefault.
// Für alle anderen gilt weiterhin: was versioniert wird, entscheidet allein das
// Projekt.
func TestCreateLocalSchreibtGitignoreNurFuerVorbelegteEintraege(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	for _, entry := range LocalStructure() {
		if entry.IsFile || entry.PrivateByDefault {
			continue
		}
		if pathExists(filepath.Join(LocalDir(root), entry.Path, PrivateIgnoreFile)) {
			t.Errorf("%s hat eine .gitignore, die CreateLocal nicht schreiben darf", entry.Path)
		}
	}
}

// Ein frisch angelegter vorbelegter Eintrag bekommt genau den verwalteten
// Inhalt — nicht mehr und nicht weniger.
func TestCreateLocalLegtVorbelegteGitignoreAn(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	vorbelegt := 0
	for _, entry := range LocalStructure() {
		if !entry.PrivateByDefault {
			continue
		}
		vorbelegt++
		if !entry.Private {
			t.Errorf("%s ist vorbelegt, aber nicht als privat geführt", entry.Path)
		}
		ignore := filepath.Join(LocalDir(root), entry.Path, PrivateIgnoreFile)
		if !hasManagedContent(ignore) {
			content, _ := os.ReadFile(ignore)
			t.Errorf("%s trägt nicht den verwalteten Inhalt:\n%s", entry.Path, content)
		}
	}
	if vorbelegt == 0 {
		t.Fatalf("kein vorbelegter Eintrag in der Struktur")
	}
}

// Zweiter Lauf über ein bestehendes Verzeichnis ohne .gitignore: nichts wird
// geschrieben. Sonst käme der Default nach jedem makePublic() still zurück, und
// Bestandsprojekte mit getrackten Dateien landeten in PrivacyPartial.
func TestCreateLocalBringtEntfernteGitignoreNichtZurueck(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}
	for _, entry := range LocalStructure() {
		if !entry.PrivateByDefault {
			continue
		}
		if err := os.Remove(filepath.Join(LocalDir(root), entry.Path, PrivateIgnoreFile)); err != nil {
			t.Fatalf("%s/.gitignore entfernen: %v", entry.Path, err)
		}
	}

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal, zweiter Lauf: %v", err)
	}

	for _, entry := range LocalStructure() {
		if !entry.PrivateByDefault {
			continue
		}
		if pathExists(filepath.Join(LocalDir(root), entry.Path, PrivateIgnoreFile)) {
			t.Errorf("%s hat die .gitignore still zurückbekommen", entry.Path)
		}
	}
}
