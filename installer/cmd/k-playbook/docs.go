package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// openDocsWork und openDocsReadmes sind austauschbar, damit Tests den Node
// nicht fragen.
var (
	openDocsWork    = project.OpenDocsWork
	openDocsReadmes = project.OpenDocsReadmes
)

// runDocs holt erzeugte Doku aus der Projekt-Collection in ein
// Arbeitsverzeichnis und veröffentlicht sie wieder (Task 109): k-playbook
// docs checkout|publish <erzeuger> — dieselbe Fachlogik, die die
// MCP-Werkzeuge k_playbook_docs_checkout und k_playbook_docs_publish als
// Hülle anbieten. docs exclude add trägt dauerhafte Ausschlüsse für den
// Code-Scan ein (Task 116, runDocsExclude); docs readme schreibt die README.md
// unter keph:/docs/ aus den Vorlagen (Task 137, runDocsReadme).
//
// Ohne --json steht auf stdout eine knappe Zeile je Dokument; mit --json die
// Antwort als JSON. Ein Fehler endet mit Exit 1 und nennt seinen Code; ein
// publish, das nicht alles veröffentlicht hat, ebenso — nach der Ausgabe.
func runDocs(args []string) error {
	if len(args) == 0 {
		printDocsUsage(os.Stdout)
		return nil
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "-h", "--help", "help":
		printDocsUsage(os.Stdout)
		return nil
	case "checkout", "publish":
	case "exclude":
		return runDocsExclude(rest)
	case "readme":
		return runDocsReadme(rest)
	default:
		printDocsUsage(os.Stderr)
		return fmt.Errorf("unbekannter Unterbefehl: docs %s", sub)
	}

	usage := map[string]string{
		"checkout": "docs checkout <" + strings.Join(project.DocsProducerNames(), "|") + "> [--discard] [--json]",
		"publish":  "docs publish <" + strings.Join(project.DocsProducerNames(), "|") + "> [--allow-hints] [--json]",
	}[sub]
	var asJSON, discard, allowHints, help bool
	var positional []string
	for _, arg := range rest {
		switch {
		case arg == "--json":
			asJSON = true
		case arg == "-h" || arg == "--help":
			help = true
		case arg == "--discard" && sub == "checkout":
			discard = true
		case arg == "--allow-hints" && sub == "publish":
			allowHints = true
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unbekannte Option %s — erwartet wird %s", arg, usage)
		default:
			positional = append(positional, arg)
		}
	}
	if help {
		printDocsUsage(os.Stdout)
		return nil
	}
	if len(positional) != 1 {
		return fmt.Errorf("erwartet wird %s", usage)
	}
	if _, err := project.DocsProducerByName(positional[0]); err != nil {
		return docError(err)
	}

	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}
	work, err := openDocsWork(projectDir)
	if err != nil {
		return docError(err)
	}
	ctx := context.Background()

	if sub == "checkout" {
		result, err := work.Checkout(ctx, positional[0], project.DocsCheckoutOptions{Discard: discard})
		if err != nil {
			return docError(err)
		}
		if asJSON {
			return printDocsJSON(result)
		}
		printDocsCheckout(os.Stdout, result)
		return nil
	}

	result, err := work.Publish(ctx, positional[0], project.DocsPublishOptions{AllowHints: allowHints})
	if asJSON && (err == nil || result.Producer != "") {
		if printErr := printDocsJSON(result); printErr != nil {
			return printErr
		}
	} else if err == nil {
		printDocsPublish(os.Stdout, result)
	}
	if err != nil {
		return docError(err)
	}
	if !result.Cleaned {
		open := result.Open
		if len(open) == 0 {
			open = []string{result.Readme.Path}
		}
		return fmt.Errorf("nicht alles veröffentlicht, offen: %s — Arbeitsverzeichnis und Merkdatei bleiben", strings.Join(open, ", "))
	}
	if result.Readme.Status == project.DocsFailed {
		// Nur die Vorlage fehlt: Die Dokumente sind veröffentlicht und
		// aufgeräumt, gescheitert ist allein der README-Schritt.
		return fmt.Errorf("Dokumente veröffentlicht, aber %s nicht geschrieben: %s (%s)",
			result.Readme.Ref, result.Readme.Message, result.Readme.Code)
	}
	return nil
}

// runDocsExclude ist `k-playbook docs exclude add <muster>… [--json]`: trägt
// Muster in docs.exclude von K-PLAYBOOK.yaml ein (project.AddDocsExclude) —
// dieselbe Fachlogik wie das MCP-Werkzeug k_playbook_docs_exclude_add. Nach
// `--` zählt alles als Muster, auch was mit - beginnt. Verworfene Muster enden
// nach der Ausgabe mit Exit 1.
func runDocsExclude(args []string) error {
	const usage = "docs exclude add <muster>… [--json]"
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printDocsUsage(os.Stdout)
		return nil
	}
	if args[0] != "add" {
		printDocsUsage(os.Stderr)
		return fmt.Errorf("unbekannter Unterbefehl: docs exclude %s — erwartet wird %s", args[0], usage)
	}
	var asJSON, literal bool
	var patterns []string
	for _, arg := range args[1:] {
		switch {
		case literal:
			patterns = append(patterns, arg)
		case arg == "--":
			literal = true
		case arg == "--json":
			asJSON = true
		case arg == "-h" || arg == "--help":
			printDocsUsage(os.Stdout)
			return nil
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unbekannte Option %s — erwartet wird %s", arg, usage)
		default:
			patterns = append(patterns, arg)
		}
	}
	if len(patterns) == 0 {
		return fmt.Errorf("kein Muster angegeben — erwartet wird %s", usage)
	}

	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}
	result, err := project.AddDocsExclude(projectDir, patterns)
	if err != nil {
		return docError(err)
	}
	if asJSON {
		if err := printDocsJSON(result); err != nil {
			return err
		}
	} else {
		printDocsExclude(os.Stdout, result)
	}
	if len(result.Rejected) > 0 {
		rejected := make([]string, 0, len(result.Rejected))
		for _, problem := range result.Rejected {
			rejected = append(rejected, displayDocsPattern(problem.Pattern))
		}
		return fmt.Errorf("verworfen, nicht eingetragen: %s", strings.Join(rejected, ", "))
	}
	return nil
}

// runDocsReadme ist `k-playbook docs readme [<pfad>] [--check] [--json]`:
// schreibt die README.md unter keph:/docs/ aus den Vorlagen unter
// k-playbook:/docs/ (project.DocsReadmes) — ohne Pfad alle, mit Pfad die eine;
// dieselbe Fachlogik wie das MCP-Werkzeug k_playbook_docs_readme. --check
// schreibt nichts und meldet je Vorlage missing, differs oder unchanged. Eine
// README mit failed endet nach der Ausgabe mit Exit 1.
func runDocsReadme(args []string) error {
	const usage = "docs readme [<pfad>] [--check] [--json]"
	var asJSON, check bool
	var positional []string
	for _, arg := range args {
		switch {
		case arg == "--json":
			asJSON = true
		case arg == "--check":
			check = true
		case arg == "-h" || arg == "--help" || arg == "help":
			printDocsUsage(os.Stdout)
			return nil
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unbekannte Option %s — erwartet wird %s", arg, usage)
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) > 1 {
		return fmt.Errorf("höchstens ein Pfad — erwartet wird %s", usage)
	}
	dir := ""
	if len(positional) == 1 {
		cleaned, err := project.CleanDocsDir(positional[0])
		if err != nil {
			return docError(err)
		}
		dir = cleaned
	}

	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}
	readmes, err := openDocsReadmes(projectDir)
	if err != nil {
		return docError(err)
	}
	ctx := context.Background()
	options := project.DocsReadmeOptions{Check: check}
	var result project.DocsReadmeResult
	if dir == "" {
		result, err = readmes.All(ctx, options)
		if err != nil {
			return docError(err)
		}
	} else {
		result = project.DocsReadmeResult{Check: check, Readmes: []project.DocsReadmeItem{readmes.One(ctx, dir, options)}}
	}
	if asJSON {
		if err := printDocsJSON(result); err != nil {
			return err
		}
	} else {
		printDocsReadme(os.Stdout, result)
	}
	if failed := result.Failed(); failed > 0 {
		return fmt.Errorf("%d README nicht geschrieben", failed)
	}
	return nil
}

// printDocsReadme schreibt die Antwort von docs readme ohne --json: je README
// Zustand, Verweis und Revision, bei failed Code und Meldung.
func printDocsReadme(out io.Writer, result project.DocsReadmeResult) {
	for _, item := range result.Readmes {
		text := fmt.Sprintf("%s\t%s\t%d", item.Status, item.Ref, item.Revision)
		if item.Code != "" {
			text += fmt.Sprintf("\t%s: %s", item.Code, item.Message)
		}
		fmt.Fprintln(out, text)
	}
	if result.Hint != "" {
		fmt.Fprintf(out, "Nichts zu tun: %s.\n", result.Hint)
	}
	if result.Check {
		fmt.Fprintln(out, "Geprüft, nichts geschrieben.")
	}
}

// printDocsExclude schreibt die Antwort von docs exclude add ohne --json.
func printDocsExclude(out io.Writer, result project.DocsExcludeResult) {
	if len(result.Added) > 0 {
		fmt.Fprintf(out, "Eingetragen in %s: %s\n", project.DisplayPath(result.Config), strings.Join(result.Added, ", "))
	} else {
		fmt.Fprintf(out, "Nichts neu; %s unverändert.\n", project.DisplayPath(result.Config))
	}
	for _, problem := range result.Rejected {
		fmt.Fprintf(out, "Verworfen: %s — %s\n", displayDocsPattern(problem.Pattern), problem.Reason)
	}
	list := "—"
	if len(result.Exclude) > 0 {
		list = strings.Join(result.Exclude, ", ")
	}
	fmt.Fprintf(out, "docs.exclude: %s\n", list)
}

// displayDocsPattern zeigt ein verworfenes Muster in einer Zeile: Trägt es ein
// Steuerzeichen, etwa einen Zeilenumbruch, erscheint es in Anführungszeichen
// mit Escapes, statt die Ausgabe umzubrechen.
func displayDocsPattern(pattern string) string {
	if strings.ContainsFunc(pattern, func(r rune) bool { return unicode.IsControl(r) || r == '\u2028' || r == '\u2029' }) {
		return strconv.Quote(pattern)
	}
	return pattern
}

func printDocsJSON(result any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// printDocsCheckout schreibt die Antwort von checkout ohne --json.
func printDocsCheckout(out io.Writer, result project.DocsCheckoutResult) {
	if result.State == project.DocsStateExisting {
		existing := result.Existing
		fmt.Fprintf(out, "Arbeitsverzeichnis liegt schon da: %s\n", project.DisplayPath(result.Dir))
		fmt.Fprintf(out, "  seit %s (vor %d Minuten), %d Dateien, Merkdatei %s\n",
			existing.Since, existing.AgeMinutes, existing.Files, map[bool]string{true: "vorhanden", false: "fehlt"}[existing.Record])
		fmt.Fprintf(out, "Nichts geholt, nichts überschrieben. Weitermachen oder verwerfen: k-playbook docs checkout %s --discard\n", result.Producer)
		return
	}
	if result.Discarded {
		fmt.Fprintln(out, "Vorhandenes Arbeitsverzeichnis verworfen.")
	}
	fmt.Fprintf(out, "Geholt: %d Dokumente aus %s nach %s\n", len(result.Docs), result.Ref, project.DisplayPath(result.Dir))
	for _, doc := range result.Docs {
		fmt.Fprintf(out, "  %s\t%d\n", doc.Path, doc.Revision)
	}
}

// printDocsPublish schreibt die Antwort von publish ohne --json: je Dokument
// Zustand, Verweis und Revision, dazu Code und Meldung bei conflict und
// failed.
func printDocsPublish(out io.Writer, result project.DocsPublishResult) {
	line := func(item project.DocsPublishItem) {
		text := fmt.Sprintf("%s\t%s\t%d", item.Status, item.Ref, item.Revision)
		if item.Code != "" {
			text += fmt.Sprintf("\t%s: %s", item.Code, item.Message)
		}
		fmt.Fprintln(out, text)
	}
	for _, item := range result.Docs {
		line(item)
	}
	line(result.Readme)
	if len(result.HintsChanged) > 0 {
		fmt.Fprintf(out, "hints geändert (allow_hints): %s\n", strings.Join(result.HintsChanged, ", "))
	}
	if result.Cleaned {
		fmt.Fprintf(out, "Veröffentlicht nach %s; Arbeitsverzeichnis und Merkdatei entfernt.\n", result.Ref)
		return
	}
	fmt.Fprintf(out, "Offen: %s — Arbeitsverzeichnis %s und Merkdatei bleiben; ein zweites publish nach der Klärung schreibt nur den Rest.\n",
		strings.Join(result.Open, ", "), project.DisplayPath(result.Dir))
}

func printDocsUsage(out io.Writer) {
	fmt.Fprintf(out, `k-playbook docs — erzeugte Doku auschecken und veröffentlichen

Aufruf:
  k-playbook docs checkout <erzeuger> [--discard] [--json]
  k-playbook docs publish <erzeuger> [--allow-hints] [--json]
  k-playbook docs readme [<pfad>] [--check] [--json]
  k-playbook docs exclude add <muster>… [--json]

Erzeuger: %s. Jeder hat ein Verzeichnis keph:/docs/generated/<erzeuger>/ in
der Projekt-Collection.

  checkout  Holt alle .md des Verzeichnisses außer README.md, rekursiv, nach
            .k-playbook/work/docs/<erzeuger>/ und schreibt daneben die
            Merkdatei <erzeuger>.checkout.json (Revision, Inhalts-Hash und
            hints je Dokument). /work kommt in .k-playbook/.gitignore. Liegt
            Arbeitsverzeichnis oder Merkdatei schon da, wird nichts geholt und
            nichts überschrieben: die Ausgabe nennt Alter und Anzahl Dateien.
            --discard verwirft beides und holt neu.
  publish   Gleicht das Arbeitsverzeichnis an der Merkdatei ab: neu → anlegen,
            geändert → schreiben mit der Revision beim checkout, gelöscht →
            löschen mit dieser Revision, unverändert → nichts. Vorher wird
            geprüft: nur .md, keine README.md, je Dokument Frontmatter mit
            title und description; hints unverändert — sonst, und wenn ein
            Dokument mit hints gelöscht oder neu angelegt wird, nur mit
            --allow-hints (nach Bestätigung durch den Nutzer). Eine Ablehnung
            schreibt nichts. Was inzwischen in Kephalaion geändert ist, wird
            als conflict gemeldet, nicht geschrieben. Danach schreibt publish
            die README.md des Verzeichnisses aus ihrer Vorlage, wenn sie
            fehlt oder abweicht (wie docs readme). Arbeitsverzeichnis und
            Merkdatei werden nur entfernt, wenn nichts in Konflikt oder
            fehlgeschlagen ist; ein zweites publish schreibt dann nur noch
            den Rest. Fehlt nur die Vorlage der README (template_missing),
            wird trotzdem aufgeräumt; der Aufruf endet mit Exit 1.

  readme    Schreibt die README.md unter keph:/docs/ aus den Vorlagen
            k-playbook:/docs/<pfad>/README.md, die mit dem Hub ausgeliefert
            werden: je Verzeichnis mit Vorlage die README, die fehlt oder
            inhaltlich abweicht — anlegen, sonst schreiben mit ihrer Revision,
            sonst nichts. Eine von Hand geänderte README wird überschrieben.
            Ohne <pfad> alle Vorlagen, mit <pfad> die eine: relativ zu docs/
            (generated/code) oder als Verweis (keph:/docs/generated/code/).
            Verzeichnisse ohne Vorlage und docs/README.md ganz oben fasst es
            nicht an. --check schreibt nichts und meldet je Vorlage missing,
            differs oder unchanged.

  exclude add
            Trägt Muster in den Block docs.exclude von .k-playbook/K-PLAYBOOK.yaml
            ein und legt ihn an, wenn er fehlt; außerhalb des Blocks bleibt die
            Datei unverändert. Die Muster gelten wie in einer .gitignore,
            relativ zu project.repo_root. Ein absolutes Muster, eines, das
            mit .. aus project.repo_root führt, oder eines mit Zeilenumbruch
            oder anderem Steuerzeichen wird nicht eingetragen, sondern
            mit Grund gemeldet (Exit 1 nach der Ausgabe); eines, das schon
            dasteht, wird übersprungen. Ist nichts neu, bleibt die Datei
            unberührt. Die Docs-Erzeuger lesen die Liste aus context.docs.exclude.
            --json: config, exclude (die Liste danach), added, rejected
            (pattern, reason) und changed. Nach -- zählt alles als Muster.

Ausgabe von publish: je Dokument Zustand (created, written, deleted,
unchanged, conflict, failed), Verweis und Revision, durch Tabulatoren
getrennt. Bleibt etwas offen, endet der Aufruf mit Exit 1. docs readme
gibt je README dieselbe Zeile aus (created, written, unchanged, failed; mit
--check missing, differs, unchanged); --json: readmes (dir, ref, template,
status, revision, base, code, message), check, hint. Eine README mit failed
endet mit Exit 1.

Gesucht wird das Projekt ab dem Arbeitsverzeichnis aufwärts; gebraucht wird
eine bereite Kephalaion-Anbindung (k-playbook context, kephalaion.ok) und für
publish und readme ein Node ab v0.3.1; exclude add braucht keine. Dasselbe
wie die MCP-Werkzeuge k_playbook_docs_checkout, k_playbook_docs_publish,
k_playbook_docs_readme und k_playbook_docs_exclude_add.

Fehlercodes stehen in Klammern am Ende der Meldung. kephalaion_unavailable:
beim Hub ist nichts angekommen, nichts geschrieben. invalid_input: unbekannter
Erzeuger, Ablehnung im Arbeitsverzeichnis oder exclude add ohne Muster. hints_changed: hints geändert
ohne --allow-hints. not_checked_out: publish ohne checkout. template_missing:
die Vorlage einer README fehlt auf dem Hub (Lieferumfang älter als das
Programm); nichts geschrieben. Bei einem Dokument mit outcome_unknown nicht
noch einmal veröffentlichen, sondern nachsehen.
`, strings.Join(project.DocsProducerNames(), ", "))
}
