package project

import (
	"errors"
)

// AssistantSetup ist das Ergebnis des Einrichtens: was an den
// Instruktionsdateien aufgelöst wurde, der Zustand der Wurzeldatei und die
// Link-Zustände danach.
type AssistantSetup struct {
	Instructions InstructionsResult    `json:"instructions"`
	Root         RootInstructionsState `json:"root"`
	Links        []LinkStatus          `json:"links"`
	// RootCreated: AGENTS.md gab es vorher nicht und wurde aus der Vorlage
	// angelegt.
	RootCreated bool `json:"rootCreated,omitempty"`
	// RootExtended: der Anstoß wurde einer vorhandenen Datei angehängt.
	RootExtended bool `json:"rootExtended,omitempty"`
	// RootRefreshed: ein vorhandener, aber veralteter Anstoß wurde ersetzt.
	// Das ist eine Änderung an einer Projektdatei und gehört deshalb in die
	// Antwort, nicht nur ins Log.
	RootRefreshed bool `json:"rootRefreshed,omitempty"`
}

// ApplyAssistantSetup richtet einen Assistenten vollständig ein: Einordnen und
// Auflösen der Instruktionsdateien, dann der Anstoß, dann die Verlinkung.
//
// Die Reihenfolge steckt bewusst hier und nicht in ApplyLinks. Im
// Einrichten-Pfad läuft ApplyLinks nach ApplyRootInstructions — dann hätte die
// Vorlage längst ein AGENTS.md angelegt, und die Umbenennung käme zu spät.
// ApplyLinks ist außerdem generisch über Links(); eine Sonderbehandlung für
// genau eine Instruktionsdatei gehört nicht hinein. So bekommt jeder Einstieg
// denselben Ablauf, auch das Aktualisieren.
//
// Ein Fehler beendet den Ablauf nicht: die vier Katalog-Links haben mit der
// Wurzeldatei nichts zu tun und sollen auch dann stehen, wenn dort etwas
// schiefgeht. Gesammelt wird alles, was aufgetreten ist.
func ApplyAssistantSetup(projectDir string) (AssistantSetup, error) {
	setup := AssistantSetup{}
	errs := []error{}

	result, err := resolveInstructions(projectDir)
	setup.Instructions = result
	if err != nil {
		errs = append(errs, err)
	}

	// Zeile 2 der Fallmatrix: an AGENTS.md steht ein Verzeichnis. Dort liefe
	// os.ReadFile nur in den Fehlerzweig; die Einordnung kennt den Fall bereits.
	if result.runInstructions {
		before := CheckRootInstructions(projectDir)
		outdatedBefore := before.HasMarker && before.HasOutdatedAnstoss
		root, err := applyRootInstructions(projectDir, result.MayCreate)
		if err != nil {
			errs = append(errs, err)
		}
		setup.Root = root
		setup.RootCreated = !before.Present && root.Present
		setup.RootExtended = before.Present && !before.HasMarker && root.HasMarker
		setup.RootRefreshed = outdatedBefore && !root.HasOutdatedAnstoss
	} else {
		setup.Root = CheckRootInstructions(projectDir)
	}

	statuses, err := ApplyLinks(projectDir)
	if err != nil {
		errs = append(errs, err)
	}
	setup.Links = statuses

	return setup, errors.Join(errs...)
}
