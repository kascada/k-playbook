package inventory

import (
	"fmt"
	"strconv"
	"strings"
)

// GeneratedBy ist der Erzeuger im Frontmatter. Er steht auf beiden
// Aufrufwegen gleich: die Herkunft benennt den Erzeuger, nicht den Aufrufweg.
const GeneratedBy = "k-doc-inventory"

// Titel und Beschreibung stehen wortgleich im Vertrag. Ein fehlendes `title`
// oder `description` wird von /k-docs Schritt 3 und /k-docs-index Schritt 4 als
// Befund gemeldet — `generated.by` allein genügt nicht.
const (
	frontmatterType        = "Version Inventory"
	frontmatterTitle       = "Versionsinventar"
	frontmatterDescription = "Vollständige Übersicht der deklarierten Versionen dieses Projekts, nach Umgebung getrennt und mit Herkunft je Zeile."
)

// Render erzeugt die Inventardatei. Der Zeitstempel wird hereingereicht, damit
// derselbe Bestand zweimal gerendert werden kann — einmal mit dem Zeitstempel
// des Bestands für den Vergleich, einmal mit dem des Laufs zum Schreiben. Genau
// darauf beruht die Byte-Stabilitätsregel.
func Render(result Result, at string) string {
	var out strings.Builder

	out.WriteString("---\n")
	fmt.Fprintf(&out, "type: %s\n", frontmatterType)
	fmt.Fprintf(&out, "title: %s\n", frontmatterTitle)
	fmt.Fprintf(&out, "description: %s\n", frontmatterDescription)
	out.WriteString("tags: [versions, inventory, dependencies]\n")
	out.WriteString("status: stable\n")
	fmt.Fprintf(&out, "generated: { by: %s, at: %s }\n", GeneratedBy, at)
	out.WriteString("inventory:\n")
	fmt.Fprintf(&out, "  sources-configured: %d\n", result.ConfiguredSources)
	fmt.Fprintf(&out, "  sources-read: %d\n", len(result.Sources))
	fmt.Fprintf(&out, "  entries: %d\n", len(result.Entries))
	fmt.Fprintf(&out, "  deviations: %d\n", len(result.Deviations))
	fmt.Fprintf(&out, "  rejected: %d\n", len(result.Rejections))
	fmt.Fprintf(&out, "  sources-excluded: %d\n", excludedCount(result))
	out.WriteString("---\n\n")

	fmt.Fprintf(&out, "# %s\n\n", frontmatterTitle)
	fmt.Fprintf(&out, "Erzeugt von `%s` am %s. Diese Datei wird bei jedem Lauf neu erzeugt;\n", GeneratedBy, at)
	out.WriteString("Änderungen von Hand gehen dabei verloren. Sie führt die **deklarierten**\n")
	out.WriteString("Versionen aus den Quellen des Projekts — nicht das, was zur Laufzeit aktiv ist.\n\n")

	renderOverview(&out, result)
	renderContexts(&out, result)
	renderDeviations(&out, result)
	renderSources(&out, result)
	renderExclusions(&out, result)
	renderRejections(&out, result)

	return out.String()
}

func renderOverview(out *strings.Builder, result Result) {
	out.WriteString("## Übersicht\n\n")
	fmt.Fprintf(out, "- Einträge: %d\n", len(result.Entries))
	fmt.Fprintf(out, "- Ausgewertete Quellen: %d\n", len(result.Sources))
	fmt.Fprintf(out, "- Konfigurierte Zusatzquellen: %d\n", result.ConfiguredSources)
	fmt.Fprintf(out, "- Abweichungen: %d\n", len(result.Deviations))
	fmt.Fprintf(out, "- Abgelehnte Quellen: %d\n", len(result.Rejections))
	fmt.Fprintf(out, "- Nicht durchsuchte Quellen: %d\n", excludedCount(result))
	fmt.Fprintf(out, "- Hinweise: %d\n\n", len(result.Notes))
}

// excludedCount ist die Zahl der Quellen, die von einer Ausschlussregel
// übergangen wurden. Sie steht in der Übersicht und im Frontmatter, damit
// „hier wurde nichts gefunden" und „hier wurde nicht gesucht" auch maschinell
// zwei verschiedene Aussagen bleiben.
func excludedCount(result Result) int {
	total := 0
	for _, exclusion := range result.Exclusions {
		total += exclusion.Skipped
	}
	return total
}

// renderExclusions schreibt, wo die Standarderkennung nicht gesucht hat.
//
// Der Abschnitt steht immer, auch wenn keine Regel etwas getroffen hat: ein
// Ausschluss, den niemand sieht, wäre ein stiller Filter, und den gibt es hier
// nicht.
func renderExclusions(out *strings.Builder, result Result) {
	out.WriteString("## Nicht durchsuchte Bereiche\n\n")
	out.WriteString("Diese Bereiche liegen im Projekt, werden aber von der Standarderkennung nicht\n")
	out.WriteString("gesucht. Sie sind nicht gesperrt: eine Quelle daraus kommt ins Inventar, sobald\n")
	out.WriteString("sie in `sources:` der Quellenkonfiguration steht.\n\n")
	if len(result.Exclusions) == 0 {
		out.WriteString("Keine.\n\n")
		return
	}
	out.WriteString("| Muster | Herkunft | Übergangene Quellen | Grund |\n")
	out.WriteString("|---|---|---|---|\n")
	for _, exclusion := range result.Exclusions {
		fmt.Fprintf(out, "| %s | %s | %d | %s |\n",
			code(exclusion.Pattern), cell(exclusion.Origin), exclusion.Skipped, cell(exclusion.Reason))
	}
	out.WriteString("\n")
}

// renderContexts schreibt je Umgebungslabel einen Abschnitt, in der festen
// Reihenfolge des Vertrags. Label ohne Einträge werden weggelassen.
func renderContexts(out *strings.Builder, result Result) {
	present := contextsPresent(result.Entries)
	if len(present) == 0 {
		out.WriteString("## Einträge\n\nKeine.\n\n")
		return
	}
	for _, env := range present {
		fmt.Fprintf(out, "## %s\n\n", env)
		out.WriteString("| Gegenstand | Art | Version | Pin | Scope | Herkunft |\n")
		out.WriteString("|---|---|---|---|---|---|\n")
		for _, entry := range result.Entries {
			if entry.Context != env {
				continue
			}
			fmt.Fprintf(out, "| %s | %s | %s | %s | %s | %s |\n",
				code(entry.Group), cell(entry.KindOfThing), code(entry.Version),
				cell(entry.Pin), cell(entry.Scope), origin(entry))
		}
		out.WriteString("\n")
	}
}

// origin ist die Herkunftszelle: relative Datei, Zeile und Schlüssel. Zusammen
// müssen sie ausreichen, um die Aussage ohne Suche wiederzufinden.
func origin(entry Entry) string {
	location := entry.SourceFile
	if entry.SourceLine > 0 {
		location += ":" + strconv.Itoa(entry.SourceLine)
	}
	parts := []string{code(location), code(entry.SourceKey)}
	if entry.Digest != "" {
		parts = append(parts, code(entry.Digest))
	}
	if entry.Note != "" {
		parts = append(parts, cell(entry.Note))
	}
	return strings.Join(parts, " · ")
}

func renderDeviations(out *strings.Builder, result Result) {
	out.WriteString("## Abweichungen\n\n")
	if len(result.Deviations) == 0 {
		out.WriteString("Keine.\n\n")
		return
	}
	out.WriteString("Eine Abweichung wird ausgewiesen, nicht aufgelöst: `widersprüchlich` heißt,\n")
	out.WriteString("dass dieselbe Umgebung Verschiedenes sagt, `umgebungsbedingt`, dass verschiedene\n")
	out.WriteString("Umgebungen es tun. Gezählt werden Gruppen, nicht Zeilen.\n\n")
	for _, deviation := range result.Deviations {
		fmt.Fprintf(out, "### %s — `%s`\n\n", deviation.Art, deviation.Group)
		out.WriteString("| Version | Pin | Kontext | Herkunft |\n")
		out.WriteString("|---|---|---|---|\n")
		for _, entry := range deviation.Entries {
			fmt.Fprintf(out, "| %s | %s | %s | %s |\n",
				code(entry.Version), cell(entry.Pin), cell(entry.Context), origin(entry))
		}
		out.WriteString("\n")
	}
}

func renderSources(out *strings.Builder, result Result) {
	out.WriteString("## Ausgewertete Quellen\n\n")
	if len(result.Sources) == 0 {
		out.WriteString("Keine.\n\n")
		return
	}
	out.WriteString("| Datei | Quellart | Label | Einträge |\n")
	out.WriteString("|---|---|---|---|\n")
	for _, source := range result.Sources {
		kind := source.Kind
		if source.Configured {
			kind += " (konfiguriert)"
		}
		fmt.Fprintf(out, "| %s | %s | %s | %d |\n",
			code(source.File), cell(kind), cell(source.Env), source.Entries)
	}
	out.WriteString("\n")
}

func renderRejections(out *strings.Builder, result Result) {
	out.WriteString("## Abgelehnte Quellen und Hinweise\n\n")
	if len(result.Rejections) == 0 && len(result.Notes) == 0 {
		out.WriteString("Keine.\n")
		return
	}
	if len(result.Rejections) > 0 {
		out.WriteString("| Angefragt | Aufgelöst | Grund |\n")
		out.WriteString("|---|---|---|\n")
		for _, rejection := range result.Rejections {
			fmt.Fprintf(out, "| %s | %s | %s |\n",
				code(rejection.Requested), code(rejection.Resolved), cell(rejection.Reason))
		}
		out.WriteString("\n")
	}
	for _, note := range result.Notes {
		if note.Source == "" {
			fmt.Fprintf(out, "- %s\n", cell(note.Text))
			continue
		}
		fmt.Fprintf(out, "- `%s`: %s\n", note.Source, cell(note.Text))
	}
	if len(result.Notes) > 0 {
		out.WriteString("\n")
	}
}
