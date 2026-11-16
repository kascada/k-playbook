package review

import (
	"errors"
	"testing"
)

// Ohne die Naht kennt das Programm keinen Ort der Review-Ergebnisse:
// ResultsDir liefert ErrResultsDirUndecided. Mit der Naht, die nur Tests
// setzen, liefert es deren Verzeichnis.
func TestResultsDirOhneNahtIstUnentschieden(t *testing.T) {
	if ResultsDirOverride != "" {
		t.Fatalf("die Naht ist außerhalb eines Tests gesetzt: %q", ResultsDirOverride)
	}
	dir, err := ResultsDir()
	if dir != "" || !errors.Is(err, ErrResultsDirUndecided) {
		t.Fatalf("ohne Naht: %q, %v", dir, err)
	}
	if err.Error() != "Der Ort der Review-Ergebnisse ist noch nicht entschieden." {
		t.Errorf("Meldung = %q", err)
	}

	want := t.TempDir()
	ResultsDirOverride = want
	t.Cleanup(func() { ResultsDirOverride = "" })
	if dir, err := ResultsDir(); err != nil || dir != want {
		t.Errorf("mit Naht: %q, %v", dir, err)
	}
}

// Die Sperre gilt ohne Ausnahme: Paused meldet ErrPaused mit dem einen Satz.
func TestPausedMeldetDieSperre(t *testing.T) {
	if err := Paused(); !errors.Is(err, ErrPaused) || err.Error() != PausedMessage {
		t.Errorf("Paused() = %v", err)
	}
}
