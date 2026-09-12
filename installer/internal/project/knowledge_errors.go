package project

import (
	"errors"
	"fmt"
)

// InputError ist ein Fehler, den der Aufrufer selbst beheben kann: ein
// unbekannter Erzeuger, ein Pfad aus der Zone heraus, ein fehlendes Feld, ein
// leerer Satz — und auch „nicht vorhanden": wer einen Pfad oder eine Kennung
// nennt, die es nicht gibt, kann sie korrigieren. Alles andere — ein nicht
// beschreibbares Verzeichnis, eine vorhandene, aber unlesbare Datei — ist die
// Umgebung, und Korrigieren hilft dort nicht.
//
// Die Unterscheidung muss aus project/ kommen: die Hüllen — MCP, Subkommando —
// können sie nur treffen, wenn der Kern sie unterscheidbar zurückgibt. Die
// MCP-Hüllen melden diesen Typ als invalid_input und alles andere als
// write_failed beziehungsweise read_failed (docs/mcp.md, „Knowledge
// Contract"). Erkannt wird er per errors.As, also auch durch Umhüllungen mit
// %w hindurch.
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

// IsInputError meldet, ob ein Fehler ein Eingabefehler ist oder einen umhüllt.
func IsInputError(err error) bool {
	var input *InputError
	return errors.As(err, &input)
}
