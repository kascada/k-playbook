package project

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// SchemaVersion ist die Fassung, die dieses Werkzeug schreibt und versteht.
//
// Die 2 ist an das abgelöste Layout vergeben: Konfiguration im
// k-playbook-Verzeichnis, `_dist/`, `paths.*`. Eine Datei mit dieser Nummer
// beschreibt etwas anderes als das, was hier gelesen würde.
const SchemaVersion = "3"

// Config sind die Werte aus der K-PLAYBOOK.yaml, soweit sie derzeit gebraucht
// werden.
type Config struct {
	SchemaVersion string `json:"schemaVersion"`
	RepoRoot      string `json:"repoRoot"`
	VCS           string `json:"vcs"`
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
	// SchemaOutdated: die Fassung gehört zu einem abgelösten Modell.
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
	case "1", "2":
		return SchemaOutdated
	default:
		return SchemaNewer
	}
}

// LegacyModels beschreibt, wofür die abgelösten Fassungen standen. Wer eine
// solche Datei vor sich hat, braucht vor allem die Auskunft, welches Modell
// sie beschreibt — daran hängt, wo seine Inhalte liegen.
var LegacyModels = map[string]string{
	"1": "zentrale Basisinstallation unter ~/dev/k-playbook, paths.*, Projekteigenes im " +
		PlaybookDirName + "-Verzeichnis",
	"2": "Anker im " + PlaybookDirName + "-Verzeichnis, Installation unter _dist/, paths.*",
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
		return fmt.Errorf("%s hat schema_version %s und beschreibt ein abgelöstes Modell (%s); "+
			"dieses Werkzeug erwartet %s — %s",
			ConfigFileName, config.SchemaVersion, LegacyModels[config.SchemaVersion],
			SchemaVersion, resetHint)
	default:
		return fmt.Errorf("%s hat schema_version %s, dieses Werkzeug versteht %s — vermutlich ist die Installation älter als die Konfiguration; hilft `git pull` im %s-Verzeichnis nicht weiter, gehört die Datei zu einem anderen Projekt",
			ConfigFileName, config.SchemaVersion, SchemaVersion, PlaybookDirName)
	}
}

// resetHint nennt den Ausweg. Ohne ihn endete die Meldung bei der Diagnose:
// von Hand ist der Weg nicht zu erraten, und `CreateConfig` verweigert, solange
// die alte Datei liegt.
const resetHint = "`k-playbook gui` starten, der Block „Projektkonfiguration\" " +
	"sichert die alte Datei weg und legt sie neu an"

// ReadConfig liest die Konfiguration eines Projekts.
//
// Bewusst kein YAML-Parser: gelesen werden nur wenige Skalare, und der
// zeilenweise Zugriff lässt Kommentare, Reihenfolge und unbekannte Blöcke
// unangetastet, wenn später zurückgeschrieben wird.
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
		value = strings.TrimSpace(value)

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
	return config, nil
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
// Danach kommt der Ort des Binaries.
func Suggest() Suggestion {
	suggestion := Suggestion{}
	workdir, _ := os.Getwd()

	if workdir != "" {
		if root, ok := gitWorktreeRoot(workdir); ok {
			suggestion.ProjectCandidates = append(suggestion.ProjectCandidates, root)
		}
	}

	// Das Binary liegt in <X>/dist/; X ist die Installation. Ob X selbst das
	// Hauptverzeichnis ist oder eine Ebene darunter liegt, hängt daran, ob die
	// Installation geklont wurde oder das Repo selbst ist — beides ist möglich,
	// deshalb kommen beide Orte in die Auswahl.
	if install, ok := InstallDir(); ok {
		if filepath.Base(install) == PlaybookDirName {
			suggestion.ProjectCandidates = addUnique(suggestion.ProjectCandidates, filepath.Dir(install))
		}
		suggestion.ProjectCandidates = addUnique(suggestion.ProjectCandidates, install)
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

// InstallDir liefert das Verzeichnis der Installation.
//
// Vorrang hat, was der Wrapper sagt: er kennt seinen eigenen Ort und ist die
// einzige Stelle, die ihn noch kennt, seit das Binary auch aus dem Cache
// kommen kann. Die Ableitung aus dem Ort des Binaries bleibt Rückfall für
// direkt gestartete Binaries.
func InstallDir() (string, bool) {
	if dir := strings.TrimSpace(os.Getenv(InstallDirEnv)); dir != "" {
		if absolute, err := filepath.Abs(dir); err == nil {
			dir = absolute
		}
		if isDir(dir) {
			return dir, true
		}
	}

	exe, err := os.Executable()
	if err != nil {
		return "", false
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	// Das Binary liegt in <X>/dist/; X wäre die Installation. Für ein Binary
	// aus dem Cache (<cache>/bin/<version>/<binary>) trifft das nicht zu, und
	// <cache>/bin existiert trotzdem — ein bloßes isDir lieferte dann ein
	// falsches Verzeichnis statt eines Fehlers. Deshalb muss darunter auch
	// wirklich eine Installation liegen.
	dir := filepath.Dir(filepath.Dir(exe))
	if !isDir(dir) {
		return "", false
	}
	if !fileExists(filepath.Join(dir, BinDirName, WrapperName)) && !fileExists(filepath.Join(dir, ConfigFileName)) {
		return "", false
	}
	return dir, true
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
// wenn das Repo parallel zur Installation ausgecheckt ist.
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
		if !entry.IsDir() || name == PlaybookDirName || strings.HasPrefix(name, ".") {
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

// CreateConfig legt die K-PLAYBOOK.yaml im Hauptverzeichnis an.
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

	return os.WriteFile(path, []byte(renderConfig(repoRoot, vcs)), 0o644)
}

func renderConfig(repoRoot string, vcs string) string {
	return fmt.Sprintf(`# k-playbook
#
# Der Ort dieser Datei bestimmt das Hauptverzeichnis des Projekts.
# Die Installation liegt daneben unter %s/ und ist vollständig
# ersetzbar; projekteigene Dateien gehören nicht hinein.

schema_version: %s

project:
  # Ort des Projekt-Repositorys, relativ zu dieser Datei.
  repo_root: %s
  vcs: %s

%s
tools:
%s`, PlaybookDirName, SchemaVersion, repoRoot, vcs,
		remediationBlock(DefaultRemediationMode), ghBlock(DefaultGHStatus))
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
