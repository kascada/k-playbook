package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// readProject legt ein Projekt mit beiden Ebenen unter instructions/ an und
// macht es zum Arbeitsverzeichnis.
func readProject(t *testing.T) string {
	t.Helper()

	root := todoProject(t)
	files := map[string]string{
		"k-playbook/defaults/instructions/always/010-a.md":    "---\ntitle: A\n---\n\n## A\n",
		"k-playbook/defaults/instructions/always/100-b.md":    "## B mitgeliefert\n",
		"k-playbook/defaults/instructions/always/200-c.md":    "## C\n",
		"k-playbook/defaults/instructions/on-demand/thema.md": "---\ntitle: Thema\ndescription: D\n---\n\n## Thema\n",
		"k-playbook-local/instructions/always/100-b.md":       "## B lokal\n\nText.\n",
		"k-playbook-local/instructions/always/200-c.md":       "# Abgeschaltet\n",
		"k-playbook-local/instructions/always/notiz.md":       "## Notiz\n\nText.\n",
		"k-playbook-local/instructions/on-demand/aus.md":      "",
		"k-playbook/defaults/instructions/on-demand/aus.md":   "## Aus\n",
		"k-playbook/commands/k-upgrade.md":                    "---\ndescription: Hülle\n---\n\n# k-upgrade\n\nHülle\n",
		"k-playbook/defaults/commands/k-upgrade.md":           "# k-upgrade\n\nText aus defaults.\n",
		"k-playbook/defaults/commands/k-gui.md":               "# k-gui\n\nText aus defaults.\n",
		"k-playbook-local/commands/k-gui.md":                  "# k-gui\n\nlokal\n",
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

// runReadCaptured führt das Subkommando aus und fängt stdout und stderr ab.
func runReadCaptured(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	dir := t.TempDir()
	stdoutFile, err := os.Create(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatalf("Ausgabedatei: %v", err)
	}
	stderrFile, err := os.Create(filepath.Join(dir, "stderr"))
	if err != nil {
		t.Fatalf("Fehlerdatei: %v", err)
	}
	beforeOut, beforeErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutFile, stderrFile
	runErr := runRead(args)
	os.Stdout, os.Stderr = beforeOut, beforeErr
	stdoutFile.Close()
	stderrFile.Close()

	stdout, err := os.ReadFile(stdoutFile.Name())
	if err != nil {
		t.Fatalf("Ausgabe lesen: %v", err)
	}
	stderr, err := os.ReadFile(stderrFile.Name())
	if err != nil {
		t.Fatalf("Fehlerausgabe lesen: %v", err)
	}
	return string(stdout), string(stderr), runErr
}

func TestReadBautVerzeichnisZusammen(t *testing.T) {
	readProject(t)

	stdout, stderr, err := runReadCaptured(t, "k-playbook:/instructions/always/")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if stdout != "## A\n\n## B lokal\n\nText.\n" {
		t.Errorf("stdout: %q", stdout)
	}
	if !strings.Contains(stderr, "notiz.md") {
		t.Errorf("Datei ohne Ziffer nicht auf stderr gemeldet: %q", stderr)
	}
}

func TestReadAbgeschalteteDateiIstKeinFehler(t *testing.T) {
	readProject(t)

	stdout, stderr, err := runReadCaptured(t, "k-playbook:/instructions/on-demand/aus.md")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if stdout != "" {
		t.Errorf("stdout bei abgeschalteter Datei: %q", stdout)
	}
	if !strings.Contains(stderr, "abgeschaltet") {
		t.Errorf("Hinweis fehlt auf stderr: %q", stderr)
	}
}

func TestReadJSON(t *testing.T) {
	readProject(t)

	stdout, _, err := runReadCaptured(t, "k-playbook:/instructions/on-demand/thema.md", "--json")
	if err != nil {
		t.Fatalf("read --json: %v", err)
	}
	var result project.DocResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("kein JSON: %v — %s", err, stdout)
	}
	if result.Content != "---\ntitle: Thema\ndescription: D\n---\n\n## Thema\n" || result.Disabled || result.Directory {
		t.Errorf("Antwort: %+v", result)
	}
	if len(result.Parts) != 1 || result.Parts[0].Origin != "dist" {
		t.Errorf("parts: %+v", result.Parts)
	}
}

func TestReadUnbekannterVerweisIstFehler(t *testing.T) {
	readProject(t)

	_, _, err := runReadCaptured(t, "k-playbook:/instructions/on-demand/fehlt.md")
	if err == nil || !strings.Contains(err.Error(), "not_found") {
		t.Fatalf("unbekannter Verweis: %v", err)
	}
	// Datei wie ein Verzeichnis behandelt: unbekannt, kein Systemfehler.
	_, _, err = runReadCaptured(t, "k-playbook:/instructions/always/010-a.md/x.md")
	if err == nil || !strings.Contains(err.Error(), "not_found") || strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("…/010-a.md/x.md: %v", err)
	}
	// Mit „/" am Ende fehlt der Dateiname — wie bei einem Verzeichnis, das
	// nicht zusammengebaut wird.
	for _, ref := range []string{
		"k-playbook:/instructions/always/010-a.md/",
		"k-playbook:/instructions/always/100-b.md/",
		"k-playbook:/instructions/on-demand/",
	} {
		_, _, err = runReadCaptured(t, ref)
		if err == nil || !strings.Contains(err.Error(), "(invalid_input)") || !strings.Contains(err.Error(), "Dateiname fehlt") {
			t.Fatalf("%s: %v", ref, err)
		}
	}
	// Ohne Anbindung ist die Collection nicht erreichbar; der Node wird nicht
	// gefragt.
	_, _, err = runReadCaptured(t, "keph:/manual/abc.md")
	if err == nil || !strings.Contains(err.Error(), "(kephalaion_unavailable)") {
		t.Fatalf("keph: %v", err)
	}
}

// Der Text eines Commands kommt aus defaults/commands/, nicht aus der Hülle
// oben in commands/; lokal ersetzt (Task 092).
func TestReadLiestCommandTextAusDefaults(t *testing.T) {
	readProject(t)

	stdout, stderr, err := runReadCaptured(t, "k-playbook:/commands/k-upgrade.md")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if stdout != "# k-upgrade\n\nText aus defaults.\n" || stderr != "" {
		t.Errorf("stdout %q, stderr %q", stdout, stderr)
	}
	stdout, _, err = runReadCaptured(t, "k-playbook:/commands/k-gui.md", "--json")
	if err != nil {
		t.Fatalf("read --json: %v", err)
	}
	var result project.DocResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("kein JSON: %v — %s", err, stdout)
	}
	if result.Content != "# k-gui\n\nlokal\n" || len(result.Parts) != 1 || result.Parts[0].Origin != "override" {
		t.Errorf("Antwort: %+v", result)
	}

	_, _, err = runReadCaptured(t, "k-playbook:/commands/")
	if err == nil || !strings.Contains(err.Error(), "(invalid_input)") || !strings.Contains(err.Error(), "Dateiname fehlt") {
		t.Fatalf("commands/ mit / am Ende: %v", err)
	}
	_, _, err = runReadCaptured(t, "k-playbook:/checks/check_x.sh")
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("checks: %v", err)
	}
}

// project: liest eine Datei des Projekts ab der Wurzel (dem Verzeichnis von
// K-PLAYBOOK.yaml), ohne Überlagerung.
func TestReadLiestProjektdatei(t *testing.T) {
	root := readProject(t)
	for rel, content := range map[string]string{
		"scripts/123.sh":                  "#!/bin/sh\necho hallo\n",
		"k-playbook-local/docs/README.md": "# Doku\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	stdout, stderr, err := runReadCaptured(t, "project:/scripts/123.sh")
	if err != nil || stdout != "#!/bin/sh\necho hallo\n" || stderr != "" {
		t.Fatalf("read project: %v — stdout %q, stderr %q", err, stdout, stderr)
	}
	stdout, _, err = runReadCaptured(t, "project:/k-playbook-local/docs/README.md", "--json")
	if err != nil {
		t.Fatalf("read --json: %v", err)
	}
	var result project.DocResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("kein JSON: %v — %s", err, stdout)
	}
	if result.Content != "# Doku\n" || len(result.Parts) != 1 || result.Parts[0].Origin != "project" || result.Disabled {
		t.Errorf("Antwort: %+v", result)
	}

	_, _, err = runReadCaptured(t, "project:/scripts/fehlt.sh")
	if err == nil || !strings.Contains(err.Error(), "(not_found)") {
		t.Fatalf("fehlende Projektdatei: %v", err)
	}
	_, _, err = runReadCaptured(t, "project:/scripts/")
	if err == nil || !strings.Contains(err.Error(), "(invalid_input)") || !strings.Contains(err.Error(), "Dateiname fehlt") {
		t.Fatalf("project: mit / am Ende: %v", err)
	}
}

func TestReadHilfeIstFolgenlos(t *testing.T) {
	for _, args := range [][]string{{}, {"--help"}} {
		stdout, _, err := runReadCaptured(t, args...)
		if err != nil || !strings.Contains(stdout, "k-playbook read <verweis>") {
			t.Errorf("read %v: %v — %q", args, err, stdout)
		}
	}
}

// Includes: eingesetzt, mit --raw roh, Zyklus als Fehler mit Code, --json mit
// includes.
func TestReadSetztIncludesEin(t *testing.T) {
	root := readProject(t)
	for rel, content := range map[string]string{
		"k-playbook/defaults/includes/context.md":             "---\ntitle: Kontext\n---\n\n## Kontext\n\nBaustein.\n",
		"k-playbook/defaults/commands/k-danke.md":             "# k-danke\n\ninclude: k-playbook:/includes/context.md\n",
		"k-playbook/defaults/commands/verschachtelt.md":       "# V\n\ninclude: k-danke.md\n",
		"k-playbook/defaults/commands/zyklus-a.md":            "include: zyklus-b.md\n",
		"k-playbook/defaults/commands/zyklus-b.md":            "include: zyklus-a.md\n",
		"k-playbook-local/instructions/always/1000-kaputt.md": "## Kaputt\n\ninclude: fehlt.md\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	stdout, stderr, err := runReadCaptured(t, "k-playbook:/commands/verschachtelt.md")
	if err != nil || stdout != "# V\n\n# k-danke\n\n## Kontext\n\nBaustein.\n" || stderr != "" {
		t.Fatalf("verschachtelt: %v — stdout %q, stderr %q", err, stdout, stderr)
	}
	stdout, _, err = runReadCaptured(t, "k-playbook:/commands/verschachtelt.md", "--raw")
	if err != nil || stdout != "# V\n\ninclude: k-danke.md\n" {
		t.Fatalf("--raw: %v — %q", err, stdout)
	}
	_, _, err = runReadCaptured(t, "k-playbook:/commands/zyklus-a.md")
	if err == nil || !strings.Contains(err.Error(), "(include_cycle)") || !strings.Contains(err.Error(), "Include-Kette") {
		t.Fatalf("Zyklus: %v", err)
	}
	if _, _, err = runReadCaptured(t, "k-playbook:/commands/zyklus-a.md", "--raw"); err != nil {
		t.Fatalf("Zyklus roh: %v", err)
	}

	stdout, _, err = runReadCaptured(t, "k-playbook:/commands/verschachtelt.md", "--json")
	if err != nil {
		t.Fatalf("read --json: %v", err)
	}
	var result project.DocResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("kein JSON: %v — %s", err, stdout)
	}
	if len(result.Includes) != 2 || result.Includes[0].Ref != "k-playbook:/commands/k-danke.md" || result.Includes[0].Depth != 1 ||
		result.Includes[1].Ref != "k-playbook:/includes/context.md" || result.Includes[1].Depth != 2 || result.Includes[1].Origin != "dist" {
		t.Errorf("includes: %+v", result.Includes)
	}
	// Im zusammengebauten Verzeichnis fällt nur der kaputte Teil aus; der
	// Hinweis steht auf stderr, Exit 0.
	stdout, stderr, err = runReadCaptured(t, "k-playbook:/instructions/always/")
	if err != nil || stdout != "## A\n\n## B lokal\n\nText.\n" {
		t.Fatalf("Verzeichnis mit kaputtem Teil: %v — %q", err, stdout)
	}
	if !strings.Contains(stderr, "1000-kaputt.md") || !strings.Contains(stderr, "Include-Kette") {
		t.Errorf("Hinweis fehlt auf stderr: %q", stderr)
	}
	// Die Sorten includes/ und testdata/ stehen nicht in der Hilfe.
	stdout, _, _ = runReadCaptured(t, "--help")
	if strings.Contains(stdout, "includes/") || strings.Contains(stdout, "testdata/") {
		t.Errorf("Hilfe nennt Sorten für Leser, die keine sind: %q", stdout)
	}
	if !strings.Contains(stdout, "--raw") || !strings.Contains(stdout, "project:/") || !strings.Contains(stdout, "keph:/") {
		t.Errorf("Hilfe ohne --raw oder Präfixe: %q", stdout)
	}
}

// Der Arbeitsstand defaults/ des Repos als mitgelieferte Ebene eines
// Testprojekts: `k-playbook read` zeigt /k-upgrade mit eingesetztem Rumpf von
// _shared/context.md, mit --raw die Include-Zeile (Task 094). Die Installation
// daneben trägt den vorigen Stand und taugt nicht als Abnahme.
func TestReadKUpgradeAusDefaultsMitInclude(t *testing.T) {
	// Den Repo-Pfad vor dem Wechsel ins Testprojekt auflösen.
	repoDefaults, err := filepath.Abs(filepath.Join("..", "..", "..", "defaults"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repoDefaults, "commands", "_shared", "context.md")); err != nil {
		t.Fatalf("defaults/commands/_shared/context.md im Repo nicht gefunden: %v", err)
	}
	root := todoProject(t)
	playbook := filepath.Join(root, "k-playbook")
	if err := os.MkdirAll(playbook, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(repoDefaults, filepath.Join(playbook, "defaults")); err != nil {
		t.Skipf("Symlink nicht möglich: %v", err)
	}

	stdout, stderr, err := runReadCaptured(t, "k-playbook:/commands/k-upgrade.md")
	if err != nil || stderr != "" {
		t.Fatalf("read: %v — stderr %q", err, stderr)
	}
	for _, want := range []string{"# k-upgrade\n", "Liegt das Briefing in dieser Sitzung vor, nimm es.", "`context.playbook.dir`"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("eingesetzter Text ohne %q:\n%s", want, stdout)
		}
	}
	for _, absent := range []string{"include:", "title: Briefing laden", "k-playbook context"} {
		if strings.Contains(stdout, absent) {
			t.Errorf("eingesetzter Text enthält %q:\n%s", absent, stdout)
		}
	}

	raw, _, err := runReadCaptured(t, "k-playbook:/commands/k-upgrade.md", "--raw")
	if err != nil {
		t.Fatalf("read --raw: %v", err)
	}
	if !strings.Contains(raw, "\ninclude: _shared/context.md\n") || strings.Contains(raw, "Liegt das Briefing") {
		t.Errorf("--raw zeigt nicht die Include-Zeile:\n%s", raw)
	}

	stdout, _, err = runReadCaptured(t, "k-playbook:/commands/k-upgrade.md", "--json")
	if err != nil {
		t.Fatalf("read --json: %v", err)
	}
	var result project.DocResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("kein JSON: %v — %s", err, stdout)
	}
	if len(result.Includes) != 1 || result.Includes[0].Ref != "k-playbook:/commands/_shared/context.md" || result.Includes[0].Depth != 1 || result.Includes[0].Origin != "dist" {
		t.Errorf("includes: %+v", result.Includes)
	}
}
