package kephalaion

import (
	"reflect"
	"strings"
	"testing"
)

func TestWorkspaceFoldersOeffentlicheZuerst(t *testing.T) {
	folders, err := WorkspaceFolders(Binding{Hub: "vm", Collection: "omni", Account: "kamran-wsl"})
	if err != nil {
		t.Fatal(err)
	}
	want := []WorkspaceFolder{
		{URI: "keph://vm/k-playbook", Name: "Keph vm:k-playbook"},
		{URI: "keph://vm/omni", Name: "Keph vm:omni"},
	}
	if !reflect.DeepEqual(folders, want) {
		t.Fatalf("Ordner = %+v, erwartet %+v", folders, want)
	}
}

// Ist die Collection des Projekts die öffentliche, steht sie einmal da: ein
// doppelter Ordner im Workspace wäre zweimal dieselbe Wurzel.
func TestWorkspaceFoldersOhneDoppelteOeffentliche(t *testing.T) {
	folders, err := WorkspaceFolders(Binding{Hub: "vm", Collection: PublicCollection})
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 || folders[0].URI != "keph://vm/k-playbook" {
		t.Fatalf("Ordner = %+v, erwartet nur die öffentliche", folders)
	}
}

// Hub und Collection fließen in eine Adresse und werden deshalb geprüft. Der
// Account nicht: Ohne ihn ist die Adresse vollständig.
func TestWorkspaceFoldersPrueftNamen(t *testing.T) {
	if _, err := WorkspaceFolders(Binding{Hub: "vm", Collection: "omni"}); err != nil {
		t.Fatalf("ohne Account: %v", err)
	}
	for _, binding := range []Binding{
		{Hub: "", Collection: "omni"},
		{Hub: "vm/x", Collection: "omni"},
		{Hub: "vm", Collection: ""},
		{Hub: "vm", Collection: "../omni"},
	} {
		if _, err := WorkspaceFolders(binding); err == nil {
			t.Fatalf("%+v: kein Fehler", binding)
		} else if strings.Contains(err.Error(), "/") {
			t.Fatalf("%+v: Fehler zitiert den unzulässigen Wert: %v", binding, err)
		}
	}
}
