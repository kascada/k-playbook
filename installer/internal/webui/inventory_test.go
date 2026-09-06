package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// newInventoryProject legt ein Projekt mit Anker und einer Quelle an und macht
// es zum Arbeitsverzeichnis: die Handler leiten ihr Projekt über
// project.Detect() daraus ab. Zurück kommt die aufgelöste Projektwurzel — die
// Vertrauensgrenze vergleicht aufgelöste Pfade, und t.TempDir() liegt auf
// manchen Rechnern hinter einem Symlink.
func newInventoryProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "requirements.txt"), []byte("redis==5.0.1\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	chdir(t, root)
	return root
}

func getJSON(t *testing.T, path string, target any) int {
	t.Helper()

	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("Antwort auf %s lesen: %v\n%s", path, err, recorder.Body.String())
	}
	return recorder.Code
}

// postInventory stößt die Erhebung so an, wie die Seite es tut: mit einem
// Origin, der zum Host passt.
func postInventory(t *testing.T) (int, inventoryRunResponse) {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/api/inventory", nil)
	request.Header.Set("Origin", "http://"+request.Host)
	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, request)

	var response inventoryRunResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Antwort lesen: %v\n%s", err, recorder.Body.String())
	}
	return recorder.Code, response
}

func writeVersionSources(t *testing.T, root string, content string) string {
	t.Helper()

	path := filepath.Join(project.LocalDir(root), project.VersionSourcesFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("anlegen: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	return path
}

// Vor dem ersten Lauf: die Datei fehlt, das ist ein definierter Zustand und
// kein Fehler — und eine fehlende Quellenkonfiguration ebenso.
func TestInventarStatusOhneInventarUndOhneKonfiguration(t *testing.T) {
	root := newInventoryProject(t)

	var response inventoryResponse
	if status := getJSON(t, "/api/inventory", &response); status != http.StatusOK {
		t.Fatalf("Status = %d", status)
	}
	if !response.Available {
		t.Fatal("das Projekt ist eingerichtet, die Antwort sagt nein")
	}
	if response.Status.Present || response.Status.Problem != "" {
		t.Errorf("Inventarstatus = %+v, erwartet fehlend ohne Befund", response.Status)
	}
	if response.DisplayPath != "k-playbook-local/docs/versions/inventory.md" {
		t.Errorf("DisplayPath = %q", response.DisplayPath)
	}
	if response.Sources.Present || response.Sources.Error != "" {
		t.Errorf("Quellenkonfiguration = %+v, erwartet fehlend ohne Fehler", response.Sources)
	}
	if response.Sources.Path != filepath.Join(project.LocalDir(root), project.VersionSourcesFileName) {
		t.Errorf("Pfad der Quellenkonfiguration = %q", response.Sources.Path)
	}
	if response.Sources.DisplayPath != "k-playbook-local/version-sources.yaml" {
		t.Errorf("DisplayPath der Quellenkonfiguration = %q", response.Sources.DisplayPath)
	}

	var file inventoryFileResponse
	getJSON(t, "/api/inventory/file", &file)
	if !file.Available || file.Present || file.HTML != "" || file.Message == "" {
		t.Errorf("Datei-Antwort ohne Inventar = %+v", file)
	}
}

// Der Anstoß schreibt die Datei über inventory.Run; der Status danach ist der
// aus dem Frontmatter — dieselbe Auskunft, die GET liefert. Ein zweiter
// Anstoß ohne Änderung lässt die Datei byte-identisch stehen.
func TestInventarAnstossSchreibtUndBleibtDanachByteStabil(t *testing.T) {
	root := newInventoryProject(t)
	path := inventory.FilePath(project.LocalDir(root))

	code, first := postInventory(t)
	if code != http.StatusOK || !first.Available || !first.OK {
		t.Fatalf("erster Anstoß: Status %d, Antwort %+v", code, first)
	}
	if !first.Outcome.Written || first.Outcome.Path != path {
		t.Errorf("Outcome = %+v, erwartet geschrieben nach %s", first.Outcome, path)
	}
	if first.Summary.Sources != 1 || first.Summary.Entries != 1 || first.Summary.Rejected != 0 {
		t.Errorf("Summary = %+v", first.Summary)
	}
	if first.DisplayPath != "k-playbook-local/docs/versions/inventory.md" {
		t.Errorf("DisplayPath = %q", first.DisplayPath)
	}
	if !first.Status.Present || first.Status.GeneratedBy != inventory.GeneratedBy || first.Status.Entries != 1 {
		t.Errorf("Status nach dem Lauf = %+v", first.Status)
	}
	if first.Rejections == nil || first.Exclusions == nil || first.Notes == nil {
		t.Error("Listen müssen als leere Listen kommen, nicht als null")
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Inventar lesen: %v", err)
	}
	// GET sagt dasselbe wie der Lauf.
	var state inventoryResponse
	getJSON(t, "/api/inventory", &state)
	if state.Status != first.Status {
		t.Errorf("GET-Status = %+v, Lauf-Status = %+v", state.Status, first.Status)
	}

	_, second := postInventory(t)
	if !second.OK || second.Outcome.Written {
		t.Fatalf("zweiter Anstoß muss unverändert melden: %+v", second.Outcome)
	}
	if second.Outcome.At != first.Outcome.At {
		t.Errorf("Zeitstempel = %q, erwartet der alte %q", second.Outcome.At, first.Outcome.At)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Inventar lesen: %v", err)
	}
	if string(before) != string(after) {
		t.Error("der zweite Lauf hat die Datei verändert")
	}

	// Die Datei ist im Bereich lesbar — ohne den Frontmatter-Block, der
	// steht als Status daneben.
	var file inventoryFileResponse
	getJSON(t, "/api/inventory/file", &file)
	if !file.Present || file.Message != "" {
		t.Fatalf("Datei-Antwort = %+v", file)
	}
	if !strings.Contains(file.HTML, "Versionsinventar") || !strings.Contains(file.HTML, "redis") {
		t.Errorf("gerenderte Datei ohne Inhalt: %q", file.HTML)
	}
	if strings.Contains(file.HTML, "sources-read") {
		t.Error("das Frontmatter steht im gerenderten Text")
	}
	if file.Path != "k-playbook-local/docs/versions/inventory.md" {
		t.Errorf("Path = %q", file.Path)
	}
}

// Ein Pfad außerhalb der erlaubten Wurzeln wird abgelehnt — von der
// Vertrauensgrenze in internal/inventory, nicht vom Handler. Die Antwort
// trägt die Ablehnung so, wie die Fachlogik sie meldet: angefragter Pfad,
// aufgelöster Pfad, Grund. Der Lauf bricht deswegen nicht ab.
func TestInventarAnstossFuehrtAbgelehntePfadeWieDieFachlogik(t *testing.T) {
	root := newInventoryProject(t)
	outside := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(outside); err == nil {
		outside = resolved
	}
	foreign := filepath.Join(outside, "values.yaml")
	if err := os.WriteFile(foreign, []byte("image: nginx:1.27\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	// Ohne Eintrag unter roots: — ein absoluter Pfad gibt seine Wurzel nicht
	// selbst frei.
	writeVersionSources(t, root, "schema_version: 1\nsources:\n  - path: "+foreign+"\n    kind: helm\n    env: deployment\n")

	code, response := postInventory(t)
	if code != http.StatusOK || !response.OK {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	if len(response.Rejections) != 1 {
		t.Fatalf("Ablehnungen = %+v, erwartet genau eine", response.Rejections)
	}
	if response.Summary.Rejected != 1 || response.Status.Rejected != 1 {
		t.Errorf("Zahl der Ablehnungen: Summary %d, Status %d", response.Summary.Rejected, response.Status.Rejected)
	}

	// Dieselbe Meldung wie auf dem CLI-Weg: dort ruft das Subkommando
	// dieselbe Funktion mit denselben Optionen.
	result, err := inventory.Collect(inventoryOptions(root))
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(result.Rejections) != 1 || result.Rejections[0] != response.Rejections[0] {
		t.Errorf("API-Ablehnung %+v, Fachlogik %+v", response.Rejections[0], result.Rejections)
	}
	if response.Rejections[0].Requested != foreign || response.Rejections[0].Reason == "" {
		t.Errorf("Ablehnung = %+v", response.Rejections[0])
	}

	// Die Ablehnung ist Inhalt der Datei.
	content, err := os.ReadFile(response.Outcome.Path)
	if err != nil {
		t.Fatalf("Inventar lesen: %v", err)
	}
	if !strings.Contains(string(content), foreign) {
		t.Error("die abgelehnte Quelle steht nicht in der Inventardatei")
	}
}

// Eine defekte Quellenkonfiguration bricht den Lauf ab, bevor etwas
// geschrieben wird — und GET zeigt sie als sichtbaren Zustand statt als
// Nullergebnis.
func TestInventarDefekteKonfigurationBrichtAbUndIstSichtbar(t *testing.T) {
	root := newInventoryProject(t)
	writeVersionSources(t, root, "schema_version: 1\nroots: [/srv\n")

	var state inventoryResponse
	getJSON(t, "/api/inventory", &state)
	if !state.Sources.Present || state.Sources.Error == "" {
		t.Errorf("Quellenkonfiguration = %+v, erwartet vorhanden mit Fehler", state.Sources)
	}

	code, response := postInventory(t)
	if code != http.StatusOK || !response.Available || response.OK {
		t.Fatalf("Status %d, Antwort %+v — erwartet Abbruch", code, response)
	}
	if !strings.Contains(response.Message, project.VersionSourcesFileName) {
		t.Errorf("die Meldung nennt die Datei nicht: %q", response.Message)
	}
	if _, err := os.Stat(inventory.FilePath(project.LocalDir(root))); !os.IsNotExist(err) {
		t.Error("trotz Abbruch wurde eine Inventardatei geschrieben")
	}
	if response.Status.Present {
		t.Errorf("Status nach dem Abbruch = %+v", response.Status)
	}
}

// Auch ein Schreibfehler ist ein sichtbarer Abbruch, kein stiller Erfolg: die
// Meldung nennt den Grund, und Outcome.Written sagt, dass die Datei nicht
// angefasst wurde. Provoziert wird er über eine reguläre Datei an der Stelle
// des Zielverzeichnisses.
func TestInventarSchreibfehlerBrichtSichtbarAb(t *testing.T) {
	root := newInventoryProject(t)
	inventoryFile := inventory.FilePath(project.LocalDir(root))
	if err := os.MkdirAll(filepath.Dir(filepath.Dir(inventoryFile)), 0o755); err != nil {
		t.Fatalf("anlegen: %v", err)
	}
	if err := os.WriteFile(filepath.Dir(inventoryFile), []byte("im Weg\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}

	code, response := postInventory(t)
	if code != http.StatusOK || !response.Available || response.OK {
		t.Fatalf("Status %d, Antwort %+v — erwartet Abbruch", code, response)
	}
	if !strings.HasPrefix(response.Message, "Erhebung abgebrochen: ") || !strings.Contains(response.Message, "anlegen") {
		t.Errorf("die Meldung nennt den Schreibfehler nicht: %q", response.Message)
	}
	if response.Outcome.Written {
		t.Error("Outcome.Written trotz Schreibfehler")
	}
	// Der Stand kommt neu von der Datei: dort liegt nichts Lesbares, und das
	// sagt der Status als Problem — kein Erzeuger, keine Einträge.
	if response.Status.GeneratedBy != "" || response.Status.Entries != 0 || response.Status.Problem == "" {
		t.Errorf("Status nach dem Abbruch = %+v", response.Status)
	}
}

// Die konfigurierten Zusatzquellen und Ausschlüsse erscheinen als Zahlen —
// gelesen mit demselben Leser wie im Sammler. Ein Formular gibt es nicht.
func TestInventarZeigtDieQuellenkonfigurationNurAn(t *testing.T) {
	root := newInventoryProject(t)
	writeVersionSources(t, root, "schema_version: 1\nroots:\n  - /srv/deploy\nsources:\n"+
		"  - path: /srv/deploy/values.yaml\n    kind: helm\n    env: deployment\n"+
		"  - path: docs/extra.txt\n    kind: python\n    env: lokal\n    optional: true\n"+
		"exclude:\n  - tests/fixtures/**\n")

	var state inventoryResponse
	getJSON(t, "/api/inventory", &state)
	if !state.Sources.Present || state.Sources.Error != "" {
		t.Fatalf("Quellenkonfiguration = %+v", state.Sources)
	}
	if state.Sources.Roots != 1 || state.Sources.Sources != 2 || state.Sources.Exclude != 1 {
		t.Errorf("Zahlen = %+v, erwartet 1 Wurzel, 2 Quellen, 1 Ausschluss", state.Sources)
	}
}

// Ohne Installation gibt es nichts zu erheben: GET meldet nicht anwendbar,
// POST weist mit 409 ab.
func TestInventarOhneInstallation(t *testing.T) {
	chdir(t, t.TempDir())

	var state inventoryResponse
	getJSON(t, "/api/inventory", &state)
	if state.Available {
		t.Error("ohne Anker darf das Inventar nicht anwendbar sein")
	}

	code, response := postInventory(t)
	if code != http.StatusConflict || response.Available || response.OK {
		t.Errorf("Status %d, Antwort %+v — erwartet 409 und nicht anwendbar", code, response)
	}
}

// Der Anstoß ist ein POST und steht damit hinter der Herkunftsprüfung: eine
// fremde Seite im Browser des Nutzers darf keine Erhebung auslösen.
func TestInventarAnstossFremderHerkunftWirdAbgewiesen(t *testing.T) {
	root := newInventoryProject(t)

	request := httptest.NewRequest(http.MethodPost, "/api/inventory", nil)
	request.Header.Set("Origin", "http://boese.example")
	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("Status = %d, erwartet %d", recorder.Code, http.StatusForbidden)
	}
	if _, err := os.Stat(inventory.FilePath(project.LocalDir(root))); !os.IsNotExist(err) {
		t.Error("die abgewiesene Anfrage hat trotzdem geschrieben")
	}
}

// Die Seite /inventory ist ein eigener Bereich: eigener Eintrag im Umschalter,
// die Karten für Stand, Quellenkonfiguration und Datei, der Knopf zum
// Aktualisieren — und kein Formular für die Quellenkonfiguration.
func TestInventarseiteTraegtDenBereich(t *testing.T) {
	newInventoryProject(t)

	status, body := getPage(t, "/inventory")
	if status != http.StatusOK {
		t.Fatalf("Status = %d", status)
	}
	for _, needle := range []string{
		`<a class="area-nav-item active" href="/inventory" aria-current="page">Inventar</a>`,
		`id="status-card"`, `id="sources-card"`, `id="file-card"`,
		`id="inventory-run"`, `/static/inventory.js`,
	} {
		if !strings.Contains(body, needle) {
			t.Errorf("auf /inventory fehlt %s", needle)
		}
	}
	if strings.Contains(body, "<input") || strings.Contains(body, "<textarea") {
		t.Error("die Seite trägt ein Eingabefeld — die Quellenkonfiguration wird nur angezeigt")
	}
}
