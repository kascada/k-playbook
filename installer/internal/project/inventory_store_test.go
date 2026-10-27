package project

import (
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Ohne bereite Anbindung öffnet OpenInventoryStore keinen Store: der Fehler
// trägt kephalaion_unavailable und den Grund aus der Anbindung. Einen
// Rückfall auf die Platte gibt es nicht — der Aufrufer bekommt nil.
func TestOpenInventoryStoreOhneAnbindungIstKephalaionUnavailable(t *testing.T) {
	original := kephalaionCheck
	kephalaionCheck = func(string) kephalaion.Status {
		return kephalaion.Status{State: kephalaion.StateNotConfigured, Message: "Keine Collection eingetragen."}
	}
	t.Cleanup(func() { kephalaionCheck = original })

	store, err := OpenInventoryStore(t.TempDir())
	if store != nil {
		t.Errorf("ohne Anbindung darf es keinen Store geben: %#v", store)
	}
	if DocRefErrorCode(err) != DocCodeKephalaionUnavailable {
		t.Fatalf("Code = %q, erwartet %s: %v", DocRefErrorCode(err), DocCodeKephalaionUnavailable, err)
	}
	if !strings.Contains(err.Error(), "Keine Collection eingetragen.") {
		t.Errorf("der Grund aus der Anbindung fehlt: %v", err)
	}
}
