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
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// newInventoryProject legt ein Projekt mit Anker und einer Quelle an und macht
// es zum Arbeitsverzeichnis: die Handler leiten ihr Projekt über
// project.Detect() daraus ab. Das Inventar landet in einer Collection im
// Speicher, die der Test an die Stelle des Zugangs setzt. Zurück kommt die
// aufgelöste Projektwurzel — die Vertrauensgrenze vergleicht aufgelöste Pfade,
// und t.TempDir() liegt auf manchen Rechnern hinter einem Symlink.
func newInventoryProject(t *testing.T) (string, *kephtest.Store) {
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
	return root, inventoryStore(t)
}

// inventoryStore setzt eine Collection im Speicher an die Stelle von
// project.OpenInventoryStore.
func inventoryStore(t *testing.T) *kephtest.Store {
	t.Helper()
	store := kephtest.New()
	original := project.OpenInventoryStore
	realOpenInventoryStore = original
	project.OpenInventoryStore = func(string) (inventory.Store, error) { return store, nil }
	t.Cleanup(func() { project.OpenInventoryStore = original })
	return store
}

// realOpenInventoryStore ist der echte Zugang, bevor ein Test ihn ersetzt hat.
var realOpenInventoryStore func(string) (inventory.Store, error)

// inventoryUnavailable setzt den echten Zugang wieder ein: Das Projekt hat
// keine Anbindung, also scheitert er mit kephalaion_unavailable, ohne den
// Node zu fragen.
func inventoryUnavailable(t *testing.T, real func(string) (inventory.Store, error)) {
	t.Helper()
	original := project.OpenInventoryStore
	project.OpenInventoryStore = real
	t.Cleanup(func() { project.OpenInventoryStore = original })
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

// storedInventory ist das Inventar in der Collection; fehlt es, scheitert der
// Test.
func storedInventory(t *testing.T, store *kephtest.Store) string {
	t.Helper()
	content, ok := store.Content(inventory.DocName)
	if !ok {
		t.Fatalf("%s liegt nicht in der Collection: %v", inventory.DocName, store.Names())
	}
	return content
}

// Vor dem ersten Lauf: das Dokument fehlt, das ist ein definierter Zustand und
// kein Fehler — und eine fehlende Quellenkonfiguration ebenso.
func TestInventarStatusOhneInventarUndOhneKonfiguration(t *testing.T) {
	root, _ := newInventoryProject(t)

	var response inventoryResponse
	if status := getJSON(t, "/api/inventory", &response); status != http.StatusOK {
		t.Fatalf("Status = %d", status)
	}
	if !response.Available {
		t.Fatal("das Projekt ist eingerichtet, die Antwort sagt nein")
	}
	if response.Status.Present || response.Status.Unavailable || response.Status.Problem != "" {
		t.Errorf("Inventarstatus = %+v, erwartet fehlend ohne Befund", response.Status)
	}
	if response.DisplayPath != inventory.DocRef || response.Status.Path != inventory.DocRef {
		t.Errorf("DisplayPath = %q, Status.Path = %q", response.DisplayPath, response.Status.Path)
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
	if !file.Available || file.Present || file.HTML != "" || file.Message == "" || file.Path != inventory.DocRef {
		t.Errorf("Antwort ohne Inventar = %+v", file)
	}
}

// Der Anstoß schreibt das Dokument über inventory.Run in die Collection; der
// Status danach ist der aus dem Frontmatter — dieselbe Auskunft, die GET
// liefert. Ein zweiter Anstoß ohne Änderung lässt das Dokument byte-identisch
// stehen, Revision eingeschlossen.
func TestInventarAnstossSchreibtUndBleibtDanachByteStabil(t *testing.T) {
	_, store := newInventoryProject(t)

	code, first := postInventory(t)
	if code != http.StatusOK || !first.Available || !first.OK {
		t.Fatalf("erster Anstoß: Status %d, Antwort %+v", code, first)
	}
	if !first.Outcome.Written || first.Outcome.Path != inventory.DocRef || first.Outcome.Revision == 0 {
		t.Errorf("Outcome = %+v, erwartet geschrieben nach %s", first.Outcome, inventory.DocRef)
	}
	if first.Summary.Sources != 1 || first.Summary.Entries != 1 || first.Summary.Rejected != 0 {
		t.Errorf("Summary = %+v", first.Summary)
	}
	if first.DisplayPath != inventory.DocRef {
		t.Errorf("DisplayPath = %q", first.DisplayPath)
	}
	if !first.Status.Present || first.Status.GeneratedBy != inventory.GeneratedBy || first.Status.Entries != 1 ||
		first.Status.Revision != first.Outcome.Revision {
		t.Errorf("Status nach dem Lauf = %+v", first.Status)
	}
	if first.Rejections == nil || first.Exclusions == nil || first.Notes == nil {
		t.Error("Listen müssen als leere Listen kommen, nicht als null")
	}
	if names := store.Names(); len(names) != 1 || names[0] != inventory.DocName {
		t.Errorf("in der Collection liegt %v, erwartet nur %s", names, inventory.DocName)
	}

	before := storedInventory(t, store)
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
	if second.Outcome.At != first.Outcome.At || second.Outcome.Revision != first.Outcome.Revision {
		t.Errorf("Outcome = %+v, erwartet den alten Zeitstempel %q und Revision %d",
			second.Outcome, first.Outcome.At, first.Outcome.Revision)
	}
	if after := storedInventory(t, store); before != after {
		t.Error("der zweite Lauf hat das Dokument verändert")
	}

	// Das Dokument ist im Bereich lesbar — ohne den Frontmatter-Block, der
	// steht als Status daneben.
	var file inventoryFileResponse
	getJSON(t, "/api/inventory/file", &file)
	if !file.Present || file.Message != "" {
		t.Fatalf("Antwort = %+v", file)
	}
	if !strings.Contains(file.HTML, "Versionsinventar") || !strings.Contains(file.HTML, "redis") {
		t.Errorf("gerendertes Dokument ohne Inhalt: %q", file.HTML)
	}
	if strings.Contains(file.HTML, "sources-read") {
		t.Error("das Frontmatter steht im gerenderten Text")
	}
	if file.Path != inventory.DocRef {
		t.Errorf("Path = %q", file.Path)
	}
}

// Ein Pfad außerhalb der erlaubten Wurzeln wird abgelehnt — von der
// Vertrauensgrenze in internal/inventory, nicht vom Handler. Die Antwort
// trägt die Ablehnung so, wie die Fachlogik sie meldet: angefragter Pfad,
// aufgelöster Pfad, Grund. Der Lauf bricht deswegen nicht ab.
func TestInventarAnstossFuehrtAbgelehntePfadeWieDieFachlogik(t *testing.T) {
	root, store := newInventoryProject(t)
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

	// Die Ablehnung ist Inhalt des Dokuments.
	if !strings.Contains(storedInventory(t, store), foreign) {
		t.Error("die abgelehnte Quelle steht nicht im Inventar")
	}
}

// Eine defekte Quellenkonfiguration bricht den Lauf ab, bevor etwas
// geschrieben wird — und GET zeigt sie als sichtbaren Zustand statt als
// Nullergebnis.
func TestInventarDefekteKonfigurationBrichtAbUndIstSichtbar(t *testing.T) {
	root, store := newInventoryProject(t)
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
	if len(store.Names()) != 0 {
		t.Errorf("trotz Abbruch wurde geschrieben: %v", store.Names())
	}
	if response.Status.Present || response.Status.Unavailable {
		t.Errorf("Status nach dem Abbruch = %+v", response.Status)
	}
}

// Auch ein Schreibfehler ist ein sichtbarer Abbruch, kein stiller Erfolg: die
// Meldung nennt den Grund und den Code, und Outcome.Written sagt, dass das
// Dokument nicht angefasst wurde. Provoziert wird er über einen Hub, der den
// write mit stale_revision ablehnt.
func TestInventarSchreibfehlerBrichtSichtbarAb(t *testing.T) {
	_, store := newInventoryProject(t)
	store.Put(inventory.DocName, inventory.Render(inventory.Result{}, "2026-09-01T08:00:00Z"))
	store.Fail["write"] = &kephalaion.ToolError{Tool: "write", Code: kephalaion.CodeStaleRevision, Message: "aktuelle Revision 7"}

	code, response := postInventory(t)
	if code != http.StatusOK || !response.Available || response.OK {
		t.Fatalf("Status %d, Antwort %+v — erwartet Abbruch", code, response)
	}
	if !strings.HasPrefix(response.Message, "Erhebung abgebrochen: ") || !strings.Contains(response.Message, "(stale_revision)") {
		t.Errorf("die Meldung nennt den Schreibfehler nicht mit Code: %q", response.Message)
	}
	if response.Outcome.Written {
		t.Error("Outcome.Written trotz Schreibfehler")
	}
	// Der Stand kommt neu aus der Collection: dort liegt der alte Bestand,
	// unverändert.
	if !response.Status.Present || response.Status.GeneratedAt != "2026-09-01T08:00:00Z" || response.Status.Problem != "" {
		t.Errorf("Status nach dem Abbruch = %+v", response.Status)
	}
}

// Ohne bereite Kephalaion-Anbindung gibt es kein Inventar: GET zeigt den
// Zustand „nicht erreichbar“ mit dem Grund — nicht „fehlt“ —, der Anstoß
// bricht mit kephalaion_unavailable ab, ohne eine Quelle zu lesen, und die
// Ansicht sagt, warum nichts da ist. Einen Rückfall auf die Platte gibt es
// nicht.
func TestInventarOhneKephalaionIstEinSichtbarerZustand(t *testing.T) {
	root, _ := newInventoryProject(t)
	inventoryUnavailable(t, realOpenInventoryStore)

	var state inventoryResponse
	getJSON(t, "/api/inventory", &state)
	if !state.Available || !state.Status.Unavailable || state.Status.Present {
		t.Fatalf("Status = %+v, erwartet nicht erreichbar", state.Status)
	}
	if !strings.Contains(state.Status.Message, "nicht bereit") || state.Status.Path != inventory.DocRef {
		t.Errorf("Status = %+v", state.Status)
	}

	code, response := postInventory(t)
	if code != http.StatusOK || !response.Available || response.OK {
		t.Fatalf("Status %d, Antwort %+v — erwartet Abbruch", code, response)
	}
	if !strings.Contains(response.Message, "(kephalaion_unavailable)") || !strings.Contains(response.Message, "nicht bereit") {
		t.Errorf("die Meldung nennt Code und Grund nicht: %q", response.Message)
	}
	if response.Outcome.Written || response.Outcome.Path != inventory.DocRef || !response.Status.Unavailable {
		t.Errorf("Outcome = %+v, Status = %+v", response.Outcome, response.Status)
	}
	if _, err := os.Stat(filepath.Join(project.LocalDir(root), "docs")); !os.IsNotExist(err) {
		t.Error("ohne Kephalaion darf nichts auf der Platte entstehen")
	}

	var file inventoryFileResponse
	getJSON(t, "/api/inventory/file", &file)
	if !file.Available || file.Present || !strings.Contains(file.Message, "kephalaion_unavailable") {
		t.Errorf("Antwort = %+v", file)
	}
}

// Ist der Node da, antwortet aber nicht, ist das ebenfalls „nicht erreichbar“
// — gemeldet mit dem Fehler des Stores.
func TestInventarStatusBeiNichtAntwortendemNode(t *testing.T) {
	_, store := newInventoryProject(t)
	store.Fail["read"] = &kephalaion.ToolError{Tool: "read", Code: kephalaion.CodeNodeUnreachable, Message: "Node antwortet nicht"}

	var state inventoryResponse
	getJSON(t, "/api/inventory", &state)
	if !state.Status.Unavailable || !strings.Contains(state.Status.Message, "Node antwortet nicht") {
		t.Errorf("Status = %+v", state.Status)
	}
}

// Die konfigurierten Zusatzquellen und Ausschlüsse erscheinen als Zahlen —
// gelesen mit demselben Leser wie im Sammler. Ein Formular gibt es nicht.
func TestInventarZeigtDieQuellenkonfigurationNurAn(t *testing.T) {
	root, _ := newInventoryProject(t)
	writeVersionSources(t, root, "schema_version: 1\nroots:\n  - /srv/deploy\nsources:\n"+
		"  - path: /srv/deploy/values.yaml\n    kind: helm\n    env: deployment\n"+
		"  - path: docs/extra.txt\n    kind: python\n    env: lokal\n    optional: true\n"+
		"exclude:\n  - tests/fixtures/**\n"+
		"helm_values:\n  - path: helm/values.yaml\n    key: redis.standalone.tag\n    item: container/redis\n")

	var state inventoryResponse
	getJSON(t, "/api/inventory", &state)
	if !state.Sources.Present || state.Sources.Error != "" {
		t.Fatalf("Quellenkonfiguration = %+v", state.Sources)
	}
	if state.Sources.Roots != 1 || state.Sources.Sources != 2 || state.Sources.Exclude != 1 || state.Sources.HelmValues != 1 {
		t.Errorf("Zahlen = %+v, erwartet 1 Wurzel, 2 Quellen, 1 Ausschluss, 1 Helm-Wert", state.Sources)
	}
}

// Ohne Installation gibt es nichts zu erheben: GET meldet nicht anwendbar,
// POST weist mit 409 ab.
func TestInventarOhneInstallation(t *testing.T) {
	chdir(t, t.TempDir())
	inventoryStore(t)

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
	_, store := newInventoryProject(t)

	request := httptest.NewRequest(http.MethodPost, "/api/inventory", nil)
	request.Header.Set("Origin", "http://boese.example")
	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("Status = %d, erwartet %d", recorder.Code, http.StatusForbidden)
	}
	if len(store.Calls) != 0 || len(store.Names()) != 0 {
		t.Error("die abgewiesene Anfrage hat trotzdem die Collection angefasst")
	}
}

// Die Seite /inventory ist ein eigener Bereich: eigener Eintrag im Umschalter,
// die Karten für Stand, Quellenkonfiguration und Dokument, der Knopf zum
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
		`docs/generated/versions/`,
	} {
		if !strings.Contains(body, needle) {
			t.Errorf("auf /inventory fehlt %s", needle)
		}
	}
	if strings.Contains(body, "<input") || strings.Contains(body, "<textarea") {
		t.Error("die Seite trägt ein Eingabefeld — die Quellenkonfiguration wird nur angezeigt")
	}
}

// Eine nicht auswertbare Quelle steht in der Antwort mit Datei und Quellart,
// und Lauf, Frontmatter und Fachlogik nennen dieselbe Zahl.
func TestInventarAnstossFuehrtNichtAuswertbareQuellen(t *testing.T) {
	root, store := newInventoryProject(t)
	if err := os.WriteFile(filepath.Join(root, "docker-compose.yml"), []byte("services: [\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}

	code, response := postInventory(t)
	if code != http.StatusOK || !response.OK {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	if len(response.Unevaluable) != 1 || response.Unevaluable[0].File != "docker-compose.yml" || !response.Unevaluable[0].Unevaluable {
		t.Fatalf("nicht auswertbare Quellen = %+v", response.Unevaluable)
	}

	result, err := inventory.Collect(inventoryOptions(root))
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	want := len(result.UnevaluableSources())
	if want != 1 || response.Summary.Unevaluable != want || response.Status.SourcesUnevaluable != want {
		t.Errorf("Fachlogik %d, Summary %d, Status %d — erwartet überall 1",
			want, response.Summary.Unevaluable, response.Status.SourcesUnevaluable)
	}

	if !strings.Contains(storedInventory(t, store), "| `docker-compose.yml` | compose | dev | 0 | nicht auswertbar |") {
		t.Errorf("die Quellentabelle zeigt den Zustand nicht:\n%s", storedInventory(t, store))
	}
}
