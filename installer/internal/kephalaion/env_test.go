package kephalaion

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testToken hat die Form eines Kephalaion-Tokens und ist leicht wiederzufinden:
// Taucht diese Zeichenfolge in einem Ergebnis, einem Fehler oder einer Antwort
// auf, ist das Token durchgesickert.
const testToken = "keph_TOKENLEAKCANARYabcdefghijklmnopqrstuvw12345"

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTestTokenHasTokenForm(t *testing.T) {
	if len(testToken) != tokenLength {
		t.Fatalf("Testtoken hat %d Zeichen, erwartet %d", len(testToken), tokenLength)
	}
}

func TestReadBindingStates(t *testing.T) {
	dir := t.TempDir()
	if _, configured, err := ReadBinding(dir); configured || err != nil {
		t.Fatalf("fehlende Datei: configured=%v err=%v", configured, err)
	}

	writeFile(t, LocalFilePath(dir), "# nur ein Kommentar\nother:\n  key: value\n")
	if _, configured, err := ReadBinding(dir); configured || err != nil {
		t.Fatalf("fehlender Block: configured=%v err=%v", configured, err)
	}

	writeFile(t, LocalFilePath(dir), "# Kopf\n\nkephalaion:\n  # Hub\n  hub: vm\n  collection: k-playbook\n  account: kamran-wsl\n")
	b, configured, err := ReadBinding(dir)
	if err != nil || !configured {
		t.Fatalf("Block: configured=%v err=%v", configured, err)
	}
	if b != (Binding{Hub: "vm", Collection: "k-playbook", Account: "kamran-wsl"}) || b.Address() != "vm:k-playbook" {
		t.Fatalf("gelesen: %+v", b)
	}

	writeFile(t, LocalFilePath(dir), "kephalaion:\n  hub: [unbalanciert\n")
	if _, _, err := ReadBinding(dir); err == nil {
		t.Fatal("kaputtes YAML: Fehler erwartet")
	}
}

func TestValidateBindingRejectsNamesOutsideTheRule(t *testing.T) {
	good := Binding{Hub: "vm", Collection: "k-playbook", Account: "kamran-wsl"}
	if err := ValidateBinding(good); err != nil {
		t.Fatalf("gültig abgewiesen: %v", err)
	}
	cases := []Binding{
		{Hub: "../vm", Collection: "c", Account: "a"},
		{Hub: "vm/x", Collection: "c", Account: "a"},
		{Hub: "..", Collection: "c", Account: "a"},
		{Hub: "VM", Collection: "c", Account: "a"},
		{Hub: "vm", Collection: "a:b", Account: "a"},
		{Hub: "vm", Collection: "c", Account: "../../etc/passwd"},
		{Hub: "vm", Collection: "c", Account: "admin"},
		{Hub: "vm", Collection: "system-x", Account: "a"},
		{Hub: "vm", Collection: "c", Account: ""},
		{Hub: "vm", Collection: "c", Account: testToken},
	}
	for _, c := range cases {
		err := ValidateBinding(c)
		if err == nil {
			t.Errorf("%+v: Fehler erwartet", c)
			continue
		}
		if strings.Contains(err.Error(), testToken) {
			t.Errorf("Fehler zitiert einen unzulässigen Wert: %v", err)
		}
	}
}

func TestLoopbackEndpoint(t *testing.T) {
	cases := []struct {
		listen string
		want   string
	}{
		{"127.0.0.1:7433", "http://127.0.0.1:7433/mcp"},
		{"localhost:7433", "http://127.0.0.1:7433/mcp"},
		{"0.0.0.0:7433", "http://127.0.0.1:7433/mcp"},
		{"[::]:7433", "http://127.0.0.1:7433/mcp"},
		{":7433", "http://127.0.0.1:7433/mcp"},
		{"[::1]:7433", "http://[::1]:7433/mcp"},
		{"192.168.1.5:7433", ""},
		{"example.org:7433", ""},
		{"127.0.0.2:7433", ""},
		{"127.0.0.1", ""},
		{"127.0.0.1:0", ""},
		{"127.0.0.1:x", ""},
	}
	for _, c := range cases {
		got, problem := loopbackEndpoint(c.listen)
		if got != c.want {
			t.Errorf("%q: %q, erwartet %q", c.listen, got, c.want)
		}
		if (got == "") != (problem != "") {
			t.Errorf("%q: Endpunkt %q und Problem %q passen nicht zusammen", c.listen, got, problem)
		}
	}
}

func TestLoadEnv(t *testing.T) {
	dir := t.TempDir()
	env := LoadEnv(dir)
	if env.ConfigPresent || env.Endpoint != "" {
		t.Fatalf("ohne config: %+v", env)
	}

	writeFile(t, filepath.Join(dir, "config.yaml"), "node:\n  db: sqlite:///x\n  listen: 0.0.0.0:7499\n")
	if env := LoadEnv(dir); env.Endpoint != "http://127.0.0.1:7499/mcp" {
		t.Fatalf("Wildcard: %+v", env)
	}

	writeFile(t, filepath.Join(dir, "config.yaml"), "node:\n  db: sqlite:///x\n  listen: 10.0.0.1:7433\n")
	if env := LoadEnv(dir); env.Endpoint != "" || env.AddressProblem == "" {
		t.Fatalf("fremde Adresse: %+v", env)
	}

	// Ohne listen gilt, was Kephalaion selbst annimmt: 127.0.0.1:7433.
	writeFile(t, filepath.Join(dir, "config.yaml"), "node:\n  db: sqlite:///x\n")
	if env := LoadEnv(dir); env.Endpoint != "http://127.0.0.1:7433/mcp" {
		t.Fatalf("ohne listen: %+v", env)
	}

	// Ohne Abschnitt node: ist kein Node eingerichtet — kein Endpunkt.
	writeFile(t, filepath.Join(dir, "config.yaml"), "hub:\n  db: sqlite:///x\n")
	if env := LoadEnv(dir); env.Endpoint != "" || env.AddressProblem == "" || !env.ConfigPresent {
		t.Fatalf("ohne node: %+v", env)
	}
}

func TestDefaultConfigDirHonoursXDG(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	got, err := DefaultConfigDir()
	if err != nil || got != filepath.Join(xdg, "kephalaion") {
		t.Fatalf("%q %v", got, err)
	}
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "relativ")
	t.Setenv("HOME", home)
	got, _ = DefaultConfigDir()
	if got != filepath.Join(home, ".config", "kephalaion") {
		t.Fatalf("relatives XDG: %q", got)
	}
}

func TestCandidates(t *testing.T) {
	dir := t.TempDir()
	tokens := filepath.Join(dir, "tokens")
	writeFile(t, filepath.Join(tokens, "vm", "kamran-wsl.token"), testToken+"\n")
	writeFile(t, filepath.Join(tokens, "vm", "other.token.pending"), "x")
	writeFile(t, filepath.Join(tokens, "vm", "neu.pending"), "x")
	writeFile(t, filepath.Join(tokens, "vm", "notes.txt"), "x")
	writeFile(t, filepath.Join(tokens, "vm", "Bad Name.token"), "x")
	writeFile(t, filepath.Join(tokens, "privat", "alice.token"), testToken)
	writeFile(t, filepath.Join(tokens, "Upper", "bob.token"), testToken)
	if err := os.Symlink(filepath.Join(tokens, "vm", "kamran-wsl.token"), filepath.Join(tokens, "vm", "link.token")); err != nil {
		t.Fatal(err)
	}

	env := Env{ConfigDir: dir}
	candidates, pending := env.Candidates()
	var got []string
	for _, c := range candidates {
		got = append(got, c.Hub+"/"+c.Account)
	}
	if strings.Join(got, ",") != "privat/alice,vm/kamran-wsl" {
		t.Fatalf("Kandidaten: %v", got)
	}
	if len(pending) != 2 || pending[0].Account != "neu" || pending[1].Account != "other" {
		t.Fatalf("pending: %+v", pending)
	}

	if _, ok := env.FindCandidate("vm", "kamran-wsl"); !ok {
		t.Fatal("vorhandenes Paar nicht gefunden")
	}
	for _, pair := range [][2]string{
		{"vm", "other"},            // nur .pending
		{"privat", "kamran-wsl"},   // Paar ohne Kandidat
		{"vm", "../vm/kamran-wsl"}, // Pfadteil im Namen
		{"..", "kamran-wsl"},
		{"vm/../vm", "kamran-wsl"},
		{"vm", "link"}, // Symlink
	} {
		if _, ok := env.FindCandidate(pair[0], pair[1]); ok {
			t.Errorf("%v: kein Kandidat erwartet", pair)
		}
	}
}

func TestReadTokenChecksForm(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.token")
	writeFile(t, good, testToken+"\n")
	if token, err := (Candidate{path: good}).readToken(); err != nil || token != testToken {
		t.Fatalf("gültig: err=%v", err)
	}
	for _, content := range []string{"", "keph_kurz", testToken + "\nX-Evil: 1", "abcd" + testToken[4:], strings.Replace(testToken, "a", "\r", 1)} {
		path := filepath.Join(dir, "bad.token")
		writeFile(t, path, content)
		_, err := (Candidate{path: path}).readToken()
		if err == nil {
			t.Errorf("%q: Fehler erwartet", content)
			continue
		}
		if strings.Contains(err.Error(), "keph_") {
			t.Errorf("Fehler nennt den Inhalt: %v", err)
		}
	}
}

// TestFileContentNeverLeaks schreibt ein erkennbares Token als Zeilentext in
// K-PLAYBOOK.local.yaml und in die config von Kephalaion — als nackte Zeile,
// eingerückt, in einem Flow-Wert und als node.listen. Fehler, Zustand und
// Meldung nennen höchstens Zeilennummer und Art des Fehlers, nie den Inhalt.
func TestFileContentNeverLeaks(t *testing.T) {
	leaked := func(text string) bool {
		return strings.Contains(text, testToken) || strings.Contains(text, "TOKENLEAKCANARY")
	}

	locals := []string{
		testToken + "\n",
		"kephalaion:\n  hub: vm\n" + testToken + "\n  account: kamran-wsl\n",
		"# Kopf\nkephalaion:\n  hub: vm\n  collection: k-playbook\n  account: kamran-wsl\n\n" + testToken + "\n",
		"kephalaion:\n  hub: [" + testToken + "\n",
		"kephalaion: {" + testToken + "}\n",
		"kephalaion: [a, b] " + testToken + "\n",
	}
	env := Env{ConfigDir: t.TempDir()}
	for i, content := range locals {
		local := t.TempDir()
		writeFile(t, LocalFilePath(local), content)
		_, _, err := ReadBinding(local)
		if err == nil {
			t.Errorf("Datei %d: Fehler erwartet", i)
			continue
		}
		if leaked(err.Error()) || !strings.Contains(err.Error(), "Zeile ") {
			t.Errorf("Datei %d: Fehler %q", i, err)
		}
		status := Check(t.Context(), local, env)
		data, _ := json.Marshal(status)
		if status.State != StateInvalidEntry || leaked(string(data)) {
			t.Errorf("Datei %d: Zustand %s", i, data)
		}
	}

	configs := []string{
		testToken + "\n",
		"node:\n  listen: 127.0.0.1:7433\n" + testToken + "\n",
		"node:\n  listen: [" + testToken + "\n",
		"node:\n  listen: " + testToken + "\n",
		"node:\n  listen: " + testToken + ":7433\n",
		"node:\n  listen: 127.0.0.1:" + testToken + "\n",
	}
	for i, content := range configs {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "config.yaml"), content)
		env := LoadEnv(dir)
		if env.ConfigError == "" && env.AddressProblem == "" {
			t.Errorf("config %d: Problem erwartet: %+v", i, env)
		}
		if leaked(env.ConfigError) || leaked(env.AddressProblem) {
			t.Errorf("config %d: %+v", i, env)
		}
		status := CheckBinding(t.Context(), goodBinding, env)
		data, _ := json.Marshal(status)
		if leaked(string(data)) {
			t.Errorf("config %d: Zustand %s", i, data)
		}
	}

	// Eine gewöhnliche fremde Adresse bleibt zitiert — sie hilft beim Finden.
	if _, problem := loopbackEndpoint("10.0.0.1:7433"); !strings.Contains(problem, `"10.0.0.1:7433"`) {
		t.Errorf("Adresse nicht zitiert: %q", problem)
	}
}
