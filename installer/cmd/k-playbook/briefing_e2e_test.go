package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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
	// Vorweg der Text der immer geltenden Anweisungen, wie `k-playbook read`
	// ihn liefert; die Liste seiner Teile mit Herkunft steht nur im JSON.
	// Davor der Kopf, der jede Kürzung übersteht: Kennung, Aufforderung,
	// die Titel der geltenden Teile — ohne den abgeschalteten.
	head := regexp.MustCompile(`^Sitzung: \d{8}-\d{6}-[0-9a-f]{6}\nLies diese Ausgabe ganz, nicht gekürzt: Alles darin gilt für die ganze Sitzung\.\nEs gelten: A · B lokal\n\n`)
	loc := head.FindStringIndex(stdout)
	if loc == nil {
		t.Fatalf("Ausgabe beginnt nicht mit dem Kopf:\n%s", stdout)
	}
	if !strings.HasPrefix(stdout[loc[1]:], "## A\n\n## B lokal\n\nText.\nHinweis: k-playbook:/instructions/always/notiz.md (lokal)") {
		t.Errorf("auf den Kopf folgen nicht die immer geltenden Anweisungen:\n%s", stdout)
	}
	for _, absent := range []string{"Immer (", "[dist]  A\n", "[override]  B lokal\n", "[disabled]  200-c.md\n"} {
		if strings.Contains(stdout, absent) {
			t.Errorf("Ausgabe enthält %q:\n%s", absent, stdout)
		}
	}
	for _, want := range []string{
		"Auf Abruf:\n  k-playbook:/instructions/on-demand/aus.md  [disabled]  aus.md\n  k-playbook:/instructions/on-demand/thema.md  [dist]  Thema — D\n",
		"Regeln:\n  k-playbook:/rules/befunde.md  [disabled]  befunde.md\n",
		"Reviews:\n  k-playbook:/reviews/review-code.md  [dist]  Code-Review\n  k-playbook:/reviews/review-eigen.md  [local]  Eigenes Review\n",
		"Leitlinien:\n  k-playbook:/guidelines/betrieb.md  [dist]  Betrieb\n  k-playbook:/guidelines/stil.md  [local]  Stil\n",
		"Eigene Arbeit: Kephalaion nicht nutzbar — ",
		"Projektdoku: project:/k-playbook-local/docs/README.md\n",
		"\n" + briefingContextSeparator + "\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("Ausgabe enthält nicht %q:\n%s", want, stdout)
		}
	}
	if strings.Count(stdout, "Sitzung: ") != 1 {
		t.Errorf("Sitzungskennung nicht genau einmal, im Kopf:\n%s", stdout)
	}
	briefingPart, contextPart, found := strings.Cut(stdout, briefingContextSeparator)
	if !found {
		t.Fatalf("keine Trennzeile:\n%s", stdout)
	}
	// Keine Pfade der Platte im Briefing: es nennt Verweise. Pfade stehen nur
	// im Rest von `k-playbook context` hinter der Trennzeile.
	if strings.Contains(briefingPart, root) {
		t.Errorf("Briefing nennt den Projektpfad:\n%s", briefingPart)
	}
	for _, want := range []string{
		"\n  project:\n    config: " + filepath.Join(root, "K-PLAYBOOK.yaml") + "\n    dir: " + root + "\n",
		"\n  now:\n    date: ",
		"\n  catalogs:\n    checks: (keine)\n",
		"\n  kephalaion:\n",
		"\n  remediation:\n    branchPrefix: ",
	} {
		if !strings.Contains(contextPart, want) {
			t.Errorf("Rest enthält nicht %q:\n%s", want, contextPart)
		}
	}
	// Die abgedeckten Felder fehlen im Rest.
	for _, absent := range []string{"\n  guidelines", "    rules:", "    reviews:"} {
		if strings.Contains(contextPart, absent) {
			t.Errorf("Rest enthält %q:\n%s", absent, contextPart)
		}
	}
}

// Ein Rest, der sich nicht bauen lässt, ist ein Zustand hinter der Trennzeile;
// das Briefing steht davor, und das Subkommando endet mit 0.
func TestBriefingKlartextOhneKontext(t *testing.T) {
	root := briefingProject(t)
	if err := os.WriteFile(filepath.Join(root, "K-PLAYBOOK.yaml"), []byte("schema_version: 99\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, err := runBriefingCaptured(t)
	if err != nil {
		t.Fatalf("briefing: %v", err)
	}
	if !strings.Contains(stdout, "Regeln:\n") || !strings.Contains(stdout, briefingContextSeparator+"\n  nicht verfügbar — K-PLAYBOOK.yaml hat schema_version 99") {
		t.Errorf("Ausgabe:\n%s", stdout)
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
	if !strings.HasPrefix(briefing.Documents.Always.Content, "## A\n\n## B lokal\n") {
		t.Errorf("always.content: %q", briefing.Documents.Always.Content)
	}
	if briefing.Work.Available || briefing.Work.Message == "" {
		t.Errorf("work: %+v", briefing.Work)
	}
	if _, ok := briefing.Context.Field("project"); !briefing.Context.Available || !ok {
		t.Errorf("context: %+v", briefing.Context)
	}
	if _, ok := briefing.Context.Field("guidelines"); ok {
		t.Errorf("context trägt guidelines: %+v", briefing.Context)
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

// Der Block der Umgebungsprüfung steht im Klartext unmittelbar nach dem Text
// der immer geltenden Anweisungen und vor „Auf Abruf“ — so liest die Sitzung
// ihn, bevor sie etwas tut. Im Testprojekt ist die Anbindung nicht
// eingetragen: eine Warnung; mit unlesbarer Quellenkonfiguration dazu ein
// Fehler, und das Subkommando endet trotzdem mit 0 — das Briefing meldet, es
// urteilt nicht.
func TestBriefingKlartextTraegtUmgebungsblock(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	root := briefingProject(t)

	stdout, err := runBriefingCaptured(t)
	if err != nil {
		t.Fatalf("briefing: %v", err)
	}
	block := "nicht aufgenommen\n\nUmgebung:\n  [warning] kephalaion: Keine Collection eingetragen"
	if !strings.Contains(stdout, block) {
		t.Errorf("Block fehlt oder steht falsch:\n%s", stdout)
	}
	umgebung, abruf := strings.Index(stdout, "\nUmgebung:\n"), strings.Index(stdout, "\nAuf Abruf:\n")
	if umgebung < 0 || abruf < 0 || umgebung > abruf {
		t.Errorf("Umgebung (%d) steht nicht vor Auf Abruf (%d):\n%s", umgebung, abruf, stdout)
	}

	path := filepath.Join(project.LocalDir(root), project.VersionSourcesFileName)
	if err := os.WriteFile(path, []byte(": ungültig\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, err = runBriefingCaptured(t)
	if err != nil {
		t.Fatalf("briefing mit Fehler: %v", err)
	}
	if !strings.Contains(stdout, "\n  [error] versionSources: Die Quellenkonfiguration des Versionsinventars "+path+" ist nicht lesbar") {
		t.Errorf("Fehler fehlt:\n%s", stdout)
	}

	stdout, err = runBriefingCaptured(t, "--json")
	if err != nil {
		t.Fatalf("briefing --json: %v", err)
	}
	var briefing project.Briefing
	if err := json.Unmarshal([]byte(stdout), &briefing); err != nil {
		t.Fatalf("kein JSON: %v", err)
	}
	if briefing.SchemaVersion != "5" || briefing.Health.OK || len(briefing.Health.Problems) != 2 {
		t.Errorf("health im JSON: %q %+v", briefing.SchemaVersion, briefing.Health)
	}
}
