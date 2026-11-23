package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// openDocsOrders ist austauschbar, damit Tests den Node nicht fragen.
var openDocsOrders = project.OpenDocsOrders

// runDocsOrders ist `k-playbook docs orders <code|tools> [--json]` (Task 179):
// liest die offenen Doku-Aufträge eines Erzeugers und bewertet sie
// (project.DocsOrders.Orders) — dieselbe Fachlogik wie das MCP-Werkzeug
// k_playbook_docs_orders. `docs orders split` ist ein eigener Unterbefehl.
func runDocsOrders(args []string) error {
	if len(args) > 0 && args[0] == "split" {
		return runDocsOrdersSplit(args[1:])
	}
	usage := "docs orders <" + strings.Join(project.DocsOrderProducers, "|") + "> [--json]"
	var asJSON bool
	var positional []string
	for _, arg := range args {
		switch {
		case arg == "--json":
			asJSON = true
		case arg == "-h" || arg == "--help" || arg == "help":
			printDocsUsage(os.Stdout)
			return nil
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unbekannte Option %s — erwartet wird %s", arg, usage)
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) != 1 {
		return fmt.Errorf("erwartet wird %s", usage)
	}
	if err := project.ValidDocsOrderProducer(positional[0]); err != nil {
		return docError(err)
	}
	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}
	orders, err := openDocsOrders(projectDir)
	if err != nil {
		return docError(err)
	}
	result, err := orders.Orders(context.Background(), positional[0])
	if err != nil {
		return docError(err)
	}
	if asJSON {
		return printDocsJSON(result)
	}
	printDocsOrders(os.Stdout, result)
	return nil
}

// printDocsOrders schreibt die Antwort von docs orders ohne --json.
func printDocsOrders(out io.Writer, result project.DocsOrdersResult) {
	if result.Run != "" {
		fmt.Fprintf(out, "Lauf: %s\n", map[string]string{
			project.DocsRunFull: "voll (Auftrag über .)", project.DocsRunPartial: "Teil-Lauf",
			project.DocsRunResume: "Wiederaufnahme (nur zerlegte Aufträge)", project.DocsRunNone: "keiner",
		}[result.Run])
	}
	for _, order := range result.Orders {
		state := order.Kind
		if order.Waiting {
			state = fmt.Sprintf("roh, wartet noch %d Min", order.RemainingMinutes)
		} else if order.Kind == project.DocsOrderKindRaw {
			state = "roh, reif"
		} else {
			state = fmt.Sprintf("zerlegt, %s %s", order.Dokument, order.Aktion)
		}
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", order.Ref, order.Entstanden, state, order.Anlass)
		fmt.Fprintf(out, "  %s\n", strings.Join(order.Pfade, ", "))
	}
	for i, raw := range result.RawOrders {
		fmt.Fprintf(out, "roh %d: %s (%s) aus %d Auftrag/Aufträgen\n", i+1, strings.Join(raw.Pfade, ", "), raw.Entstanden, len(raw.Refs))
	}
	for _, problem := range result.Problems {
		fmt.Fprintf(out, "Nicht lesbar: %s\n", problem)
	}
	if result.NothingToDo {
		fmt.Fprintf(out, "Nichts zu tun: %s.\n", result.Reason)
	}
}

// runDocsOrder ist `k-playbook docs order add <code|tools> --pfad <pfad>…
// --anlass <satz> --sitzung <kennung> [--title <titel>] [--json]` (Task 179):
// reiht einen rohen Doku-Auftrag ein oder ergänzt den vorhandenen derselben
// Sitzung (project.DocsOrders.Add) — dieselbe Fachlogik wie das MCP-Werkzeug
// k_playbook_docs_order_add. Der Singular order (legt einen Auftrag an) steht
// gegen orders (wirkt auf mehrere).
func runDocsOrder(args []string) error {
	usage := "docs order add <" + strings.Join(project.DocsOrderProducers, "|") + "> --pfad <pfad>… --anlass <satz> --sitzung <kennung> [--title <titel>] [--json]"
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printDocsUsage(os.Stdout)
		return nil
	}
	if args[0] != "add" {
		printDocsUsage(os.Stderr)
		return fmt.Errorf("unbekannter Unterbefehl: docs order %s — erwartet wird %s", args[0], usage)
	}
	var asJSON bool
	var producer string
	var in project.DocsOrderAddInput
	var mode string
	setValue := func(option string, value string) error {
		switch option {
		case "--anlass":
			in.Anlass = value
		case "--sitzung":
			in.Sitzung = value
		case "--title":
			in.Title = value
		default:
			return fmt.Errorf("unbekannte Option %s — erwartet wird %s", option, usage)
		}
		return nil
	}
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		switch {
		case arg == "--json":
			asJSON, mode = true, ""
		case arg == "-h" || arg == "--help":
			printDocsUsage(os.Stdout)
			return nil
		case arg == "--pfad":
			mode = "pfad"
		case arg == "--anlass" || arg == "--sitzung" || arg == "--title":
			if i+1 >= len(rest) {
				return fmt.Errorf("%s braucht einen Wert — erwartet wird %s", arg, usage)
			}
			i++
			mode = ""
			if err := setValue(arg, rest[i]); err != nil {
				return err
			}
		case strings.HasPrefix(arg, "--pfad="):
			in.Pfade = append(in.Pfade, strings.TrimPrefix(arg, "--pfad="))
		case strings.HasPrefix(arg, "--") && strings.Contains(arg, "="):
			option, value, _ := strings.Cut(arg, "=")
			mode = ""
			if err := setValue(option, value); err != nil {
				return err
			}
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unbekannte Option %s — erwartet wird %s", arg, usage)
		case mode == "pfad":
			in.Pfade = append(in.Pfade, arg)
		case producer == "":
			producer = arg
		default:
			return fmt.Errorf("unerwartetes Argument %q — erwartet wird %s", arg, usage)
		}
	}
	if producer == "" {
		return fmt.Errorf("erwartet wird %s", usage)
	}
	in.Producer = producer
	if err := project.ValidDocsOrderProducer(producer); err != nil {
		return docError(err)
	}
	if len(in.Pfade) == 0 || strings.TrimSpace(in.Anlass) == "" || strings.TrimSpace(in.Sitzung) == "" {
		return fmt.Errorf("--pfad, --anlass und --sitzung sind Pflicht — erwartet wird %s", usage)
	}
	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}
	var result project.DocsOrderAddResult
	orders, err := openDocsOrders(projectDir)
	switch {
	case err == nil:
		result, err = orders.Add(context.Background(), in)
		if err != nil {
			return docError(err)
		}
	case project.DocRefErrorCode(err) == project.DocCodeKephalaionUnavailable:
		// Ohne Kephalaion wird nicht eingereiht; das ist kein Fehler des Aufrufs.
		result = project.DocsOrderAddUnavailable(producer, err)
	default:
		return docError(err)
	}
	if asJSON {
		return printDocsJSON(result)
	}
	printDocsOrderAdd(os.Stdout, result)
	return nil
}

// printDocsOrderAdd schreibt die Antwort von docs order add ohne --json.
func printDocsOrderAdd(out io.Writer, result project.DocsOrderAddResult) {
	text := fmt.Sprintf("%s\t%s", result.Status, result.Producer)
	if result.Ref != "" {
		text += fmt.Sprintf("\t%s\t%d", result.Ref, result.Revision)
	}
	if result.Reason != "" {
		text += fmt.Sprintf("\t%s: %s", result.Reason, result.Message)
	}
	fmt.Fprintln(out, text)
}

// runDocsOrdersSplit ist `k-playbook docs orders split --json` (Task 179): liest
// von stdin, was die Sitzung zerlegt und der Nutzer bestätigt hat — {"sitzung":
// …, "documents": [{"dokument", "aktion", "pfade", "anlaesse": [{"datum",
// "anlass"}], "anlass", "entstanden", "abgelehnt"}], "raw": [<keph:-Verweis>…]}
// —, schreibt je Dokument einen zerlegten Auftrag oder ergänzt den offenen und
// schließt danach die aufgegangenen rohen Aufträge (project.DocsOrders.Split);
// dieselbe Fachlogik wie das MCP-Werkzeug k_playbook_docs_orders_split. Ein
// Dokument mit failed endet nach der Ausgabe mit Exit 1.
func runDocsOrdersSplit(args []string) error {
	const usage = "docs orders split --json   (Eingabe als JSON von stdin)"
	var asJSON bool
	for _, arg := range args {
		switch {
		case arg == "--json":
			asJSON = true
		case arg == "-h" || arg == "--help" || arg == "help":
			printDocsUsage(os.Stdout)
			return nil
		default:
			return fmt.Errorf("unbekannte Option %s — erwartet wird %s", arg, usage)
		}
	}
	if !asJSON {
		return fmt.Errorf("--json ist Pflicht: die Eingabe kommt als JSON von stdin — erwartet wird %s", usage)
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("stdin lesen: %w", err)
	}
	var in project.DocsSplitInput
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		return docError(fmt.Errorf("Eingabe ist kein gültiges JSON für docs orders split: %w (invalid_input)", err))
	}
	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}
	orders, err := openDocsOrders(projectDir)
	if err != nil {
		return docError(err)
	}
	result, err := orders.Split(context.Background(), in)
	if err != nil {
		return docError(err)
	}
	if err := printDocsJSON(result); err != nil {
		return err
	}
	var failed []string
	for _, doc := range result.Documents {
		if doc.Status == project.DocsSplitFailed {
			failed = append(failed, doc.Dokument)
		}
	}
	for _, raw := range result.Raw {
		if raw.Status == project.DocsSplitFailed {
			failed = append(failed, raw.Ref)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("nicht alles geschrieben oder geschlossen: %s", strings.Join(failed, ", "))
	}
	return nil
}
