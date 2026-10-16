package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// briefingProject legt ein Projekt mit beiden Ebenen in allen Sorten an und
// macht es zum Arbeitsverzeichnis.
func briefingProject(t *testing.T) string {
	t.Helper()

	root := readProject(t)
	files := map[string]string{
		"k-playbook/rules/befunde.md":              "# Regel: Befunde\n\nText.\n",
		"k-playbook/reviews/review-code.md":        "---\nname: review-code\ntitle: Code-Review\n---\n# Code\n",
		"k-playbook/guidelines/betrieb.md":         "# Betrieb\n\nText.\n",
		"k-playbook-local/rules/befunde.md":        "# Abgeschaltet.\n",
		"k-playbook-local/guidelines/stil.md":      "# Stil\n\nText.\n",
		"k-playbook-local/reviews/review-eigen.md": "# Eigenes Review\n\nText.\n",
	}
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("%s schreiben: %v", rel, err)
		}
	}
	return root
}

// runBriefingCaptured führt das Subkommando aus und fängt stdout ab.
func runBriefingCaptured(t *testing.T, args ...string) (string, error) {
	t.Helper()

	file, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatalf("Ausgabedatei: %v", err)
	}
	before := os.Stdout
	os.Stdout = file
	runErr := runBriefing(args)
	os.Stdout = before
	file.Close()

	stdout, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("Ausgabe lesen: %v", err)
	}
	return string(stdout), runErr
}

func TestBriefingKlartext(t *testing.T) {
	root := briefingProject(t)

	stdout, err := runBriefingCaptured(t)
	if err != nil {
		t.Fatalf("briefing: %v", err)
	}
	for _, want := range []string{
		"Immer (k-playbook:/instructions/always/):",
		"  k-playbook:/instructions/always/010-a.md  [dist]  A\n",
		"  k-playbook:/instructions/always/100-b.md  [override]  B lokal\n",
		"  k-playbook:/instructions/always/200-c.md  [disabled]  200-c.md\n",
		"  Hinweis: k-playbook:/instructions/always/notiz.md (lokal)",
		"Auf Abruf:\n  k-playbook:/instructions/on-demand/aus.md  [disabled]  aus.md\n  k-playbook:/instructions/on-demand/thema.md  [dist]  Thema — D\n",
		"Regeln:\n  k-playbook:/rules/befunde.md  [disabled]  befunde.md\n",
		"Reviews:\n  k-playbook:/reviews/review-code.md  [dist]  Code-Review\n  k-playbook:/reviews/review-eigen.md  [local]  Eigenes Review\n",
		"Leitlinien:\n  k-playbook:/guidelines/betrieb.md  [dist]  Betrieb\n  k-playbook:/guidelines/stil.md  [local]  Stil\n",
		"Eigene Arbeit: Kephalaion nicht nutzbar — ",
		"Projektdoku: project:/k-playbook-local/docs/README.md\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("Ausgabe enthält nicht %q:\n%s", want, stdout)
		}
	}
	// Keine Pfade der Platte: das Briefing nennt Verweise.
	if strings.Contains(stdout, root) {
		t.Errorf("Ausgabe nennt den Projektpfad:\n%s", stdout)
	}
}

func TestBriefingJSON(t *testing.T) {
	briefingProject(t)

	stdout, err := runBriefingCaptured(t, "--json")
	if err != nil {
		t.Fatalf("briefing --json: %v", err)
	}
	var briefing project.Briefing
	if err := json.Unmarshal([]byte(stdout), &briefing); err != nil {
		t.Fatalf("Ausgabe ist kein JSON: %v\n%s", err, stdout)
	}
	if briefing.SchemaVersion != project.BriefingSchemaVersion {
		t.Errorf("schemaVersion %q", briefing.SchemaVersion)
	}
	if len(briefing.Documents.Always.Parts) != 3 || len(briefing.Documents.Rules) != 1 || len(briefing.Documents.Reviews) != 2 || len(briefing.Documents.Guidelines) != 2 {
		t.Errorf("Listen: %+v", briefing.Documents)
	}
	if briefing.Work.Available || briefing.Work.Message == "" {
		t.Errorf("work: %+v", briefing.Work)
	}
	// Die JSON-Ausgabe trägt die Listen auch leer als Listen, nie als null.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw["work"]), "null") {
		t.Errorf("work trägt null: %s", raw["work"])
	}
}

func TestBriefingOptionen(t *testing.T) {
	briefingProject(t)

	if _, err := runBriefingCaptured(t, "--nein"); err == nil || !strings.Contains(err.Error(), "unbekannte Option") {
		t.Errorf("unbekannte Option: %v", err)
	}
	stdout, err := runBriefingCaptured(t, "--help")
	if err != nil || !strings.Contains(stdout, "k-playbook briefing [--json]") {
		t.Errorf("Hilfe: %v\n%s", err, stdout)
	}
}
