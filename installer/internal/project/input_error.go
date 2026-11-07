package project

import "fmt"

// InputError ist ein Fehler, den der Aufrufer selbst beheben kann: ein
// fehlender oder ungültiger Pfad, ein Pfad aus der erlaubten Zone heraus,
// eine falsche Dateiart. Alles andere — ein nicht beschreibbares
// Verzeichnis, eine vorhandene, aber unlesbare Datei — ist die Umgebung, und
// Korrigieren hilft dort nicht.
//
// Die Unterscheidung muss aus project/ kommen: die Hüllen — MCP, Subkommando —
// können sie nur treffen, wenn der Kern sie unterscheidbar zurückgibt.
// Erkannt wird der Typ per errors.As, also auch durch Umhüllungen mit %w
// hindurch.
type InputError struct {
	err error
}

func (e *InputError) Error() string { return e.err.Error() }

// Unwrap gibt den umhüllten Fehler frei, damit errors.Is auch durch einen
// Eingabefehler hindurch trifft.
func (e *InputError) Unwrap() error { return e.err }

// InputErrorf bildet einen Eingabefehler wie fmt.Errorf; %w bleibt möglich.
func InputErrorf(format string, args ...any) error {
	return &InputError{err: fmt.Errorf(format, args...)}
}
