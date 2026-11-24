package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/program"
	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/version"
	"github.com/kascada/k-playbook/installer/internal/webui"
)

// runGUI ist der argumentlose Einstieg, der Client-Pfad. Er pflegt den Wirt,
// sieht dann nach, ob für dieses Projekt schon ein Server läuft, und handelt
// nach dem Ergebnis: ein zweiter Aufruf startet nichts Neues, er öffnet nur
// den Browser. Das Terminal ist danach in jedem Fall wieder frei.
func runGUI() error {
	careForHost()
	noteOutdatedProgram(os.Stdout, guiproc.OwnVersion())

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

// careForHost ist die Startpflege: Wirt und Projekt nachziehen, soweit das
// ohne Schaden geht. Sie läuft bei jedem argumentlosen Aufruf und im Server,
// den eine Programmaktualisierung neu startet (guiproc.HostCareEnv) — dort
// gibt es keinen Aufruf, der sie sonst übernähme. Einen Browser öffnet sie nie.
func careForHost() {
	repairMCPRegistration()
	repairRootInstructions()
}

// noteOutdatedProgram meldet in einer Zeile, dass dieses Programm älter ist
// als das neueste Release.
//
// Nur ein Hinweis: beim Start wird nichts heruntergeladen. Die Oberfläche
// bietet dieselbe Aktualisierung als Knopf an, und erst der Klick installiert.
// Ohne die Zeile bliebe der Zustand „neueres Release, Programm alt" im
// Terminal unsichtbar.
//
// Gefragt wird der neueste Release-Tag (program.CheckStatus) aus dem Cache —
// ein Netzaufruf mit kurzer Frist nur, wenn der Eintrag nicht frisch ist, und
// nur, wenn dieses Programm eine Version trägt. Ist der Tag unbekannt, bleibt
// die Zeile aus.
func noteOutdatedProgram(out io.Writer, running string) {
	environment := project.Detect()
	if !environment.Installed {
		return
	}
	status := program.CheckStatus(context.Background(), running, program.Cached)
	if line := outdatedProgramNotice(status.Running, status.Latest); line != "" {
		fmt.Fprintln(out, line)
	}
}

// outdatedProgramNotice ist die Zeile selbst, leer, wenn nichts zu melden ist.
// Nur „älter" meldet: gleich, neuer und unbekannt sind kein Anlass. Der Weg im
// Terminal ist `k-playbook upgrade`.
func outdatedProgramNotice(running string, latest string) string {
	if version.Compare(running, latest) != version.Older {
		return ""
	}
	return fmt.Sprintf("Hinweis: Dieses Programm (%s) ist älter als das neueste Release (%s). "+
		"Die Oberfläche bietet „Programm aktualisieren“ an; im Terminal: %s.",
		running, latest, project.UpgradeCommand)
}

// repairMCPRegistration zieht die MCP-Registrierung des Projekts nach — ein
// selbsttätiger Weg neben „Aktualisieren“ in der Oberfläche und dem
// Projektschritt von `k-playbook upgrade`, im selben Umfang wie dieser und
// weiter als jenes. Zum selben Umfang gehören seit Task 190 die
// Berechtigungen der Clients für GitHub (repairGitHubPermissions): Sie werden
// nachgezogen und beim Wegfall des Dienstes entfernt, wie bei MCP nur in nicht
// von git erfassten Dateien.
//
// Er ist der Auffangweg für alles, was nicht über die Oberfläche oder
// `k-playbook upgrade` nachgezogen wurde. Ein Klick auf „Einrichten" ist dafür
// nicht nötig.
//
// Eine fehlende Datei oder ein fehlender Eintrag wird eingetragen, aber nur,
// wenn die Datei nicht von git erfasst ist; ein vorhandener Eintrag bleibt
// stehen. Die Meldung sagt, dass die ergänzte Datei einen rechnerbezogenen
// Pfad trägt und unversioniert bleiben sollte. Ein Fehler hält den Start nicht
// auf.
func repairMCPRegistration() {
	environment := project.Detect()
	if !environment.Installed {
		return
	}

	repaired, err := project.RepairMCP(environment.ProjectDir, project.MCPWriteUnversioned)
	for _, write := range repaired {
		fmt.Printf("MCP-Registrierung eingetragen: %s\n", write.Path)
	}
	if len(repaired) > 0 {
		fmt.Println("Die eingetragene Datei trägt den rechnerbezogenen Pfad des installierten k-playbook und sollte unversioniert bleiben.")
	}
	if len(repaired) > 0 {
		fmt.Println("Der Assistent liest den neuen Eintrag erst nach einem Neustart.")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hinweis: MCP-Registrierung nicht vollständig nachgezogen: %v\n", err)
	}

	repairGitHubPermissions(environment.ProjectDir)
}

// repairGitHubPermissions zieht die Berechtigungen der Clients für GitHub nach
// (project.ApplyGitHubPermissions) und meldet nur, was geschah oder einen
// Blick braucht: geschriebene, entfernte und aus git ausgenommene Dateien,
// unberührte Dateien in einem Projekt mit Dienst und Widersprüche des
// Projekts. Ein Fehler hält den Start nicht auf.
func repairGitHubPermissions(projectDir string) {
	report := project.ApplyGitHubPermissions(projectDir)
	for _, line := range report.Lines() {
		fmt.Println(line)
	}
}

// repairRootInstructions zieht AGENTS.md nach — das Gegenstück zur
// MCP-Ergänzung für die zweite Datei, die im Hauptverzeichnis liegt.
//
// Eine ganz fehlende Datei entsteht aus der Vorlage — und das wird eigens
// gesagt, denn es ist eine neue, versionierbare
// Datei im Hauptverzeichnis, kein stiller Nebeneffekt. Ein fremder Text wird
// nie angefasst. Ein Fehler hält den Start nicht auf.
func repairRootInstructions() {
	environment := project.Detect()
	if !environment.Installed {
		return
	}

	repaired, err := project.RepairRootInstructions(environment.ProjectDir)
	if repaired.Created {
		fmt.Printf("%s aus der Vorlage angelegt — eine neue Datei im Hauptverzeichnis, die zum Projekt gehört und eingecheckt werden kann.\n", project.RootInstructionsFile)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hinweis: %s nicht nachgezogen: %v\n", project.RootInstructionsFile, err)
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
	exe, err := os.Executable()
	if err != nil {
		return guiproc.Record{}, fmt.Errorf("eigenes Binary: %w", err)
	}
	child, err := guiproc.Spawn(exe, location.Log)
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
