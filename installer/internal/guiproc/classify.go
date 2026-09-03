package guiproc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Status ist das Ergebnis der Einordnung einer Laufzeitdatei.
type Status int

const (
	// StatusAbsent: keine Laufzeitdatei, also kein Server.
	StatusAbsent Status = iota
	// StatusRunning: der eigene Server läuft unter dieser URL, mit gleichem
	// Schlüssel und gleicher Version.
	StatusRunning
	// StatusOtherVersion: der eigene Server läuft, aber aus einer anderen
	// Version der Installation.
	StatusOtherVersion
	// StatusOrphaned: der Prozess aus der Datei läuft nachweislich nicht mehr,
	// PID tot oder Startzeit passt nicht. Nur dann darf die Datei weg.
	StatusOrphaned
	// StatusUnresponsive: der Prozess lebt und ist der aus der Datei, aber
	// /api/health antwortet nicht oder mit fremdem Schlüssel. Die Datei
	// bleibt liegen; der Weg ist `k-playbook stop`.
	StatusUnresponsive
)

func (s Status) String() string {
	switch s {
	case StatusAbsent:
		return "nicht vorhanden"
	case StatusRunning:
		return "läuft unter dieser URL"
	case StatusOtherVersion:
		return "läuft mit anderer Version"
	case StatusOrphaned:
		return "verwaist"
	case StatusUnresponsive:
		return "lebt ohne Antwort"
	}
	return fmt.Sprintf("Status(%d)", int(s))
}

// Health ist die Antwort von GET /api/health. Der Server meldet Schlüssel,
// Version und PID; der Client vergleicht mit seiner eigenen Auflösung.
type Health struct {
	Status  string `json:"status"`
	Key     string `json:"key"`
	Version string `json:"version"`
	PID     int    `json:"pid"`
}

// Finding ist die eingeordnete Laufzeitdatei.
type Finding struct {
	Status Status
	// Path ist der Ort der Datei, auch wenn sie fehlt.
	Path   string
	Record Record
	// Health ist die Antwort des Servers, sofern er als eigener geantwortet hat.
	Health Health
}

// Inspector sind die beiden Prüfungen, von denen die Einordnung abhängt.
// Austauschbar, damit die Einordnung ohne Prozesse und Ports prüfbar ist.
type Inspector struct {
	// Identity meldet, ob pid noch der Prozess ist, der zu startTime
	// gestartet wurde.
	Identity func(pid int, startTime time.Time) bool
	// Health holt /api/health von addr.
	Health func(addr string) (Health, error)
}

// DefaultInspector prüft echte Prozesse und echte Ports.
func DefaultInspector() Inspector {
	return Inspector{Identity: IdentityMatches, Health: ProbeHealth}
}

// Inspect liest die Laufzeitdatei zu key und ordnet sie ein. version ist die
// eigene Version, gegen die der laufende Server verglichen wird.
func Inspect(key string, version string, inspector Inspector) (Finding, error) {
	location, err := Locate(key)
	if err != nil {
		return Finding{}, err
	}
	record, ok, err := Read(location.File)
	if err != nil {
		// Eine unlesbare Datei benennt keinen Prozess, typisch ein
		// abgebrochenes Schreiben. Sie zählt als verwaist und wird ersetzt.
		return Finding{Status: StatusOrphaned, Path: location.File}, nil
	}
	if !ok {
		return Finding{Status: StatusAbsent, Path: location.File}, nil
	}
	finding := Classify(record, key, version, inspector)
	finding.Path = location.File
	return finding, nil
}

// Classify ordnet eine gelesene Laufzeitdatei ein, in zwei Stufen: erst die
// Prozessidentität, dann die Antwort von /api/health.
//
// Verwaist ist eine Datei nur, wenn ihr Prozess nachweislich nicht mehr läuft.
// Passt die Identität, antwortet aber niemand oder ein Fremder, ist das der
// eigene, hängende Server: die Datei eines lebenden eigenen Prozesses zu
// löschen hieße, beim nächsten Aufruf einen zweiten Server für dasselbe
// Projekt hochzuziehen.
func Classify(record Record, key string, version string, inspector Inspector) Finding {
	finding := Finding{Record: record}

	if !inspector.Identity(record.PID, time.Unix(record.StartTime, 0)) {
		finding.Status = StatusOrphaned
		return finding
	}

	health, err := inspector.Health(record.Addr)
	if err != nil || health.Key != key || (health.PID != 0 && health.PID != record.PID) {
		finding.Status = StatusUnresponsive
		return finding
	}
	finding.Health = health

	if health.Version != version {
		finding.Status = StatusOtherVersion
		return finding
	}
	finding.Status = StatusRunning
	return finding
}

// healthTimeout begrenzt die Anfrage an einen Server, der vielleicht hängt.
const healthTimeout = 2 * time.Second

// ProbeHealth holt /api/health von addr.
func ProbeHealth(addr string) (Health, error) {
	client := &http.Client{Timeout: healthTimeout}
	response, err := client.Get("http://" + addr + "/api/health")
	if err != nil {
		return Health{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return Health{}, fmt.Errorf("/api/health: Status %d", response.StatusCode)
	}
	var health Health
	if err := json.NewDecoder(response.Body).Decode(&health); err != nil {
		return Health{}, fmt.Errorf("/api/health: %w", err)
	}
	return health, nil
}
