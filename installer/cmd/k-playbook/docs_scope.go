package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// runDocsScope ist `k-playbook docs scope [<pfad>] [--check <pfad>…] [--json]`
// (Task 179): rechnet den Scope der Docs-Erzeuger, die den Code durchsuchen
// (project.DocsScope) — dieselbe Fachlogik wie das MCP-Werkzeug
// k_playbook_docs_scope. Ein Pfad vor --check ergänzt den Scope (EXTRA_PATH),
// die Pfade nach --check werden einzeln auf Ausschluss geprüft.
func runDocsScope(args []string) error {
	const usage = "docs scope [<pfad>] [--check <pfad>…] [--json]"
	var asJSON, checking bool
	var extra []string
	var check []string
	for _, arg := range args {
		switch {
		case arg == "--json":
			asJSON = true
		case arg == "--check":
			checking = true
		case arg == "-h" || arg == "--help" || arg == "help":
			printDocsUsage(os.Stdout)
			return nil
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unbekannte Option %s — erwartet wird %s", arg, usage)
		case checking:
			check = append(check, arg)
		default:
			extra = append(extra, arg)
		}
	}
	if len(extra) > 1 {
		return fmt.Errorf("höchstens ein zusätzlicher Pfad — erwartet wird %s", usage)
	}
	if checking && len(check) == 0 {
		return fmt.Errorf("--check ohne Pfad — erwartet wird %s", usage)
	}
	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}
	options := project.DocsScopeOptions{Check: check}
	if len(extra) == 1 {
		options.ExtraPath = extra[0]
	}
	result, err := project.DocsScope(context.Background(), projectDir, options)
	if err != nil {
		return docError(err)
	}
	if asJSON {
		return printDocsJSON(result)
	}
	printDocsScope(os.Stdout, result)
	return nil
}

// printDocsScope schreibt den Scope ohne --json in den Zeilen des Preflights.
func printDocsScope(out io.Writer, result project.DocsScopeResult) {
	stand := "kein git-Arbeitsbaum"
	if result.Stand.Git {
		stand = fmt.Sprintf("%s @ %s, %s", result.Stand.Branch, result.Stand.Commit, map[bool]string{true: "sauber", false: "geändert"}[result.Stand.Clean])
	}
	extra := "—"
	if result.Extra != nil {
		extra = result.Extra.Path
	}
	hidden := "ausgeschlossen (.*)"
	if len(result.Exclude.Hidden.Excepted) > 0 {
		hidden += "; ausgenommen: " + strings.Join(result.Exclude.Hidden.Excepted, ", ")
	}
	fmt.Fprintf(out, "Scope         %s (project.repoRoot)\n", result.ScanRoot)
	fmt.Fprintf(out, "Stand         %s\n", stand)
	fmt.Fprintf(out, "Zusätzlich    %s\n", extra)
	fmt.Fprintf(out, "Immer außen   %s\n", strings.Join(result.Exclude.Always, ", "))
	fmt.Fprintf(out, "Versteckt     %s\n", hidden)
	config := "—"
	if len(result.Exclude.Config) > 0 {
		config = strings.Join(result.Exclude.Config, ", ")
	}
	fmt.Fprintf(out, "Aus K-PLAYBOOK.yaml  %s\n", config)
	for _, problem := range result.Exclude.Rejected {
		fmt.Fprintf(out, "              verworfen: %s — %s\n", displayDocsPattern(problem.Pattern), problem.Reason)
	}
	rest := append([]string{}, result.Exclude.Further...)
	for _, file := range result.Exclude.Gitignore {
		rest = append(rest, fmt.Sprintf("%s (%d)", file.File, len(file.Patterns)))
	}
	fmt.Fprintf(out, "Ausschlüsse   %s\n", strings.Join(rest, ", "))
	if result.GitignoreTruncated > 0 {
		fmt.Fprintf(out, "              Grenze: nur die ersten %d .gitignore gelesen, %d weitere nicht\n", len(result.Exclude.Gitignore), result.GitignoreTruncated)
	}
	for _, item := range result.Checks {
		text := "im Scope\t" + item.Path
		if item.Excluded {
			text = fmt.Sprintf("ausgeschlossen\t%s\t%s\t%s", item.Path, item.Reason, item.By)
		}
		if !item.Exists {
			text += "\tfehlt"
		}
		if item.Suggest != "" {
			text += "\tVorschlag: " + item.Suggest
		}
		fmt.Fprintln(out, text)
	}
}
