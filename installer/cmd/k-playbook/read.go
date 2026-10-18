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
	const usage = "read <verweis> [--raw] [--json]"

	asJSON := false
	raw := false
	var positional []string
	for _, arg := range args {
		switch arg {
		case "--json":
			asJSON = true
		case "--raw":
			raw = true
		case "-h", "--help", "help":
			printReadUsage(os.Stdout)
			return nil
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
		project.DocReadOptions{Raw: raw})
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
  k-playbook read <verweis> [--raw] [--json]

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
  k-playbook:/rules/<name>.md
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
            andere mit / am Ende — on-demand/, instructions/ selbst, rules/,
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

Ohne --json steht auf stdout nur der Inhalt. Mit --json kommt die Antwort als
JSON: ref, directory, content, disabled, hint, parts (name, origin, disabled),
problems und includes (ref, origin, depth, disabled — die eingesetzten Ziele
in Lesereihenfolge). Ein unbekannter Verweis ist ein Fehler (not_found);
k-playbook: außerhalb von instructions/, rules/, reviews/, guidelines/ und
commands/ wird nicht aufgelöst (unsupported).
`)
}
