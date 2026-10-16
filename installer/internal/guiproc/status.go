package guiproc

// State ist die maschinenlesbare Marke eines Status, eine je Ergebnis der
// Einordnung. Status.String() bleibt daneben die deutsche Prosa für den
// Menschen; ein Aufrufer, der die Antwort auswertet, braucht einen stabilen
// Wert, und ein übersetzter Satz ist keiner.
type State string

const (
	// StateNone: kein Dienst. Dasselbe gilt für ein fehlendes
	// Laufzeitverzeichnis — fehlt es, läuft per Definition keiner.
	StateNone State = "none"
	// StateRunning: der eigene Dienst läuft unter dieser Adresse.
	StateRunning State = "running"
	// StateOtherVersion: der eigene Dienst läuft, aber aus einem anderen Stand.
	StateOtherVersion State = "otherVersion"
	// StateOrphaned: die Laufzeitdatei nennt einen Prozess, den es nicht mehr
	// gibt. Aufgeräumt wird das hier nicht; das bleibt `k-playbook stop`.
	StateOrphaned State = "orphaned"
	// StateUnresponsive: der Prozess lebt und ist der aus der Datei, aber
	// /api/health antwortet nicht oder mit fremdem Schlüssel.
	StateUnresponsive State = "unresponsive"
)

// stateOf ordnet ein Ergebnis der Einordnung seiner Marke zu.
func stateOf(status Status) State {
	switch status {
	case StatusRunning:
		return StateRunning
	case StatusOtherVersion:
		return StateOtherVersion
	case StatusOrphaned:
		return StateOrphaned
	case StatusUnresponsive:
		return StateUnresponsive
	}
	return StateNone
}

// StatusReport ist der Wertesatz einer Auskunft über den Dienst eines
// Projekts: ein benannter Verbund, damit ein später zusätzlicher Wert keinen
// Aufrufer bricht.
//
// Nicht gemessene Felder fehlen in der Antwort, sie stehen nicht als Nullwert
// da — ein Nullwert wäre eine Aussage, die niemand geprüft hat:
//
//   - URL und PID stehen für einen Prozess, der nachweislich lebt und der aus
//     der Laufzeitdatei ist. Bei einem verwaisten Stand fehlen sie: dort nennt
//     die Datei einen Prozess, den es nicht mehr gibt.
//   - Version und Build kommen aus /api/health, also vom laufenden Prozess
//     selbst. Die Werte in der Laufzeitdatei taugen dafür nicht (siehe
//     Record.Build); antwortet der Dienst nicht, fehlen beide.
type StatusReport struct {
	State   State  `json:"state"`
	URL     string `json:"url,omitempty"`
	PID     int    `json:"pid,omitempty"`
	Version string `json:"version,omitempty"`
	Build   string `json:"build,omitempty"`
}

// StatusForDir ordnet den Dienst des Projekts ein, zu dem dir gehört, und
// liefert den Wertesatz dazu. Sie ist die eine Mitte dahinter: `k-playbook gui
// status` rendert ihr Ergebnis, das MCP-Werkzeug k_playbook_gui_status
// serialisiert es — zusammengesetzt wird es genau hier.
//
// Ohne jede Nebenwirkung: es startet nichts, beendet nichts, entfernt keine
// verwaiste Laufzeitdatei und legt auch das Laufzeitverzeichnis nicht an.
// Darum ruft es Inspect nicht, obwohl Inspect dieselbe Folge ist: dessen
// Locate legt über RuntimeDir() das Verzeichnis an. Hier läuft die Folge mit
// der Auflösung, die nur auflöst.
//
// Ein Fehler meldet allein, dass gar nicht eingeordnet werden konnte — weder
// XDG_RUNTIME_DIR noch XDG_STATE_HOME noch ein Home-Verzeichnis sind bekannt.
// Kein Dienst, ein fehlendes Laufzeitverzeichnis und eine unlesbare Datei sind
// Auskünfte und keine Fehler.
func StatusForDir(dir string) (StatusReport, error) {
	key := KeyFrom(dir)
	location, err := resolveLocation(key)
	if err != nil {
		return StatusReport{}, err
	}

	record, ok, err := Read(location.File)
	switch {
	case err != nil:
		// Wie in Inspect: eine unlesbare Datei benennt keinen Prozess, typisch
		// ein abgebrochenes Schreiben. Sie zählt als verwaist.
		return StatusReport{State: StateOrphaned}, nil
	case !ok:
		return StatusReport{State: StateNone}, nil
	}
	return reportOf(Classify(record, key, OwnIdentity(), DefaultInspector())), nil
}

// reportOf setzt den Wertesatz aus einer Einordnung zusammen.
func reportOf(finding Finding) StatusReport {
	report := StatusReport{State: stateOf(finding.Status)}
	switch finding.Status {
	case StatusRunning, StatusOtherVersion:
		report.URL = addressOf(finding.Record)
		report.PID = finding.Record.PID
		report.Version = finding.Health.Version
		report.Build = finding.Health.Build
	case StatusUnresponsive:
		report.URL = addressOf(finding.Record)
		report.PID = finding.Record.PID
	}
	return report
}

// addressOf ist die Adresse aus der Laufzeitdatei, leer ohne Adresse darin:
// Record.URL() baute daraus "http:///", eine Angabe, die keine ist.
func addressOf(record Record) string {
	if record.Addr == "" {
		return ""
	}
	return record.URL()
}
