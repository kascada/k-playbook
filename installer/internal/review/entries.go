package review

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	// RawDirName nimmt die SARIF-Dateien der Jobs auf. Angelegt wird es vom
	// Ausführer beim ersten Job; das Anlegen eines Laufs kennt es nicht.
	RawDirName = "raw"
	// EntrySchemaVersion ist die Fassung von entries/<name>.json.
	EntrySchemaVersion = 1
)

// JobStatus ist der Ausgang eines einzelnen Aufrufs.
//
// ExitCode und Findings sind Zeiger, weil 0 hier zweierlei heißen könnte:
// gemessen und null, oder gar nicht gemessen. Ein übersprungener Job hat
// keinen Exit-Code, und das soll man ihm ansehen.
type JobStatus struct {
	Job   string `json:"job"`
	State State  `json:"state"`
	// Module ist das geprüfte Modul, relativ zum Ziel des Laufs; die Wurzel
	// selbst steht als „.". Gesetzt wird es unabhängig von der Auffächerung:
	// bei genau einem Modul heißt der Job wie im Katalog, und ohne das Feld
	// sähe man dem fertigen Lauf ausgerechnet im Normalfall nicht an, was
	// geprüft wurde. Leer bei workdir target — dort gibt es kein Modul, auf
	// das es zeigen könnte.
	Module   string `json:"module,omitempty"`
	ExitCode *int   `json:"exitCode,omitempty"`
	// SARIF ist der Ort der Datei, relativ zum Laufverzeichnis.
	SARIF    string `json:"sarif,omitempty"`
	Findings *int   `json:"findings,omitempty"`
	// Candidates ist die Zahl der Dateien, die unter dem Bezugspunkt des Jobs
	// als Gegenstand in Frage kamen — die Auskunft, ohne die ein leeres
	// Ergebnis nicht von einem ungeprüften zu unterscheiden ist. Welche Dateien
	// zählen, sagt die Spalte candidates im Katalog.
	//
	// Zeiger wie Findings, und aus demselben Grund: 0 heißt „gemessen und
	// null", also „hier gab es nichts zu prüfen". Nicht gemessen wurde, wo das
	// Feld fehlt — bei einem übersprungenen Job, bei der Sorte none und dann,
	// wenn der Baumlauf selbst gescheitert ist.
	//
	// Die Zahl ist eine Obergrenze und keine Abdeckungsmessung: die
	// werkzeugeigenen Ausschlüsse kennt die Zählung nicht.
	Candidates *int   `json:"candidates,omitempty"`
	Started    string `json:"started,omitempty"`
	Finished   string `json:"finished,omitempty"`
	// Reason nennt bei skipped und failed den Grund im Klartext.
	Reason string `json:"reason,omitempty"`
}

// EntryStatus ist der Inhalt von entries/<name>.json: der Fortschritt eines
// Eintrags samt seiner Jobs.
type EntryStatus struct {
	SchemaVersion int    `json:"schemaVersion"`
	Name          string `json:"name"`
	Kind          Kind   `json:"kind"`
	State         State  `json:"state"`
	Started       string `json:"started,omitempty"`
	Finished      string `json:"finished,omitempty"`
	// Reason erklärt den Zustand dort, wo kein Job ihn erklären kann — bei
	// einem Werkzeug ohne Scan-Job gibt es sonst gar keine Auskunft.
	Reason string      `json:"reason,omitempty"`
	Jobs   []JobStatus `json:"jobs"`
}

// terminal meldet, ob ein Zustand ein Endzustand ist.
func terminal(state State) bool {
	return state == StateDone || state == StateFailed || state == StateSkipped
}

// DeriveEntryState macht aus n Job-Ausgängen den Zustand des Eintrags.
//
// Zuerst die Frage, ob der Eintrag überhaupt durch ist; erst danach die nach
// dem Ausgang:
//
//  0. Läuft noch ein Job oder steht einer aus, ist der Eintrag running. Diese
//     Regel steht voran, weil ein laufender Eintrag sonst über Regel 3 als
//     skipped gälte — also über einen Endzustand, aus dem die Laufableitung ein
//     done für einen laufenden Lauf machte.
//  1. Ein failed macht den Eintrag failed: sein Ergebnis ist unvollständig, und
//     das darf nicht hinter einem grünen Zustand verschwinden.
//  2. Sonst genügt ein done. Nicht der schlechteste Ausgang gewinnt — sonst
//     wäre gitleaks skipped, sobald gitleaks-dir übersprungen wird, und die
//     Datei, die gitleaks-git geschrieben hat, wäre versteckt.
//  3. Sonst skipped. Das heißt genau eins: der Eintrag ist durch, und kein
//     einziger Job ist gelaufen — auch der Fall „gar kein Job" (syft).
func DeriveEntryState(jobs []JobStatus) State {
	failed := false
	done := false
	for _, job := range jobs {
		if !terminal(job.State) {
			return StateRunning
		}
		switch job.State {
		case StateFailed:
			failed = true
		case StateDone:
			done = true
		}
	}
	switch {
	case failed:
		return StateFailed
	case done:
		return StateDone
	default:
		return StateSkipped
	}
}

// ReadEntryStatus liest die Datei eines Eintrags.
func ReadEntryStatus(runDir string, name string) (EntryStatus, error) {
	data, err := os.ReadFile(EntryFile(runDir, name))
	if err != nil {
		return EntryStatus{}, err
	}
	var status EntryStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return EntryStatus{}, err
	}
	return status, nil
}

// EntryState ist der Zustand eines Eintrags, wie er auf der Platte steht.
// Fehlt die Datei, ist der Eintrag noch nicht gestartet: start.
func EntryState(runDir string, name string) State {
	status, err := ReadEntryStatus(runDir, name)
	if err != nil || status.State == "" {
		return StateStart
	}
	return status.State
}

// WriteEntryStatus schreibt die Datei eines Eintrags atomar: erst eine
// Temp-Datei im selben Verzeichnis, dann rename. Sonst sähe ein Leser, der
// während des Laufs nachschaut, irgendwann eine halbe Datei.
func WriteEntryStatus(runDir string, status EntryStatus) error {
	if status.SchemaVersion == 0 {
		status.SchemaVersion = EntrySchemaVersion
	}
	if status.Jobs == nil {
		status.Jobs = []JobStatus{}
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	target := EntryFile(runDir, status.Name)
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	temp, err := os.CreateTemp(dir, "."+status.Name+".*.json")
	if err != nil {
		return err
	}
	name := temp.Name()
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(name)
		return err
	}
	if err := temp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, target); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// DeriveRunState ist der Gesamtzustand eines Laufs, gelesen aus entries/.
//
// run.json hält fest, was ausgewählt wurde; den Fortschritt führen die Dateien
// unter entries/. Weichen beide voneinander ab, gilt entries/.
//
// Ein Laufzustand failed entsteht dabei nicht: ein technischer Fehlschlag steht
// am Eintrag, nicht am Lauf.
func DeriveRunState(runDir string, run Run) State {
	if len(run.Entries) == 0 {
		return StateCreated
	}

	allStart := true
	allTerminal := true
	for _, entry := range run.Entries {
		state := EntryState(runDir, entry.Name)
		if state != StateStart {
			allStart = false
		}
		if !terminal(state) {
			allTerminal = false
		}
	}

	switch {
	case allStart:
		return StateCreated
	case allTerminal:
		return StateDone
	default:
		return StateRunning
	}
}
