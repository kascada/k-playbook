package project

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// SchemaVersion ist die Fassung, die dieses Werkzeug schreibt und versteht.
// Eine Datei mit kleinerer Nummer beschreibt etwas anderes als das, was hier
// gelesen würde; das Projekt ist dann neu einzurichten.
const SchemaVersion = "3"

// Config sind die Werte aus der K-PLAYBOOK.yaml, soweit sie derzeit gebraucht
// werden.
type Config struct {
	SchemaVersion string `json:"schemaVersion"`
	RepoRoot      string `json:"repoRoot"`
	VCS           string `json:"vcs"`
	// DocsExclude sind die gültigen Muster aus docs.exclude, DocsProblems die
	// verworfenen mit Grund (docs_exclude.go). Ohne Block sind beide leer.
	DocsExclude  []string             `json:"docsExclude,omitempty"`
	DocsProblems []DocsExcludeProblem `json:"docsProblems,omitempty"`
	// Kephalaion ist der Block kephalaion: — Hub-Alias und Collection der
	// Anbindung (kephalaion_block.go). Ohne Block ist er leer.
	Kephalaion ConfigKephalaion `json:"kephalaion"`
	// Maintenance ist der Block maintenance: — Tagesprogramm und Doku-Sicherung
	// des Wartungslaufs (maintenance_config.go). Ohne Block gelten die
	// Vorgaben, Present ist false.
	Maintenance ConfigMaintenance `json:"maintenance"`
}

// SchemaStatus benennt, wie die gefundene schema_version zum Werkzeug steht.
//
// Der Fehlertext allein genügt dafür nicht: die Oberfläche muss die Fälle
// auseinanderhalten können. Bei einer zu alten Datei ist Zurücksetzen die
// Lösung, bei einer zu neuen wäre es genau falsch — dort ist die Installation
// hinterher, und ein Zurücksetzen würde die neuere Konfiguration wegwerfen.
type SchemaStatus string

const (
	// SchemaOK: die Fassung ist die, die dieses Werkzeug schreibt.
	SchemaOK SchemaStatus = "ok"
	// SchemaMissing: die Datei trägt gar keine schema_version.
	SchemaMissing SchemaStatus = "missing"
	// SchemaOutdated: die Fassung ist älter als die des Werkzeugs.
	SchemaOutdated SchemaStatus = "outdated"
	// SchemaNewer: die Fassung ist neuer als das Werkzeug.
	SchemaNewer SchemaStatus = "newer"
)

// Resettable sagt, ob der Zustand sich durch Zurücksetzen der Konfiguration
// auflösen lässt.
func (s SchemaStatus) Resettable() bool {
	return s == SchemaOutdated || s == SchemaMissing
}

// SchemaState ordnet die gefundene Fassung ein.
func SchemaState(config Config) SchemaStatus {
	switch config.SchemaVersion {
	case SchemaVersion:
		return SchemaOK
	case "":
		return SchemaMissing
	}
	if found, err := strconv.Atoi(config.SchemaVersion); err == nil {
		if own, _ := strconv.Atoi(SchemaVersion); found < own {
			return SchemaOutdated
		}
	}
	return SchemaNewer
}

// CheckSchema meldet, wenn die Konfiguration nicht zu diesem Werkzeug passt.
//
// Stillschweigend weiterzumachen wäre das Gefährlichste: die Datei ließe
// sich lesen, ihre Werte bedeuteten aber etwas anderes.
func CheckSchema(config Config) error {
	switch SchemaState(config) {
	case SchemaOK:
		return nil
	case SchemaMissing:
		return fmt.Errorf("%s hat keine schema_version; erwartet wird %s — %s",
			ConfigFileName, SchemaVersion, resetHint)
	case SchemaOutdated:
		return fmt.Errorf("%s hat schema_version %s, dieses Werkzeug erwartet %s — bitte neu einrichten: %s",
			ConfigFileName, config.SchemaVersion, SchemaVersion, resetHint)
	default:
		return fmt.Errorf("%s hat schema_version %s, dieses Werkzeug versteht %s — vermutlich ist die Installation älter als die Konfiguration; hilft `k-playbook upgrade` nicht weiter, gehört die Datei zu einem anderen Projekt",
			ConfigFileName, config.SchemaVersion, SchemaVersion)
	}
}

// resetHint nennt den Ausweg. Ohne ihn endete die Meldung bei der Diagnose:
// von Hand ist der Weg nicht zu erraten, und `CreateConfig` verweigert, solange
// die alte Datei liegt.
const resetHint = "die Oberfläche starten (`k-playbook` ohne Argument oder `/k-gui`), der Block „Projektkonfiguration\" " +
	"sichert die alte Datei weg und legt sie neu an"

// ReadConfig liest die Konfiguration eines Projekts.
//
// Bewusst kein YAML-Parser: gelesen werden nur wenige Skalare, die Liste
// docs.exclude (parseDocsExclude), der Block kephalaion:
// (parseKephalaionBlock) und der Block maintenance: (parseMaintenanceBlock),
// alle ebenfalls zeilenweise, und der
// zeilenweise Zugriff lässt Kommentare, Reihenfolge und unbekannte Blöcke
// unangetastet, wenn später zurückgeschrieben wird.
//
// Einem Skalar nimmt es ein Paar umschließender Anführungszeichen ab
// (unquoteConfigScalar): `schema_version: "3"` ist dieselbe Fassung wie
// `schema_version: 3` und wurde vorher als neuere abgelehnt. Mehr YAML
// versteht es weiter nicht — keine Kommentare hinter dem Wert, keine
// Escape-Folgen.
func ReadConfig(projectDir string) (Config, error) {
	data, err := os.ReadFile(ConfigPath(projectDir))
	if err != nil {
		return Config{}, err
	}

	config := Config{}
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		indented := strings.HasPrefix(key, " ") || strings.HasPrefix(key, "\t")
		key = strings.TrimSpace(key)
		value = unquoteConfigScalar(strings.TrimSpace(value))

		if !indented {
			section = key
			if value == "" {
				continue
			}
		}

		switch {
		case !indented && key == "schema_version":
			config.SchemaVersion = value
		case indented && section == "project" && key == "repo_root":
			config.RepoRoot = value
		case indented && section == "project" && key == "vcs":
			config.VCS = value
		}
	}
	config.DocsExclude, config.DocsProblems = parseDocsExclude(string(data))
	config.Kephalaion = parseKephalaionBlock(string(data))
	config.Maintenance = parseMaintenanceBlock(string(data))
	return config, nil
}

// unquoteConfigScalar nimmt einem Wert genau ein Paar umschließender
// Anführungszeichen ab, doppelte oder einfache. Ein Wert, der nur vorn oder
// nur hinten eines trägt oder zwei verschiedene, bleibt, wie er ist: Das ist
// kein gequoteter Skalar, und raten soll der Leser nicht.
func unquoteConfigScalar(value string) string {
	if len(value) < 2 {
		return value
	}
	first, last := value[0], value[len(value)-1]
	if first == last && (first == '"' || first == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}

// RepoRootDir löst project.repo_root gegen das Hauptverzeichnis auf.
func RepoRootDir(projectDir string, config Config) string {
	repoRoot := config.RepoRoot
	if repoRoot == "" {
		repoRoot = "."
	}
	return filepath.Clean(filepath.Join(projectDir, repoRoot))
}

// Suggestion ist der Vorschlag für eine noch nicht angelegte Konfiguration.
type Suggestion struct {
	// ProjectDir ist das vorgeschlagene Hauptverzeichnis.
	ProjectDir string `json:"projectDir"`
	// ProjectCandidates sind alle plausiblen Orte, ProjectDir zuerst. Kommt mehr
	// als einer in Frage, zeigt die Oberfläche sie zur Auswahl.
	ProjectCandidates []string `json:"projectCandidates"`
	// RepoRoot ist der Vorschlag für project.repo_root, relativ zu ProjectDir.
	// Leer, wenn kein Repository gefunden wurde oder mehrere in Frage kommen.
	RepoRoot string `json:"repoRoot"`
	// RepoCandidates sind alle gefundenen Repositories unterhalb von ProjectDir.
	RepoCandidates []string `json:"repoCandidates"`
}

// Suggest leitet einen Vorschlag ab, ohne etwas zu schreiben.
//
// Anders als Discover darf das raten: geschrieben wird erst nach Bestätigung.
// Der stärkste Hinweis ist das Repository, in dem der Aufruf stattfindet — wer
// das Werkzeug startet, steht in aller Regel in dem Projekt, das er meint.
// Danach kommt das Arbeitsverzeichnis selbst.
func Suggest() Suggestion {
	suggestion := Suggestion{}
	workdir, _ := os.Getwd()

	if workdir != "" {
		if root, ok := gitWorktreeRoot(workdir); ok {
			suggestion.ProjectCandidates = append(suggestion.ProjectCandidates, root)
		}
	}

	if workdir != "" {
		suggestion.ProjectCandidates = addUnique(suggestion.ProjectCandidates, workdir)
	}

	if len(suggestion.ProjectCandidates) > 0 {
		suggestion.ProjectDir = suggestion.ProjectCandidates[0]
		suggestion.RepoRoot, suggestion.RepoCandidates = suggestRepoRoot(suggestion.ProjectDir)
	}
	return suggestion
}

// gitWorktreeRoot sucht ab dir aufwärts das Repository, in dem dir liegt.
func gitWorktreeRoot(dir string) (string, bool) {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	home := homeDir()

	for {
		if pathExists(filepath.Join(dir, ".git")) {
			return dir, true
		}
		if dir == home {
			return "", false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func addUnique(list []string, value string) []string {
	if value == "" || slices.Contains(list, value) {
		return list
	}
	return append(list, value)
}

// suggestRepoRoot sucht das Projekt-Repository. Liegt es im Hauptverzeichnis
// selbst, ist der Wert "."; sonst kommen die Unterverzeichnisse in Frage, etwa
// wenn das Repo eine Ebene unter dem Hauptverzeichnis ausgecheckt ist.
func suggestRepoRoot(projectDir string) (string, []string) {
	if pathExists(filepath.Join(projectDir, ".git")) {
		return ".", []string{"."}
	}

	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return "", nil
	}

	var candidates []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		if pathExists(filepath.Join(projectDir, name, ".git")) {
			candidates = append(candidates, name)
		}
	}

	// Nur bei genau einem Fund ist die Zuordnung eindeutig.
	if len(candidates) == 1 {
		return candidates[0], candidates
	}
	return "", candidates
}

// CreateConfig legt die K-PLAYBOOK.yaml unter .k-playbook/ im Hauptverzeichnis
// an; das Verzeichnis entsteht, wenn es fehlt.
//
// Eine vorhandene Datei wird nie überschrieben: sie gehört dem Projekt und
// kann Werte enthalten, die hier nicht bekannt sind.
func CreateConfig(projectDir string, repoRoot string) error {
	if strings.TrimSpace(projectDir) == "" {
		return fmt.Errorf("kein Hauptverzeichnis angegeben")
	}
	if !isDir(projectDir) {
		return fmt.Errorf("%s ist kein Verzeichnis", projectDir)
	}

	path := ConfigPath(projectDir)
	if pathExists(path) {
		return fmt.Errorf("%s existiert bereits", ConfigFileName)
	}

	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		repoRoot = "."
	}
	vcs := "none"
	if pathExists(filepath.Join(projectDir, repoRoot, ".git")) {
		vcs = "git"
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("%s anlegen: %w", ProjectDirName, err)
	}
	return os.WriteFile(path, []byte(renderConfig(repoRoot, vcs)), 0o644)
}

func renderConfig(repoRoot string, vcs string) string {
	return fmt.Sprintf(`# k-playbook
#
# Der Ort dieser Datei bestimmt das Hauptverzeichnis des Projekts: das
# Verzeichnis über .k-playbook/.

schema_version: %s

project:
  # Ort des Projekt-Repositorys, relativ zum Hauptverzeichnis.
  repo_root: %s
  vcs: %s

%s
%s
%s`, SchemaVersion, repoRoot, vcs,
		remediationBlock(DefaultRemediationMode), docsBlockExample, maintenanceBlock())
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
