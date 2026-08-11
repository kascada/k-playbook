package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Context ist der aufgeloeste Arbeitsstand eines Projekts: alles, was ein
// Command sonst selbst aus Konfiguration und Dateisystem zusammenrechnen
// muesste.
//
// Die Security-Tools fehlen bewusst. Ihr Preflight ruft je Tool ein --version
// auf und dauert spuerbar; dieser Aufruf soll billig genug sein, um am Anfang
// jedes Commands zu stehen.
type Context struct {
	SchemaVersion string `json:"schemaVersion"`
	// Instructions sind die Dateien, die vor der Arbeit zu lesen sind, in
	// dieser Reihenfolge: erst was fuer alle k-playbook-Projekte gilt, dann was
	// dieses Projekt ergaenzt.
	Instructions []string                  `json:"instructions"`
	Project      ContextProject            `json:"project"`
	Playbook     ContextDir                `json:"playbook"`
	Local        ContextDir                `json:"local"`
	Remediation  Remediation               `json:"remediation"`
	Catalogs     map[string][]CatalogEntry `json:"catalogs"`
	Guidelines   []string                  `json:"guidelines"`
}

// InstructionsFileName ist die Instruktionsdatei je Ebene. Sie heisst bewusst
// nicht AGENTS.md: diesen Namen lesen die Assistenten von sich aus, und er ist
// dem Hauptverzeichnis vorbehalten.
const InstructionsFileName = "k-playbook.md"

// instructionFiles sammelt die vorhandenen Instruktionsdateien in Lesereihenfolge.
// Was fehlt, faellt weg — ein Pfad ins Leere waere schlechter als keiner.
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
}

type ContextDir struct {
	Dir string `json:"dir"`
}

// CatalogEntry ist ein aufgeloester Eintrag einer der drei Overlay-Sorten.
type CatalogEntry struct {
	// Name ist der Dateiname und zugleich die Vergleichseinheit zwischen
	// mitgeliefert und projekteigen.
	Name string `json:"name"`
	// Key ist der handliche Aufrufname: ohne Endung und ohne Sortenpraefix.
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
	// Bei unbekannter Fassung wird abgebrochen statt geraten: die Werte liessen
	// sich lesen, bedeuteten aber etwas anderes.
	if err := CheckSchema(config); err != nil {
		return Context{}, err
	}
	remediation, err := ReadRemediation(projectDir)
	if err != nil {
		return Context{}, err
	}

	playbookDir := PlaybookDir(projectDir)
	localDir := LocalDir(projectDir)

	context := Context{
		SchemaVersion: config.SchemaVersion,
		Instructions:  instructionFiles(playbookDir, localDir),
		Project: ContextProject{
			Dir:      projectDir,
			RepoRoot: RepoRootDir(projectDir, config),
			VCS:      config.VCS,
			Config:   ConfigPath(projectDir),
		},
		Playbook:    ContextDir{Dir: playbookDir},
		Local:       ContextDir{Dir: localDir},
		Remediation: remediation,
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

// resolveCatalog fuehrt mitgelieferte und projekteigene Eintraege zusammen.
//
// Vergleichseinheit ist der Dateiname: beide Seiten benutzen dieselbe
// Namenskonvention. Ein projekteigener Eintrag ersetzt den gleichnamigen
// mitgelieferten vollstaendig; ist er leer, gilt der Eintrag als abgeschaltet.
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

// catalogFiles liest ein Katalogverzeichnis, ohne Nicht-Eintraege.
func catalogFiles(dir string, kind catalogKind) map[string]string {
	files := map[string]string{}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}
	for _, entry := range entries {
		name := entry.Name()
		// Unterverzeichnisse wie lib/ enthalten Hilfscode, keine Eintraege.
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

// catalogKey bildet den Aufrufnamen: ohne Endung, ohne Sortenpraefix.
func catalogKey(name string, kind catalogKind) string {
	return strings.TrimPrefix(strings.TrimSuffix(name, kind.suffix), kind.prefix)
}

// isEmptyFile meldet, ob eine Datei ausser Leerzeilen und Kommentaren nichts
// enthaelt. So kann eine abgeschaltete Datei ihren Grund tragen.
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
		return Context{}, fmt.Errorf("keine %s gefunden (gesucht ab %s aufwaerts)", ConfigFileName, startDir)
	}
	return BuildContext(projectDir)
}
