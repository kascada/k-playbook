// Package versionsources liest die Quellenkonfiguration des Versionsinventars,
// k-playbook-local/version-sources.yaml.
//
// Es gibt genau diese eine Implementierung. Sie wird von zwei Seiten benutzt:
// vom Sammler des Inventars und von `k-playbook context`, das den Zustand der
// Datei ausgibt, damit Commands sie nicht selbst öffnen müssen. Zwei Leser
// derselben Datei wären zwei Auslegungen derselben Vertrauensgrenze.
//
// Das Paket liegt neben `project` statt darin, weil `project` es importiert und
// der Sammler es ebenfalls braucht; läge es in `project`, importierte der
// Sammler das ganze Projektpaket und `project` könnte den Sammler nie
// benutzen. Es hängt deshalb von nichts ab außer `yamllite`.
//
// Der Vertrag steht in docs/versionsinventar.md, Abschnitt
// „Quellenkonfiguration".
package versionsources

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// SchemaVersion ist die einzige Fassung, die dieses Werkzeug versteht. Eine
// andere bricht ab, statt Felder zu deuten, die etwas anderes bedeuten könnten
// — dieselbe Regel wie bei der K-PLAYBOOK.yaml.
const SchemaVersion = 1

// Envs ist die geschlossene Menge der Umgebungslabels.
var Envs = []string{"lokal", "dev", "devcontainer", "ci", "deployment"}

// Kinds ist die geschlossene Menge der Quellarten. `auto` bestimmt die Art am
// Dateinamen wie bei den Standardquellen.
var Kinds = []string{
	"auto", "python", "go", "node", "rust", "ruby", "php", "java", "elixir",
	"dockerfile", "compose", "devcontainer", "helm", "ci", "tool-versions",
}

// Source ist ein Eintrag aus `sources:`. Die Felder heißen wie die
// YAML-Schlüssel.
type Source struct {
	Path     string
	Kind     string
	Env      string
	Note     string
	Optional bool
	// Line ist die Zeile des Eintrags in der Konfigurationsdatei. Sie steht in
	// der Ablehnung, damit ein fehlerhafter Eintrag ohne Suche zu finden ist.
	Line int
	// Valid ist false, wenn `kind` oder `env` unbekannt sind. Der Eintrag bleibt
	// trotzdem in der Liste: die Kontextausgabe zeigt die Datei so, wie sie
	// dasteht. Der Sammler überspringt ihn und führt die Ablehnung sichtbar.
	Valid bool
}

// Rejection ist ein abgelehnter Eintrag. Ablehnungen sind sichtbar: eine
// Konfiguration, die zur Hälfte gilt und zur Hälfte stumm verschwindet, ist
// schlimmer als ein Fehler.
type Rejection struct {
	Path   string
	Line   int
	Reason string
}

// Config ist der gelesene Zustand der Datei.
type Config struct {
	// Path ist der absolute Pfad — auch wenn die Datei fehlt, damit klar ist,
	// wo sie hingehört.
	Path string
	// Present sagt, ob die Datei da ist. Fehlt sie, ist das kein Fehler: es
	// gelten die Standardquellen unterhalb der Projektwurzel.
	Present bool
	// SchemaVersion ist die deklarierte Fassung; 0, solange nichts gelesen wurde.
	SchemaVersion int
	// Roots sind die zusätzlich lesbaren Wurzeln, wortgleich wie in der Datei.
	// Die Projektwurzel steht nicht darin — sie ist immer erlaubt.
	Roots []string
	// Sources sind die konfigurierten Zusatzquellen in Dateireihenfolge.
	Sources []Source
	// Exclude sind die Muster, die bei der Standarderkennung übergangen werden,
	// wortgleich wie in der Datei. Sie wirken nur auf die Standardquellen: was
	// unter `sources:` ausdrücklich hingeschrieben ist, bleibt gelesen.
	Exclude []string
	// Rejections nennt die Einträge mit unbekanntem `kind` oder `env`.
	Rejections []Rejection
}

// Valid liefert die Einträge, mit denen der Sammler arbeiten darf.
func (c Config) Valid() []Source {
	valid := make([]Source, 0, len(c.Sources))
	for _, source := range c.Sources {
		if source.Valid {
			valid = append(valid, source)
		}
	}
	return valid
}

// Read liest die Datei.
//
// Der Rückgabefehler ist der abbrechende Fall — unlesbares YAML oder eine
// fremde schema_version. Beide Aufrufwege bekommen denselben Befund und
// entscheiden verschieden: der Erhebungslauf bricht ab, `k-playbook context`
// gibt den Fehler als Zustand aus und läuft weiter. So steht es im Vertrag,
// Abschnitt „Zustand in der Kontextausgabe": context steht am Anfang jedes
// Commands, und eine defekte Zusatzkonfiguration darf nicht jeden Command
// lahmlegen.
//
// Eine fehlende Datei ist kein Fehler.
func Read(path string) (Config, error) {
	config := Config{Path: path}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		config.Present = true
		return config, fmt.Errorf("%s: %w", path, err)
	}
	config.Present = true

	root, err := yamllite.Parse(data)
	if err != nil {
		return config, fmt.Errorf("%s ist kein lesbares YAML: %w", path, err)
	}
	if root.Kind != yamllite.Mapping {
		return config, fmt.Errorf("%s: erwartet werden Schlüssel auf oberster Ebene", path)
	}

	declared := root.Get("schema_version")
	version, ok := declared.Int()
	if !ok {
		return config, fmt.Errorf("%s hat keine lesbare schema_version; erwartet wird %d", path, SchemaVersion)
	}
	if version != SchemaVersion {
		return config, fmt.Errorf("%s hat schema_version %d, dieses Werkzeug versteht %d", path, version, SchemaVersion)
	}
	config.SchemaVersion = version

	for _, item := range root.Get("roots").List() {
		value := strings.TrimSpace(item.Str())
		if value == "" {
			continue
		}
		config.Roots = append(config.Roots, value)
	}

	for index, item := range root.Get("exclude").List() {
		value := strings.TrimSpace(item.Str())
		label := value
		if label == "" {
			label = fmt.Sprintf("exclude[%d]", index)
		}
		switch {
		case value == "":
			config.Rejections = append(config.Rejections, Rejection{Path: label,
				Line: item.At(), Reason: "leeres Ausschlussmuster"})
		case strings.HasPrefix(value, "/"), strings.Contains(value, ":\\"):
			// Ein Ausschluss beschreibt einen Bereich des Projekts. Absolut
			// hinge er vom Rechner ab und träfe auf einem anderen nichts —
			// stillschweigend, und genau das darf hier nicht passieren.
			config.Rejections = append(config.Rejections, Rejection{Path: label,
				Line: item.At(), Reason: "Ausschlussmuster muss relativ zur Projektwurzel sein"})
		default:
			config.Exclude = append(config.Exclude, value)
		}
	}

	for index, item := range root.Get("sources").List() {
		source, rejection := readSource(item, index)
		if rejection != nil {
			config.Rejections = append(config.Rejections, *rejection)
		}
		config.Sources = append(config.Sources, source)
	}
	return config, nil
}

func readSource(item *yamllite.Node, index int) (Source, *Rejection) {
	source := Source{
		Path:  strings.TrimSpace(item.Get("path").Str()),
		Kind:  strings.TrimSpace(item.Get("kind").Str()),
		Env:   strings.TrimSpace(item.Get("env").Str()),
		Note:  strings.TrimSpace(item.Get("note").Str()),
		Line:  item.At(),
		Valid: true,
	}
	if strings.EqualFold(strings.TrimSpace(item.Get("optional").Str()), "true") {
		source.Optional = true
	}

	label := source.Path
	if label == "" {
		label = fmt.Sprintf("sources[%d]", index)
	}

	switch {
	case source.Path == "":
		source.Valid = false
		return source, &Rejection{Path: label, Line: source.Line,
			Reason: "kein `path` angegeben"}
	case source.Kind == "":
		source.Valid = false
		return source, &Rejection{Path: label, Line: source.Line,
			Reason: "kein `kind` angegeben; gültig sind " + list(Kinds)}
	case !contains(Kinds, source.Kind):
		source.Valid = false
		return source, &Rejection{Path: label, Line: source.Line,
			Reason: fmt.Sprintf("unbekannte Quellart %q; gültig sind %s", source.Kind, list(Kinds))}
	case source.Env == "":
		source.Valid = false
		return source, &Rejection{Path: label, Line: source.Line,
			Reason: "kein `env` angegeben; gültig sind " + list(Envs)}
	case !contains(Envs, source.Env):
		source.Valid = false
		return source, &Rejection{Path: label, Line: source.Line,
			Reason: fmt.Sprintf("unbekanntes Umgebungslabel %q; gültig sind %s", source.Env, list(Envs))}
	}
	return source, nil
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func list(values []string) string {
	return "`" + strings.Join(values, "`, `") + "`"
}
