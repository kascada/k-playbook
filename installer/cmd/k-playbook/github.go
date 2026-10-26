package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
)

// Exit-Codes von `k-playbook github`: 0 ohne Problem, 1 mit mindestens einem
// error, 2 bei Aufruffehlern — unbekannter Unterbefehl, unbekannte Option,
// unbekannter Dienst. Die einmalige warning der frisch angelegten Vorlage
// ändert den Exit-Code nicht: eine angelegte Datei ist ein Zustand, den der
// Nutzer auflöst, kein defekter.
const (
	githubExitProblems = 1
	githubExitUsage    = 2
)

// runGitHub liest und prüft die persönliche GitHub-Zugangsdatei nach dem
// Vertrag docs/github-access.md: `check` nennt die Probleme, `show` die
// Konfiguration, wie sie gelesen wurde. Beide legen die Vorlage an, wenn die
// Datei fehlt. Die Datei ist persönlich, nicht projektbezogen — ein Projekt
// wird nicht gesucht.
func runGitHub(args []string) error {
	if len(args) == 0 {
		printGitHubUsage(os.Stdout)
		return nil
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "-h", "--help", "help":
		printGitHubUsage(os.Stdout)
		return nil
	case "check", "show":
	default:
		printGitHubUsage(os.Stderr)
		return exitError{code: githubExitUsage, err: fmt.Errorf("unbekannter Unterbefehl: github %s", sub)}
	}

	usage := map[string]string{"check": "github check [--json]", "show": "github show [<dienst>] [--json]"}[sub]
	asJSON := false
	var positional []string
	for _, arg := range rest {
		switch {
		case arg == "--json":
			asJSON = true
		case arg == "-h" || arg == "--help":
			printGitHubUsage(os.Stdout)
			return nil
		case strings.HasPrefix(arg, "-"):
			return exitError{code: githubExitUsage, err: fmt.Errorf("unbekannte Option %s — erwartet wird %s", arg, usage)}
		default:
			positional = append(positional, arg)
		}
	}
	if (sub == "check" && len(positional) > 0) || len(positional) > 1 {
		return exitError{code: githubExitUsage, err: fmt.Errorf("erwartet wird %s", usage)}
	}

	path, err := githubaccess.Path()
	if err != nil {
		return exitError{code: githubExitUsage, err: err}
	}
	result := githubaccess.Load(path)

	// check — und show, sobald ein error vorliegt: Es gibt dann keine
	// Konfiguration zu zeigen, nur die Probleme, in derselben Form.
	if sub == "check" || result.HasErrors() {
		if asJSON {
			if err := writeJSON(os.Stdout, result); err != nil {
				return err
			}
		} else if _, err := io.WriteString(os.Stdout, githubCheckText(result)); err != nil {
			return err
		}
		if result.HasErrors() {
			return exitError{code: githubExitProblems}
		}
		return nil
	}

	if len(positional) == 1 {
		service, ok := result.Services.Lookup(positional[0])
		if !ok {
			known := "keiner"
			if names := result.Services.Names(); len(names) > 0 {
				known = strings.Join(names, ", ")
			}
			return exitError{code: githubExitUsage, err: fmt.Errorf("unbekannter Dienst %s in %s; bekannt: %s", positional[0], path, known)}
		}
		result.Services = githubaccess.Services{service}
	}
	if asJSON {
		return writeJSON(os.Stdout, githubShow{File: result.File, Created: result.Created, Services: result.Services})
	}
	_, err = io.WriteString(os.Stdout, githubShowText(result))
	return err
}

// githubShow ist das JSON von `show`: das Objekt von `check` ohne problems.
type githubShow struct {
	File     string                `json:"file"`
	Created  bool                  `json:"created"`
	Services githubaccess.Services `json:"services"`
}

func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

// githubCheckText rendert das Ergebnis von check: ohne Problem eine Zeile mit
// Datei und Zahl der Dienste, sonst je Problem eine Zeile
// „[stufe] pfad: satz — abhilfe“ — dieselbe Form wie die Umgebungsprüfung.
func githubCheckText(result githubaccess.Result) string {
	if len(result.Problems) == 0 {
		return fmt.Sprintf("%s: %s, keine Probleme\n", result.File, countServices(len(result.Services)))
	}
	var b strings.Builder
	for _, problem := range result.Problems {
		fmt.Fprintf(&b, "[%s] %s: %s", problem.Level, problem.Path, problem.Message)
		if problem.Hint != "" {
			fmt.Fprintf(&b, " — %s", problem.Hint)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// githubShowText rendert die Konfiguration: die Datei, die einmalige
// Warnung der Vorlage, dann je Dienst ein Block mit Repository und den
// Zugängen in Vorrangreihenfolge — was die Datei sagt, aufgelöste Pfade
// eingeschlossen, und nichts, was sie nicht enthält.
func githubShowText(result githubaccess.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Datei: %s\n", result.File)
	for _, problem := range result.Problems {
		fmt.Fprintf(&b, "[%s] %s: %s — %s\n", problem.Level, problem.Path, problem.Message, problem.Hint)
	}
	if len(result.Services) == 0 {
		b.WriteString("Keine Dienste eingetragen.\n")
		return b.String()
	}
	for _, service := range result.Services {
		b.WriteString("\n")
		fmt.Fprintf(&b, "%s: %s\n", service.Name, service.Repo)
		for index, access := range service.Access {
			fmt.Fprintf(&b, "  %d. %s", index+1, access.Type)
			for _, field := range []struct{ key, value string }{{"login", access.Login}, {"file", access.File}, {"key", access.Key}, {"host", access.Host}} {
				if field.value != "" {
					fmt.Fprintf(&b, "  %s: %s", field.key, field.value)
				}
			}
			if access.Disabled {
				b.WriteString("  (disabled)")
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func countServices(n int) string {
	switch n {
	case 0:
		return "keine Dienste"
	case 1:
		return "1 Dienst"
	}
	return fmt.Sprintf("%d Dienste", n)
}

func printGitHubUsage(out io.Writer) {
	fmt.Fprint(out, `k-playbook github — die persönliche GitHub-Zugangsdatei lesen und prüfen

Aufruf:
  k-playbook github check [--json]
  k-playbook github show [<dienst>] [--json]

Die Datei ~/.config/k-playbook/github.yaml (bzw. $XDG_CONFIG_HOME/k-playbook/
github.yaml, Override K_PLAYBOOK_GITHUB_CONFIG) nennt je Dienst ein Repository
<owner>/<repo> und die Zugänge in Vorrangreihenfolge: gh (login), token
(file), ssh (key oder host), public. Sie enthält keine Geheimnisse, nur wo
ein Zugang liegt; die Dateien, auf die sie zeigt, werden nicht gelesen.
Vertrag: docs/github-access.md.

  check   Liest die Datei und nennt alle Probleme, sortiert nach Position in
          der Datei, je Zeile „[stufe] pfad: satz — abhilfe“. Ohne Problem
          eine Zeile mit Datei und Zahl der Dienste. Mit --json das Objekt
          {file, created, services, problems}; sobald ein error vorliegt, ist
          services leer.
  show    Zeigt die Konfiguration, wie sie gelesen wurde: je Dienst ein Block
          mit Repository und Zugängen, Pfade mit aufgelöstem ~; mit <dienst>
          nur diesen. Mit --json dasselbe Objekt wie check ohne problems.
          Bei einer Datei mit error gibt show die Probleme aus wie check.

Fehlt die Datei, legen beide einmal eine Vorlage an (Verzeichnis 0700, Datei
0600, services: {} und ein Beispiel je Zugangsart) und melden das als
warning; der nächste Aufruf findet eine leere Datei und meldet nichts.

Exit-Code 0 ohne Problem (die Warnung der Vorlage zählt nicht), 1 mit
mindestens einem error, 2 bei Aufruffehlern — auch bei einem Dienst, der
nicht in der Datei steht. Dieselben Probleme nennt die Umgebungsprüfung
(k-playbook health, Bereich github) und damit das Briefing.
`)
}
