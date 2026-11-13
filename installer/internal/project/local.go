package project

import "path/filepath"

// LocalDirName ist das Verzeichnis, in dem früher alles lag, was dem Projekt
// gehört. Es läuft aus: kein Weg im Programm legt es oder etwas darin noch an
// (Task 147) — weder der Start der Oberfläche noch ein Knopf noch ein
// Werkzeug. Der Name bleibt, weil Altbestand darin noch gelesen wird
// (Gesundheitsprüfung, Umzug in `k-playbook upgrade` und /k-upgrade).
//
// Was früher hier angelegt wurde und wohin es gegangen ist: docs, tasks, data
// und material liegen in der Projekt-Collection (Task 107), commands und
// skills in .k-playbook/commands/ und .k-playbook/skills/ (Task 129), rules
// und checks sind entfernt (Task 135, Task 134), guidelines liegen unter
// keph:/docs/guidelines/ (Todo #40), instructions, includes und reviews unter
// keph:/k-playbook/<sorte>/ (Task 143), die Quellenkonfiguration des
// Versionsinventars in keph:/k-playbook/inventory.md (Task 144). results
// führt das Einrichten seit Task 146 nicht mehr; priv und k-playbook.md, die
// letzten beiden Einträge, sind mit Task 147 gegangen, samt der Vorlagen und
// der Schaltung „privat oder versioniert". /k-upgrade räumt die leeren
// Verzeichnisse eines Bestandsprojekts ab.
const LocalDirName = "k-playbook-local"

// LocalDir ist das auslaufende lokale Verzeichnis eines Projekts. Der Pfad
// dient nur dem Lesen von Altbestand; angelegt wird darunter nichts.
func LocalDir(projectDir string) string {
	return filepath.Join(projectDir, LocalDirName)
}
