package guiproc

import (
	"os"
	"time"
)

// startTimeTolerance ist der Spielraum beim Vergleich der Startzeit: die
// Datei trägt ganze Sekunden, und macOS liefert die Zeit nur sekundengenau
// über ps.
const startTimeTolerance = 3 * time.Second

// IdentityMatches meldet, ob pid noch der Prozess ist, der zu startTime
// gestartet wurde: die PID lebt, und die Startzeit aus dem System passt zu
// der aus der Datei. Eine PID allein reicht nicht — nach einem unsauberen
// Ende kann sie längst neu vergeben sein. Eine Prüfung des Prozessnamens gibt
// es nicht; /proc/<pid>/comm ist auf 15 Zeichen begrenzt.
//
// Lässt sich die Startzeit nicht lesen, gilt die Identität als nicht
// bestätigt: lieber ein zweiter Server für dasselbe Projekt als ein SIGTERM
// an einen fremden Prozess.
func IdentityMatches(pid int, startTime time.Time) bool {
	if !processAlive(pid) {
		return false
	}
	actual, err := ProcessStartTime(pid)
	if err != nil {
		return false
	}
	return actual.Sub(startTime).Abs() <= startTimeTolerance
}

// OwnStartTime ist die Startzeit dieses Prozesses, gelesen über dieselbe
// Funktion, die die Prüfung später nutzt. Schlägt das Lesen fehl, gilt jetzt:
// für einen eben gestarteten Prozess liegt das innerhalb der Toleranz.
func OwnStartTime() time.Time {
	if start, err := ProcessStartTime(os.Getpid()); err == nil {
		return start
	}
	return time.Now()
}
