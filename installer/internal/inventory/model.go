// Package inventory erhebt das Versionsinventar eines Projekts: es findet die
// deklarativen Quellen, liest sie, normalisiert die Funde, bündelt
// Abweichungen und rendert die lesbare Inventardatei.
//
// Der Vertrag steht in docs/versionsinventar.md und ist verbindlich. Was hier
// steht, formuliert ihn nicht neu, sondern setzt ihn um; wo dieser Code und der
// Vertrag auseinandergingen, gilt der Vertrag.
//
// Drei Grenzen halten das Paket zusammen:
//
//   - Es liest nur Dateien. Kein Netz, keine Installation, kein Ausführen eines
//     gefundenen Werkzeugs.
//   - Jede Datei wird durch die Vertrauensgrenze in trust.go geöffnet, und zwar
//     ausschließlich über Boundary.ReadFile. Jeder Aufrufweg — Subkommando,
//     Command, später die Web-API — geht damit durch dieselbe Prüfung.
//   - Nichts verschwindet still. Was abgelehnt, nicht gefunden oder nicht
//     gelesen werden konnte, steht als Ablehnung oder Hinweis im Ergebnis.
package inventory

// Pin ist die Pin-Art. Die ersten drei Werte sind die aus
// commands/_docs/tools.md und behalten dort ihre Bedeutung; die letzten drei
// sind die Erweiterung aus dem Vertrag. Eine zweite Taxonomie entsteht nicht.
const (
	PinExact    = "exact"
	PinRange    = "range"
	PinFloating = "floating"
	PinDigest   = "digest"
	PinLocal    = "local"
	PinUnknown  = "unknown"
)

// Die Umgebungslabels sind eine geschlossene Menge. Die Reihenfolge hier ist
// zugleich die Reihenfolge der Abschnitte in der Inventardatei.
const (
	EnvLokal        = "lokal"
	EnvDev          = "dev"
	EnvDevcontainer = "devcontainer"
	EnvCI           = "ci"
	EnvDeployment   = "deployment"
)

// EnvOrder ist die feste Abschnittsreihenfolge der Inventardatei.
var EnvOrder = []string{EnvLokal, EnvDev, EnvDevcontainer, EnvCI, EnvDeployment}

// Was ein Gegenstand ist (`kindOfThing` im Vertrag).
const (
	ThingPackage         = "package"
	ThingTool            = "tool"
	ThingRuntime         = "runtime"
	ThingImage           = "image"
	ThingChart           = "chart"
	ThingChartDependency = "chart-dependency"
	ThingAction          = "action"
)

// Die Ökosysteme. `runtime` ist eines davon: eine Sprachversion aus
// `.python-version` gehört in keine Paketwelt.
const (
	EcoPython    = "python"
	EcoGo        = "go"
	EcoNode      = "node"
	EcoRust      = "rust"
	EcoRuby      = "ruby"
	EcoPHP       = "php"
	EcoJava      = "java"
	EcoElixir    = "elixir"
	EcoContainer = "container"
	EcoHelm      = "helm"
	EcoCI        = "ci"
	EcoRuntime   = "runtime"
)

// Die Quellarten. Sie sind zugleich die gültigen Werte von `kind` in der
// Quellenkonfiguration; `auto` bestimmt die Art am Dateinamen.
const (
	KindAuto         = "auto"
	KindPython       = "python"
	KindGo           = "go"
	KindNode         = "node"
	KindRust         = "rust"
	KindRuby         = "ruby"
	KindPHP          = "php"
	KindJava         = "java"
	KindElixir       = "elixir"
	KindDockerfile   = "dockerfile"
	KindCompose      = "compose"
	KindDevcontainer = "devcontainer"
	KindHelm         = "helm"
	KindCI           = "ci"
	KindToolVersions = "tool-versions"
)

// Herkunft des Umgebungslabels.
const (
	ContextDefault    = "default"
	ContextConfigured = "configured"
)

// Entry ist genau eine Aussage aus genau einer Quelle. Zwei Quellen, die
// dasselbe sagen, sind zwei Einträge — zusammengeführt wird erst beim Rendern.
//
// Die Feldnamen und ihre Bedeutung stehen im Vertrag, Abschnitt „Datenmodell
// einer Inventarzeile".
type Entry struct {
	Ecosystem         string `json:"ecosystem"`
	Name              string `json:"name"`
	KindOfThing       string `json:"kindOfThing"`
	Version           string `json:"version"`
	VersionNormalized string `json:"versionNormalized,omitempty"`
	Pin               string `json:"pin"`
	Digest            string `json:"digest,omitempty"`
	Context           string `json:"context"`
	ContextOrigin     string `json:"contextOrigin"`
	Scope             string `json:"scope,omitempty"`
	SourceFile        string `json:"sourceFile"`
	SourceKey         string `json:"sourceKey"`
	SourceLine        int    `json:"sourceLine,omitempty"`
	Group             string `json:"group"`
	Deviation         string `json:"deviation,omitempty"`
	Note              string `json:"note,omitempty"`
}

// Die beiden Arten einer Abweichung.
const (
	// DeviationConflicting: dieselbe Umgebung sagt Verschiedenes. Das wirft eine
	// Frage auf — Manifest gegen Lockfile, zwei Compose-Dateien derselben
	// Umgebung, Chart.yaml gegen Chart.lock.
	DeviationConflicting = "widersprüchlich"
	// DeviationEnvironmental: verschiedene Umgebungen sagen Verschiedenes. Das
	// ist der Normalfall und meist Absicht.
	DeviationEnvironmental = "umgebungsbedingt"
)

// Deviation bündelt die Aussagen eines Gegenstands, die auseinandergehen. Sie
// wird nie aufgelöst, zusammengefasst oder auf einen „richtigen" Wert
// reduziert.
type Deviation struct {
	// Group ist zugleich die Kennung, auf die Entry.Deviation zeigt.
	Group   string  `json:"group"`
	Art     string  `json:"art"`
	Entries []Entry `json:"entries"`
}

// SourceRead ist eine tatsächlich ausgewertete Quelle.
type SourceRead struct {
	File    string `json:"file"`
	Kind    string `json:"kind"`
	Env     string `json:"env"`
	Entries int    `json:"entries"`
	// Configured sagt, ob die Quelle aus der Quellenkonfiguration stammt.
	Configured bool `json:"configured,omitempty"`
	// Note ist der Anzeigetext aus version-sources.yaml; er gehört nur in die
	// Quellenliste der Inventardatei.
	Note string `json:"note,omitempty"`
}

// Rejection ist ein Pfad, den die Vertrauensgrenze abgelehnt hat. Sie nennt den
// angefragten und den aufgelösten Pfad, damit erkennbar ist, was tatsächlich
// gelesen worden wäre.
type Rejection struct {
	Requested string `json:"requested"`
	Resolved  string `json:"resolved,omitempty"`
	Reason    string `json:"reason"`
}

// Exclusion ist ein Bereich, in dem die Standarderkennung nicht gesucht hat.
//
// Ein Ausschluss ist keine Ablehnung der Vertrauensgrenze: der Bereich dürfte
// gelesen werden, er wird nur nicht von selbst gesucht. Sichtbar ist er
// trotzdem — ein stiller Filter wäre eine Lücke, die niemand sieht.
type Exclusion struct {
	Pattern string `json:"pattern"`
	// Origin ist `installation` (feste Regel) oder `configured` (aus `exclude:`).
	Origin string `json:"origin"`
	Reason string `json:"reason"`
	// Skipped ist die Zahl der Quellen, die diese Regel übergangen hat. `0`
	// heißt: die Regel gilt, hat aber nichts getroffen.
	Skipped int `json:"skipped"`
}

// Note ist ein sichtbarer Hinweis zu einer Quelle: gefunden, aber nicht lesbar;
// konfiguriert, aber nicht da; bekannt, aber defekt.
type Note struct {
	Source string `json:"source,omitempty"`
	Text   string `json:"text"`
}

// Result ist das vollständige Ergebnis eines Erhebungslaufs.
type Result struct {
	Entries    []Entry      `json:"entries"`
	Deviations []Deviation  `json:"deviations"`
	Sources    []SourceRead `json:"sources"`
	Rejections []Rejection  `json:"rejections"`
	Notes      []Note       `json:"notes"`
	// Exclusions sind die Bereiche, in denen die Standarderkennung nicht
	// gesucht hat, mit der Zahl der dadurch übergangenen Quellen.
	Exclusions []Exclusion `json:"exclusions"`
	// ConfiguredSources ist die Zahl der Einträge in der Quellenkonfiguration,
	// abgelehnte eingeschlossen. Sie steht so im Frontmatter.
	ConfiguredSources int `json:"configuredSources"`
}
