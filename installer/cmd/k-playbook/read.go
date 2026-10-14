package main

import (
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
	const usage = "read <verweis> [--json]"

	asJSON := false
	var positional []string
	for _, arg := range args {
		switch arg {
		case "--json":
			asJSON = true
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
	result, err := project.ResolveDocRef(project.ProjectDocLayers(projectDir), positional[0])
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
  k-playbook read <verweis> [--json]

Löst einen k-playbook:-Verweis auf (docs/verweisschema.md) — lokal und
mitgeliefert schon gegeneinander aufgelöst. Gesucht wird das Projekt ab dem
Arbeitsverzeichnis aufwärts.

  k-playbook:/instructions/on-demand/<name>.md
            Eine Datei: die lokale, sonst die mitgelieferte, unverändert mit
            Frontmatter. Eine leere lokale Datei schaltet ab: kein Inhalt,
            ein Hinweis auf stderr, Exit 0.
  k-playbook:/instructions/always/
            Ein Verzeichnis (mit / am Ende): alle Teile mit Ziffer vorn aus
            beiden Ebenen, numerisch sortiert, ohne Frontmatter zu einem Text
            zusammengefügt. Dateien ohne Ziffer werden übergangen und auf
            stderr gemeldet.

Ohne --json steht auf stdout nur der Inhalt. Mit --json kommt die Antwort als
JSON: ref, directory, content, disabled, hint, parts (name, origin, disabled)
und problems. Ein unbekannter Verweis ist ein Fehler (not_found); project:-
und keph:-Verweise sowie k-playbook: außerhalb von instructions/ werden noch
nicht aufgelöst (unsupported).
`)
}
