package review

import "errors"

// Die Review-Wege pausieren (Task 147): Die Ablage der Review-Ergebnisse wird
// umgebaut, und bis dahin legt kein Weg einen Lauf an, liest einen oder
// schreibt Ergebnisse — die fünf MCP-Werkzeuge k_playbook_review_create,
// _status, _scan, _merge und _write_ai_entry nicht, die Subkommandos scan und
// merge nicht, und die Oberfläche zeigt statt der Läufe die Meldung der
// Sperre. Gesperrt wird an den Einstiegen; der Review-Code darunter bleibt.
//
// Nicht gesperrt ist der Weg eines eigenständigen Reviews (Task 197): Das
// Paket review/workdir bestimmt ein Arbeitsverzeichnis und fasst die
// Fundlisten darin zusammen (CLI `k-playbook review`, MCP
// k_playbook_review_workdir und k_playbook_review_merge_dir). Er legt keinen
// Lauf an, liest keinen und braucht ResultsDir nicht.
//
// Die Sperre nennt keinen Termin: Sie gilt, bis die neue Ablage steht. Wer sie
// aufhebt, braucht zusätzlich zwei Dinge: den Ort der Ergebnisse (ResultsDir
// liefert bis zur Entscheidung nur einen Fehler) und einen Ausschluss dieses
// Orts für die Scanner, damit sie Rohausgaben früherer Läufe nicht lesen.

// PausedCode ist der Fehlercode, mit dem die gesperrten Wege antworten.
const PausedCode = "reviews_paused"

// PausedMessage ist der eine Satz mit dem Grund, wortgleich an allen Wegen.
const PausedMessage = "Reviews sind vorübergehend nicht verfügbar: Die Ablage der Review-Ergebnisse wird umgebaut, " +
	"bis dahin legt k-playbook keinen Lauf an und schreibt keine Ergebnisse. Es wurde nichts ausgeführt und nichts geschrieben."

// ErrPaused ist die Sperre als Fehler, für die Subkommandos.
var ErrPaused = errors.New(PausedMessage)

// Paused meldet die Sperre. Die Einstiege fragen hier nach, bevor sie etwas
// lesen oder auflösen; solange die Review-Wege pausieren, ist die Antwort
// immer ErrPaused.
func Paused() error {
	return ErrPaused
}

// ErrResultsDirUndecided sagt, dass das Programm keinen Ort für die
// Review-Ergebnisse kennt.
var ErrResultsDirUndecided = errors.New("Der Ort der Review-Ergebnisse ist noch nicht entschieden.")

// ResultsDirOverride ist eine Naht für Tests: Nur Tests setzen sie, auf ein
// temporäres Verzeichnis, und setzen sie danach zurück. Im Programm bleibt sie
// leer.
var ResultsDirOverride string

// ResultsDir ist die eine Stelle, von der der Code hinter der Sperre das
// Ergebnisverzeichnis bekommt: das Verzeichnis, unter dem die Läufe und die
// known-decisions.md liegen. Bis der Ort entschieden ist, liefert sie immer
// ErrResultsDirUndecided; die Funktionen darunter (RunDir, ListRuns,
// CreateRun, merge.Options, knowndecisions.Load) nehmen das Verzeichnis als
// Parameter.
func ResultsDir() (string, error) {
	if ResultsDirOverride != "" {
		return ResultsDirOverride, nil
	}
	return "", ErrResultsDirUndecided
}
