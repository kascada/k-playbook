package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// runBriefing gibt das Briefing aus — dieselbe Fachlogik, die das MCP-Werkzeug
// k_playbook_briefing als Hülle anbietet: welche Anweisungen, Regeln, Reviews
// und Leitlinien für dieses Projekt gelten und was in der Projekt-Collection
// für die eigene Arbeit ansteht, als Verweise im Präfixschema. Ohne --json
// steht der Klartext zum Lesen beim Testen; mit --json die Antwort als JSON,
// dieselbe Serialisierung wie über MCP. Umgebungsangaben stehen nicht darin —
// die liefert `k-playbook context`.
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

	_, err := io.WriteString(out, b.String())
	return err
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
  - den Einstieg der Projektdoku als Verweis.

Gelesen wird ein Eintrag über k_playbook_doc_read bzw. k-playbook read mit
dem Verweis. Umgebungsangaben — Pfade, Werkzeuge, gh, Kephalaion-Anbindung —
stehen nicht hier, sondern in k-playbook context. Gesucht wird das Projekt ab
dem Arbeitsverzeichnis aufwärts.

Ohne --json steht der Klartext zum Lesen. Mit --json kommt die Antwort als
JSON mit schemaVersion, documents (always, onDemand, rules, reviews,
guidelines), work (available, message, user, tasks, workbench, openFindings)
und projectDocs — dieselbe Antwort wie das MCP-Werkzeug k_playbook_briefing.
`)
}
