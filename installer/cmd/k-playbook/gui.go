package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/legacy"
	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/webui"
)

// runGUI ist der argumentlose Einstieg, der Client-Pfad. Er pflegt den Wirt,
// sieht dann nach, ob für dieses Projekt schon ein Server läuft, und handelt
// nach dem Ergebnis: ein zweiter Aufruf startet nichts Neues, er öffnet nur
// den Browser. Das Terminal ist danach in jedem Fall wieder frei.
func runGUI() error {
	cleanUpLegacy()
	cleanUpFormerHostInstall()
	protectProjectInstallation()
	repairMCPRegistration()
	repairRootInstructions()

	key, err := guiproc.Key()
	if err != nil {
		return err
	}
	own := guiproc.OwnIdentity()
	finding, err := guiproc.Inspect(key, own, guiproc.DefaultInspector())
	if err != nil {
		return err
	}
	return reuseOrStart(finding, own, guiActions{
		open:    openExisting,
		stop:    stopExisting,
		discard: guiproc.Remove,
		start:   startDetached,
		out:     os.Stdout,
	})
}

// cleanUpFormerHostInstall entfernt Daten des abgelösten Wrapper-Modells.
// Die direkt installierte Datei ~/.local/bin/k-playbook bleibt erhalten.
func cleanUpFormerHostInstall() {
	removals, err := legacy.RemoveFormerHostInstall()
	if len(removals) > 0 {
		fmt.Printf("Alte Host-Installation entfernt (%d):\n", len(removals))
		for _, removal := range removals {
			fmt.Printf("  - %s\n", removal)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hinweis: alte Host-Installation nicht vollständig entfernt: %v\n", err)
	}
}

// protectProjectInstallation setzt eine vorhandene Installation read-only.
// Läuft bei jedem Aufruf und nicht im Server: der startet nur einmal.
func protectProjectInstallation() {
	environment := project.Detect()
	if !environment.Installed || !environment.PlaybookPresent {
		return
	}
	if err := project.SetInstallationReadOnly(environment.ProjectDir); err != nil {
		fmt.Fprintf(os.Stderr, "Hinweis: Installation konnte nicht read-only gesetzt werden: %v\n", err)
	}
}

// repairMCPRegistration zieht eine veraltete MCP-Registrierung des Projekts
// nach — der zweite selbsttätige Migrationsweg neben dem Clone-Update.
//
// Er ist der Auffangweg für alles, was nicht über die Oberfläche aktualisiert
// wurde: ein `git pull` von Hand oder `make -C k-playbook installer-update`
// erreicht die Registrierung nicht, weil sie im Hauptverzeichnis liegt und
// nicht im Clone. Ein Klick auf „Einrichten" ist dafür nicht nötig.
//
// Geschrieben wird nur der eine, eng definierte Fall: ein Eintrag, der auf den
// abgelösten Wrapper zeigt. Steht dort eine akzeptierte Form, bleibt die Datei
// unangetastet — sonst machte jeder Start die getrackten MCP-Dateien eines
// Projekts dreckig. Ein Fehler hält den Start nicht auf.
func repairMCPRegistration() {
	environment := project.Detect()
	if !environment.Installed {
		return
	}

	repaired, err := project.RepairMCP(environment.ProjectDir)
	for _, path := range repaired {
		fmt.Printf("Veraltete MCP-Registrierung korrigiert: %s\n", path)
	}
	if len(repaired) > 0 {
		fmt.Println("Der Assistent liest den neuen Eintrag beim nächsten Start.")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hinweis: MCP-Registrierung nicht vollständig korrigiert: %v\n", err)
	}
}

// repairRootInstructions zieht einen veralteten Anstoßblock in AGENTS.md nach
// — das Gegenstück zur MCP-Korrektur für die zweite Datei, die im
// Hauptverzeichnis liegt und die der Git-Update-Weg deshalb nicht erreicht.
//
// Ein Bestandsprojekt behielte sonst dauerhaft den Aufruf des abgelösten
// Wrappers und schickte jeden Assistenten auf eine Datei, die es nicht mehr
// gibt. Geschrieben wird nur dieser eine Fall: eine fehlende Datei wird nicht
// angelegt und ein fremder Text nicht angefasst. Ein Fehler hält den Start
// nicht auf.
func repairRootInstructions() {
	environment := project.Detect()
	if !environment.Installed {
		return
	}

	repaired, err := project.RepairRootInstructions(environment.ProjectDir)
	if repaired {
		fmt.Printf("Veralteten Anstoß in %s korrigiert.\n", project.RootInstructionsFile)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hinweis: Anstoß nicht korrigiert: %v\n", err)
	}
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
// Ein Server aus einem anderen Stand wird ersetzt: nach einer Neuinstallation
// liegt unter ~/.local/bin schon das neue Binary, der alte Daemon liefe sonst
// weiter und bediente die Oberfläche mit dem alten Code. Ein
// eigener Prozess, der nicht antwortet, wird nicht angetastet — seine Datei zu
// löschen hieße, einen zweiten Server für dasselbe Projekt hochzuziehen; der
// Weg ist `k-playbook stop`, das mit SIGTERM immer greift.
func reuseOrStart(finding guiproc.Finding, own guiproc.Identity, actions guiActions) error {
	switch finding.Status {
	case guiproc.StatusRunning:
		fmt.Fprintf(actions.out, "Der Server für dieses Projekt läuft bereits (PID %d).\n", finding.Record.PID)
		actions.open(finding.Record)
		return nil
	case guiproc.StatusOtherVersion:
		fmt.Fprintf(actions.out, "%s läuft für dieses Projekt und wird beendet.\n",
			describeOtherStand(finding.Health, own))
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

// describeOtherStand benennt, worin sich der laufende Server unterscheidet.
// Bei gleicher Version wäre „andere Version" irreführend — dort steht auf
// beiden Seiten dieselbe, und unterschieden hat sich das gebaute Binary.
func describeOtherStand(health guiproc.Health, own guiproc.Identity) string {
	if health.Version != own.Version {
		return fmt.Sprintf("Ein Server anderer Version (%s)", describeVersion(health.Version))
	}
	return fmt.Sprintf("Ein Server aus einem anderen Build derselben Version (%s)", describeVersion(health.Version))
}

func unresponsiveError(finding guiproc.Finding) error {
	return fmt.Errorf("für dieses Projekt läuft schon ein Server (PID %d), der nicht antwortet; Laufzeitdatei: %s\nBeenden mit: k-playbook stop",
		finding.Record.PID, finding.Path)
}

// openExisting zeigt den laufenden Server: URL ausgeben, Browser öffnen. Der
// Containerfall steckt in Announce — ohne $BROWSER nur URL und Hinweis auf
// die Portweiterleitung.
func openExisting(record guiproc.Record) {
	webui.Announce(record.URL())
	fmt.Println("Beenden mit: k-playbook stop")
}

// startDetached startet den Server als eigenen Prozess und zeigt ihn, sobald
// er antwortet.
func startDetached() error {
	record, err := spawnServer(os.Stdout)
	if err != nil {
		return err
	}
	openExisting(record)
	return nil
}

// spawnServer koppelt den Server ab und wartet auf sein erstes Lebenszeichen:
// Laufzeitdatei mit seiner PID und /api/health mit demselben Schlüssel.
//
// Endet das Kind vorher, kann ein gleichzeitiger Aufruf die Laufzeitdatei
// zuerst geschrieben haben; dann gilt dessen Server, sofern er als eigener
// antwortet. Alles andere ist ein Fehler mit dem Log. Antwortet das Kind
// nicht rechtzeitig, wird es beendet — ein Server, der nicht erreichbar ist,
// bleibt nicht stehen.
func spawnServer(out io.Writer) (guiproc.Record, error) {
	key, err := guiproc.Key()
	if err != nil {
		return guiproc.Record{}, err
	}
	location, err := guiproc.Locate(key)
	if err != nil {
		return guiproc.Record{}, err
	}
	child, err := guiproc.Spawn(location.Log)
	if err != nil {
		return guiproc.Record{}, err
	}

	record, err := child.Await(key, location.File, guiproc.StartupTimeout, guiproc.ProbeHealth)
	if err == nil {
		return record, nil
	}
	if errors.Is(err, guiproc.ErrChildExited) {
		finding, inspectErr := guiproc.Inspect(key, guiproc.OwnIdentity(), guiproc.DefaultInspector())
		if inspectErr == nil && finding.Status == guiproc.StatusRunning {
			fmt.Fprintf(out, "Ein gleichzeitiger Aufruf hat den Server gestartet (PID %d).\n", finding.Record.PID)
			return finding.Record, nil
		}
	} else {
		child.Terminate()
	}
	return guiproc.Record{}, fmt.Errorf("Server nicht gestartet: %w\nLog (%s):\n%s", err, location.Log, child.Log())
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
