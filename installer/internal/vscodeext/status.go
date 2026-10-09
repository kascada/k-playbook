package vscodeext

import (
	"errors"
	"os"
	"os/exec"
)

// LookPath ist exec.LookPath, als Variable: ein Test setzt sie, damit die
// Wahl der CLI nicht am echten `code` dieses Rechners hängt. Die Remote-CLI
// ignoriert --extensions-dir und schriebe sonst in die laufende Umgebung.
var LookPath Lookup = exec.LookPath

// Status ist die Auskunft über die Erweiterung in dieser Umgebung. Dieselbe
// Struktur bedient `k-playbook vscode status` und GET /api/vscode, damit
// Terminal und Oberfläche nicht auseinanderlaufen.
type Status struct {
	// ID und Name der Erweiterung, damit die Oberfläche sie nicht doppelt
	// führen muss.
	ID   string `json:"id"`
	Name string `json:"name"`
	// Embedded ist die Version der eingebetteten VSIX, leer ohne sie.
	Embedded string `json:"embedded"`
	// EmbeddedError nennt den Grund, wenn eine VSIX da, aber unlesbar ist.
	EmbeddedError string `json:"embeddedError,omitempty"`
	// Installed sind die gefundenen Installationen, aus extensions.json.
	Installed []Installation `json:"installed"`
	// CLI ist die CLI, die ein Installationsversuch jetzt nehmen würde; nil,
	// wenn keine zu finden ist.
	CLI *CLI `json:"cli,omitempty"`
	// Target ist das Erweiterungsverzeichnis, das ein Nachzug über diese CLI
	// beschreiben würde; leer, wenn keine CLI zu finden ist. NeedsInstall
	// bezieht sich dann genau darauf.
	Target string `json:"target,omitempty"`
	// Divergent sind Installationen außerhalb von Target mit abweichender
	// Fassung. Sie bleiben liegen — ein Nachzug erreicht sie nicht —, und
	// genau deshalb stehen sie hier statt in NeedsInstall: der Zustand soll
	// sichtbar sein, nicht als dauerhaftes „Nachzuziehen" erscheinen.
	Divergent []Installation `json:"divergent,omitempty"`
	// NeedsInstall sagt, ob installiert oder nachgezogen werden müsste — mit
	// CLI bezogen auf Target, ohne CLI auf jedes Verzeichnis, weil der Weg
	// von Hand über die VSIX jedes treffen kann.
	NeedsInstall bool `json:"needsInstall"`
	// Hint ist der ausdrückliche Weg, wenn keine CLI zu finden ist.
	Hint string `json:"hint,omitempty"`
	// LastRefresh ist der letzte selbsttätige Nachzug dieses Dienstes, nil
	// solange keiner stattgefunden hat.
	LastRefresh *Refresh `json:"lastRefresh,omitempty"`
	// Error nennt einen Fehler beim Erheben selbst, etwa ein unbekanntes
	// Heimatverzeichnis.
	Error string `json:"error,omitempty"`
}

// CurrentStatus erhebt den Status für diesen Rechner.
func CurrentStatus() Status {
	home, err := os.UserHomeDir()
	if err != nil {
		return Status{ID: ID, Name: Name, Error: err.Error()}
	}
	return StatusFor(home, os.Getenv, LookPath)
}

// StatusFor erhebt den Status für ein vorgegebenes Heimatverzeichnis und eine
// vorgegebene Umgebung; so prüfen die Tests ihn ohne das echte VS Code.
func StatusFor(home string, getenv func(string) string, lookPath Lookup) Status {
	status := Status{ID: ID, Name: Name, Installed: []Installation{}}

	version, ok, versionErr := EmbeddedVersion()
	switch {
	case !ok:
		status.EmbeddedError = NoVSIX
	case versionErr != nil:
		status.EmbeddedError = versionErr.Error()
	default:
		status.Embedded = version
	}

	if found := Find(home); len(found) > 0 {
		status.Installed = found
	}
	status.NeedsInstall = NeedsInstall(status.Embedded, status.Installed)

	cli, err := ChooseCLI(home, getenv, lookPath)
	if err == nil {
		status.CLI = &cli
		// Mit einer CLI ist die Frage genauer zu beantworten: ein Nachzug
		// beschreibt genau ein Verzeichnis. Was daneben in anderer Fassung
		// liegt, wird nicht verschwiegen, sondern in Divergent genannt.
		status.Target = cli.ExtensionsDir(home)
		status.NeedsInstall = NeedsInstallIn(status.Embedded, status.Target, status.Installed)
		status.Divergent = Divergent(status.Embedded, status.Target, status.Installed)
	} else if !errors.Is(err, ErrNoCLI) {
		status.Error = err.Error()
	}
	if status.CLI == nil {
		status.Hint = ManualHint
	}
	status.LastRefresh = LastRefresh()
	return status
}
