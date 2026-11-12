// Package versionsources liest die Quellenkonfiguration des Versionsinventars,
// das Dokument keph:/k-playbook/inventory.md in der Collection des Projekts
// (Task 144). Die Konfiguration steht im Frontmatter des Dokuments, der Rumpf
// erklärt sie; die alte Datei unter k-playbook-local/ liest seitdem kein
// Programmteil mehr als Konfiguration (harter Wechsel, /k-upgrade zieht um).
//
// Es gibt genau diese eine Implementierung. Sie wird von mehreren Seiten
// benutzt: vom Sammler des Inventars, von `k-playbook context`, das den
// Zustand des Dokuments ausgibt, damit Commands es nicht selbst öffnen müssen,
// von der Gesundheitsprüfung und von der Oberfläche. Zwei Leser derselben
// Konfiguration wären zwei Auslegungen derselben Vertrauensgrenze.
//
// Das Paket liegt neben `project` statt darin, weil `project` es importiert und
// der Sammler es ebenfalls braucht; läge es in `project`, importierte der
// Sammler das ganze Projektpaket und `project` könnte den Sammler nie
// benutzen. Es hängt deshalb von nichts ab außer `yamllite` und dem Typ der
// Leseantwort aus `kephalaion`; den Zugang zur Collection öffnet der Aufrufer.
//
// Der Vertrag steht in docs/version-inventory.md, Abschnitt
// „Source Configuration".
package versionsources

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

const (
	// DocName ist der Name des Dokuments in der Collection des Projekts, ohne
	// führenden Schrägstrich, so wie Kephalaion read ihn erwartet. Es liegt an
	// der Wurzel von k-playbook/, neben den Sorten der projekteigenen Ebene
	// (Task 143), aber in keiner davon — die Überlagerung greift es nicht auf.
	// Nicht unter docs/: dort würde es indiziert und als Doku gefunden.
	DocName = "k-playbook/inventory.md"
	// DocRef ist derselbe Ort als Verweis im Präfixschema; er steht in jeder
	// Meldung, die die Konfiguration nennt.
	DocRef = "keph:/" + DocName
)

// SchemaVersion ist die einzige Fassung, die dieses Werkzeug versteht. Eine
// andere bricht ab, statt Felder zu deuten, die etwas anderes bedeuten
// könnten — dieselbe Regel wie bei der K-PLAYBOOK.yaml.
//
// Fassung 3 kam mit dem Umzug in die Collection (Task 144): `roots` gibt es
// nicht mehr, gelesen wird nur unterhalb der Projektwurzel. Die Fassungen 1
// und 2 gehörten zur alten Datei unter k-playbook-local/; ein Dokument mit
// ihnen wäre eines, das jemand von Hand mit der alten Bedeutung geschrieben
// hat, und wird abgelehnt.
const SchemaVersion = 3

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
	// Line ist die Zeile des Eintrags im Dokument. Sie steht in
	// der Ablehnung, damit ein fehlerhafter Eintrag ohne Suche zu finden ist.
	Line int
	// Valid ist false, wenn `kind` oder `env` unbekannt sind oder `path`
	// absolut ist. Der Eintrag bleibt
	// trotzdem in der Liste: die Kontextausgabe zeigt das Dokument so, wie es
	// dasteht. Der Sammler überspringt ihn und führt die Ablehnung sichtbar.
	Valid bool
}

// HelmValue ist ein Eintrag aus `helm_values:` — ein Wert in einer
// values-Datei, der keine Image-Referenz ist und deshalb ausdrücklich benannt
// wird. Die Felder heißen wie die YAML-Schlüssel.
type HelmValue struct {
	// Path ist die values-Datei oder ein Glob, wie `path` in `sources`.
	Path string
	// Key ist der Punktpfad zum Wert, Listenindex in eckigen Klammern.
	Key string
	// Item ist der Gegenstand als `container/<name>`.
	Item string
	// Line ist die Zeile des Eintrags im Dokument.
	Line int
	// Valid ist false, wenn der Eintrag abgelehnt wurde. Wie bei `sources`
	// bleibt der Eintrag in der Liste, damit die Kontextausgabe das Dokument
	// zeigt, wie es dasteht.
	Valid bool
}

// ItemName ist der Name aus `container/<name>`, ohne Ökosystem.
func (h HelmValue) ItemName() string {
	return strings.TrimPrefix(h.Item, HelmValueEcosystem+"/")
}

// HelmValueEcosystem ist das einzige Ökosystem, das ein konfigurierter
// Helm-Wert tragen darf: es gelten die Pin-Regeln der Container-Tags.
const HelmValueEcosystem = "container"

// KeyStep ist ein Schritt eines Schlüsselpfads: ein Abbildungsschlüssel und
// danach null oder mehr Listenindizes.
type KeyStep struct {
	Key     string
	Indexes []int
}

// ParseKeyPath zerlegt `a.b[0].c`. Ein leeres Segment, ein Segment ohne Namen
// oder ein ungültiger Index machen den Pfad ungültig; ein Schlüssel, der selbst
// einen Punkt enthält, ist nicht adressierbar.
func ParseKeyPath(key string) ([]KeyStep, bool) {
	if strings.TrimSpace(key) == "" {
		return nil, false
	}
	var steps []KeyStep
	for _, segment := range strings.Split(key, ".") {
		name := segment
		var indexes []int
		if open := strings.Index(segment, "["); open >= 0 {
			name = segment[:open]
			rest := segment[open:]
			for rest != "" {
				if rest[0] != '[' {
					return nil, false
				}
				closing := strings.Index(rest, "]")
				if closing < 2 {
					return nil, false
				}
				index, err := strconv.Atoi(rest[1:closing])
				if err != nil || index < 0 || strings.TrimSpace(rest[1:closing]) != rest[1:closing] {
					return nil, false
				}
				indexes = append(indexes, index)
				rest = rest[closing+1:]
			}
		}
		if name == "" || strings.ContainsAny(name, "[]") || strings.TrimSpace(name) != name {
			return nil, false
		}
		steps = append(steps, KeyStep{Key: name, Indexes: indexes})
	}
	return steps, true
}

// Rejection ist ein abgelehnter Eintrag. Ablehnungen sind sichtbar: eine
// Konfiguration, die zur Hälfte gilt und zur Hälfte stumm verschwindet, ist
// schlimmer als ein Fehler.
type Rejection struct {
	Path   string
	Line   int
	Reason string
}

// Config ist der gelesene Zustand des Dokuments.
type Config struct {
	// Ref ist der Ort des Dokuments als keph:-Verweis (DocRef) — auch wenn es
	// fehlt, damit klar ist, wo es hingehört.
	Ref string
	// Present sagt, ob das Dokument da ist. Fehlt es, ist das kein Fehler: es
	// gelten die Standardquellen unterhalb der Projektwurzel.
	Present bool
	// SchemaVersion ist die deklarierte Fassung; 0, solange nichts gelesen wurde.
	SchemaVersion int
	// Sources sind die konfigurierten Zusatzquellen in Dokumentreihenfolge.
	Sources []Source
	// Exclude sind die Muster, die bei der Standarderkennung übergangen werden,
	// wortgleich wie im Dokument. Sie wirken nur auf die Standardquellen: was
	// unter `sources:` ausdrücklich hingeschrieben ist, bleibt gelesen.
	Exclude []string
	// HelmValues sind die konfigurierten Helm-Werte in Dokumentreihenfolge.
	HelmValues []HelmValue
	// Rejections nennt die abgelehnten Einträge und Muster: `sources` mit
	// unbekanntem `kind` oder `env` oder absolutem Pfad, ungültige
	// `exclude`-Muster und ungültige `helm_values`-Einträge.
	Rejections []Rejection
}

// ValidHelmValues liefert die Helm-Werte, die der Sammler anwenden darf.
func (c Config) ValidHelmValues() []HelmValue {
	valid := make([]HelmValue, 0, len(c.HelmValues))
	for _, value := range c.HelmValues {
		if value.Valid {
			valid = append(valid, value)
		}
	}
	return valid
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

// Reader ist der eine Aufruf an Kephalaion, den das Lesen braucht: read in
// der Collection des Projekts. *kephalaion.Client erfüllt ihn, ebenso der
// Zugang des Inventars (inventory.Store); Tests setzen kephtest.Store ein.
type Reader interface {
	Read(ctx context.Context, name string) (kephalaion.ReadResult, error)
}

// UnavailableError heißt: Die Collection war nicht erreichbar, über das
// Dokument ist nichts bekannt. Das ist ein eigener Zustand, weder „fehlt“
// noch „defekt“ — und kein Anlass, auf die Platte oder still auf die
// Standardquellen zurückzufallen.
type UnavailableError struct {
	Err error
}

func (e *UnavailableError) Error() string {
	return fmt.Sprintf("%s ist nicht erreichbar: %v", DocRef, e.Err)
}

func (e *UnavailableError) Unwrap() error { return e.Err }

// IsUnavailable meldet, ob err heißt: Die Collection war nicht erreichbar.
func IsUnavailable(err error) bool {
	var unavailable *UnavailableError
	return errors.As(err, &unavailable)
}

// Load liest das Dokument über reader und wertet es mit Parse aus.
//
// Gelesen wird der Rohtext des Dokuments, nicht das von Kephalaion geparste
// Frontmatter: So gilt derselbe Parser wie für jede andere Konfiguration,
// Kommentare stören nicht, und Zeilenangaben in Meldungen beziehen sich auf
// das Dokument.
//
// Der Rückgabefehler ist der abbrechende Fall — Kephalaion nicht erreichbar
// (UnavailableError), kein lesbares Frontmatter, eine fremde schema_version
// oder ein Schlüssel `roots`. Alle Aufrufwege bekommen denselben Befund und
// entscheiden verschieden: der Erhebungslauf bricht ab, `k-playbook context`
// gibt den Fehler als Zustand aus und läuft weiter. So steht es im Vertrag,
// Abschnitt „State in the Context Output": context steht am Anfang jedes
// Commands, und eine defekte Zusatzkonfiguration darf nicht jeden Command
// lahmlegen.
//
// Ein fehlendes Dokument ist kein Fehler.
func Load(ctx context.Context, reader Reader) (Config, error) {
	config := Config{Ref: DocRef}
	if reader == nil {
		return config, &UnavailableError{Err: errors.New("kein Zugang zur Collection des Projekts")}
	}
	read, err := reader.Read(ctx, DocName)
	if err != nil {
		return config, &UnavailableError{Err: err}
	}
	switch read.Kind {
	case kephalaion.ReadKindNone:
		return config, nil
	case kephalaion.ReadKindDirectory:
		config.Present = true
		return config, fmt.Errorf("%s ist ein Verzeichnis, kein Dokument", DocRef)
	}
	content := ""
	if read.Content != nil {
		content = *read.Content
	}
	return Parse([]byte(content))
}

// Parse wertet den Text des Dokuments aus: die Konfiguration im Frontmatter
// zwischen den beiden `---`-Zeilen am Anfang. Zeilenangaben zählen ab der
// ersten Zeile des Dokuments, nicht des Frontmatters.
func Parse(data []byte) (Config, error) {
	config := Config{Ref: DocRef, Present: true}

	block, ok := yamllite.Frontmatter(data)
	if !ok {
		return config, fmt.Errorf("%s hat kein Frontmatter; die Konfiguration steht zwischen zwei Zeilen --- am Anfang des Dokuments", DocRef)
	}
	root, err := yamllite.Parse([]byte(block))
	if err != nil {
		return config, fmt.Errorf("%s: das Frontmatter ist kein lesbares YAML (Zeilen ab dem Frontmatter gezählt): %w", DocRef, err)
	}
	if root.Kind != yamllite.Mapping {
		return config, fmt.Errorf("%s: im Frontmatter werden Schlüssel auf oberster Ebene erwartet", DocRef)
	}
	// Das Frontmatter beginnt in Zeile 2 des Dokuments.
	line := func(node *yamllite.Node) int {
		if node.At() == 0 {
			return 0
		}
		return node.At() + 1
	}

	declared := root.Get("schema_version")
	version, ok := declared.Int()
	if !ok {
		return config, fmt.Errorf("%s hat keine lesbare schema_version; erwartet wird %d", DocRef, SchemaVersion)
	}
	if version != SchemaVersion {
		return config, fmt.Errorf("%s hat schema_version %d, dieses Werkzeug versteht %d", DocRef, version, SchemaVersion)
	}
	config.SchemaVersion = version

	// roots gibt es nicht mehr (Task 144): rechnerabhängig, und in einer
	// gemeinsamen Collection bestimmte jeder mit Schreibrecht, was auf fremden
	// Rechnern gelesen wird. Abgelehnt wird jeder Schlüssel roots, auch ein
	// leerer — still übergangen hieße, ein Dokument gälte anders, als es
	// dasteht.
	if node, present := root.Fields["roots"]; present {
		return config, fmt.Errorf("%s, Zeile %d: den Schlüssel roots gibt es seit schema_version %d nicht mehr; "+
			"gelesen wird nur unterhalb der Projektwurzel — den Schlüssel entfernen", DocRef, line(node), SchemaVersion)
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
				Line: line(item), Reason: "leeres Ausschlussmuster"})
		case isAbsolute(value):
			// Ein Ausschluss beschreibt einen Bereich des Projekts. Absolut
			// hinge er vom Rechner ab und träfe auf einem anderen nichts —
			// stillschweigend, und genau das darf hier nicht passieren.
			config.Rejections = append(config.Rejections, Rejection{Path: label,
				Line: line(item), Reason: "Ausschlussmuster muss relativ zur Projektwurzel sein"})
		default:
			config.Exclude = append(config.Exclude, value)
		}
	}

	for index, item := range root.Get("sources").List() {
		source, rejection := readSource(item, index, line(item))
		if rejection != nil {
			config.Rejections = append(config.Rejections, *rejection)
		}
		config.Sources = append(config.Sources, source)
	}

	for index, item := range root.Get("helm_values").List() {
		value, rejection := readHelmValue(item, index, line(item))
		if rejection != nil {
			config.Rejections = append(config.Rejections, *rejection)
		}
		config.HelmValues = append(config.HelmValues, value)
	}
	return config, nil
}

// isAbsolute erkennt einen absoluten Pfad, auch einen von Windows: Die
// Konfiguration liegt in einer gemeinsamen Collection und gilt auf jedem
// Rechner, ein absoluter Pfad nur auf einem.
func isAbsolute(value string) bool {
	return strings.HasPrefix(value, "/") || strings.HasPrefix(value, "\\") || strings.Contains(value, ":\\") ||
		(len(value) >= 3 && value[1] == ':' && (value[2] == '/' || value[2] == '\\'))
}

func readHelmValue(item *yamllite.Node, index int, line int) (HelmValue, *Rejection) {
	value := HelmValue{
		Path:  strings.TrimSpace(item.Get("path").Str()),
		Key:   strings.TrimSpace(item.Get("key").Str()),
		Item:  strings.TrimSpace(item.Get("item").Str()),
		Line:  line,
		Valid: true,
	}
	label := value.Path
	if label == "" {
		label = fmt.Sprintf("helm_values[%d]", index)
	}
	reject := func(reason string) (HelmValue, *Rejection) {
		value.Valid = false
		return value, &Rejection{Path: label, Line: value.Line, Reason: reason}
	}
	switch {
	case value.Path == "":
		return reject("kein `path` angegeben")
	case isAbsolute(value.Path):
		return reject("absoluter Pfad; gelesen wird nur unterhalb der Projektwurzel, `path` relativ zu ihr angeben")
	case value.Key == "":
		return reject("kein `key` angegeben")
	case value.Item == "":
		return reject("kein `item` angegeben; erwartet wird `container/<name>`")
	}
	if _, ok := ParseKeyPath(value.Key); !ok {
		return reject(fmt.Sprintf("ungültiger Schlüsselpfad %q; erwartet wird ein Punktpfad wie `redis.standalone.tag`, Listenindex in eckigen Klammern", value.Key))
	}
	ecosystem, name, found := strings.Cut(value.Item, "/")
	if !found || ecosystem != HelmValueEcosystem || strings.TrimSpace(name) == "" {
		return reject(fmt.Sprintf("ungültiger Gegenstand %q; erwartet wird `container/<name>`", value.Item))
	}
	return value, nil
}

func readSource(item *yamllite.Node, index int, line int) (Source, *Rejection) {
	source := Source{
		Path:  strings.TrimSpace(item.Get("path").Str()),
		Kind:  strings.TrimSpace(item.Get("kind").Str()),
		Env:   strings.TrimSpace(item.Get("env").Str()),
		Note:  strings.TrimSpace(item.Get("note").Str()),
		Line:  line,
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
	case isAbsolute(source.Path):
		// Eine absolute Quelle gälte nur auf einem Rechner und könnte aus einer
		// gemeinsamen Collection heraus bestimmen, was auf fremden Rechnern
		// gelesen wird. Die Vertrauensgrenze ist allein die Projektwurzel.
		source.Valid = false
		return source, &Rejection{Path: label, Line: source.Line,
			Reason: "absoluter Pfad; gelesen wird nur unterhalb der Projektwurzel, `path` relativ zu ihr angeben"}
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
