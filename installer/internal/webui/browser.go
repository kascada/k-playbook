package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Zeitfenster, in dem ein gestarteter Browser-Öffner noch als gescheitert
// gilt, wenn er sich mit Fehler beendet.
const openerGrace = 300 * time.Millisecond

// containerMarker meldet, ob der Prozess in einem Container läuft, und woran
// das erkannt wurde. Dort taugen die geratenen Kandidaten nicht: sie liefen im
// Container und nicht auf dem Rechner vor dem Nutzer. Nur ein ausdrücklich
// gesetzter $BROWSER weiß es besser — siehe announce().
func containerMarker() (string, bool) {
	for _, name := range []string{"REMOTE_CONTAINERS", "DEVCONTAINER", "CODESPACES"} {
		if strings.TrimSpace(os.Getenv(name)) != "" {
			return name, true
		}
	}
	if workdir, err := os.Getwd(); err == nil && strings.HasPrefix(workdir, "/workspaces/") {
		return "/workspaces/", true
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "/.dockerenv", true
	}
	return "", false
}

// browserOpener ist ein Kommando, das eine URL im Browser öffnen kann.
// Die URL wird beim Aufruf an args angehängt; steht in args ein "%s", tritt
// sie stattdessen dort an dessen Stelle.
type browserOpener struct {
	command     string
	args        []string
	placeholder bool
}

// commandLine liefert die Argumente für den Aufruf mit dieser URL.
func (o browserOpener) commandLine(url string) []string {
	args := make([]string, 0, len(o.args)+1)
	if o.placeholder {
		for _, arg := range o.args {
			args = append(args, strings.ReplaceAll(arg, "%s", url))
		}
		return args
	}
	args = append(args, o.args...)
	return append(args, url)
}

// envOpeners liest $BROWSER, die freedesktop-Konvention für den bevorzugten
// Browser: eine mit ":" getrennte Liste von Kommandos, in denen "%s" für die
// URL steht. Fehlt der Platzhalter, wird die URL angehängt.
//
// Diese Kandidaten stehen vor allen geratenen: Wer die Variable setzt, weiß
// besser als jede Kandidatenliste, was hier öffnen soll. Im DevContainer von
// VS Code ist das ein Helfer, der die URL an den Host durchreicht.
func envOpeners() []browserOpener {
	var openers []browserOpener

	for _, entry := range strings.Split(os.Getenv("BROWSER"), ":") {
		fields := strings.Fields(entry)
		if len(fields) == 0 {
			continue
		}

		opener := browserOpener{command: fields[0], args: fields[1:]}
		for _, arg := range opener.args {
			if strings.Contains(arg, "%s") {
				opener.placeholder = true
			}
		}
		openers = append(openers, opener)
	}
	return openers
}

// browserOpeners liefert die Kandidaten in der Reihenfolge, in der sie
// probiert werden. Welche davon vorhanden sind, unterscheidet sich je nach
// Desktop, Distribution und WSL-Setup, deshalb wird nichts vorausgesetzt.
//
// Windows kommt nicht vor: gebaut und als Release-Asset veröffentlicht werden
// nur Linux- und macOS-Binaries, das Programm läuft dort gar nicht erst an.
// Unter WSL greift der Linux-Zweig.
func browserOpeners() []browserOpener {
	guessed := []browserOpener{
		{command: "wslview"},                     // WSL: reicht an den Windows-Browser durch
		{command: "xdg-open"},                    // freedesktop-Standard
		{command: "gio", args: []string{"open"}}, // GNOME/GLib
		{command: "kde-open5"},
		{command: "kde-open"},
		{command: "gnome-open"},
		{command: "x-www-browser"},    // Debian-Alternativensystem
		{command: "sensible-browser"}, // dito
		// Letzter Ausweg in WSL ohne wslu: über die Windows-Seite öffnen.
		{command: "powershell.exe", args: []string{"-NoProfile", "-Command", "Start-Process"}},
	}
	if runtime.GOOS == "darwin" {
		guessed = []browserOpener{{command: "open"}}
	}
	return append(envOpeners(), guessed...)
}

// openBrowser probiert die übergebenen Kandidaten der Reihe nach und meldet
// Erfolg, sobald einer startet, ohne sofort zu scheitern.
func openBrowser(url string, openers []browserOpener) error {
	var known, tried []string

	for _, opener := range openers {
		known = append(known, opener.command)

		path, err := exec.LookPath(opener.command)
		if err != nil {
			continue
		}
		tried = append(tried, opener.command)

		if err := startOpener(path, opener.commandLine(url)); err != nil {
			continue
		}
		return nil
	}

	if len(tried) == 0 {
		return fmt.Errorf("keines dieser Programme ist installiert: %s", strings.Join(known, ", "))
	}
	return fmt.Errorf("kein Versuch war erfolgreich (probiert: %s)", strings.Join(tried, ", "))
}

// startOpener startet das Kommando und wartet kurz ab. Beendet es sich in
// dieser Zeit mit einem Fehler, gilt der Versuch als gescheitert. Öffner, die
// im Vordergrund weiterlaufen, gelten als Erfolg.
func startOpener(path string, args []string) error {
	cmd := exec.Command(path, args...)
	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(openerGrace):
		return nil
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// Die Antwort ist bereits angefangen; mehr als protokollieren geht nicht.
		fmt.Fprintf(os.Stderr, "Antwort schreiben: %v\n", err)
	}
}
