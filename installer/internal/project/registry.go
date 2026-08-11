package project

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AssetKind ist eine der beiden Sorten, die ein Assistent bei sich registriert.
type AssetKind string

const (
	// KindCommands sind einzelne Markdown-Dateien, aufrufbar als /<name>.
	// Unterverzeichnisse bilden Namensraeume: _shared/path-resolution.md wird
	// als /_shared:path-resolution aufgerufen.
	KindCommands AssetKind = "commands"
	// KindSkills sind Verzeichnisse mit einer SKILL.md darin.
	KindSkills AssetKind = "skills"
)

// skillFileName macht ein Verzeichnis zum Skill. Ohne sie ist es Beiwerk.
const skillFileName = "SKILL.md"

// RegistryEntry ist ein aufgeloester Command oder Skill: was nach der
// Verrechnung von mitgeliefert und projekteigen tatsaechlich gilt.
type RegistryEntry struct {
	// Name ist der Schluessel des Eintrags und zugleich sein Pfad unterhalb des
	// Zielverzeichnisses. Bei Commands der Pfad ab commands/ einschliesslich
	// Namensraum, bei Skills der Verzeichnisname.
	Name string `json:"name"`
	// Path ist die wirksame Quelle, absolut.
	Path string `json:"path"`
	// Origin: dist, local oder override.
	Origin string `json:"origin"`
	// Disabled: ein leerer projekteigener Eintrag schaltet den mitgelieferten
	// ab. Solche Eintraege werden nicht registriert.
	Disabled bool `json:"disabled,omitempty"`
	// IsDir unterscheidet Skills (Verzeichnis) von Commands (Datei).
	IsDir bool `json:"isDir,omitempty"`
}

// registryDirs sind die beiden Quellverzeichnisse einer Sorte, in
// Ueberlagerungsreihenfolge: erst mitgeliefert, dann projekteigen.
func registryDirs(projectDir string, kind AssetKind) (shipped string, local string) {
	return filepath.Join(PlaybookDir(projectDir), string(kind)),
		filepath.Join(LocalDir(projectDir), string(kind))
}

// RegistrySourcePresent meldet, ob es ueberhaupt eine Quelle gibt. Fehlen beide,
// ist nichts zu registrieren — das deutet auf eine unvollstaendige Installation.
func RegistrySourcePresent(projectDir string, kind AssetKind) bool {
	shipped, local := registryDirs(projectDir, kind)
	return isDir(shipped) || isDir(local)
}

// ResolveRegistry fuehrt mitgelieferte und projekteigene Eintraege zusammen.
//
// Vergleichseinheit ist der Name: eine gleichnamige projekteigene Datei ersetzt
// die mitgelieferte vollstaendig, eine leere schaltet sie ab. Das ist dieselbe
// Regel wie bei rules, reviews und checks in context.go — nur dass hier bis in
// die Namensraeume hinein verglichen wird, damit ein Projekt eine einzelne
// Datei aus _shared/ ersetzen kann, ohne den ganzen Namensraum zu kopieren.
//
// Abgeschaltete Eintraege bleiben im Ergebnis, damit die Oberflaeche sie zeigen
// kann; registriert werden sie nicht.
func ResolveRegistry(projectDir string, kind AssetKind) []RegistryEntry {
	shippedDir, localDir := registryDirs(projectDir, kind)
	shipped := registrySources(shippedDir, kind)
	local := registrySources(localDir, kind)

	names := map[string]bool{}
	for name := range shipped {
		names[name] = true
	}
	for name := range local {
		names[name] = true
	}

	entries := make([]RegistryEntry, 0, len(names))
	for name := range names {
		entry := RegistryEntry{Name: name, IsDir: kind == KindSkills}

		if path, ok := local[name]; ok {
			entry.Path = path
			entry.Origin = "local"
			if _, overlaid := shipped[name]; overlaid {
				entry.Origin = "override"
			}
			entry.Disabled = registryEntryEmpty(path, kind)
		} else {
			entry.Path = shipped[name]
			entry.Origin = "dist"
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

// ActiveRegistry sind die Eintraege, die tatsaechlich registriert gehoeren.
func ActiveRegistry(projectDir string, kind AssetKind) []RegistryEntry {
	entries := ResolveRegistry(projectDir, kind)

	active := make([]RegistryEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.Disabled {
			active = append(active, entry)
		}
	}
	return active
}

// registrySources liest ein Quellverzeichnis und liefert Name -> Pfad.
func registrySources(dir string, kind AssetKind) map[string]string {
	if kind == KindSkills {
		return skillSources(dir)
	}
	return commandSources(dir)
}

// commandSources laeuft rekursiv, damit Namensraeume wie _shared/ bis auf die
// einzelne Datei ueberlagerbar sind.
func commandSources(dir string) map[string]string {
	files := map[string]string{}
	collectCommands(dir, "", files)
	return files
}

func collectCommands(dir string, prefix string, files map[string]string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		path := filepath.Join(dir, name)
		// Ein Symlink hier waere ein Rest aus einer frueheren Verlinkung und
		// koennte im Kreis zeigen; Quellen sind immer echte Eintraege.
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		key := name
		if prefix != "" {
			key = prefix + "/" + name
		}

		if info.IsDir() {
			collectCommands(path, key, files)
			continue
		}
		if name == "README.md" || !strings.HasSuffix(name, ".md") {
			continue
		}
		files[key] = path
	}
}

// skillSources nimmt nur die oberste Ebene: ein Skill ist eine Einheit aus
// SKILL.md und Beiwerk und wird als Ganzes ueberlagert.
func skillSources(dir string) map[string]string {
	skills := map[string]string{}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return skills
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			continue
		}
		if !fileExists(filepath.Join(path, skillFileName)) {
			continue
		}
		skills[name] = path
	}
	return skills
}

// registryEntryEmpty meldet, ob ein projekteigener Eintrag nur dazu da ist, den
// mitgelieferten abzuschalten. Bei Skills entscheidet die SKILL.md darueber.
func registryEntryEmpty(path string, kind AssetKind) bool {
	if kind == KindSkills {
		return isEmptyFile(filepath.Join(path, skillFileName))
	}
	return isEmptyFile(path)
}
