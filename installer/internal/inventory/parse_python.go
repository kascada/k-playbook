package inventory

import (
	"encoding/json"
	"strings"
)

// Python: Manifest ist die Absicht, Lockfile der aufgelöste Stand. Beide werden
// gelesen, beide als eigene Zeilen geführt; widersprechen sie sich, ist das
// eine Abweichung und wird nicht zusammengezogen.
func parsePython(c *collector) {
	switch {
	case c.file.Base == "pyproject.toml":
		parsePyproject(c)
	case c.file.Base == "Pipfile":
		parsePipfile(c)
	case c.file.Base == "Pipfile.lock":
		parsePipfileLock(c)
	case c.file.Base == "poetry.lock" || c.file.Base == "uv.lock":
		parsePythonLock(c)
	case c.file.Base == ".python-version":
		parseVersionFile(c, "python", "runtime.python")
	case c.file.Base == "setup.cfg":
		parseSetupCfg(c)
	case c.file.Base == "setup.py":
		parseSetupPy(c)
	case strings.HasSuffix(c.file.Base, ".txt"):
		section := "requirements"
		if strings.HasPrefix(c.file.Base, "constraints") {
			section = "constraints"
		}
		parseRequirementsFile(c, section)
	}
}

func parseRequirementsFile(c *collector, section string) {
	for index, line := range c.lines() {
		requirement, ok := parseRequirement(line)
		if !ok {
			continue
		}
		c.add(Entry{
			Ecosystem:   EcoPython,
			Name:        requirement.Name,
			KindOfThing: ThingPackage,
			Version:     requirement.Version,
			Pin:         requirement.Pin,
			SourceKey:   section + "." + requirement.Name,
			SourceLine:  index + 1,
			Note:        requirement.Note,
		})
	}
}

// scopeFor bildet den Gruppennamen eines Extras auf die im Vertrag gültigen
// Scope-Werte ab. Was sich nicht zuordnen lässt, ist `optional` — das ist es
// in jedem dieser Fälle auch.
func scopeFor(group string) string {
	switch strings.ToLower(group) {
	case "dev", "develop", "development":
		return "dev"
	case "test", "tests", "testing":
		return "test"
	case "build":
		return "build"
	case "main", "default":
		return "main"
	}
	return "optional"
}

func parsePyproject(c *collector) {
	doc := parseTOML(c.file.Data)

	if table := doc.table("project"); table != nil {
		if entry, ok := table.get("dependencies"); ok {
			addRequirementList(c, tomlArray(entry.Raw, entry.Line), "project.dependencies", "main")
		}
		if entry, ok := table.get("requires-python"); ok {
			if value, isString := tomlString(entry.Raw); isString {
				c.add(Entry{Ecosystem: EcoRuntime, Name: "python", KindOfThing: ThingRuntime,
					Version: value, SourceKey: "project.requires-python", SourceLine: entry.Line})
			}
		}
	}
	if table := doc.table("build-system"); table != nil {
		if entry, ok := table.get("requires"); ok {
			addRequirementList(c, tomlArray(entry.Raw, entry.Line), "build-system.requires", "build")
		}
	}
	if table := doc.table("project.optional-dependencies"); table != nil {
		for _, key := range table.Keys {
			entry := table.Entries[key]
			addRequirementList(c, tomlArray(entry.Raw, entry.Line),
				"project.optional-dependencies."+key, scopeFor(key))
		}
	}
	if table := doc.table("dependency-groups"); table != nil {
		for _, key := range table.Keys {
			entry := table.Entries[key]
			addRequirementList(c, tomlArray(entry.Raw, entry.Line),
				"dependency-groups."+key, scopeFor(key))
		}
	}

	// Poetry führt seine Abhängigkeiten als Schlüssel-Wert-Paare statt als
	// Liste; die Werte sind Zeichenketten oder Inline-Tabellen.
	for _, table := range doc.Tables {
		scope := ""
		switch {
		case table.Name == "tool.poetry.dependencies":
			scope = "main"
		case table.Name == "tool.poetry.dev-dependencies":
			scope = "dev"
		case strings.HasPrefix(table.Name, "tool.poetry.group.") && strings.HasSuffix(table.Name, ".dependencies"):
			group := strings.TrimSuffix(strings.TrimPrefix(table.Name, "tool.poetry.group."), ".dependencies")
			scope = scopeFor(group)
		default:
			continue
		}
		for _, key := range table.Keys {
			entry := table.Entries[key]
			addPoetryDependency(c, table.Name, key, entry, scope)
		}
	}
}

func addRequirementList(c *collector, values []tomlValue, section string, scope string) {
	for _, value := range values {
		spec, ok := tomlString(value.Raw)
		if !ok {
			continue
		}
		requirement, ok := parseRequirement(spec)
		if !ok {
			continue
		}
		c.add(Entry{
			Ecosystem:   EcoPython,
			Name:        requirement.Name,
			KindOfThing: ThingPackage,
			Version:     requirement.Version,
			Pin:         requirement.Pin,
			Scope:       scope,
			SourceKey:   section + "." + requirement.Name,
			SourceLine:  value.Line,
			Note:        requirement.Note,
		})
	}
}

func addPoetryDependency(c *collector, table string, key string, entry tomlEntry, scope string) {
	name := key
	ecosystem := EcoPython
	kindOfThing := ThingPackage
	if strings.EqualFold(key, "python") {
		ecosystem = EcoRuntime
		kindOfThing = ThingRuntime
	}

	if value, ok := tomlString(entry.Raw); ok {
		c.add(Entry{Ecosystem: ecosystem, Name: name, KindOfThing: kindOfThing, Version: value,
			Scope: scope, SourceKey: table + "." + key, SourceLine: entry.Line})
		return
	}
	if fields := tomlInline(entry.Raw); fields != nil {
		if pathValue, ok := tomlString(fields["path"]); ok {
			c.add(Entry{Ecosystem: ecosystem, Name: name, KindOfThing: kindOfThing, Pin: PinLocal,
				Scope: scope, SourceKey: table + "." + key, SourceLine: entry.Line,
				Note: "Pfad-Abhängigkeit auf " + pathValue})
			return
		}
		if gitValue, ok := tomlString(fields["git"]); ok {
			c.add(Entry{Ecosystem: ecosystem, Name: name, KindOfThing: kindOfThing, Pin: PinLocal,
				Scope: scope, SourceKey: table + "." + key, SourceLine: entry.Line,
				Note: "VCS-Abhängigkeit auf " + gitValue})
			return
		}
		if version, ok := tomlString(fields["version"]); ok {
			c.add(Entry{Ecosystem: ecosystem, Name: name, KindOfThing: kindOfThing, Version: version,
				Scope: scope, SourceKey: table + "." + key, SourceLine: entry.Line})
			return
		}
	}
	c.add(Entry{Ecosystem: ecosystem, Name: name, KindOfThing: kindOfThing, Version: entry.Raw,
		Pin: PinUnknown, Scope: scope, SourceKey: table + "." + key, SourceLine: entry.Line,
		Note: "Versionsangabe nicht deutbar"})
}

func parsePipfile(c *collector) {
	doc := parseTOML(c.file.Data)
	for _, table := range doc.Tables {
		scope := ""
		switch table.Name {
		case "packages":
			scope = "main"
		case "dev-packages":
			scope = "dev"
		case "requires":
			for _, key := range table.Keys {
				if key != "python_version" && key != "python_full_version" {
					continue
				}
				entry := table.Entries[key]
				if value, ok := tomlString(entry.Raw); ok {
					c.add(Entry{Ecosystem: EcoRuntime, Name: "python", KindOfThing: ThingRuntime,
						Version: value, SourceKey: "requires." + key, SourceLine: entry.Line})
				}
			}
			continue
		default:
			continue
		}
		for _, key := range table.Keys {
			addPoetryDependency(c, table.Name, key, table.Entries[key], scope)
		}
	}
}

func parsePipfileLock(c *collector) {
	var document map[string]map[string]struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(c.file.Data, &document); err != nil {
		c.note("nicht lesbares JSON: %v", err)
		return
	}
	finder := newLineFinder(c.file.Data)
	for _, section := range []struct{ key, scope string }{{"default", "main"}, {"develop", "dev"}} {
		packages, ok := document[section.key]
		if !ok {
			continue
		}
		for _, name := range sortedKeys(packages) {
			c.add(Entry{Ecosystem: EcoPython, Name: name, KindOfThing: ThingPackage,
				Version: packages[name].Version, Scope: section.scope,
				SourceKey:  section.key + "." + name,
				SourceLine: finder.find(name, 0)})
		}
	}
}

// parsePythonLock liest poetry.lock und uv.lock. Beide führen ihre Pakete als
// TOML-Tabellenliste `[[package]]`.
func parsePythonLock(c *collector) {
	doc := parseTOML(c.file.Data)
	for _, table := range doc.tables("package") {
		name, ok := tomlString(table.Entries["name"].Raw)
		if !ok {
			continue
		}
		version, _ := tomlString(table.Entries["version"].Raw)
		c.add(Entry{Ecosystem: EcoPython, Name: name, KindOfThing: ThingPackage,
			Version: version, SourceKey: "package." + name, SourceLine: table.Line})
	}
}

func parseSetupCfg(c *collector) {
	lines := c.lines()
	section := ""
	key := ""
	for index, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			section = strings.Trim(trimmed, "[]")
			key = ""
			continue
		}
		indented := raw != trimmed
		if !indented {
			name, value, found := strings.Cut(trimmed, "=")
			if !found {
				continue
			}
			key = strings.TrimSpace(name)
			value = strings.TrimSpace(value)
			if section == "options" && key == "python_requires" && value != "" {
				c.add(Entry{Ecosystem: EcoRuntime, Name: "python", KindOfThing: ThingRuntime,
					Version: value, SourceKey: "options.python_requires", SourceLine: index + 1})
			}
			continue
		}
		if section != "options" && !strings.HasPrefix(section, "options.extras_require") {
			continue
		}
		if section == "options" && key != "install_requires" && key != "setup_requires" {
			continue
		}
		requirement, ok := parseRequirement(trimmed)
		if !ok {
			continue
		}
		scope := "main"
		if strings.HasPrefix(section, "options.extras_require") {
			scope = "optional"
		} else if key == "setup_requires" {
			scope = "build"
		}
		c.add(Entry{Ecosystem: EcoPython, Name: requirement.Name, KindOfThing: ThingPackage,
			Version: requirement.Version, Pin: requirement.Pin, Scope: scope,
			SourceKey: section + "." + requirement.Name, SourceLine: index + 1, Note: requirement.Note})
	}
}

// parseSetupPy sucht ausschließlich die Liste hinter install_requires. Alles
// andere wäre Ausführen von Python — und das Inventar führt nichts aus.
func parseSetupPy(c *collector) {
	text := string(c.file.Data)
	position := strings.Index(text, "install_requires")
	if position < 0 {
		return
	}
	open := strings.Index(text[position:], "[")
	if open < 0 {
		c.note("install_requires gefunden, aber keine Liste dahinter — setup.py wird nicht ausgeführt")
		return
	}
	open += position
	closing := strings.Index(text[open:], "]")
	if closing < 0 {
		c.note("install_requires gefunden, aber die Liste ist nicht geschlossen")
		return
	}
	inner := text[open+1 : open+closing]
	startLine := strings.Count(text[:open], "\n") + 1
	for _, value := range splitTOMLList(inner, startLine) {
		spec, ok := tomlString(value.Raw)
		if !ok {
			continue
		}
		requirement, ok := parseRequirement(spec)
		if !ok {
			continue
		}
		c.add(Entry{Ecosystem: EcoPython, Name: requirement.Name, KindOfThing: ThingPackage,
			Version: requirement.Version, Pin: requirement.Pin, Scope: "main",
			SourceKey: "install_requires." + requirement.Name, SourceLine: value.Line,
			Note: requirement.Note})
	}
}

// parseVersionFile liest die einzeiligen Runtime-Dateien: .python-version,
// .nvmrc, .go-version, .node-version, .ruby-version.
func parseVersionFile(c *collector, name string, sourceKey string) {
	for index, line := range c.lines() {
		value := strings.TrimSpace(line)
		if value == "" || strings.HasPrefix(value, "#") {
			continue
		}
		c.add(Entry{Ecosystem: EcoRuntime, Name: name, KindOfThing: ThingRuntime,
			Version: value, SourceKey: sourceKey, SourceLine: index + 1})
		return
	}
}
