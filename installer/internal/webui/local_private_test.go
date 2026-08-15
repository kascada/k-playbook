package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// privateProject baut ein Projekt mit Repository und angelegtem priv/ auf und
// macht es zum Arbeitsverzeichnis: die Handler leiten ihr Projekt über
// project.Detect() daraus ab.
func privateProject(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("kein git im Pfad")
	}

	root := newProject(t)
	writeFile(t, project.ConfigPath(root),
		"schema_version: "+project.SchemaVersion+"\n\nproject:\n  repo_root: .\n  vcs: git\n")
	writeFile(t, filepath.Join(project.LocalDir(root), "priv", "README.md"), "# priv\n")
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v — %s", err, output)
	}

	before, err := os.Getwd()
	if err != nil {
		t.Fatalf("Arbeitsverzeichnis: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("nach %s wechseln: %v", root, err)
	}
	t.Cleanup(func() { os.Chdir(before) })
	return root
}

// postPrivate ruft den Handler auf und liefert Status und Antwort.
func postPrivate(t *testing.T, body string) (int, privateResponse) {
	t.Helper()

	recorder := httptest.NewRecorder()
	setLocalPrivateHandler(recorder, httptest.NewRequest(http.MethodPost, "/api/local/private", strings.NewReader(body)))

	var response privateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v — %s", err, recorder.Body.String())
	}
	return recorder.Code, response
}

func TestLocalPrivateHandlerLiefertBeideVerzeichnisse(t *testing.T) {
	privateProject(t)

	recorder := httptest.NewRecorder()
	localPrivateHandler(recorder, httptest.NewRequest(http.MethodGet, "/api/local/private", nil))

	var response privateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v — %s", err, recorder.Body.String())
	}
	if !response.Available || len(response.Entries) != 2 {
		t.Fatalf("Available = %v, Einträge = %d, erwartet 2", response.Available, len(response.Entries))
	}
	if response.Entries[0].Path != "priv" || response.Entries[0].State != project.PrivacyPublic {
		t.Errorf("erster Eintrag = %+v, erwartet priv als %q", response.Entries[0], project.PrivacyPublic)
	}
}

// Der schreibende Weg: einschalten, danach ändert ein zweiter Aufruf nichts.
func TestSetLocalPrivateHandlerSchaltetUmUndIstIdempotent(t *testing.T) {
	root := privateProject(t)

	status, response := postPrivate(t, `{"path":"priv","private":true}`)
	if status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200 — %s", status, response.Message)
	}
	if response.Entries[0].State != project.PrivacyPrivate {
		t.Fatalf("State = %q, erwartet %q — %s", response.Entries[0].State, project.PrivacyPrivate, response.Message)
	}

	ignore := filepath.Join(project.LocalDir(root), "priv", project.PrivateIgnoreFile)
	content := readFile(t, ignore)

	status, response = postPrivate(t, `{"path":"priv","private":true}`)
	if status != http.StatusOK || response.Entries[0].State != project.PrivacyPrivate {
		t.Errorf("zweiter Aufruf: Status = %d, State = %q", status, response.Entries[0].State)
	}
	if readFile(t, ignore) != content {
		t.Error("der zweite Aufruf hat die Datei verändert")
	}
}

// Ein Pfad, für den diese Wahl nicht ansteht, wird abgelehnt — der Handler
// führt schreibende git-Operationen aus.
func TestSetLocalPrivateHandlerLehntFremdePfadeAb(t *testing.T) {
	root := privateProject(t)

	for _, path := range []string{"rules", "..", "../..", "/etc", ""} {
		body, err := json.Marshal(privateRequest{Path: path, Private: true})
		if err != nil {
			t.Fatalf("Anfrage bauen: %v", err)
		}
		status, response := postPrivate(t, string(body))
		if status != http.StatusBadRequest {
			t.Errorf("%q: Status = %d, erwartet 400 — %s", path, status, response.Message)
		}
	}

	if _, err := os.Stat(filepath.Join(project.LocalDir(root), "rules", project.PrivateIgnoreFile)); err == nil {
		t.Error("es wurde eine .gitignore außerhalb der privaten Verzeichnisse geschrieben")
	}
}

func TestSetLocalPrivateHandlerMeldetUnlesbareAnfrage(t *testing.T) {
	privateProject(t)

	if status, _ := postPrivate(t, "kein json"); status != http.StatusBadRequest {
		t.Errorf("Status = %d, erwartet 400", status)
	}
}
