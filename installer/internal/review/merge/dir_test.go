package merge

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/review"
)

// codeSARIF ist die Fundliste eines Evidence-Rezepts code mit zwei Funden in
// main.go und einem in vendor/, außerhalb des Pfad-Scopes.
const codeSARIF = `{
  "version": "2.1.0",
  "runs": [{
    "tool": {"driver": {"name": "code"}},
    "results": [
      {"ruleId": "code-logic-error", "level": "error", "message": {"text": "Falsche Bedingung."},
       "locations": [{"physicalLocation": {"artifactLocation": {"uri": "main.go"}, "region": {"startLine": 3}}}]},
      {"ruleId": "code-logic-error", "level": "error", "message": {"text": "Noch eine."},
       "locations": [{"physicalLocation": {"artifactLocation": {"uri": "main.go"}, "region": {"startLine": 9}}}]},
      {"ruleId": "code-missing-test", "level": "note", "message": {"text": "Ohne Test."},
       "locations": [{"physicalLocation": {"artifactLocation": {"uri": "vendor/x/y.go"}, "region": {"startLine": 1}}}]}
    ]
  }]
}
`

// testCheck prüft wie der Katalog des Projekts: Rezept code mit zwei
// Rule-IDs und dem Pfad-Scope **/*.go.
func testCheck(entry string, data []byte) (review.EvidenceReport, error) {
	if entry != "code" {
		return review.EvidenceReport{}, errors.New("kein Rezept mit dem Schlüssel " + entry + " im Review-Katalog")
	}
	return review.CheckEvidenceSARIF(data, entry, []string{"code-logic-error", "code-missing-test"}, []string{"**/*.go"})
}

func dirOptions(projectDir string, dir string) DirOptions {
	return DirOptions{ProjectDir: projectDir, Dir: dir, Check: testCheck, Now: fixedNow, KPlaybookVersion: "test"}
}

func newWorkDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "work")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func fileNames(t *testing.T, dir string) []string {
	t.Helper()
	items, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, item := range items {
		names = append(names, item.Name())
	}
	return names
}

// Ein Verzeichnis mit einer SARIF-Datei: der Beleg entsteht ohne run.json und
// entries/, mit Gruppen je Rule-ID und Datei, stabilen IDs ai-code-…, ohne
// known-decisions, und jede geschriebene Datei hat die Rechte 0600. Der Fund
// außerhalb des Scopes ist verworfen und das SARIF bereinigt zurückgeschrieben.
func TestMergeDirFasstEineFundlisteZusammen(t *testing.T) {
	projectDir := t.TempDir()
	dir := newWorkDir(t)
	writeText(t, filepath.Join(dir, "code.sarif"), codeSARIF)

	result, sources, output, err := MergeDir(dirOptions(projectDir, dir))
	if err != nil {
		t.Fatalf("MergeDir: %v", err)
	}
	if len(sources) != 1 || sources[0].Entry != "code" || sources[0].Findings != 2 || sources[0].Dropped != 1 {
		t.Fatalf("Quellen = %+v", sources)
	}
	if len(result.Findings) != 2 || len(result.Groups) != 1 {
		t.Fatalf("Findings %d, Gruppen %d, erwartet 2 und 1", len(result.Findings), len(result.Groups))
	}
	group := result.Groups[0]
	if !strings.HasPrefix(group.StableID, "ai-code-") || len(group.FindingIDs) != 2 || group.Location.URI != "main.go" {
		t.Errorf("Gruppe = %+v", group)
	}
	if result.Run.Name != "work" || result.Run.Dir != filepath.ToSlash(dir) || len(result.Run.SelectedEntries) != 1 ||
		result.Run.SelectedEntries[0].Mode != review.ModeEvidence || result.Run.SelectedEntries[0].Kind != review.KindAI {
		t.Errorf("Run = %+v", result.Run)
	}
	if len(result.KnownDecisions.Decisions) != 0 || len(result.KnownDecisions.Warnings) != 1 || result.KnownDecisions.Warnings[0] != knownDecisionsSkippedWarning {
		t.Errorf("KnownDecisions = %+v", result.KnownDecisions)
	}
	if result.Findings[0].DerivedSeverity == "" || result.Findings[0].SeveritySource != "native" {
		t.Errorf("Schwere = %q/%q", result.Findings[0].DerivedSeverity, result.Findings[0].SeveritySource)
	}

	for _, path := range []string{output.JSON, output.Markdown, filepath.Join(dir, "code.sarif")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("%s: Rechte %04o, erwartet 0600", path, info.Mode().Perm())
		}
	}
	cleaned, err := os.ReadFile(filepath.Join(dir, "code.sarif"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cleaned), "vendor/x/y.go") {
		t.Error("das SARIF trägt den verworfenen Fund noch")
	}
	var written Result
	data, err := os.ReadFile(output.JSON)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("review-input.json: %v", err)
	}
	if len(written.Groups) != 1 || written.Groups[0].StableID != group.StableID {
		t.Errorf("geschriebene Gruppen = %+v", written.Groups)
	}
	if markdown, _ := os.ReadFile(output.Markdown); !strings.Contains(string(markdown), "Bekannte Entscheidungen wurden nicht berücksichtigt") {
		t.Errorf("review-input.md nennt den Ausfall der known-decisions nicht:\n%s", markdown)
	}
}

// Die stabile ID hängt an Eintrag, Rule-ID und Pfad wie im Audit-Lauf: ein
// zweiter Merge über dieselben Funde vergibt dieselbe.
func TestMergeDirStabileIDWieImAudit(t *testing.T) {
	first, second := newWorkDir(t), newWorkDir(t)
	writeText(t, filepath.Join(first, "code.sarif"), codeSARIF)
	writeText(t, filepath.Join(second, "code.sarif"), strings.Replace(codeSARIF, `"startLine": 3`, `"startLine": 30`, 1))
	a, _, err := BuildDir(dirOptions("", first))
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := BuildDir(dirOptions("", second))
	if err != nil {
		t.Fatal(err)
	}
	if a.Groups[0].StableID != b.Groups[0].StableID {
		t.Errorf("stabile IDs %s und %s weichen ab", a.Groups[0].StableID, b.Groups[0].StableID)
	}
}

// Nur prüfen schreibt nichts, auch kein bereinigtes SARIF.
func TestCheckDirSchreibtNichts(t *testing.T) {
	dir := newWorkDir(t)
	writeText(t, filepath.Join(dir, "code.sarif"), codeSARIF)
	sources, err := CheckDir(dirOptions("", dir))
	if err != nil {
		t.Fatalf("CheckDir: %v", err)
	}
	if len(sources) != 1 || sources[0].Findings != 2 || sources[0].Dropped != 1 || !strings.Contains(sources[0].Note, "verworfen") {
		t.Errorf("Quellen = %+v", sources)
	}
	if names := fileNames(t, dir); len(names) != 1 {
		t.Errorf("Dateien nach der Prüfung: %v", names)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "code.sarif")); string(data) != codeSARIF {
		t.Error("die Prüfung hat das SARIF verändert")
	}
}

// Eine ungültige Fundliste hält den Merge an: Jede Quelle steht mit ihrem
// Grund im Fehler, und es wird nichts geschrieben.
func TestMergeDirWeistUngueltigeFundlisteAb(t *testing.T) {
	dir := newWorkDir(t)
	writeText(t, filepath.Join(dir, "code.sarif"), strings.Replace(codeSARIF, "code-missing-test", "code-erfunden", 1))
	writeText(t, filepath.Join(dir, "tech.sarif"), strings.Replace(codeSARIF, `"name": "code"`, `"name": "tech"`, 1))
	writeText(t, filepath.Join(dir, "Falsch.sarif"), codeSARIF)

	_, _, _, err := MergeDir(dirOptions("", dir))
	var checkErr *DirCheckError
	if !errors.As(err, &checkErr) {
		t.Fatalf("Fehler = %v, erwartet DirCheckError", err)
	}
	if len(checkErr.Problems) != 3 {
		t.Fatalf("Probleme = %+v", checkErr.Problems)
	}
	reasons := map[string]string{}
	for _, problem := range checkErr.Problems {
		reasons[problem.SARIF] = problem.Reason
	}
	if !strings.Contains(reasons["code.sarif"], "code-erfunden") || !strings.Contains(reasons["tech.sarif"], "kein Rezept") ||
		!strings.Contains(reasons["Falsch.sarif"], "Eintragsname") {
		t.Errorf("Gründe = %v", reasons)
	}
	if names := fileNames(t, dir); len(names) != 3 {
		t.Errorf("nach dem abgewiesenen Merge liegen %v", names)
	}
}

// Ohne SARIF-Datei gibt es nichts zusammenzufassen; andere Dateien zählen
// nicht als Quelle.
func TestMergeDirOhneFundliste(t *testing.T) {
	dir := newWorkDir(t)
	writeText(t, filepath.Join(dir, "review-triage.md"), "# alt\n")
	if _, _, _, err := MergeDir(dirOptions("", dir)); !errors.Is(err, ErrNoDirSARIF) {
		t.Errorf("Fehler = %v, erwartet ErrNoDirSARIF", err)
	}
	if _, err := CheckDir(DirOptions{Dir: dir}); err == nil || !strings.Contains(err.Error(), "Prüfung") {
		t.Errorf("ohne Check: %v", err)
	}
}

// Ein leeres SARIF ohne Ergebnisse ist eine gültige Fundliste: keine Funde,
// keine Gruppen, der Beleg entsteht trotzdem.
func TestMergeDirLeereFundliste(t *testing.T) {
	dir := newWorkDir(t)
	writeText(t, filepath.Join(dir, "code.sarif"), `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"code"}},"results":[]}]}`)
	result, _, output, err := MergeDir(dirOptions("", dir))
	if err != nil {
		t.Fatalf("MergeDir: %v", err)
	}
	if len(result.Findings) != 0 || len(result.Groups) != 0 {
		t.Errorf("Findings %d, Gruppen %d", len(result.Findings), len(result.Groups))
	}
	if _, err := os.Stat(output.JSON); err != nil {
		t.Error(err)
	}
}
