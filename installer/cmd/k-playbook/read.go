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

// runRead löst einen Verweis im Präfixschema auf und gibt aus, was dahinter
// steht — dieselbe Fachlogik, die das MCP-Werkzeug k_playbook_doc_read als
// Hülle anbietet. Das Subkommando dient zum Testen und als Ausweg, wenn der
// MCP-Server nicht registriert ist.
//
// Ohne --json steht auf stdout nur der Inhalt; Hinweise — eine abgeschaltete
// Datei, übergangene Teile — gehen nach stderr, damit der Inhalt für sich
// weiterverarbeitet werden kann. Beides ist kein Fehler: Exit 0.
func runRead(args []string) error {
	const usage = "read <verweis> [--raw] [--lines <von-bis>|--section <überschrift>] [--json]"

	asJSON := false
	raw := false
	var lines, section string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			asJSON = true
		case arg == "--raw":
			raw = true
		case arg == "-h" || arg == "--help" || arg == "help":
			printReadUsage(os.Stdout)
			return nil
		case arg == "--lines" || arg == "--section":
			if i+1 >= len(args) {
				return fmt.Errorf("%s erwartet einen Wert — erwartet wird %s", arg, usage)
			}
			i++
			if arg == "--lines" {
				lines = args[i]
			} else {
				section = args[i]
			}
		case strings.HasPrefix(arg, "--lines="):
			lines = strings.TrimPrefix(arg, "--lines=")
		case strings.HasPrefix(arg, "--section="):
			section = strings.TrimPrefix(arg, "--section=")
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unbekannte Option %s — erwartet wird %s", arg, usage)
			}
			positional = append(positional, arg)
		}
	}
	if len(args) == 0 {
		printReadUsage(os.Stdout)
		return nil
	}
	if len(positional) != 1 {
		return fmt.Errorf("erwartet wird %s", usage)
	}

	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}
	result, err := project.ResolveDocRefWith(context.Background(), project.ProjectDocLayers(projectDir), positional[0],
		project.DocReadOptions{Raw: raw, Lines: lines, Section: section})
	if err != nil {
		if code := project.DocRefErrorCode(err); code != "" {
			return fmt.Errorf("%w (%s)", err, code)
		}
		return err
	}

	if result.Hint != "" {
		fmt.Fprintf(os.Stderr, "Hinweis: %s\n", result.Hint)
	}
	for _, problem := range result.Problems {
		fmt.Fprintf(os.Stderr, "Hinweis: %s\n", problem)
	}

	if asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	if result.Content == "" {
		return nil
	}
	fmt.Print(result.Content)
	if !strings.HasSuffix(result.Content, "\n") {
		fmt.Println()
	}
	return nil
}

func printReadUsage(out io.Writer) {
	fmt.Fprint(out, `k-playbook read — einen Verweis auflösen

Aufruf:
  k-playbook read <verweis> [--raw] [--lines <von-bis>|--section <überschrift>] [--json]

Löst einen Verweis im Präfixschema auf (docs/verweisschema.md) — dieselbe
Fachlogik wie das MCP-Werkzeug k_playbook_doc_read. Gesucht wird das Projekt
ab dem Arbeitsverzeichnis aufwärts.

  project:/<pfad>
            Eine Datei des Projekts, Pfad ab dem Verzeichnis von
            K-PLAYBOOK.yaml. Eine Ebene, keine Überlagerung; eine leere Datei
            ist leerer Inhalt.
  keph:/<pfad>
            Ein Dokument der Projekt-Collection, gelesen über den Kephalaion-
            Node dieses Rechners (Replica); ~ als ganzer Pfadteil steht für
            den eigenen User. Ist die Anbindung nicht bereit, ist das ein
            Fehler (kephalaion_unavailable), kein Rückfall.
  k-playbook:/instructions/on-demand/<name>.md
  k-playbook:/reviews/review-<name>.md
  k-playbook:/guidelines/<name>.md
  k-playbook:/commands/<name>.md
            Eine Datei: die lokale, sonst die mitgelieferte, unverändert mit
            Frontmatter — dieselbe Überlagerung wie catalogs in k-playbook
            context. Eine leere lokale Datei schaltet ab: kein Inhalt, ein
            Hinweis auf stderr, Exit 0.
  k-playbook:/instructions/always/
            Ein Verzeichnis (mit / am Ende): alle Teile mit Ziffer vorn aus
            beiden Ebenen, numerisch sortiert, ohne Frontmatter zu einem Text
            zusammengefügt. Dateien ohne Ziffer werden übergangen und auf
            stderr gemeldet. Zusammengebaut wird nur dieses Verzeichnis; jedes
            andere mit / am Ende — on-demand/, instructions/ selbst, reviews/,
            eine Datei mit /, auch bei project: und keph: — ist ein Fehler:
            der Dateiname fehlt (invalid_input).

Includes: Eine Zeile, die ohne führenden Leerraum mit „include:“ beginnt,
gefolgt von genau einem Verweis (mit Präfix oder relativ zum Verzeichnis des
Dokuments im selben Präfix), wird durch den Rumpf des Ziels ohne Frontmatter
ersetzt — rekursiv bis Tiefe 10, in jedem Dokument, in Code-Blöcken nicht.
Ein abgeschaltetes Ziel setzt nichts ein. Ein Zyklus (include_cycle), eine
zu tiefe Kette (include_too_deep) und ein Ziel, das sich nicht lesen lässt,
sind Fehler mit der Kette der Dokumente; bei einem Verzeichnis fällt nur der
betroffene Teil aus und steht auf stderr. Mit --raw bleibt der Text, wie er
ist: Include-Zeilen stehen lesbar da.

Teile einer Einzeldatei, nach dem Einsetzen der Includes; beide schließen
sich aus und gelten nicht für ein Verzeichnis (invalid_input):
  --lines <von-bis>
            Nur diese Zeilen, 1-basiert und einschließlich, etwa 10-20;
            10- bis zum Ende.
  --section <überschrift>
            Nur den Abschnitt ab der ersten Zeile, die genau so lautet, etwa
            "## Intent", bis vor die nächste Überschrift gleicher oder höherer
            Ebene; Überschriften in Code-Blöcken zählen nicht. Kein Treffer:
            section_not_found.

Ohne --json steht auf stdout nur der Inhalt. Mit --json kommt die Antwort als
JSON: ref, directory, content, disabled, hint, parts (name, origin, disabled),
problems, includes (ref, origin, depth, disabled — die eingesetzten Ziele
in Lesereihenfolge), totalLines (Zeilen des ganzen Inhalts) und bei keph:
revision. Ein unbekannter Verweis ist ein Fehler (not_found);
k-playbook: außerhalb von instructions/, reviews/, guidelines/ und commands/
wird nicht aufgelöst (unsupported).
`)
}
