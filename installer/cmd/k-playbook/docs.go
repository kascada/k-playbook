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

// openDocsWork ist austauschbar, damit Tests den Node nicht fragen.
var openDocsWork = project.OpenDocsWork

// runDocs holt erzeugte Doku aus der Projekt-Collection in ein
// Arbeitsverzeichnis und veröffentlicht sie wieder (Task 109): k-playbook
// docs checkout|publish <erzeuger> — dieselbe Fachlogik, die die
// MCP-Werkzeuge k_playbook_docs_checkout und k_playbook_docs_publish als
// Hülle anbieten.
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
	return nil
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
            die README.md des Verzeichnisses, wenn sie abweicht.
            Arbeitsverzeichnis und Merkdatei werden nur entfernt, wenn nichts
            in Konflikt oder fehlgeschlagen ist; ein zweites publish schreibt
            dann nur noch den Rest.

Ausgabe von publish: je Dokument Zustand (created, written, deleted,
unchanged, conflict, failed), Verweis und Revision, durch Tabulatoren
getrennt. Bleibt etwas offen, endet der Aufruf mit Exit 1.

Gesucht wird das Projekt ab dem Arbeitsverzeichnis aufwärts; gebraucht wird
eine bereite Kephalaion-Anbindung (k-playbook context, kephalaion.ok) und für
publish ein Node ab v0.3.1. Dasselbe wie die MCP-Werkzeuge
k_playbook_docs_checkout und k_playbook_docs_publish.

Fehlercodes stehen in Klammern am Ende der Meldung. kephalaion_unavailable:
beim Hub ist nichts angekommen, nichts geschrieben. invalid_input: unbekannter
Erzeuger oder Ablehnung im Arbeitsverzeichnis. hints_changed: hints geändert
ohne --allow-hints. not_checked_out: publish ohne checkout. Bei einem Dokument
mit outcome_unknown nicht noch einmal veröffentlichen, sondern nachsehen.
`, strings.Join(project.DocsProducerNames(), ", "))
}
