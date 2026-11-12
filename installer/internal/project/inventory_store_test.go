package project

import (
	"context"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/versionsources"
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

// RunInventory schreibt nach dem Inventar die README von versions/ aus ihrer
// Vorlage; bricht das Inventar ab (hier: Quellenkonfiguration nicht lesbar),
// wird die README nicht versucht.
func TestRunInventorySchreibtReadmeNachDemInventar(t *testing.T) {
	original := inventoryTemplates
	inventoryTemplates = func(string) DocSource { return repoDefaultsSource(t) }
	t.Cleanup(func() { inventoryTemplates = original })

	root := t.TempDir()
	store := kephtest.New()
	options := inventory.Options{ProjectDir: root}
	_, outcome, readme, err := RunInventory(context.Background(), root, store, options)
	if err != nil || !outcome.Written {
		t.Fatalf("Inventar: %+v, %v", outcome, err)
	}
	if readme == nil || readme.Status != DocsCreated || readme.Dir != "generated/versions" || readme.Ref != "keph:/docs/generated/versions/README.md" {
		t.Fatalf("README: %+v", readme)
	}
	if content, _ := store.Content("docs/generated/versions/README.md"); content != readRepoDefault(t, "docs/generated/versions/README.md") {
		t.Errorf("README: %q", content)
	}

	store.Put(versionsources.DocName, "---\nschema_version: 99\n---\n")
	calls := len(store.Calls)
	_, _, readme, err = RunInventory(context.Background(), root, store, options)
	if err == nil || readme != nil {
		t.Fatalf("abgebrochenes Inventar: %+v, %v", readme, err)
	}
	for _, call := range store.Calls[calls:] {
		if strings.Contains(call, "README") {
			t.Errorf("README trotz Abbruch angefasst: %s", call)
		}
	}
}
