package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
)

// newGitHubTester baut den Tester für `test`: die echte API, zehn Sekunden je
// Aufruf. Die Naht für die e2e-Tests, die einen httptest.Server einsetzen.
var newGitHubTester = githubaccess.NewTester

// Exit-Codes von `k-playbook github`: 0 ohne Problem, 1 mit mindestens einem
// error, 2 bei Aufruffehlern — unbekannter Unterbefehl, unbekannte Option,
// unbekannter Dienst. Die einmalige warning der frisch angelegten Vorlage
// ändert den Exit-Code nicht: eine angelegte Datei ist ein Zustand, den der
// Nutzer auflöst, kein defekter. Bei `test` heißt 1: mindestens ein
// getesteter Zugang ist failed; übersprungene zählen nicht.
const (
	githubExitProblems = 1
	githubExitUsage    = 2
)

// runGitHub liest und prüft die persönliche GitHub-Zugangsdatei nach dem
// Vertrag docs/github-access.md: `check` nennt die Probleme, `show` die
// Konfiguration, wie sie gelesen wurde, beide mit der Verfügbarkeit jedes
// Zugangs ohne Netz; `test` fragt GitHub, ob die verfügbaren Zugänge
// funktionieren — der einzige Unterbefehl mit Netz, und er läuft nur auf
// diesen Aufruf hin. Alle drei legen die Vorlage an, wenn die Datei fehlt.
// Die Datei ist persönlich, nicht projektbezogen — ein Projekt wird nicht
// gesucht.
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
	case "check", "show", "test":
	default:
		printGitHubUsage(os.Stderr)
		return exitError{code: githubExitUsage, err: fmt.Errorf("unbekannter Unterbefehl: github %s", sub)}
	}

	usage := map[string]string{"check": "github check [--json]", "show": "github show [<dienst>] [--json]", "test": "github test [<dienst>] [--json]"}[sub]
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
	// Die Verfügbarkeit ist ein eigener Schritt nach dem Lesen: ohne Netz,
	// aber mit Blick auf gh-Konfiguration, Dateien und ssh -G. Bei einem
	// error gibt es keine Konfiguration und nichts zu beurteilen.
	if !result.HasErrors() {
		githubaccess.Judge(result.Services)
	}

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

	if sub == "test" {
		report := newGitHubTester().Test(context.Background(), result.File, result.Services)
		if asJSON {
			if err := writeJSON(os.Stdout, report); err != nil {
				return err
			}
		} else if _, err := io.WriteString(os.Stdout, githubTestText(report)); err != nil {
			return err
		}
		if !report.OK {
			return exitError{code: githubExitProblems}
		}
		return nil
	}

	if asJSON {
		return writeJSON(os.Stdout, githubShow{File: result.File, Created: result.Created, Services: result.Services})
	}
	_, err = io.WriteString(os.Stdout, githubShowText(result))
	return err
}

// githubTestText rendert den Testreport: die Datei, dann je Dienst ein
// Block mit Repository und je Zugang einer Zeile — was die Datei sagt, der
// Pfeil, das Ergebnis und dahinter Identität, lesen, schreiben, Ablauf oder
// der Grund.
func githubTestText(report githubaccess.Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Datei: %s\n", report.File)
	if len(report.Services) == 0 {
		b.WriteString("Keine Dienste eingetragen.\n")
		return b.String()
	}
	for _, service := range report.Services {
		b.WriteString("\n")
		fmt.Fprintf(&b, "%s: %s\n", service.Name, service.Repo)
		for index, access := range service.Access {
			fmt.Fprintf(&b, "  %d. %s", index+1, access.Type)
			for _, field := range []struct{ key, value string }{{"login", access.Login}, {"file", access.File}, {"key", access.Key}, {"host", access.Host}} {
				if field.value != "" {
					fmt.Fprintf(&b, "  %s: %s", field.key, field.value)
				}
			}
			b.WriteString("  → " + githubResultText(access) + "\n")
		}
	}
	return b.String()
}

// githubResultText ist das Ergebnis eines Zugangs in einem Satz.
func githubResultText(access githubaccess.AccessResult) string {
	switch access.Result {
	case githubaccess.ResultSkipped:
		return "übersprungen: " + access.Message
	case githubaccess.ResultFailed:
		text := "fehlgeschlagen: " + access.Message
		if access.Identity != "" {
			text += " (Identität " + access.Identity + ")"
		}
		return text
	}
	text := fmt.Sprintf("ok — Identität %s, lesen %s, schreiben %s, Ablauf %s",
		access.Identity, githubAnswerText(access.Read), githubAnswerText(access.Write), githubExpiresText(access.Expires))
	if access.Warning != "" {
		text += ", Warnung: " + access.Warning
	}
	return text
}

func githubAnswerText(answer string) string {
	switch answer {
	case githubaccess.AnswerYes:
		return "ja"
	case githubaccess.AnswerNo:
		return "nein"
	case githubaccess.AnswerUnknown:
		return "unbekannt"
	}
	return answer
}

func githubExpiresText(expires string) string {
	if expires == githubaccess.ExpiresNone || expires == "" {
		return "keiner"
	}
	return expires
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
// Datei, Zahl der Dienste und Zahl der Dienste ohne verfügbaren Zugang, dann
// je nicht verfügbarem Zugang eine Zeile mit seinem Grund; sonst je Problem
// eine Zeile „[stufe] pfad: satz — abhilfe“ — dieselbe Form wie die
// Umgebungsprüfung.
func githubCheckText(result githubaccess.Result) string {
	if len(result.Problems) == 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "%s: %s", result.File, countServices(len(result.Services)))
		if len(result.Services) > 0 {
			unusable := 0
			for _, service := range result.Services {
				if !githubaccess.Usable(service) {
					unusable++
				}
			}
			switch unusable {
			case 0:
				b.WriteString(", alle mit verfügbarem Zugang")
			case 1:
				b.WriteString(", 1 ohne verfügbaren Zugang")
			default:
				fmt.Fprintf(&b, ", %d ohne verfügbaren Zugang", unusable)
			}
		}
		b.WriteString(", keine Probleme\n")
		for _, service := range result.Services {
			for index, access := range service.Access {
				if access.State == githubaccess.StateUnavailable {
					fmt.Fprintf(&b, "services.%s.access[%d]: nicht verfügbar — %s\n", service.Name, index, access.Reason)
				}
			}
		}
		return b.String()
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
// eingeschlossen, und nichts, was sie nicht enthält — dazu je Zugang seine
// Verfügbarkeit in eckigen Klammern.
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
			b.WriteString("  " + githubStateText(access))
			b.WriteString("\n")
		}
	}
	return b.String()
}

// githubStateText ist die Verfügbarkeit eines Zugangs in Textform:
// [verfügbar], [deaktiviert] oder [nicht verfügbar — grund]. Ohne Zustand —
// der Zugang wurde nicht beurteilt — nichts.
func githubStateText(access githubaccess.Access) string {
	switch access.State {
	case githubaccess.StateAvailable:
		return "[verfügbar]"
	case githubaccess.StateDisabled:
		return "[deaktiviert]"
	case githubaccess.StateUnavailable:
		return "[nicht verfügbar — " + access.Reason + "]"
	}
	return ""
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
	fmt.Fprint(out, `k-playbook github — die persönliche GitHub-Zugangsdatei lesen, prüfen und testen

Aufruf:
  k-playbook github check [--json]
  k-playbook github show [<dienst>] [--json]
  k-playbook github test [<dienst>] [--json]

Die Datei ~/.config/k-playbook/github.yaml (bzw. $XDG_CONFIG_HOME/k-playbook/
github.yaml, Override K_PLAYBOOK_GITHUB_CONFIG) nennt je Dienst ein Repository
<owner>/<repo> und die Zugänge in Vorrangreihenfolge: gh (login), token
(file), ssh (key oder host), public. Sie enthält keine Geheimnisse, nur wo
ein Zugang liegt; die Dateien, auf die sie zeigt, werden nicht gelesen.
Vertrag: docs/github-access.md.

  check   Liest die Datei und nennt alle Probleme, sortiert nach Position in
          der Datei, je Zeile „[stufe] pfad: satz — abhilfe“. Ohne Problem
          eine Zeile mit Datei, Zahl der Dienste und Zahl der Dienste ohne
          verfügbaren Zugang, danach je nicht verfügbarem Zugang eine Zeile
          mit dem Grund. Mit --json das Objekt {file, created, services,
          problems}, je Zugang state (available, unavailable, disabled) und
          reason; sobald ein error vorliegt, ist services leer.
  show    Zeigt die Konfiguration, wie sie gelesen wurde: je Dienst ein Block
          mit Repository und Zugängen samt Verfügbarkeit, Pfade mit
          aufgelöstem ~; mit <dienst> nur diesen. Mit --json dasselbe Objekt
          wie check ohne problems. Bei einer Datei mit error gibt show die
          Probleme aus wie check.
  test    Fragt GitHub, ob die verfügbaren Zugänge funktionieren — der
          einzige Unterbefehl mit Netz, mehrere Anfragen je Zugang, zehn
          Sekunden Frist je Anfrage. gh und token über die API (Identität,
          lesen, schreiben, Ablauf; Warnung ab 14 Tage vor Ablauf), ssh über
          ssh -T und git ls-remote (schreiben bleibt unbekannt), public über
          git ls-remote ohne Anmeldung. Deaktivierte und nicht verfügbare
          Zugänge werden übersprungen. Kein Token erscheint in Ausgabe,
          Meldung oder Kommandozeile. Mit --json das Objekt {file, services,
          ok}, je Zugang result (ok, failed, skipped), identity, read, write,
          expires, warning, message.

Verfügbar heißt, ohne Netz beurteilt: der gh-Login ist in gh hinterlegt, die
Token-Datei ist eine Zeile mit Modus 0600, der Schlüssel liegt ohne Gruppen-
und Weltrechte da, der Alias löst per ssh -G nach github.com auf.

Fehlt die Datei, legen alle drei einmal eine Vorlage an (Verzeichnis 0700,
Datei 0600, services: {} und ein Beispiel je Zugangsart) und melden das als
warning; der nächste Aufruf findet eine leere Datei und meldet nichts.

Exit-Code 0 ohne Problem (die Warnung der Vorlage zählt nicht; ein nicht
verfügbarer Zugang ist ein Zustand, kein Problem), 1 mit mindestens einem
error — bei test: mit mindestens einem failed —, 2 bei Aufruffehlern, auch
bei einem Dienst, der nicht in der Datei steht. Die Probleme der Datei und
jeden Dienst ohne verfügbaren Zugang nennt die Umgebungsprüfung (k-playbook
health, Bereich github) und damit das Briefing; getestet wird dort nie.
`)
}
