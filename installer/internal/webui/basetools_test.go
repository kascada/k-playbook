package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

const baseToolsMatrixHeader = "name\tgroup\trole\tguarded\tmethods\tapt_package\tgithub_repo\tasset_ref\tasset_pattern\n"

// newProjectWithBaseMatrix legt ein Projekt an, dessen Installation eine Matrix
// der Basis-Werkzeuge führt.
func newProjectWithBaseMatrix(t *testing.T, body string) string {
	t.Helper()

	root := t.TempDir()
	scriptDir := filepath.Join(project.PlaybookDir(root), "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	matrix := filepath.Join(scriptDir, "base-tools.tsv")
	if err := os.WriteFile(matrix, []byte(baseToolsMatrixHeader+body), 0o644); err != nil {
		t.Fatalf("Matrix anlegen: %v", err)
	}
	return root
}

// fakeBin legt ein PATH-Verzeichnis mit den genannten Programmen an.
func fakeBin(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("Programm %s anlegen: %v", name, err)
		}
	}
	return dir
}

// TestBaseToolsResponseMeldetLuecke: die Karte bekommt je fehlendem Werkzeug
// Name und Rolle sowie genau einen Skriptaufruf zum Kopieren.
func TestBaseToolsResponseMeldetLuecke(t *testing.T) {
	root := newProjectWithBaseMatrix(t,
		"k-da\t-\tIst da\tja\tapt\tpaket-da\t-\t-\t-\n"+
			"k-weg\t-\tVolltextsuche\tnein\tapt,github\tpaket-weg\tbeispiel/weg\tweg\t^{tool}$\n")
	t.Setenv("PATH", fakeBin(t, "k-da"))

	got := buildBaseToolsResponse(root)
	if !got.Available || !got.Present {
		t.Fatalf("Antwort meldet keinen lesbaren Befund: %+v", got)
	}
	if got.OK {
		t.Error("OK ist true, obwohl ein Werkzeug fehlt")
	}
	if len(got.Missing) != 1 || got.Missing[0].Name != "k-weg" {
		t.Fatalf("Missing = %+v, erwartet genau k-weg", got.Missing)
	}
	if got.Missing[0].Role != "Volltextsuche" {
		t.Errorf("Role = %q, erwartet die Rolle aus der Matrix", got.Missing[0].Role)
	}
	// Genau ein Aufruf. Welchen Weg das Skript je Eintrag geht — apt oder
	// user-lokal —, rechnet die Oberfläche nicht aus.
	if !strings.HasSuffix(got.Command, "install-base-tools.sh\" --install") {
		t.Errorf("Command = %q, erwartet einen einzelnen Skriptaufruf", got.Command)
	}
	if strings.Contains(got.Command, "apt-get") || strings.Contains(got.Command, "sudo") {
		t.Errorf("Command = %q: die Oberfläche baut keinen Installationsweg nach", got.Command)
	}
}

// TestBaseToolsResponseSchweigtWennAllesDa: die Karte erscheint nur, wenn etwas
// fehlt. Ohne Lücke bleibt Missing leer, und die Seite blendet sie aus.
func TestBaseToolsResponseSchweigtWennAllesDa(t *testing.T) {
	root := newProjectWithBaseMatrix(t,
		"k-eins\t-\tErstes\tja\tapt\tpaket-eins\t-\t-\t-\n"+
			"k-zwei\t-\tZweites\tja\tapt\tpaket-zwei\t-\t-\t-\n")
	t.Setenv("PATH", fakeBin(t, "k-eins", "k-zwei"))

	got := buildBaseToolsResponse(root)
	if !got.OK || len(got.Missing) != 0 {
		t.Errorf("Antwort meldet eine Lücke, obwohl alles da ist: %+v", got)
	}
}

// TestBaseToolsResponseOhneMatrix: eine fehlende Matrix ist kein Fehler der
// Oberfläche. Present bleibt false, und die Karte zeigt sich nicht — geraten
// wird nicht.
func TestBaseToolsResponseOhneMatrix(t *testing.T) {
	got := buildBaseToolsResponse(t.TempDir())
	if got.Present {
		t.Error("Present ist true, obwohl die Matrix fehlt")
	}
	if got.Message == "" {
		t.Error("der Zustand wird verschwiegen statt gemeldet")
	}
	if len(got.Missing) != 0 {
		t.Errorf("ohne Matrix werden Werkzeuge gemeldet: %+v", got.Missing)
	}
}

// TestBaseToolsKarteWirdAusgeliefert prüft die Oberfläche selbst, nicht nur die
// Antwort: die Route ist registriert, und die Startseite trägt die Karte samt
// Kopierzeile.
//
// Die Karte startet versteckt und macht sich erst sichtbar, wenn der Befund
// eine Lücke meldet — deshalb steht `class="card hidden"` fest im Markup und
// nicht der übliche Installed-Zweig.
func TestBaseToolsKarteWirdAusgeliefert(t *testing.T) {
	handler := routes(&serverState{shutdown: func() {}})

	t.Run("Route antwortet", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/base-tools", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("Status = %d, erwartet 200", recorder.Code)
		}
		var response baseToolsResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("Antwort nicht lesbar: %v", err)
		}
	})

	t.Run("Startseite trägt die Karte", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("Status = %d, erwartet 200", recorder.Code)
		}
		page := recorder.Body.String()
		for _, want := range []string{
			`id="base-tools-card"`,
			`class="card hidden"`,
			`id="base-tools-command-text"`,
			`data-copy="base-tools-command-text"`,
		} {
			if !strings.Contains(page, want) {
				t.Errorf("Startseite enthält %q nicht", want)
			}
		}
	})
}
