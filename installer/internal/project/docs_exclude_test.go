package project

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// readDocsConfig schreibt content als K-PLAYBOOK.yaml und liest sie zurück.
func readDocsConfig(t *testing.T, content string) Config {
	t.Helper()
	root := t.TempDir()
	writeFile(t, ConfigPath(root), content)
	config, err := ReadConfig(root)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	return config
}

// Ohne Block ändert sich nichts: die Liste ist leer, ohne Problem, und die
// Kontextausgabe trägt leere Listen statt null.
func TestReadConfigOhneDocsBlock(t *testing.T) {
	config := readDocsConfig(t, "schema_version: 3\n\nproject:\n  repo_root: .\n  vcs: git\n")
	if len(config.DocsExclude) != 0 || len(config.DocsProblems) != 0 {
		t.Errorf("DocsExclude = %v, DocsProblems = %v, erwartet beide leer", config.DocsExclude, config.DocsProblems)
	}

	data, err := json.Marshal(contextDocs(config))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got, want := string(data), `{"exclude":[],"problems":[]}`; got != want {
		t.Errorf("context.docs = %s, erwartet %s", got, want)
	}
}

func TestReadConfigLeereDocsListe(t *testing.T) {
	for name, content := range map[string]string{
		"ohne Einträge": "schema_version: 3\n\ndocs:\n  exclude:\n",
		"nur Kommentar": "schema_version: 3\n\ndocs:\n  exclude:\n    # - docs/\n",
		"leere Zeilen":  "schema_version: 3\n\ndocs:\n  exclude:\n    -\n    - \"\"\n",
		"Flussform []":  "schema_version: 3\n\ndocs:\n  exclude: []\n",
	} {
		config := readDocsConfig(t, content)
		if len(config.DocsExclude) != 0 || len(config.DocsProblems) != 0 {
			t.Errorf("%s: DocsExclude = %v, DocsProblems = %v", name, config.DocsExclude, config.DocsProblems)
		}
	}
}

// Mehrere Einträge mit Kommentaren dazwischen, unbekannte Blöcke davor und
// dahinter, und die Skalare in project: stimmen weiter.
func TestReadConfigLiestDocsExclude(t *testing.T) {
	config := readDocsConfig(t, `# Kopf
schema_version: 3

project:
  repo_root: G
  vcs: git

fremd:
  exclude:
    - nicht/das/
docs:
  # Vom Code-Scan ausgenommen.
  exclude:
    - docs/
    # Kopien aus defaults/
    - vendor/

    - "guidelines/"   # Kommentar hinter dem Eintrag
    - reviews/ # auch so
    - docs/
  include:
    - .devcontainer/

tools:
  gh:
    status: enabled
`)
	if want := []string{"docs/", "vendor/", "guidelines/", "reviews/"}; !reflect.DeepEqual(config.DocsExclude, want) {
		t.Errorf("DocsExclude = %v, erwartet %v", config.DocsExclude, want)
	}
	if len(config.DocsProblems) != 0 {
		t.Errorf("DocsProblems = %v, erwartet leer", config.DocsProblems)
	}
	if config.SchemaVersion != "3" || config.RepoRoot != "G" || config.VCS != "git" {
		t.Errorf("Skalare = %q %q %q", config.SchemaVersion, config.RepoRoot, config.VCS)
	}
}

// Absolute Einträge und solche, die mit .. aus project.repo_root führen, kommen
// nicht in die Liste, sondern als Problem mit Grund.
func TestReadConfigVerwirftDocsEintraege(t *testing.T) {
	config := readDocsConfig(t, `schema_version: 3
docs:
  exclude:
    - /home/kleist/dev/x
    - ../nachbar/
    - a/../../b
    - ~/privat
    - 'C:\daten'
    - "!../zurueck"
    - gut/
    - a/../b
`)
	if want := []string{"gut/", "a/../b"}; !reflect.DeepEqual(config.DocsExclude, want) {
		t.Errorf("DocsExclude = %v, erwartet %v", config.DocsExclude, want)
	}
	rejected := []string{}
	for _, problem := range config.DocsProblems {
		if problem.Reason == "" {
			t.Errorf("Problem ohne Grund: %+v", problem)
		}
		rejected = append(rejected, problem.Pattern)
	}
	if want := []string{"/home/kleist/dev/x", "../nachbar/", "a/../../b", "~/privat", `C:\daten`, "!../zurueck"}; !reflect.DeepEqual(rejected, want) {
		t.Errorf("verworfen = %v, erwartet %v", rejected, want)
	}
}

// Die Flussform wird nicht gelesen, aber gemeldet — still übergangen sähe sie
// aus wie keine Liste.
func TestReadConfigMeldetDocsFlussform(t *testing.T) {
	config := readDocsConfig(t, "schema_version: 3\ndocs:\n  exclude: [docs/, vendor/]\n")
	if len(config.DocsExclude) != 0 {
		t.Errorf("DocsExclude = %v, erwartet leer", config.DocsExclude)
	}
	if len(config.DocsProblems) != 1 || !strings.Contains(config.DocsProblems[0].Reason, "- <muster>") {
		t.Errorf("DocsProblems = %+v, erwartet ein Problem zur Flussform", config.DocsProblems)
	}
}

// Ein exclude, das nicht direktes Kind von docs: ist, zählt nicht.
func TestReadConfigDocsNurDirektesKind(t *testing.T) {
	config := readDocsConfig(t, `schema_version: 3
docs:
  andere:
    exclude:
      - falsch/
  exclude:
    - richtig/
`)
	if want := []string{"richtig/"}; !reflect.DeepEqual(config.DocsExclude, want) {
		t.Errorf("DocsExclude = %v, erwartet %v", config.DocsExclude, want)
	}
}

// Die Kontextausgabe trägt die Liste und die Probleme unter docs.
func TestBuildContextTraegtDocs(t *testing.T) {
	root := newContextProject(t)
	writeFile(t, ConfigPath(root), "schema_version: 3\n\nproject:\n  repo_root: .\n  vcs: git\n\ndocs:\n  exclude:\n    - docs/\n    - ../weg\n")

	built, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if want := []string{"docs/"}; !reflect.DeepEqual(built.Docs.Exclude, want) {
		t.Errorf("Docs.Exclude = %v, erwartet %v", built.Docs.Exclude, want)
	}
	if len(built.Docs.Problems) != 1 || built.Docs.Problems[0].Pattern != "../weg" {
		t.Errorf("Docs.Problems = %+v", built.Docs.Problems)
	}

	data, err := json.Marshal(built)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"docs":{"exclude":["docs/"],"problems":[{"pattern":"../weg","reason":`) {
		t.Errorf("JSON ohne docs-Abschnitt: %s", data)
	}
}

// Eine neue Konfiguration trägt den Block nur als Kommentar mit Beispiel: Sie
// lässt sich lesen, docs.exclude ist leer, und die übrigen Werte stimmen.
func TestCreateConfigSchreibtDocsAlsKommentar(t *testing.T) {
	root := t.TempDir()
	if err := CreateConfig(root, "."); err != nil {
		t.Fatalf("CreateConfig: %v", err)
	}
	data, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		t.Fatalf("lesen: %v", err)
	}
	if !strings.Contains(string(data), "\n# docs:\n#   # Vom Code-Scan") || !strings.Contains(string(data), "#     - docs/\n") {
		t.Errorf("kommentierter Block fehlt:\n%s", data)
	}
	if strings.Contains(string(data), "\ndocs:") {
		t.Errorf("Block steht nicht nur als Kommentar da:\n%s", data)
	}
	// Die Vorlage legt keinen Block tools.gh mehr an, und ohne Block darunter
	// auch keine Zeile tools: (Task 128).
	if strings.Contains(string(data), "\ntools:") || strings.Contains(string(data), "  gh:") {
		t.Errorf("Vorlage schreibt tools: oder gh::\n%s", data)
	}

	config, err := ReadConfig(root)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if err := CheckSchema(config); err != nil {
		t.Errorf("CheckSchema: %v", err)
	}
	if len(config.DocsExclude) != 0 || len(config.DocsProblems) != 0 {
		t.Errorf("DocsExclude = %v, DocsProblems = %v, erwartet leer", config.DocsExclude, config.DocsProblems)
	}
	if config.RepoRoot != "." {
		t.Errorf("RepoRoot = %q", config.RepoRoot)
	}
	built, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if built.Docs.Exclude == nil || len(built.Docs.Exclude) != 0 {
		t.Errorf("context.docs.exclude = %#v, erwartet []", built.Docs.Exclude)
	}
}

// Das Umschalten der Remediation ersetzt seinen Block, lässt den
// auskommentierten Block docs: dahinter aber stehen.
func TestSetRemediationModeLaesstDocsKommentarStehen(t *testing.T) {
	root := t.TempDir()
	if err := CreateConfig(root, "."); err != nil {
		t.Fatalf("CreateConfig: %v", err)
	}
	if err := SetRemediationMode(root, ModeTaskBranchPR); err != nil {
		t.Fatalf("SetRemediationMode: %v", err)
	}
	data, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		t.Fatalf("lesen: %v", err)
	}
	if !strings.Contains(string(data), docsBlockExample) {
		t.Errorf("kommentierter Block docs: ist weg:\n%s", data)
	}
	remediation, err := ReadRemediation(root)
	if err != nil || remediation.Mode != ModeTaskBranchPR {
		t.Errorf("ReadRemediation = %+v, %v", remediation, err)
	}
}

// readConfigFile liest K-PLAYBOOK.yaml als Text.
func readConfigFile(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		t.Fatalf("lesen: %v", err)
	}
	return string(data)
}

// In einer neu angelegten Datei fehlt der Block (nur das kommentierte
// Beispiel steht da); er wird hinten angelegt, der Rest bleibt byte-gleich,
// und ReadConfig liest die Liste danach.
func TestAddDocsExcludeLegtBlockInNeuerDateiAn(t *testing.T) {
	root := t.TempDir()
	if err := CreateConfig(root, "."); err != nil {
		t.Fatalf("CreateConfig: %v", err)
	}
	before := readConfigFile(t, root)

	result, err := AddDocsExclude(root, []string{"docs/", " vendor/ "})
	if err != nil {
		t.Fatalf("AddDocsExclude: %v", err)
	}
	if !result.Changed || !reflect.DeepEqual(result.Added, []string{"docs/", "vendor/"}) ||
		!reflect.DeepEqual(result.Exclude, []string{"docs/", "vendor/"}) || len(result.Rejected) != 0 {
		t.Errorf("Antwort = %+v", result)
	}
	after := readConfigFile(t, root)
	if !strings.HasPrefix(after, strings.TrimRight(before, "\n")+"\n\n") {
		t.Errorf("Text vor dem neuen Block geändert:\n%s", after)
	}
	if !strings.HasSuffix(after, "\ndocs:\n"+docsBlockComment+"  exclude:\n    - docs/\n    - vendor/\n") {
		t.Errorf("Block nicht hinten angelegt:\n%s", after)
	}
	if !strings.Contains(after, docsBlockExample) {
		t.Errorf("kommentiertes Beispiel verschwunden:\n%s", after)
	}

	config, err := ReadConfig(root)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if !reflect.DeepEqual(config.DocsExclude, []string{"docs/", "vendor/"}) || len(config.DocsProblems) != 0 {
		t.Errorf("ReadConfig: %v %v", config.DocsExclude, config.DocsProblems)
	}
	if config.RepoRoot != "." || config.VCS == "" {
		t.Errorf("Skalare: %+v", config)
	}
}

// Steht der Block schon da, kommen neue Einträge hinten dazu; ein Duplikat
// wird übersprungen, ein verworfenes Muster gemeldet und nicht geschrieben,
// und der Text vor und hinter dem Block bleibt byte-gleich. Der Block selbst
// wird nicht neu gerendert: Nur die neuen Zeilen kommen dazu.
func TestAddDocsExcludeErgaenztVorhandenenBlock(t *testing.T) {
	root := t.TempDir()
	head := "# Kopf\nschema_version: 3\n\nproject:\n  repo_root: .\n  vcs: git\n\n# vor docs\n"
	tail := "\n# Kommentar vor tools\ntools:\n  gh:\n    status: enabled\n"
	writeFile(t, ConfigPath(root), head+"docs:\n  exclude:\n    - docs/\n"+tail)

	result, err := AddDocsExclude(root, []string{"docs/", "../weg", "vendor/", "/abs", "vendor/"})
	if err != nil {
		t.Fatalf("AddDocsExclude: %v", err)
	}
	if !reflect.DeepEqual(result.Added, []string{"vendor/"}) || !reflect.DeepEqual(result.Exclude, []string{"docs/", "vendor/"}) {
		t.Errorf("Antwort = %+v", result)
	}
	rejected := []string{}
	for _, problem := range result.Rejected {
		if problem.Reason == "" {
			t.Errorf("verworfen ohne Grund: %+v", problem)
		}
		rejected = append(rejected, problem.Pattern)
	}
	if !reflect.DeepEqual(rejected, []string{"../weg", "/abs"}) {
		t.Errorf("Rejected = %+v", result.Rejected)
	}

	after := readConfigFile(t, root)
	want := head + "docs:\n  exclude:\n    - docs/\n    - vendor/\n" + tail
	if after != want {
		t.Errorf("Datei danach:\n%s\nerwartet:\n%s", after, want)
	}
	if strings.Contains(after, "weg") || strings.Contains(after, "/abs") {
		t.Errorf("verworfenes Muster geschrieben:\n%s", after)
	}
	config, err := ReadConfig(root)
	if err != nil || !reflect.DeepEqual(config.DocsExclude, []string{"docs/", "vendor/"}) {
		t.Errorf("ReadConfig: %v %v", config.DocsExclude, err)
	}
}

// Ist nichts neu — nur Duplikate oder nur Verworfenes —, bleibt die Datei
// byte-gleich, auch ihre Schreibweise.
func TestAddDocsExcludeOhneNeuesBleibtDateiGleich(t *testing.T) {
	root := t.TempDir()
	content := "schema_version: 3\ndocs:   # eigener Kommentar\n  exclude:\n    - \"docs/\"   # Handdoku\n    - ../alt\n"
	writeFile(t, ConfigPath(root), content)

	result, err := AddDocsExclude(root, []string{"docs/", "/abs", "../noch"})
	if err != nil {
		t.Fatalf("AddDocsExclude: %v", err)
	}
	if result.Changed || len(result.Added) != 0 || len(result.Rejected) != 2 || !reflect.DeepEqual(result.Exclude, []string{"docs/"}) {
		t.Errorf("Antwort = %+v", result)
	}
	if after := readConfigFile(t, root); after != content {
		t.Errorf("Datei geändert:\n%s", after)
	}
}

// Ein Kopf mit Kommentar wird als derselbe Block erkannt, nicht ein zweiter
// angehängt; schon verworfene Einträge bleiben stehen, statt still zu
// verschwinden.
func TestAddDocsExcludeErkenntBlockMitKommentar(t *testing.T) {
	root := t.TempDir()
	writeFile(t, ConfigPath(root), "schema_version: 3\ndocs:   # eigener Kommentar\n  exclude:\n    - docs/\n    - ../alt\n")

	if _, err := AddDocsExclude(root, []string{"vendor/"}); err != nil {
		t.Fatalf("AddDocsExclude: %v", err)
	}
	after := readConfigFile(t, root)
	if strings.Count(after, "docs:") != 1 {
		t.Errorf("zweiter Block docs: angelegt:\n%s", after)
	}
	config, err := ReadConfig(root)
	if err != nil || !reflect.DeepEqual(config.DocsExclude, []string{"docs/", "vendor/"}) ||
		len(config.DocsProblems) != 1 || config.DocsProblems[0].Pattern != "../alt" {
		t.Errorf("ReadConfig: %v %+v %v\n%s", config.DocsExclude, config.DocsProblems, err, after)
	}
}

// Ohne Muster ist das invalid_input, und nichts wird geschrieben.
func TestAddDocsExcludeOhneMuster(t *testing.T) {
	root := t.TempDir()
	content := "schema_version: 3\n"
	writeFile(t, ConfigPath(root), content)
	for _, patterns := range [][]string{nil, {}, {" ", ""}} {
		_, err := AddDocsExclude(root, patterns)
		if DocRefErrorCode(err) != DocCodeInvalidInput {
			t.Errorf("%q: %v, erwartet invalid_input", patterns, err)
		}
	}
	if after := readConfigFile(t, root); after != content {
		t.Errorf("Datei geändert:\n%s", after)
	}
}

// Ein Muster mit Zeichen, die YAML anders läse, wird in Anführungszeichen
// geschrieben und genau so zurückgelesen.
func TestAddDocsExcludeQuotetSonderzeichen(t *testing.T) {
	root := t.TempDir()
	writeFile(t, ConfigPath(root), "schema_version: 3\n")
	patterns := []string{"*.gen.go", "!keep/", "a #b", `say "x"`, "#kommentar", "'x' y", "a\\"}
	if _, err := AddDocsExclude(root, patterns); err != nil {
		t.Fatalf("AddDocsExclude: %v", err)
	}
	config, err := ReadConfig(root)
	if err != nil || !reflect.DeepEqual(config.DocsExclude, patterns) {
		t.Errorf("ReadConfig: %q %v\n%s", config.DocsExclude, err, readConfigFile(t, root))
	}
}

// addDocsExcludeTo schreibt content als K-PLAYBOOK.yaml, trägt patterns ein und
// gibt Antwort, Fehler und die Datei danach zurück.
func addDocsExcludeTo(t *testing.T, content string, patterns ...string) (DocsExcludeResult, error, string) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, ConfigPath(root), content)
	result, err := AddDocsExclude(root, patterns)
	return result, err, readConfigFile(t, root)
}

// Nachbarschlüssel im Block (include:), Kommentare im Block, vorhandene und
// schon verworfene Einträge bleiben byte-gleich stehen; nur die neuen Zeilen
// kommen hinter den letzten Eintrag der Liste. Der Text außerhalb des Blocks
// bleibt ebenso byte-gleich, auch Kommentare ohne Einrückung am Blockende.
func TestAddDocsExcludeBehaeltNachbarnUndKommentare(t *testing.T) {
	head := "# Kopf\nschema_version: 3\n\nproject:\n  repo_root: .\n  vcs: git\n\n"
	before := "docs:   # Kopf-Kommentar\n" +
		"  # Kommentar im Block\n" +
		"  include:\n" +
		"    - .devcontainer/   # gebraucht\n" +
		"  exclude:\n" +
		"    - \"docs/\"   # Handdoku\n" +
		"    # Kopien aus defaults/\n" +
		"    - vendor/\n" +
		"    - ../alt\n"
	after := "    # Kommentar hinter der Liste\n" +
		"  weiteres: wert   # Nachbar dahinter\n" +
		"# Kommentar ohne Einrückung\n" +
		"\n" +
		"tools:\n  gh:\n    status: enabled\n"

	result, err, got := addDocsExcludeTo(t, head+before+after, "guidelines/", "docs/", "reviews/")
	if err != nil {
		t.Fatalf("AddDocsExclude: %v", err)
	}
	if !result.Changed || !reflect.DeepEqual(result.Added, []string{"guidelines/", "reviews/"}) ||
		!reflect.DeepEqual(result.Exclude, []string{"docs/", "vendor/", "guidelines/", "reviews/"}) {
		t.Errorf("Antwort = %+v", result)
	}
	if want := head + before + "    - guidelines/\n    - reviews/\n" + after; got != want {
		t.Errorf("Datei danach:\n%s\nerwartet:\n%s", got, want)
	}
	config, err := ReadConfig(projectOf(t, got))
	if err != nil || !reflect.DeepEqual(config.DocsExclude, result.Exclude) ||
		len(config.DocsProblems) != 1 || config.DocsProblems[0].Pattern != "../alt" {
		t.Errorf("ReadConfig: %v %+v %v", config.DocsExclude, config.DocsProblems, err)
	}
}

// projectOf legt ein Projekt mit content als K-PLAYBOOK.yaml an.
func projectOf(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, ConfigPath(root), content)
	return root
}

// Die Liste ohne Einrückung (Einträge in derselben Spalte wie exclude:) ist
// gültiges YAML: Sie wird gelesen, und neue Einträge kommen in derselben
// Spalte dazu, ohne dass ein vorhandener verloren geht oder include: dahinter
// sich ändert.
func TestDocsExcludeListeOhneEinrueckung(t *testing.T) {
	head := "schema_version: 3\n\n"
	before := "docs:\n  exclude:\n  - docs/\n  # Kommentar\n  - vendor/\n"
	after := "  include:\n  - .devcontainer/\n\ntools:\n  gh:\n    status: enabled\n"

	config := readDocsConfig(t, head+before+after)
	if want := []string{"docs/", "vendor/"}; !reflect.DeepEqual(config.DocsExclude, want) || len(config.DocsProblems) != 0 {
		t.Errorf("ReadConfig: %v %+v, erwartet %v", config.DocsExclude, config.DocsProblems, want)
	}

	result, err, got := addDocsExcludeTo(t, head+before+after, "reviews/")
	if err != nil {
		t.Fatalf("AddDocsExclude: %v", err)
	}
	if !reflect.DeepEqual(result.Exclude, []string{"docs/", "vendor/", "reviews/"}) {
		t.Errorf("Antwort = %+v", result)
	}
	if want := head + before + "  - reviews/\n" + after; got != want {
		t.Errorf("Datei danach:\n%s\nerwartet:\n%s", got, want)
	}
	config, err = ReadConfig(projectOf(t, got))
	if err != nil || !reflect.DeepEqual(config.DocsExclude, []string{"docs/", "vendor/", "reviews/"}) {
		t.Errorf("ReadConfig danach: %v %v", config.DocsExclude, err)
	}
}

// Ein Schlüssel, der mit - beginnt, ist in der Spalte von exclude: kein
// Eintrag der Liste ohne Einrückung, sondern beendet sie.
func TestDocsExcludeListeOhneEinrueckungEndetAmSchluessel(t *testing.T) {
	config := readDocsConfig(t, "schema_version: 3\ndocs:\n  exclude:\n  - docs/\n  -x: y\n  - nicht/\n")
	if want := []string{"docs/"}; !reflect.DeepEqual(config.DocsExclude, want) {
		t.Errorf("DocsExclude = %v, erwartet %v", config.DocsExclude, want)
	}
}

// Die Flussform und andere Formen, die sich nicht sicher ergänzen lassen,
// werden nicht umgeschrieben: unsupported mit Grund, die Datei bleibt
// byte-gleich, und die Antwort nennt nichts als eingetragen.
func TestAddDocsExcludeVerweigertUnsichereFormen(t *testing.T) {
	for name, content := range map[string]string{
		"Flussform":             "schema_version: 3\ndocs:\n  exclude: [docs/, vendor/]\n  include:\n    - x/\n",
		"Flussform mit Komm.":   "schema_version: 3\ndocs:\n  exclude: [docs/]   # Handdoku\n",
		"Skalar":                "schema_version: 3\ndocs:\n  exclude: docs/\n",
		"docs in einer Zeile":   "schema_version: 3\ndocs: {exclude: [docs/]}\n",
		"docs als Liste":        "schema_version: 3\ndocs:\n  - docs/\n",
		"exclude doppelt":       "schema_version: 3\ndocs:\n  exclude:\n    - a/\n  exclude:\n    - b/\n",
		"docs doppelt":          "schema_version: 3\ndocs:\n  exclude:\n    - a/\ndocs:\n  include: []\n",
		"Zuordnung statt Liste": "schema_version: 3\ndocs:\n  exclude:\n    pfad: docs/\n",
		"Spalten gemischt":      "schema_version: 3\ndocs:\n  exclude:\n    - a/\n      - b/\n",
		"Fortsetzungszeile":     "schema_version: 3\ndocs:\n  exclude:\n    - a\n      b/\n",
	} {
		result, err, got := addDocsExcludeTo(t, content, "neu/")
		if DocRefErrorCode(err) != DocCodeUnsupported || !strings.Contains(err.Error(), "nichts eingetragen") {
			t.Errorf("%s: Fehler = %v, erwartet unsupported mit Grund", name, err)
		}
		if got != content {
			t.Errorf("%s: Datei geändert:\n%s", name, got)
		}
		if result.Changed || len(result.Added) != 0 || containsString(result.Exclude, "neu/") {
			t.Errorf("%s: Antwort = %+v", name, result)
		}
	}
}

// Die leere Flussliste `exclude: []` wird verlustfrei zur Blockliste: `[]`
// fällt weg, ein Kommentar dahinter und der Rest des Blocks bleiben stehen.
func TestAddDocsExcludeLeereFlusslisteWirdBlockliste(t *testing.T) {
	for name, tc := range map[string]struct{ before, after string }{
		"ohne Kommentar": {
			"schema_version: 3\ndocs:\n  exclude: []\n  include:\n    - x/\n",
			"schema_version: 3\ndocs:\n  exclude:\n    - neu/\n  include:\n    - x/\n",
		},
		"mit Kommentar": {
			"schema_version: 3\ndocs:\n  exclude: []   # leer\n",
			"schema_version: 3\ndocs:\n  exclude:   # leer\n    - neu/\n",
		},
	} {
		_, err, got := addDocsExcludeTo(t, tc.before, "neu/")
		if err != nil {
			t.Fatalf("%s: AddDocsExclude: %v", name, err)
		}
		if got != tc.after {
			t.Errorf("%s: Datei danach:\n%s\nerwartet:\n%s", name, got, tc.after)
		}
		config := readDocsConfig(t, got)
		if !reflect.DeepEqual(config.DocsExclude, []string{"neu/"}) {
			t.Errorf("%s: ReadConfig: %v", name, config.DocsExclude)
		}
	}
}

// Ein Block docs: ohne exclude: bekommt den Schlüssel hinter seiner letzten
// eingerückten Zeile; include: und Kommentare bleiben, und ein Kommentar ohne
// Einrückung am Ende bleibt bei dem, was folgt.
func TestAddDocsExcludeLegtSchluesselImVorhandenenBlockAn(t *testing.T) {
	before := "schema_version: 3\ndocs:\n  # nur include\n  include:\n    - x/\n"
	after := "# zu tools\ntools:\n  gh:\n    status: enabled\n"
	_, err, got := addDocsExcludeTo(t, before+after, "neu/")
	if err != nil {
		t.Fatalf("AddDocsExclude: %v", err)
	}
	if want := before + "  exclude:\n    - neu/\n" + after; got != want {
		t.Errorf("Datei danach:\n%s\nerwartet:\n%s", got, want)
	}
	if config := readDocsConfig(t, got); !reflect.DeepEqual(config.DocsExclude, []string{"neu/"}) {
		t.Errorf("ReadConfig: %v", config.DocsExclude)
	}

	// Leerer Block docs: und exclude: ohne Einträge.
	for name, tc := range map[string]struct{ before, after string }{
		"docs leer":       {"docs:\ntools: {}\n", "docs:\n  exclude:\n    - neu/\ntools: {}\n"},
		"exclude leer":    {"docs:\n  exclude:\n  include: []\n", "docs:\n  exclude:\n    - neu/\n  include: []\n"},
		"ohne Zeilenende": {"docs:\n  exclude:\n    - a/", "docs:\n  exclude:\n    - a/\n    - neu/"},
		"CRLF":            {"docs:\r\n  exclude:\r\n    - a/\r\nx: 1\r\n", "docs:\r\n  exclude:\r\n    - a/\r\n    - neu/\r\nx: 1\r\n"},
	} {
		_, err, got := addDocsExcludeTo(t, tc.before, "neu/")
		if err != nil {
			t.Fatalf("%s: AddDocsExclude: %v", name, err)
		}
		if got != tc.after {
			t.Errorf("%s: Datei danach:\n%q\nerwartet:\n%q", name, got, tc.after)
		}
	}
}

// docsExcludeSteuerzeichen sind Muster, die nicht in einer Zeile stehen
// können: Zeilenumbruch, Wagenrücklauf, Tabulator, NUL, DEL, NEL und die
// Unicode-Zeilen- und Absatztrenner. Ein Zeilenumbruch landete roh eingefügt
// als eigene Zeile außerhalb von docs:.
var docsExcludeSteuerzeichen = []string{"a\nd", "a\rd", "a\td", "a\x00b", "\x7f", "a\u0085b", "a\u2028b", "a\u2029b"}

// Ein Muster mit Steuerzeichen wird verworfen wie ein absolutes, im Pfad
// „Block fehlt" (docsBlock) wie im Pfad „Block vorhanden": Die Datei trägt
// außerhalb des Blocks byte-gleich nur das gültige Muster dazu, und die
// gemeldete Liste ist die, die ReadConfig danach liest.
func TestAddDocsExcludeVerwirftSteuerzeichen(t *testing.T) {
	head, tail := "schema_version: 3\n", "tail: x\n"
	block := "docs:\n  exclude:\n    - keep/\n"
	for _, tc := range []struct {
		name, content, want string
		exclude             []string
	}{
		{"Block fehlt", head + tail, head + tail + "\n" + docsBlock([]string{"neu/"}), []string{"neu/"}},
		{"Block vorhanden", head + block + tail, head + block + "    - neu/\n" + tail, []string{"keep/", "neu/"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			patterns := append([]string{"neu/"}, docsExcludeSteuerzeichen...)
			result, err, got := addDocsExcludeTo(t, tc.content, patterns...)
			if err != nil {
				t.Fatalf("AddDocsExclude: %v", err)
			}
			if got != tc.want {
				t.Errorf("Datei danach:\n%q\nerwartet:\n%q", got, tc.want)
			}
			if !result.Changed || !reflect.DeepEqual(result.Added, []string{"neu/"}) || !reflect.DeepEqual(result.Exclude, tc.exclude) {
				t.Errorf("Antwort = %+v", result)
			}
			if len(result.Rejected) != len(docsExcludeSteuerzeichen) {
				t.Fatalf("rejected = %+v", result.Rejected)
			}
			for index, problem := range result.Rejected {
				if problem.Pattern != docsExcludeSteuerzeichen[index] || !strings.Contains(problem.Reason, "Steuerzeichen") {
					t.Errorf("rejected[%d] = %+v", index, problem)
				}
			}
			config, err := ReadConfig(projectOf(t, got))
			if err != nil || !reflect.DeepEqual(config.DocsExclude, result.Exclude) || len(config.DocsProblems) != 0 {
				t.Errorf("ReadConfig: %q %+v %v", config.DocsExclude, config.DocsProblems, err)
			}

			// Nur Verworfenes: Die Datei bleibt byte-gleich.
			only, err, unchanged := addDocsExcludeTo(t, tc.content, docsExcludeSteuerzeichen...)
			if err != nil || only.Changed || len(only.Added) != 0 || unchanged != tc.content {
				t.Errorf("nur Steuerzeichen: %+v %v\n%q", only, err, unchanged)
			}
		})
	}
}

// Beim Lesen gilt dieselbe Regel: Ein Eintrag mit Steuerzeichen mitten in der
// Zeile oder als Escape in Anführungszeichen wird mit Grund gemeldet; eine
// Datei mit CRLF verliert dadurch nichts.
func TestReadConfigVerwirftSteuerzeichen(t *testing.T) {
	config := readDocsConfig(t, "docs:\n  exclude:\n"+
		`    - "a\nb"`+"\n"+
		`    - "a\tb"`+"\n"+
		"    - x\ty\n"+
		"    - x\ry\n"+
		"    - ok/\n")
	if !reflect.DeepEqual(config.DocsExclude, []string{"ok/"}) || len(config.DocsProblems) != 4 {
		t.Fatalf("ReadConfig: %q %+v", config.DocsExclude, config.DocsProblems)
	}
	for _, problem := range config.DocsProblems {
		if !strings.Contains(problem.Reason, "Steuerzeichen") {
			t.Errorf("Problem = %+v", problem)
		}
	}

	crlf := readDocsConfig(t, "docs:\r\n  exclude:\r\n    - docs/\r\n    - 'vendor/'\r\n")
	if !reflect.DeepEqual(crlf.DocsExclude, []string{"docs/", "vendor/"}) || len(crlf.DocsProblems) != 0 {
		t.Errorf("CRLF: %q %+v", crlf.DocsExclude, crlf.DocsProblems)
	}
}
