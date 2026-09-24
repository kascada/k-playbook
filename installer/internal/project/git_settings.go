package project

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// GitSwitch ist die Projektentscheidung, ob die Oberfläche das Umschalten des
// Branches anbietet.
type GitSwitch string

const (
	// GitSwitchUnknown: noch nicht entschieden. Ausdrücklicher Zustand wie
	// tools.gh.status unknown — die Seite zeigt die Liste, bietet keinen
	// Wechsel an und nennt die offene Entscheidung.
	GitSwitchUnknown GitSwitch = "unknown"
	// GitSwitchOffer: Umschalten wird angeboten, nach Vorprüfung und
	// Bestätigung.
	GitSwitchOffer GitSwitch = "offer"
	// GitSwitchOff: das Projekt will kein Umschalten aus der Oberfläche.
	GitSwitchOff GitSwitch = "off"
)

// DefaultGitSwitch gilt, solange nichts in der Datei steht.
const DefaultGitSwitch = GitSwitchUnknown

// ValidGitSwitch meldet, ob der Wert bekannt ist.
func ValidGitSwitch(value GitSwitch) bool {
	switch value {
	case GitSwitchUnknown, GitSwitchOffer, GitSwitchOff:
		return true
	}
	return false
}

// GitEnvironment ist ein Eintrag aus git.environments: eine Umgebung und der
// langlebige Branch, der sie bedient.
type GitEnvironment struct {
	Name   string `json:"name"`
	Branch string `json:"branch"`
}

// GitSettings ist der Abschnitt git: der Konfiguration.
//
// Das Format ist bewusst flach und schlüsselweise: Die Angaben gelten heute für
// das ganze Team, und eine spätere Ebene je Nutzer soll einzelne Schlüssel
// überschreiben können, ohne den Abschnitt umzubauen.
type GitSettings struct {
	// Switch ist die Entscheidung aus git.switch; ohne Eintrag unknown.
	Switch GitSwitch `json:"switch"`
	// Allow sind die Muster aus git.allow. Leer heißt: jeder Branch.
	Allow []string `json:"allow"`
	// Environments sind die festgelegten Umgebungen in Dateireihenfolge.
	Environments []GitEnvironment `json:"environments"`
	// Configured meldet, ob der Abschnitt git: in der Datei stand.
	Configured bool `json:"configured"`
}

// gitEnvironmentNamePattern begrenzt Umgebungsnamen auf das, was als Schlüssel
// und als Beschriftung eindeutig bleibt.
var gitEnvironmentNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ReadGitSettings liest den Abschnitt git: eines Projekts.
func ReadGitSettings(projectDir string) (GitSettings, error) {
	data, err := os.ReadFile(ConfigPath(projectDir))
	if err != nil {
		return defaultGitSettings(), err
	}
	return parseGitSettings(string(data))
}

func defaultGitSettings() GitSettings {
	return GitSettings{Switch: DefaultGitSwitch, Allow: []string{}, Environments: []GitEnvironment{}}
}

// parseGitSettings schneidet den Abschnitt git: aus der Datei und liest nur ihn.
//
// Der Rest der Datei bleibt zeilenweise, wie überall in der Konfiguration: Ein
// Fehler in einem fremden Block darf den Abschnitt nicht unlesbar machen, und
// geschrieben wird hier nichts. Der ausgeschnittene Block geht an yamllite, weil
// er eine Abbildung mit erhaltener Reihenfolge und eine Liste trägt.
func parseGitSettings(content string) (GitSettings, error) {
	settings := defaultGitSettings()

	block, found := topLevelBlock(content, "git")
	if !found {
		return settings, nil
	}
	settings.Configured = true

	root, err := yamllite.Parse([]byte(block))
	if err != nil {
		return settings, fmt.Errorf("der Abschnitt git: ist nicht lesbar: %w", err)
	}
	section := root.Get("git")
	if section == nil || (section.Kind == yamllite.Scalar && strings.TrimSpace(section.Value) == "") {
		// git: ohne Inhalt: der Abschnitt steht da, entschieden ist nichts.
		return settings, nil
	}
	if section.Kind != yamllite.Mapping {
		return settings, fmt.Errorf("git: muss eine Abbildung mit switch, allow und environments sein")
	}

	for _, key := range section.MapKeys() {
		switch key {
		case "switch", "allow", "environments":
		default:
			return settings, fmt.Errorf("git.%s ist unbekannt; erlaubt sind switch, allow und environments", key)
		}
	}

	if node := section.Get("switch"); node != nil {
		value := GitSwitch(cleanGitValue(node.Str()))
		if node.Kind != yamllite.Scalar || !ValidGitSwitch(value) {
			return settings, fmt.Errorf("git.switch hat den unbekannten Wert %q; erlaubt sind unknown, offer und off", node.Str())
		}
		settings.Switch = value
	}

	if node := section.Get("allow"); node != nil {
		if node.Kind == yamllite.Scalar && strings.TrimSpace(node.Value) == "" {
			// allow: ohne Einträge ist dasselbe wie eine leere Liste.
		} else if node.Kind != yamllite.Sequence {
			return settings, fmt.Errorf("git.allow muss eine Liste von Branch-Mustern sein")
		} else {
			for _, item := range node.List() {
				pattern := cleanGitValue(item.Str())
				if item.Kind != yamllite.Scalar || pattern == "" {
					return settings, fmt.Errorf("git.allow enthält einen leeren Eintrag (Zeile %d des Abschnitts)", item.At())
				}
				if !ValidBranchPattern(pattern) {
					return settings, fmt.Errorf("git.allow enthält das unzulässige Muster %q; erlaubt sind Branch-Namen mit *", pattern)
				}
				settings.Allow = append(settings.Allow, pattern)
			}
		}
	}

	if node := section.Get("environments"); node != nil {
		if node.Kind == yamllite.Scalar && strings.TrimSpace(node.Value) == "" {
			// environments: ohne Einträge: nichts festgelegt.
		} else if node.Kind != yamllite.Mapping {
			return settings, fmt.Errorf("git.environments muss eine Abbildung Umgebung: Branch sein")
		} else {
			// Ein Branch darf mehrere Umgebungen bedienen (etwa stage für stage und
			// prod); eine Umgebung hat genau einen Branch — mehrere je Umgebung
			// sind bewusst nicht Teil dieses Formats.
			//
			// Umgebungen werden später ohne Rücksicht auf Groß- und Kleinschreibung
			// zusammengelegt, wie GitHub-Environments auch. Zwei Schreibweisen
			// desselben Namens wären dort eine Umgebung mit zwei Branches; das weist
			// schon der Parser ab.
			seen := map[string]string{}
			for _, name := range node.MapKeys() {
				cleanName := cleanGitValue(name)
				if !gitEnvironmentNamePattern.MatchString(cleanName) {
					return settings, fmt.Errorf("git.environments enthält den unzulässigen Umgebungsnamen %q; erlaubt sind Buchstaben, Ziffern und . _ -", name)
				}
				if first, ok := seen[strings.ToLower(cleanName)]; ok {
					return settings, fmt.Errorf("git.environments nennt die Umgebung %s doppelt (%s und %s); Groß- und Kleinschreibung unterscheiden Umgebungen nicht", strings.ToLower(cleanName), first, cleanName)
				}
				seen[strings.ToLower(cleanName)] = cleanName
				value := node.Get(name)
				branch := cleanGitValue(value.Str())
				if value.Kind != yamllite.Scalar || branch == "" {
					return settings, fmt.Errorf("git.environments.%s nennt keinen Branch", cleanName)
				}
				if !ValidBranchName(branch) {
					return settings, fmt.Errorf("git.environments.%s nennt den unzulässigen Branch-Namen %q", cleanName, branch)
				}
				settings.Environments = append(settings.Environments, GitEnvironment{Name: cleanName, Branch: branch})
			}
		}
	}
	return settings, nil
}

func cleanGitValue(value string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(value), `"'`))
}

// topLevelBlock liefert die Zeilen eines Blocks auf oberster Ebene, samt seiner
// Kopfzeile, bis zur nächsten Zeile auf oberster Ebene.
func topLevelBlock(content string, name string) (string, bool) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	start := -1
	for index, line := range lines {
		if !isTopLevelYAMLLine(line) {
			continue
		}
		key, _, found := strings.Cut(stripYAMLComment(line), ":")
		if found && strings.TrimSpace(key) == name {
			start = index
			break
		}
	}
	if start < 0 {
		return "", false
	}
	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		if isTopLevelYAMLLine(lines[index]) {
			end = index
			break
		}
	}
	return strings.Join(lines[start:end], "\n") + "\n", true
}

// stripYAMLComment schneidet einen Kommentar ab, der mit Leerraum vor # beginnt.
func stripYAMLComment(line string) string {
	if index := strings.Index(line, " #"); index >= 0 {
		return line[:index]
	}
	return line
}

// ValidBranchName prüft einen Branch-Namen nach den Regeln, die git für Refs
// anlegt (git check-ref-format), soweit sie ohne Aufruf prüfbar sind. Der Name
// geht als Argument in git-Aufrufe; was hier durchkommt, kann dort weder als
// Option noch als Bereichsangabe gelesen werden.
func ValidBranchName(name string) bool {
	return validRefName(name, false)
}

// ValidBranchPattern prüft ein Muster aus git.allow: ein Branch-Name, in dem
// zusätzlich * stehen darf.
func ValidBranchPattern(pattern string) bool {
	return validRefName(pattern, true)
}

func validRefName(name string, allowStar bool) bool {
	if name == "" || name == "@" || strings.HasPrefix(name, "-") || strings.HasPrefix(name, "/") ||
		strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".") || strings.HasSuffix(name, ".lock") ||
		strings.Contains(name, "..") || strings.Contains(name, "//") || strings.Contains(name, "@{") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	for _, r := range name {
		switch {
		case r < 0x20 || r == 0x7f:
			return false
		case r == ' ', r == '~', r == '^', r == ':', r == '?', r == '[', r == '\\':
			return false
		case r == '*' && !allowStar:
			return false
		}
	}
	return true
}

// BranchAllowed meldet, ob ein Branch unter git.allow fällt. Eine leere Liste
// erlaubt jeden Branch. * steht für beliebig viele Zeichen, auch für /:
// remediation/* trifft remediation/OMN-1 ebenso wie remediation/a/b.
func BranchAllowed(allow []string, branch string) (bool, string) {
	if len(allow) == 0 {
		return true, ""
	}
	for _, pattern := range allow {
		if matchBranchPattern(pattern, branch) {
			return true, pattern
		}
	}
	return false, ""
}

func matchBranchPattern(pattern string, branch string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == branch
	}
	if !strings.HasPrefix(branch, parts[0]) {
		return false
	}
	rest := branch[len(parts[0]):]
	for index := 1; index < len(parts)-1; index++ {
		position := strings.Index(rest, parts[index])
		if position < 0 {
			return false
		}
		rest = rest[position+len(parts[index]):]
	}
	return strings.HasSuffix(rest, parts[len(parts)-1])
}
