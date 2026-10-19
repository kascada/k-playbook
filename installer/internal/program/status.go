package program

import "github.com/kascada/k-playbook/installer/internal/project"

// Status ist der Vergleich des laufenden Programms mit der VERSION des
// Clones — die gemeinsame Auskunft für die Oberfläche (GET /api/update, der
// Versionswechsel im POST, POST /api/update/program) und `k-playbook upgrade`.
type Status struct {
	// Running ist die Version dieses Programms, Clone die VERSION der
	// Installation.
	Running string `json:"running"`
	Clone   string `json:"clone"`
	// Order ist das Ergebnis des Vergleichs: älter, gleich, neuer, unbekannt.
	Order string `json:"order"`
	// Outdated: das laufende Programm ist älter als der Clone. Nur das löst
	// eine Meldung oder eine Installation aus.
	Outdated bool `json:"outdated"`
	// Installable: `k-playbook` im PATH zeigt auf das Installationsziel. Nur
	// dann wird installiert; sonst steht Hint da, mit beiden Pfaden.
	Installable bool   `json:"installable"`
	Target      string `json:"target,omitempty"`
	Found       string `json:"found,omitempty"`
	Hint        string `json:"hint,omitempty"`
}

// CheckStatus vergleicht die Version des laufenden Programms mit der VERSION
// des Clones.
//
// Nur „älter" meldet. Gleich oder neuer ist im Entwicklungsrepo nach
// `make dev-install` der Normalfall, und eine nicht lesbare Version ist kein
// Nachweis. Den PATH prüft sie nur, wenn es etwas anzubieten gäbe.
func CheckStatus(running string, playbookDir string) *Status {
	clone := project.InstalledVersion(playbookDir)
	order := Compare(running, clone)
	status := &Status{Running: running, Clone: clone, Order: order.String(), Outdated: order == Older}
	if !status.Outdated {
		return status
	}
	path := CheckPath()
	status.Target, status.Found, status.Installable = path.Target, path.Found, path.OK
	status.Hint = path.Hint()
	return status
}
