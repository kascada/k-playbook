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

// openDocs ist austauschbar, damit Tests den Node nicht fragen.
var openDocs = project.OpenUserDocs

// runDoc legt nummerierte Dokumente in der Kephalaion-Collection des
// Projekts an, listet sie und verschiebt sie nach done/ und zurück —
// dieselbe Fachlogik, die die MCP-Werkzeuge k_playbook_doc_create, _list,
// _done und _reopen als Hülle anbieten.
//
// Ohne --json steht auf stdout eine knappe Zeile je Ergebnis; mit --json die
// Antwort als JSON. Ein Fehler endet mit Exit 1 und nennt seinen Code.
func runDoc(args []string) error {
	if len(args) == 0 {
		printDocUsage(os.Stdout)
		return nil
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "-h", "--help", "help":
		printDocUsage(os.Stdout)
		return nil
	case "create", "list", "done", "reopen":
	default:
		printDocUsage(os.Stderr)
		return fmt.Errorf("unbekannter Unterbefehl: doc %s", sub)
	}

	opts, positional, err := parseDocArgs(sub, rest)
	if err != nil {
		return err
	}
	if opts.help {
		printDocUsage(os.Stdout)
		return nil
	}

	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}
	docs, err := openDocs(projectDir)
	if err != nil {
		return docError(err)
	}
	ctx := context.Background()

	var result any
	var line func()
	switch sub {
	case "create":
		created, err := docs.Create(ctx, positional[0], positional[1])
		if err != nil {
			return docError(err)
		}
		result, line = created, func() { fmt.Println(created.Ref) }
	case "list":
		list, err := docs.List(ctx, project.DocListOptions{Kind: positional[0], User: opts.user, Done: opts.done})
		if err != nil {
			return docError(err)
		}
		if list.Hint != "" {
			fmt.Fprintf(os.Stderr, "Hinweis: %s\n", list.Hint)
		}
		result, line = list, func() {
			for _, item := range list.Items {
				fields := []string{item.Ref, item.Title, item.User, item.Updated}
				if item.Done {
					fields = append(fields, "done")
				}
				fmt.Println(strings.Join(fields, "\t"))
			}
		}
	case "done", "reopen":
		move := docs.Done
		if sub == "reopen" {
			move = docs.Reopen
		}
		moved, err := move(ctx, positional[0])
		if err != nil {
			return docError(err)
		}
		result, line = moved, func() { fmt.Println(moved.Ref) }
	}

	if opts.json {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	line()
	return nil
}

type docOptions struct {
	json bool
	done bool
	help bool
	user string
}

// parseDocArgs liest Schalter und Positionsargumente eines Unterbefehls.
func parseDocArgs(sub string, args []string) (docOptions, []string, error) {
	kinds := strings.Join(project.DocKindNames(project.DocKinds), "|")
	usage := map[string]string{
		"create": "doc create <" + kinds + "> <kurzname> [--json]",
		"list":   "doc list <" + kinds + "> [--user <name>|*] [--done] [--json]",
		"done":   "doc done <keph:-verweis> [--json]",
		"reopen": "doc reopen <keph:-verweis> [--json]",
	}[sub]
	want := map[string]int{"create": 2, "list": 1, "done": 1, "reopen": 1}[sub]

	var opts docOptions
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			opts.json = true
		case arg == "-h" || arg == "--help":
			opts.help = true
		case arg == "--done" && sub == "list":
			opts.done = true
		case arg == "--user" && sub == "list":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--user erwartet einen Wert — erwartet wird %s", usage)
			}
			i++
			opts.user = args[i]
		case strings.HasPrefix(arg, "--user=") && sub == "list":
			opts.user = strings.TrimPrefix(arg, "--user=")
		case strings.HasPrefix(arg, "-"):
			return opts, nil, fmt.Errorf("unbekannte Option %s — erwartet wird %s", arg, usage)
		default:
			positional = append(positional, arg)
		}
	}
	if opts.help {
		return opts, nil, nil
	}
	if len(positional) != want {
		return opts, nil, fmt.Errorf("erwartet wird %s", usage)
	}
	return opts, positional, nil
}

// docError hängt den Code an die Meldung, wie `k-playbook read`.
func docError(err error) error {
	if code := project.DocRefErrorCode(err); code != "" {
		return fmt.Errorf("%w (%s)", err, code)
	}
	return err
}

func printDocUsage(out io.Writer) {
	kinds := strings.Join(project.DocKindNames(project.DocKinds), ", ")
	fmt.Fprintf(out, `k-playbook doc — nummerierte Dokumente in der Kephalaion-Collection

Aufruf:
  k-playbook doc create <sorte> <kurzname> [--json]
  k-playbook doc list <sorte> [--user <name>|*] [--done] [--json]
  k-playbook doc done <keph:-verweis> [--json]
  k-playbook doc reopen <keph:-verweis> [--json]

Sorten: %s.

  create  Legt ein leeres Dokument <nummer>-<kurzname>.md in der Ablage der
          Sorte an, über den Hub. Die Nummer ist die höchste im Zählbereich
          plus eins, mindestens die Startnummer der Sorte. Ausgabe: der
          keph:-Verweis mit echtem Usernamen.
  list    Listet die Dokumente einer Sorte aus der Replica: ohne --user die
          eigenen, mit --user '*' alle. --done nimmt die Erledigten mit auf.
          Je Zeile Verweis, Titel, User und Änderungszeit, getrennt durch
          Tabulatoren. Bei Sorten ohne Userbereich greift --user nicht.
  done    Verschiebt ein eigenes Dokument nach done/; ~ im Verweis steht für
          den eigenen User. Ausgabe: der neue Verweis.
  reopen  Holt es aus done/ zurück.

Gesucht wird das Projekt ab dem Arbeitsverzeichnis aufwärts; gebraucht wird
eine bereite Kephalaion-Anbindung (k-playbook context, kephalaion.ok) und für
create, done und reopen ein Node ab v0.3.1. Den Inhalt liest und schreibt man
direkt über Kephalaion. Dasselbe wie die MCP-Werkzeuge k_playbook_doc_create,
_list, _done und _reopen.

Fehlercodes stehen in Klammern am Ende der Meldung. kephalaion_unavailable:
beim Hub ist nichts angekommen. outcome_unknown: create, done oder reopen kann
beim Hub angekommen sein, auch nach der Wiederholung mit demselben Schlüssel —
nicht noch einmal aufrufen, sondern mit doc list nachsehen.
`, kinds)
}
