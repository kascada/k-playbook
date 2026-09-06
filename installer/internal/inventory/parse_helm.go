package inventory

import (
	"strconv"

	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

func parseHelm(c *collector) {
	root, err := yamllite.Parse(c.file.Data)
	if err != nil {
		c.note("nicht lesbares YAML: %v", err)
		return
	}
	switch {
	case c.file.Base == "Chart.yaml":
		parseChartYAML(c, root)
	case c.file.Base == "Chart.lock":
		parseChartLock(c, root)
	default:
		parseValuesFile(c, root)
	}
}

func parseChartYAML(c *collector, root *yamllite.Node) {
	if name := root.Get("name"); name != nil && name.Str() != "" {
		if version := root.Get("version"); version != nil && version.Str() != "" {
			c.add(Entry{Ecosystem: EcoHelm, Name: name.Str(), KindOfThing: ThingChart,
				Version: version.Str(), SourceKey: "version", SourceLine: version.At()})
		}
		// appVersion ist die Version der Anwendung, die das Chart ausliefert —
		// ein runtime-Eintrag, wie es der Vertrag sagt. Sie steht deshalb im
		// Ökosystem `runtime` und nicht neben der Chart-Version: sonst lägen zwei
		// verschiedene Dinge in einer Gruppe und erzeugten eine Abweichung, die
		// keine ist.
		if appVersion := root.Get("appVersion"); appVersion != nil && appVersion.Str() != "" {
			c.add(Entry{Ecosystem: EcoRuntime, Name: name.Str(), KindOfThing: ThingRuntime,
				Version: appVersion.Str(), SourceKey: "appVersion", SourceLine: appVersion.At()})
		}
	}
	addChartDependencies(c, root.Get("dependencies"), "dependencies")
}

func parseChartLock(c *collector, root *yamllite.Node) {
	addChartDependencies(c, root.Get("dependencies"), "dependencies")
}

func addChartDependencies(c *collector, list *yamllite.Node, section string) {
	for index, item := range list.List() {
		name := item.Get("name").Str()
		if name == "" {
			continue
		}
		version := item.Get("version").Str()
		pin := classifyPin(version)
		digest := ""
		if value := item.Get("digest").Str(); value != "" {
			digest = value
			pin = PinDigest
		}
		line := item.Get("version").At()
		if line == 0 {
			line = item.At()
		}
		c.add(Entry{Ecosystem: EcoHelm, Name: name, KindOfThing: ThingChartDependency,
			Version: version, Pin: pin, Digest: digest,
			SourceKey: sectionIndex(section, index) + ".version", SourceLine: line})
	}
}

func sectionIndex(section string, index int) string {
	return section + "[" + strconv.Itoa(index) + "]"
}

// parseValuesFile sucht Image-Referenzen. Es gelten genau zwei Formen: das
// Schlüsselpaar `image.repository` + `image.tag` und ein einzelner
// `image`-String. Andere Schlüssel werden nicht geraten — ein Inventar, das
// Werte errät, ist keines.
func parseValuesFile(c *collector, root *yamllite.Node) {
	walkValues(c, root, "")
}

func walkValues(c *collector, node *yamllite.Node, prefix string) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yamllite.Mapping:
		for _, key := range node.MapKeys() {
			child := node.Get(key)
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			if key == "image" {
				if addValuesImage(c, child, path) {
					continue
				}
			}
			walkValues(c, child, path)
		}
	case yamllite.Sequence:
		for index, item := range node.Items {
			walkValues(c, item, prefix+"["+strconv.Itoa(index)+"]")
		}
	}
}

func addValuesImage(c *collector, node *yamllite.Node, path string) bool {
	if node == nil {
		return false
	}
	if node.Kind == yamllite.Scalar {
		if node.Value == "" {
			return false
		}
		name, version, pin, digest := imageEntry(node.Value)
		c.add(Entry{Ecosystem: EcoContainer, Name: name, KindOfThing: ThingImage, Version: version,
			Pin: pin, Digest: digest, SourceKey: path, SourceLine: node.At()})
		return true
	}
	if node.Kind != yamllite.Mapping {
		return false
	}
	repository := node.Get("repository")
	if repository == nil || repository.Str() == "" {
		return false
	}
	reference := repository.Str()
	if tag := node.Get("tag"); tag != nil && tag.Str() != "" {
		reference += ":" + tag.Str()
	}
	if digest := node.Get("digest"); digest != nil && digest.Str() != "" {
		reference += "@" + digest.Str()
	}
	line := repository.At()
	name, version, pin, digestValue := imageEntry(reference)
	c.add(Entry{Ecosystem: EcoContainer, Name: name, KindOfThing: ThingImage, Version: version,
		Pin: pin, Digest: digestValue, SourceKey: path + ".repository", SourceLine: line})
	return true
}
