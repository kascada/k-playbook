package project

import (
	stdcontext "context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
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
	Now         ContextNow     `json:"now"`
	Project     ContextProject `json:"project"`
	Playbook    ContextDir     `json:"playbook"`
	Local       ContextDir     `json:"local"`
	Remediation Remediation    `json:"remediation"`
	// Docs ist der Block docs: — was die Docs-Erzeuger beim Code-Scan
	// zusätzlich auslassen (exclude) und welche Einträge verworfen wurden
	// (problems). Commands lesen die Liste von hier, nie aus der Datei.
	Docs ContextDocs `json:"docs"`
	// GH ist der Befund zur GitHub CLI auf diesem Rechner. Anders als der
	// Security-Preflight kostet der nichts: ein Blick in den PATH und in die
	// gh-Konfiguration, kein Unterprozess und kein Netzzugriff.
	GH GH `json:"gh"`
	// MCP ist die Pflichtliste der MCP-Server aus tools.mcp.required samt der
	// Auskunft, ob der Block dastand. Gelesen wird nur die Konfiguration —
	// welche Server tatsächlich eingetragen sind, zeigt die Oberfläche.
	MCP MCPRequirements `json:"mcp"`
	// Git ist der Abschnitt git: — ob die Oberfläche das Umschalten des
	// Branches anbietet, wohin, und welche Umgebungen festgelegt sind. Ohne
	// Abschnitt steht switch auf unknown und configured auf false.
	Git GitSettings `json:"git"`
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
	BaseTools BaseTools                 `json:"baseTools"`
	Catalogs  map[string][]CatalogEntry `json:"catalogs"`
	// CatalogsProblem sagt, warum catalogs.reviews leer ist: Seine
	// mitgelieferte Ebene liegt in <hub>:k-playbook unter defaults/ und ist
	// ohne Kephalaion nicht lesbar. Die lokale Ebene steht dann nicht allein
	// da — der Katalog bleibt leer. Leer, wenn die Kataloge gelesen wurden.
	CatalogsProblem string `json:"catalogsProblem,omitempty"`
	// VersionSources ist der Zustand der Quellenkonfiguration des
	// Versionsinventars, des Dokuments keph:/k-playbook/inventory.md. Sie
	// steht hier, weil Commands Konfiguration ausschließlich aus dieser
	// Ausgabe lesen: eine Konfiguration, die nur der Sammler kennt, wäre für
	// jeden Command unsichtbar und würde eine zweite, unsichtbare
	// Konfigurationsquelle aufmachen.
	//
	// Das Feld ist ein Zeiger mit omitempty: Gefüllt wird es von
	// ContextForDir, nicht von BuildContext — es liest aus der Collection,
	// und die Werkzeuge, die nur den Katalog brauchen, sollen den Node dafür
	// nicht fragen (wie Kephalaion).
	VersionSources *VersionSources `json:"versionSources,omitempty"`
	// Kephalaion ist die Anbindung des Projekts an seine Collection: Hub und
	// Collection aus dem Block kephalaion: von .k-playbook/K-PLAYBOOK.yaml,
	// Account samt Herkunft (choice) aus der Auskunft kephalaion node client
	// und das Ergebnis der Prüfung über MCP am Node dieses Rechners. Ein Fehler
	// ist ein Zustand, kein Abbruch. Gefüllt wird das Feld von ContextForDir,
	// nicht von BuildContext — siehe KephalaionStatus.
	Kephalaion *kephalaion.Status `json:"kephalaion,omitempty"`
	// Links meldet die Selbstheilung der Assistenten-Registrierung: was dabei
	// nachgezogen wurde und was offen blieb. Fehlt das Feld, stimmte alles —
	// eine Meldung, die bei jedem Aufruf dasselbe sagt, liest niemand mehr.
	Links *ContextLinks `json:"links,omitempty"`
	// GitHub nennt, welcher Dienst der persönlichen Zugangsdatei
	// ~/.config/k-playbook/github.yaml zum Repo des Arbeitsstands gehört und
	// welche Zugänge `k-playbook github run` dafür wählt — dieselbe Quelle
	// und Auswahl wie `github show`. Anders als GH kostet das Unterprozesse:
	// drei bis vier git-Aufrufe für das Remote und `ssh -G` je ssh host der Datei
	// (Frist 3 s je Host), dazu der Blick in die gh-Konfiguration; kein Netz,
	// kein Token-Wert, nichts geschrieben — auch keine Zugangsdatei, wenn sie
	// fehlt; die legt nur `k-playbook github init` an. Gefüllt wird das Feld
	// von ContextForDir, nicht von BuildContext — die Oberfläche und die
	// Review-Auswahl über MCP tragen die Kosten nicht. Siehe contextGitHub.
	GitHub *ContextGitHub `json:"github,omitempty"`
}

type ContextProject struct {
	Dir      string `json:"dir"`
	RepoRoot string `json:"repoRoot"`
	VCS      string `json:"vcs"`
	Config   string `json:"config"`
	// Languages sind die Sprachen, für die dieses Projekt Werkzeuge braucht.
	// Steht nichts in der Konfiguration, gilt die Erkennung aus den
	// Manifesten, sonst DefaultLanguages — der Wert ist also immer belegt und
	// muss nicht ausgelegt werden.
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

// CatalogEntry ist ein aufgelöster Eintrag eines Katalogs: reviews mit
// Overlay, checks nur mitgeliefert.
type CatalogEntry struct {
	// Name ist der Dateiname und zugleich die Vergleichseinheit zwischen
	// mitgeliefert und projekteigen.
	Name string `json:"name"`
	// Key ist der handliche Aufrufname: ohne Endung und ohne Sortenpräfix.
	Key string `json:"key"`
	// Path sagt, wie der wirksame Eintrag gelesen wird: bei reviews der
	// Verweis k-playbook:/reviews/<name>, gleich ob mitgeliefert,
	// projekteigen oder überschreibend — beide Ebenen liegen in Collections
	// (<hub>:k-playbook unter defaults/, die Collection des Projekts unter
	// k-playbook/); gelesen wird er mit ReadCatalogEntry über die Ebenen.
	// checks liegen nur in der Installation: dort ein Pfad auf der Platte.
	Path string `json:"path"`
	// Origin: dist, local oder override; bei checks immer dist.
	Origin string `json:"origin"`
	// Location ist bei local und override, auch abgeschaltet, der Ort des
	// projekteigenen Eintrags als keph:-Verweis (keph:/k-playbook/reviews/
	// <name>); leer bei dist.
	Location string `json:"location,omitempty"`
	// Disabled: eine leere projekteigene Datei schaltet den Eintrag ab; bei
	// checks nie, sie haben keine lokale Ebene.
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

// catalogKind beschreibt einen Katalog: reviews mit lokaler Ebene, checks
// ohne (Task 134).
type catalogKind struct {
	name    string
	dirName string
	suffix  string
	prefix  string
}

func catalogKinds() []catalogKind {
	return []catalogKind{
		{name: "reviews", dirName: "reviews", suffix: ".md", prefix: "review-"},
		{name: "checks", dirName: "checks", suffix: ".sh"},
	}
}

// catalogKindNamed liefert eine Sorte aus catalogKinds() nach Namen. Ein
// unbekannter Name ist ein Programmierfehler, kein Laufzeitfall.
func catalogKindNamed(name string) catalogKind {
	for _, kind := range catalogKinds() {
		if kind.name == name {
			return kind
		}
	}
	panic("unbekannte Katalogsorte " + name)
}

// guidelinesKind ist die Sorte guidelines/ für ResolveDocRef und das
// Briefing: nur mitgeliefert, aus <hub>:k-playbook unter
// defaults/guidelines/. Eine lokale Ebene hat sie seit Task 148 nicht mehr
// (docDistOnlyKinds) — k-playbook-local/guidelines/ liest nichts, projekteigene
// Guidelines liegen unter keph:/docs/guidelines/ (Todo #40). Bewusst nicht in
// catalogKinds(): Die Kontextausgabe nennt keine Guidelines, das Briefing
// liefert sie unter documents.guidelines.
var guidelinesKind = catalogKind{name: "guidelines", dirName: docGuidelinesRoot, suffix: ".md"}

// BuildContext stellt den Arbeitsstand zusammen.
func BuildContext(projectDir string) (Context, error) {
	return buildContext(projectDir, DocLayers{})
}

// BuildContextWithLayers ist BuildContext und gibt dazu die Ebenen des
// Projekts zurück (ProjectDocLayers), deren mitgelieferte und projekteigene
// Ebene den Katalog reviews schon gelesen haben. Wer danach Einträge dieses Katalogs
// liest, nimmt sie über ReadCatalogEntryWith aus diesen Ebenen: Die
// mitgelieferte Ebene ist dann einmal geöffnet und hält die gelesenen
// Dateien, statt je Eintrag einen neuen Zugang zum Node aufzubauen (Task 132,
// die Review-Auswahl). Context selbst bleibt dabei, wie BuildContext ihn baut.
func BuildContextWithLayers(projectDir string) (Context, DocLayers, error) {
	layers := ProjectDocLayers(projectDir)
	built, err := buildContext(projectDir, layers)
	if err != nil {
		return Context{}, DocLayers{}, err
	}
	return built, layers, nil
}

// buildContext ist BuildContext mit den Ebenen des Katalogs reviews: Dist die
// mitgelieferte, Local die projekteigene; nil heißt je eine eigene
// (NewProjectDefaultsSource, NewProjectLocalDocSource). Das Briefing reicht
// seine durch, damit es dieselben Dokumente nicht zweimal vom Node liest.
func buildContext(projectDir string, shared DocLayers) (Context, error) {
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
	// Ein unzulässiger Sprachname bricht ab, statt still zu verschwinden und
	// die Werkzeugauswahl unbemerkt zu verschieben.
	languages, _, err := ReadLanguages(projectDir)
	if err != nil {
		return Context{}, err
	}
	// Und noch einmal: ein unzulässiger Pflichtname bricht ab, statt als
	// „nichts verlangt" durchzugehen.
	requiredMCP, mcpConfigured, err := ReadRequiredMCPServers(projectDir)
	if err != nil {
		return Context{}, err
	}

	// Ebenso bei git.switch: ein unbekannter Wert soll nicht wie „nicht
	// entschieden" aussehen, und eine Umgebung ohne Branch nicht wie keine.
	gitSettings, err := ReadGitSettings(projectDir)
	if err != nil {
		return Context{}, err
	}

	playbookDir := PlaybookDir(projectDir)
	localDir := LocalDir(projectDir)

	context := Context{
		SchemaVersion: config.SchemaVersion,
		Now:           buildNow(),
		Project: ContextProject{
			Dir:       projectDir,
			RepoRoot:  RepoRootDir(projectDir, config),
			VCS:       config.VCS,
			Config:    ConfigPath(projectDir),
			Languages: languages,
		},
		Playbook:    ContextDir{Dir: playbookDir},
		Local:       ContextDir{Dir: localDir},
		Remediation: remediation,
		Docs:        contextDocs(config),
		GH:          DetectGH(),
		MCP:         MCPRequirements{Required: requiredMCP, Configured: mcpConfigured},
		Git:         gitSettings,
		BaseTools:   DetectBaseTools(projectDir),
		Catalogs:    map[string][]CatalogEntry{},
	}

	layers := DocLayers{Dist: shared.Dist, Local: shared.Local}
	if layers.Dist == nil {
		layers.Dist = NewProjectDefaultsSource(projectDir)
	}
	if layers.Local == nil {
		layers.Local = NewProjectLocalDocSource(projectDir)
	}
	for _, kind := range catalogKinds() {
		entries, err := resolveCatalog(projectDir, layers, kind)
		if errDefaultsUnavailable(err) {
			// Eine Ebene in einer Collection fehlt — die mitgelieferte oder
			// die projekteigene: keine Kataloge aus der anderen allein. Der
			// Rest der Kontextausgabe bleibt — er trägt unter kephalaion,
			// warum.
			context.Catalogs[kind.name] = []CatalogEntry{}
			context.CatalogsProblem = err.Error()
			continue
		}
		if err != nil {
			return Context{}, err
		}
		context.Catalogs[kind.name] = entries
	}
	return context, nil
}

// resolveCatalog führt mitgelieferte und projekteigene Einträge zusammen —
// über overlayDocDir (docref.go), dieselbe Überlagerung, die auch die
// Verweise und das Briefing anwenden; eine zweite Fassung der Regel gäbe
// irgendwann eine zweite Antwort.
//
// Die Ebenen von reviews sind dieselben Quellen wie hinter
// k-playbook:-Verweisen: mitgeliefert <hub>:k-playbook unter defaults/,
// projekteigen die Collection des Projekts unter k-playbook/ (Task 143);
// checks liest weiter aus der Installation k-playbook/.
//
// checks hat keine lokale Ebene mehr (Task 134): Local bleibt leer, ein
// k-playbook-local/checks/ wird nicht gelesen, jeder Eintrag ist dist und
// keiner abgeschaltet.
//
// Vergleichseinheit ist der Dateiname: beide Seiten benutzen dieselbe
// Namenskonvention. Ein projekteigener Eintrag ersetzt den gleichnamigen
// mitgelieferten vollständig; ist er leer, gilt der Eintrag als abgeschaltet.
// Ein fehlendes Verzeichnis ist ein leerer Katalog; eine unlesbare Datei ist
// ein Fehler, kein stiller Eintrag ohne Befund.
func resolveCatalog(projectDir string, layers DocLayers, kind catalogKind) ([]CatalogEntry, error) {
	playbookDir := PlaybookDir(projectDir)
	layers = DocLayers{Dist: layers.Dist, Local: layers.Local}
	if kind.name == "checks" {
		layers = DocLayers{Dist: NewDiskDocSource(playbookDir)}
	}
	resolved, _, err := overlayDocDir(layers, kind.dirName, func(name string) bool { return isCatalogEntry(name, kind) })
	if err != nil {
		return nil, fmt.Errorf("Katalog %s: %w", kind.name, err)
	}

	entries := make([]CatalogEntry, 0, len(resolved))
	for _, item := range resolved {
		entry := CatalogEntry{Name: item.Name, Key: catalogKey(item.Name, kind), Origin: item.Origin,
			Location: item.Location, Disabled: item.Disabled}
		switch {
		case kind.name == "checks":
			entry.Path = filepath.Join(playbookDir, kind.dirName, item.Name)
		default:
			entry.Path = DocSchemeKPlaybook + ":/" + path.Join(kind.dirName, item.Name)
		}
		if kind.name == "reviews" && !entry.Disabled {
			modes := parseReviewModes(item.Content)
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
	return entries, nil
}

// ReadCatalogEntry liest den wirksamen Inhalt eines Katalogeintrags über
// seinen Path: einen Verweis k-playbook:/… über frische Ebenen des Projekts
// (ProjectDocLayers, Rohtext ohne eingesetzte Includes) — mitgeliefert,
// projekteigen oder überschreibend —, sonst die Datei auf der Platte, etwa
// einen check oder einen Pfad aus einem älteren Lauf. Jeder Aufruf mit einem
// Verweis öffnet die Ebenen neu;
// wer mehrere Einträge liest, nimmt ReadCatalogEntryWith.
func ReadCatalogEntry(projectDir string, entryPath string) ([]byte, error) {
	if !strings.HasPrefix(entryPath, DocSchemeKPlaybook+":/") {
		return os.ReadFile(entryPath)
	}
	return ReadCatalogEntryWith(ProjectDocLayers(projectDir), entryPath)
}

// ReadCatalogEntryWith ist ReadCatalogEntry über schon gebaute Ebenen, etwa
// die aus BuildContextWithLayers.
func ReadCatalogEntryWith(layers DocLayers, entryPath string) ([]byte, error) {
	if !strings.HasPrefix(entryPath, DocSchemeKPlaybook+":/") {
		return os.ReadFile(entryPath)
	}
	result, err := ResolveDocRefWith(stdcontext.Background(), layers, entryPath, DocReadOptions{Raw: true})
	if err != nil {
		return nil, err
	}
	return []byte(result.Content), nil
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

// parseReviewModes liest die Modi aus dem Frontmatter eines schon gelesenen
// Rezepts — etwa aus einer DocSource, die keine Datei auf der Platte sein
// muss.
func parseReviewModes(data []byte) reviewModes {
	modes := reviewModes{AuditEnabled: false, ReviewEnabled: true}
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

// isCatalogEntry filtert, was nie ein Eintrag ist: die README des Verzeichnisses,
// Dotfiles und alles, was nicht zum Muster der Sorte passt. Unterverzeichnisse
// wie lib/ enthalten Hilfscode, keine Einträge — die lässt schon
// overlayDocDir aus.
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
	return isEmptyContent(data)
}

// isEmptyContent ist die Prüfung hinter isEmptyFile, für einen Inhalt, der
// schon gelesen ist — etwa aus einer DocSource, die keine Datei auf der
// Platte sein muss.
func isEmptyContent(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		return false
	}
	return true
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
// Der Hinweis auf die laufende Sitzung steht bewusst dabei: Claude Code und
// OpenCode lesen ihre Command-Liste beim Start. Ein Command, der
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
	return contextForDir(startDir, "", DocLayers{})
}

// contextForDir ist ContextForDir; liegt startDir im Projekt sharedDir, lesen
// die Kataloge die Ebenen aus shared statt aus eigenen.
func contextForDir(startDir string, sharedDir string, shared DocLayers) (Context, error) {
	projectDir, err := Discover(startDir)
	if errors.Is(err, ErrNotFound) {
		return Context{}, fmt.Errorf("keine %s gefunden (gesucht ab %s aufwärts)", ConfigFileName, startDir)
	}
	if err != nil {
		// Der Umzug vom früheren Ort ist gescheitert; „nicht gefunden“ wäre falsch.
		return Context{}, err
	}

	// Erst bauen, dann heilen. Bricht der Aufbau an einer unlesbaren oder zu
	// neuen Konfiguration ab, wird auch nichts eingerichtet: ein Werkzeug, das
	// die Fassung nicht versteht, soll die Registrierung nicht nach seinen
	// Regeln umschreiben.
	var layers DocLayers
	if projectDir == sharedDir {
		layers = shared
	}
	built, err := buildContext(projectDir, layers)
	if err != nil {
		return Context{}, err
	}

	built.Links = contextLinks(HealLinks(projectDir))
	status := KephalaionStatus(projectDir)
	built.Kephalaion = &status
	built.VersionSources = versionSourcesWithStatus(projectDir, status)
	// Das Remote ab startDir, nicht ab projectDir: Ein eigenes Repo unterhalb
	// des Projekts hat sein eigenes.
	built.GitHub = contextGitHub(startDir)
	return built, nil
}
