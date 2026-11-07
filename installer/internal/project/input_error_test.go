package project

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"
)

// Ein Eingabefehler ist per errors.As erkennbar, auch umhüllt, und lässt
// errors.Is durch sich hindurch auf den Grundfehler treffen.
func TestInputErrorIstErkennbar(t *testing.T) {
	err := InputErrorf("%s gibt es nicht: %w", "x.md", fs.ErrNotExist)
	wrapped := fmt.Errorf("lesen: %w", err)

	var input *InputError
	if !errors.As(wrapped, &input) {
		t.Fatalf("kein Eingabefehler erkannt: %v", wrapped)
	}
	if input.Error() != "x.md gibt es nicht: file does not exist" {
		t.Errorf("Meldung: %q", input.Error())
	}
	if !errors.Is(wrapped, fs.ErrNotExist) {
		t.Error("errors.Is trifft den Grundfehler nicht")
	}
	if errors.As(fmt.Errorf("anders"), &input) {
		t.Error("gewöhnlicher Fehler als Eingabefehler erkannt")
	}
}

// docFilePath meldet ungültige Pfade als Eingabefehler.
func TestDocFilePathMeldetEingabefehler(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"", "/etc/passwd", "../x.md", "notiz.txt"} {
		_, err := docFilePath(root, rel)
		var input *InputError
		if !errors.As(err, &input) {
			t.Errorf("%q: kein Eingabefehler: %v", rel, err)
		}
	}
}
