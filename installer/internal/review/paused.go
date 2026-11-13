package review

import "errors"

// Die Review-Wege pausieren (Task 147): Die Ablage der Review-Ergebnisse wird
// umgebaut, und bis dahin legt kein Weg einen Lauf an oder schreibt Ergebnisse
// — die MCP-Werkzeuge k_playbook_review_create, _scan, _merge und
// _write_ai_entry nicht und die Subkommandos scan und merge nicht. Gesperrt
// wird an den Einstiegen; der Review-Code darunter bleibt, wie er ist.
// k_playbook_review_status liest nur und bleibt offen.
//
// Die Sperre nennt keinen Termin: Sie gilt, bis die neue Ablage steht. Dann
// fallen diese Datei und ihre Aufrufer.

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
