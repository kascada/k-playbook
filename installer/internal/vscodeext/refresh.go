package vscodeext

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Action ist der Ausgang eines Nachzugs.
type Action string

const (
	// ActionNothing: die passende Fassung liegt schon.
	ActionNothing Action = "nichts zu tun"
	// ActionInstalled: installiert oder nachgezogen.
	ActionInstalled Action = "installiert"
	// ActionFailed: versucht und gescheitert. Nur dieser Ausgang ist laut.
	ActionFailed Action = "gescheitert"
	// ActionBlocked: EnvNoInstall steht, es wurde keine CLI angefasst.
	ActionBlocked Action = "gesperrt"
)

// Refresh ist der letzte selbsttätige Nachzug dieses Dienstes.
type Refresh struct {
	At     time.Time `json:"at"`
	Action Action    `json:"action"`
	// Version ist die Fassung, die installiert wurde oder werden sollte.
	Version string `json:"version,omitempty"`
	CLI     *CLI   `json:"cli,omitempty"`
	// Error und Output nennen den Grund eines Fehlschlags. Die Ausgabe der
	// CLI gehört dazu: sie ist oft das Einzige, was ihn benennt.
	Error  string `json:"error,omitempty"`
	Output string `json:"output,omitempty"`
}

var (
	lastMu sync.RWMutex
	last   *Refresh
)

// LastRefresh ist der letzte Nachzug dieses Prozesses, nil solange keiner
// stattgefunden hat. Nur im Speicher: ein Dienst, der neu startet, zieht
// gleich wieder nach, und eine Datei dafür wäre eine weitere Begleitdatei.
func LastRefresh() *Refresh {
	lastMu.RLock()
	defer lastMu.RUnlock()
	if last == nil {
		return nil
	}
	copied := *last
	return &copied
}

func noteRefresh(refresh Refresh) {
	lastMu.Lock()
	defer lastMu.Unlock()
	refresh.At = time.Now()
	last = &refresh
}

// EnsureInstalled installiert die eingebettete Erweiterung, wenn sie fehlt
// oder in anderer Fassung liegt. Gedacht für den Aufruf im Hintergrund beim
// Start des Dienstes — der Start wird nie aufgehalten, und ein Fehlschlag
// beendet nichts.
//
// Still bleibt der Lauf, wenn kein VS Code in der Umgebung ist oder nichts zu
// tun war. Laut wird er nur bei einem Fehlschlag: der nennt im Log seinen
// Grund und erscheint über LastRefresh in der Statusanzeige, damit er nicht
// stillschweigend liegen bleibt.
//
// Der einzige Auslöser ist dieser Start. Eine von Hand entfernte Erweiterung
// kommt deshalb beim nächsten Dienststart zurück (Todo #23).
func EnsureInstalled(ctx context.Context, lockDir string, log io.Writer) {
	data, ok := VSIX()
	if !ok {
		return
	}
	version, err := Version(data)
	if err != nil {
		fmt.Fprintf(log, "VS-Code-Erweiterung: eingebettete VSIX unlesbar: %v\n", err)
		noteRefresh(Refresh{Action: ActionFailed, Error: err.Error()})
		return
	}

	// Vor der Wahl der CLI, damit ein gesperrter Lauf nicht einmal nach einer
	// sucht. Keine Zeile ins Log: still ist hier richtig, es ist nichts
	// schiefgegangen. Der Vermerk bleibt trotzdem, damit die Statusanzeige
	// den Zustand nennen kann und er nicht wie „noch nicht gelaufen"
	// aussieht.
	if installBlocked() {
		noteRefresh(Refresh{Action: ActionBlocked, Version: version})
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	cli, err := ChooseCLI(home, os.Getenv, LookPath)
	if err != nil {
		// Kein VS Code in dieser Umgebung: nichts tun, nichts melden. Der
		// ausdrückliche Weg steht in der Statusanzeige.
		return
	}

	// Gefragt wird nach dem Verzeichnis, das diese CLI beschreibt, nicht nach
	// „irgendwo": sonst bliebe die Frage nach einem erfolgreichen Nachzug
	// wahr, solange im anderen Verzeichnis eine abweichende Fassung liegt,
	// und der Dienst installierte bei jedem Start erneut.
	target := cli.ExtensionsDir(home)
	if !NeedsInstallIn(version, target, Find(home)) {
		noteRefresh(Refresh{Action: ActionNothing, Version: version, CLI: &cli})
		return
	}

	release, err := lock(lockDir)
	if err != nil {
		fmt.Fprintf(log, "VS-Code-Erweiterung: %v\n", err)
		noteRefresh(Refresh{Action: ActionFailed, Version: version, CLI: &cli, Error: err.Error()})
		return
	}
	defer release()

	// Nach der Sperre neu prüfen: während des Wartens kann der Dienst eines
	// anderen Projekts dasselbe getan haben.
	if !NeedsInstallIn(version, target, Find(home)) {
		noteRefresh(Refresh{Action: ActionNothing, Version: version, CLI: &cli})
		return
	}

	output, err := Install(ctx, cli, data)
	switch {
	case err != nil && errors.Is(err, ErrSudo):
		fmt.Fprintf(log, "VS-Code-Erweiterung %s nicht installiert: %v\n", version, err)
		noteRefresh(Refresh{Action: ActionFailed, Version: version, CLI: &cli, Error: err.Error()})
	case err != nil:
		fmt.Fprintf(log, "VS-Code-Erweiterung %s nicht installiert (%s): %v\n", version, cli.Kind, err)
		if output != "" {
			fmt.Fprintln(log, output)
		}
		noteRefresh(Refresh{Action: ActionFailed, Version: version, CLI: &cli, Error: err.Error(), Output: output})
	default:
		fmt.Fprintf(log, "VS-Code-Erweiterung %s installiert (%s)\n", version, cli.Kind)
		noteRefresh(Refresh{Action: ActionInstalled, Version: version, CLI: &cli, Output: output})
	}
}
