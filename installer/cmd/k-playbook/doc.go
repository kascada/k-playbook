package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// openDocs ist austauschbar, damit Tests den Node nicht fragen.
var openDocs = project.OpenUserDocs

// docLayers ist austauschbar, damit Tests keph:-Verweise ohne Node lesen —
// für doc grep mit einem Verweis.
var docLayers = project.ProjectDocLayers

// docStdin ist die Eingabe von doc append und doc replace; Tests setzen sie.
var docStdin io.Reader = os.Stdin

// runDoc legt nummerierte Dokumente in der Kephalaion-Collection des
// Projekts an, listet sie, verschiebt sie nach done/ und zurück, ändert sie
// (append, replace, set) und durchsucht sie (grep) — dieselbe Fachlogik, die
// die MCP-Werkzeuge k_playbook_doc_create, _list, _done, _reopen, _append,
// _replace, _set_field und _grep als Hülle anbieten.
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
	case "create", "list", "done", "reopen", "append", "replace", "set", "grep":
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
	ctx := context.Background()
	if sub == "grep" && opts.kind == "" {
		// Ein Verweis braucht keinen Zugang zu den nummerierten Dokumenten:
		// project: und k-playbook: gehen ohne Kephalaion.
		result, err := project.GrepDocRef(ctx, docLayers(projectDir), positional[0],
			project.DocGrepOptions{Pattern: positional[1], CountOnly: opts.count})
		if err != nil {
			return docError(err)
		}
		return printDocGrep(result, opts)
	}
	var text string
	var old, new string
	switch sub {
	case "append":
		if text, err = readDocStdin(); err != nil {
			return err
		}
	case "replace":
		if old, new, err = readDocReplaceStdin(); err != nil {
			return err
		}
	}
	docs, err := openDocs(projectDir)
	if err != nil {
		return docError(err)
	}

	var result any
	var line func()
	edited := func(edited project.DocEdited, err error) error {
		if err != nil {
			return docEditError(err, opts.json)
		}
		result, line = edited, func() { fmt.Println(docEditedLine(edited)) }
		return nil
	}
	editOpts := project.DocEditOptions{SinceRevision: opts.since}
	switch sub {
	case "append":
		if err := edited(docs.Append(ctx, positional[0], text, editOpts)); err != nil {
			return err
		}
	case "replace":
		if err := edited(docs.Replace(ctx, positional[0], old, new, editOpts)); err != nil {
			return err
		}
	case "set":
		var value *string
		key := opts.unset
		if key == "" {
			key, value = positional[1], &positional[2]
		}
		if err := edited(docs.SetField(ctx, positional[0], key, value, editOpts)); err != nil {
			return err
		}
	case "grep":
		grep, err := docs.Grep(ctx, project.DocListOptions{Kind: opts.kind, User: opts.user, Done: opts.done},
			project.DocGrepOptions{Pattern: positional[0], CountOnly: opts.count})
		if err != nil {
			return docError(err)
		}
		return printDocGrep(grep, opts)
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
				// Das Frontmatter-Problem als letztes Feld, nur wenn es eines gibt.
				if item.Problem != "" {
					fields = append(fields, item.Problem)
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
	json  bool
	done  bool
	help  bool
	count bool
	user  string
	kind  string
	unset string
	since int64
}

// parseDocArgs liest Schalter und Positionsargumente eines Unterbefehls.
func parseDocArgs(sub string, args []string) (docOptions, []string, error) {
	kinds := strings.Join(project.DocKindNames(project.DocKinds), "|")
	usage := map[string]string{
		"create":  "doc create <" + kinds + "> <kurzname> [--json]",
		"list":    "doc list <" + kinds + "> [--user <name>|*] [--done] [--json]",
		"done":    "doc done <keph:-verweis> [--json]",
		"reopen":  "doc reopen <keph:-verweis> [--json]",
		"append":  "doc append <keph:-verweis> [--since <revision>] [--json] (Text von stdin)",
		"replace": "doc replace <keph:-verweis> [--since <revision>] [--json] ({\"old\": …, \"new\": …} von stdin)",
		"set":     "doc set <keph:-verweis> <feld> <wert> [--since <revision>] [--json] oder doc set <keph:-verweis> --unset <feld> [--since <revision>] [--json]",
		"grep":    "doc grep (<verweis> | --kind <" + kinds + "> [--user <name>|*] [--done]) <muster> [--count] [--json]",
	}[sub]
	want := map[string]int{"create": 2, "list": 1, "done": 1, "reopen": 1, "append": 1, "replace": 1, "set": 3, "grep": 2}[sub]

	var opts docOptions
	var positional []string
	value := func(i *int, name string) (string, error) {
		arg := args[*i]
		if v, ok := strings.CutPrefix(arg, name+"="); ok {
			return v, nil
		}
		if *i+1 >= len(args) {
			return "", fmt.Errorf("%s erwartet einen Wert — erwartet wird %s", name, usage)
		}
		*i++
		return args[*i], nil
	}
	is := func(arg, name string) bool { return arg == name || strings.HasPrefix(arg, name+"=") }
	edit := sub == "append" || sub == "replace" || sub == "set"
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			opts.json = true
		case arg == "-h" || arg == "--help":
			opts.help = true
		case arg == "--done" && (sub == "list" || sub == "grep"):
			opts.done = true
		case arg == "--count" && sub == "grep":
			opts.count = true
		case is(arg, "--user") && (sub == "list" || sub == "grep"):
			v, err := value(&i, "--user")
			if err != nil {
				return opts, nil, err
			}
			opts.user = v
		case is(arg, "--kind") && sub == "grep":
			v, err := value(&i, "--kind")
			if err != nil {
				return opts, nil, err
			}
			opts.kind = v
		case is(arg, "--unset") && sub == "set":
			v, err := value(&i, "--unset")
			if err != nil {
				return opts, nil, err
			}
			opts.unset = v
		case is(arg, "--since") && edit:
			v, err := value(&i, "--since")
			if err != nil {
				return opts, nil, err
			}
			since, err := strconv.ParseInt(v, 10, 64)
			if err != nil || since < 1 {
				return opts, nil, fmt.Errorf("--since erwartet eine Revision (ganze Zahl ab 1), nicht %q — erwartet wird %s", v, usage)
			}
			opts.since = since
		case arg == "--" && (sub == "set" || sub == "grep"):
			// Alles danach ist positional, auch was mit - beginnt (ein Wert,
			// ein Muster).
			positional = append(positional, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(arg, "-") && arg != "-":
			return opts, nil, fmt.Errorf("unbekannte Option %s — erwartet wird %s", arg, usage)
		default:
			positional = append(positional, arg)
		}
	}
	if opts.help {
		return opts, nil, nil
	}
	switch {
	case sub == "set" && opts.unset != "":
		want = 1
	case sub == "grep" && opts.kind != "":
		want = 1
	case sub == "grep" && (opts.user != "" || opts.done):
		return opts, nil, fmt.Errorf("--user und --done gelten nur mit --kind — erwartet wird %s", usage)
	}
	if len(positional) != want {
		return opts, nil, fmt.Errorf("erwartet wird %s", usage)
	}
	return opts, positional, nil
}

// readDocStdin liest den Text von doc append.
func readDocStdin() (string, error) {
	data, err := io.ReadAll(docStdin)
	if err != nil {
		return "", fmt.Errorf("stdin lesen: %w", err)
	}
	return string(data), nil
}

// readDocReplaceStdin liest {"old": …, "new": …} von stdin.
func readDocReplaceStdin() (string, string, error) {
	data, err := io.ReadAll(docStdin)
	if err != nil {
		return "", "", fmt.Errorf("stdin lesen: %w", err)
	}
	var input struct {
		Old *string `json:"old"`
		New *string `json:"new"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || input.Old == nil {
		return "", "", fmt.Errorf("doc replace erwartet auf stdin JSON {\"old\": \"…\", \"new\": \"…\"} (invalid_input)")
	}
	if input.New == nil {
		empty := ""
		input.New = &empty
	}
	return *input.Old, *input.New, nil
}

// docEditedLine ist die Zeile einer Änderung ohne --json: Verweis und
// Revision, durch einen Tabulator getrennt, und „unverändert“, wenn nichts
// geschrieben wurde.
func docEditedLine(edited project.DocEdited) string {
	line := fmt.Sprintf("%s\t%d", edited.Ref, edited.Revision)
	if !edited.Changed {
		line += "\tunverändert"
	}
	return line
}

// docEditError meldet einen Fehler einer Änderung. Ein foreign_change bringt
// den Stand des Dokuments mit: mit --json als JSON auf stdout, sonst die
// letzten Zeilen auf stderr.
func docEditError(err error, asJSON bool) error {
	if change, ok := project.DocForeignChangeOf(err); ok {
		if asJSON {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			_ = encoder.Encode(change)
		} else if change.Tail != "" {
			fmt.Fprintf(os.Stderr, "Die letzten Zeilen von %s (Revision %d):\n%s", change.Ref, change.Revision, change.Tail)
			if !strings.HasSuffix(change.Tail, "\n") {
				fmt.Fprintln(os.Stderr)
			}
		}
	}
	return docError(err)
}

// printDocGrep gibt das Ergebnis von doc grep aus: ohne --json je Treffer
// ref:zeile:text, mit --count je Dokument ref:anzahl, bei keph: mit
// :revision dahinter.
func printDocGrep(result project.DocGrepResult, opts docOptions) error {
	if result.Hint != "" {
		fmt.Fprintf(os.Stderr, "Hinweis: %s\n", result.Hint)
	}
	if result.Truncated {
		fmt.Fprintf(os.Stderr, "Hinweis: %d Treffer, gezeigt die ersten %d\n", result.Count, len(result.Matches))
	}
	if opts.json {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	for _, c := range result.Counts {
		if c.Revision > 0 {
			fmt.Printf("%s:%d:%d\n", c.Ref, c.Count, c.Revision)
		} else {
			fmt.Printf("%s:%d\n", c.Ref, c.Count)
		}
	}
	for _, m := range result.Matches {
		fmt.Printf("%s:%d:%s\n", m.Ref, m.Line, m.Text)
	}
	return nil
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
	areas := "keph:/" + strings.Join(project.DocEditAreaDirs(project.DocEditAreas), ", keph:/")
	fmt.Fprintf(out, `k-playbook doc — nummerierte Dokumente in der Kephalaion-Collection

Aufruf:
  k-playbook doc create <sorte> <kurzname> [--json]
  k-playbook doc list <sorte> [--user <name>|*] [--done] [--json]
  k-playbook doc done <keph:-verweis> [--json]
  k-playbook doc reopen <keph:-verweis> [--json]
  k-playbook doc append <keph:-verweis> [--since <revision>] [--json]   (Text von stdin)
  k-playbook doc replace <keph:-verweis> [--since <revision>] [--json]  ({"old": …, "new": …} von stdin)
  k-playbook doc set <keph:-verweis> <feld> <wert> [--since <revision>] [--json]
  k-playbook doc set <keph:-verweis> --unset <feld> [--since <revision>] [--json]
  k-playbook doc grep <verweis> <muster> [--count] [--json]
  k-playbook doc grep --kind <sorte> [--user <name>|*] [--done] <muster> [--count] [--json]

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
          den eigenen User. Ausgabe: der neue Verweis; mit --json dazu
          revision, die Revision nach dem Verschieben.
  reopen  Holt es aus done/ zurück.

Ändern, ohne den Inhalt zu lesen — für ein Dokument der Sorten, offen oder
unter done/, bei Sorten mit Userbereich ein eigenes; append und replace auch
für die Handdoku unter %s, auch in Unterverzeichnissen — sie
gehört allen —, set nicht:

  append  Hängt den Text von stdin an: Endet das Dokument nicht mit einem
          Zeilenumbruch, kommt einer dazu, dann eine Leerzeile, dann der Text
          mit genau einem Zeilenumbruch. Für Text mit Backticks und $(…):
            k-playbook doc append <verweis> <<'EOF'
            …
            EOF
  replace Ersetzt "old", das genau einmal vorkommen muss, durch "new" (leer
          löscht); beides als JSON von stdin. Kein Treffer: text_not_found,
          mehrere: text_not_unique. Der Weg, eine Stelle einer Handbuchseite
          nachzuziehen, statt sie ganz neu zu schreiben.
  set     Setzt ein Feld im Frontmatter (legt eines an, wenn es fehlt) oder
          entfernt es mit --unset. Andere Felder und der Rumpf bleiben Byte
          für Byte. Ein Wert, der mit - beginnt, steht nach --.

  Ausgabe: Verweis und Revision, durch einen Tabulator getrennt, dahinter
  „unverändert“, wenn nichts geschrieben wurde. --since <revision> ist die
  Revision, die man zuletzt gesehen hat (aus der letzten Ausgabe, aus read
  --json oder grep --count): Weicht das Dokument ab, hat ein anderer
  geschrieben — foreign_change, nichts geschrieben, die letzten Zeilen auf
  stderr (mit --json der Stand als JSON auf stdout). Dann nicht still
  nachziehen, sondern klären, wer weitermacht. Ohne --since wird nach
  stale_revision neu gelesen und bis zu dreimal wiederholt.

  grep    Findet Zeilen nach einem regulären Ausdruck (RE2) in einer
          Einzeldatei hinter einem Verweis (project:, k-playbook:, keph: —
          jeder keph:-Verweis, auch die Handdoku) oder mit --kind in allen Dokumenten einer Sorte, ausgewählt wie bei
          list. Gesucht wird im gespeicherten Text, ohne eingesetzte
          Includes. Ausgabe je Treffer ref:zeile:text, höchstens 200; mit
          --count je Dokument ref:anzahl, bei keph: ref:anzahl:revision. Ein
          Muster, das mit - beginnt, steht nach --.

Gesucht wird das Projekt ab dem Arbeitsverzeichnis aufwärts; gebraucht wird
eine bereite Kephalaion-Anbindung (k-playbook context, kephalaion.ok), für
create, done, reopen, append, replace und set ein Node ab v0.3.1. Dasselbe
wie die MCP-Werkzeuge k_playbook_doc_create, _list, _done, _reopen, _append,
_replace, _set_field und _grep.

Fehlercodes stehen in Klammern am Ende der Meldung. kephalaion_unavailable:
beim Hub ist nichts angekommen. outcome_unknown: der Vorgang kann beim Hub
angekommen sein, auch nach der Wiederholung mit demselben Schlüssel — nicht
noch einmal aufrufen, sondern nachsehen: bei create, done und reopen mit doc
list, bei append, replace und set mit read oder grep --count.
`, kinds, areas)
}
