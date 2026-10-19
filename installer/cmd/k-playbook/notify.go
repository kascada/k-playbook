package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/notify"
)

// notifyUsage ist die Hilfe des Unterkommandos.
const notifyUsage = `k-playbook notify --title <text> [--body <text>] [--sound] [--json]

Zeigt dem Nutzer eine Meldung auf diesem Rechner: ein Popup, mit --sound dazu
ein Ton. Gedacht für das Ende einer langen Arbeit, wenn niemand mehr auf den
Bildschirm schaut.

Der Weg hängt vom Rechner ab: unter WSL ein Windows-Toast über powershell.exe,
auf macOS osascript, auf einem Linux-Desktop notify-send. Hat der Rechner
keinen Weg zum Nutzer — etwa ein Devcontainer —, endet das Kommando mit
Fehler und nennt den Grund. Mit --json steht das Ergebnis als JSON auf stdout:
"channel", "soundPlayed" und, wenn ein Ton verlangt war und nicht ging,
"reason". Dasselbe wie das MCP-Werkzeug k_playbook_notify.
`

// runNotify führt das Unterkommando `notify` aus. Die Meldung selbst
// baut notify.Send, damit das MCP-Werkzeug keine zweite Fassung bildet.
func runNotify(out io.Writer, args []string) error {
	flags := flag.NewFlagSet("notify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	title := flags.String("title", "", "Überschrift der Meldung")
	body := flags.String("body", "", "Text der Meldung")
	sound := flags.Bool("sound", false, "zusätzlich einen Ton spielen")
	asJSON := flags.Bool("json", false, "Ergebnis als JSON")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(out, notifyUsage)
			return nil
		}
		fmt.Fprint(os.Stderr, notifyUsage)
		return err
	}
	if rest := flags.Args(); len(rest) > 0 {
		fmt.Fprint(os.Stderr, notifyUsage)
		return fmt.Errorf("notify erwartet keine Argumente: %s", strings.Join(rest, " "))
	}
	if strings.TrimSpace(*title) == "" {
		fmt.Fprint(os.Stderr, notifyUsage)
		return errors.New("notify braucht --title")
	}

	result, err := notify.Send(context.Background(), notify.Request{Title: *title, Body: *body, Sound: *sound})
	if err != nil {
		return err
	}

	if *asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	line := "Meldung gezeigt über " + result.Channel
	if result.Reason != "" {
		line += " (" + result.Reason + ")"
	}
	_, err = fmt.Fprintln(out, line)
	return err
}
