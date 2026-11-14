package project

import (
	"context"

	"github.com/kascada/k-playbook/installer/internal/buildinfo"
	"github.com/kascada/k-playbook/installer/internal/health"
	"github.com/kascada/k-playbook/installer/internal/release"
	"github.com/kascada/k-playbook/installer/internal/version"
)

// Die Quelle program der Umgebungsprüfung (Task 149): Sie warnt, wenn dieses
// Programm älter ist als das neueste Release.
//
// Verglichen werden die beiden einzigen Versionsquellen: die eigene Version im
// Binary (buildinfo.Version) und der Tag des neuesten veröffentlichten
// Release auf GitHub (über HTTPS und anonym). Der Clone k-playbook/ des
// Projekts und seine VERSION spielen keine Rolle.
//
// Gewarnt wird nur bei „älter". Sonst schweigt die Quelle:
//
//   - ohne eigene Version (ein Ad-hoc-Build ohne Build-Flags): Dann wird der
//     Tag gar nicht abgerufen — kein Netzaufruf. health.Check läuft in
//     Briefing, CLI, MCP und Oberfläche; ein Build ohne Version berührt das
//     Netz so nie;
//   - ohne Netz, ohne veröffentlichtes Release oder bei Zeitüberschreitung:
//     Der Tag ist unbekannt, und unbekannt ist kein Nachweis. Die Frist ist
//     kurz (release.TagTimeout), weil die Prüfung in jedem Briefing läuft;
//   - bei gleicher Version;
//   - bei neuerer Version: ein Build über dem neuesten Tag
//     (`make dev-install VERSION=…`) gilt als aktuell.
//
// Als neuestes gilt das neueste veröffentlichte Release, nicht der höchste
// Tag, und die CI veröffentlicht ein Release erst mit allen Assets (Task 151).
// Ein Fenster, in dem die Quelle schon warnt, `k-playbook upgrade` aber noch
// nichts laden kann, gibt es damit nicht mehr.
//
// Die Quelle ist ein reiner Hinweis und liest deshalb den Cache
// (release.CachedLatestTag, Task 151): Ein frischer Eintrag — ein Tag bis zu
// sechs Stunden, ein Fehlschlag bis zu einer halben Stunde alt — kostet
// keinen Netzaufruf. Sie darf so einige Stunden hinterherhinken; `upgrade`
// fragt immer frisch.
func init() {
	health.Register(healthProgram)
}

func healthProgram(ctx context.Context, _ string) []health.Problem {
	running := buildinfo.Version
	if !version.Valid(running) {
		return nil
	}
	latest, err := release.CachedLatestTag(ctx)
	if err != nil {
		return nil
	}
	if version.Compare(running, latest) != version.Older {
		return nil
	}
	return []health.Problem{{
		Level:   health.LevelWarning,
		Area:    health.AreaProgram,
		Message: "Das Programm (" + running + ") ist älter als das neueste Release (" + latest + ").",
		Hint: "Aktualisieren mit " + UpgradeCommand + ": es installiert das Release-Binary des neuesten Tags, geprüft gegen dessen SHA256SUMS. " +
			"Danach die Sitzung neu starten — der MCP-Server der Sitzung behält sonst das alte Programm.",
	}}
}
