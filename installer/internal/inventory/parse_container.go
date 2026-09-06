package inventory

import (
	"strings"

	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// parseDockerfile liest FROM-Zeilen und die Versionsangaben, die eine RUN-Zeile
// wörtlich nennt.
//
// Geraten wird nichts: ein `apt-get install curl` ist keine Versionsaussage,
// ein `pip install x==1.2.3` schon.
func parseDockerfile(c *collector) {
	stages := map[string]bool{}
	args := map[string]string{}
	lines := c.lines()

	for index := 0; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		// Fortsetzungszeilen gehören zur Anweisung; sonst stünde die halbe
		// Installationszeile in der nächsten Runde als eigene Anweisung da.
		for strings.HasSuffix(line, "\\") && index+1 < len(lines) {
			index++
			line = strings.TrimSuffix(line, "\\") + " " + strings.TrimSpace(lines[index])
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		instruction := strings.ToUpper(fields[0])
		lineNumber := index + 1

		switch instruction {
		case "ARG":
			if len(fields) > 1 {
				name, value, found := strings.Cut(fields[1], "=")
				if found {
					args[name] = value
				}
			}
		case "FROM":
			addDockerfileFrom(c, fields, args, stages, lineNumber)
		case "RUN":
			addPinnedInstalls(c, line, lineNumber)
		}
	}
}

func addDockerfileFrom(c *collector, fields []string, args map[string]string, stages map[string]bool, lineNumber int) {
	if len(fields) < 2 {
		return
	}
	reference := fields[1]
	if len(fields) >= 4 && strings.EqualFold(fields[2], "AS") {
		stages[strings.ToLower(fields[3])] = true
	}

	// `FROM <stage>` auf eine Stage derselben Datei ist local und wird als
	// solche geführt, nicht weggelassen.
	if stages[strings.ToLower(reference)] {
		c.add(Entry{Ecosystem: EcoContainer, Name: reference, KindOfThing: ThingImage, Pin: PinLocal,
			SourceKey: "FROM", SourceLine: lineNumber,
			Note: "Stage derselben Datei"})
		return
	}

	resolved, note := resolveArgs(reference, args)
	name, version, pin, digest := imageEntry(resolved)
	c.add(Entry{Ecosystem: EcoContainer, Name: name, KindOfThing: ThingImage, Version: version,
		Pin: pin, Digest: digest, SourceKey: "FROM", SourceLine: lineNumber, Note: note})
}

// resolveArgs setzt ARG-Werte ein, aber nur solche, deren Default in derselben
// Datei steht. Sonst bleibt der Ausdruck stehen und der Eintrag wird `unknown`
// mit Hinweis — geraten wird nicht.
func resolveArgs(reference string, args map[string]string) (string, string) {
	if !strings.Contains(reference, "${") && !strings.Contains(reference, "$") {
		return reference, ""
	}
	resolved := reference
	for name, value := range args {
		resolved = strings.ReplaceAll(resolved, "${"+name+"}", value)
		resolved = strings.ReplaceAll(resolved, "$"+name, value)
	}
	if unresolved(resolved) {
		return reference, "Wert aus Variable, nicht auflösbar: " + reference
	}
	return resolved, "aus ARG aufgelöst: " + reference
}

// pinnedInstall trifft die Formen, in denen eine RUN-Zeile eine Version wörtlich
// nennt.
var pinnedInstallPrefixes = []struct {
	marker    string
	separator string
	ecosystem string
}{
	{"pip install", "==", EcoPython},
	{"pip3 install", "==", EcoPython},
	{"uv pip install", "==", EcoPython},
	{"npm install", "@", EcoNode},
	{"npm i ", "@", EcoNode},
	{"yarn add", "@", EcoNode},
	{"apt-get install", "=", EcoContainer},
	{"apk add", "=", EcoContainer},
}

func addPinnedInstalls(c *collector, line string, lineNumber int) {
	lower := strings.ToLower(line)
	for _, form := range pinnedInstallPrefixes {
		position := strings.Index(lower, form.marker)
		if position < 0 {
			continue
		}
		rest := line[position+len(form.marker):]
		for _, token := range strings.Fields(rest) {
			if strings.HasPrefix(token, "-") || strings.Contains(token, "&&") {
				continue
			}
			name, version, found := strings.Cut(token, form.separator)
			if !found || name == "" || version == "" {
				continue
			}
			if strings.ContainsAny(name, "/@") && form.ecosystem == EcoNode {
				// Ein `@scope/paket@1.2.3` trennt am letzten `@`.
				if at := strings.LastIndex(token, "@"); at > 0 {
					name, version = token[:at], token[at+1:]
				}
			}
			c.add(Entry{Ecosystem: form.ecosystem, Name: name, KindOfThing: ThingTool,
				Version: version, SourceKey: "RUN", SourceLine: lineNumber,
				Note: "wörtlich gepinnt in einer RUN-Zeile"})
		}
	}
}

// parseCompose liest services.<name>.image und services.<name>.build.
func parseCompose(c *collector) {
	root, err := yamllite.Parse(c.file.Data)
	if err != nil {
		c.note("nicht lesbares YAML: %v", err)
		return
	}
	services := root.Get("services")
	if services == nil {
		return
	}
	for _, name := range services.MapKeys() {
		service := services.Get(name)
		key := "services." + name
		if image := service.Get("image"); image != nil && image.Str() != "" {
			reference := image.Str()
			note := ""
			if unresolved(reference) {
				note = "Wert aus Variable, nicht auflösbar: " + reference
			}
			imageName, version, pin, digest := imageEntry(reference)
			c.add(Entry{Ecosystem: EcoContainer, Name: imageName, KindOfThing: ThingImage,
				Version: version, Pin: pin, Digest: digest,
				SourceKey: key + ".image", SourceLine: image.At(), Note: note})
			continue
		}
		if build := service.Get("build"); build != nil {
			target := build.Str()
			if build.Kind == yamllite.Mapping {
				target = strings.TrimSpace(build.Get("context").Str() + " " + build.Get("dockerfile").Str())
			}
			c.add(Entry{Ecosystem: EcoContainer, Name: name, KindOfThing: ThingImage, Pin: PinLocal,
				SourceKey: key + ".build", SourceLine: build.At(),
				Note: strings.TrimSpace("lokal gebaut " + target)})
		}
	}
}

// parseDevcontainer liest die DevContainer-Konfiguration. Sie ist JSONC —
// Kommentare und nachlaufende Kommata sind erlaubt —, deshalb geht sie durch
// denselben Standardisierer wie die MCP-Konfiguration.
func parseDevcontainer(c *collector) {
	data, err := standardizeJSONC(c.file.Data)
	if err != nil {
		c.note("nicht lesbares JSONC: %v", err)
		return
	}
	var config struct {
		Image           string         `json:"image"`
		Build           map[string]any `json:"build"`
		DockerFile      string         `json:"dockerFile"`
		DockerComposeFi any            `json:"dockerComposeFile"`
		Features        map[string]any `json:"features"`
	}
	if err := unmarshalJSON(data, &config); err != nil {
		c.note("nicht lesbares JSONC: %v", err)
		return
	}
	finder := newLineFinder(c.file.Data)

	if config.Image != "" {
		name, version, pin, digest := imageEntry(config.Image)
		c.add(Entry{Ecosystem: EcoContainer, Name: name, KindOfThing: ThingImage, Version: version,
			Pin: pin, Digest: digest, SourceKey: "image", SourceLine: finder.find("image", 0)})
	}
	if config.DockerFile != "" || config.Build != nil {
		target := config.DockerFile
		if target == "" {
			if value, ok := config.Build["dockerfile"].(string); ok {
				target = value
			}
		}
		c.add(Entry{Ecosystem: EcoContainer, Name: "devcontainer", KindOfThing: ThingImage,
			Pin: PinLocal, SourceKey: "build", SourceLine: finder.find("build", 0),
			Note: strings.TrimSpace("lokal gebaut aus " + target)})
	}

	// features sind tool-Einträge; der Feature-Name ist der Gegenstand, die
	// Version steht hinter dem `:` oder im Feld `version`.
	for _, reference := range sortedKeys(config.Features) {
		name := reference
		version := ""
		if at := strings.LastIndex(reference, ":"); at > 0 {
			name, version = reference[:at], reference[at+1:]
		}
		if settings, ok := config.Features[reference].(map[string]any); ok {
			if value, ok := settings["version"].(string); ok && value != "" {
				version = value
			}
		}
		pin := classifyPin(version)
		if strings.EqualFold(version, "latest") {
			pin = PinFloating
		}
		c.add(Entry{Ecosystem: EcoContainer, Name: name, KindOfThing: ThingTool, Version: version,
			Pin: pin, SourceKey: "features." + reference, SourceLine: finder.find(reference, 0)})
	}
}
