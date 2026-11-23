package project

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Task 176: /k-task-run und /k-task-refine sind je ein Kern mit Modulen unter
// commands/_task/, _task-run/ und _task-refine/. Die Tests lesen sie so, wie
// eine Installation nach dem Release: aufgelöst über die Repo-Defaults.

const (
	taskKernMaxBytes   = 32000
	taskModulMaxBytes  = 20000
	taskModulVerweisRe = `k-playbook:/commands/(_[a-z-]+/[a-z-]+\.md)`
)

// taskModulDateien nennt die Module unter commands/<dir>/ des Repos, ohne
// das Verzeichnis _shared und _skills.
func taskModulDateien(t *testing.T, dirs ...string) []string {
	t.Helper()
	var names []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(filepath.Join("..", "..", "..", "defaults", "commands", dir))
		if err != nil {
			t.Fatalf("defaults/commands/%s lesen: %v", dir, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				names = append(names, dir+"/"+entry.Name())
			}
		}
	}
	sort.Strings(names)
	return names
}

// taskCommandBuendel liefert den aufgelösten Kern und die aufgelösten Module
// der angegebenen Verzeichnisse hintereinander.
func taskCommandBuendel(t *testing.T, kern string, dirs ...string) string {
	t.Helper()
	layers := repoDefaultsLayers(t)
	var b strings.Builder
	b.WriteString(mustResolveDoc(t, layers, "k-playbook:/commands/"+kern).Content)
	for _, module := range taskModulDateien(t, dirs...) {
		b.WriteString("\n\n")
		b.WriteString(mustResolveDoc(t, layers, "k-playbook:/commands/"+module).Content)
	}
	return b.String()
}

// Kern höchstens 32 000 Bytes, jedes Modul höchstens 20 000, jeweils mit
// eingesetzten Includes.
func TestTaskCommandsGroessen(t *testing.T) {
	layers := repoDefaultsLayers(t)
	for _, kern := range []string{"k-task-run.md", "k-task-refine.md"} {
		got := mustResolveDoc(t, layers, "k-playbook:/commands/"+kern)
		if len(got.Problems) != 0 {
			t.Errorf("%s: Probleme beim Auflösen: %v", kern, got.Problems)
		}
		t.Logf("%s aufgelöst %d Bytes", kern, len(got.Content))
		if n := len(got.Content); n > taskKernMaxBytes {
			t.Errorf("%s aufgelöst %d Bytes, Grenze %d", kern, n, taskKernMaxBytes)
		}
	}
	for _, module := range taskModulDateien(t, "_task", "_task-run", "_task-refine") {
		got := mustResolveDoc(t, layers, "k-playbook:/commands/"+module)
		if len(got.Problems) != 0 {
			t.Errorf("%s: Probleme beim Auflösen: %v", module, got.Problems)
		}
		t.Logf("%s aufgelöst %d Bytes", module, len(got.Content))
		if n := len(got.Content); n > taskModulMaxBytes {
			t.Errorf("%s aufgelöst %d Bytes, Grenze %d", module, n, taskModulMaxBytes)
		}
	}
}

// Jeder Verweis `k-playbook:/commands/_…` in beiden Kernen und ihren Modulen
// löst auf und trägt title und description; jede Datei der Modulverzeichnisse
// wird von mindestens einem Text genannt oder eingebunden.
func TestTaskCommandsModulverweiseLoesenAuf(t *testing.T) {
	layers := repoDefaultsLayers(t)
	re := regexp.MustCompile(taskModulVerweisRe)
	includeRe := regexp.MustCompile(`(?m)^include: (_[a-z-]+/[a-z-]+\.md)$`)

	texts := []string{"k-task-run.md", "k-task-refine.md"}
	texts = append(texts, taskModulDateien(t, "_task", "_task-run", "_task-refine")...)
	genannt := map[string]bool{}
	for _, name := range texts {
		raw := readRepoDefault(t, "commands/"+name)
		var refs []string
		for _, m := range re.FindAllStringSubmatch(raw, -1) {
			refs = append(refs, m[1])
		}
		for _, m := range includeRe.FindAllStringSubmatch(raw, -1) {
			refs = append(refs, m[1])
		}
		for _, module := range refs {
			// Ein Verweis auf diff-review.md (DIFF_REVIEW_MODULE) ist ebenso ein Modul.
			ref := "k-playbook:/commands/" + module
			got := mustResolveDoc(t, layers, ref)
			if got.Disabled || got.Content == "" || len(got.Problems) != 0 {
				t.Errorf("%s verweist auf %s: %+v", name, ref, got)
				continue
			}
			title, description := docTitleAndDescription(module, []byte(got.Content))
			if title == "" || title == module || description == "" {
				t.Errorf("%s: Frontmatter title/description fehlt (title %q, description %q)", ref, title, description)
			}
			genannt[module] = true
		}
	}
	for _, module := range taskModulDateien(t, "_task", "_task-run", "_task-refine") {
		if !genannt[module] {
			t.Errorf("%s wird von keinem Text der Task-Commands genannt oder eingebunden", module)
		}
	}
}

// Kein Text des einen Commands nennt Schritte des anderen: Die Schrittnummern
// von /k-task-run bleiben, die von /k-task-refine gibt es nur in seinem Kern.
func TestTaskCommandsKeineFremdenSchrittnummern(t *testing.T) {
	runTexts := append([]string{"k-task-run.md"}, taskModulDateien(t, "_task-run")...)
	refineTexts := append([]string{"k-task-refine.md"}, taskModulDateien(t, "_task-refine")...)

	runNennt := regexp.MustCompile(`(?i)(refine-text|k-task-refine\.md|/k-task-refine)[^.\n]{0,60}(Schritte? \d)|Schritte? \d+[^.\n]{0,40}(des Refine|von /k-task-refine)`)
	refineNennt := regexp.MustCompile(`(?i)(k-task-run\.md|/k-task-run)[^.\n]{0,60}(Schritte? \d|\b5[a-g]\b)|(Schritte? \d+|\b5[a-g]\b)[^.\n]{0,40}(des Run|von /k-task-run)`)

	for _, name := range runTexts {
		raw := readRepoDefault(t, "commands/"+name)
		// Schritt 3 von /k-task-run selbst: „/k-task-refine (Schritt 3)“ meint den eigenen.
		raw = strings.ReplaceAll(raw, "`/k-task-refine` (Schritt 3)", "")
		raw = strings.ReplaceAll(raw, "von `/k-task-refine` (Schritt 3)", "")
		if m := runNennt.FindString(raw); m != "" {
			t.Errorf("%s nennt Schritte von /k-task-refine: %q", name, m)
		}
	}
	for _, name := range refineTexts {
		raw := readRepoDefault(t, "commands/"+name)
		if m := refineNennt.FindString(raw); m != "" {
			t.Errorf("%s nennt Schritte von /k-task-run: %q", name, m)
		}
		// Module tragen keine Schrittnummern des eigenen Commands.
		if strings.HasPrefix(name, "_task-refine/") && regexp.MustCompile(`Schritte? \d`).MatchString(raw) {
			t.Errorf("%s nennt eine Schrittnummer; Schrittnummern gibt es nur im Kern", name)
		}
	}
}

// Ein Sub-Agenten-Modul ist mit seinen Includes für sich vollständig: Es
// verweist auf keinen Schritt des Kerns, und wo es an den Task anhängt, trägt
// es die Revisionskette.
func TestTaskCommandsSubAgentenModuleSindFuerSichLesbar(t *testing.T) {
	layers := repoDefaultsLayers(t)
	schritt := regexp.MustCompile(`(?m)(\bSchritte? \d|\b5[a-g](\.[0-9])?\b)`)
	for _, module := range []string{"_task-run/ausfuehrung.md", "_task-run/befund-leser.md", "_task-run/intent-check.md", "_task-refine/dialog.md"} {
		got := mustResolveDoc(t, layers, "k-playbook:/commands/"+module)
		if m := schritt.FindString(got.Content); m != "" {
			t.Errorf("%s verweist auf einen Schritt des Kerns: %q", module, m)
		}
		if strings.Contains(got.Content, "k-playbook:/commands/k-task-") {
			t.Errorf("%s verweist auf den Kern eines Commands", module)
		}
	}
	for _, module := range []string{"_task-run/ausfuehrung.md", "_task-refine/dialog.md"} {
		got := mustResolveDoc(t, layers, "k-playbook:/commands/"+module)
		for _, want := range []string{"**Revisionskette.**", "`Foreign-Change: <revision> <updatedBy> <updatedAt>`", "`since_revision: REVISION`"} {
			if !strings.Contains(got.Content, want) {
				t.Errorf("%s ohne %q", module, want)
			}
		}
	}
}

// Task 177: Der Kern trägt die Weiche 4a zum Modul nachbessern.md; das Modul
// löst auf, trägt title und description, bleibt unter der Grenze und nennt die
// Obergrenze von drei Runden und den Ausschluss kreisender Befunde.
func TestTaskRunNachbessernWeicheUndModul(t *testing.T) {
	layers := repoDefaultsLayers(t)
	kern := mustResolveDoc(t, layers, "k-playbook:/commands/k-task-run.md").Content
	for _, want := range []string{"**4a. Nachbessern.**", "k-playbook:/commands/_task-run/nachbessern.md", "REVIEW_SCOPE"} {
		if !strings.Contains(kern, want) {
			t.Errorf("k-task-run.md ohne %q", want)
		}
	}

	const ref = "k-playbook:/commands/_task-run/nachbessern.md"
	got := mustResolveDoc(t, layers, ref)
	if got.Disabled || got.Content == "" || len(got.Problems) != 0 {
		t.Fatalf("%s: %+v", ref, got)
	}
	title, description := docTitleAndDescription("_task-run/nachbessern.md", []byte(got.Content))
	if title == "" || description == "" {
		t.Errorf("Frontmatter title/description fehlt (title %q, description %q)", title, description)
	}
	if n := len(got.Content); n > taskModulMaxBytes {
		t.Errorf("nachbessern.md aufgelöst %d Bytes, Grenze %d", n, taskModulMaxBytes)
	}
	for _, want := range []string{"höchstens **drei Runden**", "scheidet er aus der", "`check` der Sperre", "ausfuehrung.md"} {
		if !strings.Contains(got.Content, want) {
			t.Errorf("nachbessern.md ohne %q", want)
		}
	}
}
