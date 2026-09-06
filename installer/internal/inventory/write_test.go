package inventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newRunProject baut ein kleines Projekt mit einer Quelle und liefert die
// Optionen eines Laufs.
func newRunProject(t *testing.T) Options {
	t.Helper()

	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if err := os.WriteFile(filepath.Join(root, "requirements.txt"), []byte("redis==5.0.1\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	return Options{
		ProjectDir:    root,
		SourcesFile:   filepath.Join(root, "k-playbook-local", "version-sources.yaml"),
		InventoryFile: filepath.Join(root, "k-playbook-local", "docs", "versions", "inventory.md"),
	}
}

// Die Byte-Stabilitätsregel: ein zweiter Lauf ohne Quelländerung fasst die
// Datei nicht an — Zeitstempel und Änderungszeit eingeschlossen.
func TestZweiterLaufOhneQuellaenderungFasstDieDateiNichtAn(t *testing.T) {
	options := newRunProject(t)
	options.Now = func() time.Time { return time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC) }

	_, first, err := Run(options)
	if err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	if !first.Written {
		t.Fatal("der erste Lauf muss schreiben")
	}
	before, err := os.ReadFile(options.InventoryFile)
	if err != nil {
		t.Fatalf("lesen: %v", err)
	}
	info, _ := os.Stat(options.InventoryFile)

	// Der zweite Lauf hat eine andere Uhr: stünde der Zeitstempel des Laufs in
	// der Datei, änderte sie sich jedes Mal.
	options.Now = func() time.Time { return time.Date(2026, 12, 24, 18, 0, 0, 0, time.UTC) }
	_, second, err := Run(options)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if second.Written {
		t.Error("ohne inhaltliche Änderung darf gar nicht geschrieben werden")
	}
	after, err := os.ReadFile(options.InventoryFile)
	if err != nil {
		t.Fatalf("lesen: %v", err)
	}
	if string(after) != string(before) {
		t.Error("die Datei muss byte-identisch bleiben")
	}
	if later, _ := os.Stat(options.InventoryFile); !later.ModTime().Equal(info.ModTime()) {
		t.Error("auch die Änderungszeit im Dateisystem darf sich nicht bewegen")
	}
	if second.At != first.At {
		t.Errorf("der Zeitstempel bleibt der der letzten inhaltlichen Änderung: %q gegen %q", second.At, first.At)
	}
}

// Eine geänderte Quelle führt zu einer aktualisierten Inventur, einem
// fortgeschriebenen Zeitstempel und — weil eine zweite Quelle desselben
// Projekts weiter das Alte sagt — zu einer sichtbaren Abweichung.
func TestGeaenderteQuelleSchreibtNeuMitNeuemZeitstempelUndZeigtDieAbweichung(t *testing.T) {
	options := newRunProject(t)
	// Dieselbe Aussage aus einer zweiten Quelle: solange beide dasselbe sagen,
	// ist das keine Abweichung.
	if err := os.WriteFile(filepath.Join(options.ProjectDir, "pyproject.toml"),
		[]byte("[project]\nname = \"beispiel\"\ndependencies = [\n  \"redis==5.0.1\",\n]\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	options.Now = func() time.Time { return time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC) }
	first, firstOutcome, err := Run(options)
	if err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	if len(first.Deviations) != 0 {
		t.Fatalf("gleiche version und gleicher pin ergeben keine Abweichung: %+v", first.Deviations)
	}

	if err := os.WriteFile(filepath.Join(options.ProjectDir, "requirements.txt"),
		[]byte("redis==5.2.0\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	options.Now = func() time.Time { return time.Date(2026, 12, 24, 18, 0, 0, 0, time.UTC) }
	second, outcome, err := Run(options)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if !outcome.Written {
		t.Fatal("eine geänderte Quelle muss die Datei neu schreiben")
	}
	if !strings.HasPrefix(outcome.At, "2026-12-24") || outcome.At == firstOutcome.At {
		t.Errorf("der Zeitstempel muss fortgeschrieben werden: %q gegen %q", outcome.At, firstOutcome.At)
	}
	if len(second.Deviations) != 1 || second.Deviations[0].Group != "python/redis" {
		t.Fatalf("die Änderung muss eine Abweichung erzeugen: %+v", second.Deviations)
	}
	if second.Deviations[0].Art != DeviationConflicting {
		t.Errorf("beide Quellen sind lokal, das ist widersprüchlich: %q", second.Deviations[0].Art)
	}
	content, _ := os.ReadFile(options.InventoryFile)
	for _, needle := range []string{"5.2.0", "5.0.1", "deviations: 1",
		"### widersprüchlich — `python/redis`"} {
		if !strings.Contains(string(content), needle) {
			t.Errorf("in der Datei fehlt %q", needle)
		}
	}
}

// Auch die Ablehnungen sind Inhalt: ändert sich die Menge der abgelehnten
// Quellen, ändert sich die Datei.
func TestAblehnungenSindInhalt(t *testing.T) {
	options := newRunProject(t)
	options.Now = func() time.Time { return time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC) }
	if _, _, err := Run(options); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}

	writeSources(t, options.SourcesFile, `schema_version: 1
sources:
  - path: ../draussen/values.yaml
    kind: helm
    env: deployment
`)
	options.Now = func() time.Time { return time.Date(2026, 12, 24, 18, 0, 0, 0, time.UTC) }
	result, outcome, err := Run(options)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if !outcome.Written {
		t.Fatal("eine neue Ablehnung ändert die Datei")
	}
	if len(result.Rejections) != 1 {
		t.Fatalf("Ablehnungen = %+v", result.Rejections)
	}
	content, _ := os.ReadFile(options.InventoryFile)
	if !strings.Contains(string(content), "außerhalb der erlaubten Wurzeln") {
		t.Error("die Ablehnung muss in der Datei stehen, nicht nur im Ergebnis")
	}
	if !strings.Contains(string(content), "rejected: 1") {
		t.Error("das Frontmatter muss die Ablehnung zählen")
	}
}

// Ist die Datei da, ihr Frontmatter aber defekt, ist das ein sichtbarer Befund;
// der Lauf erhebt neu und schreibt, weil ein Vergleich nicht möglich ist.
func TestDefektesFrontmatterWirdGemeldetUndNeuGeschrieben(t *testing.T) {
	options := newRunProject(t)
	if err := os.MkdirAll(filepath.Dir(options.InventoryFile), 0o755); err != nil {
		t.Fatalf("anlegen: %v", err)
	}
	if err := os.WriteFile(options.InventoryFile, []byte("# Versionsinventar\n\nvon Hand\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}

	_, outcome, err := Run(options)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome.Problem == "" {
		t.Error("ein defektes Frontmatter ist ein sichtbarer Befund")
	}
	if !outcome.Written {
		t.Error("ohne Vergleichsmöglichkeit wird neu geschrieben")
	}
	if status := ReadStatus(options.InventoryFile); status.Problem != "" {
		t.Errorf("nach dem Lauf muss der Stand sauber sein: %q", status.Problem)
	}
}

func TestReadStatusMeldetFehlendeDateiAlsZustand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inventory.md")
	status := ReadStatus(path)
	if status.Present || status.Problem != "" {
		t.Errorf("eine fehlende Datei ist ein definierter Zustand, kein Fehler: %+v", status)
	}
	if status.Path != path {
		t.Errorf("Path = %q", status.Path)
	}
}

func TestReadStatusMeldetUnvollstaendigesFrontmatter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inventory.md")
	if err := os.WriteFile(path, []byte("---\ngenerated: { by: k-doc-inventory, at: x }\n---\n\n# Rumpf\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	status := ReadStatus(path)
	if !strings.Contains(status.Problem, "title") || !strings.Contains(status.Problem, "description") {
		t.Errorf("`generated.by` allein genügt nicht: %q", status.Problem)
	}
}
