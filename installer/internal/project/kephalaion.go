package project

import (
	"context"
	"fmt"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// KephalaionStatus prüft die Anbindung des Projekts an seine
// Kephalaion-Collection: liest Hub und Collection aus dem Block kephalaion:
// von .k-playbook/K-PLAYBOOK.yaml, holt Account, Token-Datei und Node-Adresse
// aus der Auskunft kephalaion node client und fragt den Node dieses Rechners
// über MCP (whoami). Scheitert nie — jeder Fehler ist ein Zustand. Ohne
// Block wird nichts gefragt.
//
// Die Prüfung steht bewusst nicht in BuildContext: Den rufen auch Werkzeuge,
// die nur den Katalog brauchen (etwa die Review-Auswahl), und die sollen den
// Node nicht fragen. ContextForDir — das Unterkommando context und das
// MCP-Werkzeug — hängt das Ergebnis an.
func KephalaionStatus(projectDir string) kephalaion.Status {
	return kephalaionCheck(projectDir)
}

// kephalaionBinary ist austauschbar, damit Tests nicht vom PATH des Rechners
// abhängen.
var kephalaionBinary = kephalaion.FindBinary

// kephalaionCheck ist austauschbar, damit Tests den Node nicht fragen.
var kephalaionCheck = func(projectDir string) kephalaion.Status {
	binding, err := ProjectBinding(projectDir)
	if err != nil {
		return kephalaion.Status{Configured: true, State: kephalaion.StateInvalidEntry,
			Message: err.Error() + " Abhilfe: die Datei prüfen (k-playbook gui, Abschnitt Projektkonfiguration)."}
	}
	return kephalaion.Check(context.Background(), binding, ConfigDisplayName)
}

// ProjectBinding liest Hub und Collection aus dem Block kephalaion: von
// K-PLAYBOOK.yaml. Ohne Block ist die Anbindung leer (nicht eingetragen); ein
// Fehler heißt, die Datei ist nicht lesbar.
func ProjectBinding(projectDir string) (kephalaion.Binding, error) {
	config, err := ReadConfig(projectDir)
	if err != nil {
		return kephalaion.Binding{}, fmt.Errorf("%s nicht lesbar: %v.", ConfigDisplayName, err)
	}
	return config.Kephalaion.Binding(), nil
}

// KephalaionNotReadyError ist der Fehler eines Zugangs, der an einer nicht
// bereiten Anbindung scheitert. Er trägt den Status, damit ein Aufrufer —
// etwa das Briefing — den Grund aus der Meldung von Kephalaion nennen kann,
// ohne den Node ein zweites Mal zu fragen. Umhüllt ist er immer von einem
// DocRefError mit DocCodeKephalaionUnavailable.
type KephalaionNotReadyError struct {
	Status kephalaion.Status
}

func (e *KephalaionNotReadyError) Error() string {
	return fmt.Sprintf("Kephalaion ist für dieses Projekt nicht bereit (%s): %s", e.Status.State, e.Status.Message)
}

// TroubleshootingCommand ist der Befehl, den Meldungen zu einer kaputten
// Umgebung als nächsten Schritt nennen. Bis zu seinem eigenen Task ist er ein
// Platzhalter, der sagt, was er werden soll.
const TroubleshootingCommand = InstalledCommandName + " troubleshooting"
