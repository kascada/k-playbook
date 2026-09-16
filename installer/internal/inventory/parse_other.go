package inventory

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Die weiteren Manifesttypen, die schon /k-docs-tools erkennt. Die Semantik ist
// überall dieselbe: Manifest und Lockfile beide, nur direkte Abhängigkeiten,
// Scope aus dem Abschnitt.

func parseRust(c *collector) {
	doc := parseTOML(c.file.Data)
	if c.file.Base == "Cargo.lock" {
		if c.file.Direct == nil {
			return
		}
		for _, table := range doc.tables("package") {
			name, ok := tomlString(table.Entries["name"].Raw)
			if !ok {
				continue
			}
			version, _ := tomlString(table.Entries["version"].Raw)
			scope, direct := c.file.Direct[normalizeName(EcoRust, name)]
			if !direct {
				continue
			}
			c.add(Entry{Ecosystem: EcoRust, Name: name, KindOfThing: ThingPackage,
				Version: version, Scope: scope, SourceKey: "package." + name, SourceLine: table.Line})
		}
		return
	}
	if table := doc.table("package"); table != nil {
		if entry, ok := table.get("rust-version"); ok {
			if value, isString := tomlString(entry.Raw); isString {
				c.add(Entry{Ecosystem: EcoRuntime, Name: "rust", KindOfThing: ThingRuntime,
					Version: value, SourceKey: "package.rust-version", SourceLine: entry.Line})
			}
		}
	}
	for _, section := range []struct{ table, scope string }{
		{"dependencies", "main"},
		{"dev-dependencies", "dev"},
		{"build-dependencies", "build"},
	} {
		table := doc.table(section.table)
		if table == nil {
			continue
		}
		for _, key := range table.Keys {
			addCargoDependency(c, section.table, key, table.Entries[key], section.scope)
		}
	}
}

func addCargoDependency(c *collector, table string, key string, entry tomlEntry, scope string) {
	if value, ok := tomlString(entry.Raw); ok {
		c.add(Entry{Ecosystem: EcoRust, Name: key, KindOfThing: ThingPackage, Version: value,
			Scope: scope, SourceKey: table + "." + key, SourceLine: entry.Line})
		return
	}
	fields := tomlInline(entry.Raw)
	if pathValue, ok := tomlString(fields["path"]); ok {
		c.add(Entry{Ecosystem: EcoRust, Name: key, KindOfThing: ThingPackage, Pin: PinLocal,
			Scope: scope, SourceKey: table + "." + key, SourceLine: entry.Line,
			Note: "Pfad-Abhängigkeit auf " + pathValue})
		return
	}
	version, _ := tomlString(fields["version"])
	c.add(Entry{Ecosystem: EcoRust, Name: key, KindOfThing: ThingPackage, Version: version,
		Scope: scope, SourceKey: table + "." + key, SourceLine: entry.Line})
}

var gemLine = regexp.MustCompile(`^\s*gem\s+["']([^"']+)["'](?:\s*,\s*["']([^"']+)["'])?`)

func parseRuby(c *collector) {
	switch c.file.Base {
	case ".ruby-version":
		parseVersionFile(c, "ruby", "runtime.ruby")
		return
	case "Gemfile":
		for index, line := range c.lines() {
			match := gemLine.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			c.add(Entry{Ecosystem: EcoRuby, Name: match[1], KindOfThing: ThingPackage,
				Version: match[2], SourceKey: "gem." + match[1], SourceLine: index + 1})
		}
		return
	}
	parseGemfileLock(c)
}

// parseGemfileLock liest den DEPENDENCIES-Block: dort stehen die direkten
// Abhängigkeiten, in specs: stehen auch die transitiven.
func parseGemfileLock(c *collector) {
	section := ""
	versions := map[string]string{}
	var direct []struct {
		name string
		line int
	}
	for index, raw := range c.lines() {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if raw == trimmed && strings.ToUpper(trimmed) == trimmed {
			section = trimmed
			continue
		}
		fields := strings.Fields(trimmed)
		switch section {
		case "GEM", "PATH", "GIT":
			if len(fields) == 2 && strings.HasPrefix(fields[1], "(") && strings.HasSuffix(fields[1], ")") {
				versions[fields[0]] = strings.Trim(fields[1], "()")
			}
		case "DEPENDENCIES":
			name := strings.TrimSuffix(fields[0], "!")
			direct = append(direct, struct {
				name string
				line int
			}{name, index + 1})
		}
	}
	for _, item := range direct {
		c.add(Entry{Ecosystem: EcoRuby, Name: item.name, KindOfThing: ThingPackage,
			Version: versions[item.name], Pin: PinExact,
			SourceKey: "DEPENDENCIES." + item.name, SourceLine: item.line})
	}
}

func parsePHP(c *collector) {
	if c.file.Base == "composer.json" {
		var manifest struct {
			Require    map[string]string `json:"require"`
			RequireDev map[string]string `json:"require-dev"`
		}
		if err := json.Unmarshal(c.file.Data, &manifest); err != nil {
			c.fail("nicht lesbares JSON: %v", err)
			return
		}
		finder := newLineFinder(c.file.Data)
		for _, section := range []struct {
			key    string
			block  map[string]string
			scope  string
			anchor string
		}{
			{"require", manifest.Require, "main", "require"},
			{"require-dev", manifest.RequireDev, "dev", "require-dev"},
		} {
			anchor := finder.find(section.anchor, 0)
			for _, name := range sortedKeys(section.block) {
				ecosystem := EcoPHP
				kindOfThing := ThingPackage
				if name == "php" {
					ecosystem = EcoRuntime
					kindOfThing = ThingRuntime
				}
				c.add(Entry{Ecosystem: ecosystem, Name: name, KindOfThing: kindOfThing,
					Version: section.block[name], Scope: section.scope,
					SourceKey: section.key + "." + name, SourceLine: finder.find(name, anchor)})
			}
		}
		return
	}

	if c.file.Direct == nil {
		return
	}
	var lock struct {
		Packages []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages"`
		PackagesDev []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages-dev"`
	}
	if err := json.Unmarshal(c.file.Data, &lock); err != nil {
		c.fail("nicht lesbares JSON: %v", err)
		return
	}
	finder := newLineFinder(c.file.Data)
	for _, item := range lock.Packages {
		scope, direct := c.file.Direct[normalizeName(EcoPHP, item.Name)]
		if !direct {
			continue
		}
		c.add(Entry{Ecosystem: EcoPHP, Name: item.Name, KindOfThing: ThingPackage,
			Version: item.Version, Scope: scope,
			SourceKey: "packages." + item.Name, SourceLine: finder.find(item.Name, 0)})
	}
	for _, item := range lock.PackagesDev {
		scope, direct := c.file.Direct[normalizeName(EcoPHP, item.Name)]
		if !direct {
			continue
		}
		c.add(Entry{Ecosystem: EcoPHP, Name: item.Name, KindOfThing: ThingPackage,
			Version: item.Version, Scope: scope,
			SourceKey: "packages-dev." + item.Name, SourceLine: finder.find(item.Name, 0)})
	}
}

var (
	gradleDependency = regexp.MustCompile(`(?:implementation|api|compileOnly|runtimeOnly|testImplementation|classpath)\s*[( ]\s*["']([^"']+)["']`)
	xmlTag           = regexp.MustCompile(`<(groupId|artifactId|version)>([^<]*)</`)
)

func parseJava(c *collector) {
	if c.file.Base == "pom.xml" {
		parsePom(c)
		return
	}
	for index, line := range c.lines() {
		match := gradleDependency.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		parts := strings.Split(match[1], ":")
		if len(parts) < 2 {
			continue
		}
		version := ""
		if len(parts) >= 3 {
			version = parts[2]
		}
		name := parts[0] + ":" + parts[1]
		c.add(Entry{Ecosystem: EcoJava, Name: name, KindOfThing: ThingPackage, Version: version,
			SourceKey: "dependencies." + name, SourceLine: index + 1})
	}
}

// parsePom liest die <dependency>-Blöcke zeilenweise. Ein XML-Baum brächte
// nichts dazu, kostete aber die Zeilennummer, die zur Herkunft gehört.
func parsePom(c *collector) {
	inDependency := false
	fields := map[string]string{}
	start := 0
	for index, line := range c.lines() {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "<dependency>"):
			inDependency = true
			fields = map[string]string{}
			start = index + 1
			continue
		case strings.HasPrefix(trimmed, "</dependency>"):
			if inDependency && fields["groupId"] != "" && fields["artifactId"] != "" {
				name := fields["groupId"] + ":" + fields["artifactId"]
				version := fields["version"]
				pin := classifyPin(version)
				if strings.HasPrefix(version, "${") {
					pin = PinUnknown
				}
				c.add(Entry{Ecosystem: EcoJava, Name: name, KindOfThing: ThingPackage,
					Version: version, Pin: pin,
					SourceKey: "dependencies." + name, SourceLine: start})
			}
			inDependency = false
			continue
		}
		if !inDependency {
			continue
		}
		if match := xmlTag.FindStringSubmatch(trimmed); match != nil {
			fields[match[1]] = strings.TrimSpace(match[2])
		}
	}
}

var mixDependency = regexp.MustCompile(`\{\s*:([a-z0-9_]+)\s*,\s*"([^"]+)"`)
var mixLockEntry = regexp.MustCompile(`"([a-z0-9_]+)":\s*\{:hex,\s*:[a-z0-9_]+,\s*"([^"]+)"`)

func parseElixir(c *collector) {
	if c.file.Base == "mix.lock" && c.file.Direct == nil {
		return
	}
	pattern := mixDependency
	section := "deps"
	if c.file.Base == "mix.lock" {
		pattern = mixLockEntry
		section = "lock"
	}
	lines := c.lines()
	for index, line := range lines {
		match := pattern.FindStringSubmatchIndex(line)
		if match == nil {
			continue
		}
		name, version := line[match[2]:match[3]], line[match[4]:match[5]]
		var scope string
		if c.file.Base == "mix.lock" {
			var direct bool
			scope, direct = c.file.Direct[normalizeName(EcoElixir, name)]
			if !direct {
				continue
			}
		} else {
			scope = mixDependencyScope(lines, index, match[0])
		}
		c.add(Entry{Ecosystem: EcoElixir, Name: name, KindOfThing: ThingPackage,
			Version: version, Scope: scope, SourceKey: section + "." + name, SourceLine: index + 1})
	}
}

// mixScopeRank ist die Rangfolge, nach der eine `only:`-Angabe mit mehreren
// Umgebungen auf genau einen Scope fällt. Sie ist eine Elixir-Festlegung: die
// übrigen Parser lösen Mehrfachzuordnungen über die Lesereihenfolge auf.
var mixScopeRank = map[string]int{"main": 0, "dev": 1, "test": 2, "build": 3, "optional": 4}

var mixOnly = regexp.MustCompile(`\bonly:\s*`)
var mixAtom = regexp.MustCompile(`^:([A-Za-z0-9_]+)`)

// mixDependencyScope bewertet den vollständigen Dep-Eintrag, der in Zeile
// `index` an Position `start` mit `{` beginnt. Reicht er über mehrere Zeilen,
// wird er bis zur schließenden Klammer zusammengefasst — ein `only:` in einer
// Folgezeile darf nicht zu der Aussage `main` führen.
//
// Ohne `only:` ist der Scope `main`. Bleibt der Eintrag bis zum Dateiende offen
// oder nennt `only:` keine lesbaren Atome (`only: @envs`, `only: Mix.env()`),
// bleibt der Scope leer: geraten wird nichts.
func mixDependencyScope(lines []string, index, start int) string {
	tuple, closed := mixTuple(lines, index, start)
	if !closed {
		return ""
	}
	position := mixOnly.FindStringIndex(tuple)
	if position == nil {
		return "main"
	}
	envs, readable := mixOnlyAtoms(tuple[position[1]:])
	if !readable {
		return ""
	}
	best := ""
	for _, env := range envs {
		// Einzige Ausnahme von scopeFor: `:prod` ist in Mix die
		// Produktionsumgebung, scopeFor legte den Namen auf `optional`.
		scope := "main"
		if env != "prod" {
			scope = scopeFor(env)
		}
		if best == "" || mixScopeRank[scope] < mixScopeRank[best] {
			best = scope
		}
	}
	return best
}

// mixTuple liefert den Text des Dep-Tupels ab der öffnenden Klammer bis zu der
// Klammer, die es schließt. Inhalte von Zeichenketten und Kommentaren werden
// nicht übernommen: ein `only:` darin ist keine Angabe, und eine Klammer darin
// schließt nichts.
func mixTuple(lines []string, index, start int) (string, bool) {
	var text strings.Builder
	depth := 0
	inString := false
	for row := index; row < len(lines); row++ {
		line := lines[row]
		if row == index {
			line = line[start:]
		}
	scan:
		for position := 0; position < len(line); position++ {
			character := line[position]
			if inString {
				if character == '\\' {
					position++
				} else if character == '"' {
					inString = false
					text.WriteByte(character)
				}
				continue
			}
			switch character {
			case '"':
				inString = true
			case '#':
				break scan
			case '{', '[', '(':
				depth++
			case '}', ']', ')':
				depth--
			}
			text.WriteByte(character)
			if depth == 0 {
				return text.String(), true
			}
		}
		text.WriteByte('\n')
	}
	return "", false
}

// mixOnlyAtoms liest den Wert hinter `only:`: ein einzelnes Atom oder eine
// Liste aus Atomen, jeweils als ganzer Wert bis zum nächsten Komma oder zur
// schließenden Klammer des Tupels. Alles andere ist nicht lesbar.
func mixOnlyAtoms(value string) ([]string, bool) {
	var envs []string
	var rest string
	if strings.HasPrefix(value, "[") {
		end := strings.Index(value, "]")
		if end < 0 {
			return nil, false
		}
		for _, element := range strings.Split(value[1:end], ",") {
			element = strings.TrimSpace(element)
			match := mixAtom.FindStringSubmatch(element)
			if match == nil || len(match[0]) != len(element) {
				return nil, false
			}
			envs = append(envs, match[1])
		}
		rest = value[end+1:]
	} else {
		match := mixAtom.FindStringSubmatch(value)
		if match == nil {
			return nil, false
		}
		envs = []string{match[1]}
		rest = value[len(match[0]):]
	}
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, ",") && !strings.HasPrefix(rest, "}") {
		return nil, false
	}
	return envs, true
}

// parseToolVersions liest .tool-versions von asdf und mise: je Zeile ein
// Werkzeug mit einer oder mehreren Versionen.
func parseToolVersions(c *collector) {
	for index, line := range c.lines() {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		c.add(Entry{Ecosystem: EcoRuntime, Name: fields[0], KindOfThing: ThingRuntime,
			Version: strings.Join(fields[1:], " "), SourceKey: fields[0], SourceLine: index + 1})
	}
}
