package inventory

import (
	"strconv"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// setupActions sind die Setup-Actions mit Versionseingabe. Sie liefern
// zusätzlich zum action-Eintrag einen runtime-Eintrag — genau der macht die
// Abweichung zwischen der Sprachversion in CI und der lokalen sichtbar.
var setupActions = map[string]struct {
	input   string
	runtime string
}{
	"actions/setup-go":     {"go-version", "go"},
	"actions/setup-node":   {"node-version", "node"},
	"actions/setup-python": {"python-version", "python"},
	"actions/setup-java":   {"java-version", "java"},
	"ruby/setup-ruby":      {"ruby-version", "ruby"},
}

func parseCI(c *collector) {
	root, err := yamllite.Parse(c.file.Data)
	if err != nil {
		c.note("nicht lesbares YAML: %v", err)
		return
	}
	if jobs := root.Get("jobs"); jobs != nil {
		for _, jobName := range jobs.MapKeys() {
			parseCIJob(c, jobs.Get(jobName), "jobs."+jobName)
		}
	}
	// GitLab kennt keine jobs-Ebene: dort ist jeder Schlüssel oberster Ebene ein
	// Job, und `image:` steht auch global.
	parseCIImages(c, root, "")
	for _, key := range root.MapKeys() {
		if key == "jobs" || strings.HasPrefix(key, ".") {
			continue
		}
		child := root.Get(key)
		if child != nil && child.Kind == yamllite.Mapping {
			parseCIImages(c, child, key)
		}
	}
}

func parseCIJob(c *collector, job *yamllite.Node, prefix string) {
	parseCIImages(c, job, prefix)
	for index, step := range job.Get("steps").List() {
		key := prefix + ".steps[" + strconv.Itoa(index) + "]"
		uses := step.Get("uses")
		if uses == nil || uses.Str() == "" {
			continue
		}
		addCIAction(c, uses.Str(), key+".uses", uses.At())
		addSetupRuntime(c, step, uses.Str(), key)
	}
}

func addCIAction(c *collector, reference string, sourceKey string, line int) {
	name := reference
	version := ""
	if at := strings.LastIndex(reference, "@"); at > 0 {
		name, version = reference[:at], reference[at+1:]
	}
	// Ein voller Commit-SHA ist digest, ein Tag exact, ein Branch floating.
	pin := PinFloating
	digest := ""
	switch {
	case commitSHA.MatchString(version):
		pin = PinDigest
		digest = version
	case version == "":
		pin = PinFloating
	case exactVersion.MatchString(version):
		pin = PinExact
	default:
		pin = PinFloating
	}
	if strings.HasPrefix(name, "./") || strings.HasPrefix(name, "docker://") {
		pin = PinLocal
	}
	c.add(Entry{Ecosystem: EcoCI, Name: name, KindOfThing: ThingAction, Version: version,
		Pin: pin, Digest: digest, SourceKey: sourceKey, SourceLine: line})
}

func addSetupRuntime(c *collector, step *yamllite.Node, reference string, key string) {
	action := reference
	if at := strings.LastIndex(reference, "@"); at > 0 {
		action = reference[:at]
	}
	setup, known := setupActions[action]
	if !known {
		return
	}
	value := step.Get("with", setup.input)
	if value == nil || value.Str() == "" {
		return
	}
	c.add(Entry{Ecosystem: EcoRuntime, Name: setup.runtime, KindOfThing: ThingRuntime,
		Version: value.Str(), SourceKey: key + ".with." + setup.input, SourceLine: value.At()})
}

// parseCIImages liest `container:`, `image:` und `services.*.image`.
func parseCIImages(c *collector, node *yamllite.Node, prefix string) {
	if node == nil {
		return
	}
	join := func(key string) string {
		if prefix == "" {
			return key
		}
		return prefix + "." + key
	}
	for _, key := range []string{"container", "image"} {
		child := node.Get(key)
		if child == nil {
			continue
		}
		reference := child.Str()
		line := child.At()
		if child.Kind == yamllite.Mapping {
			inner := child.Get("image")
			if inner == nil {
				continue
			}
			reference = inner.Str()
			line = inner.At()
		}
		if reference == "" {
			continue
		}
		addCIImage(c, reference, join(key), line)
	}
	services := node.Get("services")
	if services == nil {
		return
	}
	switch services.Kind {
	case yamllite.Mapping:
		for _, name := range services.MapKeys() {
			service := services.Get(name)
			image := service.Get("image")
			if image == nil || image.Str() == "" {
				continue
			}
			addCIImage(c, image.Str(), join("services."+name+".image"), image.At())
		}
	case yamllite.Sequence:
		for index, item := range services.Items {
			reference := item.Str()
			line := item.At()
			if item.Kind == yamllite.Mapping {
				image := item.Get("name")
				if image == nil {
					image = item.Get("image")
				}
				if image == nil {
					continue
				}
				reference = image.Str()
				line = image.At()
			}
			if reference == "" {
				continue
			}
			addCIImage(c, reference, join("services["+strconv.Itoa(index)+"]"), line)
		}
	}
}

func addCIImage(c *collector, reference string, sourceKey string, line int) {
	name, version, pin, digest := imageEntry(reference)
	note := ""
	if unresolved(reference) {
		note = "Wert aus Variable, nicht auflösbar: " + reference
	}
	c.add(Entry{Ecosystem: EcoContainer, Name: name, KindOfThing: ThingImage, Version: version,
		Pin: pin, Digest: digest, SourceKey: sourceKey, SourceLine: line, Note: note})
}
