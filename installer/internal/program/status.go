package program

import (
	"context"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/release"
	"github.com/kascada/k-playbook/installer/internal/version"
)

// Status ist der Vergleich des laufenden Programms mit dem neuesten
// Release-Tag — die gemeinsame Auskunft für die Oberfläche (GET /api/update,
// POST /api/update, POST /api/update/program), den Start der Oberfläche und
// `k-playbook upgrade`.
type Status struct {
	// Running ist die Version dieses Programms, Latest der neueste Release-Tag
	// auf GitHub. Latest ist leer, wenn er unbekannt ist — nicht gefragt (keine
	// eigene Version) oder nicht bestimmbar (LatestError).
	Running string `json:"running"`
	Latest  string `json:"latest"`
	// LatestError sagt, warum der neueste Tag unbekannt ist: ohne Netz, ohne
	// veröffentlichtes Release, Zeitüberschreitung. Leer, wenn er bekannt ist oder gar nicht
	// gefragt wurde.
	LatestError string `json:"latestError,omitempty"`
	// Order ist das Ergebnis des Vergleichs: älter, gleich, neuer, unbekannt.
	Order string `json:"order"`
	// Outdated: das laufende Programm ist älter als der neueste Tag. Nur das
	// löst eine Meldung oder eine Installation aus.
	Outdated bool `json:"outdated"`
	// Installable: `k-playbook` im PATH zeigt auf das Installationsziel. Nur
	// dann wird installiert; sonst steht Hint da, mit beiden Pfaden.
	Installable bool   `json:"installable"`
	Target      string `json:"target,omitempty"`
	Found       string `json:"found,omitempty"`
	Hint        string `json:"hint,omitempty"`
}

// Lookup sagt, wie CheckStatus den neuesten Release-Tag erfragt.
type Lookup int

const (
	// Fresh fragt sofort (release.LatestTag) und schreibt das Ergebnis in
	// den Cache — für alles, was installiert oder ausdrücklich prüft:
	// `k-playbook upgrade` samt `--check`, POST /api/update und
	// /api/update/program, GET /api/update mit fresh=1.
	Fresh Lookup = iota
	// Cached nimmt einen frischen Eintrag aus dem Cache und fragt nur sonst
	// (release.CachedLatestTag) — für die reinen Hinweise: den Start der
	// Oberfläche und GET /api/update beim Laden der Seite.
	Cached
)

// CheckStatus vergleicht die Version des laufenden Programms mit dem neuesten
// Release-Tag, erfragt nach lookup.
//
// Ohne eigene Version wird nichts abgerufen, kein Netzaufruf: Ein Programm
// ohne Version löst weder Warnung noch Installation aus, und ein Ad-hoc-Build
// berührt das Netz so nie. Ist der Tag unbekannt, bleibt der Vergleich
// „unbekannt“ — kein Nachweis für irgendetwas.
//
// Nur „älter" meldet. Gleich ist der Normalfall; neuer ist ein Build über dem
// neuesten Tag (`make dev-install VERSION=…`) und gilt als aktuell. Den PATH
// prüft sie nur, wenn es etwas anzubieten gäbe.
func CheckStatus(ctx context.Context, running string, lookup Lookup) *Status {
	status := &Status{Running: running, Order: version.Unknown.String()}
	if !version.Valid(running) {
		return status
	}
	ask := release.LatestTag
	if lookup == Cached {
		ask = release.CachedLatestTag
	}
	latest, err := ask(ctx)
	if err != nil {
		status.LatestError = strings.TrimSuffix(err.Error(), ".")
		return status
	}
	status.Latest = latest
	order := version.Compare(running, latest)
	status.Order = order.String()
	status.Outdated = order == version.Older
	if !status.Outdated {
		return status
	}
	path := CheckPath()
	status.Target, status.Found, status.Installable = path.Target, path.Found, path.OK
	status.Hint = path.Hint()
	return status
}
