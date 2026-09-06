package main

import (
	"fmt"
	"io"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
)

// runStop beendet den GUI-Server dieses Projekts.
//
// Die Laufzeitdatei wird gelesen und mit derselben Einordnung geprüft wie beim
// Start. Läuft der eigene Server, bekommt er POST /api/shutdown; antwortet er
// nicht, ein SIGTERM — die Identitätsprüfung hat zuvor sichergestellt, dass
// die PID noch der eigene Prozess ist. Eine verwaiste Datei wird gelöscht,
// eine fehlende ist eine Auskunft und kein Fehler.
func runStop(out io.Writer) error {
	key, err := guiproc.Key()
	if err != nil {
		return err
	}
	finding, err := guiproc.Inspect(key, guiproc.OwnIdentity(), guiproc.DefaultInspector())
	if err != nil {
		return err
	}

	switch finding.Status {
	case guiproc.StatusAbsent:
		fmt.Fprintln(out, "Kein Server für dieses Projekt.")
		return nil
	case guiproc.StatusOrphaned:
		if err := guiproc.Remove(finding.Path); err != nil {
			return fmt.Errorf("verwaiste Laufzeitdatei entfernen: %w", err)
		}
		fmt.Fprintf(out, "Kein Server für dieses Projekt; verwaiste Laufzeitdatei entfernt: %s\n", finding.Path)
		return nil
	}

	record := finding.Record
	if finding.Status != guiproc.StatusUnresponsive {
		if err := guiproc.RequestShutdown(record.Addr); err == nil {
			if guiproc.WaitForExit(finding.Path, record.PID, guiproc.ExitTimeout) {
				fmt.Fprintf(out, "Server beendet (PID %d, %s).\n", record.PID, record.URL())
				return nil
			}
		}
	}

	// Keine oder keine wirksame Antwort: SIGTERM. Der Server behandelt es wie
	// Ctrl+C und räumt seine Datei selbst weg.
	if err := guiproc.Terminate(record.PID); err != nil {
		return fmt.Errorf("SIGTERM an PID %d: %w", record.PID, err)
	}
	if !guiproc.WaitForExit(finding.Path, record.PID, guiproc.ExitTimeout) {
		return fmt.Errorf("PID %d läuft nach SIGTERM weiter; Laufzeitdatei: %s", record.PID, finding.Path)
	}
	// Ein Prozess, der SIGTERM nicht mehr verarbeitet hat, hinterlässt die
	// Datei — dann gehört sie jetzt weg.
	if err := guiproc.Remove(finding.Path); err != nil {
		return fmt.Errorf("Laufzeitdatei entfernen: %w", err)
	}
	fmt.Fprintf(out, "Server per SIGTERM beendet (PID %d).\n", record.PID)
	return nil
}
