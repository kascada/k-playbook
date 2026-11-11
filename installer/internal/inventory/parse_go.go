package inventory

import "strings"

// Go: `require` liefert package-Zeilen mit exact-Pin — Go-Module sind immer
// exakt. `go.sum` wird nicht als Versionsquelle geführt: es wiederholt `go.mod`
// und trüge nur Rauschen bei. `tools.go` nennt Werkzeuge, aber keine Versionen
// — die stehen in `go.mod`.
func parseGo(c *collector) {
	switch c.file.Base {
	case "go.mod":
		parseGoMod(c)
	case ".go-version":
		parseVersionFile(c, "go", "runtime.go")
	}
}

// goIndirectNote ist die Notiz einer `require`-Zeile mit `// indirect`. Der
// Text ist das Erkennungsmerkmal, an dem /k-docs-tools rein indirekte
// Kandidaten erkennt; Vertrag (docs/version-inventory.md) und Command nennen
// ihn wörtlich.
const goIndirectNote = "indirekt (// indirect in go.mod)"

func parseGoMod(c *collector) {
	block := ""
	for index, raw := range c.lines() {
		line := strings.TrimSpace(raw)
		comment := ""
		if start := strings.Index(line, "//"); start >= 0 {
			comment = line[start+2:]
			line = strings.TrimSpace(line[:start])
		}
		if line == "" {
			continue
		}
		if line == ")" {
			block = ""
			continue
		}
		if strings.HasSuffix(line, "(") {
			block = strings.TrimSpace(strings.TrimSuffix(line, "("))
			continue
		}

		directive := block
		if directive == "" {
			fields := strings.Fields(line)
			directive = fields[0]
			line = strings.TrimSpace(strings.TrimPrefix(line, directive))
		}

		switch directive {
		case "go":
			c.add(Entry{Ecosystem: EcoRuntime, Name: "go", KindOfThing: ThingRuntime,
				Version: strings.TrimSpace(line), SourceKey: "go", SourceLine: index + 1})
		case "toolchain":
			value := strings.TrimSpace(line)
			c.add(Entry{Ecosystem: EcoRuntime, Name: "go-toolchain", KindOfThing: ThingRuntime,
				Version: value, Pin: classifyPin(strings.TrimPrefix(value, "go")),
				SourceKey: "toolchain", SourceLine: index + 1})
		case "require":
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			note := ""
			if isGoIndirect(comment) {
				note = goIndirectNote
			}
			c.add(Entry{Ecosystem: EcoGo, Name: fields[0], KindOfThing: ThingPackage,
				Version: fields[1], Pin: PinExact,
				SourceKey: "require." + fields[0], SourceLine: index + 1, Note: note})
		case "replace":
			addGoReplace(c, line, index+1)
		}
	}
}

// isGoIndirect folgt isIndirect aus golang.org/x/mod/modfile: Der Kommentar
// hinter `//`, an Leerraum in Wörter zerlegt, ist genau `indirect`, oder sein
// erstes Wort ist `indirect;` und es folgen weitere. `indirect` an anderer
// Stelle im Kommentar zählt nicht.
func isGoIndirect(comment string) bool {
	words := strings.Fields(comment)
	if len(words) == 0 {
		return false
	}
	if len(words) == 1 {
		return words[0] == "indirect"
	}
	return words[0] == "indirect;"
}

// addGoReplace: `replace` auf einen Pfad ist local; `replace` auf eine andere
// Version ist eine eigene Zeile mit Hinweis. Beides bleibt sichtbar — ein
// Ersatzmodul ist genau die Aussage, nach der man später sucht.
func addGoReplace(c *collector, line string, lineNumber int) {
	left, right, found := strings.Cut(line, "=>")
	if !found {
		return
	}
	source := strings.Fields(left)
	target := strings.Fields(right)
	if len(source) == 0 || len(target) == 0 {
		return
	}
	name := source[0]
	if strings.HasPrefix(target[0], ".") || strings.HasPrefix(target[0], "/") {
		c.add(Entry{Ecosystem: EcoGo, Name: name, KindOfThing: ThingPackage, Pin: PinLocal,
			SourceKey: "replace." + name, SourceLine: lineNumber,
			Note: "ersetzt durch den Arbeitsbaum unter " + target[0]})
		return
	}
	version := ""
	if len(target) > 1 {
		version = target[1]
	}
	c.add(Entry{Ecosystem: EcoGo, Name: name, KindOfThing: ThingPackage, Version: version,
		Pin: PinExact, SourceKey: "replace." + name, SourceLine: lineNumber,
		Note: "ersetzt durch " + target[0]})
}
