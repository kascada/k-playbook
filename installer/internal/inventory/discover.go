package inventory

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// skippedDirs sind die Verzeichnisse, die beim Suchen der Standardquellen nicht
// betreten werden.
//
// Der Vertrag sagt „unterhalb der Projektwurzel"; ohne diese Liste hieße das
// auch: jedes Manifest jeder heruntergeladenen Abhängigkeit. Ein node_modules
// bringt Zehntausende package.json mit, und keine davon beantwortet die Frage,
// die das Inventar stellt — der Vertrag führt aus Lockfiles ausdrücklich nur
// direkte Abhängigkeiten. Übersprungen wird deshalb, was ein Werkzeug befüllt
// hat, nie ein Verzeichnis mit gepflegtem Inhalt.
var skippedDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true,
	"node_modules": true, "vendor": true, "bower_components": true,
	".venv": true, "venv": true, "__pycache__": true, ".tox": true, ".nox": true,
	".mypy_cache": true, ".pytest_cache": true, ".ruff_cache": true,
	"target": true, ".gradle": true, ".terraform": true, ".next": true, ".cache": true,
	"_build": true, "deps": true,
}

func skippedDir(name string) bool { return skippedDirs[name] }

// InstallationDirName ist der Name der k-playbook-Installation innerhalb eines
// Projekts. Er ist fest und steht so im k-playbook-Format; `project` bindet ihn
// als PlaybookDirName ein zweites Mal an denselben Wert.
//
// Der Sammler importiert `project` bewusst nicht: `project` benutzt
// `versionsources` und soll später den Sammler benutzen können, ohne dass ein
// Zyklus entsteht. Deshalb steht der Name hier noch einmal — die einzige
// Doppelung, und sie ist an eine Konstante gebunden, die sich nicht ändert.
const InstallationDirName = "k-playbook"

// Herkunft eines Ausschlusses.
const (
	// ExclusionInstallation ist die feste Regel: die Installation neben dem
	// Projekt ist keine Standardquelle.
	ExclusionInstallation = "installation"
	// ExclusionConfigured ist ein Muster aus `exclude:` der Quellenkonfiguration.
	ExclusionConfigured = "configured"
)

// installationReason begründet die feste Regel in der Inventardatei. Sie steht
// dort wörtlich, damit ein Leser den Ausschluss nicht im Werkzeug nachschlagen
// muss.
const installationReason = "Clone des Werkzeugs; wird bei jedem Update ersetzt und sagt nichts über dieses Projekt"

// excluder entscheidet, was bei der Standarderkennung übergangen wird, und
// zählt dabei mit.
//
// Übergangen wird nicht still: jede Regel steht mit ihrer Zahl im Inventar.
// Deshalb wird auch nicht abgeschnitten, sondern weitergelaufen und gezählt —
// „hier wurde nichts gefunden" und „hier wurde nicht gesucht" sind zwei
// verschiedene Aussagen.
type excluder struct {
	rules []*Exclusion
}

func newExcluder(patterns []string) *excluder {
	rules := []*Exclusion{{
		Pattern: InstallationDirName + "/**",
		Origin:  ExclusionInstallation,
		Reason:  installationReason,
	}}
	for _, pattern := range patterns {
		rules = append(rules, &Exclusion{
			Pattern: pattern,
			Origin:  ExclusionConfigured,
			Reason:  "in `exclude:` der Quellenkonfiguration ausgenommen",
		})
	}
	return &excluder{rules: rules}
}

// excluded prüft einen projektrelativen Pfad und zählt den Treffer.
func (e *excluder) excluded(relative string) bool {
	for _, rule := range e.rules {
		if matchExclude(rule.Pattern, relative) {
			rule.Skipped++
			return true
		}
	}
	return false
}

// exclusions liefert die Regeln in Dateireihenfolge, die feste zuerst.
func (e *excluder) exclusions() []Exclusion {
	out := make([]Exclusion, 0, len(e.rules))
	for _, rule := range e.rules {
		out = append(out, *rule)
	}
	return out
}

// matchExclude prüft ein Muster gegen einen projektrelativen Pfad.
//
// Verglichen wird segmentweise: `*` steht für beliebig viele Zeichen innerhalb
// eines Segments, `**` für beliebig viele Segmente. Ein Muster ohne Wildcard
// trifft den Pfad selbst und alles darunter — `build/x` schließt `build/x/y`
// mit ein, aber nicht `build/xy`.
func matchExclude(pattern string, relative string) bool {
	pattern = strings.Trim(strings.TrimPrefix(strings.TrimSpace(pattern), "./"), "/")
	if pattern == "" {
		return false
	}
	patternSegments := strings.Split(pattern, "/")
	pathSegments := strings.Split(relative, "/")
	if !strings.ContainsAny(pattern, "*?[") {
		// Präfixregel: das Muster trifft den Pfad selbst und alles darunter.
		if len(pathSegments) < len(patternSegments) {
			return false
		}
		for index, segment := range patternSegments {
			if pathSegments[index] != segment {
				return false
			}
		}
		return true
	}
	return matchSegments(patternSegments, pathSegments)
}

// matchSegments vergleicht Muster- und Pfadsegmente, `**` eingeschlossen.
func matchSegments(pattern []string, path []string) bool {
	if len(pattern) == 0 {
		return len(path) == 0
	}
	if pattern[0] == "**" {
		// `**` am Ende trifft alles Verbleibende, auch nichts.
		for index := 0; index <= len(path); index++ {
			if matchSegments(pattern[1:], path[index:]) {
				return true
			}
		}
		return false
	}
	if len(path) == 0 {
		return false
	}
	ok, err := filepath.Match(pattern[0], path[0])
	if err != nil || !ok {
		return false
	}
	return matchSegments(pattern[1:], path[1:])
}

// candidate ist eine gefundene Quelle vor dem Lesen.
type candidate struct {
	// Requested ist der Pfad, wie er angefragt wurde — relativ zur
	// Projektwurzel bei den Standardquellen, wortgleich aus der Konfiguration
	// bei den Zusatzquellen.
	Requested string
	Kind      string
	Env       string
	// EnvOrigin ist `default` oder `configured`.
	EnvOrigin string
	Note      string
	Optional  bool
	// Configured sagt, ob die Quelle aus der Quellenkonfiguration stammt.
	Configured bool
}

// discoverDefaults sucht die Standardquellen unterhalb der Projektwurzel.
//
// Gelaufen wird mit fs.WalkDir, das Symlinks auf Verzeichnisse nicht verfolgt;
// jede gefundene Datei geht trotzdem einzeln durch die Vertrauensgrenze, weil
// eine Datei selbst ein Symlink nach draußen sein kann.
func discoverDefaults(root string, skip *excluder) []candidate {
	var found []candidate
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			// Ein nicht lesbares Verzeichnis ist kein Grund abzubrechen; was
			// darin liegt, fehlt dann und taucht auch nicht als Quelle auf.
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if path != root && skippedDir(entry.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		relative = filepath.ToSlash(relative)
		kind, ok := detectKind(relative)
		if !ok {
			return nil
		}
		if skip.excluded(relative) {
			return nil
		}
		found = append(found, candidate{
			Requested: relative,
			Kind:      kind,
			Env:       defaultEnv(kind, relative),
			EnvOrigin: ContextDefault,
		})
		return nil
	})
	return found
}

// detectKind bestimmt die Quellart am Namen. Das ist zugleich das Verhalten von
// `kind: auto` in der Quellenkonfiguration — es gibt nur diese eine Zuordnung.
func detectKind(relative string) (string, bool) {
	base := relative
	if index := strings.LastIndex(relative, "/"); index >= 0 {
		base = relative[index+1:]
	}
	lower := strings.ToLower(base)
	extension := strings.ToLower(filepath.Ext(base))
	isYAML := extension == ".yml" || extension == ".yaml"

	switch {
	case containsDir(relative, ".github/workflows") && isYAML:
		return KindCI, true
	case base == ".gitlab-ci.yml" || base == ".gitlab-ci.yaml":
		return KindCI, true
	case containsDir(relative, ".gitlab") && isYAML:
		return KindCI, true
	case base == "devcontainer.json" || base == ".devcontainer.json":
		return KindDevcontainer, true
	case base == "Dockerfile" || strings.HasPrefix(base, "Dockerfile.") || strings.HasSuffix(base, ".Dockerfile"):
		return KindDockerfile, true
	case isYAML && (strings.HasPrefix(lower, "docker-compose") || strings.HasPrefix(lower, "compose")):
		return KindCompose, true
	case base == "Chart.yaml" || base == "Chart.lock":
		return KindHelm, true
	case isYAML && strings.HasPrefix(lower, "values"):
		return KindHelm, true
	case isPythonSource(base):
		return KindPython, true
	case base == "go.mod" || base == ".go-version":
		return KindGo, true
	case isNodeSource(base):
		return KindNode, true
	case base == "Cargo.toml" || base == "Cargo.lock":
		return KindRust, true
	case base == "Gemfile" || base == "Gemfile.lock" || base == ".ruby-version":
		return KindRuby, true
	case base == "composer.json" || base == "composer.lock":
		return KindPHP, true
	case base == "pom.xml" || base == "build.gradle" || base == "build.gradle.kts":
		return KindJava, true
	case base == "mix.exs" || base == "mix.lock":
		return KindElixir, true
	case base == ".tool-versions":
		return KindToolVersions, true
	}
	return "", false
}

func isPythonSource(base string) bool {
	switch base {
	case "pyproject.toml", "setup.py", "setup.cfg", "Pipfile", "Pipfile.lock",
		"poetry.lock", "uv.lock", ".python-version":
		return true
	}
	if strings.HasSuffix(base, ".txt") {
		return strings.HasPrefix(base, "requirements") || strings.HasPrefix(base, "constraints")
	}
	return false
}

func isNodeSource(base string) bool {
	switch base {
	case "package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
		".nvmrc", ".node-version":
		return true
	}
	return false
}

// defaultEnv ist die Tabelle „Quelle → Default-Label" aus dem Vertrag.
//
// Der Ort schlägt die Quellart: alles unterhalb von `.devcontainer/` trägt
// `devcontainer`, auch ein Dockerfile oder eine Compose-Datei. Genau das meint
// die Zeile „`.devcontainer/**` → devcontainer" und der Satz, dass die Funde
// einer von dort verwiesenen Datei das Label `devcontainer` bekommen.
func defaultEnv(kind string, relative string) string {
	if inDevcontainer(relative) {
		return EnvDevcontainer
	}
	switch kind {
	case KindDockerfile, KindHelm:
		return EnvDeployment
	case KindCompose:
		return EnvDev
	case KindCI:
		return EnvCI
	case KindDevcontainer:
		return EnvDevcontainer
	default:
		return EnvLokal
	}
}

func inDevcontainer(relative string) bool {
	return containsDir(relative, ".devcontainer") || relative == ".devcontainer.json" ||
		strings.HasSuffix(relative, "/.devcontainer.json")
}

// containsDir prüft, ob ein Verzeichnis im Pfad vorkommt. Geprüft wird
// segmentweise und nicht als Zeichenkette, damit `auto` in der
// Quellenkonfiguration einen absoluten Pfad genauso einordnet wie die
// Standarderkennung einen projektrelativen.
func containsDir(relative string, dir string) bool {
	return strings.HasPrefix(relative, dir+"/") || strings.Contains(relative, "/"+dir+"/")
}
