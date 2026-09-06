package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/versionsources"
)

// Context ist der aufgelöste Arbeitsstand eines Projekts: alles, was ein
// Command sonst selbst aus Konfiguration und Dateisystem zusammenrechnen
// müsste.
//
// Die Security-Tools fehlen bewusst — nicht, weil eine Werkzeugprüfung generell
// zu teuer wäre, sondern weil ihr Preflight je Tool einen Unterprozess startet
// und dessen --version liest. Dieser Aufruf soll billig genug sein, um am
// Anfang jedes Commands zu stehen.
//
// Die Anwesenheitsprüfung der Basis-Werkzeuge steht deshalb sehr wohl hier: sie
// schlägt je Werkzeug nur im PATH nach (exec.LookPath), ohne Unterprozess. Die
// Grenze verläuft zwischen Unterprozess und Nachschlagen, nicht zwischen
// Werkzeug und Nicht-Werkzeug; siehe BaseTools weiter unten.
type Context struct {
	SchemaVersion string `json:"schemaVersion"`
	// Now ist der Zeitpunkt dieses Aufrufs. Er steht hier, weil Commands Daten
	// in Dateien schreiben, die bleiben — Review-Logs, Ergebnisverzeichnisse.
	// Ein Assistent, dem sein Wirt kein Datum nennt, müsste sonst raten, und
	// ein geratenes Datum in einem Protokoll ist schlechter als keines.
	Now ContextNow `json:"now"`
	// Instructions sind die Dateien, die vor der Arbeit zu lesen sind, in
	// dieser Reihenfolge: erst was für alle k-playbook-Projekte gilt, dann was
	// dieses Projekt ergänzt.
	Instructions []string       `json:"instructions"`
	Project      ContextProject `json:"project"`
	Playbook     ContextDir     `json:"playbook"`
	Local        ContextDir     `json:"local"`
	Remediation  Remediation    `json:"remediation"`
	// GH ist die Entscheidung zur GitHub CLI samt Host-Befund. Anders als der
	// Security-Preflight kostet der nichts: ein Blick in den PATH und in die
	// gh-Konfiguration, kein Unterprozess und kein Netzzugriff.
	GH GH `json:"gh"`
	// BaseTools ist der Host-Befund zu den Werkzeugen, die k-playbook selbst
	// aufruft — bash, git, curl/wget, tar, python3, rg. Gemessen wird reine
	// Anwesenheit im PATH über exec.LookPath: kein Unterprozess je Werkzeug,
	// kein --version. Nur deshalb darf der Befund hier stehen und der
	// Security-Preflight nicht.
	//
	// Eine Shell-Funktion oder ein Alias wird dabei nicht gesehen. Unter Claude
	// Code ist `rg` genau das, und der Befund meldet es dort dauerhaft als
	// fehlend, obwohl der Aufruf funktioniert; die Begründung steht am Typ
	// BaseTools. Ein fehlendes Basis-Werkzeug warnt und blockiert nicht —
	// anders als gh.ready.
	BaseTools BaseTools `json:"baseTools"`
	// Cleanliness ist der lokale Zustand der Installation. Sie steht hier, weil
	// die Regel „in k-playbook/ wird nie geschrieben" sich nicht selbst
	// durchsetzt und ihr Bruch still bleibt: Ändert sich eine lokal veränderte
	// Datei upstream nicht mit, läuft `git pull` sauber durch und lässt sie
	// stehen. Zwei git-Aufrufe im lokalen Clone, ohne Netz.
	Cleanliness Cleanliness               `json:"cleanliness"`
	Catalogs    map[string][]CatalogEntry `json:"catalogs"`
	Guidelines  []string                  `json:"guidelines"`
	// VersionSources ist der Zustand der Quellenkonfiguration des
	// Versionsinventars. Sie steht hier, weil Commands Konfiguration
	// ausschließlich aus dieser Ausgabe lesen: eine Datei, die nur der Sammler
	// kennt, wäre für jeden Command unsichtbar und würde eine zweite,
	// unsichtbare Konfigurationsquelle aufmachen.
	//
	// Das Feld ist ein Zeiger mit omitempty, damit eine Installation, die es
	// noch nicht füllt, dieselbe Ausgabe erzeugt wie bisher.
	VersionSources *VersionSources `json:"versionSources,omitempty"`
	// Links meldet die Selbstheilung der Assistenten-Registrierung: was dabei
	// nachgezogen wurde und was offen blieb. Fehlt das Feld, stimmte alles —
	// eine Meldung, die bei jedem Aufruf dasselbe sagt, liest niemand mehr.
	Links *ContextLinks `json:"links,omitempty"`
}

// VersionSources ist der Zustand von k-playbook-local/version-sources.yaml in
// der Kontextausgabe. Der Vertrag dazu steht in docs/versionsinventar.md,
// Abschnitt „Quellenkonfiguration"; die Feldnamen sind die YAML-Schlüssel der
// Datei, in camelCase wie überall sonst in dieser Ausgabe.
//
// Hier steht nur das Schema. Gelesen wird die Datei von
// internal/versionsources — der einzigen Implementierung, die auch der Sammler
// des Inventars benutzt. context.go bekommt keinen zweiten Parser: zwei Leser
// derselben Datei wären zwei Auslegungen derselben Vertrauensgrenze.
type VersionSources struct {
	// Path ist der absolute Pfad der Datei — auch wenn sie fehlt, damit klar
	// ist, wo sie hingehört.
	Path string `json:"path"`
	// Present sagt, ob die Datei da ist. Fehlt sie, ist das kein Fehler: es
	// gelten die Standardquellen unterhalb der Projektwurzel.
	Present bool `json:"present"`
	// SchemaVersion ist die von der Datei deklarierte Fassung; 0, solange
	// nichts gelesen wurde.
	SchemaVersion int `json:"schemaVersion,omitempty"`
	// Roots sind die zusätzlich lesbaren Wurzeln außerhalb der Projektwurzel.
	// Die Projektwurzel selbst steht nicht darin — sie ist immer erlaubt.
	Roots []string `json:"roots,omitempty"`
	// Sources sind die konfigurierten Zusatzquellen. Sie ergänzen die
	// Standarderkennung, sie ersetzen sie nicht.
	Sources []VersionSource `json:"sources,omitempty"`
	// Exclude sind die Muster, die von der Standarderkennung übergangen werden.
	// Sie wirken nur dort: was unter Sources ausdrücklich steht, bleibt gelesen.
	Exclude []string `json:"exclude,omitempty"`
	// Error ist gesetzt, wenn die Datei da, aber nicht lesbar oder von
	// unbekannter Fassung ist. Dann sind Roots und Sources leer und der Zustand
	// ist ein sichtbarer Befund statt eines stillen Leerergebnisses. Der
	// Kontextaufruf bricht deswegen nicht ab: er steht am Anfang jedes
	// Commands, und eine defekte Zusatzkonfiguration darf nicht jeden Command
	// lahmlegen. Der Erhebungslauf des Inventars bricht sehr wohl ab — so
	// steht es im Vertrag.
	Error string `json:"error,omitempty"`
}

// VersionSource ist ein Eintrag aus sources:. Die Felder heißen wie die
// YAML-Schlüssel; die gültigen Werte von Kind und Env stehen im Vertrag.
type VersionSource struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Env  string `json:"env"`
	Note string `json:"note,omitempty"`
	// Optional: true heißt, dass eine fehlende Datei kein Hinweis ist.
	Optional bool `json:"optional,omitempty"`
}

// readVersionSources löst den Zustand der Quellenkonfiguration auf.
//
// Eine defekte Datei bricht den Kontextaufruf nicht ab: er steht am Anfang
// jedes Commands, und eine defekte Zusatzkonfiguration darf nicht jeden
// Command lahmlegen. Sichtbar bleibt der Zustand trotzdem — dafür ist `error`
// da. Der Erhebungslauf des Inventars bricht bei derselben Datei sehr wohl ab;
// das ist die Trennung zwischen Auskunft geben und Erheben, nicht ein zweites
// Verhalten desselben Lesers.
func readVersionSources(localDir string) *VersionSources {
	path := filepath.Join(localDir, VersionSourcesFileName)
	config, err := versionsources.Read(path)

	state := &VersionSources{Path: config.Path, Present: config.Present}
	if state.Path == "" {
		state.Path = path
	}
	if err != nil {
		state.Error = err.Error()
		return state
	}
	state.SchemaVersion = config.SchemaVersion
	state.Roots = config.Roots
	state.Exclude = config.Exclude
	// Ausgegeben werden die Einträge so, wie sie in der Datei stehen — auch die,
	// die der Sammler wegen unbekanntem kind oder env ablehnt. Sie hier
	// wegzulassen hieße, die Datei anders darzustellen als sie ist; die
	// Ablehnung selbst führt der Erhebungslauf sichtbar in der Inventardatei.
	for _, source := range config.Sources {
		state.Sources = append(state.Sources, VersionSource{
			Path:     source.Path,
			Kind:     source.Kind,
			Env:      source.Env,
			Note:     source.Note,
			Optional: source.Optional,
		})
	}
	return state
}

// InstructionsFileName ist die Instruktionsdatei je Ebene. Sie heißt bewusst
// nicht AGENTS.md: diesen Namen lesen die Assistenten von sich aus, und er ist
// dem Hauptverzeichnis vorbehalten.
const InstructionsFileName = "k-playbook.md"

// instructionFiles sammelt die vorhandenen Instruktionsdateien in Lesereihenfolge.
// Was fehlt, fällt weg — ein Pfad ins Leere wäre schlechter als keiner.
func instructionFiles(playbookDir string, localDir string) []string {
	files := []string{}
	for _, dir := range []string{playbookDir, localDir} {
		path := filepath.Join(dir, InstructionsFileName)
		if fileExists(path) {
			files = append(files, path)
		}
	}
	return files
}

type ContextProject struct {
	Dir      string `json:"dir"`
	RepoRoot string `json:"repoRoot"`
	VCS      string `json:"vcs"`
	Config   string `json:"config"`
	// Languages sind die Sprachen, für die dieses Projekt Werkzeuge braucht.
	// Steht nichts in der Konfiguration, gilt DefaultLanguages — der Wert ist
	// also immer belegt und muss nicht ausgelegt werden.
	Languages []string `json:"languages"`
}

type ContextDir struct {
	Dir string `json:"dir"`
}

// ContextNow ist der Zeitpunkt des Aufrufs in der Zeitzone des Rechners.
//
// Date ist der häufige Fall und deshalb fertig ausgeschnitten: Datumsstempel
// in Protokollen und Namen von Ergebnisverzeichnissen. Timestamp trägt
// zusätzlich Uhrzeit und Zeitzonenversatz, für alles Genauere.
type ContextNow struct {
	Date      string `json:"date"`
	Timestamp string `json:"timestamp"`
}

// now ist ausgelagert, damit Tests einen festen Zeitpunkt setzen können.
var now = time.Now

func buildNow() ContextNow {
	at := now()
	return ContextNow{
		Date:      at.Format("2006-01-02"),
		Timestamp: at.Format(time.RFC3339),
	}
}

// CatalogEntry ist ein aufgelöster Eintrag einer der drei Overlay-Sorten.
type CatalogEntry struct {
	// Name ist der Dateiname und zugleich die Vergleichseinheit zwischen
	// mitgeliefert und projekteigen.
	Name string `json:"name"`
	// Key ist der handliche Aufrufname: ohne Endung und ohne Sortenpräfix.
	Key  string `json:"key"`
	Path string `json:"path"`
	// Origin: dist, local oder override.
	Origin string `json:"origin"`
	// Disabled: eine leere projekteigene Datei schaltet den Eintrag ab.
	Disabled bool         `json:"disabled,omitempty"`
	Audit    *CatalogMode `json:"audit,omitempty"`
	Review   *CatalogMode `json:"review,omitempty"`
}

type CatalogMode struct {
	Enabled bool `json:"enabled"`
	// Mode ist die Betriebsart eines Audit-Rezepts: perspective oder evidence.
	// Leer heißt perspective — Rezepte ohne das Feld bleiben gültig.
	Mode           string `json:"mode,omitempty"`
	ResultRequired *bool  `json:"resultRequired,omitempty"`
	DefaultResult  string `json:"defaultResult,omitempty"`
	// RuleIDs ist die abschließende Rule-ID-Liste eines Evidence-Rezepts.
	RuleIDs []string      `json:"ruleIds,omitempty"`
	Scope   *CatalogScope `json:"scope,omitempty"`
}

// CatalogScope ist der Bewertungs-Scope eines Audit-Rezepts.
//
// Tools gehört zu mode: perspective und filtert die Gruppen aus
// review-input.json. Paths gehört zu mode: evidence und begrenzt, welchen Code
// der Eintrag lesen darf.
type CatalogScope struct {
	Tools []string `json:"tools,omitempty"`
	Paths []string `json:"paths,omitempty"`
}

// catalogKind beschreibt eine der drei Sorten.
type catalogKind struct {
	name    string
	dirName string
	suffix  string
	prefix  string
}

func catalogKinds() []catalogKind {
	return []catalogKind{
		{name: "rules", dirName: "rules", suffix: ".md"},
		{name: "reviews", dirName: "reviews", suffix: ".md", prefix: "review-"},
		{name: "checks", dirName: "checks", suffix: ".sh"},
	}
}

// BuildContext stellt den Arbeitsstand zusammen.
func BuildContext(projectDir string) (Context, error) {
	config, err := ReadConfig(projectDir)
	if err != nil {
		return Context{}, err
	}
	// Bei unbekannter Fassung wird abgebrochen statt geraten: die Werte ließen
	// sich lesen, bedeuteten aber etwas anderes.
	if err := CheckSchema(config); err != nil {
		return Context{}, err
	}
	remediation, err := ReadRemediation(projectDir)
	if err != nil {
		return Context{}, err
	}
	// Ein unbekannter Wert bricht ab, statt als „nicht entschieden" durchzugehen:
	// ein Tippfehler würde sonst wie eine Entscheidung aussehen.
	gh, err := GHState(projectDir)
	if err != nil {
		return Context{}, err
	}
	// Aus demselben Grund: ein unzulässiger Sprachname bricht ab, statt still
	// zu verschwinden und die Werkzeugauswahl unbemerkt zu verschieben.
	languages, _, err := ReadLanguages(projectDir)
	if err != nil {
		return Context{}, err
	}

	playbookDir := PlaybookDir(projectDir)
	localDir := LocalDir(projectDir)

	context := Context{
		SchemaVersion: config.SchemaVersion,
		Now:           buildNow(),
		Instructions:  instructionFiles(playbookDir, localDir),
		Project: ContextProject{
			Dir:       projectDir,
			RepoRoot:  RepoRootDir(projectDir, config),
			VCS:       config.VCS,
			Config:    ConfigPath(projectDir),
			Languages: languages,
		},
		Playbook:       ContextDir{Dir: playbookDir},
		Local:          ContextDir{Dir: localDir},
		Remediation:    remediation,
		GH:             gh,
		BaseTools:      DetectBaseTools(projectDir),
		Cleanliness:    CheckCleanliness(projectDir),
		Catalogs:       map[string][]CatalogEntry{},
		Guidelines:     listFiles(filepath.Join(localDir, "guidelines")),
		VersionSources: readVersionSources(localDir),
	}

	for _, kind := range catalogKinds() {
		context.Catalogs[kind.name] = resolveCatalog(
			filepath.Join(playbookDir, kind.dirName),
			filepath.Join(localDir, kind.dirName),
			kind,
		)
	}
	return context, nil
}

// resolveCatalog führt mitgelieferte und projekteigene Einträge zusammen.
//
// Vergleichseinheit ist der Dateiname: beide Seiten benutzen dieselbe
// Namenskonvention. Ein projekteigener Eintrag ersetzt den gleichnamigen
// mitgelieferten vollständig; ist er leer, gilt der Eintrag als abgeschaltet.
func resolveCatalog(shippedDir string, localDir string, kind catalogKind) []CatalogEntry {
	shipped := catalogFiles(shippedDir, kind)
	local := catalogFiles(localDir, kind)

	names := map[string]bool{}
	for name := range shipped {
		names[name] = true
	}
	for name := range local {
		names[name] = true
	}

	entries := make([]CatalogEntry, 0, len(names))
	for name := range names {
		entry := CatalogEntry{Name: name, Key: catalogKey(name, kind)}

		if path, ok := local[name]; ok {
			entry.Path = path
			entry.Origin = "local"
			if _, overlaid := shipped[name]; overlaid {
				entry.Origin = "override"
			}
			entry.Disabled = isEmptyFile(path)
		} else {
			entry.Path = shipped[name]
			entry.Origin = "dist"
		}
		if kind.name == "reviews" && !entry.Disabled {
			modes := readReviewModes(entry.Path)
			entry.Audit = &CatalogMode{
				Enabled:        modes.AuditEnabled,
				Mode:           modes.Mode,
				ResultRequired: modes.ResultRequired,
				DefaultResult:  modes.DefaultResult,
				RuleIDs:        append([]string{}, modes.RuleIDs...),
				Scope:          cloneCatalogScope(modes.Scope),
			}
			if len(entry.Audit.RuleIDs) == 0 {
				entry.Audit.RuleIDs = nil
			}
			entry.Review = &CatalogMode{Enabled: modes.ReviewEnabled}
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

type reviewModes struct {
	AuditEnabled   bool
	ReviewEnabled  bool
	Mode           string
	ResultRequired *bool
	DefaultResult  string
	RuleIDs        []string
	Scope          *CatalogScope
}

func readReviewModes(path string) reviewModes {
	modes := reviewModes{AuditEnabled: false, ReviewEnabled: true}
	data, err := os.ReadFile(path)
	if err != nil {
		return modes
	}
	content := string(data)
	if !(strings.HasPrefix(content, "---\n") || strings.HasPrefix(content, "---\r\n")) {
		return modes
	}
	end := markdownFrontmatterEnd(content)
	if end < 0 {
		return modes
	}
	parseReviewModeFrontmatter(content[:end], &modes)
	return modes
}

func parseReviewModeFrontmatter(frontmatter string, modes *reviewModes) {
	inAudit := false
	inReview := false
	inScope := false
	// listKey ist der Name der zuletzt geöffneten Blockliste — tools, paths
	// oder ruleIds. Ein einzelnes Flag reichte, solange nur scope.tools eine
	// Liste war; mit scope.paths und audit.ruleIds muss beim Einsammeln
	// bekannt sein, wohin die Einträge gehören.
	listKey := ""
	blockIndent := -1
	scopeIndent := -1
	listIndent := -1
	for _, line := range strings.Split(frontmatter, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "---" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if listKey != "" && indent <= listIndent {
			listKey = ""
		}
		if inScope && indent <= scopeIndent {
			inScope = false
			listKey = ""
		}
		if (inAudit || inReview) && indent <= blockIndent {
			inAudit = false
			inReview = false
			inScope = false
			listKey = ""
		}
		if listKey != "" && strings.HasPrefix(trimmed, "- ") {
			item := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			switch listKey {
			case "tools":
				modes.Scope = appendCatalogScopeTools(modes.Scope, item)
			case "paths":
				modes.Scope = appendCatalogScopePaths(modes.Scope, item)
			case "ruleIds":
				modes.RuleIDs = appendUniqueStrings(modes.RuleIDs, parseYAMLStringList(item))
			}
			continue
		}
		if strings.HasSuffix(trimmed, ":") {
			key := strings.TrimSuffix(trimmed, ":")
			if key == "audit" || key == "review" {
				inAudit = key == "audit"
				inReview = key == "review"
				blockIndent = indent
				continue
			}
			if inAudit && indent > blockIndent && key == "scope" {
				inScope = true
				scopeIndent = indent
				if modes.Scope == nil {
					modes.Scope = &CatalogScope{}
				}
				continue
			}
		}
		key, value, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}
		value = strings.TrimSpace(strings.Trim(strings.TrimSpace(value), `"'`))
		field := strings.TrimSpace(key)
		if (!inAudit && !inReview) || indent <= blockIndent {
			continue
		}
		if inAudit {
			switch field {
			case "enabled":
				modes.AuditEnabled = value != "false"
			case "mode":
				modes.Mode = value
			case "resultRequired":
				required := value != "false"
				modes.ResultRequired = &required
			case "defaultResult":
				modes.DefaultResult = value
			case "ruleIds":
				if value == "" {
					listKey = "ruleIds"
					listIndent = indent
					continue
				}
				modes.RuleIDs = appendUniqueStrings(modes.RuleIDs, parseYAMLStringList(value))
			case "tools", "paths":
				if inScope && indent > scopeIndent {
					if value == "" {
						listKey = field
						listIndent = indent
						continue
					}
					if field == "tools" {
						modes.Scope = appendCatalogScopeTools(modes.Scope, value)
					} else {
						modes.Scope = appendCatalogScopePaths(modes.Scope, value)
					}
				}
			}
		}
		if inReview && field == "enabled" {
			modes.ReviewEnabled = value != "false"
		}
	}
}

func cloneCatalogScope(scope *CatalogScope) *CatalogScope {
	if scope == nil {
		return nil
	}
	cloned := &CatalogScope{}
	if scope.Tools != nil {
		cloned.Tools = append([]string{}, scope.Tools...)
	}
	if scope.Paths != nil {
		cloned.Paths = append([]string{}, scope.Paths...)
	}
	return cloned
}

func appendCatalogScopeTools(scope *CatalogScope, value string) *CatalogScope {
	tools := parseYAMLStringList(value)
	if len(tools) == 0 {
		return scope
	}
	if scope == nil {
		scope = &CatalogScope{}
	}
	scope.Tools = appendUniqueStrings(scope.Tools, tools)
	return scope
}

func appendCatalogScopePaths(scope *CatalogScope, value string) *CatalogScope {
	paths := parseYAMLStringList(value)
	if len(paths) == 0 {
		return scope
	}
	if scope == nil {
		scope = &CatalogScope{}
	}
	scope.Paths = appendUniqueStrings(scope.Paths, paths)
	return scope
}

// appendUniqueStrings hängt an, was noch nicht dasteht. Doppelte Einträge
// entstehen leicht, wenn ein Rezept eine Liste zweimal schreibt; sie doppelt zu
// führen brächte nichts und machte jede Ausgabe unruhig.
func appendUniqueStrings(existing []string, values []string) []string {
	seen := map[string]bool{}
	for _, value := range existing {
		seen[value] = true
	}
	for _, value := range values {
		if seen[value] {
			continue
		}
		existing = append(existing, value)
		seen[value] = true
	}
	return existing
}

func parseYAMLStringList(value string) []string {
	value = strings.TrimSpace(strings.Trim(value, `"'`))
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
		if value == "" {
			return []string{}
		}
		parts := strings.Split(value, ",")
		values := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(strings.Trim(strings.TrimSpace(part), `"'`))
			if part != "" {
				values = append(values, part)
			}
		}
		return values
	}
	if strings.HasPrefix(value, "- ") {
		value = strings.TrimSpace(strings.TrimPrefix(value, "- "))
	}
	if value == "" {
		return nil
	}
	return []string{value}
}

func markdownFrontmatterEnd(content string) int {
	startEnd := strings.Index(content, "\n")
	if startEnd < 0 {
		return -1
	}
	offset := startEnd + 1
	for offset <= len(content) {
		next := strings.Index(content[offset:], "\n")
		lineEnd := len(content)
		if next >= 0 {
			lineEnd = offset + next
		}
		line := strings.TrimSpace(strings.TrimSuffix(content[offset:lineEnd], "\r"))
		if line == "---" {
			if next >= 0 {
				return lineEnd + 1
			}
			return lineEnd
		}
		if next < 0 {
			break
		}
		offset = lineEnd + 1
	}
	return -1
}

// catalogFiles liest ein Katalogverzeichnis, ohne Nicht-Einträge.
func catalogFiles(dir string, kind catalogKind) map[string]string {
	files := map[string]string{}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}
	for _, entry := range entries {
		name := entry.Name()
		// Unterverzeichnisse wie lib/ enthalten Hilfscode, keine Einträge.
		if entry.IsDir() || !isCatalogEntry(name, kind) {
			continue
		}
		files[name] = filepath.Join(dir, name)
	}
	return files
}

// isCatalogEntry filtert, was nie ein Eintrag ist: die README des Verzeichnisses,
// Dotfiles und alles, was nicht zum Muster der Sorte passt.
func isCatalogEntry(name string, kind catalogKind) bool {
	if strings.HasPrefix(name, ".") || name == "README.md" {
		return false
	}
	if !strings.HasSuffix(name, kind.suffix) {
		return false
	}
	return kind.prefix == "" || strings.HasPrefix(name, kind.prefix)
}

// catalogKey bildet den Aufrufnamen: ohne Endung, ohne Sortenpräfix.
func catalogKey(name string, kind catalogKind) string {
	return strings.TrimPrefix(strings.TrimSuffix(name, kind.suffix), kind.prefix)
}

// isEmptyFile meldet, ob eine Datei außer Leerzeilen und Kommentaren nichts
// enthält. So kann eine abgeschaltete Datei ihren Grund tragen.
func isEmptyFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		return false
	}
	return true
}

// listFiles liefert die Dateien eines Verzeichnisses ohne README und Dotfiles.
func listFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{}
	}

	files := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") || name == "README.md" {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	sort.Strings(files)
	return files
}

// ContextLinks ist die Selbstheilung der Registrierung, wie sie in der
// Kontextausgabe steht.
type ContextLinks struct {
	// Healed nennt die Einträge, die dazukamen, wegfielen oder die Quelle
	// wechselten.
	Healed LinkChanges `json:"healed,omitempty"`
	// Open sind die Ziele, an denen danach noch etwas zu tun ist.
	Open []LinkIssue `json:"open,omitempty"`
	// Note sagt im Klartext, was das für diese Sitzung bedeutet.
	Note string `json:"note,omitempty"`
}

// contextLinks übersetzt die Selbstheilung in den Teil, der in die
// Kontextausgabe gehört.
//
// Der Hinweis auf die laufende Sitzung steht bewusst dabei: Claude Code,
// OpenCode und Cursor lesen ihre Command-Liste beim Start. Ein Command, der
// gerade erst verlinkt wurde, ist auf der Platte da und im Assistenten trotzdem
// nicht — ohne den Satz sucht ein Assistent den Fehler bei sich.
func contextLinks(repair LinkRepair) *ContextLinks {
	if repair.Quiet() {
		return nil
	}

	notes := []string{}
	if repair.Applied && repair.registryApplied {
		notes = append(notes, "Die Assistenten-Registrierung wurde nachgezogen. Ein laufender Assistent hat noch die alte Liste; neue Commands und Skills kommen erst in einer neuen Sitzung an.")
	}
	// Die Migration ändert eine versionierte Projektdatei und wird deshalb
	// genau so benannt. Sie erscheint nur einmal: danach ist die Datei
	// eingerichtet, und der Aufruf schweigt wieder.
	if repair.IncludeMigrated {
		notes = append(notes, ClaudeInstructionsFile+" wurde vom Symlink auf eine Include-Datei mit "+ClaudeIncludeLine+
			" umgestellt. Das ist eine Änderung an einer versionierten Projektdatei: git zeigt einmalig eine geänderte "+
			ClaudeInstructionsFile+" mit gewechseltem Modus (Symlink → reguläre Datei); der Inhalt steht unverändert in "+
			RootInstructionsFile+".")
	}
	if repair.Error != "" {
		notes = append(notes, "Nicht vollständig eingerichtet: "+repair.Error+".")
	}
	if len(repair.Open) > 0 {
		notes = append(notes, "Was offen steht, löst sich nicht von selbst auf; der Assistenten-Block der Oberfläche nennt den Ausweg.")
	}

	return &ContextLinks{
		Healed: repair.Changed,
		Open:   repair.Open,
		Note:   strings.Join(notes, " "),
	}
}

// ContextForDir stellt den Kontext ab einem Startverzeichnis zusammen und
// meldet, wenn keine Installation gefunden wurde.
//
// Dabei zieht er die Assistenten-Registrierung nach. Das gehört hierher, weil
// dieser Aufruf am Anfang jeder Sitzung steht — über das Unterkommando context
// und über MCP — und weil die Registrierung projektbezogen ist: sie folgt dem
// Katalog dieses Projekts, nicht dem Weg, auf dem die Installation zu ihrem
// Stand kam.
func ContextForDir(startDir string) (Context, error) {
	projectDir, err := Discover(startDir)
	if err != nil {
		return Context{}, fmt.Errorf("keine %s gefunden (gesucht ab %s aufwärts)", ConfigFileName, startDir)
	}

	// Erst bauen, dann heilen. Bricht der Aufbau an einer unlesbaren oder zu
	// neuen Konfiguration ab, wird auch nichts eingerichtet: ein Werkzeug, das
	// die Fassung nicht versteht, soll die Registrierung nicht nach seinen
	// Regeln umschreiben.
	built, err := BuildContext(projectDir)
	if err != nil {
		return Context{}, err
	}

	built.Links = contextLinks(HealLinks(projectDir))
	return built, nil
}
