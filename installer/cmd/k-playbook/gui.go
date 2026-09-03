package main

import (
	"fmt"
	"io"
	"os"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/webui"
)

// runGUI ist der argumentlose Einstieg. Er sieht zuerst nach, ob für dieses
// Projekt schon ein Server läuft, und handelt nach dem Ergebnis: ein zweiter
// Aufruf startet nichts Neues, er öffnet nur den Browser.
func runGUI() error {
	cleanUpLegacy()
	mirrorHostInstall()

	key, err := guiproc.Key()
	if err != nil {
		return err
	}
	finding, err := guiproc.Inspect(key, guiproc.OwnVersion(), guiproc.DefaultInspector())
	if err != nil {
		return err
	}
	return reuseOrStart(finding, guiActions{
		open:    openExisting,
		stop:    stopExisting,
		discard: guiproc.Remove,
		start:   webui.Run,
		out:     os.Stdout,
	})
}

// guiActions sind die Handgriffe, zwischen denen reuseOrStart wählt.
// Austauschbar, damit die Entscheidung ohne Server und Browser prüfbar ist.
type guiActions struct {
	// open zeigt einen laufenden Server: URL ausgeben, Browser öffnen.
	open func(record guiproc.Record)
	// stop beendet einen laufenden Server anderer Version und meldet, ob er
	// innerhalb der Wartegrenze weg war.
	stop func(finding guiproc.Finding) bool
	// discard entfernt eine verwaiste Laufzeitdatei.
	discard func(path string) error
	// start zieht einen neuen Server hoch.
	start func() error
	out   io.Writer
}

// reuseOrStart setzt die Einordnung der Laufzeitdatei in Handlung um.
//
// Ein Server anderer Version wird ersetzt: nach einem git pull von Hand wählt
// der Wrapper schon das neue Binary, der alte Daemon liefe sonst weiter. Ein
// eigener Prozess, der nicht antwortet, wird nicht angetastet — seine Datei zu
// löschen hieße, einen zweiten Server für dasselbe Projekt hochzuziehen; der
// Weg ist `k-playbook stop`, das mit SIGTERM immer greift.
func reuseOrStart(finding guiproc.Finding, actions guiActions) error {
	switch finding.Status {
	case guiproc.StatusRunning:
		fmt.Fprintf(actions.out, "Der Server für dieses Projekt läuft bereits (PID %d).\n", finding.Record.PID)
		actions.open(finding.Record)
		return nil
	case guiproc.StatusOtherVersion:
		fmt.Fprintf(actions.out, "Ein Server anderer Version (%s) läuft für dieses Projekt und wird beendet.\n",
			describeVersion(finding.Health.Version))
		if !actions.stop(finding) {
			return unresponsiveError(finding)
		}
		return actions.start()
	case guiproc.StatusOrphaned:
		if err := actions.discard(finding.Path); err != nil {
			return fmt.Errorf("verwaiste Laufzeitdatei entfernen: %w", err)
		}
		fmt.Fprintf(actions.out, "Verwaiste Laufzeitdatei entfernt: %s\n", finding.Path)
		return actions.start()
	case guiproc.StatusUnresponsive:
		return unresponsiveError(finding)
	default:
		return actions.start()
	}
}

func describeVersion(version string) string {
	if version == "" {
		return "ohne VERSION"
	}
	return version
}

func unresponsiveError(finding guiproc.Finding) error {
	return fmt.Errorf("für dieses Projekt läuft schon ein Server (PID %d), der nicht antwortet; Laufzeitdatei: %s\nBeenden mit: k-playbook stop",
		finding.Record.PID, finding.Path)
}

// openExisting zeigt den laufenden Server so, wie ein frischer Start es täte.
func openExisting(record guiproc.Record) {
	webui.Announce(record.URL())
	fmt.Println("Beenden mit: k-playbook stop")
}

// stopExisting schickt dem alten Server den Shutdown und wartet auf sein
// Ende. Die Wartegrenze liegt über der Zeit, die der Server sich selbst zum
// Herunterfahren gibt — sonst gälte ein regulär endender als hängend.
func stopExisting(finding guiproc.Finding) bool {
	if err := guiproc.RequestShutdown(finding.Record.Addr); err != nil {
		return false
	}
	return guiproc.WaitForExit(finding.Path, finding.Record.PID, guiproc.ExitTimeout)
}
