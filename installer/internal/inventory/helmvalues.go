package inventory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/versionsources"
	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// Konfigurierte Helm-Werte (`helm_values:` in version-sources.yaml).
//
// Ein solcher Eintrag macht eine Datei nicht zur Quelle: er wird angewandt,
// wenn die Datei ohnehin als Helm-values gelesen wird, und bekommt deren
// Kontext. Jeder Eintrag, der keine Zeile ergibt, hinterlässt einen Hinweis mit
// Grund — ein konfigurierter Wert, der still wirkungslos bleibt, wäre eine
// Lücke, die niemand sieht.

// configuredValueNote steht an jeder Zeile aus einem konfigurierten Helm-Wert.
// Ohne sie wäre nicht zu erkennen, warum ausgerechnet dieser `tag:` im Inventar
// steht, obwohl andere Schlüssel nicht geraten werden.
const configuredValueNote = "konfiguriert in version-sources.yaml"

// valueTarget ist ein konfigurierter Helm-Wert, bezogen auf genau eine Datei.
// Ein Glob ergibt mehrere davon, damit jede getroffene Datei ihren eigenen
// Hinweis bekommen kann.
type valueTarget struct {
	Rule versionsources.HelmValue
	// Resolved ist der geprüfte, absolute Pfad der Datei.
	Resolved string
	// Handled ist gesetzt, sobald die Datei als Helm-values ausgewertet und der
	// Eintrag darin geprüft wurde — mit Zeile oder mit inhaltlichem Hinweis.
	Handled bool
	// Reason nennt, warum der Eintrag nicht geprüft werden konnte, sobald das
	// beim Lesen der Datei feststeht.
	Reason string
}

// valueNote formuliert den Hinweis zu einem Eintrag, der keine Zeile ergibt.
func valueNote(rule versionsources.HelmValue, reason string) string {
	return fmt.Sprintf("konfigurierter Helm-Wert %s → %s (version-sources.yaml, Zeile %d) nicht angewandt: %s",
		rule.Key, rule.Item, rule.Line, reason)
}

// planHelmValues löst die Pfade der gültigen Einträge über die
// Vertrauensgrenze auf. Was schon hier scheitert — außerhalb der Wurzeln, kein
// Treffer —, wird sofort zum Hinweis.
func planHelmValues(boundary *Boundary, rules []versionsources.HelmValue) (map[string][]*valueTarget, []*valueTarget, []Note) {
	byFile := map[string][]*valueTarget{}
	var all []*valueTarget
	var notes []Note
	for _, rule := range rules {
		paths, rejections := boundary.Expand(rule.Path)
		for _, rejection := range rejections {
			notes = append(notes, Note{Source: rule.Path, Text: valueNote(rule, rejection.Reason)})
		}
		if len(paths) == 0 && len(rejections) == 0 {
			notes = append(notes, Note{Source: rule.Path, Text: valueNote(rule, "das Muster trifft keine Datei")})
		}
		seen := map[string]bool{}
		for _, path := range paths {
			resolved, err := boundary.Check(path)
			if err != nil {
				notes = append(notes, Note{Source: rule.Path, Text: valueNote(rule, err.(*PathError).Reason)})
				continue
			}
			if seen[resolved] {
				continue
			}
			seen[resolved] = true
			target := &valueTarget{Rule: rule, Resolved: resolved}
			byFile[resolved] = append(byFile[resolved], target)
			all = append(all, target)
		}
	}
	return byFile, all, notes
}

// markUnread hält für die Einträge einer Datei fest, warum sie nicht geprüft
// wurden, nachdem die Datei gelesen oder abgelehnt worden ist.
func markUnread(targets []*valueTarget, reason string) {
	for _, target := range targets {
		if !target.Handled && target.Reason == "" {
			target.Reason = reason
		}
	}
}

// finishHelmValues schreibt die Hinweise für alle Einträge, die keine Zeile
// ergeben haben und noch keinen inhaltlichen Hinweis tragen. Eine Datei, die
// gar nicht als Quelle geplant war, bekommt hier ihren Grund: fehlt sie, ist
// sie ausgeschlossen, oder ist sie schlicht keine Quelle des Inventars.
func finishHelmValues(boundary *Boundary, targets []*valueTarget, excludes []string, result *Result) {
	for _, target := range targets {
		if target.Handled {
			continue
		}
		reason := target.Reason
		if reason == "" {
			reason = unplannedReason(boundary, target.Resolved, excludes)
		}
		result.Notes = append(result.Notes, Note{
			Source: displayPath(boundary.ProjectRoot(), target.Resolved),
			Text:   valueNote(target.Rule, reason),
		})
	}
}

func unplannedReason(boundary *Boundary, resolved string, excludes []string) string {
	// Nur nachgesehen, ob etwas dort liegt — geöffnet wird nichts. Gelesen
	// wird ausschließlich über Boundary.ReadFile.
	if _, err := os.Stat(resolved); err != nil {
		return "die Datei liegt nicht auf der Platte"
	}
	if relative, err := filepath.Rel(boundary.ProjectRoot(), resolved); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		relative = filepath.ToSlash(relative)
		for _, pattern := range append([]string{InstallationDirName + "/**"}, excludes...) {
			if matchExclude(pattern, relative) {
				return fmt.Sprintf("die Datei ist durch die Ausschlussregel `%s` von der Standarderkennung ausgenommen; als Quelle unter sources: eintragen", pattern)
			}
		}
	}
	return "die Datei ist keine Quelle des Inventars; als Quelle unter sources: mit kind: helm eintragen"
}

// applyHelmValues prüft die konfigurierten Einträge einer values-Datei.
func applyHelmValues(c *collector, root *yamllite.Node) {
	for _, target := range c.file.Values {
		target.Handled = true
		rule := target.Rule
		steps, ok := versionsources.ParseKeyPath(rule.Key)
		if !ok {
			// Die Quellenkonfiguration lehnt einen solchen Pfad ab, bevor er
			// hier ankommt; der Zweig hält nur die Zusage „nie still".
			c.note("%s", valueNote(rule, "ungültiger Schlüsselpfad"))
			continue
		}
		node := lookupKeyPath(root, steps)
		switch {
		case node == nil:
			c.note("%s", valueNote(rule, "der Schlüssel fehlt in der Datei"))
			continue
		case node.Kind != yamllite.Scalar:
			c.note("%s", valueNote(rule, "der Schlüssel hat keinen Skalar"))
			continue
		}
		version := node.Value
		pin := tagPin(version)
		note := configuredValueNote
		if pin == PinUnknown {
			note = unknownReason(version) + "; " + configuredValueNote
		}
		c.add(Entry{Ecosystem: EcoContainer, Name: rule.ItemName(), KindOfThing: ThingImage,
			Version: version, Pin: pin, SourceKey: rule.Key, SourceLine: node.At(), Note: note})
	}
}

// lookupKeyPath folgt einem zerlegten Schlüsselpfad. nil heißt: nicht da.
func lookupKeyPath(root *yamllite.Node, steps []versionsources.KeyStep) *yamllite.Node {
	current := root
	for _, step := range steps {
		current = current.Get(step.Key)
		for _, index := range step.Indexes {
			if current == nil || current.Kind != yamllite.Sequence || index >= len(current.Items) {
				return nil
			}
			current = current.Items[index]
		}
		if current == nil {
			return nil
		}
	}
	return current
}

// tagPin wendet die Pin-Regeln der Container-Tags auf einen einzelnen Tag an:
// leer oder ein beweglicher Tag ist `floating`, eine nicht auflösbare Variable
// `unknown`, jeder andere hingeschriebene Tag `exact`.
func tagPin(tag string) string {
	switch {
	case unresolved(tag):
		return PinUnknown
	case strings.TrimSpace(tag) == "" || movingTag(tag):
		return PinFloating
	}
	return PinExact
}
