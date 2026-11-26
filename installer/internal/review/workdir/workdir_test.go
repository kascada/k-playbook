package workdir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/project"
)

func withTempRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	before := TempRoot
	TempRoot = root
	t.Cleanup(func() { TempRoot = before })
	return root
}

var testNow = time.Date(2026, 11, 26, 14, 5, 9, 0, time.UTC)

// Das Standardverzeichnis entsteht unmittelbar unter dem temporären
// Verzeichnis, mit Projekt, Rezept, Zeitstempel und zufälligem Teil im Namen
// und den Rechten 0700; zwei Aufrufe in derselben Sekunde bekommen zwei
// Verzeichnisse.
func TestPrepareStandardverzeichnis(t *testing.T) {
	temp := withTempRoot(t)
	projectDir := filepath.Join(t.TempDir(), "Mein Projekt")

	first, err := Prepare(projectDir, "code", "", testNow)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if filepath.Dir(first.Dir) != temp {
		t.Errorf("%s liegt nicht unmittelbar unter %s", first.Dir, temp)
	}
	if name := filepath.Base(first.Dir); !strings.HasPrefix(name, "k-review-mein-projekt-code-20261126-140509-") || len(name) <= len("k-review-mein-projekt-code-20261126-140509-") {
		t.Errorf("Name = %s", name)
	}
	if !first.Created || !first.Default || first.InsideProject || len(first.Warnings) != 0 {
		t.Errorf("Ergebnis = %+v", first)
	}
	info, err := os.Stat(first.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("Rechte %04o, erwartet 0700", info.Mode().Perm())
	}
	second, err := Prepare(projectDir, "code", "", testNow)
	if err != nil {
		t.Fatal(err)
	}
	if second.Dir == first.Dir {
		t.Error("zwei Aufrufe bekamen dasselbe Verzeichnis")
	}
}

// Ein übergebener Pfad, den es nicht gibt, entsteht mit 0700; liegt er im
// Projekt, kommt die Warnung.
// Ohne Umsetzen durch einen Test entsteht das Standardverzeichnis unmittelbar
// unter /tmp, auch wenn $TMPDIR auf ein Unterverzeichnis zeigt: Dazwischen
// darf keine Zwischenstufe liegen, die niemand prüft.
func TestPrepareStandardIgnoriertTMPDIR(t *testing.T) {
	if TempRoot != "/tmp" {
		t.Fatalf("TempRoot = %q, erwartet /tmp", TempRoot)
	}
	t.Setenv("TMPDIR", t.TempDir())
	prepared, err := Prepare(filepath.Join(t.TempDir(), "projekt"), "code", "", testNow)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(prepared.Dir) })
	if filepath.Dir(prepared.Dir) != "/tmp" {
		t.Errorf("%s liegt nicht unmittelbar unter /tmp", prepared.Dir)
	}
}

func TestPrepareUebergebenerPfad(t *testing.T) {
	projectDir := t.TempDir()
	target := filepath.Join(projectDir, "tmp", "review")

	prepared, err := Prepare(projectDir, "code", target, testNow)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if prepared.Dir != target || !prepared.Created || prepared.Default || !prepared.InsideProject || len(prepared.Warnings) != 1 ||
		!strings.Contains(prepared.Warnings[0], "im Projekt") {
		t.Errorf("Ergebnis = %+v", prepared)
	}
	if info, err := os.Stat(target); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("Verzeichnis: %v, %v", info, err)
	}
}

// Ein vorhandenes, für andere offenes Verzeichnis bleibt, wie es ist, und
// bekommt eine Warnung; ein Symlink oder eine Datei wird abgewiesen.
func TestPrepareVorhandenesVerzeichnis(t *testing.T) {
	projectDir := t.TempDir()
	open := filepath.Join(t.TempDir(), "offen")
	if err := os.Mkdir(open, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(open, 0o755); err != nil {
		t.Fatal(err)
	}
	prepared, err := Prepare(projectDir, "tech", open, testNow)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if prepared.Created || prepared.InsideProject || len(prepared.Warnings) != 1 || !strings.Contains(prepared.Warnings[0], "0755") {
		t.Errorf("Ergebnis = %+v", prepared)
	}
	if info, _ := os.Stat(open); info.Mode().Perm() != 0o755 {
		t.Errorf("Rechte geändert: %04o", info.Mode().Perm())
	}

	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(open, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(projectDir, "tech", link, testNow); err == nil || !strings.Contains(err.Error(), "Symlink") {
		t.Errorf("Symlink: %v", err)
	}
	file := filepath.Join(t.TempDir(), "datei")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(projectDir, "tech", file, testNow); err == nil || !strings.Contains(err.Error(), "kein Verzeichnis") {
		t.Errorf("Datei: %v", err)
	}
}

func TestSlug(t *testing.T) {
	for input, want := range map[string]string{"Mein Projekt": "mein-projekt", "../x": "x", "": "fallback", "ÄÖÜ": "fallback"} {
		if got := slug(input, "fallback"); got != want {
			t.Errorf("slug(%q) = %q, erwartet %q", input, got, want)
		}
	}
}

// Der Vertrag einer Quelle kommt aus dem Katalog: nur ein Evidence-Rezept mit
// gültigem audit-Block erhebt eine Fundliste.
func TestContractFor(t *testing.T) {
	evidence := project.CatalogEntry{Name: "review-code.md", Key: "code", Audit: &project.CatalogMode{
		Enabled: true, Mode: "evidence", RuleIDs: []string{"code-logic-error"}, Scope: &project.CatalogScope{Paths: []string{"**/*.go"}},
	}}
	perspective := project.CatalogEntry{Name: "review-sast.md", Key: "sast", Audit: &project.CatalogMode{
		Enabled: true, Scope: &project.CatalogScope{Tools: []string{"gosec"}},
	}}
	broken := project.CatalogEntry{Name: "review-tech.md", Key: "tech", Audit: &project.CatalogMode{Enabled: true, Mode: "evidence", RuleIDs: []string{"x"}}}
	disabled := project.CatalogEntry{Name: "review-off.md", Key: "off", Disabled: true}
	entries := []project.CatalogEntry{evidence, perspective, broken, disabled}

	contract, err := contractFor(entries, "code")
	if err != nil || len(contract.RuleIDs) != 1 || contract.Scope == nil || contract.Scope.Paths[0] != "**/*.go" {
		t.Errorf("code: %+v, %v", contract, err)
	}
	for key, want := range map[string]string{"sast": "erhebt keine Fundliste", "tech": "Evidence-Vertrag", "off": "abgeschaltet", "fehlt": "kein Rezept"} {
		if _, err := contractFor(entries, key); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, erwartet %q", key, err, want)
		}
	}
}
