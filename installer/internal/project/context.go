package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Context ist der aufgelöste Arbeitsstand eines Projekts: alles, was ein
// Command sonst selbst aus Konfiguration und Dateisystem zusammenrechnen
// müsste.
//
// Die Security-Tools fehlen bewusst. Ihr Preflight ruft je Tool ein --version
// auf und dauert spürbar; dieser Aufruf soll billig genug sein, um am Anfang
// jedes Commands zu stehen.
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
	// Cleanliness ist der lokale Zustand der Installation. Sie steht hier, weil
	// die Regel „in k-playbook/ wird nie geschrieben" sich nicht selbst
	// durchsetzt und ihr Bruch still bleibt: Ändert sich eine lokal veränderte
	// Datei upstream nicht mit, läuft `git pull` sauber durch und lässt sie
	// stehen. Zwei git-Aufrufe im lokalen Clone, ohne Netz.
	Cleanliness Cleanliness               `json:"cleanliness"`
	Catalogs    map[string][]CatalogEntry `json:"catalogs"`
	Guidelines  []string                  `json:"guidelines"`
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
	Disabled bool `json:"disabled,omitempty"`
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
		Playbook:    ContextDir{Dir: playbookDir},
		Local:       ContextDir{Dir: localDir},
		Remediation: remediation,
		GH:          gh,
		Cleanliness: CheckCleanliness(projectDir),
		Catalogs:    map[string][]CatalogEntry{},
		Guidelines:  listFiles(filepath.Join(localDir, "guidelines")),
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
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
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

// ContextForDir stellt den Kontext ab einem Startverzeichnis zusammen und
// meldet, wenn keine Installation gefunden wurde.
func ContextForDir(startDir string) (Context, error) {
	projectDir, err := Discover(startDir)
	if err != nil {
		return Context{}, fmt.Errorf("keine %s gefunden (gesucht ab %s aufwärts)", ConfigFileName, startDir)
	}
	return BuildContext(projectDir)
}
