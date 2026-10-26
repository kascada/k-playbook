package githubaccess

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolate hält jeden Test von der echten Datei des Rechners fern: Override
// auf ein leeres Verzeichnis, eigenes HOME für die Auflösung von ~.
func isolate(t *testing.T) (dir string, home string) {
	t.Helper()
	dir, home = t.TempDir(), t.TempDir()
	t.Setenv(EnvConfig, filepath.Join(dir, FileName))
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	return dir, home
}

func testdata(name string) string {
	return filepath.Join("testdata", name)
}

// Die gültige Datei: alle vier Typen, disabled, Deploy Key über host, Pfade
// mit ~ aufgelöst, Dienste und Zugänge in Dateireihenfolge, keine Probleme.
func TestLoadLiestGueltigeDatei(t *testing.T) {
	_, home := isolate(t)

	result := Load(testdata("valid.yaml"))
	if len(result.Problems) != 0 || result.Created {
		t.Fatalf("Probleme: %+v", result.Problems)
	}
	if got := result.Services.Names(); strings.Join(got, ",") != "website,backend,playbook" {
		t.Errorf("Reihenfolge: %v", got)
	}
	backend, ok := result.Services.Lookup("backend")
	if !ok || backend.Repo != "acme-labs/backend" || len(backend.Access) != 2 {
		t.Fatalf("backend: %+v", backend)
	}
	if backend.Access[0] != (Access{Type: "ssh", Key: filepath.Join(home, ".ssh", "id_ed25519_acme_labs")}) {
		t.Errorf("backend.access[0]: %+v", backend.Access[0])
	}
	if backend.Access[1] != (Access{Type: "token", File: filepath.Join(home, ".config", "github-tokens", "acme-labs.token"), Disabled: true}) {
		t.Errorf("backend.access[1]: %+v", backend.Access[1])
	}
	playbook, _ := result.Services.Lookup("playbook")
	if len(playbook.Access) != 2 || playbook.Access[0] != (Access{Type: "ssh", Host: "github-example"}) || playbook.Access[1] != (Access{Type: "public"}) {
		t.Errorf("playbook: %+v", playbook.Access)
	}
	website, _ := result.Services.Lookup("website")
	if website.Access[0] != (Access{Type: "gh", Login: "alice"}) {
		t.Errorf("website: %+v", website.Access)
	}
}

// Eine leere Datei — nur Kommentare oder services: {} — ist kein Problem.
func TestLoadLeereDateiIstKeinProblem(t *testing.T) {
	isolate(t)
	for _, name := range []string{"empty.yaml", "empty-mapping.yaml"} {
		result := Load(testdata(name))
		if len(result.Problems) != 0 || len(result.Services) != 0 || result.Services == nil || result.Created {
			t.Errorf("%s: %+v", name, result)
		}
	}
}

// Jede Problemart aus dem Vertrag einzeln, mit dem erwarteten Pfad und einem
// Stück des Satzes. Nach einem error ist Services leer.
func TestLoadMeldetJedeProblemart(t *testing.T) {
	isolate(t)
	cases := []struct {
		file    string
		path    string
		message string
	}{
		{"not-yaml.yaml", "file", "kein YAML, das k-playbook liest (Zeile"},
		{"top-level-key.yaml", "file", "Der Schlüssel accounts auf oberster Ebene"},
		{"services-not-mapping.yaml", "file", "services ist keine Abbildung"},
		{"bad-name.yaml", "services.web site", "Namensregel"},
		{"service-not-mapping.yaml", "services.website", "keine Abbildung mit repo und access"},
		{"no-repo.yaml", "services.website", "Dem Dienst fehlt repo."},
		{"bad-repo.yaml", "services.website", "repo ist nicht in der Form <owner>/<repo>."},
		{"no-access.yaml", "services.website", "Dem Dienst fehlt access."},
		{"empty-access.yaml", "services.website", "access ist leer"},
		{"access-not-list.yaml", "services.website", "access ist keine Liste"},
		{"unknown-key.yaml", "services.website", "Der Schlüssel rights gehört nicht zu einem Dienst"},
		{"access-not-mapping.yaml", "services.website.access[0]", "Der Zugang ist keine Abbildung mit type."},
		{"no-type.yaml", "services.website.access[0]", "Dem Zugang fehlt type."},
		{"unknown-type.yaml", "services.website.access[0]", "type oauth ist unbekannt"},
		{"missing-required.yaml", "services.website.access[1]", "Dem Zugang vom Typ gh fehlt login."},
		{"foreign-key.yaml", "services.website.access[0]", "Der Schlüssel file gehört nicht zu einem Zugang vom Typ gh."},
		{"key-and-host.yaml", "services.backend.access[0]", "nennt key und host"},
		{"ssh-without-key-or-host.yaml", "services.backend.access[0]", "fehlt key oder host"},
		{"relative-path.yaml", "services.backend.access[1]", "Der Pfad in file ist relativ."},
		{"bad-disabled.yaml", "services.website.access[0]", "disabled hat einen anderen Wert als true oder false."},
		{"duplicate-service.yaml", "services.website", "mehr als einmal eingetragen"},
	}
	for _, tc := range cases {
		result := Load(testdata(tc.file))
		if len(result.Problems) != 1 {
			t.Errorf("%s: erwartet genau ein Problem, bekommen %+v", tc.file, result.Problems)
			continue
		}
		got := result.Problems[0]
		if got.Level != LevelError || got.Path != tc.path || !strings.Contains(got.Message, tc.message) || got.Hint == "" {
			t.Errorf("%s: %+v", tc.file, got)
		}
		if result.Services != nil || result.Created {
			t.Errorf("%s: Services trotz error: %+v", tc.file, result.Services)
		}
	}
}

// Der Fehlertext von yamllite zitiert die Zeile; die Meldung nennt nur die
// Zeilennummer. Die Datei ist eine fremde.
func TestLoadZitiertKeineZeileBeiYAMLFehler(t *testing.T) {
	isolate(t)
	result := Load(testdata("not-yaml.yaml"))
	if len(result.Problems) != 1 || strings.Contains(result.Problems[0].Message, "access") {
		t.Errorf("Meldung zitiert die Datei: %+v", result.Problems)
	}
}

// Alle Probleme, sortiert nach Position in der Datei — nicht nur das erste,
// nicht nach Pfad.
func TestLoadLiefertAlleProblemeInDateireihenfolge(t *testing.T) {
	isolate(t)
	result := Load(testdata("sorted.yaml"))
	var paths []string
	for _, problem := range result.Problems {
		paths = append(paths, problem.Path)
	}
	want := "services.zeta.access[0],services.alpha,services.alpha.access[0],file"
	if strings.Join(paths, ",") != want {
		t.Errorf("Reihenfolge: %v\n%+v", paths, result.Problems)
	}
	if result.Services != nil {
		t.Errorf("Services trotz error: %+v", result.Services)
	}
}

// Eine Datei, die nicht lesbar ist — ein Verzeichnis, keine Leserechte — ist
// ein Problem der Datei, kein Absturz.
func TestLoadMeldetUnlesbareDatei(t *testing.T) {
	dir, _ := isolate(t)
	asDir := filepath.Join(dir, "verzeichnis")
	if err := os.Mkdir(asDir, 0o700); err != nil {
		t.Fatal(err)
	}
	result := Load(asDir)
	if len(result.Problems) != 1 || result.Problems[0].Path != "file" || !strings.Contains(result.Problems[0].Message, "Verzeichnis") {
		t.Errorf("Verzeichnis: %+v", result.Problems)
	}

	if os.Getuid() == 0 {
		t.Skip("als root ist jede Datei lesbar")
	}
	locked := filepath.Join(dir, "gesperrt.yaml")
	if err := os.WriteFile(locked, []byte("services: {}\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	result = Load(locked)
	if len(result.Problems) != 1 || result.Problems[0].Path != "file" || !strings.Contains(result.Problems[0].Message, "keine Leserechte") {
		t.Errorf("ohne Rechte: %+v", result.Problems)
	}
}

// Das JSON hat die Form des Vertrags: services als Objekt in
// Dateireihenfolge, disabled nur wo wahr, problems nie null, services {} bei
// einem error.
func TestResultJSONHatDieFormDesVertrags(t *testing.T) {
	_, home := isolate(t)
	data, err := json.Marshal(Load(testdata("valid.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	wantOrder := `"services":{"website":{"repo":"acme/website","access":[{"type":"gh","login":"alice"}]},"backend":{"repo":"acme-labs/backend","access":[{"type":"ssh","key":"` + filepath.Join(home, ".ssh", "id_ed25519_acme_labs") + `"},{"type":"token","file":"` + filepath.Join(home, ".config", "github-tokens", "acme-labs.token") + `","disabled":true}]},"playbook":{"repo":"example/k-playbook","access":[{"type":"ssh","host":"github-example"},{"type":"public"}]}},"problems":[]}`
	if !strings.HasSuffix(text, wantOrder) || !strings.HasPrefix(text, `{"file":"testdata/valid.yaml","created":false,`) {
		t.Errorf("JSON: %s", text)
	}

	data, _ = json.Marshal(Load(testdata("relative-path.yaml")))
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if string(parsed["services"]) != "{}" {
		t.Errorf("services bei error: %s", parsed["services"])
	}
	var problems []map[string]string
	if err := json.Unmarshal(parsed["problems"], &problems); err != nil || len(problems) != 1 || len(problems[0]) != 4 {
		t.Errorf("problems: %s (%v)", parsed["problems"], err)
	}
	if problems[0]["level"] != "error" || problems[0]["path"] != "services.backend.access[1]" || problems[0]["message"] != "Der Pfad in file ist relativ." || problems[0]["hint"] != "Absoluten Pfad oder ~/… eintragen." {
		t.Errorf("Problem: %v", problems[0])
	}
}

// Path: Override wie gesetzt, sonst XDG_CONFIG_HOME (nur absolut), sonst
// ~/.config/k-playbook/github.yaml.
func TestPathFolgtOverrideXDGUndStandard(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(EnvConfig, "/mnt/zugang/github.yaml")
	t.Setenv("XDG_CONFIG_HOME", "/etc/xdg")
	if got, err := Path(); err != nil || got != "/mnt/zugang/github.yaml" {
		t.Errorf("Override: %q %v", got, err)
	}

	t.Setenv(EnvConfig, "")
	if got, _ := Path(); got != filepath.Join("/etc/xdg", "k-playbook", "github.yaml") {
		t.Errorf("XDG: %q", got)
	}

	t.Setenv("XDG_CONFIG_HOME", "relativ")
	if got, _ := Path(); got != filepath.Join(home, ".config", "k-playbook", "github.yaml") {
		t.Errorf("relatives XDG: %q", got)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	if got, _ := Path(); got != filepath.Join(home, ".config", "k-playbook", "github.yaml") {
		t.Errorf("Standard: %q", got)
	}
}
