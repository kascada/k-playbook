package project

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/health"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
)

// withKephalaionBlock hängt einen Block kephalaion: an die Konfiguration des
// Projekts an.
func withKephalaionBlock(t *testing.T, root, block string) {
	t.Helper()
	write(t, ConfigPath(root), "schema_version: 3\n\nproject:\n  repo_root: .\n  vcs: git\n\n"+block)
}

// Der Block kephalaion: wird zeilenweise gelesen: nur hub und collection als
// Kinder des Blocks, account nur als Vorhandensein; Kommentare,
// Anführungszeichen und tiefere Zeilen stören nicht.
func TestReadConfigKephalaionBlock(t *testing.T) {
	root := newContextProject(t)
	if config, err := ReadConfig(root); err != nil || config.Kephalaion != (ConfigKephalaion{}) {
		t.Fatalf("ohne Block: %+v %v", config.Kephalaion, err)
	}

	withKephalaionBlock(t, root, "# Anbindung\nkephalaion:  # Kommentar\n  # Hub-Alias\n  hub: vm   # am Node\n"+
		"  collection: \"k-playbook-dev\"\n  extra:\n    hub: falsch\n\ndocs:\n  hub: auch-falsch\n")
	config, err := ReadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	want := ConfigKephalaion{Present: true, Hub: "vm", Collection: "k-playbook-dev"}
	if config.Kephalaion != want || config.Kephalaion.Binding() != (kephalaion.Binding{Hub: "vm", Collection: "k-playbook-dev"}) {
		t.Fatalf("Block: %+v", config.Kephalaion)
	}

	withKephalaionBlock(t, root, "kephalaion:\r\n  hub: vm\r\n  collection: omni\r\n  account: kamran-wsl\r\n")
	config, _ = ReadConfig(root)
	if config.Kephalaion != (ConfigKephalaion{Present: true, Hub: "vm", Collection: "omni", AccountSet: true}) {
		t.Fatalf("mit account und CRLF: %+v", config.Kephalaion)
	}
}

// Der Block kephalaion steht in jeder Kontextausgabe und lässt sie nie
// scheitern — auch nicht bei ungültigem Eintrag oder fehlendem Binary.
func TestContextForDirTraegtKephalaion(t *testing.T) {
	kephtest.NoBinary(t)
	root := newContextProject(t)

	built, err := ContextForDir(root)
	if err != nil {
		t.Fatalf("ContextForDir: %v", err)
	}
	if built.Kephalaion == nil || built.Kephalaion.State != kephalaion.StateNotConfigured || built.Kephalaion.Configured ||
		!strings.Contains(built.Kephalaion.Message, ConfigDisplayName) {
		t.Fatalf("ohne Eintrag: %+v", built.Kephalaion)
	}

	withKephalaionBlock(t, root, "kephalaion:\n  hub: vm\n  collection: k-playbook\n")
	built, err = ContextForDir(root)
	if err != nil {
		t.Fatalf("ContextForDir: %v", err)
	}
	if built.Kephalaion.State != kephalaion.StateNoKephalaion || built.Kephalaion.Address != "vm:k-playbook" {
		t.Fatalf("ohne Kephalaion: %+v", built.Kephalaion)
	}

	withKephalaionBlock(t, root, "kephalaion:\n  hub: ../vm\n  collection: k-playbook\n")
	built, err = ContextForDir(root)
	if err != nil {
		t.Fatalf("ungültiger Eintrag bricht den Kontext ab: %v", err)
	}
	if built.Kephalaion.State != kephalaion.StateInvalidEntry {
		t.Fatalf("ungültig: %+v", built.Kephalaion)
	}
}

// BuildContext fragt den Node nicht; das Feld bleibt leer.
func TestBuildContextFragtKephalaionNicht(t *testing.T) {
	called := false
	original := kephalaionCheck
	kephalaionCheck = func(string) kephalaion.Status { called = true; return kephalaion.Status{} }
	t.Cleanup(func() { kephalaionCheck = original })

	built, err := BuildContext(newContextProject(t))
	if err != nil {
		t.Fatal(err)
	}
	if called || built.Kephalaion != nil {
		t.Fatal("BuildContext hat die Anbindung geprüft")
	}
}

// Ein Token im Block kephalaion: — als Wert oder als nackte Zeile — erscheint
// nicht in der Kontextausgabe.
func TestContextForDirZitiertKeinenDateiinhalt(t *testing.T) {
	const token = "keph_CONTEXTLEAKCANARYabcdefghijklmnopqrstuvw1234"
	kephtest.NoBinary(t)
	root := newContextProject(t)
	withKephalaionBlock(t, root, "kephalaion:\n  hub: "+token+"\n  collection: k-playbook\n"+token+"\n  account: "+token+"\n")

	built, err := ContextForDir(root)
	if err != nil {
		t.Fatalf("ContextForDir: %v", err)
	}
	data, err := json.Marshal(built)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "CONTEXTLEAKCANARY") {
		t.Fatalf("Dateiinhalt in der Kontextausgabe: %s", built.Kephalaion.Message)
	}
	if built.Kephalaion.State != kephalaion.StateInvalidEntry {
		t.Fatalf("Zustand: %+v", built.Kephalaion)
	}
}

// Hub und Collection aus K-PLAYBOOK.yaml, Account und Node aus der Auskunft:
// KephalaionStatus meldet ok mit der Herkunft des Accounts. Ein account: im
// Block wird nicht gelesen — gesendet wird der aus der Auskunft —, und die
// Gesundheitsprüfung warnt davor.
func TestKephalaionStatusAusKonfigUndAuskunft(t *testing.T) {
	const token = "keph_GLOBALLEAKCANARYabcdefghijklmnopqrstuvw1234"

	var gotAccount, gotToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var msg struct {
			ID     *int   `json:"id"`
			Method string `json:"method"`
		}
		_ = json.Unmarshal(data, &msg)
		switch msg.Method {
		case "initialize":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2025-06-18","capabilities":{}}}`, *msg.ID)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/call":
			gotAccount, gotToken = r.Header.Get("X-Keph-Account-vm"), r.Header.Get("X-Keph-Token-vm")
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"content":[{"type":"text","text":"x"}],"structuredContent":`+
				`{"version":"v0.4.0","hubs":[{"hub":"vm","login":"ok","node":"vm-node","account":"kamran-vm","user":"kamran",`+
				`"sync":{"revision":1},"collections":[{"address":"vm:k-playbook","collection":"k-playbook","rights":["read","write"]}]}],`+
				`"unknown_hubs":[]}}}`, *msg.ID)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	tokens := kephtest.Token(t, filepath.Join(t.TempDir(), "tokens"), "vm", "kamran-vm", token)
	hub := kephtest.Hub(tokens, "vm", "kamran-vm")
	hub.Choice = kephalaion.ChoiceDefault
	kephtest.NewFakeClient(t).SetInfo(t, kephtest.Info(server.URL+"/mcp", tokens, hub))
	stubKephalaionBinary(t, true)

	root := newContextProject(t)
	withKephalaionBlock(t, root, "kephalaion:\n  hub: vm\n  collection: k-playbook\n  account: fremd\n")

	status := KephalaionStatus(root)
	if status.State != kephalaion.StateOK || status.User != "kamran" || status.Account != "kamran-vm" || status.Choice != kephalaion.ChoiceDefault {
		t.Fatalf("Status: %+v", status)
	}
	if gotAccount != "kamran-vm" || gotToken != token {
		t.Fatalf("Node hat Account %q und ein anderes Token bekommen", gotAccount)
	}
	if data, _ := json.Marshal(status); strings.Contains(string(data), "GLOBALLEAKCANARY") || !strings.Contains(string(data), `"choice":"default"`) {
		t.Fatalf("Ergebnis: %s", data)
	}

	problems := healthKephalaion(context.Background(), root)
	if len(problems) != 1 || problems[0].Level != health.LevelWarning || !strings.Contains(problems[0].Message, "account:") ||
		!strings.Contains(problems[0].Hint, "kephalaion node account default") {
		t.Fatalf("Warnung zu account: %+v", problems)
	}
}

// SetKephalaionBinding legt einen fehlenden Block samt Kommentar hinten an;
// alle übrigen Zeilen bleiben byte-gleich.
func TestSetKephalaionBindingLegtBlockAn(t *testing.T) {
	root := newContextProject(t)
	before := "# Kopf\n\nschema_version: 3\n\nproject:\n  repo_root: .\n  vcs: git\n\n# docs:\n#   exclude:\n#     - docs/\n\ndocs:\n  exclude:\n    - vendor/\n"
	write(t, ConfigPath(root), before)

	result, err := SetKephalaionBinding(root, "vm", "omni")
	if err != nil || !result.Changed || result.Previous.Present {
		t.Fatalf("%+v %v", result, err)
	}
	data := readConfigText(t, root)
	if !strings.HasPrefix(data, before) {
		t.Fatalf("vorhandener Inhalt verändert:\n%s", data)
	}
	if !strings.HasSuffix(data, "\nkephalaion:\n"+kephalaionBlockComment+"  hub: vm\n  collection: omni\n") {
		t.Fatalf("Block:\n%s", data)
	}
	if config, _ := ReadConfig(root); config.Kephalaion.Binding() != (kephalaion.Binding{Hub: "vm", Collection: "omni"}) {
		t.Fatalf("gelesen: %+v", config.Kephalaion)
	}

	// Noch einmal dasselbe: nichts geschrieben, byte-gleich.
	result, err = SetKephalaionBinding(root, "vm", "omni")
	if err != nil || result.Changed || readConfigText(t, root) != data {
		t.Fatalf("zweites Eintragen: %+v %v", result, err)
	}
}

// Ein vorhandener Block: nur hub: und collection: ändern sich, Kommentare
// dahinter, andere Schlüssel, ein account: und die übrigen Blöcke bleiben.
func TestSetKephalaionBindingAendertVorhandenenBlock(t *testing.T) {
	root := newContextProject(t)
	write(t, ConfigPath(root), "schema_version: 3\n\nkephalaion:\n  # eigener Kommentar\n  hub: alt   # Alias\n"+
		"  account: kamran-wsl\n  collection: alt-coll\n\n# Kommentar zum nächsten Block\ndocs:\n  exclude:\n    - x/\n")

	result, err := SetKephalaionBinding(root, "vm", "k-playbook-dev")
	if err != nil || !result.Changed || result.Previous.Hub != "alt" || result.Previous.Collection != "alt-coll" {
		t.Fatalf("%+v %v", result, err)
	}
	want := "schema_version: 3\n\nkephalaion:\n  # eigener Kommentar\n  hub: vm   # Alias\n" +
		"  account: kamran-wsl\n  collection: k-playbook-dev\n\n# Kommentar zum nächsten Block\ndocs:\n  exclude:\n    - x/\n"
	if got := readConfigText(t, root); got != want {
		t.Fatalf("Ergebnis:\n%s\nerwartet:\n%s", got, want)
	}

	// Fehlt eine Zeile im Block, kommt sie hinter die letzte eingerückte.
	write(t, ConfigPath(root), "kephalaion:\r\n  hub: vm\r\n\r\ndocs:\r\n  exclude: []\r\n")
	if _, err := SetKephalaionBinding(root, "vm", "omni"); err != nil {
		t.Fatal(err)
	}
	if got := readConfigText(t, root); got != "kephalaion:\r\n  hub: vm\r\n  collection: omni\r\n\r\ndocs:\r\n  exclude: []\r\n" {
		t.Fatalf("eingefügt: %q", got)
	}
}

// Ungültige Namen und Formen, die sich nicht sicher ändern lassen: nichts
// geschrieben.
func TestSetKephalaionBindingLehntAb(t *testing.T) {
	root := newContextProject(t)
	before := readConfigText(t, root)
	for _, pair := range [][2]string{{"../vm", "omni"}, {"vm", "a:b"}, {"vm", ""}} {
		if _, err := SetKephalaionBinding(root, pair[0], pair[1]); err == nil {
			t.Errorf("%v angenommen", pair)
		}
	}
	if readConfigText(t, root) != before {
		t.Fatal("trotz Ablehnung geschrieben")
	}
	for _, content := range []string{
		"kephalaion: {hub: vm, collection: omni}\n",
		"kephalaion:\n  hub: vm\nkephalaion:\n  collection: omni\n",
		"kephalaion:\n  hub: vm\n  hub: anders\n",
	} {
		write(t, ConfigPath(root), content)
		if _, err := SetKephalaionBinding(root, "vm", "omni"); err == nil || !strings.Contains(err.Error(), "nichts eingetragen") {
			t.Errorf("%q: %v", content, err)
		}
		if readConfigText(t, root) != content {
			t.Errorf("%q: geschrieben", content)
		}
	}
}

func readConfigText(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Liegt die alte K-PLAYBOOK.local.yaml mit Block kephalaion:, warnt die
// Gesundheitsprüfung mit Hinweis auf /k-upgrade; ohne Block nicht.
func TestHealthWarntVorAlterLokalerDatei(t *testing.T) {
	root := healthProject(t)
	legacy := filepath.Join(LocalDir(root), kephalaion.LegacyFileName)
	write(t, legacy, "# nur ein Kommentar\n")
	if problems := healthKephalaion(context.Background(), root); len(problems) != 0 {
		t.Fatalf("ohne Block: %+v", problems)
	}
	write(t, legacy, "kephalaion:\n  hub: vm\n  collection: omni\n  account: kamran-wsl\n")
	problems := healthKephalaion(context.Background(), root)
	if len(problems) != 1 || problems[0].Level != health.LevelWarning || !strings.Contains(problems[0].Message, LegacyKephalaionDisplay) ||
		!strings.Contains(problems[0].Hint, "/k-upgrade") {
		t.Fatalf("Warnung: %+v", problems)
	}
}

// Ohne Auskunft nennt der Umzug den Account der alten Datei trotzdem, samt
// Grund; eine alte Datei ohne gültige Anbindung zieht nicht um.
func TestMigrateKephalaionBindingRandfaelle(t *testing.T) {
	original := kephalaionQuery
	kephalaionQuery = func(context.Context, ...string) (kephalaion.ClientInfo, error) {
		return kephalaion.ClientInfo{}, &kephalaion.ClientError{State: kephalaion.StateNoKephalaion, Message: "Kephalaion fehlt. Abhilfe: installieren."}
	}
	t.Cleanup(func() { kephalaionQuery = original })

	root := newContextProject(t)
	if migration := MigrateKephalaionBinding(root, false); migration.Legacy || migration.Messages() != nil {
		t.Fatalf("ohne alte Datei: %+v", migration)
	}

	legacy := filepath.Join(LocalDir(root), kephalaion.LegacyFileName)
	write(t, legacy, "kephalaion:\n  hub: vm\n  collection: omni\n  account: kamran-wsl\n")
	migration := MigrateKephalaionBinding(root, false)
	if !migration.Written || !strings.Contains(migration.Account, "Kephalaion fehlt") ||
		!strings.Contains(migration.Account, "kephalaion node account default vm kamran-wsl") {
		t.Fatalf("Umzug: %+v", migration)
	}

	other := newContextProject(t)
	before := readConfigText(t, other)
	write(t, filepath.Join(LocalDir(other), kephalaion.LegacyFileName), "kephalaion:\n  hub: ../vm\n  account: kamran-wsl\n")
	migration = MigrateKephalaionBinding(other, false)
	if migration.Written || migration.Error == "" || readConfigText(t, other) != before ||
		!strings.Contains(strings.Join(migration.Messages(), "\n"), "Anbindung nicht umgezogen") {
		t.Fatalf("ungültige alte Datei: %+v", migration)
	}
}
