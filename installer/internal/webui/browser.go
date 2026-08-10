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

// Zeitfenster, in dem ein gestarteter Browser-Oeffner noch als gescheitert
// gilt, wenn er sich mit Fehler beendet.
const openerGrace = 300 * time.Millisecond

// containerMarker meldet, ob der Prozess in einem Container laeuft, und woran
// das erkannt wurde. Dort ist ein Browserstart sinnlos: er liefe im Container
// und nicht auf dem Rechner vor dem Nutzer.
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

// browserOpener ist ein Kommando, das eine URL im Browser oeffnen kann.
// Die URL wird beim Aufruf an args angehaengt.
type browserOpener struct {
	command string
	args    []string
}

// browserOpeners liefert die Kandidaten in der Reihenfolge, in der sie
// probiert werden. Welche davon vorhanden sind, unterscheidet sich je nach
// Desktop, Distribution und WSL-Setup, deshalb wird nichts vorausgesetzt.
//
// Windows kommt nicht vor: der Einstiegspunkt bin/k-playbook ist ein
// Bash-Skript, das Programm laeuft dort gar nicht erst an. Unter WSL greift
// der Linux-Zweig.
func browserOpeners() []browserOpener {
	if runtime.GOOS == "darwin" {
		return []browserOpener{{command: "open"}}
	}
	return []browserOpener{
		{command: "wslview"},                     // WSL: reicht an den Windows-Browser durch
		{command: "xdg-open"},                    // freedesktop-Standard
		{command: "gio", args: []string{"open"}}, // GNOME/GLib
		{command: "kde-open5"},
		{command: "kde-open"},
		{command: "gnome-open"},
		{command: "x-www-browser"},    // Debian-Alternativensystem
		{command: "sensible-browser"}, // dito
		// Letzter Ausweg in WSL ohne wslu: ueber die Windows-Seite oeffnen.
		{command: "powershell.exe", args: []string{"-NoProfile", "-Command", "Start-Process"}},
	}
}

// openBrowser probiert die Kandidaten der Reihe nach und meldet Erfolg, sobald
// einer startet, ohne sofort zu scheitern.
func openBrowser(url string) error {
	var known, tried []string

	for _, opener := range browserOpeners() {
		known = append(known, opener.command)

		path, err := exec.LookPath(opener.command)
		if err != nil {
			continue
		}
		tried = append(tried, opener.command)

		args := make([]string, 0, len(opener.args)+1)
		args = append(args, opener.args...)
		args = append(args, url)
		if err := startOpener(path, args); err != nil {
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
// dieser Zeit mit einem Fehler, gilt der Versuch als gescheitert. Oeffner, die
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
