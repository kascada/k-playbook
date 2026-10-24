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
	// RootRefreshed: AGENTS.md wurde nachgezogen — ein veralteter Anstoß
	// ersetzt, abgelöste Zusatzblöcke entfernt, oder beides. Das ist eine
	// Änderung an einer Projektdatei und gehört deshalb in die Antwort, nicht
	// nur ins Log. Was genau geschah, sagen die zwei Felder darunter.
	RootRefreshed bool `json:"rootRefreshed,omitempty"`
	// RootAnstossRefreshed: der Anstoßblock trug einen abgelösten Aufruf und
	// wurde durch die aktuelle Fassung ersetzt.
	RootAnstossRefreshed bool `json:"rootAnstossRefreshed,omitempty"`
	// RootBlocksRemoved nennt die abgelösten Zusatzblöcke, die aus AGENTS.md
	// entfernt wurden; ihr Inhalt kommt mit dem Briefing.
	RootBlocksRemoved []string `json:"rootBlocksRemoved,omitempty"`
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
// Ein Fehler beendet den Ablauf nicht: die drei Katalog-Links haben mit der
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
		setup.RootAnstossRefreshed = outdatedBefore && !root.HasOutdatedAnstoss
		setup.RootBlocksRemoved = removedBlocks(before, root)
		setup.RootRefreshed = setup.RootAnstossRefreshed || len(setup.RootBlocksRemoved) > 0
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

// removedBlocks nennt die abgelösten Zusatzblöcke, die vor dem Lauf dastanden
// und danach nicht mehr.
func removedBlocks(before RootInstructionsState, after RootInstructionsState) []string {
	remaining := map[string]bool{}
	for _, name := range after.ObsoleteBlocks {
		remaining[name] = true
	}
	removed := []string{}
	for _, name := range before.ObsoleteBlocks {
		if !remaining[name] {
			removed = append(removed, name)
		}
	}
	return removed
}
