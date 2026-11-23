package project

import (
	"regexp"
	"strings"
	"testing"
)

// Task 180: Die vier Docs-Commands (/k-docs-code, /k-docs-tools, /k-docs,
// /k-doc-inventory) sind je ein Kern, dazu Module unter commands/_docs/,
// _docs-code/ und _docs-tools/. Die Tests lesen sie wie eine Installation nach
// dem Release: aufgelöst über die Repo-Defaults. Muster ist
// shipped_task_commands_test.go.

const (
	docsKernMaxBytes  = 32000
	docsModulMaxBytes = 20000
	// docsLuft ist der Platz, den ein Text unter seiner Grenze lassen muss: Task
	// 177 hat den Kern von /k-task-run bis über die Grenze wachsen lassen.
	docsLuft = 2000
)

var docsKerne = []string{"k-docs-code.md", "k-docs-tools.md", "k-docs.md", "k-doc-inventory.md"}

// docsModuleDirs sind die Verzeichnisse der Docs-Module.
var docsModuleDirs = []string{"_docs", "_docs-code", "_docs-tools"}

// docsCommandBundle ist der Rohtext eines Kerns samt allen Modulen der
// angegebenen Verzeichnisse hintereinander, Leerraum zusammengefasst.
func docsCommandBundle(t *testing.T, kern string, dirs ...string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString(readRepoDefault(t, "commands/"+kern))
	for _, module := range taskModulDateien(t, dirs...) {
		b.WriteString("\n\n")
		b.WriteString(readRepoDefault(t, "commands/"+module))
	}
	return squashSpace(b.String())
}

// Kern höchstens 32 000 Bytes, jedes Modul höchstens 20 000, jeweils mit
// eingesetzten Includes — und mit mindestens 2 000 Bytes Luft darunter.
func TestDocsCommandsGroessen(t *testing.T) {
	layers := repoDefaultsLayers(t)
	for _, kern := range docsKerne {
		got := mustResolveDoc(t, layers, "k-playbook:/commands/"+kern)
		if len(got.Problems) != 0 {
			t.Errorf("%s: Probleme beim Auflösen: %v", kern, got.Problems)
		}
		t.Logf("%s aufgelöst %d Bytes", kern, len(got.Content))
		if n := len(got.Content); n > docsKernMaxBytes-docsLuft {
			t.Errorf("%s aufgelöst %d Bytes, Grenze %d mit %d Bytes Luft", kern, n, docsKernMaxBytes, docsLuft)
		}
	}
	for _, module := range taskModulDateien(t, docsModuleDirs...) {
		got := mustResolveDoc(t, layers, "k-playbook:/commands/"+module)
		if len(got.Problems) != 0 {
			t.Errorf("%s: Probleme beim Auflösen: %v", module, got.Problems)
		}
		t.Logf("%s aufgelöst %d Bytes", module, len(got.Content))
		if n := len(got.Content); n > docsModulMaxBytes-docsLuft {
			t.Errorf("%s aufgelöst %d Bytes, Grenze %d mit %d Bytes Luft", module, n, docsModulMaxBytes, docsLuft)
		}
	}
}

// Jeder Verweis `k-playbook:/commands/_…` und jede Include-Zeile in den vier
// Kernen und ihren Modulen löst auf und trägt title und description; jede
// Datei der Modulverzeichnisse wird von mindestens einem Text genannt oder
// eingebunden.
func TestDocsCommandsModulverweiseLoesenAuf(t *testing.T) {
	layers := repoDefaultsLayers(t)
	re := regexp.MustCompile(`k-playbook:/commands/(_[a-z-]+/[a-z-]+\.md)`)
	includeRe := regexp.MustCompile(`(?m)^include: (?:k-playbook:/commands/)?(_[a-z-]+/[a-z-]+\.md)$`)

	texts := append([]string{}, docsKerne...)
	texts = append(texts, taskModulDateien(t, docsModuleDirs...)...)
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
	for _, module := range taskModulDateien(t, docsModuleDirs...) {
		if !genannt[module] {
			t.Errorf("%s wird von keinem Text der Docs-Commands genannt oder eingebunden", module)
		}
	}
}

// Jeder Kern nennt die Module, die er an seinen Weichen liest, mit ihrem
// Verweis; die Module für beide Erzeuger stehen unter _docs/.
func TestDocsCommandsKerneNennenIhreModule(t *testing.T) {
	want := map[string][]string{
		"k-docs-code.md": {"_docs/scope.md", "_docs/auftraege.md", "_docs/publish.md", "_docs-code/analyse.md", "_docs-code/schreiben.md"},
		"k-docs-tools.md": {"_docs/scope.md", "_docs/auftraege.md", "_docs/publish.md", "_docs-tools/kandidaten.md",
			"_docs-tools/schreiben.md"},
		"k-docs.md":          nil,
		"k-doc-inventory.md": nil,
	}
	for kern, modules := range want {
		raw := readRepoDefault(t, "commands/"+kern)
		for _, module := range modules {
			if !strings.Contains(raw, "k-playbook:/commands/"+module) {
				t.Errorf("%s nennt das Modul %s nicht", kern, module)
			}
		}
	}
	// _docs/schreiblauf.md ist Sache der beiden Schreibmodule (und der Ablage), nicht der Kerne.
	for _, module := range []string{"_docs-code/schreiben.md", "_docs-tools/schreiben.md"} {
		if !strings.Contains(readRepoDefault(t, "commands/"+module), "include: k-playbook:/commands/_docs/schreiblauf.md") {
			t.Errorf("%s bindet _docs/schreiblauf.md nicht ein", module)
		}
	}
	if !strings.Contains(readRepoDefault(t, "commands/_shared/docs.md"), "k-playbook:/commands/_docs/schreiblauf.md") {
		t.Error("_shared/docs.md verweist nicht auf _docs/schreiblauf.md")
	}
	// /k-doc-inventory braucht von _shared/docs.md nur Ablage und Kopf und nimmt
	// weder Scope noch Aufträge.
	inv := readRepoDefault(t, "commands/k-doc-inventory.md")
	for _, stale := range []string{"_docs/scope.md", "_docs/auftraege.md", "docs scope", "docs orders"} {
		if strings.Contains(inv, stale) {
			t.Errorf("k-doc-inventory.md nennt %q", stale)
		}
	}
	if !strings.Contains(inv, "include: _shared/inventory-config.md") {
		t.Error("k-doc-inventory.md bindet inventory-config.md nicht mehr ein")
	}
}

// Kein Text nennt Schrittnummern eines anderen Commands, und ein Modul trägt
// gar keine Schrittnummer: Schritte gibt es nur im Kern. Die Schrittnummern
// der vier Commands selbst bleiben.
func TestDocsCommandsKeineFremdenSchrittnummern(t *testing.T) {
	commands := []string{"k-docs-code", "k-docs-tools", "k-docs", "k-doc-inventory"}
	own := map[string][]string{
		"k-docs-code.md":     taskModulDateien(t, "_docs-code"),
		"k-docs-tools.md":    taskModulDateien(t, "_docs-tools"),
		"k-docs.md":          nil,
		"k-doc-inventory.md": nil,
	}
	step := regexp.MustCompile(`Schritte? \d`)
	for _, kern := range docsKerne {
		self := strings.TrimSuffix(kern, ".md")
		for _, text := range append([]string{kern}, own[kern]...) {
			raw := readRepoDefault(t, "commands/"+text)
			for _, other := range commands {
				if other == self {
					continue
				}
				// `/k-docs` darf in einem Satz mit `/k-docs-code` stehen; gemeint ist der volle Name.
				nennt := regexp.MustCompile("(?i)(`/" + regexp.QuoteMeta(other) + "`|k-playbook:/commands/" + regexp.QuoteMeta(other) +
					`\.md)[^.\n]{0,60}(Schritte? \d)|(Schritte? \d+)[^.\n]{0,40}(von|des) ` + "`/" + regexp.QuoteMeta(other) + "`")
				if m := nennt.FindString(raw); m != "" {
					t.Errorf("%s nennt Schritte von /%s: %q", text, other, m)
				}
			}
		}
	}
	for _, module := range taskModulDateien(t, docsModuleDirs...) {
		raw := readRepoDefault(t, "commands/"+module)
		if m := step.FindString(raw); m != "" {
			t.Errorf("%s nennt eine Schrittnummer (%q); Schrittnummern gibt es nur im Kern", module, m)
		}
	}
}

// Ein Sub-Agenten-Modul ist mit seinen Includes für sich vollständig: Es
// verweist auf keinen Schritt und keinen Kern, trägt die Regeln des
// schreibenden Laufs und sagt, dass der Sub-Agent weder publish noch Befunde
// noch Aufträge schreibt.
func TestDocsCommandsSubAgentenModuleSindFuerSichLesbar(t *testing.T) {
	layers := repoDefaultsLayers(t)
	schritt := regexp.MustCompile(`(?m)\bSchritte? \d`)
	for _, module := range []string{"_docs-code/schreiben.md", "_docs-tools/schreiben.md"} {
		got := mustResolveDoc(t, layers, "k-playbook:/commands/"+module)
		if len(got.Problems) != 0 || len(got.Includes) != 1 {
			t.Errorf("%s: problems %v, includes %+v", module, got.Problems, got.Includes)
		}
		if m := schritt.FindString(got.Content); m != "" {
			t.Errorf("%s verweist auf einen Schritt des Kerns: %q", module, m)
		}
		for _, kern := range docsKerne {
			if strings.Contains(got.Content, "k-playbook:/commands/"+kern) {
				t.Errorf("%s verweist auf den Kern %s", module, kern)
			}
		}
		for _, want := range []string{"**Zeitpunkt:**", "**Unklarheiten**", "**Keine Befunde.**", "**Sub-Agenten**",
			"`GENERATED_AT`", "`WORK_DIR`", "kein `publish`", "`generated`"} {
			if !strings.Contains(got.Content, want) {
				t.Errorf("%s ohne %q", module, want)
			}
		}
		if strings.Contains(got.Content, "\ninclude: ") {
			t.Errorf("%s trägt eine nicht eingesetzte Include-Zeile", module)
		}
	}
}

// Fehlt ein Kommando oder Werkzeug, melden die Texte es, nennen /k-upgrade und
// ersetzen nichts von Hand.
func TestDocsCommandsMeldenFehlendeKommandos(t *testing.T) {
	for _, kern := range []string{"k-docs-code.md", "k-docs-tools.md", "k-docs.md"} {
		bundle := docsCommandBundle(t, kern, "_docs", "_docs-code", "_docs-tools")
		if !strings.Contains(bundle, "`/k-upgrade`") || !strings.Contains(bundle, "nichts von Hand") {
			t.Errorf("%s nennt nicht /k-upgrade und „nichts von Hand“", kern)
		}
	}
	for _, rel := range []string{"commands/k-docs-code.md", "commands/k-docs-tools.md"} {
		if !strings.Contains(readRepoDefault(t, rel), "`k_playbook_docs_scope`") && !strings.Contains(readRepoDefault(t, rel), "_docs/scope.md") {
			t.Errorf("%s ruft den Scope nicht über das Kommando", rel)
		}
	}
}
