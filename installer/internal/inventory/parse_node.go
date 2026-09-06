package inventory

import (
	"encoding/json"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// Node: aus Lockfiles werden nur direkte Abhängigkeiten geführt. Transitive
// würden das Inventar unlesbar machen, ohne eine Frage zu beantworten, die es
// stellt.
func parseNode(c *collector) {
	switch c.file.Base {
	case "package.json":
		parsePackageJSON(c)
	case "package-lock.json":
		parsePackageLock(c)
	case "yarn.lock":
		parseYarnLock(c)
	case "pnpm-lock.yaml":
		parsePnpmLock(c)
	case ".nvmrc":
		parseVersionFile(c, "node", "runtime.node")
	case ".node-version":
		parseVersionFile(c, "node", "runtime.node")
	}
}

type packageManifest struct {
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
	Engines              map[string]string `json:"engines"`
	PackageManager       string            `json:"packageManager"`
}

var nodeScopes = []struct {
	key   string
	scope string
}{
	{"dependencies", "main"},
	{"devDependencies", "dev"},
	{"optionalDependencies", "optional"},
	{"peerDependencies", "optional"},
}

func parsePackageJSON(c *collector) {
	var manifest packageManifest
	if err := json.Unmarshal(c.file.Data, &manifest); err != nil {
		c.note("nicht lesbares JSON: %v", err)
		return
	}
	finder := newLineFinder(c.file.Data)
	sections := map[string]map[string]string{
		"dependencies":         manifest.Dependencies,
		"devDependencies":      manifest.DevDependencies,
		"optionalDependencies": manifest.OptionalDependencies,
		"peerDependencies":     manifest.PeerDependencies,
	}
	for _, section := range nodeScopes {
		block := sections[section.key]
		sectionLine := finder.find(section.key, 0)
		for _, name := range sortedKeys(block) {
			c.add(Entry{Ecosystem: EcoNode, Name: name, KindOfThing: ThingPackage,
				Version: block[name], Scope: section.scope,
				SourceKey: section.key + "." + name, SourceLine: finder.find(name, sectionLine)})
		}
	}
	if node, ok := manifest.Engines["node"]; ok {
		c.add(Entry{Ecosystem: EcoRuntime, Name: "node", KindOfThing: ThingRuntime, Version: node,
			SourceKey: "engines.node", SourceLine: finder.find("node", finder.find("engines", 0))})
	}
	if manifest.PackageManager != "" {
		name, version, _ := strings.Cut(manifest.PackageManager, "@")
		c.add(Entry{Ecosystem: EcoRuntime, Name: name, KindOfThing: ThingRuntime, Version: version,
			SourceKey: "packageManager", SourceLine: finder.find("packageManager", 0)})
	}
}

func parsePackageLock(c *collector) {
	var lock struct {
		Packages map[string]struct {
			Version              string            `json:"version"`
			Dependencies         map[string]string `json:"dependencies"`
			DevDependencies      map[string]string `json:"devDependencies"`
			OptionalDependencies map[string]string `json:"optionalDependencies"`
			Resolved             string            `json:"resolved"`
			Link                 bool              `json:"link"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(c.file.Data, &lock); err != nil {
		c.note("nicht lesbares JSON: %v", err)
		return
	}
	root, ok := lock.Packages[""]
	if !ok {
		c.note("kein Wurzelpaket in packages[\"\"] — die direkten Abhängigkeiten sind nicht zu erkennen")
		return
	}
	finder := newLineFinder(c.file.Data)
	direct := map[string]string{}
	for name := range root.Dependencies {
		direct[name] = "main"
	}
	for name := range root.DevDependencies {
		direct[name] = "dev"
	}
	for name := range root.OptionalDependencies {
		direct[name] = "optional"
	}
	for _, name := range sortedKeys(direct) {
		key := "node_modules/" + name
		entry, found := lock.Packages[key]
		if !found {
			continue
		}
		pin := PinExact
		note := ""
		if entry.Link {
			pin = PinLocal
			note = "verlinkt auf " + entry.Resolved
		}
		c.add(Entry{Ecosystem: EcoNode, Name: name, KindOfThing: ThingPackage,
			Version: entry.Version, Pin: pin, Scope: direct[name],
			SourceKey: "packages." + key, SourceLine: finder.find(key, 0), Note: note})
	}
}

// parseYarnLock liest das eigene Format von Yarn: ein Kopf mit den
// angeforderten Bereichen, darunter die aufgelöste Version.
func parseYarnLock(c *collector) {
	lines := c.lines()
	var heads []string
	var headLine int
	for index, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && strings.HasSuffix(trimmed, ":") {
			heads = nil
			headLine = index + 1
			for _, part := range strings.Split(strings.TrimSuffix(trimmed, ":"), ",") {
				heads = append(heads, strings.Trim(strings.TrimSpace(part), `"`))
			}
			continue
		}
		if strings.HasPrefix(trimmed, "version ") || strings.HasPrefix(trimmed, "version: ") {
			version := strings.Trim(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmed, "version"), ":")), `"`)
			for _, head := range heads {
				name := head
				if at := strings.LastIndex(head, "@"); at > 0 {
					name = head[:at]
				}
				c.add(Entry{Ecosystem: EcoNode, Name: name, KindOfThing: ThingPackage,
					Version: version, Pin: PinExact,
					SourceKey: head, SourceLine: headLine})
			}
			heads = nil
		}
	}
}

func parsePnpmLock(c *collector) {
	root, err := yamllite.Parse(c.file.Data)
	if err != nil {
		c.note("nicht lesbares YAML: %v", err)
		return
	}
	importers := root.Get("importers")
	if importers == nil {
		addPnpmSection(c, root, "", "dependencies", "main")
		addPnpmSection(c, root, "", "devDependencies", "dev")
		return
	}
	for _, importer := range importers.MapKeys() {
		addPnpmSection(c, importers.Get(importer), "importers."+importer+".", "dependencies", "main")
		addPnpmSection(c, importers.Get(importer), "importers."+importer+".", "devDependencies", "dev")
	}
}

func addPnpmSection(c *collector, parent *yamllite.Node, prefix string, section string, scope string) {
	block := parent.Get(section)
	if block == nil {
		return
	}
	for _, name := range block.MapKeys() {
		item := block.Get(name)
		version := item.Str()
		if item != nil && item.Kind == yamllite.Mapping {
			version = item.Get("specifier").Str()
			if version == "" {
				version = item.Get("version").Str()
			}
		}
		c.add(Entry{Ecosystem: EcoNode, Name: name, KindOfThing: ThingPackage, Version: version,
			Scope: scope, SourceKey: prefix + section + "." + name, SourceLine: item.At()})
	}
}
