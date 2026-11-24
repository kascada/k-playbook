package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
	"github.com/kascada/k-playbook/installer/internal/githubaccess/accesstest"
	"github.com/kascada/k-playbook/installer/internal/health"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/sharedir/sharetest"
)

// briefingProject legt ein Projekt mit beiden Ebenen in allen Sorten an und
// macht es zum Arbeitsverzeichnis.
func briefingProject(t *testing.T) string {
	t.Helper()

	root := readProject(t)
	files := map[string]string{
		"test-defaults/reviews/review-code.md":               "---\nname: review-code\ntitle: Code-Review\n---\n# Code\n",
		"test-defaults/guidelines/betrieb.md":                "# Betrieb\n\nText.\n",
		"test-collection/k-playbook/reviews/review-eigen.md": "# Eigenes Review\n\nText.\n",
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
	head := regexp.MustCompile(`^Sitzung: \d{8}-\d{6}-[0-9a-f]{6}\n` +
		`now: date \d{4}-\d{2}-\d{2} · timestamp \S+\n` +
		`project: dir ` + regexp.QuoteMeta(root) + ` · repoRoot ` + regexp.QuoteMeta(root) + ` · vcs [a-z]+ · languages [a-z, ]+\n` +
		`kephalaion: ok false · message [^\n]+\n` +
		`(links\.[a-z]+: [^\n]+\n){0,3}` +
		`Lies diese Ausgabe ganz, nicht gekürzt: Alles darin gilt für die ganze Sitzung\.\nEs gelten: A · B lokal\n\n`)
	loc := head.FindStringIndex(stdout)
	if loc == nil {
		t.Fatalf("Ausgabe beginnt nicht mit dem Kopf:\n%s", stdout)
	}
	// Der Block der Abschnitte nach der Zeile Sitzung hat höchstens acht
	// Zeilen (Task 187).
	if block := strings.Count(stdout[:loc[1]], "\n") - 4; block > 8 {
		t.Errorf("Block nach Sitzung hat %d Zeilen, höchstens 8:\n%s", block, stdout[:loc[1]])
	}
	if !strings.HasPrefix(stdout[loc[1]:], "## A\n\n## B lokal\n\nText.\nHinweis: k-playbook:/instructions/always/notiz.md (lokal)") {
		t.Errorf("auf den Kopf folgen nicht die immer geltenden Anweisungen:\n%s", stdout)
	}
	for _, absent := range []string{"Immer (", "[dist]", "Thema — D", "[override]  B lokal\n", "[disabled]  200-c.md\n", "Regeln:\n"} {
		if strings.Contains(stdout, absent) {
			t.Errorf("Ausgabe enthält %q:\n%s", absent, stdout)
		}
	}
	for _, want := range []string{
		// Verweis und Titel; die Herkunft nur, wenn sie nicht dist ist, die
		// Beschreibung („D“) nur im JSON (Task 187).
		"Auf Abruf:\n  k-playbook:/instructions/on-demand/aus.md  [disabled keph:/k-playbook/instructions/on-demand/aus.md]  aus.md\n" +
			"  k-playbook:/instructions/on-demand/thema.md  Thema\n",
		"Reviews:\n  k-playbook:/reviews/review-code.md  Code-Review\n" +
			"  k-playbook:/reviews/review-eigen.md  [local keph:/k-playbook/reviews/review-eigen.md]  Eigenes Review\n",
		// Leitlinien sind nur mitgeliefert (Task 148): die nächste Zeile ist
		// schon der nächste Abschnitt.
		"Leitlinien:\n  k-playbook:/guidelines/betrieb.md  Betrieb\nEigene Arbeit: ",
		"Eigene Arbeit: Kephalaion nicht nutzbar — ",
		"Projektdoku: keph:/docs/\n",
		"Projektdoku: keph:/docs/\nGitHub: ",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("Ausgabe enthält nicht %q:\n%s", want, stdout)
		}
	}
	if strings.Count(stdout, "Sitzung: ") != 1 {
		t.Errorf("Sitzungskennung nicht genau einmal, im Kopf:\n%s", stdout)
	}
	// Der Klartext endet mit dem Hinweis auf den Teilabruf; den Rest von
	// `k-playbook context` trägt er nicht mehr (Task 187) — keine Trennzeile,
	// keines der gestrichenen Felder, kein Feld des Teilabrufs.
	if !strings.HasSuffix(stdout, briefingEnvironmentHint) {
		t.Errorf("Ausgabe endet nicht mit dem Hinweis auf den Teilabruf:\n%s", stdout)
	}
	for _, absent := range []string{"Noch aus k-playbook context", "wird abgebaut", "schemaVersion", "\n  remediation:", "\n  baseTools:",
		"\n  versionSources:", "\n  mcp:", "\n  git:", "\n  gh:", "tokenFromEnv", "installCommand", project.ConfigPath(root),
		"\n  guidelines", "    rules:", "\n  catalogs:", "checks", "\n  playbook:", "playbook.dir", "\n  local:"} {
		if strings.Contains(stdout, absent) {
			t.Errorf("Ausgabe enthält %q:\n%s", absent, stdout)
		}
	}
	// Der Projektpfad steht nur in der Zeile project des Kopfs.
	if got := strings.Count(strings.Replace(stdout, "repoRoot "+root, "", 1), root); got != 1 {
		t.Errorf("Projektpfad %d-mal außerhalb von repoRoot, erwartet einmal (project.dir):\n%s", got, stdout)
	}
}

// Eine Konfiguration, die sich nicht lesen lässt, ist ein Zustand an der
// Stelle des Abschnitts project — mit der Meldung, statt dass er still fehlt;
// das Briefing steht trotzdem da, und das Subkommando endet mit 0. Der
// Teilabruf environment meldet denselben Zustand.
func TestBriefingKlartextOhneKontext(t *testing.T) {
	root := briefingProject(t)
	mustWriteMCPFile(t, project.ConfigPath(root), "schema_version: 99\n")

	stdout, err := runBriefingCaptured(t)
	if err != nil {
		t.Fatalf("briefing: %v", err)
	}
	if !strings.Contains(stdout, "Reviews:\n") ||
		!strings.Contains(stdout, "\nproject: dir "+root+" · nicht verfügbar — K-PLAYBOOK.yaml hat schema_version 99") ||
		!strings.Contains(stdout, "\nnow: date ") || !strings.Contains(stdout, "\nkephalaion: ok false") {
		t.Errorf("Ausgabe:\n%s", stdout)
	}

	stdout, err = runBriefingCaptured(t, "--json")
	if err != nil {
		t.Fatalf("briefing --json: %v", err)
	}
	var briefing project.Briefing
	if err := json.Unmarshal([]byte(stdout), &briefing); err != nil {
		t.Fatalf("kein JSON: %v", err)
	}
	if briefing.Project.Available || !strings.Contains(briefing.Project.Message, "schema_version 99") || briefing.Project.Dir != root ||
		briefing.Project.RepoRoot != "" || briefing.Links != nil {
		t.Errorf("project: %+v, links %+v", briefing.Project, briefing.Links)
	}

	stdout, err = runBriefingCaptured(t, "--part", "environment")
	if err != nil {
		t.Fatalf("briefing --part environment: %v", err)
	}
	if !strings.HasPrefix(stdout, "environment: nicht verfügbar — K-PLAYBOOK.yaml hat schema_version 99") {
		t.Errorf("Teilabruf:\n%s", stdout)
	}
}

func TestBriefingJSON(t *testing.T) {
	root := briefingProject(t)

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
	if len(briefing.Documents.Always.Parts) != 3 || len(briefing.Documents.Reviews) != 2 || len(briefing.Documents.Guidelines) != 1 {
		t.Errorf("Listen: %+v", briefing.Documents)
	}
	if !strings.HasPrefix(briefing.Documents.Always.Content, "## A\n\n## B lokal\n") {
		t.Errorf("always.content: %q", briefing.Documents.Always.Content)
	}
	if briefing.Work.Available || briefing.Work.Message == "" {
		t.Errorf("work: %+v", briefing.Work)
	}
	if !briefing.Project.Available || briefing.Project.Dir != root || briefing.Project.RepoRoot != root || briefing.Project.VCS == "" ||
		len(briefing.Project.Languages) == 0 {
		t.Errorf("project: %+v", briefing.Project)
	}
	if briefing.Now.Date == "" || !strings.HasPrefix(briefing.Now.Timestamp, briefing.Now.Date+"T") {
		t.Errorf("now: %+v", briefing.Now)
	}
	if briefing.Kephalaion.OK || briefing.Kephalaion.Message == "" {
		t.Errorf("kephalaion: %+v", briefing.Kephalaion)
	}
	// Die JSON-Ausgabe trägt die Listen auch leer als Listen, nie als null.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw["work"]), "null") {
		t.Errorf("work trägt null: %s", raw["work"])
	}
	if strings.Contains(string(raw["github"]), "null") {
		t.Errorf("github trägt null: %s", raw["github"])
	}
	// Den Abschnitt context gibt es nicht mehr (Fassung 9), und was der
	// Teilabruf environment liefert, steht nicht im Briefing.
	for _, absent := range []string{"context", "environment", "remediation", "baseTools", "versionSources", "gh", "mcp", "git", "docs", "catalogs"} {
		if _, ok := raw[absent]; ok {
			t.Errorf("Briefing trägt %s: %s", absent, raw[absent])
		}
	}
	for _, absent := range []string{`"config"`, `"hub"`, `"collection"`, `"account"`, `"rights"`, `"node"`, `"sync"`, `"state"`, `"choice"`} {
		if strings.Contains(string(raw["project"])+string(raw["kephalaion"]), absent) {
			t.Errorf("project oder kephalaion trägt das gestrichene Feld %s:\n%s\n%s", absent, raw["project"], raw["kephalaion"])
		}
	}
}

// Der Teilabruf environment (Task 187): `--part environment` liefert nur die
// Umgebungsangaben — weder session noch documents noch work, keine
// Anweisungen —, als JSON unter environment, im Klartext eingerückt unter
// „environment:“. Die gestrichenen Felder fehlen: von gh bleiben ready und
// account, baseTools verliert matrix.
func TestBriefingTeilabrufEnvironment(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	accesstest.Empty(t)
	root := briefingProject(t)

	stdout, err := runBriefingCaptured(t, "--part", "environment", "--json")
	if err != nil {
		t.Fatalf("briefing --part environment --json: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("kein JSON: %v\n%s", err, stdout)
	}
	if len(raw) != 1 || raw["environment"] == nil {
		t.Fatalf("oberste Felder des Teilabrufs: %s", stdout)
	}
	for _, absent := range []string{"session", "documents", "work", "schemaVersion", "health", "now", "project", "kephalaion", "links"} {
		if _, ok := raw[absent]; ok {
			t.Errorf("Teilabruf trägt %s", absent)
		}
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw["environment"], &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"available", "gh", "github", "baseTools", "versionSources", "docs", "remediation"} {
		if _, ok := fields[name]; !ok {
			t.Errorf("environment ohne %s: %s", name, raw["environment"])
		}
	}
	for _, absent := range []string{"session", "documents", "work", "mcp", "git", "schemaVersion", "catalogs", "kephalaion", "project", "now", "links"} {
		if _, ok := fields[absent]; ok {
			t.Errorf("environment trägt %s", absent)
		}
	}
	var gh, baseTools map[string]json.RawMessage
	if err := json.Unmarshal(fields["gh"], &gh); err != nil {
		t.Fatal(err)
	}
	if len(gh) != 2 || gh["ready"] == nil || gh["account"] == nil {
		t.Errorf("environment.gh: %s", fields["gh"])
	}
	if err := json.Unmarshal(fields["baseTools"], &baseTools); err != nil {
		t.Fatal(err)
	}
	if _, ok := baseTools["matrix"]; ok || baseTools["installCommand"] == nil || baseTools["missing"] == nil {
		t.Errorf("environment.baseTools: %s", fields["baseTools"])
	}
	environment := environmentOf(t, stdout)
	if !environment.Available || environment.Remediation == nil || environment.Docs == nil || environment.VersionSources == nil ||
		environment.VersionSources.Ref != "keph:/k-playbook/inventory.md" {
		t.Errorf("environment: %+v", environment)
	}

	text, err := runBriefingCaptured(t, "--part=environment")
	if err != nil {
		t.Fatalf("briefing --part=environment: %v", err)
	}
	if !strings.HasPrefix(text, "environment:\n  gh:\n    account: ") {
		t.Errorf("Klartext des Teilabrufs beginnt anders:\n%s", text)
	}
	for _, want := range []string{"\n  github:\n", "\n  baseTools:\n", "\n    installCommand: ", "\n  versionSources:\n", "\n  docs:\n    exclude: ", "\n  remediation:\n    branchPrefix: "} {
		if !strings.Contains(text, want) {
			t.Errorf("Klartext des Teilabrufs enthält nicht %q:\n%s", want, text)
		}
	}
	for _, absent := range []string{"Sitzung: ", "Auf Abruf:", "Eigene Arbeit", "Lies diese Ausgabe", "\n  available"} {
		if strings.Contains(text, absent) {
			t.Errorf("Klartext des Teilabrufs enthält %q:\n%s", absent, text)
		}
	}

	// Der Teilabruf zieht keine Verlinkung nach: Er schreibt nichts ins
	// Projekt.
	for _, name := range []string{".claude", ".opencode", "CLAUDE.md", "AGENTS.md"} {
		if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Errorf("Teilabruf hat %s angelegt (%v)", name, err)
		}
	}
}

func TestBriefingOptionen(t *testing.T) {
	briefingProject(t)

	if _, err := runBriefingCaptured(t, "--nein"); err == nil || !strings.Contains(err.Error(), "unbekannte Option") {
		t.Errorf("unbekannte Option: %v", err)
	}
	// Ein unbekannter oder fehlender Teil ist ein Fehler, kein volles
	// Briefing.
	for _, args := range [][]string{{"--part", "work"}, {"--part="}, {"--part"}} {
		stdout, err := runBriefingCaptured(t, args...)
		if err == nil || !strings.Contains(err.Error(), "--part environment") || stdout != "" {
			t.Errorf("%v: %v\n%s", args, err, stdout)
		}
	}
	stdout, err := runBriefingCaptured(t, "--help")
	if err != nil || !strings.Contains(stdout, "k-playbook briefing [--json]") || !strings.Contains(stdout, "k-playbook briefing --part environment [--json]") {
		t.Errorf("Hilfe: %v\n%s", err, stdout)
	}
	// Die Hilfe beschreibt den Übergangsabschnitt nicht mehr.
	for _, absent := range []string{"k-playbook context", "Trennzeile", "context.github", "abgebaut"} {
		if strings.Contains(stdout, absent) {
			t.Errorf("Hilfe nennt %q:\n%s", absent, stdout)
		}
	}
}

// Der Block der Umgebungsprüfung steht im Klartext unmittelbar nach dem Text
// der immer geltenden Anweisungen und vor „Auf Abruf“ — so liest die Sitzung
// ihn, bevor sie etwas tut. Im Testprojekt ist die Anbindung nicht
// eingetragen: ein Fehler (die mitgelieferte Ebene kommt hier aus dem Store
// des Tests, defaultstest); mit einem fehlenden Basis-Werkzeug ein zweiter,
// und das Subkommando endet trotzdem mit 0 — das Briefing meldet, es urteilt
// nicht.
func TestBriefingKlartextTraegtUmgebungsblock(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	accesstest.Empty(t)
	briefingProject(t)

	stdout, err := runBriefingCaptured(t)
	if err != nil {
		t.Fatalf("briefing: %v", err)
	}
	block := "nicht aufgenommen\n\nUmgebung:\n  [error] kephalaion: Keine Collection eingetragen"
	if !strings.Contains(stdout, block) {
		t.Errorf("Block fehlt oder steht falsch:\n%s", stdout)
	}
	umgebung, abruf := strings.Index(stdout, "\nUmgebung:\n"), strings.Index(stdout, "\nAuf Abruf:\n")
	if umgebung < 0 || abruf < 0 || umgebung > abruf {
		t.Errorf("Umgebung (%d) steht nicht vor Auf Abruf (%d):\n%s", umgebung, abruf, stdout)
	}

	// Ein Basis-Werkzeug der Matrix fehlt im PATH: eine Warnung neben dem
	// Fehler.
	path := filepath.Join(sharetest.Dir(t), "scripts", "base-tools.tsv")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("name\tgroup\trole\tguarded\tmethods\tapt_package\tgithub_repo\tasset_ref\tasset_pattern\nk-weg\t-\tFehlt\tnein\tapt\tpaket\t-\t-\t-\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, err = runBriefingCaptured(t)
	if err != nil {
		t.Fatalf("briefing mit Warnung: %v", err)
	}
	if !strings.Contains(stdout, "\n  [warning] baseTools: Das Basis-Werkzeug k-weg fehlt im PATH") {
		t.Errorf("Warnung fehlt:\n%s", stdout)
	}

	stdout, err = runBriefingCaptured(t, "--json")
	if err != nil {
		t.Fatalf("briefing --json: %v", err)
	}
	var briefing project.Briefing
	if err := json.Unmarshal([]byte(stdout), &briefing); err != nil {
		t.Fatalf("kein JSON: %v", err)
	}
	if briefing.SchemaVersion != "9" || briefing.Health.OK || len(briefing.Health.Problems) != 2 {
		t.Errorf("health im JSON: %q %+v", briefing.SchemaVersion, briefing.Health)
	}
}

// Ohne mitgelieferte Ebene endet `k-playbook briefing` mit einem Fehler —
// Exit-Code ungleich 0 — und gibt nichts aus: kein Briefing aus der lokalen
// Ebene allein.
func TestBriefingOhneMitgelieferteEbeneEndetMitFehler(t *testing.T) {
	briefingProject(t)
	original := project.OpenDefaultsStore
	project.OpenDefaultsStore = func(string) (project.DefaultsStore, string, error) {
		return nil, "vm:k-playbook", errors.New("Kephalaion ist für dieses Projekt nicht bereit")
	}
	t.Cleanup(func() { project.OpenDefaultsStore = original })

	stdout, err := runBriefingCaptured(t)
	if err == nil || exitCodeOf(err) == 0 {
		t.Fatalf("briefing: %v (Code %d), erwartet einen Fehler", err, exitCodeOf(err))
	}
	if project.DocRefErrorCode(err) != project.DocCodeKephalaionUnavailable || !strings.Contains(err.Error(), "/k-gui") ||
		!strings.HasPrefix(err.Error(), "Kephalaion ist nicht erreichbar") ||
		!strings.HasSuffix(err.Error(), "\nNächster Schritt: k-playbook troubleshooting") {
		t.Errorf("Fehler: %v", err)
	}
	if stdout != "" {
		t.Errorf("Ausgabe trotz Fehler:\n%s", stdout)
	}
}

// Ein Dokument mit unlesbarem Frontmatter (Task 136) steht im Klartext mit
// seinem Problem hinter dem Titel — dort, wo die Sitzung ohnehin hinsieht;
// ein sauberes Dokument bleibt eine Zeile aus Verweis und Titel.
func TestBriefingKlartextNenntFrontmatterProblem(t *testing.T) {
	briefing := project.Briefing{Work: project.BriefingWork{Available: true, User: "kamran",
		Tasks: []project.BriefingDoc{
			{Ref: "keph:/user/kamran/tasks/100-sauber.md", Title: "Sauber"},
			{Ref: "keph:/user/kamran/tasks/101-kaputt.md", Title: "101-kaputt.md",
				Problem: "Frontmatter nicht lesbar: YAML ungültig: mapping values are not allowed in this context"},
		},
		Workbench: []project.BriefingDoc{
			{Ref: "keph:/user/kamran/workbench/001-todo.md", Title: "Offene Punkte (Todo",
				Problem: "Titel vermutlich an ‚ #‘ abgeschnitten (YAML-Kommentar); Titel in doppelte Anführungszeichen setzen"},
		}}}
	var out strings.Builder
	if err := writeBriefingText(&out, briefing); err != nil {
		t.Fatal(err)
	}
	want := "Eigene Arbeit (kamran):\n" +
		"  Offene Tasks (2) in keph:/user/kamran/tasks/:\n" +
		"    100-sauber.md  Sauber\n" +
		"    101-kaputt.md  101-kaputt.md — Frontmatter nicht lesbar: YAML ungültig: mapping values are not allowed in this context\n" +
		"  Aktive Arbeitsstände (1) in keph:/user/kamran/workbench/:\n" +
		"    001-todo.md  Offene Punkte (Todo — Titel vermutlich an ‚ #‘ abgeschnitten (YAML-Kommentar); Titel in doppelte Anführungszeichen setzen\n" +
		"  Offene Befunde: 0\n"
	if !strings.Contains(out.String(), want) {
		t.Errorf("Klartext ohne die Probleme:\n%s", out.String())
	}
}

// Der Block nach der Zeile Sitzung (Task 187): je Abschnitt eine Zeile unter
// seinem Feldnamen, die Felder mit Namen und Wert. links steht nur da, wenn
// es etwas meldet, mit je einer Zeile für healed, open und note; eine
// mehrzeilige Meldung bleibt eine Zeile. Zusammen höchstens acht Zeilen.
func TestBriefingTextSections(t *testing.T) {
	briefing := project.Briefing{
		Now:        project.ContextNow{Date: "2026-11-24", Timestamp: "2026-11-24T10:55:30+01:00"},
		Project:    project.BriefingProject{Available: true, Dir: "/p", RepoRoot: "/p/app", VCS: "git", Languages: []string{"go", "python"}},
		Kephalaion: project.BriefingKephalaion{OK: true, Message: "Angemeldet als k-wsl (User kamran);\nvm:projekt ist lesbar.", Address: "vm:projekt", User: "kamran"},
	}
	want := "now: date 2026-11-24 · timestamp 2026-11-24T10:55:30+01:00\n" +
		"project: dir /p · repoRoot /p/app · vcs git · languages go, python\n" +
		"kephalaion: ok true · address vm:projekt · user kamran · message Angemeldet als k-wsl (User kamran); vm:projekt ist lesbar.\n"
	if got := briefingTextSections(briefing); got != want {
		t.Errorf("Block:\n%s\nerwartet\n%s", got, want)
	}

	briefing.Links = &project.ContextLinks{
		Healed: project.LinkChanges{Added: []string{"k-neu.md", "k-zwei.md"}, Removed: []string{"k-alt.md"}},
		Open:   []project.LinkIssue{{Path: ".opencode/commands", State: "blocked", Detail: "eine echte Datei\nliegt im Weg"}},
		Note:   "Die Assistenten-Registrierung wurde nachgezogen.",
	}
	wantLinks := "links.healed: added k-neu.md, k-zwei.md; removed k-alt.md\n" +
		"links.open: .opencode/commands (blocked): eine echte Datei liegt im Weg\n" +
		"links.note: Die Assistenten-Registrierung wurde nachgezogen.\n"
	got := briefingTextSections(briefing)
	if got != want+wantLinks {
		t.Errorf("Block mit links:\n%s\nerwartet\n%s", got, want+wantLinks)
	}
	if lines := strings.Count(got, "\n"); lines > 8 {
		t.Errorf("Block hat %d Zeilen, höchstens 8", lines)
	}

	// Ein Abschnitt, der sich nicht bauen ließ, sagt das an seiner Stelle.
	briefing.Links = nil
	briefing.Project = project.BriefingProject{Dir: "/p", Message: "K-PLAYBOOK.yaml hat schema_version 99"}
	briefing.Kephalaion = project.BriefingKephalaion{Message: "Keine Collection eingetragen."}
	want = "now: date 2026-11-24 · timestamp 2026-11-24T10:55:30+01:00\n" +
		"project: dir /p · nicht verfügbar — K-PLAYBOOK.yaml hat schema_version 99\n" +
		"kephalaion: ok false · message Keine Collection eingetragen.\n"
	if got := briefingTextSections(briefing); got != want {
		t.Errorf("Block ohne Projekt:\n%s\nerwartet\n%s", got, want)
	}
}

// Die Form einer Zeile unter „Auf Abruf“, „Reviews“ und „Leitlinien“
// (Task 187): Verweis und Titel; die Herkunft nur, wenn sie nicht dist ist,
// mit dem keph:-Ort, wenn es einen gibt; nie die Beschreibung.
func TestBriefingEntryLine(t *testing.T) {
	for _, tc := range []struct {
		entry project.BriefingEntry
		want  string
	}{
		{project.BriefingEntry{Ref: "k-playbook:/instructions/on-demand/tasks.md", Title: "Tasks", Description: "Wie ein Task aussieht.", Origin: project.DocOriginDist},
			"  k-playbook:/instructions/on-demand/tasks.md  Tasks\n"},
		{project.BriefingEntry{Ref: "k-playbook:/instructions/on-demand/eigen.md", Title: "Eigen", Description: "Lang.", Origin: project.DocOriginLocal,
			Location: "keph:/k-playbook/instructions/on-demand/eigen.md"},
			"  k-playbook:/instructions/on-demand/eigen.md  [local keph:/k-playbook/instructions/on-demand/eigen.md]  Eigen\n"},
		{project.BriefingEntry{Ref: "k-playbook:/reviews/review-code.md", Title: "Code-Review", Origin: project.DocOriginOverride,
			Location: "keph:/k-playbook/reviews/review-code.md"},
			"  k-playbook:/reviews/review-code.md  [override keph:/k-playbook/reviews/review-code.md]  Code-Review\n"},
		{project.BriefingEntry{Ref: "k-playbook:/reviews/review-tech.md", Title: "review-tech.md", Origin: project.BriefingOriginDisabled},
			"  k-playbook:/reviews/review-tech.md  [disabled]  review-tech.md\n"},
	} {
		if got := briefingEntryLine(tc.entry); got != tc.want {
			t.Errorf("Zeile für %s (%s):\n%q\nerwartet\n%q", tc.entry.Ref, tc.entry.Origin, got, tc.want)
		}
	}
}

// titledDocStore ist eine Collection im Speicher, deren list wie der Node das
// Frontmatter mitliefert: den Titel aus titles, sonst keinen. kephtest.Store
// allein nennt kein Frontmatter, und ein Briefing ohne Titel wäre kürzer als
// ein echtes.
type titledDocStore struct {
	*kephtest.Store
	titles map[string]string
}

func (s titledDocStore) List(ctx context.Context, req kephalaion.ListRequest) ([]kephalaion.Entry, error) {
	entries, err := s.Store.List(ctx, req)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		if title, ok := s.titles[entries[i].Name]; ok && req.Frontmatter {
			entries[i].Frontmatter = map[string]any{"title": title}
		}
	}
	return entries, nil
}

// briefingWorkStore setzt für die Dauer des Tests eine Collection mit tasks
// offenen Tasks und themes aktiven Arbeitsständen des Users kamran ein, mit
// Titeln in der Länge echter Titel, dazu einem erledigten Task und zwei
// offenen Befunden. Die Arbeitsstände tragen verschiedene Änderungszeiten:
// je höher die Nummer, desto jünger.
func briefingWorkStore(t *testing.T, tasks int, themes int) {
	t.Helper()
	store := titledDocStore{Store: kephtest.New(), titles: map[string]string{}}
	for i := 1; i <= tasks; i++ {
		name := fmt.Sprintf("user/kamran/tasks/%03d-ein-task-mit-einem-ueblichen-namen.md", 100+i)
		store.Put(name, "")
		store.titles[name] = fmt.Sprintf("Task %d: ein Titel in der Länge, die Titel offener Tasks üblicherweise haben", i)
	}
	for i := 1; i <= themes; i++ {
		name := fmt.Sprintf("user/kamran/workbench/%03d-ein-thema-mit-einem-ueblichen-namen.md", i)
		store.Put(name, "")
		store.Docs[name].Updated = fmt.Sprintf("2026-11-%02dT10:00:00.000Z", 1+i/2)
		store.titles[name] = fmt.Sprintf("Thema %d: ein Titel in der Länge, die Titel aktiver Arbeitsstände üblicherweise haben", i)
	}
	store.Put("user/kamran/tasks/done/001-erledigt.md", "")
	store.Put("queue/findings/001-a.md", "")
	store.Put("queue/findings/002-b.md", "")
	original := project.OpenBriefingDocs
	project.OpenBriefingDocs = func(string) (*project.UserDocs, error) {
		return project.NewUserDocs(project.DocKinds, store, "kamran")
	}
	t.Cleanup(func() { project.OpenBriefingDocs = original })
}

// Die Arbeitsliste im Klartext (Task 187): je Liste eine Kopfzeile mit der
// Zahl aller Dokumente und dem Verzeichnis-Verweis, darunter höchstens zehn
// Einträge mit Dateiname und Titel, dann der Hinweis, wie die übrigen zu holen
// sind. Im JSON dieselben zehn und total.
func TestBriefingKlartextDeckeltArbeitsliste(t *testing.T) {
	briefingProject(t)
	briefingWorkStore(t, 40, 40)

	stdout, err := runBriefingCaptured(t)
	if err != nil {
		t.Fatalf("briefing: %v", err)
	}
	for _, want := range []string{
		"Eigene Arbeit (kamran):\n  Offene Tasks (40) in keph:/user/kamran/tasks/:\n" +
			"    101-ein-task-mit-einem-ueblichen-namen.md  Task 1: ein Titel",
		"    110-ein-task-mit-einem-ueblichen-namen.md  Task 10: ein Titel in der Länge, die Titel offener Tasks üblicherweise haben\n" +
			"    + 30 weitere: k_playbook_doc_list (kind tasks)\n" +
			"  Aktive Arbeitsstände (40, zuletzt geänderte zuerst) in keph:/user/kamran/workbench/:\n" +
			"    040-ein-thema-mit-einem-ueblichen-namen.md  Thema 40: ein Titel",
		"    + 30 weitere: k_playbook_doc_list (kind workbench)\n  Offene Befunde: 2\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("Ausgabe enthält nicht %q:\n%s", want, stdout)
		}
	}
	if got := strings.Count(stdout, "-ein-task-mit-einem-ueblichen-namen.md  "); got != project.BriefingWorkLimit {
		t.Errorf("%d Tasks im Klartext, erwartet %d", got, project.BriefingWorkLimit)
	}
	if got := strings.Count(stdout, "-ein-thema-mit-einem-ueblichen-namen.md  "); got != project.BriefingWorkLimit {
		t.Errorf("%d Arbeitsstände im Klartext, erwartet %d", got, project.BriefingWorkLimit)
	}
	// Der Verweis je Eintrag steht nicht mehr da: Verzeichnis in der
	// Kopfzeile, Dateiname in der Zeile.
	if strings.Contains(stdout, "keph:/user/kamran/tasks/101-") {
		t.Errorf("Klartext schreibt den Verweis je Eintrag aus:\n%s", stdout)
	}

	stdout, err = runBriefingCaptured(t, "--json")
	if err != nil {
		t.Fatalf("briefing --json: %v", err)
	}
	var briefing project.Briefing
	if err := json.Unmarshal([]byte(stdout), &briefing); err != nil {
		t.Fatalf("kein JSON: %v", err)
	}
	work := briefing.Work
	if len(work.Tasks) != project.BriefingWorkLimit || len(work.Workbench) != project.BriefingWorkLimit ||
		work.Total.Tasks != 40 || work.Total.Workbench != 40 || work.OpenFindings != 2 {
		t.Errorf("work im JSON: %d Tasks, %d Arbeitsstände, total %+v, Befunde %d", len(work.Tasks), len(work.Workbench), work.Total, work.OpenFindings)
	}
	if work.Tasks[0].Ref != "keph:/user/kamran/tasks/101-ein-task-mit-einem-ueblichen-namen.md" {
		t.Errorf("erster Task im JSON: %+v", work.Tasks[0])
	}
}

// briefingTextLimit ist die Obergrenze für den Klartext des Briefings in
// Zeichen (Task 187). Claude Code legt eine Shell-Ausgabe ab 30 000 Zeichen
// ganz in eine Datei und zeigt nur eine Vorschau (gemessen 2026-11-24); der
// Abstand dazu ist der Raum für projekteigene Anweisungen und längere Titel.
const briefingTextLimit = 24000

// Der Klartext des Briefings bleibt klein genug, um über die Shell in einem
// Stück anzukommen (Task 187): über die Anweisungen, Reviews und Leitlinien,
// die das Repo ausliefert (defaults/), mit 40 offenen Tasks und 40 aktiven
// Arbeitsständen höchstens briefingTextLimit Zeichen — Zeichen, nicht Bytes.
// Die Arbeitsliste wächst mit dem Projekt; der Deckel hält sie aus der
// Rechnung. Wer die immer geltenden Anweisungen verlängert, sieht es hier.
//
// Gemessen wird ein Projekt, in dem die Umgebung stimmt: eine bereite
// Anbindung, keine Einträge der Umgebungsprüfung, eine Verlinkung, die
// nichts meldet, ein Dienst für das Repo. Das Testprojekt hätte sonst eine
// nicht eingetragene Collection und offene Links im Kopf — Fehlertext, der
// über die Größe der mitgelieferten Texte nichts sagt.
func TestBriefingKlartextBleibtUnterDerGrenze(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	accesstest.Empty(t)
	// Vor dem Wechsel ins Testprojekt: Der Pfad gilt ab dem Paketverzeichnis.
	repoDefaults, err := filepath.Abs(filepath.Join("..", "..", "..", "defaults"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repoDefaults, "instructions", "always", "030-briefing.md")); err != nil {
		t.Fatalf("defaults/ des Repos nicht gefunden: %v", err)
	}
	// Ein Projekt ohne eigene Anweisungen: nur der Lieferumfang zählt.
	root := todoProject(t)
	briefingWorkStore(t, 40, 40)
	original := project.OpenDefaultsStore
	project.OpenDefaultsStore = func(string) (project.DefaultsStore, string, error) {
		store, err := kephtest.Defaults(repoDefaults)
		return store, "test:k-playbook", err
	}
	t.Cleanup(func() { project.OpenDefaultsStore = original })

	briefing, err := project.BuildBriefingFrom(t.Context(), root, root)
	if err != nil {
		t.Fatalf("BuildBriefingFrom: %v", err)
	}
	if !briefing.Project.Available || !briefing.Work.Available || briefing.Work.Total.Tasks != 40 || briefing.Work.Total.Workbench != 40 {
		t.Fatalf("project %+v, work available %v total %+v", briefing.Project, briefing.Work.Available, briefing.Work.Total)
	}
	zero, one := 0, 1
	briefing.Kephalaion = project.BriefingKephalaion{OK: true, Address: "vm:ein-projekt", User: "kamran",
		Message: "Angemeldet als kamran-wsl (User kamran); vm:ein-projekt ist lesbar."}
	briefing.Health = health.Report{OK: true, Problems: []health.Problem{}}
	briefing.Links = nil
	briefing.GitHub = project.BriefingGitHub{ProjectServices: []string{"ein-projekt"}, Access: &project.ContextGitHub{
		File: "/home/kamran/.config/k-playbook/github.yaml", Present: true, Problems: []githubaccess.Problem{},
		Remote: "git@github-kamran:kascada/ein-projekt.git", Repo: "kascada/ein-projekt", ProjectServices: []string{"ein-projekt"},
		Services: githubaccess.Services{{Name: "ein-projekt", Repo: "kascada/ein-projekt",
			Selected: &githubaccess.Selection{Read: githubaccess.ChannelIndex{Git: &zero, API: &one}, Write: githubaccess.ChannelIndex{Git: &zero, API: &one}},
			Access: []githubaccess.Access{
				{Type: "ssh", Host: "github-kamran", State: githubaccess.StateAvailable},
				{Type: "gh", Login: "KamranBeispiel", State: githubaccess.StateAvailable},
			}}},
	}}

	var out strings.Builder
	if err := writeBriefingText(&out, briefing); err != nil {
		t.Fatal(err)
	}
	stdout := out.String()
	// Es ist das Briefing über den Lieferumfang des Repos, mit gedeckelter
	// Arbeitsliste und allen Teilen.
	for _, want := range []string{"\nkephalaion: ok true · address vm:ein-projekt · user kamran · message ", "## Verweise\n", "## Das Briefing\n",
		"\nUmgebung: keine Probleme\n", "k-playbook:/instructions/on-demand/briefing-felder.md  Die Felder des Briefings\n",
		"k-playbook:/reviews/review-code.md  ", "k-playbook:/guidelines/betrieb.md  ",
		"Offene Tasks (40) in keph:/user/kamran/tasks/:", "+ 30 weitere: k_playbook_doc_list (kind tasks)",
		"Aktive Arbeitsstände (40, zuletzt geänderte zuerst) in keph:/user/kamran/workbench/:", "+ 30 weitere: k_playbook_doc_list (kind workbench)",
		"\nGitHub: dieses Repo = Dienst ein-projekt — run: git über ssh host github-kamran, api über gh KamranBeispiel\n"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("Ausgabe enthält nicht %q:\n%s", want, stdout)
		}
	}
	got := utf8.RuneCountInString(stdout)
	if got > briefingTextLimit {
		t.Errorf("Klartext des Briefings hat %d Zeichen, höchstens %d", got, briefingTextLimit)
	}
	t.Logf("Klartext des Briefings über defaults/ mit 40 Tasks und 40 Arbeitsständen: %d Zeichen (Grenze %d)", got, briefingTextLimit)
}

// Ein total unter der Länge der Liste gilt als die Länge: Die Zahl im Kopf
// sinkt nie unter die gezeigten Einträge, und eine Zeile „weitere“ entsteht
// daraus nicht.
func TestBriefingWorkListTotalUnterLaenge(t *testing.T) {
	list := []project.BriefingDoc{
		{Ref: "keph:/user/kamran/workbench/001-a.md", Title: "A"},
		{Ref: "keph:/user/kamran/workbench/002-b.md", Title: "B"},
		{Ref: "keph:/user/kamran/workbench/003-c.md", Title: "C"},
	}
	for _, total := range []int{0, 1, 2, 3} {
		var b strings.Builder
		writeBriefingWorkList(&b, "Aktive Arbeitsstände", "zuletzt geänderte zuerst", "workbench", "kamran", list, total)
		text := b.String()
		if !strings.HasPrefix(text, "  Aktive Arbeitsstände (3, zuletzt geänderte zuerst) in keph:/user/kamran/workbench/:\n") {
			t.Errorf("total %d: Kopf %q", total, strings.SplitN(text, "\n", 2)[0])
		}
		if strings.Contains(text, "weitere") {
			t.Errorf("total %d: Zeile weitere: %q", total, text)
		}
		for _, name := range []string{"001-a.md  A", "002-b.md  B", "003-c.md  C"} {
			if !strings.Contains(text, "    "+name+"\n") {
				t.Errorf("total %d: %s fehlt: %q", total, name, text)
			}
		}
	}
}
