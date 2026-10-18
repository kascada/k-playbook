package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// runBriefing gibt das Briefing aus — dieselbe Fachlogik, die das MCP-Werkzeug
// k_playbook_briefing als Hülle anbietet: welche Anweisungen, Regeln, Reviews
// und Leitlinien für dieses Projekt gelten und was in der Projekt-Collection
// für die eigene Arbeit ansteht, als Verweise im Präfixschema; dahinter der
// Rest von `k-playbook context`, den das Briefing noch nicht selbst abdeckt.
// Ohne --json steht der Klartext zum Lesen beim Testen; mit --json die Antwort
// als JSON, dieselbe Serialisierung wie über MCP.
func runBriefing(args []string) error {
	asJSON := false
	for _, arg := range args {
		switch arg {
		case "--json":
			asJSON = true
		case "-h", "--help", "help":
			printBriefingUsage(os.Stdout)
			return nil
		default:
			return fmt.Errorf("unbekannte Option %s — erwartet wird briefing [--json]", arg)
		}
	}

	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}
	briefing, err := project.BuildBriefing(context.Background(), projectDir)
	if err != nil {
		return err
	}

	if asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(briefing)
	}
	return writeBriefingText(os.Stdout, briefing)
}

// writeBriefingText rendert das Briefing zum Lesen: je Abschnitt die Verweise
// mit Herkunft in Klammern und Titel, dahinter die Beschreibung.
func writeBriefingText(out io.Writer, briefing project.Briefing) error {
	var b strings.Builder
	entries := func(list []project.BriefingEntry) {
		if len(list) == 0 {
			b.WriteString("  (keine)\n")
			return
		}
		for _, entry := range list {
			fmt.Fprintf(&b, "  %s  [%s]  %s", entry.Ref, entry.Origin, entry.Title)
			if entry.Description != "" {
				fmt.Fprintf(&b, " — %s", entry.Description)
			}
			b.WriteString("\n")
		}
	}

	fmt.Fprintf(&b, "Immer (%s):\n", briefing.Documents.Always.Ref)
	entries(briefing.Documents.Always.Parts)
	for _, problem := range briefing.Documents.Always.Problems {
		fmt.Fprintf(&b, "  Hinweis: %s\n", problem)
	}
	b.WriteString("Auf Abruf:\n")
	entries(briefing.Documents.OnDemand)
	b.WriteString("Regeln:\n")
	entries(briefing.Documents.Rules)
	b.WriteString("Reviews:\n")
	entries(briefing.Documents.Reviews)
	b.WriteString("Leitlinien:\n")
	entries(briefing.Documents.Guidelines)

	work := briefing.Work
	if !work.Available {
		fmt.Fprintf(&b, "Eigene Arbeit: Kephalaion nicht nutzbar — %s\n", work.Message)
	} else {
		fmt.Fprintf(&b, "Eigene Arbeit (%s):\n", work.User)
		docs := func(label string, list []project.BriefingDoc) {
			fmt.Fprintf(&b, "  %s:\n", label)
			if len(list) == 0 {
				b.WriteString("    (keine)\n")
				return
			}
			for _, doc := range list {
				fmt.Fprintf(&b, "    %s  %s\n", doc.Ref, doc.Title)
			}
		}
		docs("Offene Tasks", work.Tasks)
		docs("Aktive Arbeitsstände", work.Workbench)
		fmt.Fprintf(&b, "  Offene Befunde: %d\n", work.OpenFindings)
	}
	fmt.Fprintf(&b, "Projektdoku: %s\n", briefing.ProjectDocs)

	b.WriteString("\n" + briefingContextSeparator + "\n")
	writeBriefingContext(&b, briefing.Context)

	_, err := io.WriteString(out, b.String())
	return err
}

// briefingContextSeparator trennt im Klartext das Briefing vom Übergangsabschnitt.
const briefingContextSeparator = "---- Noch aus k-playbook context — wird abgebaut ----"

// writeBriefingContext rendert den Übergangsabschnitt knapp lesbar: je Feld
// eine Zeile, Objekte eingerückt mit ihren Feldern, Listen von Werten in einer
// Zeile, Listen von Objekten mit einem Eintrag je Zeile. Die Felder stehen in
// der Reihenfolge der Kontextausgabe; innerhalb eines Objekts alphabetisch.
func writeBriefingContext(b *strings.Builder, section project.BriefingContext) {
	if !section.Available {
		fmt.Fprintf(b, "  nicht verfügbar — %s\n", section.Message)
		return
	}
	for _, field := range section.Fields {
		writeContextValue(b, "  ", field.Name, decodeContextValue(field.Value))
	}
}

// decodeContextValue liest einen JSON-Wert in seine Go-Form; Zahlen bleiben
// Text, damit sie so dastehen wie in der Ausgabe.
func decodeContextValue(raw json.RawMessage) any {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return string(raw)
	}
	return value
}

func writeContextValue(b *strings.Builder, indent string, name string, value any) {
	switch v := value.(type) {
	case map[string]any:
		if len(v) == 0 {
			fmt.Fprintf(b, "%s%s: {}\n", indent, name)
			return
		}
		fmt.Fprintf(b, "%s%s:\n", indent, name)
		for _, key := range sortedKeys(v) {
			writeContextValue(b, indent+"  ", key, v[key])
		}
	case []any:
		if len(v) == 0 {
			fmt.Fprintf(b, "%s%s: (keine)\n", indent, name)
			return
		}
		if scalars, ok := scalarList(v); ok {
			fmt.Fprintf(b, "%s%s: %s\n", indent, name, strings.Join(scalars, ", "))
			return
		}
		fmt.Fprintf(b, "%s%s:\n", indent, name)
		for _, item := range v {
			fmt.Fprintf(b, "%s  - %s\n", indent, formatContextItem(item))
		}
	default:
		fmt.Fprintf(b, "%s%s: %s\n", indent, name, formatContextScalar(v))
	}
}

// formatContextItem schreibt einen Listeneintrag in eine Zeile: ein Objekt
// als `feld=wert`-Paare, alles andere als Wert.
func formatContextItem(item any) string {
	object, ok := item.(map[string]any)
	if !ok {
		return formatContextScalar(item)
	}
	parts := make([]string, 0, len(object))
	for _, key := range sortedKeys(object) {
		parts = append(parts, key+"="+formatContextScalar(object[key]))
	}
	return strings.Join(parts, ", ")
}

// formatContextScalar schreibt einen Wert in eine Zeile; was kein Skalar ist,
// steht als kompaktes JSON.
func formatContextScalar(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return v
	case bool, json.Number:
		return fmt.Sprint(v)
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(encoded)
	}
}

func scalarList(items []any) ([]string, bool) {
	scalars := make([]string, 0, len(items))
	for _, item := range items {
		switch item.(type) {
		case map[string]any, []any:
			return nil, false
		}
		scalars = append(scalars, formatContextScalar(item))
	}
	return scalars, true
}

func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func printBriefingUsage(out io.Writer) {
	fmt.Fprint(out, `k-playbook briefing — was für dieses Projekt gilt und was ansteht

Aufruf:
  k-playbook briefing [--json]

Listet als Daten, was eine Sitzung zu Beginn wissen muss — Verweise im
Präfixschema (docs/verweisschema.md), keine Pfade, keine Inhalte:

  - die immer geltenden Anweisungen k-playbook:/instructions/always/ mit
    ihren Teilen, die Anweisungen auf Abruf, Regeln, Reviews und Leitlinien —
    lokal und mitgeliefert schon gegeneinander aufgelöst, je Eintrag Verweis,
    Titel, Beschreibung und Herkunft (dist, local, override, disabled);
  - aus der Projekt-Collection die eigenen offenen Tasks und aktiven
    Arbeitsstände mit Titel und die Zahl der offenen Befunde — offen heißt
    nicht in done/; ist Kephalaion nicht nutzbar, steht das dort;
  - den Einstieg der Projektdoku als Verweis;
  - hinter einer Trennzeile den Rest von k-playbook context — die
    Umgebungsangaben (Pfade, Werkzeuge, gh, git, Kephalaion-Anbindung …), die
    das Briefing noch nicht selbst abdeckt; ohne catalogs.rules,
    catalogs.reviews und guidelines, die oben schon stehen. Der Abschnitt wird
    Feld für Feld abgebaut; ist er leer, entfällt k-playbook context. Lässt
    sich die Kontextausgabe nicht bauen, steht das dort — kein Fehler.

Gelesen wird ein Eintrag über k_playbook_doc_read bzw. k-playbook read mit
dem Verweis. Gesucht wird das Projekt ab dem Arbeitsverzeichnis aufwärts. Wie
k-playbook context zieht der Aufruf dabei die Assistenten-Verlinkung nach; was
sich geändert hat, steht im Rest unter links.

Ohne --json steht der Klartext zum Lesen. Mit --json kommt die Antwort als
JSON mit schemaVersion, documents (always, onDemand, rules, reviews,
guidelines), work (available, message, user, tasks, workbench, openFindings),
projectDocs und context (available, message, dazu die übrigen Felder der
Kontextausgabe unter ihren Namen) — dieselbe Antwort wie das MCP-Werkzeug
k_playbook_briefing.
`)
}
