package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
)

// Exit-Codes von `k-playbook github run` außer dem des Befehls: dieselben
// wie bei env, damit sie nicht mit denen des Befehls zusammenfallen
// (docs/github-access.md, Output and exit codes).
const (
	githubRunExitFailed        = 125 // run selbst ist gescheitert
	githubRunExitCannotExecute = 126 // gefunden, aber nicht ausführbar
	githubRunExitNotFound      = 127 // nicht gefunden
)

// githubRunArgs ist der zerlegte Aufruf von run.
type githubRunArgs struct {
	service string
	options githubaccess.ChooseOptions
	quiet   bool
	command []string
}

// runGitHubRun startet einen Befehl mit dem Zugang eines Dienstes, nur für
// diesen Prozess eingerichtet: die Wahl ohne Netz (Choose), die Umgebung
// des Befehls (ChildEnv), eine Zeile auf stderr, dann genau ein Start ohne
// Rückfall — run ersetzt sich durch den Befehl (execCommand). Exit-Code und
// Ende durch ein Signal sind damit die des Befehls; eigene Fehler enden mit
// 125, 126 oder 127 und einer Meldung auf stderr. run schreibt nichts; fehlt
// die Datei, endet es mit 125 und dem Hinweis auf `k-playbook github init`.
func runGitHubRun(args []string) error {
	parsed, help, err := parseGitHubRun(args)
	if help {
		printGitHubRunUsage(os.Stdout)
		return nil
	}
	if err != nil {
		return runFailed("%v — Aufruf: k-playbook github run <dienst> [--write] [--use <typ>] [--skip <typ>]… [--quiet] -- <befehl> [<arg>…]", err)
	}

	path, err := githubaccess.Path()
	if err != nil {
		return runFailed("%v", err)
	}
	result := githubaccess.Load(path)
	if !result.Present {
		return runFailed("%s (%s); danach den Dienst %s dort eintragen (Format in docs/github-access.md)", githubaccess.MissingHint, path, parsed.service)
	}
	if result.HasErrors() {
		first := result.Problems[0]
		return runFailed("%s hat %s, zuerst %s: %s — k-playbook github check nennt alle mit Abhilfe", path, countProblems(len(result.Problems)), first.Path, first.Message)
	}
	service, ok := result.Services.Lookup(parsed.service)
	if !ok {
		known := "keiner"
		if names := result.Services.Names(); len(names) > 0 {
			known = strings.Join(names, ", ")
		}
		return runFailed("unbekannter Dienst %s in %s; bekannt: %s — einen bekannten Dienst nennen oder ihn in der Datei eintragen", parsed.service, path, known)
	}
	// Nur der genannte Dienst wird beurteilt: ssh -G kann je Alias bis zu
	// drei Sekunden brauchen, und run läuft in jedem Make-Ziel.
	service = githubaccess.Judge(githubaccess.Services{service})[0]
	choice := githubaccess.Choose(service, parsed.options)
	if choice.Empty() {
		return runFailed("kein verwendbarer Zugang für %s (%s) — k-playbook github show %s zeigt die Zugänge, k-playbook github check die Gründe", service.Name, githubaccess.NoAccessMessage(service, parsed.options), service.Name)
	}

	token := ""
	if choice.API != nil {
		token, err = githubaccess.ReadToken(context.Background(), *choice.API)
		if err != nil {
			return runFailed("das Token für %s ist nicht zu bekommen: %v — k-playbook github test %s prüft den Zugang", service.Name, err, service.Name)
		}
	}
	env, err := githubaccess.ChildEnv(os.Environ(), service, choice, token)
	if err != nil {
		return runFailed("%v", err)
	}

	executable, code, err := resolveCommand(parsed.command[0], os.Getenv("PATH"))
	if err != nil {
		return exitError{code: code, err: err}
	}

	if !parsed.quiet {
		home, _ := os.UserHomeDir()
		fmt.Fprintln(os.Stderr, githubaccess.RunLine(service.Name, choice, home))
	}
	return execCommand(executable, parsed.command, env)
}

// runFailed ist ein eigener Fehler von run: Exit-Code 125 und eine Meldung.
func runFailed(format string, args ...any) error {
	return exitError{code: githubRunExitFailed, err: fmt.Errorf("github run: "+format, args...)}
}

func countProblems(n int) string {
	if n == 1 {
		return "1 Problem"
	}
	return fmt.Sprintf("%d Probleme", n)
}

// parseGitHubRun zerlegt den Aufruf. Optionen stehen vor --, alles danach
// ist der Befehl; -- ist Pflicht, damit eine Option des Befehls nie als
// eine von run gelesen wird.
func parseGitHubRun(args []string) (parsed githubRunArgs, help bool, err error) {
	use := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		value := ""
		name, inline, hasInline := strings.Cut(arg, "=")
		if name == "--use" || name == "--skip" {
			switch {
			case hasInline:
				value = inline
			case i+1 < len(args) && args[i+1] != "--":
				i++
				value = args[i]
			default:
				return parsed, false, fmt.Errorf("%s braucht einen Typ (%s)", name, githubaccess.AccessTypes)
			}
			if !githubaccess.ValidType(value) {
				return parsed, false, fmt.Errorf("unbekannter Typ %s bei %s; erlaubt sind %s", value, name, githubaccess.AccessTypes)
			}
		}
		switch {
		case arg == "--":
			parsed.command = args[i+1:]
			if parsed.service == "" {
				return parsed, false, errors.New("der Dienst fehlt")
			}
			if len(parsed.command) == 0 {
				return parsed, false, errors.New("nach -- fehlt der Befehl")
			}
			parsed.options.Use = use
			return parsed, false, nil
		case arg == "-h" || arg == "--help":
			return parsed, true, nil
		case arg == "--write":
			parsed.options.Write = true
		case arg == "--quiet":
			parsed.quiet = true
		case name == "--use":
			if use != "" && use != value {
				return parsed, false, fmt.Errorf("--use ist schon %s; nur ein Typ ist möglich", use)
			}
			use = value
		case name == "--skip":
			parsed.options.Skip = append(parsed.options.Skip, value)
		case strings.HasPrefix(arg, "-"):
			return parsed, false, fmt.Errorf("unbekannte Option %s", arg)
		case parsed.service != "":
			return parsed, false, fmt.Errorf("zweiter Dienst %s — fehlt -- vor dem Befehl?", arg)
		default:
			parsed.service = arg
		}
	}
	if parsed.service == "" {
		return parsed, false, errors.New("der Dienst fehlt")
	}
	return parsed, false, errors.New("-- vor dem Befehl fehlt")
}

// resolveCommand sucht den Befehl wie execvp: mit / im Namen genau diese
// Datei, sonst im PATH die erste ausführbare. Nicht gefunden ist 127, auch
// ein leerer Name; gefunden, aber nicht ausführbar (kein x-Recht, ein
// Verzeichnis) 126.
func resolveCommand(name string, pathEnv string) (string, int, error) {
	if name == "" {
		// filepath.Join(dir, "") wäre das PATH-Verzeichnis selbst.
		return "", githubRunExitNotFound, errors.New("github run: der Befehl ist leer — nach -- einen Befehl nennen")
	}
	if strings.Contains(name, "/") {
		info, err := os.Stat(name)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return "", githubRunExitNotFound, fmt.Errorf("github run: %s nicht gefunden — den Pfad prüfen", name)
		case err != nil:
			return "", githubRunExitCannotExecute, fmt.Errorf("github run: %s ist nicht ausführbar: %v", name, err)
		case info.IsDir() || info.Mode().Perm()&0o111 == 0:
			return "", githubRunExitCannotExecute, fmt.Errorf("github run: %s ist nicht ausführbar — ausführbar machen (chmod +x) oder den Pfad prüfen", name)
		}
		return name, 0, nil
	}
	found := ""
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			dir = "."
		}
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if !info.IsDir() && info.Mode().Perm()&0o111 != 0 {
			return candidate, 0, nil
		}
		if found == "" {
			found = candidate
		}
	}
	if found != "" {
		return "", githubRunExitCannotExecute, fmt.Errorf("github run: %s ist nicht ausführbar — ausführbar machen (chmod +x)", found)
	}
	return "", githubRunExitNotFound, fmt.Errorf("github run: Befehl %s nicht gefunden — PATH oder Schreibweise prüfen", name)
}

// execCommand ersetzt run durch den Befehl (execve): dieselbe PID, stdin,
// stdout, stderr und Arbeitsverzeichnis des Aufrufers, die Umgebung env.
// Signale erreichen den Befehl damit unmittelbar und genau einmal, und sein
// Exit-Code oder sein Ende durch ein Signal ist das von run. Kehrt execve
// zurück, ist der Befehl nicht gestartet: ENOENT (etwa ein fehlender
// Interpreter in der #!-Zeile) ist 127, alles andere 126.
func execCommand(executable string, command []string, env []string) error {
	err := syscall.Exec(executable, command, env)
	code := githubRunExitCannotExecute
	if errors.Is(err, fs.ErrNotExist) {
		code = githubRunExitNotFound
	}
	return exitError{code: code, err: fmt.Errorf("github run: %s lässt sich nicht starten: %v", command[0], err)}
}

func printGitHubRunUsage(out io.Writer) {
	fmt.Fprint(out, `k-playbook github run — einen Befehl unter dem Zugang eines Dienstes ausführen

Aufruf:
  k-playbook github run <dienst> [--write] [--use <typ>] [--skip <typ>]… [--quiet] -- <befehl> [<arg>…]

Wählt ohne Netz, in der Reihenfolge der Datei, zwei Zugänge des Dienstes:
für git (clone, fetch, push, ls-remote) den ersten verfügbaren jeder Art,
für api (gh und alles, was GH_TOKEN/GITHUB_TOKEN liest) den ersten
verfügbaren vom Typ token oder gh. Ein leerer Kanal ist kein Fehler, nur
beide leer. Eingerichtet wird allein in der Umgebung des Befehls: insteadOf
und pushInsteadOf (die Zielform auf sich selbst festgenagelt) und
Credential-Helper über GIT_CONFIG_* nur für die URLs des Repositorys,
GIT_SSH_COMMAND für ssh key, GH_TOKEN und GITHUB_TOKEN gesetzt oder entfernt,
GIT_TERMINAL_PROMPT=0. Kein Token steht in einer Ausgabe, auf einer
Kommandozeile oder in einem Wert der git-Konfiguration; keine Datei, keine
git-Konfiguration außerhalb, kein gh auth switch. Der Befehl läuft genau
einmal, ohne Rückfall auf einen anderen Zugang: run ersetzt sich durch ihn.

  --write        public überspringen (der einzige Nur-Lese-Zugang)
  --use <typ>    nur Zugänge dieses Typs (gh, token, ssh, public)
  --skip <typ>   Zugänge dieses Typs auslassen; mehrfach möglich
  --quiet        die Zeile mit der Wahl auf stderr weglassen

Vor dem Start steht auf stderr eine Zeile wie
  github run OMNI-GW: git über ssh host github.com-km, api über gh kkl_kmde
stdout gehört dem Befehl. Exit-Code: der des Befehls; endet er durch ein
Signal, endet run ebenso (die Shell meldet 128 + Signal); 125, wenn run
selbst scheitert (Aufruf, unbekannter Dienst,
fehlerhafte Datei, kein Zugang, Token nicht lesbar), 126 nicht ausführbar,
127 nicht gefunden. Vertrag: docs/github-access.md, Abschnitt „Running a
command under a service“.
`)
}
